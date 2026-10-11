/**
 * The `Tải file mẫu` workbook of `Nạp từ Excel` (customer bug sheet row 49) — built IN THE BROWSER, from
 * the sheet on screen, in the exact layout `service-finance` reads
 * (`service-finance/internal/domain/budget_import.go`, `ParseBudgetWorkbook` / `parseBudgetSheet`):
 *
 *   row 1   the sheet's title, column A   → a heading line above the header: it names the kind
 *                                          ("thu" / "chi" as a word) and becomes the imported title
 *   row 2   `Đơn vị tính: <unit>`, col A  → the unit line, REQUIRED: the server never guesses a unit
 *   row 3   `TT` | `Nội dung` | columns…  → the header row: found by column A saying `TT` (within the
 *                                          first 20 rows); figure columns are read from column C on
 *   row 4…  empty — the preparer types the lines (TT in A, content in B, figures from C)
 *
 * The tab is named `Thu` / `Chi`, the server's fallback for the kind when the title names neither.
 *
 * NOTHING HERE IS A SECOND PARSER: the server checks the whole file on upload and refuses it with a
 * sentence. This only writes the shape that parser accepts, so the officer starts from a file that loads.
 *
 * NO COMMUNE NAME IS BAKED IN (rule 1 inv 10): the title is the loaded sheet's own (data the server
 * stores); without a sheet the heading names only the kind and the year.
 */

import type { finance_bangDayDuRa } from "@/lib/api/schema.gen";
import type { LoaiBang } from "@/lib/api/thu-chi";

// vi-name-ok: existing helpers and types of nhan-thu-chi.ts, imported not declared (rule 12 inv 3)
import { boCotKhoiDiem, DON_VI_KHOI_DIEM, DON_VI_TINH, donViCuaBang, type MaDonVi } from "./nhan-thu-chi";

export type BudgetTemplate = {
  readonly kind: LoaiBang;
  readonly year: number;
  readonly title: string;
  readonly unit: MaDonVi;
  /** In display order. `type` is `so` | `phan_tram`. */
  readonly columns: readonly { readonly name: string; readonly type: string }[];
};

/** The popup's one-line note on what the importer requires (`parseBudgetSheet`). */
export const TEMPLATE_REQUIRED_NOTE =
  "Bắt buộc: dòng “Đơn vị tính”, dòng tiêu đề có cột A là “TT”, cột B “Nội dung”; cột số từ cột C trở đi. " +
  "Cột tỷ lệ % do hệ thống tính, không cần gõ.";

/**
 * The template of the sheet on screen: its title, unit and columns when it is loaded; otherwise the
 * Lập bảng form's default columns (`boCotKhoiDiem`) and its default unit.
 */
export function budgetTemplateFor(
  kind: LoaiBang,
  year: number,
  loaded: finance_bangDayDuRa | null,
): BudgetTemplate {
  if (loaded !== null && loaded.sheet.kind === kind && loaded.sheet.year === year) {
    return {
      kind,
      year,
      title: loaded.sheet.title,
      // The unit the screen DRAWS in (`donViCuaBang`): an unmapped legacy unit draws in đồng, so the
      // template says đồng — never a guessed factor.
      unit: donViCuaBang(loaded.sheet).ma,
      columns: [...loaded.columns]
        .sort((a, b) => a.order - b.order)
        .map((c) => ({ name: c.name, type: c.type })),
    };
  }
  return {
    kind,
    year,
    title: defaultTitle(kind, year),
    unit: DON_VI_KHOI_DIEM,
    columns: boCotKhoiDiem(kind, year).map((c) => ({ name: c.name, type: c.type })),
  };
}

function defaultTitle(kind: LoaiBang, year: number): string {
  return `BÁO CÁO ${kind === "chi" ? "CHI" : "THU"} NGÂN SÁCH NHÀ NƯỚC NĂM ${year}`;
}

/**
 * The kind a heading names, by the server's rule (`guessSheetKind`): "thu" / "chi" as WHOLE words,
 * the first one wins; `null` when it names neither.
 */
function kindNamedBy(text: string): LoaiBang | null {
  for (const w of text.toLowerCase().split(/[^\p{L}]+/u)) {
    if (w === "thu") return "thu";
    if (w === "chi") return "chi";
  }
  return null;
}

/**
 * The cell text of every row, row 1 first — what `buildBudgetTemplateXlsx` writes.
 *
 * Two adjustments keep the file reading back as THIS sheet:
 *   - a title that names the OTHER kind first ("Báo cáo thu, chi…" on the chi sheet) would file the
 *     import under the wrong kind, so the heading falls back to the kind's default title;
 *   - the server tells a percentage column only by its heading (`%`, `tỷ lệ`, `tỉ lệ`), so a `phan_tram`
 *     column whose name has none gets ` (%)` — otherwise it would come back as a figure column.
 */
export function budgetTemplateRows(t: BudgetTemplate): string[][] {
  const named = kindNamedBy(t.title);
  const title = named === null || named === t.kind ? t.title : defaultTitle(t.kind, t.year);
  const unitLabel = DON_VI_TINH.find((d) => d.ma === t.unit)?.nhan ?? "Đồng";
  const header = [
    "TT",
    "Nội dung",
    ...t.columns.map((c) => (c.type === "phan_tram" && !looksLikePercent(c.name) ? `${c.name} (%)` : c.name)),
  ];
  return [[title], [`Đơn vị tính: ${unitLabel}`], header];
}

function looksLikePercent(name: string): boolean {
  const low = name.toLowerCase();
  return low.includes("%") || low.includes("tỷ lệ") || low.includes("tỉ lệ");
}

/** `mau-nap-chi-ngan-sach-2026.xlsx` — ASCII, no commune name. */
export function budgetTemplateFileName(kind: LoaiBang, year: number): string {
  return `mau-nap-${kind}-ngan-sach-${year}.xlsx`;
}

const XLSX_MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";

/**
 * The workbook. write-excel-file (the library the Tổng quan export already ships) is a DYNAMIC import,
 * loaded on the click only. Every cell is written as TEXT — there is no figure in a template.
 */
export async function buildBudgetTemplateXlsx(t: BudgetTemplate): Promise<Blob> {
  const { default: writeXlsxFile } = await import("write-excel-file/universal");
  const rows = budgetTemplateRows(t);
  const headerRow = rows.length - 1;
  const data = rows.map((r, i) =>
    r.map((value) => ({
      value,
      type: String,
      fontWeight: i === 0 || i === headerRow ? ("bold" as const) : undefined,
      wrap: i === headerRow,
    })),
  );
  const blob = await writeXlsxFile(
    data,
    {
      sheet: t.kind === "chi" ? "Chi" : "Thu",
      columns: [{ width: 8 }, { width: 60 }, ...t.columns.map(() => ({ width: 18 }))],
    },
    { fontFamily: "Arial", fontSize: 11 },
  ).toBlob();
  return new Blob([blob], { type: XLSX_MIME });
}
