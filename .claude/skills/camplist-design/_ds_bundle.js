/* @ds-bundle: {"format":4,"namespace":"CamplistDesignSystem_8dbf6a","components":[{"name":"AppHeader","sourcePath":"components/core/AppHeader.jsx"},{"name":"Button","sourcePath":"components/core/Button.jsx"},{"name":"Card","sourcePath":"components/core/Card.jsx"},{"name":"ErrorBanner","sourcePath":"components/core/ErrorBanner.jsx"},{"name":"Field","sourcePath":"components/core/Field.jsx"},{"name":"ItemRow","sourcePath":"components/core/ItemRow.jsx"},{"name":"ProgressBar","sourcePath":"components/core/ProgressBar.jsx"}],"sourceHashes":{"components/core/AppHeader.jsx":"450d624c7767","components/core/Button.jsx":"cb28c7774c36","components/core/Card.jsx":"cafa275308b9","components/core/ErrorBanner.jsx":"d53f52b2d891","components/core/Field.jsx":"ee831b77e4e4","components/core/ItemRow.jsx":"c999ce67430e","components/core/ProgressBar.jsx":"8b40a4864efb","ui_kits/camplist/app.jsx":"5f5d3dc43c65","ui_kits/camplist/data.js":"ee61a2c48b6e","ui_kits/camplist/screens.jsx":"ff900fbc0e32"},"inlinedExternals":[],"unexposedExports":[]} */

