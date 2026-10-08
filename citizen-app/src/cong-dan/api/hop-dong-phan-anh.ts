/**
 * HỢP ĐỒNG VỚI ViGov CHO PHIẾU PHẢN ÁNH CỦA CÔNG DÂN — tệp DUY NHẤT biết đường dẫn, tên trường
 * gửi đi và hình dạng phản hồi. Viết tay: `citizen-app` không có kiểu sinh từ openapi.
 *
 *   POST /api/v1/my-citizen-reports            Bearer + Idempotency-Key  → 201 phieuCuaToiRa
 *   GET  /api/v1/my-citizen-reports/{maTraCuu} Bearer                    → 200 phieuCuaToiRa · 404
 *   GET  /api/v1/my-citizen-reports            Bearer                    → 200 trang phieuCuaToiTomTatRa
 *   POST /api/v1/my-citizen-reports/{maTraCuu}/rating
 *                                              Bearer + Idempotency-Key  → 200 phieuCuaToiRa · 404 · 409
 *
 * TUYẾN ĐÁNH GIÁ (ADR 0050 điểm 2; `service-petitions/internal/http/petition_rating.go`, commit 7359484):
 * thân `{stars: 1..5, comment?: string ≤ 1000 ký tự}` — HAI khoá, không gì khác (`RATING_FIELDS`). 200 trả
 * ĐÚNG khuôn của GET, nay có thêm `rating` (số nguyên) và `rated_at` (RFC3339), cả hai TUỲ CHỌN và đi cùng
 * nhau. 409 `petition_state` là phiếu không ở trạng thái đánh giá được, vừa đổi trạng thái, hoặc lần gửi
 * cùng khoá còn đang chạy. Nhận xét KHÔNG được trả lại — máy chủ cố ý không gửi nó về.
 *
 * TUYẾN DANH SÁCH (26/09/2026), NGUYÊN VĂN HỢP ĐỒNG ĐÃ GIAO:
 *
 *   GET /api/v1/my-citizen-reports — CitizenOnly; citizen and commune come ONLY from the ViGov
 *   citizen session token (never send tenant/phone/citizen id in query/body/header);
 *   phone_verified_required: true.
 *    Query: limit (1-100, default 20), cursor (opaque, pass back next_cursor verbatim),
 *    sort=received_at (only), order=desc (only desc accepted; omit it), status (optional, one of
 *    the nine codes).
 *    200: {items: phieuCuaToiTomTatRa[], next_cursor: string ("" when has_more false),
 *    has_more: boolean}; items always [] when none.
 *    phieuCuaToiTomTatRa: {code: string (lookup code), status: string, field: string,
 *    field_label: string, content_excerpt: string (≤140 chars, "…" if cut), clock_from: date-time,
 *    acknowledge_due: date-time|null, resolve_due: date-time|null} — all keys always present.
 *    Errors: 400 invalid params; 401 no/invalid session or no commune chosen; 403 `chua_xac_thuc_so`
 *    (session has no verified phone); 500. No 404.
 *
 *   ⚠ BẢN GIAO GHI "No 403/404" — SAI Ở VẾ 403 (sửa 28/09/2026). Cả ba tuyến ở đầu tệp đi qua lớp rìa
 *   công dân, và lớp ấy trả 403 `{"code":"chua_xac_thuc_so"}` khi phiên chưa có số điện thoại đã xác
 *   thực (`core/httpx/citizen.go:199-200`, `phone_verified_required: true` ở trên). Đọc nó là
 *   `PHONE_NOT_VERIFIED_CODE` bên dưới; tài liệu phía máy chủ còn nợ dòng ấy.
 *
 *   Client gửi ĐÚNG HAI tham số: `limit` và (từ trang thứ hai) `cursor`. `sort`/`order` bỏ đi vì
 *   máy chủ chỉ có một cách xếp; `status` chưa màn nào dùng.
 *
 * Nguồn đối chiếu (đọc, không sửa): `service-petitions/internal/http/gui_phan_anh.go`,
 * `phieu_cua_toi.go`, `routes_cong_dan.go`.
 *
 * ⚠ NĂM TRƯỜNG, cộng HAI TUỲ CHỌN `lat`/`lng` (29/09/2026, `OPTIONAL_SCENE_FIELDS`, cả hai hoặc không),
 * cộng MỘT TUỲ CHỌN `field` (29/09/2026, service-petitions af3fff0, `OPTIONAL_FIELD_KEY`): MÃ lĩnh vực
 * dân chọn từ `GET /api/v1/my-citizen-report-fields` (ADR 0050 điểm 1). Mã không được xã mở cho biểu mẫu
 * — lạ, đã ngừng, xã tắt, `can-bo` — là MỘT câu trả lời: 400 `field_not_offered`; đọc được bộ mã thì mới
 * ghi, không thì 503 `field_catalogue_unavailable` (ADR 0060 §3). `guiPhanAnhVao` khai thêm các trường
 * CHỈ ĐỂ TỪ CHỐI: `citizen_id`/`cong_dan_id` (người gửi — lấy từ phiên, luật 4), `linh_vuc`, `channel`,
 * `code`, `status`, `clock_from`, `acknowledge_due`, `resolve_due`. Xã thì không có trường nào cả — xã
 * lấy từ phiên (ADR 0022). `goi-vigov.test.tsx` khẳng định thân gửi đi đúng bằng năm khoá dưới.
 */

