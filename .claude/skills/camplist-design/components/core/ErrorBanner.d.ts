import * as React from 'react';
/** Solid red validation block: white alert glyph, display-face heading, messages beneath. 12px radius like Card, no borders. */
export interface ErrorBannerProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Messages rendered as a bullet list. */
  errors?: string[];
  /** Heading; defaults to "Something needs fixing" / "A few things to fix". */
  title?: string;
  children?: React.ReactNode;
  style?: React.CSSProperties;
}
export declare function ErrorBanner(props: ErrorBannerProps): JSX.Element;
