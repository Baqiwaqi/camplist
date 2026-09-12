import * as React from 'react';
/** Pill action button. `accent` (ember orange) is reserved for the one action that gets you out the door — Start session, Check.
 * @startingPoint section="Camplist" subtitle="Primary / accent / secondary / danger / link" viewport="700x140" */
export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'accent' | 'secondary' | 'danger' | 'link';
  size?: 'sm' | 'md' | 'lg';
  disabled?: boolean;
  children?: React.ReactNode;
  style?: React.CSSProperties;
}
export declare function Button(props: ButtonProps): JSX.Element;
