/**
 * Nhãn và thứ tự bảy trạng thái nhiệm vụ của xã — `GET /api/v1/task-statuses` và
 * `PATCH /api/v1/task-statuses/{code}`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO MỘT TỆP RIÊNG, KHÔNG NẰM TRONG BẢNG BẢY DANH MỤC GHI CỦA `danh-muc.ts`
 *
 * Câu hỏi #21 đã chốt (ADR 0035 §C): xã đổi NHÃN và THỨ TỰ của bảy mã cố định; KHÔNG thêm, KHÔNG
 * xoá, KHÔNG tắt. Hợp đồng vì thế khác bảy danh mục kia ở cả ba chỗ: không có POST, không có
 * DELETE, và mục được định danh bằng `code` trên ĐƯỜNG DẪN chứ không bằng `id`. Ép nó vào
 * `MoTaDanhMucGhi` là mang theo một `gocThem` không trỏ vào đâu và một `xoaMuc` gọi được mà luôn
 * 404/405 — đúng loại nút bấm vào không có gì xảy ra.
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: xem `goi.ts`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./goi";
import type {
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_get_task_statuses,
  petitions_patch_task_statuses_by_code,
  petitions_suaTrangThaiNhiemVuVao,
  petitions_trangThaiNhiemVuRa,
} from "./schema.gen";

/**
 * GET — đủ BẢY trạng thái, đã sắp theo thứ tự hiệu lực của xã (hoà thì theo thứ tự mặc định).
 *
 * `any-authenticated`: nhãn trạng thái hiện trên cột Kanban, chip và bộ lọc của mọi cán bộ đọc sổ.
 * Không chéo xã — máy chủ buộc `tenant_id` ở tầng kho.
 */
export function layTrangThaiNhiemVu(): Promise<KetQua<petitions_danhSachTrangThaiNhiemVuRa>> {
  const duongDan: petitions_get_task_statuses["duongDan"] = "/api/v1/task-statuses";
  return docJSON<petitions_danhSachTrangThaiNhiemVuRa>(duongDan);
}

/**
 * Thân PATCH mà màn hình được phép gửi — thân của hợp đồng TRỪ `code` và `active`.
 *
 * Hợp đồng khai hai trường ấy trong `suaTrangThaiNhiemVuVao` CHỈ ĐỂ TỪ CHỐI chúng bằng 400: mã
 * nằm trên đường dẫn và không đổi được (luật 7, bất biến 3), còn nhóm này không có `Tắt` (#21).
 * `Omit` chặn lúc biên dịch; phép dựng từng trường trong `suaTrangThaiNhiemVu` chặn lúc chạy.
 */
export type SuaTrangThaiVao = Omit<petitions_suaTrangThaiNhiemVuVao, "code" | "active">;

/**
 * PATCH — đổi nhãn và/hoặc thứ tự của MỘT mã.
 *
 * DỰNG TỪNG TRƯỜNG, KHÔNG TRẢI ĐỐI TƯỢNG NGUỒN. Lối viết tự nhiên nhất — đọc một dòng rồi gửi lại
 * chính nó — mang theo `code` và 400 cho một thao tác không có gì sai. Trường `undefined` bị
 * `JSON.stringify` bỏ khỏi thân, và máy chủ đọc "không nhắc tới" là "không đổi".
 *
 * KHÔNG CÓ TUYẾN "ĐẶT LẠI": đặt lại mặc định là gửi đúng `default_label`/`default_order` qua chính
 * hàm này (bảng cấm DELETE; `customised` do máy chủ SUY RA từ chữ hiệu lực so với mặc định).
 *
 * Không `Idempotency-Key`: hợp đồng không đòi, và gửi hai lần cùng một nhãn cho cùng một mã không
 * sinh thêm dòng nào.
 */
export function suaTrangThaiNhiemVu(
  code: string,
  than: SuaTrangThaiVao,
): Promise<KetQua<petitions_trangThaiNhiemVuRa>> {
  const mau: petitions_patch_task_statuses_by_code["duongDan"] = "/api/v1/task-statuses/{code}";
  const thanGui: SuaTrangThaiVao = { label: than.label, order: than.order };
  return docThanLoiGoi<petitions_trangThaiNhiemVuRa>(
    goiGhi(mau.replace("{code}", encodeURIComponent(code)), "PATCH", thanGui, 200),
  );
}
