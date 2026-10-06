/**
 * Chữ và phép quyết định của màn "Nội dung Mini App" — `docs/ui-ux/11-noi-dung-mini-app.md`.
 *
 * TÁCH KHỎI COMPONENT ĐỂ KIỂM ĐƯỢC TỪNG PHÉP MỘT. Hàm thuần: không gọi mạng, không dựng DOM,
 * không đọc đồng hồ. Mọi chuỗi tiếng Việt lấy NGUYÊN VĂN từ đặc tả — một chữ khác đi trên màn của
 * cơ quan nhà nước là một chữ có người phải trả lời.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * ⚠ ĐIỀU QUAN TRỌNG NHẤT CỦA MÀN NÀY: KHÔNG DÒNG NÀO Ở ĐÂY ĐƯA MỘT CHUỖI HTML VÀO TRANG.
 *
 * `noi_dung` is HTML (§8). Since ADR 0067 §1 the SERVER sanitises it on every write path to the
 * allow-list p · br · strong · em · ul · ol · li · h2 · h3 · a[https] (`service-comms/internal/
 * richtext`), and the Mini App never renders HTML at all — it gets `body_blocks`. This screen edits
 * the body in Tiptap (`rich-text.ts`), whose schema is that same list; the editor draws the document
 * from the parsed structure, so no `dangerouslySetInnerHTML` is needed and none is allowed.
 *
 * Lệnh cấm ấy có phép kiểm riêng đọc thẳng mã nguồn — `ranh-gioi-html.test.ts`. Một lệnh cấm
 * không có phép kiểm là một lệnh cấm sẽ bị phá trong im lặng.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */

import { coTrangTruoc, veTrangTruoc, type NganXepConTro } from "@/features/cau-hinh/ngan-xep-con-tro"; // vi-name-ok: existing exports of the pager (rule 12 inv 3)
import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import {
  CONTENT_TYPE_BANNER,
  CONTENT_TYPE_EVENT,
  CONTENT_TYPE_VIDEO,
  type UpdateCategoryIn,
} from "@/lib/api/noi-dung";
import type {
  comms_danhMucRa,
  comms_noiDungRa,
  comms_suaNoiDungVao,
  comms_themNoiDungVao,
} from "@/lib/api/schema.gen";
import { QUYEN_CONG_KHAI_DANH_BA, quyetDinhTheoKhoa } from "@/lib/quyen";

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

/**
 * Under `Tóm tắt`: the owner's 03/10/2026 decision (ADR 0067 §Sửa đổi) — the summary IS the article's
 * sapo, shown bold under the title in the Mini App. No second field for it.
 */
export const SUMMARY_SAPO_HINT = "Đoạn mở đầu (sapo) — Mini App in đậm ngay dưới tiêu đề bài.";
export const TEN_DANH_MUC_TOI_DA = 200;
export const SLUG_DANH_MUC_TOI_DA = 64;
export const THU_TU_DANH_MUC_TOI_DA = 9999;
/**
 * `domain.EventPlaceMaxRunes`: in CHARACTERS (Go counts runes, the CHECK uses `char_length`), so it
 * is counted in code points below, not in UTF-16 units — `.length` would disagree with the server on
 * any character outside the BMP.
 */
export const EVENT_PLACE_MAX_CHARS = 500;
/** `domain.LinkToMaxRunes` — characters, as migration 0012's CHECK counts. */
export const LINK_TO_MAX_CHARS = 500;
/** The INT column's upper bound for `display_order` (`displayOrderMax`) — not a customer figure. */
export const DISPLAY_ORDER_MAX = 2147483647;
/**
 * `domain.LyDoXoaToiDa` — the soft-delete reason of a category AND of a content item (the same
 * `domain.ChuanHoaLyDoXoa`, 500 RUNES after trimming): counted in code points, see `deleteReasonLength`.
 */
export const DELETE_REASON_MAX_CHARS = 500;

/* ── Chữ trên màn ──────────────────────────────────────────────────────────────────────────── */

/** §3 của chương — tiêu đề trang, nguyên văn. */
export const TIEU_DE_MAN = "Quản trị nội dung Mini App";

/** §1 — nguyên văn câu mô tả trong giao diện. */
export const MO_TA_MAN =
  "Tin tức, sự kiện, thông báo, bản tin truyền thanh và video hiển thị cho bà con trên Zalo " +
  "Mini App.";

export const NHAN_NUT_THEM = "+ Thêm nội dung";
export const NHAN_NUT_DANH_MUC = "⊞ Danh mục tin";
/**
 * The FIRST sentence under the prototype's `Danh mục tin` dialog title (`CategoryManagerDialog.tsx`).
 * Its second sentence ("categories fetched from the portal update themselves on every sync") is left
 * out: here the portal sync maps portal categories onto content TYPES (ADR 0067 §2), it does not write
 * this catalogue — printing it would tell the commune something this system does not do.
 */
export const CATEGORY_DIALOG_DESCRIPTION = "Bà con lọc tin theo danh mục này trên Mini App.";
export const NHAN_NUT_HUY = "Huỷ";
export const NHAN_NUT_LUU = "Lưu";
/** §6 — the edit symbol of the action column. */
export const NHAN_NUT_SUA = "✎";
/** The way out of an overlay that holds no form of its own (the category manager, a loading edit). */
export const CLOSE_LABEL = "Đóng";
/** The edit overlay's heading while the full text is still loading — the title is not known yet. */
export const EDIT_FORM_TITLE = "Sửa nội dung";

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

/** §4 — the card's title, verbatim. */
export const TIEU_DE_THE_DANH_BA = "Danh bạ chính quyền";
/**
 * The card's line while the count is loading or could not be read: no number in it. A `0` shown
 * then would tell the commune that residents see nobody.
 */
