/**
 * `Nhập giải ngân` — the Excel import of disbursement vouchers (`docs/ui-ux/06-giai-ngan.md` §10).
 * Owner: `service-finance` (`routes_disbursement_import.go`):
 *
 *   GET  /api/v1/disbursements/import-template   budget.read    the .xlsx to fill in
 *   POST /api/v1/disbursements/import-previews   budget.update  200 { valid, row_count, total_amount, errors }
 *   POST /api/v1/disbursements/imports           budget.update  201 { …, batch, created } · 400 import_invalid
 *
 * The wire rules (one multipart part `file`, no hand-set `Content-Type`, the caller's key, a replayed
 * 201 is still a success, a proxy's 413/415 is not "no connection") are `excel-import.ts`'s; the
 * template and the import go through it unchanged.
 *
 * WHY THE PREVIEW HAS ITS OWN CALL: every catalogue preview lists the rows it would create, and
 * `previewImport` normalises that list. This preview lists NONE — it answers how many vouchers and how
 * much money (`row_count`, `total_amount`), which is what an accountant checks against the paper
 * ledger before writing. Forcing it through `previewImport` would drop exactly those two numbers.
 */

import { CHUNG, errorMessageOr, LOI_KHONG_RO } from "./goi";
import type { KetQua } from "./goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import { commitImport, downloadImportTemplate, fallbackFor, IMPORT_FILE_FIELD } from "./excel-import";
import type { ImportError, ImportResult, ImportRoutes } from "./excel-import";
import type {
  finance_disbursementImportCreatedRowOut,
  finance_disbursementImportPreviewOut,
  finance_get_disbursements_import_template,
  finance_post_disbursements_import_previews,
  finance_post_disbursements_imports,
} from "./schema.gen";

export type DisbursementImportCreatedRow = finance_disbursementImportCreatedRowOut;

export const DISBURSEMENT_IMPORT_ROUTES: ImportRoutes = {
  template:
    "/api/v1/disbursements/import-template" satisfies finance_get_disbursements_import_template["duongDan"],
  previews:
    "/api/v1/disbursements/import-previews" satisfies finance_post_disbursements_import_previews["duongDan"],
  imports: "/api/v1/disbursements/imports" satisfies finance_post_disbursements_imports["duongDan"],
};

/** Saved name of the template — the server's own `disbursementImportFilename`. */
export const DISBURSEMENT_TEMPLATE_FILE_NAME = "mau-nhap-giai-ngan.xlsx";

/** The preview, as the screen reads it. Amounts in full đồng, never rounded here. */
export type DisbursementImportPreview = {
  readonly valid: boolean;
  readonly rowCount: number;
  readonly totalAmount: number;
  readonly errors: readonly ImportError[];
};

export function downloadDisbursementImportTemplate(): Promise<KetQua<Blob>> {
  return downloadImportTemplate(DISBURSEMENT_IMPORT_ROUTES);
}

/**
 * POST a preview. 200 WHETHER OR NOT THE FILE IS VALID — `valid: false` with `errors` is the answer.
 * A 4xx is a refusal of the FILE (not xlsx, too big): the server's sentence, verbatim, or the shared
 * 413/415 fallback when a proxy answered with a page of its own. Writes nothing; no `Idempotency-Key`.
 */
export async function previewDisbursementImport(
  file: Blob,
  fileName: string,
): Promise<KetQua<DisbursementImportPreview>> {
  const form = new FormData();
  form.append(IMPORT_FILE_FIELD, file, fileName);
  let res: Response;
  try {
    res = await fetch(DISBURSEMENT_IMPORT_ROUTES.previews, { ...CHUNG, method: "POST", body: form });
  } catch {
    // No log: the file holds the commune's vouchers as staff typed them.
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 200) return { ok: false, thongBao: await errorMessageOr(res, fallbackFor(res.status)) };
  try {
    const body = (await res.json()) as Partial<finance_disbursementImportPreviewOut>;
    const rowCount = body.row_count;
    const totalAmount = body.total_amount;
    // A 200 without both numbers is a broken contract, not "0 vouchers, 0 đ" — offering `Nhập` on a
    // summary nobody read would be the screen inventing it.
    if (typeof rowCount !== "number" || typeof totalAmount !== "number") return { ok: false, thongBao: LOI_KHONG_RO };
    return {
      ok: true,
      duLieu: {
        valid: body.valid === true,
        rowCount,
        totalAmount,
        errors: Array.isArray(body.errors) ? body.errors : [],
      },
    };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/**
 * POST the import — the whole file or nothing. `idempotencyKey` belongs to ONE attempt at ONE chosen
 * file and is reused on retry (`excel-import.ts`, `commitImport`). `created` is `null` on a replay.
 */
export function commitDisbursementImport(
  file: Blob,
  fileName: string,
  idempotencyKey: string,
): Promise<ImportResult<DisbursementImportCreatedRow>> {
  return commitImport<DisbursementImportCreatedRow>(DISBURSEMENT_IMPORT_ROUTES, file, fileName, idempotencyKey);
}
