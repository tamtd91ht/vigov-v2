import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { BlocksData } from "@/features/dashboard/view";
import type { Loaded } from "@/features/dashboard/view";

import { reportAccess } from "./report-access";
import { customWindows, reportWindows } from "./report-period";
import type { ReportWindows } from "./report-period";
import { ReportHeader, ReportView, UNIT_LOAD_ERROR } from "./report-view";
import { UNIT_TABLE_NOTE, UNIT_TABLE_TITLE, UNKNOWN_UNIT_LABEL } from "./unit-table";
import type { UnitRow } from "./unit-table";

/**
 * The DENIED branches first — the ones a developer holding every key never sees — then the page's
 * shape. Rendered to a string in Node, like `features/dashboard/view.test.tsx`.
 */

const AT = Date.parse("2026-09-28T09:43:00Z"); // 16:43 in Viet Nam
const MONTH = reportWindows({ kind: "month" }, new Date(AT));

const ALL = ["report.read", "report.export", "task.read", "document.read", "feedback.read", "budget.read"];

function figures(windows: ReportWindows): BlocksData {
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

const ROWS: UnitRow[] = [
  { key: "01JBP1", name: "VĂN PHÒNG ĐẢNG ỦY", kind: "unit", total: 21, completed: 13, onTime: 4, onTimeSample: 12, overdue: 3 },
  { key: "01JGONE", name: UNKNOWN_UNIT_LABEL, kind: "unknown", total: 2, completed: 0, onTime: 0, onTimeSample: 0, overdue: 0 },
];

function render(
  permissions: readonly string[],
  units: Loaded<UnitRow[]> = { ok: true, duLieu: ROWS },
  windows: ReportWindows = MONTH,
): string {
  return renderToStaticMarkup(
    <ReportView
      windows={windows}
      fetchedAt={AT}
      figures={figures(windows)}
      access={reportAccess(permissions)}
      units={units}
      onNamedPeriod={() => {}}
      onCustomPeriod={() => {}}
      onReload={() => {}}
      onReloadUnits={() => {}}
    />,
  );
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

describe("export row", () => {
  it("drawn disabled with one '?' when the session holds report.export — the prototype's 'Xuất …' labels", () => {
    const html = render(ALL);
    expect(html).toContain('aria-label="Xuất báo cáo"');
    for (const label of ["Xuất PDF", "Xuất XLSX", "Xuất PPTX"]) {
      expect(html).toMatch(new RegExp(`<button[^>]*disabled=""[^>]*>.*?${label}</button>`));
    }
    expect(html.split(pendingMarkerLabel("Xuất báo cáo PDF, XLSX, PPTX"))).toHaveLength(2);
    expect(html).not.toContain("Trình chiếu");
  });

  it("DENIED: no export row at all without report.export", () => {
    const html = render(ALL.filter((p) => p !== "report.export"));
    expect(html).not.toContain('aria-label="Xuất báo cáo"');
    expect(html).not.toContain("Xuất PDF");
    expect(html).not.toContain(pendingMarkerLabel("Xuất báo cáo PDF, XLSX, PPTX"));
  });
});

describe("ReportView — unit table gating", () => {
  it("DENIED: no table and no request-shaped trace without task.read", () => {
    const html = render(["report.read", "budget.read"]);
    expect(html).not.toContain(UNIT_TABLE_TITLE);
  });

  it("DENIED: no Thu – Chi block without budget.read", () => {
    const html = render(["report.read", "task.read"]);
    expect(html).not.toContain('aria-label="Thu – Chi ngân sách"');
    expect(render(ALL)).toContain('aria-label="Thu – Chi ngân sách"');
  });

  it("allowed: the table, its columns and the note", () => {
    const html = render(ALL);
    expect(html).toContain(`aria-label="${UNIT_TABLE_TITLE}"`);
    expect(html).not.toContain("Xếp hạng");
    expect(html).toContain("Quá hạn (hiện tại)");
    expect(html).toContain("4 — 33,3%");
    expect(html).toContain(UNKNOWN_UNIT_LABEL);
    expect(html).toContain(UNIT_TABLE_NOTE);
    // only the table scrolls: the labelled, focusable scroller
    expect(html).toMatch(/role="region" tabindex="0"[^>]*aria-label="Tình hình thực hiện theo bộ phận"|aria-label="Tình hình thực hiện theo bộ phận"[^>]*role="region"/);
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

  it("empty: one sentence, no table", () => {
    const html = render(ALL, { ok: true, duLieu: [] });
    expect(html).not.toContain("<table");
    expect(html).toContain('role="status"');
  });
});

describe("ReportView — page shape (spec 13 §1–§2, ADR 0053 B4/B5a)", () => {
  it("no 'Cần xử lý ngay', no 'Tính lại ngay', no 'Trình chiếu'", () => {
    const html = render(ALL);
    expect(html).not.toContain("Cần xử lý ngay");
    expect(html).not.toContain("Tính lại ngay");
    expect(html).not.toContain("Trình chiếu");
  });

  it("the meta line: 'Số liệu tính đến' + /tong-quan's comparison for a named period", () => {
    const html = render(ALL);
    expect(html).toContain("Số liệu tính đến 16:43 28/09/2026.");
    expect(html).toContain("So với cùng khoảng thời gian đã trôi qua của kỳ trước");
    expect(html).toContain("Kỳ tháng này: 1/9/2026 – 30/9/2026");
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

  it("the comparison chart is a pending card with '?' after the table", () => {
    const html = render(ALL);
    expect(html).toContain(pendingMarkerLabel("So sánh với kỳ trước"));
    expect(html.indexOf(UNIT_TABLE_TITLE)).toBeLessThan(html.indexOf("So sánh với kỳ trước"));
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

  it("the KPI grid uses the report spacing: tiles always 2 across, never 3", () => {
    const html = render(ALL);
    const grid = /<div data-dashboard-grid="" class="([^"]*)"/.exec(html)?.[1] ?? "";
    expect(grid.split(" ")).toEqual(expect.arrayContaining(["grid-cols-1", "md:grid-cols-2", "xl:grid-cols-3", "gap-4"]));
    expect(html).not.toContain("@md:grid-cols-3");
  });

  it("'Tuỳ chọn' not chosen: no date box", () => {
    expect(render(ALL)).not.toContain('id="bao-cao-tu-ngay"');
  });

  it("the header outside the gate: the title only — no period, no button", () => {
    const html = renderToStaticMarkup(<ReportHeader />);
    expect(html).toContain(">Báo cáo điều hành</h1>");
    expect(html).not.toContain("<button");
    expect(html).not.toContain("tính đến");
  });

  it("the citizen-report block keeps /tong-quan's label 'Nhận vào trong kỳ'", () => {
    const html = render(ALL);
    expect(html).toContain("Nhận vào trong kỳ");
    expect(html).not.toContain(">Tiếp nhận<");
  });
});