export const MO_TA_THE_DANH_BA =
  "Chọn thêm hoặc bớt cán bộ hiện cho bà con ở ngăn Danh bạ cán bộ.";
/** Said under the line when the count could not be read — a fact, never a guessed figure. */
export const PUBLISHED_STAFF_UNREAD = "Chưa đọc được số cán bộ đang hiện cho bà con.";

/** §4 verbatim, with the count the server returned: `Đang hiện 26 cán bộ cho bà con. …` */
export function publishedStaffLine(n: number): string {
  return `Đang hiện ${n} cán bộ cho bà con. Chọn thêm hoặc bớt ở ngăn Danh bạ cán bộ.`;
}

/**
 * The card's text for the three states of the count. `null` = still loading.
 *
 * ONLY A SUCCESSFUL READ CARRIES A NUMBER. Loading and failure both give the line without one, and a
 * failure says so underneath. A `0` here on success is true (nobody published yet); a `0` on failure
 * would be a lie about a public channel.
 */
export function publishedStaffText(count: KetQua<number> | null): {
  readonly line: string;
  readonly note: string | null;
} {
  if (count === null) return { line: MO_TA_THE_DANH_BA, note: null };
  if (!count.ok) return { line: MO_TA_THE_DANH_BA, note: PUBLISHED_STAFF_UNREAD };
  return { line: publishedStaffLine(count.duLieu), note: null };
}

/* ── Permission: who sees the write controls ──────────────────────────────────────────────────── */

/**
 * `content.update` — the key every write route of this screen declares (`lib/api/noi-dung.ts` header,
 * `lib/api/portal-sync.ts` header; seeded at `service-identity/migrations/0001_init.sql:293`, "Sửa nội
 * dung và danh bạ Mini App").
 *
 * AN ALIAS, NOT A SECOND LITERAL: `lib/quyen.ts` already holds this string as `QUYEN_CONG_KHAI_DANH_BA`
 * (named for the directory screen, where it was first gated). One key, one literal — a second copy of
 * the string is the copy that drifts the day the key is split.
 */
export const CONTENT_UPDATE_PERMISSION = QUYEN_CONG_KHAI_DANH_BA;

/**
 * Whether this screen draws its WRITE controls: `+ Thêm nội dung`, `✎`, `⊞ Danh mục tin` (the
 * category tree and its edit/hide/delete), `⟳ Đồng bộ ngay` and `Cấu hình` of the portal sync card.
 * The prototype gates the same set (`vigov-require/.../ContentWorkspace.tsx:99,174,272,447`).
 *
 * FAIL CLOSED, AND NO FLASH: a session not read yet (`null`) or read with an error is "no". The
 * controls can therefore only APPEAR once the session is in, never appear and then vanish.
 *
 * CONVENIENCE, NOT PROTECTION: `service-comms` checks `content.update` on every write (rule 5,
 * forbidden #1). The READ key `content.read` gets no gate here on purpose — the menu item is gated by
 * it (`QUYEN_XEM_NOI_DUNG`), and an account without it gets the server's own 403 sentence on the list,
 * the same pattern as `/van-ban`.
 */
export function canEditContent(session: PhienDaDoc): boolean {
  return session !== null && quyetDinhTheoKhoa(session, CONTENT_UPDATE_PERMISSION).hien;
}

/**
 * Sổ rỗng. Đặc tả không có câu nào cho trạng thái này, nên câu dưới là câu viết mới, cùng giọng
 * với các màn đã có — và điều ấy được nói ra ở đây thay vì để người sau tưởng mình đọc lại một câu
 * của đặc tả.
 */
export const SO_RONG = "Chưa có nội dung nào trong lát cắt đang xem.";
export const DANH_MUC_RONG = "Xã chưa có danh mục tin nào.";

export const DANG_TAI_SO = "Đang tải danh sách nội dung…";
export const DANG_TAI_TOAN_VAN = "Đang tải toàn văn bài viết…";

/* ── Câu cảnh báo phải ĐỨNG CẠNH ô nhập, không nằm trong chú thích mã ─────────────────────── */

/**
 * The one line under §7's `Nội dung` box (ADR 0067 §1). It says what survives and WHERE that is
 * decided: a paste from Word keeps only these, and the server — not this screen — enforces it.
 */
export const CANH_BAO_HTML_THO =
  "Giữ được: đoạn văn, tiêu đề lớn/nhỏ, chữ đậm, chữ nghiêng, danh sách và liên kết https. Máy chủ " +
  "làm sạch thân bài mỗi lần lưu — ảnh, bảng, màu chữ và mọi định dạng khác bị bỏ trước khi tới bà con.";

/* ── Xoá một mục nội dung — soft delete with a mandatory reason (rule 7) ─────────────────────────
 *
 * The prototype's trash button (`vigov-require/.../ContentWorkspace.tsx:469-477`, user's decision
 * 02/10/2026), with one deviation rule 7 mandates: the prototype deletes on one click without a reason;
 * here a dialog asks for the reason first, and the server refuses a blank one anyway.
 */

/** The action column's header: it now holds `✎` and `🗑`. */
export const ACTIONS_COLUMN_LABEL = "Thao tác";
/** The row's delete symbol; the accessible name carries the title (`contentDeleteAriaLabel`). */
export const CONTENT_DELETE_SYMBOL = "🗑";
export const CONTENT_DELETE_TITLE = "Xoá nội dung";
export const CONTENT_DELETE_SUBMIT = "Xoá";
export const CONTENT_DELETE_REASON_LABEL = "Lý do xoá *";
/** Said in the dialog, before the reason: what the act does and what it keeps. */
export const CONTENT_DELETE_NOTE =
  "Nội dung sẽ được gỡ khỏi Mini App và không còn trong danh sách. Hồ sơ, lý do xoá và lịch sử " +
  "thao tác vẫn được lưu lại. Nếu chỉ muốn tạm ẩn với bà con, hãy sửa nội dung và bỏ chọn “Đăng " +
  "lên Mini App”.";
