import { packingStatus } from './status.mjs';
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
  const state=packingStatus(view,navigator.onLine);
  status.textContent=state.text;
  status.classList.toggle('card',view.session.shared&&state.warning);
  if(!view.session.shared&&!view.issue&&!view.pending&&!Object.keys(view.conflicts).length&&navigator.onLine){
   status.textContent+=filesReady?' · Available offline on this device':filesError?' · Offline reopening unavailable in this browser':' · Preparing offline access…';
  }
  const checklist=document.getElementById('packing-checklist');
  for(const item of view.session.list.items){
   const button=document.getElementById('pack-'+item.id);if(!button)continue;
   button.classList.toggle('packed',item.checked);
   button.setAttribute('aria-pressed',String(item.checked));
   button.querySelector('.pack-action').textContent=item.checked?'Unpack':'Pack';
   button.form.elements.checked.value=String(!item.checked);
   button.form.elements.expectedRevision.value=String(item.revision||0);
   const row=button.closest('li');row.querySelector('[data-conflict]')?.remove();
   const remote=view.conflicts[item.id];
   if(remote){
    const conflict=document.createElement('div');conflict.dataset.conflict='';
    const explanation=document.createElement('p');explanation.textContent=`${remote.changedBy||'Another camper'} marked this ${remote.checked?'packed':'unpacked'}. Your waiting change would mark it ${item.checked?'packed':'unpacked'}.`;conflict.append(explanation);
    for(const [label,choice] of [['Keep shared state','server'],[item.checked?'Mark packed instead':'Mark unpacked instead','mine']]){
     const action=document.createElement('button');action.type='button';action.className='btn btn-secondary btn-sm';action.textContent=label;
     action.onclick=async()=>{try{await packing.resolve(session.accountId||session.userId,session.id,item.id,choice);await automatic.notify();await automatic.sync();}catch(error){render(null,error);}};
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
 document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible')automatic.sync().catch(error=>render(null,error));});
 window.addEventListener('pagehide',()=>automatic.stop(),{once:true});
 window.addEventListener('pageshow',event=>{if(event.persisted)location.reload();});
}
