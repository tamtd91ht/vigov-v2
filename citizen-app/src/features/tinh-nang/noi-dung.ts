/**
 * MỌI CHỮ NGƯỜI DÙNG ĐỌC TRÊN CÁC TÍNH NĂNG CỦA PHẦN GIỚI THIỆU VIHAT — một tệp, giống `content/company-profile.ts`.
 *
 * VÌ SAO TÁCH RA KHỎI COMPONENT:
 *
 *   Người duyệt của Zalo đọc đúng những câu này, và đây là thứ quyết định app có được cấp các
 *   quyền hay không: chính sách Mini App (điều 3.3.4, trích trong `zmp-sdk/index.d.ts` ngay trên
 *   `getPhoneNumber`) từ chối xét duyệt những app xin quyền mà không nói rõ mục đích. Một câu
 *   giải thích nằm rải trong JSX là một câu không ai đọc lại trước khi nộp.
 *
 *   Tách ra còn để `bundle-for-zalo.test.ts` **dùng lại đúng danh sách này** khi khẳng định bản
 *   dựng nộp CÓ chứa đủ chữ của các tính năng. Một danh sách chép tay ở phía test sẽ lệch, và khi
 *   nó lệch thì phép kiểm xanh vì không tìm thấy gì — chứ không vì bản dựng đúng.
 *
 * ⚠ KHÔNG CÂU NÀO Ở ĐÂY ĐƯỢC HỨA MỘT VIỆC ỨNG DỤNG KHÔNG LÀM. Mô tả sai cho người duyệt là thứ
 * không sửa lại được sau đó — và mô tả sai cho một người đang chờ được gọi lại thì tệ hơn thế.
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
 * BA TÍNH NĂNG (11/10/2026: "Số hoá thiếp giấy", "Kiểm tra đường truyền" and the office location request were
 * removed by the owner). Thứ tự này là thứ tự hiện ra.
 *
 * MỘT TÍNH NĂNG CÓ THỂ CẦN HAI QUYỀN, VÀ ĐÓ LÀ LÝ DO HAI CON SỐ KHÔNG BẰNG NHAU: giữ màn hình
 * sáng và ghi tệp xuống máy là hai quyền, nhưng chúng phục vụ ĐÚNG MỘT việc người dùng làm —
 * chìa tấm danh thiếp ra cho người khác. Tách chúng thành hai "tính năng" để con số đẹp lên là
 * dựng hai cái nút không ai hiểu để làm gì, đúng thứ điều 3.3.4 của chính sách Mini App từ chối.
 */
export type MaTinhNang =
  | "danh-thiep"
  // ĐỔI TÊN TỪ `tu-van`, KHÔNG PHẢI THÊM MỘT TÍNH NĂNG: khối `getPhoneNumber` trên màn Liên hệ
  // nay là KHỐI ĐĂNG NHẬP (ADR 0020), không còn là "đăng ký nhận tư vấn". Tên mã đi theo việc
  // nó làm — một cái tên cũ còn lại là một cái tên sẽ được người sau đọc thành lời hứa cũ.
  | "dang-nhap"
  | "thiep-cua-chung-toi";

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
    ma: "thiep-cua-chung-toi",
    nhan_ngan: "Danh thiếp của chúng tôi",
    // TÊN CÔNG TY VIẾT THẲNG, KHÔNG GHÉP BẰNG `${COMPANY.name}`: một chuỗi ghép lúc chạy không có
    // trong bundle, và ca "bản nộp có đủ chữ của các tính năng" trong `bundle-for-zalo.test.ts` bắt
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
  tieu_de: "Văn phòng ViHAT Group",
  nut_chi_duong: "Chỉ đường",
} as const;

/**
 * "Tìm văn phòng gần tôi" (owner 12/10/2026). Every failure sentence says what to do next and that the list is still
 * usable as it is — the offices never disappear because a location could not be had.
 *
 * `purpose` is the ONE sentence of this app's voice that names a server: it is the disclosure shown before Zalo's own
 * dialog (Mini App policy 3.3.4), and where the location goes IS the fact the person is agreeing to. Its wording is
 * the owner's (12/10/2026); `tinh-nang.test.tsx` exempts this key alone from the "no infrastructure talk" rule.
 */
