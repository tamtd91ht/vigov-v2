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
  LETTER_STATUS_GROUPS,
  buildResult,
  drawerDueAt,
  letterDate,
  letterGroupChip,
  letterGroupHint,
  letterGroupLabel,
  letterSourceLabel,
  letterStatusLabel,
  mayWorkOnLetter,
  canBookLetters,
  reportFigure,
  resultNeedsDocument,
  senderView,
  stageDueAt,
  statusStrip,
} from "./letter-display";

const NOW = new Date("2026-10-08T03:00:00Z");

describe("status strip (ADR 0084 #2) — the prototype's four steps + two branches, drawn from the server's arrows", () => {
  const strip = (over: Partial<Parameters<typeof statusStrip>[0]> = {}) =>
    statusStrip({ group: "moi-vao-so", nextStatuses: ["dang-xu-ly-don"], held: false, closed: false, mayWork: true, canRoute: true, ...over });

  it("main row and branch row are the prototype's, both always drawn", () => {
    const s = strip({ group: "da-phan-cong", held: true });
    expect(s.main.map((c) => c.label)).toEqual(["Mới vào sổ", "Đã phân công", "Đang xử lý", "Đã giải quyết"]);
    expect(s.branches.map((c) => c.label)).toEqual(["Chuyển cấp trên", "Lưu, không thụ lý"]);
    expect(s.main.find((c) => c.current)?.group).toBe("da-phan-cong");
  });

  it("no holding unit: the second chip is “Phân công · chọn bộ phận xử lý” and is an ASSIGN, not a status move", () => {
    const s = strip();
    const assign = s.main[1]!;
    expect([assign.label, assign.sub, assign.action, assign.clickable]).toEqual(["Phân công", "chọn bộ phận xử lý", "assign", true]);
    // Without the routing right (or on a finished letter) the chip is not pressable.
    expect(strip({ canRoute: false }).main[1]!.clickable).toBe(false);
    expect(strip({ closed: true, nextStatuses: [] }).main[1]!.clickable).toBe(false);
  });

  it("a chip is reachable only when some allowed next status maps to it; the sub-lines say so", () => {
    const s = strip();
    expect(s.main.map((c) => c.sub)).toEqual(["đang ở đây", "chọn bộ phận xử lý", "chuyển sang", "—"]);
    expect(s.main[2]!.steps).toEqual(["dang-xu-ly-don"]);
    expect(s.branches.every((c) => !c.clickable && c.sub === "—")).toBe(true);
  });

  it("from Đang xử lý đơn: the CURRENT group stays pressable for Thụ lý; the branches carry their steps", () => {
    const s = strip({ group: "dang-xu-ly", held: true, nextStatuses: ["thu-ly", "khong-thu-ly", "huong-dan", "chuyen-don", "luu-don"] });
    const here = s.main[2]!;
    expect([here.current, here.clickable, here.sub, here.steps]).toEqual([true, true, "đang ở đây", ["thu-ly"]]);
    expect(s.branches.map((c) => [c.group, c.clickable, c.steps])).toEqual([
      ["chuyen-cap-tren", true, ["chuyen-don"]],
      ["luu-khong-thu-ly", true, ["khong-thu-ly", "huong-dan", "luu-don"]],
    ]);
  });

  it("the step select appears only for a group of several TT 05 statuses", () => {
    const s = strip({ group: "dang-xu-ly", held: true, nextStatuses: ["da-giai-quyet", "dinh-chi"] });
    expect(s.main[3]!.pickStep).toBe(false);
    expect(s.main[2]!.pickStep).toBe(true);
    expect(s.branches[0]!.pickStep).toBe(false);
    expect(s.branches[1]!.pickStep).toBe(true);
    expect(s.branches[1]!.steps).toEqual(["dinh-chi"]);
  });

  it("a viewer who may not work on the letter: nothing clickable, whatever the server lists", () => {
    const s = strip({ group: "dang-xu-ly", held: true, nextStatuses: ["da-giai-quyet", "dinh-chi"], mayWork: false });
    expect([...s.main, ...s.branches].some((c) => c.clickable)).toBe(false);
  });

  it("a finished letter: nothing clickable; its branch shown as current", () => {
    const s = strip({ group: "luu-khong-thu-ly", held: true, closed: true, nextStatuses: [] });
    expect([...s.main, ...s.branches].some((c) => c.clickable)).toBe(false);
    expect(s.branches.map((c) => [c.group, c.current])).toEqual([
      ["chuyen-cap-tren", false],
      ["luu-khong-thu-ly", true],
    ]);
  });

  it("group labels, filter order (no “Chờ phân công”), hints — the prototype's words", () => {
    expect(LETTER_STATUS_GROUPS.map((g) => g.label)).toEqual([
      "Mới vào sổ",
      "Đã phân công",
      "Đang xử lý",
      "Đã giải quyết",
      "Chuyển cấp trên",
      "Lưu, không thụ lý",
    ]);
    expect(LETTER_STATUS_GROUPS.map((g) => g.code)).toEqual([
      "moi-vao-so",
      "da-phan-cong",
      "dang-xu-ly",
      "da-giai-quyet",
      "chuyen-cap-tren",
      "luu-khong-thu-ly",
    ]);
    expect(letterGroupHint("da-phan-cong")).toBe("Đã giao cho bộ phận, chờ bộ phận bắt tay vào việc.");
    expect(letterGroupHint("moi-vao-so")).toBe("Đã vào sổ, chưa giao cho bộ phận nào.");
    expect(letterGroupChip("moi-vao-so")).toContain("bg-ink-muted/12");
    expect(letterGroupChip("da-phan-cong")).toContain("bg-brand/12");
    expect(letterGroupChip("da-giai-quyet")).toContain("bg-leaf/12");
    expect(letterGroupLabel("x-y")).toContain("chưa có nhãn");
  });

  it("an unknown TT 05 code is said to have no label, never guessed", () => {
    expect(letterStatusLabel("x-y")).toContain("chưa có nhãn");
  });
});

