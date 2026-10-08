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
import type { KetQua } from "@/lib/api/goi";

export const SYSTEM_MESSAGES_TITLE = "Lời hệ thống";

/**
 * Spec 07's sentence with its "Zalo Mini App" clause removed (owner, 08/10/2026: "Giữ spec, chỉ bỏ
 * phần Mini App"). `citizen-app/` reads no system message and every key is raised on a STAFF route
 * (petitions `refusalSentence`, `petition_publication.go`; finance `du_an.go` scopeNotice), so the
 * Mini App clause told the administrator something untrue about where a change lands.
 */
export const SYSTEM_MESSAGES_GUIDANCE =
  "Những câu dưới đây là lời hệ thống hiện ra trên trang quản trị. Câu đi kèm phần mềm có thể sửa " +
  "lời nhưng không xoá được — xoá đi thì lúc từ chối, hệ thống không còn gì để nói.";

/**
 * The three sections, ordered by group code like the prototype (spec 07's "Dùng chung" has no keys
 * here; "Báo cáo điều hành" is this app's third group, decision 3 of the fix card). No per-group or per-card notes:
 * owner, 08/10/2026, "Bỏ hết, đúng prototype".
 */
type SystemMessageSection = {
  module: SystemMessageModule;
  /** The prototype's group code (`MessageTemplateTable` GROUP_LABEL); `bao-cao` is this app's own. */
  group: string;
  title: string;
};

export const SYSTEM_MESSAGE_SECTIONS: readonly SystemMessageSection[] = ([

  { module: "petitions", group: "phan-anh", title: "Phản ánh của người dân" },
  { module: "finance", group: "giai-ngan", title: "Theo dõi giải ngân" },
  { module: "reporting", group: "bao-cao", title: "Báo cáo điều hành" },
  // Drawn in ascending group-code order, as the prototype sorts its groups (`MessageTemplateTable.tsx:40`).
] satisfies SystemMessageSection[]).sort((a, b) => (a.group < b.group ? -1 : a.group > b.group ? 1 : 0));

// Button and badge words are the prototype's (`MessageTemplateTable`, ADR 0068 lần 5).
export const SHIPPED_BADGE = "Đi kèm phần mềm";
export const OVERRIDDEN_BADGE = "Đã sửa lời";
/** Label of the disabled "?" control; its entry in `PHAN_CHUA_DUNG` is "Tắt câu hệ thống". */
export const SWITCH_OFF_BUTTON = "Tắt";
export const SAVE_BUTTON = "Lưu";
export const RESTORE_BUTTON = "Khôi phục lời gốc";
export const SAVED_SENTENCE = "Đã lưu lời mới.";
export const RESTORED_SENTENCE = "Đã khôi phục lời gốc.";

/**
 * The bound the server stores (`MessageTextMax`, in characters — runes — and the CHECK of migration
 * 0020 in both services). Checked here only so a staff member learns it before the round trip; the
 * server checks again and its sentence wins when they disagree.
 */
export const SYSTEM_MESSAGE_MAX = 1000;
export const TEXT_EMPTY =
  "Câu không được để trống. Muốn dùng lại câu mặc định thì bấm Khôi phục lời gốc.";
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
 * "Lưu" is enabled only when this is true (spec 07; prototype compares trimmed text): saving what is
 * already in force would write an audit entry recording no change.
 */
export function isTextChanged(draft: string, current: string): boolean {
  return draft.trim() !== current.trim();
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
