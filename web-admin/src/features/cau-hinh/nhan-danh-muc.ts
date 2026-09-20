/**
 * Câu chữ của tab "Danh mục". Hàm thuần, không gọi mạng, không dựng DOM — nên kiểm được đúng
 * những ca dễ nói sai nhất.
 *
 * Cùng khuôn với `nhan-can-bo.ts` và `nhan-ma-tran.ts`: chữ trên màn hình của một cơ quan nhà
 * nước là thứ có người phải trả lời, không phải chỗ để diễn đạt cho gọn. Nhãn lấy theo
 * `docs/ui-ux/14-cau-hinh.md §5`, không tự đặt lại.
 */

/**
 * Chip cột "Trạng thái" — `active`.
 *
 * HAI CHỮ NÀY LÀ CHỮ CỦA ĐẶC TẢ §5 (`Đang dùng` / `Đã tắt`), KHÔNG PHẢI CHỮ CỦA TAB NGƯỜI DÙNG
 * (`Đang hoạt động` / `Đã ngừng`). Hai màn hình nói về hai thứ khác nhau: ở đó là một CON NGƯỜI
 * còn công tác hay đã nghỉ, ở đây là một MÃ còn được chọn khi lập hồ sơ mới hay không. Mượn chữ
 * của nhau là mời người đọc suy ra một quan hệ không có.
 */
export function nhanTrangThaiMuc(active: boolean): string {
  return active ? "Đang dùng" : "Đã tắt";
}

/**
 * Lớp CSS của chip trạng thái. Màu KHÔNG phải tín hiệu duy nhất — chữ trong chip đã nói rõ ca
 * nào là ca nào, nên người không phân biệt được màu vẫn đọc ra (a11y, `15-phu-luc §8`).
 */
export function lopTrangThaiMuc(active: boolean): string {
  return active ? "chip chip-hoat-dong" : "chip chip-ngung";
}

/**
 * Cột "Mặc định" — `is_default`, mục mà biểu mẫu chọn sẵn.
 *
 * MỘT CHỮ CHO CẢ HAI CA, KHÔNG PHẢI MỘT DẤU GẠCH. Đặc tả vẽ badge `Mặc định` ở mục được chọn sẵn
 * và không vẽ gì ở những mục khác; một ô trống hoặc một dấu `—` đọc bằng trình đọc màn hình thì
 * thành im lặng, và người dùng không biết ô ấy trống vì chưa có dữ liệu hay vì mục này không phải
 * mặc định. Nút `Đặt mặc định` của đặc tả không có ở đây: nó là một thao tác GHI (xem `tab-danh-muc.tsx`).
 */
export function nhanMacDinh(laMacDinh: boolean): string {
  return laMacDinh ? "Mặc định" : "Không";
}

/** Số mục của một nhóm, đặt cạnh tên nhóm. Đếm cả mục đã tắt — xem `GIAI_THICH_DA_TAT`. */
export function nhanSoMuc(soMuc: number): string {
  return `${soMuc} mục`;
}

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * CÂU QUAN TRỌNG NHẤT CỦA MÀN HÌNH NÀY. Đọc kỹ trước khi sửa một chữ.
 *
 * Bảy danh mục hôm nay RỖNG ở mọi đơn vị và sẽ còn rỗng: migration cố ý không gieo mục nào, và
 * bước khởi tạo đơn vị — nơi những mục đầu tiên được lập — chưa tồn tại (`nhom-danh-muc.ts`).
 * Nên đây là đường THÔNG THƯỜNG, và câu chữ phải nói đủ HAI điều, vì thiếu điều nào cũng để lại
 * một người ngồi chờ hoặc đi tìm nút không có:
 *
 *   1. Đơn vị chưa có mục nào — hệ thống KHÔNG hỏng, không phải mất mạng, không phải mất quyền.
 *   2. Không thêm được từ màn hình này — để không ai đi tìm nút `+ Thêm mục` mà đặc tả có vẽ.
 *
 * TÊN NHÓM NẰM TRONG CHÍNH CÂU, không chỉ ở tiêu đề phía trên: trình đọc màn hình đọc từng đoạn
 * một, và một câu "Đơn vị chưa có mục nào trong danh mục này" tách khỏi tiêu đề thì không còn
 * biết đang nói về danh mục nào.
 *
 * HAI CÂU NGẮN, KHÔNG PHẢI MỘT CÂU DÀI: cán bộ đọc màn hình này gồm cả người lớn tuổi, và một
 * câu ghép nhiều mệnh đề là câu phải đọc lại lần hai (`skills/accessibility-elderly`).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function nhanNhomRong(tenNhom: string): string {
  return `Đơn vị chưa có mục nào trong danh mục ${tenNhom}. Màn hình này chỉ xem, không thêm được mục mới.`;
}

/**
 * Ghi chú đầu tab — vì sao không có nút thêm, sửa, tắt hay nhập Excel nào.
 *
 * NÓI LÝ DO THẬT, KHÔNG NÓI "SẮP CÓ". Lý do là câu hỏi mở #21 chưa được đơn vị chốt: đơn vị được
 * sửa cả DANH SÁCH MÃ hay chỉ được sửa nhãn và thứ tự (`kb/00-foundation/open-questions.json`).
 * "Tính năng đang phát triển" là một lời hứa không ai đặt ra và sẽ bị hỏi lại sau ba tháng.
 */
export const GHI_CHU_CHI_XEM =
  "Màn hình hiện chỉ xem. Thêm, sửa, tắt mục và nhập từ tệp Excel chưa mở vì chưa có quy định " +
  "đơn vị được sửa danh sách mã hay chỉ được sửa nhãn hiển thị và thứ tự.";

/**
 * Vì sao bảng vẫn liệt kê những mục đã tắt.
 *
 * Chỉ hiện khi trên màn hình ĐANG CÓ mục để mà giải thích — cùng lý do `BaoLoiDanhMuc` của tab
 * Người dùng đặt dòng báo cạnh bảng chứ không trên đầu màn hình: một câu giải thích đứng cạnh
 * bảy bảng rỗng không giải thích được gì cả.
 */
export const GIAI_THICH_DA_TAT =
  "Mục đã tắt vẫn nằm trong bảng: hồ sơ đã lập theo mã đó vẫn cần nhãn để hiển thị. Mục đã tắt " +
  "không còn được chọn khi lập hồ sơ mới.";

/**
 * Câu đi kèm nhóm "Mức ưu tiên nhiệm vụ" — nhóm duy nhất mà THỨ TỰ LÀ DỮ LIỆU.
 *
 * KHÔNG NÓI ĐẦU NÀO CAO NHẤT, và đó là một sự thận trọng có cơ sở chứ không phải nói vòng: máy
 * chủ sắp theo `thu_tu` của chính đơn vị và CỐ Ý không khẳng định đầu nào là đầu gấp nhất
 * (`service-petitions/internal/store/muc_uu_tien_nhiem_vu.go`: "Whichever end that is"). Viết
 * "bậc 1 là ưu tiên cao nhất" ở đây là màn hình tự đặt ra một quy ước mà dữ liệu không có — và
 * một cán bộ đọc nó sẽ lập nhiệm vụ theo đúng quy ước bịa ra ấy.
 */
export const GIAI_THICH_THANG_BAC =
  "Thứ tự trong bảng là thang bậc do đơn vị sắp. Bảng giữ nguyên thứ tự đơn vị đã sắp và không " +
  "sắp xếp lại.";
