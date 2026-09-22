/**
 * Một xã mới đã khai đủ hai bảng BẮT BUỘC chưa — phép quyết định đứng sau khối cảnh báo ở đầu tab
 * "Thời hạn xử lý". Hàm thuần: không gọi mạng, không dựng DOM.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO KHỐI ẤY LÀ PHẦN ĐÁNG GIÁ NHẤT CỦA MÀN HÌNH, chứ không phải bảng số liệu.
 *
 * Chuỗi phía sau `POST /api/v1/incoming-documents` đi qua `identity.ResolveDeadlines`, và hàm ấy
 * TỪ CHỐI khi bảng thời hạn rỗng, TỪ CHỐI khi lịch làm việc rỗng. Một xã mới nhận hệ thống vì thế
 * không vào sổ được văn bản đến và không nhận được phản ánh — mà thứ hiện ra cho cán bộ chỉ là một
 * lỗi lúc bấm Lưu, ở một màn hình khác hẳn màn hình sửa được nó.
 *
 * Cả hai bảng ĐỀU đã có tuyến gieo idempotent. Thứ duy nhất còn thiếu là một chỗ để người ta biết
 * mình đang thiếu gì và bấm được. Đó là việc của khối này.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG ĐỌC ĐƯỢC THÌ KHÔNG KÊU, VÀ ĐÂY KHÔNG PHẢI MỘT LỖ HỔNG "fail open". Khối này là một LỜI
 * KHẲNG ĐỊNH về trạng thái dữ liệu của xã ("đơn vị chưa khai xong, nên đang không tiếp nhận
 * được"), không phải một cánh cổng chặn đường. Khẳng định một điều mình vừa không đọc được là nói
 * sai với một cơ quan nhà nước — và câu sai ấy sẽ đẩy cán bộ đi bấm nút gieo cho một bảng có thể
 * đang đầy đủ. Khi lượt đọc hỏng, thứ phải hiện là CÂU CỦA MÁY CHỦ (403 thiếu quyền, phiên hết
 * hạn, mạng hỏng), và màn hình hiện nó ở ngay bảng tương ứng.
 *
 * Phép đóng-khi-không-chắc thật sự nằm ở máy chủ và vẫn nguyên vẹn: không có cấu hình thì tuyến
 * tiếp nhận từ chối, bất kể màn hình này nói gì.
 */

import type { KetQua } from "@/lib/api/goi";

/** Trạng thái khai báo của MỘT bảng, theo đúng những gì màn hình biết chắc. */
export type TinhTrangBang =
  /** Đang đọc, hoặc đọc hỏng — màn hình KHÔNG biết bảng này rỗng hay đầy. */
  | "chuaBiet"
  /** Đọc được, và không có dòng nào. */
  | "trong"
  /** Đọc được, và có ít nhất một dòng. */
  | "daKhai";

/**
 * Kết quả một lượt đọc danh sách → trạng thái của bảng.
 *
 * `null` LÀ "CHƯA ĐỌC XONG", khác hẳn "đọc xong và rỗng". Gộp hai thứ này lại thì khối cảnh báo
 * nhấp nháy ở mọi lần mở trang của một xã đã cấu hình đầy đủ — và một lời báo động xuất hiện rồi
 * biến mất là lời báo động không ai còn tin.
 */
export function tinhTrangBang(kq: KetQua<{ items: readonly unknown[] }> | null): TinhTrangBang {
  if (kq === null || !kq.ok) return "chuaBiet";
  return kq.duLieu.items.length === 0 ? "trong" : "daKhai";
}

/** Khối cảnh báo hiện hay không, và nếu hiện thì đơn vị đang thiếu bảng nào. */
export type KhoiCanhBao =
  | { hien: false }
  | { hien: true; thieuThoiHan: boolean; thieuLichTuan: boolean };

/**
 * Hai bảng, một khối cảnh báo.
 *
 * HIỆN KHI **BẤT KỲ** BẢNG NÀO RỖNG, không phải khi cả hai rỗng: thiếu một trong hai là đã đủ để
 * `ResolveDeadlines` từ chối, nên một xã đã gieo thời hạn mà chưa gieo lịch vẫn đang không tiếp
 * nhận được gì. Điều kiện `&&` ở đây sẽ tạo ra đúng một trạng thái im lặng nguy hiểm nhất: màn
 * hình trông đã xong một nửa và không nói gì.
 */
export function khoiCanhBao(thoiHan: TinhTrangBang, lichTuan: TinhTrangBang): KhoiCanhBao {
  const thieuThoiHan = thoiHan === "trong";
  const thieuLichTuan = lichTuan === "trong";
  if (!thieuThoiHan && !thieuLichTuan) return { hien: false };
  return { hien: true, thieuThoiHan, thieuLichTuan };
}
