import React from 'react';
const link = { color: 'var(--text-link)', textDecoration: 'none', fontWeight: 'var(--weight-bold)' };
export function AppHeader({ userName, links = [{ label: 'Sessions', href: '/sessions' }], onNavigate, brand = 'Camplist', logoSrc, style }) {
  const go = (href) => (e) => { if (onNavigate) { e.preventDefault(); onNavigate(href); } };
  return <header style={{ borderBottom: 'var(--border-width) solid var(--border-header)', background: 'var(--surface-header)', ...style }}>
    <nav style={{ maxWidth: 'var(--content-max)', margin: '0 auto', padding: 'var(--nav-padding)', display: 'flex', gap: 'var(--space-4)', alignItems: 'center', fontFamily: 'var(--font-body)' }}>
      <a href="/" onClick={go('/')} style={{ ...link, display: 'inline-flex', alignItems: 'center', gap: 8 }}>{logoSrc && <img src={logoSrc} alt="" width="22" height="22" style={{ display: 'block' }} />}{brand}</a>
      {links.map(l => <a key={l.href} href={l.href} onClick={go(l.href)} style={link}>{l.label}</a>)}
      <a href="/auth/signout" onClick={go('/auth/signout')} style={link}>signout</a>
      {userName && <div style={{ marginLeft: 'auto', color: 'var(--text-muted)' }}>{userName}</div>}
    </nav>
  </header>;
}
