import React from 'react';
export function ProgressBar({ value = 0, max = 1, label, inverse = false, style }) {
  const pct = max ? Math.round((value / max) * 100) : 0;
  const done = max > 0 && value >= max;
  const labelColor = inverse ? '#fff' : done ? 'var(--accent-hover)' : 'var(--text-body)';
  const countColor = inverse ? 'rgba(255,255,255,.85)' : 'var(--text-muted)';
  const track = inverse ? 'rgba(255,255,255,.25)' : 'var(--progress-track)';
  return <div style={{ display: 'grid', gap: 'var(--space-1)', ...style }}>
    {(label || max > 0) && <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 'var(--text-sm)', fontWeight: 'var(--weight-bold)', color: labelColor }}><span>{label || (done ? 'Packed. Go!' : 'Packing')}</span><span style={{ fontFamily: 'var(--font-mono)', fontWeight: 400, color: countColor }}>{value} / {max}</span></div>}
    <div style={{ height: 8, background: track, borderRadius: 'var(--radius-pill)', overflow: 'hidden' }}><div style={{ width: pct + '%', height: '100%', background: done ? 'var(--progress-done)' : 'var(--progress-fill)', borderRadius: 'var(--radius-pill)', transition: 'width 240ms ease, background 240ms ease' }} /></div>
  </div>;
}