import { diaChiViGov } from "./dia-chi-vigov";

/** Đường dẫn tài nguyên. Một hằng — GET ghép mã tra cứu vào sau nó, không gõ lại. */
export const DUONG_DAN_PHAN_ANH_CUA_TOI = "/api/v1/my-citizen-reports";

/** Năm khoá máy chủ NHẬN. Xuất ra để phép kiểm đối chiếu, không để tệp khác dựng thân. */
export const TRUONG_DUOC_NHAN = [
  "content",
  "address",
  "reporter_name",
  "reporter_phone",
  "anonymous",
] as const;

/**
 * Giới hạn độ dài — CHÉP từ `service-petitions/internal/domain/gui_phan_anh.go` (đếm theo KÝ TỰ,
 * không theo byte). Client kiểm trước để nói bằng câu của người dân thay vì câu kỹ thuật của 400;
 * máy chủ vẫn là bên quyết định. Máy chủ đổi số thì sửa ở đây.
 */
export const DO_DAI_TOI_DA = {
  noi_dung: 4000,
  dia_chi: 500,
  ho_ten: 200,
  dien_thoai: 32,
} as const;

/**
 * THE TWO OPTIONAL KEYS (ViGov b5d17bb, `service-petitions/internal/http/gui_phan_anh.go` `Lat`/`Lng`):
 * JSON numbers, BOTH OR NEITHER, -90..90 / -180..180; anything else is 400. The server rounds to 6
 * digits (NUMERIC(9,6)); the 201 and GET-by-code echo them, omitted when absent. They describe where the
 * problem is and grant nothing — the commune still comes only from the session (ADR 0022).
 */
export const OPTIONAL_SCENE_FIELDS = ["lat", "lng"] as const;

/** The optional field-code key (af3fff0). Sent only when the citizen picked a field from the catalogue. */
export const OPTIONAL_FIELD_KEY = "field";

/* ────────────────────────────────────────────────────────────────────────────────────────────
 * DANH MỤC LĨNH VỰC CỦA XÃ CHO BIỂU MẪU — `GET /api/v1/my-citizen-report-fields`
 * (`service-petitions/internal/http/petition_fields.go:124-165`, af3fff0)
 *
 *   Bearer (phiên công dân; KHÔNG cần số đã xác thực — `XaTuPhienChiXem`), không tham số
 *   200 { items: [{ code, label, icon: string|null, tone: string|null }] }  — theo thứ tự của xã
 *   401 phiên · 503 `field_catalogue_unavailable` (không đọc được bộ mã — KHÔNG có danh sách dự phòng)
 *
 * `icon` là TÊN biểu tượng lucide, `tone` một trong sáu tông của nền tảng (`blue`, `green`, `orange`,
 * `purple`, `cyan`, `red`) — `null` khi nền tảng không khai: màn hình dùng biểu tượng và tông trung tính,
 * không đoán (`service-platform/migrations/0011_petition_field.sql:50-53`).
 * ──────────────────────────────────────────────────────────────────────────────────────────── */

export const CITIZEN_FIELDS_PATH = "/api/v1/my-citizen-report-fields";

export function citizenFieldsAddress(): string {
  return diaChiViGov("petitions", CITIZEN_FIELDS_PATH);
}

/** One field the commune offers on the new-submission form. `code` is what a petition stores. */
export type CitizenField = {
  readonly code: string;
  readonly label: string;
  readonly icon: string | null;
  readonly tone: string | null;
};

/**
 * 200 body → the offered fields, in the commune's order, or `null` if malformed. Field by field, never
 * cast; one malformed row is a malformed page (a silently dropped field is a field the citizen cannot pick
 * and cannot know about). An empty code or label is malformed: nothing to send, or nothing to show.
 */
export function readCitizenFields(body: unknown): readonly CitizenField[] | null {
  if (typeof body !== "object" || body === null) return null;
  const items = (body as Record<string, unknown>)["items"];
  if (!Array.isArray(items)) return null;
  const out: CitizenField[] = [];
  const optional = (v: unknown): string | null | undefined =>
    v === null || v === undefined ? null : typeof v === "string" ? (v === "" ? null : v) : undefined;
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const r = m as Record<string, unknown>;
    const icon = optional(r["icon"]);
    const tone = optional(r["tone"]);
    if (typeof r["code"] !== "string" || r["code"] === "" || typeof r["label"] !== "string" || r["label"].trim() === "") {
      return null;
    }
    if (icon === undefined || tone === undefined) return null;
    out.push({ code: r["code"], label: r["label"], icon, tone });
  }
  return out;
}

/** 400 code: the field sent is not one the commune's form offers (any reason — one answer). */
export const FIELD_NOT_OFFERED_CODE = "field_not_offered";

/** 503 code: the field catalogue could not be read — clears by itself; nothing was written. */
export const FIELD_CATALOGUE_UNAVAILABLE_CODE = "field_catalogue_unavailable";

