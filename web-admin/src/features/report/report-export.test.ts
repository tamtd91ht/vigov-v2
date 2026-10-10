// `/bao-cao`'s export files (owner 09/10/2026, D3): Tổng quan's builders with the REPORT's figures,
// its unit table and its comparison as a table. Built with the real libraries and read back, like
// `features/dashboard/export.test.ts`.

import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";

import { strFromU8, unzipSync } from "fflate";
import { describe, expect, it } from "vitest";

import { buildPdf, buildPptx, buildXlsx } from "@/features/dashboard/export-files";
import { EXPORT_MIME, EXPORT_NO_FIGURE } from "@/features/dashboard/export-model";
import type { BlocksData } from "@/features/dashboard/view";
import { NHAN_CHENH_LECH } from "@/features/thu-chi/nhan-thu-chi";
import type {
  finance_chiSoNamRa,
  petitions_citizenReportSummaryOut,
  petitions_taskSummaryOut,
} from "@/lib/api/schema.gen";

import { reportAccess } from "./report-access";
import { buildReportExport, REPORT_EXPORT_SOURCE_NOTE, reportFileStem } from "./report-export";
import { customWindows, reportWindows } from "./report-period";
import { UNASSIGNED_UNIT_LABEL, UNIT_TABLE_TITLE } from "./unit-table";
import type { UnitRow } from "./unit-table";

const AT = Date.parse("2026-09-28T09:43:00Z"); // 16:43 in Viet Nam
const CLICK = Date.parse("2026-09-28T09:45:00Z");
const MONTH = reportWindows({ kind: "month" }, new Date(AT));
const COMMUNE = { displayName: "Xã Kiểm Thử", parentAuthority: "Thành phố Đà Nẵng" };
const ALL = ["report.read", "report.export", "task.read", "document.read", "feedback.read", "budget.read"];

const ok = <T,>(duLieu: T) => ({ ok: true as const, duLieu });

const TASKS: petitions_taskSummaryOut = { in_progress: 24, overdue: 2, suspended: 1, completed: 9, on_time_sample: 8, on_time: 6 };
const REPORTS: petitions_citizenReportSummaryOut = { received: 17, in_progress: 5, on_time_sample: 10, on_time: 9, late: 1 };
const FISCAL: finance_chiSoNamRa = {
  year: 2026,
  revenue_achievement: { name: "Thu đạt dự toán", basis_points: 7350 },
  expenditure_achievement: { name: "Chi đạt dự toán", basis_points: 9130 },
  balance: { amount: 3_400_000_000 },
  revenue_totals: [{ column_id: "c1", name: "Thu xã hưởng", value: 690_000_000 }],
};

function figures(): BlocksData {
  return {
    windows: MONTH,
    tasks: { current: ok(TASKS), previous: ok({ ...TASKS, completed: 8 }) },
    incomingDocuments: { current: null, previous: null },
    citizenReports: { current: ok(REPORTS), previous: ok({ ...REPORTS, received: 20 }) },
    fiscal: ok(FISCAL),
    fiscalYear: 2026,
    queue: null,
    taskTypeLabels: null,
  };
}

const UNITS: UnitRow[] = [
  { key: "01JBP1", name: "VĂN PHÒNG ĐẢNG ỦY", kind: "unit", total: 21, completed: 13, onTime: 4, onTimeSample: 12, overdue: 3 },
  { key: "", name: UNASSIGNED_UNIT_LABEL, kind: "unassigned", total: 2, completed: 0, onTime: 0, onTimeSample: 0, overdue: 0 },
];

const PERMS = ALL.filter((p) => p !== "document.read");

function doc(permissions: readonly string[] = PERMS) {
  return buildReportExport({
    commune: COMMUNE,
    windows: MONTH,
    fetchedAt: AT,
    generatedAt: CLICK,
    figures: figures(),
    access: reportAccess(permissions),
    units: { ok: true, duLieu: UNITS },
  });
}

async function ooxmlText(blob: Blob): Promise<string> {
  const files = unzipSync(new Uint8Array(await blob.arrayBuffer()));
  return Object.entries(files)
    .filter(([n]) => n.endsWith(".xml"))
    .map(([, b]) => strFromU8(b))
    .join("\n");
}

const require = createRequire(import.meta.url);
const FONT_DIR = path.dirname(require.resolve("@expo-google-fonts/roboto/package.json"));
const nodeFonts = () =>
  Promise.resolve({
    normal: new Uint8Array(readFileSync(path.join(FONT_DIR, "400Regular", "Roboto_400Regular.ttf"))),
    bold: new Uint8Array(readFileSync(path.join(FONT_DIR, "700Bold", "Roboto_700Bold.ttf"))),
  });

