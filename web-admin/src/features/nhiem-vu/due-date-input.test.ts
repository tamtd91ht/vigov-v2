import { describe, expect, it } from "vitest";

import { autoSeparate, dueTextFromValue, parseDueText } from "./due-date-input";
import { softRefresh } from "./so-nhiem-vu";

describe("DueDateInput — dd/mm/yyyy hh:mm in every browser language (customer sheet row 16)", () => {
  it("shows the form's value in the commune's order", () => {
    expect(dueTextFromValue("2026-10-07T17:00")).toBe("07/10/2026 17:00");
    expect(dueTextFromValue("2026-10-07T17:00:30")).toBe("07/10/2026 17:00");
    expect(dueTextFromValue("")).toBe("");
  });

  it("gives back exactly the value the native box gave", () => {
    expect(parseDueText("07/10/2026 17:00")).toEqual({ value: "2026-10-07T17:00", incomplete: false });
    expect(parseDueText("7/1/2027 9:05")).toEqual({ value: "2027-01-07T09:05", incomplete: false });
  });

  it("blank is NO deadline; anything half typed or impossible is incomplete, never a guessed date", () => {
    expect(parseDueText("  ")).toEqual({ value: "", incomplete: false });
    for (const t of ["07/10/2026", "07/10/2026 17", "31/02/2026 10:00", "07/13/2026 10:00", "07/10/2026 24:00", "07/10/2026 10:60", "abc"]) {
      expect(parseDueText(t)).toEqual({ value: "", incomplete: true });
    }
  });

  it("adds the next separator only when typing at the end", () => {
    expect(autoSeparate("0", "07")).toBe("07/");
    expect(autoSeparate("07/1", "07/10")).toBe("07/10/");
    expect(autoSeparate("07/10/202", "07/10/2026")).toBe("07/10/2026 ");
    expect(autoSeparate("07/10/2026 1", "07/10/2026 17")).toBe("07/10/2026 17:");
    // Backspace and mid-box edits are left as typed: the caret never jumps.
    expect(autoSeparate("07/", "07")).toBe("07");
    expect(autoSeparate("07/10/2026 17:00", "08/10/2026 17:00")).toBe("08/10/2026 17:00");
  });
});

describe("softRefresh — a re-read after a write keeps the answer on screen (customer sheet row 17)", () => {
  const ok = { ok: true as const, duLieu: {} };
  const failed = { ok: false as const, thongBao: "Lỗi" };

  it("same filters, newer counter: draw the answer already on screen", () => {
    expect(softRefresh({ khoa: "{}||3", kq: ok }, "{}||4")).toBe("{}||3");
    expect(softRefresh({ khoa: "{}|3", cot: [{ kq: ok }] }, "{}|4")).toBe("{}|3");
  });

  it("other filters, or a failed answer, or nothing yet: the new key (loading)", () => {
    expect(softRefresh({ khoa: '{"phamVi":"mine"}||3', kq: ok }, "{}||4")).toBe("{}||4");
    expect(softRefresh({ khoa: "{}||3", kq: failed }, "{}||4")).toBe("{}||4");
    expect(softRefresh({ khoa: "{}|3", cot: [{ kq: ok }, { kq: failed }] }, "{}|4")).toBe("{}|4");
    expect(softRefresh(null, "{}||4")).toBe("{}||4");
    expect(softRefresh({ khoa: "{}||4", kq: ok }, "{}||4")).toBe("{}||4");
  });
});
