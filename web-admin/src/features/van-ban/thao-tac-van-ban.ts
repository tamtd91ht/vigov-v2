/**
 * Ba thao tác ghi mà màn hình KIỂM TRƯỚC KHI GỌI MẠNG — và đây là chỗ duy nhất chúng được gọi.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO KHÔNG ĐỂ PHÉP KIỂM NẰM TRONG COMPONENT: một `if (lyDo === "") return;` giữa thân một
 * `useCallback` là một phép kiểm KHÔNG có bài test nào chạm tới được — muốn kiểm nó phải dựng
 * trình duyệt giả lập, bấm nút, rồi đếm `fetch`. Gỡ nó đi thì mọi bài kiểm vẫn xanh, và hậu quả
 * chỉ hiện ra ở một lời gọi chắc chắn bị máy chủ từ chối.
 *
 * Gom thành ba hàm ở đây thì phép kiểm ấy nằm trên đường đi DUY NHẤT tới lời gọi mạng: bài kiểm
 * gọi đúng hàm component gọi, thay `fetch`, và ĐẾM số lần nó được gọi. Xoá phép kiểm là bài kiểm
 * đỏ ngay, vì `fetch` chạy một lần đáng lẽ không được chạy.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ HÀM NÀO CHO "VÀO SỔ" VÀ "SỬA" Ở ĐÂY, có chủ ý: ngoài các ô bắt buộc mà máy chủ kiểm
 * kèm câu tiếng Việt nói rõ phải sửa gì, hai thao tác ấy không có phép kiểm nào của riêng client.
 * Dựng một lớp bọc rỗng cho chúng là mời người sau chép luật nghiệp vụ của máy chủ xuống đây
 * (luật 9, cấm #2).
 */

import type { KetQua } from "@/lib/api/goi";
import type { documents_vanBanDenRa } from "@/lib/api/schema.gen";
import { chuyenVanBanDen, goVanBanDen, goVanBanDi } from "@/lib/api/van-ban";

import { soanChuyen, soanGo, type BanChuyen } from "./nhan-van-ban";

/**
 * Một lượt bấm “Chuyển và ghi vết”.
 *
 * LÝ DO RỖNG THÌ KHÔNG CÓ LỜI GỌI MẠNG NÀO. Máy chủ cũng từ chối, nhưng chặn ở đây có hai cái
 * được mà chặn ở máy chủ không có: cán bộ biết ngay còn thiếu ô nào, và — quan trọng hơn — không
 * có một yêu cầu nào đi ra với ý định chuyển một hồ sơ mà không nói được vì sao.
 */
export function guiChuyenVanBan(
  id: string,
  ban: BanChuyen,
): Promise<KetQua<documents_vanBanDenRa>> {
  const soan = soanChuyen(ban);
  if (!soan.ok) return Promise.resolve({ ok: false, thongBao: soan.loi });
  return chuyenVanBanDen(id, soan.than);
}

/**
 * Một lượt bấm “Xác nhận gỡ khỏi sổ” ở sổ văn bản đến.
 *
 * LÝ DO LÀ BẮT BUỘC, và nó được lưu vĩnh viễn: `delete_reason` là câu trả lời duy nhất cho câu hỏi
 * "vì sao quyển sổ thiếu số này" — số đã cấp không quay về dãy (luật 7, bất biến 3).
 */
export function guiGoVanBanDen(id: string, lyDo: string): Promise<KetQua<null>> {
  const soan = soanGo(lyDo);
  if (!soan.ok) return Promise.resolve({ ok: false, thongBao: soan.loi });
  return goVanBanDen(id, soan.than.reason);
}

/** Một lượt bấm “Xác nhận gỡ khỏi sổ” ở sổ văn bản đi. Cùng phép kiểm, khác tuyến. */
export function guiGoVanBanDi(id: string, lyDo: string): Promise<KetQua<null>> {
  const soan = soanGo(lyDo);
  if (!soan.ok) return Promise.resolve({ ok: false, thongBao: soan.loi });
  return goVanBanDi(id, soan.than.reason);
}
