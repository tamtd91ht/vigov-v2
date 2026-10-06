// The Tổng quan export files, built with the REAL libraries (jsPDF, pptxgenjs, write-excel-file) in
// Node and read back: the workbook and the deck are zips of XML, so the text is inspected directly;
// the PDF draws glyph ids, so for it the test proves the Vietnamese-capable font is EMBEDDED.

import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";

import { strFromU8, unzipSync } from "fflate";
import { afterEach, describe, expect, it, vi } from "vitest";

import type {
  documents_incomingSummaryOut,
  finance_chiSoNamRa,
  petitions_citizenReportSummaryOut,
  petitions_taskSummaryOut,
} from "@/lib/api/schema.gen";

import { buildPdf, buildPptx, buildXlsx, loadPdfFonts, PDF_FONT_URLS } from "./export-files";
import { EXPORT_MIME, EXPORT_NO_FIGURE, exportFileStem, URGENT_ROWS_OMITTED } from "./export-model";
import type { DashboardExport } from "./export-model";
import type { MergedQueue } from "./figures";
import { periodWindows } from "./period";
import { blockVisibility, dashboardExport, exportBlocks, figuresSettled } from "./view";
import type { DashboardData } from "./view";

const AT = Date.parse("2026-09-28T09:43:00Z"); // 16:43 in Viet Nam
const CLICK = Date.parse("2026-09-28T09:45:00Z"); // 16:45
const COMMUNE = { displayName: "Phường Đống Đa Kiểm Thử", parentAuthority: "Thành phố Hà Nội" };

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
  expenditure_achievement: { name: "Chi đạt dự toán", basis_points: null, unavailable_reason: "Chưa đánh dấu dòng tổng của bảng chi." },
  balance: { amount: 9_640_000_000 },
  revenue_totals: [{ column_id: "c1", name: "Thu xã hưởng", value: 690_000_000 }],
};

// Two overdue rows whose CODES must never reach a file (rule 3 — aggregates only).
const QUEUE: MergedQueue = {
  rows: [
    { module: "citizen-report", code: "PA-7F3K-9QXR-MNPT", kind: "han-xu-ly-xong", categoryCode: "", missedAt: "2026-09-20T08:00:00+07:00", critical: true },
    { module: "task", code: "NV-0042", kind: "han-xu-ly", categoryCode: "", missedAt: "2026-09-21T08:00:00+07:00", critical: false },
  ],
  failures: [],
};

const ALL = blockVisibility(["report.read", "task.read", "document.read", "feedback.read", "budget.read"]);

function data(): DashboardData {
  return {
    windows: periodWindows("month", new Date(AT)),
    fetchedAt: AT,
    fiscalYear: 2026,
    tasks: { current: ok(TASKS), previous: ok({ ...TASKS, completed: 8 }) },
    incomingDocuments: { current: ok(DOCS), previous: ok(DOCS) },
    citizenReports: { current: ok(REPORTS), previous: ok(REPORTS) },
    fiscal: ok(FISCAL),
    queue: ok(QUEUE),
    taskTypeLabels: null,
  };
}

const DOC: DashboardExport = dashboardExport(data(), ALL, COMMUNE, CLICK);

/** Every XML part of an OOXML zip, concatenated — what the reader's Office will read. */
async function ooxmlText(blob: Blob): Promise<{ all: string; parts: Record<string, string> }> {
  const files = unzipSync(new Uint8Array(await blob.arrayBuffer()));
  const parts: Record<string, string> = {};
  for (const [name, bytes] of Object.entries(files)) if (name.endsWith(".xml")) parts[name] = strFromU8(bytes);
  return { all: Object.values(parts).join("\n"), parts };
}

const require = createRequire(import.meta.url);
const FONT_DIR = path.dirname(require.resolve("@expo-google-fonts/roboto/package.json"));
const nodeFonts = () =>
  Promise.resolve({
    normal: new Uint8Array(readFileSync(path.join(FONT_DIR, "400Regular", "Roboto_400Regular.ttf"))),
    bold: new Uint8Array(readFileSync(path.join(FONT_DIR, "700Bold", "Roboto_700Bold.ttf"))),
  });

