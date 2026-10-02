"use client";

import { CircleAlert, type LucideIcon } from "lucide-react";
import { useId, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from "react";

import { cn } from "@/lib/cn";

/**
 * The pieces every form here repeats: a labelled input whose error sits UNDER it and is announced
 * with it, and the form-level message. Labels are always above the input, never a placeholder
 * (skills/accessibility-elderly #4).
 *
 * LOOK: web-admin's control frame (`web-admin/src/components/ui/field.tsx` `controlClass`) — hairline,
 * `--r-control` radius, brand focus ring — at 44px instead of 40px: this console's inputs were 44px
 * touch targets before the redesign and stay so. 16px text below 768px, because iOS zooms the page
 * on focus into anything smaller.
 *
 * The native `<input>` / `<textarea>` keeps every prop the screen passes (`name`, `autoComplete`,
 * `inputMode`, `maxLength`, `value`, `onChange`): this file draws, it does not decide.
 */

/** Frame shared by every native control on this console; exported for the one bare `<select>`. */
export const controlClass = cn(
  "h-11 w-full min-w-0 rounded-control border border-line-strong bg-surface px-3 [font-family:inherit] text-base text-ink-900 md:text-sm",
  "transition-[border-color,box-shadow] duration-150",
  "placeholder:text-ink-500 hover:not-disabled:border-[#c3ccd9]",
  "focus-visible:border-brand-500 focus-visible:shadow-[0_0_0_3px_var(--brand-100)] focus-visible:outline-none",
  "disabled:cursor-not-allowed disabled:bg-surface-muted disabled:text-ink-500",
  // An invalid field is marked by a thicker border AND the sentence under it — never colour alone.
  "aria-invalid:border-2 aria-invalid:border-danger-600",
);

export const labelClass = "text-[13px] leading-tight font-semibold text-ink-700";

export const hintClass = "m-0 text-[13px] text-ink-500";

export function FieldError({ id, text }: { id: string; text: string }) {
  return (
    <p id={id} className="m-0 flex items-start gap-1.5 text-[13px] font-semibold text-danger-600">
      <CircleAlert aria-hidden="true" focusable="false" strokeWidth={2} className="mt-0.5 size-3.5 shrink-0" />
      {text}
    </p>
  );
}

type FieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, "id" | "aria-describedby" | "aria-invalid"> & {
  label: string;
  hint?: ReactNode;
  error?: string | null;
  /** Leading decorative icon inside the control (the sign-in card, like web-admin's). */
  icon?: LucideIcon;
  /** 46px instead of 44px — the sign-in card's inputs (web-admin `.login-input`). */
  tall?: boolean;
};

export function TextField({ label, hint, error, icon: Icon, tall = false, ...input }: FieldProps) {
  const id = useId();
  const hintId = hint ? id + "-hint" : undefined;
  const errorId = error ? id + "-error" : undefined;
  const describedBy = [hintId, errorId].filter(Boolean).join(" ") || undefined;
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <label htmlFor={id} className={labelClass}>
        {label}
      </label>
      {hint ? (
        <p id={hintId} className={hintClass}>
          {hint}
        </p>
      ) : null}
      <div className="relative">
        {Icon ? (
          <Icon
            aria-hidden="true"
            focusable="false"
            strokeWidth={1.8}
            className="pointer-events-none absolute top-1/2 left-3.5 size-[18px] -translate-y-1/2 text-ink-500"
          />
        ) : null}
        <input
          id={id}
          aria-describedby={describedBy}
          aria-invalid={error ? true : undefined}
          className={cn(controlClass, tall && "h-[46px]", Icon && "pl-11")}
          {...input}
        />
      </div>
      {error && errorId ? <FieldError id={errorId} text={error} /> : null}
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
    <div className="flex min-w-0 flex-col gap-1.5">
      <label htmlFor={id} className={labelClass}>
        {label}
      </label>
      {hint ? (
        <p id={hintId} className={hintClass}>
          {hint}
        </p>
      ) : null}
      <textarea
        id={id}
        aria-describedby={describedBy}
        aria-invalid={error ? true : undefined}
        className={cn(controlClass, "h-auto min-h-20 resize-y py-2.5 leading-normal")}
        {...area}
      />
      {error && errorId ? <FieldError id={errorId} text={error} /> : null}
    </div>
  );
}

/**
 * Always in the DOM, even empty: a live region inserted after load is missed by some screen
 * readers, and adding it later shifts the form. Empty, it keeps only a small reserved height; with a
 * sentence it gains a soft red frame so it reads as a message, not stray red text (web-admin
 * `.thong-bao-loi`).
 */
export function FormMessage({ text, trace }: { text: string | null; trace?: string | null }) {
  return (
    <div
      className={cn(
        "min-h-5 text-sm text-danger-600",
        text ? "flex items-start gap-2 rounded-control border border-danger-200 bg-danger-50 px-3 py-2" : null,
      )}
      role="alert"
      aria-live="assertive"
    >
      {text ? (
        <CircleAlert aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-0.5 size-4 shrink-0" />
      ) : null}
      {text ? (
        <div className="min-w-0">
          <p className="m-0">{text}</p>
          {trace ? <p className="m-0 mt-1 text-[13px] text-ink-500">{trace}</p> : null}
        </div>
      ) : null}
    </div>
  );
}
