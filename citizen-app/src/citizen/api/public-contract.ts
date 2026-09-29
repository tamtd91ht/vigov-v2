/**
 * HỢP ĐỒNG CỦA BA TUYẾN CÔNG KHAI THEO TÊN MIỀN XÃ — đường dẫn, tham số, và cách đọc thân trả lời.
 *
 *   GET identity  /api/v1/communes?host=<tên miền>        → { items: [{ name, province }] }
 *   GET identity  /api/v1/commune-profiles?host=…         → { items: [{ name, office_address, hotline,
 *                                                             office_hours_text }] }   (cdbf276)
 *   GET identity  /api/v1/commune-staff?host=<tên miền>   → { items: [{ full_name, position,
 *                                                             department_name, phone, mobile, has_zalo,
 *                                                             display_order?, residential_units_headed? }] }
 *   GET comms     /api/v1/commune-news?host=…[&cursor=…][&type=…]
 *                                                         → { items: [{ id, title, summary, published_on,
 *                                                             category_name, type?, source? }],
 *                                                             next_cursor, has_more }   (type: b22bf76)
 *   GET comms     /api/v1/commune-news/{id}?host=…        → cùng một mục, thêm `body` (văn bản thuần)
 *
 *   Nguồn: `kb/20-contracts/openapi.json` (sinh từ mã, commit 48d99fe); ba phần thêm 29/09/2026 đọc từ
 *   `service-identity/internal/http/commune_profile.go`, `danh_ba_cong_khai.go:39-60` và
 *   `service-comms/internal/http/tin_xa_cong_khai.go:103-123`. Mọi trường thêm đều TUỲ CHỌN: vắng mặt là
 *   máy chủ cũ, không phải sai khuôn.
 *
 * ⚠ CÔNG KHAI, KHÔNG BEARER. Ba tuyến này chỉ trả thứ xã đã công bố cho người dân. Tệp gọi mạng
 * (`vigov-client.ts`) không gắn `Authorization` cho chúng — gắn vào là gửi phiên công dân tới một tuyến
 * không cần nó.
 *
 * ⚠ `host` LÀ KHOÁ TRA, KHÔNG PHẢI THAM CHIẾU XÃ (ADR 0047 câu 3, điều kiện dừng #1). Không có mã xã
 * nào đi lên hay đi về: `/communes` cố ý chỉ trả tên và tỉnh.
 *
 * ⚠ KIỂM TỪNG TRƯỜNG, KHÔNG ÉP KIỂU — cùng lý do với `readReport`: một `as` cho `undefined` đi tiếp và
 * hiện ra màn hình thành chữ "undefined". Sai khuôn ở một dòng thì cả trang là `null`, không bỏ dòng
 * ấy lặng lẽ.
 */
import { vigovAddress } from "./vigov-address";

export const COMMUNES_PATH = "/api/v1/communes";
export const COMMUNE_PROFILES_PATH = "/api/v1/commune-profiles";
export const DIRECTORY_PATH = "/api/v1/commune-staff";
export const COMMUNE_NEWS_PATH = "/api/v1/commune-news";

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
function withHost(base: string, domain: string, more: Record<string, string> = {}): string {
  if (base === "") return "";
  return `${base}?${new URLSearchParams({ host: domain, ...more }).toString()}`;
}

export function communeLookupAddress(domain: string): string {
  return withHost(vigovAddress("identity", COMMUNES_PATH), domain);
}

export function communeProfilesAddress(domain: string): string {
  return withHost(vigovAddress("identity", COMMUNE_PROFILES_PATH), domain);
}

export function directoryAddress(domain: string): string {
  return withHost(vigovAddress("identity", DIRECTORY_PATH), domain);
}

/**
 * `cursor` rỗng = trang đầu. Con trỏ đi NGUYÊN VĂN — nó mờ đục, không phải số trang. `type` (tuỳ chọn):
 * máy chủ lọc theo loại; `null` = mọi loại, đúng như trước.
 */
export function communeNewsAddress(domain: string, cursor: string, type: NewsType | null = null): string {
  return withHost(vigovAddress("comms", COMMUNE_NEWS_PATH), domain, {
    ...(cursor === "" ? {} : { cursor }),
    ...(type === null ? {} : { type }),
  });
}

