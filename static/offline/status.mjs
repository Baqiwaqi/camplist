// A reconnect event is not evidence of a successful refresh.
export function packingStatus(view, connected) {
 const conflicts=Object.keys(view.conflicts).length;
 const exportHint='Export your changes from Saved on this device, under Recovery options.';
 const issues={signin:'Sign in again to sync. Your changes are saved on this device.',account:'Sign in to the account that saved this trip. Pending changes are preserved.',access_removed:'Your access was removed. Changes will not upload. '+exportHint,deleted:'This trip was deleted online. '+exportHint};
 if(view.session.shared){
  const last=view.lastSyncedAt?'Last synced '+new Date(view.lastSyncedAt).toLocaleString()+'.':'Not synced yet.';
  const blocked=tripGone(view);
  const pending=view.pending?(blocked?`Saved locally: ${view.pending} change(s); upload blocked.`:`Waiting to sync: ${view.pending} change(s).`):'No pending changes.';
  let reason=issues[view.issue]||'';
  if(!reason){
   if(!connected)reason="You're offline. Other people's packing changes may be missing. Changes saved on this device will sync when you reconnect.";
   else if(view.issue==='network')reason="Can't sync this shared trip. Other people's packing changes may be missing. Changes saved on this device will retry automatically.";
   else if(conflicts)reason='Needs review. The shared state has been kept; choose how to resolve your waiting changes below.';
   else if(view.pending||!view.fresh)reason="Checking this shared trip. Other people's packing changes may be missing until sync succeeds.";
   else reason='Shared trip · All changes saved.';
  }
  const warning=Boolean(view.issue||conflicts||view.pending||!view.fresh||!connected);
  const review=conflicts&&!reason.startsWith('Needs review')?'Conflicting changes need your review.':'';
  return {text:[reason,pending,last,review].filter(Boolean).join(' '),warning};
 }
 return {text:issues[view.issue]||(conflicts?'Needs review: another device changed an item.':!connected||view.issue==='network'?'Offline · Changes saved on this device. We’ll sync automatically when connected.':view.pending?'Saved on this device · Syncing…':'All changes saved'),warning:Boolean(view.issue||conflicts)};
}

// The trip was deleted or this account lost access: a kept copy only exports.
export function tripGone(view) {
 return view?.issue==='deleted'||view?.issue==='access_removed';
}
