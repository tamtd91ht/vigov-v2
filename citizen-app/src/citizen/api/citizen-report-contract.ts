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
 * lấy từ phiên (ADR 0022). `vigov-client.test.tsx` khẳng định thân gửi đi đúng bằng năm khoá dưới.
 */

import { vigovAddress } from "./vigov-address";

/** Đường dẫn tài nguyên. Một hằng — GET ghép mã tra cứu vào sau nó, không gõ lại. */
export const MY_REPORTS_PATH = "/api/v1/my-citizen-reports";

/** Năm khoá máy chủ NHẬN. Xuất ra để phép kiểm đối chiếu, không để tệp khác dựng thân. */
export const ACCEPTED_FIELDS = [
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
export const MAX_LENGTH = {
  content: 4000,
  address: 500,
  full_name: 200,
  phone: 32,
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
  return vigovAddress("petitions", CITIZEN_FIELDS_PATH);
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
export type NewReport = {
  content: string;
  address: string;
  full_name: string;
  phone: string;
  anonymous: boolean;
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
 * `NewReport` → thân yêu cầu. CHỖ DUY NHẤT năm tên trường gửi đi được viết ra — and the two optional
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
export function submitReportBody(report: NewReport): string {
  const location = isSceneLocation(report.scene_location) ? report.scene_location : null;
  const field = typeof report.field === "string" ? report.field.trim() : "";
  return JSON.stringify({
    content: report.content.trim(),
    address: report.address.trim(),
    reporter_name: report.anonymous ? "" : report.full_name.trim(),
    reporter_phone: report.anonymous ? "" : report.phone.trim(),
    anonymous: report.anonymous,
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
export type MyReport = {
  readonly lookup_code: string;
  readonly status: string;
  readonly field: string;
  readonly field_label: string;
  readonly content: string;
  readonly address: string;
  readonly masked_reporter_name: string;
  readonly masked_reporter_phone: string;
  readonly anonymous: boolean;
  readonly clock_from: string;
  /** `null` = KHÔNG ÁP DỤNG (phiếu cán bộ nhập hộ). */
  readonly acknowledge_due: string | null;
  /** `null` = CHƯA CÓ (phiếu chưa được phân loại). Hai `null` nghĩa trái nhau — đừng gộp. */
  readonly resolve_due: string | null;
  readonly result: string;
  /**
   * Lý do xã KHÔNG TIẾP NHẬN hoặc CHUYỂN CẤP TRÊN — viết CHO công dân (migration 0011). RỖNG ở mọi
   * trạng thái khác, kể cả khi máy chủ lỡ gửi: `readReport` bỏ nó đi (xem `isBranchEnd`).
   */
  readonly reason: string;
  /** Cơ quan nhận phiếu — chỉ có ở `chuyen-cap-tren`, RỖNG ở mọi trạng thái khác. */
  readonly receiving_body: string;
  /**
   * The citizen's own star rating, 1..5 — `null` until they rate. A later rating REPLACES an earlier
   * one on the server, so this is always the latest. The comment is never sent back (see the header).
   */
  readonly rating: number | null;
  /** When that rating was recorded (RFC3339), `null` exactly when `rating` is. */
  readonly rated_at: string | null;
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
export function isRateable(status: string): boolean {
  return status === "da-xu-ly" || status === "cho-dan-xac-nhan";
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
export const BRANCH_END_MAX_LENGTH = {
  reason: 2000,
  receiving_body: 200,
} as const;

/**
 * Hai trạng thái mang `reason`/`receiving_body`. Máy chủ tự kiểm điều kiện này trước khi gửi
 * (`phieu_cua_toi.go`); client kiểm LẠI vì đây là chỗ chữ cán bộ viết tới tay người dân — một lý do
 * từ chối hiện trên phiếu đang xử lý là một câu sai do cơ quan nhà nước nói ra.
 */
export function isBranchEnd(status: string): boolean {
  return status === "khong-tiep-nhan" || status === "chuyen-cap-tren";
}

/** Số KÝ TỰ (code point), không phải số đơn vị UTF-16: "ă" tổ hợp hay emoji không bị đếm đôi. */
const charCount = (s: string): number => [...s].length;

/**
 * Thân trả lời → `MyReport`, hoặc `null` nếu sai khuôn.
 *
 * KIỂM TỪNG TRƯỜNG, không ép kiểu: một `as` cho `undefined` đi tiếp và hiện ra màn hình thành chữ
 * "undefined" — ở đúng chỗ đáng lẽ là mã tra cứu của người dân.
 */
export function readReport(body: unknown): MyReport | null {
  if (typeof body !== "object" || body === null) return null;
  const t = body as Record<string, unknown>;
  const str = (k: string): string | null => (typeof t[k] === "string" ? (t[k] as string) : null);
  const stringOrNull = (k: string): string | null | undefined =>
    t[k] === null ? null : typeof t[k] === "string" ? (t[k] as string) : undefined;

  const code = str("code");
  const status = str("status");
  const field = str("field");
  const label = str("field_label");
  const content = str("content");
  const address = str("address");
  const reporter_name = str("reporter_name");
  const reporter_phone = str("reporter_phone");
  const clock_from = str("clock_from");
  const result = str("result");
  const acknowledge_due = stringOrNull("acknowledge_due");
  const resolve_due = stringOrNull("resolve_due");
  const anonymous = t["anonymous"];
  // Hai trường TUỲ CHỌN (`omitempty`): vắng mặt là "", có mặt thì phải là chuỗi trong giới hạn.
  const optionalField = (k: string, max: number): string | null => {
    if (t[k] === undefined) return "";
    return typeof t[k] === "string" && charCount(t[k] as string) <= max ? (t[k] as string) : null;
  };
  const reason = optionalField("reason", BRANCH_END_MAX_LENGTH.reason);
  const receiving_body = optionalField("receiving_body", BRANCH_END_MAX_LENGTH.receiving_body);
  const rating = readRating(t);

  if (
    rating === null ||
    code === null ||
    code === "" ||
    status === null ||
    field === null ||
    label === null ||
    content === null ||
    address === null ||
    reporter_name === null ||
    reporter_phone === null ||
    clock_from === null ||
    result === null ||
    acknowledge_due === undefined ||
    resolve_due === undefined ||
    typeof anonymous !== "boolean" ||
    reason === null ||
    receiving_body === null
  ) {
    return null;
  }

  const branch = isBranchEnd(status);

  return {
    lookup_code: code,
    status,
    field,
    field_label: label,
    content,
    address,
    masked_reporter_name: reporter_name,
    masked_reporter_phone: reporter_phone,
    anonymous,
    clock_from,
    acknowledge_due,
    resolve_due,
    result,
    reason: branch ? reason : "",
    // Cơ quan nhận chỉ có nghĩa khi phiếu ĐƯỢC CHUYỂN; ở `khong-tiep-nhan` không ai nhận cả.
    receiving_body: status === "chuyen-cap-tren" ? receiving_body : "",
    rating: rating.rating,
    rated_at: rating.rated_at,
  };
}

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
export function submitAddress(): string {
  return vigovAddress("petitions", MY_REPORTS_PATH);
}

/**
 * MỘT DÒNG CỦA "PHẢN ÁNH CỦA TÔI" (`phieuCuaToiTomTatRa`). Ít hơn `MyReport`: không địa chỉ,
 * không người gửi, không kết quả — chạm vào dòng là mở màn tra cứu, nơi đọc đủ phiếu.
 */
export type MyReportSummary = {
  readonly lookup_code: string;
  readonly status: string;
  readonly field: string;
  readonly field_label: string;
  /** Tối đa 140 ký tự, máy chủ đã cắt và thêm "…". */
  readonly content_excerpt: string;
  readonly clock_from: string;
  readonly acknowledge_due: string | null;
  readonly resolve_due: string | null;
  /** Same pair, same rule as on `MyReport`. */
  readonly rating: number | null;
  readonly rated_at: string | null;
};

/** Một trang. `has_more = false` thì `cursor` luôn rỗng. */
export type MyReportsPage = {
  readonly entries: readonly MyReportSummary[];
  readonly cursor: string;
  readonly has_more: boolean;
};

/** Số dòng mỗi trang — trong khoảng 1..100 hợp đồng cho phép. */
export const PAGE_SIZE = 20;

function readSummary(body: unknown): MyReportSummary | null {
  if (typeof body !== "object" || body === null) return null;
  const t = body as Record<string, unknown>;
  const str = (k: string): string | null => (typeof t[k] === "string" ? (t[k] as string) : null);
  const stringOrNull = (k: string): string | null | undefined =>
    t[k] === null ? null : typeof t[k] === "string" ? (t[k] as string) : undefined;

  const code = str("code");
  const status = str("status");
  const field = str("field");
  const label = str("field_label");
  const excerpt = str("content_excerpt");
  const clock_from = str("clock_from");
  const acknowledge_due = stringOrNull("acknowledge_due");
  const resolve_due = stringOrNull("resolve_due");
  const rating = readRating(t);
  if (
    rating === null ||
    code === null ||
    code === "" ||
    status === null ||
    field === null ||
    label === null ||
    excerpt === null ||
    clock_from === null ||
    acknowledge_due === undefined ||
    resolve_due === undefined
  ) {
    return null;
  }
  return {
    lookup_code: code,
    status,
    field,
    field_label: label,
    content_excerpt: excerpt,
    clock_from,
    acknowledge_due,
    resolve_due,
    rating: rating.rating,
    rated_at: rating.rated_at,
  };
}

/**
 * Thân trả lời của tuyến danh sách → `MyReportsPage`, hoặc `null` nếu sai khuôn.
 *
 * MỘT DÒNG SAI KHUÔN LÀ CẢ TRANG SAI KHUÔN: bỏ lặng lẽ một dòng là giấu một phiếu của chính người
 * dân khỏi danh sách của họ — họ sẽ tưởng phiếu ấy chưa từng được gửi.
 *
 * `has_more = true` mà con trỏ rỗng cũng là sai khuôn: bấm "Xem thêm" với con trỏ rỗng là tải lại
 * trang đầu và nhân đôi danh sách.
 */
export function readMyReportsPage(body: unknown): MyReportsPage | null {
  if (typeof body !== "object" || body === null) return null;
  const t = body as Record<string, unknown>;
  const items = t["items"];
  const cursor = t["next_cursor"];
  const has_more = t["has_more"];
  if (!Array.isArray(items) || typeof cursor !== "string" || typeof has_more !== "boolean") return null;
  if (has_more && cursor === "") return null;

  const entries: MyReportSummary[] = [];
  for (const item of items) {
    const p = readSummary(item);
    if (p === null) return null;
    entries.push(p);
  }
  return { entries, cursor: has_more ? cursor : "", has_more };
}

/**
 * Địa chỉ tuyến danh sách, hoặc RỖNG.
 *
 * CHỈ `limit` VÀ `cursor`. Không một tham số nào nói "của ai" hay "xã nào": máy chủ lấy cả hai từ
 * phiên (luật 4, cấm #1; luật 1, cấm #2). Con trỏ đi NGUYÊN VĂN — `URLSearchParams` mã hoá nó để
 * một ký tự `&` hay `=` trong chuỗi mờ không đổi được tham số, và máy chủ giải mã về đúng chuỗi ấy.
 */
export function listAddress(cursor: string): string {
  const base = vigovAddress("petitions", MY_REPORTS_PATH);
  if (base === "") return "";
  const q = new URLSearchParams({ limit: String(PAGE_SIZE) });
  if (cursor !== "") q.set("cursor", cursor);
  return `${base}?${q.toString()}`;
}

/**
 * Địa chỉ tuyến tra một mã, hoặc RỖNG.
 *
 * MÃ TRA CỨU ĐI TRÊN ĐƯỜNG DẪN vì hợp đồng đặt nó ở đó (`{maTraCuu}`). Nó là mã nghiệp vụ, không
 * phải dữ liệu cá nhân, và chỉ mở được phiếu khi đi kèm đúng phiên của người gửi (404 cho mọi
 * trường hợp khác). `encodeURIComponent` để một ký tự lạ người dân gõ không đổi được đường dẫn.
 */
export function lookupAddress(lookup_code: string): string {
  const base = vigovAddress("petitions", MY_REPORTS_PATH);
  return base === "" ? "" : `${base}/${encodeURIComponent(lookup_code)}`;
}

/**
 * Address of the rating route for one lookup code, or EMPTY. Built on `lookupAddress` so the code is
 * encoded exactly once, the same way — the route is a sub-path of that resource on the server too.
 */
export function ratingAddress(lookup_code: string): string {
  const report_address = lookupAddress(lookup_code);
  return report_address === "" ? "" : `${report_address}/rating`;
}
