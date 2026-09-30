/**
 * Đọc phản hồi của `GET /api/v1/role-permissions` thành trạng thái mà màn hình dựng được — và
 * tra một ô `(vai trò, quyền)` đã cấp hay chưa.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO TÁCH KHỎI `.tsx`, cùng lý do đã ghi ở `quyen-tab.ts` và `tra-danh-muc.ts`: ca đáng lo
 * nhất của màn hình này — KHÔNG ĐỌC ĐƯỢC — là ca không nhìn thấy bằng mắt, mà lại là ca sẽ xảy
 * ra thật (phiên hết hạn, quyền vừa bị gỡ, máy chủ chạm trần). Nằm lẫn trong một component thì
 * không bài test nào chạm tới nó.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * NĂM TRẠNG THÁI CỦA TAB, BỐN CỦA TỆP NÀY. Trạng thái thứ năm — `không đủ quyền` — đến từ
 * `quyen-tab.ts` chứ không từ đây: nó được biết TRƯỚC khi có lời gọi nào, từ danh sách quyền của
 * phiên, và tab thiếu `admin.role` thì không gọi tuyến này lần nào. Bốn trạng thái còn lại:
 *
 *   `dangDoc`        chưa có câu trả lời nào.
 *   `khongDocDuoc`   401 · 403 · 500 · mạng hỏng. Hiện đúng câu của máy chủ, không diễn giải.
 *   `chuaCauHinh`    đọc được, và ma trận không có cột hoặc không có hàng để dựng. KHÔNG phải
 *                    lỗi: một xã vừa onboard chưa có vai trò nào là chuyện bình thường. Phải nói
 *                    ra bằng một câu, không được để một bảng rỗng đứng đó.
 *   `coDuLieu`       dựng được.
 *
 * KHÔNG CÓ TRẠNG THÁI "MỘT Ô KHÔNG RÕ": hợp đồng gửi danh sách các ô ĐÃ CẤP; ô nào không có mặt
 * là chưa cấp. Đúng cách luật 5, bất biến 2 đọc một khai báo thiếu — thiếu nghĩa là từ chối, chứ
 * không phải chưa biết.
 */

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_capQuyenRa,
  identity_maTranQuyenRa,
  identity_nhomQuyenRa,
  identity_vaiTroCotRa,
} from "@/lib/api/schema.gen";

/** Bảng tra các ô đã cấp: id vai trò → tập khoá quyền vai trò ấy đang giữ. */
export type BangDaCap = ReadonlyMap<string, ReadonlySet<string>>;

/** Ma trận thiếu trục nào. Hai trục hỏng vì hai lý do khác nhau nên câu chữ cũng khác nhau. */
export type ThieuTruc = "vaiTro" | "danhMucQuyen" | "caHai";

export type TrangThaiMaTran =
  | { pha: "dangDoc" }
  | { pha: "khongDocDuoc"; thongBao: string }
  | { pha: "chuaCauHinh"; thieu: ThieuTruc }
  | {
      pha: "coDuLieu";
      /** Hàng, đã gom theo nhóm, GIỮ NGUYÊN thứ tự máy chủ trả (`thu_tu` trong danh mục). */
      nhom: readonly identity_nhomQuyenRa[];
      /** Cột, giữ nguyên thứ tự xã đã sắp. */
      vaiTro: readonly identity_vaiTroCotRa[];
      daCap: BangDaCap;
    };

/**
 * Phản hồi → trạng thái. `null` là **chưa đọc xong**, khác hẳn "đọc xong và hỏng" (xem `goi.ts`).
 *
 * KHÔNG SẮP XẾP LẠI TRỤC NÀO. Thứ tự hàng là thứ tự danh mục quyền được ghi trong migration, thứ
 * tự cột là thứ tự xã đã sắp vai trò; sắp lại ở đây cho "gọn" là lặng lẽ phủ quyết cả hai
 * (`service-identity/internal/http/quyen.go`, `gomTheoNhom`).
 */
export function trangThaiMaTran(kq: KetQua<identity_maTranQuyenRa> | null): TrangThaiMaTran {
  if (kq === null) return { pha: "dangDoc" };
  if (!kq.ok) return { pha: "khongDocDuoc", thongBao: kq.thongBao };

  const { groups, roles, grants } = kq.duLieu;
  const thieuVaiTro = roles.length === 0;
  const thieuDanhMuc = groups.length === 0;

  // MỘT TRONG HAI TRỤC RỖNG LÀ ĐÃ KHÔNG DỰNG ĐƯỢC MA TRẬN, nên nhánh này bắt cả hai — nhưng nó
  // mang theo trục nào thiếu, vì hai ca ấy có hai người sửa khác nhau: không có vai trò là việc
  // của xã, còn danh mục quyền rỗng là chuyện của danh mục dùng chung toàn hệ thống.
  if (thieuVaiTro || thieuDanhMuc) {
    const thieu: ThieuTruc =
      thieuVaiTro && thieuDanhMuc ? "caHai" : thieuVaiTro ? "vaiTro" : "danhMucQuyen";
    return { pha: "chuaCauHinh", thieu };
  }

  return { pha: "coDuLieu", nhom: groups, vaiTro: roles, daCap: dungBangDaCap(grants) };
}

/**
 * Danh sách ô đã cấp → bảng tra. Dựng MỘT lần cho cả màn hình, không dò mảng ở từng ô.
 *
 * Một ma trận 33 hàng × 8 cột là 264 ô; tìm tuyến tính trong danh sách `grants` ở mỗi ô là 264
 * lượt quét, và con số ấy đi lên theo cả hai trục chứ không đứng yên.
 *
 * MỘT `Map<vai trò, Set<quyền>>` CHỨ KHÔNG PHẢI MỘT KHOÁ GHÉP `"<vai trò>|<quyền>"`: khoá ghép
 * cần một ký tự ngăn cách không bao giờ xuất hiện trong hai giá trị — một điều kiện không ai
 * kiểm lại vào ngày mã quyền đổi dạng.
 */
export function dungBangDaCap(danhSach: readonly identity_capQuyenRa[]): BangDaCap {
  const bang = new Map<string, Set<string>>();
  for (const o of danhSach) {
    let tap = bang.get(o.role_id);
    if (tap === undefined) {
      tap = new Set<string>();
      bang.set(o.role_id, tap);
    }
    tap.add(o.permission);
  }
  return bang;
}

/**
 * Ô `(vai trò, quyền)` đã cấp chưa. So sánh chuỗi chính xác, không tiền tố, không ký tự thay thế
 * — cùng lý do đã ghi ở `lib/quyen.ts`: `task.approve` cố ý không phải `task.extend`.
 *
 * KHÔNG TÌM THẤY LÀ **CHƯA CẤP**, không phải "chưa biết": `?? false` ở đây là mặc định ĐÓNG, đúng
 * chiều luật 1 đòi trên đường cách ly. Và nó chỉ quyết định một dấu hiệu trên màn hình — cái
 * chặn thật là `RequirePermission` ở máy chủ, trên từng yêu cầu của từng tuyến nghiệp vụ.
 *
 * Một ô đã cấp mà vai trò của nó không nằm trong danh sách cột thì KHÔNG hiện ở đâu cả, và cũng
 * không có chỗ nào để hiện: bảng chỉ có cột cho những vai trò máy chủ trả về. Đó là lý do hợp
 * đồng gửi ô kèm cả hai đầu của nó thay vì một ma trận dày theo vị trí.
 */
export function oDaCap(bang: BangDaCap, vaiTroId: string, khoaQuyen: string): boolean {
  return bang.get(vaiTroId)?.has(khoaQuyen) ?? false;
}