export const NEARBY_OFFICES = {
  find: "Tìm văn phòng gần tôi",
  purpose:
    "Ứng dụng lấy vị trí hiện tại của bạn MỘT LẦN, gửi tới máy chủ của Tập đoàn ViHAT Group để đổi ra toạ độ, rồi xếp văn phòng gần bạn nhất lên đầu. Máy chủ không lưu vị trí của bạn.",
  allow: "Cho phép",
  deny: "Không",
  locating: "Đang tìm văn phòng gần bạn…",
  sorted: "Văn phòng gần bạn nhất ở trên cùng. Khoảng cách tính theo đường chim bay.",
  /** "cách khoảng 3,4 km" — the number comes from `formatDistanceKm`. */
  distance: (km: string) => `Cách khoảng ${km} km`,
  denied: "Bạn chưa cho phép lấy vị trí, nên danh sách giữ thứ tự sẵn có. Bạn vẫn bấm “Chỉ đường” ở văn phòng muốn tới được ngay.",
  zalo_refused:
    "Bạn chưa cho Zalo chia sẻ vị trí, nên danh sách giữ thứ tự sẵn có. Muốn xếp theo khoảng cách, bấm lại “Tìm văn phòng gần tôi” và đồng ý chia sẻ vị trí.",
  outside_zalo: "Tính năng này chỉ khả dụng trên ứng dụng Zalo. Bạn vẫn bấm “Chỉ đường” ở văn phòng muốn tới được ngay.",
  too_many: "Bạn đã thử nhiều lần trong ít phút. Vui lòng chờ vài phút rồi bấm lại; danh sách vẫn giữ thứ tự sẵn có.",
  retry: "Chưa lấy được vị trí của bạn. Vui lòng bấm lại sau ít phút, hoặc bấm “Chỉ đường” ở văn phòng muốn tới.",
  paused: "Tính năng xếp theo khoảng cách đang tạm ngưng. Bạn vẫn bấm “Chỉ đường” ở văn phòng muốn tới được ngay.",
} as const;

/** ---------- Đăng nhập bằng số Zalo ---------- */

