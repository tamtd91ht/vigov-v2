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

import type { ReactNode } from "react";

import type { ImportError, ImportPreview, ImportResult, ImportRoutes } from "@/lib/api/excel-import";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)

/** One column of the preview table. `cell` returns TEXT: React escapes it, no markup is ever built. */
export type ImportColumn<R> = {
  readonly header: string;
  readonly cell: (row: R) => string;
  /** Codes are drawn monospaced, like everywhere on this screen (§5). */
  readonly mono?: boolean;
};

/**
 * Everything one import needs. See the header.
 *
 * `R` is a row of the preview, `C` a row of the 201. They are the same type for every catalogue; the
 * staff import is the one where they differ — the preview plans a person, the 201 carries the minted
 * code, the login and, once, the temporary password.
 */
export type ImportTarget<R, C = R> = {
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
  /**
   * What a successful import shows BESIDES `importedSentence`, or absent for "the sentence is enough".
   * `created` is `null` on a replay. When present it owns closing: the panel's own "Đóng" is not drawn,
   * because the staff import's result holds values that must be closed by an explicit act, never by the
   * reflex button (`staff-import-result.tsx`).
   */
  readonly resultView?: (created: readonly C[] | null, onClose: () => void) => ReactNode;
};

/** The file the staff member chose, with the name it had on their machine. */
export type ChosenFile = { readonly blob: Blob; readonly name: string };

/**
 * One attempt at one file, as the panel holds it. EVERYTHING THE SERVER SENT BACK LIVES HERE AND ONLY
 * HERE — for the staff import that includes the temporary passwords of the 201 — so the transitions
 * below are the whole answer to "when does that value stop existing on this screen".
 */
export type ImportAttempt<R, C = R> = {
  readonly file: ChosenFile | null;
  readonly preview: KetQua<ImportPreview<R>> | null;
  readonly key: string | null;
  readonly result: ImportResult<C> | null;
};

export type AttemptEvent<R, C = R> =
  | { readonly type: "chosen"; readonly file: ChosenFile | null }
  | { readonly type: "previewStarted" }
  | { readonly type: "previewed"; readonly preview: KetQua<ImportPreview<R>> }
  | { readonly type: "importStarted"; readonly key: string }
  | { readonly type: "imported"; readonly key: string; readonly result: ImportResult<C> }
  | { readonly type: "closed" };

export const EMPTY_ATTEMPT: ImportAttempt<never, never> = { file: null, preview: null, key: null, result: null };

/**
 * The attempt after one event. Pure, so the rules have tests:
 *
 *   chosen          a NEW FILE IS A NEW ATTEMPT — preview, key and result all start over
 *   previewStarted  the previous result goes (it answered another check)
 *   previewed       a new preview is a new attempt: the file may have been re-saved under the same name
 *   importStarted   the key of this attempt is recorded before the send, so a retry reuses it
 *   imported        key kept on failure, dropped on success (`keyAfterAttempt`)
 *   closed          EVERYTHING goes — the file, the preview, the key and the 201 with its passwords.
 *                   The caller also unmounts the panel; clearing here as well means no path that keeps
 *                   the panel mounted (a parent that forgets to) keeps the values
 */
export function nextAttempt<R, C>(state: ImportAttempt<R, C>, event: AttemptEvent<R, C>): ImportAttempt<R, C> {
  switch (event.type) {
    case "chosen":
      return { file: event.file, preview: null, key: null, result: null };
    case "previewStarted":
      return { ...state, result: null };
    case "previewed":
      return { ...state, preview: event.preview, key: null };
    case "importStarted":
      return { ...state, key: event.key };
    case "imported":
      return { ...state, result: event.result, key: keyAfterAttempt(event.key, event.result.ok) };
    case "closed":
      return EMPTY_ATTEMPT;
  }
}

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
// No emoji: the screens draw a lucide `Upload` beside the word (ADR 0068 §2, spec §4).
export const IMPORT_BUTTON = "Nhập từ Excel";
export const NOTHING_TO_CREATE = "Tệp không có dòng nào để nhập.";
