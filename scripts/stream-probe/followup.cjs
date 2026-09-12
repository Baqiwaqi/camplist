const {chromium}=require('playwright');const fs=require('fs');
const access=JSON.parse(fs.readFileSync('/tmp/camplist-probe-access.json','utf8'));
const base='https://camplist-stream-probe.yellowcliff-1d686ee3.westeurope.azurecontainerapps.io';
const trip=access.owner+'-trip';const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const result={started:new Date().toISOString(),checks:[]};const save=()=>fs.writeFileSync('/tmp/camplist-probe-followup.json',JSON.stringify(result,null,2));
const assert=(ok,msg)=>{if(!ok)throw Error(msg);result.checks.push(msg);console.log('PASS',msg);save()};
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});
 const c1=await browser.newContext(),c2=await browser.newContext();
 const login=async(c,account)=>{const r=await c.request.post(base+'/__probe/login?account='+account,{headers:{'X-Probe-Secret':access.secret}});assert(r.ok(),'fixture sign-in '+account)};
 await login(c1,'owner');await login(c2,'member');
 const post=async(c,url,form={})=>{const identity=await (await c.request.get(base+'/api/identity')).json();const r=await c.request.post(url,{headers:{Origin:base,Referer:base+'/'},form:{_csrf:identity.csrfToken,...form}});if(!r.ok())throw Error('sharing action '+r.status());return r;};
 const invitation=await post(c1,base+'/sharing/packing-session/'+trip+'/invitations');
 const html=await invitation.text();const match=html.match(/id="invitation-link"[^>]*value="([^"]+)"/);if(!match)throw Error('invitation missing');
 const link=match[1];await post(c2,link);
 const hash=require('crypto').createHash('sha256').update(link.split('/').pop()).digest('base64url');
 await post(c1,base+'/sharing/packing-session/'+trip+'/invitations/'+hash,{decision:'approve'});
 assert(true,'member rejoins through invitation request and owner approval');
 const a=await c1.newPage(),b=await c2.newPage();await Promise.all([a.goto(base+'/packing-session/'+trip),b.goto(base+'/packing-session/'+trip)]);
 for(const p of[a,b])await p.waitForFunction(()=>window.__probe?.opened>=1&&document.querySelector('#packing-save-status')?.textContent.includes('All changes saved'),{},{timeout:60000});
 const stats=async()=>{const r=await fetch(base+'/__probe/stats',{method:'POST',headers:{'X-Probe-Secret':access.secret}});return r.json()};
 await sleep(2000);result.quietStart=await stats();console.log('QUIET_COST_SAMPLE_STARTED');await sleep(65000);result.quietEnd=await stats();save();console.log('QUIET_COST_SAMPLE',JSON.stringify({start:result.quietStart,end:result.quietEnd}));
 const id=await a.locator('#packing-checklist button[id^="pack-"]').first().getAttribute('id');
 const val=async p=>(await p.locator('#'+id).getAttribute('aria-pressed'))==='true';
 const wait=(p,value)=>p.waitForFunction(({id,value})=>document.getElementById(id).getAttribute('aria-pressed')===String(value),{id,value},{timeout:15000});
 const initial=await val(a);await c2.setOffline(true);await b.locator('#'+id).click();
 await a.locator('#'+id).click();await wait(a,!initial);await a.waitForFunction(()=>document.querySelector('#packing-save-status').textContent.includes('All changes saved'));
 await a.locator('#'+id).click();await wait(a,initial);await a.waitForFunction(()=>document.querySelector('#packing-save-status').textContent.includes('All changes saved'));
 await c2.setOffline(false);await b.waitForSelector('[data-conflict]',{timeout:20000});assert(true,'stale disagreement requires explicit conflict review');
 await b.getByRole('button',{name:'Keep shared state',exact:true}).click();await wait(b,initial);assert(await val(a)===initial,'keeping shared state preserves server value');
 // Change signed account while a device has a pending local intent.
 await c2.setOffline(true);await b.locator('#'+id).click();await sleep(500);await c2.route('**/api/sessions/*/sync',route=>route.abort());await c2.setOffline(false);
 // Block outgoing writes before changing account to keep the intent pending.
 await login(c2,'stranger');
 const accountIssue=await b.evaluate(async({owner,trip})=>{const {openDatabase}=await import('/static/offline/db.mjs');const {OfflinePacking}=await import('/static/offline/packing.mjs');const {transport}=await import('/static/offline/transport.mjs');const db=await openDatabase();return new OfflinePacking(db,transport).sync(owner,trip);},{owner:access.owner+'-member',trip});
 assert(accountIssue.issue==='account','account switch blocks synchronization of another account copy');
 await browser.close();result.finished=new Date().toISOString();save();console.log('FOLLOWUP_COMPLETE');
})().catch(e=>{result.error=e.message;save();console.error('FAIL',e.message);process.exit(1)});
