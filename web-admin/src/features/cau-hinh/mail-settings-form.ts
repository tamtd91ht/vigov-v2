/**
 * The decision half of the "Máy chủ thư" tab (spec `10-may-chu-thu.md`, ADR 0079). Pure, so the rules
 * about the password — the one field of this screen that must never travel the wrong way — have tests:
 *
 *   1. the password box is NEVER pre-filled: no response carries the password, and the draft built
 *      from a response starts it at "" whatever the response holds;
 *   2. a blank box is NEVER sent (it means "keep the stored password");
 *   3. changing host, port or username with a password already stored means it must be typed
 *      again — the server refuses otherwise (400 `password_required_for_new_host`: the old password
 *      is never sent somewhere new), and the screen says so in place with §10's sentence.
 */

import { formatDateTime } from "@/features/dashboard/period";
import type { comms_mailLastTestOut, comms_mailSettingsIn, comms_mailSettingsOut } from "@/lib/api/schema.gen";

/**
 * The two security modes, as the spec's two tick boxes. ONE server field (`security`), so the boxes
 * are exclusive BY CONSTRUCTION: each box is "checked" when the field holds its value, and ticking a
 * box writes its value. There is no third state to reach — no plaintext SMTP (rule 13), which the
 * server also refuses (`ErrMailSecurityUnknown`).
 */
export const MAIL_SECURITY: readonly { readonly value: string; readonly label: string }[] = [
  { value: "starttls", label: "STARTTLS (thường dùng với cổng 587)" },
  { value: "tls", label: "TLS ngay từ đầu (thường dùng với cổng 465)" },
];

export type MailDraft = {
  readonly host: string;
  readonly port: number;
  readonly security: string;
  readonly username: string;
  /** Typed this session only. Starts "" — always. */
  readonly password: string;
  readonly fromAddress: string;
  readonly fromName: string;
  readonly isEnabled: boolean;
};

/**
 * Draft from the server's answer. `password` IS "" BY CONSTRUCTION, not copied from anything: the
 * response type has no password field today, and this line keeps it empty on the day someone adds one.
 */
export function draftFromSettings(s: comms_mailSettingsOut): MailDraft {
  return {
    host: s.host,
    port: s.port,
    security: s.security,
    username: s.username,
    password: "",
    fromAddress: s.from_address,
    fromName: s.from_name,
    isEnabled: s.is_enabled,
  };
}

/**
 * Host, port or username differ from what is saved — the three that decide where the password goes.
 * Compared AFTER the server's own normalisation (`NormalizeMailSettings`: host trimmed and lower-cased,
 * username trimmed), so retyping the same host in capitals is not "a new destination".
 */
export function destinationChanged(saved: comms_mailSettingsOut, d: MailDraft): boolean {
  return (
    d.host.trim().toLowerCase() !== saved.host || d.port !== saved.port || d.username.trim() !== saved.username
  );
}

/**
 * The in-place sentence for a refused save. When the draft is exactly the server's
 * `password_required_for_new_host` case (a password stored, the destination changed, nothing typed),
 * §10's sentence; otherwise the server's own sentence, verbatim.
 *
 * DECIDED FROM THE DRAFT, NOT FROM THE RESPONSE'S `code`: `saveMailSettings` (lib/api) does not ask
 * `goiGhi` for the code, and `KetQua.code` is reserved for one documented caller. The condition is the
 * server's own (`app/mail_settings.go` Save), so the two cannot disagree on a request this screen sent.
 */
export function saveRefusalMessage(saved: comms_mailSettingsOut, d: MailDraft, serverMessage: string): string {
  return saved.password_set && d.password === "" && destinationChanged(saved, d) ? PASSWORD_NEW_DESTINATION : serverMessage;
}

/** PUT body. Built field by field; `password` only when typed (see `saveMailSettings`). */
export function saveBody(d: MailDraft): comms_mailSettingsIn {
  const body: comms_mailSettingsIn = {
    host: d.host.trim(),
    port: d.port,
    security: d.security,
    username: d.username.trim(),
    from_address: d.fromAddress.trim(),
    from_name: d.fromName,
    is_enabled: d.isEnabled,
  };
  if (d.password !== "") body.password = d.password;
  return body;
}

/**
 * The `Idempotency-Key` for a test send: kept while the same send is retried (same recipient, no
 * success yet), minted otherwise. `mint` is a parameter so tests count calls.
 */
export function testKey(
  current: { readonly key: string; readonly recipient: string } | null,
  recipient: string,
  mint: () => string,
): { readonly key: string; readonly recipient: string } {
  return current !== null && current.recipient === recipient ? current : { key: mint(), recipient };
}

// No emoji: the screen draws lucide `Mail` / `Save` / `TriangleAlert` (ADR 0068 §2).
export const MAIL_TITLE = "Máy chủ thư của xã";
/**
 * Spec §10 #2, FIRST SENTENCE ONLY. Its second sentence ("Chưa khai thì hệ thống dùng máy chủ thư của
 * nền tảng nếu có.") is not true of this system: service-comms has no platform mail server and no
 * fallback path — the mail sender is wired to the commune's own settings only (`cmd/server/main.go`).
 * And the owner decided none will be built this round (ADR 0079 lô 2 Q1 #8: `platform_fallback` false).
 */
