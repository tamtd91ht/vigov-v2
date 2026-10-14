/**
 * CUỘN TỚI MỘT MỎ NEO — và về ĐẦU màn khi không có mỏ neo nào. Cả bốn điều kiện dưới đây đều là
 * điều kiện, không phải khẩu vị.
 *
 * 1. **KHÔNG `behavior: "smooth"`.** Cuộn mượt là CHUYỂN ĐỘNG, và mọi chuyển động trong app này
 *    phải nằm sau rào `prefers-reduced-motion` (`styles.css`, `accessibility.test.ts` canh điều
 *    đó cho CSS). Một chuyển động sinh ra từ JavaScript đi vòng qua rào ấy: nó không có một dòng
 *    CSS nào để phép kiểm nhìn thấy, và người đã xin điện thoại của mình bớt chuyển động vẫn nhận
 *    đủ. Nhảy thẳng tới nơi thì không ai chóng mặt, và người lớn tuổi cũng không phải chờ.
 *
 * 2. **KHÔNG CÓ MỎ NEO, HOẶC MỎ NEO KHÔNG CÓ TRONG DOM → VỀ ĐẦU MÀN.** `.app-main` là khung cuộn,
 *    và vỏ app KHÔNG dựng lại nó khi đổi màn — nên vị trí cuộn của màn trước ở lại. Không đưa về
 *    đầu thì bấm một tab, hay mở một màn con, thả người dùng xuống giữa trang, nút "Quay lại" nằm
 *    khuất phía trên (lỗi B, 14/10/2026). `moc` là tên một màn CON (`MOC_QUAN_LY_QUYEN`) hay mã một
 *    giải pháp thì không có phần tử nào mang `id` ấy — màn cha mở màn con, và màn con mở từ đầu.
 *
 * 3. **CHẠY ĐƯỢC KHI KHÔNG CÓ DOM.** Bộ test dựng bằng `react-dom/server`, nơi `document` không
 *    tồn tại. Một tham chiếu trần tới `document` ở đây làm đổ những phép kiểm chẳng liên quan gì
 *    tới việc cuộn — và một ca đỏ vì lý do sai là một ca sắp bị ai đó tắt.
 *
 * 4. **TÌM KHUNG CUỘN BẰNG LỚP `.app-main`, KHÔNG BẰNG `#main`.** Trang của xã (`TrangXa.tsx`) cũng
 *    có một `<main id="main">`; lớp `.app-main` chỉ vỏ app ViHAT (`KhungApp`) mang.
 */
export function cuonToiMoc(moc: string | undefined): void {
  if (typeof document === "undefined") return;

  const o = moc === undefined || moc === "" ? null : document.getElementById(moc);
  // `scrollIntoView` vắng mặt trên vài WebView cũ và trong mọi môi trường test không có DOM thật.
  if (o && typeof o.scrollIntoView === "function") {
    o.scrollIntoView({ behavior: "auto", block: "start" });
    return;
  }
  scrollMainToTop();
}

/**
 * Đưa khung cuộn `.app-main` về đầu. Gọi TRƯỚC khi đổi màn con (Giải pháp → chi tiết, Liên hệ →
 * Quản lý quyền / Chính sách, và chiều quay lại): đặt `scrollTop = 0` lên nội dung cũ, nội dung
 * mới vẽ ra vẫn ở 0 — không cần chờ một lượt vẽ.
 */
export function scrollMainToTop(): void {
  if (typeof document === "undefined") return;
  const main = document.querySelector(".app-main");
  if (main) main.scrollTop = 0;
}
