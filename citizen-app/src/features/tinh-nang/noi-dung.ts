/**
 * MỌI CHỮ NGƯỜI DÙNG ĐỌC TRÊN SÁU TÍNH NĂNG — một tệp, giống `content/company-profile.ts`.
 *
 * VÌ SAO TÁCH RA KHỎI COMPONENT:
 *
 *   Người duyệt của Zalo đọc đúng những câu này, và đây là thứ quyết định app có được cấp các
 *   quyền hay không: chính sách Mini App (điều 3.3.4, trích trong `zmp-sdk/index.d.ts` ngay trên
 *   `getPhoneNumber`) từ chối xét duyệt những app xin quyền mà không nói rõ mục đích. Một câu
 *   giải thích nằm rải trong JSX là một câu không ai đọc lại trước khi nộp.
 *
 *   Tách ra còn để `bundle-for-zalo.test.ts` **dùng lại đúng danh sách này** khi khẳng định bản
 *   dựng nộp CÓ chứa đủ chữ của sáu tính năng. Một danh sách chép tay ở phía test sẽ lệch, và khi
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
/**
 * SÁU TÍNH NĂNG, CHÍN QUYỀN NỀN TẢNG. Thứ tự này là thứ tự hiện ra.
 *
 * MỘT TÍNH NĂNG CÓ THỂ CẦN HAI QUYỀN, VÀ ĐÓ LÀ LÝ DO HAI CON SỐ KHÔNG BẰNG NHAU: giữ màn hình
 * sáng và ghi tệp xuống máy là hai quyền, nhưng chúng phục vụ ĐÚNG MỘT việc người dùng làm —
 * chìa tấm danh thiếp ra cho người khác. Tách chúng thành hai "tính năng" để con số đẹp lên là
 * dựng hai cái nút không ai hiểu để làm gì, đúng thứ điều 3.3.4 của chính sách Mini App từ chối.
 */
