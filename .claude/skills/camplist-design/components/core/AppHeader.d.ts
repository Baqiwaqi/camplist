import * as React from 'react';
/** Top bar: brand link, nav links, "signout", and the signed-in user's name. 900px centered. */
export interface AppHeaderProps {
  /** Signed-in user's display name, shown at the right. */
  userName?: string;
  /** Nav links after the brand. Default: Sessions. */
  links?: { label: string; href: string }[];
  /** Intercepts link clicks for in-page routing. */
  onNavigate?: (href: string) => void;
  /** Brand text. Default "Camplist". */
  brand?: string;
  /** Path to assets/logomark.svg (22px, left of the brand text). */
  logoSrc?: string;
  style?: React.CSSProperties;
}
export declare function AppHeader(props: AppHeaderProps): JSX.Element;
