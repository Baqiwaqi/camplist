import { openDatabase } from './db.mjs';
import { OfflinePacking } from './packing.mjs';
import { transport, downloadJSON } from './transport.mjs';

let modulePromise;
function module() {
 return modulePromise ||= openDatabase().then(db=>({db,packing:new OfflinePacking(db,transport)}));
}
function message(text) {
 const banner=document.getElementById('request-error');if(banner){(document.getElementById('request-error-message')||banner).textContent=text;banner.hidden=false;}
}
async function readyForOffline() {
 if (!('serviceWorker' in navigator)) throw new Error('This browser cannot save sessions for offline use.');
 await navigator.serviceWorker.register('/sw.js');
 const registration=await Promise.race([navigator.serviceWorker.ready,new Promise((_,reject)=>setTimeout(()=>reject(new Error('Offline files are not ready. Try again while connected.')),15000))]);
 await new Promise((resolve,reject)=>{
  const channel=new MessageChannel();
  const timer=setTimeout(()=>{channel.port1.close();reject(new Error('Could not verify offline files.'));},5000);
  channel.port1.onmessage=event=>{clearTimeout(timer);channel.port1.close();event.data.ready?resolve():reject(new Error('Offline files could not be saved.'));};
  registration.active.postMessage('offline-ready',[channel.port2]);
 });
}

document.addEventListener('click',async event=>{
 const button=event.target.closest('[data-save-offline]');if(!button)return;
 button.disabled=true;
 try {
  await readyForOffline();
  const identity=await transport.identity();
  const session=await transport.getSession(identity.userId,button.dataset.saveOffline);
  const {packing}=await module();await packing.save(identity.userId,session);
  location.assign('/offline#'+encodeURIComponent(session.id));
 }catch(error){message('Not saved for offline use. '+error.message);button.disabled=false;}
});

// Ask before removing a local queue. Export and sync are explicit choices.
document.addEventListener('submit',async event=>{
 const form=event.target;if(!form.matches('[data-signout]'))return;
 event.preventDefault();
 try {
  const identity=await transport.identity();
  const {db,packing}=await module();
  const records=await db.list(identity.userId);
  const pending=records.some(record=>Object.keys(record.pending).length);
  const finish=async exported=>{
   await packing.forget(identity.userId,exported);
   form.querySelector('[name="_csrf"]').value=identity.csrfToken;
   form.submit();
  };
  if(!pending){await finish();return;}
  const dialog=document.createElement('dialog');
  const explanation=document.createElement('p');explanation.textContent='This device has packing changes waiting to sync. Keep a copy or synchronize them before signing out.';dialog.append(explanation);
  for(const [label,action] of [
   ['Cancel',async()=>dialog.remove()],
   ['Export and sign out',async()=>{
    const exported=await db.list(identity.userId);
    downloadJSON(JSON.stringify({format:1,sessions:exported},null,2),'camplist-pending.json');
    await finish(exported);
   }],
   ['Sync and sign out',async()=>{
    for(const record of records)await packing.sync(identity.userId,record.id);
    await finish();
   }]
  ]){
   const button=document.createElement('button');button.textContent=label;button.className='btn btn-primary';
   button.onclick=async()=>{button.disabled=true;try{await action();}catch(error){explanation.textContent=error.message;button.disabled=false;}};dialog.append(button);
  }
  document.body.append(dialog);dialog.showModal();
  }catch(error){
  const dialog=document.createElement('dialog');
  const explanation=document.createElement('p');
  explanation.textContent='Could not inspect or clear offline copies: '+error.message+' You can sign out of the server now, but local copies may remain on this device. Return to Saved offline to export and remove them when storage is available.';
  dialog.append(explanation);
  for(const [label,action] of [['Cancel',()=>dialog.remove()],['Sign out; keep local copies',()=>form.submit()]]){
   const button=document.createElement('button');button.className='btn btn-secondary';button.textContent=label;button.onclick=action;dialog.append(button);
  }
  document.body.append(dialog);dialog.showModal();
 }

});
async function synchronizeSaved() {
 try {
  const identity=await transport.identity();const {db,packing}=await module();await db.setOwner(identity.userId);
  for(const record of await db.list(identity.userId))if(Object.keys(record.pending).length)await packing.sync(identity.userId,record.id);
 }catch {/* Offline storage is optional for the normal online app. */}
}
synchronizeSaved();
window.addEventListener('online',synchronizeSaved);
