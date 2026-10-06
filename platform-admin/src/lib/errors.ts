import { ApiError, type PasswordRejection } from "@/lib/api";
import { SIGN_IN_PATH } from "@/lib/route-access";

/**
 * What a person reads when a call fails. Every sentence says what happened and what to do next;
 * the server's `code` never appears on screen (skills/accessibility-elderly #5). The `trace_id`
 * does, on its own line, because it is the one thing support needs to find the request.
 */

export const NETWORK_ERROR = "Không kết nối được tới máy chủ. Kiểm tra kết nối mạng rồi thử lại.";

/**
 * 503 on a guarded route. It is NOT "signed out": identity being down says nothing about the
 * session, and sending the person to sign in would make them retype credentials into an outage.
 */
export const AUTH_UNAVAILABLE =
  "Hệ thống xác thực tạm không phản hồi. Phiên làm việc của bạn chưa bị đăng xuất; vui lòng thử lại sau ít phút.";

export const GENERIC_ERROR = "Đã xảy ra lỗi. Vui lòng thử lại; nếu vẫn lỗi, báo bộ phận kỹ thuật kèm mã tra cứu.";

export const INVALID_BODY = "Dữ liệu gửi lên không hợp lệ. Kiểm tra lại các ô rồi gửi lại.";

/** "Thử lại sau N giây / khoảng N phút", from `Retry-After`. Unknown wait: say so, never invent one. */
export function waitMessage(seconds: number | null): string {
  if (seconds === null) return "Bạn đã thử quá nhiều lần. Vui lòng đợi một lúc rồi thử lại.";
  if (seconds < 60) return `Bạn đã thử quá nhiều lần. Vui lòng thử lại sau ${seconds} giây.`;
  return `Bạn đã thử quá nhiều lần. Vui lòng thử lại sau khoảng ${Math.ceil(seconds / 60)} phút.`;
}

/**
 * The new-password rule that refused, with the bounds the SERVER sent — this console holds no copy
 * of the minimum length. `previous` names what it must differ from ("mật khẩu tạm" at enrolment).
 */
export function passwordRejectionText(r: PasswordRejection | null, previous: string): string {
  switch (r?.problem) {
    case "empty":
      return "Hãy nhập mật khẩu mới.";
    case "too_short":
      return `Mật khẩu mới phải có ít nhất ${r.minLength} ký tự.`;
    case "too_long":
      return `Mật khẩu mới không được dài quá ${r.maxLength} ký tự.`;
    case "same_as_current":
      return `Mật khẩu mới phải khác ${previous}.`;
    case "not_utf8":
      return "Mật khẩu mới chứa ký tự không hợp lệ. Hãy gõ lại.";
    default:
      return "Mật khẩu mới không đạt yêu cầu. Hãy chọn mật khẩu khác.";
  }
}

/** Shown under a message when the server gave one. */
export function traceLine(err: unknown): string | null {
  return err instanceof ApiError && err.traceId !== "" ? `Mã tra cứu: ${err.traceId}` : null;
}

/**
 * The common part of every GUARDED call's failure (rule: ADR 0048 §01/10 #2 — the server decides).
 *
 *   401 → the session is gone: go to sign-in, show nothing (returns null).
 *   403 `forbidden` → name the ACTION not permitted; the key list is the server's.
 *   503 → the auth service is down: say so, stay on the page.
 *   anything else → `specific(err)` if the screen knows the code, else the generic sentence.
 */
export function handleGuardedError(
  err: unknown,
  action: string,
  navigate: (path: string) => void,
  specific?: (err: ApiError) => string | null,
): string | null {
  if (!(err instanceof ApiError)) return GENERIC_ERROR;
  if (err.status === 401) {
    navigate(SIGN_IN_PATH);
    return null;
  }
  if (err.status === 0) return NETWORK_ERROR;
  if (err.status === 403 && err.code === "forbidden") {
    return `Tài khoản vận hành của bạn không có quyền ${action}. Nếu cần, đề nghị người quản lý tài khoản vận hành cấp quyền.`;
  }
  // A 503 the screen names (mini_app_secret_unavailable: identity could not store a secret) is not
  // the auth service being down; any other 503 is.
  if (err.status === 503) return specific?.(err) ?? AUTH_UNAVAILABLE;
  if (err.status === 429) return waitMessage(err.retryAfterSeconds);
  if (err.code === "invalid_body") return INVALID_BODY;
  return specific?.(err) ?? GENERIC_ERROR;
}

