/**
 * HỢP ĐỒNG CỦA BA TUYẾN CÔNG KHAI THEO TÊN MIỀN XÃ — đường dẫn, tham số, và cách đọc thân trả lời.
 *
 *   GET identity  /api/v1/communes?host=<tên miền>        → { items: [{ name, province }] }
 *   GET identity  /api/v1/commune-profiles?host=…         → { items: [{ name, office_address, hotline,
 *                                                             office_hours_text, logo_url? }] }
 *                                                             (cdbf276; logo_url: d93851b9, ADR 0069)
 *   GET identity  /api/v1/commune-staff?host=<tên miền>   → { items: [{ full_name, position,
 *                                                             department_name, phone, mobile, has_zalo,
 *                                                             display_order?, residential_units_headed? }] }
 *   GET comms     /api/v1/commune-news?host=…[&cursor=…][&type=…]
 *                                                         → { items: [{ id, title, summary, published_on,
 *                                                             category_name, type?, source?, image_url?,
 *                                                             published_at?, event_starts_at?,
 *                                                             event_ends_at?, event_place?, video_url?,
 *                                                             audio_url?, audio_url_expires_at?,
 *                                                             audio_duration_seconds? }],
 *                                                             next_cursor, has_more }   (type: b22bf76;
 *                                                             the rest: ADR 0047 §6, 2026-09-30/10-01)
 *   GET comms     /api/v1/commune-news/{id}?host=…        → cùng một mục, thêm `body` (văn bản thuần) và
 *                                                             `body_blocks?` (cấu trúc, ADR 0067 §1, abf0ca1d)
 *   GET comms     /api/v1/commune-news?host=…&type=banner → { items: [{ id, type: "banner", title,
 *                                                             image_url, link_to?, … }], has_more: false }
 *                                                             (dải ảnh trang chủ, ADR 0067 §5, abf0ca1d)
 *   GET comms     /api/v1/commune-news/categories?host=…[&type=…]
 *                                                         → { items: [{ id, name, parent_id?, order }] }
 *                                                             (58abea4c; list route gained `&category=`)
 *
 *   Nguồn: `kb/20-contracts/openapi.json` (sinh từ mã, commit 48d99fe); ba phần thêm 29/09/2026 đọc từ
 *   `service-identity/internal/http/commune_profile.go`, `danh_ba_cong_khai.go:39-60` và
 *   `service-comms/internal/http/tin_xa_cong_khai.go:103-123`; `published_at`, `event_*` (2ed818bd) và
 *   `image_url` (01/10/2026) từ cùng tệp `:155-177,209-226`; `video_url` (01/10/2026) `:168-171,217-221`. Mọi trường thêm đều TUỲ CHỌN: vắng mặt là máy chủ cũ, không phải sai khuôn.
 *
 * ⚠ CÔNG KHAI, KHÔNG BEARER. Ba tuyến này chỉ trả thứ xã đã công bố cho người dân. Tệp gọi mạng
 * (`goi-vigov.ts`) không gắn `Authorization` cho chúng — gắn vào là gửi phiên công dân tới một tuyến
 * không cần nó.
 *
 * ⚠ `host` LÀ KHOÁ TRA, KHÔNG PHẢI THAM CHIẾU XÃ (ADR 0047 câu 3, điều kiện dừng #1). Không có mã xã
 * nào đi lên, và không TRƯỜNG nào mang mã xã đi về: `/communes` cố ý chỉ trả tên và tỉnh.
 *
 * ONE KIND OF EXCEPTION, AND IT IS OPAQUE: a news item's `image_url` and a profile's `logo_url` are object
 * URLs in the public bucket, whose key carries `t_<tenant_id>` (ADR 0052, ADR 0069). This app never reads that
 * out of them — the URL is handed to `<img src>` as received (after the protocol check in `readImageUrl` /
 * `readLogoUrl`), never split, never logged, never used as a key — so the commune id is not a value anything
 * here can act on.
 *
 * ⚠ KIỂM TỪNG TRƯỜNG, KHÔNG ÉP KIỂU — cùng lý do với `docPhieu`: một `as` cho `undefined` đi tiếp và
 * hiện ra màn hình thành chữ "undefined". Sai khuôn ở một dòng thì cả trang là `null`, không bỏ dòng
 * ấy lặng lẽ.
 */
import { diaChiViGov } from "./dia-chi-vigov";

export const DUONG_DAN_XA = "/api/v1/communes";
export const COMMUNE_PROFILES_PATH = "/api/v1/commune-profiles";
export const DUONG_DAN_DANH_BA = "/api/v1/commune-staff";
export const DUONG_DAN_TIN_XA = "/api/v1/commune-news";
export const NEWS_CATEGORIES_PATH = "/api/v1/commune-news/categories";

/**
 * The news types the public list may be filtered by — `service-comms` `domain.LoaiNoiDung` (six closed
 * codes; an unknown one is a 400 there). `banner` is left out on purpose: it is a picture strip, not an
 * article, and no screen here lists it. Labels are the staff register's own words
 * (`web-admin/src/features/noi-dung/nhan-noi-dung.ts`), so the two apps call a type the same thing.
 */
