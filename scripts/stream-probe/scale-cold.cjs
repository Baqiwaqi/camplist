const fs=require('fs'),{execFile}=require('child_process'),{promisify}=require('util'),{chromium}=require('playwright');const run=promisify(execFile);const access=JSON.parse(fs.readFileSync('/tmp/camplist-probe-access.json','utf8'));const base='https://camplist-stream-probe.yellowcliff-1d686ee3.westeurope.azurecontainerapps.io';const sleep=ms=>new Promise(r=>setTimeout(r,ms));const result={samples:[]};const save=()=>fs.writeFileSync('/tmp/camplist-probe-scale-cold.json',JSON.stringify(result,null,2));
(async()=>{
 for(;;){try{const r=JSON.parse(fs.readFileSync('/tmp/camplist-probe-followup.json'));if(r.error)throw Error(r.error);if(r.finished)break;}catch(e){if(e.code!=='ENOENT')throw e;}await sleep(1000);}
 result.idleStarted=new Date().toISOString();console.log('SCALE_TO_ZERO_OBSERVATION_STARTED');save();
 for(let i=0;i<21;i++){
  const {stdout}=await run('az',['containerapp','replica','list','-g','rg-camplist-prod','-n','camplist-stream-probe','--query','length(@)','-o','tsv']);const count=Number(stdout.trim());result.samples.push({at:new Date().toISOString(),count});save();console.log('REPLICAS',count);
  if(count===0){result.scaledToZero=true;break};await sleep(30000);
 }
 if(!result.scaledToZero)throw Error('did not scale to zero within observation window');
 const browser=await chromium.launch({headless:true,executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'});const context=await browser.newContext();const start=Date.now();const login=await context.request.post(base+'/__probe/login?account=owner',{headers:{'X-Probe-Secret':access.secret},timeout:90000});if(!login.ok())throw Error('cold login '+login.status());result.coldFirstResponseMs=Date.now()-start;
 const page=await context.newPage();await page.goto(base+'/packing-session/'+access.owner+'-trip');await page.waitForFunction(()=>window.__probe?.opened>=1&&document.querySelector('#packing-save-status')?.textContent.includes('All changes saved'),{},{timeout:60000});result.coldReadyMs=Date.now()-start;result.coldRecovered=true;await browser.close();
 const stats=await fetch(base+'/__probe/stats',{method:'POST',headers:{'X-Probe-Secret':access.secret}});result.coldStats=await stats.json();
 const cleanup=await fetch(base+'/__probe/cleanup',{method:'POST',headers:{'X-Probe-Secret':access.secret}});if(!cleanup.ok)throw Error('cleanup '+cleanup.status);result.cleanup=await cleanup.json();result.finished=new Date().toISOString();save();console.log('COLD_START_COMPLETE',JSON.stringify(result));
})().catch(e=>{result.error=e.message;save();console.error('FAIL',e.message);process.exit(1)});
