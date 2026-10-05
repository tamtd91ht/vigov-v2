/**
 * Chữ và phép quyết định của màn "Thông báo" — `docs/ui-ux/08-thong-bao.md`.
 *
 * TÁCH KHỎI COMPONENT ĐỂ KIỂM ĐƯỢC TỪNG PHÉP MỘT. Hàm thuần: không gọi mạng, không dựng DOM,
 * không đọc đồng hồ. Mọi chuỗi tiếng Việt lấy NGUYÊN VĂN từ đặc tả — một chữ khác đi trên màn của
 * cơ quan nhà nước là một chữ có người phải trả lời.
 */

import { nhanLuaChonCanBo } from "@/features/phan-anh/nhan-phieu";
import type { comms_thongBaoRa, identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";

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

// ADR 0068 §2: the `+` and `➤` glyphs of §5 are lucide icons now (`Plus`, `Send`), drawn by the
// button; the words are unchanged.
export const NHAN_NUT_SOAN = "Soạn thông báo";
export const NHAN_NUT_HUY = "Huỷ";
/** §5 — nguyên văn chữ; mũi tên `➤` của đặc tả là icon `Send` trên nút. */
export const NHAN_NUT_PHAT_HANH = "Phát hành";
/** Busy words of the publish button (spec v2 §8b). */
export const PUBLISH_BUSY_LABEL = "Đang gửi…";

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
 * §2 segmented filter `[Gửi cho tôi] [Cả sổ thông báo]`. Only the second is answerable: the first is
 * drawn disabled with its "?" (ADR 0068 §14), so `Cả sổ thông báo` is the one selected segment and
 * says what the list shows.
 */
export const SCOPE_LEGEND = "Phạm vi";
export const SCOPE_MINE_LABEL = "Gửi cho tôi";
export const SCOPE_ALL_LABEL = "Cả sổ thông báo";
/** Segment values — presentation only: the read route takes no scope parameter at all. */
export const SCOPE_MINE = "gui-cho-toi";
export const SCOPE_ALL = "ca-so";

/** §5 and §4 words of the parts drawn as placeholders (ADR 0068 §14). */
export const RECIPIENT_UNITS_FIELD_LABEL = "Bộ phận nhận thông báo";
/** The one disabled chip standing for the unit list — unit names are the commune's data, never typed here. */
export const RECIPIENT_UNITS_CHIP = "Chọn bộ phận";
export const RECIPIENT_UNITS_BLOCK_TITLE = "Bộ phận nhận";
export const SAVE_DRAFT_LABEL = "Lưu nháp";
export const WITHDRAW_LABEL = "Gỡ";
/**
 * The ACT, not the state (TB-03): beside "0/1 đã xác nhận" a disabled "Đã xác nhận" read as a fact
 * about the officer. The spec names the action "Xác nhận đã đọc"; it stays a "?" placeholder.
 */
export const ACKNOWLEDGED_CHIP = "Xác nhận đã đọc";

/**
 * The prototype's label of the named-recipient field (`AnnouncementForm`). The parenthesis is kept
 * verbatim although the unit chips are a placeholder today: it says how the two will combine.
 */
export const RECIPIENTS_EXTRA_LABEL = "Gửi thêm đích danh (ngoài các bộ phận đã chọn)";

/** §4 `NGƯỜI NHẬN (12)` — the count is the server's own `recipient_count`; only the list is missing. */
export function recipientsTitle(count: number): string {
  return `Người nhận (${count})`;
}

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

/**
 * Câu đứng dưới ô nhập người nhận — nói rõ đây là MÃ, không phải họ tên.
 *
 * Ô GÕ MÃ VẪN LÀ THỨ ĐI LÊN MÁY CHỦ: ô chọn theo họ tên ngay trên chỉ THÊM mã vào đây. Một nguồn cho
 * `recipient_codes`, không hai — và ô gõ vẫn dùng được khi danh bạ tải hỏng.
 */
export const GHI_CHU_NGUOI_NHAN =
  "Mỗi dòng một mã cán bộ, ví dụ CB-2026-7K3M9Q. Đây là mã nghiệp vụ, không phải họ tên. Chọn theo " +
  "họ tên ở ô phía trên để thêm mã vào đây, hoặc gõ thẳng mã.";

/** Nhãn ô chọn người nhận theo họ tên, và nút thêm người đã chọn vào ô mã. */
export const NHAN_CHON_NGUOI_NHAN = "Chọn cán bộ nhận thông báo";
export const CHON_NGUOI_NHAN_RONG = "— Chọn một cán bộ —";
export const NUT_THEM_NGUOI_NHAN = "Thêm vào danh sách người nhận";
export const DANG_TAI_DANH_BA = "Đang tải danh bạ cán bộ…";
/** Danh bạ chỉ gồm người có tài khoản đang hoạt động, nên rỗng là một câu trả lời thật. */
export const DANH_BA_NGUOI_NHAN_RONG =
  "Danh bạ chưa có cán bộ nào có tài khoản đang hoạt động. Vẫn gõ được mã cán bộ vào ô dưới.";

/** Danh bạ tải hỏng: câu của máy chủ nguyên văn, kèm lối còn lại. Ô gõ mã không phụ thuộc danh bạ. */
export function cauLoiDanhBa(thongBao: string): string {
  return `Không tải được danh bạ cán bộ: ${thongBao} Vẫn gõ được mã cán bộ vào ô dưới.`;
}

/**
 * Một dòng của ô chọn: `Họ tên · Chức vụ — MÃ`.
 *
 * KHÔNG CÓ EMAIL như đặc tả §5 vẽ (`Họ tên — email`): danh bạ chọn người
 * (`GET /api/v1/staff-directory`) cố ý không trả email — cùng cách ô chọn cán bộ của màn Phản ánh
 * (`nhanLuaChonCanBo`). MÃ đứng cuối vì hai người trùng họ tên và chức vụ là chuyện có thật ở một
 * xã, và mã là thứ thật sự được gửi đi.
 */
export function nhanLuaChonNguoiNhan(cb: identity_canBoChonNguoiRa): string {
  return `${nhanLuaChonCanBo(cb)} — ${cb.code}`;
}

/**
 * Thêm một mã vào ô "Gửi thêm đích danh": một dòng mới ở cuối, TRỪ KHI mã ấy đã có trong ô.
 *
 * Một mã hai lần là một người nhận hai lần trong đếm `recipient_count`. So bằng đúng phép tách
 * `tachMaNguoiNhan` mà lần gửi dùng, để "đã có" ở đây nghĩa đúng như ở thân yêu cầu.
 */
export function themMaNguoiNhan(chu: string, ma: string): string {
  const gon = ma.trim();
  if (gon === "" || tachMaNguoiNhan(chu).includes(gon)) return chu;
  const dau = chu.replace(/\s+$/, "");
  return dau === "" ? gon : `${dau}\n${gon}`;
}

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
 * phép cắt của CSS (`line-clamp`), phụ thuộc bề rộng thật của cột. The card now also applies
 * `line-clamp-2` (ADR 0068) ON TOP of this cut, so a narrow column never shows more than two lines;
 * the character cut stays because the markup — and the tests — carry the excerpt, not the CSS. Cắt
 * theo ký tự là một phép XẤP XỈ; toàn văn nằm ở cột phải, nên không có chữ nào mất hẳn khỏi màn.
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
  nhap: "Bản nháp",
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

/**
 * Dấu ghim ở đầu thẻ §3. Was the `📌` glyph alone; now a lucide `Pin` WITH this word, so the state is
 * never carried by a symbol only (ADR 0068 §2, spec §7 "icon + chữ").
 */
export const PINNED_LABEL = "Đã ghim";

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
 * NHỮNG PHẦN CỦA ĐẶC TẢ **CHƯA DỰNG ĐƯỢC** — mô tả sau dấu "?" (ADR 0068 §14)
 *
 * Mỗi phần được vẽ ĐÚNG CHỖ đặc tả đặt nó, đúng loại control, bị vô hiệu, kèm dấu "?"; bấm "?" là
 * đọc `ten` + `viSao` của mục ấy, nguyên văn. Vì thế `viSao` viết cho CÁN BỘ đọc: ngắn, không tên
 * tuyến, không tên bảng. Lý do kỹ thuật đầy đủ nằm ở chú thích ngay trên từng mục.
 *
 * `id` là khoá để màn lấy đúng mục (`pendingPart`), không lấy theo vị trí trong mảng: thêm một mục
 * ở giữa không được làm dấu "?" của nút này mở mô tả của nút khác.
 *
 * Mục KHÔNG có chỗ giữ trên màn nhưng vẫn ở đây (`tools/tien_do_san_pham.py` đếm mảng này cho báo
 * cáo tiến độ): thư điện tử trong ô chọn người nhận (chưa nằm trong bảng vị trí đã duyệt), ghim cả
 * sổ (ADR 0068 §14: lựa chọn bố cục, không có gì để giữ chỗ). Biểu mẫu soạn đã là hộp thoại
 * (ADR 0068 lần 5), nên mục "lớp phủ" đã xoá.
 *
 * MỖI MỤC PHẢI ĐÚNG VÀO NGÀY NÓ CÒN Ở ĐÂY. Dựng xong phần nào thì xoá mục ấy trong cùng lượt.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

export type PhanChuaDung = {
  /** English key the screen looks the entry up by — see `pendingPart`. */
  readonly id: string;
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  // §5 chips + §4 `BỘ PHẬN NHẬN`. `POST /api/v1/announcements` answers 501 `not_implemented` to any
  // body carrying `org_unit_ids` (service-comms/internal/http/thong_bao_noi_bo.go, re-read
  // 02/10/2026): expanding a unit into its staff is identity's data and `IdentityService` has no RPC
  // for it (rule 2). Recording the units and sending to nobody is the one outcome this module must
  // never have — a card in the book, `0/0`, and an author who believes the whole commune was told.
  {
    id: "byUnit",
    ten: "Gửi theo bộ phận",
    viSao:
      "Hệ thống chưa lấy được danh sách cán bộ của từng bộ phận, nên chưa gửi thông báo theo bộ " +
      "phận được. Hãy chọn từng người ở mục “Gửi thêm đích danh”.",
  },
  {
    id: "emailInPicker",
    ten: "Thư điện tử trong ô chọn người nhận `Họ tên — email` (§5)",
    viSao:
      "Ô chọn theo họ tên ĐÃ DỰNG, trên danh bạ chọn người `GET /api/v1/staff-directory` — mọi cán " +
      "bộ đã đăng nhập của xã đọc được, không cần `admin.user`. Danh bạ ấy cố ý chỉ trả mã, họ tên, " +
      "chức vụ và bộ phận — không số điện thoại, không email — nên mỗi dòng hiện `Họ tên · Chức vụ " +
      "— mã` thay vì `Họ tên — email`. Hiện email ở đây nghĩa là mở một tuyến trả email cho mọi " +
      "cán bộ, một thay đổi hợp đồng chứ không phải một việc của màn hình.",
  },
  // §5 `Lưu nháp`. No route creates a draft: status `nhap` exists in the schema (§6) but no write
  // path produces it — an announcement that exists is an issued one. Drawing the button and letting
  // it publish would turn "save to edit later" into "send to the whole commune".
  {
    id: "saveDraft",
    ten: "Lưu nháp",
    viSao:
      "Chưa lưu được thông báo ở dạng nháp: thông báo được tạo ra là phát hành ngay tới người " +
      "nhận. Bấm “Huỷ” sẽ bỏ phần đang soạn.",
  },
  // §2 `Gửi cho tôi`. The read route takes NO scope parameter, on purpose: "every officer reads what
  // was sent to them" needs a read key no migration seeds today (`announcement.create` is the group's
  // only key), and a key is never added by an ad-hoc `INSERT` (rule 5, invariant 3c).
  {
    id: "scopeMine",
    ten: "Bộ lọc “Gửi cho tôi”",
    viSao:
      "Lọc riêng thông báo gửi cho mình cần một quyền đọc thông báo dành cho mọi cán bộ, mà phần " +
      "mềm chưa có quyền ấy.",
  },
  // §4 `NGƯỜI NHẬN (12)`. No detail route (`GET /api/thong-bao/:id` of §7 does not exist) and the list
  // response carries two COUNTS, not the people. The list would add staff NAMES, which service-comms
  // does not own (rule 2) — an identity call, not an SQL join.
  {
    id: "recipients",
    ten: "Danh sách người nhận",
    viSao:
      "Chưa xem được từng người nhận và trạng thái của họ (chưa mở, đã mở, đã xác nhận). Hiện chỉ " +
      "có hai con số: số người nhận và số người đã xác nhận.",
  },
  // §4 `🗑 Gỡ`. No route. §9.2 forbids editing after issue ("gỡ và soạn lại"), so without it a typo has
  // no fix on this screen. Status `da-go` exists and has a chip — nothing sets it.
  {
    id: "withdraw",
    ten: "Gỡ thông báo",
    viSao:
      "Chưa gỡ được thông báo đã phát hành. Nội dung đã phát hành không sửa được, nên hiện chưa có " +
      "cách thu hồi một thông báo gõ sai trên màn này.",
  },
  // §3/§4 `✓ Đã xác nhận` of the SIGNED-IN officer, and the acknowledge / opened acts. The response
  // carries no per-viewer field — only two totals — and `…/xac-nhan`, `…/da-mo` of §7 do not exist.
  // Deriving the chip from `ack_count > 0` answers another question: "someone confirmed" is not "I did".
  {
    id: "acknowledge",
    ten: "Xác nhận đã đọc",
    viSao:
      "Chưa ghi được việc bạn xác nhận đã đọc thông báo, và hệ thống chưa cho biết bạn đã xác " +
      "nhận hay chưa — chỉ có tổng số người đã xác nhận.",
  },
  // §3 e-mail status chip. The repository has no mail sender, so `trang_thai_thu` is `chua-gui` on
  // EVERY row (service-comms/internal/domain/thong_bao_noi_bo.go; no write path sets the other three,
  // re-read 02/10/2026). The `Gửi thư điện tử` box is still drawn: `gui_thu_dien_tu` records what was
  // REQUESTED, a different fact from "mail went out".
  {
    id: "emailStatus",
    ten: "Trạng thái gửi thư",
    viSao:
      "Hệ thống chưa gửi thư điện tử, nên chưa có trạng thái “Đang gửi thư”, “Đã gửi thư” hay “Gửi " +
      "thư lỗi”. Ô “Gửi thư điện tử” hiện chỉ ghi lại yêu cầu.",
  },
  {
    id: "pinWholeBook",
    ten: "Ghim lên đầu CẢ SỔ (§3)",
    viSao:
      "`core/page` mang ĐÚNG MỘT cột sắp xếp cộng `id` phá hoà, nên `ORDER BY ghim DESC, tao_luc " +
      "DESC` không diễn đạt được bằng con trỏ của kho này; xấp xỉ nó sẽ làm trang hai vừa lặp vừa " +
      "bỏ sót hàng, im lặng. Trường `pinned` CÓ trên phản hồi, nên màn nâng thẻ ghim lên đầu " +
      "TRANG ĐANG XEM và nói rõ đó là nâng trong trang. Hệ quả thật: một thông báo ghim nằm ở " +
      "trang 3 thì nó ở đầu trang 3.",
  },
];

/**
 * The entry behind one "?" on this screen. THROWS on an unknown `id`: a renamed entry must turn the
 * screen's tests red, never open an empty description in front of an officer.
 */
export function pendingPart(id: string): PhanChuaDung {
  const entry = PHAN_CHUA_DUNG.find((p) => p.id === id);
  if (entry === undefined) throw new Error(`nhan-thong-bao: PHAN_CHUA_DUNG has no entry "${id}"`);
  return entry;
}
