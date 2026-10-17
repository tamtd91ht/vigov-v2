/**
 * Cấu hình → Kênh SMS (ADR 0099 #2 #3) — the pure half: the words, the result and network labels, the
 * balance format, the dialog draft and the body it becomes, and which field a refusal belongs under.
 * Every check here is a HINT: comms re-checks each rule and its sentence still reaches the screen.
 *
 * NO PROTOTYPE FOR THIS TAB: built on "Máy chủ thư" (write-only secret, "Kiểm tra kết nối") and "Con bot
 * của xã" / "OA Zalo của xã" ("đặt lúc … bởi …", a confirmation with a reason) — ADR 0068 lần 6.
 */

import { SMS_ERROR, type comms_smsAccountCreateIn, type comms_smsAccountOut, type comms_smsAccountUpdateIn } from "@/lib/api/sms-accounts";

export const SMS_DESCRIPTION =
  "Tài khoản eSMS dùng để gửi tin nhắn SMS thương hiệu tới người dân. Xã có thể khai nhiều tài khoản và chọn tài khoản khi gửi.";

export const NO_ACCOUNTS = "Xã chưa khai tài khoản eSMS nào.";
export const ADD_BUTTON = "Thêm tài khoản";
export const DEFAULT_BADGE = "Mặc định";

/** Under ApiKey / SecretKey when editing: the stored key is never shown, an empty box keeps it. */
export const KEY_KEPT_HINT = "Để trống thì giữ khoá đang lưu. Khoá đã lưu không bao giờ hiện lại.";
/** Under the keys when adding. */
export const KEY_NEW_HINT = "Chép nguyên từ trang quản trị eSMS. Hệ thống hỏi eSMS bằng khoá này trước khi lưu.";
/** After a refused save the keys are gone from the form (they are never kept in memory past a send). */
export const KEY_RETYPE_HINT = "Vì an toàn, khoá đã gõ được xoá sau mỗi lần gửi. Hãy nhập lại nếu cần đổi khoá.";

export const SAVED_TOAST = "Đã lưu tài khoản SMS.";
export const DEFAULT_SET_TOAST = "Đã đặt làm tài khoản mặc định.";
export const RETIRED_TOAST = "Đã ngừng dùng tài khoản SMS.";
/** A write that failed with no explained answer (`LOI_KHONG_RO`). */
export const SAVE_FAILED_TOAST = "Chưa lưu được. Vui lòng thử lại.";
export const CHECK_FAILED_TOAST = "Không kiểm tra được.";

export const RETIRE_REASON_MAX = 200;
export const RETIRE_CONSEQUENCE =
  "Tài khoản sẽ không còn chọn được khi gửi tin. Các đợt đã gửi vẫn giữ nguyên. Nếu đây là tài khoản mặc định, xã sẽ không còn tài khoản mặc định cho tới khi đặt tài khoản khác.";

export const TEMPLATES_TITLE = "Mẫu tin SMS";
export const TEMPLATES_DESCRIPTION =
  "Mẫu tin CSKH của Brandname đã đăng ký với nhà mạng trên eSMS. Chỉ gửi được bằng mẫu đã đăng ký.";
export const NO_TEMPLATES = "Chưa nạp mẫu nào. Bấm “Nạp mẫu từ eSMS” sau khi mẫu được nhà mạng duyệt.";
export const SYNC_BUTTON = "Nạp mẫu từ eSMS";

// ---- check results ----------------------------------------------------------------------------------

/**
 * The sentence for each class comms records (migration 0032). ESMS IS NEVER QUOTED. `long` is the toast
 * after "Kiểm tra kết nối"; `short` the "Lần kiểm gần nhất" line.
 */
const RESULT_TEXT: Readonly<Record<string, { long: string; short: string }>> = {
  "thanh-cong": { long: "Kết nối tốt: eSMS nhận khoá và Brandname của tài khoản.", short: "Kết nối tốt" },
  "sai-khoa": { long: "eSMS không nhận ApiKey / SecretKey đã lưu. Hãy sửa tài khoản và nhập lại khoá.", short: "Sai khoá" },
  "brandname-chua-kich-hoat": {
    long: "Brandname chưa được kích hoạt trên tài khoản eSMS (hoặc viết chưa đúng). Hãy liên hệ eSMS.",
    short: "Brandname chưa kích hoạt",
  },
  "khong-kha-dung": { long: "Chưa hỏi được eSMS. Vui lòng thử lại sau ít phút.", short: "eSMS không trả lời" },
  "bi-tu-choi": {
    long: "eSMS từ chối yêu cầu (tài khoản có thể đang bị khoá, hoặc máy chủ chưa được eSMS cho phép). Hãy liên hệ eSMS.",
    short: "eSMS từ chối",
  },
  "phan-hoi-sai-dang": {
    long: "eSMS trả lời theo dạng hệ thống không đọc được. Hãy báo đơn vị vận hành hệ thống kèm thời điểm thử.",
    short: "eSMS trả lời sai dạng",
  },
};