/** The `code` of an error body (`{code, message, trace_id}`), or `null`. `message` is never read. */
export function errorCode(body: unknown): string | null {
  if (typeof body !== "object" || body === null) return null;
  const c = (body as Record<string, unknown>)["code"];
  return typeof c === "string" ? c : null;
}

/**
 * Where the problem is, as exchanged from the citizen's own `getLocation` tap (`vihat-miniapp`
 * `POST /api/v1/location`). Never a guessed address, never typed: the address box stays the citizen's.
 */
export type SceneLocation = { readonly lat: number; readonly lng: number };

/**
 * A location ViGov will accept — both numbers finite and inside the world's range. The same bounds as
 * the server's `domain.NormaliseSceneLocation`; checked here so a bad pair is dropped WHOLE, never sent
 * as a half the server would answer 400 to after the citizen pressed send.
 */
export function isSceneLocation(v: unknown): v is SceneLocation {
  if (typeof v !== "object" || v === null) return false;
  const { lat, lng } = v as Record<string, unknown>;
  return (
    typeof lat === "number" &&
    typeof lng === "number" &&
    Number.isFinite(lat) &&
    Number.isFinite(lng) &&
    lat >= -90 &&
    lat <= 90 &&
    lng >= -180 &&
    lng <= 180
  );
}

/** Thứ công dân gõ — đặt tên theo việc, không theo dây. */
export type PhanAnhMoi = {
  noi_dung: string;
  dia_chi: string;
  ho_ten: string;
  dien_thoai: string;
  an_danh: boolean;
  /**
   * OPTIONAL: set only by the citizen's "Lấy vị trí hiện tại" tap; absent or `null` = nothing sent.
   * Optional in the type so every existing literal of this type stays valid.
   */
  scene_location?: SceneLocation | null;
  /**
   * OPTIONAL: the field CODE the citizen picked from the commune's catalogue (`CitizenField.code`);
   * absent, `null` or "" = no `field` key at all. Never a label.
   */
  field?: string | null;
};

/**
 * `PhanAnhMoi` → thân yêu cầu. CHỖ DUY NHẤT năm tên trường gửi đi được viết ra — and the two optional
 * ones (`OPTIONAL_SCENE_FIELDS`).
 *
 * ẨN DANH THÌ KHÔNG GỬI HỌ TÊN VÀ SỐ ĐIỆN THOẠI — gửi chuỗi rỗng. Người bấm "Gửi ẩn danh" đã nói
 * họ không muốn tên mình gắn với phiếu; gửi hai ô ấy đi rồi trông vào máy chủ che là giữ lời hứa
 * bằng hệ thống của người khác (luật 3, bất biến 6: chỉ gửi đúng thứ cần).
 *
 * `lat`/`lng` GO TOGETHER OR NOT AT ALL: only a pair passing `isSceneLocation` is written, so a half
 * pair (or `NaN`, or out of range) sends neither key. The anonymous switch does not remove them: the
 * server keeps them like `address` (b5d17bb), and they exist only because the citizen tapped for them.
 */
export function thanGuiPhanAnh(pa: PhanAnhMoi): string {
  const location = isSceneLocation(pa.scene_location) ? pa.scene_location : null;
  const field = typeof pa.field === "string" ? pa.field.trim() : "";
  return JSON.stringify({
    content: pa.noi_dung.trim(),
    address: pa.dia_chi.trim(),
    reporter_name: pa.an_danh ? "" : pa.ho_ten.trim(),
    reporter_phone: pa.an_danh ? "" : pa.dien_thoai.trim(),
    anonymous: pa.an_danh,
    ...(location === null ? {} : { lat: location.lat, lng: location.lng }),
    ...(field === "" ? {} : { [OPTIONAL_FIELD_KEY]: field }),
  });
}

/**
 * Một phiếu như máy chủ trả cho CHÍNH NGƯỜI GỬI (`phieuCuaToiRa`). Không có ghi chú cán bộ, không
 * có lịch sử chuyển, không có người xử lý — máy chủ không gửi, và kiểu này không có chỗ cho chúng
 * (luật 4, cấm #5; luật 10, bất biến 7).
 *
 * Họ tên và số điện thoại về ĐÃ CHE (`Nguyễn V. A.`, `09****0000`), và RỖNG khi gửi ẩn danh.
 */