/** Shown above the table after a 204 — the prototype's toast, worded for a delete. */
export const CONTENT_DELETED = "Đã xoá khỏi Mini App.";
/**
 * Shown above the table after a 404. The server gives ONE answer for "already deleted", "another
 * commune's" and "never existed" (rule 4, forbidden #2), so this sentence guesses no further than that.
 */
export const CONTENT_DELETE_GONE =
  "Nội dung này không còn trong sổ của xã — có thể cán bộ khác đã xoá. Danh sách đã được tải lại.";

export function contentDeleteAriaLabel(title: string): string {
  return `Xoá: ${title}`;
}

/**
 * Length of a reason AS THE SERVER COUNTS IT: trimmed, in code points (Go runes) — `.length` counts
 * UTF-16 units and would disagree on any character outside the BMP.
 */
export function deleteReasonLength(reason: string): number {
  return [...reason.trim()].length;
}

/** Whether `Xoá` may be pressed: a non-blank reason within the server's bound. */
export function deleteReasonReady(reason: string): boolean {
  const n = deleteReasonLength(reason);
  return n > 0 && n <= DELETE_REASON_MAX_CHARS;
}

/**
 * The page to show after a delete. The CURRENT page is read again (not the first: the officer stays
 * where they were) — unless the deleted row was the last one on it, then one page back, so the screen
 * does not land on an empty page with rows still before it. On the first page it stays put.
 */
export function pageAfterDelete(stack: NganXepConTro, rowsOnPage: number): NganXepConTro {
  return rowsOnPage <= 1 && coTrangTruoc(stack) ? veTrangTruoc(stack) : stack;
}

/** The live counter under the reason box: `12/500`. */
export function deleteReasonCounter(reason: string): string {
  return `${deleteReasonLength(reason)}/${DELETE_REASON_MAX_CHARS}`;
}

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

/** The status code the portal sync's review mode puts imports in — the `Chờ duyệt` queue. */
export const STATUS_PENDING_REVIEW = "cho-duyet";

/**
 * §6 `Trạng thái` filter, in the order the owner named it (02/10/2026): `Tất cả` (no `status` sent),
 * then the three codes the server accepts. The server refuses any other code with a 400.
 */
export const STATUS_FILTER_OPTIONS: readonly { value: string; label: string }[] = [
  { value: "", label: "Tất cả" },
  { value: "dang-hien", label: "Đang hiện" },
  { value: "an", label: "Ẩn" },
  { value: STATUS_PENDING_REVIEW, label: "Chờ duyệt" },
];

/** Under the source line of a synced row, and in the detail: the portal category it came in under. */
export function portalCategoryLabel(name: string): string {
  return `Chuyên mục Cổng: ${name}`;
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
 * `dong-bo-cong` is written by the portal sync (ADR 0067 §2, service-comms fa7b8377). §6 is the only
 * screen that tells the commune which items came from its portal, so the table shows this label under
 * the title of every synced row (`BangNoiDung`), and the edit form's read-only block shows it too.
 */
export function nhanNguon(ma: string): string {
  return NHAN_NGUON[ma] ?? ma;
}

/** The source code the portal sync writes (migration 0006 `nguon`). */
export const SOURCE_PORTAL_SYNC = "dong-bo-cong";

/**
 * Shown on the edit form of an item waiting for approval (`cho-duyet` — where the portal sync puts
 * imports in its default mode). Publishing IS the existing tick: `thanSua` sends `publish: true` only
 * when the officer ticks it, and the server turns the item `dang-hien`.
 */
export const PENDING_REVIEW_HINT =
  "Bài này đang chờ duyệt — bà con chưa thấy. Tích “Đăng lên Mini App” rồi Lưu để đăng cho bà con.";

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

/* ── Event instants: the datetime-local input ↔ RFC 3339 with Vietnam's offset ────────────────
 *
 * ⚠ THE BROWSER'S TIME ZONE IS NEVER CONSULTED. A `datetime-local` input yields a wall-clock string
 * with no zone ("2026-10-05T08:00"). Feeding it to `new Date(...)` reads it in the MACHINE's zone, so
 * the same form typed on a laptop set to UTC would publish an event seven hours late on every
 * resident's phone — and look right on the author's screen. The commune's clock is Vietnam's.
 *
 * Asia/Ho_Chi_Minh is a fixed +07:00 with no daylight saving, so the offset is a constant and the
 * conversion is string work, not a zone lookup. Like `MUI_GIO` this is a platform constant, not a
 * per-commune value: every commune is in the same zone.
 */
const VIETNAM_OFFSET = "+07:00";

const LOCAL_INPUT_PATTERN = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})(:\d{2})?$/;

/**
 * Formatter for reading an instant back in Vietnam's wall-clock time. `hourCycle: "h23"` rather than
 * `hour12: false`: the latter prints midnight as `24` on some ICU builds.
 */