/** A class this build does not know is a failure of unknown cause — never a success. */
const UNKNOWN_RESULT = {
  long: "Không rõ kết quả lần hỏi eSMS. Hãy báo đơn vị vận hành hệ thống kèm thời điểm thử.",
  short: "Không rõ kết quả",
};

function resultText(result: string): { long: string; short: string } {
  // Own keys only: `"toString" in RESULT_TEXT` is true and would print a function as a sentence.
  return (Object.prototype.hasOwnProperty.call(RESULT_TEXT, result) ? RESULT_TEXT[result] : undefined) ?? UNKNOWN_RESULT;
}

export const isCheckOk = (result: string): boolean => result === "thanh-cong";

/** The short Vietnamese label of a check class — "Kết nối tốt", "Sai khoá", … */
export function checkResultLabel(result: string): string {
  return resultText(result).short;
}

/** The toast after "Kiểm tra kết nối"; with the balance when there is one. */
export function checkToastText(result: string, balance: string | null): string {
  const base = resultText(result).long;
  return isCheckOk(result) && balance !== null ? `${base} Số dư: ${formatBalance(balance)}.` : base;
}

/** "Lần kiểm gần nhất: {at} · Kết nối tốt · Số dư 36.693 đ" — `at` already formatted by the caller. */
export function lastCheckText(at: string, result: string, balance: string | null): string {
  const tail = isCheckOk(result) && balance !== null ? ` · Số dư ${formatBalance(balance)}` : "";
  return `Lần kiểm gần nhất: ${at} · ${checkResultLabel(result)}${tail}`;
}

/** "Khoá đặt lúc {at} · bởi {code}" — `set_by` is the staff code (rule 6 inv. 8); comms sends no name. */
export function setByText(at: string, code: string): string {
  return `Khoá đặt lúc ${at} · bởi ${code}`;
}

/**
 * eSMS's balance, DECIMAL TEXT ("36693.00"), in the vi-VN shape "36.693 đ" — done on the digits, never
 * through a float (a balance is money). A fraction that is not all zeros is kept ("1.234,5 đ"). Anything
 * that is not a plain decimal is shown as comms sent it, never guessed.
 */
export function formatBalance(raw: string): string {
  const m = /^(-?)(\d+)(?:\.(\d+))?$/.exec(raw.trim());
  if (m === null) return `${raw} đ`;
  const [, sign, intPart, frac] = m;
  const digits = (intPart ?? "0").replace(/^0+(?=\d)/, "");
  const grouped = digits.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  const fraction = (frac ?? "").replace(/0+$/, "");
  return `${sign}${grouped}${fraction !== "" ? `,${fraction}` : ""} đ`;
}

// ---- templates ---------------------------------------------------------------------------------------

/** eSMS NetworkID → network name (`sms_accounts.go` smsTemplateNetworkOut). An unknown id is shown as is. */
const NETWORKS: Readonly<Record<number, string>> = {
  1: "Viettel",
  2: "Mobifone",
  3: "Vinaphone",
  4: "Vietnamobile",
  5: "Gtel",
  6: "Itel",
  7: "Reddi",
};

export function networkLabel(id: number): string {
  return Object.prototype.hasOwnProperty.call(NETWORKS, id) ? (NETWORKS[id] ?? `Nhà mạng ${id}`) : `Nhà mạng ${id}`;
}

/** "P1 ≤20" — one slot of a template. */
export function paramText(p: { name: string; max_length: number }): string {
  return `${p.name} ≤${p.max_length}`;
}

/** The "Nạp mẫu từ eSMS" toast: the server's four counts, never estimated here. */
export function syncResultText(r: { added: number; updated: number; vanished: number; skipped: number }): string {
  const parts = [`thêm ${r.added}`, `cập nhật ${r.updated}`, `không còn trên eSMS ${r.vanished}`];
  if (r.skipped > 0) parts.push(`bỏ qua ${r.skipped} mẫu không lưu được`);
  return `Đã nạp mẫu: ${parts.join(", ")}.`;
}

/** "Nạp lúc …" — the latest `synced_at` of the list; null when nothing was ever loaded. */
export function latestSyncedAt(items: readonly { synced_at: string }[]): string | null {
  let latest: string | null = null;
  for (const t of items) if (latest === null || Date.parse(t.synced_at) > Date.parse(latest)) latest = t.synced_at;
  return latest;
}

