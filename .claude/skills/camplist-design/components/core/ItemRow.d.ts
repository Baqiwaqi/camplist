import * as React from 'react';
/** Horizontal row for one packing item: flex, 0.75rem gap, hairline divider below. */
export interface ItemRowProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode;
  /** Suppress the bottom divider (last row in a group). */
  last?: boolean;
  style?: React.CSSProperties;
}
export declare function ItemRow(props: ItemRowProps): JSX.Element;
