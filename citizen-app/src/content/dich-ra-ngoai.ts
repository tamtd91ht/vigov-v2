/**
 * MỌI CHỖ ỨNG DỤNG ĐƯA NGƯỜI DÙNG RA MỘT TRANG BÊN NGOÀI — MỘT DANH SÁCH CÓ TÊN.
 *
 * ⚠ VÌ SAO DANH SÁCH NÀY TỒN TẠI, VÀ VÌ SAO NÓ KHÔNG PHẢI MỘT DANH SÁCH TRANG TRÍ:
 *
 *   Chính sách quyền riêng tư khai *"Có <N> chỗ ứng dụng mở một trang bên ngoài"* rồi liệt kê đủ
 *   N chỗ. Con số ấy đã phải sửa BA lần trong hai ngày (20/09: ba · 21/09 sáng: hai · 21/09 chiều:
 *   ba), mỗi lần vì một nút được thêm hoặc bớt ở một tệp khác — và mỗi lần đều do người đọc lại
 *   văn bản bằng mắt mà phát hiện, không do một phép kiểm nào.
 *
 *   Một con số đếm bằng mắt trong một VĂN BẢN PHÁP LÝ sắp nộp là con số sẽ có lần không ai đếm.
 *   Khai thiếu một nơi dữ liệu người dùng có thể đi tới là đúng thứ Nghị định 13/2023/NĐ-CP nhắm
 *   tới, và là thứ không sửa lại được sau khi công bố.
 *
 * NÊN CƠ CHẾ Ở ĐÂY LÀ: **câu trong chính sách được DỰNG RA từ danh sách này** (`cauKhaiDichRaNgoai`),
 * và **mã nguồn không có đường nào mở một trang ngoài mà không gọi tên một `ma` trong danh sách**
 * (`features/tinh-nang/mo-ra-ngoai.ts` là cửa duy nhất; `content/dich-ra-ngoai.test.ts` cấm mọi
 * hình dạng đi vòng). Hai vế ấy khoá nhau: thêm một lối ra mà quên khai là một ca ĐỎ, không phải
 * một câu sai trong hồ sơ.
 *
 * ⚠ ĐẾM THEO **ĐÍCH ĐẾN**, KHÔNG THEO **SỐ NÚT** — quy ước giữ nguyên từ 20/09/2026.
 *
 *   `trang-chu` có HAI nút dẫn tới (màn Liên hệ và trang chi tiết giải pháp) nhưng là MỘT dòng ở
 *   đây: người đọc chính sách cần biết dữ liệu của họ có thể tới những NƠI nào, không cần biết có
 *   bao nhiêu nút dẫn tới cùng một nơi. Đếm theo nút thì câu ấy phải sửa mỗi lần thêm một nút, và
 *   một câu phải sửa thường xuyên là câu sẽ có lần không ai sửa.
 *
 * ⚠ `tin-tuc` ĐỨNG RIÊNG DÙ CÙNG TÊN MIỀN VỚI `trang-chu`, và đó là một lựa chọn có chủ đích:
 *   gộp chúng lại đòi người đọc mã tự so hai tên miền và tự kết luận "cùng một nơi" — một phép so
 *   không ai kiểm và sẽ sai ngày trang tin dọn sang một tên miền khác. Khai thừa một dòng thì
 *   người đọc chính sách biết nhiều hơn một chút; khai thiếu một dòng thì văn bản sai.
 */

/** Mã đích đến. Là một union để `moRaNgoai` không nhận nổi một đích chưa khai. */
export type MaDichRaNgoai = "ban-do" | "ma-qr" | "trang-chu" | "tin-tuc" | "chat-oa";

export type DichRaNgoai = {
  ma: MaDichRaNgoai;
  /** Cụm từ in NGUYÊN VĂN vào câu khai của chính sách. Viết cho người dùng đọc. */
  trong_chinh_sach: string;
};

/**
 * NĂM ĐÍCH ĐẾN. Thứ tự này là thứ tự đọc ra trong câu khai của chính sách.
 *
 * Thêm một dòng ở đây là một thay đổi HÀNH VI XỬ LÝ DỮ LIỆU: bề mặt "dữ liệu của bạn có thể tới
 * những nơi nào" rộng ra thật sự. Xem bảng lên số phiên bản trong `chinh-sach-rieng-tu.ts`.
 */
export const DICH_MO_RA_NGOAI: readonly DichRaNgoai[] = [
  {
    ma: "ban-do",
    trong_chinh_sach: "trang bản đồ để chỉ đường tới một văn phòng",
  },
  {
    ma: "ma-qr",
    trong_chinh_sach: "trang web ghi trên mã QR bạn vừa quét",
  },
  {
    ma: "trang-chu",
    trong_chinh_sach: "trang web chính thức của chúng tôi",
  },
  {
    ma: "tin-tuc",
    trong_chinh_sach: "bài viết trên trang tin của chúng tôi",
  },
  {
    ma: "chat-oa",
    trong_chinh_sach: "cửa sổ trò chuyện với Official Account của chúng tôi trên Zalo",
  },
];

