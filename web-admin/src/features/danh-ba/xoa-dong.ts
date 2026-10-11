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
 * Xoá là xoá MỀM (luật 7): dòng rời mọi đường đọc, mã cán bộ không bao giờ cấp lại.
 * MODULE THUẦN, KHÔNG JSX.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { QUYEN_XOA_DONG_DANH_BA, quyetDinhTheoKhoa } from "@/lib/quyen";

/**
 * Nút theo dòng — nhãn trợ năng (và `title`) nói việc và tên người. Hình của nút là icon `Trash2`
 * (ADR 0068: bỏ emoji làm icon), nên nhãn này là chữ DUY NHẤT của nút: kiểm "không có nút xoá" là
 * kiểm vắng nhãn này.
 */
export const NHAN_XOA_DONG = "Xoá khỏi danh bạ";

/** Chữ HIỆN của nút xoá trên thẻ điện thoại — phần đầu của `ariaXoaDong`, nên khớp nhãn trợ năng. */
export const DELETE_SHORT = "Xoá";

export function ariaXoaDong(hoTen: string): string {
  return `${NHAN_XOA_DONG}: ${hoTen}`;
}

export function tieuDeXoa(hoTen: string): string {
  return `${NHAN_XOA_DONG}: ${hoTen}`;
}

/**
 * Câu hỏi hiện làm tiêu đề hộp xác nhận (đặc tả giao diện v2 §7: tiêu đề là câu hỏi cụ thể). Cùng
 * việc với `NHAN_XOA_DONG` — xoá một dòng KHỎI DANH BẠ — chứ không phải "Xoá cán bộ": việc này gỡ
 * một dòng nhập trùng, không xoá một con người khỏi hồ sơ cơ quan.
 */
export function deleteQuestion(hoTen: string): string {
  return `Xoá ${hoTen} khỏi danh bạ?`;
}

/**
 * The dialog's description — the prototype's own sentence (`StaffDirectoryWorkspace.tsx:427-429`, spec
 * `08-modal-can-bo-va-excel.md` "Xoá hàng loạt"), said of one person. No reason box (bug sheet row 27,
 * owner decision 10/10/2026): the server records a fixed sentence in the trail.
 */
export const GIAI_THICH_XOA =
  "Người này sẽ khuất khỏi danh bạ và khỏi Mini App. Bản ghi vẫn được giữ trong hệ thống, nhưng " +
  "giao diện không có nút hoàn tác — muốn có lại thì phải nhập hoặc thêm lại.";

/**
 * Câu cho dòng ĐANG CÓ tài khoản — hiện trong hộp THAY cho nút xác nhận, trước khi gửi gì. Máy chủ
 * vẫn từ chối (409 `staff_has_account`, câu của nó hiện nguyên văn nếu tới được đó).
 */
export const CAU_CO_TAI_KHOAN =
  "Người này đang có tài khoản đăng nhập nên không xoá được. Nếu đây là cán bộ nghỉ hưu hoặc " +
  "chuyển công tác, hãy khoá tài khoản ở màn Người dùng.";

/**
 * The confirm button. The question above it already names the person and "khỏi danh bạ", so the button
 * is the verb alone — as the prototype's red confirm buttons are.
 */
export const NUT_XAC_NHAN_XOA = "Xoá";

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
 * Bấm "Xoá": một câu từ chối (KHÔNG gọi mạng) cho dòng có tài khoản, hoặc `null` — gửi được.
 */
export function yeuCauXoa(cb: identity_canBoTomTat): string | null {
  return cb.has_account ? CAU_CO_TAI_KHOAN : null;
}
