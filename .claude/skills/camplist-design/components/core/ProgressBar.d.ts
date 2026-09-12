import * as React from 'react';
/** Packing progress: pine bar that turns ember when everything is checked. Shows "n / m" count. */
export interface ProgressBarProps {
  /** Checked items. */
  value?: number;
  /** Total items. */
  max?: number;
  /** Left label; defaults to "Packing" / "Packed. Go!". */
  label?: string;
  /** White label/count and translucent track for use on a `tone="brand"` Card. */
  inverse?: boolean;
  style?: React.CSSProperties;
}
export declare function ProgressBar(props: ProgressBarProps): JSX.Element;
