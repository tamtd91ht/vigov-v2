/**
 * Wording and decisions of the "Lời hệ thống" tab (`docs/ui-ux/14-cau-hinh.md §7`). Pure, except the
 * two flow functions that call the API client — kept here so the refusal paths are testable without
 * a DOM.
 */

import {
  restoreSystemMessage,
  rewordSystemMessage,
  type SystemMessage,
  type SystemMessageModule,
} from "@/lib/api/system-messages";
import type { KetQua } from "@/lib/api/request";

export const SYSTEM_MESSAGES_TITLE = "Lời hệ thống";

/**
 * NOT the spec's guidance sentence verbatim: §7 says these sentences are "said to citizens" and show
 * "on the admin site and the Zalo Mini App". Of today's catalogue several are refusals only staff
 * see (e.g. `feedback.assignment_required`), so that sentence would tell the administrator something
 * untrue about where a change lands. What stays: a shipped sentence may be reworded, never removed.
 */
export const SYSTEM_MESSAGES_GUIDANCE =
  "Những câu dưới đây là lời hệ thống nói khi từ chối một thao tác. Câu đi kèm phần mềm sửa được " +
  "lời nhưng không xoá được — xoá đi thì lúc từ chối, hệ thống không còn gì để nói. Muốn dùng lại " +
  "câu gốc thì bấm Khôi phục câu mặc định.";

/**
 * The three sections, in the order of §7's groups. `note` is a section-wide sentence, drawn under the
 * heading when the whole group shares one fact an administrator must know before rewording.
 *
 * WHY "Báo cáo" CARRIES A NOTE: its sentences are the captions of the exported report and the titles
 * of the report notifications. `/bao-cao` is outside the first phase (ADR 0053 §6) and no other service
 * reads these keys yet, so a reworded caption shows nowhere today. Said once for the group rather than
 * on 38 cards (`MESSAGES_NOT_RAISED_YET` is per key, for the one-off case). Remove the note in the same
 * change that makes a feature print these sentences.
 */
export const SYSTEM_MESSAGE_SECTIONS: readonly {
  module: SystemMessageModule;
  title: string;
  note?: string;
}[] = [
  { module: "petitions", title: "Phản ánh" },
  { module: "finance", title: "Thu – Chi" },
  {
    module: "reporting",
    title: "Báo cáo",
    note:
      "Các câu nhóm này là tiêu đề, tên khối và tên chỉ số trên báo cáo xuất ra và trên thông báo " +
      "báo cáo gửi lãnh đạo. Màn Báo cáo chưa dựng, nên câu sửa ở đây được lưu cho xã nhưng hôm nay " +
      "chưa hiện ở đâu.",
  },
];

/**
 * Keys in the catalogue that NO refusal branch raises yet (backend report, 29/09/2026): rewording one
 * changes nothing any officer or citizen will see until a feature uses it. Said on the card so an
 * administrator does not reword it and wait for a change that cannot come. Remove a key from here in
 * the same change that makes a branch raise it.
 */
export const MESSAGES_NOT_RAISED_YET: ReadonlySet<string> = new Set([
  "feedback.after_photo_required",
  "feedback.unknown_field",
]);

export const NOT_RAISED_NOTE = "Chưa có chức năng nào dùng câu này.";
export const OVERRIDDEN_BADGE = "Đang dùng câu của xã";
export const EDIT_BUTTON = "Sửa lời";
export const SAVE_BUTTON = "Lưu";
export const CANCEL_BUTTON = "Huỷ";
export const RESTORE_BUTTON = "Khôi phục câu mặc định";
export const RESTORE_CONFIRM =
  "Dùng lại câu mặc định của phần mềm cho câu này? Câu xã đã sửa sẽ không còn được dùng.";
export const RESTORE_CONFIRM_BUTTON = "Khôi phục";
export const SAVED_SENTENCE = "Đã lưu câu của xã.";
export const RESTORED_SENTENCE = "Đã khôi phục câu mặc định.";

/**
 * The bound the server stores (`MessageTextMax`, in characters — runes — and the CHECK of migration
 * 0020 in both services). Checked here only so a staff member learns it before the round trip; the
 * server checks again and its sentence wins when they disagree.
 */
export const SYSTEM_MESSAGE_MAX = 1000;
export const TEXT_EMPTY =
  "Câu không được để trống. Muốn dùng lại câu mặc định thì bấm Khôi phục câu mặc định.";
export const TEXT_TOO_LONG = `Câu dài quá ${SYSTEM_MESSAGE_MAX} ký tự.`;

/** Trim, then refuse empty and over-long. Counts characters, not UTF-16 units (Vietnamese marks). */
export function validateMessageText(
  raw: string,
): { ok: true; text: string } | { ok: false; message: string } {
  const text = raw.trim();
  if (text === "") return { ok: false, message: TEXT_EMPTY };
  if ([...text].length > SYSTEM_MESSAGE_MAX) return { ok: false, message: TEXT_TOO_LONG };
  return { ok: true, text };
}

/**
 * Save: validate, then PUT. A refusal — ours or the server's 400 sentence (markup, control
 * characters…) — comes back as the one sentence to show; the request is not sent when ours refuses.
 */
export async function saveMessageFlow(
  module: SystemMessageModule,
  code: string,
  raw: string,
): Promise<KetQua<SystemMessage>> {
  const v = validateMessageText(raw);
  if (!v.ok) return { ok: false, thongBao: v.message };
  return rewordSystemMessage(module, code, v.text);
}

/** Restore: DELETE the commune's wording. The caller re-reads the list — the server is the state. */
export function restoreMessageFlow(
  module: SystemMessageModule,
  code: string,
): Promise<KetQua<null>> {
  return restoreSystemMessage(module, code);
}

const TIME_FORMAT = new Intl.DateTimeFormat("vi-VN", {
  timeZone: "Asia/Ho_Chi_Minh",
  hour: "2-digit",
  minute: "2-digit",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour12: false,
});

/** "Sửa lần cuối: CB-00123, 14:05 22/09/2026" — only for an overridden sentence. */
export function lastEditLine(m: SystemMessage): string | null {
  if (!m.overridden) return null;
  const who = m.updated_by !== undefined && m.updated_by !== "" ? m.updated_by : "không rõ người sửa";
  if (m.updated_at === undefined || m.updated_at === null) return `Sửa lần cuối: ${who}`;
  const d = new Date(m.updated_at);
  const when = Number.isNaN(d.getTime()) ? "mốc thời gian không đọc được" : TIME_FORMAT.format(d);
  return `Sửa lần cuối: ${who}, ${when}`;
}
