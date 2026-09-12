import React from 'react';
export function ItemRow({ children, last, style, ...rest }) {
  return <div style={{ display: 'flex', gap: 'var(--row-gap)', alignItems: 'center', padding: 'var(--row-padding)', borderBottom: last ? 0 : 'var(--border-width) solid var(--border-row)', ...style }} {...rest}>{children}</div>;
}
