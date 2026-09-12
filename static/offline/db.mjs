// All edits to a record (snapshot + pending operations) commit in one transaction.
export function openDatabase(factory = indexedDB, name = 'camplist-offline') {
 return new Promise((resolve, reject) => {
  const request = factory.open(name, 1);
  request.onupgradeneeded = () => {
   const db = request.result;
   if (!db.objectStoreNames.contains('sessions')) db.createObjectStore('sessions', { keyPath: ['owner', 'id'] });
   if (!db.objectStoreNames.contains('meta')) db.createObjectStore('meta');
  };
  request.onerror = () => reject(request.error);
  request.onblocked = () => reject(new Error('Close other Camplist tabs to update offline storage.'));
  request.onsuccess = () => {
   const db = request.result;
   db.onversionchange = () => db.close();
   function read(store, key) {
    return new Promise((done, fail) => {
     const tx = db.transaction(store, 'readonly');
     const req = key === undefined ? tx.objectStore(store).getAll() : tx.objectStore(store).get(key);
     req.onsuccess = () => done(req.result);
     req.onerror = () => fail(req.error);
    });
   }
   resolve({
    get: (owner, id) => read('sessions', [owner, id]),
    list: async (owner) => (await read('sessions')).filter(record => record.owner === owner),
    owner: () => read('meta', 'activeAccount'),
    setOwner: (owner) => new Promise((done, fail) => {
     const tx = db.transaction('meta', 'readwrite');
     if (owner) tx.objectStore('meta').put(owner, 'activeAccount');
     else tx.objectStore('meta').delete('activeAccount');
     tx.oncomplete = () => done(); tx.onabort = () => fail(tx.error);
    }),
    update: (owner, id, change) => new Promise((done, fail) => {
     const tx = db.transaction('sessions', 'readwrite');
     const store = tx.objectStore('sessions');
     let result, failure;
     const req = store.get([owner, id]);
     req.onsuccess = () => {
      try {
       result = change(req.result);
       if (result === null) store.delete([owner, id]);
       else store.put(result);
      } catch (error) { failure = error; tx.abort(); }
     };
     tx.oncomplete = () => done(structuredClone(result));
     tx.onabort = () => fail(failure || tx.error || new Error('Offline save failed.'));
    }),
    clear: (owner, exported) => new Promise((done, fail) => {
     const tx = db.transaction(['sessions', 'meta'], 'readwrite');
     const store = tx.objectStore('sessions');
     let failure;
     const req = store.getAll();
     req.onsuccess = () => {
      const records = req.result.filter(record => record.owner === owner);
      const serialize = records => JSON.stringify([...records].sort((a,b) => a.id.localeCompare(b.id)));
      if (exported ? serialize(records) !== serialize(exported) : records.some(record => Object.keys(record.pending).length)) {
       failure = new Error('Packing changed or has pending work. Export or synchronize again before signing out.');
       tx.abort(); return;
      }
      for (const record of records) store.delete([owner, record.id]);
      const meta = tx.objectStore('meta'); const active = meta.get('activeAccount');
      active.onsuccess = () => { if (active.result === owner) meta.delete('activeAccount'); };
     };
     tx.oncomplete = () => done(); tx.onabort = () => fail(failure || tx.error);
    }),
    close: () => db.close()
   });
  };
 });
}