afterEach(() => vi.unstubAllGlobals());
describe("export model — the figures on screen, nothing else", () => {
  it("header: runtime commune name, the page title, the period as on screen, 'Số liệu tính đến HH:mm dd/MM/yyyy'", () => {
    expect(DOC.communeName).toBe(COMMUNE.displayName);
    expect(DOC.parentAuthority).toBe(COMMUNE.parentAuthority);
    expect(DOC.title).toBe("Tổng quan điều hành");
    expect(DOC.periodLabel).toBe("Kỳ tháng này: 1/9/2026 – 30/9/2026");
    expect(DOC.asOfLine).toBe("Số liệu tính đến 16:43 28/09/2026");
    expect(DOC.generatedLine).toBe("Xuất lúc 16:45 28/09/2026");
  });

  it("file name: tong-quan-<period>-<first day>, ASCII, no commune and no personal data", () => {
    expect(DOC.fileStem).toBe("tong-quan-thang-20260901");
    expect(exportFileStem(periodWindows("week", new Date(AT)))).toBe("tong-quan-tuan-20260928");
    expect(exportFileStem(periodWindows("quarter", new Date(AT)))).toBe("tong-quan-quy-20260701");
    expect(exportFileStem(periodWindows("year", new Date(AT)))).toBe("tong-quan-nam-20260101");
  });

  it("blocks in the grid's order; each tile's text verbatim, with the server's count behind it", () => {
    expect(DOC.blocks.map((b) => b.title)).toEqual([
      "Nhiệm vụ",
      "Văn bản & Đơn thư",
      "Giải ngân ngân sách",
      "Thu – Chi ngân sách",
      "Phản ánh người dân",
      "Kinh tế & Tài nguyên",
    ]);
    const tasks = DOC.blocks[0]!.figures;
    expect(tasks.map((f) => f.value)).toEqual(["24", "2", "9", "1", "75,0%"]);
    expect(tasks.find((f) => f.label === "Quá hạn")).toMatchObject({ number: 2, unit: "count", alert: true });
    expect(tasks.find((f) => f.value === "9")?.comparison).toBe("+12,5% so với kỳ trước");
    expect(tasks.find((f) => f.value === "75,0%")).toMatchObject({ note: "6/8 việc có hạn" });
    expect(tasks.find((f) => f.value === "75,0%")?.number).toBeUndefined();
    const fiscal = DOC.blocks[3]!;
    expect(fiscal.figures.map((f) => f.value)).toEqual([
      "73,50%",
      "Chưa đánh dấu dòng tổng của bảng chi.",
      "9,64 tỷ đồng",
      "690 triệu đồng",
    ]);
    expect(fiscal.figures[2]).toMatchObject({ number: 9_640_000_000, unit: "dong" });
    expect(fiscal.notes).toContain("Luỹ kế năm 2026, không so với kỳ trước.");
    expect(fiscal.notes.some((n) => n.includes("đang chờ khách hàng xác nhận"))).toBe(true);
  });

  it("'?' parts carry NO number: unbuilt blocks and the letters tile say 'Chưa có số liệu'", () => {
    for (const title of ["Giải ngân ngân sách", "Kinh tế & Tài nguyên"]) {
      const b = DOC.blocks.find((x) => x.title === title)!;
      expect(b.pending).toBeTruthy();
      expect(b.figures.length).toBeGreaterThan(0);
      for (const f of b.figures) {
        expect(f.value).toBe(EXPORT_NO_FIGURE);
        expect(f.number).toBeUndefined();
        expect(f.noFigure).toBe(true);
      }
    }
    const letters = DOC.blocks[1]!.figures.find((f) => f.label === "Đơn thư trong kỳ")!;
    expect(letters).toMatchObject({ value: EXPORT_NO_FIGURE, noFigure: true });
    expect(letters.number).toBeUndefined();
    const onTime = DOC.blocks[1]!.figures.find((f) => f.label === "Tỷ lệ đúng hạn văn bản")!;
    expect(onTime).toMatchObject({ value: "Chưa có dữ liệu", noFigure: true });
    expect(onTime.number).toBeUndefined();
  });

  it("'Cần xử lý ngay' is an aggregate: a count and a sentence, never a record code", () => {
    expect(DOC.urgent?.lines).toEqual([
      "2 việc quá hạn cần xử lý ngay, trong đó 1 việc nghiêm trọng.",
      URGENT_ROWS_OMITTED,
    ]);
    const json = JSON.stringify(DOC);
    expect(json).not.toContain("PA-7F3K-9QXR-MNPT");
    expect(json).not.toContain("NV-0042");
  });

  it("a full (capped) list is written 'từ 10 trở lên', never as an exact 10", () => {
    const rows = Array.from({ length: 10 }, (_, i) => ({ ...QUEUE.rows[1]!, code: `NV-${i}` }));
    const doc = dashboardExport({ ...data(), queue: ok({ rows, failures: [] }) }, ALL, COMMUNE, CLICK);
    expect(doc.urgent?.lines[0]).toMatch(/^Từ 10 việc quá hạn trở lên/);
  });

  it("the comparison note and the source sentence travel with the file", () => {
    expect(DOC.footnotes[0]).toMatch(/^So với cùng khoảng thời gian đã trôi qua của kỳ trước/);
    expect(DOC.footnotes.at(-1)).toContain("tệp không tính lại");
  });

  it("DENIED module: a block the account cannot see is not in the file either", () => {
    const only = blockVisibility(["report.read", "task.read"]);
    const titles = exportBlocks(data(), only).map((b) => b.title);
    expect(titles).toEqual(["Nhiệm vụ", "Kinh tế & Tài nguyên"]);
  });

  it("a FAILED call goes in as '—' with the server's sentence, never as 0", () => {
    const d = { ...data(), tasks: { current: { ok: false as const, thongBao: "Máy chủ bận." }, previous: ok(TASKS) } };
    const b = exportBlocks(d, ALL)[0]!;
    expect(b.figures.map((f) => f.value)).toEqual(["—", "—", "—", "—", "—"]);
    expect(b.errors).toEqual(["Chưa tải được số liệu nhiệm vụ: Máy chủ bận."]);
  });

  it("refuses a commune with no name — a file nobody can attribute is not written", () => {
    expect(() => dashboardExport(data(), ALL, { displayName: " ", parentAuthority: "" }, CLICK)).toThrow();
  });

  it("figuresSettled: false while any visible block is loading, true when all answered (even failed)", () => {
    expect(figuresSettled(data(), ALL)).toBe(true);
    expect(figuresSettled({ ...data(), queue: null }, ALL)).toBe(false);
    expect(figuresSettled({ ...data(), fiscal: null }, ALL)).toBe(false);
    expect(figuresSettled({ ...data(), tasks: { current: ok(TASKS), previous: null } }, ALL)).toBe(false);
    expect(figuresSettled({ ...data(), fiscal: { ok: false, thongBao: "x" } }, ALL)).toBe(true);
    // report.read alone: no figure block, nothing to export
    expect(figuresSettled(data(), blockVisibility(["report.read"]))).toBe(false);
  });
});

