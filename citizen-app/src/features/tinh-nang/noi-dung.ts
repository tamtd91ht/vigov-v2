/**
 * MỌI CHỮ NGƯỜI DÙNG ĐỌC TRÊN BA TÍNH NĂNG — một tệp, giống `content/company-profile.ts`.
 *
 * VÌ SAO TÁCH RA KHỎI COMPONENT:
 *
 *   Người duyệt của Zalo đọc đúng những câu này, và đây là thứ quyết định app có được cấp ba
 *   quyền hay không: chính sách Mini App (điều 3.3.4, trích trong `zmp-sdk/index.d.ts` ngay trên
 *   `getPhoneNumber`) từ chối xét duyệt những app xin quyền mà không nói rõ mục đích. Một câu
 *   giải thích nằm rải trong JSX là một câu không ai đọc lại trước khi nộp.
 *
 *   Tách ra còn để `bundle-for-zalo.test.ts` **dùng lại đúng danh sách này** khi khẳng định bản
 *   dựng nộp CÓ chứa đủ chữ của ba tính năng. Một danh sách chép tay ở phía test sẽ lệch, và khi
 *   nó lệch thì phép kiểm xanh vì không tìm thấy gì — chứ không vì bản dựng đúng.
 *
 * ⚠ KHÔNG CÂU NÀO Ở ĐÂY ĐƯỢC HỨA MỘT VIỆC BẢN DỰNG NÀY KHÔNG LÀM. Không có máy chủ nào nhận dữ
 * liệu, nên không câu nào được viết "chúng tôi sẽ lưu để…" hay "yêu cầu của bạn đã được gửi".
 * Mô tả sai cho người duyệt là thứ không sửa lại được sau đó — và mô tả sai cho một người đang
 * chờ được gọi lại thì tệ hơn thế.
 *
 * ⚠ NGỮ CẢNH LÀ MỘT DOANH NGHIỆP CÔNG NGHỆ, KHÔNG PHẢI MỘT DỊCH VỤ CÔNG. VihatSoftware bán giải
 * pháp tổng đài đám mây, CRM và ứng dụng AI cho doanh nghiệp. Mọi câu ở đây nói với một khách
 * hàng hoặc một đối tác đang tìm hiểu giải pháp.
 */

/** Ba tính năng, và cũng là ba quyền nền tảng ứng dụng xin. Thứ tự này là thứ tự hiện ra. */
export type MaTinhNang = "danh-thiep" | "van-phong" | "tu-van";

export type NoiDungTinhNang = {
  ma: MaTinhNang;
  /** Nhãn ngắn, dùng trên nút và trên tiêu đề phụ. Ngắn để không tràn trên máy 320px. */
  nhan_ngan: string;
  tieu_de: string;
  /** VÌ SAO app cần quyền này — câu người duyệt đọc để quyết định. */
  vi_sao: string;
  nut: string;
  dang_cho: string;
  /** Người dùng từ chối: ĐƯỜNG ĐI BÌNH THƯỜNG, không phải lỗi. */
  tu_choi: string;
  /** Ngoài Zalo thì SDK không nạp được. Nói ra, không để màn trắng. */
  ngoai_zalo: string;
  /** Mọi trường hợp còn lại. Nói việc cần làm, không nói mã lỗi (README §Error message shape). */
  khong_lay_duoc: string;
};

/**
 * Câu nói ra ĐIỀU QUAN TRỌNG NHẤT của hai tính năng dùng token, và nó không phải lời trấn an:
 *
 *   `GetPhoneNumberReturns` và `GetLocationReturns` (đo từ `node_modules/zmp-sdk/index.d.ts`) chỉ
 *   còn `token` là trường dùng được — `number`, `latitude`, `longitude` đều `@deprecated`. Token
 *   dùng được MỘT LẦN, hết hạn sau 2 phút, và chỉ máy chủ có app secret mới đổi được.
 *
 *   Nên số điện thoại và toạ độ **không bao giờ tới thiết bị**. Màn hình không có gì để che vì nó
 *   không có dữ liệu cá nhân ngay từ đầu — đó là một sự thật về nền tảng, và nói ra được với
 *   người dùng là lý do đáng tin nhất để họ bấm đồng ý.
 */
