// Render only the reconciled device view. Never replace pending state with an HTML push.
export function renderEntries(container,view,kind,onToggle,onResolve) {
 const active=document.activeElement?.id;
 container.replaceChildren();
 const groups=new Map();
 for(const item of view.session.list.items.filter(item=>(item.kind||'')===kind)){
  const key=item.assignee||'';
  if(!groups.has(key))groups.set(key,{name:key?(item.assigneeName||'Participant'):'Shared',items:[]});
  groups.get(key).items.push(item);
 }
 for(const group of groups.values()){
  const section=document.createElement('section');section.className='pack-group';
  const title=document.createElement('h3');title.textContent=group.name;section.append(title);
  const list=document.createElement('ul');list.className='item-list pack-list';section.append(list);
  for(const item of group.items){
   const row=document.createElement('li');row.className='item-row';
   const button=document.createElement('button');button.id='pack-'+item.id;button.type='button';button.className='pack-row'+(item.checked?' packed':'');button.setAttribute('aria-pressed',String(item.checked));
   button.disabled=Boolean(item.assignee&&item.assignee!==(view.session.accountId||view.session.userId));
   button.onclick=()=>onToggle(item.id,!item.checked);
   const tick=document.createElement('span');tick.className='tick';tick.setAttribute('aria-hidden','true');button.append(tick);
   const name=document.createElement('span');name.className='pack-name';name.textContent=item.name+(item.category?' · '+item.category:'');button.append(name);
   const action=document.createElement('span');action.className='pack-action';action.textContent=kind==='task'?(item.checked?'Undo':'Mark done'):(item.checked?'Unpack':'Pack');button.append(action);row.append(button);
   if(item.changedBy){const attribution=document.createElement('span');attribution.className='muted';attribution.textContent='Updated by '+item.changedBy;row.append(attribution);}
   const remote=view.conflicts[item.id];
   if(remote){
    const conflict=document.createElement('div');const explanation=document.createElement('p');explanation.textContent=`${remote.changedBy||'Another camper'} marked this ${remote.checked?'done':'not done'}. Your waiting change would mark it ${item.checked?'done':'not done'}.`;conflict.append(explanation);
    for(const [label,choice] of [['Keep shared state','server'],['Use my change instead','mine']]){
     const action=document.createElement('button');action.type='button';action.className='btn btn-secondary btn-sm';action.textContent=label;action.onclick=()=>onResolve(item.id,choice);conflict.append(action);
    }row.append(conflict);
   }
   list.append(row);
  }container.append(section);
 }
 if(active)document.getElementById(active)?.focus({preventScroll:true});
}
// Replace the trip's categories in the picker's options: the default categories
// the session snapshot carries, then those on its items. Options the server
// rendered (defaults and remembered categories) stay; case and spaces do not
// make a second option.
export function updateCategories(datalist,session) {
 if(!datalist)return;
 for(const option of Array.from(datalist.options))if('trip' in option.dataset)option.remove();
 const key=value=>value.trim().toLowerCase();
 const known=new Set(Array.from(datalist.options,option=>key(option.value)));
 for(const value of [...(session.categories||[]),...session.list.items.map(item=>item.category)]){const category=(value||'').trim();if(!category||known.has(key(category)))continue;known.add(key(category));const option=document.createElement('option');option.value=category;option.dataset.trip='';datalist.append(option);}
}
export function entryFromForm(form){return {name:form.elements.name.value,category:form.elements.category.value,kind:form.elements.kind.value,scope:form.elements.scope.value,saveForFuture:form.elements.saveForFuture.checked};}
export function renderFutureSaves(container,view,onCancel) {
 container.replaceChildren();
 for(const id of view.futureSaves||[]){
  const p=document.createElement('p');p.className='card';const item=view.session.list.items.find(item=>item.id===id);
  p.append(document.createTextNode(`${item?.name||'Entry'} is saved to this trip. Saving it for future trips failed. Check your packing list access; it will retry when you sync. `));
  const cancel=document.createElement('button');cancel.type='button';cancel.className='btn btn-secondary btn-sm';cancel.textContent='Keep only in this trip';cancel.onclick=()=>onCancel(id);p.append(cancel);container.append(p);
 }
}
