/**
 * Khai kiểu tại chỗ cho ĐÚNG MỘT hàm của `react-dom/server`.
 *
 * VÌ SAO KHÔNG CÀI `@types/react-dom`: đó là thêm một phụ thuộc vào `package.json`, tức đổi đầu
 * vào của `npm ci` trong Dockerfile và của mọi lượt dựng ảnh — một thay đổi thuộc về người giữ
 * hạ tầng, không phải thứ tiện tay thêm để chạy được một ca test. `react-dom` đã nằm trong
 * `dependencies` ở RUNTIME; thiếu duy nhất là phần kiểu.
 *
 * PHẠM VI HẸP CÓ CHỦ Ý: chỉ `renderToStaticMarkup`, chỉ chữ ký đang dùng. Một tệp khai kiểu rộng
 * viết tay là một bản sao của API bên thứ ba, và bản sao ấy trôi khỏi bản thật trong im lặng —
 * đúng thứ luật 9 cấm. Khai hẹp thì ngày API đổi, chỗ hỏng là dòng gọi, không phải một niềm tin
 * sai nằm trong tệp này.
 *
 * XOÁ TỆP NÀY nếu có ngày `@types/react-dom` được thêm thật: hai nguồn cho một kiểu là hai nguồn
 * sẽ mâu thuẫn, và trình biên dịch sẽ chỉ vào tệp này chứ không chỉ vào gói kiểu.
 */
declare module "react-dom/server" {
  import type { ReactElement } from "react";

  export function renderToStaticMarkup(element: ReactElement): string;
}
