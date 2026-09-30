/**
 * Xoá MỘT dòng danh bạ nhập trùng — câu chữ và phép quyết định (`docs/ui-ux/12-danh-ba-can-bo.md §4`,
 * câu mở #10, ADR 0035).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * XOÁ KHÔNG PHẢI KHOÁ, và câu chữ ở đây tồn tại để người bấm không nhầm hai việc ấy:
 *
 *   xoá  chỉ cho một người bị nhập HAI LẦN do nhầm; dòng có tài khoản đăng nhập thì không xoá được.
 *   khoá cho cán bộ nghỉ hưu / chuyển công tác — người vẫn còn trong danh bạ, tên vẫn đọc được trên
 *        mọi hồ sơ đã xử lý.
 *
 * Xoá là xoá MỀM (luật 7): dòng rời mọi đường đọc, lý do được lưu, mã cán bộ không bao giờ cấp lại.
 * MODULE THUẦN, KHÔNG JSX.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import { LY_DO_XOA_TOI_DA, chuanHoaLyDoXoa, type LyDoXoaHopLe } from "@/lib/api/can-bo";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { QUYEN_XOA_DONG_DANH_BA, quyetDinhTheoKhoa } from "@/lib/quyen";

/** Nút theo dòng — biểu tượng của đặc tả §4; nhãn trợ năng nói việc và tên người. */
export const NUT_XOA_DONG = "🗑";
export const NHAN_XOA_DONG = "Xoá khỏi danh bạ";

export function ariaXoaDong(hoTen: string): string {
  return `${NHAN_XOA_DONG}: ${hoTen}`;
}

export function tieuDeXoa(hoTen: string): string {
  return `${NHAN_XOA_DONG}: ${hoTen}`;
}

/**
 * Nói TRƯỚC khi bấm — bốn điều, mỗi điều chặn một hiểu nhầm có thật: xoá để "dọn" người đã nghỉ;
 * xoá một người đang đăng nhập được; nghĩ rằng mã sẽ được dùng lại cho người sau; nghĩ rằng lý do là
 * ô ghi chú không ai đọc.
 */
export const GIAI_THICH_XOA =
  "Chỉ dùng khi một người bị nhập vào danh bạ hai lần do nhầm. Cán bộ nghỉ hưu hoặc chuyển công " +
  "tác thì phải khoá tài khoản (Khoá tài khoản, ở Cấu hình → Người dùng), không xoá. Dòng đang có " +
  "tài khoản đăng nhập không xoá được. Mã cán bộ của dòng bị xoá không bao giờ được cấp lại, và lý " +
  "do xoá được lưu vào hồ sơ.";

/**
 * Câu cho dòng ĐANG CÓ tài khoản — hiện trong hộp THAY cho nút xác nhận, trước khi gửi gì. Máy chủ
 * vẫn từ chối (409 `staff_has_account`, câu của nó hiện nguyên văn nếu tới được đó).
 */
export const CAU_CO_TAI_KHOAN =
  "Người này đang có tài khoản đăng nhập nên không xoá được. Nếu đây là cán bộ nghỉ hưu hoặc " +
  "chuyển công tác, hãy khoá tài khoản ở Cấu hình → Người dùng.";

export const O_LY_DO_XOA = "Lý do xoá";
export const MO_TA_LY_DO_XOA = `Bắt buộc, tối đa ${LY_DO_XOA_TOI_DA} ký tự. Ví dụ: nhập trùng với dòng của cùng người này.`;

export const NUT_XAC_NHAN_XOA = "Xác nhận xoá khỏi danh bạ";

export const CAU_THIEU_LY_DO = "Hãy nhập lý do xoá.";
export const CAU_LY_DO_QUA_DAI = `Lý do xoá quá dài (tối đa ${LY_DO_XOA_TOI_DA} ký tự).`;

export function daXoa(hoTen: string): string {
  return `Đã xoá ${hoTen} khỏi danh bạ.`;
}

/**
 * Có vẽ nút xoá hay không. Phiên CHƯA ĐỌC XONG hay đọc hỏng → không (fail closed). Chỉ là tiện dụng:
 * máy chủ kiểm `admin.user.delete` trên từng lời gọi (luật 5, cấm #1).
 */
export function duocXoaTheoPhien(phien: PhienDaDoc): boolean {
  return phien !== null && quyetDinhTheoKhoa(phien, QUYEN_XOA_DONG_DANH_BA).hien;
}

/**
 * Bấm "Xác nhận xoá": hoặc một câu từ chối (KHÔNG gọi mạng), hoặc lý do đã chuẩn hoá để gửi.
 *
 * Dòng có tài khoản, lý do rỗng, lý do quá dài — cả ba dừng ở đây. Không có đường nào dựng ra một
 * yêu cầu xoá thiếu lý do: `xoaCanBo` chỉ nhận `LyDoXoaHopLe`.
 */
export function yeuCauXoa(
  cb: identity_canBoTomTat,
  lyDoTho: string,
): { readonly loi: string } | { readonly lyDo: LyDoXoaHopLe } {
  if (cb.has_account) return { loi: CAU_CO_TAI_KHOAN };
  const lyDo = chuanHoaLyDoXoa(lyDoTho);
  if (lyDo.loai === "rong") return { loi: CAU_THIEU_LY_DO };
  if (lyDo.loai === "quaDai") return { loi: CAU_LY_DO_QUA_DAI };
  return { lyDo };
}
