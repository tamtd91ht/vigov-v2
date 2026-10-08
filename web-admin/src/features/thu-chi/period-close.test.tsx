import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type {
  finance_bangDayDuRa,
  finance_budgetPeriodCloseOut,
  finance_cotRa,
  finance_danhSachDotRa,
} from "@/lib/api/schema.gen";

import { BangDayDu, SheetSelectionBar } from "./bang-thu-chi"; // vi-name-ok: existing component under test
import { FormGhiDot, HopDotThuChi, NoiDungHopDot } from "./dot-thu-chi"; // vi-name-ok: existing components under test
import { donViCuaBang, dungThanDot, nhanGoKhoanMuc, nhanNutDot, nhanThemCon } from "./nhan-thu-chi"; // vi-name-ok: existing helpers under test
import {
  buildCloseBody,
  closeConsequence,
  closedMonthsHint,
  entryLockReason,
  periodLabel,
  REOPEN_REASON_MAX,
  sheetLockReason,
  sortCloses,
  validateReopenReason,
} from "./period-close";
import {
  BudgetPeriodClosePanel,
  CloseConfirm,
  CloseForm,
  PeriodCloseList,
  ReopenForm,
} from "./period-close-panel";

/**
 * Budget period close on the Thu - Chi screen. The DENIED branch is tested first and hardest: the
 * developer's account holds `budget.confirm`, so the screen without it is the one nobody sees.
 *
 * Lock display is DISPLAY ONLY — the server answers 409 `budget_period_closed` — so the helper cases
 * pin exactly the rule the route states and no more: month close → entries dated in that
 * year-month; year close → the sheet year and entries dated in that year; reopened → nothing.
 */

function close(over: Partial<finance_budgetPeriodCloseOut> = {}): finance_budgetPeriodCloseOut {
  return {
    id: "01JCLOSE9",
    code: "CK-2026-09-01",
    year: 2026,
    month: 9,
    scope: "month",
    revision: 1,
    active: true,
    closed_at: "2026-10-01T02:00:00Z",
    closed_by: "CB-00123",
    ...over,
  };
}

const MONTH_9 = close();
const YEAR_2026 = close({ id: "01JCLOSEY", code: "CK-2026-CN-01", month: null, scope: "year" });
const REOPENED_8 = close({
  id: "01JCLOSE8",
  code: "CK-2026-08-01",
  month: 8,
  active: false,
  reopened_at: "2026-09-15T03:00:00Z",
  reopened_by: "CB-00456",
  reopen_reason: "Bổ sung chứng từ thu tháng 8",
});

const COT: finance_cotRa[] = [
  { id: "C1", name: "Dự toán năm", order: 1, type: "so", role: "du-toan-nam", numerator_column_id: null, denominator_column_id: null },
  { id: "C2", name: "Chi ngân sách", order: 2, type: "so", role: "chi-ngan-sach", numerator_column_id: null, denominator_column_id: null },
];

const SHEET: finance_bangDayDuRa = {
  sheet: {
    id: "01JBANG",
    code: "NS-2026-CHI-01",
    year: 2026,
    kind: "chi",
    revision: 1,
    title: "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC NĂM 2026",
    unit: "dong",
    unit_label: "Đồng",
  },
  columns: COT,
  lines: [
    {
      id: "I",
      no: "I",
      name: "Chi đầu tư phát triển",
      order: 1,
      method: "entries",
      level: 0,
      is_headline: true,
      values: { C1: 100, C2: 50 },
    },
  ],
  summary: {
    headline_line_id: "I",
    cells: [],
    indicator: { name: "Chi đạt dự toán", basis_points: 5000 },
  },
};
const UNIT = donViCuaBang(SHEET.sheet);
/** The one line of `SHEET` — its row buttons are named after it. */
const LEAF_NAME = "Chi đầu tư phát triển";

const ENTRIES: finance_danhSachDotRa = {
  line_id: "I",
  method: "entries",
  entries: [
    { id: "D9", line_id: "I", date: "2026-09-20", content: "Đợt tháng 9", values: { C1: null, C2: 10 } },
    {
      id: "D10",
      line_id: "I",
      date: "2026-10-02",
      content: "Điều chỉnh đợt tháng 9",
      adjustment_reason: "Sai số chứng từ PT-12",
      values: { C1: null, C2: -2 },
    },
  ],
};

