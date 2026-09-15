import { test } from 'node:test';
import assert from 'node:assert/strict';

// A small stand-in for the DOM calls renderEntries makes.
class Element {
 constructor(tag){this.tag=tag;this.children=[];this.attributes={};this.dataset={};this.className='';this.textContent='';}
 append(...nodes){this.children.push(...nodes);}
 replaceChildren(...nodes){this.children=nodes;}
 setAttribute(name,value){this.attributes[name]=value;}
 *walk(){yield this;for(const child of this.children)yield* child.walk();}
 text(){return this.textContent+this.children.map(child=>child.text()).join('');}
}
globalThis.document={activeElement:null,createElement:tag=>new Element(tag),getElementById:()=>null};
const {renderEntries}=await import('../static/offline/checklist.mjs');

function render(session,kind=''){
 const container=new Element('div');
 renderEntries(container,{conflicts:{},session},kind,()=>{},()=>{});
 const nodes=[...container.walk()];
 return {container,nodes,byClass:name=>nodes.filter(node=>node.className.split(' ').includes(name))};
}

test('attribution names only changes made by someone else', () => {
 const captions=accountId=>render({userId:'owner',accountId,list:{items:[
  {id:'tent',name:'Tent',checked:true,changedBy:'Trip owner',changedById:'owner'},
  {id:'stove',name:'Stove',checked:false,changedBy:'Alex',changedById:'guest'}
 ]}}).byClass('pack-by').map(node=>node.textContent);
 assert.deepEqual(captions('owner'), ['Unpacked by Alex']);
 assert.deepEqual(captions('guest'), ['Packed by Trip owner']);
});

test('a private trip is one sheet grouped by category, with tick rows and a Yours tag', () => {
 const {byClass,nodes}=render({userId:'me',list:{items:[
  {id:'tent',name:'Tent',category:'Shelter',checked:true},
  {id:'stove',name:'Stove',category:'Kitchen',checked:false},
  {id:'tarp',name:'Tarp',category:' shelter ',checked:false},
  {id:'bag',name:'Sleeping bag',category:'',checked:false,assignee:'me',assigneeName:'Me'},
  {id:'fuel',kind:'task',name:'Buy fuel',checked:false}
 ]}});
 assert.equal(byClass('pack-sheet').length,1);
 assert.equal(byClass('person-head').length,0);
 assert.deepEqual(byClass('group-head').map(head=>[head.tag,head.text()]),[['h2','Shelter1 of 2'],['h2','Kitchen0 of 1'],['h2','Other0 of 1']]);
 assert.deepEqual(byClass('group-count').map(count=>count.id),['pack-count-0-0','pack-count-0-1','pack-count-0-2']);
 assert.deepEqual(byClass('pack-row').map(row=>row.id),['pack-tent','pack-tarp','pack-stove','pack-bag']);
 assert.equal(byClass('pack-action').length,0,'no Pack/Unpack pill');
 assert.deepEqual(byClass('tag').map(tag=>tag.textContent),['Yours']);
 assert.ok(nodes.find(node=>node.id==='pack-bag').children.some(child=>child.textContent==='Yours'),'the tag is inside the row button');
});

test('a shared trip with personal items nests categories under each person', () => {
 const {byClass,nodes}=render({userId:'owner',accountId:'me',shared:true,list:{items:[
  {id:'tent',name:'Tent',category:'Shelter',checked:true,changedBy:'Sam',changedById:'sam'},
  {id:'mine',name:'Sleeping bag',category:'Sleep',checked:true,assignee:'me',assigneeName:'Me'},
  {id:'sams',name:'Sleeping bag',category:'Sleep',checked:true,assignee:'sam',assigneeName:'Sam Rivera'}
 ]}});
 assert.deepEqual(byClass('person-head').map(head=>[head.tag,head.children[0].textContent,head.children[1].textContent]),[['h2','Shared','All packed'],['h2','Yours','All packed'],['h2','Sam Rivera','All packed']]);
 assert.ok(byClass('group-head').every(head=>head.tag==='h3'));
 assert.equal(byClass('pack-sheet').length,3);
 assert.equal(byClass('tag').length,0);
 assert.equal(nodes.find(node=>node.id==='pack-sams').disabled,true);
 assert.deepEqual(byClass('pack-by').map(node=>node.textContent),['Packed by Sam']);
});

test('tasks render as tick rows without category groups', () => {
 const {byClass}=render({userId:'me',list:{items:[
  {id:'tent',name:'Tent',category:'Shelter',checked:false},
  {id:'fuel',kind:'task',name:'Buy fuel',checked:true,changedBy:'Sam',changedById:'sam'}
 ]}},'task');
 assert.equal(byClass('group-head').length,0);
 assert.deepEqual(byClass('pack-row').map(row=>row.id),['pack-fuel']);
 assert.deepEqual(byClass('pack-by').map(node=>node.textContent),['Done by Sam']);
});

test('an empty checklist says what to do', () => {
 assert.deepEqual(render({userId:'me',list:{items:[]}}).byClass('card').map(node=>node.textContent),['Nothing to pack yet. Add something to this trip below.']);
});
