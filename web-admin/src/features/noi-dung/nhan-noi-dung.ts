/**
 * Chữ và phép quyết định của màn "Nội dung Mini App" — `docs/ui-ux/11-noi-dung-mini-app.md`.
 *
 * TÁCH KHỎI COMPONENT ĐỂ KIỂM ĐƯỢC TỪNG PHÉP MỘT. Hàm thuần: không gọi mạng, không dựng DOM,
 * không đọc đồng hồ. Mọi chuỗi tiếng Việt lấy NGUYÊN VĂN từ đặc tả — một chữ khác đi trên màn của
 * cơ quan nhà nước là một chữ có người phải trả lời.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * ⚠ ĐIỀU QUAN TRỌNG NHẤT CỦA MÀN NÀY: HTML KHÔNG ĐƯỢC LÀM SẠCH Ở BẤT KỲ ĐÂU.
 *
 * §8 khai `noi_dung` là HTML và §7 muốn một ô soạn thảo rich text. Máy chủ lưu NGUYÊN VĂN những
 * gì được gửi lên và nói thẳng giới hạn ấy trong mã của chính nó
 * (`service-comms/internal/domain/noi_dung_mini_app.go`, `ChuanHoaVanBanDai`): kho không có bộ
 * làm sạch HTML nào, và tự viết một cái là cách các bộ làm sạch bị viết sai.
 *
 * Hệ quả cho MÀN NÀY: một cán bộ có `content.update` đặt được markup tuỳ ý — kể cả `<script>` —
 * vào thứ mọi cư dân của xã mở ra xem, và màn Phân quyền của xã có thể đã cấp khoá ấy cho nhiều
 * người. Nên màn này KHÔNG dựng HTML ở bất kỳ đâu, kể cả để "xem trước": toàn văn hiện dưới dạng
 * VĂN BẢN THUẦN, kèm một câu nói rõ đây là mã nguồn chứ không phải bản dựng.
 *
 * Lệnh cấm ấy có phép kiểm riêng đọc thẳng mã nguồn — `ranh-gioi-html.test.ts`. Một lệnh cấm
 * không có phép kiểm là một lệnh cấm sẽ bị phá trong im lặng.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */

import type {
  comms_danhMucRa,
  comms_noiDungRa,
  comms_suaNoiDungVao,
  comms_themNoiDungVao,
} from "@/lib/api/schema.gen";

/* ── Trần độ dài, đúng bằng trần máy chủ ────────────────────────────────────────────────────
 *
 * Chép từ `service-comms/internal/domain/noi_dung_mini_app.go` và CHỈ để ô nhập dừng lại đúng chỗ
 * máy chủ sẽ dừng. Máy chủ vẫn là nơi từ chối thật; `maxLength` ở ô nhập chỉ để cán bộ thấy giới
 * hạn thay vì thấy một câu 400 sau khi đã gõ xong cả bài.
 *
 * ⚠ MÁY CHỦ TỪ CHỐI CHỨ KHÔNG CẮT BỚT — "một bài viết bị cắt âm thầm là một bài viết đã đổi nghĩa
 * trên đường vào CSDL, trên một kênh cư dân đọc". Nên hai bên phải dừng ở cùng một con số.
 *
 * VÌ SAO KHÔNG DÙNG LẠI HẰNG CỦA MÀN THÔNG BÁO dù hai con số trùng nhau: bên ấy chặn một THÔNG
 * BÁO NỘI BỘ một cán bộ gõ vào textarea; bên này chặn một BÀI ĐĂNG công khai, phần lớn sẽ về từ
 * một cổng thông tin có giới hạn riêng. Ngày một trong hai phải đổi, nó phải đổi một mình.
 */
export const TIEU_DE_TOI_DA = 300;
export const TOM_TAT_TOI_DA = 2000;
export const THAN_BAI_TOI_DA = 200000;
export const URL_TOI_DA = 2000;
export const TEN_DANH_MUC_TOI_DA = 200;
export const SLUG_DANH_MUC_TOI_DA = 64;
export const THU_TU_DANH_MUC_TOI_DA = 9999;

/* ── Chữ trên màn ──────────────────────────────────────────────────────────────────────────── */

/** §3 của chương — tiêu đề trang, nguyên văn. */
export const TIEU_DE_MAN = "Quản trị nội dung Mini App";

/** §1 — nguyên văn câu mô tả trong giao diện. */
export const MO_TA_MAN =
  "Tin tức, sự kiện, thông báo, bản tin truyền thanh và video hiển thị cho bà con trên Zalo " +
  "Mini App.";

export const NHAN_NUT_THEM = "+ Thêm nội dung";
export const NHAN_NUT_DANH_MUC = "⊞ Danh mục tin";
export const NHAN_NUT_HUY = "Huỷ";
export const NHAN_NUT_LUU = "Lưu";
/** §6 — cột hành động chỉ có ký hiệu này. KHÔNG có nút xoá: không tuyến nào xoá được. */
export const NHAN_NUT_SUA = "✎";