export type PhieuCuaToi = {
  readonly ma_tra_cuu: string;
  readonly trang_thai: string;
  readonly linh_vuc: string;
  readonly nhan_linh_vuc: string;
  readonly noi_dung: string;
  readonly dia_chi: string;
  readonly ho_ten_da_che: string;
  readonly dien_thoai_da_che: string;
  readonly an_danh: boolean;
  readonly goc_dem_han: string;
  /** `null` = KHÔNG ÁP DỤNG (phiếu cán bộ nhập hộ). */
  readonly han_tiep_nhan: string | null;
  /** `null` = CHƯA CÓ (phiếu chưa được phân loại). Hai `null` nghĩa trái nhau — đừng gộp. */
  readonly han_xu_ly_xong: string | null;
  readonly ket_qua: string;
  /**
   * Lý do xã KHÔNG TIẾP NHẬN hoặc CHUYỂN CẤP TRÊN — viết CHO công dân (migration 0011). RỖNG ở mọi
   * trạng thái khác, kể cả khi máy chủ lỡ gửi: `docPhieu` bỏ nó đi (xem `laNhanhKetThuc`).
   */
  readonly ly_do: string;
  /** Cơ quan nhận phiếu — chỉ có ở `chuyen-cap-tren`, RỖNG ở mọi trạng thái khác. */
  readonly co_quan_nhan: string;
  /**
   * The citizen's own star rating, 1..5 — `null` until they rate. A later rating REPLACES an earlier
   * one on the server, so this is always the latest. The comment is never sent back (see the header).
   */
  readonly rating: number | null;
  /** When that rating was recorded (RFC3339), `null` exactly when `rating` is. */
  readonly rated_at: string | null;
  /**
   * `contact_unverified` (ADR 0080 #5): `true` = sent from a session with no verified phone — the name and
   * number are typed contact, the petition belongs to the Zalo account, and NO notification will ever be sent.
   * Read from the server's explicit field, never inferred. Absent here means the server did not say `true`.
   */
  readonly contact_unverified?: true;
};

/** Stars bounds and comment limit — COPIED from `service-petitions/internal/domain/petition_rating.go`. */
export const RATING_MIN_STARS = 1;
export const RATING_MAX_STARS = 5;
/** Counted in CHARACTERS (code points), as the server's `utf8.RuneCountInString`. */
export const RATING_COMMENT_MAX_LEN = 1000;

/** The two keys the rating route accepts. Exported for the test, never for another file to build a body. */
export const RATING_FIELDS = ["stars", "comment"] as const;

/**
 * The server's rule for "may be rated now" (`domain.RatingOpen`): `da-xu-ly` and `cho-dan-xac-nhan`,
 * nothing else. Written as the two codes, NOT as "the group 'Đã xử lý xong'": the group table is a display
 * decision that may change; this is the server's lifecycle rule, and a status added to that group later
 * must not silently become rateable here. FAIL CLOSED — an unknown code is not rateable.
 */
export function isRateable(trang_thai: string): boolean {
  return trang_thai === "da-xu-ly" || trang_thai === "cho-dan-xac-nhan";
}

/**
 * Stars + comment → request body. THE ONLY PLACE the two field names are written.
 *
 * A BLANK COMMENT IS LEFT OUT, not sent as `""`: the server treats blank as absent anyway, and not sending
 * free text nobody typed is the rule-3 habit (invariant 6: only the fields actually needed).
 */
export function ratingBody(stars: number, comment: string): string {
  const c = comment.trim();
  return JSON.stringify(c === "" ? { stars } : { stars, comment: c });
}

/**
 * `rating` / `rated_at` of a response body → the pair, or `null` if MALFORMED.
 *
 * Both absent is "not rated" (`omitempty`). Present, `rating` must be an integer in 1..5 and `rated_at` a
 * string; ONE WITHOUT THE OTHER is malformed too — the server writes them together, and a star count with
 * no instant (or the reverse) is a response this client does not understand.
 */
function readRating(t: Record<string, unknown>): { rating: number | null; rated_at: string | null } | null {
  const r = t["rating"];
  const at = t["rated_at"];
  if (r === undefined && at === undefined) return { rating: null, rated_at: null };
  if (typeof r !== "number" || !Number.isInteger(r) || r < RATING_MIN_STARS || r > RATING_MAX_STARS) return null;
  if (typeof at !== "string" || at === "") return null;
  return { rating: r, rated_at: at };
}

/**
 * Giới hạn hai trường của hai nhánh kết thúc — CHÉP từ `service-petitions/internal/domain/
 * xu_ly_phan_anh.go` (`LyDoToiDa`, `CoQuanNhanToiDa`), đếm theo KÝ TỰ như `utf8.RuneCountInString`.
 * Ở chiều ĐỌC, vượt giới hạn nghĩa là máy chủ trả thứ nó không bao giờ ghi được — sai khuôn, không
 * phải một câu dài để cắt bớt.
 */
export const DO_DAI_NHANH_KET_THUC = {
  ly_do: 2000,
  co_quan_nhan: 200,
} as const;

/**
 * Hai trạng thái mang `reason`/`receiving_body`. Máy chủ tự kiểm điều kiện này trước khi gửi
 * (`phieu_cua_toi.go`); client kiểm LẠI vì đây là chỗ chữ cán bộ viết tới tay người dân — một lý do
 * từ chối hiện trên phiếu đang xử lý là một câu sai do cơ quan nhà nước nói ra.
 */
export function laNhanhKetThuc(trang_thai: string): boolean {
  return trang_thai === "khong-tiep-nhan" || trang_thai === "chuyen-cap-tren";
}

/** Số KÝ TỰ (code point), không phải số đơn vị UTF-16: "ă" tổ hợp hay emoji không bị đếm đôi. */
const soKyTu = (s: string): number => [...s].length;

