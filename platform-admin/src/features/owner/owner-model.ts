import type { ApiError, OperatorAccount, StaffAccount } from "@/lib/api";
import { passwordRejectionText } from "@/lib/errors";

import { formatDateFirst } from "../communes/commune-parts";

/**
 * The owner-only screens' decisions that are not drawing (ADR 0048 §Sửa đổi 13/10/2026): what a
 * dialog refuses before sending, how an account's state reads, and what each server refusal says.
 * Pure, so every case is tested without a DOM.
 *
 * NONE OF THIS IS THE GATE. The server re-checks the owner, the password rule, the reason, the 7-day
 * wait and the re-typed domain; these checks only spare a round trip and say the problem in place.
 */

export type OwnerField = "email" | "name" | "password" | "confirm" | "reason" | "confirmDomain" | "form";
export type OwnerError = { field: OwnerField; text: string };

export const REASON_MAX = 500;

export const TEMP_PASSWORD_HINT =
  "Máy chủ kiểm mật khẩu theo quy định độ dài của hệ thống; nếu chưa đạt, màn hình báo độ dài cần có. Mật khẩu tạm không hiện lại sau khi lưu.";

/** ADR 0048 §13/10 #6: the reset keeps the TOTP; the next sign-in is temporary password + own TOTP. */
export const OPERATOR_RESET_NOTE =
  "Lần đăng nhập tới người này dùng mật khẩu tạm và mã TOTP của chính họ, rồi phải đổi mật khẩu.";
export const OPERATOR_CREATE_NOTE = "Lần đầu đăng nhập người này phải đổi mật khẩu và đăng ký TOTP.";
export const COMMUNE_ADMIN_NOTE = "Người này phải đổi mật khẩu ở lần đăng nhập đầu tiên vào trang quản trị của xã.";
export const STAFF_RESET_NOTE =
  "Người này phải đổi mật khẩu ở lần đăng nhập tới vào trang quản trị của xã; mọi phiên đăng nhập hiện có của tài khoản bị thu hồi.";
export const SESSIONS_REVOKED_NOTE = "Mọi phiên đăng nhập hiện có của tài khoản bị thu hồi.";
export const PERMISSIONS_REDUCED_NOTE = "Bớt quyền thì mọi phiên đăng nhập hiện có của người này bị thu hồi.";

export const DELETION_EXPLANATION =
  "Xã sẽ biến khỏi danh sách, tên miền ngừng hoạt động và không cấp lại cho xã khác; dữ liệu của xã được giữ nguyên theo quy định lưu trữ; không khôi phục từ màn hình.";
export const DELETION_NEEDS_INACTIVE =
  "Chỉ xoá được xã đã ngừng hoạt động liên tục đủ 7 ngày. Xã đang hoạt động: ngừng hoạt động xã trước.";
export const DELETED_NOTICE =
  "Đã xoá xã. Xã không còn trong danh sách; tên miền của xã ngừng hoạt động và không cấp lại cho xã khác.";

export const ACCOUNTS_NOT_READY = "Chức năng đang được triển khai phía máy chủ, vui lòng thử lại sau";
export const ACCOUNTS_UNAVAILABLE = "Hệ thống tạm thời không quản lý được tài khoản. Vui lòng thử lại sau ít phút.";
export const CONFIRM_DOMAIN_MISMATCH = "Tên miền nhập lại không khớp";

const REASON_TEXT = `Hãy ghi lý do, tối đa ${REASON_MAX} ký tự.`;

// --- client-side checks ---------------------------------------------------------------------------

/** The temporary password typed twice. Its rule (length…) is the server's; only presence and match here. */
export function checkTemporaryPassword(password: string, confirm: string): OwnerError | null {
  if (password === "") return { field: "password", text: "Hãy nhập mật khẩu tạm." };
  if (password !== confirm) return { field: "confirm", text: "Hai lần nhập mật khẩu tạm không khớp. Hãy nhập lại." };
  return null;
}

export function checkReason(reason: string): OwnerError | null {
  const r = reason.trim();
  if (r === "") return { field: "reason", text: "Hãy ghi lý do." };
  if ([...r].length > REASON_MAX) return { field: "reason", text: REASON_TEXT };
  return null;
}

function checkEmail(email: string): OwnerError | null {
  const e = email.trim();
  if (e === "") return { field: "email", text: "Hãy nhập email đăng nhập." };
  if (!/^[^\s@]+@[^\s@]+$/.test(e)) return { field: "email", text: "Email không đúng dạng (ví dụ: ten@example.vn)." };
  return null;
}

function checkName(name: string, what: string): OwnerError | null {
  return name.trim() === "" ? { field: "name", text: `Hãy nhập ${what}.` } : null;
}

/** Create an operator account: email, display name, temporary password ×2. Keys may be none. */
export function checkCreateOperator(f: { email: string; displayName: string; password: string; confirm: string }): OwnerError | null {
  return checkEmail(f.email) ?? checkName(f.displayName, "tên hiển thị") ?? checkTemporaryPassword(f.password, f.confirm);
}

/** Create a commune's administrator: email, full name, temporary password ×2. */
export function checkCreateCommuneAdmin(f: { email: string; fullName: string; password: string; confirm: string }): OwnerError | null {
  return checkEmail(f.email) ?? checkName(f.fullName, "họ tên") ?? checkTemporaryPassword(f.password, f.confirm);
}

