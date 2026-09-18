/**
 * DANH MỤC XÃ CHO BẢN TRÌNH DIỄN — DỮ LIỆU TẠM, CÓ HẠN SỬ DỤNG, PHẢI XOÁ.
 *
 * ┌────────────────────────────────────────────────────────────────────────────────────────┐
 * │ TỆP NÀY KHÔNG ĐƯỢC ĐI THEO BẢN PHỤC VỤ NGƯỜI THẬT.                                     │
 * │ Một cơ quan nhà nước hiển thị đơn vị hành chính không tồn tại là một sự cố, không phải  │
 * │ một lỗi giao diện. Trong một buổi trình diễn thì đây là dữ liệu mẫu và được nói ra như  │
 * │ vậy trên màn hình; trong bản phát hành thì nó là một tuyên bố sai dưới tên một cơ quan. │
 * └────────────────────────────────────────────────────────────────────────────────────────┘
 *
 * NGUỒN THẬT LÀ GÌ, VÀ VÌ SAO CHƯA DÙNG ĐƯỢC:
 *
 *   `ListTenants` của service `platform` — đã khai trong `proto/vigov/platform/v1/platform.proto`
 *   và chú thích ở đó nói đúng việc này: kênh công dân không có tên miền để suy ra xã, nên phải
 *   cho công dân thấy danh sách xã TRƯỚC khi biết xã nào. Nó trả về tên và tỉnh/thành của các xã
 *   ĐANG HOẠT ĐỘNG, và không có trường nào chở được dữ liệu nghiệp vụ.
 *
 *   RPC ấy **chưa có cài đặt**, và chú thích trong proto nói rõ điều kiện tiên quyết: cổng gRPC
 *   của service `platform` chưa có xác thực người gọi, nên `ListTenants` đang bị từ chối bằng
 *   INVALID_ARGUMENT. Xác thực trước, mở RPC sau. Tệp này biến mất ở bước "sau".
 *
 * VÌ SAO TÊN ĐƯỢC ĐẶT RA CHỨ KHÔNG LẤY TÊN XÃ CÓ THẬT — đây là lựa chọn an toàn hơn, không phải
 * lựa chọn tiện hơn:
 *
 *   Một danh mục demo mang tên xã có thật ngụ ý rằng xã ấy **đã là khách hàng của hệ thống**.
 *   Đó là một hàm ý sai, và nó được đưa ra trước đúng những người sẽ hỏi lại. Tên đặt ra không
 *   mang hàm ý đó. Cặp (tên xã, tỉnh/thành) dưới đây không ứng với đơn vị hành chính nào: các
 *   tỉnh/thành ở đây không có trong danh sách 34 tỉnh/thành sau sắp xếp 01/7/2025.
 *
 * VÌ SAO DANH SÁCH CÓ CẢ XÃ, PHƯỜNG VÀ ĐẶC KHU:
 *
 *   Từ 01/7/2025 chính quyền địa phương hai cấp: tỉnh → **xã · phường · đặc khu**, cấp huyện
 *   không còn. Một danh mục chỉ toàn "Xã" là một danh mục kể thiếu, và nó dạy người đọc mã một
 *   mô hình dữ liệu sai — rằng loại đơn vị là một hằng số chứ không phải một trường.
 *
 * VÌ SAO MÃ LÀ ULID CHỨ KHÔNG PHẢI MÃ HÀNH CHÍNH:
 *
 *   Luật 1, bất biến 2. `tenant_id` là định danh mờ, bất biến. QR in ở trụ sở xã sống nhiều năm
 *   và phải sống qua một lần sáp nhập; một mã mang nghĩa thì lần sáp nhập đầu tiên buộc phải
 *   viết lại khoá ngoại trên hồ sơ lưu trữ. Mã ở đây mang tiền tố `01JDEMXA` để bất kỳ ai nhìn
 *   thấy nó trong log hay trong một đường liên kết đều biết ngay đó là dữ liệu của bản trình
 *   diễn chứ không phải một xã thật.
 */

/**
 * Loại dịch vụ một xã mở trên kênh công dân.
 *
 * ⚠ DANH SÁCH DỊCH VỤ KHÁC NHAU GIỮA CÁC XÃ, VÀ ĐÓ LÀ NGHIỆP VỤ CHỨ KHÔNG PHẢI GIAO DIỆN.
 *
 *   Mỗi xã bật những dịch vụ mình thực sự tiếp nhận: một xã chưa có bộ phận một cửa điện tử thì
 *   không được hiện "Tra cứu hồ sơ một cửa", vì công dân bấm vào rồi chờ một thứ không tồn tại.
 *   Nên ở giai đoạn 2 danh sách này đọc **lúc chạy** từ cấu hình của từng xã (luật 1, bất biến
 *   10) — **không bao giờ** là hằng số trong mã, vì một mã nguồn phục vụ nhiều xã.
 *
 *   Ở đây nó là hằng số **chỉ vì đây là dữ liệu trình diễn**, và nó biến mất cùng tệp này.
 */