export const TOKEN_KHONG_CHUA_GI = {
  "tu-van":
    "Số điện thoại của bạn không nằm trong mã này và không được gửi về máy. Chỉ máy chủ của VihatSoftware mới đổi được mã thành số điện thoại, và mã chỉ dùng được một lần trong 2 phút.",
  "van-phong":
    "Toạ độ của bạn không nằm trong mã này và không được gửi về máy. Chỉ máy chủ của VihatSoftware mới đổi được mã thành vị trí, và mã chỉ dùng được một lần trong 2 phút.",
} as const;

/** Lời hứa của chính bản dựng này. Xem `zalo-api.ts` về việc giữ nó. */
export const CHI_HIEN_LEN_MAN_HINH =
  "Bản dựng này chỉ hiện kết quả lên màn hình. Không có gì được lưu lại trên máy và không có gì được gửi đi.";

export const NOI_DUNG_TINH_NANG: readonly NoiDungTinhNang[] = [
  {
    ma: "danh-thiep",
    nhan_ngan: "Quét danh thiếp",
    tieu_de: "Quét danh thiếp số của đối tác",
    vi_sao:
      "Ở một buổi gặp khách hàng hay một hội thảo công nghệ, mỗi người đưa một tấm thiếp có mã QR. Ứng dụng xin dùng máy ảnh để quét mã ấy và tách sẵn tên, công ty, số điện thoại, email — bạn bấm một lần là gọi được, thay vì gõ lại từng ký tự.",
    nut: "Mở máy ảnh để quét mã",
    dang_cho: "Đang mở máy ảnh…",
    tu_choi:
      "Bạn đã dừng quét mã. Không sao cả — bạn vẫn dùng được ứng dụng bình thường, và có thể bấm lại nút bên trên bất cứ lúc nào.",
    ngoai_zalo:
      "Máy ảnh chỉ mở được bên trong ứng dụng Zalo. Bạn hãy mở lại trang này trong Zalo trên điện thoại rồi bấm lại.",
    khong_lay_duoc:
      "Chưa quét được mã. Bạn hãy bấm lại nút bên trên và giữ máy cách tấm thiếp khoảng một gang tay.",
  },
  {
    ma: "van-phong",
    nhan_ngan: "Tìm văn phòng",
    tieu_de: "Tìm văn phòng gần bạn",
    vi_sao:
      "VihatSoftware có ba văn phòng. Ứng dụng xin vị trí để chỉ ra văn phòng gần bạn nhất khi bạn muốn tới gặp đội kinh doanh, thay vì bắt bạn tự đối chiếu ba địa chỉ. Vị trí chỉ để GỢI Ý — bạn luôn là người chọn nơi mình tới.",
    nut: "Tìm văn phòng gần tôi",
    dang_cho: "Đang chờ bạn trả lời trên Zalo…",
    tu_choi:
      "Bạn đã từ chối chia sẻ vị trí. Không sao cả — cả ba văn phòng vẫn nằm ngay bên dưới, kèm nút chỉ đường.",
    ngoai_zalo:
      "Phần này chỉ chạy được bên trong ứng dụng Zalo. Cả ba văn phòng vẫn nằm ngay bên dưới, kèm nút chỉ đường.",
    khong_lay_duoc:
      "Chưa nhận được trả lời từ Zalo. Bạn hãy bấm lại nút bên trên; cả ba văn phòng vẫn nằm ngay bên dưới, kèm nút chỉ đường.",
  },
  {
    ma: "tu-van",
    nhan_ngan: "Nhận tư vấn",
    tieu_de: "Đăng ký nhận tư vấn giải pháp",
    vi_sao:
      "Khi bạn muốn được tư vấn về tổng đài đám mây, CRM hay ứng dụng AI cho doanh nghiệp mình, đội kinh doanh cần một số điện thoại để gọi lại. Ứng dụng xin số điện thoại Zalo của bạn để làm đúng việc đó, và chỉ xin đúng lúc bạn bấm nút đăng ký.",
    nut: "Đăng ký nhận tư vấn",
    dang_cho: "Đang chờ bạn trả lời trên Zalo…",
    tu_choi:
      "Bạn đã từ chối chia sẻ số điện thoại. Không sao cả — bạn vẫn gọi hotline hoặc gửi email cho chúng tôi được ngay bên dưới.",
    ngoai_zalo:
      "Phần này chỉ chạy được bên trong ứng dụng Zalo. Bạn vẫn gọi hotline hoặc gửi email cho chúng tôi được ngay bên dưới.",
    khong_lay_duoc:
      "Chưa nhận được trả lời từ Zalo. Bạn hãy bấm lại nút bên trên, hoặc gọi hotline ngay bên dưới nếu bạn cần trao đổi luôn.",
  },
];

