import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import { EXPORT_WAITING } from "@/features/dashboard/view";
import type { BlocksData, Loaded } from "@/features/dashboard/view";
import { NHAN_CHENH_LECH } from "@/features/thu-chi/nhan-thu-chi";
import type {
  documents_incomingSummaryOut,
  finance_chiSoNamRa,
  petitions_citizenReportSummaryOut,
  petitions_taskSummaryOut,
} from "@/lib/api/schema.gen";

import { COMPARISON_EMPTY } from "./comparison-chart";
import { reportAccess } from "./report-access";
import { customWindows, reportWindows } from "./report-period";
import type { ReportWindows } from "./report-period";
import { CUSTOM_PERIOD_HINT, ReportHeader, ReportView, UNIT_LOAD_ERROR } from "./report-view";
import { UNIT_TABLE_EMPTY, UNIT_TABLE_NOTE, UNIT_TABLE_TITLE, UNKNOWN_UNIT_LABEL } from "./unit-table";
import type { UnitRow } from "./unit-table";

/**
 * The DENIED branches first — the ones a developer holding every key never sees — then the page's
 * shape. Rendered to a string in Node, like `features/dashboard/view.test.tsx`.
 */

const AT = Date.parse("2026-09-28T09:43:00Z"); // 16:43 in Viet Nam
const MONTH = reportWindows({ kind: "month" }, new Date(AT));

const ALL = ["report.read", "report.export", "task.read", "document.read", "feedback.read", "budget.read"];
const COMMUNE = { displayName: "Xã Kiểm Thử", parentAuthority: "" };

const ok = <T,>(duLieu: T) => ({ ok: true as const, duLieu });

const TASKS: petitions_taskSummaryOut = {
  in_progress: 24,
  overdue: 2,
  suspended: 1,
  completed: 9,
  on_time_sample: 8,
  on_time: 6,
};
const DOCS: documents_incomingSummaryOut = {
  from: "2026-09-01T00:00:00+07:00",
  to: "2026-09-28T16:43:01+07:00",
  as_of: "2026-09-28T09:43:00Z",
  arrived: 31,
  open: 7,
  overdue: 3,
};
const REPORTS: petitions_citizenReportSummaryOut = {
  received: 17,
  in_progress: 5,
  on_time_sample: 10,
  on_time: 9,
  late: 1,
  rating_sample: 4,
  rating_sum: 18,
};
const FISCAL: finance_chiSoNamRa = {
  year: 2026,
  revenue_achievement: { name: "Thu đạt dự toán", basis_points: 7350 },
  expenditure_achievement: { name: "Chi đạt dự toán", basis_points: 9130 },
  balance: { amount: 3_400_000_000 },
  revenue_totals: [{ column_id: "c1", name: "Thu ngân sách xã hưởng", value: 690_000_000 }],
};

/** Nothing answered yet — every block loading. */
function loading(windows: ReportWindows): BlocksData {
  const none = { current: null, previous: null };
  return {
    windows,
    tasks: none,
    incomingDocuments: none,
    citizenReports: none,
    fiscal: null,
    fiscalYear: 2026,
    queue: null,
    taskTypeLabels: null,
  };
}

/** Every block answered; the previous period differs on purpose. */
function loaded(windows: ReportWindows = MONTH): BlocksData {
  return {
    ...loading(windows),
    tasks: { current: ok(TASKS), previous: ok({ ...TASKS, completed: 8, overdue: 9 }) },
    incomingDocuments: { current: ok(DOCS), previous: ok({ ...DOCS, arrived: 0 }) },
    citizenReports: { current: ok(REPORTS), previous: ok({ ...REPORTS, received: 20, late: 2 }) },
    fiscal: ok(FISCAL),
  };
}

