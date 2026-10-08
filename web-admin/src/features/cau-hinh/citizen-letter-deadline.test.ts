import { describe, expect, it } from "vitest";

import type { identity_citizenLetterDeadlineRuleOut } from "@/lib/api/schema.gen";

import {
  AMOUNT_NOT_POSITIVE,
  NOTHING_CHANGED,
  SLOTS,
  composeSave,
  deadlineRows,
} from "./citizen-letter-deadline";

const rule = (
  more: Partial<identity_citizenLetterDeadlineRuleOut>,
): identity_citizenLetterDeadlineRuleOut => ({
  id: "R1",
  letter_type: "to-cao",
  deadline_kind: "xu-ly-don",
  amount: 10,
  unit: "ngay-lam-viec",
  required_unit: "ngay-lam-viec",
  problem: null,
  ...more,
});

describe("deadlineRows", () => {
  it("six slots; a rule fills its slot; the server's required_unit wins over the slot's", () => {
    const rows = deadlineRows([rule({ required_unit: "ngay-lich" })]);
    expect(rows).toHaveLength(SLOTS.length);
    const filled = rows.find((r) => r.rule !== null)!;
    expect(filled.key).toBe("R1");
    expect(filled.unit).toBe("ngay-lich");
  });

  it("a rule matching no slot is still drawn, so it can be cleared", () => {
    const rows = deadlineRows([
      rule({
        id: "X",
        letter_type: "de-nghi",
        deadline_kind: "giai-quyet",
        required_unit: "",
      }),
    ]);
    expect(rows).toHaveLength(SLOTS.length + 1);
    expect(rows.at(-1)!.key).toBe("X");
  });
});

describe("composeSave", () => {
  const [unset] = deadlineRows([]);

  it("unset row → create with the slot's unit", () => {
    expect(composeSave(unset!, " 7 ")).toEqual({
      ok: true,
      action: "create",
      body: {
        letter_type: "kien-nghi-phan-anh",
        deadline_kind: "xu-ly-don",
        amount: 7,
        unit: "ngay-lam-viec",
      },
    });
  });

  it.each(["0", "-1", "", "1.5", "abc", "?"])("refuses %j", (text) => {
    expect(composeSave(unset!, text)).toEqual({
      ok: false,
      error: AMOUNT_NOT_POSITIVE,
    });
  });

  it("set row → update only what changed; unchanged → nothing to send", () => {
    const row = deadlineRows([rule({})]).find((r) => r.rule !== null)!;
    expect(composeSave(row, "12")).toEqual({
      ok: true,
      action: "update",
      id: "R1",
      body: { amount: 12 },
    });
    expect(composeSave(row, "10")).toEqual({
      ok: false,
      error: NOTHING_CHANGED,
    });
  });

  it("a row the lock refuses is repaired by saving: the required unit travels", () => {
    const row = deadlineRows([
      rule({ unit: "gio-lam-viec", problem: "x" }),
    ]).find((r) => r.rule !== null)!;
    expect(composeSave(row, "10")).toEqual({
      ok: true,
      action: "update",
      id: "R1",
      body: { unit: "ngay-lam-viec" },
    });
  });
});
