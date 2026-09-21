/**
 * CUỘN TỚI MỘT MỎ NEO — mười dòng, và cả ba điều kiện dưới đây đều là điều kiện, không phải khẩu vị.
 *
 * 1. **KHÔNG `behavior: "smooth"`.** Cuộn mượt là CHUYỂN ĐỘNG, và mọi chuyển động trong app này
 *    phải nằm sau rào `prefers-reduced-motion` (`styles.css`, `accessibility.test.ts` canh điều
 *    đó cho CSS). Một chuyển động sinh ra từ JavaScript đi vòng qua rào ấy: nó không có một dòng
 *    CSS nào để phép kiểm nhìn thấy, và người đã xin điện thoại của mình bớt chuyển động vẫn nhận
 *    đủ. Nhảy thẳng tới nơi thì không ai chóng mặt, và người lớn tuổi cũng không phải chờ.
 *
 * 2. **KHÔNG TÌM THẤY THÌ KHÔNG LÀM GÌ, và đó là một nhánh BÌNH THƯỜNG.** `moc` có thể là tên một
 *    màn CON chưa được vẽ (`MOC_QUAN_LY_QUYEN`) — lúc ấy không có phần tử nào để cuộn tới, và màn
 *    cha mới là chỗ xử lý nó. Ném lỗi ở đây là làm trắng màn hình vì một việc trang trí.
 *
 * 3. **CHẠY ĐƯỢC KHI KHÔNG CÓ DOM.** Bộ test dựng bằng `react-dom/server`, nơi `document` không
 *    tồn tại. Một tham chiếu trần tới `document` ở đây làm đổ những phép kiểm chẳng liên quan gì
 *    tới việc cuộn — và một ca đỏ vì lý do sai là một ca sắp bị ai đó tắt.
 */
export function cuonToiMoc(moc: string | undefined): void {
  if (moc === undefined || moc === "") return;
  if (typeof document === "undefined") return;

  const o = document.getElementById(moc);
  // `scrollIntoView` vắng mặt trên vài WebView cũ và trong mọi môi trường test không có DOM thật.
  if (o && typeof o.scrollIntoView === "function") {
    o.scrollIntoView({ behavior: "auto", block: "start" });
  }
}