describe("report export model — this page's figures, nothing recomputed", () => {
  const d = doc();

  it("header: runtime commune, 'Báo cáo điều hành', the report's period line, the click time", () => {
    expect(d.communeName).toBe(COMMUNE.displayName);
    expect(d.title).toBe("Báo cáo điều hành");
    expect(d.periodLabel).toBe("Kỳ tháng này: 1/9/2026 – 30/9/2026");
    expect(d.asOfLine).toBe("Số liệu tính đến 16:43 28/09/2026");
    expect(d.generatedLine).toBe("Xuất lúc 16:45 28/09/2026");
    expect(d.urgent).toBeNull();
    expect(d.footnotes.at(-1)).toBe(REPORT_EXPORT_SOURCE_NOTE);
    expect(REPORT_EXPORT_SOURCE_NOTE).toContain("màn Báo cáo điều hành");
  });

  it("file name: bao-cao-<period>-<first day>, a custom range by both days — ASCII, no commune", () => {
    expect(d.fileStem).toBe("bao-cao-thang-20260901");
    const r = customWindows("2026-09-01", "2026-09-17");
    if (!r.ok) throw new Error(r.message);
    expect(reportFileStem(r.windows)).toBe("bao-cao-tuy-chon-20260901-20260917");
  });

  it("blocks as the report draws them: 'Tiếp nhận trong kỳ', no red, fiscal in the prototype's order and sums", () => {
    const citizen = d.blocks.find((b) => b.title === "Phản ánh người dân")!;
    expect(citizen.figures[0]).toMatchObject({ label: "Tiếp nhận trong kỳ", value: "17", comparison: "-15,0% so với kỳ trước" });
    expect(d.blocks.flatMap((b) => b.figures).some((f) => f.alert === true)).toBe(false);
    const fiscal = d.blocks.find((b) => b.title === "Thu - Chi ngân sách xã")!;
    expect(fiscal.figures.map((f) => f.label)).toEqual(["Thu đạt dự toán", "Thu xã hưởng", "Chi đạt dự toán", "Tổng chi", NHAN_CHENH_LECH]);
    expect(fiscal.figures.map((f) => f.value)).toEqual(["73,50%", "690 triệu", "91,30%", EXPORT_NO_FIGURE, "3,4 tỷ"]);
    expect(fiscal.figures[4]).toMatchObject({ number: 3_400_000_000, unit: "dong" });
  });

  it("the unit table: our rows, the screen's columns, counts as numbers, the rounded on-time rate", () => {
    const t = d.tables!.find((x) => x.title === UNIT_TABLE_TITLE)!;
    expect(t.columns).toEqual(["Bộ phận", "Tổng việc", "Quá hạn (hiện tại)", "Hoàn thành", "Đúng hạn"]);
    expect(t.rows.map((r) => r.map((c) => c.text))).toEqual([
      ["VĂN PHÒNG ĐẢNG ỦY", "21", "3", "13", "33%"],
      [UNASSIGNED_UNIT_LABEL, "2", "0", "0", "—"],
    ]);
    expect(t.rows[0]![1]).toMatchObject({ number: 21 });
  });

  it("the comparison as a table: period figures only, both periods and the change", () => {
    const t = d.tables!.find((x) => x.title === "So sánh với kỳ trước")!;
    expect(t.rows.map((r) => r.map((c) => c.text))).toEqual([
      ["Hoàn thành trong kỳ", "9", "8", "+12,5%", "Tốt lên"],
      ["Đúng hạn trong kỳ (nhiệm vụ)", "75,0%", "75,0%", "0,0%", "—"],
      ["Tiếp nhận trong kỳ", "17", "20", "-15,0%", "—"],
      ["Đúng hạn trong kỳ (phản ánh)", "90,0%", "90,0%", "0,0%", "—"],
      ["Trễ hạn trong kỳ", "1", "1", "0,0%", "—"],
    ]);
  });

  it("DENIED: a table the account cannot see is not in the file — no task.read, no unit table", () => {
    const titles = doc(["report.read", "report.export", "feedback.read"]).tables!.map((t) => t.title);
    expect(titles).toEqual(["So sánh với kỳ trước"]);
    expect(doc(["report.read", "report.export", "budget.read"]).tables).toEqual([]);
  });

  it("refuses a commune with no name — a file nobody can attribute is not written", () => {
    expect(() =>
      buildReportExport({
        commune: { displayName: " ", parentAuthority: "" },
        windows: MONTH,
        fetchedAt: AT,
        generatedAt: CLICK,
        figures: figures(),
        access: reportAccess(PERMS),
        units: null,
      }),
    ).toThrow();
  });
});

// Building real XLSX/PPTX/PDF files takes seconds; under a loaded full run the 5 s default timed out
// with nothing wrong (the dashboard export tests hit the same). 30 s keeps a hang visible.
describe("report files — the shared builders carry the tables", { timeout: 30_000 }, () => {
  it("XLSX: the unit table and the comparison, counts as NUMBER cells, Vietnamese intact", async () => {
    const blob = await buildXlsx(doc());
    expect(blob.type).toBe(EXPORT_MIME.xlsx);
    const all = await ooxmlText(blob);
    for (const s of ["Báo cáo điều hành", "Xếp hạng bộ phận", "VĂN PHÒNG ĐẢNG ỦY", "Quá hạn (hiện tại)", "So sánh với kỳ trước", "Tiếp nhận trong kỳ", "Tổng chi"]) {
      expect(all).toContain(s);
    }
    expect(all).toMatch(/<v>21<\/v>/);
  });

  it("PPTX: a slide per table after the blocks", async () => {
    const blob = await buildPptx(doc());
    expect(blob.type).toBe(EXPORT_MIME.pptx);
    const all = await ooxmlText(blob);
    expect(all).toContain("Xếp hạng bộ phận");
    expect(all).toContain("VĂN PHÒNG ĐẢNG ỦY");
    expect(all).toContain("So sánh với kỳ trước");
  });

  it("PDF: built with the embedded font", async () => {
    const blob = await buildPdf(doc(), nodeFonts);
    expect(blob.type).toBe(EXPORT_MIME.pdf);
    const text = new TextDecoder("latin1").decode(new Uint8Array(await blob.arrayBuffer()));
    expect(text.startsWith("%PDF-")).toBe(true);
    expect(text).toContain("/FontFile2");
  });
});