// ---- the add / edit dialog ---------------------------------------------------------------------------

/** What the dialog holds. `apiKey` / `secretKey` start "" and are NEVER filled from the server. */
export type SmsAccountDraft = {
  label: string;
  apiKey: string;
  secretKey: string;
  brandname: string;
  isDefault: boolean;
};

export function emptyDraft(firstAccount: boolean): SmsAccountDraft {
  // The first account of a commune is offered as the default: with none, the send screen has nothing preselected.
  return { label: "", apiKey: "", secretKey: "", brandname: "", isDefault: firstAccount };
}

export function draftFromAccount(a: comms_smsAccountOut): SmsAccountDraft {
  return { label: a.label, apiKey: "", secretKey: "", brandname: a.brandname, isDefault: a.is_default };
}

/** The fields a refusal can be drawn under. `form` = above the buttons. */
export type SmsField = "label" | "keys" | "brandname" | "form";
export type SmsFieldErrors = Partial<Record<SmsField, string>>;

const LABEL_REQUIRED = "Cần nhập tên gợi nhớ.";
const BRANDNAME_REQUIRED = "Cần nhập Brandname.";
const KEYS_REQUIRED = "Tài khoản mới cần nhập cả ApiKey và SecretKey.";
const ONE_KEY_ONLY = "Đổi khoá thì nhập cả ApiKey và SecretKey.";

/**
 * The create body, or the field errors that stop it before a request. Label and Brandname are trimmed
 * (comms refuses control characters and leading spaces anyway); the keys go VERBATIM.
 */
export function buildCreate(d: SmsAccountDraft): { ok: true; body: comms_smsAccountCreateIn } | { ok: false; errors: SmsFieldErrors } {
  const errors: SmsFieldErrors = {};
  const label = d.label.trim();
  const brandname = d.brandname.trim();
  if (label === "") errors.label = LABEL_REQUIRED;
  if (brandname === "") errors.brandname = BRANDNAME_REQUIRED;
  if (d.apiKey === "" || d.secretKey === "") errors.keys = KEYS_REQUIRED;
  if (Object.keys(errors).length > 0) return { ok: false, errors };
  return { ok: true, body: { label, api_key: d.apiKey, secret_key: d.secretKey, brandname, is_default: d.isDefault } };
}

/**
 * The PATCH body: only what changed. Empty keys = keep. One key without the other is refused here — an
 * ApiKey and a SecretKey are a pair on eSMS, and half a pair is a save eSMS will reject.
 * `empty: true` when nothing changed (no request is sent).
 */
export function buildUpdate(
  saved: comms_smsAccountOut,
  d: SmsAccountDraft,
): { ok: true; body: comms_smsAccountUpdateIn; empty: boolean } | { ok: false; errors: SmsFieldErrors } {
  const errors: SmsFieldErrors = {};
  const label = d.label.trim();
  const brandname = d.brandname.trim();
  if (label === "") errors.label = LABEL_REQUIRED;
  if (brandname === "") errors.brandname = BRANDNAME_REQUIRED;
  if ((d.apiKey === "") !== (d.secretKey === "")) errors.keys = ONE_KEY_ONLY;
  if (Object.keys(errors).length > 0) return { ok: false, errors };
  const body: comms_smsAccountUpdateIn = {};
  if (label !== saved.label) body.label = label;
  if (brandname !== saved.brandname) body.brandname = brandname;
  if (d.isDefault !== saved.is_default) body.is_default = d.isDefault;
  if (d.apiKey !== "") {
    body.api_key = d.apiKey;
    body.secret_key = d.secretKey;
  }
  return { ok: true, body, empty: Object.keys(body).length === 0 };
}

/**
 * Which field a refused save is drawn under, by the server's CODE (never its wording). The sentence is
 * the server's, verbatim. `sms_account_invalid` covers several fields with one code, so it goes above the
 * buttons rather than under a guessed field.
 */
export function refusalField(code: string | undefined): SmsField {
  switch (code) {
    case SMS_ERROR.labelTaken:
      return "label";
    case SMS_ERROR.keyRejected:
      return "keys";
    case SMS_ERROR.brandnameInactive:
      return "brandname";
    default:
      return "form";
  }
}

/** The retire reason, trimmed, or why not (comms: required, at most 200 characters). */
export function checkRetireReason(raw: string): { ok: true; reason: string } | { ok: false; text: string } {
  const reason = raw.trim();
  if (reason === "" || [...reason].length > RETIRE_REASON_MAX) {
    return { ok: false, text: `Cần nhập lý do ngừng dùng (tối đa ${RETIRE_REASON_MAX} ký tự).` };
  }
  return { ok: true, reason };
}
