/**
 * The pure half of the operator log (ADR 0073 #2): verbs in words, the date filter as the server's
 * half-open RFC 3339 range, and before/after printed compactly. No React, tested on its own.
 */

/**
 * The verbs the operator log can hold today: audit_log `actor_kind = 'operator'` rows
 * (service-platform/internal/store/operator_writes.go `Action…`) and platform_audit_log rows
 * (upload_policy.go, shared_mini_app.go, petition_field.go, migration seeds). An UNKNOWN verb is
 * shown raw, never guessed: a new verb reaching this screen before this table is a gap someone can
 * see, not a sentence that misdescribes an act.
 */
const ACTION_LABELS: Record<string, string> = {
  tao_xa: "Tạo xã",
  them_ten_mien: "Thêm tên miền",
  dat_ten_mien_chinh: "Đặt tên miền chính",
  sua_ten_xa: "Sửa lỗi gõ trong tên xã",
  ngung_hoat_dong_xa: "Ngừng hoạt động xã",
  mo_lai_hoat_dong_xa: "Bật hoạt động trở lại cho xã",
  gan_mini_app: "Gắn Mini App riêng",
  tat_mini_app: "Gỡ (tắt) Mini App riêng",
  bat_lai_mini_app: "Bật lại Mini App riêng",
  dat_khoa_mini_app: "Đặt khoá bí mật App ID",
  thu_hoi_khoa_mini_app: "Thu hồi khoá bí mật App ID",
  "upload_policy.changed": "Sửa giới hạn tải lên",
  "upload_policy.seeded": "Khởi tạo giới hạn tải lên",
  "shared_mini_app.changed": "Khai báo / đổi Mini App dùng chung",
  "petition_field.created": "Cấp mã lĩnh vực phản ánh",
  "petition_field.changed": "Sửa lĩnh vực phản ánh",
  "petition_field.deactivated": "Ngừng dùng lĩnh vực phản ánh",
  "petition_field.reactivated": "Dùng lại lĩnh vực phản ánh",
};

export function actionLabel(action: string): string {
  return ACTION_LABELS[action] ?? action;
}

export const VALUE_MAX_CHARS = 160;

/**
 * One line for a before / after value. Printed as TEXT (React escapes it), never parsed field by
 * field: the shape differs per action, and a screen that assumed a shape would hide whatever does
 * not fit it. Long values are clipped; null / absent is "".
 */
export function compactValue(v: unknown): string {
  if (v === null || v === undefined) return "";
  let s: string;
  if (typeof v === "string") s = v;
  else {
    try {
      s = JSON.stringify(v) ?? "";
    } catch {
      s = "";
    }
  }
  return s.length > VALUE_MAX_CHARS ? s.slice(0, VALUE_MAX_CHARS - 1) + "…" : s;
}

/**
 * The business day is Asia/Ho_Chi_Minh, a fixed +07:00 (no daylight saving). A `to` date is
 * INCLUSIVE for the person, so the request's exclusive end is the start of the next day.
 */
const DAY = /^(\d{4})-(\d{2})-(\d{2})$/;
const OFFSET = "+07:00";

function nextDay(date: string): string | null {
  const m = DAY.exec(date);
  if (!m) return null;
  const t = Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]) + 1);
  return Number.isNaN(t) ? null : new Date(t).toISOString().slice(0, 10);
}

export type LogRange = { from?: string; to?: string };

/** The `<input type="date">` values as the query's range, or a sentence saying what to fix. */
export function logRange(fromDate: string, toDate: string): { ok: true; range: LogRange } | { ok: false; text: string } {
  const range: LogRange = {};
  if (fromDate !== "") {
    if (!DAY.test(fromDate)) return { ok: false, text: "Ngày bắt đầu không hợp lệ." };
    range.from = `${fromDate}T00:00:00${OFFSET}`;
  }
  if (toDate !== "") {
    const end = nextDay(toDate);
    if (end === null) return { ok: false, text: "Ngày kết thúc không hợp lệ." };
    range.to = `${end}T00:00:00${OFFSET}`;
  }
  if (fromDate !== "" && toDate !== "" && fromDate > toDate) {
    return { ok: false, text: "Ngày bắt đầu phải trước hoặc bằng ngày kết thúc." };
  }
  return { ok: true, range };
}
