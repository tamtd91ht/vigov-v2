/**
 * Ba tuyến GHI của Sơ đồ tổ chức (`docs/ui-ux/14-cau-hinh.md §1`) — thêm, sửa, xoá mềm một bộ phận.
 * Nhập từ Excel ở `org-unit-import.ts`.
 *
 * TUYẾN ĐỌC KHÔNG Ở ĐÂY: `GET /api/v1/org-units` đã có chủ là `layDanhMucBoPhan` trong
 * `danh-muc.ts`, và màn Sơ đồ tổ chức dùng lại đúng hàm ấy. Hàm đọc thứ hai của cùng một tuyến là
 * một bản sao sẽ trôi (luật 9, cấm #2).
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: ba quy tắc
 * ấy đúng cho MỌI tuyến và được nói một lần ở `goi.ts`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * `code` KHÔNG BAO GIỜ ĐI LÊN TRONG PATCH. Mã bộ phận đã cấp thì không đổi (luật 7, bất biến 3),
 * và máy chủ không lặng lẽ bỏ qua trường ấy mà TỪ CHỐI 400 cả yêu cầu
 * (`service-identity/internal/http/bo_phan.go:208`). Kiểu `SuaBoPhanVao` trừ `code` để chặn lúc
 * biên dịch; thân được dựng TỪNG TRƯỜNG để chặn lúc chạy — một `...than` là đường để `code` đi lên
 * vào ngày ai đó truyền vào một dòng đọc được từ máy chủ.
 *
 * XOÁ (`DELETE /api/v1/org-units/{id}`, ADR 0056) là xoá MỀM kèm lý do bắt buộc, và máy chủ từ chối
 * khi bộ phận còn cán bộ, bộ phận con hay hồ sơ đang mở ở phân hệ khác (§12.4) — xem `deleteOrgUnit`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { CHUNG, docThanKetQua, errorMessageOr, goiGhi, LOI_KHONG_RO, type KetQua } from "./goi";
import type {
  identity_boPhanDaGhiRa,
  identity_delete_org_units_by_id,
  identity_orgUnitDeleteIn,
  identity_orgUnitHoldingsOut,
  identity_orgUnitInUseOut,
  identity_patch_org_units_by_id,
  identity_post_org_units,
  identity_suaBoPhanVao,
  identity_themBoPhanVao,
} from "./schema.gen";

/** Thân POST — đúng kiểu của hợp đồng. */
export type ThemBoPhanVao = identity_themBoPhanVao;

/** Thân PATCH — kiểu của hợp đồng TRỪ `code`. Xem đầu tệp. */
export type SuaBoPhanVao = Omit<identity_suaBoPhanVao, "code">;

const DUONG_DAN_THEM = "/api/v1/org-units" satisfies identity_post_org_units["duongDan"];
const MAU_DUONG_DAN_SUA = "/api/v1/org-units/{id}" satisfies identity_patch_org_units_by_id["duongDan"];

/**
 * POST /api/v1/org-units — thêm một bộ phận. 201 kèm bộ phận vừa ghi, KHÔNG có `staff_count`:
 * màn hình đọc lại danh sách sau khi ghi, vì một số đếm 0 vẽ từ phản hồi này là một con số sai.
 *
 * `khoaChongTrung` LÀ THAM SỐ, KHÔNG SINH TẠI CHỖ. Hợp đồng đòi `Idempotency-Key`; sinh khoá bên
 * trong hàm thì mỗi lần bấm lại sau một lỗi mạng là một khoá mới — tức một bộ phận thứ hai cùng tên,
 * vì lần gửi đầu CÓ THỂ đã tới máy chủ. Khoá do biểu mẫu giữ, sinh lúc MỞ biểu mẫu.
 *
 * TRƯỜNG TUỲ CHỌN RỖNG THÌ VẮNG MẶT. `parent_id` vắng = gốc; `code` vắng = máy chủ tự sinh từ tên;
 * `order` vắng = 0 (`bo_phan.go:135-142`). Gửi `code: ""` thì máy chủ đọc là KHÔNG nhập, nhưng
 * `JSON.stringify` bỏ hẳn `undefined` nên thân mang đúng những gì cán bộ đã gõ và không hơn.
 */
export function themBoPhan(
  than: ThemBoPhanVao,
  khoaChongTrung: string,
): Promise<KetQua<identity_boPhanDaGhiRa>> {
  const thanGui: ThemBoPhanVao = {
    name: than.name,
    parent_id: than.parent_id === "" ? undefined : than.parent_id,
    order: than.order ?? undefined,
    code: than.code === "" ? undefined : than.code,
  };
  return goiGhi(DUONG_DAN_THEM, "POST", thanGui, 201, { "Idempotency-Key": khoaChongTrung }).then(
    docThanKetQua<identity_boPhanDaGhiRa>,
  );
}