const VIETNAM_PARTS = new Intl.DateTimeFormat("en-GB", {
  timeZone: MUI_GIO,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

/**
 * `datetime-local` value → RFC 3339 with `+07:00`. `""` stays `""` (none / clear).
 *
 * A value not in the input's shape is returned unchanged: `validateTypeFields` refuses it before a
 * save, and if one ever got through the server answers with its own sentence rather than this code
 * guessing a time.
 */
export function localInputToInstant(value: string): string {
  if (value === "") return "";
  const m = LOCAL_INPUT_PATTERN.exec(value);
  if (m === null) return value;
  return `${m[1]}${m[2] ?? ":00"}${VIETNAM_OFFSET}`;
}

/** An instant from the server (UTC) → the `datetime-local` value it is in Vietnam. `""` if none. */
export function instantToLocalInput(instant: string | null | undefined): string {
  if (instant === null || instant === undefined || instant === "") return "";
  const t = Date.parse(instant);
  if (Number.isNaN(t)) return "";
  const p = VIETNAM_PARTS.formatToParts(new Date(t));
  return (
    `${phanCua(p, "year")}-${phanCua(p, "month")}-${phanCua(p, "day")}` +
    `T${phanCua(p, "hour")}:${phanCua(p, "minute")}`
  );
}

/**
 * An instant → `dd/MM/yyyy HH:mm` in Vietnam. Unreadable strings are shown as they are, for the same
 * reason as `nhanMoc`.
 */
export function formatVietnamDateTime(instant: string): string {
  const t = Date.parse(instant);
  if (Number.isNaN(t)) return instant;
  const p = VIETNAM_PARTS.formatToParts(new Date(t));
  return (
    `${phanCua(p, "day")}/${phanCua(p, "month")}/${phanCua(p, "year")} ` +
    `${phanCua(p, "hour")}:${phanCua(p, "minute")}`
  );
}

/**
 * "Đăng lần đầu lúc …", or `null` when the item has never been published.
 *
 * `published_at` is fixed by the FIRST publication and never moves (ADR 0047 §6, G1): unpublishing and
 * republishing keeps it. That is why the label says "lần đầu" — it is not "the time it went live
 * again", and a reader of §6 must not take it for that.
 */
export function publishedAtLabel(nd: comms_noiDungRa): string | null {
  if (nd.published_at === undefined || nd.published_at === null || nd.published_at === "") {
    return null;
  }
  return `Đăng lần đầu lúc ${formatVietnamDateTime(nd.published_at)}`;
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

/** §6 column and detail row label for `view_count`. */
export const VIEW_COUNT_LABEL = "Lượt xem";

const VIEW_COUNT_FORMAT = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 0 });

/**
 * `view_count` → `1.234`. READ-ONLY: the server counts each open of the article in the Mini App
 * (ADR 0047, row 02/10/2026); the screen never sends it back (`thanThem`/`thanSua`). `0` prints `0`,
 * never a dash: the server always sends the key, and "nobody opened it yet" is a real answer.
 */
export function viewCountText(n: number): string {
  return VIEW_COUNT_FORMAT.format(n);
}

/**
 * Tóm tắt cho dòng phụ §6 (*"tóm tắt cắt 1 dòng"*).
 *
 * NOT CUT HERE ANY MORE (02/10/2026). "1 dòng" is a cut by the column's real width, so it is CSS's
 * (`.summary-one-line`, `line-clamp: 1` in `globals.css`); a character count could only approximate it —
 * too long on a narrow screen, too short on a wide one. The whole summary stays in the page, so a screen
 * reader reads all of it, and the edit form's `Tóm tắt` box holds it in full.
 *
 * Runs of whitespace (line breaks included) still become one space: a multi-line summary squeezed into
 * one line would otherwise glue the last word of a line to the first word of the next.
 */
export function trichTomTat(tomTat: string): string {
  return tomTat.replace(/\s+/g, " ").trim();
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

/**
 * Bảy ô của §7, cộng bốn ô theo loại của :131 (ADR 0047 §6): ba ô cho `Sự kiện`, một cho `Video`.
 *
 * `Ảnh đại diện` IS THE UPLOADED COVER'S ID (`cover_image_file_id`), not a link. The legacy
 * `image_url` link has no box any more and is never sent from this form: the server keeps whatever
 * a row already holds (absent on a PATCH = leave alone), and the read-only block shows it as text.
 *
 * THE TWO EVENT INSTANTS ARE HELD AS THE INPUT'S OWN STRING ("2026-10-05T08:00", Vietnam wall-clock
 * time), not as the wire instant: the box shows exactly what was typed, and the conversion to
 * `+07:00` happens once, in `thanThem` / `thanSua`.
 *
 * A HIDDEN BOX KEEPS ITS VALUE. Switching `Sự kiện` → `Tin tức` → `Sự kiện` before saving gives the
 * dates back; the builders never send a field the chosen type cannot carry, so a hidden value cannot
 * leave the form.
 */
export type GiaTriFormNoiDung = {
  readonly type: string;
  readonly category_id: string;
  readonly title: string;
  readonly summary: string;
  readonly body: string;
  /**
   * §7 `Ảnh đại diện`: the id of a `ready` cover upload, `""` = none. On the edit form it starts as the
   * article's current cover; a new upload replaces it, `Gỡ ảnh` empties it.
   */
  readonly cover_image_file_id: string;
  /** §7 `☐ Đăng lên Mini App`. */
  readonly publish: boolean;
  /** `Sự kiện` — `Bắt đầu`, a `datetime-local` value. `""` = none. */
  readonly event_starts_local: string;
  /** `Sự kiện` — `Kết thúc`, a `datetime-local` value. `""` = none. */
  readonly event_ends_local: string;
  /** `Sự kiện` — `Địa điểm`. */
  readonly event_place: string;
  /** `Video` — `Liên kết video`. */
  readonly video_url: string;
  /** `Banner` — the tap target: `/…` inside the Mini App or `https://…`. `""` = not tappable. */
  readonly link_to: string;
  /** `Banner` — `Thứ tự hiển thị`, AS TYPED (a number box gives a string). `""` = none. */
  readonly display_order: string;
};

/** Biểu mẫu trống của §7 — loại mặc định `Tin tức`, ô tích TẮT. */
export const FORM_TRONG: GiaTriFormNoiDung = {
  type: LOAI_MAC_DINH,
  category_id: "",
  title: "",
  summary: "",
  body: "",
  cover_image_file_id: "",
  publish: false,
  event_starts_local: "",
  event_ends_local: "",
  event_place: "",
  video_url: "",
  link_to: "",
  display_order: "",
};

/* ── Client-side check of the per-type fields ─────────────────────────────────────────────────
 *
 * MIRRORS the server (`service-comms/internal/domain/noi_dung_mini_app.go`: CheckEventWindow,
 * NormalizeEventPlace, ChuanHoaURL) so the author sees the problem next to the box instead of after
 * Lưu. The server is still where the refusal is real, and its 400 sentence is shown as it comes.
 */
export const ERR_EVENT_END_WITHOUT_START = "Có thời gian kết thúc thì phải có thời gian bắt đầu.";
export const ERR_EVENT_END_BEFORE_START =
  "Thời gian kết thúc không được sớm hơn thời gian bắt đầu.";
export const ERR_EVENT_TIME_INVALID = "Thời gian bắt đầu hoặc kết thúc không hợp lệ.";
export const ERR_EVENT_PLACE_TOO_LONG = `Địa điểm tối đa ${EVENT_PLACE_MAX_CHARS} ký tự.`;
export const ERR_EVENT_PLACE_CONTROL_CHAR = "Địa điểm không được chứa ký tự điều khiển.";
export const ERR_VIDEO_URL_INVALID =
  "Liên kết video phải bắt đầu bằng http:// hoặc https:// và không chứa dấu cách.";
export const ERR_VIDEO_URL_TOO_LONG = `Liên kết video tối đa ${URL_TOI_DA} ký tự.`;

export const ERR_LINK_TO_INVALID =
  "Liên kết khi bấm phải là đường trong Mini App (bắt đầu bằng một dấu /) hoặc địa chỉ https://, " +
  `tối đa ${LINK_TO_MAX_CHARS} ký tự, không có dấu cách.`;
export const ERR_DISPLAY_ORDER_INVALID = "Thứ tự hiển thị phải là số nguyên không âm.";
export const ERR_DISPLAY_ORDER_CANNOT_CLEAR =
  "Banner đã có thứ tự hiển thị thì không bỏ trống được nữa — hãy nhập một số không âm.";
export const ERR_BANNER_COVER_REQUIRED =
  "Banner bắt buộc có Ảnh đại diện — hãy tải ảnh lên trước khi lưu.";

/** Under the banner boxes: the picture IS the banner, and the title is what a screen reader says. */
export const BANNER_COVER_NOTICE =
  "Banner là một tấm ảnh ở dải đầu trang chủ Mini App: bắt buộc có Ảnh đại diện, và tiêu đề được " +
  "đọc thay cho ảnh với người dùng trình đọc màn hình.";
export const LINK_TO_HINT =
  "Để trống thì banner không bấm được. Đường trong Mini App bắt đầu bằng /, ví dụ /tin-tuc; trang " +
  "bên ngoài phải bắt đầu bằng https://.";
export const DISPLAY_ORDER_HINT = "Số nhỏ hiện trước trên dải banner. Để trống nếu chưa cần xếp thứ tự.";

/** The hint under the two event boxes — says which clock they are read in. */
export const EVENT_TIME_HINT =
  "Giờ Việt Nam (GMT+7), không phụ thuộc múi giờ đặt trên máy tính này.";
/** The hint under the video box. */
export const VIDEO_URL_HINT =
  "Chỉ nhận địa chỉ bắt đầu bằng http:// hoặc https://. Bà con bấm vào sẽ mở video ở ngoài ứng dụng.";

const CONTROL_CHAR = /[\u0000-\u001f\u007f-\u009f]/;
const CONTROL_OR_SPACE = /[\s\u0000-\u001f\u007f-\u009f]/;
const CONTROL_SPACE_OR_BACKSLASH = /[\s\u0000-\u001f\u007f-\u009f\\]/;

/**
 * A banner's `link_to`, checked as `domain.NormalizeLinkTo` does: `""` is fine (not tappable); an in-app
 * path is `/` NOT followed by a second `/` (`//host` leaves the app); otherwise `https://` + a host with
 * no `@` (a userinfo prefix would make `https://gov.vn@other.example` read as a government link).
 */
export function isValidLinkTo(raw: string): boolean {
  const s = raw.trim();
  if (s === "") return true;
  if ([...s].length > LINK_TO_MAX_CHARS) return false;
  if (CONTROL_SPACE_OR_BACKSLASH.test(s)) return false;
  if (s.startsWith("/")) return !s.startsWith("//");
  const https = "https://";
  if (s.length <= https.length || s.slice(0, https.length).toLowerCase() !== https) return false;
  const rest = s.slice(https.length);
  const end = rest.search(/[/?#]/);
  const host = end < 0 ? rest : rest.slice(0, end);
  return host !== "" && !host.includes("@");
}

/**
 * The in-app paths the citizen Mini App can OPEN from a banner — a COPY of the keys of `BANNER_ROUTES` in
 * `citizen-app/src/cong-dan/man/TrangXa.tsx`, plus its `/tin-tuc/<id>` article form (`bannerScreen`).
 *
 * WHY A COPY, AND WHAT KEEPS IT HONEST: the two apps share no package, and the server accepts ANY in-app
 * path (`domain.NormalizeLinkTo`) — the closed table lives in the Mini App. `nhan-noi-dung.test.ts` reads
 * that file and turns red when its table and this one differ. These are SUGGESTIONS and a warning only:
 * a path outside the table is still saved, it is just not tappable on the Mini App today.
 */
export const BANNER_APP_PATHS: readonly { readonly path: string; readonly label: string }[] = [
  { path: "/tin-tuc", label: "Tab Tin tức" },
  { path: "/phan-anh", label: "Tab Phản ánh" },
  { path: "/ca-nhan", label: "Tab Cá nhân" },
  { path: "/gui-phan-anh", label: "Gửi phản ánh" },
  { path: "/danh-ba", label: "Danh bạ chính quyền" },
  { path: "/su-kien", label: "Sự kiện" },
  { path: "/truyen-thanh", label: "Truyền thanh" },
  { path: "/video", label: "Video" },
];

export const BANNER_ARTICLE_PATH_HINT =
  "Gợi ý: chọn một đường có sẵn trong danh sách, hoặc /tin-tuc/<mã bài> để mở thẳng một bài.";
export const BANNER_PATH_NOT_TAPPABLE =
  "Mini App chưa có màn nào cho đường này — banner vẫn lưu được, nhưng bà con bấm vào sẽ không mở gì. " +
  "Hãy chọn một đường trong danh sách gợi ý hoặc /tin-tuc/<mã bài>.";

/** Mirror of the Mini App's `bannerScreen`: is this in-app path one it opens? */
export function isMiniAppBannerPath(path: string): boolean {
  if (!path.startsWith("/") || path.startsWith("//") || /[?#\s\\]/.test(path)) return false;
  const p = path.length > 1 && path.endsWith("/") ? path.slice(0, -1) : path;
  if (BANNER_APP_PATHS.some((r) => r.path === p)) return true;
  const article = /^\/tin-tuc\/([^/]+)$/.exec(p);
  if (article === null) return false;
  try {
    return decodeURIComponent(article[1] ?? "").trim() !== "";
  } catch {
    return false; // a malformed %-escape
  }
}

/**
 * The warning under `Liên kết khi bấm`, or `null`. Only for a VALID in-app path the Mini App cannot open:
 * an invalid value already has its refusal (`ERR_LINK_TO_INVALID`), and https:// / empty are fine.
 */
export function bannerLinkTapWarning(raw: string): string | null {
  const s = raw.trim();
  if (s === "" || !s.startsWith("/") || !isValidLinkTo(s)) return null;
  return isMiniAppBannerPath(s) ? null : BANNER_PATH_NOT_TAPPABLE;
}

/** `display_order` as typed → the number, or `null` when it is not a non-negative INT. */
export function parseDisplayOrder(raw: string): number | null {
  const s = raw.trim();
  if (!/^\d+$/.test(s)) return null;
  const n = Number(s);
  return n <= DISPLAY_ORDER_MAX ? n : null;
}

/**
 * The first problem with the per-type fields of the CHOSEN type, or `null`.
 *
 * Only the chosen type is checked: the other type's boxes are hidden and never sent, so an error on
 * them would block a save for a value that cannot leave the form.
 *
 * `before` is the edit form's starting values (absent on the create form). The banner rules need it:
 * `display_order` cannot be cleared by a PATCH, and the cover rule is migration 0012's trigger, which
 * lets a LEGACY banner that already had no cover be edited (`domain.CheckBannerCover`).
 */
export function validateTypeFields(
  gt: GiaTriFormNoiDung,
  before?: GiaTriFormNoiDung,
): string | null {
  if (gt.type === CONTENT_TYPE_EVENT) {
    const start = gt.event_starts_local;
    const end = gt.event_ends_local;
    if (
      (start !== "" && !LOCAL_INPUT_PATTERN.test(start)) ||
      (end !== "" && !LOCAL_INPUT_PATTERN.test(end))
    ) {
      return ERR_EVENT_TIME_INVALID;
    }
    if (end !== "" && start === "") return ERR_EVENT_END_WITHOUT_START;
    // Both normalised to the same shape and the same offset, so the strings order exactly as the
    // instants do.
    if (end !== "" && localInputToInstant(end) < localInputToInstant(start)) {
      return ERR_EVENT_END_BEFORE_START;
    }
    const place = gt.event_place.trim();
    if ([...place].length > EVENT_PLACE_MAX_CHARS) return ERR_EVENT_PLACE_TOO_LONG;
    if (CONTROL_CHAR.test(place)) return ERR_EVENT_PLACE_CONTROL_CHAR;
  }
  if (gt.type === CONTENT_TYPE_VIDEO) {
    const url = gt.video_url.trim();
    if (url !== "") {
      // Bytes, as the server counts (`len(u)` in Go).
      if (new TextEncoder().encode(url).length > URL_TOI_DA) return ERR_VIDEO_URL_TOO_LONG;
      const lower = url.toLowerCase();
      if (!lower.startsWith("http://") && !lower.startsWith("https://")) {
        return ERR_VIDEO_URL_INVALID;
      }
      if (CONTROL_OR_SPACE.test(url)) return ERR_VIDEO_URL_INVALID;
    }
  }
  if (gt.type === CONTENT_TYPE_BANNER) {
    if (!isValidLinkTo(gt.link_to)) return ERR_LINK_TO_INVALID;
    const order = gt.display_order.trim();
    if (order !== "" && parseDisplayOrder(order) === null) return ERR_DISPLAY_ORDER_INVALID;
    const wasBanner = before !== undefined && before.type === CONTENT_TYPE_BANNER;
    if (order === "" && wasBanner && before.display_order !== "") {
      return ERR_DISPLAY_ORDER_CANNOT_CLEAR;
    }
    const legacyCoverless = wasBanner && before.cover_image_file_id === "";
    if (gt.cover_image_file_id === "" && !legacyCoverless) return ERR_BANNER_COVER_REQUIRED;
  }
  return null;
}

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
    cover_image_file_id: nd.cover_image_file_id ?? "",
    publish: nd.status === "dang-hien",
    event_starts_local: instantToLocalInput(nd.event_starts_at),
    event_ends_local: instantToLocalInput(nd.event_ends_at),
    event_place: nd.event_place ?? "",
    video_url: nd.video_url ?? "",
    link_to: nd.link_to ?? "",
    display_order:
      nd.display_order === undefined || nd.display_order === null ? "" : String(nd.display_order),
  };
}

/**
 * Thân `POST` từ biểu mẫu. Các trường của §7, cộng các trường theo loại CHỈ KHI loại đã chọn mang
 * được chúng VÀ chúng có giá trị — gửi một trường sự kiện cho `Tin tức` là 400 ở máy chủ.
 *
 * `cover_image_file_id` only when a cover was uploaded; no `image_url` (no box for it).
 */
export function thanThem(gt: GiaTriFormNoiDung): comms_themNoiDungVao {
  const ra: comms_themNoiDungVao = {
    type: gt.type,
    title: gt.title.trim(),
    category_id: gt.category_id,
    summary: gt.summary.trim(),
    body: gt.body,
    publish: gt.publish,
  };
  if (gt.cover_image_file_id !== "") ra.cover_image_file_id = gt.cover_image_file_id;
  if (gt.type === CONTENT_TYPE_EVENT) {
    const start = localInputToInstant(gt.event_starts_local);
    const end = localInputToInstant(gt.event_ends_local);
    const place = gt.event_place.trim();
    if (start !== "") ra.event_starts_at = start;
    if (end !== "") ra.event_ends_at = end;
    if (place !== "") ra.event_place = place;
  }
  if (gt.type === CONTENT_TYPE_VIDEO) {
    const url = gt.video_url.trim();
    if (url !== "") ra.video_url = url;
  }
  if (gt.type === CONTENT_TYPE_BANNER) {
    const link = gt.link_to.trim();
    const order = parseDisplayOrder(gt.display_order);
    if (link !== "") ra.link_to = link;
    if (order !== null) ra.display_order = order;
  }
  return ra;
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
  if (moi.publish !== dau.publish) ra.publish = moi.publish;
  // The cover: only when it changed. A new upload sends its id; `Gỡ ảnh` on an article that had one
  // sends `""` — the server's spelling of DETACH. Untouched = absent = leave alone.
  if (moi.cover_image_file_id !== dau.cover_image_file_id) {
    ra.cover_image_file_id = moi.cover_image_file_id;
  }

  // THE PER-TYPE FIELDS: only for the type AFTER the edit, only when changed, and an emptied box is
  // sent as `""` — the server's one spelling of "clear". A type moved away from `su-kien` / `video`
  // sends none of them: the server clears that type's columns itself, and a hidden box's leftover
  // value must not ride along (it would be a 400).
  if (moi.type === CONTENT_TYPE_EVENT) {
    if (moi.event_starts_local !== dau.event_starts_local) {
      ra.event_starts_at = localInputToInstant(moi.event_starts_local);
    }
    if (moi.event_ends_local !== dau.event_ends_local) {
      ra.event_ends_at = localInputToInstant(moi.event_ends_local);
    }
    if (moi.event_place.trim() !== dau.event_place) ra.event_place = moi.event_place.trim();
  }
  if (moi.type === CONTENT_TYPE_VIDEO && moi.video_url.trim() !== dau.video_url) {
    ra.video_url = moi.video_url.trim();
  }
  // Banner: same shape. `link_to: ""` is the server's "not tappable any more". `display_order` has no
  // clear on the wire, so an emptied box sends nothing — `validateTypeFields` holds Lưu with a sentence
  // instead of letting the officer believe the order was removed. A type moved away from `banner` sends
  // neither: the server clears both itself, and sending one would be a 422.
  if (moi.type === CONTENT_TYPE_BANNER) {
    if (moi.link_to.trim() !== dau.link_to) ra.link_to = moi.link_to.trim();
    const order = parseDisplayOrder(moi.display_order);
    if (order !== null && moi.display_order.trim() !== dau.display_order) ra.display_order = order;
  }

  return ra;
}

/** Có gì để gửi không. Không có thì màn nói ra, thay vì gọi một `PATCH` rỗng. */
export function coThayDoi(than: Record<string, unknown>): boolean {
  return Object.keys(than).length > 0;
}

/** Câu hiện khi bấm Lưu mà không ô nào đổi. */
export const KHONG_CO_GI_DOI = "Chưa có ô nào thay đổi, nên không có gì để lưu.";

/* ── `⊞ Danh mục tin`: edit, hide, delete (ADR 0067 §3) ─────────────────────────────────────── */

/** The owner's answer of 01/10/2026, said next to the toggle: hiding is about the chip, not the items. */
export const CATEGORY_HIDE_EXPLAINER =
  "Ẩn một danh mục chỉ bỏ nút lọc của danh mục ấy trên Mini App. Các bài trong danh mục vẫn hiện " +
  "cho bà con như cũ.";
export const CATEGORY_DELETE_NOTE =
  "Xoá là xoá mềm: danh mục và lý do xoá vẫn được lưu, slug không cấp lại. Chỉ xoá được danh mục " +
  "không còn bài nào và không còn danh mục con — còn thì hãy ẩn danh mục thay vì xoá.";
export const CATEGORY_PARENT_HINT =
  "Danh sách đã bỏ chính danh mục này và các danh mục con của nó. Máy chủ vẫn kiểm lại khi lưu.";
export const CATEGORY_SLUG_FIXED = "Slug đã cấp thì không đổi được.";
export const CATEGORY_SHOWN = "Đang hiện trên Mini App";
export const CATEGORY_HIDDEN = "Đã ẩn trên Mini App";
export const CATEGORY_HIDE_BUTTON = "Ẩn trên Mini App";
export const CATEGORY_SHOW_BUTTON = "Hiện lại trên Mini App";
export const CATEGORY_DELETE_BUTTON = "Xoá";
export const CATEGORY_EDIT_BUTTON = "Sửa";
export const CATEGORY_REASON_LABEL = "Lý do xoá *";

/**
 * The category itself and every category under it — the parents an edit must not offer.
 *
 * A HINT, NOT THE RULE: the server walks the ancestors under the commune's tree lock and answers 409
 * `category_cycle`. This only keeps the officer from picking a value that is certain to be refused. A
 * cycle already in the data cannot hang it: every id is visited once.
 */
export function selfAndDescendants(id: string, ds: readonly comms_danhMucRa[]): ReadonlySet<string> {
  const out = new Set<string>([id]);
  let grew = true;
  while (grew) {
    grew = false;
    for (const d of ds) {
      if (!out.has(d.id) && d.parent_id !== "" && out.has(d.parent_id)) {
        out.add(d.id);
        grew = true;
      }
    }
  }
  return out;
}

/** The tree for the parent select of the edit form, without the category and its descendants. */
export function parentChoices(id: string, ds: readonly comms_danhMucRa[]): readonly MucDanhMuc[] {
  const excluded = selfAndDescendants(id, ds);
  return dungCayDanhMuc(ds).filter((m) => !excluded.has(m.dm.id));
}

/** The three boxes of the edit form, as typed. */
export type CategoryEditValues = {
  readonly name: string;
  readonly parentId: string;
  readonly order: string;
};

export function categoryEditValues(dm: comms_danhMucRa): CategoryEditValues {
  return { name: dm.name, parentId: dm.parent_id, order: String(dm.order) };
}

/**
 * The PATCH body: ONLY what changed. Every field is a pointer at the server, so sending the order back
 * unchanged is harmless — but sending `parent_id` back unchanged re-runs the cycle walk under a lock for
 * nothing, and a body that names only what moved is the one the audit entry will describe. `hidden` is
 * not here: it has its own button and its own one-field PATCH. `slug` is never here (400).
 *
 * An unreadable order is sent as nothing: the box is `type=number` with `min=0`, and the server's own
 * 400 names the rule if a value ever gets through.
 */
export function categoryPatchBody(
  before: comms_danhMucRa,
  v: CategoryEditValues,
): UpdateCategoryIn {
  const out: UpdateCategoryIn = {};
  const name = v.name.trim();
  if (name !== before.name) out.name = name;
  if (v.parentId !== before.parent_id) out.parent_id = v.parentId;
  const order = Number.parseInt(v.order, 10);
  if (!Number.isNaN(order) && order !== before.order) out.order = order;
  return out;
}

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * NHỮNG PHẦN CỦA ĐẶC TẢ **KHÔNG DỰNG ĐƯỢC**, VÀ CHÚNG PHẢI RA TỚI MÀN HÌNH
 *
 * Không giấu trong chú thích, không vẽ một nút chắc chắn hỏng. Cùng khuôn `PHAN_CHUA_DUNG` của màn
 * Thông báo, màn Biên bản họp và màn Nhiệm vụ.
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

/** Name of the table's thumbnail column part — the prototype's first column. */
export const THUMBNAIL_PART = "Ảnh thu nhỏ trong bảng";

export type PhanChuaDung = {
  readonly ten: string;
  readonly viSao: string;
};

export const PHAN_CHUA_DUNG: readonly PhanChuaDung[] = [
  // §3 `{n} chuyên mục` on the card's meta line. The category list is never copied (ADR 0067 §2): every
  // read is an outbound call to the commune's portal, so counting on the card would call the portal on
  // every open of this screen. Drawn as a "?" on the portal-sync card (`portal-sync-card.tsx`, MA-02).
  {
    ten: "Số chuyên mục đang đồng bộ",
    viSao:
      "Thẻ chưa hiện số chuyên mục đang đồng bộ: danh sách chuyên mục được hỏi thẳng Cổng thông tin " +
      "của xã mỗi lần cần, không lưu lại, nên đếm trên thẻ là hỏi Cổng mỗi lần mở màn này. Số chuyên " +
      "mục đã chọn xem ở mục “Cấu hình” của thẻ, dạng “đã chọn n/30” (mỗi xã chọn tối đa 30 chuyên mục).",
  },
  // The prototype's first table column (06/10/2026): a thumbnail of each item. The LIST route serves only
  // `has_image`; the signed preview link is on the DETAIL route alone, so the column holds a "?" header
  // and "—" cells (`so-noi-dung.tsx`, `BangNoiDung`).
  // A literal, not `THUMBNAIL_PART`: `tools/tien_do_san_pham.py` counts `ten: "` lines in this block, and
  // a constant here drops the entry from the product progress table. `pendingContentPart(THUMBNAIL_PART)`
  // throws on any mismatch, so the two cannot drift silently (the table test renders it).
  {
    ten: "Ảnh thu nhỏ trong bảng",
    viSao:
      "Bảng chưa hiện ảnh thu nhỏ của từng bài: danh sách máy chủ trả chỉ cho biết bài có ảnh hay không, " +
      "còn đường xem ảnh chỉ có khi mở từng bài. Ảnh của một bài xem ở biểu mẫu sửa bài ấy.",
  },
];

/** One entry by its `ten`. Throws on an unknown name — a "?" with no description is never drawn. */
export function pendingContentPart(ten: string): PhanChuaDung {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === ten);
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry "${ten}"`);
  return found;
}
