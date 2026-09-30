/**
 * Trạng thái SỬA của ma trận phân quyền — bật/tắt ô, cột nào đã sửa, lưu MỘT cột, huỷ MỘT cột.
 * Hàm thuần, không gọi mạng (trừ `guiCot`, nhận hàm gọi làm tham số), không dựng DOM.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐƠN VỊ LƯU LÀ MỘT CỘT, KHÔNG PHẢI MA TRẬN (`docs/ui-ux/14-cau-hinh.md §12.5`).
 *
 * `PUT /api/v1/roles/{id}/permissions` nhận TOÀN BỘ tập quyền của một vai trò: khoá nào vắng mặt
 * là khoá bị gỡ. Gửi nhầm tập của cột khác — hay gửi hợp của mọi cột — không báo lỗi gì: máy chủ
 * lưu đúng thứ nhận được, và một vai trò lặng lẽ được cấp những quyền không ai tick cho nó. Nên thân
 * được dựng ở ĐÚNG MỘT chỗ (`thanLuuCot`), từ ĐÚNG MỘT cột, và `guiCot` gọi mạng ĐÚNG MỘT lần.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * HAI BẢNG, KHÔNG MỘT: `goc` là trạng thái máy chủ đã xác nhận (đọc về, hoặc trả về sau 200);
 * `sua` chỉ chứa những cột ĐANG KHÁC `goc`. Cột không có trong `sua` là cột sạch. Gộp hai thứ vào
 * một bảng thì "Huỷ" không còn gì để quay về, và "đã sửa" phải so từng ô ở mỗi lần vẽ.
 *
 * LỖI THUỘC VỀ CỘT, và KHÔNG xoá phần đã sửa: cán bộ vừa tick sáu ô thì một lần 403 không được bắt
 * họ tick lại từ đầu. Không có tự thử lại — một lần từ chối vì `last_holder` thử lại vẫn bị từ chối,
 * và một lần thử lại tự động trên tuyến phân quyền là một lần ghi không ai bấm.
 */

import type { KetQua } from "@/lib/api/goi";
import type { identity_cotPhanQuyenRa, identity_luuPhanQuyenVao } from "@/lib/api/schema.gen";

import type { BangDaCap } from "./ma-tran-quyen";

export type BanSua = {
  /** Trạng thái máy chủ đã xác nhận: id vai trò → tập khoá. */
  readonly goc: BangDaCap;
  /** Chỉ những cột đang khác `goc`. Vắng mặt = cột sạch. */
  readonly sua: BangDaCap;
  /** Cột đang chờ phản hồi lưu. */
  readonly dangLuu: ReadonlySet<string>;
  /** Câu máy chủ viết cho lần lưu hỏng gần nhất của cột ấy — nguyên văn. */
  readonly loi: ReadonlyMap<string, string>;
};

const TAP_RONG: ReadonlySet<string> = new Set();

export function banSuaMoi(goc: BangDaCap): BanSua {
  return { goc, sua: new Map(), dangLuu: new Set(), loi: new Map() };
}

/** Tập khoá đang hiện trên cột — bản đang sửa nếu có, không thì bản máy chủ. */
export function tapHienTai(b: BanSua, vaiTroId: string): ReadonlySet<string> {
  return b.sua.get(vaiTroId) ?? b.goc.get(vaiTroId) ?? TAP_RONG;
}

/** Bảng tra cho phần vẽ: `goc` đè bởi các cột đang sửa. */
export function bangHienThi(b: BanSua): BangDaCap {
  if (b.sua.size === 0) return b.goc;
  const bang = new Map(b.goc);
  for (const [id, tap] of b.sua) bang.set(id, tap);
  return bang;
}

export function cotDaSua(b: BanSua, vaiTroId: string): boolean {
  return b.sua.has(vaiTroId);
}

function bangNhau(a: ReadonlySet<string>, c: ReadonlySet<string>): boolean {
  if (a.size !== c.size) return false;
  for (const k of a) if (!c.has(k)) return false;
  return true;
}

function boKhoa<V>(m: ReadonlyMap<string, V>, k: string): Map<string, V> {
  const moi = new Map(m);
  moi.delete(k);
  return moi;
}

