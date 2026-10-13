// @vitest-environment jsdom
//
// jsdom for this file: the caret is a property of a focused DOM box, and keeping it beside the digit
// just typed is what makes live grouping usable — a caret thrown to the end breaks every mid-number edit.

import type { ChangeEvent } from "react";
import { afterEach, describe, expect, it } from "vitest";

import { formatAmountInput } from "./amount-input";

function typed(value: string, caret: number, inputType = "insertText"): HTMLInputElement {
  const box = document.createElement("input");
  box.type = "text";
  document.body.append(box);
  box.focus();
  box.value = value;
  box.setSelectionRange(caret, caret);
  const event = { target: box, nativeEvent: { inputType } } as unknown as ChangeEvent<HTMLInputElement>;
  formatAmountInput(event);
  return box;
}

afterEach(() => {
  document.body.innerHTML = "";
});

describe("formatAmountInput", () => {
  it("rewrites the box and keeps the caret at the end while typing forward", () => {
    const box = typed("2500000", 7);
    expect(box.value).toBe("2.500.000");
    expect(box.selectionStart).toBe(9);
  });

  it("keeps the caret just after a digit inserted mid-number", () => {
    // "250.000" with "1" typed after "2": caret was after the 2nd digit.
    const box = typed("2150.000", 2);
    expect(box.value).toBe("2.150.000");
    expect(box.selectionStart).toBe(3);
  });

  it("regroups after a deletion, caret beside the same digit", () => {
    // Backspace on the last digit of "250.000" (caret at the end).
    const box = typed("250.00", 6, "deleteContentBackward");
    expect(box.value).toBe("25.000");
    expect(box.selectionStart).toBe(6);
  });

  it("returns the text untouched when it is not a number", () => {
    const box = typed("12a", 3);
    expect(box.value).toBe("12a");
  });
});
