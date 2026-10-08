/**
 * `⬆ Nạp từ Excel` of Thu - Chi ngân sách (`docs/ui-ux/07-thu-chi-ngan-sach.md` §6, ADR 0081 #6).
 * Owner: `service-finance` (`routes_budget_import.go`, `budget_import.go`):
 *
 *   POST /api/v1/budget-sheets/imports?year=   budget.update   201 budgetImportOut · 400 import_invalid
 *                                                              · 409 budget_period_closed / budget_sheet_has_entries
 *
 * ONLY THE IMPORT IS CALLED: the screen loads the file the moment it is picked, as the prototype does
 * (`FiscalReportPanel.tsx:131-153`, ADR 0081 #6 "theo tài liệu + prototype"). The server's
 * `import-previews` route exists and is unused here.
 *
 * The upload rules are `excel-import.ts`'s (one multipart part `file`, no hand-set `Content-Type`, the
 * caller's key, the 413/415 fallbacks of a proxy) and are taken from there, never re-typed.
 *
 * `year` IS THE SCREEN'S SELECTED YEAR, sent explicitly: the server has no default and never reads the
 * year from the file's headings.
 *
 * THE FAILURE CARRIES THE SERVER'S `code`, for ONE documented use (`KetQua.code`'s rule in `goi.ts`): the
 * button maps the unreadable-file codes to the prototype's own toast sentences
 * (`budget-import-button.tsx`). The sentence is still the server's everywhere else.
 *
 * NO LOG ANYWHERE HERE: the file holds the commune's budget as the finance office wrote it.
 */

import {
  CHUNG,
  errorMessageOr,
  LOI_KHONG_RO,
  stripTechnicalPrefix,
} from "./goi";
import { fallbackFor, IMPORT_FILE_FIELD } from "./excel-import";
import type {
  finance_budgetImportIssueOut,
  finance_budgetImportOut,
  finance_budgetImportRejectedOut,
  finance_post_budget_sheets_imports,
} from "./schema.gen";

export type BudgetImportOut = finance_budgetImportOut;
export type BudgetImportIssue = finance_budgetImportIssueOut;

export const BUDGET_IMPORTS =
  "/api/v1/budget-sheets/imports" satisfies finance_post_budget_sheets_imports["duongDan"];

/** `?year=` — the query word the route declares (`truyVan.year`). */
export function budgetImportPath(year: number): string {
  const name: keyof finance_post_budget_sheets_imports["truyVan"] = "year";
  return `${BUDGET_IMPORTS}?${new URLSearchParams({ [name]: String(year) }).toString()}`;
}

/** A 201 body with its sheets, or `null` (a replay `{ code, replayed: true }`, or an unreadable body). */
function readOut(body: unknown): BudgetImportOut | null {
  if (typeof body !== "object" || body === null) return null;
  const b = body as Partial<BudgetImportOut>;
  if (!Array.isArray(b.sheets) || typeof b.year !== "number") return null;
  return {
    valid: b.valid === true,
    year: b.year,
    source_file: typeof b.source_file === "string" ? b.source_file : "",
    sheets: b.sheets,
    warnings: Array.isArray(b.warnings) ? b.warnings : [],
    errors: Array.isArray(b.errors) ? b.errors : [],
  };
}

/** One import attempt. */
export type BudgetImportResult =
  | {
      readonly ok: true;
      /** The sheets created, or `null` when the server replayed an earlier success of the same key. */
      readonly out: BudgetImportOut | null;
    }
  | {
      readonly ok: false;
      /** The server's sentence (technical prefix removed), or the status fallback. */
      readonly message: string;
      /** The server's machine code when its body had one — see the header for the one use. */
      readonly code?: string;
      /** Non-empty only on 400 `import_invalid`: every error, nothing was written. */
      readonly errors: readonly BudgetImportIssue[];
    };

/**
 * POST the file — every sheet in one transaction, or nothing. `idempotencyKey` is minted by the caller
 * for each pick: a double click on one pick is one import, never a second revision of the same sheet.
 */
export async function commitBudgetImport(
  file: Blob,
  fileName: string,
  year: number,
  idempotencyKey: string,
): Promise<BudgetImportResult> {
  const form = new FormData();
  form.append(IMPORT_FILE_FIELD, file, fileName);
  let res: Response;
  try {
    res = await fetch(budgetImportPath(year), {
      ...CHUNG,
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body: form,
    });
  } catch {
    return { ok: false, message: LOI_KHONG_RO, errors: [] };
  }

  if (res.status === 201) {
    try {
      return { ok: true, out: readOut(await res.json()) };
    } catch {
      // 201 is the fact; an unreadable body does not undo the import.
      return { ok: true, out: null };
    }
  }

  const message = await errorMessageOr(res.clone(), fallbackFor(res.status));
  try {
    const body = (await res.json()) as Partial<finance_budgetImportRejectedOut>;
    return {
      ok: false,
      message:
        typeof body.message === "string" && body.message !== ""
          ? stripTechnicalPrefix(body.message)
          : message,
      code:
        typeof body.code === "string" && body.code !== ""
          ? body.code
          : undefined,
      errors: Array.isArray(body.errors) ? body.errors : [],
    };
  } catch {
    return { ok: false, message, errors: [] };
  }
}
