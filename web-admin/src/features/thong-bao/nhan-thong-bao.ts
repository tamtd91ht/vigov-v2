/**
 * Chữ và phép quyết định của màn "Thông báo" — `docs/ui-ux/08-thong-bao.md`.
 *
 * TÁCH KHỎI COMPONENT ĐỂ KIỂM ĐƯỢC TỪNG PHÉP MỘT. Hàm thuần: không gọi mạng, không dựng DOM,
 * không đọc đồng hồ. Mọi chuỗi tiếng Việt lấy NGUYÊN VĂN từ đặc tả — một chữ khác đi trên màn của
 * cơ quan nhà nước là một chữ có người phải trả lời.
 */

import type { comms_thongBaoRa } from "@/lib/api/schema.gen";

/* ── Trần độ dài, đúng bằng trần máy chủ ────────────────────────────────────────────────────
 *
 * Chép từ `service-comms/internal/domain/thong_bao_noi_bo.go` và CHỈ để ô nhập dừng lại đúng chỗ
 * máy chủ sẽ dừng — không phải để thay phép kiểm ấy. Máy chủ vẫn là nơi từ chối thật; `maxLength`
 * ở ô nhập chỉ để cán bộ thấy giới hạn thay vì thấy một câu 400 sau khi đã gõ xong cả trang.
 *
 * ⚠ MÁY CHỦ TỪ CHỐI CHỨ KHÔNG CẮT BỚT. Một tiêu đề bị cắt âm thầm ở 300 ký tự là một thông báo đã
 * đổi chủ đề trên đường vào CSDL, nên hai bên phải dừng ở cùng một con số.
 */
export const TIEU_DE_TOI_DA = 300;
export const NOI_DUNG_TOI_DA = 20000;
/** Bao nhiêu người nhận MỘT thông báo mang được. */
export const NGUOI_NHAN_TOI_DA = 500;
/** Độ dài một mã cán bộ (`CB-2026-7K3M9Q`). */
export const MA_CAN_BO_TOI_DA = 64;

/* ── Chữ trên màn ──────────────────────────────────────────────────────────────────────────── */

export const TIEU_DE_MAN = "Thông báo";

/** §1 — nguyên văn câu mô tả trong giao diện. */
export const MO_TA_MAN =
  "Gửi tới các bộ phận, kèm thư điện tử. Người nhận thấy ngay ở trang này.";

export const NHAN_NUT_SOAN = "+ Soạn thông báo";
export const NHAN_NUT_HUY = "Huỷ";
/** §5 — nguyên văn, kể cả mũi tên. */
export const NHAN_NUT_PHAT_HANH = "➤ Phát hành";

/** §5 — nguyên văn mô tả dưới tiêu đề modal. */
export const MO_TA_SOAN =
  "Thông báo gửi tới từng cán bộ của các bộ phận đã chọn, kèm thư điện tử nếu bật.";

/** §5 — nguyên văn placeholder ô tiêu đề. */
export const PLACEHOLDER_TIEU_DE = "Mời họp giao ban tháng 9";

/** §4 — nguyên văn câu của cột phải khi chưa chọn thẻ nào. */
export const CHUA_CHON_THONG_BAO = "Chọn một thông báo để xem.";

/**
 * Sổ rỗng. Đặc tả không có câu nào cho trạng thái này, nên câu dưới là câu viết mới, cùng giọng
 * với các màn đã có — và điều ấy được nói ra ở đây thay vì để người sau tưởng mình đọc lại một câu
 * của đặc tả.
 */
export const SO_RONG = "Chưa có thông báo nào.";

export const DANG_TAI_SO = "Đang tải danh sách thông báo…";

/**
 * Nhãn của phạm vi ĐANG hiện. §2 vẽ hai nút phân đoạn `Gửi cho tôi` · `Cả sổ thông báo`; máy chủ
 * chỉ trả lời được vế thứ hai, nên màn hiện MỘT nhãn nói rõ mình đang hiện gì, thay vì hai nút mà
 * một nút không đổi được gì.
 */