/** §7 — nguyên văn tiêu đề và mô tả của modal thêm. */
export const TIEU_DE_FORM_THEM = "Thêm nội dung cho Mini App";
export const MO_TA_FORM_THEM =
  "Chưa bật “Đăng lên Mini App” thì bà con chưa thấy — soạn trước, đăng sau được.";

/** §7 — nhãn ô tích, nguyên văn. */
export const NHAN_O_DANG = "Đăng lên Mini App";

/** §7 — giá trị `— Chưa xếp danh mục —` của ô chọn danh mục, nguyên văn kể cả hai gạch. */
export const CHUA_XEP_DANH_MUC = "— Chưa xếp danh mục —";

/** §6 — nhãn mặc định của ô lọc danh mục, nguyên văn. */
export const MOI_DANH_MUC = "Tất cả danh mục";

/** §6 — placeholder ô tìm, nguyên văn kể cả dấu ba chấm. */
export const TIM_PLACEHOLDER = "Tìm theo tiêu đề…";

/** §4 — nguyên văn hai dòng của thẻ Danh bạ chính quyền, TRỪ con số không đọc về được. */
export const TIEU_DE_THE_DANH_BA = "Danh bạ chính quyền";
export const MO_TA_THE_DANH_BA =
  "Chọn thêm hoặc bớt cán bộ hiện cho bà con ở màn Danh bạ cán bộ.";

/**
 * Sổ rỗng. Đặc tả không có câu nào cho trạng thái này, nên câu dưới là câu viết mới, cùng giọng
 * với các màn đã có — và điều ấy được nói ra ở đây thay vì để người sau tưởng mình đọc lại một câu
 * của đặc tả.
 */
export const SO_RONG = "Chưa có nội dung nào trong lát cắt đang xem.";
export const DANH_MUC_RONG = "Xã chưa có danh mục tin nào.";

export const DANG_TAI_SO = "Đang tải danh sách nội dung…";
export const DANG_TAI_TOAN_VAN = "Đang tải toàn văn bài viết…";

/* ── Ba câu cảnh báo phải ĐỨNG CẠNH ô nhập, không nằm trong chú thích mã ────────────────── */

/**
 * Câu đứng ngay dưới ô `Nội dung` của §7.
 *
 * Nó nói HAI điều, và cả hai đều là sự thật của máy chủ hôm nay: ô này nhận MÃ NGUỒN HTML, và
 * không có bộ làm sạch nào phía sau. Cán bộ cần biết cả hai trước khi dán một đoạn từ nơi khác.
 */
export const CANH_BAO_HTML_THO =
  "Ô này nhận MÃ NGUỒN HTML (§8), không phải văn bản có định dạng — ô soạn thảo rich text chưa " +
  "dựng được. Hệ thống KHÔNG làm sạch HTML: chỉ dán mã từ nguồn bạn tin được, vì mọi thẻ gõ vào " +
  "đây sẽ tới điện thoại của bà con.";

/** Câu đứng trên phần toàn văn ở khối chi tiết — nói rõ đây là mã nguồn, không phải bản dựng. */
export const CANH_BAO_XEM_MA_NGUON =
  "Dưới đây là MÃ NGUỒN HTML của bài, hiện dưới dạng văn bản thuần. Màn quản trị cố ý KHÔNG dựng " +
  "HTML — xem phần chưa dựng được ở đầu màn.";

/** Câu đứng dưới ô nhập liên kết ảnh — nói trước điều máy chủ sẽ từ chối. */
export const CANH_BAO_LIEN_KET_ANH =
  "Chỉ nhận địa chỉ bắt đầu bằng http:// hoặc https://, tối đa " +
  `${URL_TOI_DA} ký tự. Tải ảnh từ máy chưa dựng được — xem phần chưa dựng được ở đầu màn.`;

/** Câu đứng cạnh cột `Lượt xem` — con số hôm nay luôn là 0, và im lặng về điều đó là nói dối. */
export const GHI_CHU_LUOT_XEM =
  "Lượt xem hôm nay LUÔN là 0: thứ duy nhất được phép tăng nó là một cư dân mở bài, mà tuyến " +
  "công khai cho Mini App chưa dựng.";

/** Câu đứng cạnh cột hành động — vì sao không có nút xoá. */
export const GHI_CHU_KHONG_CO_XOA =
  "Không có nút xoá, có chủ ý: gỡ một bài khỏi Mini App là tắt ô “Đăng lên Mini App” ở màn sửa. " +
  "Bài vẫn còn trong sổ của xã.";

/* ── Sáu loại nội dung §5 ──────────────────────────────────────────────────────────────────── */

/**
 * Sáu mã của §5, đúng thứ tự tab đặc tả vẽ. DANH SÁCH ĐÓNG — ràng buộc CHECK của máy chủ không
 * nhận mã thứ bảy, và một mã lạ gửi lên bị trả 400 chứ không bị bỏ qua.
 */
export const MOI_LOAI = [
  "tin-tuc",
  "su-kien",
  "thong-bao",
  "truyen-thanh",
  "video",
  "banner",
] as const;

