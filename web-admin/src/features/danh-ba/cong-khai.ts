/**
 * Công khai MỘT cán bộ lên danh bạ Zalo Mini App — câu chữ và phép quyết định
 * (`docs/ui-ux/12-danh-ba-can-bo.md §4`, câu mở #12).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * #12 DO KHÁCH CHỐT, VÀ HAI ĐIỀU CỦA NÓ ĐỊNH HÌNH TỆP NÀY:
 *
 *   1. MỘT NGƯỜI MỘT LẦN. Không có thao tác hàng loạt ở bất kỳ đâu — không cột ô tick, không thanh
 *      hành động. Công khai số di động cá nhân là công khai dữ liệu cá nhân (Nghị định 13/2023/NĐ-CP),
 *      và phải hỏi ý TỪNG người.
 *   2. ĐỒNG Ý DẠNG GỌN: người quản trị tick "đã hỏi ý và người này đồng ý" cho lần công khai ấy; máy
 *      chủ ghi thời điểm và người ghi. Không tick thì không có lời gọi nào đi ra — nút mờ đi, và
 *      `yeuCauCongKhai` từ chối trước khi dựng thân.
 *
 * MODULE THUẦN, KHÔNG JSX: mọi quyết định ở đây kiểm được bằng một phép so giá trị.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { YeuCauCongKhai } from "@/lib/api/can-bo";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { QUYEN_CONG_KHAI_DANH_BA, quyetDinhTheoKhoa } from "@/lib/quyen";

/* ---- bảng ----------------------------------------------------------------------------------- */

/** Cột và hai nhãn chip — nguyên văn đặc tả §4. */
export const COT_MINI_APP = "Trên Mini App";
export const CHIP_DANG_HIEN = "✓ Đang hiện";
export const CHIP_CHUA_HIEN = "Chưa hiện";

/** Dòng phụ dưới số di động (đặc tả §4). */
export const DONG_PHU_CO_ZALO = "Có Zalo";

/** Hai nút theo dòng — nguyên văn đặc tả §4, gắn thêm tên người cho trình đọc màn hình. */
export const NUT_THEM_MINI_APP = "Thêm vào danh bạ Mini App";
export const NUT_RUT_MINI_APP = "Rút khỏi danh bạ Mini App";

export function ariaThemMiniApp(hoTen: string): string {
  return `${NUT_THEM_MINI_APP}: ${hoTen}`;
}

export function ariaRutMiniApp(hoTen: string): string {
  return `${NUT_RUT_MINI_APP}: ${hoTen}`;
}

/**
 * `dd/mm/yyyy hh:mm` theo giờ Việt Nam, GHIM chứ không theo máy cán bộ — cùng lý do với
 * `nhanThoiDiem` của màn phản ánh. Dựng từ `formatToParts` chứ không dùng thẳng `format` của
 * `vi-VN`, vì định dạng ấy đặt GIỜ trước NGÀY; câu ở đây đọc ngày trước.
 */
