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
  return { session, pending: Object.keys(record.pending).length+Object.keys(record.additions||{}).length, conflicts: structuredClone(record.conflicts), issue: record.issue, futureSaves: Object.keys(record.futureSaves||{}), lastSyncedAt:record.lastSyncedAt||null, fresh:record.fresh===true };
 }
 async add(owner,id,entry) {
  if(this.closing.has(owner))throw new Error('Sign-out is in progress.');
  const name=entry.name?.trim(),itemId=entry.id||crypto.randomUUID();
  if(!name||name.length>200||!['','shared','mine','person'].includes(entry.scope||''))throw new Error('Enter a name and choose who this is for.');
  await this.db.update(owner,id,record=>{
   if(!record)throw new Error('Open this trip while connected first.');
   if(record.session.list.items.some(item=>item.id===itemId))throw new Error('Item already exists.');
   const item={id:itemId,name,category:entry.category?.trim()||'',scope:entry.scope||'',kind:entry.kind||'',checked:false,revision:0};
   if(['mine','person'].includes(item.scope)){item.assignee=owner;item.assigneeName='You';}
   record.session.list.items.push(item);
   record.additions||={};record.additions[itemId]={id:crypto.randomUUID(),itemId,action:'add',name,category:item.category,scope:item.scope,kind:item.kind,saveForFuture:Boolean(entry.saveForFuture)};
   return record;
  });return this.open(owner,id);
 }
 async set(owner, id, itemId, checked) {
  if (this.closing.has(owner)) throw new Error("Sign-out is in progress.");
  await this.db.update(owner, id, record => {
   if (!record || !record.session.list.items.some(item => item.id === itemId)) throw new Error('Save this session on this device first.');
   const item=record.session.list.items.find(item=>item.id===itemId);
   if(item.assignee&&item.assignee!==owner)throw new Error('Only this participant can check their personal entry.');
   record.pending[itemId] = { id: crypto.randomUUID(), checked: Boolean(checked) };
   return record;
  });
  return this.open(owner, id);
 }

 async cancelFutureSave(owner,id,itemId) {await this.db.update(owner,id,record=>{delete record.futureSaves?.[itemId];return record;});return this.open(owner,id);}
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
   const retriedFuture=new Set();
   for (;;) {
    let operation;
    const record = await this.db.update(owner,id, record => {
     if (!record) throw new Error('Session not saved on this device.');
     record.issue = record.gone || null;
     const addition=Object.values(record.additions||{})[0];
     if(addition){operation=structuredClone(addition);return record;}
     const itemId = Object.keys(record.pending).find(key => !record.conflicts[key]);
     if (itemId) {
      const pending = record.pending[itemId];
      const item = record.session.list.items.find(item => item.id === itemId);
      operation = record.flight[itemId] ||= {id:pending.id,itemId,checked:pending.checked,expectedRevision:item.revision || 0,...(item.kind?{kind:item.kind}:{})};
     }
     return record;
    });
    if (!operation) {
     const future=Object.values(record.futureSaves||{}).find(op=>!retriedFuture.has(op.itemId));
     if(future){
      retriedFuture.add(future.itemId);
      const result=await this.transport.send(owner,id,future,identity);
      if(result.operationId!==future.id)throw new Error('Invalid synchronization acknowledgement.');
      if(!result.futureSaveError)await this.db.update(owner,id,record=>{delete record.futureSaves?.[future.itemId];return record;});
      continue;
     }
     const remote = await this.transport.getSession(owner,id,identity).catch(error=>{throw Object.assign(error,{trip:true});});
     await this.db.update(owner,id,record => {record=mergeRemote(record,remote);record.lastSyncedAt=new Date().toISOString();record.fresh=true;record.issue=null;delete record.gone;return record;});
     return this.open(owner,id);
    }
    let result;
    try { result = await this.transport.send(owner,id,operation,identity); }
    catch (error) {
     if(operation.action==='add')throw error;
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
     if(operation.action==='add'){
      if(result.futureSaveError){record.futureSaves||={};record.futureSaves[operation.itemId]=operation;retriedFuture.add(operation.itemId);}
      if(record.additions?.[operation.itemId]?.id===operation.id)delete record.additions[operation.itemId];
      return mergeRemote(record,result.session,operation.itemId);
     }
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
   let issue = classify(error);
   if (issue === 'deleted' && !error.trip) {
    try { await this.transport.getSession(owner,id); issue = 'network'; }
    catch (check) { issue = classify(check); }
   }
   if (GONE.includes(issue)) return this.gone(owner,id,issue,error);
   await this.db.update(owner,id,record => {
    if (!record) throw error;
    record.fresh=false;
    // Once the server said the trip is gone, only a successful refresh clears it.
    record.issue = record.gone || issue;
    return record;
   });
   return this.open(owner,id);
  }
 }
 // The trip was deleted or this account lost access. A copy without unsynced
 // work is removed; one with unsynced work stays, marked, for export.
 async gone(owner,id,issue,error) {
  await this.db.update(owner,id,record => {
   if (!record) { if (error) throw error; return null; }
   if (!unsynced(record)) return null;
   return {...record,fresh:false,issue,gone:issue};
  });
  return this.open(owner,id);
 }
 // Decide what the trips overview shows for this account's saved copies:
 // listed trips are available offline; every other copy is checked with the
 // server first, so only kept copies of gone trips are reported, never an
 // archived trip or one the check could not reach.
 async reconcile(owner,listedIds) {
  const listed = new Set(listedIds), available = [], gone = [];
  for (const record of await this.db.list(owner)) {
   if (listed.has(record.id)) { available.push(record.id); continue; }
   const view = await this.sync(owner,record.id);
   if (!view || !GONE.includes(view.issue)) continue;
   gone.push({id:record.id,name:view.session.name||view.session.list.name,issue:view.issue,unsynced:view.pending+view.futureSaves.length});
  }
  return {available,gone};
 }
}

const GONE = ['deleted','access_removed'];
function classify(error) {
 return error.code === 'access_removed' ? 'access_removed' : error.code === 'account' ? 'account' : error.status === 401 || error.status === 403 ? 'signin' : error.status === 404 ? 'deleted' : 'network';
}
export function unsynced(record) {
 return Object.keys(record.pending).length+Object.keys(record.additions||{}).length+Object.keys(record.futureSaves||{}).length;
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
 for(const item of previous.list.items){if(record.additions?.[item.id]&&!record.session.list.items.some(remote=>remote.id===item.id))record.session.list.items.push(item);}
 return record;
}
