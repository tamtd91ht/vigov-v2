/**
 * Kiểm đường dẫn `tiep-tuc` trước khi chuyển hướng tới nó.
 *
 * Tham số này đến từ thanh địa chỉ, tức là từ bất kỳ ai gửi cho cán bộ một đường liên kết.
 * Không kiểm thì `/dang-nhap?tiep-tuc=https://ten-mien-gia.example` biến trang đăng nhập của
 * một cơ quan nhà nước thành bàn đạp chuyển hướng — đăng nhập xong, người dùng đang ở một
 * trang lạ mà vẫn tin là mình đang ở trong hệ thống của xã.
 *
 * Chỉ chấp nhận đường dẫn nội bộ. Nghi ngờ thì về trang chủ; không có ngoại lệ "cho tiện".
 */
export function duongDanTiepTuc(tho: string | null | undefined): string {
  if (typeof tho !== "string" || tho === "") return "/";

  // Phải bắt đầu bằng đúng một dấu "/". "//ten-mien" và "/\ten-mien" đều là URL tuyệt đối
  // sang máy chủ khác trong con mắt của trình duyệt, dù nhìn như đường dẫn nội bộ.
  if (!tho.startsWith("/")) return "/";
  if (tho.startsWith("//") || tho.startsWith("/\\")) return "/";

  // Chặn luôn vòng lại chính trang đăng nhập: đăng nhập xong mà quay về trang đăng nhập là một
  // vòng lặp người dùng không thoát ra được.
  if (tho === "/dang-nhap" || tho.startsWith("/dang-nhap?")) return "/";

  return tho;
}
