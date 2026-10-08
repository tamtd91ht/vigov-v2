import { describe, expect, it } from "vitest";

import { PETITION_CREATE_PERMISSION, PETITION_READ_PERMISSION } from "@/lib/quyen";

import {
  DUE_NOT_SET,
  NO_DEADLINE,
  SUMMARY_REQUIRED,
  buildBooking,
  buildRouting,
  buildSenderCorrection,
  daysOpenView,
  dueDateText,
  dueInputValue,
  dueInstantOf,
  dueLabel,
  dueSettable,
  duplicateQuery,
  emptyEntryDraft,
  isPastDue,
  letterStatusLabel,
  mayWorkOnLetter,
  canBookLetters,
  reportFigure,
  senderView,
  statusStrip,
} from "./letter-display";

const NOW = new Date("2026-10-08T03:00:00Z");

describe("status strip (C3) — only the server's arrows are clickable", () => {
  it("from Mới vào sổ: one move (Đang xử lý đơn); no branch row", () => {
    const s = statusStrip("moi-vao-so", ["dang-xu-ly-don"], true);
    expect(s.main.map((c) => c.label)).toEqual(["Mới vào sổ", "Đang xử lý đơn", "Thụ lý", "Đang giải quyết", "Đã giải quyết"]);
    expect(s.main.filter((c) => c.clickable).map((c) => c.code)).toEqual(["dang-xu-ly-don"]);
    expect(s.main.find((c) => c.current)?.code).toBe("moi-vao-so");
    expect(s.branches).toEqual([]);
  });

  it("from Đang xử lý đơn: Thụ lý on the main row, the four processing ends on the branch row", () => {
    const next = ["thu-ly", "khong-thu-ly", "huong-dan", "chuyen-don", "luu-don"];
    const s = statusStrip("dang-xu-ly-don", next, true);
    expect(s.main.filter((c) => c.clickable).map((c) => c.code)).toEqual(["thu-ly"]);
    expect(s.branches.map((c) => c.label)).toEqual(["Không thụ lý", "Hướng dẫn", "Chuyển đơn", "Lưu đơn"]);
    expect(s.branches.every((c) => c.clickable)).toBe(true);
  });

  it("a finished letter: nothing clickable; its branch end shown as current", () => {
    const s = statusStrip("luu-don", [], true);
    expect([...s.main, ...s.branches].some((c) => c.clickable)).toBe(false);
    expect(s.branches.map((c) => [c.code, c.current])).toEqual([["luu-don", true]]);
  });

  it("a viewer who may not work on the letter: nothing clickable, whatever the server lists", () => {
    const s = statusStrip("dang-giai-quyet", ["da-giai-quyet", "dinh-chi"], false);
    expect([...s.main, ...s.branches].some((c) => c.clickable)).toBe(false);
  });

  it("an unknown code is said to have no label, never guessed", () => {
    expect(letterStatusLabel("x-y")).toContain("chưa có nhãn");
  });
});

describe("permissions (C13/C14) — convenience only", () => {
  const read = [PETITION_READ_PERMISSION];
  const both = [PETITION_READ_PERMISSION, PETITION_CREATE_PERMISSION];
  it("petition.create books; petition.read alone does not", () => {
    expect(canBookLetters(both)).toBe(true);
    expect(canBookLetters(read)).toBe(false);
  });
  it("status/result/log: read AND (assignee OR create); an empty code never matches an empty assignee", () => {
    expect(mayWorkOnLetter(both, "CB-1", "CB-9")).toBe(true);
    expect(mayWorkOnLetter(read, "CB-1", "CB-1")).toBe(true);
    expect(mayWorkOnLetter(read, "CB-1", "CB-9")).toBe(false);
    expect(mayWorkOnLetter(read, "", "")).toBe(false);
    expect(mayWorkOnLetter([PETITION_CREATE_PERMISSION], "CB-1", "CB-1")).toBe(false);
  });
});

describe("deadlines (rule 10) — read, compared, never computed", () => {
  it("no deadline reads “Không đặt hạn”", () => {
    expect(dueLabel(null, false, NOW)).toEqual({ text: NO_DEADLINE, tone: "none" });
  });
  it("a stored deadline reads as its DATE; open and past it is “Quá hạn · date” — never a count (R1)", () => {
    expect(dueLabel("2026-10-05T03:00:00Z", false, NOW)).toEqual({ text: "Quá hạn · 05/10/2026", tone: "overdue" });
    expect(dueLabel("2026-10-05T03:00:00Z", true, NOW)).toEqual({ text: "Hạn 05/10/2026", tone: "normal" });
    expect(dueLabel("2026-10-12T03:00:00Z", false, NOW)).toEqual({ text: "Hạn 12/10/2026", tone: "normal" });
    for (const iso of ["2026-10-05T03:00:00Z", "2026-10-08T05:00:00Z", "2026-10-20T03:00:00Z"]) {
      expect(dueLabel(iso, false, NOW).text).not.toMatch(/Còn|\d+ (ngày|giờ)/);
    }
  });
  it("the drawer figure is the date or the prototype's “Không đặt”", () => {
    expect(dueDateText(null)).toBe(DUE_NOT_SET);
    expect(DUE_NOT_SET).toBe("Không đặt");
    expect(dueDateText("2026-10-05T03:00:00Z")).toBe("05/10/2026");
  });
  it("the clerk's date ↔ the stored instant: 17:00 pinned to +07:00, read back in the Vietnamese zone", () => {
    expect(dueInstantOf("2026-10-20")).toBe("2026-10-20T17:00:00+07:00");
    expect(dueInstantOf("")).toBeNull();
    expect(dueInputValue("2026-10-20T17:00:00+07:00")).toBe("2026-10-20");
    // 20:00Z on the 19th is already the 20th in Vietnam — never sliced from the UTC string.
    expect(dueInputValue("2026-10-19T20:00:00Z")).toBe("2026-10-20");
    expect(dueInputValue(null)).toBe("");
  });
  it("only the four open phases have a deadline to set (domain.LetterDueColumn)", () => {
    expect(["moi-vao-so", "dang-xu-ly-don", "thu-ly", "dang-giai-quyet"].every(dueSettable)).toBe(true);
    expect(["da-giai-quyet", "dinh-chi", "luu-don", "chuyen-don"].some(dueSettable)).toBe(false);
  });
  it("isPastDue uses the PHASE's deadline and only while open", () => {
    const base = { processing_due_at: "2026-10-01T00:00:00Z", resolution_due_at: null };
    expect(isPastDue({ ...base, status: "dang-xu-ly-don", is_closed: false }, NOW)).toBe(true);
    expect(isPastDue({ ...base, status: "thu-ly", is_closed: false }, NOW)).toBe(false);
    expect(isPastDue({ ...base, status: "luu-don", is_closed: true }, NOW)).toBe(false);
  });
});