/** Reset a password (operator or staff): temporary password ×2 and a reason. */
export function checkReset(f: { password: string; confirm: string; reason: string }): OwnerError | null {
  return checkTemporaryPassword(f.password, f.confirm) ?? checkReason(f.reason);
}

/** The deletion button stays disabled until both are filled (the server judges the domain). */
export function canSubmitDeletion(confirmDomain: string, reason: string): boolean {
  return confirmDomain.trim() !== "" && reason.trim() !== "";
}

// --- how an account reads --------------------------------------------------------------------------

export type StatusChip = { tone: "info" | "danger" | "warning" | "neutral" | "success"; text: string };

/** A lock instant is live only while in the future; a past one is no lock (migration 0012). */
function liveLock(until: string | null, now: Date): string | null {
  if (until === null) return null;
  const t = new Date(until).getTime();
  return Number.isNaN(t) || t <= now.getTime() ? null : until;
}

export function operatorStatus(a: OperatorAccount, now: Date): StatusChip[] {
  const chips: StatusChip[] = [];
  if (a.is_owner) chips.push({ tone: "info", text: "Owner" });
  if (a.disabled) chips.push({ tone: "neutral", text: "Đã vô hiệu" });
  const lock = liveLock(a.locked_until, now);
  if (lock !== null) chips.push({ tone: "danger", text: `Bị khoá đến ${formatDateFirst(lock)}` });
  if (a.must_change_password) chips.push({ tone: "warning", text: "Phải đổi mật khẩu" });
  if (chips.length === 0) chips.push({ tone: "success", text: "Đang dùng" });
  return chips;
}

export function staffStatus(a: StaffAccount, now: Date): StatusChip[] {
  const chips: StatusChip[] = [];
  if (a.disabled) chips.push({ tone: "neutral", text: "Đã vô hiệu" });
  const lock = liveLock(a.sign_in_locked_until, now);
  if (lock !== null) chips.push({ tone: "danger", text: `Bị khoá đăng nhập đến ${formatDateFirst(lock)}` });
  if (a.must_change_password) chips.push({ tone: "warning", text: "Phải đổi mật khẩu" });
  if (chips.length === 0) chips.push({ tone: "success", text: "Đang dùng" });
  return chips;
}

export const STAFF_DISABLED_RESET =
  "Tài khoản đã bị xã vô hiệu nên không đặt lại mật khẩu được. Xã mở lại tài khoản trước.";

// --- server refusals -------------------------------------------------------------------------------

const ACCOUNT_ERRORS: Record<string, OwnerError> = {
  account_email_taken: { field: "email", text: "Email này đã có tài khoản. Dùng email khác." },
  invalid_account_input: {
    field: "form",
    text: "Thông tin tài khoản không hợp lệ: kiểm tra email, tên và các quyền đã chọn.",
  },
  invalid_reason: { field: "reason", text: REASON_TEXT },
  operator_account_not_found: {
    field: "form",
    text: "Không tìm thấy tài khoản vận hành này. Tải lại trang rồi chọn lại.",
  },
  staff_account_not_found: { field: "form", text: "Không tìm thấy tài khoản này ở xã. Tải lại trang rồi chọn lại." },
  staff_account_locked: {
    field: "form",
    text: "Tài khoản đang bị xã khoá nên không đặt lại mật khẩu được. Đề nghị xã mở khoá trước.",
  },
  commune_not_found: { field: "form", text: "Không tìm thấy xã. Quay lại danh sách xã và chọn lại." },
  commune_inactive: {
    field: "form",
    text: "Xã đang ngừng hoạt động nên không tạo được tài khoản. Bật hoạt động trở lại trước, nếu việc đó đúng.",
  },
  account_management_not_ready: { field: "form", text: ACCOUNTS_NOT_READY },
  account_management_unavailable: { field: "form", text: ACCOUNTS_UNAVAILABLE },
};

/**
 * The account routes' refusals. `operator_auth_unavailable` is not listed: `handleGuardedError` says
 * the auth service is down for any 503 the screen does not name.
 */
export function ownerAccountError(err: ApiError): OwnerError | null {
  if (err.code === "new_password_rejected") {
    return { field: "password", text: passwordRejectionText(err.passwordRejection, "mật khẩu đang dùng của tài khoản") };
  }
  return ACCOUNT_ERRORS[err.code] ?? null;
}

/** POST …/deletion refusals (operator_commune_deletion.go `writeDeletionError`). */
export function communeDeletionError(err: Pick<ApiError, "code" | "earliestDeletionAt">): OwnerError | null {
  switch (err.code) {
    case "commune_inactive_too_recent": {
      const at = err.earliestDeletionAt;
      const valid = at !== null && !Number.isNaN(new Date(at).getTime());
      return {
        field: "form",
        text: valid
          ? `Xã chưa ngừng hoạt động đủ 7 ngày. Có thể xoá từ ${formatDateFirst(at)}.`
          : "Xã chưa ngừng hoạt động đủ 7 ngày. Thử lại sau.",
      };
    }
    case "commune_active":
      return { field: "form", text: "Xã đang hoạt động. Chỉ xoá được xã đã ngừng hoạt động liên tục đủ 7 ngày." };
    case "confirm_domain_mismatch":
      return { field: "confirmDomain", text: CONFIRM_DOMAIN_MISMATCH };
    case "invalid_reason":
      return { field: "reason", text: REASON_TEXT };
    case "commune_not_found":
      return { field: "form", text: "Không tìm thấy xã — có thể xã đã bị xoá. Quay lại danh sách xã." };
    default:
      return null;
  }
}