describe("source (ADR 0084 #7) — the prototype's labels", () => {
  it("four sources; an unknown code says it has no label", () => {
    expect(["nhap-tay", "nhap-excel", "mini-app", "thu-dien-tu"].map(letterSourceLabel)).toEqual([
      "Nhập tay",
      "Nhập từ Excel",
      "Mini App",
      "Thư điện tử",
    ]);
    expect(letterSourceLabel("fax")).toContain("chưa có nhãn");
  });
});

describe("drawer deadline figure (prototype :513-525) — the stored instant of the current phase", () => {
  it("resolution once accepted, processing before, null when none", () => {
    const base = { accepted_at: null, processing_due_at: "2026-10-03T10:00:00Z", resolution_due_at: null };
    expect(drawerDueAt(base)).toBe("2026-10-03T10:00:00Z");
    expect(drawerDueAt({ ...base, accepted_at: "2026-10-02T02:00:00Z", resolution_due_at: "2026-10-20T10:00:00Z" })).toBe("2026-10-20T10:00:00Z");
    expect(drawerDueAt({ ...base, accepted_at: "2026-10-02T02:00:00Z" })).toBeNull();
    expect(drawerDueAt({ ...base, processing_due_at: null })).toBeNull();
  });

  // TASK-09b row 4: the list row (no `accepted_at` on the wire) and the drawer's chip row read the SAME
  // instant as the figure. Admission is read from the status — 0006's CHECK sets `accepted_at` exactly on
  // thu-ly · dang-giai-quyet · da-giai-quyet · dinh-chi.
  it("stageDueAt (list rows) agrees with drawerDueAt for every status the database allows", () => {
    const due = { processing_due_at: "2026-10-03T10:00:00Z", resolution_due_at: "2026-10-20T10:00:00Z" };
    const admitted = ["thu-ly", "dang-giai-quyet", "da-giai-quyet", "dinh-chi"];
    const notAdmitted = ["moi-vao-so", "dang-xu-ly-don", "khong-thu-ly", "huong-dan", "chuyen-don", "luu-don"];
    for (const status of admitted) {
      expect(stageDueAt({ ...due, status }), status).toBe(drawerDueAt({ ...due, accepted_at: "2026-10-02T02:00:00Z" }));
      expect(stageDueAt({ ...due, status }), status).toBe("2026-10-20T10:00:00Z");
    }
    for (const status of notAdmitted) {
      expect(stageDueAt({ ...due, status }), status).toBe(drawerDueAt({ ...due, accepted_at: null }));
      expect(stageDueAt({ ...due, status }), status).toBe("2026-10-03T10:00:00Z");
    }
    // Admitted with no resolution deadline: "Không đặt hạn", never the processing one.
    expect(stageDueAt({ ...due, resolution_due_at: null, status: "thu-ly" })).toBeNull();
  });

  it("a FINISHED letter's list cell reads its stage's date like the drawer figure, never “Không đặt hạn”", () => {
    const row = { status: "luu-don", processing_due_at: "2026-10-03T10:00:00Z", resolution_due_at: null };
    expect(dueLabel(stageDueAt(row), true, NOW)).toEqual({ text: "Hạn 3/10/2026", tone: "normal" });
  });
});