describe("masking (ADR 0078 #4)", () => {
  it("a withheld identity is withheld, whatever else arrives", () => {
    expect(senderView({ identity_withheld: true, sender_name: "x", sender_phone: "09****0000" })).toEqual({ kind: "withheld" });
  });
  it("the phone is the server's masked text, passed through", () => {
    expect(senderView({ identity_withheld: false, sender_name: "Nguyễn Văn A", sender_phone: "09****0000" })).toEqual({
      kind: "named",
      name: "Nguyễn Văn A",
      phone: "09****0000",
    });
  });
  it("only the server's flag says “Không rõ người gửi”", () => {
    expect(senderView({ identity_withheld: false, sender_name: null, sender_phone: null, sender_unknown: true }).kind).toBe("unknown");
    expect(senderView({ identity_withheld: false, sender_name: null, sender_phone: null }).kind).toBe("named");
  });
});

describe("drafts → bodies", () => {
  it("booking: “Cần nội dung đơn.” is the only client check; the sender is optional (C7)", () => {
    const d = emptyEntryDraft("2026-10-08");
    expect(buildBooking(d)).toEqual({ ok: false, error: SUMMARY_REQUIRED });
    expect(buildBooking({ ...d, summary: " Đường ngập " })).toEqual({
      ok: true,
      body: { received_date: "2026-10-08", letter_type: "kien-nghi-phan-anh", summary: "Đường ngập" },
    });
  });
  it("booking with “Không rõ người gửi”: no sender field at all", () => {
    const d = { ...emptyEntryDraft("2026-10-08"), summary: "x", senderUnknown: true, senderName: "A", senderPhone: "0900000000" };
    const built = buildBooking(d);
    expect(built.ok && built.body).not.toHaveProperty("sender_name");
    expect(built.ok && built.body).not.toHaveProperty("sender_phone");
  });
  it("the duplicate check runs only with a name and a summary typed, never for an unknown sender", () => {
    const d = { ...emptyEntryDraft("2026-10-08"), senderName: "Nguyễn Văn A", summary: "Đường thôn bị ngập" };
    expect(duplicateQuery(d)).toEqual({ sender_name: "Nguyễn Văn A", summary: "Đường thôn bị ngập", letter_type: "kien-nghi-phan-anh" });
    expect(duplicateQuery({ ...d, summary: "ngắn" })).toBeNull();
    expect(duplicateQuery({ ...d, senderUnknown: true })).toBeNull();
  });
  it("routing needs a unit and a reason", () => {
    expect(buildRouting({ toUnit: "", assignee: "", reason: "x" }).ok).toBe(false);
    expect(buildRouting({ toUnit: "U", assignee: "", reason: "x" })).toEqual({ ok: true, body: { to_unit: "U", reason: "x" } });
  });
  it("sender correction: empty fields are NOT sent; “Không rõ” clears all three", () => {
    expect(buildSenderCorrection({ name: "", phone: "", address: "", clearAll: false }).ok).toBe(false);
    expect(buildSenderCorrection({ name: "", phone: "0900000000", address: "", clearAll: false })).toEqual({
      ok: true,
      body: { sender_phone: "0900000000" },
    });
    expect(buildSenderCorrection({ name: "A", phone: "", address: "", clearAll: true })).toEqual({
      ok: true,
      body: { sender_name: null, sender_phone: null, sender_address: null },
    });
  });
});

describe("numbers", () => {
  it("days open: running bold, stopped muted with its reason", () => {
    expect(daysOpenView({ days_open: 3, is_closed: false, status: "thu-ly" })).toEqual({ text: "3 ngày", stopped: false, note: null });
    expect(daysOpenView({ days_open: 9, is_closed: true, status: "da-giai-quyet" }).note).toBe("đã giải quyết");
    expect(daysOpenView({ days_open: 2, is_closed: true, status: "luu-don" }).note).toBe("đã kết thúc");
  });
  it("a null report figure is “—”, never 0", () => {
    expect(reportFigure(null, "%")).toBe("—");
    expect(reportFigure(0)).toBe("0");
    expect(reportFigure(87.5, "%")).toBe("87,5%");
  });
});
