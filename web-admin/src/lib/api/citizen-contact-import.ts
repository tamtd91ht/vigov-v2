/**
 * "Nhập từ Excel" of the citizen directory — `service-identity` (`internal/http/citizen_directory_import.go`,
 * ADR 0097 D3/D13, chapter 16 §7). Three routes, all `citizen.update`:
 *
 *   GET  /api/v1/citizen-contact-import-template   the .xlsx to fill in — seven columns, one example row with a FAKE number
 *   POST /api/v1/citizen-contact-import-previews   200 { valid, row_count, errors } — checks, WRITES NOTHING
 *   POST /api/v1/citizen-contact-imports           201 { batch_id, row_count } — all or nothing (Idempotency-Key) · 400 import_invalid
 *
 * THE WIRE RULES ARE `excel-import.ts`'s (one multipart part `file`, no hand-set `Content-Type`, the
 * caller's key, 413/415 fallbacks); the template goes through it unchanged. THE TWO POSTS HAVE THEIR OWN
 * CALLS, for the reason `disbursement-import.ts` gives: this preview lists NO rows — it answers how many
 * (`row_count`), which `previewImport` would drop — and the 201 carries `row_count`, not a `created`
 * list, which `commitImport` would read as a replay.
 *
 * The file is NOT kept by the server (D13) and is never logged here: it is the commune's directory as
 * staff typed it (rule 3). Preview errors name a row and a column, never a value.
 */

import { CHUNG, errorMessageOr, LOI_KHONG_RO, stripTechnicalPrefix } from "./goi";
import type { KetQua } from "./goi"; // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
import { downloadImportTemplate, fallbackFor, IMPORT_FILE_FIELD } from "./excel-import";
import type { ImportError, ImportRoutes } from "./excel-import";
import type {
  identity_citizenContactImportCreatedOut,
  identity_citizenContactImportPreviewOut,
  identity_citizenContactImportRejectedOut,
  identity_get_citizen_contact_import_template,
  identity_post_citizen_contact_import_previews,
  identity_post_citizen_contact_imports,
} from "./schema.gen";

export const CITIZEN_CONTACT_IMPORT_ROUTES: ImportRoutes = {
  template: "/api/v1/citizen-contact-import-template" satisfies identity_get_citizen_contact_import_template["duongDan"],
  previews: "/api/v1/citizen-contact-import-previews" satisfies identity_post_citizen_contact_import_previews["duongDan"],
  imports: "/api/v1/citizen-contact-imports" satisfies identity_post_citizen_contact_imports["duongDan"],
};

/** The template's saved name — the server's own (`citizen_directory_import.go`, `Content-Disposition`). */
export const CITIZEN_CONTACT_TEMPLATE_FILE_NAME = "mau-nhap-danh-ba-nguoi-dan.xlsx";

/** The preview, as the dialog reads it: a verdict, a row count, the refused cells. */
export type CitizenContactImportPreview = {
  readonly valid: boolean;
  readonly rowCount: number;
  readonly errors: readonly ImportError[];
};

/**
 * One import attempt. `rowCount` is `null` when the server REPLAYED an earlier success of the same key
 * (`core/idem` answers `{ replayed: true }`, never the first body): the file is in, the count unknown.
 */
export type CitizenContactImportResult =
  | { readonly ok: true; readonly rowCount: number | null }
  | { readonly ok: false; readonly message: string; readonly errors: readonly ImportError[] };

function formWith(file: Blob, fileName: string): FormData {
  const form = new FormData();
  form.append(IMPORT_FILE_FIELD, file, fileName);
  return form;
}

/** GET the template. Fetched fresh each time: its hamlet dropdown lists the commune's live units of this moment. */
export function downloadCitizenContactImportTemplate(): Promise<KetQua<Blob>> {
  return downloadImportTemplate(CITIZEN_CONTACT_IMPORT_ROUTES);
}

/**
 * POST a preview. 200 WHETHER OR NOT THE FILE IS VALID — `valid: false` plus `errors` IS the answer. A
 * 4xx is a refusal of the FILE (not xlsx, too big): the server's sentence, or the shared 413/415
 * fallback when a proxy answered with a page of its own. Writes nothing; no `Idempotency-Key`.
 */
export async function previewCitizenContactImport(file: Blob, fileName: string): Promise<KetQua<CitizenContactImportPreview>> {
  let res: Response;
  try {
    res = await fetch(CITIZEN_CONTACT_IMPORT_ROUTES.previews, { ...CHUNG, method: "POST", body: formWith(file, fileName) });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 200) return { ok: false, thongBao: await errorMessageOr(res, fallbackFor(res.status)) };
  try {
    const body = (await res.json()) as Partial<identity_citizenContactImportPreviewOut>;
    // A 200 without the count is a broken contract, not "0 rows": offering `Nhập` on a figure nobody
    // sent would be the screen inventing it.
    if (typeof body.row_count !== "number") return { ok: false, thongBao: LOI_KHONG_RO };
    return {
      ok: true,
      duLieu: { valid: body.valid === true, rowCount: body.row_count, errors: Array.isArray(body.errors) ? body.errors : [] },
    };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/**
 * POST the import — the whole file or nothing. `idempotencyKey` belongs to ONE attempt at ONE previewed
 * file and is reused on a retry (the first send may have reached the server; a new key would import the
 * file twice). 400 `import_invalid` returns every refused row; 409 `citizen_contacts_changed` (the
 * directory moved between preview and import) is a sentence only — a new preview shows the new state.
 */
export async function commitCitizenContactImport(file: Blob, fileName: string, idempotencyKey: string): Promise<CitizenContactImportResult> {
  let res: Response;
  try {
    res = await fetch(CITIZEN_CONTACT_IMPORT_ROUTES.imports, {
      ...CHUNG,
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body: formWith(file, fileName),
    });
  } catch {
    return { ok: false, message: LOI_KHONG_RO, errors: [] };
  }

  if (res.status === 201) {
    try {
      const body = (await res.json()) as Partial<identity_citizenContactImportCreatedOut>;
      return { ok: true, rowCount: typeof body.row_count === "number" ? body.row_count : null };
    } catch {
      // 201 is the fact; an unreadable body does not undo the import.
      return { ok: true, rowCount: null };
    }
  }

  if (res.status === 400) {
    try {
      const body = (await res.json()) as Partial<identity_citizenContactImportRejectedOut>;
      return {
        ok: false,
        message: typeof body.message === "string" && body.message !== "" ? stripTechnicalPrefix(body.message) : LOI_KHONG_RO,
        errors: Array.isArray(body.errors) ? body.errors : [],
      };
    } catch {
      return { ok: false, message: LOI_KHONG_RO, errors: [] };
    }
  }

  return { ok: false, message: await errorMessageOr(res, fallbackFor(res.status)), errors: [] };
}
