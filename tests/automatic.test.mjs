import { test } from 'node:test';
import assert from 'node:assert/strict';
import { indexedDB } from 'fake-indexeddb';
import { openDatabase } from '../static/offline/db.mjs';
import { OfflinePacking } from '../static/offline/packing.mjs';
import { AutomaticPacking } from '../static/offline/automatic.mjs';

test('opening a trip saves it automatically; offline edits sync on reconnect without a save button', async t => {
 const db = await openDatabase(indexedDB, crypto.randomUUID());
 const session = {id:'trip',userId:'camper',list:{name:'Weekend',items:[{id:'tent',name:'Tent',checked:false,revision:0}]}};
 let online = true;
 const remote = {
  identity: async () => {if(!online)throw new Error('offline');return {userId:'camper'};},
  getSession: async () => structuredClone(session),
  send: async (_owner,_id,operation) => {
   session.list.items[0].checked=operation.checked;session.list.items[0].revision++;
   return {operationId:operation.id,session:structuredClone(session)};
  }
 };
 const events=new EventTarget();
 const automatic=new AutomaticPacking(new OfflinePacking(db,remote),()=>{},events);
 t.after(()=>{automatic.stop();db.close();});
 await automatic.save(session);
 online=false;events.dispatchEvent(new Event('offline'));
 await automatic.set('tent',true);
 await automatic.sync();
 const reopened=new OfflinePacking(db,remote);
 assert.equal((await reopened.open('camper','trip')).session.list.items[0].checked,true);
 assert.equal(session.list.items[0].checked,false);
 online=true;events.dispatchEvent(new Event('online'));
 // Joining the in-flight automatic sync observes the reconnect result.
 await automatic.sync();
 assert.equal(session.list.items[0].checked,true);
 assert.equal((await reopened.open('camper','trip')).pending,0);
});
