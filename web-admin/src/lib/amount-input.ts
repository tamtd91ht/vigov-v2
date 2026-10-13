import type { ChangeEvent } from "react";

/**
 * Live thousands grouping for money boxes: `250000000` is shown as `250.000.000` while it is typed.
 *
 * THE SEPARATOR IS THE DOT, as everywhere else this app prints money (`vi-VN`). The comma is the
 * decimal mark here: `docSoTien` refuses it and `docSoNhap` reads it as the fraction, so grouping with
 * commas would turn every typed amount into one the parsers refuse — or read a thousand times smaller.
 *
 * THIS ONLY CHANGES HOW THE DIGITS ARE SHOWN, never which digits there are. Anything that is not a plain
 * number (`1,5,2`, a letter, the "không tính được" words of a sheet cell) is returned UNCHANGED, so the
 * box's own parser still says what is wrong with it — a formatter that "cleaned" it would replace what
 * the officer typed with a figure they never wrote.
 */

/** A digit, the minus sign or the decimal comma — the characters grouping keeps; dots and spaces it redraws. */
const SIGNIFICANT = /[\d,-]/;

/**
 * Regroups the integer digits of `raw` by three with dots, keeping a leading `-` and a `,fraction`
 * exactly as typed.
 *
 * AN AMBIGUOUS DOT IS LEFT ALONE unless the edit was a deletion. `1.5` is either a decimal typed on a
 * foreign keyboard or a broken grouping; `docSoNhap` refuses it on purpose (reading it as fifteen or as
 * one-point-five is a guess off by a factor of a thousand). Regrouping it to `15` would make that guess
 * silently, so a typed or pasted dot followed by fewer than three digits keeps the raw text and the parser
 * speaks. A deletion is different: backspacing the last digit of `250.000` leaves `250.00`, and the
 * officer means `25.000`.
 */
export function groupAmountDigits(raw: string, deleting = false): string {
  const parts = /^(-?)([\d.\s]*)(,\d*)?$/.exec(raw.trim());
  if (parts === null) return raw;
  const [, sign = "", integerRaw = "", fraction = ""] = parts;

  if (!deleting && /\.(?!\d{3})/.test(integerRaw.replace(/\s/g, ""))) return raw;

  const digits = integerRaw.replace(/[.\s]/g, "");
  // Leading zeros carry no value and would print as `0.001`; one zero stays (`0`, `0,5`).
  const integer = digits.replace(/^0+(?=\d)/, "");
  return `${sign}${integer.replace(/\B(?=(\d{3})+(?!\d))/g, ".")}${fraction}`;
}

/** Caret index in `formatted` just after its `count`-th significant character. */
function caretAfter(formatted: string, count: number): number {
  if (count <= 0) return 0;
  let seen = 0;
  for (let i = 0; i < formatted.length; i++) {
    if (SIGNIFICANT.test(formatted[i] ?? "")) seen++;
    if (seen === count) return i + 1;
  }
  return formatted.length;
}

/**
 * `onChange` step for a money `<input type="text">`: rewrites the box in place with
 * `groupAmountDigits`, keeps the caret beside the same digit, and returns the new value for state.
 *
 * Works for a controlled box (store the returned value; React then finds the DOM already equal and
 * leaves the caret where it is put here) and an uncontrolled one read through `FormData`.
 */
export function formatAmountInput(e: ChangeEvent<HTMLInputElement>): string {
  const box = e.target;
  const raw = box.value;
  const inputType = (e.nativeEvent as InputEvent | undefined)?.inputType ?? "";
  const formatted = groupAmountDigits(raw, inputType.startsWith("delete"));
  if (formatted === raw) return raw;

  const caret = box.selectionStart ?? raw.length;
  let before = 0;
  for (let i = 0; i < caret && i < raw.length; i++) if (SIGNIFICANT.test(raw[i] ?? "")) before++;

  box.value = formatted;
  if (box.ownerDocument.activeElement === box) {
    const at = caretAfter(formatted, before);
    box.setSelectionRange(at, at);
  }
  return formatted;
}