/**
 * Thân trả lời → `PhieuCuaToi`, hoặc `null` nếu sai khuôn.
 *
 * KIỂM TỪNG TRƯỜNG, không ép kiểu: một `as` cho `undefined` đi tiếp và hiện ra màn hình thành chữ
 * "undefined" — ở đúng chỗ đáng lẽ là mã tra cứu của người dân.
 */
export function docPhieu(than: unknown): PhieuCuaToi | null {
  if (typeof than !== "object" || than === null) return null;
  const t = than as Record<string, unknown>;
  const chuoi = (k: string): string | null => (typeof t[k] === "string" ? (t[k] as string) : null);
  const chuoiHoacNull = (k: string): string | null | undefined =>
    t[k] === null ? null : typeof t[k] === "string" ? (t[k] as string) : undefined;

  const ma = chuoi("code");
  const trang_thai = chuoi("status");
  const linh_vuc = chuoi("field");
  const nhan = chuoi("field_label");
  const noi_dung = chuoi("content");
  const dia_chi = chuoi("address");
  const ho_ten = chuoi("reporter_name");
  const dien_thoai = chuoi("reporter_phone");
  const goc = chuoi("clock_from");
  const ket_qua = chuoi("result");
  const han_tiep_nhan = chuoiHoacNull("acknowledge_due");
  const han_xu_ly = chuoiHoacNull("resolve_due");
  const an_danh = t["anonymous"];
  // Hai trường TUỲ CHỌN (`omitempty`): vắng mặt là "", có mặt thì phải là chuỗi trong giới hạn.
  const tuyChon = (k: string, toi_da: number): string | null => {
    if (t[k] === undefined) return "";
    return typeof t[k] === "string" && soKyTu(t[k] as string) <= toi_da ? (t[k] as string) : null;
  };
  const ly_do = tuyChon("reason", DO_DAI_NHANH_KET_THUC.ly_do);
  const co_quan_nhan = tuyChon("receiving_body", DO_DAI_NHANH_KET_THUC.co_quan_nhan);
  const rating = readRating(t);
  // OPTIONAL (`boolean | null`, omitted for an identified petition); anything else is a body out of shape.
  const unverified = t["contact_unverified"];
  const unverifiedOk = unverified === undefined || unverified === null || typeof unverified === "boolean";

  if (
    !unverifiedOk ||
    rating === null ||
    ma === null ||
    ma === "" ||
    trang_thai === null ||
    linh_vuc === null ||
    nhan === null ||
    noi_dung === null ||
    dia_chi === null ||
    ho_ten === null ||
    dien_thoai === null ||
    goc === null ||
    ket_qua === null ||
    han_tiep_nhan === undefined ||
    han_xu_ly === undefined ||
    typeof an_danh !== "boolean" ||
    ly_do === null ||
    co_quan_nhan === null
  ) {
    return null;
  }

  const nhanh = laNhanhKetThuc(trang_thai);

  return {
    ma_tra_cuu: ma,
    trang_thai,
    linh_vuc,
    nhan_linh_vuc: nhan,
    noi_dung,
    dia_chi,
    ho_ten_da_che: ho_ten,
    dien_thoai_da_che: dien_thoai,
    an_danh,
    goc_dem_han: goc,
    han_tiep_nhan,
    han_xu_ly_xong: han_xu_ly,
    ket_qua,
    ly_do: nhanh ? ly_do : "",
    // Cơ quan nhận chỉ có nghĩa khi phiếu ĐƯỢC CHUYỂN; ở `khong-tiep-nhan` không ai nhận cả.
    co_quan_nhan: trang_thai === "chuyen-cap-tren" ? co_quan_nhan : "",
    rating: rating.rating,
    rated_at: rating.rated_at,
    ...(unverified === true ? { contact_unverified: true as const } : {}),
  };
}

/**
 * 429 code of the send route (ADR 0080 #7): this Zalo account already sent the day's ceiling of unverified
 * petitions in this commune. Nothing was written and no code issued. Its `message` is the one server sentence
 * this app shows (owner decision for this code only — `goi-vigov.ts` `readLimitMessage`).
 */
export const UNVERIFIED_DAILY_LIMIT_CODE = "unverified_daily_limit";

/**
 * Mã lỗi của thân 403 khi phiên chưa có số điện thoại đã xác thực — `core/httpx/citizen.go:199-200`
 * (`WriteError` → `{code, message, trace_id}`, `core/httpx/edge.go:88-98`).
 */
export const PHONE_NOT_VERIFIED_CODE = "chua_xac_thuc_so";

/**
 * Thân 403 có phải "chưa xác thực số" không. CHỈ đọc `code`: `message` là câu của máy chủ, và màn
 * người dân có câu riêng. Một 403 với mã khác (hay thân sai khuôn) KHÔNG phải nhánh này — nó là lỗi
 * máy chủ, và xin số điện thoại cho nó là xin một thứ không sửa được gì.
 */
export function isPhoneNotVerified(body: unknown): boolean {
  return (
    typeof body === "object" && body !== null && (body as Record<string, unknown>)["code"] === PHONE_NOT_VERIFIED_CODE
  );
}

/** Địa chỉ tuyến gửi trên host của `service-petitions`, hoặc RỖNG khi host ấy chưa có. */
export function diaChiGuiPhanAnh(): string {
  return diaChiViGov("petitions", DUONG_DAN_PHAN_ANH_CUA_TOI);
}

