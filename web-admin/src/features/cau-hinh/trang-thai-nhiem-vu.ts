/**
 * Nhóm thứ tám của tab "Danh mục" — `Trạng thái nhiệm vụ`: câu chữ và hai phép dựng thân PATCH.
 * Hàm thuần, không gọi mạng, không dựng DOM — kiểm được bằng Node trơn.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO NHÓM NÀY KHÔNG DÙNG `nhom-danh-muc.ts` / `tang-danh-muc.ts`
 *
 * Câu hỏi #21 đã chốt ngày 24/09/2026 (ADR 0035 §C): xã đổi NHÃN và THỨ TỰ bảy trạng thái; KHÔNG
 * thêm, KHÔNG tắt, KHÔNG xoá — danh sách mã do vòng đời §6 cố định. Quy tắc ba tầng của bảy nhóm
 * kia (`source`, `tier`, `active`, `is_default`) không có nghĩa ở đây, và hợp đồng cũng không phát
 * ra các trường ấy (`petitions_trangThaiNhiemVuRa`). Nhồi nhóm này vào khuôn chung là phải bịa ra
 * một tầng và một cờ `active` — tức vẽ ra nút `Tắt` cho một thứ không tắt được.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { SuaTrangThaiVao } from "@/lib/api/trang-thai-nhiem-vu";
import type { petitions_trangThaiNhiemVuRa } from "@/lib/api/schema.gen";

export const TIEU_DE_NHOM_TRANG_THAI = "Trạng thái nhiệm vụ";

/**
 * Câu một dòng dưới tiêu đề nhóm: vì sao nhóm này không có `+ Thêm mục`, `Tắt`, `Xoá`.
 *
 * NÓI TRƯỚC, không để cán bộ phát hiện bằng cách thiếu nút — một nhóm có nút ở mọi nhóm khác mà
 * không có ở đây đọc lên là "màn hình hỏng".
 */
export const GHI_CHU_NHOM_TRANG_THAI =
  "Bảy trạng thái này gắn với vòng đời nhiệm vụ nên không thêm, tắt hay xoá được. Đơn vị chỉ đổi " +
  "được nhãn hiển thị và thứ tự.";

/** Thứ tự ở nhóm này CÓ HỆ QUẢ ở màn khác — nói ra để người sắp biết mình đang sắp cái gì. */
export const GIAI_THICH_THU_TU_TRANG_THAI =
  "Thứ tự quyết định thứ tự cột trên bảng Kanban và trong ô lọc Trạng thái của màn Nhiệm vụ.";

/**
 * Cột "Vai trò" — `role` của hợp đồng. Hai giá trị là hai vai của sơ đồ vòng đời §6.
 *
 * GIÁ TRỊ LẠ HIỆN NGUYÊN VĂN: hợp đồng khai `role` là `string` trơn, nên một giá trị thứ ba nghĩa là
 * hợp đồng đã trôi; dịch bừa nó là giấu sự cố sau một chữ trông bình thường.
 */
export function nhanVaiTro(role: string): string {
  switch (role) {
    case "chinh":
      return "Chính";
    case "re-nhanh":
      return "Rẽ nhánh";
    default:
      return role;
  }
}

/** Badge cột "Tuỳ chỉnh" — `customised`, do máy chủ SUY RA từ chữ hiệu lực so với mặc định. */
export function nhanTuyChinh(daDoi: boolean): string {
  return daDoi ? "Đã đổi" : "Mặc định";
}

export function lopTuyChinh(daDoi: boolean): string {
  return daDoi ? "chip chip-hoat-dong" : "chip chip-ngung";
}

/** Dòng phụ dưới badge `Đã đổi`: chữ và thứ tự mặc định, để biết `Đặt lại mặc định` sẽ đưa về đâu. */
export function chuMacDinh(d: petitions_trangThaiNhiemVuRa): string {
  return `Mặc định: ${d.default_label} · thứ tự ${d.default_order}`;
}

export const NUT_DAT_LAI = "Đặt lại mặc định";

export function tieuDeSuaTrangThai(nhan: string): string {
  return `Sửa trạng thái ${nhan}`;
}

export function daLuuTrangThai(ma: string): string {
  return `Đã lưu nhãn và thứ tự của trạng thái ${ma}.`;
}

export function daDatLaiTrangThai(ma: string): string {
  return `Đã đặt trạng thái ${ma} về nhãn và thứ tự mặc định.`;
}

export const CHUA_DOI_GI = "Chưa có gì thay đổi so với nhãn và thứ tự đang dùng.";
export const THU_TU_KHONG_PHAI_SO = "Thứ tự phải là một số nguyên.";

/** Bản nháp của biểu mẫu sửa. Chuỗi hết — ô nhập của trình duyệt trả về chuỗi. */
export type BanNhapTrangThai = { readonly nhan: string; readonly thuTu: string };

export function banTuTrangThai(d: petitions_trangThaiNhiemVuRa): BanNhapTrangThai {
  return { nhan: d.label, thuTu: String(d.order) };
}

/**
 * Bản nháp → thân PATCH, CHỈ những trường đã đổi so với dòng đang hiện.
 *
 * KHÔNG BAO GIỜ CÓ `code` HAY `active` — máy chủ trả 400 cho cả hai (`code` nằm trên đường dẫn, và
 * nhóm này không có `Tắt`). Kiểu trả về đã `Omit` hai trường ấy; `suaTrangThaiNhiemVu` dựng lại thân
 * từng trường thêm một lần nữa lúc chạy.
 *
 * KHÔNG KIỂM ĐỘ DÀI NHÃN HAY KHOẢNG 1..9999 Ở ĐÂY: máy chủ kiểm và trả câu tiếng Việt nói rõ phải
 * sửa gì; chép quy tắc ấy xuống là dựng bản sao thứ hai của nó (luật 9). Phép kiểm duy nhất ở đây là
 * ĐỌC ĐƯỢC CON SỐ: `Number("3 chữ")` là `NaN`, và lặng lẽ bỏ trường ấy đi là nuốt một lỗi gõ.
 *
 * Ô `Thứ tự` rỗng = KHÔNG ĐỔI, không phải số 0.
 */
export function thanSuaTrangThai(
  d: petitions_trangThaiNhiemVuRa,
  ban: BanNhapTrangThai,
): { ok: true; than: SuaTrangThaiVao } | { ok: false; loi: string } {
  const than: SuaTrangThaiVao = {};
  const nhan = ban.nhan.trim();
  if (nhan !== d.label) than.label = nhan;

  const oThuTu = ban.thuTu.trim();
  if (oThuTu !== "") {
    const n = Number(oThuTu);
    if (!Number.isInteger(n)) return { ok: false, loi: THU_TU_KHONG_PHAI_SO };
    if (n !== d.order) than.order = n;
  }

  if (than.label === undefined && than.order === undefined) return { ok: false, loi: CHUA_DOI_GI };
  return { ok: true, than };
}

/**
 * Thân của `Đặt lại mặc định` — ĐÚNG hai giá trị mặc định máy chủ vừa trả về.
 *
 * Không có tuyến đặt lại riêng, và bảng cấm DELETE (migration 0010): "về mặc định" là một lần GHI
 * chữ mặc định. Lấy `default_label`/`default_order` từ CHÍNH dòng máy chủ trả, không từ bảng nhãn
 * đường lui của màn Nhiệm vụ — bản ấy là bản sao, và bản sao là thứ trôi.
 */
export function thanDatLaiMacDinh(d: petitions_trangThaiNhiemVuRa): SuaTrangThaiVao {
  return { label: d.default_label, order: d.default_order };
}
