const fs=require('fs'),{execFile}=require('child_process'),{promisify}=require('util');const run=promisify(execFile);const access=JSON.parse(fs.readFileSync('/tmp/camplist-probe-access.json','utf8'));const base='https://camplist-stream-probe.yellowcliff-1d686ee3.westeurope.azurecontainerapps.io';const sleep=ms=>new Promise(r=>setTimeout(r,ms));
async function stats(){const r=await fetch(base+'/__probe/stats',{method:'POST',headers:{'X-Probe-Secret':access.secret}});if(!r.ok)throw Error('stats '+r.status);return r.json();}
(async()=>{
 while(!fs.existsSync('/tmp/camplist-probe-ready-for-revision'))await sleep(1000);
 const report={started:new Date().toISOString(),before:await stats()};console.log('ROLLOUT_STARTED');
 await run('az',['containerapp','update','-g','rg-camplist-prod','-n','camplist-stream-probe','--image','camplistjefeoxrtdy32k.azurecr.io/stream-probe:recheck60','--set-env-vars','PROBE_RECHECK_SECONDS=60','--only-show-errors','-o','none']);
 for(let i=0;i<12;i++){await sleep(5000);try{const s=await stats();report.after=s;if(s.replica!==report.before.replica&&s.opened>=2){report.automaticReconnect=true;break}}catch{}}
 report.finished=new Date().toISOString();fs.writeFileSync('/tmp/camplist-probe-rollout.json',JSON.stringify(report,null,2));fs.writeFileSync('/tmp/camplist-probe-revision-done','ready');console.log('ROLLOUT_FINISHED',JSON.stringify(report));
})().catch(e=>{console.error(e.message);process.exit(1)});
