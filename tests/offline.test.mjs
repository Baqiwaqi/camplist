import { test } from 'node:test';
import assert from 'node:assert/strict';
import { indexedDB } from 'fake-indexeddb';
import { openDatabase } from '../static/offline/db.mjs';
import { OfflinePacking } from '../static/offline/packing.mjs';

const trip = () => ({ id: 'trip', userId: 'camper', list: { name: 'Weekend', items: [
  { id: 'tent', name: 'Tent', checked: false, revision: 0 },
  { id: 'stove', name: 'Stove', checked: false, revision: 0 }
] } });
const database = () => openDatabase(indexedDB, `test-${crypto.randomUUID()}`);

test('saved session and pending check-off survive reopening', async () => {
 const db = await database();
 const packing = new OfflinePacking(db, {});
 await packing.save('camper', trip());
 await packing.set('camper', 'trip', 'tent', true);
 const reopened = new OfflinePacking(db, {});
 const view = await reopened.open('camper', 'trip');
 assert.equal(view.session.list.items[0].checked, true);
 assert.equal(view.pending, 1);
 assert.equal(await reopened.open('another-account', 'trip'), null);
});

function server() {
 const state = trip(); const receipts = new Map();
 return {
  state, receipts,
  identity: async () => ({userId: 'camper'}),
  getSession: async () => structuredClone(state),
  send: async (_owner, _id, op) => {
   if (!receipts.has(op.id)) {
    const item = state.list.items.find(item => item.id === op.itemId);
    if (item.revision !== op.expectedRevision) throw Object.assign(new Error('Conflict'), {status:409, code:'conflict', session:structuredClone(state)});
    item.checked = op.checked; item.revision++; receipts.set(op.id, true);
   }
   return {operationId:op.id, session:structuredClone(state)};
  }
 };
}

test('lost acknowledgement survives restart and does not duplicate a write', async () => {
 const db = await database(), remote = server();
 const packing = new OfflinePacking(db, remote);
 await packing.save('camper', trip()); await packing.set('camper','trip','tent',true);
 const send = remote.send;
 remote.send = async (...args) => { await send(...args); throw new Error('connection lost'); };
 await packing.sync('camper','trip');
 assert.equal((await packing.open('camper','trip')).pending, 1);
 remote.send = send;
 await new OfflinePacking(db,remote).sync('camper','trip');
 assert.equal(remote.state.list.items[0].revision, 1);
 assert.equal((await packing.open('camper','trip')).pending, 0);
});

test('same-item conflicts preserve both choices until explicitly resolved', async () => {
 const db = await database(), remote = server(), packing = new OfflinePacking(db,remote);
 await packing.save('camper',trip()); await packing.set('camper','trip','tent',true);
 remote.state.list.items[0].revision = 1;
 let view = await packing.sync('camper','trip');
 assert.equal(view.session.list.items[0].checked,true);
 assert.equal(view.conflicts.tent.checked,false);
 assert.equal(remote.state.list.items[0].checked,false);
 await packing.resolve('camper','trip','tent','mine');
 view = await packing.sync('camper','trip');
 assert.equal(view.pending,0); assert.equal(remote.state.list.items[0].checked,true);
});

test('an acknowledgement cannot erase a newer local edit', async () => {
 const db = await database(), remote = server(), packing = new OfflinePacking(db,remote);
 await packing.save('camper',trip()); await packing.set('camper','trip','tent',true);
 let release, entered;
 const waiting = new Promise(resolve => { entered=resolve; });
 const send=remote.send;
 remote.send=async (...args)=>{remote.send=send; entered(); await new Promise(resolve=>{release=resolve;}); return send(...args);};
 const syncing=packing.sync('camper','trip');await waiting;
 await packing.set('camper','trip','tent',false);release();await syncing;
 assert.equal(remote.state.list.items[0].checked,false);
 assert.equal(remote.state.list.items[0].revision,2);
 assert.equal((await packing.open('camper','trip')).pending,0);
});