/** An a11y label as it sits in the HTML string (`renderToStaticMarkup` escapes quotes and `&`). */
function inHtml(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}

/** The opening `<button …>` tag whose text is `text` — to read its `disabled` attribute. */
function buttonTag(html: string, text: string): string {
  const at = html.indexOf(`>${text}</button>`);
  expect(at).toBeGreaterThan(-1);
  return openingTagAround(html, at);
}

/**
 * The opening `<button …>` tag whose `aria-label` is `label` — for the icon-only row buttons, which
 * have no text of their own since the `＋` / `🗑` / `⇄` glyphs became lucide icons (ADR 0068 §5).
 */
function buttonTagByLabel(html: string, label: string): string {
  const at = html.indexOf(`aria-label="${inHtml(label)}"`);
  expect(at).toBeGreaterThan(-1);
  return openingTagAround(html, at);
}

function openingTagAround(html: string, at: number): string {
  const start = html.lastIndexOf("<button", at);
  return html.slice(start, html.indexOf(">", at) + 1);
}

/**
 * `disabled=""` — the ATTRIBUTE. Not the bare word: since the buttons carry Tailwind utilities, their
 * `class` contains `disabled:opacity-60`, so `not.toContain("disabled")` would be red on a live
 * button and `toContain("disabled")` green on one (the same trap `so-phan-anh.test.tsx` measured).
 */
const DISABLED = 'disabled=""';

describe("lock display helpers", () => {
  it("period label: month and whole year", () => {
    expect(periodLabel(MONTH_9)).toBe("Tháng 9/2026");
    expect(periodLabel(YEAR_2026)).toBe("Cả năm 2026");
  });

  it("month close covers entries dated in THAT year-month only, and never the sheet", () => {
    expect(entryLockReason([MONTH_9], "2026-09-01", 2026)).toContain("CK-2026-09-01");
    expect(entryLockReason([MONTH_9], "2026-09-30", 2026)).not.toBeNull();
    expect(entryLockReason([MONTH_9], "2026-10-01", 2026)).toBeNull();
    expect(entryLockReason([MONTH_9], "2025-09-10", 2026)).toBeNull();
    expect(sheetLockReason([MONTH_9], 2026)).toBeNull();
  });

  it("year close covers the sheet of that year and every entry dated in it or on that sheet", () => {
    expect(sheetLockReason([YEAR_2026], 2026)).toContain("CK-2026-CN-01");
    expect(sheetLockReason([YEAR_2026], 2025)).toBeNull();
    expect(entryLockReason([YEAR_2026], "2026-01-05", 2026)).toContain("cả năm 2026");
    // An entry dated in another year but on a sheet of the closed year: the route refuses it too.
    expect(entryLockReason([YEAR_2026], "2027-01-05", 2026)).not.toBeNull();
    expect(entryLockReason([YEAR_2026], "2027-01-05", 2027)).toBeNull();
  });

  it("a REOPENED close locks nothing", () => {
    expect(entryLockReason([REOPENED_8], "2026-08-10", 2026)).toBeNull();
    const reopenedYear = { ...YEAR_2026, active: false };
    expect(sheetLockReason([reopenedYear], 2026)).toBeNull();
    expect(entryLockReason([reopenedYear], "2026-03-10", 2026)).toBeNull();
  });

  it("a date that is not YYYY-MM-DD is not guessed — the server decides", () => {
    expect(entryLockReason([YEAR_2026], "20/9/2026", 2026)).toBeNull();
  });

  it("active closes first, history after", () => {
    expect(sortCloses([REOPENED_8, MONTH_9, YEAR_2026]).map((c) => c.code)).toEqual([
      "CK-2026-CN-01",
      "CK-2026-09-01",
      "CK-2026-08-01",
    ]);
  });

  it("closed-months hint names active month closes only", () => {
    expect(closedMonthsHint([MONTH_9, REOPENED_8], 2026)).toContain("9/2026");
    expect(closedMonthsHint([MONTH_9, REOPENED_8], 2026)).not.toContain("8/2026");
    expect(closedMonthsHint([REOPENED_8], 2026)).toBeNull();
  });
});

