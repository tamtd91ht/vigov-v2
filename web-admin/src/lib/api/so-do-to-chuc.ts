/**
 * Hai tuyến GHI của Sơ đồ tổ chức (`docs/ui-ux/14-cau-hinh.md §1`) — thêm và sửa một bộ phận.
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
 * KHÔNG CÓ TUYẾN XOÁ, và không có hàm xoá nào chờ sẵn ở đây: hợp đồng chưa có tuyến ấy, vì §12.4
 * đòi chặn khi bộ phận còn hồ sơ đang giữ ở các dịch vụ khác (`bo_phan.go:24`).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { docThanKetQua, goiGhi, type KetQua } from "./goi";
import type {
  identity_boPhanDaGhiRa,
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
