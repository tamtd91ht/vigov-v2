/**
 * "⬆ Nhập từ Excel" — the ONE client for every import of the configuration screen (ADR 0059). Each
 * import is three routes of the service that owns the rows, same shape everywhere:
 *
 *   GET  …/import-template  the .xlsx to fill in
 *   POST …/import-previews  check a filled file — WRITES NOTHING, 200 whether or not it is valid
 *   POST …/imports          write it, all or nothing (`Idempotency-Key`), 201 · 400 `import_invalid`
 *
 * ONE CLIENT, NOT ONE PER IMPORT: the org-unit import was the first and its rules — one multipart part
 * named `file`, no hand-set `Content-Type`, the caller's key, a replayed 201 is still a success, a
 * proxy's 413 is not "no connection" — are the same for every owner. A second copy of them is the copy
 * that forgets one. What differs per import is only the PATHS and the NAME of the row list in the
 * preview body (`units` for org units, `types` for map-asset types): both are parameters here.
 *
 * THE UPLOAD IS `multipart/form-data`, ONE PART NAMED `file`. The contract declares NO request body for
 * either POST — the part name is taken from the handlers (`importFormField` in identity,
 * `xlsxFormField` in comms). A contract gap, reported, not worked around.
 *
 * NO `tenant_id`, RELATIVE PATHS, `credentials: same-origin`: the three rules of `goi.ts`.
 */

import { CHUNG, errorMessageOr, LOI_KHONG_RO } from "./request";
import type { KetQua } from "./request";

/** The three paths of one import. Each caller writes them with `satisfies <generated>["duongDan"]`. */
export type ImportRoutes = {
  readonly template: string;
  readonly previews: string;
  readonly imports: string;
};

/**
 * One refused row. Every owner answers this shape (`orgUnitImportErrorOut`,
 * `mapAssetTypeImportErrorOut`): `row: 0` is the whole file, `column: ""` the whole row.
 */
export type ImportError = { readonly row: number; readonly column: string; readonly message: string };

/** A preview, normalised: the owner's row list is read from its own field name into `rows`. */
export type ImportPreview<R> = {
  readonly valid: boolean;
  readonly rows: readonly R[];
  readonly errors: readonly ImportError[];
};

/** One import attempt. */
export type ImportResult<R> =
  | {
      readonly ok: true;
      /**
       * The rows created — or `null` when the server REPLAYED an earlier success of the same key
       * (`core/idem`: a replay carries `{ replayed: true }`, never the first body). Either way the file
       * is in; the screen re-reads rather than trusting a list it did not get.
       */
      readonly created: readonly R[] | null;
    }
  | {
      readonly ok: false;
      readonly message: string;
      /** Non-empty only on 400 `import_invalid`: every row the server refused, all or nothing. */
      readonly errors: readonly ImportError[];
    };

/** Part name the handlers read. See the header: not in the contract. */
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

function formWith(file: Blob, fileName: string): FormData {
  const form = new FormData();
  form.append(IMPORT_FILE_FIELD, file, fileName);
  return form;
}

/**
 * GET the template. Some templates list this commune's live rows as choices (the org chart's parent
 * column), so it is fetched fresh each time (`no-store` in `CHUNG`) and never kept.
 */
export async function downloadImportTemplate(routes: ImportRoutes): Promise<KetQua<Blob>> {
  let res: Response;
  try {
    res = await fetch(routes.template, { ...CHUNG, method: "GET" });
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

/**
 * POST a preview. 200 WHETHER OR NOT THE FILE IS VALID: `valid: false` plus `errors` IS the answer to
 * "what is wrong with my file". A 4xx here is a refusal of the FILE itself (not xlsx, too big,
 * macro-enabled), and comes back as one sentence.
 *
 * `rowsField` names the owner's row list in the body. The raw body is returned too, so a caller whose
 * existing type names that field (`identity_orgUnitImportPreviewOut.units`) keeps it.
 *
 * No `Idempotency-Key`: the preview writes nothing, and the contract declares none.
 */
export async function previewImport<R>(
  routes: ImportRoutes,
  rowsField: string,
  file: Blob,
  fileName: string,
): Promise<KetQua<ImportPreview<R>>> {
  let res: Response;
  try {
    res = await fetch(routes.previews, { ...CHUNG, method: "POST", body: formWith(file, fileName) });
  } catch {
    // No log: the file holds the commune's data as staff typed it.
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 200) return { ok: false, thongBao: await errorMessageOr(res, fallbackFor(res.status)) };
  try {
    const body = (await res.json()) as Record<string, unknown>;
    const rows = body[rowsField];
    return {
      ok: true,
      duLieu: {
        valid: body.valid === true,
        rows: Array.isArray(rows) ? (rows as R[]) : [],
        errors: Array.isArray(body.errors) ? (body.errors as ImportError[]) : [],
      },
    };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/**
 * POST the import — the whole file or nothing.
 *
 * `idempotencyKey` IS A PARAMETER, NOT MINTED HERE: the key belongs to ONE attempt at importing ONE
 * previewed file, and a retry after a network error must carry the same key — the first send may have
 * reached the server, and a new key would import the file twice. The caller mints it when the attempt
 * starts and keeps it until that attempt succeeds (`features/cau-hinh/excel-import-flow.ts`).
 *
 * A 409 (the org chart's `org_chart_changed`: someone changed the rows between preview and import) is
 * a sentence only; nothing was written, and a new preview shows the new state.
 */
export async function commitImport<R>(
  routes: ImportRoutes,
  file: Blob,
  fileName: string,
  idempotencyKey: string,
): Promise<ImportResult<R>> {
  let res: Response;
  try {
    res = await fetch(routes.imports, {
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
      const body = (await res.json()) as { created?: unknown };
      return { ok: true, created: Array.isArray(body.created) ? (body.created as R[]) : null };
    } catch {
      // 201 is the fact; an unreadable body does not undo the import.
      return { ok: true, created: null };
    }
  }

  if (res.status === 400) {
    try {
      const body = (await res.json()) as { message?: unknown; errors?: unknown };
      return {
        ok: false,
        message: typeof body.message === "string" && body.message !== "" ? body.message : LOI_KHONG_RO,
        errors: Array.isArray(body.errors) ? (body.errors as ImportError[]) : [],
      };
    } catch {
      return { ok: false, message: LOI_KHONG_RO, errors: [] };
    }
  }

  return { ok: false, message: await errorMessageOr(res, fallbackFor(res.status)), errors: [] };
}
