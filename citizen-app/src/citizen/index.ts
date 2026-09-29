/**
 * `src/citizen/` — NỬA NHÀ NƯỚC của Mini App. Tệp này là CỬA DUY NHẤT vào nó.
 *
 * Một App ID, một bundle, một origin — HAI NỬA NGHIỆP VỤ:
 *
 *   Thương mại   `src/content/` · `features/company-intro/` · `features/tinh-nang/`
 *                `features/log-in/` · `api/`     -> khách hàng doanh nghiệp của ViHAT Group
 *   Nhà nước     `src/citizen/` (thư mục này)      -> công dân làm việc với xã/phường
 *
 * BA RÀNG BUỘC, MỖI CÁI LÀ MỘT CA TEST (`src/two-halves-boundary.test.ts`):
 *
 *   1. RANH GIỚI HAI CHIỀU ĐỀU CẤM, và nặng nhất: KHÔNG TỆP NÀO NGOÀI `./citizen/` NHẬP CLIENT API
 *      CỦA ViGov (`./citizen/api/`) — kể cả `App.tsx`.
 *   2. KHÔNG LƯU TRỮ ĐỊNH DANH. `localStorage` · `sessionStorage` · `IndexedDB` là chung giữa hai
 *      nửa theo cấu tạo; không phiên, không số điện thoại, không xã đã chọn được ghi xuống máy. MỘT
 *      ngoại lệ, và nó không nằm trong nửa này: nháp phản ánh của APP RIÊNG của xã (ADR 0050 #7) do lớp
 *      vỏ tiêm vào (`FeedbackDraftStore`); tệp duy nhất chạm kho là `src/commune-app/feedback-draft-store.ts`.
 *   3. MỖI LỜI GỌI SDK KHAI MỤC ĐÍCH TẠI CHỖ (`KHAI_BAO_LOI_GOI`). Nửa này hôm nay KHÔNG gọi SDK nào.
 *
 * ⚠ KHÔNG CÒN SAU `resolve.alias` (27/09/2026): bản dựng chỉ còn một, và nó mang kênh này. Kênh
 *   MỞ chỉ khi cầu phiên phát một phiên ViGov sau khi công dân xác nhận xã (`screens/CommuneConfirmation.tsx`);
 *   không có phiên thì ba màn phản ánh nói "kênh chưa mở" và không gọi mạng. `App.tsx` nhập qua đúng
 *   tệp này, không nhập thẳng một tệp bên trong.
 *
 * ⚠ CỬA NÀY KHÔNG XUẤT `setVigovSession` HAY BẤT CỨ HÀM NÀO CỦA `api/`. Thứ đi ra là MÀN và KIỂU của
 *   hàm mở phiên mà `App.tsx` phải tiêm vào (`api/open-vigov-session.ts`) — kiểu không mang theo đường
 *   gọi nào.
 */
export { CitizenChannel, CitizenChannelButton } from "./screens/CitizenChannel";
export { CommuneHome, type AppCommune } from "./screens/CommuneHome";
export type { FeedbackDraftStore, NameResult, GetZaloName, ReportDraft } from "./screens/commune-app-model";
export type { GetSceneLocation, SceneLocationFailure, SceneLocationResult } from "./screens/scene-location";
export { type ConfirmationOutcome, CommuneConfirmation } from "./screens/CommuneConfirmation";
export type {
  CommuneAppSessionResult,
  SessionOpenResult,
  OpenCommuneAppSession,
  OpenVigovSession,
  ReopenWithPhone,
  ReopenWithPhoneResult,
  SessionOpenRequest,
} from "./api/open-vigov-session";
