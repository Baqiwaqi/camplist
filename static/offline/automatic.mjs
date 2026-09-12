// Public save/edit/sync interface shared by the packing screen and its offline copy.
export class AutomaticPacking {
 constructor(packing, changed, events = window) {
  this.packing=packing;this.changed=changed;this.events=events;
  this.reconnect=()=>{this.sync().catch(error=>this.changed(null,error));};
  this.disconnected=()=>{this.notify().catch(error=>this.changed(null,error));};
 }
 async save(session) {
  this.owner=session.accountId||session.userId;this.id=session.id;
  await this.packing.save(this.owner,session);
  await this.notify();
  this.events.addEventListener('online',this.reconnect);
  this.events.addEventListener('offline',this.disconnected);
  this.events.addEventListener('focus',this.reconnect);
  this.timer=setInterval(async()=>{
   if(typeof document!=="undefined"&&document.visibilityState!=="visible")return;
   try {
    const view=await this.packing.open(this.owner,this.id);
    if(view?.issue==='network'||(!view?.issue&&(view?.pending||view?.session.shared)))await this.sync();
   }catch(error){this.changed(null,error);}
  },15000);
  this.reconnect();
 }
 async set(itemId,checked) {
  await this.packing.set(this.owner,this.id,itemId,checked);
  await this.notify();
  this.reconnect();
 }
 async sync() {
  await this.packing.sync(this.owner,this.id);
  return this.notify();
 }
 async notify() {
  const view=await this.packing.open(this.owner,this.id);
  this.changed(view);return view;
 }
 stop() {
  clearInterval(this.timer);
  this.events.removeEventListener('online',this.reconnect);
  this.events.removeEventListener('offline',this.disconnected);
  this.events.removeEventListener('focus',this.reconnect);
 }
}