describe("reply (ADR 0084 #2) — one textarea; the issued document for KN/TC only", () => {
  const draft = { documentNo: " 12/TB ", documentDate: "2026-10-07", signer: "A", issuer: "B", summary: " Đã xong " };
  it("kiến nghị-phản ánh / đề nghị: the summary alone", () => {
    expect(resultNeedsDocument("kien-nghi-phan-anh")).toBe(false);
    expect(resultNeedsDocument("de-nghi")).toBe(false);
    expect(buildResult(draft, "de-nghi")).toEqual({ result_summary: "Đã xong" });
  });
  it("khiếu nại / tố cáo: the four document fields and the summary", () => {
    expect(resultNeedsDocument("khieu-nai")).toBe(true);
    expect(resultNeedsDocument("to-cao")).toBe(true);
    expect(buildResult(draft, "khieu-nai")).toEqual({
      result_document_no: "12/TB",
      result_document_date: "2026-10-07",
      result_signer: "A",
      result_issuer: "B",
      result_summary: "Đã xong",
    });
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
    expect(dueLabel("2026-10-05T03:00:00Z", false, NOW)).toEqual({ text: "Quá hạn · 5/10/2026", tone: "overdue" });
    expect(dueLabel("2026-10-05T03:00:00Z", true, NOW)).toEqual({ text: "Hạn 5/10/2026", tone: "normal" });
    expect(dueLabel("2026-10-12T03:00:00Z", false, NOW)).toEqual({ text: "Hạn 12/10/2026", tone: "normal" });
    for (const iso of ["2026-10-05T03:00:00Z", "2026-10-08T05:00:00Z", "2026-10-20T03:00:00Z"]) {
      expect(dueLabel(iso, false, NOW).text).not.toMatch(/Còn|\d+ (ngày|giờ)/);
    }
  });
  it("the drawer figure is the date or the prototype's “Không đặt”", () => {
    expect(dueDateText(null)).toBe(DUE_NOT_SET);
    expect(DUE_NOT_SET).toBe("Không đặt");
    expect(dueDateText("2026-10-05T03:00:00Z")).toBe("5/10/2026");
    // Read in the Vietnamese zone: 20:00Z on 31/8 is already 1/9 there.
    expect(dueDateText("2026-08-31T20:00:00Z")).toBe("1/9/2026");
  });
  it("the clerk's date ↔ the stored instant: 17:00 pinned to +07:00, read back in the Vietnamese zone", () => {
    expect(dueInstantOf("2026-10-20")).toBe("2026-10-20T17:00:00+07:00");
    expect(dueInstantOf("")).toBeNull();
    expect(dueInputValue("2026-10-20T17:00:00+07:00")).toBe("2026-10-20");
    // 20:00Z on the 19th is already the 20th in Vietnam — never sliced from the UTC string.
    expect(dueInputValue("2026-10-19T20:00:00Z")).toBe("2026-10-20");
    // The date input needs zero-padded parts even though the screen shows d/m/yyyy.
    expect(dueInputValue("2026-09-04T10:00:00Z")).toBe("2026-09-04");
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

describe("dates — d/m/yyyy without zero padding (prototype `formatDay`, vi-VN)", () => {
  it("a received date is cut as text, never padded", () => {
    expect(letterDate("2026-08-23")).toBe("23/8/2026");
    expect(letterDate("2026-01-05")).toBe("5/1/2026");
    expect(letterDate("2026-10-12")).toBe("12/10/2026");
    expect(letterDate("")).toBe("—");
    // Not the contract's shape: shown verbatim, never guessed.
    expect(letterDate("2026/8/23")).toBe("2026/8/23");
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