describe("close form: month or whole year", () => {
  it("builds {year, month} for a month, {year} for the whole year, refuses anything else", () => {
    expect(buildCloseBody(2026, "9")).toEqual({ ok: true, than: { year: 2026, month: 9 } });
    expect(buildCloseBody(2026, "12")).toEqual({ ok: true, than: { year: 2026, month: 12 } });
    expect(buildCloseBody(2026, "year")).toEqual({ ok: true, than: { year: 2026 } });
    expect(buildCloseBody(2026, "").ok).toBe(false);
    expect(buildCloseBody(2026, "13").ok).toBe(false);
    expect(buildCloseBody(2026, "0").ok).toBe(false);
  });

  it("the form offers the 12 months and the whole year", () => {
    const html = renderToStaticMarkup(
      <CloseForm year={2026} busy={false} chosen={() => {}} invalid={() => {}} />,
    );
    for (let m = 1; m <= 12; m++) expect(html).toContain(`value="${m}"`);
    expect(html).toContain('value="year"');
    expect(html).toContain("Cả năm 2026");
  });

  it("confirm step says what a MONTH close locks, and that sheets stay editable", () => {
    const html = renderToStaticMarkup(
      <CloseConfirm body={{ year: 2026, month: 9 }} busy={false} confirm={() => {}} cancel={() => {}} />,
    );
    expect(html).toContain("Xác nhận chốt tháng 9/2026");
    expect(html).toContain(closeConsequence({ year: 2026, month: 9 }));
    expect(closeConsequence({ year: 2026, month: 9 })).toContain("vẫn sửa được");
  });

  it("confirm step says what a YEAR close locks — sheets, lines, headline and values too", () => {
    const sentence = closeConsequence({ year: 2026 });
    expect(sentence).toContain("không sửa được bảng, khoản mục, dòng tổng hay số liệu");
    expect(sentence).not.toBe(closeConsequence({ year: 2026, month: 9 }));
  });
});

describe("reopen requires a reason, at most 500 characters", () => {
  it("blank refused, trimmed, 500 accepted, 501 refused — counted in characters", () => {
    expect(validateReopenReason("   ").ok).toBe(false);
    expect(validateReopenReason("  Bổ sung chứng từ  ")).toEqual({ ok: true, than: "Bổ sung chứng từ" });
    // A composed Vietnamese letter is ONE character, even as two UTF-16 units.
    expect(validateReopenReason("ệ".repeat(REOPEN_REASON_MAX)).ok).toBe(true);
    const tooLong = validateReopenReason("a".repeat(REOPEN_REASON_MAX + 1));
    expect(tooLong.ok).toBe(false);
    if (!tooLong.ok) expect(tooLong.thongBao).toContain("500");
  });

  it("the reopen form has a required textarea and a counter", () => {
    const html = renderToStaticMarkup(
      <ReopenForm close={MONTH_9} busy={false} submit={() => {}} cancel={() => {}} />,
    );
    expect(html).toContain("<textarea");
    expect(html).toContain('required=""');
    expect(html).toContain("0/500 ký tự");
    expect(html).toContain("CK-2026-09-01");
  });
});

