"use client";

import { Check, ChevronsUpDown, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import {
  COMBOBOX_CLOSED,
  comboboxKey,
  comboboxTyped,
  staffOptions,
  visibleOptions,
  type ComboboxState,
} from "@/components/staff-combobox-logic";
import type { identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { INPUT_CLASS } from "./task-spec";

/** The prototype's placeholder of the staff search box (`PersonPicker.tsx:30`). */
export const PERSON_PICKER_PLACEHOLDER = "Gõ tên để tìm…";
/** Accessible name of the ✕ that clears a chosen person (prototype `Bỏ chọn`). */
export const PERSON_PICKER_CLEAR = "Bỏ chọn";

/** Spec 06: said in the list when typing matches nobody. */
export function personNoMatchText(query: string): string {
  return `Không có cán bộ nào khớp “${query.trim()}”.`;
}

/**
 * The Nhiệm vụ create form's staff box — spec 06 `UserCombobox` / prototype `PersonPicker`:
 *
 *   chosen, closed   a 36px button "Họ tên · chức vụ" with a ✕ (`Bỏ chọn`) that clears it
 *   open / empty     a 36px search input (`Gõ tên để tìm…`, ⇕ inside it), the list FLOATING under it:
 *                    the "nobody" line, then "Họ tên · chức vụ" rows with a ✓ on the chosen one
 *
 * The prototype's second line (the officer's e-mail) is not drawn: the staff directory route does not
 * carry e-mail (BACKEND DEPENDENCY, and a personal-data question — rule 3, stop 1).
 *
 * Matching and keys are `components/staff-combobox-logic.ts` (the same as `StaffCombobox`): every typed
 * word must appear in the full name, accents optional; Enter is ALWAYS consumed — a stray Enter must
 * not submit a form whose `Lãnh đạo giao việc` can never be set afterwards. The value sent is the
 * business code `CB-…`.
 *
 * The listbox is always in the DOM (`hidden` while closed) so its options are in the static markup the
 * tests read — which is how they check `Lãnh đạo giao việc` offers approvers only.
 */
export function TaskPersonPicker({
  id,
  label,
  emptyLabel,
  value,
  directory,
  disabled = false,
  labelClassName,
  hint,
  onChange,
}: {
  id: string;
  label: string;
  /** Text of the "nobody" line — its meaning differs per field, so the caller says it. */
  emptyLabel: string;
  value: string;
  directory: readonly identity_canBoChonNguoiRa[];
  /** Directory still loading. */
  disabled?: boolean;
  labelClassName: string;
  /** The field's hint line, under the box. */
  hint?: ReactNode;
  onChange: (code: string) => void;
}) {
  const [state, setState] = useState<ComboboxState>(COMBOBOX_CLOSED);
  const inputRef = useRef<HTMLInputElement>(null);
  const focusInput = useRef(false);
  const options = useMemo(() => staffOptions(directory, value), [directory, value]);
  const visible = visibleOptions(options, emptyLabel, state.query);
  const chosen = value === "" ? null : (options.find((o) => o.value === value) ?? null);
  const listId = `${id}-danh-sach`;
  const optionId = (i: number) => `${id}-lua-chon-${i}`;

  useEffect(() => {
    // The chosen-person button turned into the search input: put the caret in it.
    if (state.open && focusInput.current) {
      focusInput.current = false;
      inputRef.current?.focus();
    }
  }, [state.open]);

  useEffect(() => {
    if (!state.open || state.active < 0) return;
    document.getElementById(`${id}-lua-chon-${state.active}`)?.scrollIntoView({ block: "nearest" });
  }, [state.open, state.active, id]);

  function commit(code: string) {
    setState(COMBOBOX_CLOSED);
    if (code !== value) onChange(code);
  }

  return (
    <div
      onBlur={(e) => {
        // Closes when focus leaves the whole box — not when it moves between the input and the list.
        if (state.open && !e.currentTarget.contains(e.relatedTarget as Node | null)) setState(COMBOBOX_CLOSED);
      }}
    >
      <label htmlFor={id} className={labelClassName}>
        {label}
      </label>
      <div className="relative">
        {chosen !== null && !state.open ? (
          <button
            type="button"
            id={id}
            disabled={disabled}
            aria-haspopup="listbox"
            className="border-line focus-visible:ring-ring/50 flex h-9 w-full cursor-pointer items-center gap-2 rounded-md border bg-white px-2.5 text-left text-[12.5px] [font-family:inherit] outline-none focus-visible:ring-[3px] disabled:opacity-60"
            onClick={() => {
              focusInput.current = true;
              setState({ open: true, query: null, active: visible.findIndex((o) => o.value === value) });
            }}
          >
            <span className="text-navy min-w-0 flex-1 truncate">{chosen.label}</span>
            <span
              role="button"
              tabIndex={-1}
              aria-label={PERSON_PICKER_CLEAR}
              className="text-ink-muted hover:text-danger shrink-0"
              onClick={(e) => {
                e.stopPropagation();
                onChange("");
              }}
            >
              <X aria-hidden="true" focusable="false" className="size-3.5" />
            </span>
          </button>
        ) : (
          <>
            <input
              ref={inputRef}
              id={id}
              type="text"
              role="combobox"
              aria-expanded={state.open}
              aria-controls={listId}
              aria-autocomplete="list"
              aria-activedescendant={state.open && state.active >= 0 ? optionId(state.active) : undefined}
              autoComplete="off"
              disabled={disabled}
              value={state.query ?? ""}
              placeholder={chosen !== null ? chosen.label : disabled ? emptyLabel : PERSON_PICKER_PLACEHOLDER}
              className={cn(INPUT_CLASS, "pr-8 text-[12.5px] md:text-[12.5px]")}
              onFocus={() => {
                if (!state.open) setState({ ...state, open: true });
              }}
              onChange={(e) => setState(comboboxTyped(e.target.value, options, emptyLabel))}
              onKeyDown={(e) => {
                const r = comboboxKey(state, e.key, visible, value);
                if (r.handled) e.preventDefault();
                setState(r.state);
                if (r.commit !== null) commit(r.commit);
              }}
            />
            <ChevronsUpDown
              aria-hidden="true"
              focusable="false"
              className="text-ink-muted pointer-events-none absolute top-1/2 right-2.5 size-3.5 -translate-y-1/2"
            />
          </>
        )}
        <ul
          id={listId}
          role="listbox"
          aria-label={label}
          hidden={!state.open}
          className="border-line shadow-card absolute z-50 m-0 mt-1 max-h-64 w-full list-none overflow-auto rounded-md border bg-white px-0 py-1"
        >
          {visible.map((o, i) => (
            <li
              key={o.value === "" ? "(trong)" : o.value}
              id={optionId(i)}
              role="option"
              aria-selected={i === state.active}
              data-value={o.value}
              className={cn(
                "hover:bg-canvas flex cursor-pointer items-start gap-2 px-2.5 py-1.5 text-left text-[12.5px]",
                o.value !== "" && o.value === value && "bg-brand/8",
                i === state.active && "bg-canvas",
              )}
              // Keep focus in the input, so its blur does not close what this click picks.
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => commit(o.value)}
            >
              {o.value === "" ? (
                <span className="text-ink-muted">{o.label}</span>
              ) : (
                <>
                  <Check
                    aria-hidden="true"
                    focusable="false"
                    className={cn("text-brand mt-0.5 size-3.5 shrink-0", o.value === value ? "opacity-100" : "opacity-0")}
                  />
                  <span className="text-navy min-w-0 truncate">{o.label}</span>
                </>
              )}
            </li>
          ))}
          {state.query !== null && state.query.trim() !== "" && visible.length === 0 && (
            <li role="presentation" className="text-ink-muted px-2.5 py-2 text-[12px]">
              {personNoMatchText(state.query)}
            </li>
          )}
        </ul>
      </div>
      {hint}
    </div>
  );
}
