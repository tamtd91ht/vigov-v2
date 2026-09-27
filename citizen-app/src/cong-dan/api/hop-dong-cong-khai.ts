/**
 * HỢP ĐỒNG CỦA BA TUYẾN CÔNG KHAI THEO TÊN MIỀN XÃ — đường dẫn, tham số, và cách đọc thân trả lời.
 *
 *   GET identity  /api/v1/communes?host=<tên miền>        → { items: [{ name, province }] }
 *   GET identity  /api/v1/commune-staff?host=<tên miền>   → { items: [{ full_name, position,
 *                                                             department_name, phone, mobile, has_zalo }] }
 *   GET comms     /api/v1/commune-news?host=…[&cursor=…]  → { items: [{ id, title, summary,
 *                                                             published_on, category_name }],
 *                                                             next_cursor, has_more }
 *   GET comms     /api/v1/commune-news/{id}?host=…        → cùng một mục, thêm `body` (văn bản thuần)
 *
 *   Nguồn: `kb/20-contracts/openapi.json` (sinh từ mã, commit 48d99fe).
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
export const DUONG_DAN_DANH_BA = "/api/v1/commune-staff";
export const DUONG_DAN_TIN_XA = "/api/v1/commune-news";

/** Tên miền trong `?host=` — `URLSearchParams` mã hoá, không ghép chuỗi tay. */
function voiHost(goc: string, ten_mien: string, them: Record<string, string> = {}): string {
  if (goc === "") return "";
  return `${goc}?${new URLSearchParams({ host: ten_mien, ...them }).toString()}`;
}

export function diaChiTraXa(ten_mien: string): string {
  return voiHost(diaChiViGov("identity", DUONG_DAN_XA), ten_mien);
}

export function diaChiDanhBa(ten_mien: string): string {
  return voiHost(diaChiViGov("identity", DUONG_DAN_DANH_BA), ten_mien);
}

/** `con_tro` rỗng = trang đầu. Con trỏ đi NGUYÊN VĂN — nó mờ đục, không phải số trang. */
export function diaChiTinXa(ten_mien: string, con_tro: string): string {
  return voiHost(
    diaChiViGov("comms", DUONG_DAN_TIN_XA),
    ten_mien,
    con_tro === "" ? {} : { cursor: con_tro },
  );
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
};

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
    ra.push({
      ho_ten: r.full_name,
      chuc_vu: r.position,
      bo_phan: r.department_name,
      so_co_quan: r.phone,
      di_dong: r.mobile,
      co_zalo: r.has_zalo,
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
  return {
    id: r.id,
    tieu_de: r.title,
    tom_tat: r.summary,
    ngay_dang: r.published_on,
    chuyen_muc: r.category_name,
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
