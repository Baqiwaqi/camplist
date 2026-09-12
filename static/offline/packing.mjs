export class OfflinePacking {
 constructor(database, transport) { this.db = database; this.transport = transport; this.running = new Map(); this.closing = new Set(); }
 async save(owner, session) {
  this.closing.delete(owner);
  if ((session.accountId||session.userId) !== owner) throw new Error('Account mismatch.');
  await this.db.update(owner, session.id, old => old ? {...old,fresh:false,session:{...old.session,shared:session.shared,accountId:session.accountId}} : {
   owner, id: session.id, fresh:false, session: structuredClone(session), pending: {}, flight: {}, conflicts: {}, issue: null
  });
  await this.db.setOwner(owner);
  return this.open(owner, session.id);
 }
 async open(owner, id) {
  const record = await this.db.get(owner, id);
  if (!record) return null;
  const session = structuredClone(record.session);
  for (const item of session.list.items) if (record.pending[item.id]) item.checked = record.pending[item.id].checked;
  return { session, pending: Object.keys(record.pending).length, conflicts: structuredClone(record.conflicts), issue: record.issue, lastSyncedAt:record.lastSyncedAt||null, fresh:record.fresh===true };
 }
 async set(owner, id, itemId, checked) {
  if (this.closing.has(owner)) throw new Error("Sign-out is in progress.");
  await this.db.update(owner, id, record => {
   if (!record || !record.session.list.items.some(item => item.id === itemId)) throw new Error('Save this session on this device first.');
   record.pending[itemId] = { id: crypto.randomUUID(), checked: Boolean(checked) };
   return record;
  });
  return this.open(owner, id);
 }

 async resolve(owner,id,itemId,choice) {
  await this.db.update(owner,id,record => {
   const remote = record?.conflicts[itemId];
   if (!remote || !['mine','server'].includes(choice)) throw new Error('Choose a conflicting item.');
   record.session.list.items = record.session.list.items.map(item => item.id === itemId ? structuredClone(remote) : item);
   if (choice === 'server') delete record.pending[itemId];
   else record.pending[itemId].id = crypto.randomUUID();
   delete record.conflicts[itemId]; delete record.flight[itemId];
   return record;
  });
  return this.open(owner,id);
 }
 async export(owner,id) {
  const record = await this.db.get(owner,id);
  if (!record) throw new Error('Session not saved on this device.');
  return JSON.stringify({format:1,...record},null,2);
 }
 async forget(owner,exported) {
  this.closing.add(owner);
  try {
   await Promise.all([...this.running.entries()].filter(([key])=>JSON.parse(key)[0]===owner).map(([,run])=>run));
   await this.db.clear(owner,exported);
  } catch(error) { this.closing.delete(owner); throw error; }
 }
 sync(owner, id) {
  if (this.closing.has(owner)) return this.open(owner,id);
  const key = JSON.stringify([owner,id]);
  if (this.running.has(key)) return this.running.get(key);
  const run = this.synchronize(owner,id).finally(() => this.running.delete(key));
  this.running.set(key, run);
  return run;
 }
 async synchronize(owner,id) {
  try {
   const identity = await this.transport.identity();
   if (identity.userId !== owner) throw Object.assign(new Error('Sign in to the account that saved this trip.'),{code:'account'});
   for (;;) {
    let operation;
    const record = await this.db.update(owner,id, record => {
     if (!record) throw new Error('Session not saved on this device.');
     record.issue = null;
     const itemId = Object.keys(record.pending).find(key => !record.conflicts[key]);
     if (itemId) {
      const pending = record.pending[itemId];
      const item = record.session.list.items.find(item => item.id === itemId);
      operation = record.flight[itemId] ||= {id:pending.id,itemId,checked:pending.checked,expectedRevision:item.revision || 0};
     }
     return record;
    });
    if (!operation) {
     const remote = await this.transport.getSession(owner,id,identity);
     await this.db.update(owner,id,record => {record=mergeRemote(record,remote);record.lastSyncedAt=new Date().toISOString();record.fresh=true;return record;});
     return this.open(owner,id);
    }
    let result;
    try { result = await this.transport.send(owner,id,operation,identity); }
    catch (error) {
     if (error.status !== 409 || error.code !== 'conflict' || !error.session?.list) throw error;
     await this.db.update(owner,id,record => {
      if (record.flight[operation.itemId]?.id !== operation.id) return record;
      const remoteItem = error.session.list.items.find(item => item.id === operation.itemId);
      if (!remoteItem) throw Object.assign(new Error('Item no longer available.'),{status:404});
      delete record.flight[operation.itemId];
      if (record.pending[operation.itemId]?.checked === remoteItem.checked) delete record.pending[operation.itemId];
      else record.conflicts[operation.itemId] = structuredClone(remoteItem);
      return mergeRemote(record,error.session,operation.itemId);
     });
     continue;
    }
    if (result.operationId !== operation.id) throw new Error('Invalid synchronization acknowledgement.');
    await this.db.update(owner,id,record => {
     if (record.flight[operation.itemId]?.id !== operation.id) return record;
     delete record.flight[operation.itemId];
     if (record.pending[operation.itemId]?.id === operation.id) delete record.pending[operation.itemId];
     else if (record.pending[operation.itemId]) {
      const remoteItem = result.session.list.items.find(item => item.id === operation.itemId);
      if (remoteItem.revision !== operation.expectedRevision + 1) {
       if (record.pending[operation.itemId].checked === remoteItem.checked) delete record.pending[operation.itemId];
       else record.conflicts[operation.itemId] = structuredClone(remoteItem);
      }
     }
     return mergeRemote(record,result.session,operation.itemId);
    });
   }
  } catch(error) {
   await this.db.update(owner,id,record => {
    if (!record) throw error;
    record.fresh=false;
    record.issue = error.code === 'access_removed' ? 'access_removed' : error.code === 'account' ? 'account' : error.status === 401 || error.status === 403 ? 'signin' : error.status === 404 ? 'deleted' : 'network';
    return record;
   });
   return this.open(owner,id);
  }
 }
}

function mergeRemote(record, remote, advanceItem) {
 if (remote.id !== record.id || (remote.accountId||remote.userId) !== record.owner || remote.userId !== record.session.userId) throw Object.assign(new Error('Account mismatch.'),{code:'account'});
 const previous = record.session;
 record.session = structuredClone(remote);
 for (let i=0;i<record.session.list.items.length;i++) {
  const item = record.session.list.items[i];
  if (record.pending[item.id] && item.id !== advanceItem) {
   const base = previous.list.items.find(old => old.id === item.id);
   if (base) record.session.list.items[i] = base;
  }
 }
 return record;
}