export type MaTinhNang =
  | "danh-thiep"
  | "van-phong"
  // ĐỔI TÊN TỪ `tu-van`, KHÔNG PHẢI THÊM MỘT TÍNH NĂNG: khối `getPhoneNumber` trên màn Liên hệ
  // nay là KHỐI ĐĂNG NHẬP (ADR 0020), không còn là "đăng ký nhận tư vấn". Tên mã đi theo việc
  // nó làm — một cái tên cũ còn lại là một cái tên sẽ được người sau đọc thành lời hứa cũ.
  | "dang-nhap"
  | "duong-truyen"
  | "thiep-cua-chung-toi"
  | "so-hoa-thiep";

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
  "dang-nhap":
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
    /**
     * KHỐI ĐĂNG NHẬP — `getPhoneNumber` là đường đăng nhập MỘT CHẠM, không phải một biểu mẫu
     * đăng ký. Đây là câu chữ người duyệt Zalo đọc để quyết định có cấp quyền hay không (điều
     * 3.3.4), nên mục đích phải nói thẳng: số điện thoại dùng để ĐĂNG NHẬP và để NHẬN THÔNG BÁO.
     *
     * ⚠ KHÔNG MỘT CHỮ NÀO HỨA MỘT MÃ OTP. ADR 0020 bác đường OTP có lý do đo được: màn nhập sáu
     * số là rào thật với người cao tuổi — đổi ứng dụng để đọc tin nhắn, nhớ sáu chữ số, quay
     * lại, gõ đúng trước khi hết hạn; mỗi bước là một chỗ bỏ cuộc. Hứa một mã rồi không gửi còn
     * tệ hơn: người dùng ngồi chờ một tin nhắn không bao giờ tới.
     */
    ma: "dang-nhap",
    nhan_ngan: "Đăng nhập",
    tieu_de: "Đăng nhập bằng số Zalo",
    vi_sao:
      "Ứng dụng dùng số điện thoại Zalo của bạn làm tên đăng nhập, để lần sau mở lại là bạn thấy đúng phần việc của mình. Một lần chạm là xong: không mật khẩu, không phải chờ một mã sáu số gửi qua tin nhắn. Số ấy cũng là nơi chúng tôi gửi thông báo ZNS cho bạn khi có kết quả. Ứng dụng chỉ hỏi đúng lúc bạn bấm nút đăng nhập, và bạn có quyền từ chối.",
    nut: "Đăng nhập bằng số Zalo",
    dang_cho: "Đang chờ bạn trả lời trên Zalo…",
    tu_choi:
      "Bạn đã từ chối chia sẻ số điện thoại, nên chưa đăng nhập được. Không sao cả — bạn vẫn xem được toàn bộ ứng dụng, và vẫn gọi hotline hoặc gửi email cho chúng tôi ngay bên dưới.",
    ngoai_zalo:
      "Việc đăng nhập chỉ chạy được bên trong ứng dụng Zalo. Bạn hãy mở lại trang này trong Zalo trên điện thoại rồi bấm lại; hotline và email ngay bên dưới thì lúc nào cũng dùng được.",
    khong_lay_duoc:
      "Chưa đăng nhập được. Bạn hãy bấm lại nút bên trên sau vài giây, hoặc gọi hotline ngay bên dưới nếu bạn cần trao đổi luôn.",
  },
  {
    ma: "duong-truyen",
    nhan_ngan: "Kiểm tra đường truyền",
    tieu_de: "Kiểm tra đường truyền trước cuộc gọi",
    vi_sao:
      "Tổng đài đám mây truyền cuộc gọi qua chính đường mạng của máy bạn, nên biết mình đang đi bằng Wi-Fi hay bằng mạng di động là điều đầu tiên cần biết khi một cuộc gọi nghe không rõ. Ứng dụng xin đọc KIỂU kết nối hiện tại — chỉ kiểu kết nối. Không đọc địa chỉ IP, không đọc tên mạng Wi-Fi, không đo tốc độ.",
    nut: "Kiểm tra đường truyền",
    dang_cho: "Đang đọc kiểu kết nối…",
    tu_choi:
      "Bạn đã dừng việc kiểm tra. Không sao cả — bạn vẫn dùng được ứng dụng bình thường, và có thể bấm lại nút bên trên bất cứ lúc nào.",
    ngoai_zalo:
      "Phần này chỉ chạy được bên trong ứng dụng Zalo trên điện thoại. Bạn hãy mở lại trang này trong Zalo rồi bấm lại.",
    khong_lay_duoc:
      "Chưa đọc được kiểu kết nối. Bạn hãy bấm lại nút bên trên sau vài giây; nếu máy vừa chuyển giữa Wi-Fi và mạng di động thì cần một lúc để ổn định.",
  },
  {
    ma: "thiep-cua-chung-toi",
    nhan_ngan: "Danh thiếp của chúng tôi",
    // TÊN CÔNG TY VIẾT THẲNG, KHÔNG GHÉP BẰNG `${COMPANY.name}` — đã thử và đã đo:
    // một chuỗi ghép lúc chạy KHÔNG hề có trong bundle, bundle chỉ chứa hai mảnh rời. Ca
    // "bản nộp có đủ chữ của sáu tính năng" trong `bundle-for-zalo.test.ts` bắt được đúng điều
    // đó. Cùng lối viết với `TOKEN_KHONG_CHUA_GI` ngay trên, vì cùng một lý do.
    tieu_de: "Danh thiếp số của VihatSoftware",
    vi_sao:
      "Khi bạn gặp đội kinh doanh của chúng tôi, đây là tấm thiếp để bạn lưu lại. Ứng dụng xin quyền giữ màn hình sáng để mã không tối đi giữa lúc người đối diện đang quét, và xin quyền ghi tệp để bạn tải tấm thiếp về máy rồi thêm thẳng vào danh bạ.",
    nut: "Tải danh thiếp (.vcf)",
    dang_cho: "Đang ghi tệp xuống máy…",
    tu_choi:
      "Bạn đã dừng việc tải tệp. Không sao cả — mã bên trên vẫn quét được, và mọi thông tin trên tấm thiếp đều nằm ngay dưới mã dưới dạng chữ.",
    ngoai_zalo:
      "Việc tải tệp chỉ chạy được bên trong ứng dụng Zalo trên điện thoại. Mã bên trên vẫn quét được, và mọi thông tin đều nằm ngay dưới mã dưới dạng chữ.",
    khong_lay_duoc:
      "Chưa ghi được tệp xuống máy. Bạn hãy bấm lại nút bên trên, hoặc lưu thông tin bằng cách quét mã bên trên bằng một máy khác.",
  },
  {
    ma: "so-hoa-thiep",
    nhan_ngan: "Số hoá thiếp giấy",
    tieu_de: "Số hoá danh thiếp giấy",
    vi_sao:
      "Sau một hội thảo, thứ còn lại trong túi áo thường là một xấp thiếp giấy. Ứng dụng xin quyền dùng máy ảnh và quyền mở cửa sổ chọn ảnh để bạn chụp hoặc chọn ảnh tấm thiếp ngay tại chỗ, thay vì để nó nằm đó tới lúc quên mất người đã đưa.",
    nut: "Chuẩn bị camera",
    dang_cho: "Đang hỏi quyền dùng máy ảnh…",
    tu_choi:
      // NGUỒN CỦA CÂU VỀ iOS: bảng Quản lý quyền trong Zalo Developer Console, ô ghi chú của
      // quyền "Yêu cầu thiết bị cấp quyền truy cập camera" — KHÔNG phải `zmp-sdk/index.d.ts`,
      // nơi `requestCameraPermission` (dòng ~4002) không có một dòng chú thích nào.
      //
      // Ghi nguồn ra đây vì một lượt trước đã suýt viết câu này với nguồn sai, và một khẳng
      // định về hành vi nền tảng đặt trong app của một pháp nhân có thật thì phải truy được.
      // Đây cũng là thứ người dùng cần biết NHẤT ở đúng khoảnh khắc họ vừa bấm Từ chối: trên
      // iOS họ sẽ không được hỏi lại, nên nếu không nói bây giờ thì không còn lúc nào để nói.
      "Bạn đã từ chối quyền dùng máy ảnh. Không sao cả — bạn vẫn chọn được ảnh đã chụp sẵn bằng nút bên dưới. Lưu ý trên iPhone: Zalo chỉ hỏi quyền máy ảnh một lần, nên muốn cấp lại bạn phải vào phần cài đặt của máy.",
    ngoai_zalo:
      "Máy ảnh và cửa sổ chọn ảnh chỉ mở được bên trong ứng dụng Zalo trên điện thoại. Bạn hãy mở lại trang này trong Zalo rồi bấm lại.",
    khong_lay_duoc:
      "Chưa hỏi được quyền dùng máy ảnh. Bạn hãy bấm lại nút bên trên, hoặc chọn ảnh đã chụp sẵn bằng nút bên dưới.",
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

/** ---------- Đăng nhập bằng số Zalo ---------- */

export const DANG_NHAP = {
  dan_nhap:
    "Đăng nhập bằng chính số Zalo bạn đang dùng. Một lần chạm là xong — không mật khẩu, không phải gõ mã sáu số nào.",
  da_nhan_ma: "Đã nhận được mã số điện thoại từ Zalo.",
  /**
   * CÂU "BẢN NÀY CHƯA NỐI MÁY CHỦ" ĐÃ BỊ XOÁ — 20/09/2026, và nó bị xoá vì đã thành SAI.
   *
   * Cả hai biến thể nay gọi máy chủ thật (`features/dang-nhap/`), nên một câu nói ngược lại là
   * một lời nói dối bằng giao diện — đúng thứ mà §"RANH GIỚI" của README cấm theo chiều ngược:
   * ở đó ta không được hứa việc bản dựng không làm, và ở đây ta không được chối việc nó có làm.
   *
   * Thứ thay chỗ nó là các câu trạng thái thật trong `features/dang-nhap/PhatHanhPhien.tsx`:
   * đang gửi · đã đăng nhập · mã hết hạn · Zalo không trả lời · không gọi được.
   */
  nhac_lien_he: "Cần trao đổi ngay? Hai đường này chạy được ngay bây giờ:",
} as const;

/** ---------- Kiểm tra đường truyền ---------- */

/**
 * NHÃN VÀ Ý NGHĨA CỦA TỪNG KIỂU KẾT NỐI.
 *
 * ⚠ KHÔNG MỘT CON SỐ NÀO Ở ĐÂY, VÀ ĐÓ LÀ MỘT QUYẾT ĐỊNH, KHÔNG PHẢI MỘT THIẾU SÓT:
 *
 *   `getNetworkType` trả về ĐÚNG MỘT chuỗi — kiểu kết nối. Nó không đo độ trễ, không đo băng
 *   thông, không nói gì về chất lượng đường truyền tới tổng đài. Viết "Wi-Fi: khoảng 30ms, đủ
 *   để gọi" là bịa ra một phép đo không hề chạy — và bịa nó trong ứng dụng của một nhà cung cấp
 *   hạ tầng thoại, tức là bịa đúng chỗ khách hàng của họ sẽ đem ra đối chiếu với máy đo thật.
 *
 *   Nên mỗi câu dưới đây chỉ nói một điều ĐÚNG VỀ BẢN CHẤT của kiểu kết nối ấy, thứ không cần
 *   đo cũng biết, và để người đọc tự kết luận.
 */
export const KIEU_KET_NOI = {
  wifi: {
    nhan: "Wi-Fi",
    y_nghia:
      "Cuộc gọi thoại đang đi qua Wi-Fi. Khi bạn bước ra khỏi vùng phủ của bộ phát, máy chuyển sang mạng di động, và cuộc gọi đang nói có thể bị gián đoạn ở đúng lúc chuyển.",
  },
  cellular: {
    nhan: "Mạng di động",
    y_nghia:
      "Cuộc gọi thoại đang đi qua mạng di động. Vùng phủ sóng nơi bạn đứng quyết định đường truyền này, nên chất lượng thay đổi khi bạn di chuyển.",
  },
  none: {
    nhan: "Không có mạng",
    y_nghia:
      "Máy đang không có kết nối nào, nên chưa gọi được qua tổng đài đám mây. Bạn hãy bật Wi-Fi hoặc dữ liệu di động rồi bấm kiểm tra lại.",
  },
  "khong-xac-dinh": {
    nhan: "Không xác định",
    y_nghia:
      "Máy không cho biết đang dùng kiểu kết nối nào. Đây là câu trả lời bình thường trên một số thiết bị Android, và nó không có nghĩa là mạng đang hỏng.",
  },
} as const;

export type KieuKetNoi = keyof typeof KIEU_KET_NOI;

/**
 * Quy một giá trị `networkType` của nền tảng về một nhãn ứng dụng có.
 *
 * ⚠ GIÁ TRỊ LẠ RƠI VỀ "KHÔNG XÁC ĐỊNH", KHÔNG NÉM LỖI VÀ KHÔNG HIỆN CHUỖI THÔ.
 *
 *   `NetworkType` hôm nay có bốn giá trị (`node_modules/zmp-sdk/index.d.ts` dòng 7–16). Nền
 *   tảng được phép thêm giá trị thứ năm ở một bản SDK sau, và lúc ấy ứng dụng này đã nằm trên
 *   máy người dùng rồi. Hiện thẳng chuỗi thô (`"5g"`, `"ethernet"`) là hiện chữ kỹ thuật tiếng
 *   Anh giữa một màn hình tiếng Việt; ném lỗi là làm vỡ một tính năng vì nền tảng vừa tốt lên.
 *   Rơi về nhãn an toàn thì màn hình vẫn đọc được và vẫn không nói sai điều gì.
 */
export function kieuKetNoi(tu_nen_tang: string): KieuKetNoi {
  return tu_nen_tang === "wifi" || tu_nen_tang === "cellular" || tu_nen_tang === "none"
    ? tu_nen_tang
    : "khong-xac-dinh";
}

export const DUONG_TRUYEN = {
  dan_nhap:
    "Trước khi gọi thử tổng đài, xem máy bạn đang nối mạng bằng đường nào. Kết quả chỉ cho biết KIỂU kết nối, không phải tốc độ.",
  nhan_ket_qua: "Kiểu kết nối hiện tại",
  /** Câu khẳng định ranh giới, hiện cùng mọi kết quả. Cùng khuôn với `VAN_PHONG.chua_xep_duoc`. */
  khong_do_toc_do:
    "Ứng dụng không đo tốc độ, không đo độ trễ và không chấm điểm chất lượng cuộc gọi — nền tảng chỉ trả về kiểu kết nối. Muốn biết đường truyền có đủ cho tổng đài của bạn hay không, đội kỹ thuật của chúng tôi đo trực tiếp trên hệ thống thật.",
} as const;

/** ---------- Danh thiếp số của chúng tôi ---------- */

export const THIEP_CUA_CHUNG_TOI = {
  dan_nhap:
    "Chìa mã này ra để người đối diện quét và lưu thẳng vào danh bạ. Mọi thông tin trong mã đều nằm dưới dạng chữ ngay bên dưới.",
  nhan_ma: "Mã QR danh thiếp",
  tieu_de_thong_tin: "Thông tin trong mã",
  nut_giu_sang: "Giữ màn hình sáng",
  nut_thoi_giu_sang: "Thôi giữ màn hình sáng",
  dang_giu_sang:
    "Màn hình đang được giữ sáng để người khác kịp quét. Ứng dụng tự tắt chế độ này khi bạn rời màn hình, để không làm hao pin máy bạn.",
  da_tai_xong:
    "Đã ghi tệp danh thiếp xuống máy bạn. Bạn mở tệp ấy ra là thêm được chúng tôi vào danh bạ.",
  /** Nói ra đúng thứ tệp ấy chứa — và đúng thứ nó KHÔNG chứa. */
  tep_la_cua_chung_toi:
    "Tệp này chứa thông tin liên hệ của chúng tôi, không chứa thông tin nào của bạn. Ứng dụng không đọc và không ghi bất kỳ tệp nào khác trên máy.",
  /** Ranh giới của `downloadFile`: nền tảng quyết định chỗ lưu, ta không chọn được. */
  zalo_quyet_dinh_cho_luu:
    "Zalo là bên quyết định tệp được lưu vào thư mục nào của máy; ứng dụng chỉ đưa nội dung tấm thiếp cho Zalo ghi hộ.",
} as const;

/** ---------- Số hoá danh thiếp giấy ---------- */

export const SO_HOA_THIEP = {
  dan_nhap:
    "Có một tấm thiếp giấy trong tay? Chụp hoặc chọn ảnh của nó ở đây để không phải giữ tờ giấy ấy nữa.",
  nut_chon_anh: "Chọn ảnh danh thiếp",
  dang_chon_anh: "Đang mở cửa sổ chọn ảnh…",
  cho_phep: "Bạn đã cho phép dùng máy ảnh. Bấm nút bên dưới để chụp hoặc chọn ảnh tấm thiếp.",
  /**
   * TỪ CHỐI QUYỀN LÀ ĐƯỜNG ĐI BÌNH THƯỜNG, VÀ NÓ KHÔNG LÀM MẤT TÍNH NĂNG: cửa sổ chọn ảnh vẫn
   * mở được từ thư viện. Câu này nói việc cần làm tiếp, không trách móc, và không có mã lỗi.
   *
   * Về việc đổi ý sau khi đã từ chối: `zmp-sdk/index.d.ts` ghi ở phần `authorize` rằng *"sau khi
   * user đồng ý hoặc từ chối cấp quyền, trạng thái cấp quyền sẽ được ghi nhận và đồng bộ cho
   * những lần sử dụng sau này"*, và ghi ở `openPermissionSetting` rằng có một cửa sổ Quản lý
   * quyền để người dùng cấp thêm hoặc thu hồi. Câu dưới nói đúng hai điều ấy, không hơn.
   */
  tu_choi_quyen:
    "Bạn đã từ chối quyền dùng máy ảnh. Zalo ghi nhớ câu trả lời này cho những lần sau, nên hộp thoại sẽ không hiện lại; muốn đổi ý, bạn mở phần Quản lý quyền của ứng dụng này trong Zalo và bật lại. Bạn vẫn chọn được ảnh đã chụp sẵn bằng nút bên dưới.",
  nhan_anh: "Ảnh danh thiếp bạn vừa chọn",
  khong_hien_duoc_anh:
    "Chưa hiện được ảnh này lên màn hình. Bạn hãy bấm lại nút chọn ảnh và chọn một tấm khác.",
  khong_chon_anh:
    "Bạn chưa chọn tấm ảnh nào. Bấm lại nút bên trên bất cứ lúc nào bạn muốn.",
  /**
   * ⚠ RANH GIỚI PHẢI NÓI RA, KHÔNG ĐƯỢC GIẢ VỜ VƯỢT QUA.
   *
   *   Bóc chữ khỏi ảnh cần OCR, và OCR cần một bước máy chủ — thứ bản dựng này không có và dây
   *   bẫy trong `phase1-collects-nothing.test.ts` cấm. Viết một hàm giả vờ đang nhận dạng, hay
   *   một câu "đang phân tích…", là nói dối bằng giao diện với người vừa đưa ảnh của mình vào.
   */
  chua_doc_duoc_chu:
    "Bản hiện tại chưa đọc được chữ trên ảnh. Việc bóc tên, số điện thoại và email ra khỏi một tấm ảnh cần bước nhận dạng chạy trên máy chủ, và bản này chưa có bước đó — ảnh chỉ được hiện lên màn hình.",
  /** Luật 3: tấm thiếp giấy là dữ liệu cá nhân của NGƯỜI KHÁC. */
  anh_khong_roi_may:
    "Ảnh bạn chọn không rời khỏi máy: ứng dụng không tải nó lên máy chủ nào, không lưu lại và không gửi đi đâu. Nó biến mất khi bạn chọn ảnh khác hoặc rời màn hình. Nếu tấm thiếp là của một người khác, thông tin trên đó là dữ liệu cá nhân của họ.",
} as const;

/**
 * MỤC VỀ CÁC QUYỀN TRONG CHÍNH SÁCH QUYỀN RIÊNG TƯ — dựng từ chính `NOI_DUNG_TINH_NANG`.
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
  "Ứng dụng xin các quyền của nền tảng Zalo cho đúng sáu tính năng dưới đây. Mỗi quyền chỉ được hỏi khi bạn tự bấm nút, và bạn có quyền từ chối mà vẫn dùng được ứng dụng.",
  ...NOI_DUNG_TINH_NANG.map((mot) => `${mot.nhan_ngan} — ${mot.vi_sao}`),
  "Với số điện thoại và vị trí, Zalo không trả giá trị thật về máy: ứng dụng chỉ nhận một mã dùng được một lần và hết hạn sau 2 phút. Số điện thoại và toạ độ của bạn không nằm trong mã đó.",
  "Với quét mã QR, nội dung mã hiện lên màn hình và mất đi khi bạn quét mã khác hoặc rời màn hình. Nếu mã là một tấm danh thiếp, nội dung ấy là dữ liệu cá nhân của người đã đưa nó cho bạn, và ứng dụng cũng không lưu lại.",
  // VẾ TRONG NGOẶC THÊM Ở BẢN 1.4, VÀ NÓ BẮT BUỘC PHẢI CÓ. Câu này nói "ứng dụng không nhận địa
  // chỉ IP" — đúng, vì `getNetworkType` chỉ trả về kiểu kết nối. Nhưng từ bản 1.4, mục Đăng nhập
  // khai rằng MÁY CHỦ ghi lại địa chỉ IP của mỗi lần đăng nhập. Hai câu ấy khác chủ ngữ và đều
  // đúng, nhưng đọc liền nhau thì người ta kết luận "địa chỉ IP không bao giờ bị chạm tới" — một
  // kết luận SAI mà chính văn bản này vừa mời gọi. Nói ra ngay tại chỗ, đừng bắt người đọc tự
  // đối chiếu hai mục cách nhau nửa trang.
  "Với thông tin mạng, ứng dụng chỉ nhận về KIỂU kết nối: Wi-Fi, mạng di động, không có mạng, hoặc không xác định. Ứng dụng không nhận địa chỉ IP, không nhận tên mạng Wi-Fi, không đo tốc độ và không biết bạn đang ở đâu. (Riêng khi bạn bấm đăng nhập, máy chủ nhìn thấy địa chỉ IP của lời gọi ấy và ghi vào nhật ký đăng nhập — xem mục Đăng nhập.)",
  "Với máy ảnh và cửa sổ chọn ảnh, tấm ảnh bạn chọn KHÔNG RỜI KHỎI MÁY: ứng dụng nhận một đường dẫn tạm trên chính thiết bị của bạn, hiện ảnh lên màn hình, và không tải ảnh lên bất kỳ máy chủ nào. Ứng dụng không tự đọc thư viện ảnh — nó chỉ nhận đúng tấm ảnh bạn tự chọn trong cửa sổ của Zalo.",
  "Với việc giữ màn hình sáng, ứng dụng chỉ bật chế độ ấy khi bạn tự bấm, và tự tắt lại khi bạn rời màn hình danh thiếp. Chế độ này không đọc gì và không gửi gì; nó chỉ ngăn màn hình tối đi trong lúc người khác đang quét mã.",
  "Với việc tải tệp, ứng dụng GHI MỘT TỆP XUỐNG MÁY BẠN, và đây là hành vi duy nhất ứng dụng viết lên thiết bị. Tệp ấy là danh thiếp của chúng tôi — tên, hotline, email và trang web của công ty — không phải dữ liệu của bạn. Ứng dụng không đọc, không sửa và không xoá bất kỳ tệp nào khác.",
  // CÂU CUỐI ĐÃ ĐỔI Ở PHIÊN BẢN 1.3, VÀ VIỆC ĐỔI NÓ LÀ BẮT BUỘC. Bản 1.2 viết "chưa gửi bất kỳ
  // dữ liệu nào của bạn đi đâu" — một câu ĐÚNG với bản nộp và SAI với bản dựng có bước đăng
  // nhập. Một câu chỉ đúng ở một nửa số bản dựng là một câu sai ở nửa kia, và không có gì đỏ
  // lên. Nên nó trỏ sang mục "Đăng nhập", nơi từng biến thể tự nói ra điều nó thật sự làm.
  "Các mục đích nêu trên là mục đích ứng dụng sẽ dùng các quyền này khi có đầy đủ chức năng. Ngoài đúng một tệp danh thiếp của chúng tôi nói ở trên, ứng dụng không lưu gì xuống máy bạn; việc đăng nhập có gửi gì đi hay không thì mục Đăng nhập bên dưới nói rõ cho đúng bản bạn đang dùng.",
];

/**
 * TỪNG QUYỀN MỘT, GỌI ĐÚNG TÊN API — mục thứ hai của chính sách về quyền.
 *
 * VÌ SAO KHÔNG GỘP VÀO ĐOẠN TRÊN: đoạn trên nói theo TÍNH NĂNG, thứ người dùng hiểu. Bảng này
 * nói theo QUYỀN, thứ Developer Console cấp và người duyệt đối chiếu. Một tính năng dùng hai
 * quyền, nên hai cách kể không thay thế nhau được — và người duyệt phải tra được từng quyền họ
 * sắp bấm nút cấp.
 *
 * `getAccessToken` CÓ MẶT Ở ĐÂY DÙ KHÔNG PHẢI MỘT QUYỀN PHẢI CẤP: `zmp-sdk/index.d.ts` dòng
 * 3009 ghi rằng từ SDK 2.35.0 ứng dụng mặc định truy xuất được access token mà không cần người
 * dùng xác nhận. Nhưng nó vẫn là một thứ ứng dụng NHẬN VỀ và GỬI ĐI ở bước đăng nhập, nên
 * Nghị định 13 buộc nói ra. Số quyền phải xin ở Developer Console vẫn là CHÍN.
 */
export const DOAN_CHINH_SACH_TUNG_QUYEN: readonly string[] = [
  "Số điện thoại (getPhoneNumber) — để bạn đăng nhập bằng một lần chạm, và để gửi thông báo ZNS tới đúng số ấy. Ứng dụng chỉ nhận một mã, không nhận số.",
  "Thông tin xác thực phiên Zalo (getAccessToken) — mã này cho biết bạn là người dùng Zalo nào đối với riêng ứng dụng này. Nó đi cùng mã số điện thoại ở bước đăng nhập, và không cho ứng dụng biết tên hay ảnh đại diện của bạn.",
  "Vị trí (getLocation) — để chỉ ra văn phòng gần bạn. Ứng dụng chỉ nhận một mã, không nhận toạ độ.",
  "Quét mã QR (scanQRCode) — để đọc danh thiếp số của đối tác.",
  "Kiểu kết nối mạng (getNetworkType) — để cho bạn biết cuộc gọi sắp tới đi qua Wi-Fi hay mạng di động.",
  "Rung (vibrate) — để báo bằng một nhịp rung khi một việc bạn vừa bấm đã xong, cho người không nhìn màn hình liên tục.",
  "Giữ màn hình sáng (keepScreen) — để màn hình không tối đi trong lúc người khác quét mã danh thiếp của chúng tôi.",
  "Máy ảnh (requestCameraPermission) — để chụp lại một tấm danh thiếp giấy.",
  "Chọn ảnh (openMediaPicker) — để bạn chọn ảnh tấm danh thiếp từ máy. Ảnh không rời khỏi máy.",
  "Tải tệp (downloadFile) — để ghi tệp danh thiếp của chúng tôi xuống máy bạn.",
];
