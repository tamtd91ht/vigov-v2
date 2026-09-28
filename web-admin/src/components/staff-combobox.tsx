"use client";

import { useEffect, useMemo, useState } from "react";

import type { identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";

import {
  COMBOBOX_CLOSED,
  comboboxKey,
  comboboxTyped,
  staffOptions,
  visibleOptions,
  type ComboboxState,
} from "./staff-combobox-logic";

/** Visible under the input, so the keyboard contract is not a secret of screen-reader users. */
export const STAFF_COMBOBOX_HINT =
  "Gõ một phần họ tên, có dấu hay không dấu đều được. Dùng phím mũi tên để chọn, Enter để xác " +
  "nhận, Esc để đóng.";

/** Said when typing matches nobody — an empty list alone reads as "the directory is empty". */
export function noMatchText(query: string): string {
  return `Không có cán bộ nào có họ tên khớp “${query.trim()}”.`;
}

/**
 * Pick ONE staff member from the staff directory by typing part of the name — the searchable
 * sibling of `OChonCanBo`. Same contract: sends the business code `CB-…`, shows
 * `Họ tên · Chức vụ`, keeps a line for a stored value missing from the directory.
 *
 * A SEPARATE COMPONENT, NOT A CHANGE TO `OChonCanBo`. Biên bản uses `OChonCanBo` directly, and
 * this pass was scoped to the Nhiệm vụ screen; another screen adopts this box by swapping the
 * element — the props mirror `OChonCanBo` one for one.
 *
 * WAI-ARIA combobox pattern: `role="combobox"` input, `aria-activedescendant` into a
 * `role="listbox"`, keys in `comboboxKey`. The list sits IN THE FLOW under the input, not
 * floating over the form: nothing to position, nothing covered at 320px.
 *
 * The listbox is always rendered (`hidden` while closed) so the options are in the static markup
 * the tests read — which is how they check that `Lãnh đạo giao việc` offers approvers only.
 */
export function StaffCombobox({
  id,
  label,
  emptyLabel,
  value,
  directory,
  disabled = false,
  onChange,
}: {
  id: string;
  label: string;
  /** Text of the "nobody" choice — its meaning differs per field, so the caller says it. */
  emptyLabel: string;
  value: string;
  directory: readonly identity_canBoChonNguoiRa[];
  /** Directory still loading. */
  disabled?: boolean;
  onChange: (code: string) => void;
}) {
  const [state, setState] = useState<ComboboxState>(COMBOBOX_CLOSED);
  const options = useMemo(() => staffOptions(directory, value), [directory, value]);
  const visible = visibleOptions(options, emptyLabel, state.query);
  const listId = `${id}-danh-sach`;
  const hintId = `${id}-goi-y`;
  const optionId = (i: number) => `${id}-lua-chon-${i}`;
  const selectedLabel = options.find((o) => o.value === value)?.label ?? "";
  const shown = state.query ?? selectedLabel;

  useEffect(() => {
    if (!state.open || state.active < 0) return;
    // Same shape as `optionId`, inlined so the effect depends on `id` only.
    document.getElementById(`${id}-lua-chon-${state.active}`)?.scrollIntoView({ block: "nearest" });
  }, [state.open, state.active, id]);

  function commit(code: string) {
    setState(COMBOBOX_CLOSED);
    if (code !== value) onChange(code);
  }

  return (
    <div className="o-nhap o-tim-can-bo">
      <label htmlFor={id}>{label}</label>
      <div className="hop-tim-can-bo">
        <input
          id={id}
          type="text"
          role="combobox"
          aria-expanded={state.open}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={
            state.open && state.active >= 0 ? optionId(state.active) : undefined
          }
          aria-describedby={hintId}
          autoComplete="off"
          value={shown}
          placeholder={value === "" ? emptyLabel : undefined}
          disabled={disabled}
          onChange={(e) => setState(comboboxTyped(e.target.value, options, emptyLabel))}
          onKeyDown={(e) => {
            const r = comboboxKey(state, e.key, visible, value);
            if (r.handled) e.preventDefault();
            setState(r.state);
            if (r.commit !== null) commit(r.commit);
          }}
          onClick={() => {
            if (!state.open) setState({ ...state, open: true });
          }}
          onBlur={() => setState(COMBOBOX_CLOSED)}
        />
        <button
          type="button"
          className="nut-phu nut-mo-danh-sach"
          // Out of the tab order (APG): the input already opens the list by keyboard.
          tabIndex={-1}
          aria-label={`Mở danh sách ${label}`}
          aria-controls={listId}
          aria-expanded={state.open}
          disabled={disabled}
          // Keep focus in the input, so its blur does not close what this click opens.
          onMouseDown={(e) => e.preventDefault()}
          onClick={(e) => {
            setState(state.open ? COMBOBOX_CLOSED : { ...state, open: true });
            (e.currentTarget.previousElementSibling as HTMLInputElement | null)?.focus();
          }}
        >
          <span aria-hidden="true">▾</span>
        </button>
      </div>
      <p id={hintId} className="goi-y-tim">
        {disabled ? emptyLabel : STAFF_COMBOBOX_HINT}
      </p>
      <ul id={listId} role="listbox" aria-label={label} className="danh-sach-goi-y" hidden={!state.open}>
        {visible.map((o, i) => (
          <li
            key={o.value === "" ? "(trong)" : o.value}
            id={optionId(i)}
            role="option"
            aria-selected={i === state.active}
            data-value={o.value}
            className={i === state.active ? "dang-tro" : undefined}
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => commit(o.value)}
          >
            {o.value === value && <span aria-hidden="true">✓ </span>}
            {o.label}
          </li>
        ))}
      </ul>
      {/* Always in the DOM: a live region added later is one not every screen reader announces. */}
      <p role="status" className="goi-y-tim">
        {state.open && state.query !== null && visible.length === 0 ? noMatchText(state.query) : ""}
      </p>
    </div>
  );
}