const ROWS: UnitRow[] = [
  { key: "01JBP1", name: "VĂN PHÒNG ĐẢNG ỦY", kind: "unit", total: 21, completed: 13, onTime: 4, onTimeSample: 12, overdue: 3 },
  { key: "01JBP2", name: "VĂN PHÒNG HĐND – UBND", kind: "unit", total: 7, completed: 5, onTime: 5, onTimeSample: 5, overdue: 0 },
  { key: "01JGONE", name: UNKNOWN_UNIT_LABEL, kind: "unknown", total: 2, completed: 0, onTime: 0, onTimeSample: 0, overdue: 0 },
];

function render(
  permissions: readonly string[],
  units: Loaded<UnitRow[]> = { ok: true, duLieu: ROWS },
  windows: ReportWindows = MONTH,
  figures: BlocksData = loading(windows),
): string {
  return renderToStaticMarkup(
    <ReportView
      windows={windows}
      fetchedAt={AT}
      figures={figures}
      access={reportAccess(permissions)}
      units={units}
      commune={COMMUNE}
      onNamedPeriod={() => {}}
      onCustomPeriod={() => {}}
      onReload={() => {}}
      onReloadUnits={() => {}}
    />,
  );
}

/** The `<section>` of one card, by its accessible name. */
function section(html: string, name: string): string {
  return new RegExp(`<section[^>]*aria-label="${name}"[^>]*>.*?</section>`, "s").exec(html)?.[0] ?? "";
}

describe("reportAccess — permission gating (rule 5: UI convenience, the server checks)", () => {
  it("unit table needs task.read AND report.read — either one missing hides it", () => {
    expect(reportAccess(["report.read", "task.read"]).unitTable).toBe(true);
    expect(reportAccess(["report.read"]).unitTable).toBe(false);
    expect(reportAccess(["task.read"]).unitTable).toBe(false);
    expect(reportAccess(["report.read", "task.reads", "task.approve"]).unitTable).toBe(false);
  });

  it("Thu – Chi needs budget.read on top of report.read (ADR 0053 B6)", () => {
    expect(reportAccess(["report.read", "budget.read"]).blocks.budget).toBe(true);
    expect(reportAccess(["report.read"]).blocks.budget).toBe(false);
    expect(reportAccess(["budget.read"]).blocks.budget).toBe(false);
  });

  it("export needs report.export (spec 13 §9.4), and report.read with it", () => {
    expect(reportAccess(["report.read", "report.export"]).exportReport).toBe(true);
    expect(reportAccess(["report.read"]).exportReport).toBe(false);
    expect(reportAccess(["report.export"]).exportReport).toBe(false);
  });

  it("blocks follow /tong-quan's rule exactly", () => {
    const a = reportAccess(ALL).blocks;
    expect(a).toEqual({ tasks: true, incomingDocuments: true, citizenReports: true, budget: true });
  });
});

describe("export row (D3) — built, the prototype's outline buttons", () => {
  it("DENIED: no export row at all without report.export", () => {
    const html = render(ALL.filter((p) => p !== "report.export"), undefined, MONTH, loaded());
    expect(html).not.toContain('aria-label="Xuất báo cáo"');
    expect(html).not.toContain("Xuất PDF");
  });

  it("ALLOWED and loaded: three enabled 'Xuất …' buttons, outline 36px, Download icon, NO '?'", () => {
    const html = render(ALL, undefined, MONTH, loaded());
    expect(html).toContain('aria-label="Xuất báo cáo"');
    for (const f of ["pdf", "xlsx", "pptx"]) {
      const b = new RegExp(`<button[^>]*data-export-format="${f}"[^>]*>(.*?)</button>`, "s").exec(html);
      expect(b?.[0]).toBeDefined();
      expect(b?.[0]).not.toContain('disabled=""');
      expect(b?.[0]).toContain("h-9");
      expect(b?.[1]).toContain(`Xuất ${f.toUpperCase()}`);
      expect(b?.[1]).toContain("<svg");
    }
    expect(html).not.toContain(pendingMarkerLabel("Xuất báo cáo PDF, XLSX, PPTX"));
    expect(html).not.toContain("Trình chiếu");
  });

  it("LOADING: disabled while a visible block has not answered, the reason on the group", () => {
    const html = render(ALL, undefined, MONTH, loading(MONTH));
    for (const f of ["pdf", "xlsx", "pptx"]) {
      expect(html).toMatch(new RegExp(`<button[^>]*disabled=""[^>]*data-export-format="${f}"`));
    }
    expect(html).toContain(`title="${EXPORT_WAITING}"`);
  });

  it("LOADING: the unit table still loading also holds the export", () => {
    const html = render(ALL, null, MONTH, loaded());
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*data-export-format="pdf"/);
  });
});

