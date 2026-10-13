"use client";

import { ChevronDown, CircleAlert, type LucideIcon } from "lucide-react";
import { useId, type InputHTMLAttributes, type ReactNode, type TextareaHTMLAttributes } from "react";

import { cn } from "@/lib/cn";

/**
 * The pieces every form here repeats: a labelled input whose error sits UNDER it and is announced
 * with it, and the form-level message. Labels are always above the input, never a placeholder
 * (skills/accessibility-elderly #4).
 *
 * LOOK: web-admin's Field (`web-admin/src/components/ui/field.tsx`, spec 00 §5, ADR 0068 lần 6) —
 * shadcn's Input: 32px, `rounded-lg`, the `input` hairline, focus = the `ring` border plus a 3px
 * `ring/50` halo; the label navy 13px/600 above, the hint and the error 12px under the control.
 * 32px so a control and the h-8 button beside it line up (until 13/10/2026 this console drew 44px
 * inputs next to 32px buttons). 16px text below 768px, because iOS zooms the page on focus into
 * anything smaller. Copied, not shared (no package yet): change both.
 *
 * DIFFERENCE FROM WEB-ADMIN: the hint stays visible while an error shows. web-admin hides it; here
 * several hints carry the rule the error refers to (a password's length, a domain's shape), and the
 * control's `aria-describedby` already names both.
 *
 * The native `<input>` / `<textarea>` keeps every prop the screen passes (`name`, `autoComplete`,
 * `inputMode`, `maxLength`, `value`, `onChange`): this file draws, it does not decide.
 */

/** Frame shared by every native control on this console; exported for the bare `<select>`s. */
export const controlClass = cn(
  "h-8 w-full min-w-0 rounded-lg border border-solid border-input bg-surface px-2.5 py-1 [font-family:inherit] text-base text-navy md:text-sm",
  "transition-[border-color,box-shadow,background-color] duration-(--dur-fast) ease-(--ease) outline-none",
  "placeholder:text-muted-foreground",
  "focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50",
  "disabled:cursor-not-allowed disabled:bg-input/50 disabled:opacity-50",
  // An invalid field is marked by the red border AND the sentence under it — never colour alone.
  "aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20",
);

/** A `<select>` in the control frame: the native arrow hidden, `SelectChevron` drawn instead, so every
 *  select looks alike across browsers (web-admin Field `kind="select"`). Wrap both in `relative`. */
export const selectClass = cn(controlClass, "cursor-pointer appearance-none pr-8");

export function SelectChevron() {
  return (
    <ChevronDown
      aria-hidden="true"
      focusable="false"
      strokeWidth={1.8}
      className="pointer-events-none absolute top-1/2 right-2.5 size-4 -translate-y-1/2 text-ink-muted"
    />
  );
}

export const labelClass = "block text-[13px] leading-tight font-semibold text-navy";

export const hintClass = "m-0 text-[12px] text-ink-muted";

export function FieldError({ id, text }: { id: string; text: string }) {
  return (
    <p id={id} className="m-0 flex items-start gap-1 text-[12px] font-medium text-danger">
      <CircleAlert aria-hidden="true" focusable="false" strokeWidth={2} className="mt-px size-3.5 shrink-0" />
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
  /** 46px instead of 32px — the sign-in card's inputs (web-admin `.login-input`). */
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
      <div className="relative">
        {Icon ? (
          <Icon
            aria-hidden="true"
            focusable="false"
            strokeWidth={1.8}
            className={cn(
              "pointer-events-none absolute top-1/2 -translate-y-1/2 text-ink-muted",
              tall ? "left-3.5 size-[18px]" : "left-2.5 size-4",
            )}
          />
        ) : null}
        <input
          id={id}
          aria-describedby={describedBy}
          aria-invalid={error ? true : undefined}
          className={cn(controlClass, tall && "h-[46px] px-3", Icon && (tall ? "pl-11" : "pl-8"))}
          {...input}
        />
      </div>
      {hint ? (
        <p id={hintId} className={hintClass}>
          {hint}
        </p>
      ) : null}
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
      <textarea
        id={id}
        aria-describedby={describedBy}
        aria-invalid={error ? true : undefined}
        className={cn(controlClass, "h-auto min-h-16 resize-y py-2 leading-normal")}
        {...area}
      />
      {hint ? (
        <p id={hintId} className={hintClass}>
          {hint}
        </p>
      ) : null}
      {error && errorId ? <FieldError id={errorId} text={error} /> : null}
    </div>
  );
}

/**
 * Always in the DOM, even empty: a live region inserted after load is missed by some screen
 * readers, and adding it later shifts the form. Empty, it takes no height (`empty:hidden` would
 * drop the region from the tree, so it is a zero-height box instead); with a sentence it gains a
 * soft red frame so it reads as a message, not stray red text (web-admin `.thong-bao-loi`).
 */
export function FormMessage({ text, trace, className }: { text: string | null; trace?: string | null; className?: string }) {
  return (
    <div
      className={cn(
        "text-sm text-danger",
        text ? "flex items-start gap-2 rounded-lg border border-solid border-danger/25 bg-danger/10 px-3 py-2" : null,
        // Spacing only while there is a sentence: an empty region takes no room in a card.
        text ? className : null,
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
          {trace ? <p className="m-0 mt-1 text-[12px] text-ink-muted">{trace}</p> : null}
        </div>
      ) : null}
    </div>
  );
}
