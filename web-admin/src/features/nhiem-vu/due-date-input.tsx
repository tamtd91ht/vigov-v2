"use client";

import { CalendarDays } from "lucide-react";
import { useRef, useState, type ChangeEvent } from "react";

import { cn } from "@/lib/cn";

import { INPUT_CLASS } from "./task-spec";

/**
 * The deadline box of `Giao việc mới`, written `dd/mm/yyyy hh:mm` WHATEVER THE BROWSER'S LANGUAGE
 * (customer sheet row 16).
 *
 * WHY NOT `type="datetime-local"` ALONE: the native box draws its value in the browser's UI locale, not
 * the page's `lang` — an English Chrome shows `10/07/2026, 05:00 PM`, which a Vietnamese clerk reads as
 * the 10th of July. No attribute changes that. So the VISIBLE box is text in the commune's own order,
 * and the browser's calendar is still one click away: the button opens the native picker of a hidden
 * `datetime-local` (`showPicker()`), whose answer is written back in `dd/mm/yyyy hh:mm`.
 *
 * THE VALUE IN AND OUT IS UNCHANGED: `yyyy-mm-ddThh:mm`, exactly what the native box produced, so
 * `splitNewTaskDue`, `newTaskDueInputProblem` and the instant sent are the ones they were. A box typed
 * half-way reports `incomplete` — the old `validity.badInput` — and the form says so under the field.
 */
export function DueDateInput({
  id,
  name,
  value,
  invalidProps,
  onChange,
}: {
  id: string;
  name: string;
  /** `yyyy-mm-ddThh:mm`, or `""` for no deadline. Read once, when the box mounts. */
  value: string;
  /** `aria-invalid` / `aria-describedby` of the form's error line, when one is shown. */
  invalidProps: { "aria-invalid"?: true; "aria-describedby"?: string };
  onChange: (value: string, incomplete: boolean) => void;
}) {
  const [text, setText] = useState(() => dueTextFromValue(value));
  const pickerRef = useRef<HTMLInputElement>(null);

  function put(next: string): void {
    setText(next);
    const parsed = parseDueText(next);
    onChange(parsed.value, parsed.incomplete);
  }

  function typed(e: ChangeEvent<HTMLInputElement>): void {
    put(autoSeparate(text, e.target.value.replace(/[^\d/: ]/g, "")));
  }

  function openPicker(): void {
    const picker = pickerRef.current;
    if (picker === null) return;
    // The picker starts on the date already in the box, or today when the box is empty or half typed.
    picker.value = parseDueText(text).value;
    try {
      picker.showPicker();
    } catch {
      // No `showPicker` (old browser) or refused: the text box is still there to type in.
      document.getElementById(id)?.focus();
    }
  }

  return (
    <div className="relative">
      <input
        id={id}
        name={name}
        type="text"
        inputMode="numeric"
        autoComplete="off"
        placeholder={DUE_PLACEHOLDER}
        maxLength={16}
        value={text}
        className={cn(INPUT_CLASS, "pr-10 tabular-nums")}
        {...invalidProps}
        onChange={typed}
        // `7/10/2026 9:00` becomes `07/10/2026 09:00` once the clerk leaves the box.
        onBlur={() => {
          const parsed = parseDueText(text);
          if (!parsed.incomplete && parsed.value !== "") setText(dueTextFromValue(parsed.value));
        }}
      />
      <button
        type="button"
        className="text-ink-muted hover:text-navy absolute top-1/2 right-1 grid size-7 -translate-y-1/2 cursor-pointer place-items-center rounded-md border-0 bg-transparent focus-visible:outline-2 focus-visible:outline-brand-500"
        aria-label={OPEN_CALENDAR_LABEL}
        title={OPEN_CALENDAR_LABEL}
        onClick={openPicker}
      >
        <CalendarDays aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4" />
      </button>
      {/* The browser's calendar, anchored under the box. Never focused, never read: the text box is the
          field; this only lends its picker. Rendered (not `display: none`), or `showPicker()` refuses. */}
      <input
        ref={pickerRef}
        type="datetime-local"
        tabIndex={-1}
        aria-hidden="true"
        className="pointer-events-none absolute bottom-0 left-0 h-px w-full opacity-0"
        onChange={(e) => {
          if (e.target.value !== "") put(dueTextFromValue(e.target.value));
        }}
      />
    </div>
  );
}

export const DUE_PLACEHOLDER = "dd/mm/yyyy hh:mm";
export const OPEN_CALENDAR_LABEL = "Chọn ngày trên lịch";

const VALUE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/;
const TEXT_PATTERN = /^(\d{1,2})\/(\d{1,2})\/(\d{4})\s+(\d{1,2}):(\d{2})$/;

/** `2026-10-07T17:00` → `07/10/2026 17:00`; `""` (or anything unreadable) → `""`. */
export function dueTextFromValue(value: string): string {
  const m = VALUE_PATTERN.exec(value);
  return m === null ? "" : `${m[3]}/${m[2]}/${m[1]} ${m[4]}:${m[5]}`;
}

/**
 * The typed text → the form's value. Blank is "no deadline" (`""`, not incomplete); a real day and
 * minute is `yyyy-mm-ddThh:mm`; anything else — half typed, 31/02, 25:00 — is `incomplete`, never a
 * guessed date.
 */
export function parseDueText(text: string): { readonly value: string; readonly incomplete: boolean } {
  const t = text.trim();
  if (t === "") return { value: "", incomplete: false };
  const m = TEXT_PATTERN.exec(t);
  if (m === null) return { value: "", incomplete: true };
  const day = Number(m[1]);
  const month = Number(m[2]);
  const year = Number(m[3]);
  const hour = Number(m[4]);
  const minute = Number(m[5]);
  const probe = new Date(Date.UTC(year, month - 1, day));
  const realDay =
    probe.getUTCFullYear() === year && probe.getUTCMonth() === month - 1 && probe.getUTCDate() === day;
  if (!realDay || hour > 23 || minute > 59) return { value: "", incomplete: true };
  const two = (n: number) => String(n).padStart(2, "0");
  return { value: `${year}-${two(month)}-${two(day)}T${two(hour)}:${two(minute)}`, incomplete: false };
}

/**
 * Adds the next separator when the clerk has just typed the last digit of a part AT THE END of the box
 * (`07` → `07/`, `07/10/2026` → `07/10/2026 `). Only then: an edit in the middle, a paste or a deletion
 * is left exactly as typed, so the caret never jumps and Backspace never fights back.
 */
export function autoSeparate(previous: string, next: string): string {
  const oneMoreDigit = next.length === previous.length + 1 && next.startsWith(previous) && /\d$/.test(next);
  if (!oneMoreDigit) return next;
  if (/^\d{2}$/.test(next) || /^\d{1,2}\/\d{2}$/.test(next)) return `${next}/`;
  if (/^\d{1,2}\/\d{1,2}\/\d{4}$/.test(next)) return `${next} `;
  if (/^\d{1,2}\/\d{1,2}\/\d{4} \d{2}$/.test(next)) return `${next}:`;
  return next;
}