describe("XLSX", () => {
  it("a non-empty workbook with the spreadsheet MIME, Vietnamese text intact, numbers as numbers", async () => {
    const blob = await buildXlsx(DOC);
    expect(blob.type).toBe(EXPORT_MIME.xlsx);
    expect(blob.size).toBeGreaterThan(1000);
    const { all, parts } = await ooxmlText(blob);
    for (const s of [
      "Tổng quan điều hành",
      COMMUNE.displayName,
      COMMUNE.parentAuthority,
      "Kỳ tháng này: 1/9/2026 – 30/9/2026",
      "Số liệu tính đến 16:43 28/09/2026",
      "Phản ánh người dân",
      "Thu – Chi ngân sách",
      "Luỹ kế năm 2026, không so với kỳ trước.",
      "+12,5% so với kỳ trước",
      EXPORT_NO_FIGURE,
      URGENT_ROWS_OMITTED,
    ]) {
      expect(all).toContain(s.replaceAll("&", "&amp;"));
    }
    // counts and the exact balance are NUMBER cells
    const sheet = Object.entries(parts).find(([n]) => n.startsWith("xl/worksheets/"))![1];
    expect(sheet).toMatch(/<v>24<\/v>/);
    expect(sheet).toMatch(/<v>9640000000<\/v>/);
    expect(all).not.toContain("PA-7F3K-9QXR-MNPT");
    expect(all).not.toContain("NV-0042");
  });
});

