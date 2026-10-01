"use client";

import { useId, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from "react";

/**
 * The two pieces every form here repeats: a labelled input whose error sits UNDER it and is
 * announced with it, and the form-level message. Labels are always above the input, never a
 * placeholder (skills/accessibility-elderly #4).
 */

type FieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, "id" | "aria-describedby" | "aria-invalid"> & {
  label: string;
  hint?: ReactNode;
  error?: string | null;
};

export function TextField({ label, hint, error, ...input }: FieldProps) {
  const id = useId();
  const hintId = hint ? id + "-hint" : undefined;
  const errorId = error ? id + "-error" : undefined;
  const describedBy = [hintId, errorId].filter(Boolean).join(" ") || undefined;
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {hint ? (
        <p id={hintId} className="field-hint">
          {hint}
        </p>
      ) : null}
      <input id={id} aria-describedby={describedBy} aria-invalid={error ? true : undefined} {...input} />
      {error ? (
        <p id={errorId} className="field-error">
          {error}
        </p>
      ) : null}
    </div>
  );
}

type AreaProps = Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, "id" | "aria-describedby" | "aria-invalid"> & {
  label: string;
  hint?: ReactNode;
  error?: string | null;
};

export function TextAreaField({ label, hint, error, ...area }: AreaProps) {
  const id = useId();
  const hintId = hint ? id + "-hint" : undefined;
  const errorId = error ? id + "-error" : undefined;
  const describedBy = [hintId, errorId].filter(Boolean).join(" ") || undefined;
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {hint ? (
        <p id={hintId} className="field-hint">
          {hint}
        </p>
      ) : null}
      <textarea id={id} aria-describedby={describedBy} aria-invalid={error ? true : undefined} {...area} />
      {error ? (
        <p id={errorId} className="field-error">
          {error}
        </p>
      ) : null}
    </div>
  );
}

/**
 * Always in the DOM, even empty: a live region inserted after load is missed by some screen
 * readers, and adding it later shifts the form.
 */
export function FormMessage({ text, trace }: { text: string | null; trace?: string | null }) {
  return (
    <div className="form-error" role="alert" aria-live="assertive">
      {text ? <p>{text}</p> : null}
      {text && trace ? <p className="trace-line">{trace}</p> : null}
    </div>
  );
}
