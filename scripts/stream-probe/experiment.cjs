const {chromium}=require('playwright');
const fs=require('fs');
const access=JSON.parse(fs.readFileSync('/tmp/camplist-probe-access.json','utf8'));
const base='https://camplist-stream-probe.yellowcliff-1d686ee3.westeurope.azurecontainerapps.io';
const result={started:new Date().toISOString(),checks:[],latencies:[],stats:[]};
const save=()=>fs.writeFileSync('/tmp/camplist-probe-results.json',JSON.stringify(result,null,2));
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const assert=(value,message)=>{if(!value)throw new Error(message);result.checks.push(message);console.log('PASS',message);save();};
async function control(path){const r=await fetch(base+'/__probe/'+path,{method:'POST',headers:{'X-Probe-Secret':access.secret}});if(!r.ok)throw new Error('control '+path.split('?')[0]+' '+r.status);return r.status===204?null:r.json();}
async function stat(label){const s=await control('stats');result.stats.push({label,at:new Date().toISOString(),...s});save();console.log('STATS',label,JSON.stringify(s));}
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});
 const owner=await browser.newContext();const member=await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true});
 const login=async(ctx,account,ttl=3600)=>{const r=await ctx.request.post(base+'/__probe/login?account='+account+'&ttl='+ttl,{headers:{'X-Probe-Secret':access.secret}});if(!r.ok())throw new Error('login '+r.status());};
 await control('seed');await login(owner,'owner');await login(member,'member');
 const a=await owner.newPage(),b=await member.newPage();
 for(const p of[a,b])p.on('pageerror',e=>console.log('PAGEERROR',e.message));
 await a.addInitScript(()=>sessionStorage.probeSilent='1');
 await Promise.all([a.goto(base+'/packing-session/'+access.owner+'-trip'),b.goto(base+'/packing-session/'+access.owner+'-trip')]);
 for(const p of[a,b])await p.waitForFunction(()=>window.__probe?.opened>=1&&document.querySelector('#packing-save-status')?.textContent.includes('All changes saved'),{},{timeout:60000});
 assert(true,'two isolated browser accounts connected through ACA');
 const ids=await a.locator('#packing-checklist button[id^="pack-"]').evaluateAll(nodes=>nodes.map(n=>n.id));
 const state=async(p,index)=>p.locator('#'+ids[index]).getAttribute('aria-pressed');
 const waitState=(p,index,value,timeout=10000)=>p.waitForFunction(({id,value})=>document.getElementById(id)?.getAttribute('aria-pressed')===String(value),{id:ids[index],value},{timeout});
 const toggle=async(p,other,index)=>{const next=(await state(p,index))!=='true';const start=Date.now();await p.locator('#'+ids[index]).click();await waitState(other,index,next);const ms=Date.now()-start;result.latencies.push(ms);console.log('LATENCY',ms);save();};
 await stat('start');
 // Silent owner stream and heartbeat member stream remain connected without edits.
 for(let minute=1;minute<=5;minute++){await sleep(60000);console.log('MINUTE',minute);result['minute'+minute]={owner:await a.evaluate(()=>({...window.__probe,start:undefined,stop:undefined})),member:await b.evaluate(()=>({...window.__probe,start:undefined,stop:undefined}))};save();}
 await stat('five-minute-quiet');
 for(let i=0;i<4;i++)await toggle(a,b,0);
 await Promise.all([a.locator('#'+ids[0]).click(),b.locator('#'+ids[1]).click()]);await waitState(a,1,true);await waitState(b,0,true);assert(true,'simultaneous edits on different items converge');
 await member.setOffline(true);await b.locator('#'+ids[1]).click();await sleep(500);assert((await state(b,1))==='false','offline change remains visible locally');
 await member.setOffline(false);await waitState(a,1,false,15000);assert(true,'offline queue synchronizes on reconnect');
 await control('drop?value=1');const next=(await state(a,0))!=='true';await a.locator('#'+ids[0]).click();await waitState(b,0,next,75000);assert(true,'lost notification recovers through fallback polling');await control('drop?value=0');
 await stat('after-edits');
 // Exercise explicit background lifecycle hook and same adapter reconnect.
 await b.evaluate(()=>{Object.defineProperty(document,'visibilityState',{configurable:true,value:'hidden'});document.dispatchEvent(new Event('visibilitychange'));});await sleep(1000);
 assert((await control('stats')).connections===1,'simulated background closes member SSE connection');
 await toggle(a,a,0);
 await b.evaluate(()=>{Object.defineProperty(document,'visibilityState',{configurable:true,value:'visible'});document.dispatchEvent(new Event('visibilitychange'));});await waitState(b,0,(await state(a,0))==='true');assert(true,'simulated foreground refreshes current state');
 // Reauthenticate before production cookie decoder age limit, explicitly exercising reconnect.
 await login(owner,'owner');await login(member,'member');
 const elapsed=Date.now()-Date.parse(result.started);if(elapsed<665000){console.log('WAITING_FOR_TEN_MINUTES');await sleep(665000-elapsed);}
 assert(true,'two browsers exercised over more than ten minutes');await stat('ten-minute');
 fs.writeFileSync('/tmp/camplist-probe-ready-for-revision','ready');
 console.log('READY_FOR_REVISION');
 // Parent deploys same code as new revision while the clients remain open.
 for(let i=0;i<180&&!fs.existsSync('/tmp/camplist-probe-revision-done');i++)await sleep(1000);
 if(!fs.existsSync('/tmp/camplist-probe-revision-done'))throw new Error('revision trigger missing');
 await login(owner,'owner');await login(member,'member');
 await a.evaluate(()=>{window.__probe.stop();window.__probe.start()});await b.evaluate(()=>{window.__probe.stop();window.__probe.start()});
 await toggle(a,b,0);assert(true,'reconciliation survives revision replacement');
 // Short expiry is signed fixture policy; production auth and API are still exercised.
 await login(member,'member',8);await b.reload();await b.waitForFunction(()=>window.__probe?.opened>=1);await sleep(17000);
 assert((await b.locator('#packing-save-status').textContent()).includes('Sign in'),'expired fixture session produces sign-in state');
 await login(member,'member');await b.reload();await b.waitForFunction(()=>window.__probe?.opened>=1);
 await member.setOffline(true);await b.locator('#'+ids[1]).click();await sleep(500);await control('revoke');await member.setOffline(false);
 await b.waitForFunction(()=>document.querySelector('#packing-save-status').textContent.includes('access was removed'),{},{timeout:20000});assert(true,'revocation stops uploads and shows access-removed status');
 const exported=await b.evaluate(async()=>{const {openDatabase}=await import('/static/offline/db.mjs');const db=await openDatabase();const records=await db.list(await db.owner());return records.map(r=>({pending:Object.keys(r.pending).length,issue:r.issue}));});assert(exported.some(r=>r.pending>0&&r.issue==='access_removed'),'revoked member retains exportable pending work');
 await login(member,'stranger');const denied=await member.request.get(base+'/api/sessions/'+access.owner+'-trip');assert([403,404].includes(denied.status()),'different account cannot read the trip');
 const cross=await owner.request.get(base+'/api/probe/events',{headers:{Origin:'https://untrusted.example'}});assert(cross.status()===403,'cross-origin stream request rejected');
 const anon=await browser.newContext();const unauth=await anon.request.get(base+'/api/probe/events');assert(unauth.status()===401,'unauthenticated stream request rejected');await anon.close();
 await stat('before-close');await browser.close();await sleep(1000);await stat('clients-closed');result.finished=new Date().toISOString();save();console.log('COMPLETE');
})().catch(e=>{result.error=e.message;save();console.error('FAIL',e.message);process.exit(1)});