export type LoaiDichVu = "phan-anh" | "ho-so" | "tiep-cong-dan" | "thong-bao" | "chung-thuc";

/**
 * Tên dịch vụ, viết MỘT LẦN cho cả tám xã.
 *
 * Tám xã chép lại cùng một chuỗi là tám chỗ để một lần sửa bỏ sót — và cái sót lại là hai cái
 * tên khác nhau cho cùng một việc, trên hai trang của cùng một hệ thống.
 */
export const DEMO_TEN_DICH_VU: Readonly<Record<LoaiDichVu, string>> = {
  "phan-anh": "Phản ánh hiện trường",
  "ho-so": "Tra cứu hồ sơ một cửa",
  "tiep-cong-dan": "Lịch tiếp công dân",
  "thong-bao": "Thông báo của xã",
  "chung-thuc": "Chứng thực bản sao",
};

export type XaDemo = {
  /** ULID. Mờ và bất biến — không phải mã hành chính, không phải tên miền (luật 1, bất biến 2). */
  id: string;
  /** Tên hiển thị đầy đủ, kèm loại đơn vị. Đây là thứ công dân xác nhận, không phải mã. */
  ten: string;
  /** Tỉnh/thành. Cột này tồn tại vì hai xã trùng tên ở hai tỉnh là chuyện bình thường. */
  tinh: string;
  /** Một dòng giới thiệu. Một dòng, vì màn này là nơi công dân đi TIẾP, không phải nơi đọc. */
  gioi_thieu: string;
  /**
   * ⚠ SỐ TRỰC — **DẢI GIẢ ĐÃ THOẢ THUẬN `090000000x`**, luật 3 bất biến 5. KHÔNG BAO GIỜ MỘT SỐ
   * THẬT.
   *
   *   Tệp này đi vào một bundle được tải về máy người dùng và được một người ngoài tổ chức đọc
   *   khi duyệt ứng dụng. Một số điện thoại thật ở đây là dữ liệu cá nhân đã xuất bản, không thu
   *   hồi được — và nếu là số của một cán bộ thì đó là số máy cá nhân bị công bố dưới tên một cơ
   *   quan nhà nước.
   *
   *   Chỉ chứa chữ số: dấu cách để đọc do màn hình thêm vào lúc hiển thị. Hai cách viết cho một
   *   số là hai cách viết sẽ lệch nhau.
   */
  dien_thoai_truc: string;
  /** Giờ làm việc, viết ra thành câu người dân đọc được — không phải một cấu trúc lịch. */
  gio_lam_viec: string;
  /** Dịch vụ xã này mở. 3–5 mục: xem ghi chú ở `LoaiDichVu` về vì sao chúng khác nhau. */
  dich_vu: readonly LoaiDichVu[];
};

/**
 * Danh mục mẫu. Tám dòng — đủ để màn chọn xã trông như một danh mục thật, đủ ngắn để không cần
 * ô tìm kiếm (giai đoạn 1 không có thẻ nhập liệu nào, xem `phase1-collects-nothing.test.ts`).
 *
 * Mỗi xã có nội dung RIÊNG — dịch vụ, giờ làm việc, số trực, câu giới thiệu — vì một trang xã
 * giống hệt trang xã bên cạnh thì công dân không có cách nào biết mình vào đúng chỗ, và đó đúng
 * là chế độ hỏng mà bất di dịch #2 dựng ra để chặn.
 */