export function noiDung(ma: MaTinhNang): NoiDungTinhNang {
  const tim = NOI_DUNG_TINH_NANG.find((mot) => mot.ma === ma);
  // Fail closed: một mã không có nội dung là lỗi lập trình, và một màn trắng là thứ người duyệt
  // của Zalo thấy trước tiên.
  if (!tim) throw new Error(`Khong co noi dung cho tinh nang: ${ma}`);
  return tim;
}

/**
 * Nền tảng trả về mã RỖNG ở môi trường phát triển — ghi thẳng trong `zmp-sdk/index.d.ts`:
 * *"Ở môi trường phát triển, API này sẽ luôn thành công và trả về token rỗng"*. Nói ra đúng như
 * vậy, thay vì vẽ ra một ô kết quả trống mà người đọc tưởng là hỏng.
 */
export const MA_RONG =
  "Zalo trả về mã rỗng. Điều này xảy ra khi ứng dụng chưa chạy trong Zalo trên điện thoại thật.";

/** ---------- Màn quét danh thiếp ---------- */

export const DANH_THIEP = {
  // Câu dẫn nói VIỆC CẦN LÀM BÂY GIỜ; `vi_sao` của tính năng nói vì sao app cần máy ảnh. Hai câu
  // khác nhau có chủ đích — lặp lại cùng một ý hai lần là hai lần người đọc bỏ qua cả hai.
  dan_nhap: "Vừa nhận một tấm thiếp có mã QR? Quét ngay ở đây.",
  nut_quet_lai: "Quét mã khác",
  tieu_de_ket_qua: "Thông tin đọc được từ mã",
  nhan_ho_ten: "Họ và tên",
  nhan_to_chuc: "Công ty",
  nhan_chuc_danh: "Chức danh",
  nhan_dien_thoai: "Điện thoại",
  nhan_email: "Email",
  nhan_trang_web: "Trang web",
  nut_goi: "Gọi",
  nut_email: "Gửi email",
  nut_mo_lien_ket: "Mở liên kết",
  /** Mã QR quét được nhưng không có nội dung. Hiếm, nhưng một ô trống thì không nói được gì. */
  ma_rong: "Mã vừa quét không có nội dung nào.",
  /** `BEGIN:VCARD` hợp lệ nhưng bên trong không có trường nào dùng được. */
  thiep_rong:
    "Mã này là một danh thiếp nhưng không ghi tên, số điện thoại hay email nào. Bạn hãy xin đối tác một mã khác.",
  tieu_de_lien_ket: "Mã này là một liên kết",
  tieu_de_van_ban: "Mã này là một đoạn văn bản",
  /** Người dùng cần biết chữ hiện ra là nguyên văn mã, không phải một bản đã sửa. */
  nguyen_van: "Nội dung nguyên văn của mã:",
  rieng_tu:
    "Thông tin trên tấm thiếp là dữ liệu cá nhân của người đưa nó cho bạn. Ứng dụng chỉ hiện lên màn hình này và xoá đi khi bạn quét mã khác hoặc rời màn hình.",
} as const;

/**
 * Gọi điện, mở trang web và chỉ đường đều đi qua API của nền tảng (`openPhone` · `openWebview`),
 * và cả hai đều `@zaloOnly`. Ngoài Zalo thì chúng không chạy — nói ra bằng một câu, không để
 * người dùng bấm vào một cái nút im lặng.
 *
 * CÂU NÀY TỪNG BẮT ĐẦU BẰNG "Chưa mở được", và `bundle-for-zalo.test.ts` đã bắt: "Chưa mở" là
 * nhãn của lớp khám phá (bản demo nội bộ), và bản NỘP phải không chứa một chuỗi nào của lớp ấy.
 * Ghi lại để lần sau ai định viết "Chưa mở…" ở đây thì biết vì sao nó đỏ.
 */
export const LOI_MO_NGOAI =
  "Thao tác này chỉ chạy bên trong ứng dụng Zalo trên điện thoại. Bạn hãy mở lại trang này trong Zalo rồi bấm lại.";

/** ---------- Tìm văn phòng ---------- */

