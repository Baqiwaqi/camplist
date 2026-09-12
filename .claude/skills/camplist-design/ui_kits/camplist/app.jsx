const { AppHeader } = window.CamplistDesignSystem_8dbf6a || {};
function CamplistApp() {
  const seed = window.camplistSeed;
  const [auth, setAuth] = React.useState(false);
  const [route, setRoute] = React.useState('/');
  const [lists, setLists] = React.useState(seed.lists);
  const [sessions, setSessions] = React.useState(seed.sessions);
  const go = (r) => { if (r === '/auth/signout') { setAuth(false); setRoute('/'); return; } setRoute(r); window.scrollTo(0, 0); };
  if (!auth) return <LoginScreen onLogin={() => setAuth(true)} />;
  const uid = () => Math.random().toString(36).slice(2, 8);
  let body;
  const m = (re) => route.match(re);
  let mm;
  if (route === '/') body = <ListsScreen lists={lists} sessions={sessions} go={go} onDelete={id => setLists(ls => ls.filter(l => l.id !== id))} />;
  else if (route === '/packing-list/new') body = <ListFormScreen go={go} onSave={v => { setLists(ls => [...ls, { id: uid(), items: [], ...v }]); go('/'); }} />;
  else if ((mm = m(/^\/packing-list\/(\w+)\/edit$/))) { const l = lists.find(x => x.id === mm[1]); body = <ListFormScreen key={l.id} list={l} go={go} onSave={v => { setLists(ls => ls.map(x => x.id === l.id ? { ...x, ...v } : x)); go('/'); }} />; }
  else if ((mm = m(/^\/packing-list\/(\w+)$/))) { const l = lists.find(x => x.id === mm[1]); body = <ListDetailsScreen list={l} go={go}
    onAddItem={it => setLists(ls => ls.map(x => x.id === l.id ? { ...x, items: [...x.items, { id: uid(), ...it }] } : x))}
    onRemoveItem={iid => setLists(ls => ls.map(x => x.id === l.id ? { ...x, items: x.items.filter(i => i.id !== iid) } : x))}
    onStartSession={() => { const id = uid(); setSessions(ss => [{ id, listId: l.id, createdAt: 'Sep 12, 2026', checked: [], snapshot: JSON.parse(JSON.stringify(l)) }, ...ss]); go('/packing-session/' + id); }} />; }
  else if ((mm = m(/^\/packing-session\/(\w+)$/))) { const s = sessions.find(x => x.id === mm[1]); const l = s.snapshot || lists.find(x => x.id === s.listId); body = <SessionScreen session={s} list={l} go={go}
    onToggle={iid => setSessions(ss => ss.map(x => x.id === s.id ? { ...x, checked: x.checked.includes(iid) ? x.checked.filter(c => c !== iid) : [...x.checked, iid] } : x))} />; }
  else if (route === '/sessions') body = <SessionsScreen sessions={sessions} lists={lists} go={go} onDelete={id => setSessions(ss => ss.filter(s => s.id !== id))} />;
  else body = <p>Not found</p>;
  return <div>
    <AppHeader userName={seed.user} logoSrc="../../assets/logomark.svg" onNavigate={go} />
    <main style={{ maxWidth: 'var(--content-max)', margin: '0 auto', padding: 'var(--main-padding)' }}>{body}</main>
  </div>;
}
const camplistRoot = document.getElementById('root');
if (camplistRoot && window.LoginScreen) ReactDOM.createRoot(camplistRoot).render(<CamplistApp />);