export type LoaiNoiDung = (typeof MOI_LOAI)[number];

/** Mã mặc định của ô chọn §7 — `Tin tức`. */
export const LOAI_MAC_DINH: LoaiNoiDung = "tin-tuc";

const NHAN_LOAI: Readonly<Record<string, string>> = {
  "tin-tuc": "Tin tức",
  "su-kien": "Sự kiện",
  "thong-bao": "Thông báo",
  "truyen-thanh": "Truyền thanh",
  video: "Video",
  banner: "Banner",
};

/**
 * Nhãn loại để hiện, kể cả khi máy chủ gửi một mã màn hình chưa biết.
 *
 * MÃ LẠ HIỆN NGUYÊN VĂN, không hiện dấu gạch và không im lặng bỏ qua: một loại mới ở máy chủ mà
 * màn hình vẽ thành `—` là một bài viết trông như chưa được phân loại.
 */
export function nhanLoai(ma: string): string {
  return NHAN_LOAI[ma] ?? ma;
}

/** Nhãn tab `Tất cả` — §5 vẽ sáu tab, nhưng sổ phải mở được ở trạng thái không lọc. */
export const MOI_LOAI_NHAN = "Tất cả";

/* ── Ba trạng thái §6 ──────────────────────────────────────────────────────────────────────── */

const NHAN_TRANG_THAI: Readonly<Record<string, string>> = {
  "dang-hien": "Đang hiện",
  "cho-duyet": "Chờ duyệt",
  an: "Ẩn",
};

export function nhanTrangThai(ma: string): string {
  return NHAN_TRANG_THAI[ma] ?? ma;
}

/**
 * Lớp CSS của chip trạng thái §6.
 *
 * `globals.css` có đúng `chip-hoat-dong` (xanh) và `chip-ngung` (xám), không có lớp cam nào. §6
 * muốn ba màu; hai lớp sẵn có diễn đạt được hai, và `cho-duyet` dùng lớp trung tính kèm CHỮ nói
 * rõ nó là gì. Lượt này không được thêm CSS — tên lớp cần thêm đã báo về.
 */
export function lopChipTrangThai(ma: string): string {
  return ma === "dang-hien" ? "chip chip-hoat-dong" : "chip chip-ngung";
}

/* ── Hai nguồn §8 ──────────────────────────────────────────────────────────────────────────── */

const NHAN_NGUON: Readonly<Record<string, string>> = {
  "thu-cong": "Soạn tay",
  "dong-bo-cong": "Đồng bộ từ Cổng",
};

/**
 * Nhãn nguồn của một bài.
 *
 * HÔM NAY MỌI HÀNG ĐỀU LÀ `thu-cong`, vì lượt đồng bộ chưa dựng. Nhãn thứ hai vẫn được viết ra vì
 * §6 là màn hình duy nhất nói được cho xã biết bài nào về từ Cổng của họ — và ngày lượt đồng bộ
 * hạ cánh, màn hình không được cần thêm một trường mới để nói điều ấy.
 */
export function nhanNguon(ma: string): string {
  return NHAN_NGUON[ma] ?? ma;
}

/** §10.4 — bài đồng bộ đã bị cán bộ sửa tay thì lượt đồng bộ sau không ghi đè nữa. */
export const NHAN_DA_SUA_TAY = "Đã sửa tay — lượt đồng bộ sau không ghi đè";

/* ── Phép định dạng ────────────────────────────────────────────────────────────────────────── */

/** Ô rỗng. Chưa có gì để hiện thì **dấu gạch**. */
export const DAU_GACH = "—";

/**
 * Múi giờ GHIM cho mọi phép in MỐC của màn này.
 *
 * ĐÂY LÀ HẰNG CỦA NỀN TẢNG, KHÔNG PHẢI GIÁ TRỊ CỦA MỘT XÃ (luật 8, bất biến 5): Việt Nam dùng một
 * múi giờ duy nhất trên toàn quốc, nên nó không khác nhau giữa 300 xã. Không ghim thì `updated_at`
 * in ra giờ khác nhau trên máy đặt múi giờ khác nhau — và `vitest.config.mts` ghim `TZ=UTC` đúng
 * để một phép định dạng quên `timeZone` phải đỏ ngay trên máy người viết.
 */
const MUI_GIO = "Asia/Ho_Chi_Minh";

