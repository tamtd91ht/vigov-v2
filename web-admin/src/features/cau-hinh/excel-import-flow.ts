/**
 * The decision half of every "⬆ Nhập từ Excel" on the configuration screen (ADR 0059). Pure, so the
 * rows staff read when a file is refused — "dòng 4, cột Bộ phận cha: …" — and the one key per attempt
 * have tests.
 *
 * THE FLOW IS TWO REQUESTS ON PURPOSE: preview (writes nothing) then import (all or nothing). The
 * import is only offered once a preview of THE SAME FILE came back `valid`, and its `Idempotency-Key`
 * is minted at the first press — one key per attempt at importing one previewed file, kept across
 * retries of that attempt, dropped when a new file is chosen, a new preview is run, or the attempt
 * succeeds.
 *
 * WIRING A NEW IMPORT IS ONE `ImportTarget` VALUE — its routes, its row list's field name, its words
 * and its preview columns (`excel-import-targets.tsx`). Nothing in the flow or the panel knows which
 * rows it is importing.
 */

import type { ImportError, ImportRoutes } from "@/lib/api/excel-import";

/** One column of the preview table. `cell` returns TEXT: React escapes it, no markup is ever built. */
export type ImportColumn<R> = {
  readonly header: string;
  readonly cell: (row: R) => string;
  /** Codes are drawn monospaced, like everywhere on this screen (§5). */
  readonly mono?: boolean;
};

/** Everything one import needs. See the header. */
export type ImportTarget<R> = {
  /** DOM id fragment, unique per target (`tieu-de-nhap-${id}`, `o-tep-nhap-${id}`). */
  readonly id: string;
  readonly title: string;
  readonly explanation: string;
  readonly templateFileName: string;
  readonly confirmButton: string;
  /** Said above the errors table of an invalid preview — names what was NOT created. */
  readonly errorsHeading: string;
  /** `aria-label` of the preview table's scroll region. */
  readonly rowsLabel: string;
  readonly previewLead: (count: number) => string;
  /** `null` = the server replayed an earlier success of the same key; the count is unknown. */
  readonly importedSentence: (count: number | null) => string;
  readonly columns: readonly ImportColumn<R>[];
  readonly rowKey: (row: R) => string | number;
  readonly routes: ImportRoutes;
  /** Name of the row list in the preview body (`units`, `types`, …). */
  readonly rowsField: string;
};

export type ErrorRow = { readonly row: string; readonly column: string; readonly message: string };

/**
 * Server errors → table rows. `row: 0` is an error of the whole file and `column: ""` one of the whole
 * row: said in words, never shown as "0".
 */
export function errorRows(errors: readonly ImportError[]): readonly ErrorRow[] {
  return errors.map((e) => ({
    row: e.row === 0 ? "Cả tệp" : String(e.row),
    column: e.column === "" ? "—" : e.column,
    message: e.message,
  }));
}

/**
 * A preview as either shape: the normalised one (`rows`) or the org chart's contract shape (`units`),
 * which its panel still reads.
 */
type PreviewLike = { readonly valid: boolean } & (
  | { readonly rows: readonly unknown[] }
  | { readonly units: readonly unknown[] }
);

/** Whether the import button may be offered: a valid preview with something to create. */
export function canImport(preview: PreviewLike | null): boolean {
  if (preview === null || !preview.valid) return false;
  return ("rows" in preview ? preview.rows : preview.units).length > 0;
}

/**
 * The key for the import attempt. Kept if one exists (a retry of the SAME attempt), minted otherwise.
 * `mint` is a parameter so the test counts calls; the panel passes `crypto.randomUUID`.
 */
export function keyForAttempt(current: string | null, mint: () => string): string {
  return current ?? mint();
}

/**
 * The key after one send of the attempt: DROPPED on success (the attempt is over; the next import is a
 * new one), KEPT on any failure — a network error may have reached the server, and the retry must be
 * recognised as the same attempt, never imported twice.
 */
export function keyAfterAttempt(key: string, succeeded: boolean): string | null {
  return succeeded ? null : key;
}

export const TEMPLATE_BUTTON = "Tải tệp mẫu";
export const FILE_LABEL = "Chọn tệp Excel đã điền (.xlsx)";
export const PREVIEW_BUTTON = "Kiểm tra tệp";
export const PREVIEW_SENDING = "Đang kiểm tra tệp…";
export const IMPORT_SENDING = "Đang nhập…";
export const CLOSE_BUTTON = "Đóng";
export const IMPORT_BUTTON = "⬆ Nhập từ Excel";
export const NOTHING_TO_CREATE = "Tệp không có dòng nào để nhập.";
