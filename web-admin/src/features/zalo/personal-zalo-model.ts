/**
 * The pure half of "Nhận nhắc việc qua Zalo" on `/ca-nhan` (ADR 0074): countdown, polling decision,
 * the sentences. No React, no fetch, so every branch is tested without rendering.
 */

/** Poll `GET zalo-links/current` this often while a pairing code is alive (owner brief, 05/10/2026). */
export const POLL_INTERVAL_MS = 5_000;

export const CARD_TITLE = "Nhận nhắc việc qua Zalo";

export const CARD_DESCRIPTION =
  "Nhận nhắc việc sắp đến hạn, quá hạn và bản tin tuần qua Zalo, bên cạnh chuông thông báo. Chưa ghép nối thì bạn vẫn nhận đủ thông báo trên chuông.";

export const CHANNEL_OFF =
  "Xã chưa bật kênh nhắc việc qua Zalo, nên chưa ghép nối hay gửi tin thử được. Khi quản trị viên của xã bật kênh ở Cấu hình → Kênh Zalo, bạn ghép nối tại đây.";

export const CHANNEL_OFF_LINKED =
  "Xã đang tắt kênh nhắc việc qua Zalo: Zalo của bạn vẫn được ghép nối nhưng hệ thống không gửi tin nào cho tới khi xã bật lại.";

export const NOT_LINKED = "Tài khoản của bạn chưa ghép nối với Zalo.";

export const PAIR_STEPS = [
  "Bấm “Mở Zalo” để mở cuộc trò chuyện với bot (hoặc tìm bot trên Zalo).",
  "Gửi mã này cho bot, đúng 8 ký tự.",
  "Chờ vài giây: trang này tự cập nhật khi ghép nối xong.",
] as const;

export const CODE_EXPIRED = "Mã đã hết hạn và không dùng được nữa. Lấy mã mới để ghép nối.";

export const LINKED_NOW = "Ghép nối thành công. Từ giờ bạn nhận nhắc việc qua Zalo.";

export const TEST_SENT = "Đã gửi tin thử. Mở Zalo để kiểm tra; nếu không thấy tin, gỡ ghép nối rồi ghép lại.";

export const UNLINK_QUESTION = "Gỡ ghép nối Zalo?";

export const UNLINK_CONSEQUENCE =
  "Bạn sẽ không nhận nhắc việc qua Zalo nữa; chuông thông báo vẫn hoạt động như cũ. Muốn nhận lại, bạn ghép nối lại bằng mã mới.";

export const UNLINKED = "Đã gỡ ghép nối Zalo.";

/** Whole seconds left before `expiresAt`; 0 once past, and 0 for an unreadable instant (fail closed). */
export function secondsLeft(expiresAt: string, now: number): number {
  const t = Date.parse(expiresAt);
  if (Number.isNaN(t)) return 0;
  return Math.max(0, Math.ceil((t - now) / 1000));
}

/** `mm:ss`. */
export function formatCountdown(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds));
  return `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;
}

/** Poll only while a code is alive and the account is not linked yet. */
export function shouldPoll(code: { expires_at: string } | null, linked: boolean, now: number): boolean {
  return code !== null && !linked && secondsLeft(code.expires_at, now) > 0;
}

/**
 * The bot's chat link as an `href`, or null. Comms validates it on write; re-checked here because an
 * `href` built from stored text is where a `javascript:` URL would run.
 */
export function safeHttpsHref(v: string | undefined): string | null {
  if (v === undefined) return null;
  try {
    const u = new URL(v);
    return u.protocol === "https:" && u.username === "" && u.password === "" ? v : null;
  } catch {
    return null;
  }
}
