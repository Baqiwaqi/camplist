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
 remote.getSession=remote.send;
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

test('shared copies belong to the signed-in member and removed access preserves pending work', async () => {
 const db=await database(),remote=server();
 remote.state.userId='owner';remote.state.accountId='camper';remote.state.shared=true;
 const packing=new OfflinePacking(db,remote);
 await packing.save('camper',remote.state);
 await packing.set('camper','trip','stove',true);
 assert.equal(await packing.open('owner','trip'),null);
 let view=await packing.sync('camper','trip');
 assert.equal(view.pending,0);assert.ok(view.lastSyncedAt);assert.equal(view.fresh,true);
 await packing.set('camper','trip','tent',true);
 remote.send=async()=>{throw Object.assign(new Error('removed'),{status:403,code:'access_removed'});};
 view=await packing.sync('camper','trip');
 assert.equal(view.issue,'access_removed');assert.equal(view.pending,1);assert.equal(view.fresh,false);
 assert.match(await packing.export('camper','trip'),/"tent"/);
});

test('shared status does not claim freshness on reconnect until a server refresh succeeds', async()=>{
 const {packingStatus}=await import('../static/offline/status.mjs');
 const view={session:{shared:true},conflicts:{},pending:1,issue:'network',fresh:false,lastSyncedAt:null};
 assert.match(packingStatus(view,true).text,/Can't sync this shared trip/);
 assert.match(packingStatus(view,true).text,/Not synced yet/);
 assert.match(packingStatus(view,false).text,/You're offline/);
 assert.equal(packingStatus({...view,pending:0,issue:null},true).warning,true);
 assert.equal(packingStatus({...view,pending:0,issue:null,fresh:true,lastSyncedAt:'2026-09-12T12:00:00Z'},true).warning,false);
});

test('a tick on a synced shared trip keeps the status calm until a sync actually fails', async()=>{
 const {packingStatus}=await import('../static/offline/status.mjs');
 const db=await database(),remote=server();
 remote.state.shared=true;
 const packing=new OfflinePacking(db,remote);
 await packing.save('camper',remote.state);
 assert.equal(packingStatus(await packing.open('camper','trip'),true).warning,true);
 const synced=packingStatus(await packing.sync('camper','trip'),true);
 assert.equal(synced.warning,false);
 const ticked=packingStatus(await packing.set('camper','trip','tent',true),true);
 assert.equal(ticked.warning,false);
 assert.doesNotMatch(ticked.text,/Checking|may be missing/);
 assert.match(ticked.text,/Waiting to sync: 1 change/);
 remote.send=async()=>{throw new TypeError('Failed to fetch');};
 const failed=packingStatus(await packing.sync('camper','trip'),true);
 assert.equal(failed.warning,true);
 assert.match(failed.text,/Can't sync this shared trip/);
 assert.equal(packingStatus(await packing.set('camper','trip','stove',true),true).warning,true);
});

test('offline additions survive reopening and a check while their acknowledgement is lost', async()=>{
 const db=await database(), state=trip(),receipts=new Set();let lose=true;
 const remote={identity:async()=>({userId:'camper'}),getSession:async()=>structuredClone(state),send:async(_a,_b,op)=>{
  if(!receipts.has(op.id)) {if(op.action==='add')state.list.items.push({id:op.itemId,name:op.name,kind:op.kind,checked:false,revision:0});else{const item=state.list.items.find(i=>i.id===op.itemId);item.checked=op.checked;item.revision++;}receipts.add(op.id);}
  if(lose)throw new Error('lost response');return {operationId:op.id,session:structuredClone(state)};
 }};
 let packing=new OfflinePacking(db,remote);await packing.save('camper',state);
 await packing.add('camper','trip',{id:'charge',name:'Charge car',kind:'task',scope:'shared'});
 await packing.sync('camper','trip');await packing.set('camper','trip','charge',true);
 packing=new OfflinePacking(db,remote);assert.equal((await packing.open('camper','trip')).session.list.items.find(i=>i.id==='charge').checked,true);
 lose=false;const view=await packing.sync('camper','trip');assert.equal(view.pending,0);assert.equal(state.list.items.filter(i=>i.id==='charge').length,1);assert.equal(state.list.items.find(i=>i.id==='charge').checked,true);
});

test('future-list permission failure does not block trip edits and can be cancelled explicitly', async()=>{
 const db=await database(),state=trip();let allowed=false;
 const remote={identity:async()=>({userId:'camper'}),getSession:async()=>structuredClone(state),send:async(_a,_b,op)=>{
  if(op.action==='add'){if(!state.list.items.some(i=>i.id===op.itemId))state.list.items.push({id:op.itemId,name:op.name,revision:0,checked:false});return{operationId:op.id,session:structuredClone(state),futureSaveError:!allowed};}
  const item=state.list.items.find(i=>i.id===op.itemId);item.checked=op.checked;item.revision++;return{operationId:op.id,session:structuredClone(state)};
 }};
 const packing=new OfflinePacking(db,remote);await packing.save('camper',state);await packing.add('camper','trip',{id:'bag',name:'Bag',saveForFuture:true});
 let view=await packing.sync('camper','trip');assert.equal(view.issue,null);assert.equal(view.pending,0);assert.deepEqual(view.futureSaves,['bag']);
 await packing.set('camper','trip','tent',true);view=await packing.sync('camper','trip');assert.equal(view.pending,0);assert.equal(state.list.items[0].checked,true);
 await assert.rejects(packing.forget('camper'),/pending/);
 view=await packing.cancelFutureSave('camper','trip','bag');assert.deepEqual(view.futureSaves,[]);assert.equal(view.session.list.items.some(i=>i.id==='bag'),true);
});

const failing = failure => ({ identity: async () => ({ userId: 'camper' }), getSession: async () => { throw failure; }, send: async () => { throw failure; } });
const deleted = () => Object.assign(new Error('Deleted'), { status: 404 });
const removed = () => Object.assign(new Error('removed'), { status: 403, code: 'access_removed' });

for (const [name, failure] of [['deleted', deleted], ['access removed', removed]]) {
 test(`a saved trip with no unsynced work is removed from this device once the server reports it ${name}`, async () => {
  const db = await database(), packing = new OfflinePacking(db, failing(failure()));
  await packing.save('camper', trip());
  assert.equal(await packing.sync('camper', 'trip'), null);
  assert.deepEqual(await db.list('camper'), []);
 });
}

test('a deleted trip with unsynced work stays marked deleted through later offline failures', async () => {
 const db = await database(), server = failing(deleted());
 const packing = new OfflinePacking(db, server);
 await packing.save('camper', trip()); await packing.set('camper', 'trip', 'tent', true);
 assert.equal((await packing.sync('camper', 'trip')).issue, 'deleted');
 server.identity = async () => { throw new TypeError('Failed to fetch'); };
 let view = await packing.sync('camper', 'trip');
 assert.equal(view.issue, 'deleted'); assert.equal(view.pending, 1);
 server.identity = async () => ({ userId: 'camper' }); server.send = async () => { throw new TypeError('Failed to fetch'); };
 view = await packing.sync('camper', 'trip');
 assert.equal(view.issue, 'deleted');
});

for (const [name, failure] of [
 ['a 404', () => Object.assign(new Error('Item no longer available'), { status: 404, code: 'unavailable' })],
 ['a conflict without the item', state => Object.assign(new Error('Conflict'), { status: 409, code: 'conflict', session: structuredClone(state) })]
]) {
 test(`a pending check whose item was removed online (${name}) is dropped and the rest still uploads`, async () => {
  const { packingStatus } = await import('../static/offline/status.mjs');
  const db = await database(), remote = server(), send = remote.send;
  const packing = new OfflinePacking(db, remote);
  await packing.save('camper', trip());
  await packing.set('camper', 'trip', 'tent', true); await packing.set('camper', 'trip', 'stove', true);
  remote.state.list.items = remote.state.list.items.filter(item => item.id !== 'tent');
  remote.send = async (owner, id, op) => { if (op.itemId === 'tent') throw failure(remote.state); return send(owner, id, op); };
  let view = await packing.sync('camper', 'trip');
  assert.equal(view.issue, null); assert.equal(view.pending, 0); assert.equal(view.fresh, true);
  assert.equal(remote.state.list.items[0].checked, true);
  assert.deepEqual(view.session.list.items.map(item => item.id), ['stove']);
  assert.equal((await db.get('camper', 'trip')).gone, undefined);
  assert.match(packingStatus(view, true).text, /removed online/);
  view = await packing.sync('camper', 'trip');
  assert.equal(view.notice, null);
 });
}

test('a gone marker clears only after a successful refresh', async () => {
 const db = await database(), remote = server(), send = remote.send;
 const packing = new OfflinePacking(db, remote);
 await packing.save('camper', trip()); await packing.set('camper', 'trip', 'tent', true);
 remote.send = async () => { throw removed(); };
 assert.equal((await packing.sync('camper', 'trip')).issue, 'access_removed');
 remote.send = send;
 const view = await packing.sync('camper', 'trip');
 assert.equal(view.issue, null); assert.equal(view.pending, 0);
 remote.getSession = async () => { throw new TypeError('Failed to fetch'); };
 assert.equal((await packing.sync('camper', 'trip')).issue, 'network');
});

test('a trip deleted from this device drops a clean copy and marks one with unsynced work', async () => {
 const db = await database(), packing = new OfflinePacking(db, {});
 await packing.save('camper', trip());
 assert.equal(await packing.gone('camper', 'trip', 'deleted'), null);
 assert.equal(await packing.open('camper', 'trip'), null);
 await packing.save('camper', trip()); await packing.add('camper', 'trip', { id: 'rope', name: 'Rope' });
 const view = await packing.gone('camper', 'trip', 'deleted');
 assert.equal(view.issue, 'deleted'); assert.equal(view.pending, 1);
 assert.equal(await packing.gone('camper', 'missing', 'deleted'), null);
});

test('the trips overview only offers listed copies and labels deleted ones that hold unsynced work', async () => {
 const db = await database(), sessions = new Map(), failures = new Map();
 const remote = {
  identity: async () => ({ userId: 'camper' }),
  getSession: async (_owner, id) => { if (failures.has(id)) throw failures.get(id)(); return structuredClone(sessions.get(id)); },
  send: async (_owner, id) => { throw failures.get(id)(); }
 };
 const packing = new OfflinePacking(db, remote);
 const copy = (id, name) => { const session = { ...trip(), id, name }; sessions.set(id, session); return packing.save('camper', session); };
 await copy('listed', 'Listed'); await copy('archived', 'Archived');
 await copy('deleted', 'Deleted'); failures.set('deleted', deleted);
 await copy('removed', 'Removed'); failures.set('removed', removed);
 await copy('kept', 'Kept'); await packing.set('camper', 'kept', 'tent', true); failures.set('kept', deleted);
 await copy('left', 'Left'); await packing.add('camper', 'left', { id: 'rope', name: 'Rope' }); await packing.set('camper', 'left', 'tent', true); failures.set('left', removed);
 const overview = await packing.reconcile('camper', ['listed']);
 assert.deepEqual(overview.available, ['listed']);
 assert.deepEqual(overview.gone.sort((a, b) => a.id.localeCompare(b.id)), [
  { id: 'kept', name: 'Kept', issue: 'deleted', unsynced: 1 },
  { id: 'left', name: 'Left', issue: 'access_removed', unsynced: 2 }
 ]);
 assert.deepEqual((await db.list('camper')).map(record => record.id).sort(), ['archived', 'kept', 'left', 'listed']);
});

test('shared status points to the export and only mentions conflicts when there are some', async () => {
 const { packingStatus } = await import('../static/offline/status.mjs');
 const view = { session: { shared: true }, conflicts: {}, pending: 1, issue: 'deleted', fresh: false, lastSyncedAt: null };
 assert.doesNotMatch(packingStatus(view, true).text, /Conflicting|from Trips/);
 assert.match(packingStatus(view, true).text, /Saved on this device/);
 assert.doesNotMatch(packingStatus({ ...view, issue: 'network' }, true).text, /Conflicting/);
 assert.match(packingStatus({ ...view, issue: 'network', conflicts: { tent: {} } }, true).text, /Conflicting changes need your review/);
 assert.doesNotMatch(packingStatus({ ...view, issue: null, pending: 0, fresh: true }, false).text, /Conflicting/);
 assert.doesNotMatch(packingStatus({ ...view, session: { shared: false } }, true).text, /from Trips/);
});
