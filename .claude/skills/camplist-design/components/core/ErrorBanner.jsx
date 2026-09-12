import React from 'react';
export function ErrorBanner({ errors = [], title, children, style, ...rest }) {
  const heading = title || (errors.length > 1 ? 'A few things to fix' : 'Something needs fixing');
  return <div role="alert" style={{ display: 'flex', gap: 'var(--space-3)', alignItems: 'flex-start', background: 'var(--error-text)', color: '#fff', padding: 'var(--space-3) var(--space-4)', borderRadius: 'var(--radius-card)', fontFamily: 'var(--font-body)', ...style }} {...rest}>
    <span aria-hidden="true" style={{ flex: 'none', width: 24, height: 24, borderRadius: '50%', background: '#fff', color: 'var(--error-text)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontWeight: 800, fontSize: 15, lineHeight: 1 }}>!</span>
    <div style={{ display: 'grid', gap: 2, minWidth: 0 }}>
      <div style={{ fontFamily: 'var(--font-display)', fontWeight: 'var(--weight-display)', fontSize: 'var(--text-lg)', lineHeight: 'var(--leading-tight)', letterSpacing: 'var(--tracking-display)' }}>{heading}</div>
      {errors.length > 0 && <div style={{ fontSize: 'var(--text-sm)', lineHeight: 'var(--leading-normal)', opacity: .92 }}>{errors.map((e, i) => <div key={i}>{e}</div>)}</div>}
      {children}
    </div>
  </div>;
}