/**
 * MỘT DÒNG CỦA "PHẢN ÁNH CỦA TÔI" (`phieuCuaToiTomTatRa`). Ít hơn `PhieuCuaToi`: không địa chỉ,
 * không người gửi, không kết quả — chạm vào dòng là mở màn tra cứu, nơi đọc đủ phiếu.
 */
export type PhieuCuaToiTomTat = {
  readonly ma_tra_cuu: string;
  readonly trang_thai: string;
  readonly linh_vuc: string;
  readonly nhan_linh_vuc: string;
  /** Tối đa 140 ký tự, máy chủ đã cắt và thêm "…". */
  readonly trich_noi_dung: string;
  readonly goc_dem_han: string;
  readonly han_tiep_nhan: string | null;
  readonly han_xu_ly_xong: string | null;
  /** Same pair, same rule as on `PhieuCuaToi`. */
  readonly rating: number | null;
  readonly rated_at: string | null;
};

/** Một trang. `con_nua = false` thì `con_tro` luôn rỗng. */
export type TrangPhieuCuaToi = {
  readonly muc: readonly PhieuCuaToiTomTat[];
  readonly con_tro: string;
  readonly con_nua: boolean;
};

/** Số dòng mỗi trang — trong khoảng 1..100 hợp đồng cho phép. */
export const SO_DONG_MOI_TRANG = 20;

function docTomTat(than: unknown): PhieuCuaToiTomTat | null {
  if (typeof than !== "object" || than === null) return null;
  const t = than as Record<string, unknown>;
  const chuoi = (k: string): string | null => (typeof t[k] === "string" ? (t[k] as string) : null);
  const chuoiHoacNull = (k: string): string | null | undefined =>
    t[k] === null ? null : typeof t[k] === "string" ? (t[k] as string) : undefined;

  const ma = chuoi("code");
  const trang_thai = chuoi("status");
  const linh_vuc = chuoi("field");
  const nhan = chuoi("field_label");
  const trich = chuoi("content_excerpt");
  const goc = chuoi("clock_from");
  const han_tiep_nhan = chuoiHoacNull("acknowledge_due");
  const han_xu_ly = chuoiHoacNull("resolve_due");
  const rating = readRating(t);
  if (
    rating === null ||
    ma === null ||
    ma === "" ||
    trang_thai === null ||
    linh_vuc === null ||
    nhan === null ||
    trich === null ||
    goc === null ||
    han_tiep_nhan === undefined ||
    han_xu_ly === undefined
  ) {
    return null;
  }
  return {
    ma_tra_cuu: ma,
    trang_thai,
    linh_vuc,
    nhan_linh_vuc: nhan,
    trich_noi_dung: trich,
    goc_dem_han: goc,
    han_tiep_nhan,
    han_xu_ly_xong: han_xu_ly,
    rating: rating.rating,
    rated_at: rating.rated_at,
  };
}

/**
 * Thân trả lời của tuyến danh sách → `TrangPhieuCuaToi`, hoặc `null` nếu sai khuôn.
 *
 * MỘT DÒNG SAI KHUÔN LÀ CẢ TRANG SAI KHUÔN: bỏ lặng lẽ một dòng là giấu một phiếu của chính người
 * dân khỏi danh sách của họ — họ sẽ tưởng phiếu ấy chưa từng được gửi.
 *
 * `has_more = true` mà con trỏ rỗng cũng là sai khuôn: bấm "Xem thêm" với con trỏ rỗng là tải lại
 * trang đầu và nhân đôi danh sách.
 */
export function docTrangPhieuCuaToi(than: unknown): TrangPhieuCuaToi | null {
  if (typeof than !== "object" || than === null) return null;
  const t = than as Record<string, unknown>;
  const items = t["items"];
  const con_tro = t["next_cursor"];
  const con_nua = t["has_more"];
  if (!Array.isArray(items) || typeof con_tro !== "string" || typeof con_nua !== "boolean") return null;
  if (con_nua && con_tro === "") return null;

  const muc: PhieuCuaToiTomTat[] = [];
  for (const mot of items) {
    const p = docTomTat(mot);
    if (p === null) return null;
    muc.push(p);
  }
  return { muc, con_tro: con_nua ? con_tro : "", con_nua };
}

/**
 * Địa chỉ tuyến danh sách, hoặc RỖNG.
 *
 * CHỈ `limit` VÀ `cursor`. Không một tham số nào nói "của ai" hay "xã nào": máy chủ lấy cả hai từ
 * phiên (luật 4, cấm #1; luật 1, cấm #2). Con trỏ đi NGUYÊN VĂN — `URLSearchParams` mã hoá nó để
 * một ký tự `&` hay `=` trong chuỗi mờ không đổi được tham số, và máy chủ giải mã về đúng chuỗi ấy.
 */
export function diaChiDanhSach(con_tro: string): string {
  const goc = diaChiViGov("petitions", DUONG_DAN_PHAN_ANH_CUA_TOI);
  if (goc === "") return "";
  const q = new URLSearchParams({ limit: String(SO_DONG_MOI_TRANG) });
  if (con_tro !== "") q.set("cursor", con_tro);
  return `${goc}?${q.toString()}`;
}