export const NEWS_TYPES = ["tin-tuc", "su-kien", "thong-bao", "truyen-thanh", "video"] as const;
export type NewsType = (typeof NEWS_TYPES)[number];

const isNewsType = (v: unknown): v is NewsType => typeof v === "string" && (NEWS_TYPES as readonly string[]).includes(v);

/** Tên miền trong `?host=` — `URLSearchParams` mã hoá, không ghép chuỗi tay. */
function voiHost(goc: string, ten_mien: string, them: Record<string, string> = {}): string {
  if (goc === "") return "";
  return `${goc}?${new URLSearchParams({ host: ten_mien, ...them }).toString()}`;
}

export function diaChiTraXa(ten_mien: string): string {
  return voiHost(diaChiViGov("identity", DUONG_DAN_XA), ten_mien);
}

export function communeProfilesAddress(ten_mien: string): string {
  return voiHost(diaChiViGov("identity", COMMUNE_PROFILES_PATH), ten_mien);
}

export function diaChiDanhBa(ten_mien: string): string {
  return voiHost(diaChiViGov("identity", DUONG_DAN_DANH_BA), ten_mien);
}

/**
 * `con_tro` rỗng = trang đầu. Con trỏ đi NGUYÊN VĂN — nó mờ đục, không phải số trang. `type` (tuỳ chọn):
 * máy chủ lọc theo loại; `null` = mọi loại, đúng như trước.
 */
export function diaChiTinXa(
  ten_mien: string,
  con_tro: string,
  type: NewsType | null = null,
  /**
   * A category `id` the server itself returned from `/commune-news/categories` — never typed by the citizen.
   * `null` or `""` = no category filter, and the key is not sent at all (the privacy declaration in
   * `content/ket-xuat-ho-so.ts` lists it as sent only when a chip is tapped).
   */
  category: string | null = null,
): string {
  return voiHost(diaChiViGov("comms", DUONG_DAN_TIN_XA), ten_mien, {
    ...(con_tro === "" ? {} : { cursor: con_tro }),
    ...(type === null ? {} : { type }),
    ...(category === null || category === "" ? {} : { category }),
  });
}

/**
 * The commune's news categories that hold at least one published item of `type` (themselves or a
 * descendant), plus their ancestors — `service-comms` 58abea4c. `type` `null` = not sent.
 */
export function newsCategoriesAddress(ten_mien: string, type: NewsType | null = null): string {
  return voiHost(diaChiViGov("comms", NEWS_CATEGORIES_PATH), ten_mien, type === null ? {} : { type });
}

/** `encodeURIComponent` cho `id`: một ký tự lạ không được đổi đường dẫn. */
/**
 * `noView`: add `no_view=1`, which tells the server this read is NOT an open and must not be counted (owner,
 * 02/10/2026). Only the re-read for a fresh broadcast audio link sends it — the article is already open, and
 * counting that read would add a view for every expired link. Every real open (first load, "Thử lại", a related
 * article) leaves it out, so it counts.
 */
export function diaChiBaiTin(ten_mien: string, id: string, options: { noView?: boolean } = {}): string {
  const goc = diaChiViGov("comms", DUONG_DAN_TIN_XA);
  return goc === "" ? "" : voiHost(`${goc}/${encodeURIComponent(id)}`, ten_mien, options.noView === true ? { no_view: "1" } : {});
}

const laChuoi = (v: unknown): v is string => typeof v === "string";

