/**
 * NGUỒN DUY NHẤT CỦA PHIÊN CÔNG DÂN ViGov — TRONG BỘ NHỚ, và `null` cho tới khi cầu phiên phát một phiên.
 *
 * ⚠ `null` LÀ FAIL-CLOSED CÓ CHỦ ĐÍCH, KHÔNG PHẢI MỘT CHỖ CÒN DỞ ĐỂ AI ĐÓ "NỐI TẠM".
 *
 *   Tuyến của ViGov (`/api/v1/my-citizen-reports`) nhận một bearer do CHÍNH ViGov phát hành: rìa
 *   công dân (`core/httpx/citizen.go`, `CitizenEdge`) tra token qua `identity` và lấy danh tính
 *   cùng xã TỪ PHIÊN ẤY (ADR 0022). Phiên ấy đến qua cầu phiên của ADR 0045: `vihat-miniapp` đổi
 *   token Zalo rồi chuyển NGUYÊN VĂN phiên ViGov về, dưới khoá riêng `vigovSession`
 *   (`vihat-miniapp` `internal/httpapi/sessions_vigov.go:158-175`). Mọi màn hình của nửa nhà nước
 *   đọc `null` là "kênh chưa mở" rồi DỪNG — không gọi mạng.
 *
 * ⚠ MỘT NGƯỜI GHI, MỘT CHỖ: `api/open-vigov-session.ts`, SAU hành vi xác nhận xã của công dân, với thứ
 *   hàm mở phiên (tiêm vào từ `App.tsx`) trả về. `index.ts` KHÔNG xuất `setVigovSession` — lớp vỏ và
 *   nửa thương mại không có đường nào "đưa" một phiên vào đây.
 *
 * BA ĐƯỜNG TẮT BỊ CẤM, mỗi cái trông vô hại trong một diff:
 *
 *   1. Đọc phiếu phiên của khối đăng nhập (`features/log-in/`, khoá `token` ở GỐC thân trả lời của
 *      `/api/v1/sessions`). Phiếu ấy do `vihat-miniapp` KÝ cho một khách hàng doanh nghiệp. Nó KHÔNG
 *      phải phiên công dân ViGov và KHÔNG BAO GIỜ được gửi tới tuyến của ViGov (ADR 0032: bề mặt
 *      thuộc kho nào là do KHOÁ NÀO KÝ nó quyết định). Chỉ `vigovSession.token` — do ViGov ký — vào đây.
 *   2. Suy ra phiên từ dữ liệu Zalo (`getAccessToken`, `getPhoneNumber`, mã người dùng Zalo). Danh
 *      tính công dân là thứ MÁY CHỦ phát hành sau khi xác minh (ADR 0020), không phải thứ client
 *      tự dựng từ một mã nền tảng.
 *   3. Lấy xã từ tham số QR / deep link (`d`, `src`). Tham số ấy DẪN GIAO DIỆN, không cấp gì cả
 *      (ADR 0005 · 0019 · 0022 · 0047). `commune_name` dưới đây là tên máy chủ trả CÙNG phiên, không phải
 *      tên màn xác nhận đã hiện (ADR 0047 §Trả lời mục 4: "phiên nói thật").
 *
 * ⚠ KHÔNG LƯU XUỐNG MÁY. Một biến của mô-đun: đóng app là mất, mở lại là xác nhận lại
 *   (`two-halves-boundary.test.ts` §3b). Tệp này KHÔNG NHẬP GÌ (`citizen.test.tsx`).
 */

/**
 * Phiên công dân ViGov như nửa nhà nước cần: bearer để gửi đi, và tên xã để HIỆN RA.
 *
 * `commune_name` có mặt vì hai bất biến của kênh công dân (README §Non-negotiables #2 và #5): tên xã
 * hiện trên mọi màn, và được xác nhận lại ở bước cuối trước khi gửi. Xã ấy phải là xã CỦA PHIÊN —
 * xã mà máy chủ sẽ dùng để ghi phiếu — nếu không thì bước xác nhận đang xác nhận một thứ khác.
 *
 * KHÔNG CÓ MÃ XÃ, KHÔNG CÓ TÊN MIỀN: xã của phiên nằm ở máy chủ (ADR 0047 điều kiện dừng #1).
 */
export type VigovSession = {
  /** Bearer do ViGov phát hành. Chỉ sống trong bộ nhớ, không vẽ ra màn hình, không ghi log. */
  readonly token: string;
  /** Tên xã của phiên, nguyên văn máy chủ trả về (`tenantDisplayName`). */
  readonly commune_name: string;
};

let current_session: VigovSession | null = null;

/** Phiên công dân ViGov hiện tại, hoặc `null`. Không tham số. */
export function getVigovSession(): VigovSession | null {
  return current_session;
}

/**
 * Ghi — hoặc quên, với `null` — phiên hiện tại. CHỈ `open-vigov-session.ts` gọi hàm này.
 *
 * Phiên thiếu bearer hoặc thiếu tên xã KHÔNG được ghi: một phiên không xã là phiên máy chủ cố ý không
 * phát token (`sessions_vigov.go:165-167`), và vẽ nó ra như "đã có phiên" là nói dối người dân.
 */
export function setVigovSession(session: VigovSession | null): void {
  current_session = session !== null && session.token !== "" && session.commune_name.trim() !== "" ? session : null;
}