describe("ReportView — unit table (D1, the prototype's RankingTable)", () => {
  it("DENIED: no table without task.read", () => {
    const html = render(["report.read", "budget.read"]);
    expect(html).not.toContain(UNIT_TABLE_TITLE);
  });

  it("DENIED: no Thu – Chi block without budget.read", () => {
    const html = render(["report.read", "task.read"]);
    expect(html).not.toContain('aria-label="Thu - Chi ngân sách xã"');
    expect(render(ALL)).toContain('aria-label="Thu - Chi ngân sách xã"');
  });

  it("title 'Xếp hạng bộ phận' drawn ONCE (no sr-only caption); the scroller keeps its label", () => {
    const html = render(ALL);
    expect(html.split(`>${UNIT_TABLE_TITLE}<`)).toHaveLength(2);
    expect(html).not.toContain("<caption");
    expect(html).not.toContain("Tình hình thực hiện theo bộ phận");
    expect(html).toMatch(/role="region" tabindex="0"[^>]*aria-label="Xếp hạng bộ phận"/);
  });

  it("the prototype's frame and header: rounded-[10px] border, 12.5px, header bg-canvas muted", () => {
    const card = section(render(ALL), UNIT_TABLE_TITLE);
    expect(card).toMatch(/<h2 class="[^"]*text-\[14px\][^"]*font-bold[^"]*text-navy[^"]*">Xếp hạng bộ phận<\/h2>/);
    expect(card).toContain('class="overflow-hidden rounded-[10px] border border-line"');
    expect(card).toContain("text-[12.5px]");
    expect(card).toMatch(/<tr class="bg-canvas text-ink-muted">/);
  });

  it("columns in the prototype's order: Bộ phận · Tổng việc · Quá hạn (hiện tại) · Hoàn thành · Đúng hạn", () => {
    const card = section(render(ALL), UNIT_TABLE_TITLE);
    const heads = [...card.matchAll(/<th scope="col"[^>]*>([^<]*)<\/th>/g)].map((m) => m[1]);
    expect(heads).toEqual(["Bộ phận", "Tổng việc", "Quá hạn (hiện tại)", "Hoàn thành", "Đúng hạn"]);
  });

  it("bars: brand for Tổng việc, danger for Quá hạn, each scaled to its column's largest value", () => {
    const card = section(render(ALL), UNIT_TABLE_TITLE);
    const brand = [...card.matchAll(/data-bar="brand".*?bg-brand\/70" style="width:(\d+)%"/gs)].map((m) => m[1]);
    const danger = [...card.matchAll(/data-bar="danger".*?bg-danger\/70" style="width:(\d+)%"/gs)].map((m) => m[1]);
    // totals 21 · 7 · 2 against 21; overdue 3 · 0 · 0 against 3
    expect(brand).toEqual(["100", "33", "10"]);
    expect(danger).toEqual(["100", "0", "0"]);
    // the track: 16px high, up to 8rem, page colour, 4px radius; the fill at 70%
    expect(card).toContain('class="relative h-4 w-full max-w-[8rem] overflow-hidden rounded-[4px] bg-canvas"');
  });

  it("Quá hạn number red and semibold only when above 0", () => {
    const card = section(render(ALL), UNIT_TABLE_TITLE);
    expect(card).toMatch(/<span class="w-8 shrink-0 text-right tabular-nums font-semibold text-danger">3<\/span>/);
    expect(card).toMatch(/<span class="w-8 shrink-0 text-right tabular-nums">0<\/span>/);
  });

  it("Đúng hạn: rounded % by threshold (≥80 leaf, ≥50 tangerine, else danger); an empty sample '—' muted", () => {
    const card = section(render(ALL), UNIT_TABLE_TITLE);
    expect(card).toMatch(/font-semibold text-danger">33%<\/td>/); // 4/12
    expect(card).toMatch(/font-semibold text-leaf">100%<\/td>/); // 5/5
    expect(card).toMatch(/font-normal text-ink-muted">—<\/td>/); // 0/0
    expect(card).not.toContain("33,3%");
  });

  it("rows keep OUR set and order (B3/B5d): every unit, then the retired unit, by Tổng việc", () => {
    const card = section(render(ALL), UNIT_TABLE_TITLE);
    const names = [...card.matchAll(/<th scope="row"[^>]*>([^<]*)<\/th>/g)].map((m) => m[1]);
    expect(names).toEqual(["VĂN PHÒNG ĐẢNG ỦY", "VĂN PHÒNG HĐND – UBND", UNKNOWN_UNIT_LABEL]);
  });

  it("the footnote under a hairline, 11px muted (spec 05 B)", () => {
    const card = section(render(ALL), UNIT_TABLE_TITLE);
    expect(card).toContain(
      `<p class="m-0 mt-3 border-t border-line pt-2 text-[11px] leading-snug text-ink-muted">${UNIT_TABLE_NOTE}</p>`,
    );
  });

  it("loading: a status sentence, no figures", () => {
    const html = render(ALL, null);
    expect(html).toContain('aria-busy="true"');
    expect(html).toContain("Đang tải…");
  });

  it("failed: the server's sentence verbatim with Tải lại", () => {
    const html = render(ALL, { ok: false, thongBao: "Kỳ báo cáo không hợp lệ." });
    expect(html).toContain(UNIT_LOAD_ERROR);
    expect(html).toContain("Kỳ báo cáo không hợp lệ.");
    expect(html).toContain("Tải lại");
  });

  it("empty: the prototype's sentence, no table", () => {
    const card = section(render(ALL, { ok: true, duLieu: [] }), UNIT_TABLE_TITLE);
    expect(card).not.toContain("<table");
    expect(card).toContain(`<p role="status" class="m-0 py-8 text-center text-[13px] text-ink-muted">${UNIT_TABLE_EMPTY}</p>`);
    expect(UNIT_TABLE_EMPTY).toBe("Chưa có bộ phận nào được giao việc trong kỳ.");
  });
});

describe("ReportView — comparison chart (D2)", () => {
  const card = (figures: BlocksData, permissions = ALL) =>
    section(render(permissions, undefined, MONTH, figures), "So sánh với kỳ trước");

  it("an SVG chart, navy 14px title, no '?' — one bar per PERIOD figure with a non-zero previous", () => {
    const html = card(loaded());
    expect(html).toMatch(/<h2 class="[^"]*text-\[14px\][^"]*text-navy[^"]*">So sánh với kỳ trước<\/h2>/);
    expect(html).toContain('<svg role="img"');
    expect(html).not.toContain("data-pending-marker");
    const bars = [...html.matchAll(/<g data-tone="(\w+)"><title>([^<]*)<\/title>/g)].map((m) => [m[2], m[1]]);
    // Đến trong kỳ is absent: its previous was 0. Stock figures and fiscal never appear. An unchanged
    // figure stays, as a zero-length bar (the prototype keeps every metric with a previous value).
    expect(bars).toEqual([
      ["Hoàn thành trong kỳ: +12,5%", "good"],
      ["Đúng hạn trong kỳ (nhiệm vụ): 0,0%", "neutral"],
      ["Tiếp nhận trong kỳ: -15,0%", "neutral"],
      ["Đúng hạn trong kỳ (phản ánh): 0,0%", "neutral"],
      ["Trễ hạn trong kỳ: -50,0%", "good"],
    ]);
    expect(html).not.toContain("Đang thực hiện:");
    expect(html).not.toContain("Quá hạn:");
    expect(html).not.toContain("Thu đạt dự toán");
    expect(html).not.toContain("Điểm hài lòng");
  });

  it("two blocks' 'Đúng hạn trong kỳ' are told apart; the bar colour follows direction, not sign", () => {
    const html = card({
      ...loaded(),
      tasks: { current: ok(TASKS), previous: ok({ ...TASKS, on_time: 7 }) }, // 75% vs 87,5%
      citizenReports: { current: ok(REPORTS), previous: ok({ ...REPORTS, on_time: 6 }) }, // 90% vs 60%
    });
    expect(html).toContain("<title>Đúng hạn trong kỳ (nhiệm vụ): -14,3%</title>");
    expect(html).toContain("<title>Đúng hạn trong kỳ (phản ánh): +50,0%</title>");
    expect(html).toMatch(/data-tone="bad"><title>Đúng hạn trong kỳ \(nhiệm vụ\)/);
    expect(html).toMatch(/data-tone="good"><title>Đúng hạn trong kỳ \(phản ánh\)/);
  });

  it("a text table for screen readers: each figure, both periods, the change and its direction in words", () => {
    const html = card(loaded());
    expect(html).toMatch(/<table class="an-thi-giac">/);
    expect(html).toContain("<th scope=\"row\">Hoàn thành trong kỳ</th><td>9</td><td>8</td><td>+12,5%</td><td>tốt lên</td>");
  });

  it("height is max(220, 30 · n)", () => {
    expect(card(loaded())).toMatch(/<svg[^>]*height="220"/);
  });

  it("an unchanged figure draws no bar shape (zero length), only its label", () => {
    const html = card(loaded());
    const flat = /<g data-tone="neutral"><title>Đúng hạn trong kỳ \(nhiệm vụ\)[^<]*<\/title>.*?<\/g>/s.exec(html)?.[0] ?? "";
    expect(flat).toContain("<text");
    expect(flat).not.toContain("<path");
  });

  it("empty: no period figure with a previous value → the prototype's sentence, no SVG", () => {
    const zero = { ...TASKS, completed: 0, on_time: 0, on_time_sample: 0 };
    const html = card({
      ...loaded(),
      tasks: { current: ok(TASKS), previous: ok(zero) },
      incomingDocuments: { current: ok(DOCS), previous: ok({ ...DOCS, arrived: 0 }) },
      citizenReports: { current: ok(REPORTS), previous: ok({ ...REPORTS, received: 0, late: 0, on_time_sample: 0 }) },
    });
    expect(html).toContain(COMPARISON_EMPTY);
    expect(html).not.toContain("<svg");
  });

  it("loading: a quiet line, never the empty sentence", () => {
    const html = card(loading(MONTH));
    expect(html).toContain("Đang tải…");
    expect(html).not.toContain(COMPARISON_EMPTY);
  });

  it("DENIED: no compared block visible (budget only) → no comparison card at all", () => {
    expect(render(["report.read", "budget.read"])).not.toContain('aria-label="So sánh với kỳ trước"');
  });

  it("a failed previous call leaves that block's figures out — never a bar from a missing half", () => {
    const html = card({
      ...loaded(),
      tasks: { current: ok(TASKS), previous: { ok: false, thongBao: "Mạng hỏng." } },
    });
    expect(html).not.toContain("Hoàn thành trong kỳ:");
  });
});

describe("ReportView — the report tile (D4)", () => {
  const html = render(ALL, undefined, MONTH, loaded());
  const grid = /<div data-dashboard-grid=""[^>]*>.*?(?=<section[^>]*aria-label="Xếp hạng bộ phận")/s.exec(html)?.[0] ?? html;

  it("tiles are NOT links and never red; the value is the prototype's clamp(19px,1.7vw,26px) navy", () => {
    expect(grid).not.toContain("<a ");
    expect(grid).not.toContain("text-danger-600");
    expect(grid).toMatch(/text-\[clamp\(19px,1\.7vw,26px\)\][^"]*text-navy/);
    expect(grid).not.toContain("12cqi");
    expect(grid).toMatch(/<li class="min-w-0 overflow-hidden px-1 py-0\.5"/);
  });

  it("delta line: arrow + '+12,5% so với kỳ trước' in leaf, 11px", () => {
    expect(grid).toMatch(/<span class="mt-1 flex items-center gap-1 text-\[11px\] text-leaf"><svg[^>]*>.*?<\/svg><span class="min-w-0">\+12,5% so với kỳ trước<\/span>/s);
  });

  it("a volume (neutral) moves in muted text; a worse figure in danger", () => {
    // Tiếp nhận 17 vs 20: neutral, muted. Trễ hạn 1 vs 2: better (lower is better), leaf.
    expect(grid).toMatch(/text-ink-muted"><svg[^>]*>.*?<\/svg><span class="min-w-0">-15,0% so với kỳ trước/s);
    expect(grid).toMatch(/text-leaf"><svg[^>]*>.*?<\/svg><span class="min-w-0">-50,0% so với kỳ trước/s);
  });

  it("'không đổi' under 0,05%, with the Minus icon", () => {
    const same = render(ALL, undefined, MONTH, { ...loaded(), tasks: { current: ok(TASKS), previous: ok(TASKS) } });
    expect(same).toMatch(/text-ink-muted"><svg[^>]*lucide-minus[^>]*>.*?<\/svg><span class="min-w-0">không đổi<\/span>/s);
  });

  it("no percentage (stock, previous 0) → 'chưa có kỳ trước để so' muted /60; 'Kỳ trước: 0' kept as a sub-line", () => {
    expect(grid).toContain('<span class="mt-1 block text-[11px] text-ink-muted/60">chưa có kỳ trước để so</span>');
    // Đến trong kỳ: previous 0 → the quiet line, then "Kỳ trước: 0" as a sub-line
    expect(grid).toMatch(/>Đến trong kỳ<\/span><span class="mt-1 block text-\[11px\] text-ink-muted\/60">chưa có kỳ trước để so<\/span><span title="Kỳ trước: 0" class="mt-0\.5 line-clamp-2 text-\[10\.5px\] leading-snug text-ink-muted\/70">Kỳ trước: 0<\/span>/);
  });

  it("sub-lines come AFTER the delta line, small and clamped to two lines", () => {
    expect(grid).toMatch(/>không đổi<\/span><\/span><span title="6\/8 việc có hạn" class="mt-0\.5 line-clamp-2[^"]*">6\/8 việc có hạn<\/span>/);
  });

  it("label 'Tiếp nhận trong kỳ' on the report (Tổng quan keeps 'Nhận vào trong kỳ')", () => {
    expect(grid).toContain(">Tiếp nhận trong kỳ<");
    expect(grid).not.toContain("Nhận vào trong kỳ");
  });

  it("unbuilt tile: '—' navy bold and 'Tính năng đang phát triển' visible; no-source tile: '—' + 'Chưa có dữ liệu', no icon", () => {
    const letters = /<li[^>]*data-pending=""[^>]*>(?:(?!<\/li>).)*Đơn thư trong kỳ(?:(?!<\/li>).)*<\/li>/s.exec(grid)?.[0] ?? "";
    expect(letters).toMatch(/<span aria-hidden="true" class="[^"]*font-bold[^"]*text-navy[^"]*">—<\/span>/);
    expect(letters).toContain('<span class="mt-1 block text-[11px] text-ink-muted/60">Tính năng đang phát triển</span>');
    expect(letters).toContain(`aria-label="${pendingMarkerLabel("Đơn thư trong kỳ")}"`);
    const noSource = /<li[^>]*title="Hệ thống chưa có nguồn số liệu cho mục này\."[^>]*>.*?<\/li>/s.exec(grid)?.[0] ?? "";
    expect(noSource).toContain(">Chưa có dữ liệu</span>");
    expect(noSource).not.toContain("<svg");
  });

  it("the '?' beside a block title is the small muted ring, right after the title (gap-1)", () => {
    const budget = /<section[^>]*data-block="budget".*?<\/section>/s.exec(grid)?.[0] ?? "";
    expect(budget).toMatch(/<div class="flex shrink-0 items-center gap-1 mb-3">/);
    expect(budget).toMatch(/data-pending-marker=""[^>]*class="[^"]*size-3\.5[^"]*text-ink-muted\/70/);
  });
});

describe("ReportView — fiscal block on the report (D6)", () => {
  const html = render(ALL, undefined, MONTH, loaded());
  const block = /<section[^>]*data-block="fiscal".*?<\/section>/s.exec(html)?.[0] ?? "";
  const labels = [...block.matchAll(/<span class="mt-0\.5 block text-\[12px\] text-ink-muted">([^<]*)<\/span>/g)].map((m) => m[1]);

  it("title 'Thu - Chi ngân sách xã'", () => {
    expect(block).toContain(">Thu - Chi ngân sách xã</h2>");
  });

  it("the prototype's order: Thu đạt dự toán · Tổng thu · Chi đạt dự toán · Tổng chi ('?') · Cân đối", () => {
    expect(labels).toEqual(["Thu đạt dự toán", "Thu ngân sách xã hưởng", "Chi đạt dự toán", "Tổng chi", NHAN_CHENH_LECH]);
    expect(block).toContain(`aria-label="${pendingMarkerLabel("Tổng chi")}"`);
  });

  it("money as the prototype writes it: '3,4 tỷ', '690 triệu', no 'đồng'; the exact sum on hover", () => {
    expect(block).toContain(">3,4 tỷ<");
    expect(block).toContain(">690 triệu<");
    expect(block).not.toContain("tỷ đồng<");
    expect(block).toContain('title="3.400.000.000 đồng"');
  });

  it("footnotes under a hairline, 11px muted", () => {
    expect(block).toContain('<div class="mt-auto border-t border-line pt-2"><p class="m-0 text-[11px] leading-snug text-ink-muted">Luỹ kế năm 2026, không so với kỳ trước.</p>');
  });
});

describe("ReportView — page shape (spec 13 §1–§2, ADR 0053 B4/B5a, D5)", () => {
  it("no 'Cần xử lý ngay', no 'Tính lại ngay', no 'Trình chiếu'", () => {
    const html = render(ALL);
    expect(html).not.toContain("Cần xử lý ngay");
    expect(html).not.toContain("Tính lại ngay");
    expect(html).not.toContain("Trình chiếu");
  });

  it("the meta line: 'Số liệu tính đến' + /tong-quan's comparison for a named period, at most 46rem", () => {
    const html = render(ALL);
    expect(html).toContain("Số liệu tính đến 16:43 28/09/2026.");
    expect(html).toContain("So với cùng khoảng thời gian đã trôi qua của kỳ trước");
    expect(html).toContain("Kỳ tháng này: 1/9/2026 – 30/9/2026");
    expect(html).toContain('<span class="min-w-0 max-w-[46rem]">');
  });

  it("header row items-end gap-3; the buttons' row items-end gap-2", () => {
    const html = render(ALL);
    expect(html).toMatch(/<header class="[^"]*items-end[^"]*gap-3[^"]*"/);
    expect(html).toContain('<div class="flex flex-wrap items-end gap-2"><div role="group" aria-label="Kỳ báo cáo"');
  });

  it("a custom period says it compares with the same number of days just before", () => {
    const r = customWindows("2026-09-01", "2026-09-17");
    if (!r.ok) throw new Error(r.message);
    const html = render(ALL, { ok: true, duLieu: ROWS }, r.windows);
    expect(html).toContain("So với cùng số ngày liền trước (17 ngày): 15/8/2026 – 31/8/2026.");
    expect(html).not.toContain("đã trôi qua của kỳ trước");
    expect(html).toContain("Kỳ tuỳ chọn: 1/9/2026 – 17/9/2026 (17 ngày)");
  });

  it("five period buttons, Tháng này pressed by default", () => {
    const html = render(ALL);
    for (const label of ["Tuần này", "Tháng này", "Quý này", "Năm nay", "Tuỳ chọn"]) expect(html).toContain(`>${label}</button>`);
    expect(html).toMatch(/aria-pressed="true"[^>]*>Tháng này</);
  });

  it("prototype order: title · period buttons · export row · KPI grid · unit table · comparison", () => {
    const html = render(ALL);
    const at = [
      ">Báo cáo điều hành</h1>",
      ">Tuỳ chọn</button>",
      'aria-label="Xuất báo cáo"',
      "data-dashboard-grid",
      `aria-label="${UNIT_TABLE_TITLE}"`,
      'aria-label="So sánh với kỳ trước"',
    ].map((s) => html.indexOf(s));
    expect(at.every((i) => i >= 0)).toBe(true);
    expect([...at].sort((a, b) => a - b)).toEqual(at);
  });

  it("the KPI grid uses the report spacing: 1 column under 768px (D7), tiles always 2 across", () => {
    const html = render(ALL);
    const grid = /<div data-dashboard-grid="" class="([^"]*)"/.exec(html)?.[1] ?? "";
    expect(grid.split(" ")).toEqual(expect.arrayContaining(["grid-cols-1", "md:grid-cols-2", "xl:grid-cols-3", "gap-4"]));
    expect(html).not.toContain("@md:grid-cols-3");
  });

  it("'Tuỳ chọn' not chosen: no date box; no 'Xem' button anywhere", () => {
    const html = render(ALL);
    expect(html).not.toContain('id="bao-cao-tu-ngay"');
    expect(html).not.toContain(">Xem</button>");
  });

  it("a custom period on screen: the date box (page colour), 11.5px labels, h-9 w-44 inputs, no hint once set", () => {
    const r = customWindows("2026-09-01", "2026-09-17");
    if (!r.ok) throw new Error(r.message);
    const html = render(ALL, { ok: true, duLieu: ROWS }, r.windows);
    expect(html).toContain('class="flex max-w-full flex-wrap items-end gap-3 rounded-[10px] border border-line bg-canvas p-3"');
    expect(html).toMatch(/<label for="bao-cao-tu-ngay" class="[^"]*text-\[11\.5px\][^"]*">Từ ngày<\/label>/);
    expect(html).toMatch(/<input id="bao-cao-tu-ngay" type="date"[^>]*class="[^"]*h-9[^"]*w-44[^"]*text-\[12\.5px\]/);
    // inputs start EMPTY (prototype), so the hint shows
    expect(html).toContain(CUSTOM_PERIOD_HINT);
    expect(html).not.toContain(">Xem</button>");
  });

  it("the header outside the gate: the title only — no period, no button", () => {
    const html = renderToStaticMarkup(<ReportHeader />);
    expect(html).toContain(">Báo cáo điều hành</h1>");
    expect(html).not.toContain("<button");
    expect(html).not.toContain("tính đến");
  });
});