/**
 * Địa chỉ tuyến tra một mã, hoặc RỖNG.
 *
 * MÃ TRA CỨU ĐI TRÊN ĐƯỜNG DẪN vì hợp đồng đặt nó ở đó (`{maTraCuu}`). Nó là mã nghiệp vụ, không
 * phải dữ liệu cá nhân, và chỉ mở được phiếu khi đi kèm đúng phiên của người gửi (404 cho mọi
 * trường hợp khác). `encodeURIComponent` để một ký tự lạ người dân gõ không đổi được đường dẫn.
 */
export function diaChiTraCuu(ma_tra_cuu: string): string {
  const goc = diaChiViGov("petitions", DUONG_DAN_PHAN_ANH_CUA_TOI);
  return goc === "" ? "" : `${goc}/${encodeURIComponent(ma_tra_cuu)}`;
}

/**
 * Address of the rating route for one lookup code, or EMPTY. Built on `diaChiTraCuu` so the code is
 * encoded exactly once, the same way — the route is a sub-path of that resource on the server too.
 */
export function ratingAddress(ma_tra_cuu: string): string {
  const one = diaChiTraCuu(ma_tra_cuu);
  return one === "" ? "" : `${one}/rating`;
}

/* ────────────────────────────────────────────────────────────────────────────────────────────
 * SCENE PHOTOS ON THE CITIZEN'S OWN PETITION (ADR 0047, row "Ảnh hiện trường khi gửi phản ánh"; server
 * `service-petitions/internal/http/petition_photo.go`, contract `kb/20-contracts/openapi.json`):
 *
 *   POST /api/v1/my-citizen-reports/{maTraCuu}/photos                  Bearer + Idempotency-Key
 *        {content_type, size} → 201 {photo: photoOut, upload: {url, fields, expires_at}}
 *   (the phone)  multipart POST to `upload.url`: every `fields` entry, then the file LAST as `file`
 *   POST /api/v1/my-citizen-reports/{maTraCuu}/photos/{id}/completion  Bearer → 200 photoOut
 *   GET  /api/v1/my-citizen-reports/{maTraCuu}/photos                  Bearer → 200 {items: photoLinkOut[]}
 *
 * Errors are `{code, message, trace_id}`; only `code` is read (`errorCode`), and the screen says its own
 * sentence. The petition is untouched by every photo failure (owner, 02/10/2026: a photo failure never
 * fails the send) — which is why photos go up AFTER the petition exists, by its lookup code.
 *
 * ⚠ BOTH REPLIES CARRY BEARER CREDENTIALS: the upload form (15 minutes) and each read URL (≤ 15 minutes).
 *   They live in the screen's memory and nowhere else — never logged, never put in a key, never stored.
 * ──────────────────────────────────────────────────────────────────────────────────────────── */

/**
 * The owner's ceiling (ADR 0047 G3: "không bắt buộc, tối đa 5"). The SERVER decides — platform's
 * `petition-photo` policy, counted under the petition's lock — and answers 409 `photo_limit`; this number
 * only sizes the picker and the label, so a citizen is not invited to pick a sixth photo that will be refused.
 */
export const MAX_SCENE_PHOTOS = 5;

/** The three types the server accepts (G3). HEIC and video are refused there; this app never declares them. */
export const SCENE_PHOTO_TYPES = ["image/jpeg", "image/png", "image/webp"] as const;
export type ScenePhotoType = (typeof SCENE_PHOTO_TYPES)[number];

/** The two keys the slot request carries — no file name, no location, nothing about the citizen. */
export const PHOTO_UPLOAD_FIELDS = ["content_type", "size"] as const;

/** The multipart field the file goes in, LAST, after every `fields` entry (presigned POST). */
export const STORAGE_FILE_FIELD = "file";

/** `code`s of the photo routes' refusals (`petition_photo.go:221-286`). Unlisted codes are a server fault. */
export const PHOTO_ERROR = {
  invalid: "invalid_request",
  notFound: "not_found",
  petitionState: "petition_state",
  limit: "photo_limit",
  photoState: "photo_state",
  notReceived: "upload_not_received",
  expired: "upload_expired",
  changed: "upload_changed",
  rejected: "photo_rejected",
  storageNotConfigured: "storage_not_configured",
  scanUnavailable: "malware_scan_unavailable",
  limitsUnavailable: "upload_limits_unavailable",
} as const;

/**
 * The image type the BYTES say, from their first 12 bytes — or `null` for anything else (HEIC, video, junk).
 * The declared type must match the bytes: the server sniffs them at completion and refuses a mismatch
 * (`RejectTypeMismatch`), so trusting the picker's own label would be a refusal after the upload.
 */
export function sniffScenePhotoType(head: Uint8Array): ScenePhotoType | null {
  const at = (i: number) => head[i] ?? -1;
  if (at(0) === 0xff && at(1) === 0xd8 && at(2) === 0xff) return "image/jpeg";
  if (at(0) === 0x89 && at(1) === 0x50 && at(2) === 0x4e && at(3) === 0x47) return "image/png";
  const ascii = (from: number, to: number) => String.fromCharCode(...head.slice(from, to));
  if (head.length >= 12 && ascii(0, 4) === "RIFF" && ascii(8, 12) === "WEBP") return "image/webp";
  return null;
}

