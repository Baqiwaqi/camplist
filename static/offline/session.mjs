import {renderEntries,updateCategories,entryFromForm,renderFutureSaves} from './checklist.mjs';
import { packingStatus, tripGone } from './status.mjs';
import { AutomaticPacking } from './automatic.mjs';

// Enhance the existing forms only after local persistence succeeds. Without it,
// the server-rendered packing forms continue to work while connected.
export async function mountSession(packing, readyForOffline) {
 const snapshot=document.getElementById('packing-snapshot');
 if(!snapshot)return;
 const session=JSON.parse(snapshot.textContent);
 const status=document.getElementById('packing-save-status');
 const owner=session.accountId||session.userId;
 let ready=false,filesReady=false,filesError=false,removed=false;
 // Disable packing while the trip is gone; re-enable only what this disabled.
 function lock(locked){
  for(const control of document.querySelectorAll('#packing-checklist button,#session-preparation button,#session-preparation input,#trip-entry-form input,#trip-entry-form select,#trip-entry-form button')){
   if(locked&&!control.disabled){control.disabled=true;control.dataset.goneLocked='';}
   else if(!locked&&'goneLocked' in control.dataset){control.disabled=false;delete control.dataset.goneLocked;}
  }
 }
 // The copy disappeared: this device dropped it because the trip is gone, or
 // another tab removed it. Ask the server which, then stop offering packing.
 async function missing(){
  if(removed||await packing.open(owner,session.id))return;
  removed=true;automatic.stop();lock(true);
  let reason='';
  try{await packing.transport.getSession(owner,session.id);}catch(error){reason=error.code==='access_removed'?'access_removed':error.status===404?'deleted':'';}
  status.textContent=reason==='deleted'?'This trip was deleted. Packing is no longer available.':reason==='access_removed'?'Your access to this trip was removed. Packing is no longer available.':'This trip is no longer saved on this device. Reload the page to save it again.';
  status.classList.toggle('card',Boolean(session.shared));
 }
 function render(view,error) {
  if(removed)return;
  if(error||!view)missing().catch(()=>{});
  if(error){status.textContent='Could not save on this device. '+error.message;return;}
  if(!view)return;
  const state=packingStatus(view,navigator.onLine);
  status.textContent=state.text;
  status.classList.toggle('card',view.session.shared&&state.warning);
  if(!view.session.shared&&!view.issue&&!view.pending&&!Object.keys(view.conflicts).length&&navigator.onLine){
   status.textContent+=filesReady?' · Available offline on this device':filesError?' · Offline reopening unavailable in this browser':' · Preparing offline access…';
  }
  let future=document.getElementById('future-save-status');if(!future){future=document.createElement('div');future.id='future-save-status';status.after(future);}
  renderFutureSaves(future,view,async id=>{await packing.cancelFutureSave(owner,session.id,id);await automatic.notify();});
  const checklist=document.getElementById('packing-checklist');
  const toggle=async(id,checked)=>{try{await automatic.set(id,checked);}catch(error){render(null,error);}};
  const resolve=async(id,choice)=>{try{await packing.resolve(owner,session.id,id,choice);await automatic.notify();await automatic.sync();}catch(error){render(null,error);}};
  let entries=checklist.querySelector('[data-trip-items]');
  if(!entries){entries=document.createElement('div');entries.dataset.tripItems='';const card=checklist.querySelector('.card:not(.progress-card)');card.replaceChildren(entries);}
  renderEntries(entries,view,'',toggle,resolve);
  let preparation=document.querySelector('#session-preparation [data-trip-tasks]');
  if(!preparation){const fallback=document.querySelector('#session-preparation .item-list');if(fallback){preparation=document.createElement('div');preparation.dataset.tripTasks='';fallback.replaceWith(preparation);}}
  if(preparation)renderEntries(preparation,view,'task',toggle,resolve);
  updateCategories(document.getElementById('trip-categories'),view.session);
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
  lock(tripGone(view));
 }
 const automatic=new AutomaticPacking(packing,render);
 try {
  await automatic.save(session);ready=true;
  readyForOffline().then(()=>{filesReady=true;return automatic.notify();}).catch(()=>{filesError=true;automatic.notify().catch(error=>render(null,error));});
 }catch(error){status.textContent='Offline saving unavailable. Packing requires a connection. '+error.message;return;}
 document.addEventListener('submit',async event=>{
  const form=event.target;
  if(!ready||removed)return;
  if(form.matches('#trip-entry-form')){event.preventDefault();event.stopImmediatePropagation();try{await automatic.add(entryFromForm(form));form.reset();}catch(error){render(null,error);}return;}
  if(!form.matches('#packing-checklist .pack-form'))return;
  event.preventDefault();event.stopImmediatePropagation();
  try {await automatic.set(form.elements.itemId.value,form.elements.checked.value==='true');}
  catch(error){await automatic.notify();render(null,error);}
 },true);
 // The device view owns the checklist once mounted. A server swap still in
 // flight from before would overwrite pending state, so fetch it instead.
 document.addEventListener('htmx:oobBeforeSwap',event=>{
  if(!event.target.closest('#packing-checklist'))return;
  event.detail.shouldSwap=false;
  if(!removed)automatic.sync().catch(error=>render(null,error));
 });
 // The heading is server HTML; a rename swaps it. Refresh the saved copy's name.
 document.addEventListener('camplist:trip-renamed',()=>{if(!removed)automatic.sync().catch(error=>render(null,error));});
 document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible'&&!removed)automatic.sync().catch(error=>render(null,error));});
 window.addEventListener('pagehide',()=>automatic.stop(),{once:true});
 window.addEventListener('pageshow',event=>{if(event.persisted)location.reload();});
}