/**
 * Bật/tắt một ô. Bật rồi tắt lại đúng ô ấy thì cột trở về SẠCH — nút `Lưu` tắt đi, vì không còn
 * gì để lưu. Cột đang lưu thì không đổi: phản hồi sắp về sẽ thay cả cột.
 */
export function batTatO(b: BanSua, vaiTroId: string, khoa: string): BanSua {
  if (b.dangLuu.has(vaiTroId)) return b;
  const tap = new Set(tapHienTai(b, vaiTroId));
  if (tap.has(khoa)) tap.delete(khoa);
  else tap.add(khoa);

  const goc = b.goc.get(vaiTroId) ?? TAP_RONG;
  const sua = bangNhau(tap, goc) ? boKhoa(b.sua, vaiTroId) : new Map(b.sua).set(vaiTroId, tap);
  return { ...b, sua };
}

/** Huỷ: cột quay về bản máy chủ đã xác nhận, lỗi của cột cũng đi theo. */
export function huyCot(b: BanSua, vaiTroId: string): BanSua {
  if (b.dangLuu.has(vaiTroId)) return b;
  return { ...b, sua: boKhoa(b.sua, vaiTroId), loi: boKhoa(b.loi, vaiTroId) };
}

/**
 * Thân của `PUT` cho MỘT cột — dựng từng trường, từ đúng tập của cột ấy.
 *
 * Sắp theo chữ và không trùng: `Set` đã khử trùng, còn sắp là để hai lần lưu cùng một tập cho ra
 * cùng một thân — máy chủ không cần thứ tự, nhưng người đọc vết kiểm toán thì có.
 */
export function thanLuuCot(b: BanSua, vaiTroId: string): identity_luuPhanQuyenVao {
  return { permissions: [...tapHienTai(b, vaiTroId)].sort() };
}

export function batDauLuu(b: BanSua, vaiTroId: string): BanSua {
  return { ...b, dangLuu: new Set(b.dangLuu).add(vaiTroId), loi: boKhoa(b.loi, vaiTroId) };
}

/** Câu khi máy chủ trả 200 cho một vai trò KHÁC vai trò đã gửi — không áp vào cột nào. */
export const LOI_SAI_VAI_TRO =
  "Máy chủ trả về kết quả không khớp với vai trò vừa lưu. Tải lại trang để xem phân quyền hiện tại.";

/**
 * Phản hồi lưu → bản sửa mới.
 *
 * 200: thay cột bằng tập MÁY CHỦ TRẢ, không bằng tập đã gửi — đó là trạng thái thật sau lần ghi.
 * Lỗi: giữ nguyên phần đã sửa, gắn câu của máy chủ vào cột.
 *
 * `role_id` khác vai trò đã gửi thì KHÔNG áp: một tập quyền đặt nhầm cột sẽ hiện ra như trạng thái
 * đã lưu của vai trò này, và cán bộ không có cách nào nhận ra.
 */
export function ketThucLuu(
  b: BanSua,
  vaiTroId: string,
  kq: KetQua<identity_cotPhanQuyenRa>,
): BanSua {
  const dangLuu = new Set(b.dangLuu);
  dangLuu.delete(vaiTroId);

  if (!kq.ok) return { ...b, dangLuu, loi: new Map(b.loi).set(vaiTroId, kq.thongBao) };
  if (kq.duLieu.role_id !== vaiTroId) {
    return { ...b, dangLuu, loi: new Map(b.loi).set(vaiTroId, LOI_SAI_VAI_TRO) };
  }

  return {
    goc: new Map(b.goc).set(vaiTroId, new Set(kq.duLieu.permissions)),
    sua: boKhoa(b.sua, vaiTroId),
    dangLuu,
    loi: boKhoa(b.loi, vaiTroId),
  };
}

export type HamLuuCot = (
  vaiTroId: string,
  than: identity_luuPhanQuyenVao,
) => Promise<KetQua<identity_cotPhanQuyenRa>>;

/**
 * Gửi MỘT cột. Một lời gọi, một vai trò, một thân — các cột khác, sửa hay chưa, không đi kèm.
 * Hàm gọi mạng là tham số để ca kiểm thấy được chính xác thứ đi ra dây.
 */
export function guiCot(
  b: BanSua,
  vaiTroId: string,
  luu: HamLuuCot,
): Promise<KetQua<identity_cotPhanQuyenRa>> {
  return luu(vaiTroId, thanLuuCot(b, vaiTroId));
}