(() => {

const __ds_ns = (window.CamplistDesignSystem_8dbf6a = window.CamplistDesignSystem_8dbf6a || {});

const __ds_scope = {};

(__ds_ns.__errors = __ds_ns.__errors || []);

// components/core/AppHeader.jsx
try { (() => {
const link = {
  color: 'var(--text-link)',
  textDecoration: 'none',
  fontWeight: 'var(--weight-bold)'
};
function AppHeader({
  userName,
  links = [{
    label: 'Sessions',
    href: '/sessions'
  }],
  onNavigate,
  brand = 'Camplist',
  logoSrc,
  style
}) {
  const go = href => e => {
    if (onNavigate) {
      e.preventDefault();
      onNavigate(href);
    }
  };
  return /*#__PURE__*/React.createElement("header", {
    style: {
      borderBottom: 'var(--border-width) solid var(--border-header)',
      background: 'var(--surface-header)',
      ...style
    }
  }, /*#__PURE__*/React.createElement("nav", {
    style: {
      maxWidth: 'var(--content-max)',
      margin: '0 auto',
      padding: 'var(--nav-padding)',
      display: 'flex',
      gap: 'var(--space-4)',
      alignItems: 'center',
      fontFamily: 'var(--font-body)'
    }
  }, /*#__PURE__*/React.createElement("a", {
    href: "/",
    onClick: go('/'),
    style: {
      ...link,
      display: 'inline-flex',
      alignItems: 'center',
      gap: 8
    }
  }, logoSrc && /*#__PURE__*/React.createElement("img", {
    src: logoSrc,
    alt: "",
    width: "22",
    height: "22",
    style: {
      display: 'block'
    }
  }), brand), links.map(l => /*#__PURE__*/React.createElement("a", {
    key: l.href,
    href: l.href,
    onClick: go(l.href),
    style: link
  }, l.label)), /*#__PURE__*/React.createElement("a", {
    href: "/auth/signout",
    onClick: go('/auth/signout'),
    style: link
  }, "signout"), userName && /*#__PURE__*/React.createElement("div", {
    style: {
      marginLeft: 'auto',
      color: 'var(--text-muted)'
    }
  }, userName)));
}
Object.assign(__ds_scope, { AppHeader });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/AppHeader.jsx", error: String((e && e.message) || e) }); }

// components/core/Button.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
const variants = {
  primary: {
    background: 'var(--button-primary-bg)',
    color: 'var(--button-primary-text)',
    border: 'var(--border-width) solid var(--button-primary-bg)'
  },
  accent: {
    background: 'var(--accent)',
    color: '#fff',
    border: 'var(--border-width) solid var(--accent)'
  },
  secondary: {
    background: 'var(--button-secondary-bg)',
    color: 'var(--text-body)',
    border: 'var(--border-width) solid var(--button-secondary-border)'
  },
  danger: {
    background: 'transparent',
    color: 'var(--button-danger-text)',
    border: 'var(--border-width) solid var(--error-border)'
  },
  link: {
    background: 'transparent',
    color: 'var(--text-link)',
    border: 'none',
    padding: 0,
    textDecoration: 'underline'
  }
};
const hovers = {
  primary: {
    background: 'var(--brand-hover)',
    borderColor: 'var(--brand-hover)'
  },
  accent: {
    background: 'var(--accent-hover)',
    borderColor: 'var(--accent-hover)'
  },
  secondary: {
    background: 'var(--surface-page)'
  },
  danger: {
    background: 'var(--error-bg)'
  },
  link: {
    opacity: 0.7
  }
};
function Button({
  variant = 'primary',
  size = 'md',
  disabled,
  children,
  style,
  ...rest
}) {
  const [hover, setHover] = React.useState(false);
  const pad = size === 'lg' ? 'var(--space-3) var(--space-6)' : size === 'sm' ? 'var(--space-1) var(--space-3)' : 'var(--space-2) var(--space-4)';
  const fs = size === 'lg' ? 'var(--text-lg)' : size === 'sm' ? 'var(--text-sm)' : 'var(--text-base)';
  const lift = hover && !disabled && variant !== 'link' ? 'var(--lift-hover)' : 'none';
  return /*#__PURE__*/React.createElement("button", _extends({
    type: rest.type || 'button',
    disabled: disabled,
    onMouseEnter: () => setHover(true),
    onMouseLeave: () => setHover(false),
    style: {
      fontFamily: 'var(--font-body)',
      fontSize: fs,
      fontWeight: 'var(--weight-bold)',
      lineHeight: 'var(--leading-normal)',
      padding: pad,
      borderRadius: 'var(--radius-pill)',
      cursor: disabled ? 'default' : 'pointer',
      opacity: disabled ? 0.5 : 1,
      transition: 'background var(--motion-fast), transform var(--motion-fast)',
      transform: lift,
      ...variants[variant],
      ...(hover && !disabled ? hovers[variant] : null),
      ...style
    }
  }, rest), children);
}
Object.assign(__ds_scope, { Button });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Button.jsx", error: String((e && e.message) || e) }); }

// components/core/Card.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function Card({
  children,
  style,
  tone,
  ...rest
}) {
  const tones = {
    brand: {
      background: 'var(--brand)',
      color: '#fff',
      border: 'var(--border-width) solid var(--brand)'
    },
    soft: {
      background: 'var(--brand-soft)',
      border: 'var(--border-width) solid var(--brand-soft)'
    }
  };
  return /*#__PURE__*/React.createElement("div", _extends({
    style: {
      background: 'var(--surface-card)',
      border: 'var(--border-width) solid var(--border-card)',
      borderRadius: 'var(--radius-card)',
      padding: 'var(--card-padding)',
      marginBottom: 'var(--card-gap)',
      ...(tone ? tones[tone] : null),
      ...style
    }
  }, rest), children);
}
Object.assign(__ds_scope, { Card });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Card.jsx", error: String((e && e.message) || e) }); }

// components/core/ErrorBanner.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function ErrorBanner({
  errors = [],
  title,
  children,
  style,
  ...rest
}) {
  const heading = title || (errors.length > 1 ? 'A few things to fix' : 'Something needs fixing');
  return /*#__PURE__*/React.createElement("div", _extends({
    role: "alert",
    style: {
      display: 'flex',
      gap: 'var(--space-3)',
      alignItems: 'flex-start',
      background: 'var(--error-text)',
      color: '#fff',
      padding: 'var(--space-3) var(--space-4)',
      borderRadius: 'var(--radius-card)',
      fontFamily: 'var(--font-body)',
      ...style
    }
  }, rest), /*#__PURE__*/React.createElement("span", {
    "aria-hidden": "true",
    style: {
      flex: 'none',
      width: 24,
      height: 24,
      borderRadius: '50%',
      background: '#fff',
      color: 'var(--error-text)',
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontWeight: 800,
      fontSize: 15,
      lineHeight: 1
    }
  }, "!"), /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'grid',
      gap: 2,
      minWidth: 0
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      fontFamily: 'var(--font-display)',
      fontWeight: 'var(--weight-display)',
      fontSize: 'var(--text-lg)',
      lineHeight: 'var(--leading-tight)',
      letterSpacing: 'var(--tracking-display)'
    }
  }, heading), errors.length > 0 && /*#__PURE__*/React.createElement("div", {
    style: {
      fontSize: 'var(--text-sm)',
      lineHeight: 'var(--leading-normal)',
      opacity: .92
    }
  }, errors.map((e, i) => /*#__PURE__*/React.createElement("div", {
    key: i
  }, e))), children));
}
Object.assign(__ds_scope, { ErrorBanner });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/ErrorBanner.jsx", error: String((e && e.message) || e) }); }

