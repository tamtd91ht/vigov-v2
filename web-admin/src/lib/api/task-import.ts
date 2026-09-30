/**
 * `⬆ Nhập từ Excel` of the task register (`docs/ui-ux/02-nhiem-vu.md` §8, backend 816ef81) — two
 * routes, both `task.create` (the import IS task creation, row by row):
 *
 *   GET  /api/v1/tasks/import-template   the empty template with one invented example row
 *   POST /api/v1/tasks/imports           multipart `file`; `?dry_run=true` checks and writes nothing
 *
 * ALL OR NOTHING: every row is checked first; ONE refused row and nothing is written (200,
 * `committed: false`, the row report — row, column, sentence, never the cell's value). All rows pass
 * → one transaction books every task with an auto-issued register number (201, `codes`).
 *
 * THE UPLOAD IS `multipart/form-data`, ONE PART NAMED `file` — from the handler
 * (`service-petitions/internal/http/task_import.go`); the contract declares no request body for it.
 * `Content-Type` is NOT set by hand: the browser writes it WITH the boundary.
 *
 * NO `tenant_id`, RELATIVE PATHS, `credentials: same-origin` — `goi.ts`'s three rules, via `CHUNG`.
 */

import {
  CHUNG, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  LOI_KHONG_RO, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  thongBaoLoi, // vi-name-ok: existing export of goi.ts, imported not declared (rule 12 inv 3)
  type KetQua, // vi-name-ok: existing type of goi.ts, imported not declared (rule 12 inv 3)
} from "./goi";
import type {
  petitions_get_tasks_import_template,
  petitions_post_tasks_imports,
  petitions_taskImportResultOut,
} from "./schema.gen";

const TEMPLATE_PATH: petitions_get_tasks_import_template["duongDan"] = "/api/v1/tasks/import-template";
const IMPORT_PATH: petitions_post_tasks_imports["duongDan"] = "/api/v1/tasks/imports";

/** The part name the handler reads (`task_import.go`, `readImportUpload`). Not in the contract. */
export const TASK_IMPORT_FILE_FIELD = "file";

/** Saved name of the template, when the reply names none — the server's own `taskImportTemplateName`. */
export const TASK_TEMPLATE_FILE_NAME = "mau-nhap-nhiem-vu.xlsx";

/**
 * 413 is also what the INGRESS answers, with an HTML page, before the service sees a byte. Then the
 * server wrote no sentence, and "Không kết nối được máy chủ" would be wrong: the connection worked and
 * the body was refused. The server's own sentence still wins whenever it wrote one.
 */
export const TASK_IMPORT_TOO_LARGE =
  "Tệp quá lớn (tối đa 2 MB) nên hệ thống chưa nhận. Hãy chia thành nhiều tệp nhỏ hơn rồi nhập lần lượt.";

async function refusal(res: Response): Promise<string> {
  const m = await thongBaoLoi(res);
  return m === LOI_KHONG_RO && res.status === 413 ? TASK_IMPORT_TOO_LARGE : m;
}

/** GET the template. `no-store` (`CHUNG`): fetched fresh, never kept. */
export async function downloadTaskImportTemplate(): Promise<KetQua<Blob>> {
  let res: Response;
  try {
    res = await fetch(TEMPLATE_PATH, { ...CHUNG, method: "GET" });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 200) return { ok: false, thongBao: await refusal(res) };
  try {
    return { ok: true, duLieu: await res.blob() };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/** What one send answered. */
export type TaskImportOutcome =
  | {
      /** 200 — a dry run, or a refused file: NOTHING was written. */
      readonly kind: "checked";
      readonly report: petitions_taskImportResultOut;
    }
  | {
      /** 201 — every row booked. `codes` in row order. */
      readonly kind: "imported";
      readonly report: petitions_taskImportResultOut;
    }
  | {
      /**
       * 201 REPLAYED (`Idempotent-Replay: true`): this key already imported the file — the first
       * send reached the server even if its answer was lost. The replay carries only the FIRST issued
       * number (`core/idem` stores a code, never a body). Nothing new was written.
       */
      readonly kind: "replayed";
      readonly firstCode: string;
    };

/** Coerce the report: missing arrays become empty, so a render never meets `undefined`. */
function readReport(body: Partial<petitions_taskImportResultOut>): petitions_taskImportResultOut {
  return {
    total_rows: typeof body.total_rows === "number" ? body.total_rows : 0,
    created: typeof body.created === "number" ? body.created : 0,
    committed: body.committed === true,
    errors: Array.isArray(body.errors) ? body.errors : [],
    codes: Array.isArray(body.codes) ? body.codes : [],
  };
}

/**
 * POST the file. `dryRun` checks everything — identity included — and writes nothing.
 *
 * `idempotencyKey` IS A PARAMETER: the route declares `idem.Required` on BOTH modes. Two consequences
 * the caller owns (`features/nhiem-vu/task-import.ts`):
 *   - a CHECK uses a fresh key every time. The key store records a finished answer; a real import sent
 *     later under a check's key would be told the check's answer and import nothing;
 *   - an IMPORT keeps its key across retries of the same file: a retry after a lost answer must
 *     replay, never book the whole file a second time — issued numbers are never reissued (rule 7).
 *
 * The body is never logged: the file is the commune's work as staff typed it.
 */
export async function submitTaskImport(
  file: Blob,
  fileName: string,
  idempotencyKey: string,
  dryRun: boolean,
): Promise<KetQua<TaskImportOutcome>> {
  const form = new FormData();
  form.append(TASK_IMPORT_FILE_FIELD, file, fileName);
  const path = dryRun ? `${IMPORT_PATH}?dry_run=true` : IMPORT_PATH;
  let res: Response;
  try {
    res = await fetch(path, {
      ...CHUNG,
      method: "POST",
      headers: { "Idempotency-Key": idempotencyKey },
      body: form,
    });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }

  if (res.status !== 200 && res.status !== 201) return { ok: false, thongBao: await refusal(res) };

  let body: Record<string, unknown>;
  try {
    body = (await res.json()) as Record<string, unknown>;
  } catch {
    // A 201 is the fact even with an unreadable body: the file is in. Say so; the register reloads.
    return res.status === 201
      ? { ok: true, duLieu: { kind: "replayed", firstCode: "" } }
      : { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status === 201 && (res.headers.get("Idempotent-Replay") === "true" || body.replayed === true)) {
    return {
      ok: true,
      duLieu: { kind: "replayed", firstCode: typeof body.code === "string" ? body.code : "" },
    };
  }
  const report = readReport(body as Partial<petitions_taskImportResultOut>);
  return { ok: true, duLieu: { kind: res.status === 201 ? "imported" : "checked", report } };
}