const DINH_DANG_MOC = new Intl.DateTimeFormat("en-GB", {
  timeZone: MUI_GIO,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

function phanCua(
  phan: readonly Intl.DateTimeFormatPart[],
  loai: Intl.DateTimeFormatPartTypes,
): string {
  return phan.find((p) => p.type === loai)?.value ?? "";
}

/**
 * Mốc `date-time` của hợp đồng (`created_at`, `updated_at`) → `16:35 07/09/2026`.
 *
 * ⚠ DỰNG TỪ `formatToParts`, KHÔNG NHỜ LOCALE GHÉP HỘ: một `Intl.DateTimeFormat("vi-VN", …)` in ra
 * thứ tự của locale ấy, và thứ tự ấy đổi theo phiên bản ICU của máy chạy — tức một mốc in đúng
 * trên máy người viết và sai trên máy chủ, lặng lẽ.
 *
 * CHUỖI KHÔNG ĐỌC ĐƯỢC HIỆN NGUYÊN VĂN, không hiện `Invalid Date` và không hiện dấu gạch: một mốc
 * đoán sai trông y hệt một mốc đúng, còn dấu gạch nói dối rằng máy chủ không gửi gì.
 */
export function nhanMoc(mocISO: string | null): string {
  if (mocISO === null || mocISO === "") return DAU_GACH;
  const t = Date.parse(mocISO);
  if (Number.isNaN(t)) return mocISO;

  const phan = DINH_DANG_MOC.formatToParts(new Date(t));
  return (
    `${phanCua(phan, "hour")}:${phanCua(phan, "minute")} ` +
    `${phanCua(phan, "day")}/${phanCua(phan, "month")}/${phanCua(phan, "year")}`
  );
}

/**
 * `published_on` → `14/9/2026`, khuôn `d/M/yyyy` của §6 (KHÔNG đệm số 0).
 *
 * ⚠ CẮT CHUỖI CHỨ KHÔNG QUA `Date`, VÀ ĐÓ KHÔNG PHẢI MỘT LỐI TẮT. `published_on` là một NGÀY, không
 * phải một mốc: máy chủ gửi `2026-09-14` vì "một bài mang về từ cổng được đăng vào một NGÀY, không
 * phải vào một khoảnh khắc" (`noi_dung_mini_app.go`, chú thích của `PublishedOn`). Đưa nó qua
 * `new Date(...)` là bịa ra một khoảnh khắc — JavaScript đọc chuỗi ấy thành nửa đêm UTC — rồi in
 * lại ở một múi giờ khác, tức lệch đúng một ngày với mọi múi giờ âm và với mọi lần ai đó ghim sai
 * `timeZone`. Không có `Date` thì không có gì để lệch.
 *
 * Chuỗi không đúng khuôn hiện NGUYÊN VĂN, cùng lý do với `nhanMoc`.
 */
export function nhanNgayDang(ngay: string | null): string {
  if (ngay === null || ngay === "") return DAU_GACH;
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(ngay);
  if (m === null) return ngay;
  return `${Number(m[3])}/${Number(m[2])}/${m[1]}`;
}

/** §6 cột `Tệp đính kèm`: `🔗 Có ảnh` / `—`. Suy từ `has_image`, thứ máy chủ đã suy từ URL ảnh. */
export function nhanTepDinhKem(coAnh: boolean): string {
  return coAnh ? "🔗 Có ảnh" : DAU_GACH;
}

/** §6 cột `Lượt xem`: `👁 0`. */
export function nhanLuotXem(n: number): string {
  return `👁 ${n}`;
}

/**
 * Tóm tắt cho dòng phụ §6 (*"tóm tắt cắt 1 dòng"*).
 *
 * CẮT THEO KÝ TỰ, KHÔNG THEO DÒNG, và sự khác ấy được nói thẳng: "1 dòng" là một phép cắt của CSS
 * (`line-clamp`) phụ thuộc bề rộng thật của cột — `globals.css` chưa có lớp nào cho nó và lượt này
 * không được thêm CSS. Toàn văn tóm tắt vẫn đọc được ở khối chi tiết, nên không chữ nào mất hẳn.
 *
 * Xuống dòng bị đổi thành dấu cách: một đoạn nhiều dòng nhét vào một dòng trích sẽ dính chữ cuối
 * dòng trên vào chữ đầu dòng dưới.
 */
export const TRICH_TOI_DA = 140;

export function trichTomTat(tomTat: string, tran: number = TRICH_TOI_DA): string {
  const mot = tomTat.replace(/\s+/g, " ").trim();
  if (mot === "") return "";
  if (mot.length <= tran) return mot;
  return `${mot.slice(0, tran).trimEnd()}…`;
}

/* ── Toàn văn: chỉ lấy được từ TUYẾN CHI TIẾT ─────────────────────────────────────────────── */

/**
 * Toàn văn của một hàng, nếu hàng ấy đến từ tuyến chi tiết.
 *
 * ⚠ BA KẾT QUẢ, KHÔNG HAI, và đó là toàn bộ lý do `body` là `string | null | undefined` trong hợp
 * đồng thay vì một chuỗi:
 *
 *   vắng / `null` → hàng này đến từ DANH SÁCH, tuyến ấy cố ý không mang thân bài.
 *   `""`          → bài này THẬT SỰ không có thân.
 *   một chuỗi     → thân bài, dưới dạng MÃ NGUỒN HTML.
 *
 * Trộn hai trường hợp đầu lại là hiện một bài rỗng mà không dòng nào nói ra rằng màn hình chưa
 * hỏi. Nên hàm này trả `null` cho trường hợp đầu và bên gọi phải đi hỏi tuyến chi tiết.
 */
export function thanBaiNeuCo(nd: comms_noiDungRa): string | null {
  return nd.body === undefined || nd.body === null ? null : nd.body;
}

/** Câu thay chỗ khi thân bài thật sự rỗng. */
export const THAN_BAI_RONG = "Bài này không có nội dung.";

/* ── Danh mục: cây phẳng → ô chọn thụt đầu dòng ───────────────────────────────────────────── */

export type MucDanhMuc = {
  readonly dm: comms_danhMucRa;
  /** Độ sâu trong cây, 0 là gốc. Dùng để thụt đầu dòng nhãn ô chọn. */
  readonly muc: number;
};

/** Bao nhiêu cấp cây được vẽ. Chặn một chu trình `cha_id` làm treo phép duyệt. */
const SAU_TOI_DA = 8;

/**
 * Dựng cây danh mục từ danh sách PHẲNG máy chủ trả.
 *
 * MÁY CHỦ TRẢ PHẲNG CÓ CHỦ Ý: §7 vẽ một ô chọn và §3 vẽ một danh sách thụt đầu dòng, hai hình dạng
 * khác nhau của cùng một bộ hàng. Dựng cây là việc của bên hiển thị.
 *
 * MỘT DANH MỤC CÓ `parent_id` TRỎ TỚI HÀNG KHÔNG CÓ TRONG DANH SÁCH VẪN PHẢI RA MÀN HÌNH, ở mức
 * gốc. Bỏ nó đi là làm một danh mục biến mất khỏi ô chọn mà không dòng nào nói ra — rồi cán bộ đi
 * tìm một danh mục họ chắc chắn vừa tạo. Cùng lý do, một chu trình `cha_id` không được làm mất
 * hàng: những hàng còn lại sau khi chạm trần độ sâu được nối vào cuối.
 *
 * THỨ TỰ: `order` rồi `name`, ổn định. `order` là cột §8 dành cho việc ấy; hai hàng cùng `order`
 * thì xếp theo tên để hai lần tải cho ra cùng một thứ tự.
 */
export function dungCayDanhMuc(ds: readonly comms_danhMucRa[]): readonly MucDanhMuc[] {
  const coID = new Set(ds.map((d) => d.id));
  const theoCha = new Map<string, comms_danhMucRa[]>();
  for (const d of ds) {
    // `parent_id` rỗng là gốc; trỏ tới một hàng không có trong danh sách cũng được coi là gốc.
    const cha = d.parent_id !== "" && coID.has(d.parent_id) ? d.parent_id : "";
    const nhom = theoCha.get(cha);
    if (nhom === undefined) theoCha.set(cha, [d]);
    else nhom.push(d);
  }
  for (const nhom of theoCha.values()) {
    nhom.sort((a, b) => a.order - b.order || a.name.localeCompare(b.name, "vi"));
  }

  const ra: MucDanhMuc[] = [];
  const daXep = new Set<string>();
  const di = (chaID: string, muc: number) => {
    if (muc > SAU_TOI_DA) return;
    for (const d of theoCha.get(chaID) ?? []) {
      if (daXep.has(d.id)) continue;
      daXep.add(d.id);
      ra.push({ dm: d, muc });
      di(d.id, muc + 1);
    }
  };
  di("", 0);

  // Hàng còn sót vì một chu trình hoặc vì quá sâu: nối vào cuối ở mức gốc thay vì để mất.
  for (const d of ds) {
    if (!daXep.has(d.id)) {
      daXep.add(d.id);
      ra.push({ dm: d, muc: 0 });
    }
  }
  return ra;
}

/** Nhãn của một mục trong ô chọn — thụt đầu dòng theo độ sâu. */
export function nhanMucDanhMuc(muc: MucDanhMuc): string {
  return muc.muc === 0 ? muc.dm.name : `${"　".repeat(muc.muc)}└ ${muc.dm.name}`;
}

/**
 * Tên danh mục của một bài, cho cột §6.
 *
 * `category_id` RỖNG LÀ MỘT TRẠNG THÁI BÌNH THƯỜNG (`— Chưa xếp danh mục —`), không phải một giá
 * trị thiếu. Một id KHÔNG rỗng mà không tra được thì hiện chính id ấy chứ không hiện dấu gạch:
 * dấu gạch nói "chưa xếp danh mục", còn sự thật là màn hình chưa nạp xong cây danh mục.
 */
export function tenDanhMuc(id: string, ds: readonly comms_danhMucRa[]): string {
  if (id === "") return CHUA_XEP_DANH_MUC;
  return ds.find((d) => d.id === id)?.name ?? id;
}

/* ── Biểu mẫu §7: giá trị, thân THÊM, thân SỬA ────────────────────────────────────────────── */

/** Bảy ô của §7, đúng bảy trường hợp đồng nhận. */
export type GiaTriFormNoiDung = {
  readonly type: string;
  readonly category_id: string;
  readonly title: string;
  readonly summary: string;
  readonly body: string;
  readonly image_url: string;
  /** §7 `☐ Đăng lên Mini App`. */
  readonly publish: boolean;
};

/** Biểu mẫu trống của §7 — loại mặc định `Tin tức`, ô tích TẮT. */
export const FORM_TRONG: GiaTriFormNoiDung = {
  type: LOAI_MAC_DINH,
  category_id: "",
  title: "",
  summary: "",
  body: "",
  image_url: "",
  publish: false,
};

/**
 * Giá trị ban đầu của biểu mẫu SỬA, lấy từ một hàng đã đọc TOÀN VĂN.
 *
 * ⚠ Ô TÍCH SUY TỪ `status`, VÀ CHỈ `dang-hien` MỚI LÀ BẬT. `cho-duyet` hiện ra là ô TẮT — đúng
 * nghĩa "bà con chưa thấy" — và chính vì thế `thanSua` không được gửi `publish` khi cán bộ không
 * đụng vào ô: gửi `false` ở đó sẽ lặng lẽ chuyển một bài đang CHỜ DUYỆT thành ẨN, một chuyển
 * trạng thái không ai yêu cầu.
 *
 * `body` LẤY BẰNG `thanBaiNeuCo`, nên hàm này chỉ dùng được với hàng của TUYẾN CHI TIẾT. Gọi nó
 * với một hàng của danh sách sẽ mở ra một biểu mẫu có ô nội dung rỗng, và lần Lưu đầu tiên xoá
 * trắng thân bài.
 */
export function giaTriTuHang(nd: comms_noiDungRa): GiaTriFormNoiDung {
  return {
    type: nd.type,
    category_id: nd.category_id,
    title: nd.title,
    summary: nd.summary,
    body: thanBaiNeuCo(nd) ?? "",
    image_url: nd.image_url,
    publish: nd.status === "dang-hien",
  };
}

/** Thân `POST` từ biểu mẫu. Bảy trường, đúng bộ hợp đồng nhận — `noi-dung.ts` không thêm gì. */
export function thanThem(gt: GiaTriFormNoiDung): comms_themNoiDungVao {
  return {
    type: gt.type,
    title: gt.title.trim(),
    category_id: gt.category_id,
    summary: gt.summary.trim(),
    body: gt.body,
    image_url: gt.image_url.trim(),
    publish: gt.publish,
  };
}

/**
 * Thân `PATCH`: CHỈ những ô cán bộ thật sự đổi.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * ĐÂY LÀ PHÉP QUAN TRỌNG NHẤT CỦA BIỂU MẪU SỬA, và nó canh một thứ không màn hình nào nhìn thấy.
 *
 * Ở máy chủ mỗi trường của `suaNoiDungVao` là một CON TRỎ: vắng nghĩa là "để nguyên", có giá trị
 * nghĩa là "đặt thành đúng cái này". Gửi cả bảy trường mỗi lần Lưu thì một cán bộ chỉ sửa dấu
 * chính tả trong tiêu đề cũng gửi kèm `publish` — và với một bài đang `cho-duyet`, `publish:
 * false` sẽ đổi nó thành `an` trong im lặng. Không có gì đỏ, không có gì báo, bà con chỉ đơn giản
 * không còn thấy bài ấy nữa.
 *
 * `body` KHÔNG ĐƯỢC `trim()` KHI SO SÁNH VÀ KHÔNG ĐƯỢC `trim()` KHI GỬI. Máy chủ đã cắt hai đầu
 * khi lưu, nên một bản `trim()` ở đây sẽ làm phép so "đã đổi chưa" báo ĐỔI ở mọi lần mở lại một
 * bài có khoảng trắng ở đầu — tức mọi lần Lưu đều ghi đè thân bài dù không ai sửa nó.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */
export function thanSua(
  dau: GiaTriFormNoiDung,
  moi: GiaTriFormNoiDung,
): comms_suaNoiDungVao {
  const ra: comms_suaNoiDungVao = {};

  if (moi.type !== dau.type) ra.type = moi.type;
  if (moi.category_id !== dau.category_id) ra.category_id = moi.category_id;
  if (moi.title.trim() !== dau.title) ra.title = moi.title.trim();
  if (moi.summary.trim() !== dau.summary) ra.summary = moi.summary.trim();
  if (moi.body !== dau.body) ra.body = moi.body;
  if (moi.image_url.trim() !== dau.image_url) ra.image_url = moi.image_url.trim();
  if (moi.publish !== dau.publish) ra.publish = moi.publish;

  return ra;
}

/** Có gì để gửi không. Không có thì màn nói ra, thay vì gọi một `PATCH` rỗng. */
export function coThayDoi(than: Record<string, unknown>): boolean {
  return Object.keys(than).length > 0;
}

/** Câu hiện khi bấm Lưu mà không ô nào đổi. */
export const KHONG_CO_GI_DOI = "Chưa có ô nào thay đổi, nên không có gì để lưu.";

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA ĐẶC TẢ **KHÔNG DỰNG ĐƯỢC**, VÀ CHÚNG PHẢI RA TỚI MÀN HÌNH
 *
 * Không giấu trong chú thích, không vẽ một nút chắc chắn hỏng. Cùng khuôn `PHAN_CHUA_DUNG` của màn
 * Thông báo, màn Biên bản họp và màn Nhiệm vụ.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  {
    ten: "Ô soạn thảo rich text, và phép LÀM SẠCH HTML đứng sau nó (§7, §8)",
    viSao:
      "ĐÂY LÀ MỤC QUAN TRỌNG NHẤT CỦA DANH SÁCH NÀY. §8 khai `noi_dung` là HTML và §7 muốn một ô " +
      "soạn thảo rich text. Máy chủ LƯU NGUYÊN VĂN và nói thẳng giới hạn ấy trong mã của chính nó " +
      "(`domain/noi_dung_mini_app.go`, `ChuanHoaVanBanDai`): kho chưa có bộ làm sạch HTML nào, và " +
      "tự viết một cái là cách các bộ làm sạch bị viết sai. Hệ quả: một cán bộ có `content.update` " +
      "đặt được markup tuỳ ý — kể cả `<script>` — vào thứ mọi cư dân xã mở trên điện thoại, và " +
      "QUYỀN là biện pháp duy nhất hôm nay. Vì thế màn quản trị KHÔNG dựng HTML ở bất kỳ đâu, kể " +
      "cả để xem trước: toàn văn hiện dưới dạng VĂN BẢN THUẦN. Thứ còn thiếu là một bộ làm sạch " +
      "đã được kiểm chứng, đặt ở chỗ Mini App dựng bài hoặc ở biên máy chủ — một quyết định có " +
      "chủ, không phải một dòng ai đó thêm vào.",
  },
  {
    ten: "Toàn bộ thẻ “Đồng bộ tin từ Cổng thông tin điện tử” (§3): chip trạng thái, `⟳ Đồng bộ ngay`, `Cấu hình`, `Chạy lần cuối`, khối log lỗi, cây 60 chuyên mục",
    viSao:
      "Bảng `cau_hinh_dong_bo_cong` chưa tồn tại, và thứ chặn nó là ADR 0009 quyết định 7: " +
      "`core/crypto` chưa tồn tại, mà cột trung tâm của bảng ấy là `ma_bao_mat` — credential thô " +
      "của một cổng thông tin chính quyền, thứ sẽ nằm trong mọi bản sao lưu nếu lưu thẳng. Bảng " +
      "`chuyen_muc_cong` chỉ có nghĩa cùng bảng trên, và nhịp `Mỗi 6 giờ` cần một bộ lập lịch cộng " +
      "một adapter HTTP đi ra THEO XÃ mà kho chưa có. Vẽ thẻ ấy với số liệu bịa là dựng một màn " +
      "hình nói với xã rằng cổng của họ đang được đồng bộ.",
  },
  {
    ten: "Ô `Ảnh đại diện` dạng `Chọn tệp từ máy` — `JPG, PNG hoặc WebP — tối đa 50MB` (§7, §10.6)",
    viSao:
      "`core/storage` chưa tồn tại: không có chỗ nhận tệp, không có đường phát ra một liên kết đã " +
      "ký. Máy chủ chỉ nhận `image_url`, một LIÊN KẾT, và chỉ nhận lược đồ `http`/`https` — danh " +
      "sách trắng ấy không phải làm đẹp: `anh_dai_dien_url` được dựng thành `src` của một thẻ ảnh " +
      "trong ứng dụng bà con cầm trên tay, nên `javascript:…` ở đó là thực thi mã trên kênh công " +
      "dân. Màn hình vì thế vẽ một ô NHẬP LIÊN KẾT và nói trước điều máy chủ sẽ từ chối, thay vì " +
      "một nút chọn tệp không có nơi để gửi tệp tới.",
  },
  {
    ten: "Sửa và xoá một danh mục tin (§6, `⊞ Danh mục tin`)",
    viSao:
      "Hợp đồng chỉ có `GET` và `POST` trên `/api/v1/content-categories` — không có `PATCH`, không " +
      "có `DELETE`. Nên màn này THÊM được danh mục và không sửa được tên đã gõ sai. Một nút sửa vẽ " +
      "ra ở đây là một nút không có tuyến nào phía sau. Lưu ý kèm theo, vì nó sẽ làm người dùng " +
      "ngạc nhiên: slug đã cấp thì KHÔNG cấp lại, kể cả sau khi danh mục mang slug ấy bị xoá — máy " +
      "chủ trả 409 kèm nguyên câu giải thích, và câu ấy ra thẳng màn hình.",
  },
  {
    ten: "Bốn nhóm trường theo loại nội dung (§7, dòng cuối)",
    viSao:
      "§7 kết bằng một câu “NÊN BỔ SUNG”: `Video` thêm URL video, `Truyền thanh` thêm tệp audio và " +
      "thời lượng, `Sự kiện` thêm thời gian và địa điểm, `Banner` thêm link đích và thứ tự hiển " +
      "thị. §8 KHÔNG có cột nào cho bốn nhóm ấy — đặc tả đang đề xuất với chính nó, nên đây là một " +
      "CÂU CHỜ KHÁCH chứ không phải một phần bị bỏ sót. Đoán một hình dạng rồi dựng ô nhập là tự " +
      "quyết một câu của khách, và bốn nhóm trường đoán sai là bốn cột phải di trú lại trên dữ " +
      "liệu thật.",
  },
  {
    ten: "Tuyến công khai cho Mini App đọc (§9: `/api/cong/mini-app/noi-dung`, `/api/cong/mini-app/danh-ba`)",
    viSao:
      "Chặn bởi một điều nặng hơn thứ tự ưu tiên: **Mini App KHÔNG CÓ TÊN MIỀN**. Hệ thống phân " +
      "biệt xã bằng `Host`, nên một tuyến không phiên, gọi từ một App ID dùng chung cho mọi xã, " +
      "không xác định được xã — và luật 1 bất biến 3 nói rõ phải làm gì khi không xác định được " +
      "xã: 404, không bao giờ một xã mặc định. Hình dạng đúng của tuyến ấy là một quyết định về " +
      "kênh công dân (ADR 0005 · 0019 · 0022), không phải một route thêm vào cho đủ §9.",
  },
  {
    ten: "Con số `Đang hiện 26 cán bộ cho bà con` trên thẻ Danh bạ chính quyền (§4)",
    viSao:
      "Liên kết sang `/danh-ba` thì vẽ được và có vẽ. Con số thì không: nó đếm cờ " +
      "`hien_tren_mini_app` trên danh bạ cán bộ, mà danh bạ thuộc `service-identity` (luật 2) và " +
      "không tuyến nào của hợp đồng hôm nay trả về con số ấy. Hiện một số 0 ở chỗ đó là nói với xã " +
      "rằng bà con không thấy cán bộ nào.",
  },
  {
    ten: "Cột `Lượt xem` luôn bằng 0 (§6)",
    viSao:
      "Không phải lỗi hiển thị: `luot_xem` KHÔNG BAO GIỜ giảm và màn cán bộ KHÔNG tăng nó. Thứ duy " +
      "nhất được phép tăng nó là một cư dân mở bài trên Mini App, mà tuyến công khai của §9 chưa " +
      "dựng (mục trên). Cột vẫn được vẽ vì nó là cột của §6 và vì ngày tuyến ấy ra đời, con số " +
      "chạy mà không màn nào phải sửa — kèm một dòng chữ nói rõ vì sao hôm nay nó là 0.",
  },
  {
    ten: "Nút xoá một bài (§9 đề xuất `DELETE`)",
    viSao:
      "§6 chỉ có `✎` và hợp đồng không có tuyến `DELETE` nào — cả hai khớp nhau, nên đây là một " +
      "quyết định chứ không phải một thiếu sót. Gỡ một bài khỏi Mini App là `PATCH` với ô tích " +
      "`Đăng lên Mini App` tắt đi. Nếu ngày nào cần xoá thật, luật 7 biến nó thành xoá MỀM bắt " +
      "buộc có `delete_reason`, mà không màn nào thu câu ấy — tức tuyến xoá kéo theo một ô nhập lý " +
      "do, không phải một nút thùng rác.",
  },
  {
    ten: "Bố cục §2: tabs thật, hai thẻ đầu màn, modal dạng lớp phủ, cắt tóm tắt đúng 1 dòng",
    viSao:
      "Cả bốn là chuyện của CSS: `globals.css` chưa có `.man-noi-dung`, chưa có lớp cho tab, cho " +
      "lớp phủ, cho `line-clamp`, và lượt này không được thêm CSS. Sáu tab vì thế là sáu nút " +
      "`aria-pressed` dùng lại `.thanh-sap-xep`; hai biểu mẫu dựng NỐI TIẾP trong trang thay vì " +
      "làm lớp phủ; tóm tắt cắt theo KÝ TỰ, một phép xấp xỉ, với toàn văn nằm ở khối chi tiết. Tên " +
      "lớp cần thêm đã báo về.",
  },
  {
    ten: "Cổng quyền `content.read` / `content.update` ở phía giao diện",
    viSao:
      "`src/lib/quyen.ts` chưa có hằng cho hai khoá ấy và lượt này không được sửa tệp đó, còn gõ " +
      "thẳng chuỗi vào màn là dựng bản sao thứ hai của một khoá phân quyền. Vì thế màn này KHÔNG " +
      "có cổng ở client — đúng khuôn màn Thông báo và màn Văn bản: `service-comms` kiểm quyền trên " +
      "TỪNG lời gọi, và tài khoản thiếu khoá nhận nguyên câu 403 của máy chủ ra màn hình. Ẩn một " +
      "nút chưa bao giờ là biện pháp (luật 5, cấm #1); thiếu nó ở đây chỉ tốn một lần bấm.",
  },
];
