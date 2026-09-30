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
 * mặc định.
 */
export function nhanMacDinh(laMacDinh: boolean): string {
  return laMacDinh ? "Mặc định" : "Không";
}

/** Số mục của một nhóm, đặt cạnh tên nhóm. Đếm cả mục đã tắt — xem `GIAI_THICH_DA_TAT`. */
export function nhanSoMuc(soMuc: number): string {
  return `${soMuc} mục`;
}

/**
 * Cột "Nguồn" của đặc tả §5 — `he-thong` / `don-vi` thành chữ người đọc.
 *
 * GIÁ TRỊ LẠ HIỆN NGUYÊN VĂN, KHÔNG ĐOÁN VÀ KHÔNG GIẤU. Ràng buộc CHECK của mọi bảng danh mục
 * chỉ nhận đúng hai giá trị, nên một giá trị thứ ba nghĩa là hợp đồng đã trôi khỏi CSDL. Dịch
 * bừa nó thành "Đơn vị" là giấu một sự cố sau một chữ trông bình thường — và chính chữ ấy quyết
 * định dòng này có nút Xoá hay không.
 */
export function nhanNguon(source: string): string {
  switch (source) {
    case "he-thong":
      return "Hệ thống";
    case "don-vi":
      return "Đơn vị";
    default:
      return source;
  }
}

/**
 * Vì sao một dòng không có nút — câu đặt ngay trong ô hành động của chính dòng ấy.
 *
 * ĐẶT Ở DÒNG, KHÔNG PHẢI MỘT CHÚ THÍCH CHUNG Ở CUỐI BẢNG. Người đọc đang nhìn một dòng cụ thể và
 * hỏi "vì sao dòng này không xoá được"; một câu giải thích chung ở chỗ khác bắt họ tự ghép, và
 * người dùng trình đọc màn hình thì không nghe thấy nó ở đâu cả. Ô trống thì càng tệ: nó đọc
 * thành im lặng, không phân biệt được với "màn hình chưa dựng xong".
 */
export function giaiThichKhongThaoTac(tang: number | null): string {
  switch (tang) {
    case 2:
      return "Mục do phần mềm cấp: tắt được, không xoá được.";
    case 3:
      return "Phần mềm có xử lý riêng theo mã của mục này: chỉ đổi được nhãn hiển thị.";
    default:
      return "Không có thao tác nào cho mục này.";
  }
}

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * CÂU CỦA TRẠNG THÁI RỖNG. Đọc kỹ trước khi sửa một chữ.
 *
 * Bảy danh mục hôm nay rỗng ở hầu hết đơn vị: migration cố ý không gieo mục nào, và bước khởi
 * tạo đơn vị chưa tồn tại (`nhom-danh-muc.ts`). Nên đây là đường THÔNG THƯỜNG, và câu chữ phải
 * nói đủ HAI điều, vì thiếu điều nào cũng để lại một người ngồi chờ hoặc đi tìm nút không có:
 *
 *   1. Đơn vị chưa có mục nào — hệ thống KHÔNG hỏng, không mất mạng, không mất quyền.
 *   2. Từ đây làm được gì tiếp theo — và câu trả lời khác nhau theo BA ca, nên tham số này là
 *      một union ba nhánh chứ không phải một `boolean`. Gộp "chưa có tuyến" với "thiếu quyền"
 *      vào một câu là nói sai với một trong hai người đọc: một người cần đi xin quyền, người
 *      kia xin quyền cũng không có gì mở ra.
 *
 * TÊN NHÓM NẰM TRONG CHÍNH CÂU, không chỉ ở tiêu đề phía trên: trình đọc màn hình đọc từng đoạn
 * một, và một câu tách khỏi tiêu đề thì không còn biết đang nói về danh mục nào.
 *
 * HAI CÂU NGẮN, KHÔNG PHẢI MỘT CÂU DÀI: cán bộ đọc màn hình này gồm cả người lớn tuổi, và một
 * câu ghép nhiều mệnh đề là câu phải đọc lại lần hai (`skills/accessibility-elderly`).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export type LoiRaCuaNhomRong = "themDuoc" | "thieuQuyen" | "khongCoTuyen";

export function nhanNhomRong(tenNhom: string, loiRa: LoiRaCuaNhomRong): string {
  const dau = `Đơn vị chưa có mục nào trong danh mục ${tenNhom}.`;
  switch (loiRa) {
    case "themDuoc":
      return `${dau} Bấm ${NUT_THEM} ở trên để lập mục đầu tiên.`;
    case "thieuQuyen":
      return `${dau} Tài khoản của bạn không có quyền thêm mục cho danh mục này.`;
    case "khongCoTuyen":
      return `${dau} Danh mục này chưa sửa được từ màn hình.`;
  }
}

/** Nhãn các nút thao tác. Có chữ, không chỉ có biểu tượng — xem `GHI_CHU_NUT_CO_CHU`. */
export const NUT_THEM = "+ Thêm mục";
export const NUT_SUA = "✎ Sửa";
export const NUT_TAT = "Tắt";
export const NUT_BAT_LAI = "Bật lại";
export const NUT_XOA = "🗑 Xoá";
export const NUT_LUU = "Lưu";
export const NUT_HUY = "Huỷ";
export const NUT_XAC_NHAN_XOA = "Xoá mục";

/**
 * Vì sao mọi nút đều có CHỮ bên cạnh biểu tượng, dù đặc tả §5 chỉ vẽ `✎` và `🗑`.
 *
 * Một nút chỉ có biểu tượng là một nút phải đoán, và người đoán sai ở đây bấm `🗑` khi định bấm
 * `✎`. Trình đọc màn hình cũng đọc `🗑` thành một tên emoji chứ không thành "xoá". Giữ biểu
 * tượng làm dấu nhận mặt, thêm chữ làm nghĩa (`skills/accessibility-elderly`).
 */
