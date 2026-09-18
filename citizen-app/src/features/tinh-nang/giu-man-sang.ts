/**
 * CẶP BẬT/TẮT CỦA `keepScreen`, TÁCH RA THÀNH MỘT HÀM THUẦN ĐỂ KIỂM ĐƯỢC.
 *
 * VÌ SAO MỘT VIỆC BA DÒNG LẠI CÓ TỆP RIÊNG:
 *
 *   "Giữ màn hình sáng" là trạng thái ở tầng HỆ ĐIỀU HÀNH, không thuộc về màn hình React đã bật
 *   nó. Bật rồi rời màn mà không tắt thì màn hình của người dùng sáng mãi tới khi họ đóng hẳn
 *   ứng dụng — ta lấy pin của họ cho một tính năng họ đã bỏ đi. Không có gì báo lỗi, không có
 *   test nào đỏ, và người phát hiện ra là người thấy máy mình nóng và hết pin giữa buổi.
 *
 *   Viết thẳng vào `useEffect` thì lời hứa "luôn tắt lại" chỉ kiểm được khi có DOM để tháo một
 *   component ra — bộ test ở đây dựng bằng `react-dom/server` và không có DOM. Tách ra thì hợp
 *   đồng của hiệu ứng (bật khi vào, LUÔN tắt khi ra) kiểm được bằng hai lời gọi hàm.
 *
 * ⚠ HÀM DỌN DẸP LUÔN TẮT, KỂ CẢ KHI KHÔNG BẬT. Gọi `keepScreen({ keepScreenOn: false })` trên
 * một màn chưa từng bật là vô hại; bỏ sót một lần tắt thì không.
 */

/**
 * Hợp đồng của hiệu ứng giữ màn sáng.
 *
 * @param bat  người dùng đang bật chế độ giữ màn sáng hay không
 * @param dat  cách nói với nền tảng — thật ở màn hình, giả ở phép kiểm
 * @returns    hàm dọn dẹp mà React gọi khi rời màn hoặc khi `bat` đổi
 */
export function hieuUngGiuManSang(bat: boolean, dat: (bat: boolean) => void): () => void {
  if (bat) dat(true);
  return () => {
    dat(false);
  };
}