export const PHAM_VI_DANG_HIEN =
  "Đang hiện: Cả sổ thông báo. Bộ lọc “Gửi cho tôi” chưa dựng được — xem phần chưa dựng được ở " +
  "đầu màn.";

/**
 * Câu đứng ngay dưới ô tick "Gửi thư điện tử" của §5.
 *
 * Ô tick VẪN ĐƯỢC VẼ và vẫn mặc định bật, đúng §5, vì cột `gui_thu_dien_tu` ghi lại ĐIỀU ĐÃ ĐƯỢC
 * YÊU CẦU — một sự thật riêng, khác hẳn với việc thư đã gửi hay chưa. Nhưng để nó đứng một mình là
 * hứa với cán bộ một lá thư sẽ không bao giờ đi; câu này đứng cạnh nó, không nằm trong chú thích mã.
 */
export const CANH_BAO_CHUA_GUI_THU =
  "Hệ thống chưa gửi được thư điện tử — ô này chỉ ghi lại yêu cầu, chưa có thư nào rời khỏi máy chủ.";

/** Câu đứng ngay dưới ô tick "Ghim" của §5 — nói đúng phạm vi của việc ghim hôm nay. */
export const CANH_BAO_GHIM_TRONG_TRANG =
  "Ghim chỉ nâng thẻ lên đầu TRANG ĐANG XEM, chưa phải đầu cả quyển sổ.";

/** Câu đứng dưới ô nhập người nhận — nói rõ đây là MÃ, không phải họ tên. */
export const GHI_CHU_NGUOI_NHAN =
  "Mỗi dòng một mã cán bộ, ví dụ CB-2026-7K3M9Q. Đây là mã nghiệp vụ, không phải họ tên — ô chọn " +
  "theo tên và email chưa dựng được, xem phần chưa dựng được ở đầu màn.";

/** Chỗ đáng lẽ là nút `🗑 Gỡ` của §4 — một dòng chữ, không phải một nút mờ. */
export const CHO_NUT_GO = "🗑 Gỡ — chưa dựng, xem phần chưa dựng được ở đầu màn";

/** Chỗ đáng lẽ là danh sách `NGƯỜI NHẬN (12)` của §4. */
export const CHO_DANH_SACH_NGUOI_NHAN =
  "Danh sách người nhận và trạng thái từng người chưa đọc về được — xem phần chưa dựng được ở đầu màn.";

/* ── Phép định dạng ────────────────────────────────────────────────────────────────────────── */

/** Ô rỗng. Chưa có gì để hiện thì **dấu gạch**. */
export const DAU_GACH = "—";

/**
 * Múi giờ GHIM cho mọi phép in mốc của màn này.
 *
 * ĐÂY LÀ HẰNG CỦA NỀN TẢNG, KHÔNG PHẢI GIÁ TRỊ CỦA MỘT XÃ (luật 8, bất biến 5): Việt Nam dùng một
 * múi giờ duy nhất trên toàn quốc, nên nó không khác nhau giữa 300 xã. Không ghim thì `issued_at` —
 * một mốc `date-time` — in ra giờ khác nhau trên máy đặt múi giờ khác nhau, và §3 in giờ phát hành
 * tới từng phút.
 */
const MUI_GIO = "Asia/Ho_Chi_Minh";

/**
 * `HH:mm dd/MM/yyyy` của §3 — `16:35 07/09/2026`, có đệm số 0.
 *
 * ⚠ DỰNG TỪ `formatToParts`, KHÔNG GHÉP HAI BỘ ĐỊNH DẠNG VÀ KHÔNG TIN THỨ TỰ CỦA LOCALE. Một
 * `Intl.DateTimeFormat("vi-VN", {…})` in ra thứ tự của locale ấy, và thứ tự ấy đổi theo phiên bản
 * ICU của máy chạy — tức một mốc in đúng trên máy người viết và sai trên máy chủ, lặng lẽ. Khuôn
 * `HH:mm dd/MM/yyyy` là chữ của đặc tả, nên nó được ghép ở đây chứ không được nhờ locale ghép hộ.
 */
const DINH_DANG_MOC = new Intl.DateTimeFormat("en-GB", {
  timeZone: MUI_GIO,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hour12: false,
});