export const VAN_PHONG = {
  dan_nhap:
    "Ba văn phòng của chúng tôi luôn nằm ngay bên dưới. Chia sẻ vị trí thì bước sau ứng dụng chỉ thẳng ra văn phòng gần bạn nhất.",
  nut_chi_duong: "Chỉ đường",
  da_nhan_ma: "Đã nhận được mã vị trí từ Zalo.",
  /**
   * NÓI THẲNG RANH GIỚI, KHÔNG HỨA.
   *
   * `getLocation` chỉ trả token; đổi token thành toạ độ là một bước ở máy chủ, có app secret.
   * Nên bản này KHÔNG xếp được ba văn phòng theo khoảng cách, và câu dưới nói đúng như vậy. Viết
   * "đang tìm văn phòng gần bạn…" rồi hiện danh sách theo thứ tự cũ là nói dối bằng giao diện.
   */
  chua_xep_duoc:
    "Việc xếp ba văn phòng theo khoảng cách cần một bước máy chủ đổi mã này thành toạ độ. Bản hiện tại chưa có bước đó, nên danh sách bên dưới giữ nguyên thứ tự và chưa sắp theo khoảng cách.",
} as const;

/** ---------- Đăng ký nhận tư vấn ---------- */

export const TU_VAN = {
  dan_nhap:
    "Để lại số điện thoại Zalo để đội kinh doanh gọi lại tư vấn giải pháp tổng đài đám mây, CRM và ứng dụng AI cho doanh nghiệp của bạn.",
  da_nhan_ma: "Đã nhận được mã số điện thoại từ Zalo.",
  /** Cùng lý do với `VAN_PHONG.chua_xep_duoc`: nói ra thứ bản này CHƯA làm. */
  chua_gui_di:
    "Yêu cầu tư vấn sẽ được chuyển tới đội kinh doanh khi phần dịch vụ được nối vào. Bản hiện tại chưa gửi gì đi, nên nếu bạn cần trao đổi ngay, hãy dùng hotline hoặc email bên dưới.",
  nhac_lien_he: "Cần trao đổi ngay? Hai đường này chạy được ngay bây giờ:",
} as const;

/**
 * MỤC VỀ BA QUYỀN TRONG CHÍNH SÁCH QUYỀN RIÊNG TƯ — dựng từ chính `NOI_DUNG_TINH_NANG`.
 *
 * VÌ SAO DỰNG TỪ ĐÓ CHỨ KHÔNG VIẾT LẠI: `vi_sao` là câu nói mục đích của từng quyền, và nó đã
 * có người chủ — chính mảng trên. Chép sang chính sách là tạo bản thứ hai của một sự thật, và
 * khi hai bản lệch nhau thì bản sai là bản nằm trong văn bản pháp lý (luật 9).
 *
 * ĐOẠN CUỐI NÓI CẢ HAI VẾ, CÓ CHỦ ĐÍCH. Người duyệt cần biết mục đích để cấp quyền; người dùng
 * cần biết bản họ đang cầm thật sự làm gì. Viết mỗi mục đích mà giấu việc bản này chưa truyền gì
 * là mô tả sai; viết mỗi "chưa làm gì" mà giấu mục đích là không đủ để xét duyệt. Nói cả hai.
 */
export const DOAN_CHINH_SACH_TINH_NANG: readonly string[] = [
  "Ứng dụng xin ba quyền của nền tảng Zalo, mỗi quyền cho đúng một tính năng. Mỗi quyền chỉ được hỏi khi bạn tự bấm nút, và bạn có quyền từ chối mà vẫn dùng được ứng dụng.",
  ...NOI_DUNG_TINH_NANG.map((mot) => `${mot.nhan_ngan} — ${mot.vi_sao}`),
  "Với số điện thoại và vị trí, Zalo không trả giá trị thật về máy: ứng dụng chỉ nhận một mã dùng được một lần và hết hạn sau 2 phút. Số điện thoại và toạ độ của bạn không nằm trong mã đó.",
  "Với quét mã QR, nội dung mã hiện lên màn hình và mất đi khi bạn quét mã khác hoặc rời màn hình. Nếu mã là một tấm danh thiếp, nội dung ấy là dữ liệu cá nhân của người đã đưa nó cho bạn, và ứng dụng cũng không lưu lại.",
  "Các mục đích nêu trên là mục đích ứng dụng sẽ dùng ba quyền này khi có đầy đủ chức năng. Bản hiện tại chỉ hiện kết quả lên màn hình: nó chưa lưu và chưa gửi bất kỳ dữ liệu nào đi đâu.",
];
