import {renderEntries,updateCategories,entryFromForm,renderFutureSaves} from './checklist.mjs';
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
  let future=document.getElementById('future-save-status');if(!future){future=document.createElement('div');future.id='future-save-status';status.after(future);}
  renderFutureSaves(future,view,async id=>{await packing.cancelFutureSave(session.accountId||session.userId,session.id,id);await automatic.notify();});
  const checklist=document.getElementById('packing-checklist');
  const toggle=async(id,checked)=>{try{await automatic.set(id,checked);}catch(error){render(null,error);}};
  const resolve=async(id,choice)=>{try{await packing.resolve(session.accountId||session.userId,session.id,id,choice);await automatic.notify();await automatic.sync();}catch(error){render(null,error);}};
  let entries=checklist.querySelector('[data-trip-items]');
  if(!entries){entries=document.createElement('div');entries.dataset.tripItems='';const card=checklist.querySelector('.card:not(.progress-card)');card.replaceChildren(entries);}
  renderEntries(entries,view,'',toggle,resolve);
  let preparation=document.querySelector('#session-preparation [data-trip-tasks]');
  if(!preparation){const fallback=document.querySelector('#session-preparation .item-list');if(fallback){preparation=document.createElement('div');preparation.dataset.tripTasks='';fallback.replaceWith(preparation);}}
  if(preparation)renderEntries(preparation,view,'task',toggle,resolve);
  updateCategories(document.getElementById('trip-categories'),view.session.list.items);
  const heading=document.querySelector('h1');if(heading)heading.textContent=view.session.name||view.session.list.name;
  const gear=view.session.list.items.filter(item=>item.kind!=="task"),total=gear.length,checked=gear.filter(item=>item.checked).length;
  const complete=total>0&&checked===total;
  const count=`${checked} of ${total} items packed`;
  checklist.querySelector('.progress-count').textContent=count;
  // The live region sits outside the checklist; only a changed count is announced.
  const announcement=document.getElementById('packing-progress-status');
  if(announcement&&announcement.textContent!==count)announcement.textContent=count;
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
  if(!ready)return;
  if(form.matches('#trip-entry-form')){event.preventDefault();event.stopImmediatePropagation();try{await automatic.add(entryFromForm(form));form.reset();}catch(error){render(null,error);}return;}
  if(!form.matches('#packing-checklist .pack-form'))return;
  event.preventDefault();event.stopImmediatePropagation();
  try {await automatic.set(form.elements.itemId.value,form.elements.checked.value==='true');}
  catch(error){await automatic.notify();render(null,error);}
 },true);
 document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible')automatic.sync().catch(error=>render(null,error));});
 window.addEventListener('pagehide',()=>automatic.stop(),{once:true});
 window.addEventListener('pageshow',event=>{if(event.persisted)location.reload();});
}
