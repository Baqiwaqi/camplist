import { AutomaticPacking } from './automatic.mjs';

// Enhance the existing forms only after local persistence succeeds. Without it,
// the server-rendered packing forms continue to work while connected.
export async function mountSession(packing, readyForOffline) {
 const snapshot=document.getElementById('packing-snapshot');
 if(!snapshot)return;
 const session=JSON.parse(snapshot.textContent);
 const status=document.getElementById('packing-save-status');
 let ready=false,filesReady=false,filesError=false;
 function render(view,error) {
  if(error){status.textContent='Could not save on this device. '+error.message;return;}
  if(!view)return;
  const issues={signin:'Sign in again to sync. Your changes are saved on this device.',account:'Sign in to the original account to sync. Your changes are saved on this device.',deleted:'This trip was deleted online. Your local copy is available under On this device.'};
  status.textContent=issues[view.issue] || (Object.keys(view.conflicts).length ? 'Another device changed an item. Choose which version to keep below.' :
   !navigator.onLine||view.issue==='network' ? 'Offline · Changes saved on this device. We’ll sync automatically when connected.' :
   view.pending ? 'Saved on this device · Syncing…' :
   filesReady ? 'All changes saved · Available offline on this device' :
   filesError ? 'Changes saved · Offline reopening unavailable in this browser' : 'All changes saved · Preparing offline access…');
  const checklist=document.getElementById('packing-checklist');
  for(const item of view.session.list.items){
   const button=document.getElementById('pack-'+item.id);if(!button)continue;
   button.classList.toggle('packed',item.checked);
   button.setAttribute('aria-pressed',String(item.checked));
   button.querySelector('.pack-action').textContent=item.checked?'Unpack':'Pack';
   button.form.elements.checked.value=String(!item.checked);
   const row=button.closest('li');row.querySelector('[data-conflict]')?.remove();
   const remote=view.conflicts[item.id];
   if(remote){
    const conflict=document.createElement('div');conflict.dataset.conflict='';
    const explanation=document.createElement('p');explanation.textContent=`This device: ${item.checked?'packed':'unpacked'}. Other device: ${remote.checked?'packed':'unpacked'}.`;conflict.append(explanation);
    for(const [label,choice] of [['Keep this device’s choice','mine'],['Use other device’s choice','server']]){
     const action=document.createElement('button');action.type='button';action.className='btn btn-secondary btn-sm';action.textContent=label;
     action.onclick=async()=>{try{await packing.resolve(session.userId,session.id,item.id,choice);await automatic.notify();await automatic.sync();}catch(error){render(null,error);}};
     conflict.append(action);
    }
    row.append(conflict);
   }
  }
  const total=view.session.list.items.length,checked=view.session.list.items.filter(item=>item.checked).length;
  const complete=total>0&&checked===total;
  checklist.querySelector('.progress-count').textContent=`${checked} of ${total} items packed`;
  checklist.querySelector('.progress-track').setAttribute('aria-valuenow',String(checked));
  checklist.querySelector('.progress-fill').style.width=`${total?checked/total*100:0}%`;
  checklist.querySelector('.progress').classList.toggle('progress-done',complete);
  checklist.querySelector('.progress-card').classList.toggle('card-brand',complete);
  const label=checklist.querySelector('.progress-label, .display');
  if(label)label.textContent=complete?'All packed. Go!':'Keep packing';
 }
 const automatic=new AutomaticPacking(packing,render);
 try {
  await automatic.save(session);ready=true;
  readyForOffline().then(()=>{filesReady=true;return automatic.notify();}).catch(()=>{filesError=true;automatic.notify().catch(error=>render(null,error));});
 }catch(error){status.textContent='Offline saving unavailable. Packing requires a connection. '+error.message;return;}
 document.addEventListener('submit',async event=>{
  const form=event.target;
  if(!ready||!form.matches('#packing-checklist .pack-form'))return;
  event.preventDefault();event.stopImmediatePropagation();
  try {await automatic.set(form.elements.itemId.value,form.elements.checked.value==='true');}
  catch(error){await automatic.notify();render(null,error);}
 },true);
 window.addEventListener('pagehide',()=>automatic.stop(),{once:true});
 window.addEventListener('pageshow',event=>{if(event.persisted)location.reload();});
}
