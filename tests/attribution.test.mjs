import { test } from 'node:test';
import assert from 'node:assert/strict';

// A small stand-in for the DOM calls renderEntries makes.
class Element {
 constructor(tag){this.tag=tag;this.children=[];this.attributes={};this.className='';this.textContent='';}
 append(...nodes){this.children.push(...nodes);}
 replaceChildren(...nodes){this.children=nodes;}
 setAttribute(name,value){this.attributes[name]=value;}
 *walk(){yield this;for(const child of this.children)yield* child.walk();}
}
globalThis.document={activeElement:null,createElement:tag=>new Element(tag),getElementById:()=>null};
const {renderEntries}=await import('../static/offline/checklist.mjs');

const attributions=(accountId)=>{
 const view={conflicts:{},session:{userId:'owner',accountId,list:{items:[
  {id:'tent',name:'Tent',checked:true,changedBy:'Trip owner',changedById:'owner'},
  {id:'stove',name:'Stove',checked:true,changedBy:'Alex',changedById:'guest'}
 ]}}};
 const container=new Element('div');
 renderEntries(container,view,'',()=>{},()=>{});
 return [...container.walk()].filter(node=>node.className==='muted').map(node=>node.textContent);
};

test('attribution names only changes made by someone else', () => {
 assert.deepEqual(attributions('owner'), ['Updated by Alex']);
 assert.deepEqual(attributions('guest'), ['Updated by Trip owner']);
});
