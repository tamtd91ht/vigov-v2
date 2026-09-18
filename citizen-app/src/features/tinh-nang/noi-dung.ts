/**
 * MỌI CHỮ NGƯỜI DÙNG ĐỌC TRÊN BA MÀN QUYỀN — một tệp, giống `content/company-profile.ts`.
 *
 * VÌ SAO TÁCH RA KHỎI COMPONENT:
 *
 *   Người duyệt của Zalo đọc đúng những câu này, và đây là thứ quyết định app có được cấp ba
 *   quyền hay không: chính sách Mini App (điều 3.3.4, trích trong `zmp-sdk/index.d.ts` ngay trên
 *   `getPhoneNumber`) từ chối xét duyệt những app xin quyền mà không nói rõ mục đích. Một câu
 *   giải thích nằm rải trong JSX là một câu không ai đọc lại trước khi nộp.
 *
 *   Tách ra còn để `bundle-for-zalo.test.ts` **dùng lại đúng danh sách này** khi khẳng định bản
 *   `goc` không chứa chữ nào của ba màn ấy. Một danh sách chép tay ở phía test sẽ lệch, và khi
 *   nó lệch thì phép kiểm xanh vì không tìm thấy gì — chứ không vì bản `goc` sạch.
 *
 * ⚠ KHÔNG CÂU NÀO Ở ĐÂY ĐƯỢC HỨA MỘT VIỆC BẢN DỰNG NÀY KHÔNG LÀM. Ba màn chỉ lấy dữ liệu rồi
 * hiện lên màn hình: không gửi đi đâu, không lưu lại. Viết "chúng tôi sẽ lưu để..." là mô tả sai
 * một ứng dụng đang xin quyền — và mô tả sai cho người duyệt là thứ không sửa lại được sau đó.
 */

import type { MucChinhSach } from "../../content/chinh-sach-rieng-tu";

/** Ba quyền đang xin. Thứ tự này là thứ tự hiện trên màn. */
export type MaQuyen = "so-dien-thoai" | "vi-tri" | "quet-qr";

export type NoiDungQuyen = {
  ma: MaQuyen;
  /** Nhãn trên hàng nút chọn màn. Ngắn để không tràn trên máy 320px. */
  nhan_chon: string;
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
 * Câu nói ra ĐIỀU QUAN TRỌNG NHẤT của hai màn token, và nó không phải lời trấn an:
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
  "so-dien-thoai":
    "Số điện thoại của bạn không nằm trong mã này và không được gửi về máy. Chỉ máy chủ của cơ quan mới đổi được mã thành số điện thoại, và mã chỉ dùng được một lần trong 2 phút.",
  "vi-tri":
    "Toạ độ của bạn không nằm trong mã này và không được gửi về máy. Chỉ máy chủ của cơ quan mới đổi được mã thành vị trí, và mã chỉ dùng được một lần trong 2 phút.",
} as const;

/** Lời hứa của chính bản dựng này, nói ra trên cả ba màn. Xem `zalo-api.ts` về việc giữ nó. */
export const CHI_HIEN_LEN_MAN_HINH =
  "Bản dựng này chỉ hiện kết quả lên màn hình. Không có gì được lưu lại trên máy và không có gì được gửi đi.";

/** Câu dẫn của cả khu vực — thứ người duyệt đọc trước khi bấm bất cứ nút nào. */
export const DAN_NHAP_QUYEN =
  "Ba màn dưới đây cho thấy ứng dụng dùng ba quyền vào việc gì. Mỗi màn có một nút; bạn bấm thì Zalo mới hỏi, và bạn có quyền từ chối.";

export const NOI_DUNG_QUYEN: readonly NoiDungQuyen[] = [
  {
    ma: "so-dien-thoai",
    nhan_chon: "Số điện thoại",
    tieu_de: "Xác thực bạn khi gửi yêu cầu hỗ trợ",
    vi_sao:
      "Khi bạn gửi một yêu cầu hỗ trợ, cơ quan tiếp nhận phải biết chắc yêu cầu đó là của bạn và liên hệ lại được. Ứng dụng xin số điện thoại Zalo của bạn để làm việc đó, và chỉ xin đúng lúc bạn gửi yêu cầu.",
    nut: "Chia sẻ số điện thoại Zalo",
    dang_cho: "Đang chờ bạn trả lời trên Zalo…",
    tu_choi:
      "Bạn đã từ chối chia sẻ số điện thoại. Không sao cả — bạn vẫn dùng được ứng dụng, và có thể bấm lại nút bên trên bất cứ lúc nào.",
    ngoai_zalo:
      "Phần này chỉ chạy được bên trong ứng dụng Zalo. Bạn hãy mở lại trang này trong Zalo trên điện thoại rồi bấm lại.",
    khong_lay_duoc:
      "Chưa nhận được trả lời từ Zalo. Bạn hãy bấm lại nút bên trên; nếu vẫn chưa được, hãy đóng rồi mở lại ứng dụng.",
  },
  {
    ma: "vi-tri",
    nhan_chon: "Vị trí",
    tieu_de: "Gợi ý điểm hỗ trợ gần bạn nhất",
    vi_sao:
      "Ứng dụng dùng vị trí để gợi ý điểm hỗ trợ gần bạn nhất, thay vì bắt bạn dò trong một danh sách dài. Vị trí chỉ để GỢI Ý — bạn luôn là người chọn, và chọn nơi khác lúc nào cũng được.",
    nut: "Chia sẻ vị trí của tôi",
    dang_cho: "Đang chờ bạn trả lời trên Zalo…",
    tu_choi:
      "Bạn đã từ chối chia sẻ vị trí. Không sao cả — bạn vẫn tự chọn được điểm hỗ trợ trong danh sách, và có thể bấm lại nút bên trên bất cứ lúc nào.",
    ngoai_zalo:
      "Phần này chỉ chạy được bên trong ứng dụng Zalo. Bạn hãy mở lại trang này trong Zalo trên điện thoại rồi bấm lại.",
    khong_lay_duoc:
      "Chưa nhận được trả lời từ Zalo. Bạn hãy bấm lại nút bên trên; nếu vẫn chưa được, hãy đóng rồi mở lại ứng dụng.",
  },
  {
    ma: "quet-qr",
    nhan_chon: "Quét mã QR",
    tieu_de: "Quét mã tra cứu thay cho gõ tay",
    vi_sao:
      "Mã tra cứu hồ sơ in trên giấy hẹn dài và dễ gõ nhầm một ký tự. Ứng dụng xin dùng máy ảnh để quét mã ấy, thay cho việc bạn gõ lại bằng tay.",
    nut: "Mở máy ảnh để quét mã",
    dang_cho: "Đang mở máy ảnh…",
    tu_choi:
      "Bạn đã dừng quét mã. Không sao cả — bạn vẫn gõ mã bằng tay được, và có thể bấm lại nút bên trên bất cứ lúc nào.",
    ngoai_zalo:
      "Phần này chỉ chạy được bên trong ứng dụng Zalo. Bạn hãy mở lại trang này trong Zalo trên điện thoại rồi bấm lại.",
    khong_lay_duoc:
      "Chưa quét được mã. Bạn hãy bấm lại nút bên trên và giữ máy cách mã khoảng một gang tay.",
  },
];

