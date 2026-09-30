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
 *   GET comms     /api/v1/commune-news/categories?host=…[&type=…]
 *                                                         → { items: [{ id, name, parent_id?, order }] }
 *                                                             (58abea4c; list route gained `&category=`)
 *
 *   Nguồn: `kb/20-contracts/openapi.json` (sinh từ mã, commit 48d99fe); ba phần thêm 29/09/2026 đọc từ
 *   `service-identity/internal/http/commune_profile.go`, `danh_ba_cong_khai.go:39-60` và
 *   `service-comms/internal/http/tin_xa_cong_khai.go:103-123`. Mọi trường thêm đều TUỲ CHỌN: vắng mặt là
 *   máy chủ cũ, không phải sai khuôn.
 *
 * ⚠ CÔNG KHAI, KHÔNG BEARER. Ba tuyến này chỉ trả thứ xã đã công bố cho người dân. Tệp gọi mạng
 * (`goi-vigov.ts`) không gắn `Authorization` cho chúng — gắn vào là gửi phiên công dân tới một tuyến
 * không cần nó.
 *
 * ⚠ `host` LÀ KHOÁ TRA, KHÔNG PHẢI THAM CHIẾU XÃ (ADR 0047 câu 3, điều kiện dừng #1). Không có mã xã
 * nào đi lên hay đi về: `/communes` cố ý chỉ trả tên và tỉnh.
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
export function diaChiBaiTin(ten_mien: string, id: string): string {
  const goc = diaChiViGov("comms", DUONG_DAN_TIN_XA);
  return goc === "" ? "" : voiHost(`${goc}/${encodeURIComponent(id)}`, ten_mien);
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
  const items = docMang(body);
  if (items === null) return null;
  const out: CommuneProfile[] = [];
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const r = m as Record<string, unknown>;
    if (!laChuoi(r.name) || !laChuoi(r.office_address) || !laChuoi(r.hotline) || !laChuoi(r.office_hours_text)) {
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
};

export type BaiTinXa = TinXaTomTat & {
  /** VĂN BẢN THUẦN, đoạn cách nhau bằng một dòng trống. Không bao giờ được vẽ như HTML. */
  readonly noi_dung: string;
};

export type TrangTinXa = {
  readonly muc: readonly TinXaTomTat[];
  readonly con_tro: string;
  readonly con_nua: boolean;
};

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
  return {
    id: r.id,
    tieu_de: r.title,
    tom_tat: r.summary,
    ngay_dang: r.published_on,
    chuyen_muc: r.category_name,
    type: isNewsType(r.type) ? r.type : null,
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
  const { body } = than as Record<string, unknown>;
  if (!laChuoi(body)) return null;
  return { ...t, noi_dung: body };
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
