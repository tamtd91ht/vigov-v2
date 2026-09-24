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
 *      nửa theo cấu tạo; không phiên, không số điện thoại, không xã đã chọn được ghi xuống máy.
 *   3. MỖI LỜI GỌI SDK KHAI MỤC ĐÍCH TẠI CHỖ (`KHAI_BAO_LOI_GOI`). Nửa này hôm nay KHÔNG gọi SDK nào.
 *
 * ⚠ CỬA NÀY ĐỨNG SAU `resolve.alias` — `bien-the/cong-dan` (24/09/2026).
 *
 *   Hai màn "Gửi phản ánh" và "Tra cứu phiếu" đã dựng, nhưng CẦU PHIÊN CÔNG DÂN ViGov chưa có
 *   (`api/phien-vigov.ts`, mục sổ `citizen-app/cau-phien-cong-dan-vigov`), nên chúng KHÔNG được đi
 *   vào bản nộp. Bản `goc` trỏ cửa này sang `index.rong.ts` — không vẽ gì, không chuỗi nào, không
 *   lời gọi mạng nào — và `bundle-for-zalo.test.ts` dựng thật rồi đọc bundle để chứng minh.
 *   Bản `day-du` (thử nghiệm) mang hai màn, và cả hai nói "kênh chưa mở" vì phiên luôn `null`.
 *
 *   `App.tsx` nhập qua `bien-the/cong-dan`, KHÔNG nhập thẳng một tệp bên trong: một đường nhập
 *   thẳng đi vòng qua alias và mang cả kênh vào bản nộp mà không có gì báo (`bien-the.test.ts`).
 */
export { KenhCongDan, NutVaoKenhCongDan } from "./man/KenhCongDan";

/** Kênh công dân có mặt trong bản dựng này hay không. Bản rỗng trả `false`. */
export const CO_KENH_CONG_DAN: boolean = true;
