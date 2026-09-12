import React from 'react';
export function Field({ label, id, multiline, error, style, ...rest }) {
  const base = { fontFamily: 'var(--font-body)', fontSize: 'var(--text-base)', color: 'var(--text-body)', background: 'var(--surface-card)', border: 'var(--border-width) solid ' + (error ? 'var(--error-border)' : 'var(--border-header)'), borderRadius: 'var(--radius-sm)', padding: 'var(--space-2) var(--space-3)', width: '100%', boxSizing: 'border-box', lineHeight: 'var(--leading-normal)' };
  return <div style={{ display: 'grid', gap: 'var(--space-1)', marginBottom: 'var(--space-4)', ...style }}>
    <label htmlFor={id} style={{ fontSize: 'var(--text-sm)', fontWeight: 'var(--weight-bold)', color: 'var(--text-body)' }}>{label}</label>
    {multiline ? <textarea id={id} rows={3} style={{ ...base, resize: 'vertical' }} {...rest} /> : <input id={id} type="text" style={base} {...rest} />}
  </div>;
}
