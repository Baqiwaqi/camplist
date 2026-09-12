import React from 'react';
const variants = {
  primary: { background: 'var(--button-primary-bg)', color: 'var(--button-primary-text)', border: 'var(--border-width) solid var(--button-primary-bg)' },
  accent: { background: 'var(--accent)', color: '#fff', border: 'var(--border-width) solid var(--accent)' },
  secondary: { background: 'var(--button-secondary-bg)', color: 'var(--text-body)', border: 'var(--border-width) solid var(--button-secondary-border)' },
  danger: { background: 'transparent', color: 'var(--button-danger-text)', border: 'var(--border-width) solid var(--error-border)' },
  link: { background: 'transparent', color: 'var(--text-link)', border: 'none', padding: 0, textDecoration: 'underline' },
};
const hovers = { primary: { background: 'var(--brand-hover)', borderColor: 'var(--brand-hover)' }, accent: { background: 'var(--accent-hover)', borderColor: 'var(--accent-hover)' }, secondary: { background: 'var(--surface-page)' }, danger: { background: 'var(--error-bg)' }, link: { opacity: 0.7 } };
export function Button({ variant = 'primary', size = 'md', disabled, children, style, ...rest }) {
  const [hover, setHover] = React.useState(false);
  const pad = size === 'lg' ? 'var(--space-3) var(--space-6)' : size === 'sm' ? 'var(--space-1) var(--space-3)' : 'var(--space-2) var(--space-4)';
  const fs = size === 'lg' ? 'var(--text-lg)' : size === 'sm' ? 'var(--text-sm)' : 'var(--text-base)';
  const lift = hover && !disabled && variant !== 'link' ? 'var(--lift-hover)' : 'none';
  return <button type={rest.type || 'button'} disabled={disabled} onMouseEnter={() => setHover(true)} onMouseLeave={() => setHover(false)} style={{ fontFamily: 'var(--font-body)', fontSize: fs, fontWeight: 'var(--weight-bold)', lineHeight: 'var(--leading-normal)', padding: pad, borderRadius: 'var(--radius-pill)', cursor: disabled ? 'default' : 'pointer', opacity: disabled ? 0.5 : 1, transition: 'background var(--motion-fast), transform var(--motion-fast)', transform: lift, ...variants[variant], ...(hover && !disabled ? hovers[variant] : null), ...style }} {...rest}>{children}</button>;
}
