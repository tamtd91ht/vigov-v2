/**
 * "Nhập từ Excel" of the citizen-letter register and "Xuất Excel" of its Báo cáo tab — `service-documents`
 * (`internal/http/routes_citizen_letter_import.go`, ADR 0084 #5/#6).
 *
 *   GET  /api/v1/citizen-letters/import-template   petition.create — the .xlsx to fill in
 *   POST /api/v1/citizen-letters/import-previews   petition.create — check, WRITES NOTHING, takes no number
 *   POST /api/v1/citizen-letters/imports           petition.create — book every row or none (Idempotency-Key)
 *   GET  /api/v1/citizen-letter-report/exports?year=   report.export AND petition.read — audited by the server
 *
 * THE IMPORT IS THE SHARED THREE-ROUTE SHAPE: `excel-import.ts` is its one client (part `file`, no
 * hand-set `Content-Type`, the caller's key, 413/415 fallbacks). Only the paths and the preview's row-list
 * name are given here.
 *
 * Nothing here names a commune or a person: the Host does, the session does (`goi.ts`). Replies are not
 * logged — the preview rows and the file are the commune's register as staff typed it (rule 3).
 */

import type { ImportRoutes } from "./excel-import";
import {
  CHUNG, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  LOI_KHONG_RO, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  thongBaoLoi, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  type KetQua, // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
} from "./goi";
import { REGISTER_EXPORT_FALLBACK_NAME, registerFileName } from "./nhiem-vu";
import type {
  documents_get_citizen_letter_report_exports,
  documents_get_citizen_letters_import_template,
  documents_letterImportPreviewOut,
  documents_post_citizen_letters_import_previews,
  documents_post_citizen_letters_imports,
} from "./schema.gen";

export type { documents_letterImportRowOut as LetterImportRow } from "./schema.gen";

export const LETTER_IMPORT_ROUTES: ImportRoutes = {
  template: "/api/v1/citizen-letters/import-template" satisfies documents_get_citizen_letters_import_template["duongDan"],
  previews: "/api/v1/citizen-letters/import-previews" satisfies documents_post_citizen_letters_import_previews["duongDan"],
  imports: "/api/v1/citizen-letters/imports" satisfies documents_post_citizen_letters_imports["duongDan"],
};

/** The field of the preview body that holds the planned letters. */
export const LETTER_ROWS_FIELD = "letters" satisfies keyof documents_letterImportPreviewOut;

/** The template's saved name — the server's own (`citizen_letter_import.go`, `mau-so-don-thu.xlsx`). */
export const LETTER_TEMPLATE_FILE_NAME = "mau-so-don-thu.xlsx";

/* ---- the report export -------------------------------------------------------------------- */

/** The file of one export: bytes and the name to save them under. */
export type LetterReportFile = { readonly blob: Blob; readonly fileName: string };

/** The server's own name for the year's file — used only when the reply names none. */
export function letterReportFallbackName(year: number): string {
  return `bao-cao-don-thu-nam-${year}.xlsx`;
}

/**
 * The export path. ⚠ CONTRACT GAP: the generated route declares no query, but the handler REQUIRES
 * `year` (`citizen_letter_report_export.go`; 400 without it). Written by hand until the contract is
 * regenerated — the year is never left to a server default, for the reason the report read gives
 * (`citizenLetterReportPath`).
 */
export function letterReportExportPath(year: number): string {
  const path: documents_get_citizen_letter_report_exports["duongDan"] = "/api/v1/citizen-letter-report/exports";
  return `${path}?${new URLSearchParams({ year: String(year) }).toString()}`;
}

/**
 * GET the year's report as .xlsx. READ AS A BLOB, NOT NAVIGATED TO: a refusal (403, 400, 503) comes back
 * as the server's sentence on this screen, never as JSON in a new tab. No Idempotency-Key: a second click
 * is a second export, and the server audits each one.
 */
export async function downloadLetterReport(year: number): Promise<KetQua<LetterReportFile>> {
  let res: Response;
  try {
    res = await fetch(letterReportExportPath(year), { ...CHUNG, method: "GET" });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 200) return { ok: false, thongBao: await thongBaoLoi(res) };
  try {
    const blob = await res.blob();
    // The task register's Content-Disposition reader, with THIS file's fallback name.
    const named = registerFileName(res.headers.get("Content-Disposition"));
    return { ok: true, duLieu: { blob, fileName: named === REGISTER_EXPORT_FALLBACK_NAME ? letterReportFallbackName(year) : named } };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}
