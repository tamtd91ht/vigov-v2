"use client";

import * as PopoverPrimitive from "@radix-ui/react-popover";
import { ChevronDown } from "lucide-react";

import { controlClass, Field } from "@/components/ui/field";
import type { CitizenAttributes } from "@/lib/api/citizen-contacts";
import { displayModeOf, type CitizenField, type CitizenFieldOption } from "@/lib/api/citizen-fields";
import { cn } from "@/lib/cn";

import { dateInputKind, toggleChoice, type AttributeInput, type AttributeInputs } from "./citizen-attributes";

/** The empty first choice of a drop-down, and the trigger's text while nothing is chosen. */
export const NOTHING_CHOSEN = "Chưa chọn";

/** DOM id of one dynamic control — the code is `[a-z][a-z0-9]{5,7}`, safe in an id. */
export function attributeInputId(code: string): string {
  return `nguoi-dan-tt-${code}`; // vi-name-ok: DOM id, same prefix as the dialog's other ids (`nguoi-dan-ho-ten`)
}

const LEGEND = "m-0 block p-0 text-[13px] leading-tight font-semibold text-navy";
const ERROR = "text-danger m-0 text-[12px] font-medium";

/**
 * The commune's live attributes, after the static fields, in the configured order (ADR 0101 #3) — one
 * control per type:
 *
 *   singleText  input            multiText  textarea          number    input, decimal keypad when decimal
 *   combobox    checkboxes       singleBox  radio group       dateTime  the pattern's native picker
 *
 * CHOICE GROUPS ARE A `fieldset`, NOT A `Field`: `Field` styles every input inside it as a 32px full-width
 * box, which a checkbox is not. A value the person already holds whose option was since retired stays
 * listed (ticked, "ngừng dùng") — unticking it is the officer's act, never the form's.
 *
 * `display_mode` (owner, 17/10/2026) only changes the DRAWING of a choice type: `dropdown` turns a singleBox
 * into a native `<select>` and a combobox into a popover of checkboxes; missing or `""` is `full`. The value
 * sent is the same either way.
 */
export function CitizenAttributeInputs({
  fields,
  inputs,
  stored,
  errors,
  onChange,
}: {
  fields: readonly CitizenField[];
  inputs: AttributeInputs;
  /** The contact's saved attributes (edit) — to keep a retired option on display. */
  stored: CitizenAttributes | undefined;
  errors: Readonly<Record<string, string>>;
  onChange: (code: string, value: AttributeInput) => void;
}) {
  return (
    <>
      {fields.map((f) => {
        const id = attributeInputId(f.code);
        const value = inputs[f.code];
        const error = errors[f.code];
        const wide = f.field_type === "multiText" || f.field_type === "combobox" || f.field_type === "singleBox";

        if (f.field_type === "combobox" || f.field_type === "singleBox") {
          const held = stored?.[f.code];
          const heldValues = Array.isArray(held) ? held : typeof held === "string" ? [held] : [];
          const shown = f.options.filter((o) => !o.retired || heldValues.includes(o.value));
          const chosen = f.field_type === "combobox" ? (Array.isArray(value) ? value : []) : typeof value === "string" ? [value] : [];
          if (displayModeOf(f) === "dropdown") {
            return (
              <Field key={f.code} label={f.label} htmlFor={id} required={f.required} grow="auto" error={error} className="sm:col-span-2">
                {f.field_type === "singleBox" ? (
                  <select id={id} value={chosen[0] ?? ""} aria-invalid={error !== undefined} data-attribute={f.code} onChange={(e) => onChange(f.code, e.target.value)}>
                    <option value="">{NOTHING_CHOSEN}</option>
                    {shown.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.retired ? `${o.label} (ngừng dùng)` : o.label}
                      </option>
                    ))}
                  </select>
                ) : (
                  <MultiChoiceDropdown
                    id={id}
                    field={f}
                    options={shown}
                    chosen={chosen}
                    invalid={error !== undefined}
                    onChange={(next) => onChange(f.code, next)}
                  />
                )}
              </Field>
            );
          }
          return (
            <fieldset key={f.code} className={cn("m-0 flex min-w-0 flex-col gap-1.5 border-0 p-0", wide && "sm:col-span-2")} data-attribute={f.code} aria-invalid={error !== undefined || undefined}>
              <legend className={LEGEND}>
                {f.label}
                {f.required && <span className="text-danger ml-1">*</span>}
              </legend>
              <div className="flex flex-wrap gap-x-4 gap-y-1.5" role={f.field_type === "singleBox" ? "radiogroup" : undefined}>
                {shown.map((o) => (
                  <label key={o.value} className="text-ink flex cursor-pointer items-center gap-2 text-[13px] font-normal">
                    <input
                      type={f.field_type === "combobox" ? "checkbox" : "radio"}
                      name={f.field_type === "singleBox" ? id : undefined}
                      value={o.value}
                      className="accent-brand m-0 size-4 shrink-0"
                      checked={chosen.includes(o.value)}
                      onChange={() =>
                        onChange(f.code, f.field_type === "combobox" ? toggleChoice(f, chosen, o.value) : o.value)
                      }
                    />
                    {o.label}
                    {o.retired && <span className="text-ink-muted text-[12px]">(ngừng dùng)</span>}
                  </label>
                ))}
                {f.field_type === "singleBox" && !f.required && chosen.length > 0 && (
                  <button
                    type="button"
                    className="text-brand cursor-pointer border-0 bg-transparent p-0 [font-family:inherit] text-[12.5px] underline"
                    onClick={() => onChange(f.code, "")}
                  >
                    Bỏ chọn
                  </button>
                )}
              </div>
              {error !== undefined && (
                <p className={ERROR} role="alert">
                  {error}
                </p>
              )}
            </fieldset>
          );
        }

        const text = typeof value === "string" ? value : "";
        const placeholder = f.placeholder !== "" ? f.placeholder : undefined;
        let control;
        if (f.field_type === "multiText") {
          control = <textarea id={id} rows={3} value={text} placeholder={placeholder} aria-invalid={error !== undefined} onChange={(e) => onChange(f.code, e.target.value)} />;
        } else if (f.field_type === "number") {
          control = (
            <input
              id={id}
              type="text"
              inputMode={f.number_kind === "integer" ? "numeric" : "decimal"}
              className="tabular-nums"
              value={text}
              aria-invalid={error !== undefined}
              onChange={(e) => onChange(f.code, e.target.value)}
            />
          );
        } else if (f.field_type === "dateTime") {
          const kind = dateInputKind(f.date_pattern);
          control =
            kind === "year" ? (
              <input id={id} type="number" min={1900} max={9999} step={1} inputMode="numeric" value={text} aria-invalid={error !== undefined} onChange={(e) => onChange(f.code, e.target.value)} />
            ) : (
              <input id={id} type={kind} value={text} aria-invalid={error !== undefined} onChange={(e) => onChange(f.code, e.target.value)} />
            );
        } else {
          control = <input id={id} type="text" autoComplete="off" value={text} placeholder={placeholder} aria-invalid={error !== undefined} onChange={(e) => onChange(f.code, e.target.value)} />;
        }
        return (
          <Field
            key={f.code}
            label={f.label}
            htmlFor={id}
            required={f.required}
            grow="auto"
            error={error}
            hint={f.field_type === "dateTime" && f.date_pattern !== "" ? `Định dạng ${f.date_pattern}` : undefined}
            className={cn(wide && "sm:col-span-2")}
          >
            {control}
          </Field>
        );
      })}
    </>
  );
}

