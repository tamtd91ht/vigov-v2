/**
 * "Nhập từ Excel" of the org chart (`docs/ui-ux/14-cau-hinh.md §1`) — three routes, all `admin.org`:
 *
 *   GET  /api/v1/org-units/import-template  the .xlsx to fill in
 *   POST /api/v1/org-units/import-previews  check a filled file — WRITES NOTHING
 *   POST /api/v1/org-units/imports          write it, all or nothing (`Idempotency-Key`)
 *
 * THE UPLOAD IS `multipart/form-data`, ONE PART NAMED `file`. The contract (`openapi.json`) declares
 * NO request body for either POST — the shape is taken from the handler
 * (`service-identity/internal/http/org_unit_import.go`, `importFormField`). That is a contract gap,
 * reported, not worked around: the day the contract declares it, the part name should come from it.
 *
 * `Content-Type` IS NOT SET BY HAND: the browser writes it WITH the multipart boundary. A hand-set
 * `multipart/form-data` without the boundary is a body the server cannot split (415/400).
 *
 * NO `tenant_id`, RELATIVE PATHS, `credentials: same-origin`: the three rules of `goi.ts`.
 */

import { CHUNG, errorMessageOr, LOI_KHONG_RO } from "./goi";
import type { KetQua } from "./goi";
import type {
  identity_get_org_units_import_template,
  identity_orgUnitImportCreatedOut,
  identity_orgUnitImportErrorOut,
  identity_orgUnitImportPreviewOut,
  identity_orgUnitImportRejectedOut,
  identity_orgUnitImportUnitOut,
  identity_post_org_units_import_previews,
  identity_post_org_units_imports,
} from "./schema.gen";

const TEMPLATE_PATH =
  "/api/v1/org-units/import-template" satisfies identity_get_org_units_import_template["duongDan"];
const PREVIEW_PATH =
  "/api/v1/org-units/import-previews" satisfies identity_post_org_units_import_previews["duongDan"];
const IMPORT_PATH = "/api/v1/org-units/imports" satisfies identity_post_org_units_imports["duongDan"];

/** Part name the handler reads. See the header: not in the contract. */
export const IMPORT_FILE_FIELD = "file";

/**
 * Sentences for the two statuses a PROXY also answers with a page of its own (the ingress body cap
 * gives 413 before the service sees a byte). The server's own sentence wins whenever it wrote one.
 */
export const FILE_TOO_LARGE_FALLBACK =
  "Tệp quá lớn nên hệ thống chưa nhận. Hãy chia tệp thành nhiều phần nhỏ hơn rồi nhập lần lượt.";
export const FILE_TYPE_FALLBACK =
  "Chỉ nhận tệp Excel .xlsx. Hãy tải tệp mẫu, nhập vào đó rồi lưu lại dưới dạng .xlsx.";

/** A refusal by status: 413 / 415 get the fallbacks above, everything else `LOI_KHONG_RO`. */
function fallbackFor(status: number): string {
  if (status === 413) return FILE_TOO_LARGE_FALLBACK;
  if (status === 415) return FILE_TYPE_FALLBACK;
  return LOI_KHONG_RO;
}

/**
 * GET the template. The file lists this commune's live units as parent choices, so it is fetched
 * fresh each time (`no-store` in `CHUNG`) and never kept.
 */
export async function downloadOrgUnitTemplate(): Promise<KetQua<Blob>> {
  let res: Response;
  try {
    res = await fetch(TEMPLATE_PATH, { ...CHUNG, method: "GET" });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 200) return { ok: false, thongBao: await errorMessageOr(res, LOI_KHONG_RO) };
  try {
    return { ok: true, duLieu: await res.blob() };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

function formWith(file: Blob, fileName: string): FormData {
  const form = new FormData();
  form.append(IMPORT_FILE_FIELD, file, fileName);
  return form;
}

/**
 * POST a preview. 200 WHETHER OR NOT THE FILE IS VALID: `valid: false` plus `errors` IS the answer
 * to "what is wrong with my file". A 4xx here is a refusal of the FILE itself (not xlsx, too big,
 * macro-enabled), and comes back as one sentence.
 *
 * No `Idempotency-Key`: the preview writes nothing, and the contract declares none.
 */
export async function previewOrgUnitImport(
  file: Blob,
  fileName: string,
): Promise<KetQua<identity_orgUnitImportPreviewOut>> {
  let res: Response;
  try {
    res = await fetch(PREVIEW_PATH, { ...CHUNG, method: "POST", body: formWith(file, fileName) });
  } catch {
    // No log: the file holds the commune's org chart as staff typed it.
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 200) return { ok: false, thongBao: await errorMessageOr(res, fallbackFor(res.status)) };
  try {
    const body = (await res.json()) as identity_orgUnitImportPreviewOut;
    return {
      ok: true,
      duLieu: {
        valid: body.valid === true,
        units: Array.isArray(body.units) ? body.units : [],
        errors: Array.isArray(body.errors) ? body.errors : [],
      },
    };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/** One import attempt. */
export type OrgUnitImportResult =
  | {
      readonly ok: true;
      /**
       * The units created — or `null` when the server REPLAYED an earlier success of the same key
       * (`core/idem`: a replay carries `{ replayed: true }`, never the first body). Either way the
       * file is in; the screen reloads the tree rather than trusting a list it did not get.
       */
      readonly created: readonly identity_orgUnitImportUnitOut[] | null;
    }
  | {
      readonly ok: false;
      readonly message: string;
      /** Non-empty only on 400 `import_invalid`: every row the server refused, all or nothing. */
      readonly errors: readonly identity_orgUnitImportErrorOut[];
    };

/**
 * POST the import — the whole file or nothing.
 *
 * `idempotencyKey` IS A PARAMETER, NOT MINTED HERE: the key belongs to ONE attempt at importing ONE
 * previewed file, and a retry after a network error must carry the same key — the first send may
 * have reached the server, and a new key would import the file twice. The caller mints it when the
 * preview is accepted and keeps it until that attempt succeeds (`features/cau-hinh/org-unit-import.ts`).
 *
 * 409 `org_chart_changed` (someone changed the chart between preview and import) is a sentence
 * only; nothing was written, and a new preview shows the new state.
 */
export async function importOrgUnits(
  file: Blob,
  fileName: string,
  idempotencyKey: string,
): Promise<OrgUnitImportResult> {
  let res: Response;
  try {
    res = await fetch(IMPORT_PATH, {
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
      const body = (await res.json()) as Partial<identity_orgUnitImportCreatedOut>;
      return { ok: true, created: Array.isArray(body.created) ? body.created : null };
    } catch {
      // 201 is the fact; an unreadable body does not undo the import.
      return { ok: true, created: null };
    }
  }

  if (res.status === 400) {
    try {
      const body = (await res.json()) as Partial<identity_orgUnitImportRejectedOut>;
      return {
        ok: false,
        message: typeof body.message === "string" && body.message !== "" ? body.message : LOI_KHONG_RO,
        errors: Array.isArray(body.errors) ? body.errors : [],
      };
    } catch {
      return { ok: false, message: LOI_KHONG_RO, errors: [] };
    }
  }

  return { ok: false, message: await errorMessageOr(res, fallbackFor(res.status)), errors: [] };
}