/** Một mảnh của mốc đã định dạng, lấy theo TÊN chứ không theo vị trí trong mảng. */
function phanCua(
  phan: readonly Intl.DateTimeFormatPart[],
  loai: Intl.DateTimeFormatPartTypes,
): string {
  return phan.find((p) => p.type === loai)?.value ?? "";
}

/**
 * Mốc `date-time` của hợp đồng → `16:35 07/09/2026`. `null` → `—`.
 *
 * CHUỖI KHÔNG ĐỌC ĐƯỢC HIỆN NGUYÊN VĂN, không hiện `Invalid Date` và không hiện dấu gạch: một mốc
 * đoán sai trông y hệt một mốc đúng, còn dấu gạch nói dối rằng máy chủ không gửi gì.
 */
export function nhanMoc(mocISO: string | null): string {
  if (mocISO === null || mocISO === "") return DAU_GACH;
  const t = Date.parse(mocISO);
  if (Number.isNaN(t)) return mocISO;

  const phan = DINH_DANG_MOC.formatToParts(new Date(t));
  const gio = phanCua(phan, "hour");
  const phut = phanCua(phan, "minute");
  const ngay = phanCua(phan, "day");
  const thang = phanCua(phan, "month");
  const nam = phanCua(phan, "year");
  return `${gio}:${phut} ${ngay}/${thang}/${nam}`;
}

/**
 * Mốc hiện trên thẻ §3: giờ PHÁT HÀNH.
 *
 * ⚠ RƠI VỀ `created_at` KHI `issued_at` RỖNG, và đó KHÔNG phải một giá trị mặc định trên đường
 * cách ly: cả hai mốc đều là của chính bản ghi ấy, không có gì bị đoán. `issued_at` rỗng nghĩa là
 * bản ghi còn ở trạng thái `nhap` — hôm nay không đường nào tạo ra trạng thái ấy, nhưng lược đồ có
 * nó, và một thẻ nháp hiện dấu gạch ở chỗ thời gian trông như dữ liệu hỏng.
 */
export function mocThe(tb: comms_thongBaoRa): string {
  return nhanMoc(tb.issued_at !== null && tb.issued_at !== "" ? tb.issued_at : tb.created_at);
}

/**
 * Trích nội dung cho thẻ §3 (*"2 dòng, cắt bớt"*).
 *
 * CẮT THEO KÝ TỰ, KHÔNG THEO DÒNG, và sự khác ấy được nói thẳng chứ không giấu: "2 dòng" là một
 * phép cắt của CSS (`line-clamp`), phụ thuộc bề rộng thật của cột — `globals.css` chưa có lớp nào
 * cho nó và lượt này không được thêm CSS. Cắt theo ký tự là một phép XẤP XỈ; toàn văn nằm ở cột
 * phải, nên không có chữ nào mất hẳn khỏi màn.
 *
 * Xuống dòng bị đổi thành dấu cách: một đoạn văn nhiều dòng nhét vào một dòng trích sẽ dính chữ
 * cuối dòng trên vào chữ đầu dòng dưới.
 */
export const TRICH_TOI_DA = 160;

export function trichNoiDung(noiDung: string, tran: number = TRICH_TOI_DA): string {
  const mot = noiDung.replace(/\s+/g, " ").trim();
  if (mot.length <= tran) return mot;
  return `${mot.slice(0, tran).trimEnd()}…`;
}

/* ── Trạng thái: hai bộ mã, hai bảng nhãn ──────────────────────────────────────────────────── */

/** Ba trạng thái §6 của bản ghi. Danh sách ĐÓNG — lược đồ CSDL cưỡng chế đúng ba giá trị này. */
const NHAN_TRANG_THAI: Readonly<Record<string, string>> = {
  nhap: "Nháp",
  "da-phat-hanh": "Đã phát hành",
  "da-go": "Đã gỡ",
};

/**
 * Nhãn trạng thái để hiện, kể cả khi máy chủ gửi một mã màn hình chưa biết.
 *
 * MÃ LẠ HIỆN NGUYÊN VĂN, KHÔNG HIỆN DẤU GẠCH và không im lặng bỏ qua: một trạng thái mới ở máy chủ
 * mà màn hình vẽ thành `—` là một thông báo trông như chưa có trạng thái.
 */