/**
 * ĐỊA CHỈ CỬA SỔ TRÒ CHUYỆN VỚI OFFICIAL ACCOUNT.
 *
 * ⚠ CON SỐ NÀY LẤY TỪ BẢN MẪU GIAO DIỆN CỦA PM, **CHƯA AI ĐỐI CHIẾU VỚI DEVELOPER CONSOLE**.
 *
 *   Ghi ra ở đây thay vì rải trong một component chính vì thế: nó là một giá trị CHƯA XÁC MINH
 *   nằm trong một bản sắp nộp, và một giá trị chưa xác minh phải có đúng một chỗ để sửa khi có
 *   người xác minh xong.
 *
 * ⚠ VÀ NÓ PHẢI LÀ OA **NỀN TẢNG** (`Vihat`), KHÔNG PHẢI OA CỦA MỘT XÃ NÀO.
 *
 *   ADR 0018 §Hệ quả, điểm 3: *"Mọi bề mặt OA hiển thị bên trong Mini App trỏ về OA nền tảng,
 *   không trỏ về OA của xã"* — và ADR 0031 chuyển OA ấy từ `VihatSoftware` sang `Vihat` cùng lúc
 *   với việc đổi pháp nhân đứng tên app.
 *
 *   Nếu ID dưới đây KHÔNG phải OA ấy thì hỏng theo đúng kiểu ADR 0018 mô tả và nặng hơn một lỗi
 *   giao diện: người dùng bấm "quan tâm"/"theo dõi" trong cửa sổ mở ra và tưởng mình đã theo dõi
 *   đúng nơi, trong khi họ vừa theo dõi một tài khoản khác. Ở giai đoạn 2, "nơi đúng" với họ là
 *   xã của họ — và app thì chỉ có một OA cho mọi xã.
 *
 *   → VIỆC CÒN PHẢI LÀM TRƯỚC KHI NỘP: mở Developer Console, đối chiếu ID này với OA xác thực của
 *     Mini App. Xem `README.md` §Còn thiếu.
 */
export const OA_NEN_TANG_ID = "1573178902494572730";

/** Nguồn của `OA_NEN_TANG_ID`, đi kèm giá trị để không ai tách nó ra khỏi lời cảnh báo. */
export const OA_NEN_TANG_NGUON =
  "Bản mẫu giao diện của PM, 21/09/2026 — CHƯA đối chiếu với Zalo Developer Console";

export const DUONG_DAN_CHAT_OA = `https://zalo.me/${OA_NEN_TANG_ID}`;

/**
 * Số đếm đọc thành chữ, đúng phạm vi cần dùng.
 *
 * Trả về chữ số khi vượt bảng thay vì ném lỗi: một câu chính sách đọc hơi cứng ("Có 11 chỗ…")
 * vẫn ĐÚNG, còn một ngoại lệ ném ra lúc dựng văn bản là hồ sơ không sinh được. Hỏng về phía nói
 * đúng nhưng xấu, không về phía im lặng.
 */
const SO_CHU = ["không", "một", "hai", "ba", "bốn", "năm", "sáu", "bảy", "tám", "chín", "mười"];

export function soChu(n: number): string {
  return SO_CHU[n] ?? String(n);
}

/**
 * CÂU KHAI TRONG CHÍNH SÁCH — dựng từ danh sách, không gõ tay.
 *
 * Con số và danh sách vì thế KHÔNG THỂ lệch nhau: chúng là cùng một mảng đọc ra hai lần. Đó là
 * toàn bộ lý do câu này không nằm thẳng trong `chinh-sach-rieng-tu.ts` như mọi câu khác.
 */
export function cauKhaiDichRaNgoai(danh_sach: readonly DichRaNgoai[] = DICH_MO_RA_NGOAI): string {
  const so = soChu(danh_sach.length);
  const ke = danh_sach.map((d) => d.trong_chinh_sach);
  const cuoi = ke[ke.length - 1] ?? "";
  const ke_ra = ke.length <= 1 ? cuoi : `${ke.slice(0, -1).join(", ")}, và ${cuoi}`;

  return (
    `Có ${so} chỗ ứng dụng mở một trang bên ngoài, và cả ${so} đều chỉ mở khi chính bạn bấm: ` +
    `${ke_ra}. Ứng dụng không gửi kèm thông tin nào của bạn khi mở chúng; từ lúc trang mở ra, ` +
    "việc bạn dùng trang ấy chịu sự điều chỉnh của chính sách bên sở hữu nó."
  );
}
