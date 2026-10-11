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
 * ⚠ NGỮ CẢNH LÀ MỘT DOANH NGHIỆP CÔNG NGHỆ, KHÔNG PHẢI MỘT DỊCH VỤ CÔNG. ViHAT Group bán giải
 * pháp tổng đài đám mây, CRM và ứng dụng AI cho doanh nghiệp. Mọi câu ở đây nói với một khách
 * hàng hoặc một đối tác đang tìm hiểu giải pháp.
 *
 * ⚠ HAI CÂU TRONG TỆP NÀY NÓI "máy chủ của Tập đoàn ViHAT Group" — ĐỔI 25/09/2026 (câu mở #28):
 * từ 21/09 tới 25/09 chúng cố ý còn nói "VihatSoftware" vì chưa ai xác nhận ai vận hành kho
 * `vihat-miniapp` sau chuyển giao; chủ dự án đã xác nhận đó là Tập đoàn ViHAT Group (ADR 0044
 * câu 2). Hai câu ấy phải khớp từng chữ với chính sách quyền riêng tư — người dùng đọc cả hai
 * chỗ, và hai chỗ nói hai nơi nhận khác nhau là một mâu thuẫn trong chính thứ Nghị định 13 bắt
 * khai. Xem `content/chinh-sach-rieng-tu.ts`, khối "CHUYỂN QUYỀN SỞ HỮU APP".
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
 * GIỌNG VĂN (11/10/2026, chủ dự án): đây là ứng dụng đã phát hành của một tập đoàn — câu ngắn, một ý,
 * không kể cách ứng dụng được dựng, không chỉ tay "nút bên trên/bên dưới", không trấn an "Không sao cả".
 * Mục đích dùng từng quyền vẫn nói rõ trong `vi_sao` (điều 3.3.4); chi tiết dữ liệu đi đâu nằm ở
 * chính sách quyền riêng tư và màn Quản lý quyền, không lặp lại dưới mỗi nút.
 */
const CHI_TRONG_ZALO = "Tính năng này chỉ khả dụng trên ứng dụng Zalo.";

export const NOI_DUNG_TINH_NANG: readonly NoiDungTinhNang[] = [
  {
    ma: "danh-thiep",
    nhan_ngan: "Quét danh thiếp",
    tieu_de: "Quét danh thiếp số của đối tác",
    vi_sao:
      "Dùng máy ảnh quét mã QR trên danh thiếp của đối tác để lưu nhanh tên, công ty, số điện thoại và email.",
    nut: "Quét mã QR",
    dang_cho: "Đang mở máy ảnh…",
    tu_choi: "Đã huỷ quét mã.",
    ngoai_zalo: CHI_TRONG_ZALO,
    khong_lay_duoc: "Không quét được mã. Vui lòng thử lại.",
  },
  {
    ma: "van-phong",
    nhan_ngan: "Tìm văn phòng",
    tieu_de: "Tìm văn phòng gần bạn",
    vi_sao: "Dùng vị trí của bạn để gợi ý văn phòng ViHAT Group thuận tiện nhất khi bạn muốn đến gặp trực tiếp.",
    nut: "Tìm văn phòng gần tôi",
    dang_cho: "Đang xác định vị trí…",
    tu_choi: "Bạn có thể chọn một văn phòng trong danh sách để xem chỉ đường.",
    ngoai_zalo: CHI_TRONG_ZALO,
    khong_lay_duoc: "Không xác định được vị trí. Vui lòng thử lại.",
  },
  {
    /**
     * KHỐI ĐĂNG NHẬP — `getPhoneNumber` là đường đăng nhập MỘT CHẠM, không phải một biểu mẫu
     * đăng ký. Mục đích phải nói thẳng (điều 3.3.4): số điện thoại dùng để ĐĂNG NHẬP và để NHẬN
     * THÔNG BÁO.
     *
     * ⚠ KHÔNG MỘT CHỮ NÀO HỨA MỘT MÃ OTP (ADR 0020).
     */
    ma: "dang-nhap",
    nhan_ngan: "Đăng nhập",
    tieu_de: "Đăng nhập bằng số Zalo",
    vi_sao:
      "Dùng số điện thoại Zalo để đăng nhập nhanh, gửi yêu cầu, theo dõi tình trạng xử lý và nhận thông báo kết quả qua ZNS.",
    nut: "Đăng nhập bằng số Zalo",
    dang_cho: "Đang đăng nhập…",
    tu_choi: "Bạn chưa chia sẻ số điện thoại nên chưa thể đăng nhập.",
    ngoai_zalo: CHI_TRONG_ZALO,
    khong_lay_duoc: "Đăng nhập không thành công. Vui lòng thử lại.",
  },
  {
    ma: "duong-truyen",
    nhan_ngan: "Kiểm tra đường truyền",
    tieu_de: "Kiểm tra đường truyền trước cuộc gọi",
    vi_sao:
      "Xem thiết bị đang kết nối qua Wi-Fi hay mạng di động — yếu tố ảnh hưởng trực tiếp tới chất lượng cuộc gọi tổng đài.",
    nut: "Kiểm tra đường truyền",
    dang_cho: "Đang kiểm tra…",
    tu_choi: "Đã huỷ kiểm tra.",
    ngoai_zalo: CHI_TRONG_ZALO,
    khong_lay_duoc: "Không kiểm tra được kết nối. Vui lòng thử lại.",
  },
  {
    ma: "thiep-cua-chung-toi",
    nhan_ngan: "Danh thiếp của chúng tôi",
    // TÊN CÔNG TY VIẾT THẲNG, KHÔNG GHÉP BẰNG `${COMPANY.name}`: một chuỗi ghép lúc chạy không có
    // trong bundle, và ca "bản nộp có đủ chữ của sáu tính năng" trong `bundle-for-zalo.test.ts` bắt
    // đúng điều đó.
    tieu_de: "Danh thiếp số của ViHAT Group",
    vi_sao:
      "Đưa mã QR cho đối tác quét, hoặc tải danh thiếp về máy để thêm vào danh bạ. Có thể giữ màn hình luôn sáng để mã không tắt trong lúc quét.",
    nut: "Tải danh thiếp (.vcf)",
    dang_cho: "Đang tải…",
    tu_choi: "Đã huỷ tải danh thiếp.",
    ngoai_zalo: CHI_TRONG_ZALO,
    khong_lay_duoc: "Không tải được danh thiếp. Vui lòng thử lại.",
  },
  {
    ma: "so-hoa-thiep",
    nhan_ngan: "Số hoá thiếp giấy",
    // "Chụp ảnh", không "Số hoá": tính năng hiện ảnh lên màn hình, không bóc chữ khỏi ảnh. Tiêu đề
    // không được hứa một việc ứng dụng không làm (`phase1-collects-nothing.test.ts` cấm bước OCR).
    tieu_de: "Chụp ảnh danh thiếp giấy",
    vi_sao: "Dùng máy ảnh hoặc chọn ảnh có sẵn để lưu lại danh thiếp giấy ngay trên điện thoại.",
    nut: "Bật máy ảnh",
    dang_cho: "Đang yêu cầu quyền máy ảnh…",
    tu_choi:
      // Nguồn của câu về iOS: bảng Quản lý quyền trong Zalo Developer Console, ô ghi chú của quyền
      // "Yêu cầu thiết bị cấp quyền truy cập camera". Trên iOS người dùng không được hỏi lại, nên
      // đây là lúc duy nhất để nói.
      "Chưa có quyền dùng máy ảnh. Bạn vẫn có thể chọn ảnh có sẵn. Trên iPhone, hãy bật lại quyền trong phần Cài đặt.",
    ngoai_zalo: CHI_TRONG_ZALO,
    khong_lay_duoc: "Không yêu cầu được quyền máy ảnh. Vui lòng thử lại.",
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
 * Nền tảng trả về mã RỖNG ở môi trường phát triển (`zmp-sdk/index.d.ts`: *"Ở môi trường phát triển,
 * API này sẽ luôn thành công và trả về token rỗng"*). Trên máy thật thì hiếm; câu nói việc cần làm.
 */
export const MA_RONG = "Không nhận được phản hồi từ Zalo. Vui lòng thử lại.";

/** ---------- Màn quét danh thiếp ---------- */

export const DANH_THIEP = {
  nut_quet_lai: "Quét mã khác",
  tieu_de_ket_qua: "Thông tin trên danh thiếp",
  nhan_ho_ten: "Họ và tên",
  nhan_to_chuc: "Công ty",
  nhan_chuc_danh: "Chức danh",
  nhan_dien_thoai: "Điện thoại",
  nhan_email: "Email",
  nhan_trang_web: "Trang web",
  nut_goi: "Gọi",
  nut_email: "Gửi email",
  nut_mo_lien_ket: "Mở liên kết",
  ma_rong: "Mã QR không có nội dung.",
  /** `BEGIN:VCARD` hợp lệ nhưng bên trong không có trường nào dùng được. */
  thiep_rong: "Danh thiếp này không có thông tin liên hệ.",
  tieu_de_lien_ket: "Liên kết",
  tieu_de_van_ban: "Nội dung mã QR",
  /** Người dùng cần biết chữ hiện ra là nguyên văn mã, không phải một bản đã sửa. */
  nguyen_van: "Nguyên văn:",
} as const;

/**
 * Gọi điện, mở trang web và chỉ đường đều đi qua API `@zaloOnly` của nền tảng (`openPhone` ·
 * `openWebview`). Ngoài Zalo thì chúng không chạy — nói ra bằng một câu, không để người dùng bấm vào
 * một cái nút im lặng. Không mở đầu bằng "Chưa mở": đó là nhãn của trang xã mẫu, thứ
 * `bundle-for-zalo.test.ts` cấm bản nộp mang.
 */
export const LOI_MO_NGOAI = CHI_TRONG_ZALO;

/** ---------- Tìm văn phòng ---------- */

export const VAN_PHONG = {
  nut_chi_duong: "Chỉ đường",
  /**
   * Sau khi người dùng chia sẻ vị trí. `getLocation` chỉ trả token, và màn này chưa đổi token thành
   * toạ độ — nên câu KHÔNG nói "đã xếp theo khoảng cách". Nó mời chọn, không khẳng định.
   */
  da_nhan_vi_tri: "Chọn văn phòng thuận tiện nhất để xem chỉ đường.",
} as const;

/** ---------- Đăng nhập bằng số Zalo ---------- */

export const DANG_NHAP = {
  nhac_lien_he: "Hoặc liên hệ trực tiếp",
} as const;

/** ---------- Kiểm tra đường truyền ---------- */

/**
 * NHÃN VÀ Ý NGHĨA CỦA TỪNG KIỂU KẾT NỐI.
 *
 * ⚠ KHÔNG MỘT CON SỐ NÀO Ở ĐÂY, VÀ ĐÓ LÀ MỘT QUYẾT ĐỊNH: `getNetworkType` trả về đúng một chuỗi —
 * kiểu kết nối. Không đo độ trễ, không đo băng thông. Bịa một con số trong ứng dụng của một nhà cung
 * cấp hạ tầng thoại là bịa đúng chỗ khách hàng sẽ đem đối chiếu với máy đo thật.
 */
export const KIEU_KET_NOI = {
  wifi: {
    nhan: "Wi-Fi",
    y_nghia: "Cuộc gọi có thể bị gián đoạn khi bạn ra khỏi vùng phủ sóng Wi-Fi.",
  },
  cellular: {
    nhan: "Mạng di động",
    y_nghia: "Chất lượng cuộc gọi phụ thuộc vào vùng phủ sóng tại vị trí của bạn.",
  },
  none: {
    nhan: "Không có mạng",
    y_nghia: "Vui lòng bật Wi-Fi hoặc dữ liệu di động để thực hiện cuộc gọi.",
  },
  "khong-xac-dinh": {
    nhan: "Không xác định",
    y_nghia: "Thiết bị không cung cấp thông tin về loại kết nối.",
  },
} as const;

export type KieuKetNoi = keyof typeof KIEU_KET_NOI;

/**
 * Quy một giá trị `networkType` của nền tảng về một nhãn ứng dụng có.
 *
 * ⚠ GIÁ TRỊ LẠ RƠI VỀ "KHÔNG XÁC ĐỊNH", KHÔNG NÉM LỖI VÀ KHÔNG HIỆN CHUỖI THÔ: nền tảng được phép
 * thêm giá trị ở một bản SDK sau, khi ứng dụng này đã nằm trên máy người dùng.
 */
export function kieuKetNoi(tu_nen_tang: string): KieuKetNoi {
  return tu_nen_tang === "wifi" || tu_nen_tang === "cellular" || tu_nen_tang === "none"
    ? tu_nen_tang
    : "khong-xac-dinh";
}

export const DUONG_TRUYEN = {
  nhan_ket_qua: "Kết nối hiện tại",
} as const;

/** ---------- Danh thiếp số của chúng tôi ---------- */

export const THIEP_CUA_CHUNG_TOI = {
  nhan_ma: "Mã QR danh thiếp",
  tieu_de_thong_tin: "Thông tin liên hệ",
  nut_giu_sang: "Giữ màn hình sáng",
  nut_thoi_giu_sang: "Tắt giữ màn hình sáng",
  dang_giu_sang: "Màn hình luôn sáng cho tới khi bạn rời trang này.",
  da_tai_xong: "Đã tải danh thiếp. Mở tệp để thêm vào danh bạ.",
} as const;

/** ---------- Số hoá danh thiếp giấy ---------- */

export const SO_HOA_THIEP = {
  nut_chon_anh: "Chọn ảnh danh thiếp",
  dang_chon_anh: "Đang mở thư viện ảnh…",
  cho_phep: "Máy ảnh đã sẵn sàng. Chọn ảnh danh thiếp để tiếp tục.",
  /**
   * Zalo ghi nhớ câu trả lời (`authorize` trong `zmp-sdk/index.d.ts`), và `openPermissionSetting` mở
   * phần Quản lý quyền. Câu dưới nói đúng hai điều ấy.
   */
  tu_choi_quyen:
    "Chưa có quyền dùng máy ảnh. Bạn có thể bật lại trong phần Quản lý quyền của ứng dụng trên Zalo, hoặc chọn ảnh có sẵn.",
  nhan_anh: "Ảnh danh thiếp",
  khong_hien_duoc_anh: "Không hiển thị được ảnh. Vui lòng chọn ảnh khác.",
  khong_chon_anh: "Chưa có ảnh nào được chọn.",
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
  // 06/10/2026 DRAFT (`PENDING_APPROVAL_MARK`, `content/chinh-sach-rieng-tu.ts`): "đúng sáu tính năng" became
  // false when the shared app opened from a commune QR began asking phone, location, camera, picker and Zalo
  // name for petitions (`App.tsx` `QrCommuneApp`). The six features stay the intro's; the commune page is said.
  // 07/10/2026: "Chat với chuyên viên" and "Nhận ưu đãi qua SMS" ask the phone (through the one-tap login) and,
  // for the chat, the Zalo name — in the intro, so the intro is no longer "six features" only.
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Trong phần giới thiệu, ứng dụng xin các quyền của nền tảng Zalo cho sáu tính năng dưới đây, và cho hai nút một chạm: 'Chat với chuyên viên' ở màn Liên hệ xin số điện thoại và tên Zalo của bạn để chuyên viên biết ai đang nhắn; 'Đăng ký nhận' ưu đãi SMS ở Trang chủ xin số điện thoại để gửi tin nhắn ưu đãi tới đúng số ấy. Trên trang của một xã, ứng dụng dùng thêm một số quyền ấy cho việc gửi phản ánh — số điện thoại, vị trí, máy ảnh và chọn ảnh — và xin thêm quyền đọc tên Zalo của bạn; mục 'Phản ánh gửi tới xã' và mục 'Danh sách từng quyền' nói rõ. Mỗi quyền chỉ được hỏi khi bạn tự bấm nút, và bạn có quyền từ chối mà vẫn dùng được ứng dụng.",
  ...NOI_DUNG_TINH_NANG.map((mot) => `${mot.nhan_ngan} — ${mot.vi_sao}`),
  "Với số điện thoại và vị trí, Zalo không trả giá trị thật về máy: ứng dụng chỉ nhận một mã dùng được một lần và hết hạn sau 2 phút. Số điện thoại và toạ độ của bạn không nằm trong mã đó.",
  // 06/10/2026 DRAFT: the sentence above is still true of what ZALO returns; this one says what the commune
  // page's location exchange (`vihat-miniapp` `/api/v1/location`) brings back. The verified number never does.
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Riêng khi bạn bấm 'Lấy vị trí hiện tại' lúc gửi phản ánh trên trang của một xã, máy chủ của Tập đoàn ViHAT Group đổi mã vị trí thành toạ độ và trả toạ độ về máy bạn, để hiện lên màn hình và gửi kèm phản ánh nếu bạn bấm gửi. Số điện thoại Zalo của bạn thì không bao giờ được trả về máy.",
  "Với quét mã QR, nội dung mã hiện lên màn hình và mất đi khi bạn quét mã khác hoặc rời màn hình. Nếu mã là một tấm danh thiếp, nội dung ấy là dữ liệu cá nhân của người đã đưa nó cho bạn, và ứng dụng cũng không lưu lại.",
  // VẾ TRONG NGOẶC LÀ BẮT BUỘC, KHÔNG PHẢI TRANG TRÍ. Câu này nói "ứng dụng không nhận địa
  // chỉ IP" — đúng, vì `getNetworkType` chỉ trả về kiểu kết nối. Nhưng mục Đăng nhập
  // khai rằng MÁY CHỦ ghi lại địa chỉ IP của mỗi lần đăng nhập. Hai câu ấy khác chủ ngữ và đều
  // đúng, nhưng đọc liền nhau thì người ta kết luận "địa chỉ IP không bao giờ bị chạm tới" — một
  // kết luận SAI mà chính văn bản này vừa mời gọi. Nói ra ngay tại chỗ, đừng bắt người đọc tự
  // đối chiếu hai mục cách nhau nửa trang.
  "Với thông tin mạng, ứng dụng chỉ nhận về KIỂU kết nối: Wi-Fi, mạng di động, không có mạng, hoặc không xác định. Ứng dụng không nhận địa chỉ IP, không nhận tên mạng Wi-Fi, không đo tốc độ và không biết bạn đang ở đâu. (Riêng khi bạn bấm đăng nhập, máy chủ nhìn thấy địa chỉ IP của lời gọi ấy và ghi vào nhật ký đăng nhập — xem mục Đăng nhập.)",
  // DRAFT 02/10/2026 (`PENDING_APPROVAL_MARK`, `content/chinh-sach-rieng-tu.ts`): "KHÔNG RỜI KHỎI MÁY" is true of
  // the business-card feature only; the commune app's scene photos are uploaded. The feature is now named and the
  // exception pointed to — the approved promise itself is unchanged.
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Với máy ảnh và cửa sổ chọn ảnh ở tính năng số hoá danh thiếp giấy, tấm ảnh bạn chọn KHÔNG RỜI KHỎI MÁY: ứng dụng nhận một đường dẫn tạm trên chính thiết bị của bạn, hiện ảnh lên màn hình, và không tải ảnh lên bất kỳ máy chủ nào. Ứng dụng không tự đọc thư viện ảnh — nó chỉ nhận đúng tấm ảnh bạn tự chọn trong cửa sổ của Zalo. Riêng ảnh hiện trường bạn tự gửi kèm một phản ánh trên trang của một xã thì được tải lên: mục 'Ảnh hiện trường gửi kèm phản ánh' nói rõ.",
  "Với việc giữ màn hình sáng, ứng dụng chỉ bật chế độ ấy khi bạn tự bấm, và tự tắt lại khi bạn rời màn hình danh thiếp. Chế độ này không đọc gì và không gửi gì; nó chỉ ngăn màn hình tối đi trong lúc người khác đang quét mã.",
  // LIỆT KÊ ĐÚNG NHỮNG TRƯỜNG TẤM THIẾP THẬT SỰ CHỨA. Câu này và câu `ghi-tep` trong chính sách
  // phải khớp nhau VÀ khớp với `vcard.ts`.
  //
  // "trang web" ĐÃ RỜI KHỎI CÂU NÀY RỒI QUAY LẠI TRONG CÙNG MỘT NGÀY, và lần quay lại suýt không
  // xảy ra — đó mới là phần đáng ghi. Sáng 21/09/2026 nó bị bỏ vì `COMPANY.website` đang trống
  // nên `vcard.ts` không sinh dòng `URL`. Chiều cùng ngày website được điền, dòng `URL` sinh trở
  // lại — nhưng CÂU NÀY thì không, vì tiền đề của nó nằm trong một chú thích chứ không nằm trong
  // một phép kiểm. Hệ quả: văn bản khai THIẾU một trường mà tệp người dùng tải về CÓ, tức mô tả
  // sai đúng thứ vừa gửi xuống máy họ.
  //
  // Câu tương ứng trong chính sách thì thoát, vì nó CÓ ca canh "khi và chỉ khi". Nay ca ấy phủ
  // cả hai câu — xem `chinh-sach.test.ts`, khối "khai của tệp danh thiếp khớp với tấm thiếp thật".
  //
  // 06/10/2026 DRAFT: "hành vi duy nhất ứng dụng viết lên thiết bị" is false since the commune page keeps a
  // petition draft in the app's storage (`commune-app/feedback-draft-store.ts`). The card is still the only FILE.
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Với việc tải tệp, ứng dụng GHI MỘT TỆP XUỐNG MÁY BẠN, và đây là tệp duy nhất ứng dụng ghi vào thư mục của máy. Tệp ấy là danh thiếp của chúng tôi — tên, hotline, email và trang web của công ty — không phải dữ liệu của bạn. Ứng dụng không đọc, không sửa và không xoá bất kỳ tệp nào khác.",
  // CÂU CUỐI ĐÃ PHẢI ĐỔI, VÀ VIỆC ĐỔI NÓ LÀ BẮT BUỘC. Bản trước viết "chưa gửi bất kỳ
  // dữ liệu nào của bạn đi đâu" — một câu ĐÚNG với bản nộp và SAI với bản dựng có bước đăng
  // nhập. Một câu chỉ đúng ở một nửa số bản dựng là một câu sai ở nửa kia, và không có gì đỏ
  // lên. Nên nó trỏ sang mục "Đăng nhập", nơi từng biến thể tự nói ra điều nó thật sự làm.
  // 06/10/2026 DRAFT: "Ngoài đúng một tệp danh thiếp…, ứng dụng không lưu gì xuống máy bạn" — the draft again.
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Các mục đích nêu trên là mục đích ứng dụng sẽ dùng các quyền này khi có đầy đủ chức năng. Ngoài tệp danh thiếp của chúng tôi nói ở trên và bản nháp phản ánh trên trang của một xã — mục 'Bản nháp phản ánh trên máy bạn' nói rõ —, ứng dụng không lưu gì xuống máy bạn; việc gửi gì đi thì mục Đăng nhập và mục 'Phản ánh gửi tới xã' nói rõ.",
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
  // ⚠ HAI CÂU NÀY RỘNG RA 22/09/2026, CÙNG LÚC VỚI `KHAI_BAO_LOI_GOI`. Phiên đăng nhập nay còn là
  // CỬA VÀO màn "Tư vấn và báo giá" và màn "Yêu cầu của tôi". Người đã đồng ý cho "đăng nhập +
  // ZNS" chưa đồng ý cho "gửi yêu cầu tư vấn", và câu khai từng quyền là chỗ họ đọc điều đó.
  // 06/10/2026 DRAFT (`PENDING_APPROVAL_MARK`): the three lines below gained the commune page's purposes
  // (`KHAI_BAO_LOI_GOI` rows `getPhoneNumber`, `getAccessToken`, `getLocation`, `nua: "ca-hai"`), and "Ứng dụng
  // chỉ nhận một mã, không nhận toạ độ" was false there — the exchanged coordinates come back to the phone.
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Số điện thoại (getPhoneNumber) — để bạn đăng nhập bằng một lần chạm; để gửi thông báo ZNS tới đúng số ấy; để chúng tôi gọi lại đúng số ấy nếu bạn tự đề nghị gọi lại ở màn Tư vấn và báo giá; để chuyên viên biết ai đang nhắn khi bạn bấm 'Chat với chuyên viên'; để gửi tin nhắn SMS ưu đãi tới đúng số ấy nếu bạn bấm 'Đăng ký nhận', cho tới khi bạn bấm 'Huỷ nhận ưu đãi SMS'; và, trên trang của một xã, để xã biết phản ánh là của ai và chỉ bạn xem được phản ánh của bạn. Ứng dụng chỉ nhận một mã, không nhận số.",
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Thông tin xác thực phiên Zalo (getAccessToken) — mã này cho biết bạn là người dùng Zalo nào đối với riêng ứng dụng này. Nó đi cùng mã số điện thoại ở bước đăng nhập, và không cho ứng dụng biết tên hay ảnh đại diện của bạn. Phiên mở ra từ hai mã ấy là thứ cho bạn gửi yêu cầu tư vấn — kể cả yêu cầu trò chuyện với chuyên viên và việc đăng ký hay huỷ nhận ưu đãi SMS — và chỉ xem được yêu cầu của chính mình. Trên trang của một xã, mã này còn đi cùng mã số điện thoại để mở phiên làm việc với xã, và đi cùng mã vị trí khi bạn bấm 'Lấy vị trí hiện tại'.",
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Vị trí (getLocation) — để chỉ ra văn phòng gần bạn; và, trên trang của một xã, để lấy vị trí nơi xảy ra sự việc khi bạn gửi phản ánh. Ứng dụng chỉ nhận một mã. Ở màn Liên hệ, mã ở lại trên máy; khi gửi phản ánh, máy chủ của Tập đoàn ViHAT Group đổi mã thành toạ độ và trả toạ độ về máy.",
  "Quét mã QR (scanQRCode) — để đọc danh thiếp số của đối tác.",
  "Kiểu kết nối mạng (getNetworkType) — để cho bạn biết cuộc gọi sắp tới đi qua Wi-Fi hay mạng di động.",
  "Rung (vibrate) — để báo bằng một nhịp rung khi một việc bạn vừa bấm đã xong, cho người không nhìn màn hình liên tục.",
  "Giữ màn hình sáng (keepScreen) — để màn hình không tối đi trong lúc người khác quét mã danh thiếp của chúng tôi.",
  // DRAFT 02/10/2026 (`PENDING_APPROVAL_MARK`): both permissions gained a second purpose in the commune app
  // (`zalo-api.ts` rows `camera` / `photos`, `nua: "ca-hai"`). "Ảnh không rời khỏi máy" now names its feature.
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Máy ảnh (requestCameraPermission) — để chụp lại một tấm danh thiếp giấy; và, trên trang của một xã, để bạn chụp ảnh hiện trường gửi kèm một phản ánh.",
  // 08/10/2026: "Chụp ảnh" left this call for `chooseImage` (the phone's own camera, line below) — this one now
  // only picks existing photos on a commune page.
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Chọn ảnh (openMediaPicker) — để bạn chọn ảnh tấm danh thiếp từ máy, và ảnh danh thiếp không rời khỏi máy; trên trang của một xã, còn để bạn chọn tối đa 5 ảnh hiện trường có sẵn gửi kèm một phản ánh, và những ảnh ấy được tải lên hệ thống của xã sau khi phản ánh đã được ghi nhận.",
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Chụp ảnh bằng máy ảnh của điện thoại (chooseImage) — trên trang của một xã, để bạn chụp ảnh hiện trường gửi kèm một phản ánh khi chính bạn bấm 'Chụp ảnh'; ảnh ấy được tải lên hệ thống của xã sau khi phản ánh đã được ghi nhận.",
  "Tải tệp (downloadFile) — để ghi tệp danh thiếp của chúng tôi xuống máy bạn.",
  // 06/10/2026 DRAFT: `getUserInfo` was `nua: "nha-nuoc"` (commune app only) when this list was written; the
  // shared App ID calls it too since 3fe60cd0 (`App.tsx` `QrCommuneApp` passes `layTenChoXa`). Only the name is
  // read (`zalo-api.ts` `layTenZalo`).
  // 07/10/2026: "chỉ trên trang của một xã" became false — "Chat với chuyên viên" asks it in the intro and SENDS
  // it to ViHAT's server with the chat request (`api/hop-dong-yeu-cau.ts` `displayName`).
  "[CHỜ DUYỆT — bản nháp 02/10/2026, chủ dự án chưa duyệt] Tên Zalo (getUserInfo) — ở màn Liên hệ, khi bạn bấm 'Chat với chuyên viên': để gửi kèm yêu cầu trò chuyện tới máy chủ của Tập đoàn ViHAT Group, cho chuyên viên biết ai đang nhắn; và trên trang của một xã: để điền sẵn ô họ tên khi bạn gửi phản ánh, tên chỉ đi tới xã khi bạn tự bấm gửi phản ánh. Ứng dụng chỉ dùng tên hiển thị Zalo của bạn, sau khi bạn đồng ý; từ chối thì cửa sổ trò chuyện vẫn mở, chỉ là không có tên.",
];
