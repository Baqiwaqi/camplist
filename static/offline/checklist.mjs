// Render only the reconciled device view. Never replace pending state with an HTML push.
// The markup matches PackingChecklist and SessionPreparation in
// internal/views/packing-session.templ, so the trip page reads the same before
// and after this takes over, and in the offline shell.
export function renderEntries(container,view,kind,onToggle,onResolve) {
 const active=document.activeElement?.id;
 container.replaceChildren();
 const task=kind==='task',viewer=view.session.accountId||view.session.userId,prefix=task?'task-count':'pack-count';
 const entries=view.session.list.items.filter(item=>(item.kind||'')===kind);
 if(!entries.length){const empty=element('p','card text-muted'+(task?' mb-3':''),task?'No preparation tasks on this trip.':'Nothing to pack yet. Add something to this trip below.');container.append(empty);}
 groupByPerson(entries,view.session.shared,viewer).forEach((person,i)=>{
  if(!person.items.length)return;
  if(person.name)container.append(head('h2','person-head',person.name,person.items,task,`${prefix}-${i}`));
  const sheet=element('div','pack-sheet');container.append(sheet);
  (task?[{name:'',items:person.items}]:groupByCategory(person.items)).forEach((group,j)=>{
   const section=element('div','pack-group');sheet.append(section);
   if(group.name)section.append(head(person.name?'h3':'h2','group-head',group.name,group.items,task,`${prefix}-${i}-${j}`));
   const list=element('ul');section.append(list);
   for(const item of group.items)list.append(row(item,view,task,!person.name&&item.assignee===viewer,viewer,onToggle,onResolve));
  });
 });
 if(active)document.getElementById(active)?.focus({preventScroll:true});
}
function element(tag,className,text){const node=document.createElement(tag);if(className)node.className=className;if(text)node.textContent=text;return node;}
// A group's name with its count: "2 of 5", then "All packed" or "All done".
function head(tag,className,name,items,task,id){
 const done=items.filter(item=>item.checked).length,finished=done===items.length;
 const heading=element(tag,className);heading.append(element('span','',name));
 const count=element('span',finished?'group-count group-done':'group-count',finished?(task?'All done':'All packed'):`${done} of ${items.length}`);count.id=id;heading.append(count);
 return heading;
}
function row(item,view,task,mine,viewer,onToggle,onResolve){
 const li=element('li','pack-item');
 const button=element('button','pack-row'+(item.checked?' packed':''));button.id='pack-'+item.id;button.type='button';button.setAttribute('aria-pressed',String(item.checked));
 button.disabled=Boolean(item.assignee&&item.assignee!==viewer);
 button.onclick=()=>onToggle(item.id,!item.checked);
 const tick=element('span','tick');tick.setAttribute('aria-hidden','true');
 const text=element('span','pack-text');text.append(element('span','pack-name',item.name));
 const caption=changedByCaption(item,viewer,task);if(caption)text.append(element('span','pack-by',caption));
 button.append(tick,text);
 if(mine)button.append(element('span','tag tag-quiet','Yours'));
 li.append(button);
 const remote=view.conflicts[item.id];
 if(remote){
  const conflict=element('div');conflict.dataset.conflict='';conflict.append(element('p','',`${remote.changedBy||'Another camper'} marked this ${remote.checked?'done':'not done'}. Your waiting change would mark it ${item.checked?'done':'not done'}.`));
  for(const [label,choice] of [['Keep shared state','server'],['Use my change instead','mine']]){
   const action=element('button','btn btn-secondary btn-sm',label);action.type='button';action.onclick=()=>onResolve(item.id,choice);conflict.append(action);
  }li.append(conflict);
 }
 return li;
}
// Attribution is for other campers' changes; the viewer knows what they did.
export function changedByCaption(item,viewer,task){
 if(!item.changedBy||item.changedById===viewer)return '';
 return (task?(item.checked?'Done by ':'Marked not done by '):(item.checked?'Packed by ':'Unpacked by '))+item.changedBy;
}
// A shared trip with personal entries splits them by person (views.groupByPerson);
// otherwise every entry stays in one unnamed group.
export function groupByPerson(items,shared,viewer){
 if(!shared||!items.some(item=>item.assignee))return [{name:'',items}];
 const groups=new Map();
 for(const item of items){
  const key=item.assignee||'';
  if(!groups.has(key))groups.set(key,{name:!key?'Shared':key===viewer?'Yours':item.assigneeName||'Another camper',items:[]});
  groups.get(key).items.push(item);
 }
 return [...groups.values()];
}
// Categories in order of first use, compared trimmed and case-insensitively;
// uncategorised items last, named "Other" only beside other groups
// (views.groupByCategory).
export function groupByCategory(items){
 const groups=new Map(),other=[];
 for(const item of items){
  const name=(item.category||'').trim();
  if(!name){other.push(item);continue;}
  const key=name.toLowerCase();
  if(!groups.has(key))groups.set(key,{name,items:[]});
  groups.get(key).items.push(item);
 }
 const result=[...groups.values()];
 if(other.length)result.push({name:result.length?'Other':'',items:other});
 return result;
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