function docMang(than: unknown): unknown[] | null {
  if (typeof than !== "object" || than === null) return null;
  const { items } = than as Record<string, unknown>;
  return Array.isArray(items) ? items : null;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * XÃ THEO TÊN MIỀN
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/** Xã như màn xác nhận cần — cùng hình dạng `XaGoiY` của lớp khám phá. */
export type XaTraDuoc = { readonly ten: string; readonly tinh: string };

/**
 * `null` = sai khuôn. Mảng rỗng = máy chủ nói "không xã nào" (chưa đăng ký, đã giữ chỗ, ngừng hoạt
 * động — hợp đồng cố ý không tách ba ca ấy).
 *
 * HƠN MỘT MỤC LÀ KHÔNG XÃ NÀO: một tên miền chỉ trỏ một xã (`ResolveHost`). Hai mục là máy chủ nói
 * một điều không nhất quán, và màn xác nhận không được tự chọn một trong hai.
 */
export function docXa(than: unknown): readonly XaTraDuoc[] | null {
  const items = docMang(than);
  if (items === null) return null;
  const ra: XaTraDuoc[] = [];
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const { name, province } = m as Record<string, unknown>;
    if (!laChuoi(name) || !laChuoi(province)) return null;
    ra.push({ ten: name, tinh: province });
  }
  return ra;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * HỒ SƠ HIỂN THỊ CỦA XÃ — trụ sở, đường dây nóng, giờ làm việc (identity cdbf276)
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * What the commune declared about its office. Every field may be `""` (not declared) — the screen then
 * shows nothing for it, never a default.
 *
 * `logo_url` (ADR 0069 #4, #8; identity d93851b9) is the logo the COMMUNE uploaded in web-admin, as the public
 * derivative's absolute `https:` URL — never the old hand-typed URL (the server sends only platform
 * `logo_public_url`). `""` = no uploaded logo: the header then falls back to the bundled file, then the icon
 * (`khung-xa.tsx` `LogoXa`). OPAQUE like a news `image_url`: its key carries `t_<tenant_id>`, and it goes to
 * `<img src>` untouched — never parsed, logged, or used as a key.
 *
 * `hotline` is the commune's OFFICIAL line — public-service information, not personal data (#16).
 * `office_hours_text` is DISPLAY TEXT: never parsed — the calendar deadlines count against is identity's
 * own (ADR 0007).
 */
export type CommuneProfile = {
  readonly name: string;
  readonly office_address: string;
  readonly hotline: string;
  readonly office_hours_text: string;
  /** An absolute `https:` URL, or `""` (none uploaded, an older server, or a value `readLogoUrl` refused). */
  readonly logo_url: string;
};

/**
 * `logo_url` on the wire → what the header may load. Absent → `""` (a server from before d93851b9 — additive
 * field, not malformed). Present and not a string → `null`: malformed, the whole answer is refused, as for
 * every other field here. A string is kept ONLY when `readHttpsLink` accepts it (absolute `https:`, a host, no
 * user part); anything else becomes `""`, not malformed — the office block is still worth showing without a
 * logo. Why only `https:`: the Mini App runs over https, so an `http:` picture is mixed content (a broken box
 * on a public authority's header), and `javascript:` / `data:` / a relative path must never become an `src` —
 * the server writes only its own public bucket's URL here, so anything else means something upstream is wrong.
 */
function readLogoUrl(v: unknown): string | null {
  if (v === undefined) return "";
  if (!laChuoi(v)) return null;
  return readHttpsLink(v) ?? "";
}

/**
 * `null` = malformed. `[]` = no commune for that domain (same answer `/communes` gives). More than one item
 * is not a profile this app can show — the caller treats it like none.
 */
export function readCommuneProfiles(body: unknown): readonly CommuneProfile[] | null {
  const items = docMang(body);
  if (items === null) return null;
  const out: CommuneProfile[] = [];
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const r = m as Record<string, unknown>;
    if (!laChuoi(r.name) || !laChuoi(r.office_address) || !laChuoi(r.hotline) || !laChuoi(r.office_hours_text)) {
      return null;
    }
    const logo_url = readLogoUrl(r.logo_url);
    if (logo_url === null) return null;
    out.push({ name: r.name, office_address: r.office_address, hotline: r.hotline, office_hours_text: r.office_hours_text, logo_url });
  }
  return out;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * DANH BẠ CÁN BỘ
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Một cán bộ đã được công khai. `di_dong` là số CÁ NHÂN, công khai theo đồng ý đã ghi nhận (#12):
 * nó là dữ liệu cá nhân — chỉ hiện ra và thành liên kết gọi, không ghi log, không lưu, không gửi đi.
 */
export type CanBoCongKhai = {
  readonly ho_ten: string;
  readonly chuc_vu: string;
  /** `""` khi người ấy không thuộc bộ phận nào. */
  readonly bo_phan: string;
  readonly so_co_quan: string;
  readonly di_dong: string;
  readonly co_zalo: boolean;
  /** The commune's explicit position in the directory, or `null` when none was set (optional on the wire). */
  readonly display_order: number | null;
  /**
   * Names of the thôn / tổ dân phố this person heads — `[]` for everybody else (optional on the wire).
   * Names only, and only for a person this directory already publishes.
   */
  readonly residential_units_headed: readonly string[];
};

/** Optional `display_order`: absent → `null`; present → an integer, else malformed (`undefined`). */
function readDisplayOrder(v: unknown): number | null | undefined {
  if (v === undefined) return null;
  return typeof v === "number" && Number.isInteger(v) ? v : undefined;
}

/** Optional `residential_units_headed`: absent → `[]`; present → an array of strings, else malformed. */
function readUnitsHeaded(v: unknown): readonly string[] | undefined {
  if (v === undefined) return [];
  if (!Array.isArray(v) || !v.every(laChuoi)) return undefined;
  return v.map((s) => s.trim()).filter((s) => s !== "");
}

export function docDanhBa(than: unknown): readonly CanBoCongKhai[] | null {
  const items = docMang(than);
  if (items === null) return null;
  const ra: CanBoCongKhai[] = [];
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const r = m as Record<string, unknown>;
    if (
      !laChuoi(r.full_name) ||
      !laChuoi(r.position) ||
      !laChuoi(r.department_name) ||
      !laChuoi(r.phone) ||
      !laChuoi(r.mobile) ||
      typeof r.has_zalo !== "boolean"
    ) {
      return null;
    }
    const display_order = readDisplayOrder(r.display_order);
    const residential_units_headed = readUnitsHeaded(r.residential_units_headed);
    if (display_order === undefined || residential_units_headed === undefined) return null;
    ra.push({
      ho_ten: r.full_name,
      chuc_vu: r.position,
      bo_phan: r.department_name,
      so_co_quan: r.phone,
      di_dong: r.mobile,
      co_zalo: r.has_zalo,
      display_order,
      residential_units_headed,
    });
  }
  return ra;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * TIN CỦA XÃ
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type TinXaTomTat = {
  readonly id: string;
  readonly tieu_de: string;
  readonly tom_tat: string;
  /** NGÀY đăng, `YYYY-MM-DD` — một ngày, không phải một thời điểm (`lib/thoi-diem.ts` `ngayVN`). */
  readonly ngay_dang: string;
  readonly chuyen_muc: string;
  /**
   * The item's type (`tin-tuc`, `su-kien`…), or `null` when the server sent none (an older server) or a
   * type this app does not list (`banner`, or a seventh code added later) — never guessed from the category.
   */
  readonly type: NewsType | null;
  /**
   * The cover picture — the public derivative's absolute `https:` URL, or ABSENT (no uploaded cover, an older
   * server, or a value `readImageUrl` refused). OPAQUE: it carries `t_<tenant_id>` in its path, so it goes to
   * `<img src>` untouched and nowhere else — never parsed, logged, or used as a key.
   */
  readonly imageUrl?: string;
  /**
   * The instant of the FIRST publish (RFC 3339; G1, ADR 0047 §6) — set once, never moved. ABSENT for items
   * published before the server recorded it (no backfill) or an unreadable value: the screen then shows
   * `ngay_dang` alone, never a guessed time.
   */
  readonly publishedAt?: string;
  /**
   * An event's window and place (RFC 3339 · PLAIN TEXT) — only ever on a `su-kien` item, each only when set.
   * `eventEndsAt` is kept only beside an `eventStartsAt` (the server refuses an end without a start).
   */
  readonly eventStartsAt?: string;
  readonly eventEndsAt?: string;
  readonly eventPlace?: string;
  /**
   * The link the commune posted for a `video` item — an absolute `https:` URL, or ABSENT (another type, no
   * link, or a value `readVideoUrl` refused). Opened OUTSIDE the app only on the citizen's tap ("Xem video"),
   * through the shell's opener (`moRaNgoai("video", …)`); never fetched, embedded or logged here.
   */
  readonly videoUrl?: string;
  /**
   * A broadcast's audio (ADR 0067 §4, comms 026ae398) — only on a `truyen-thanh` item whose file is attached and
   * verified, or ABSENT. See `readBroadcastAudio`.
   */
  readonly audio?: BroadcastAudio;
  /**
   * How many times the article was opened (owner, 02/10/2026) — a whole number ≥ 0, or ABSENT: an older server
   * that sends no `view_count`, or a value `readViewCount` refused. Absent shows NOTHING — never a made-up "0",
   * which would tell a commune nobody read an article the server simply did not count.
   */
  readonly viewCount?: number;
};

/**
 * The playable audio of one `truyen-thanh` item.
 *
 * `url` is a PRESIGNED link to the commune's PRIVATE original (no public copy exists, ADR 0067 §4.2), valid until
 * `expiresAt` (≤ 15 minutes). It is OPAQUE and SHORT-LIVED: handed to `<audio src>` as received and nowhere else —
 * never logged, stored, shared or used as a key; when it has expired the screen re-reads the item for a fresh one
 * (`broadcast-player.tsx`). Like `imageUrl`, its path carries `t_<tenant_id>`, which nothing here reads.
 */
export type BroadcastAudio = {
  readonly url: string;
  /** RFC 3339; ABSENT when unreadable — the player then learns of expiry from the media error alone. */
  readonly expiresAt?: string;
  /** What the commune's officer typed, in whole seconds (> 0); ABSENT when not usable. */
  readonly durationSeconds?: number;
};

export type BaiTinXa = TinXaTomTat & {
  /** VĂN BẢN THUẦN, đoạn cách nhau bằng một dòng trống. Không bao giờ được vẽ như HTML. */
  readonly noi_dung: string;
  /**
   * The body as STRUCTURE (ADR 0067 §1 decision 3) — ABSENT when the server sent none (an older server, or a
   * body with no text) or when nothing in it was readable. The screen then shows `noi_dung`, exactly as
   * before. Never HTML: every `text` is drawn as a React text node.
   */
  readonly bodyBlocks?: readonly BodyBlock[];
};

/** A stretch of text with one formatting. `"\n"` inside `text` is a line break. */
export type BodyRun = {
  readonly text: string;
  readonly bold: boolean;
  readonly italic: boolean;
  /** An absolute `https:` URL with no user part, or ABSENT — then the run is plain text. */
  readonly href?: string;
};

/** One block of `body_blocks` — the server's closed list (`tin_xa_cong_khai.go` `bodyBlockOut`). */
export type BodyBlock =
  | { readonly kind: "paragraph"; readonly runs: readonly BodyRun[] }
  | { readonly kind: "heading"; readonly level: 2 | 3; readonly runs: readonly BodyRun[] }
  | { readonly kind: "bullet_list" | "ordered_list"; readonly items: readonly (readonly BodyRun[])[] }
  /**
   * A body image (ADR 0067 §Sửa đổi 03/10/2026, H1–H5). `src` is an absolute `https:` URL — the server sends one
   * only for the article's own PUBLISHED file on ViGov's public store; this app adds the protocol wall again and
   * nothing more. `alt` is `""` when staff typed none. `caption` ABSENT when it has none or nothing in it read.
   */
  | { readonly kind: "image"; readonly src: string; readonly alt: string; readonly caption?: readonly BodyRun[] }
  /** A quotation: one entry per paragraph, each with at least one run. */
  | { readonly kind: "quote"; readonly paragraphs: readonly (readonly BodyRun[])[] }
  /** The author / source line staff typed at the end of the article (H9) — drawn where it stands. */
  | { readonly kind: "byline"; readonly runs: readonly BodyRun[] };

/**
 * A link target the app may offer to open: an absolute `https:` URL with a host and no user part, or
 * `null`. Same stance as `readVideoUrl`, one rule more: `https://gov.vn@other.example` names `other.example`,
 * and the confirmation would then name a host the citizen did not read in the text — the server refuses it
 * on banners (`domain.NormalizeLinkTo`), and nothing here trusts that it did. PURE.
 */
export function readHttpsLink(v: unknown): string | null {
  if (!laChuoi(v)) return null;
  try {
    const u = new URL(v);
    return u.protocol === "https:" && u.hostname !== "" && u.username === "" && u.password === "" ? v : null;
  } catch {
    return null;
  }
}

/**
 * One run. Not an object, or `text` not a string → `null` (the run is skipped). An `href` that is not a
 * readable https link is DROPPED and the words stay, as plain text — the citizen still reads them, nothing
 * opens.
 */
function readRun(v: unknown): BodyRun | null {
  if (typeof v !== "object" || v === null) return null;
  const r = v as Record<string, unknown>;
  if (!laChuoi(r.text) || r.text === "") return null;
  const href = r.href === undefined ? null : readHttpsLink(r.href);
  return { text: r.text, bold: r.bold === true, italic: r.italic === true, ...(href === null ? {} : { href }) };
}

function readRuns(v: unknown): BodyRun[] {
  if (!Array.isArray(v)) return [];
  return v.map(readRun).filter((r): r is BodyRun => r !== null);
}

/**
 * `body_blocks` → the blocks this app can draw, or `undefined` (show `body` instead).
 *
 * LENIENT, ON PURPOSE, unlike the rest of this file: the plain `body` beside it is always there, so a block
 * this app cannot read costs the citizen nothing if it is skipped — while refusing the whole article over
 * one would cost them the article. So: not an array → absent; an unknown `kind` (a later server's block, or
 * anything that ever said `script`) → that block skipped; a block left with no text → skipped; nothing left
 * → absent. A heading level other than 3 is drawn as level 2 — a size, not a fact.
 *
 * AN IMAGE whose `src` is not an absolute `https:` URL with a host and no user part is skipped WHOLE, caption included: the
 * server sends an image block only for a resolved https file, so anything else means something upstream is wrong
 * — and the caption's words are already in the plain `body`, which is not what is drawn here, so nothing is
 * claimed about them. An `alt` that is not a string is `""` (decorative), never a reason to drop the picture.
 */
export function readBodyBlocks(v: unknown): readonly BodyBlock[] | undefined {
  if (!Array.isArray(v)) return undefined;
  const out: BodyBlock[] = [];
  for (const b of v) {
    if (typeof b !== "object" || b === null) continue;
    const r = b as Record<string, unknown>;
    if (r.kind === "paragraph" || r.kind === "heading" || r.kind === "byline") {
      const runs = readRuns(r.runs);
      if (runs.length === 0) continue;
      if (r.kind === "heading") out.push({ kind: "heading", level: r.level === 3 ? 3 : 2, runs });
      else out.push({ kind: r.kind, runs });
    } else if (r.kind === "image") {
      // `readHttpsLink`, not only `readImageUrl`'s protocol check: `https://store@other.example/x.jpg` is an https
      // URL that loads from `other.example` — a host every resident's phone would then be sent to (stop condition 5).
      const src = readHttpsLink(r.src);
      if (src === null) continue;
      const caption = readRuns(r.caption);
      out.push({ kind: "image", src, alt: laChuoi(r.alt) ? r.alt : "", ...(caption.length === 0 ? {} : { caption }) });
    } else if (r.kind === "quote") {
      if (!Array.isArray(r.paragraphs)) continue;
      const paragraphs = r.paragraphs
        .map((p) => (typeof p === "object" && p !== null ? readRuns((p as Record<string, unknown>).runs) : []))
        .filter((runs) => runs.length > 0);
      if (paragraphs.length > 0) out.push({ kind: "quote", paragraphs });
    } else if (r.kind === "bullet_list" || r.kind === "ordered_list") {
      if (!Array.isArray(r.items)) continue;
      const items = r.items
        .map((it) => (typeof it === "object" && it !== null ? readRuns((it as Record<string, unknown>).runs) : []))
        .filter((runs) => runs.length > 0);
      if (items.length > 0) out.push({ kind: r.kind, items });
    }
  }
  return out.length === 0 ? undefined : out;
}

export type TrangTinXa = {
  readonly muc: readonly TinXaTomTat[];
  readonly con_tro: string;
  readonly con_nua: boolean;
};

/**
 * Optional `image_url`: absent → `undefined` (no cover); present and not a string → malformed (`null`, the
 * caller refuses the whole page, as for every other field).
 *
 * A STRING IS USED ONLY WHEN IT IS AN ABSOLUTE `https:` URL; anything else is treated as absent, not as
 * malformed — the article is still worth reading without its picture. Why only `https:`:
 *   · the Mini App runs in Zalo's webview over https, so an `http:` picture is mixed content: blocked, or a
 *     warning in front of a public authority's page — either way a broken box;
 *   · `javascript:` / `data:` / a relative path must never become an `src` — the server writes only its
 *     own bucket's URL here, so anything else means something upstream is wrong, and showing it is worse
 *     than showing the icon.
 * The check reads the PROTOCOL and nothing else; the value returned is the string as received.
 */
function readImageUrl(v: unknown): string | undefined | null {
  if (v === undefined) return undefined;
  if (!laChuoi(v)) return null;
  try {
    return new URL(v).protocol === "https:" ? v : undefined;
  } catch {
    return undefined; // not an absolute URL
  }
}

/**
 * Optional instant (`published_at`, `event_starts_at`, `event_ends_at`): absent, or `null` — which the contract
 * allows (`kb/20-contracts/openapi.json` `comms.tinXaRa`, `["string","null"]`) — → `undefined`; not a string →
 * malformed (`null`). A string no `Date` can read is treated as absent: the item is still worth reading, and
 * a time the app cannot read is a time it must not print.
 */
function readInstant(v: unknown): string | undefined | null {
  if (v === undefined || v === null) return undefined;
  if (!laChuoi(v)) return null;
  return Number.isNaN(new Date(v).getTime()) ? undefined : v;
}

/**
 * Optional `video_url` (`comms.tinXaRa`, `type: string`, omitted when empty): absent → `undefined`; present and
 * not a string → malformed (`null`, the whole page is refused — `null` included, the contract does not allow it).
 *
 * ONLY AN ABSOLUTE `https:` URL IS KEPT, though the server accepts `http:` too (`domain.ChuanHoaURL`). Anything
 * else is absent, not malformed — the article is still worth reading; it just has no "Xem video" button. Why
 * stricter than the server, when the link opens in Zalo's browser and not inside this page (so mixed content is
 * not the reason it is for `image_url`):
 *   · the button sits on a public authority's screen, so the citizen trusts what opens. Over `http:` anyone on
 *     the same network (a café Wi-Fi) can replace that page — a fake "đăng nhập Zalo để xem" form is then shown
 *     under the commune's name. Over `https:` the page that opens is the one the commune posted;
 *   · `javascript:` / `data:` / a relative path must never reach the opener at all.
 * COST, stated: a commune that posts an `http:` link sees no button for it in the app. Video hosts serve
 * `https:`; the write path accepting `http:` is a finding for `service-comms` / web-admin, not something this
 * screen papers over. The check reads the PROTOCOL only; the string returned is the one received.
 */
function readVideoUrl(v: unknown): string | undefined | null {
  if (v === undefined) return undefined;
  if (!laChuoi(v)) return null;
  try {
    return new URL(v).protocol === "https:" ? v : undefined;
  } catch {
    return undefined; // not an absolute URL
  }
}

/**
 * `audio_url` · `audio_url_expires_at` · `audio_duration_seconds` (`tin_xa_cong_khai.go` `tinXaRa`, all omitempty)
 * → the item's audio, or `undefined`. PURE.
 *
 * LENIENT, ON PURPOSE, unlike the other optional fields of an item: a broadcast whose audio fields are odd is still
 * an item worth listing and reading (title, summary, body), so nothing here ever makes the page malformed — a
 * wrong value is DROPPED. The link decides: no readable `https:` link (`readHttpsLink`: a host, no user part) → no
 * audio at all, and the duration goes with it, as on the server ("a player bar with nothing to play is the
 * failure"). Why only `https:`: the Mini App runs over https, so an `http:` source is mixed content — blocked, or
 * a broken player on a public authority's page. An expiry no `Date` can read → absent (expiry is then learnt
 * from the media error). A duration that is not a positive whole number → absent (no total is printed).
 */
export function readBroadcastAudio(url: unknown, expiresAt: unknown, durationSeconds: unknown): BroadcastAudio | undefined {
  const link = readHttpsLink(url);
  if (link === null) return undefined;
  const exp = laChuoi(expiresAt) && !Number.isNaN(new Date(expiresAt).getTime()) ? expiresAt : undefined;
  const dur =
    typeof durationSeconds === "number" && Number.isInteger(durationSeconds) && durationSeconds > 0 ? durationSeconds : undefined;
  return { url: link, ...(exp === undefined ? {} : { expiresAt: exp }), ...(dur === undefined ? {} : { durationSeconds: dur }) };
}

/** Optional plain text (`event_place`): absent → `undefined`; not a string → malformed; blank → absent. */
function readOptionalText(v: unknown): string | undefined | null {
  if (v === undefined) return undefined;
  if (!laChuoi(v)) return null;
  const t = v.trim();
  return t === "" ? undefined : t;
}

/**
 * Optional `view_count` → a whole number ≥ 0, or `undefined`. PURE.
 *
 * LENIENT, like the audio fields: a count the app cannot read costs the citizen nothing if it is dropped, while
 * refusing the whole page over it would cost them the news. So a fraction, a negative, a string, `null` → absent.
 */
export function readViewCount(v: unknown): number | undefined {
  return typeof v === "number" && Number.isSafeInteger(v) && v >= 0 ? v : undefined;
}

function docTin(m: unknown): TinXaTomTat | null {
  if (typeof m !== "object" || m === null) return null;
  const r = m as Record<string, unknown>;
  if (
    !laChuoi(r.id) ||
    r.id === "" ||
    !laChuoi(r.title) ||
    !laChuoi(r.summary) ||
    !laChuoi(r.published_on) ||
    !laChuoi(r.category_name)
  ) {
    return null;
  }
  // `type` is optional (additive): absent is an older server. Present, it must be a string.
  if (r.type !== undefined && !laChuoi(r.type)) return null;
  const imageUrl = readImageUrl(r.image_url);
  const publishedAt = readInstant(r.published_at);
  const eventStartsAt = readInstant(r.event_starts_at);
  const eventEndsAt = readInstant(r.event_ends_at);
  const eventPlace = readOptionalText(r.event_place);
  const videoUrl = readVideoUrl(r.video_url);
  if (
    imageUrl === null ||
    publishedAt === null ||
    eventStartsAt === null ||
    eventEndsAt === null ||
    eventPlace === null ||
    videoUrl === null
  ) {
    return null;
  }
  const type = isNewsType(r.type) ? r.type : null;
  // THE TYPE GATES THE EVENT FIELDS HERE TOO, as on the server (`tin_xa_cong_khai.go:210-216`): an event window
  // printed on a news article is a date a resident acts on. An end with no start is dropped for the same reason.
  const isEvent = type === "su-kien";
  // Gated by type, as on the server (`tin_xa_cong_khai.go:348`): a player on an item that is not a broadcast would
  // play something the commune never published as one.
  const audio = type === "truyen-thanh" ? readBroadcastAudio(r.audio_url, r.audio_url_expires_at, r.audio_duration_seconds) : undefined;
  const viewCount = readViewCount(r.view_count);
  return {
    id: r.id,
    tieu_de: r.title,
    tom_tat: r.summary,
    ngay_dang: r.published_on,
    chuyen_muc: r.category_name,
    type,
    // Keys are left out, not set to `undefined`: "not there" has one shape.
    ...(imageUrl === undefined ? {} : { imageUrl }),
    ...(publishedAt === undefined ? {} : { publishedAt }),
    ...(isEvent && eventStartsAt !== undefined ? { eventStartsAt } : {}),
    ...(isEvent && eventStartsAt !== undefined && eventEndsAt !== undefined ? { eventEndsAt } : {}),
    ...(isEvent && eventPlace !== undefined ? { eventPlace } : {}),
    // Gated by type like the event fields, and as on the server (`tin_xa_cong_khai.go:217-221`): a "Xem video"
    // button on an article that is not a video would open a link the commune never offered as one.
    ...(type === "video" && videoUrl !== undefined ? { videoUrl } : {}),
    ...(audio === undefined ? {} : { audio }),
    ...(viewCount === undefined ? {} : { viewCount }),
  };
}

export function docTrangTinXa(than: unknown): TrangTinXa | null {
  const items = docMang(than);
  if (items === null) return null;
  const { next_cursor, has_more } = than as Record<string, unknown>;
  if (!laChuoi(next_cursor) || typeof has_more !== "boolean") return null;
  // `has_more` mà không có con trỏ là một trang không đọc tiếp được — nút "Xem thêm" sẽ tải lại
  // đúng trang đầu, và người dân thấy tin trùng. Sai khuôn, không đoán.
  if (has_more && next_cursor === "") return null;
  const muc: TinXaTomTat[] = [];
  for (const m of items) {
    const t = docTin(m);
    if (t === null) return null;
    muc.push(t);
  }
  return { muc, con_tro: next_cursor, con_nua: has_more };
}

/** `body` là bắt buộc ở tuyến chi tiết: một tin không có thân là tuyến trả nhầm khuôn danh sách. */
export function docBaiTin(than: unknown): BaiTinXa | null {
  const t = docTin(than);
  if (t === null) return null;
  const { body, body_blocks } = than as Record<string, unknown>;
  if (!laChuoi(body)) return null;
  const bodyBlocks = readBodyBlocks(body_blocks);
  return { ...t, noi_dung: body, ...(bodyBlocks === undefined ? {} : { bodyBlocks }) };
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * DẢI ẢNH TRANG CHỦ — `?type=banner` (ADR 0067 §5, comms abf0ca1d)
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/** The strip's address: the list route with `type=banner` — host and type, nothing else. */
export function bannersAddress(ten_mien: string): string {
  return voiHost(diaChiViGov("comms", DUONG_DAN_TIN_XA), ten_mien, { type: "banner" });
}

/**
 * One banner. `title` is the picture's `alt` (§5 decision 1). `linkTo` is the tap target as the server
 * re-checked it — an in-app path (`/…`, never `//…`) or an https link (`readHttpsLink`) — or ABSENT: the
 * banner is then a picture and nothing else. WHICH in-app paths lead anywhere is the screen's call
 * (`TrangXa.tsx` `bannerScreen`), not this file's.
 */
export type CommuneBannerItem = {
  readonly id: string;
  readonly title: string;
  readonly imageUrl: string;
  readonly linkTo?: string;
};

function readLinkTo(v: unknown): string | undefined | null {
  if (v === undefined) return undefined;
  if (!laChuoi(v)) return null;
  if (v.startsWith("/")) return v.startsWith("//") || /[\s\\]/.test(v) ? undefined : v;
  return readHttpsLink(v) ?? undefined;
}

/**
 * `null` = malformed (a field of the wrong type, or not a list) — the caller then keeps the bundled picture,
 * same stance as `docTrangTinXa`. A banner with no usable `https:` picture is DROPPED, not malformed: §5 makes
 * the picture mandatory, so one without is a row the server should not have sent, and showing an empty frame
 * for it is worse than not showing it. ORDER AS RECEIVED — the server sorts by `display_order`.
 */
export function readBanners(body: unknown): readonly CommuneBannerItem[] | null {
  const items = docMang(body);
  if (items === null) return null;
  const out: CommuneBannerItem[] = [];
  const seen = new Set<string>();
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const r = m as Record<string, unknown>;
    if (!laChuoi(r.id) || r.id === "" || !laChuoi(r.title)) return null;
    const imageUrl = readImageUrl(r.image_url);
    const linkTo = readLinkTo(r.link_to);
    if (imageUrl === null || linkTo === null) return null;
    // A repeated id is one banner twice and a duplicate React key; the first one stands.
    if (imageUrl === undefined || seen.has(r.id)) continue;
    seen.add(r.id);
    const title = r.title.trim();
    // A tappable picture with no words is a button a screen reader announces as nothing: no title, no tap.
    out.push({ id: r.id, title, imageUrl, ...(linkTo === undefined || title === "" ? {} : { linkTo }) });
  }
  return out;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * DANH MỤC TIN — hàng chip lọc hai tầng của tab Tin tức (58abea4c)
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/** One category as the server sent it. `parent_id` `null` = a root (the key is absent on the wire). */
export type NewsCategory = {
  readonly id: string;
  readonly name: string;
  readonly parent_id: string | null;
  readonly order: number;
};

/**
 * `null` = malformed, the whole list — same stance as `docTrangTinXa`. Beyond the field types, three
 * shapes are refused because each would put something wrong on screen: an empty `id` (a chip that filters
 * by nothing), a blank `name` (a button with no words — a screen reader reads nothing), and a repeated
 * `id` (two chips that are the same filter, and a duplicate React key).
 */
export function readNewsCategories(body: unknown): readonly NewsCategory[] | null {
  const items = docMang(body);
  if (items === null) return null;
  const out: NewsCategory[] = [];
  const seen = new Set<string>();
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const r = m as Record<string, unknown>;
    if (!laChuoi(r.id) || r.id === "" || seen.has(r.id)) return null;
    if (!laChuoi(r.name) || r.name.trim() === "") return null;
    if (typeof r.order !== "number" || !Number.isInteger(r.order)) return null;
    // Optional: absent on a root. Present, it must name a category — an empty string is not "no parent".
    if (r.parent_id !== undefined && (!laChuoi(r.parent_id) || r.parent_id === "")) return null;
    seen.add(r.id);
    out.push({ id: r.id, name: r.name.trim(), parent_id: laChuoi(r.parent_id) ? r.parent_id : null, order: r.order });
  }
  return out;
}

/**
 * Thân tin → các đoạn, theo đúng quy ước của máy chủ: đoạn cách nhau bằng MỘT DÒNG TRỐNG ("\n\n").
 * Đoạn rỗng (nhiều dòng trống liền nhau) bị bỏ; dòng đơn trong một đoạn được giữ và hiện xuống dòng.
 */
export function chiaDoan(noi_dung: string): string[] {
  return noi_dung
    .replace(/\r\n?/g, "\n")
    .split(/\n\s*\n/)
    .map((d) => d.trim())
    .filter((d) => d !== "");
}
