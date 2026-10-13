import { describe, expect, it } from "vitest";

import { docSoTien } from "@/features/giai-ngan/nhan-ghi-giai-ngan";
import { docSoNhap } from "@/features/thu-chi/nhan-thu-chi";

import { groupAmountDigits } from "./amount-input";

// Owner request 13/10/2026: money boxes group thousands while typing (250000000 → 250.000.000).
describe("groupAmountDigits", () => {
  it("groups plain digits by three with dots", () => {
    expect(groupAmountDigits("250000000")).toBe("250.000.000");
    expect(groupAmountDigits("999")).toBe("999");
    expect(groupAmountDigits("1000")).toBe("1.000");
    expect(groupAmountDigits("")).toBe("");
  });

  it("regroups after a digit is inserted or removed inside a grouped number", () => {
    expect(groupAmountDigits("2501.000")).toBe("2.501.000");
    expect(groupAmountDigits("250.1000")).toBe("2.501.000");
    expect(groupAmountDigits("250.00", true)).toBe("25.000");
  });

  it("keeps the sign and the decimal comma as typed", () => {
    expect(groupAmountDigits("-3463459,2")).toBe("-3.463.459,2");
    expect(groupAmountDigits("1234,")).toBe("1.234,");
    expect(groupAmountDigits("0,5")).toBe("0,5");
    expect(groupAmountDigits("-")).toBe("-");
  });

  it("drops leading zeros but keeps a single zero", () => {
    expect(groupAmountDigits("0001234")).toBe("1.234");
    expect(groupAmountDigits("0")).toBe("0");
  });

  // `docSoNhap` refuses `1.5` on purpose: grouping it to `15` would guess a factor of a thousand.
  it("leaves an ambiguous typed dot to the parser", () => {
    expect(groupAmountDigits("1.5")).toBe("1.5");
    expect(groupAmountDigits("1.")).toBe("1.");
    expect(docSoNhap(groupAmountDigits("1.5"), "dong").loai).toBe("loi");
  });

  it("never rewrites text that is not a number", () => {
    expect(groupAmountDigits("12a3")).toBe("12a3");
    expect(groupAmountDigits("1,5,2")).toBe("1,5,2");
    expect(groupAmountDigits("Không tính được")).toBe("Không tính được");
  });

  it("produces text both money parsers read as the same amount", () => {
    expect(docSoTien(groupAmountDigits("7500000000"))).toEqual({ loai: "so", dong: 7_500_000_000 });
    expect(docSoNhap(groupAmountDigits("3463459,2"), "trieu-dong")).toEqual({ loai: "so", gia: 3_463_459_200_000 });
  });
});