/**
 * A combobox in `dropdown` mode: a trigger naming what is chosen, opening a list of checkboxes. Keyboard:
 * Enter / Space on the trigger opens it, focus moves into the list, Tab walks the boxes, Space ticks, Esc
 * closes and gives focus back to the trigger (Radix Popover).
 *
 * NO PORTAL, ON PURPOSE: the contact form is a native `<dialog>` opened with `showModal()`, which makes
 * everything outside the top layer inert — a list portalled to `<body>` would draw but take no click and
 * no focus. Rendered in place it stays inside the dialog; `modal` stays false for the same reason as
 * `pending-feature.tsx` (no injected scroll-lock `<style>`).
 */
function MultiChoiceDropdown({
  id,
  field,
  options,
  chosen,
  invalid,
  onChange,
}: {
  id: string;
  field: CitizenField;
  options: readonly CitizenFieldOption[];
  chosen: readonly string[];
  invalid: boolean;
  onChange: (next: string[]) => void;
}) {
  const labels = options.filter((o) => chosen.includes(o.value)).map((o) => o.label);
  const summary = labels.length === 0 ? NOTHING_CHOSEN : labels.join(", ");
  return (
    <PopoverPrimitive.Root>
      <PopoverPrimitive.Trigger asChild>
        <button
          id={id}
          type="button"
          // No `aria-invalid`: a button does not support it. The error is said by Field's `role="alert"` line.
          data-attribute={field.code}
          data-invalid={invalid || undefined}
          className={cn(
            controlClass,
            "flex h-8 w-full min-w-0 cursor-pointer items-center justify-between gap-2 bg-surface text-left [font-family:inherit]",
            labels.length === 0 && "text-muted-foreground",
            invalid && "border-danger",
          )}
        >
          <span className="min-w-0 truncate">{summary}</span>
          <ChevronDown aria-hidden="true" focusable="false" className="size-4 shrink-0" />
        </button>
      </PopoverPrimitive.Trigger>
      <PopoverPrimitive.Content
        align="start"
        sideOffset={4}
        collisionPadding={8}
        aria-label={field.label}
        className="border-line bg-surface z-[60] max-h-64 w-(--radix-popover-trigger-width) min-w-48 overflow-y-auto rounded-lg border p-1.5 shadow-lg"
      >
        <div className="flex flex-col" data-choice-list={field.code}>
          {options.map((o) => (
            <label key={o.value} className="text-ink hover:bg-muted flex min-h-9 cursor-pointer items-center gap-2 rounded-md px-2 text-[13px] font-normal">
              <input
                type="checkbox"
                value={o.value}
                className="accent-brand m-0 size-4 shrink-0"
                checked={chosen.includes(o.value)}
                onChange={() => onChange(toggleChoice(field, chosen, o.value))}
              />
              {o.label}
              {o.retired && <span className="text-ink-muted text-[12px]">(ngừng dùng)</span>}
            </label>
          ))}
        </div>
      </PopoverPrimitive.Content>
    </PopoverPrimitive.Root>
  );
}