/**
 * Commune-registry refusals (operator_communes.go `writeRegistryError`), each with the next step.
 * `field` says which input the sentence belongs under, so a form shows it there.
 */
export type CommuneField = "name" | "province" | "domain" | "reason" | "appId" | "note" | "secret" | "form";

const COMMUNE_ERRORS: Record<string, { field: CommuneField; text: string }> = {
  commune_not_found: { field: "form", text: "Không tìm thấy xã. Quay lại danh sách xã và chọn lại." },
  unknown_province: { field: "province", text: "Tỉnh đã chọn không có trong danh mục. Tải lại trang rồi chọn lại." },
  invalid_name: {
    field: "name",
    text: "Tên xã không hợp lệ. Nhập tên từ 1 đến 200 ký tự, không xuống dòng.",
  },
  duplicate_name: { field: "name", text: "Tỉnh này đã có xã cùng tên. Kiểm tra lại tên xã hoặc tỉnh." },
  invalid_domain: {
    field: "domain",
    text: "Tên miền không hợp lệ. Nhập tên miền trần gồm chữ thường không dấu, chữ số, dấu gạch ngang và dấu chấm — không kèm https://, đường dẫn hay cổng.",
  },
  reserved_domain: { field: "domain", text: "Tên miền này dành riêng cho nền tảng, không gắn được cho xã." },
  domain_taken: { field: "domain", text: "Tên miền này đã được gắn cho một xã. Mỗi tên miền chỉ thuộc một xã." },
  domain_not_in_commune: { field: "domain", text: "Tên miền này không thuộc xã. Chọn trong các tên miền xã đang có." },
  commune_inactive: {
    field: "form",
    text: "Xã đang ngừng hoạt động nên không thay đổi được. Bật hoạt động trở lại trước, nếu việc đó đúng.",
  },
  invalid_reason: { field: "reason", text: "Hãy ghi lý do, tối đa 500 ký tự." },
  invalid_app_id: { field: "appId", text: "App ID chỉ gồm chữ số, tối đa 32 chữ số." },
  invalid_note: { field: "note", text: "Ghi chú tối đa 500 ký tự, không chứa ký tự điều khiển." },
  mini_app_taken: { field: "appId", text: "App ID này đã có trong sổ Mini App, không gắn thêm được." },
  commune_succeeded: {
    field: "form",
    text: "Xã đã được sáp nhập hoặc chia tách vào đơn vị khác, không mở lại hoạt động được.",
  },
  mini_app_not_found: {
    field: "form",
    text: "Không tìm thấy App ID này trong các Mini App riêng của xã. Tải lại trang rồi chọn lại.",
  },
  mini_app_inactive: {
    field: "form",
    text: "App ID này không còn chạy ở xã nên không đổi được. Tải lại trang để xem App ID đang chạy của xã.",
  },
  mini_app_already_running: {
    field: "form",
    text: "Xã đã có một Mini App riêng đang chạy. Muốn dùng App ID khác, chọn “Đổi App ID” ở dòng App ID đang chạy.",
  },
  // ADR 0070 §Sửa đổi 05/10/2026 #2: removal is permanent; the primary key keeps the removed row.
  mini_app_removed: {
    field: "appId",
    text: "App ID này đã bị gỡ trước đây nên không gắn lại được, kể cả cho chính xã đã dùng nó. Đăng ký Mini App mới trên Zalo rồi gắn App ID mới.",
  },
  mini_app_reactivation_removed: {
    field: "form",
    text: "App ID đã gỡ không bật lại được. Muốn xã có lại Mini App riêng, đăng ký Mini App mới trên Zalo rồi gắn App ID mới.",
  },
  mini_app_not_bound: {
    field: "form",
    text: "App ID này không còn gắn với xã (đã gỡ hoặc đã đổi) nên không đặt được khoá bí mật. Tải lại trang để xem App ID đang chạy của xã.",
  },
  invalid_secret: {
    field: "secret",
    text: "Khoá bí mật không hợp lệ: không để trống, không chứa khoảng trắng hay ký tự điều khiển. Sao chép lại khoá từ trang quản lý Mini App của Zalo.",
  },
  mini_app_secret_unavailable: {
    field: "form",
    text: "Hệ thống tạm thời không lưu được khoá bí mật, khoá chưa được đặt. Vui lòng thử lại sau ít phút.",
  },
};