export const DANG_NHAP = {
  nhac_lien_he: "Hoặc liên hệ trực tiếp",
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

/**
 * MỤC VỀ CÁC QUYỀN TRONG CHÍNH SÁCH QUYỀN RIÊNG TƯ — dựng từ chính `NOI_DUNG_TINH_NANG`.
 *
 * VÌ SAO DỰNG TỪ ĐÓ CHỨ KHÔNG VIẾT LẠI: `vi_sao` là câu nói mục đích của từng quyền, và nó đã
 * có người chủ — chính mảng trên. Chép sang chính sách là tạo bản thứ hai của một sự thật, và
 * khi hai bản lệch nhau thì bản sai là bản nằm trong văn bản pháp lý (luật 9).
 *
 * 11/10/2026 (owner): this is the ViHAT Group app's policy only — no commune, petition or ViGov sentence
 * (`content/commune-privacy.ts` holds the commune interface's), and no sentence about the features removed that
 * day (paper-card photo, network type). Every paragraph that is not joined at run time is a literal, so
 * `bundle-for-zalo.test.ts` can find it in the bundle.
 * 12/10/2026 (owner): the location is asked again, for "Tìm văn phòng gần tôi" — the first paragraph names it.
 */
export const DOAN_CHINH_SACH_TINH_NANG: readonly string[] = [
  "Ứng dụng xin các quyền của nền tảng Zalo cho các tính năng dưới đây; cho hai nút một chạm trong thẻ 'Kết nối với ViHAT' ở màn Liên hệ: 'Chat với chuyên viên' xin số điện thoại và tên Zalo của bạn để chuyên viên biết ai đang nhắn; 'Đăng ký nhận' ưu đãi SMS xin số điện thoại để gửi tin nhắn ưu đãi tới đúng số ấy; và cho nút 'Tìm văn phòng gần tôi' ở màn Liên hệ, xin vị trí hiện tại của bạn một lần để xếp văn phòng gần bạn nhất lên đầu. Mỗi quyền chỉ được hỏi khi bạn tự bấm nút, và bạn có quyền từ chối mà vẫn dùng được ứng dụng.",
  ...NOI_DUNG_TINH_NANG.map((mot) => `${mot.nhan_ngan} — ${mot.vi_sao}`),
  "Với số điện thoại, Zalo không trả số thật về máy: ứng dụng chỉ nhận một mã dùng được một lần và hết hạn sau 2 phút. Số điện thoại của bạn không nằm trong mã đó.",
  "Với quét mã QR, nội dung mã hiện lên màn hình và mất đi khi bạn quét mã khác hoặc rời màn hình. Nếu mã là một tấm danh thiếp, nội dung ấy là dữ liệu cá nhân của người đã đưa nó cho bạn, và ứng dụng cũng không lưu lại.",
  // 12/10/2026 (owner): the consultation photos — the camera and the picker are used again, on one screen.
  "Với máy ảnh và thư viện ảnh, ứng dụng chỉ dùng chúng khi chính bạn bấm: máy ảnh để quét mã QR khi bạn bấm 'Quét mã QR', và để chụp ảnh gửi kèm một yêu cầu tư vấn khi bạn bấm 'Chụp ảnh' ở màn 'Tư vấn và báo giá'; thư viện ảnh khi bạn bấm 'Chọn ảnh' ở cùng màn ấy. Trước khi Zalo hỏi quyền, ứng dụng hỏi bạn bằng một câu nói rõ ảnh dùng để làm gì. Ứng dụng chỉ nhận ảnh bạn chụp hoặc chọn, không xem các ảnh khác trong máy, và không lưu ảnh trên máy bạn.",
  "Với việc giữ màn hình sáng, ứng dụng chỉ bật chế độ ấy khi bạn tự bấm, và tự tắt lại khi bạn rời màn hình danh thiếp. Chế độ này không đọc gì và không gửi gì; nó chỉ ngăn màn hình tối đi trong lúc người khác đang quét mã.",
  // LIỆT KÊ ĐÚNG NHỮNG TRƯỜNG TẤM THIẾP THẬT SỰ CHỨA. Câu này và câu `ghi-tep` trong chính sách
  // phải khớp nhau VÀ khớp với `vcard.ts` — `chinh-sach.test.ts`, khối "khai của tệp danh thiếp khớp
  // với tấm thiếp thật", hỏi cả hai câu cùng một câu hỏi.
  "Với việc tải tệp, ứng dụng GHI MỘT TỆP XUỐNG MÁY BẠN, và đây là tệp duy nhất ứng dụng ghi xuống máy. Tệp ấy là danh thiếp của chúng tôi — tên, hotline, email và trang web của công ty — không phải dữ liệu của bạn. Ứng dụng không đọc, không sửa và không xoá bất kỳ tệp nào khác.",
  "Ngoài tệp danh thiếp của chúng tôi, ứng dụng không lưu gì xuống máy bạn; những gì được gửi đi, và gửi tới đâu, được nói ở mục Đăng nhập, mục 'Yêu cầu tư vấn và đề nghị gọi lại', mục 'Chat với chuyên viên và nhận ưu đãi qua SMS' và mục 'Vị trí khi bạn tìm văn phòng gần nhất'.",
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
 * Nghị định 13 buộc nói ra.
 *
 * 11/10/2026: exactly the calls the ViHAT intro's "Quản lý quyền" shows that need a Console permission
 * (`chinh-sach.test.ts` checks the list against `introPermissionRows`). 12/10/2026: the camera and the picker are
 * the intro's too again ("Tư vấn và báo giá" photos), and so is `getLocation` ("Tìm văn phòng gần
 * tôi"), so they are listed here in the intro's words; `content/commune-privacy.ts` keeps the commune's own words.
 * Same day (owner, on a real device): "Chụp ảnh" is Zalo's camera again, i.e. `openMediaPicker` `zcamera_photo` —
 * so the picker's line names both uses and the phone-camera line (`chooseImage`) is gone.
 */
export const DOAN_CHINH_SACH_TUNG_QUYEN: readonly string[] = [
  "Số điện thoại (getPhoneNumber) — để bạn đăng nhập bằng một lần chạm; để gửi thông báo ZNS tới đúng số ấy; để chúng tôi gọi lại đúng số ấy nếu bạn tự đề nghị gọi lại ở màn Tư vấn và báo giá; để chuyên viên biết ai đang nhắn khi bạn bấm 'Chat với chuyên viên'; và để gửi tin nhắn SMS ưu đãi tới đúng số ấy nếu bạn bấm 'Đăng ký nhận', cho tới khi bạn bấm 'Huỷ nhận ưu đãi SMS'. Ứng dụng chỉ nhận một mã, không nhận số.",
  "Ghi nhớ quyền số điện thoại (authorize) — ngay trước khi lấy mã số điện thoại, ứng dụng xin Zalo ghi nhớ quyền bạn vừa đồng ý, để mục quyền của ứng dụng trong Zalo hiện đúng là đã bật. Ứng dụng chỉ xin đúng quyền số điện thoại; bạn tắt được nó bất cứ lúc nào trong phần cài đặt của Zalo.",
  "Trạng thái quyền số điện thoại (getSetting) — để xem bạn đã cho phép chia sẻ số điện thoại trong Zalo hay chưa, và không hỏi lại điều bạn đã đồng ý. Kết quả chỉ dùng ngay trên máy, không lưu và không gửi đi.",
  "Thông tin xác thực phiên Zalo (getAccessToken) — mã này cho biết bạn là người dùng Zalo nào đối với riêng ứng dụng này. Nó đi cùng mã số điện thoại ở bước đăng nhập, và không cho ứng dụng biết tên hay ảnh đại diện của bạn. Phiên mở ra từ hai mã ấy là thứ cho bạn gửi yêu cầu tư vấn — kể cả yêu cầu trò chuyện với chuyên viên và việc đăng ký hay huỷ nhận ưu đãi SMS — và chỉ xem được yêu cầu của chính mình. Khi bạn bấm 'Tìm văn phòng gần tôi', mã này đi cùng mã vị trí để máy chủ của Tập đoàn ViHAT Group đổi mã vị trí thành toạ độ.",
  "Vị trí (getLocation) — để xếp văn phòng của chúng tôi gần bạn nhất lên đầu khi bạn bấm 'Tìm văn phòng gần tôi' ở màn Liên hệ rồi bấm 'Cho phép' ở câu hỏi của ứng dụng. Ứng dụng chỉ nhận một mã; máy chủ của Tập đoàn ViHAT Group đổi mã thành toạ độ, không lưu mã lẫn toạ độ, và khoảng cách tới từng văn phòng được tính ngay trên máy bạn. Bấm 'Không' thì Zalo không được hỏi.",
  "Tên Zalo (getUserInfo) — khi bạn bấm 'Chat với chuyên viên' ở màn Liên hệ: để gửi kèm yêu cầu trò chuyện tới máy chủ của Tập đoàn ViHAT Group, cho chuyên viên biết ai đang nhắn. Ứng dụng chỉ dùng tên hiển thị Zalo của bạn, sau khi bạn đồng ý; từ chối thì cửa sổ trò chuyện vẫn mở, chỉ là không có tên.",
  "Quét mã QR (scanQRCode) — để đọc danh thiếp số của đối tác.",
  "Rung (vibrate) — để báo bằng một nhịp rung khi danh thiếp của chúng tôi đã được tải xuống máy bạn.",
  "Giữ màn hình sáng (keepScreen) — để màn hình không tối đi trong lúc người khác quét mã danh thiếp của chúng tôi.",
  "Tải tệp (downloadFile) — để ghi tệp danh thiếp của chúng tôi xuống máy bạn.",
  "Quyền máy ảnh (requestCameraPermission) — để hỏi bạn cho phép dùng máy ảnh khi bạn bấm 'Chụp ảnh' ở màn 'Tư vấn và báo giá', sau khi bạn đã bấm 'Cho phép' ở câu hỏi của ứng dụng. Bấm 'Không' thì Zalo không được hỏi.",
  "Máy ảnh của Zalo và chọn ảnh (openMediaPicker) — để bạn chụp một ảnh bằng máy ảnh của Zalo khi bạn bấm 'Chụp ảnh', hoặc chọn ảnh có sẵn trong máy khi bạn bấm 'Chọn ảnh', gửi kèm yêu cầu tư vấn ở màn 'Tư vấn và báo giá'; tối đa 3 ảnh cho một yêu cầu. Ứng dụng chỉ nhận những ảnh bạn vừa chụp hoặc đã chọn, không xem các ảnh khác trong máy.",
];
