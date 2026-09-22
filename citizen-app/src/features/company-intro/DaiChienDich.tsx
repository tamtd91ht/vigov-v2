import { chienDichCua, KHOA_CHIEN_DICH } from "../../content/chien-dich";
import { thamSo, thamSoMoApp } from "../../lib/launch-params";

/**
 * DẢI CHIẾN DỊCH — một lời chào hợp cảnh cho người vừa tới từ một mã QR đã biết.
 *
 * ⚠ HAI CÂU TRÊN DẢI ĐỀU LẤY TỪ BẢNG TRONG BUNDLE, KHÔNG MỘT KÝ TỰ NÀO TỪ THAM SỐ.
 *
 *   Tham số mở app là dữ liệu client tự đặt (luật 1, cấm #2): người gửi đường liên kết chọn nội
 *   dung của nó. In nó ra màn hình là để người ngoài viết chữ lên một màn hình đứng tên ViHAT
 *   Group. Nên `ma` ở đây chỉ được dùng đúng một việc — TRA BẢNG — và thứ hiện ra luôn là chữ của
 *   dòng bảng khớp được. Mã lạ thì `chienDichCua` trả `null` và hàm này trả `null`: không dải,
 *   không câu lỗi, không dấu vết gì rằng có một tham số.
 *
 *   `content/chien-dich.ts` giữ bảng và lý do; `chien-dich.test.tsx` cho cả hai nửa ăn đúng hình
 *   dạng tấn công (mã lạ · mã là tên một thuộc tính nguyên mẫu · một câu chèn chữ).
 *
 * ⚠ KHÔNG CÓ NÚT NÀO TRÊN DẢI. Một chiến dịch mời gọi đúng một việc: gắn vào đó một nút "nhận ưu
 *   đãi" hay "đăng ký". Giai đoạn A không có tuyến nào nhận, nên một nút như thế là một nút hứa
 *   thứ chưa có. Dải này chỉ chào; sáu ô menu ngay dưới nó mới là nơi đi tiếp.
 *
 * ĐỌC ĐỒNG BỘ, KHÔNG SDK, KHÔNG HIỆU ỨNG: `thamSoMoApp` đọc `location.search` và đã bọc try/catch,
 * nên ngoài trình duyệt (vitest, lúc dựng tĩnh) nó trả về "không có tham số" và dải không hiện.
 */
export function DaiChienDich() {
  const chien_dich = chienDichCua(thamSo(thamSoMoApp())[KHOA_CHIEN_DICH] ?? "");
  if (chien_dich === null) return null;

  return (
    <aside className="dai-cd">
      {/* `<p>` chứ không `<h2>`: dải này là một lời chào, không phải một mục của màn chủ. Một tiêu
          đề bậc hai ở đây chen vào giữa `<h1>` tên pháp nhân và `<h2>` "Giải pháp nổi bật", và
          trình đọc màn hình thông báo một mục không có nội dung nào thuộc về nó. */}
      <p className="dai-cd__tieu-de">{chien_dich.tieu_de}</p>
      <p className="dai-cd__chao">{chien_dich.cau_chao}</p>
    </aside>
  );
}