export function nhanTrangThai(ma: string): string {
  return NHAN_TRANG_THAI[ma] ?? ma;
}

/**
 * §3 chỉ vẽ chip trạng thái cho thông báo KHÔNG còn bình thường. `da-phat-hanh` là trạng thái của
 * gần như mọi thẻ, và một chip "Đã phát hành" trên mọi thẻ là một chip không nói gì.
 */
export function coChipTrangThai(ma: string): boolean {
  return ma !== "da-phat-hanh";
}

/**
 * Nhãn chip trạng thái thư §3 — `null` nghĩa là KHÔNG VẼ CHIP NÀO.
 *
 * ⚠ HÔM NAY HÀM NÀY LUÔN TRẢ `null`, và đó là sự thật đáng nói ra chứ không phải mã chết: kho
 * không có bộ gửi thư nào, nên `trang_thai_thu` ở MỌI hàng là `chua-gui`
 * (`service-comms/internal/domain/thong_bao_noi_bo.go`, `TrangThaiThuThongBao`). Ba nhãn còn lại
 * được viết ra đúng chữ đặc tả để ngày có bộ gửi thư, chỗ này không phải nghĩ lại — và §3 KHÔNG có
 * nhãn nào cho `chua-gui`, nên `null` là câu trả lời đúng chứ không phải một chỗ còn thiếu.
 */
export function nhanTrangThaiThu(ma: string): string | null {
  switch (ma) {
    case "dang-gui":
      return "Đang gửi thư…";
    case "da-gui":
      return "Đã gửi thư";
    case "loi":
      return "Gửi thư lỗi";
    default:
      return null;
  }
}

/* ── Bộ đếm xác nhận ───────────────────────────────────────────────────────────────────────── */

/**
 * `{x}/{y} đã xác nhận` của §3 — `null` nghĩa là KHÔNG VẼ.
 *
 * CHỈ HIỆN KHI `ack_required`, đúng §3. Hai con số VẪN được máy chủ gửi kèm khi cờ tắt, có chủ ý:
 * "có nên vẽ hay không" đã có đúng một nguồn là cái cờ, và một cờ thứ hai suy từ con số là bản sao
 * sẽ trôi (luật 9, cấm #2).
 *
 * HAI CON SỐ DO MÁY CHỦ ĐẾM từ bảng người nhận và không nằm ở cột nào. Cộng lại ở client thì không
 * có gì để cộng — màn hình không có danh sách người nhận (xem `PHAN_CHUA_DUNG`).
 */
export function nhanBoDemXacNhan(tb: comms_thongBaoRa): string | null {
  if (!tb.ack_required) return null;
  return `${tb.ack_count}/${tb.recipient_count} đã xác nhận`;
}

/** Chip cam của §3, chỉ hiện khi bật cờ. */
export const CHIP_BAT_BUOC_XAC_NHAN = "Bắt buộc xác nhận";

/** Ký hiệu ghim ở đầu thẻ §3. */
export const DAU_GHIM = "📌";

/* ── Ghim: nâng trong TRANG, không phải thứ tự toàn sổ ─────────────────────────────────────── */

/**
 * Đưa các thẻ `pinned` lên đầu **trang đang xem**, giữ nguyên thứ tự tương đối của hai nhóm.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * ĐÂY KHÔNG PHẢI "sắp xếp lại quyển sổ", VÀ SỰ KHÁC BIỆT ẤY PHẢI RA TỚI MÀN HÌNH.
 *
 * `core/page` mang ĐÚNG MỘT cột sắp xếp cộng `id` để phá hoà, nên `ORDER BY ghim DESC, tao_luc
 * DESC` không diễn đạt được bằng con trỏ của kho này. Xấp xỉ nó trong con trỏ sẽ làm trang hai vừa
 * LẶP vừa BỎ SÓT hàng, im lặng — thứ chỉ lộ ra khi một cán bộ đi tìm một thông báo họ chắc chắn đã
 * thấy. Nên máy chủ trả đúng thứ tự `tao_luc` giảm dần, và phép nâng này chỉ đổi thứ tự BÊN TRONG
 * một trang đã tải.
 *
 * Hệ quả nhìn thấy được, và màn nói thẳng: một thông báo ghim từ tháng trước nằm ở trang 3 thì nó
 * ở đầu TRANG 3, không phải ở đầu quyển sổ.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * PHÂN HOẠCH ỔN ĐỊNH, KHÔNG PHẢI `sort`: `Array.prototype.sort` trên một khoá boolean là một phép
 * so trả 0 cho mọi cặp cùng nhóm, và bảo toàn thứ tự khi ấy là chuyện của cách cài đặt chứ không
 * phải của đặc tả ngôn ngữ trong mọi phiên bản. Hai lượt lọc thì không có gì để tin.
 */
