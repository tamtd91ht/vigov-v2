/**
 * The two capital-plan category writes whose shape DIFFERS from the shared catalogue table
 * (`danh-muc.ts`): add by label only, delete with no reason (backend e9f669f1, user decision
 * 07/10/2026, prototype `CategoryManagerDialog.tsx:64-80, 170-182`).
 *
 * WHY NOT `themMuc` / `xoaMuc`: those take the INTERSECTION of seven services' bodies, where `code` and
 * `reason` are still required by the six other catalogues — and the Cấu hình tab must keep sending them.
 * Loosening the intersection would let a Cấu hình call drop a field its own route refuses. These two
 * functions are typed against `finance_*` alone, so they can only ever reach this catalogue's routes.
 * Rename and turn off/on keep going through `suaMuc`: their shape did not change.
 */

import { CAPITAL_PLAN_CATEGORY_WRITES } from "./danh-muc";
import { CHUNG, LOI_CHUA_HO_TRO, LOI_KHONG_RO, stripTechnicalPrefix } from "./goi";
import type { KetQua } from "./goi"; // vi-name-ok: existing result type of goi.ts (rule 12 invariant 3)
import type {
  finance_delete_capital_plan_categories_by_id,
  finance_hangMucRa,
  finance_post_capital_plan_categories,
  finance_themHangMucVao,
  httpx_Error,
} from "./schema.gen";

type AddConflictCode = finance_post_capital_plan_categories["errorCodes"][409];

/**
 * The 409s of an add, re-worded for a form that has NO code box. WHY A CODE IS READ when `goi.ts` says
 * never to branch on `code`: the server's sentences were written for a caller who types the code
 * ("Hãy nhập mã riêng…", "Hãy chọn một mã khác"), and `catalogue_full` names the document-type
 * catalogue (a shared handler). Shown verbatim here, each would tell the officer to fix a box that does
 * not exist. The one thing the officer CAN change is the label, so every sentence says that. The keys
 * are typed by the contract: a code renamed or removed there turns `tsc` red on this line.
 */
const ADD_CONFLICT_SENTENCES: Readonly<Record<AddConflictCode, string>> = {
  code_series_blocked: "Tên này đã dùng quá nhiều lần, hãy đặt tên khác.",
  code_taken: "Tên này trùng với một hạng mục đã có hoặc đã xoá, hãy đặt tên khác.",
  catalogue_full:
    "Danh mục hạng mục kế hoạch vốn đã đủ số mục tối đa. Hãy tắt hoặc xoá bớt hạng mục không dùng.",
};

function itemPath(id: string): string {
  return CAPITAL_PLAN_CATEGORY_WRITES.mauMuc.replace("{id}", encodeURIComponent(id));
}

/** The refusal sentence: a known 409 of the add re-worded, anything else the server's own words. */
async function refusal(res: Response, conflicts: Readonly<Record<string, string>>): Promise<string> {
  const fallback = res.status === 404 ? LOI_CHUA_HO_TRO : LOI_KHONG_RO;
  try {
    const body = (await res.json()) as httpx_Error;
    // Own keys only: a code such as "constructor" must not pick up a prototype member.
    if (res.status === 409 && typeof body?.code === "string" && Object.prototype.hasOwnProperty.call(conflicts, body.code)) {
      return conflicts[body.code]!;
    }
    return typeof body?.message === "string" && body.message !== "" ? stripTechnicalPrefix(body.message) : fallback;
  } catch {
    return fallback;
  }
}

/**
 * POST — add one category the commune owns, by LABEL. No `code`: the server derives it from the label
 * and never reissues one already used (rule 7, invariant 3). `idempotencyKey` is the form's, reused on a
 * retry, for the same reason as `themMuc`'s: the first send may have reached the server.
 */
export async function addCapitalPlanCategory(
  label: string,
  order: number,
  idempotencyKey: string,
): Promise<KetQua<finance_hangMucRa>> {
  // Built field by field: `source`/`tier` are the server's, and `code` is deliberately absent.
  const body: finance_themHangMucVao = { label, order };
  let res: Response;
  try {
    res = await fetch(CAPITAL_PLAN_CATEGORY_WRITES.gocThem, {
      ...CHUNG,
      method: "POST" satisfies finance_post_capital_plan_categories["phuongThuc"],
      headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey },
      body: JSON.stringify(body),
    });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 201) return { ok: false, thongBao: await refusal(res, ADD_CONFLICT_SENTENCES) };
  try {
    return { ok: true, duLieu: (await res.json()) as finance_hangMucRa };
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}

/**
 * DELETE — soft delete, NO BODY. The server then writes its fixed default reason, so the
 * `delete_reason` column is never empty (rule 7, invariant 1); the code stays taken forever.
 */
export async function deleteCapitalPlanCategory(id: string): Promise<KetQua<null>> {
  let res: Response;
  try {
    res = await fetch(itemPath(id), {
      ...CHUNG,
      method: "DELETE" satisfies finance_delete_capital_plan_categories_by_id["phuongThuc"],
    });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
  if (res.status !== 204) return { ok: false, thongBao: await refusal(res, {}) };
  return { ok: true, duLieu: null };
}