export const DEMO_DANH_MUC_XA: readonly XaDemo[] = [
  {
    id: "01JDEMXA00000000000000000A",
    ten: "Xã An Thịnh",
    tinh: "Tỉnh Đông Hải",
    gioi_thieu: "Địa bàn ven biển, dân cư sống chủ yếu bằng nghề nuôi trồng thuỷ sản.",
    dien_thoai_truc: "0900000001",
    gio_lam_viec: "Thứ Hai đến Thứ Sáu · Sáng 7:30–11:30 · Chiều 13:30–17:00",
    dich_vu: ["phan-anh", "ho-so", "thong-bao"],
  },
  {
    id: "01JDEMXA00000000000000000B",
    ten: "Xã Bình Khê",
    tinh: "Tỉnh Đông Hải",
    gioi_thieu: "Địa bàn trung du, có ba thôn nằm cách trụ sở xã hơn mười cây số.",
    dien_thoai_truc: "0900000002",
    gio_lam_viec: "Thứ Hai đến Thứ Sáu · Sáng 7:00–11:30 · Chiều 13:00–17:00",
    dich_vu: ["phan-anh", "tiep-cong-dan", "thong-bao", "chung-thuc"],
  },
  {
    id: "01JDEMXA00000000000000000C",
    ten: "Đặc khu Hòn Mây",
    tinh: "Tỉnh Đông Hải",
    gioi_thieu: "Đảo, đi lại bằng tàu khách; hồ sơ gửi qua kênh trực tuyến được ưu tiên.",
    dien_thoai_truc: "0900000003",
    gio_lam_viec: "Thứ Hai đến Thứ Bảy · Sáng 7:30–11:30 · Chiều 13:30–16:30",
    dich_vu: ["phan-anh", "ho-so", "tiep-cong-dan", "thong-bao", "chung-thuc"],
  },
  {
    id: "01JDEMXA00000000000000000D",
    ten: "Phường Tân Lộc",
    tinh: "Thành phố Nam Giang",
    gioi_thieu: "Địa bàn đô thị, nhiều khu dân cư mới và hai chợ dân sinh.",
    dien_thoai_truc: "0900000004",
    gio_lam_viec: "Thứ Hai đến Thứ Sáu · Sáng 7:30–11:30 · Chiều 13:30–17:00",
    dich_vu: ["phan-anh", "ho-so", "chung-thuc"],
  },
  {
    id: "01JDEMXA00000000000000000E",
    ten: "Phường Hoà Mỹ",
    tinh: "Thành phố Nam Giang",
    gioi_thieu: "Địa bàn đô thị trung tâm, tiếp nhận nhiều hồ sơ hộ tịch và chứng thực.",
    dien_thoai_truc: "0900000005",
    gio_lam_viec: "Thứ Hai đến Thứ Sáu · Sáng 7:30–11:30 · Chiều 13:30–17:30",
    dich_vu: ["ho-so", "chung-thuc", "thong-bao"],
  },
  {
    id: "01JDEMXA00000000000000000F",
    ten: "Phường Trung Chánh",
    tinh: "Thành phố Nam Giang",
    gioi_thieu: "Địa bàn giáp ranh, có khu công nghiệp và đông người tạm trú.",
    dien_thoai_truc: "0900000006",
    gio_lam_viec: "Thứ Hai đến Thứ Sáu · Sáng 7:00–11:00 · Chiều 13:00–17:00",
    dich_vu: ["phan-anh", "ho-so", "tiep-cong-dan", "thong-bao"],
  },
  {
    id: "01JDEMXA00000000000000000G",
    ten: "Xã Cẩm Sơn",
    tinh: "Tỉnh Tây Hoà",
    gioi_thieu: "Địa bàn miền núi, phần lớn thủ tục còn tiếp nhận trực tiếp tại trụ sở.",
    dien_thoai_truc: "0900000007",
    gio_lam_viec: "Thứ Hai đến Thứ Sáu · Sáng 7:00–11:00 · Chiều 13:30–16:30",
    dich_vu: ["phan-anh", "tiep-cong-dan", "thong-bao"],
  },
  {
    id: "01JDEMXA00000000000000000H",
    ten: "Xã Long Phú",
    tinh: "Tỉnh Tây Hoà",
    gioi_thieu: "Địa bàn đồng bằng, có tuyến quốc lộ chạy qua và hai cụm dân cư lớn.",
    dien_thoai_truc: "0900000008",
    gio_lam_viec: "Thứ Hai đến Thứ Sáu · Sáng 7:30–11:30 · Chiều 13:30–17:00",
    dich_vu: ["phan-anh", "ho-so", "tiep-cong-dan", "chung-thuc"],
  },
];

/**
 * Câu chữ hiện dưới danh mục. Nó nằm ở ĐÂY, cạnh dữ liệu nó nói về, chứ không nằm trong màn
 * hình: xoá tệp này thì câu ấy mất theo, và không còn một lời chú thích mồ côi nói về một danh
 * mục không còn tồn tại.
 */
export const DEMO_GHI_CHU = "Danh mục mẫu dùng cho bản trình diễn.";

/**
 * Câu chữ hiện trên TRANG XÃ, và nó phải nói rộng hơn câu trên.
 *
 * Trang xã in ra một số điện thoại, một khung giờ làm việc và một danh sách dịch vụ. "Danh mục
 * mẫu" chỉ nói về cái danh sách xã; một người đọc trang xã có quyền hiểu rằng **số điện thoại
 * thì thật**. Một người dân gọi vào số ấy là một người dân bị hệ thống của cơ quan nhà nước chỉ
 * sai đường, nên câu ở đây gọi đúng tên tất cả những gì đang là dữ liệu mẫu.
 */
export const DEMO_GHI_CHU_TRANG_XA =
  "Thông tin liên hệ, giờ làm việc và dịch vụ trên trang này là dữ liệu mẫu dùng cho bản trình diễn.";

/**
 * Tra một mã sang xã. **Không tìm thấy thì trả `null`, không đoán** — luật 1 cấm mặc định trên
 * đường cô lập, và ở đây "đoán" nghĩa là hiển thị một tên xã cho công dân xác nhận trong khi
 * không biết mã ấy trỏ vào đâu. Công dân xác nhận theo TÊN, nên một cái tên sai ở bước này là
 * một hồ sơ gửi nhầm cơ quan.
 */
export function DEMO_timTheoMa(ma: string): XaDemo | null {
  return DEMO_DANH_MUC_XA.find((xa) => xa.id === ma) ?? null;
}
