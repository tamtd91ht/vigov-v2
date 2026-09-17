/**
 * Trang 404.
 *
 * Đây cũng là trang phục vụ cho một `Host` không khớp xã nào (`lib/tenant.server.ts`), nên câu
 * chữ ở đây phải KHÔNG nói gì về xã: không "xã này chưa đăng ký", không "tên miền không hợp
 * lệ", không gợi ý xã nào có thật. Ai gõ vào một host để dò xem xã nào tồn tại phải nhận đúng
 * câu mà người gõ nhầm một đường dẫn nhận được (`kb/00-foundation/multi-tenant-model.md`,
 * §Ranh giới tin cậy).
 */
export default function KhongTimThay() {
  return (
    <main className="trang-404">
      <h1>Không tìm thấy trang</h1>
      <p>Đường dẫn bạn truy cập không tồn tại.</p>
    </main>
  );
}
