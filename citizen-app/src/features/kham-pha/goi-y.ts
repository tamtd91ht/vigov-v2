import { PRESET_SOURCES } from "../../lib/launch-params";

/**
 * LỚP KHÁM PHÁ (ADR 0005 · 0044 · 0047) — quy tắc đọc một đường liên kết, tách hẳn khỏi màn hình.
 *
 * ĐÂY LÀ NƠI CÂU "THAM SỐ KHÔNG CHỌN XÃ" ĐƯỢC VIẾT RA THÀNH MÃ, nên nó nằm trong một hàm thuần
 * và có test riêng: một quy tắc chỉ tồn tại bên trong JSX là một quy tắc không ai kiểm được.
 *
 * ADR 0005 tách ba lớp và cấm gộp:
 *
 *   Khám phá  — công dân MUỐN làm việc với xã nào.  QR · deep link.        KHÔNG tin được.
 *   Phiên     — phiên này ĐANG thao tác ở xã nào.   Máy chủ ghi sau khi công dân xác nhận.  Tin được.
 *   Uỷ quyền  — công dân này được đọc/ghi gì ở đó.  Quan hệ công dân↔xã + luật 4.  Tin được.
 *
 * ⚠ KHÔNG CÒN DANH MỤC XÃ, BỘ CHỌN XÃ, "ĐỔI XÃ" HAY GỢI Ý THEO GPS (ADR 0044 câu 4 · ADR 0047,
 * quyết định 27/09/2026). Một phiên, một xã; xã đến từ tên miền trên QR, được MÁY CHỦ tra ra tên
 * (`GET /api/v1/communes?host=`), rồi công dân xác nhận. Không có nguồn thứ hai.
 *
 * Tham số trên QR là **dữ liệu do client cung cấp**. Luật 1, cấm #2: nhận xã từ client là để một
 * client tự cấp quyền cho chính nó. Nên hàm này chỉ trả lời "màn hình nên dẫn người dùng đi đâu",
 * và không bao giờ trả lời "xã của phiên này là gì".
 */

/**
 * Xã như màn xác nhận cần: TÊN và TỈNH/THÀNH, nguyên văn máy chủ trả về.
 *
 * Hình dạng theo `GET /api/v1/communes?host=` (`name`, `province`). Không có mã xã: công dân xác
 * nhận theo TÊN, và một mã ở phía client là một thứ để ai đó gửi ngược lên như thể nó cấp quyền.
 */
export type XaGoiY = { readonly ten: string; readonly tinh: string };

/**
 * MỨC TIN THEO NGUỒN.
 *
 * Người đang đứng ở trụ sở xã và vừa quét mã QR dán trên bảng tin thì một chạm xác nhận là đủ.
 * Người mở một liên kết ai đó chuyển cho thì không: một liên kết chuyển tay nói lên ý định của
 * NGƯỜI GỬI, không nói gì về người nhận — nên nó không mở màn xác nhận, và app mở phần giới thiệu.
 *
 * Bảng nguồn tin được nằm ở `lib/launch-params.ts` (`NGUON_CHON_SAN`), vì chính nó quyết app có đi
 * hỏi máy chủ hay không. Ở đây đọc lại cùng bảng ấy — một nguồn, không chép.
 */

export type GoiY =
  /** Nguồn đủ tin VÀ máy chủ đã tra ra xã: một chạm xác nhận. */
  | { kieu: "chon-san"; xa: XaGoiY; nguon: string }
  /** Mọi trường hợp còn lại: không gợi ý xã nào, app mở phần giới thiệu. */
  | { kieu: "khong-co" };

/**
 * Đọc (nguồn, xã máy chủ tra được) thành một gợi ý.
 *
 * FAIL CLOSED Ở HAI CHỖ, và cả hai đều cố ý:
 *
 *   1. `src` không nằm trong danh sách tin được → không gợi ý. Không có mặc định "coi như qr".
 *   2. Máy chủ không tra ra xã (`null`) → không gợi ý. **Không bao giờ hiện một cái tên đoán ra**:
 *      công dân xác nhận theo tên xã, nên một tên sai ở bước này là một hồ sơ gửi sang cơ quan khác.
 *
 * `xa_tra_duoc` là KẾT QUẢ CỦA MÁY CHỦ (`GET /api/v1/communes?host=`, gọi từ
 * `cong-dan/man/XacNhanXa.tsx`), không phải tham số trên QR.
 */
export function phanGiaiGoiY(nguon: string, xa_tra_duoc: XaGoiY | null): GoiY {
  if (xa_tra_duoc === null) return { kieu: "khong-co" };
  if (xa_tra_duoc.ten.trim() === "") return { kieu: "khong-co" };
  if (!PRESET_SOURCES.has(nguon)) return { kieu: "khong-co" };
  return { kieu: "chon-san", xa: xa_tra_duoc, nguon };
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

/** Tiêu đề header trong lúc công dân đang xác nhận xã. */
export const TIEU_DE_XAC_NHAN_XA = "Xác nhận xã";