describe("PERMISSION — the denied branch", () => {
  it("without `budget.confirm`: history visible, no close form, no reopen button", () => {
    const html = renderToStaticMarkup(
      <BudgetPeriodClosePanel
        year={2026}
        view={{ phase: "ready", closes: [MONTH_9, REOPENED_8] }}
        canConfirm={false}
        onChanged={() => {}}
      />,
    );
    expect(html).toContain("CK-2026-09-01");
    expect(html).toContain("CB-00456");
    expect(html).toContain("Bổ sung chứng từ thu tháng 8");
    expect(html).not.toContain('name="period"');
    expect(html).not.toContain("Chốt kỳ…");
    expect(html).not.toContain(inHtml("Mở chốt tháng 9/2026"));
  });

  it("with `budget.confirm`: close form, and a reopen button on ACTIVE closes only", () => {
    const html = renderToStaticMarkup(
      <PeriodCloseList closes={[MONTH_9, REOPENED_8]} canConfirm busy={false} onReopen={() => {}} />,
    );
    expect(html).toContain(inHtml("Mở chốt tháng 9/2026"));
    expect(html).not.toContain(inHtml("Mở chốt tháng 8/2026"));

    const panel = renderToStaticMarkup(
      <BudgetPeriodClosePanel
        year={2026}
        view={{ phase: "ready", closes: [] }}
        canConfirm
        onChanged={() => {}}
      />,
    );
    expect(panel).toContain('name="period"');
    expect(panel).toContain("Năm này chưa có kỳ nào được chốt.");
  });

  it("an unreadable history shows the server sentence verbatim", () => {
    const html = renderToStaticMarkup(
      <BudgetPeriodClosePanel
        year={2026}
        view={{ phase: "error", message: "Bạn không có quyền thực hiện thao tác này." }}
        canConfirm={false}
        onChanged={() => {}}
      />,
    );
    expect(html).toContain("Bạn không có quyền thực hiện thao tác này.");
  });
});

describe("entries dialog: lock state and adjustment entries", () => {
  function renderEntries(closes: readonly finance_budgetPeriodCloseOut[]): string {
    return renderToStaticMarkup(
      <NoiDungHopDot
        method="entries"
        cot={COT}
        donVi={UNIT}
        danhSach={{ pha: "xong", duLieu: ENTRIES }}
        coXacNhan
        closes={closes}
        sheetYear={2026}
        dangGui={false}
        moGo={() => {}}
      />,
    );
  }

  it("month close: the entry of that month has no remove button and says why", () => {
    const html = renderEntries([MONTH_9]);
    expect(html).not.toContain(inHtml("Gỡ đợt ngày 20/9/2026"));
    expect(html).toContain(inHtml("Gỡ đợt ngày 2/10/2026"));
    expect(html).toContain("Đợt thuộc kỳ đã chốt: tháng 9/2026 (mã CK-2026-09-01)");
  });

  it("no active close: every entry keeps its remove button", () => {
    const html = renderEntries([REOPENED_8]);
    expect(html).toContain(inHtml("Gỡ đợt ngày 20/9/2026"));
    expect(html).toContain(inHtml("Gỡ đợt ngày 2/10/2026"));
  });

  it("adjustment entry: badge and reason; an ordinary entry has neither", () => {
    const html = renderEntries([]);
    expect(html.match(/>Điều chỉnh</g)?.length).toBe(1);
    expect(html).toContain("Lý do điều chỉnh: Sai số chứng từ PT-12");
  });

  it("entry form has the optional adjustment reason field, capped at 500", () => {
    const html = renderToStaticMarkup(
      <FormGhiDot cot={COT} donVi={UNIT} dangGui={false} gui={() => {}} />,
    );
    expect(html).toContain('name="adjustment_reason"');
    expect(html).toMatch(/name="adjustment_reason"[^>]*maxLength="500"/);
    expect(html).not.toMatch(/name="adjustment_reason"[^>]*required/);
  });

  it("adjustment reason: trimmed and sent; blank omitted; over 500 refused", () => {
    const base = { ngay: "2026-10-02", noiDung: "Điều chỉnh", doiTac: "", soChungTu: "", gia: { C2: "5" } };
    const sent = dungThanDot({ ...base, adjustmentReason: "  Sai số PT-12 " }, COT, "dong");
    expect(sent.ok && sent.than.adjustment_reason).toBe("Sai số PT-12");

    const blank = dungThanDot({ ...base, adjustmentReason: "   " }, COT, "dong");
    expect(blank.ok && "adjustment_reason" in blank.than).toBe(false);

    expect(dungThanDot({ ...base, adjustmentReason: "a".repeat(501) }, COT, "dong").ok).toBe(false);
  });

  it("year close: the entry form is replaced by the reason", () => {
    const html = renderToStaticMarkup(
      <HopDotThuChi
        khoanMucId="I"
        tenKhoanMuc="Chi đầu tư phát triển"
        method="entries"
        cot={COT}
        donVi={UNIT}
        coGhi
        coXacNhan
        closes={[YEAR_2026]}
        sheetYear={2026}
        dong={() => {}}
        daDoiSoLieu={() => {}}
      />,
    );
    expect(html).not.toContain('name="adjustment_reason"');
    expect(html).toContain(sheetLockReason([YEAR_2026], 2026) as string);
  });

  it("month close: the form stays, with the closed months named", () => {
    const html = renderToStaticMarkup(
      <HopDotThuChi
        khoanMucId="I"
        tenKhoanMuc="Chi đầu tư phát triển"
        method="entries"
        cot={COT}
        donVi={UNIT}
        coGhi
        coXacNhan
        closes={[MONTH_9]}
        sheetYear={2026}
        dong={() => {}}
        daDoiSoLieu={() => {}}
      />,
    );
    expect(html).toContain('name="adjustment_reason"');
    expect(html).toContain("Các tháng đã chốt: 9/2026");
  });
});

