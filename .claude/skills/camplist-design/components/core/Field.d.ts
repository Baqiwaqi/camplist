import * as React from 'react';
/** Labelled text input or textarea, stacked (label above), full width. */
export interface FieldProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label: string;
  id: string;
  /** Render a textarea instead of an input. */
  multiline?: boolean;
  /** Red border when the field has a validation error. */
  error?: boolean;
  style?: React.CSSProperties;
}
export declare function Field(props: FieldProps): JSX.Element;
