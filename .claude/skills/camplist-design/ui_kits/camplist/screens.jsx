const camplistNs = window.CamplistDesignSystem_8dbf6a || {};
const { Card, ItemRow, ErrorBanner, AppHeader, Button, Field } = camplistNs;
const ProgressBar = camplistNs.ProgressBar || (() => null);
const Muted = ({ children, style }) => <p style={{ color: 'var(--text-muted)', margin: '0 0 1rem', ...style }}>{children}</p>;
const H1 = ({ children, style }) => <h1 style={{ fontFamily: 'var(--font-display)', fontWeight: 'var(--weight-display)', fontSize: 'var(--text-3xl)', lineHeight: 'var(--leading-tight)', letterSpacing: 'var(--tracking-display)', margin: '0 0 .5rem', ...style }}>{children}</h1>;
const H2 = ({ children, style }) => <h2 style={{ fontFamily: 'var(--font-display)', fontWeight: 'var(--weight-display)', fontSize: 'var(--text-xl)', lineHeight: 'var(--leading-tight)', letterSpacing: 'var(--tracking-display)', margin: '0 0 .25rem', ...style }}>{children}</h2>;
const Eyebrow = ({ children }) => <div style={{ fontSize: 'var(--text-xs)', fontWeight: 'var(--weight-bold)', letterSpacing: 'var(--tracking-eyebrow)', textTransform: 'uppercase', color: 'var(--text-eyebrow)', marginBottom: 'var(--space-2)' }}>{children}</div>;
const Actions = ({ children, style }) => <div style={{ display: 'flex', gap: 'var(--space-2)', alignItems: 'center', flexWrap: 'wrap', ...style }}>{children}</div>;
const Cat = ({ children }) => <span style={{ whiteSpace: 'nowrap', flex: 'none', fontSize: 'var(--text-xs)', fontWeight: 700, letterSpacing: '0.04em', textTransform: 'uppercase', color: 'var(--brand)', background: 'var(--brand-soft)', padding: '2px 8px', borderRadius: 'var(--radius-pill)' }}>{children}</span>;
const Tick = ({ on }) => <span style={{ width: 22, height: 22, borderRadius: '50%', flex: 'none', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', border: '2px solid ' + (on ? 'var(--brand)' : 'var(--border-header)'), background: on ? 'var(--brand)' : 'transparent', color: '#fff', fontSize: 13, fontWeight: 700, transition: 'all var(--motion-fast)' }}>{on ? '✓' : ''}</span>;

function LoginScreen({ onLogin }) {
  return <div style={{ minHeight: '100vh', display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)' }}>
    <div style={{ background: 'var(--brand)', color: '#fff', padding: 'var(--space-8)', display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontWeight: 700, fontSize: 'var(--text-lg)' }}><img src="../../assets/logomark-white.svg" alt="" width="28" height="28" />Camplist</div>
      <div><div style={{ fontFamily: 'var(--font-display)', fontWeight: 800, fontSize: 'var(--text-4xl)', lineHeight: 1, letterSpacing: 'var(--tracking-display)', marginBottom: 'var(--space-4)' }}>Pack once.<br />Go every weekend.</div><p style={{ margin: 0, fontSize: 'var(--text-lg)', opacity: .85, maxWidth: 380 }}>Keep your camping kit as a list, start a session for each trip, and get out the door faster than last time.</p></div>
      <div style={{ fontSize: 'var(--text-sm)', opacity: .7 }}>Weekend camping · Day hike · Canoe trip</div>
    </div>
    <div style={{ display: 'grid', placeItems: 'center', padding: 'var(--space-8)' }}>
      <div style={{ width: 340 }}>
        <Eyebrow>Sign in</Eyebrow>
        <H1 style={{ marginBottom: 'var(--space-4)' }}>Let's get packing.</H1>
        <Button variant="secondary" size="lg" onClick={onLogin} style={{ width: '100%' }}>Login with google</Button>
        <Muted style={{ marginTop: 'var(--space-4)', fontSize: 'var(--text-sm)' }}>Your lists and sessions stay with your Google account.</Muted>
      </div>
    </div>
  </div>;
}

function ListsScreen({ lists, sessions, go, onDelete }) {
  return <section>
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 16, marginBottom: 'var(--space-6)' }}>
      <div><Eyebrow>Your lists</Eyebrow><H1>Where to next?</H1><Muted style={{ margin: 0 }}>Pick a list and start a session — your list stays untouched.</Muted></div>
      <Button onClick={() => go('/packing-list/new')}>Create</Button>
    </div>
    {lists.length === 0 && <Card><Muted style={{ margin: 0 }}>No lists yet. Create one to get started.</Muted></Card>}
    {lists.map(l => <Card key={l.id}>
      <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
        <div style={{ flex: 1 }}><H2>{l.name}</H2><Muted style={{ marginBottom: 'var(--space-3)' }}>{l.description}</Muted>
          <Actions><Cat>{l.items.length} items</Cat>{[...new Set(l.items.map(i => i.category))].slice(0, 4).map(c => <span key={c} style={{ fontSize: 'var(--text-sm)', color: 'var(--text-muted)' }}>{c}</span>)}</Actions></div>
        <Actions style={{ flex: 'none', flexWrap: 'nowrap' }}>
          <Button variant="secondary" size="sm" onClick={() => go('/packing-list/' + l.id + '/edit')}>Edit</Button>
          <Button variant="secondary" size="sm" onClick={() => go('/packing-list/' + l.id)}>Details</Button>
          <Button variant="danger" size="sm" onClick={() => confirm('Delete this list?') && onDelete(l.id)}>Delete</Button>
        </Actions>
      </div>
    </Card>)}
  </section>;
}

function ListFormScreen({ list, go, onSave }) {
  const [name, setName] = React.useState(list ? list.name : '');
  const [desc, setDesc] = React.useState(list ? list.description : '');
  const [errors, setErrors] = React.useState([]);
  const submit = (e) => { e.preventDefault(); if (!name.trim()) return setErrors(['Name is required']); onSave({ name, description: desc }); };
  return <section style={{ maxWidth: 560 }}>
    <Eyebrow>{list ? 'Edit list' : 'New list'}</Eyebrow>
    <H1>{list ? list.name : 'Start a new list'}</H1>
    {!list && <Muted>Start simple and adjust over time. You'll refine it after the first trip.</Muted>}
    <Card><form onSubmit={submit}>
      <Field label="Name" id="name" placeholder="Camping" value={name} error={errors.length > 0} onChange={e => setName(e.target.value)} />
      <Field label="Description" id="description" multiline placeholder="A short description of your list" value={desc} onChange={e => setDesc(e.target.value)} />
      {errors.length > 0 && <ErrorBanner errors={errors} style={{ marginBottom: 'var(--space-4)' }} />}
      <Actions><Button variant="link" onClick={() => go('/')}>Cancel</Button><Button type="submit" style={{ marginLeft: 'auto' }}>{list ? 'Save' : 'Create New'}</Button></Actions>
    </form></Card>
  </section>;
}

function ListDetailsScreen({ list, go, onAddItem, onRemoveItem, onStartSession }) {
  const [name, setName] = React.useState(''); const [cat, setCat] = React.useState(''); const [errors, setErrors] = React.useState([]);
  const submit = (e) => { e.preventDefault(); if (!name.trim()) return setErrors(['Name is required']); onAddItem({ name, category: cat }); setName(''); setCat(''); setErrors([]); };
  return <section>
    <Card tone="brand" style={{ padding: 'var(--space-6)', marginBottom: 'var(--space-6)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 16, flexWrap: 'wrap' }}>
        <div><div style={{ fontSize: 'var(--text-xs)', fontWeight: 700, letterSpacing: 'var(--tracking-eyebrow)', textTransform: 'uppercase', opacity: .8, marginBottom: 8 }}>Packing list · {list.items.length} items</div><H1 style={{ margin: 0 }}>{list.name}</H1><p style={{ margin: '8px 0 0', opacity: .85 }}>{list.description}</p></div>
        <Actions><Button variant="secondary" onClick={() => go('/packing-list/' + list.id + '/edit')}>Edit</Button><Button variant="accent" size="lg" onClick={onStartSession}>Start session →</Button></Actions>
      </div>
    </Card>
    <Card>
      {list.items.length === 0 && <Muted style={{ margin: 0 }}>No items yet. Add your first one below.</Muted>}
      {list.items.map((it, i) => <ItemRow key={it.id} last={i === list.items.length - 1}>
        <span style={{ fontWeight: 700 }}>{it.name}</span>{it.category && <Cat>{it.category}</Cat>}
        <Button variant="danger" size="sm" style={{ marginLeft: 'auto' }} onClick={() => confirm('Delete this item?') && onRemoveItem(it.id)}>Delete</Button>
      </ItemRow>)}
    </Card>
    <H2 style={{ margin: '0 0 .75rem' }}>Add Item</H2>
    <Card><form onSubmit={submit}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 'var(--space-4)' }}>
        <Field label="Name" id="item-name" placeholder="Headlamp" value={name} error={errors.length > 0} onChange={e => setName(e.target.value)} style={{ marginBottom: 0 }} />
        <Field label="Category" id="item-category" placeholder="Light" value={cat} onChange={e => setCat(e.target.value)} style={{ marginBottom: 0 }} />
      </div>
      {errors.length > 0 && <ErrorBanner errors={errors} style={{ marginTop: 'var(--space-4)' }} />}
      <Actions style={{ marginTop: 'var(--space-4)' }}><Button type="submit">Add Item</Button></Actions>
    </form></Card>
  </section>;
}

function SessionScreen({ session, list, onToggle, go }) {
  const checked = new Set(session.checked); const done = checked.size === list.items.length && list.items.length > 0;
  const cats = [...new Set(list.items.map(i => i.category || 'Other'))];
  return <section>
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', gap: 16, marginBottom: 'var(--space-4)' }}>
      <div><Eyebrow>Packing session · {session.createdAt}</Eyebrow><H1 style={{ margin: 0 }}>{done ? 'All packed. Go!' : list.name}</H1></div>
      <Button variant="secondary" size="sm" onClick={() => go('/sessions')}>All sessions</Button>
    </div>
    <Card tone={done ? 'brand' : undefined}><ProgressBar inverse={done} value={checked.size} max={list.items.length} label={done ? 'Packed. Go!' : "Let's pack " + list.name.toLowerCase()} /></Card>
    {cats.map(c => <Card key={c}>
      <div style={{ fontSize: 'var(--text-xs)', fontWeight: 700, letterSpacing: 'var(--tracking-eyebrow)', textTransform: 'uppercase', color: 'var(--text-muted)', paddingBottom: 'var(--space-1)' }}>{c}</div>
      {list.items.filter(i => (i.category || 'Other') === c).map((it, i, arr) => { const on = checked.has(it.id); return <ItemRow key={it.id} last={i === arr.length - 1} onClick={() => onToggle(it.id)} style={{ cursor: 'pointer' }}>
        <Tick on={on} /><span style={{ fontWeight: 700, textDecoration: on ? 'line-through' : 'none', color: on ? 'var(--text-muted)' : 'var(--text-body)' }}>{it.name}</span>
        <Button size="sm" variant={on ? 'secondary' : 'accent'} style={{ marginLeft: 'auto' }} onClick={(e) => { e.stopPropagation(); onToggle(it.id); }}>{on ? 'Uncheck' : 'Check'}</Button>
      </ItemRow>; })}
    </Card>)}
  </section>;
}

function SessionsScreen({ sessions, lists, go, onDelete }) {
  return <section>
    <Eyebrow>Sessions</Eyebrow><H1>Trips in progress</H1>
    <Muted>Each session is a snapshot — check things off without touching the list.</Muted>
    {sessions.length === 0 && <Card><Muted style={{ margin: 0 }}>No sessions yet. Start one from a list's details page.</Muted></Card>}
    {sessions.map(s => { const l = s.snapshot || lists.find(x => x.id === s.listId); const done = s.checked.length === l.items.length; return <Card key={s.id}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16 }}><div><H2>{l.name}</H2><Muted style={{ marginBottom: 'var(--space-3)' }}>{s.createdAt}</Muted></div>
      <Actions style={{ flex: 'none', flexWrap: 'nowrap' }}><Button variant={done ? 'secondary' : 'accent'} size="sm" onClick={() => go('/packing-session/' + s.id)}>{done ? 'Open' : 'Keep packing'}</Button><Button variant="danger" size="sm" onClick={() => confirm('Delete this session') && onDelete(s.id)}>Delete Session</Button></Actions></div>
      <ProgressBar value={s.checked.length} max={l.items.length} />
    </Card>; })}
  </section>;
}
Object.assign(window, { LoginScreen, ListsScreen, ListFormScreen, ListDetailsScreen, SessionScreen, SessionsScreen });