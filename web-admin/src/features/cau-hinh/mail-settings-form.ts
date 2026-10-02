/**
 * The decision half of the "Máy chủ thư" tab (`docs/ui-ux/14-cau-hinh.md §10`). Pure, so the rules
 * about the password — the one field of this screen that must never travel the wrong way — have tests:
 *
 *   1. the password box is NEVER pre-filled: no response carries the password, and the draft built
 *      from a response starts it at "" whatever the response holds;
 *   2. a blank box is NEVER sent (it means "keep the stored password");
 *   3. changing host, port or username with a password already stored means it must be typed
 *      again — said BEFORE saving, because the server refuses otherwise (400
 *      `password_required_for_new_host`: the old password is never sent somewhere new).
 */

import type { comms_mailSettingsIn, comms_mailSettingsOut } from "@/lib/api/schema.gen";

/** The four ports the server accepts (`service-comms/internal/domain/mail_settings.go`, `allowedMailPorts`). */
export const MAIL_PORTS: readonly number[] = [587, 465, 25, 2525];

/** The two security modes; there is no plaintext one. Labels from §10. */
export const MAIL_SECURITY: readonly { readonly value: string; readonly label: string }[] = [
  { value: "starttls", label: "START TLS (thường dùng với cổng 587)" },
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

/** Host, port or username differ from what is saved — the three that decide where the password goes. */
export function destinationChanged(saved: comms_mailSettingsOut, d: MailDraft): boolean {
  return d.host.trim() !== saved.host || d.port !== saved.port || d.username.trim() !== saved.username;
}

/**
 * The note under the password box, or "" for none:
 *   - nothing stored yet → the first save needs it;
 *   - stored, and the destination changed, and nothing typed → it must be typed again.
 */
export function passwordNote(saved: comms_mailSettingsOut, d: MailDraft): string {
  if (!saved.password_set) return "Lần lưu đầu tiên phải nhập mật khẩu của tài khoản thư.";
  if (destinationChanged(saved, d) && d.password === "") {
    return (
      "Bạn đã đổi máy chủ, cổng hoặc tài khoản nên phải nhập lại mật khẩu. Mật khẩu đã lưu không " +
      "được gửi tới một nơi chưa từng dùng nó."
    );
  }
  return "";
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

// No emoji in these three: the screen draws lucide `Mail` / `Save` / `Send` (ADR 0068 §2).
export const MAIL_TITLE = "Máy chủ thư của xã";
export const MAIL_DESCRIPTION =
  "Thông báo nội bộ gửi từ hộp thư của cán bộ bằng chính địa chỉ công vụ của xã. Chưa khai thì hệ " +
  "thống dùng máy chủ thư của tỉnh nếu tỉnh có.";
export const NOT_CONFIGURED_WARNING =
  "Chưa có máy chủ thư nào. Thông báo vẫn đúng đường nhưng chưa thật sự gửi đi."; // ⚠ is drawn as a lucide icon (ADR 0068 §2)
export const ENCRYPTION_MISSING =
  "Nền tảng chưa cấu hình khoá mã hoá bí mật, nên chưa lưu được mật khẩu máy chủ thư và chưa gửi thử " +
  "được. Biểu mẫu tạm chỉ để xem. Hãy báo đơn vị vận hành hệ thống.";
export const PASSWORD_SAVED = "Đã lưu mật khẩu. Để trống ô này nếu không đổi mật khẩu.";
export const PORT_HINT = "587 dùng START TLS, 465 dùng TLS ngay từ đầu.";
export const SAVE_BUTTON = "Lưu cấu hình";
export const SAVED_SENTENCE = "Đã lưu cấu hình máy chủ thư.";
export const TEST_LABEL = "Gửi thư thử tới";
export const TEST_BUTTON = "Gửi thử";
export const TEST_SAVED_ONLY =
  "Thư thử được gửi bằng cấu hình ĐÃ LƯU — hãy lưu trước nếu vừa sửa biểu mẫu.";

export function testSentSentence(recipient: string): string {
  return `Máy chủ thư đã nhận thư thử gửi tới ${recipient}. Hãy kiểm tra hộp thư (kể cả mục thư rác).`;
}
