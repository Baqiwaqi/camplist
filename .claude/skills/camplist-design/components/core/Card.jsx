import React from 'react';
export function Card({ children, style, tone, ...rest }) {
  const tones = { brand: { background: 'var(--brand)', color: '#fff', border: 'var(--border-width) solid var(--brand)' }, soft: { background: 'var(--brand-soft)', border: 'var(--border-width) solid var(--brand-soft)' } };
  return <div style={{ background: 'var(--surface-card)', border: 'var(--border-width) solid var(--border-card)', borderRadius: 'var(--radius-card)', padding: 'var(--card-padding)', marginBottom: 'var(--card-gap)', ...(tone ? tones[tone] : null), ...style }} {...rest}>{children}</div>;
}
