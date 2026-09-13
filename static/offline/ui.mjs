import {renderEntries,updateCategories,entryFromForm,renderFutureSaves} from './checklist.mjs';
import { packingStatus } from './status.mjs';
import { openDatabase } from './db.mjs';
import { OfflinePacking } from './packing.mjs';
import { transport, downloadJSON } from './transport.mjs';

const byId=id=>document.getElementById(id);
let db,packing,owner;
const selected=()=>{try{return decodeURIComponent(location.hash.slice(1)||location.pathname.match(/^\/(?:packing-session|trips)\/([^/]+)$/)?.[1]||'');}catch{return '';}};
function message(text){const element=byId('offline-message');element.textContent=text;element.hidden=!text;element.className='error';}
function button(text,action){const element=document.createElement('button');element.textContent=text;element.className='btn btn-secondary btn-sm';element.onclick=async()=>{try{await action();}catch(error){message('Could not save that change. '+error.message);await render();}};return element;}
async function render(){
 const account=owner,id=selected();
 const records=account?await db.list(account):[];
 if(account!==owner)return;
 const list=byId('saved-sessions');list.replaceChildren();
 if(!records.length){const p=document.createElement('p');p.textContent='No trips saved on this device yet. Open a packing session while connected and it will be saved automatically.';list.append(p);}
 for(const record of records){const p=document.createElement('p');const a=document.createElement('a');a.href='#'+encodeURIComponent(record.id);a.textContent=record.session.name||record.session.list.name;p.append(a);list.append(p);}
 const view=account&&id?await packing.open(account,id):null;
 if(account!==owner||id!==selected())return;
 byId('offline-trip').hidden=!view;if(!view)return;
 byId('trip-name').textContent=view.session.name||view.session.list.name;
 renderFutureSaves(byId('future-save-status'),view,async itemId=>{await packing.cancelFutureSave(account,id,itemId);await render();});
 const state=packingStatus(view,navigator.onLine);
 byId('sync-status').textContent=state.text;
 byId('sync-status').classList.toggle('card',view.session.shared&&state.warning);
 const gear=view.session.list.items.filter(item=>item.kind!=='task');
 byId('packing-progress').textContent=`${gear.filter(item=>item.checked).length} of ${gear.length} items packed`;
 byId('review-trip').href='/trips/'+encodeURIComponent(id)+'/review';
 const toggle=async(itemId,checked)=>{try{await packing.set(account,id,itemId,checked);await render();synchronize();}catch(error){message(error.message);}};
 const resolve=async(itemId,choice)=>{try{await packing.resolve(account,id,itemId,choice);await render();await synchronize();}catch(error){message(error.message);}};
 renderEntries(byId('offline-items'),view,'',toggle,resolve);
 renderEntries(byId('offline-tasks'),view,'task',toggle,resolve);
 updateCategories(byId('trip-categories'),view.session);
}
async function synchronize(){
 if(!owner)return;
 const account=owner;
 try {for(const record of await db.list(account)){await packing.sync(account,record.id);if(owner===account)await render();}}
 catch(error){message('Could not synchronize. '+error.message);}
}
async function load(){
 try {
  db ||= await openDatabase();packing ||= new OfflinePacking(db,transport);
  owner=await db.owner();
  try{const identity=await transport.identity();owner=identity.userId;await db.setOwner(owner);}catch(error){if(error.status&&error.status!==401)message('Could not verify the signed-in account. Saved sessions remain local.');}
  for(const record of owner?await db.list(owner):[])await packing.save(owner,record.session);
  await render();await synchronize();
 }catch(error){message('Offline storage is unavailable. '+error.message);}
}
byId('sync-now').onclick=load;
byId('export-session').onclick=async()=>{try{downloadJSON(await packing.export(owner,selected()),'camplist-session.json');}catch(error){message(error.message);}};
window.addEventListener('hashchange',()=>render().catch(error=>message(error.message)));
window.addEventListener('online',load);
window.addEventListener('offline',()=>render().catch(error=>message(error.message)));
// Some outages end without an online event (for example, the server recovers).
setInterval(async()=>{
 if(!db||!owner||document.visibilityState!=='visible')return;
 try{if((await db.list(owner)).some(record=>record.issue==='network'||(!record.issue&&((Object.keys(record.pending).length+Object.keys(record.additions||{}).length+Object.keys(record.futureSaves||{}).length)||record.session.shared))))await synchronize();}
 catch(error){message(error.message);}
},15000);
document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible')load();});
load();

byId('forget-account').onclick=async()=>{
 try {
  if(!owner)return;
  const account=owner,exported=await db.list(account);
  downloadJSON(JSON.stringify({format:1,sessions:exported},null,2),'camplist-pending.json');
  await packing.forget(account,exported);await render();message('Exported and removed this account’s saved copies.');
 }catch(error){message('Copies were not removed. '+error.message);}
};

byId('trip-entry-form').onsubmit=async event=>{event.preventDefault();try{await packing.add(owner,selected(),entryFromForm(event.target));event.target.reset();await render();synchronize();}catch(error){message(error.message);}};