test('expired login, account changes and deleted sessions preserve pending work', async () => {
 const db=await database(), remote=server(), packing=new OfflinePacking(db,remote);
 await packing.save('camper',trip());await packing.set('camper','trip','tent',true);
 remote.identity=async()=>{throw Object.assign(new Error('Expired'),{status:401});};
 assert.equal((await packing.sync('camper','trip')).issue,'signin');
 remote.identity=async()=>({userId:'someone-else'});
 assert.equal((await packing.sync('camper','trip')).issue,'account');
 assert.equal(remote.receipts.size,0);
 remote.identity=async()=>({userId:'camper'});
 remote.send=async()=>{throw Object.assign(new Error('Deleted'),{status:404});};
 const view=await packing.sync('camper','trip');
 assert.equal(view.issue,'deleted');assert.equal(view.pending,1);
 const exported=JSON.parse(await packing.export('camper','trip'));
 assert.equal(exported.pending.tent.checked,true);
 await assert.rejects(packing.forget('camper'),/pending/i);
 await packing.forget('camper',await db.list('camper'));
 assert.equal(await packing.open('camper','trip'),null);
});

test('a storage transaction failure does not report or persist a check-off', async () => {
 const db=await database(), packing=new OfflinePacking(db,{});
 await packing.save('camper',trip());
 const broken={...db,update:(owner,id,change)=>db.update(owner,id,record=>{
  const next=change(record);next.uncloneable=()=>{};return next;
 })};
 await assert.rejects(new OfflinePacking(broken,{}).set('camper','trip','tent',true));
 const view=await packing.open('camper','trip');
 assert.equal(view.pending,0);assert.equal(view.session.list.items[0].checked,false);
});

test('closing the database preserves queued operations and account cleanup is isolated', async () => {
 const name=`restart-${crypto.randomUUID()}`;
 let db=await openDatabase(indexedDB,name);
 let packing=new OfflinePacking(db,{});
 await packing.save('camper',trip());await packing.set('camper','trip','tent',true);
 const other=trip();other.userId='other';await packing.save('other',other);
 db.close();db=await openDatabase(indexedDB,name);packing=new OfflinePacking(db,{});
 assert.equal((await packing.open('camper','trip')).pending,1);
 await packing.forget('other');
 assert.equal(await packing.open('other','trip'),null);
 assert.equal((await packing.open('camper','trip')).pending,1);
 db.close();
});

test('different-item changes merge and choosing the server resolves a conflict without a write', async () => {
 const db=await database(),remote=server(),packing=new OfflinePacking(db,remote);
 await packing.save('camper',trip());await packing.set('camper','trip','tent',true);
 remote.state.list.items[1].checked=true;remote.state.list.items[1].revision=1;
 let view=await packing.sync('camper','trip');
 assert.equal(view.pending,0);assert.ok(view.session.list.items.every(item=>item.checked));
 await packing.set('camper','trip','tent',false);
 remote.state.list.items[0].revision++;
 view=await packing.sync('camper','trip');assert.ok(view.conflicts.tent);
 const count=remote.receipts.size;
 await packing.resolve('camper','trip','tent','server');view=await packing.sync('camper','trip');
 assert.equal(view.pending,0);assert.equal(view.session.list.items[0].checked,true);
 assert.equal(remote.receipts.size,count);
});

test('sign-out cannot discard a change made in another tab after export', async () => {
 const db=await database(),packing=new OfflinePacking(db,{}),second=new OfflinePacking(db,{});
 await packing.save('camper',trip());await packing.set('camper','trip','tent',true);
 const exported=await db.list('camper');
 await second.set('camper','trip','tent',false);
 await assert.rejects(packing.forget('camper',exported),/changed|export/i);
 assert.equal((await second.open('camper','trip')).session.list.items[0].checked,false);
 await packing.forget('camper',await db.list('camper'));
 assert.equal(await packing.open('camper','trip'),null);
});
