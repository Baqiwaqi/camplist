// Experiment adapter: notifications feed the existing durable sync module.
let source;
window.__probe={events:0,opened:0,errors:0,blocked:null,refreshes:0};
function start(){
 if(source||!document.getElementById('packing-snapshot')||document.visibilityState!=='visible')return;
 source=new EventSource('/api/probe/events'+(sessionStorage.probeSilent==='1'?'?silent=1':''));
 source.onopen=()=>window.__probe.opened++;
 source.onerror=()=>window.__probe.errors++;
 for(const type of ['ready','changed'])source.addEventListener(type,()=>{window.__probe.events++;window.dispatchEvent(new Event('probe:refresh'));});
 for(const type of ['signin','access_removed'])source.addEventListener(type,()=>{window.__probe.blocked=type;stop();window.dispatchEvent(new Event('probe:refresh'));});
}
function stop(){source?.close();source=null;}
window.addEventListener('online',start);window.addEventListener('offline',stop);
document.addEventListener('visibilitychange',()=>document.visibilityState==='visible'?start():stop());
window.addEventListener('pagehide',stop);
window.__probe.start=start;window.__probe.stop=stop;
start();
