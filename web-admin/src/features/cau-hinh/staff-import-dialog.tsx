"use client";

import type { ImportPreview, ImportResult } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import type { StaffImportCreatedRow, StaffImportPlannedRow } from "@/lib/api/staff-import";

import { ExcelImportDialog, ExcelImportDialogView, type ImportDialogBusy } from "./excel-import-dialog";
import type { ImportTarget } from "./excel-import-flow";
import { STAFF_IMPORT_TARGET } from "./excel-import-targets";
import { STAFF_IMPORT_ERRORS_HEADING, STAFF_IMPORT_TITLE } from "./nhan-can-bo";

/** `check` = the preview that runs first when `Nhập` is pressed; `import` = the write. */
export type StaffImportBusy = ImportDialogBusy;

/**
 * The staff target as the dialog shows it: the user's title (09/10/2026) and a heading that ends in
 * "nhập lại" — there is no separate check button any more. Routes, columns and the one-time password
 * view (`resultView`) are `STAFF_IMPORT_TARGET`'s, unchanged.
 */
const STAFF_DIALOG_TARGET: ImportTarget<StaffImportPlannedRow, StaffImportCreatedRow> = {
  ...STAFF_IMPORT_TARGET,
  title: STAFF_IMPORT_TITLE,
  errorsHeading: STAFF_IMPORT_ERRORS_HEADING,
};

/**
 * `Nhập người dùng từ Excel` — the shared 672px dialog (`excel-import-dialog.tsx`) bound to the staff
 * target. The flow (preview, then the write under one key per attempt) and the 201's temporary
 * passwords — drawn once by `StaffImportResult`, which owns closing — are described there.
 *
 * Opened only by `admin.user` holders (the screen's gate); the three routes check the key again (rule 5).
 */
export function StaffImportDialog({
  onClose,
  onImported,
}: {
  onClose: () => void;
  /** A 201 (or a replayed 201): the list must be read again — nothing is spliced in. */
  onImported: () => void;
}) {
  return <ExcelImportDialog target={STAFF_DIALOG_TARGET} onClose={onClose} onImported={onImported} />;
}

/** The body, without state or hooks — rendered by the tests with `renderToStaticMarkup`. */
export function StaffImportView(props: {
  fileName: string | null;
  preview: KetQua<ImportPreview<StaffImportPlannedRow>> | null;
  result: ImportResult<StaffImportCreatedRow> | null;
  busy: StaffImportBusy;
  templateError: string;
  onTemplate: () => void;
  onChoose: (f: File | null) => void;
  onImport: () => void;
  onClose: () => void;
}) {
  return <ExcelImportDialogView target={STAFF_DIALOG_TARGET} {...props} />;
}
