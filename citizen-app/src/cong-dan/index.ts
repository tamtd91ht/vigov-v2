/**
 * `src/cong-dan/` — NỬA NHÀ NƯỚC của Mini App. Tệp này là CỬA DUY NHẤT vào nó.
 *
 * Một App ID, một bundle, một origin — HAI NỬA NGHIỆP VỤ:
 *
 *   Thương mại   `src/content/` · `features/company-intro/` · `features/tinh-nang/`
 *                `features/dang-nhap/` · `api/`     -> khách hàng doanh nghiệp của ViHAT Group
 *   Nhà nước     `src/cong-dan/` (thư mục này)      -> công dân làm việc với xã/phường
 *
 * BA RÀNG BUỘC, MỖI CÁI LÀ MỘT CA TEST (`src/ranh-gioi-hai-nua.test.ts`):
 *
 *   1. RANH GIỚI HAI CHIỀU ĐỀU CẤM, và nặng nhất: KHÔNG TỆP NÀO NGOÀI `./cong-dan/` NHẬP CLIENT API
 *      CỦA ViGov (`./cong-dan/api/`) — kể cả `App.tsx`.
 *   2. KHÔNG LƯU TRỮ ĐỊNH DANH. `localStorage` · `sessionStorage` · `IndexedDB` là chung giữa hai
 *      nửa theo cấu tạo; không phiên, không số điện thoại, không xã đã chọn được ghi xuống máy. MỘT
 *      ngoại lệ, và nó không nằm trong nửa này: nháp phản ánh của APP RIÊNG của xã (ADR 0050 #7) do lớp
 *      vỏ tiêm vào (`FeedbackDraftStore`); tệp duy nhất chạm kho là `src/app-rieng/feedback-draft-store.ts`.
 *   3. MỖI LỜI GỌI SDK KHAI MỤC ĐÍCH TẠI CHỖ (`KHAI_BAO_LOI_GOI`). Nửa này hôm nay KHÔNG gọi SDK nào.
 *
 * ⚠ KHÔNG CÒN SAU `resolve.alias` (27/09/2026): bản dựng chỉ còn một, và nó mang kênh này. Kênh
 *   MỞ chỉ khi cầu phiên phát một phiên ViGov sau khi công dân xác nhận xã (`man/XacNhanXa.tsx`);
 *   không có phiên thì ba màn phản ánh nói "kênh chưa mở" và không gọi mạng. `App.tsx` nhập qua đúng
 *   tệp này, không nhập thẳng một tệp bên trong.
 *
 * ⚠ CỬA NÀY KHÔNG XUẤT `datPhienViGov` HAY BẤT CỨ HÀM NÀO CỦA `api/`. Thứ đi ra là MÀN và KIỂU của
 *   hàm mở phiên mà `App.tsx` phải tiêm vào (`api/mo-phien-vigov.ts`) — kiểu không mang theo đường
 *   gọi nào.
 */
export { KenhCongDan, NutVaoKenhCongDan } from "./man/KenhCongDan";
export { TrangXa, type XaCuaApp } from "./man/TrangXa";
export type { FeedbackDraftStore, KetQuaLayTen, KetQuaViTri, LayMaViTri, LayTenZalo, NhapPhieu } from "./man/trai-nghiem";
export { type KetThucXacNhan, XacNhanXa } from "./man/XacNhanXa";
export type {
  KetQuaMoPhien,
  MoPhienViGov,
  ReopenWithPhone,
  ReopenWithPhoneResult,
  YeuCauMoPhien,
} from "./api/mo-phien-vigov";