/** `secret_retirement_error` of a change answer, as the next step to take. */
export function secretRetirementText(code: string | undefined): string {
  switch (code) {
    case "identity_unavailable":
      return "Hệ thống xác thực tạm không phản hồi. Thử thu hồi lại sau ít phút.";
    case "session_not_live":
      return "Phiên vận hành không còn hiệu lực ở hệ thống xác thực. Đăng nhập lại rồi thu hồi lại.";
    case "forbidden":
      return "Tài khoản vận hành của bạn không có quyền thu hồi khoá. Đề nghị người quản lý tài khoản vận hành cấp quyền.";
    default:
      return "Hệ thống xác thực từ chối thu hồi khoá. Thử thu hồi lại; nếu vẫn lỗi, báo bộ phận kỹ thuật.";
  }
}

export function communeError(err: ApiError): { field: CommuneField; text: string } | null {
  return COMMUNE_ERRORS[err.code] ?? null;
}

// --- wave 2 (ADR 0073) --------------------------------------------------------------------------

const REASON_TEXT = "Hãy ghi lý do, tối đa 500 ký tự.";

/** PUT /upload-policies/{purpose} (operator_platform.go `writePolicyError`). */
export type UploadPolicyField = "maxBytes" | "mimeTypes" | "maxFiles" | "reason" | "form";

const UPLOAD_POLICY_ERRORS: Record<string, { field: UploadPolicyField; text: string }> = {
  upload_policy_not_found: {
    field: "form",
    text: "Mục đích tải lên này không còn giới hạn nào để sửa. Tải lại trang rồi chọn lại.",
  },
  invalid_max_bytes: {
    field: "maxBytes",
    text: "Dung lượng tối đa phải lớn hơn 0 và không vượt mức trần của mục đích này.",
  },
  invalid_mime_types: {
    field: "mimeTypes",
    text: "Chọn ít nhất một kiểu tệp, và chỉ trong các kiểu mục đích này xử lý được.",
  },
  invalid_max_files: {
    field: "maxFiles",
    text: "Số tệp tối đa phải từ 1 đến 100, hoặc chọn “Không giới hạn”.",
  },
  invalid_reason: { field: "reason", text: REASON_TEXT },
};

export function uploadPolicyError(err: ApiError): { field: UploadPolicyField; text: string } | null {
  return UPLOAD_POLICY_ERRORS[err.code] ?? null;
}

/** GET/PUT /shared-mini-app (operator_launch.go). */
export type SharedMiniAppField = "appId" | "reason" | "form";

export const SHARED_APP_NOT_DECLARED = "Nền tảng chưa khai báo App ID của Mini App dùng chung.";

const SHARED_APP_AMBIGUOUS =
  "Sổ Mini App đang có hơn một Mini App dùng chung chạy cùng lúc. Báo nhóm nền tảng xử lý; không tự chọn App ID nào.";

const SHARED_APP_ERRORS: Record<string, { field: SharedMiniAppField; text: string }> = {
  shared_mini_app_not_declared: { field: "form", text: SHARED_APP_NOT_DECLARED },
  shared_mini_app_ambiguous: { field: "form", text: SHARED_APP_AMBIGUOUS },
  invalid_app_id: { field: "appId", text: "App ID chỉ gồm chữ số, tối đa 32 chữ số." },
  mini_app_taken: {
    field: "appId",
    text: "App ID này đã có trong sổ Mini App (của một xã, hoặc đã từng dùng). Mỗi App ID chỉ dùng cho một Mini App.",
  },
  invalid_reason: { field: "reason", text: REASON_TEXT },
};

export function sharedMiniAppError(err: ApiError): { field: SharedMiniAppField; text: string } | null {
  return SHARED_APP_ERRORS[err.code] ?? null;
}

/**
 * GET /communes/{id}/mini-app-launch-link refusals (409s of operator_launch.go), and the same codes
 * on its `unavailable` entries — one link the commune cannot get while the other is shown.
 * `toSharedAppPage` says the next step is on the "Mini App dùng chung" page, so the card links there.
 */
