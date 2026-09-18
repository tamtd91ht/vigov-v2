/**
 * LỚP KHÁM PHÁ (ADR 0005) — quy tắc đọc một đường liên kết, tách hẳn khỏi màn hình.
 *
 * ĐÂY LÀ NƠI CÂU "THAM SỐ KHÔNG CHỌN XÃ" ĐƯỢC VIẾT RA THÀNH MÃ, nên nó nằm trong một hàm thuần
 * và có test riêng: một quy tắc chỉ tồn tại bên trong JSX là một quy tắc không ai kiểm được.
 *
 * ADR 0005 tách ba lớp và cấm gộp:
 *
 *   Khám phá  — công dân MUỐN làm việc với xã nào.  QR · deep link · GPS · picker.  KHÔNG tin được.
 *   Phiên     — phiên này ĐANG thao tác ở xã nào.   Máy chủ ghi sau khi công dân xác nhận.  Tin được.
 *   Uỷ quyền  — công dân này được đọc/ghi gì ở đó.  Quan hệ công dân↔xã + luật 4.  Tin được.
 *
 * Tham số trên QR là **dữ liệu do client cung cấp**. Luật 1, cấm #2: nhận xã từ client là để một
 * client tự cấp quyền cho chính nó. Nên hàm này chỉ trả lời "màn hình nên dẫn người dùng đi đâu",
 * và không bao giờ trả lời "xã của phiên này là gì".
 */
import { DEMO_timTheoMa, type XaDemo } from "./demo-danh-muc-xa";

/**
 * MỨC TIN THEO NGUỒN — bảng này là phần đáng giá nhất của lớp khám phá.
 *
 * Người đang đứng ở trụ sở xã và vừa quét mã QR dán trên bảng tin thì không nên bị bắt đi tìm
 * lại chính cái xã mình đang đứng trong đó. Người mở một liên kết ai đó chuyển cho thì phải
 * chọn: một liên kết chuyển tay nói lên ý định của NGƯỜI GỬI, không nói gì về người nhận.
 *
 * `share` ở đây không phải "kém tin hơn một chút" — nó là một câu hỏi khác hẳn.
 */
const NGUON_CHON_SAN = new Set(["qr", "zns"]);

/** Lý do vì sao phải chọn tường minh. Mỗi lý do có một câu nói với công dân — xem `LOI_NHAN`. */
export type LiDoPhaiChon = "nguon-yeu" | "khong-tra-duoc";

export type GoiY =
  /** Nguồn đủ tin VÀ tra được tên xã: chọn sẵn, một chạm xác nhận. */
  | { kieu: "chon-san"; xa: XaDemo; nguon: string }
  /** Mọi trường hợp còn lại: danh mục xã, chọn tường minh. */
  | { kieu: "phai-chon"; li_do: LiDoPhaiChon };

/**
 * Đọc cặp (`t`, `src`) thành một gợi ý.
 *
 * FAIL CLOSED Ở HAI CHỖ, và cả hai đều cố ý:
 *
 *   1. `src` không nằm trong danh sách tin được → bắt chọn. Không có mặc định "coi như qr".
 *   2. Tra mã không ra xã → bắt chọn. **Không bao giờ hiện một cái tên đoán ra**: công dân xác
 *      nhận theo tên xã, nên một tên sai ở bước này là một hồ sơ gửi sang cơ quan khác.
 *
 * Cả hai nhánh đều dẫn tới cùng một màn danh mục, nên người dùng không bao giờ rơi vào ngõ cụt.
 */
export function phanGiaiGoiY(maXa: string, nguon: string): GoiY {
  const xa = maXa ? DEMO_timTheoMa(maXa) : null;
  if (!xa) return { kieu: "phai-chon", li_do: "khong-tra-duoc" };
  if (!NGUON_CHON_SAN.has(nguon)) return { kieu: "phai-chon", li_do: "nguon-yeu" };
  return { kieu: "chon-san", xa, nguon };
}

/**
 * Câu mô tả nguồn, bằng tiếng Việt của người dân chứ không phải bằng khoá kỹ thuật.
 *
 * `skills/accessibility-elderly`: không bao giờ hiện khoá nội bộ (`qr`, `zns`) cho công dân đọc.
 * Và trạng thái "vì sao xã này được chọn sẵn" được nói bằng CHỮ — không bằng màu, không bằng
 * riêng một biểu tượng (README §Non-negotiables #6).
 */
export function nhanNguon(nguon: string): string {
  if (nguon === "qr") return "Bạn vừa quét mã QR tại trụ sở xã";
  if (nguon === "zns") return "Đường liên kết do xã gửi cho bạn";
  return "Đường liên kết bạn vừa mở";
}

/** Câu giải thích vì sao màn danh mục hiện ra. Nói việc cần làm, không nói mã lỗi (luật: README §7). */
export const LOI_NHAN: Record<LiDoPhaiChon, string> = {
  "nguon-yeu":
    "Đường liên kết này được chuyển tay nên chưa đủ để chọn xã thay bạn. Bạn hãy chọn xã cần liên hệ.",
  "khong-tra-duoc":
    "Bạn hãy chọn xã cần liên hệ để tiếp tục.",
};
