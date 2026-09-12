import * as React from 'react';
/** White content container with a sand border and 12px radius. The only elevation device in Camplist (no shadows).
 * @startingPoint section="Camplist" subtitle="White card with sand border" viewport="700x180" */
export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  children?: React.ReactNode;
  /** `brand` = solid pine with white text (hero/session header); `soft` = pale pine. Default white. */
  tone?: 'brand' | 'soft';
  style?: React.CSSProperties;
}
export declare function Card(props: CardProps): JSX.Element;
