"use client";

import type { FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import type { CitizenField } from "@/lib/api/citizen-fields";
import { cn } from "@/lib/cn";

import { dateInputKind, isFilterable, toggleChoice, type DateInputKind, type FilterDraft, type FilterInput } from "./citizen-attributes";
import {
  FILTER_APPLY,
  FILTER_CLEAR,
  FILTER_CREATED,
  FILTER_FROM,
  FILTER_PANEL_LABEL,
  FILTER_RANGE_HINT,
  FILTER_TO,
} from "./citizen-directory-words";

export const FILTER_PANEL_ID = "bo-loc-nguoi-dan"; // vi-name-ok: DOM id, same convention as `tim-nguoi-dan`

const LEGEND = "text-navy m-0 block p-0 text-[13px] leading-tight font-semibold";
const SUB_LABEL = "text-ink-muted m-0 text-[11.5px] font-medium";
const ERROR = "text-danger m-0 text-[12px] font-medium";
const inputCls = cn(controlClass, "h-9 bg-white text-[12.5px]");

/**
 * The advanced filters of the directory (ADR 0101 #10), in a panel under the search row: "Ngày tạo"
 * from–to, then one control per LIVE filterable attribute in the configured order — a set of checkboxes
 * for "Chọn một" / "Chọn nhiều", a from–to pair for "Số" / "Ngày giờ" (one bound is enough).
 *
 * NOTHING IS SENT UNTIL "Áp dụng": a range typed half-way is not a filter. The draft lives with the list,
 * so closing the panel keeps it, and "Xoá lọc" empties it and reads the list again. Every condition goes in
 * the search BODY (`citizenContactSearchBody`) — a value of a personal attribute is never put on a URL.
 */
export function CitizenContactFilters({
  fields,
  draft,
  errors,
  onDraft,
  onApply,
  onClear,
}: {
  fields: readonly CitizenField[];
  draft: FilterDraft;
  errors: Readonly<Record<string, string>>;
  onDraft: (d: FilterDraft) => void;
  onApply: () => void;
  onClear: () => void;
}) {
  const filterable = fields.filter(isFilterable);

  function setField(code: string, input: FilterInput) {
    onDraft({ ...draft, fields: { ...draft.fields, [code]: input } });
  }

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    onApply();
  }

  return (
    <form
      id={FILTER_PANEL_ID}
      className="border-line bg-background m-0 flex min-w-0 flex-col gap-4 rounded-[10px] border border-solid p-3"
      aria-label={FILTER_PANEL_LABEL}
      method="post"
      noValidate
      onSubmit={submit}
    >
      <div className="grid min-w-0 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <RangeControl
          legend={FILTER_CREATED}
          idPrefix="loc-ngay-tao" // vi-name-ok: DOM id prefix, same convention as the panel's
          kind="date"
          from={draft.createdFrom}
          to={draft.createdTo}
          error={errors.created}
          onChange={(from, to) => onDraft({ ...draft, createdFrom: from, createdTo: to })}
        />

        {filterable.map((f) => {
          const input = draft.fields[f.code];
          if (f.field_type === "combobox" || f.field_type === "singleBox") {
            const values = input !== undefined && "values" in input ? input.values : [];
            const shown = f.options.filter((o) => !o.retired);
            return (
              <fieldset key={f.code} className="m-0 flex min-w-0 flex-col gap-1.5 border-0 p-0" data-filter={f.code}>
                <legend className={LEGEND}>{f.label}</legend>
                <div className="flex flex-wrap gap-x-4 gap-y-1.5">
                  {shown.map((o) => (
                    <label key={o.value} className="text-ink flex cursor-pointer items-center gap-2 text-[12.5px] font-normal">
                      <input
                        type="checkbox"
                        className="accent-brand m-0 size-4 shrink-0"
                        checked={values.includes(o.value)}
                        onChange={() => setField(f.code, { values: toggleChoice(f, values, o.value) })}
                      />
                      {o.label}
                    </label>
                  ))}
                </div>
              </fieldset>
            );
          }
          const range = input !== undefined && "from" in input ? input : { from: "", to: "" };
          return (
            <RangeControl
              key={f.code}
              legend={f.label}
              idPrefix={`loc-${f.code}`}
              kind={f.field_type === "number" ? "number" : dateInputKind(f.date_pattern)}
              decimal={f.number_kind !== "integer"}
              from={range.from}
              to={range.to}
              error={errors[f.code]}
              onChange={(from, to) => setField(f.code, { from, to })}
            />
          );
        })}
      </div>

      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button type="button" variant="outline" onClick={onClear}>
          {FILTER_CLEAR}
        </Button>
        <Button type="submit" variant="primary">
          {FILTER_APPLY}
        </Button>
      </div>
    </form>
  );
}

/** A from–to pair: number boxes, or the pattern's native picker on both ends. */
function RangeControl({
  legend,
  idPrefix,
  kind,
  decimal = true,
  from,
  to,
  error,
  onChange,
}: {
  legend: string;
  idPrefix: string;
  kind: DateInputKind | "number";
  decimal?: boolean;
  from: string;
  to: string;
  error?: string;
  onChange: (from: string, to: string) => void;
}) {
  const box = (side: "from" | "to", value: string) => {
    const id = `${idPrefix}-${side === "from" ? "tu" : "den"}`;
    const set = (v: string) => (side === "from" ? onChange(v, to) : onChange(from, v));
    const common = { id, value, className: inputCls, "aria-invalid": error !== undefined, onChange: (e: { target: { value: string } }) => set(e.target.value) };
    return (
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <label htmlFor={id} className={SUB_LABEL}>
          {side === "from" ? FILTER_FROM : FILTER_TO}
        </label>
        {kind === "number" ? (
          <input type="text" inputMode={decimal ? "decimal" : "numeric"} {...common} />
        ) : kind === "year" ? (
          <input type="number" min={1900} max={9999} step={1} inputMode="numeric" {...common} />
        ) : (
          <input type={kind} {...common} />
        )}
      </div>
    );
  };
  return (
    <fieldset className="m-0 flex min-w-0 flex-col gap-1.5 border-0 p-0" data-filter={idPrefix}>
      <legend className={LEGEND}>{legend}</legend>
      <div className="flex min-w-0 flex-wrap gap-2">
        {box("from", from)}
        {box("to", to)}
      </div>
      {error !== undefined ? (
        <p className={ERROR} role="alert">
          {error}
        </p>
      ) : (
        <p className="text-ink-muted m-0 text-[11.5px]">{FILTER_RANGE_HINT}</p>
      )}
    </fieldset>
  );
}