describe("sheet under a year close", () => {
  function renderBar(sheetLock: string | null): string {
    return renderToStaticMarkup(
      <SheetSelectionBar
        year={2026}
        anchorYear={2026}
        onYearChange={() => {}}
        kind="chi"
        onKindChange={() => {}}
        sheetState="ready"
        canRecord
        canConfirm
        sheetLock={sheetLock}
        busy={false}
        onCreate={() => {}}
        onEdit={() => {}}
        onRemove={() => {}}
        onImported={() => {}}
      />,
    );
  }

  function renderSheet(sheetLock: string | null): string {
    return renderToStaticMarkup(
      <BangDayDu
        duLieu={SHEET}
        thuGon={new Set()}
        datThuGon={() => {}}
        coGhi
        coXacNhan
        sheetLock={sheetLock}
        dangGui={false}
        dangSuaDong={null}
        moSua={() => {}}
        huySua={() => {}}
        luuSua={() => {}}
        moThem={() => {}}
        moGoDong={() => {}}
        datTong={() => {}}
        moCachTinh={() => {}}
        moDot={() => {}}
      />,
    );
  }

  it("one notice, and sheet edit / delete / add disabled with the reason", () => {
    const reason = sheetLockReason([YEAR_2026], 2026) as string;
    const html = renderSheet(reason);

    expect(html.split(reason).length - 1).toBeGreaterThanOrEqual(1);
    // Presentational pins (ADR 0068 §5): the `🔒` glyph of the notice became a lucide icon, so the
    // notice is found by its `role="note"`; the glyph prefixes of the buttons became icons too.
    expect(html).toContain('role="note"');
    expect(buttonTag(html, "Thêm khoản mục cấp cao nhất")).toContain(DISABLED);
    // Sheet edit and removal live in the selection bar (prototype layout, ADR 0068 lần 5).
    const bar = renderBar(reason);
    expect(buttonTag(bar, "Sửa thông tin bảng")).toContain(DISABLED);
    expect(buttonTagByLabel(bar, "Gỡ bảng")).toContain(DISABLED);
    for (const label of [nhanThemCon(LEAF_NAME), nhanGoKhoanMuc(LEAF_NAME)]) {
      expect(buttonTagByLabel(html, label)).toContain(DISABLED);
    }
    // Reading entries stays open.
    expect(buttonTagByLabel(html, nhanNutDot(LEAF_NAME))).not.toContain(DISABLED);
  });

  it("no year close: the same controls are live", () => {
    const html = renderSheet(null);
    expect(html).not.toContain('role="note"');
    expect(buttonTag(html, "Thêm khoản mục cấp cao nhất")).not.toContain(DISABLED);
    const bar = renderBar(null);
    expect(buttonTag(bar, "Sửa thông tin bảng")).not.toContain(DISABLED);
    expect(buttonTagByLabel(bar, "Gỡ bảng")).not.toContain(DISABLED);
  });
});
