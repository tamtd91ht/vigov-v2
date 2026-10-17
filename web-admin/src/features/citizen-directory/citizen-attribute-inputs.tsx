"use client";

import { Field } from "@/components/ui/field";
import type { CitizenAttributes } from "@/lib/api/citizen-contacts";
import type { CitizenField } from "@/lib/api/citizen-fields";
import { cn } from "@/lib/cn";

import { dateInputKind, toggleChoice, type AttributeInput, type AttributeInputs } from "./citizen-attributes";

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
            hint={f.field_type === "dateTime" && f.date_pattern !== null ? `Định dạng ${f.date_pattern}` : undefined}
            className={cn(wide && "sm:col-span-2")}
          >
            {control}
          </Field>
        );
      })}
    </>
  );
}