export function nangGhimLenDau(
  ds: readonly comms_thongBaoRa[],
): readonly comms_thongBaoRa[] {
  return [...ds.filter((t) => t.pinned), ...ds.filter((t) => !t.pinned)];
}

/** Câu đứng trên danh sách, nói đúng phạm vi của phép nâng ngay trên. */
export const GHI_CHU_GHIM_TRONG_TRANG =
  "Thông báo ghim được nâng lên đầu TRANG ĐANG XEM, không phải đầu cả quyển sổ.";

/* ── Biểu mẫu §5 ───────────────────────────────────────────────────────────────────────────── */

/**
 * Tách ô "Gửi thêm đích danh" thành danh sách mã cán bộ: mỗi dòng một mã.
 *
 * CHỈ TÁCH VÀ BỎ DÒNG TRẮNG — KHÔNG kiểm hình dạng mã. `service-comms` cố ý không kiểm khuôn của
 * `CB-2026-7K3M9Q` vì khuôn ấy là của `identity` và đổi được (luật 2); dựng lại một biểu thức
 * chính quy ở đây sẽ là bản sao thứ ba của một sự thật thuộc về service khác, và bản sao ấy từ
 * chối một mã thật vào ngày `identity` nới khuôn.
 *
 * Gõ Enter hai lần là một thói quen gõ văn bản, không phải một người nhận.
 */