/** `encodeURIComponent` cho `id`: một ký tự lạ không được đổi đường dẫn. */
export function newsArticleAddress(domain: string, id: string): string {
  const base = vigovAddress("comms", COMMUNE_NEWS_PATH);
  return base === "" ? "" : withHost(`${base}/${encodeURIComponent(id)}`, domain);
}

const isString = (v: unknown): v is string => typeof v === "string";

function readArray(body: unknown): unknown[] | null {
  if (typeof body !== "object" || body === null) return null;
  const { items } = body as Record<string, unknown>;
  return Array.isArray(items) ? items : null;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * XÃ THEO TÊN MIỀN
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Xã như màn xác nhận cần — cùng hình dạng `XaGoiY` của lớp khám phá. Hai trường `ten` · `tinh` giữ tên
 * cũ vì hình dạng ấy dùng chung với nửa thương mại (`features/kham-pha`), nằm ngoài lượt đổi tên này.
 */
export type FoundCommune = { readonly ten: string; readonly tinh: string };

/**
 * `null` = sai khuôn. Mảng rỗng = máy chủ nói "không xã nào" (chưa đăng ký, đã giữ chỗ, ngừng hoạt
 * động — hợp đồng cố ý không tách ba ca ấy).
 *
 * HƠN MỘT MỤC LÀ KHÔNG XÃ NÀO: một tên miền chỉ trỏ một xã (`ResolveHost`). Hai mục là máy chủ nói
 * một điều không nhất quán, và màn xác nhận không được tự chọn một trong hai.
 */
export function readCommune(body: unknown): readonly FoundCommune[] | null {
  const items = readArray(body);
  if (items === null) return null;
  const out: FoundCommune[] = [];
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const { name, province } = m as Record<string, unknown>;
    if (!isString(name) || !isString(province)) return null;
    out.push({ ten: name, tinh: province });
  }
  return out;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * HỒ SƠ HIỂN THỊ CỦA XÃ — trụ sở, đường dây nóng, giờ làm việc (identity cdbf276)
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * What the commune declared about its office. Every field may be `""` (not declared) — the screen then
 * shows nothing for it, never a default. NO LOGO: the route deliberately has none (a typed URL is not an
 * approved public-bucket object, ADR 0052 §2), so the header keeps the logo file shipped with the build.
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
};

/**
 * `null` = malformed. `[]` = no commune for that domain (same answer `/communes` gives). More than one item
 * is not a profile this app can show — the caller treats it like none.
 */
export function readCommuneProfiles(body: unknown): readonly CommuneProfile[] | null {
  const items = readArray(body);
  if (items === null) return null;
  const out: CommuneProfile[] = [];
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const r = m as Record<string, unknown>;
    if (!isString(r.name) || !isString(r.office_address) || !isString(r.hotline) || !isString(r.office_hours_text)) {
      return null;
    }
    out.push({ name: r.name, office_address: r.office_address, hotline: r.hotline, office_hours_text: r.office_hours_text });
  }
  return out;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * DANH BẠ CÁN BỘ
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

/**
 * Một cán bộ đã được công khai. `mobile` là số CÁ NHÂN, công khai theo đồng ý đã ghi nhận (#12):
 * nó là dữ liệu cá nhân — chỉ hiện ra và thành liên kết gọi, không ghi log, không lưu, không gửi đi.
 */
export type PublicStaff = {
  readonly full_name: string;
  readonly position: string;
  /** `""` khi người ấy không thuộc bộ phận nào. */
  readonly org_unit: string;
  readonly office_phone: string;
  readonly mobile: string;
  readonly has_zalo: boolean;
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
  if (!Array.isArray(v) || !v.every(isString)) return undefined;
  return v.map((s) => s.trim()).filter((s) => s !== "");
}

