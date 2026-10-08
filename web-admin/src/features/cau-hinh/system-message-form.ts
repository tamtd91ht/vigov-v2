/**
 * Wording and decisions of the "Lời hệ thống" tab (`docs/ui-ux/14-cau-hinh.md §7`, spec 07, ADR 0079
 * Q2/Q5). Pure, except the flow functions that call the API client — kept here so the refusal paths are
 * testable without a DOM.
 */

import {
  createCommuneMessage,
  deleteCommuneMessage,
  editCommuneMessage,
  restoreSystemMessage,
  rewordSystemMessage,
  switchSystemMessage,
  type CommuneMessageModule,
  type CreateMessageBody,
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
 * One section of the tab: a GROUP (the prototype's `group_code`), the service whose list feeds it, and
 * its title. No per-group or per-card notes: owner, 08/10/2026, "Bỏ hết, đúng prototype".
 *
 * `primary` is the one section of a service that is always drawn — it carries that service's loading
 * bars, its error and its empty state, so one service being down shows once, under its own heading.
 * A non-primary section ("Dùng chung", fed by petitions — ADR 0079 Q5a) appears only when it holds
 * sentences, as the prototype draws only the groups its list contains.
 */
type SystemMessageSection = {
  module: SystemMessageModule;
  group: string;
  title: string;
  primary: boolean;
};

export const SYSTEM_MESSAGE_SECTIONS: readonly SystemMessageSection[] = ([
  { module: "petitions", group: "phan-anh", title: "Phản ánh của người dân", primary: true },
  { module: "petitions", group: "chung", title: "Dùng chung", primary: false },
  { module: "finance", group: "giai-ngan", title: "Theo dõi giải ngân", primary: true },
  // `bao-cao` is this app's third group (decision 3 of the fix card); it takes no commune sentences.
  { module: "reporting", group: "bao-cao", title: "Báo cáo điều hành", primary: true },
  // Drawn in ascending group-code order, as the prototype sorts its groups (`MessageTemplateTable.tsx:40`).
] satisfies SystemMessageSection[]).sort((a, b) => (a.group < b.group ? -1 : a.group > b.group ? 1 : 0));

export const SYSTEM_MESSAGE_MODULES: readonly SystemMessageModule[] = ["petitions", "finance", "reporting"];

/**
 * The sentences of one service that a section lists. A group code the screen does not know goes to the
 * service's PRIMARY section rather than vanishing: a sentence the commune can no longer see is one it can
 * no longer switch off or delete.
 */
export function sectionMessages(
  section: SystemMessageSection,
  all: readonly SystemMessage[],
): readonly SystemMessage[] {
  const known = new Set(
    SYSTEM_MESSAGE_SECTIONS.filter((s) => s.module === section.module).map((s) => s.group),
  );
  return all.filter(
    (m) => m.group_code === section.group || (section.primary && !known.has(m.group_code)),
  );
}

/**
 * The groups a commune may add a sentence to, and the service that stores each (ADR 0079 Q5a:
 * "Dùng chung" lives in petitions; reporting takes none).
 *
 * THE FORM HAS NO GROUP CONTROL, as the prototype's (`MessageTemplateTable.tsx:246-283`; owner "đúng
 * prototype", VALIDATE round 4): the group is the code's prefix — `chung.loi-chao` is a "Dùng chung"
 * sentence. The server requires exactly that prefix anyway (`domain.NormalizeCustomKey`).
 */
export const ADD_GROUPS: readonly { group: string; module: CommuneMessageModule }[] = [
  { group: "phan-anh", module: "petitions" },
  { group: "chung", module: "petitions" },
  { group: "giai-ngan", module: "finance" },
];

/** Spec 07's fixed placeholder of Mã câu. */
export const ADD_CODE_PLACEHOLDER = "chung.loi-chao";

/** The group a code belongs to, read from its `<nhóm>.` prefix; `undefined` when it has none of the three. */
export function groupOfCode(code: string): (typeof ADD_GROUPS)[number] | undefined {
  return ADD_GROUPS.find((g) => code.startsWith(`${g.group}.`));
}

// Button, badge and toast words are the prototype's (`MessageTemplateTable`, ADR 0068 lần 5) unless noted.
export const SHIPPED_BADGE = "Đi kèm phần mềm";
export const COMMUNE_BADGE = "Xã tự thêm";
export const OVERRIDDEN_BADGE = "Đã sửa lời";
export const SWITCHED_OFF_BADGE = "Đang tắt — dùng lời gốc";
/**
 * A commune sentence has no software sentence behind it, so "dùng lời gốc" would be untrue on its card
 * (assumption of this card, reported): the switched-off pill says only that it is off.
 */
export const SWITCHED_OFF_COMMUNE_BADGE = "Đang tắt";
export const SWITCH_OFF_BUTTON = "Tắt";
export const SWITCH_ON_BUTTON = "Bật lại";
export const SAVE_BUTTON = "Lưu";
export const RESTORE_BUTTON = "Khôi phục lời gốc";
export const DELETE_BUTTON = "Xoá";
export const ADD_BUTTON = "Thêm câu mới";
export const ADD_SUBMIT_BUTTON = "Thêm câu";
/** "Huỷ" of the add form and of the delete step — NOT a restore confirmation (there is none, lô 3 Q7e). */
export const FORM_CANCEL_BUTTON = "Huỷ";
export const SAVED_SENTENCE = "Đã lưu lời mới.";
export const RESTORED_SENTENCE = "Đã khôi phục lời gốc.";
export const ADDED_SENTENCE = "Đã thêm câu mới.";
export const DELETED_SENTENCE = "Đã xoá câu này.";
// Neither spec 07 nor the prototype toasts the switch; short neutral sentences of this card.
export const SWITCHED_OFF_SENTENCE = "Đã tắt câu này.";
export const SWITCHED_ON_SENTENCE = "Đã bật lại câu này.";

export const ADD_CODE_LABEL = "Mã câu";
export const ADD_DESCRIPTION_LABEL = "Giải thích câu này dùng ở đâu";
export const ADD_TEXT_LABEL = "Nội dung";
export const ADD_MISSING = "Cần cả mã và nội dung câu.";
export const ADD_CODE_PREFIX = "Mã câu phải bắt đầu bằng chung., phan-anh. hoặc giai-ngan.";

/**
 * The bound the server stores (`MessageTextMax`, in characters — runes — and the CHECK of migration
 * 0020 in both services). Checked here only so a staff member learns it before the round trip; the
 * server checks again and its sentence wins when they disagree.
 */
export const SYSTEM_MESSAGE_MAX = 1000;
/** `CustomMessageDeleteReasonMax` (migration 0033 CHECK), enforced by the box's `maxLength`. */
export const DELETE_REASON_MAX = 200;
export const TEXT_EMPTY =
  "Câu không được để trống. Muốn dùng lại câu mặc định thì bấm Khôi phục lời gốc.";
/** A commune sentence has no "Khôi phục lời gốc" to point to. */
export const TEXT_EMPTY_COMMUNE = "Câu không được để trống.";
export const TEXT_TOO_LONG = `Câu dài quá ${SYSTEM_MESSAGE_MAX} ký tự.`;
export const DELETE_REASON_MISSING = "Hãy nêu lý do xoá câu này.";

export function deleteReasonLabel(code: string): string {
  return `Lý do xoá câu ${code}`;
}

export function isCommune(m: SystemMessage): boolean {
  return m.origin === "commune";
}

/**
 * Whether the card offers "Tắt / Bật lại": only a sentence carrying the commune's own wording — a
 * reworded shipped sentence or a commune sentence (ADR 0079 lô 3, "Khi nào hiện Tắt"). A shipped
 * sentence in force has nothing to switch; the server answers 409 `no_commune_wording`.
 */
export function canSwitch(m: SystemMessage): boolean {
  return isCommune(m) || m.overridden;
}

/**
 * The words the edit box holds. A switched-off reworded sentence has the DEFAULT in force
 * (`current_text`) and the commune's words in `override_text`; the box edits the commune's words, which
 * is also what the server compares a save against (`app.Reword`).
 */
export function editableText(m: SystemMessage): string {
  if (!isCommune(m) && m.overridden && m.override_text !== undefined) return m.override_text;
  return m.current_text;
}

/** Trim, then refuse empty and over-long. Counts characters, not UTF-16 units (Vietnamese marks). */
export function validateMessageText(
  raw: string,
  emptyMessage: string = TEXT_EMPTY,
): { ok: true; text: string } | { ok: false; message: string } {
  const text = raw.trim();
  if (text === "") return { ok: false, message: emptyMessage };
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
 * Save: validate, then PUT the override (shipped) or PATCH the sentence (commune). A refusal — ours or
 * the server's 400 sentence (markup, control characters…) — comes back as the one sentence to show; the
 * request is not sent when ours refuses.
 */
export async function saveMessageFlow(
  module: SystemMessageModule,
  message: SystemMessage,
  raw: string,
): Promise<KetQua<SystemMessage>> {
  const commune = isCommune(message) && module !== "reporting";
  const v = validateMessageText(raw, commune ? TEXT_EMPTY_COMMUNE : TEXT_EMPTY);
  if (!v.ok) return { ok: false, thongBao: v.message };
  if (commune) return editCommuneMessage(module, message.code, { text: v.text });
  return rewordSystemMessage(module, message.code, v.text);
}

/** Restore: DELETE the commune's wording. The caller re-reads the list — the server is the state. */
export function restoreMessageFlow(
  module: SystemMessageModule,
  code: string,
): Promise<KetQua<null>> {
  return restoreSystemMessage(module, code);
}

/** "Tắt" / "Bật lại": flips `is_active` on whichever route owns the sentence's switch. */
export function switchMessageFlow(
  module: SystemMessageModule,
  message: SystemMessage,
): Promise<KetQua<SystemMessage>> {
  const next = !message.is_active;
  if (isCommune(message) && module !== "reporting") {
    return editCommuneMessage(module, message.code, { is_active: next });
  }
  return switchSystemMessage(module, message.code, next);
}

/** The add form's fields, as typed. */
export type AddDraft = { code: string; description: string; text: string };

export function emptyAddDraft(): AddDraft {
  return { code: "", description: "", text: "" };
}

/**
 * Add: both code and text are required (spec 07's sentence); the text has the same bound as a wording.
 * The group — and so the service that stores the sentence — is the code's prefix (`groupOfCode`). A code
 * with none of the three prefixes is refused HERE, not sent: there is no service to send it to, and
 * guessing one would file the sentence under a group nobody chose (fail closed). The rest of the code's
 * SHAPE (lowercase slug, length) stays the server's rule (`domain.NormalizeCustomKey`) and its 400
 * sentence reaches the form verbatim — a second copy here would drift (rule 9). An empty description is
 * omitted (none), never sent as "".
 */
export async function addMessageFlow(
  draft: AddDraft,
  idempotencyKey: string,
): Promise<KetQua<{ module: CommuneMessageModule; message: SystemMessage }>> {
  const code = draft.code.trim();
  const text = draft.text.trim();
  if (code === "" || text === "") return { ok: false, thongBao: ADD_MISSING };
  const target = groupOfCode(code);
  if (target === undefined) return { ok: false, thongBao: ADD_CODE_PREFIX };
  const v = validateMessageText(text, TEXT_EMPTY_COMMUNE);
  if (!v.ok) return { ok: false, thongBao: v.message };
  const description = draft.description.trim();
  const body: CreateMessageBody = {
    group_code: target.group,
    code,
    text: v.text,
    ...(description === "" ? {} : { description }),
  };
  const r = await createCommuneMessage(target.module, body, idempotencyKey);
  return r.ok ? { ok: true, duLieu: { module: target.module, message: r.duLieu } } : r;
}

/** Delete a commune sentence: the reason is required (rule 7) and checked before the round trip. */
export function deleteMessageFlow(
  module: CommuneMessageModule,
  code: string,
  reason: string,
): Promise<KetQua<null>> {
  const r = reason.trim();
  if (r === "") return Promise.resolve({ ok: false, thongBao: DELETE_REASON_MISSING });
  return deleteCommuneMessage(module, code, r);
}
