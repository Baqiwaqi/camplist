import { openDatabase } from './db.mjs';
import { OfflinePacking } from './packing.mjs';
import { transport, downloadJSON } from './transport.mjs';

const byId=id=>document.getElementById(id);
let db,packing,owner;
const selected=()=>{try{return decodeURIComponent(location.hash.slice(1));}catch{return '';}};
function message(text){const element=byId('offline-message');element.textContent=text;element.hidden=!text;element.className='error';}
function button(text,action){const element=document.createElement('button');element.textContent=text;element.className='btn btn-secondary btn-sm';element.onclick=async()=>{try{await action();}catch(error){message('Could not save that change. '+error.message);await render();}};return element;}
async function render(){
 const account=owner,id=selected();
 const records=account?await db.list(account):[];
 if(account!==owner)return;
 const list=byId('saved-sessions');list.replaceChildren();
 if(!records.length){const p=document.createElement('p');p.textContent='No saved sessions for this account. Open a session online and choose “Save for offline packing”.';list.append(p);}
 for(const record of records){const p=document.createElement('p');const a=document.createElement('a');a.href='#'+encodeURIComponent(record.id);a.textContent=record.session.list.name;p.append(a);list.append(p);}
 const view=account&&id?await packing.open(account,id):null;
 if(account!==owner||id!==selected())return;
 byId('offline-trip').hidden=!view;if(!view)return;
 byId('trip-name').textContent=view.session.list.name;
 const issues={signin:'Sign in to synchronize. Your changes are saved on this device.',account:'Sign in to the account that saved this trip. Pending changes are preserved.',deleted:'This session was deleted online. Export your local copy; it will not be recreated.',network:'Connection unavailable. Your changes are saved on this device.'};
 byId('sync-status').textContent=issues[view.issue]||(Object.keys(view.conflicts).length?'Needs a decision: another device changed an item.':view.pending?`Waiting to sync ${view.pending} change(s). Saved on this device.`:'Saved on this device · Synchronized.');
 byId('packing-progress').textContent=`${view.session.list.items.filter(item=>item.checked).length} of ${view.session.list.items.length} items packed`;
 byId('review-trip').href='/packing-session/'+encodeURIComponent(id)+'/review';
 const active=document.activeElement?.id;
 const items=byId('offline-items');items.replaceChildren();
 for(const item of view.session.list.items){
  const row=document.createElement('li');row.className='item-row';
  const label=document.createElement('label');const input=document.createElement('input');input.type='checkbox';input.id='offline-'+item.id;input.checked=item.checked;
  input.onchange=async()=>{try{await packing.set(account,id,item.id,input.checked);message('');await render();synchronize();}catch(error){message('Not saved on this device. '+error.message);await render();}};
  label.append(input,document.createTextNode(' '+item.name+(item.category?' · '+item.category:'')));row.append(label);
  if(view.conflicts[item.id]){
   const conflict=document.createElement('div');const text=document.createElement('p');text.textContent=`This device: ${item.checked?'packed':'unpacked'}. Online: ${view.conflicts[item.id].checked?'packed':'unpacked'}.`;conflict.append(text);
   for(const [label,choice]of[['Keep this device’s choice','mine'],['Use online choice','server']])conflict.append(button(label,async()=>{await packing.resolve(account,id,item.id,choice);await render();await synchronize();}));
   row.append(conflict);
  }
  items.append(row);
 }
 if(active)document.getElementById(active)?.focus({preventScroll:true});
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
  await render();await synchronize();
 }catch(error){message('Offline storage is unavailable. '+error.message);}
}
byId('sync-now').onclick=load;
byId('export-session').onclick=async()=>{try{downloadJSON(await packing.export(owner,selected()),'camplist-session.json');}catch(error){message(error.message);}};
window.addEventListener('hashchange',()=>render().catch(error=>message(error.message)));
window.addEventListener('online',load);
document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='visible')load();});
load();