/**
 * PATCH /api/v1/org-units/{id} — đổi tên, dời sang bộ phận cha khác, đổi thứ tự.
 *
 * VẮNG = KHÔNG ĐỔI; `parent_id: ""` = DỜI LÊN GỐC. Hai nghĩa ấy khác hẳn nhau, và đó là lý do
 * `parent_id` ở đây được chép nguyên — kể cả chuỗi rỗng — chứ không đi qua phép "rỗng thì bỏ" của
 * `themBoPhan` ngay trên. Bỏ chuỗi rỗng ở đây thì nút "dời lên gốc" gửi đi một thân không đổi gì.
 *
 * KHÔNG CÓ `Idempotency-Key`: hợp đồng không đòi, và gửi lại cùng một lần sửa cho ra cùng một
 * trạng thái.
 */
export function suaBoPhan(id: string, than: SuaBoPhanVao): Promise<KetQua<identity_boPhanDaGhiRa>> {
  const thanGui: SuaBoPhanVao = {
    name: than.name ?? undefined,
    parent_id: than.parent_id ?? undefined,
    order: than.order ?? undefined,
  };
  const duongDan = MAU_DUONG_DAN_SUA.replace("{id}", encodeURIComponent(id));
  return goiGhi(duongDan, "PATCH", thanGui, 200).then(docThanKetQua<identity_boPhanDaGhiRa>);
}

const DELETE_PATH = "/api/v1/org-units/{id}" satisfies identity_delete_org_units_by_id["duongDan"];

/**
 * One delete attempt. A refusal because the unit still HOLDS something carries the counts, so the
 * screen can say what must be moved first; every other refusal is the server's sentence only.
 */
export type OrgUnitDeleteResult =
  | { readonly ok: true }
  | {
      readonly ok: false;
      readonly message: string;
      /** Only on 409 `org_unit_in_use`; `null` for every other refusal. */
      readonly holdings: identity_orgUnitHoldingsOut | null;
    };

/** 503 whose body is not the server's (a proxy page): the same fact, said without the server. */
export const DELETE_UNAVAILABLE_FALLBACK =
  "Chưa kiểm được hồ sơ bộ phận đang giữ ở các phân hệ khác, nên chưa xoá. Vui lòng thử lại sau ít phút.";

/**
 * DELETE /api/v1/org-units/{id} — SOFT delete, reason required (rule 7, invariant 1). 204, no body.
 *
 * WHY NOT `goiGhi`: it reduces every refusal to `message`, and the 409 here carries `holdings` —
 * the counts are the answer to "what do I move before I can delete". The branch is on the HTTP
 * STATUS and the SHAPE of the body, never on `code` (`goi.ts`): a 409 without a readable
 * `holdings` object is shown as its sentence alone.
 *
 * 503 IS NEVER "DELETED": it means petitions or documents did not answer, and an unanswered
 * question is not "holds nothing" (ADR 0056). The server's sentence says so; a proxy page gets
 * `DELETE_UNAVAILABLE_FALLBACK`.
 *
 * NO `Idempotency-Key`: the contract declares none, and deleting an already-deleted unit is a 404,
 * not a second row.
 */
export async function deleteOrgUnit(id: string, reason: string): Promise<OrgUnitDeleteResult> {
  const body: identity_orgUnitDeleteIn = { reason };
  let res: Response;
  try {
    res = await fetch(DELETE_PATH.replace("{id}", encodeURIComponent(id)), {
      ...CHUNG,
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch {
    // No log: the body carries the reason staff typed (rule 3).
    return { ok: false, message: LOI_KHONG_RO, holdings: null };
  }

  if (res.status === 204) return { ok: true };
  if (res.status === 409) return readInUse(res);
  if (res.status === 503) {
    return { ok: false, message: await errorMessageOr(res, DELETE_UNAVAILABLE_FALLBACK), holdings: null };
  }
  return { ok: false, message: await errorMessageOr(res, LOI_KHONG_RO), holdings: null };
}

async function readInUse(res: Response): Promise<OrgUnitDeleteResult> {
  try {
    const body = (await res.json()) as Partial<identity_orgUnitInUseOut>;
    const message = typeof body.message === "string" && body.message !== "" ? body.message : LOI_KHONG_RO;
    const h = body.holdings;
    const holdings = h !== null && typeof h === "object" && isHoldings(h) ? h : null;
    return { ok: false, message, holdings };
  } catch {
    return { ok: false, message: LOI_KHONG_RO, holdings: null };
  }
}

/** Every count present and a number — a partial object is not shown as "0 of the missing kind". */
function isHoldings(h: object): h is identity_orgUnitHoldingsOut {
  const r = h as Record<string, unknown>;
  return (["staff", "child_units", "open_petitions", "open_tasks", "open_incoming_documents"] as const).every(
    (k) => typeof r[k] === "number",
  );
}