export function tachMaNguoiNhan(chu: string): string[] {
  return chu
    .split("\n")
    .map((d) => d.trim())
    .filter((d) => d !== "");
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA ĐẶC TẢ **KHÔNG DỰNG ĐƯỢC**, VÀ CHÚNG PHẢI RA TỚI MÀN HÌNH
 *
 * Không giấu trong chú thích, không vẽ một nút chắc chắn hỏng. Cùng khuôn `PHAN_CHUA_DUNG` của màn
 * Biên bản họp và màn Nhiệm vụ.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  {
    ten: "Ô chọn `Bộ phận nhận thông báo` (§5) và khối `BỘ PHẬN NHẬN` (§4)",
    viSao:
      "Máy chủ trả **501 `not_implemented`** cho bất kỳ thân yêu cầu nào mang `org_unit_ids`, kèm " +
      "nguyên câu chỉ đường: “Hãy chọn từng người ở mục Gửi thêm đích danh”. Đó là TỪ CHỐI CÓ CHỦ " +
      "Ý chứ không phải lỗi — nở một bộ phận thành danh sách cán bộ là dữ liệu của `identity`, và " +
      "`IdentityService` không có RPC nào làm việc ấy (`proto/` là nguồn chuẩn của hợp đồng ấy, " +
      "luật 2). Lối còn lại — ghi nhận bộ phận rồi gửi cho không ai — là kết cục duy nhất module " +
      "này không được có: không lỗi ở đâu cả, một tấm thẻ trong sổ, bộ đếm đọc `0/0`, và người " +
      "soạn tin rằng cả xã đã được báo. Vì thế màn KHÔNG vẽ ô chọn bộ phận: một ô mà mọi lần bấm " +
      "đều nhận 501 là một ô tệ hơn là không có ô.",
  },
  {
    ten: "Ô chọn người nhận dạng `Họ tên — email` (§5)",
    viSao:
      "Tuyến duy nhất tra được danh bạ cán bộ — `GET /api/v1/staff` — đòi `admin.user`, một khoá " +
      "quản trị hệ thống không liên quan tới việc soạn thông báo. Nên ô người nhận là ô GÕ MÃ " +
      "nghiệp vụ, mỗi dòng một mã. Khác với ô `Chủ trì` của màn Biên bản họp — chỗ ấy bỏ hẳn ô đi " +
      "được vì trường tuỳ chọn — ở đây người nhận là thứ DUY NHẤT còn gửi được: bỏ ô này là tuyến " +
      "phát hành không còn đường nào chạy, vì một thông báo không người nhận bị máy chủ từ chối.",
  },
  {
    ten: "Nút `Lưu nháp` (§5)",
    viSao:
      "Không có tuyến nào tạo nháp. Trạng thái `nhap` CÓ trong lược đồ vì §6 khai nó, nhưng không " +
      "đường ghi nào viết ra nó — một thông báo tồn tại là một thông báo đã phát hành. Vẽ nút ấy " +
      "rồi cho nó phát hành luôn là biến một nút “lưu để sửa tiếp” thành một nút gửi đi cả xã.",
  },
  {
    ten: "Bộ lọc `Gửi cho tôi` (§2)",
    viSao:
      "Tuyến đọc KHÔNG nhận `pham_vi` — cố ý, chứ không phải nhận rồi bỏ qua: một tham số được " +
      "đọc mà không có tác dụng là cách một màn hình mang nhãn “Gửi cho tôi” hiện ra cả sổ. Lý do " +
      "sâu hơn nằm ở phân quyền: bảng `quyen` KHÔNG CÓ khoá đọc thông báo nào — `announcement." +
      "create` là khoá duy nhất của nhóm `THÔNG BÁO` — nên “mọi cán bộ đọc thông báo gửi cho " +
      "mình” hôm nay chỉ diễn đạt được bằng một khoá chưa tồn tại (câu hỏi mở #27, không bao giờ " +
      "là một `INSERT`) hoặc bằng `AnyAuthenticated`, đúng điều kiện dừng thứ nhất của luật 5. Cả " +
      "hai đều là quyết định của khách.",
  },
  {
    ten: "Panel chi tiết: danh sách `NGƯỜI NHẬN` và trạng thái từng người (§4)",
    viSao:
      "Không có tuyến chi tiết (`GET /api/thong-bao/:id` của §7 chưa tồn tại), và phản hồi danh " +
      "sách chỉ mang HAI CON SỐ đếm chứ không mang danh sách người. Thứ một tuyến chi tiết sẽ " +
      "THÊM chính là danh sách ấy kèm HỌ TÊN cán bộ — mà `service-comms` không sở hữu danh bạ " +
      "(luật 2), nên đó là một lời gọi `identity`, không phải một phép nối SQL. Cột phải vì thế " +
      "hiện TOÀN VĂN nội dung (đã có sẵn trên phản hồi danh sách) và nói thẳng rằng phần người " +
      "nhận chưa đọc về được.",
  },
  {
    ten: "Nút `🗑 Gỡ` thu hồi thông báo (§4)",
    viSao:
      "Không có tuyến nào. §9.2 cấm sửa nội dung sau khi phát hành và bảo “muốn sửa thì gỡ và " +
      "soạn lại”, nên thiếu nút gỡ nghĩa là một thông báo gõ sai chữ không có đường sửa nào ở màn " +
      "này. Trạng thái `da-go` có trong lược đồ và màn hình vẽ được chip cho nó — chỉ không có " +
      "đường nào đặt nó.",
  },
  {
    ten: "Chip `✓ Đã xác nhận` của người đang đăng nhập, và hành vi `xác nhận` / `đã mở` (§3, §4)",
    viSao:
      "Phản ánh của §3 là chip theo NGƯỜI ĐANG XEM, nhưng phản hồi không mang trường nào nói " +
      "người đang xem đã xác nhận hay chưa — chỉ có hai con số tổng. Hai tuyến `…/xac-nhan` và " +
      "`…/da-mo` của §7 cũng chưa có. Suy chip ấy từ `ack_count > 0` là dựng một câu trả lời cho " +
      "một câu hỏi khác hẳn: “có người xác nhận” không phải “TÔI đã xác nhận”.",
  },
  {
    ten: "Chip trạng thái thư trên thẻ (§3) và ô `Cấu hình → Máy chủ thư`",
    viSao:
      "Kho không có bộ gửi thư nào, nên `trang_thai_thu` ở MỌI hàng là `chua-gui` và §3 không có " +
      "nhãn cho giá trị ấy — chip thư vì thế không bao giờ hiện. Ô tick `Gửi thư điện tử` VẪN " +
      "được vẽ, vì cột `gui_thu_dien_tu` ghi lại ĐIỀU ĐÃ ĐƯỢC YÊU CẦU (một sự thật khác với “thư " +
      "đã đi”), nhưng ngay dưới nó là một câu nói rõ chưa có thư nào rời khỏi máy chủ.",
  },
  {
    ten: "Ghim lên đầu CẢ SỔ (§3)",
    viSao:
      "`core/page` mang ĐÚNG MỘT cột sắp xếp cộng `id` phá hoà, nên `ORDER BY ghim DESC, tao_luc " +
      "DESC` không diễn đạt được bằng con trỏ của kho này; xấp xỉ nó sẽ làm trang hai vừa lặp vừa " +
      "bỏ sót hàng, im lặng. Trường `pinned` CÓ trên phản hồi, nên màn nâng thẻ ghim lên đầu " +
      "TRANG ĐANG XEM và nói rõ đó là nâng trong trang. Hệ quả thật: một thông báo ghim nằm ở " +
      "trang 3 thì nó ở đầu trang 3.",
  },
  {
    ten: "Hộp thư chuông ở header (§8)",
    viSao:
      "Bảng `hop_thu_thong_bao` chưa tồn tại và §8 nói rõ nó là hộp thư HỢP NHẤT — bốn loại mục, " +
      "ba trong số đó do `service-petitions` đẩy vào. Đó là một hợp đồng giữa các service, không " +
      "phải một hàng mà module này được tự tạo. Không có tuyến `…/chua-doc`, nên chuông không có " +
      "gì để đếm.",
  },
  {
    ten: "Cắt trích đúng 2 dòng, viền xanh thẻ đang chọn, và bố cục hai cột 2/3 – 1/3 (§2, §3)",
    viSao:
      "Cả ba là chuyện của CSS: `globals.css` chưa có lớp nào cho lưới hai cột, cho viền thẻ đang " +
      "chọn, cho `line-clamp`, và cũng chưa có `.man-thong-bao` — lượt này không được thêm CSS. " +
      "Thẻ đang chọn vì thế đánh dấu bằng `aria-current`, thứ không cần lớp nào. Màn dùng lại các " +
      "lớp sẵn có " +
      "(`khoi-chi-tiet`, `chip`, `dong-phu`), xếp hai phần NỐI TIẾP thay vì hai cột, và trích nội " +
      "dung bằng cách cắt theo KÝ TỰ — một phép xấp xỉ, với toàn văn nằm ngay ở phần chi tiết. " +
      "Biểu mẫu §5 cũng dựng nối tiếp trong trang thay vì làm lớp phủ. Tên lớp cần thêm đã báo về.",
  },
  {
    ten: "Cổng quyền `announcement.create` ở phía giao diện",
    viSao:
      "`src/lib/quyen.ts` chưa có hằng cho khoá ấy và lượt này không được sửa tệp đó, còn gõ " +
      "thẳng chuỗi vào màn là dựng bản sao thứ hai của một khoá phân quyền. Vì thế màn này KHÔNG " +
      "có cổng ở client — đúng khuôn màn Văn bản đang dùng cho `document.read`: `service-comms` " +
      "kiểm quyền trên TỪNG lời gọi, và tài khoản thiếu khoá nhận nguyên câu 403 của máy chủ ra " +
      "màn hình. Ẩn một nút chưa bao giờ là biện pháp (luật 5, cấm #1); thiếu nó ở đây chỉ tốn " +
      "một lần bấm.",
  },
];
