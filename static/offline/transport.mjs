async function request(path, options = {}) {
 const response = await fetch(path, {...options, credentials:'same-origin', cache:'no-store', signal:AbortSignal.timeout(15000)});
 let body;
 try { body = await response.json(); } catch { body = {}; }
 if (!response.ok) throw Object.assign(new Error(body.message || 'Could not synchronize.'), body, {status:response.status});
 return body;
}
export const transport = {
 identity: () => request('/api/identity'),
 getSession: async (owner,id) => (await request(`/api/sessions/${encodeURIComponent(id)}`,{headers:{'X-Camplist-Account':owner}})).session,
 send: (owner,id,operation,identity) => request(`/api/sessions/${encodeURIComponent(id)}/sync`,{
  method:'POST', headers:{'Content-Type':'application/json','X-CSRF-Token':identity.csrfToken,'X-Camplist-Account':owner},body:JSON.stringify(operation)
 })
};
export function downloadJSON(text, filename) {
 const url = URL.createObjectURL(new Blob([text], {type:'application/json'}));
 const link = document.createElement('a'); link.href=url; link.download=filename; link.click();
 setTimeout(()=>URL.revokeObjectURL(url),1000);
}