export function noiDung(ma: MaQuyen): NoiDungQuyen {
  const tim = NOI_DUNG_QUYEN.find((mot) => mot.ma === ma);
  // Fail closed: một mã không có nội dung là lỗi lập trình, và một màn trắng là thứ người duyệt
  // của Zalo thấy trước tiên.
  if (!tim) throw new Error(`Khong co noi dung cho quyen: ${ma}`);
  return tim;
}

/**
 * Nền tảng trả về mã RỖNG ở môi trường phát triển — ghi thẳng trong `zmp-sdk/index.d.ts`:
 * *"Ở môi trường phát triển, API này sẽ luôn thành công và trả về token rỗng"*. Nói ra đúng như
 * vậy, thay vì vẽ ra một ô kết quả trống mà người đọc tưởng là hỏng.
 */
export const MA_RONG =
  "Zalo trả về mã rỗng. Điều này xảy ra khi ứng dụng chưa chạy trong Zalo trên điện thoại thật.";

/** Mã QR quét được nhưng không có nội dung. Hiếm, nhưng một ô trống thì không nói được gì. */
export const QR_RONG = "Mã vừa quét không có nội dung nào.";

/**
 * MỤC VỀ BA QUYỀN TRONG CHÍNH SÁCH QUYỀN RIÊNG TƯ.
 *
 * VÌ SAO NÓ Ở ĐÂY CHỨ KHÔNG Ở `content/chinh-sach-rieng-tu.ts`: bản `goc` không xin quyền nào.
 * Một chính sách trong bản `goc` mà nhắc tới số điện thoại hay vị trí là mô tả SAI đúng bản dựng
 * người duyệt đang cầm — và họ đọc nó cạnh một app không hề xin quyền. Tệp này nằm sau cửa
 * `bien-the/quyen`, nên mục này biến mất cùng ba màn.
 *
 * VÌ SAO DỰNG TỪ `NOI_DUNG_QUYEN` CHỨ KHÔNG VIẾT LẠI: `vi_sao` là câu nói mục đích của từng
 * quyền, và nó đã có người chủ — chính là mảng trên. Chép sang đây là tạo bản thứ hai của một
 * sự thật, và khi hai bản lệch nhau thì bản sai là bản nằm trong văn bản pháp lý (rule 9).
 *
 * ĐOẠN CUỐI NÓI CẢ HAI VẾ, CÓ CHỦ ĐÍCH. Người duyệt cần biết mục đích để cấp quyền; người dùng
 * cần biết bản họ đang cầm thật sự làm gì. Viết mỗi mục đích mà giấu việc bản này chưa truyền gì
 * là mô tả sai; viết mỗi "chưa làm gì" mà giấu mục đích là không đủ để xét duyệt. Nói cả hai.
 */
export const MUC_CHINH_SACH_QUYEN: MucChinhSach = {
  ma: "ba-quyen",
  tieu_de: "Ba quyền ứng dụng xin, và vì sao",
  doan: [
    "Bản dựng này xin ba quyền của nền tảng Zalo. Mỗi quyền chỉ được hỏi khi bạn tự bấm nút trong ứng dụng, và bạn có quyền từ chối mà vẫn dùng được ứng dụng.",
    ...NOI_DUNG_QUYEN.map((q) => `${q.nhan_chon} — ${q.vi_sao}`),
    "Với số điện thoại và vị trí, Zalo không trả giá trị thật về máy: ứng dụng chỉ nhận một mã dùng được một lần và hết hạn sau 2 phút. Số điện thoại và toạ độ của bạn không nằm trong mã đó.",
    "Với quét mã QR, nội dung mã hiện lên màn hình và mất đi khi bạn rời màn hình.",
    "Các mục đích nêu trên là mục đích ứng dụng sẽ dùng ba quyền này khi có đầy đủ chức năng. Bản hiện tại chỉ hiện kết quả lên màn hình để bạn thấy tính năng hoạt động: nó chưa lưu và chưa gửi bất kỳ dữ liệu nào đi đâu.",
  ],
};