/** `{content_type, size}` — THE ONLY PLACE the two field names are written. */
export function photoUploadBody(content_type: ScenePhotoType, size: number): string {
  return JSON.stringify({ content_type, size });
}

/** POST/GET …/{maTraCuu}/photos, or EMPTY. Built on `diaChiTraCuu`: the code is encoded once, the same way. */
export function photosAddress(ma_tra_cuu: string): string {
  const one = diaChiTraCuu(ma_tra_cuu);
  return one === "" ? "" : `${one}/photos`;
}

/**
 * GET …/{maTraCuu}/verification-photos, or EMPTY — the commune's "sau xử lý" photos of the citizen's OWN
 * petition (ADR 0047 row "Ảnh 'sau xử lý' của cán bộ — THAY G8", (b)(c)(e)). Same `photoListOut` as the scene
 * list (`readScenePhotoList`). The SERVER decides visibility: an empty list before `cho-dan-xac-nhan`.
 */
export function verificationPhotosAddress(ma_tra_cuu: string): string {
  const one = diaChiTraCuu(ma_tra_cuu);
  return one === "" ? "" : `${one}/verification-photos`;
}

/** POST …/{maTraCuu}/photos/{id}/completion, or EMPTY. The id is the server's own, encoded like the code. */
export function photoCompletionAddress(ma_tra_cuu: string, id: string): string {
  const photos = photosAddress(ma_tra_cuu);
  return photos === "" ? "" : `${photos}/${encodeURIComponent(id)}/completion`;
}

/** One photo as the slot and completion replies describe it. `status` is `pending` until stored. */
export type ScenePhotoOut = { readonly id: string; readonly status: string };

/** The presigned POST form. A bearer credential for 15 minutes — memory only. */
export type PhotoUploadForm = {
  readonly url: string;
  readonly fields: Readonly<Record<string, string>>;
  readonly expires_at: string;
};

export type PhotoSlot = { readonly photo: ScenePhotoOut; readonly upload: PhotoUploadForm };

/** An https URL, or `null`. A credential is never sent, and a picture never loaded, over plain http. */
function httpsUrl(v: unknown): string | null {
  if (typeof v !== "string") return null;
  try {
    return new URL(v).protocol === "https:" ? v : null;
  } catch {
    return null;
  }
}

/** `photoOut` → the two fields this app reads, or `null` if malformed. */
export function readScenePhoto(body: unknown): ScenePhotoOut | null {
  if (typeof body !== "object" || body === null) return null;
  const t = body as Record<string, unknown>;
  if (typeof t["id"] !== "string" || t["id"] === "" || typeof t["status"] !== "string") return null;
  return { id: t["id"], status: t["status"] };
}

/**
 * 201 of the slot request → the pending photo and its form, or `null` if malformed. An idempotent REPLAY
 * (`{code, replayed: true}`, `core/idem`) carries no form and is malformed here on purpose: the form is never
 * replayed, so the caller asks for a new slot with a new key.
 */
export function readPhotoSlot(body: unknown): PhotoSlot | null {
  if (typeof body !== "object" || body === null) return null;
  const t = body as Record<string, unknown>;
  const photo = readScenePhoto(t["photo"]);
  const u = t["upload"];
  if (photo === null || typeof u !== "object" || u === null) return null;
  const up = u as Record<string, unknown>;
  const url = httpsUrl(up["url"]);
  const fields = up["fields"];
  if (url === null || typeof up["expires_at"] !== "string" || typeof fields !== "object" || fields === null) return null;
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(fields as Record<string, unknown>)) {
    // `file` is OURS to write, last; a form that names it would put the bytes in the wrong place.
    if (typeof v !== "string" || k === STORAGE_FILE_FIELD) return null;
    out[k] = v;
  }
  return { photo, upload: { url, fields: out, expires_at: up["expires_at"] } };
}

/** One stored photo with its short-lived read link. Shown, never kept: refetch when `url_expires_at` passes. */
export type ScenePhotoLink = {
  readonly id: string;
  readonly url: string;
  readonly url_expires_at: string;
};

/** 200 of the list → the photos, or `null` if malformed (one bad row = a bad page, as `docTrangPhieuCuaToi`). */
export function readScenePhotoList(body: unknown): readonly ScenePhotoLink[] | null {
  if (typeof body !== "object" || body === null) return null;
  const items = (body as Record<string, unknown>)["items"];
  if (!Array.isArray(items)) return null;
  const out: ScenePhotoLink[] = [];
  for (const m of items) {
    if (typeof m !== "object" || m === null) return null;
    const r = m as Record<string, unknown>;
    const url = httpsUrl(r["url"]);
    if (typeof r["id"] !== "string" || r["id"] === "" || url === null || typeof r["url_expires_at"] !== "string") {
      return null;
    }
    out.push({ id: r["id"], url, url_expires_at: r["url_expires_at"] });
  }
  return out;
}