export const GHI_CHU_NUT_CO_CHU =
  "Mỗi nút đều có chữ bên cạnh biểu tượng để đọc được bằng trình đọc màn hình.";

/** Nhãn đọc-được-một-mình cho nút của một dòng: trình đọc màn hình đọc nút tách khỏi bảng. */
export function nhanNutCuaDong(nut: string, nhanMuc: string): string {
  return `${nut} — mục ${nhanMuc}`;
}

/**
 * Ghi chú đầu tab — BA TẦNG, và vì sao có dòng không hiện đủ nút.
 *
 * NÓI QUY TẮC RA TRƯỚC, KHÔNG ĐỂ NGƯỜI DÙNG PHÁT HIỆN BẰNG CÁCH THIẾU NÚT. Một cán bộ thấy dòng
 * này có `Xoá` còn dòng kia không sẽ kết luận màn hình hỏng, rồi gọi hỗ trợ — trong khi đó là
 * quy tắc đang làm đúng việc của nó (ADR 0024).
 */
export const GHI_CHU_BA_TANG =
  "Mục do đơn vị tự thêm thì sửa, tắt và xoá được. Mục do phần mềm cấp chỉ tắt và đổi nhãn " +
  "được, không xoá được. Riêng mục mà phần mềm có xử lý theo mã thì chỉ đổi được nhãn.";

/** Ghi chú cho hai nhóm chưa có tuyến ghi nào — nói rõ là CHƯA, và không hứa khi nào có. */
export const GHI_CHU_NHOM_CHI_XEM = "Danh mục này hiện chỉ xem, chưa sửa được từ màn hình.";

/**
 * Vì sao bảng vẫn liệt kê những mục đã tắt.
 *
 * Chỉ hiện khi trên màn hình ĐANG CÓ mục để mà giải thích — một câu giải thích đứng cạnh bảy
 * bảng rỗng không giải thích được gì cả.
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

/* ---- câu chữ của ba biểu mẫu ghi --------------------------------------------------------- */

export function tieuDeThem(tenNhom: string): string {
  return `Thêm mục vào danh mục ${tenNhom}`;
}

export function tieuDeSua(nhanMuc: string): string {
  return `Sửa mục ${nhanMuc}`;
}

export function tieuDeXoa(nhanMuc: string): string {
  return `Xoá mục ${nhanMuc}`;
}

export const O_MA = "Mã";
export const O_NHAN = "Nhãn hiển thị";
export const O_THU_TU = "Thứ tự";
export const O_MAC_DINH = "Đặt làm mục mặc định";
export const O_LY_DO_XOA = "Lý do xoá";

/**
 * Chú thích dưới ô `Mã` — mã gõ một lần, và không sửa lại được bao giờ.
 *
 * NÓI TRƯỚC KHI GÕ, KHÔNG NÓI SAU KHI LƯU. Mã đã cấp thì không đổi (luật 7, bất biến 3) vì hồ sơ
 * nghiệp vụ giữ nó làm giá trị; biết điều đó sau khi đã lưu một mã gõ sai là biết quá muộn.
 */
export const GIAI_THICH_O_MA =
  "Chữ thường không dấu, nối bằng dấu gạch ngang — ví dụ: cong-van. Mã đã lưu thì không sửa " +
  "được nữa; muốn đổi cách gọi thì sửa nhãn hiển thị.";

/** Chú thích dưới ô `Nhãn hiển thị` — đây là thứ cán bộ khác nhìn thấy trên mọi màn hình. */
export const GIAI_THICH_O_NHAN =
  "Nhãn là chữ hiện trên các màn hình khác. Sửa nhãn không ảnh hưởng tới hồ sơ đã lập.";

/**
 * Chú thích trên biểu mẫu xoá — nói đúng chuyện gì sẽ xảy ra, gồm cả chuyện mã bị giữ lại.
 *
 * "XOÁ" Ở ĐÂY KHÔNG PHẢI XOÁ, và người bấm phải biết điều đó TRƯỚC. Dòng vẫn nằm lại trong CSDL,
 * và mã của nó vẫn bị chiếm vĩnh viễn — thêm lại đúng mã ấy sẽ bị từ chối (luật 7, bất biến 3).
 * Một cán bộ tưởng xoá xong là làm lại được từ đầu sẽ mất buổi chiều để hiểu ra.
 */
export const CANH_BAO_XOA =
  "Mục sẽ không còn hiện trong danh mục, nhưng hồ sơ đã lập theo mục này vẫn giữ nguyên. Mã của " +
  "mục vẫn bị giữ chỗ và không dùng lại được cho mục mới.";

/** Câu báo sau khi ghi xong. Nói rõ ĐÃ LÀM GÌ, vì một chữ "Thành công" không xác nhận điều gì. */
export function daLuu(tenNhom: string): string {
  return `Đã lưu thay đổi cho danh mục ${tenNhom}.`;
}

export function daThem(tenNhom: string): string {
  return `Đã thêm mục mới vào danh mục ${tenNhom}.`;
}

export function daXoa(tenNhom: string): string {
  return `Đã xoá mục khỏi danh mục ${tenNhom}.`;
}

/** Câu hiện ở chỗ các nút khi tài khoản không có quyền `admin.lookup`. */
export const CAU_THIEU_QUYEN_GHI =
  "Tài khoản của bạn chỉ xem được danh mục. Việc thêm, sửa, tắt và xoá mục cần quyền Quản lý " +
  "danh mục.";