// components/core/Field.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function Field({
  label,
  id,
  multiline,
  error,
  style,
  ...rest
}) {
  const base = {
    fontFamily: 'var(--font-body)',
    fontSize: 'var(--text-base)',
    color: 'var(--text-body)',
    background: 'var(--surface-card)',
    border: 'var(--border-width) solid ' + (error ? 'var(--error-border)' : 'var(--border-header)'),
    borderRadius: 'var(--radius-sm)',
    padding: 'var(--space-2) var(--space-3)',
    width: '100%',
    boxSizing: 'border-box',
    lineHeight: 'var(--leading-normal)'
  };
  return /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'grid',
      gap: 'var(--space-1)',
      marginBottom: 'var(--space-4)',
      ...style
    }
  }, /*#__PURE__*/React.createElement("label", {
    htmlFor: id,
    style: {
      fontSize: 'var(--text-sm)',
      fontWeight: 'var(--weight-bold)',
      color: 'var(--text-body)'
    }
  }, label), multiline ? /*#__PURE__*/React.createElement("textarea", _extends({
    id: id,
    rows: 3,
    style: {
      ...base,
      resize: 'vertical'
    }
  }, rest)) : /*#__PURE__*/React.createElement("input", _extends({
    id: id,
    type: "text",
    style: base
  }, rest)));
}
Object.assign(__ds_scope, { Field });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/Field.jsx", error: String((e && e.message) || e) }); }

// components/core/ItemRow.jsx
try { (() => {
function _extends() { return _extends = Object.assign ? Object.assign.bind() : function (n) { for (var e = 1; e < arguments.length; e++) { var t = arguments[e]; for (var r in t) ({}).hasOwnProperty.call(t, r) && (n[r] = t[r]); } return n; }, _extends.apply(null, arguments); }
function ItemRow({
  children,
  last,
  style,
  ...rest
}) {
  return /*#__PURE__*/React.createElement("div", _extends({
    style: {
      display: 'flex',
      gap: 'var(--row-gap)',
      alignItems: 'center',
      padding: 'var(--row-padding)',
      borderBottom: last ? 0 : 'var(--border-width) solid var(--border-row)',
      ...style
    }
  }, rest), children);
}
Object.assign(__ds_scope, { ItemRow });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/ItemRow.jsx", error: String((e && e.message) || e) }); }

// components/core/ProgressBar.jsx
try { (() => {
function ProgressBar({
  value = 0,
  max = 1,
  label,
  inverse = false,
  style
}) {
  const pct = max ? Math.round(value / max * 100) : 0;
  const done = max > 0 && value >= max;
  const labelColor = inverse ? '#fff' : done ? 'var(--accent-hover)' : 'var(--text-body)';
  const countColor = inverse ? 'rgba(255,255,255,.85)' : 'var(--text-muted)';
  const track = inverse ? 'rgba(255,255,255,.25)' : 'var(--progress-track)';
  return /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'grid',
      gap: 'var(--space-1)',
      ...style
    }
  }, (label || max > 0) && /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'flex',
      justifyContent: 'space-between',
      fontSize: 'var(--text-sm)',
      fontWeight: 'var(--weight-bold)',
      color: labelColor
    }
  }, /*#__PURE__*/React.createElement("span", null, label || (done ? 'Packed. Go!' : 'Packing')), /*#__PURE__*/React.createElement("span", {
    style: {
      fontFamily: 'var(--font-mono)',
      fontWeight: 400,
      color: countColor
    }
  }, value, " / ", max)), /*#__PURE__*/React.createElement("div", {
    style: {
      height: 8,
      background: track,
      borderRadius: 'var(--radius-pill)',
      overflow: 'hidden'
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      width: pct + '%',
      height: '100%',
      background: done ? 'var(--progress-done)' : 'var(--progress-fill)',
      borderRadius: 'var(--radius-pill)',
      transition: 'width 240ms ease, background 240ms ease'
    }
  })));
}
Object.assign(__ds_scope, { ProgressBar });
})(); } catch (e) { __ds_ns.__errors.push({ path: "components/core/ProgressBar.jsx", error: String((e && e.message) || e) }); }

