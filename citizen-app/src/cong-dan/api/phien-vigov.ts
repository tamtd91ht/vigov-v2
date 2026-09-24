/**
 * NGUỒN DUY NHẤT CỦA PHIÊN CÔNG DÂN ViGov — và hôm nay nó LUÔN trả lời "chưa có phiên".
 *
 * ⚠ ĐÂY LÀ FAIL-CLOSED CÓ CHỦ ĐÍCH, KHÔNG PHẢI MỘT CHỖ CÒN DỞ ĐỂ AI ĐÓ "NỐI TẠM".
 *
 *   Tuyến của ViGov (`/api/v1/my-citizen-reports`) nhận một bearer do CHÍNH ViGov phát hành: rìa
 *   công dân (`core/httpx/citizen.go`, `CitizenEdge`) tra token qua `identity` và lấy danh tính
 *   cùng xã TỪ PHIÊN ẤY (ADR 0022). Hôm nay KHÔNG tuyến nào của ViGov phát hành phiên công dân —
 *   cây cầu "đăng nhập `vihat-miniapp` → phiên công dân ViGov" chưa tồn tại. Mục sổ tiến độ:
 *   `citizen-app/cau-phien-cong-dan-vigov`.
 *
 *   Nên hàm dưới trả `null` vô điều kiện, và mọi màn hình của nửa nhà nước đọc `null` là "kênh
 *   chưa mở" rồi DỪNG — không gọi mạng.
 *
 * BA ĐƯỜNG TẮT BỊ CẤM, mỗi cái trông vô hại trong một diff:
 *
 *   1. Đọc phiếu phiên của khối đăng nhập (`features/dang-nhap/`, `/api/v1/sessions`). Phiếu ấy do
 *      `vihat-miniapp` — backend THƯƠNG MẠI, kho khác — phát hành cho một khách hàng doanh nghiệp.
 *      Nó KHÔNG phải phiên công dân ViGov và KHÔNG BAO GIỜ được gửi tới tuyến của ViGov (ADR 0032:
 *      bề mặt thuộc kho nào là do KHOÁ NÀO KÝ nó quyết định). `ranh-gioi-hai-nua.test.ts` cấm nửa
 *      nhà nước nhập nửa thương mại; `cong-dan.test.ts` cấm riêng tệp này nhập bất cứ thứ gì.
 *   2. Suy ra phiên từ dữ liệu Zalo (`getAccessToken`, `getPhoneNumber`, mã người dùng Zalo). Danh
 *      tính công dân là thứ MÁY CHỦ phát hành sau khi xác minh (ADR 0020), không phải thứ client
 *      tự dựng từ một mã nền tảng.
 *   3. Lấy xã từ tham số QR / deep link (`t`, `src`). Tham số ấy DẪN GIAO DIỆN, không cấp gì cả
 *      (ADR 0005 · 0019 · 0022). Xã vào phiên chỉ bằng hành vi xác nhận của công dân, và MÁY CHỦ
 *      ghi nó.
 *
 * NGÀY CẦU PHIÊN CÓ: hàm này đọc phiên từ nơi giữ nó TRONG BỘ NHỚ (không `localStorage` — xem
 * `ranh-gioi-hai-nua.test.ts` §3b), và `ten_xa` là tên xã MÁY CHỦ trả về cùng phiên — không phải xã
 * lớp khám phá gợi ý. Đổi tệp này là đổi đúng một chỗ; mọi màn hình đã đọc qua nó.
 */

/**
 * Phiên công dân ViGov như nửa nhà nước cần: bearer để gửi đi, và tên xã để HIỆN RA.
 *
 * `ten_xa` có mặt vì hai bất biến của kênh công dân (README §Non-negotiables #2 và #5): tên xã
 * hiện trên mọi màn, và được xác nhận lại ở bước cuối trước khi gửi. Xã ấy phải là xã CỦA PHIÊN —
 * xã mà máy chủ sẽ dùng để ghi phiếu — nếu không thì bước xác nhận đang xác nhận một thứ khác.
 *
 * ⚠ HÌNH DẠNG NÀY LÀ GIẢ ĐỊNH của phía client, vì hợp đồng phát hành phiên chưa có. Cầu phiên có
 * hình dạng khác thì sửa kiểu này — không dựng thêm một kiểu thứ hai bên cạnh.
 */
export type PhienViGov = {
  /** Bearer do ViGov phát hành. Chỉ sống trong bộ nhớ, không vẽ ra màn hình, không ghi log. */
  readonly token: string;
  /** Tên xã của phiên, nguyên văn máy chủ trả về. */
  readonly ten_xa: string;
};

/**
 * Phiên công dân ViGov hiện tại, hoặc `null`.
 *
 * `null` HÔM NAY VÀ MỌI LẦN GỌI — xem khối chú thích đầu tệp. Không tham số: không có gì bên ngoài
 * được phép "đưa" một phiên vào đây.
 */
export function layPhienViGov(): PhienViGov | null {
  return null;
}