const DINH_DANG = new Intl.DateTimeFormat("vi-VN", {
  timeZone: "Asia/Ho_Chi_Minh",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

/**
 * "Đồng ý ghi lúc 24/09/2026 14:05" — bằng chứng đồng ý mà máy chủ giữ (`consent_recorded_at`).
 * `null` → `null`: không có dấu đồng ý thì không in câu nào, thay vì in một mốc bịa ra.
 */
export function dongYGhiLuc(iso: string | null): string | null {
  if (iso === null) return null;
  const moc = new Date(iso);
  // Chuỗi không đọc được là hợp đồng hỏng: hiện nguyên văn thay vì "Invalid Date".
  if (Number.isNaN(moc.getTime())) return `Đồng ý ghi lúc ${iso}`;
  const p = Object.fromEntries(DINH_DANG.formatToParts(moc).map((x) => [x.type, x.value]));
  return `Đồng ý ghi lúc ${p.day}/${p.month}/${p.year} ${p.hour}:${p.minute}`;
}

/* ---- hộp công khai -------------------------------------------------------------------------- */

export function tieuDeCongKhai(hoTen: string): string {
  return `Thêm vào danh bạ Mini App: ${hoTen}`;
}

/**
 * Điều người bấm phải biết TRƯỚC khi tick: đây là công khai dữ liệu cá nhân, ra ngoài cơ quan, cho
 * bất kỳ ai. Không nói ra thì ô tick chỉ là một thủ tục bấm cho qua.
 */
export const CANH_BAO_CONG_KHAI =
  "Số di động cá nhân của người này sẽ hiện công khai cho mọi người dân dùng Zalo Mini App của " +
  "xã. Đây là dữ liệu cá nhân theo Nghị định 13/2023/NĐ-CP: chỉ công khai khi đã hỏi ý chính người " +
  "này và được người này đồng ý. Hệ thống ghi lại thời điểm và người xác nhận.";

export const O_DA_HOI_Y = "Đã hỏi ý và người này đồng ý công khai số di động lên Zalo Mini App";

export const O_THU_TU = "Thứ tự hiển thị";
export const MO_TA_THU_TU =
  "Không bắt buộc. Số nhỏ hiện trước; để trống nếu không cần sắp xếp riêng.";

export const NUT_XAC_NHAN_CONG_KHAI = "Công khai lên Mini App";

export const CAU_CHUA_XAC_NHAN =
  "Hãy xác nhận đã hỏi ý và được người này đồng ý trước khi công khai.";

export const CAU_THU_TU_SAI = "Thứ tự hiển thị phải là số nguyên từ 0 trở lên.";

/* ---- hộp rút -------------------------------------------------------------------------------- */

export function tieuDeRut(hoTen: string): string {
  return `Rút khỏi danh bạ Mini App: ${hoTen}`;
}

/**
 * Hậu quả của việc rút, nói ra TRƯỚC khi bấm. Nửa sau là nửa hay bị bỏ qua: máy chủ XOÁ dấu đồng
 * ý khi rút (`can_bo_ghi.go`, `datCongKhaiVao`), nên công khai lại là phải hỏi ý lại từ đầu.
 */
export const CANH_BAO_RUT =
  "Số của người này sẽ không còn hiện trên Zalo Mini App. Dấu ghi nhận đồng ý sẽ bị xoá: muốn " +
  "công khai lại, phải hỏi ý người này một lần nữa.";

export const NUT_XAC_NHAN_RUT = "Xác nhận rút khỏi danh bạ";

/* ---- câu sau khi ghi xong ------------------------------------------------------------------- */

export function daCongKhai(hoTen: string): string {
  return `Đã thêm ${hoTen} vào danh bạ Zalo Mini App.`;
}

export function daRut(hoTen: string): string {
  return `Đã rút ${hoTen} khỏi danh bạ Zalo Mini App.`;
}

/* ---- phép quyết định ------------------------------------------------------------------------ */

/**
 * Có vẽ hai nút Mini App hay không, theo phiên hiện tại. Phiên CHƯA ĐỌC XONG (`null`) hay đọc hỏng
 * → không (fail closed, `quyetDinhTheoKhoa`). Chỉ là tiện dụng: máy chủ kiểm `content.update` trên
 * từng lời gọi (luật 5, cấm #1).
 */
export function duocCongKhaiTheoPhien(phien: PhienDaDoc): boolean {
  return phien !== null && quyetDinhTheoKhoa(phien, QUYEN_CONG_KHAI_DANH_BA).hien;
}

/** Bản nháp của hộp công khai. */
export type BanCongKhai = { readonly daHoiY: boolean; readonly thuTu: string };

/**
 * Mở hộp công khai: ô tick LUÔN BẮT ĐẦU TRỐNG — kể cả khi người này từng được công khai rồi rút.
 * Đồng ý là cho LẦN NÀY; tick sẵn là xác nhận thay người quản trị. Thứ tự nạp giá trị đang có.
 */
export function banCongKhaiTu(cb: identity_canBoTomTat): BanCongKhai {
  return { daHoiY: false, thuTu: cb.display_order === null ? "" : String(cb.display_order) };
}

/** Nút công khai bấm được hay chưa — chỉ khi đã tick. */
export function guiCongKhaiDuoc(ban: BanCongKhai): boolean {
  return ban.daHoiY;
}

/**
 * Ô thứ tự → số. Rỗng → `null` (không đặt thứ tự). Chỉ nhận số nguyên từ 0; máy chủ vẫn kiểm lại.
 * Cần ở client vì thân phải mang một SỐ, không phải chuỗi gõ tay.
 */
export function docThuTu(tho: string): { ok: true; thuTu: number | null } | { ok: false } {
  const s = tho.trim();
  if (s === "") return { ok: true, thuTu: null };
  if (!/^\d+$/.test(s)) return { ok: false };
  const n = Number(s);
  return Number.isSafeInteger(n) ? { ok: true, thuTu: n } : { ok: false };
}

/**
 * Bấm "Công khai lên Mini App": hoặc một câu từ chối (không gọi mạng), hoặc yêu cầu gửi đi.
 *
 * CHƯA TICK THÌ KHÔNG CÓ YÊU CẦU NÀO — kể cả khi nút mờ đã bị ai đó bật lại bằng DevTools. Máy chủ
 * cũng từ chối (400 `consent_required`), nhưng một yêu cầu công khai không có xác nhận không được
 * rời trình duyệt ngay từ đầu.
 */
export function yeuCauCongKhai(
  ban: BanCongKhai,
): { readonly loi: string } | { readonly yeuCau: YeuCauCongKhai } {
  if (!ban.daHoiY) return { loi: CAU_CHUA_XAC_NHAN };
  const thuTu = docThuTu(ban.thuTu);
  if (!thuTu.ok) return { loi: CAU_THU_TU_SAI };
  return { yeuCau: { congKhai: true, daXacNhanDongY: ban.daHoiY, thuTu: thuTu.thuTu } };
}

/**
 * Rút một người khỏi danh bạ. GỬI LẠI THỨ TỰ ĐANG CÓ: tuyến là PUT, `display_order` vắng hay `null`
 * là XOÁ vị trí đã đặt — người được công khai lại sau này sẽ rơi xuống cuối mà không ai biết vì sao.
 */
export function yeuCauRut(cb: identity_canBoTomTat): YeuCauCongKhai {
  return { congKhai: false, daXacNhanDongY: false, thuTu: cb.display_order };
}