describe("PPTX", () => {
  it("a non-empty deck with the presentation MIME: cover, one slide per built block, urgent, notes", async () => {
    const blob = await buildPptx(DOC);
    expect(blob.type).toBe(EXPORT_MIME.pptx);
    expect(blob.size).toBeGreaterThan(1000);
    const { all, parts } = await ooxmlText(blob);
    const slides = Object.keys(parts).filter((n) => /^ppt\/slides\/slide\d+\.xml$/.test(n));
    // cover + 4 built blocks + Cần xử lý ngay + Ghi chú
    expect(slides).toHaveLength(7);
    for (const s of [
      "Tổng quan điều hành",
      COMMUNE.displayName,
      "Kỳ tháng này: 1/9/2026 – 30/9/2026",
      "Số liệu tính đến 16:43 28/09/2026",
      "Nhiệm vụ",
      "Quá hạn",
      "75,0%",
      "9,64 tỷ đồng",
      "Cần xử lý ngay",
      "Giải ngân ngân sách — chưa có số liệu",
    ]) {
      expect(all).toContain(s.replaceAll("&", "&amp;"));
    }
    expect(all).toContain('typeface="Arial"');
    expect(all).not.toContain("PA-7F3K-9QXR-MNPT");
    expect(all).not.toContain("NV-0042");
  });
});

describe("PDF", () => {
  it("a non-empty PDF with Roboto EMBEDDED (Identity-H + ToUnicode) — the standard fonts cannot draw Vietnamese", async () => {
    const blob = await buildPdf(DOC, nodeFonts);
    expect(blob.type).toBe(EXPORT_MIME.pdf);
    const text = new TextDecoder("latin1").decode(new Uint8Array(await blob.arrayBuffer()));
    expect(text.startsWith("%PDF-")).toBe(true);
    expect(text).toContain("/FontFile2");
    expect(text).toContain("/Identity-H");
    expect(text).toContain("/ToUnicode");
    // jsPDF always DECLARES its 14 standard fonts; what matters is that both Roboto faces are EMBEDDED.
    expect(text).toContain("/BaseFont /Roboto");
    expect(text.match(/\/FontFile2/g)?.length).toBe(2);
    // The ToUnicode map carries the Vietnamese code points drawn: ổ (U+1ED5 "Tổng"), đ (U+0111).
    expect(text).toMatch(/<1ed5>/i);
    expect(text).toMatch(/<0111>/i);
  });

  it("the fonts come from the same-origin static path; a failed fetch fails the export", async () => {
    const fetchSpy = vi.fn((url: string) =>
      Promise.resolve(new Response(url === PDF_FONT_URLS.normal ? new Uint8Array([0, 1, 0, 0]) : null, { status: url === PDF_FONT_URLS.normal ? 200 : 404 })),
    );
    vi.stubGlobal("fetch", fetchSpy);
    await expect(loadPdfFonts()).rejects.toThrow(/404/);
    expect(fetchSpy.mock.calls.map((c) => c[0]).sort()).toEqual([PDF_FONT_URLS.bold, PDF_FONT_URLS.normal]);
    await expect(buildPdf(DOC)).rejects.toThrow();
  });
});
