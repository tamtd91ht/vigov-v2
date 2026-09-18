/**
 * CỬA DUY NHẤT vào lớp khám phá — và nó tồn tại để lớp này **biến mất được**.
 *
 * VÌ SAO KHÔNG NHẬP THẲNG TỪNG TỆP:
 *
 *   Bản nộp Zalo duyệt (biến thể `goc`) không được chứa lớp khám phá, danh mục xã mẫu hay bảng
 *   chẩn đoán. Tree-shaking **không** làm được việc đó: một `import` tĩnh có mặt là mô-đun vào
 *   bundle, kể cả khi không nhánh nào gọi tới nó. Thứ làm được là `resolve.alias` — trỏ đúng
 *   cái tên `bien-the/kham-pha` sang `index.rong.ts` lúc dựng, và lúc ấy mã thật không có đường
 *   nào đi vào bundle.
 *
 *   Cả cơ chế ấy chỉ đứng được khi có **một** cửa. `App.tsx` nhập thẳng `./ChonXaScreen` là một
 *   đường vòng qua alias, và nó không báo lỗi gì cả: bản `goc` vẫn dựng xanh, vẫn chạy, và vẫn
 *   mang theo tám tên đơn vị hành chính đặt ra vào bản gửi duyệt. `bien-the.test.ts` là thứ giữ
 *   cho hai bản không lệch nhau; `bundle-for-zalo.test.ts` là thứ kiểm bản `goc` bằng cách dựng
 *   thật rồi đọc bundle, chứ không bằng lời hứa.
 *
 * → vite.config.ts (bảng biến thể) · README §"Hai biến thể bản dựng"
 */
export { ChonXaScreen } from "./ChonXaScreen";
export { GoiYXaScreen } from "./GoiYXaScreen";
export { TrangXaScreen } from "./TrangXaScreen";
export { phanGiaiGoiY } from "./goi-y";
export type { XaDemo } from "./demo-danh-muc-xa";

/**
 * Lớp khám phá có mặt trong bản dựng này hay không.
 *
 * `App.tsx` đọc cờ này thay vì tự đoán: bản rỗng trả `false`, và khi ấy tham số `t` trên đường
 * liên kết **không dẫn đi đâu cả** — app là đúng bốn màn giới thiệu. Không có cờ này thì nhánh
 * khám phá vẫn chạy, chỉ là dựng ra một màn hình trắng vì các màn đã bị thay bằng bản rỗng, và
 * một màn trắng là thứ người duyệt của Zalo thấy trước tiên.
 *
 * Kiểu ghi rõ `boolean` chứ không để suy ra `true`: bản rỗng phải gán được vào cùng một kiểu.
 */
export const CO_LOP_KHAM_PHA: boolean = true;
