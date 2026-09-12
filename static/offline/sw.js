// Only this public shell and its static assets are cached. Never cache API/auth HTML.
const CACHE = 'camplist-offline-v4';
const ASSETS = ['/offline','/static/style.css','/static/offline/ui.mjs','/static/offline/db.mjs','/static/offline/packing.mjs','/static/offline/transport.mjs'];
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
 if (!ASSETS.includes(url.pathname)) return;
 event.respondWith(caches.open(CACHE).then(async cache=>(await cache.match(url.pathname))||fetch(event.request)));
});