const LAUNCH_LINK_ERRORS: Record<string, { text: string; toSharedAppPage: boolean }> = {
  commune_inactive: {
    text: "Xã đang ngừng hoạt động nên không phát hành mã QR: mã QR sẽ đưa người dân tới một xã không còn nhận phản ánh.",
    toSharedAppPage: false,
  },
  commune_no_primary_domain: {
    text: "Xã chưa có tên miền chính dùng được trong liên kết mở Mini App. Kiểm tra mục Tên miền của xã; nếu xã đã có tên miền chính, báo nhóm nền tảng.",
    toSharedAppPage: false,
  },
  shared_mini_app_not_declared: {
    text: "Chưa khai báo Mini App dùng chung, nên chưa tạo được liên kết mở Mini App. Người giữ quyền Mini App khai báo App ID ở trang Mini App dùng chung trước.",
    toSharedAppPage: true,
  },
  shared_mini_app_ambiguous: { text: SHARED_APP_AMBIGUOUS, toSharedAppPage: false },
  own_mini_app_ambiguous: {
    text: "Xã đang có hơn một Mini App riêng chạy cùng lúc nên không biết mã QR phải mở app nào. Gỡ App ID thừa ở mục Mini App riêng của xã; hệ thống không tự chọn.",
    toSharedAppPage: false,
  },
  commune_not_found: { text: "Không tìm thấy xã. Quay lại danh sách xã và chọn lại.", toSharedAppPage: false },
};

export function launchLinkError(err: Pick<ApiError, "code">): { text: string; toSharedAppPage: boolean } | null {
  return LAUNCH_LINK_ERRORS[err.code] ?? null;
}

/** Tier-1 petition fields (operator_petition_fields.go `writeFieldError`). */
export type PetitionFieldFormField = "code" | "label" | "sortOrder" | "icon" | "tone" | "reason" | "form";

const PETITION_FIELD_ERRORS: Record<string, { field: PetitionFieldFormField; text: string }> = {
  petition_field_not_found: { field: "form", text: "Không còn mã lĩnh vực này. Tải lại trang rồi chọn lại." },
  petition_field_code_taken: {
    field: "code",
    text: "Mã này đã được cấp (kể cả mã đã ngừng dùng). Mã đã cấp không cấp lại; nếu là mã đã ngừng dùng, chọn “Dùng lại” ở dòng của mã đó.",
  },
  invalid_code: {
    field: "code",
    text: "Mã không đúng dạng: chữ thường không dấu và chữ số, các phần nối bằng dấu gạch ngang; tối đa 64 ký tự.",
  },
  invalid_label: { field: "label", text: "Nhãn mặc định không để trống, tối đa 200 ký tự, trên một dòng." },
  invalid_sort_order: { field: "sortOrder", text: "Thứ tự phải là số nguyên từ 1 đến 10000." },
  invalid_icon: {
    field: "icon",
    text: "Tên biểu tượng không hợp lệ: viết như tên biểu tượng lucide (ví dụ Trash2), hoặc để trống.",
  },
  invalid_tone: { field: "tone", text: "Tông màu không thuộc bộ cho phép. Chọn lại trong danh sách." },
  invalid_reason: { field: "reason", text: REASON_TEXT },
};

export function petitionFieldError(err: ApiError): { field: PetitionFieldFormField; text: string } | null {
  return PETITION_FIELD_ERRORS[err.code] ?? null;
}

/** The operator log's refusals (operator_platform.go `parseLogQuery`, core/page). */
const OPERATOR_LOG_ERRORS: Record<string, string> = {
  invalid_range: "Khoảng thời gian không hợp lệ: ngày bắt đầu phải trước hoặc bằng ngày kết thúc.",
  invalid_cursor: "Không tải tiếp được trang sau. Tải lại trang để xem lại từ đầu.",
  invalid_limit: "Số dòng mỗi trang không hợp lệ. Tải lại trang.",
  commune_not_found: "Không tìm thấy xã. Quay lại danh sách xã và chọn lại.",
};

export function operatorLogError(err: ApiError): string | null {
  return OPERATOR_LOG_ERRORS[err.code] ?? null;
}
