// Only this public shell and its static assets are cached. Never cache API/auth HTML.
const CACHE = 'camplist-offline-v9';
const ASSETS = ['/offline','/static/style.css','/static/logomark.svg','/static/offline/ui.mjs','/static/offline/checklist.mjs','/static/offline/status.mjs','/static/offline/db.mjs','/static/offline/packing.mjs','/static/offline/transport.mjs'];
self.addEventListener('install', event => {
 event.waitUntil(caches.open(CACHE).then(cache=>cache.addAll(ASSETS)).then(()=>self.skipWaiting()));
});
self.addEventListener('activate', event => {
 event.waitUntil(caches.keys().then(keys=>Promise.all(keys.filter(key=>key.startsWith('camplist-offline-')&&key!==CACHE).map(key=>caches.delete(key)))).then(()=>self.clients.claim()));
});
self.addEventListener('message', event => {
 if (event.data !== 'offline-ready' || !event.ports[0]) return;
 event.waitUntil(caches.open(CACHE).then(async cache=>{
  const present=await Promise.all(ASSETS.map(path=>cache.match(path)));
  event.ports[0].postMessage({ready:present.every(Boolean)});
 }));
});
self.addEventListener('fetch', event => {
 const url=new URL(event.request.url);
 if (event.request.method!=='GET'||url.origin!==self.location.origin) return;
 // A failed session navigation opens the public shell; account data stays in IndexedDB.
 if(event.request.mode==='navigate' && /^\/(?:packing-session\/[^/]+|trips(?:\/[^/]+)?)$/.test(url.pathname)) {
  event.respondWith(fetch(event.request).then(async response=>response.status>=500 ? (await caches.match('/offline'))||response : response).catch(async()=> (await caches.match('/offline'))||Response.error()));
  return;
 }
 if (!ASSETS.includes(url.pathname)) return;
 event.respondWith(caches.open(CACHE).then(async cache=>(await cache.match(url.pathname))||fetch(event.request)));
});