// ui_kits/camplist/app.jsx
try { (() => {
const {
  AppHeader
} = window.CamplistDesignSystem_8dbf6a || {};
function CamplistApp() {
  const seed = window.camplistSeed;
  const [auth, setAuth] = React.useState(false);
  const [route, setRoute] = React.useState('/');
  const [lists, setLists] = React.useState(seed.lists);
  const [sessions, setSessions] = React.useState(seed.sessions);
  const go = r => {
    if (r === '/auth/signout') {
      setAuth(false);
      setRoute('/');
      return;
    }
    setRoute(r);
    window.scrollTo(0, 0);
  };
  if (!auth) return /*#__PURE__*/React.createElement(LoginScreen, {
    onLogin: () => setAuth(true)
  });
  const uid = () => Math.random().toString(36).slice(2, 8);
  let body;
  const m = re => route.match(re);
  let mm;
  if (route === '/') body = /*#__PURE__*/React.createElement(ListsScreen, {
    lists: lists,
    sessions: sessions,
    go: go,
    onDelete: id => setLists(ls => ls.filter(l => l.id !== id))
  });else if (route === '/packing-list/new') body = /*#__PURE__*/React.createElement(ListFormScreen, {
    go: go,
    onSave: v => {
      setLists(ls => [...ls, {
        id: uid(),
        items: [],
        ...v
      }]);
      go('/');
    }
  });else if (mm = m(/^\/packing-list\/(\w+)\/edit$/)) {
    const l = lists.find(x => x.id === mm[1]);
    body = /*#__PURE__*/React.createElement(ListFormScreen, {
      key: l.id,
      list: l,
      go: go,
      onSave: v => {
        setLists(ls => ls.map(x => x.id === l.id ? {
          ...x,
          ...v
        } : x));
        go('/');
      }
    });
  } else if (mm = m(/^\/packing-list\/(\w+)$/)) {
    const l = lists.find(x => x.id === mm[1]);
    body = /*#__PURE__*/React.createElement(ListDetailsScreen, {
      list: l,
      go: go,
      onAddItem: it => setLists(ls => ls.map(x => x.id === l.id ? {
        ...x,
        items: [...x.items, {
          id: uid(),
          ...it
        }]
      } : x)),
      onRemoveItem: iid => setLists(ls => ls.map(x => x.id === l.id ? {
        ...x,
        items: x.items.filter(i => i.id !== iid)
      } : x)),
      onStartSession: () => {
        const id = uid();
        setSessions(ss => [{
          id,
          listId: l.id,
          createdAt: 'Sep 12, 2026',
          checked: [],
          snapshot: JSON.parse(JSON.stringify(l))
        }, ...ss]);
        go('/packing-session/' + id);
      }
    });
  } else if (mm = m(/^\/packing-session\/(\w+)$/)) {
    const s = sessions.find(x => x.id === mm[1]);
    const l = s.snapshot || lists.find(x => x.id === s.listId);
    body = /*#__PURE__*/React.createElement(SessionScreen, {
      session: s,
      list: l,
      go: go,
      onToggle: iid => setSessions(ss => ss.map(x => x.id === s.id ? {
        ...x,
        checked: x.checked.includes(iid) ? x.checked.filter(c => c !== iid) : [...x.checked, iid]
      } : x))
    });
  } else if (route === '/sessions') body = /*#__PURE__*/React.createElement(SessionsScreen, {
    sessions: sessions,
    lists: lists,
    go: go,
    onDelete: id => setSessions(ss => ss.filter(s => s.id !== id))
  });else body = /*#__PURE__*/React.createElement("p", null, "Not found");
  return /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(AppHeader, {
    userName: seed.user,
    logoSrc: "../../assets/logomark.svg",
    onNavigate: go
  }), /*#__PURE__*/React.createElement("main", {
    style: {
      maxWidth: 'var(--content-max)',
      margin: '0 auto',
      padding: 'var(--main-padding)'
    }
  }, body));
}
const camplistRoot = document.getElementById('root');
if (camplistRoot && window.LoginScreen) ReactDOM.createRoot(camplistRoot).render(/*#__PURE__*/React.createElement(CamplistApp, null));
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/camplist/app.jsx", error: String((e && e.message) || e) }); }

// ui_kits/camplist/data.js
try { (() => {
window.camplistSeed = {
  user: 'Sam Rivera',
  lists: [{
    id: 'l1',
    name: 'Weekend camping',
    description: 'Two nights, car access, no showers',
    items: [{
      id: 'i1',
      name: 'Tent',
      category: 'Shelter'
    }, {
      id: 'i2',
      name: 'Sleeping bag',
      category: 'Sleep'
    }, {
      id: 'i3',
      name: 'Sleeping pad',
      category: 'Sleep'
    }, {
      id: 'i4',
      name: 'Headlamp',
      category: 'Light'
    }, {
      id: 'i5',
      name: 'Stove + gas',
      category: 'Kitchen'
    }, {
      id: 'i6',
      name: 'Water filter',
      category: 'Kitchen'
    }, {
      id: 'i7',
      name: 'Rain jacket',
      category: 'Clothing'
    }]
  }, {
    id: 'l2',
    name: 'Day hike',
    description: 'Light pack, back before dark',
    items: [{
      id: 'i8',
      name: 'Daypack',
      category: 'Bags'
    }, {
      id: 'i9',
      name: 'Snacks',
      category: 'Food'
    }, {
      id: 'i10',
      name: 'First aid kit',
      category: 'Safety'
    }]
  }],
  sessions: [{
    id: 's1',
    listId: 'l1',
    createdAt: 'Sep 5, 2026',
    checked: ['i1', 'i2', 'i4']
  }, {
    id: 's2',
    listId: 'l2',
    createdAt: 'Aug 22, 2026',
    checked: ['i8', 'i9', 'i10']
  }]
};
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/camplist/data.js", error: String((e && e.message) || e) }); }

// ui_kits/camplist/screens.jsx
try { (() => {
const camplistNs = window.CamplistDesignSystem_8dbf6a || {};
const {
  Card,
  ItemRow,
  ErrorBanner,
  AppHeader,
  Button,
  Field
} = camplistNs;
const ProgressBar = camplistNs.ProgressBar || (() => null);
const Muted = ({
  children,
  style
}) => /*#__PURE__*/React.createElement("p", {
  style: {
    color: 'var(--text-muted)',
    margin: '0 0 1rem',
    ...style
  }
}, children);
const H1 = ({
  children,
  style
}) => /*#__PURE__*/React.createElement("h1", {
  style: {
    fontFamily: 'var(--font-display)',
    fontWeight: 'var(--weight-display)',
    fontSize: 'var(--text-3xl)',
    lineHeight: 'var(--leading-tight)',
    letterSpacing: 'var(--tracking-display)',
    margin: '0 0 .5rem',
    ...style
  }
}, children);
const H2 = ({
  children,
  style
}) => /*#__PURE__*/React.createElement("h2", {
  style: {
    fontFamily: 'var(--font-display)',
    fontWeight: 'var(--weight-display)',
    fontSize: 'var(--text-xl)',
    lineHeight: 'var(--leading-tight)',
    letterSpacing: 'var(--tracking-display)',
    margin: '0 0 .25rem',
    ...style
  }
}, children);
const Eyebrow = ({
  children
}) => /*#__PURE__*/React.createElement("div", {
  style: {
    fontSize: 'var(--text-xs)',
    fontWeight: 'var(--weight-bold)',
    letterSpacing: 'var(--tracking-eyebrow)',
    textTransform: 'uppercase',
    color: 'var(--text-eyebrow)',
    marginBottom: 'var(--space-2)'
  }
}, children);
const Actions = ({
  children,
  style
}) => /*#__PURE__*/React.createElement("div", {
  style: {
    display: 'flex',
    gap: 'var(--space-2)',
    alignItems: 'center',
    flexWrap: 'wrap',
    ...style
  }
}, children);
const Cat = ({
  children
}) => /*#__PURE__*/React.createElement("span", {
  style: {
    whiteSpace: 'nowrap',
    flex: 'none',
    fontSize: 'var(--text-xs)',
    fontWeight: 700,
    letterSpacing: '0.04em',
    textTransform: 'uppercase',
    color: 'var(--brand)',
    background: 'var(--brand-soft)',
    padding: '2px 8px',
    borderRadius: 'var(--radius-pill)'
  }
}, children);
const Tick = ({
  on
}) => /*#__PURE__*/React.createElement("span", {
  style: {
    width: 22,
    height: 22,
    borderRadius: '50%',
    flex: 'none',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    border: '2px solid ' + (on ? 'var(--brand)' : 'var(--border-header)'),
    background: on ? 'var(--brand)' : 'transparent',
    color: '#fff',
    fontSize: 13,
    fontWeight: 700,
    transition: 'all var(--motion-fast)'
  }
}, on ? '✓' : '');
function LoginScreen({
  onLogin
}) {
  return /*#__PURE__*/React.createElement("div", {
    style: {
      minHeight: '100vh',
      display: 'grid',
      gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)'
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      background: 'var(--brand)',
      color: '#fff',
      padding: 'var(--space-8)',
      display: 'flex',
      flexDirection: 'column',
      justifyContent: 'space-between'
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'flex',
      alignItems: 'center',
      gap: 10,
      fontWeight: 700,
      fontSize: 'var(--text-lg)'
    }
  }, /*#__PURE__*/React.createElement("img", {
    src: "../../assets/logomark-white.svg",
    alt: "",
    width: "28",
    height: "28"
  }), "Camplist"), /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement("div", {
    style: {
      fontFamily: 'var(--font-display)',
      fontWeight: 800,
      fontSize: 'var(--text-4xl)',
      lineHeight: 1,
      letterSpacing: 'var(--tracking-display)',
      marginBottom: 'var(--space-4)'
    }
  }, "Pack once.", /*#__PURE__*/React.createElement("br", null), "Go every weekend."), /*#__PURE__*/React.createElement("p", {
    style: {
      margin: 0,
      fontSize: 'var(--text-lg)',
      opacity: .85,
      maxWidth: 380
    }
  }, "Keep your camping kit as a list, start a session for each trip, and get out the door faster than last time.")), /*#__PURE__*/React.createElement("div", {
    style: {
      fontSize: 'var(--text-sm)',
      opacity: .7
    }
  }, "Weekend camping \xB7 Day hike \xB7 Canoe trip")), /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'grid',
      placeItems: 'center',
      padding: 'var(--space-8)'
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      width: 340
    }
  }, /*#__PURE__*/React.createElement(Eyebrow, null, "Sign in"), /*#__PURE__*/React.createElement(H1, {
    style: {
      marginBottom: 'var(--space-4)'
    }
  }, "Let's get packing."), /*#__PURE__*/React.createElement(Button, {
    variant: "secondary",
    size: "lg",
    onClick: onLogin,
    style: {
      width: '100%'
    }
  }, "Login with google"), /*#__PURE__*/React.createElement(Muted, {
    style: {
      marginTop: 'var(--space-4)',
      fontSize: 'var(--text-sm)'
    }
  }, "Your lists and sessions stay with your Google account."))));
}
function ListsScreen({
  lists,
  sessions,
  go,
  onDelete
}) {
  return /*#__PURE__*/React.createElement("section", null, /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'flex-end',
      gap: 16,
      marginBottom: 'var(--space-6)'
    }
  }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(Eyebrow, null, "Your lists"), /*#__PURE__*/React.createElement(H1, null, "Where to next?"), /*#__PURE__*/React.createElement(Muted, {
    style: {
      margin: 0
    }
  }, "Pick a list and start a session \u2014 your list stays untouched.")), /*#__PURE__*/React.createElement(Button, {
    onClick: () => go('/packing-list/new')
  }, "Create")), lists.length === 0 && /*#__PURE__*/React.createElement(Card, null, /*#__PURE__*/React.createElement(Muted, {
    style: {
      margin: 0
    }
  }, "No lists yet. Create one to get started.")), lists.map(l => /*#__PURE__*/React.createElement(Card, {
    key: l.id
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'flex',
      gap: 16,
      alignItems: 'flex-start'
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      flex: 1
    }
  }, /*#__PURE__*/React.createElement(H2, null, l.name), /*#__PURE__*/React.createElement(Muted, {
    style: {
      marginBottom: 'var(--space-3)'
    }
  }, l.description), /*#__PURE__*/React.createElement(Actions, null, /*#__PURE__*/React.createElement(Cat, null, l.items.length, " items"), [...new Set(l.items.map(i => i.category))].slice(0, 4).map(c => /*#__PURE__*/React.createElement("span", {
    key: c,
    style: {
      fontSize: 'var(--text-sm)',
      color: 'var(--text-muted)'
    }
  }, c)))), /*#__PURE__*/React.createElement(Actions, {
    style: {
      flex: 'none',
      flexWrap: 'nowrap'
    }
  }, /*#__PURE__*/React.createElement(Button, {
    variant: "secondary",
    size: "sm",
    onClick: () => go('/packing-list/' + l.id + '/edit')
  }, "Edit"), /*#__PURE__*/React.createElement(Button, {
    variant: "secondary",
    size: "sm",
    onClick: () => go('/packing-list/' + l.id)
  }, "Details"), /*#__PURE__*/React.createElement(Button, {
    variant: "danger",
    size: "sm",
    onClick: () => confirm('Delete this list?') && onDelete(l.id)
  }, "Delete"))))));
}
function ListFormScreen({
  list,
  go,
  onSave
}) {
  const [name, setName] = React.useState(list ? list.name : '');
  const [desc, setDesc] = React.useState(list ? list.description : '');
  const [errors, setErrors] = React.useState([]);
  const submit = e => {
    e.preventDefault();
    if (!name.trim()) return setErrors(['Name is required']);
    onSave({
      name,
      description: desc
    });
  };
  return /*#__PURE__*/React.createElement("section", {
    style: {
      maxWidth: 560
    }
  }, /*#__PURE__*/React.createElement(Eyebrow, null, list ? 'Edit list' : 'New list'), /*#__PURE__*/React.createElement(H1, null, list ? list.name : 'Start a new list'), !list && /*#__PURE__*/React.createElement(Muted, null, "Start simple and adjust over time. You'll refine it after the first trip."), /*#__PURE__*/React.createElement(Card, null, /*#__PURE__*/React.createElement("form", {
    onSubmit: submit
  }, /*#__PURE__*/React.createElement(Field, {
    label: "Name",
    id: "name",
    placeholder: "Camping",
    value: name,
    error: errors.length > 0,
    onChange: e => setName(e.target.value)
  }), /*#__PURE__*/React.createElement(Field, {
    label: "Description",
    id: "description",
    multiline: true,
    placeholder: "A short description of your list",
    value: desc,
    onChange: e => setDesc(e.target.value)
  }), errors.length > 0 && /*#__PURE__*/React.createElement(ErrorBanner, {
    errors: errors,
    style: {
      marginBottom: 'var(--space-4)'
    }
  }), /*#__PURE__*/React.createElement(Actions, null, /*#__PURE__*/React.createElement(Button, {
    variant: "link",
    onClick: () => go('/')
  }, "Cancel"), /*#__PURE__*/React.createElement(Button, {
    type: "submit",
    style: {
      marginLeft: 'auto'
    }
  }, list ? 'Save' : 'Create list')))));
}
function ListDetailsScreen({
  list,
  go,
  onAddItem,
  onRemoveItem,
  onStartSession
}) {
  const [name, setName] = React.useState('');
  const [cat, setCat] = React.useState('');
  const [errors, setErrors] = React.useState([]);
  const submit = e => {
    e.preventDefault();
    if (!name.trim()) return setErrors(['Name is required']);
    onAddItem({
      name,
      category: cat
    });
    setName('');
    setCat('');
    setErrors([]);
  };
  return /*#__PURE__*/React.createElement("section", null, /*#__PURE__*/React.createElement(Card, {
    tone: "brand",
    style: {
      padding: 'var(--space-6)',
      marginBottom: 'var(--space-6)'
    }
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'flex-end',
      gap: 16,
      flexWrap: 'wrap'
    }
  }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement("div", {
    style: {
      fontSize: 'var(--text-xs)',
      fontWeight: 700,
      letterSpacing: 'var(--tracking-eyebrow)',
      textTransform: 'uppercase',
      opacity: .8,
      marginBottom: 8
    }
  }, "Packing list \xB7 ", list.items.length, " items"), /*#__PURE__*/React.createElement(H1, {
    style: {
      margin: 0
    }
  }, list.name), /*#__PURE__*/React.createElement("p", {
    style: {
      margin: '8px 0 0',
      opacity: .85
    }
  }, list.description)), /*#__PURE__*/React.createElement(Actions, null, /*#__PURE__*/React.createElement(Button, {
    variant: "secondary",
    onClick: () => go('/packing-list/' + list.id + '/edit')
  }, "Edit"), /*#__PURE__*/React.createElement(Button, {
    variant: "accent",
    size: "lg",
    onClick: onStartSession
  }, "Start session \u2192")))), /*#__PURE__*/React.createElement(Card, null, list.items.length === 0 && /*#__PURE__*/React.createElement(Muted, {
    style: {
      margin: 0
    }
  }, "No items yet. Add your first one below."), list.items.map((it, i) => /*#__PURE__*/React.createElement(ItemRow, {
    key: it.id,
    last: i === list.items.length - 1
  }, /*#__PURE__*/React.createElement("span", {
    style: {
      fontWeight: 700
    }
  }, it.name), it.category && /*#__PURE__*/React.createElement(Cat, null, it.category), /*#__PURE__*/React.createElement(Button, {
    variant: "danger",
    size: "sm",
    style: {
      marginLeft: 'auto'
    },
    onClick: () => confirm('Delete this item?') && onRemoveItem(it.id)
  }, "Delete")))), /*#__PURE__*/React.createElement(H2, {
    style: {
      margin: '0 0 .75rem'
    }
  }, "Add Item"), /*#__PURE__*/React.createElement(Card, null, /*#__PURE__*/React.createElement("form", {
    onSubmit: submit
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'grid',
      gridTemplateColumns: '1fr 1fr',
      gap: 'var(--space-4)'
    }
  }, /*#__PURE__*/React.createElement(Field, {
    label: "Name",
    id: "item-name",
    placeholder: "Headlamp",
    value: name,
    error: errors.length > 0,
    onChange: e => setName(e.target.value),
    style: {
      marginBottom: 0
    }
  }), /*#__PURE__*/React.createElement(Field, {
    label: "Category",
    id: "item-category",
    placeholder: "Light",
    value: cat,
    onChange: e => setCat(e.target.value),
    style: {
      marginBottom: 0
    }
  })), errors.length > 0 && /*#__PURE__*/React.createElement(ErrorBanner, {
    errors: errors,
    style: {
      marginTop: 'var(--space-4)'
    }
  }), /*#__PURE__*/React.createElement(Actions, {
    style: {
      marginTop: 'var(--space-4)'
    }
  }, /*#__PURE__*/React.createElement(Button, {
    type: "submit"
  }, "Add Item")))));
}
function SessionScreen({
  session,
  list,
  onToggle,
  go
}) {
  const checked = new Set(session.checked);
  const done = checked.size === list.items.length && list.items.length > 0;
  const cats = [...new Set(list.items.map(i => i.category || 'Other'))];
  return /*#__PURE__*/React.createElement("section", null, /*#__PURE__*/React.createElement("div", {
    style: {
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'flex-end',
      gap: 16,
      marginBottom: 'var(--space-4)'
    }
  }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(Eyebrow, null, "Packing session \xB7 ", session.createdAt), /*#__PURE__*/React.createElement(H1, {
    style: {
      margin: 0
    }
  }, done ? 'All packed. Go!' : list.name)), /*#__PURE__*/React.createElement(Button, {
    variant: "secondary",
    size: "sm",
    onClick: () => go('/sessions')
  }, "All sessions")), /*#__PURE__*/React.createElement(Card, {
    tone: done ? 'brand' : undefined
  }, /*#__PURE__*/React.createElement(ProgressBar, {
    inverse: done,
    value: checked.size,
    max: list.items.length,
    label: done ? 'Packed. Go!' : "Let's pack " + list.name.toLowerCase()
  })), cats.map(c => /*#__PURE__*/React.createElement(Card, {
    key: c
  }, /*#__PURE__*/React.createElement("div", {
    style: {
      fontSize: 'var(--text-xs)',
      fontWeight: 700,
      letterSpacing: 'var(--tracking-eyebrow)',
      textTransform: 'uppercase',
      color: 'var(--text-muted)',
      paddingBottom: 'var(--space-1)'
    }
  }, c), list.items.filter(i => (i.category || 'Other') === c).map((it, i, arr) => {
    const on = checked.has(it.id);
    return /*#__PURE__*/React.createElement(ItemRow, {
      key: it.id,
      last: i === arr.length - 1,
      onClick: () => onToggle(it.id),
      style: {
        cursor: 'pointer'
      }
    }, /*#__PURE__*/React.createElement(Tick, {
      on: on
    }), /*#__PURE__*/React.createElement("span", {
      style: {
        fontWeight: 700,
        textDecoration: on ? 'line-through' : 'none',
        color: on ? 'var(--text-muted)' : 'var(--text-body)'
      }
    }, it.name), /*#__PURE__*/React.createElement(Button, {
      size: "sm",
      variant: on ? 'secondary' : 'accent',
      style: {
        marginLeft: 'auto'
      },
      onClick: e => {
        e.stopPropagation();
        onToggle(it.id);
      }
    }, on ? 'Uncheck' : 'Check'));
  }))));
}
function SessionsScreen({
  sessions,
  lists,
  go,
  onDelete
}) {
  return /*#__PURE__*/React.createElement("section", null, /*#__PURE__*/React.createElement(Eyebrow, null, "Sessions"), /*#__PURE__*/React.createElement(H1, null, "Trips in progress"), /*#__PURE__*/React.createElement(Muted, null, "Each session is a snapshot \u2014 check things off without touching the list."), sessions.length === 0 && /*#__PURE__*/React.createElement(Card, null, /*#__PURE__*/React.createElement(Muted, {
    style: {
      margin: 0
    }
  }, "No sessions yet. Start one from a list's details page.")), sessions.map(s => {
    const l = s.snapshot || lists.find(x => x.id === s.listId);
    const done = s.checked.length === l.items.length;
    return /*#__PURE__*/React.createElement(Card, {
      key: s.id
    }, /*#__PURE__*/React.createElement("div", {
      style: {
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'flex-start',
        gap: 16
      }
    }, /*#__PURE__*/React.createElement("div", null, /*#__PURE__*/React.createElement(H2, null, l.name), /*#__PURE__*/React.createElement(Muted, {
      style: {
        marginBottom: 'var(--space-3)'
      }
    }, s.createdAt)), /*#__PURE__*/React.createElement(Actions, {
      style: {
        flex: 'none',
        flexWrap: 'nowrap'
      }
    }, /*#__PURE__*/React.createElement(Button, {
      variant: done ? 'secondary' : 'accent',
      size: "sm",
      onClick: () => go('/packing-session/' + s.id)
    }, done ? 'Open' : 'Keep packing'), /*#__PURE__*/React.createElement(Button, {
      variant: "danger",
      size: "sm",
      onClick: () => confirm('Delete this session') && onDelete(s.id)
    }, "Delete Session"))), /*#__PURE__*/React.createElement(ProgressBar, {
      value: s.checked.length,
      max: l.items.length
    }));
  }));
}
Object.assign(window, {
  LoginScreen,
  ListsScreen,
  ListFormScreen,
  ListDetailsScreen,
  SessionScreen,
  SessionsScreen
});
})(); } catch (e) { __ds_ns.__errors.push({ path: "ui_kits/camplist/screens.jsx", error: String((e && e.message) || e) }); }

__ds_ns.AppHeader = __ds_scope.AppHeader;

__ds_ns.Button = __ds_scope.Button;

__ds_ns.Card = __ds_scope.Card;

__ds_ns.ErrorBanner = __ds_scope.ErrorBanner;

__ds_ns.Field = __ds_scope.Field;

__ds_ns.ItemRow = __ds_scope.ItemRow;

__ds_ns.ProgressBar = __ds_scope.ProgressBar;

})();