export function readDirectory(body: unknown): readonly PublicStaff[] | null {
  const items = readArray(body);
  if (items === null) return null;
  const out: PublicStaff[] = [];
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const r = m as Record<string, unknown>;
    if (
      !isString(r.full_name) ||
      !isString(r.position) ||
      !isString(r.department_name) ||
      !isString(r.phone) ||
      !isString(r.mobile) ||
      typeof r.has_zalo !== "boolean"
    ) {
      return null;
    }
    const display_order = readDisplayOrder(r.display_order);
    const residential_units_headed = readUnitsHeaded(r.residential_units_headed);
    if (display_order === undefined || residential_units_headed === undefined) return null;
    out.push({
      full_name: r.full_name,
      position: r.position,
      org_unit: r.department_name,
      office_phone: r.phone,
      mobile: r.mobile,
      has_zalo: r.has_zalo,
      display_order,
      residential_units_headed,
    });
  }
  return out;
}

/* ════════════════════════════════════════════════════════════════════════════════════════════
 * TIN CỦA XÃ
 * ════════════════════════════════════════════════════════════════════════════════════════════ */

export type CommuneNewsSummary = {
  readonly id: string;
  readonly title: string;
  readonly summary: string;
  /** NGÀY đăng, `YYYY-MM-DD` — một ngày, không phải một thời điểm (`lib/date-time.ts` `vnDate`). */
  readonly published_on: string;
  readonly category: string;
  /**
   * The item's type (`tin-tuc`, `su-kien`…), or `null` when the server sent none (an older server) or a
   * type this app does not list (`banner`, or a seventh code added later) — never guessed from the category.
   */
  readonly type: NewsType | null;
};

export type CommuneNewsArticle = CommuneNewsSummary & {
  /** VĂN BẢN THUẦN, đoạn cách nhau bằng một dòng trống. Không bao giờ được vẽ như HTML. */
  readonly content: string;
};

export type CommuneNewsPage = {
  readonly entries: readonly CommuneNewsSummary[];
  readonly cursor: string;
  readonly has_more: boolean;
};

function readNews(m: unknown): CommuneNewsSummary | null {
  if (typeof m !== "object" || m === null) return null;
  const r = m as Record<string, unknown>;
  if (
    !isString(r.id) ||
    r.id === "" ||
    !isString(r.title) ||
    !isString(r.summary) ||
    !isString(r.published_on) ||
    !isString(r.category_name)
  ) {
    return null;
  }
  // `type` is optional (additive): absent is an older server. Present, it must be a string.
  if (r.type !== undefined && !isString(r.type)) return null;
  return {
    id: r.id,
    title: r.title,
    summary: r.summary,
    published_on: r.published_on,
    category: r.category_name,
    type: isNewsType(r.type) ? r.type : null,
  };
}

export function readCommuneNewsPage(body: unknown): CommuneNewsPage | null {
  const items = readArray(body);
  if (items === null) return null;
  const { next_cursor, has_more } = body as Record<string, unknown>;
  if (!isString(next_cursor) || typeof has_more !== "boolean") return null;
  // `has_more` mà không có con trỏ là một trang không đọc tiếp được — nút "Xem thêm" sẽ tải lại
  // đúng trang đầu, và người dân thấy tin trùng. Sai khuôn, không đoán.
  if (has_more && next_cursor === "") return null;
  const entries: CommuneNewsSummary[] = [];
  for (const m of items) {
    const t = readNews(m);
    if (t === null) return null;
    entries.push(t);
  }
  return { entries, cursor: next_cursor, has_more };
}

/** `body` là bắt buộc ở tuyến chi tiết: một tin không có thân là tuyến trả nhầm khuôn danh sách. */
export function readNewsArticle(raw: unknown): CommuneNewsArticle | null {
  const t = readNews(raw);
  if (t === null) return null;
  const { body } = raw as Record<string, unknown>;
  if (!isString(body)) return null;
  return { ...t, content: body };
}

/**
 * Thân tin → các đoạn, theo đúng quy ước của máy chủ: đoạn cách nhau bằng MỘT DÒNG TRỐNG ("\n\n").
 * Đoạn rỗng (nhiều dòng trống liền nhau) bị bỏ; dòng đơn trong một đoạn được giữ và hiện xuống dòng.
 */
export function splitParagraphs(content: string): string[] {
  return content
    .replace(/\r\n?/g, "\n")
    .split(/\n\s*\n/)
    .map((d) => d.trim())
    .filter((d) => d !== "");
}