export const MAIL_DESCRIPTION =
  "Thông báo nội bộ gửi tới hộp thư cán bộ bằng chính địa chỉ công vụ của xã.";
/** True whenever the commune's server is not saved or not switched on: there is no other mail path. */
export const NOT_CONFIGURED_WARNING = "Chưa có máy chủ thư nào. Thông báo vẫn đăng được nhưng thư sẽ không đi.";
export const ENCRYPTION_MISSING =
  "Nền tảng chưa cấu hình khoá mã hoá bí mật, nên chưa lưu được mật khẩu máy chủ thư và chưa gửi thử " +
  "được. Biểu mẫu tạm chỉ để xem. Hãy báo đơn vị vận hành hệ thống.";
export const PORT_HINT = "587 dùng STARTTLS, 465 dùng TLS ngay từ đầu.";
export const PASSWORD_KEPT_PLACEHOLDER = "Giữ nguyên mật khẩu cũ";
export const PASSWORD_NEW_DESTINATION =
  "Đổi máy chủ, cổng hoặc tài khoản thì phải gõ lại mật khẩu. Mật khẩu cũ không đi theo sang máy chủ mới.";
export const ENABLE_LABEL = "Dùng máy chủ thư này cho xã";
export const SAVE_BUTTON = "Lưu cấu hình";
export const SAVED_SENTENCE = "Đã lưu cấu hình máy chủ thư.";
export const TEST_LABEL = "Gửi thư thử tới";
export const TEST_BUTTON = "Gửi thử";
/** Under the locked test box until a configuration is saved (spec 10, user 09/10/2026). */
export const TEST_SAVE_FIRST_HINT = "Lưu cấu hình trước khi gửi thử.";
export const TEST_SENT_TOAST = "Đã gửi. Kiểm tra hộp thư.";
/** The request never reached an answer from service-comms (network, or a proxy page instead of it). */
export const CALL_FAILED = "Không gọi được máy chủ.";

/**
 * Placeholders name NO commune and NO vendor (rule 1 inv 10, ADR 0068 lần 6 #2): one bundle serves
 * every commune, so a real domain here is one commune's address shown to all the others.
 */
export const HOST_PLACEHOLDER = "smtp.ten-mien-xa.gov.vn";
export const FROM_ADDRESS_PLACEHOLDER = "ubnd@ten-mien-xa.gov.vn";
export const TEST_RECIPIENT_PLACEHOLDER = "canbo@ten-mien-xa.gov.vn";

/**
 * One short sentence per `error_class` the server stores (`service-comms/internal/domain/mail_settings.go`
 * `MailTestError*`, CHECK of migration 0019). Each is the first clause of the advice sentence the test
 * route returns for the same category (`internal/http/mail_settings.go` `mailSendFailures`): the stored
 * line names WHAT failed; the full advice was shown when the test ran.
 */
export const LAST_TEST_ERRORS: Readonly<Record<string, string>> = {
  "khong-ket-noi": "không kết nối được tới máy chủ thư",
  "het-thoi-gian": "máy chủ thư không trả lời kịp",
  "chung-chi-khong-hop-le": "chứng chỉ của máy chủ thư không xác minh được",
  "loi-tls": "không thiết lập được kết nối mã hoá",
  "khong-co-starttls": "máy chủ thư không hỗ trợ START TLS ở cổng này",
  "tu-choi-khong-ma-hoa": "hệ thống không gửi thư qua kết nối không mã hoá",
  "khong-ho-tro-dang-nhap": "máy chủ thư không cho đăng nhập bằng tài khoản và mật khẩu",
  "sai-tai-khoan": "máy chủ thư từ chối tài khoản hoặc mật khẩu",
  "tu-choi-dia-chi": "máy chủ thư từ chối địa chỉ gửi hoặc địa chỉ nhận",
  "sai-giao-thuc": "máy chủ thư trả lời không đúng giao thức SMTP",
  khac: "không gửi được",
};

export const LAST_TEST_OK = "gửi được";

/** A failure the server stored WITHOUT a class — the prototype's own fallback (`EmailSettingPanel.tsx:279`). */
export const LAST_TEST_NO_DETAIL = "hỏng";

/**
 * "Lần thử gần nhất {thời gian} tới {email}: gửi được | {lỗi}" (spec 10 #7). `to` is the server's
 * MASKED recipient, shown as is (rule 3). A class this screen does not know reads as "không gửi được",
 * no class at all as the prototype's "hỏng" — both still a failure, never "gửi được", so a new server
 * class cannot turn into a false success line.
 * An `at` that does not parse is shown verbatim rather than as "NaN:NaN".
 */
export function lastTestSentence(t: comms_mailLastTestOut): string {
  const instant = Date.parse(t.at);
  const when = Number.isNaN(instant) ? t.at : formatDateTime(instant);
  const outcome = t.ok
    ? LAST_TEST_OK
    : t.error_class === null
      ? LAST_TEST_NO_DETAIL
      : (LAST_TEST_ERRORS[t.error_class] ?? LAST_TEST_ERRORS.khac);
  return `Lần thử gần nhất ${when} tới ${t.to}: ${outcome}`;
}

export function testFailedToast(error: string): string {
  return `Không gửi được: ${error}`;
}

export function testSentSentence(recipient: string): string {
  return `Máy chủ thư đã nhận thư thử gửi tới ${recipient}. Hãy kiểm tra hộp thư (kể cả mục thư rác).`;
}
