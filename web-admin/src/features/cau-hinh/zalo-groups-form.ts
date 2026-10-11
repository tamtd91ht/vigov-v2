/**
 * "Cấu hình → Kênh Zalo → Nhóm Zalo" (ADR 0092) — the pure half: the words, which kinds a group may
 * tick, the request bodies, and where each refusal is drawn. Every check here is a HINT: comms re-checks
 * every rule (`service-comms/internal/domain/zalo_group.go`) and its refusal still reaches the screen.
 *
 * No prototype exists for this section; it keeps to the tab's own patterns.
 */

import type { comms_zaloGroupOut, comms_zaloGroupPatchIn } from "@/lib/api/zalo";

import { rowKinds, ZALO_EVENT_GROUPS, type ZaloEventRow } from "./zalo-channel-form";

/** The empty state, and the instruction repeated next to a fresh code. */
export const GROUPS_HOW_IT_WORKS =
  "Thêm bot vào nhóm Zalo, tạo mã gắn nhóm ở đây rồi gửi mã trong nhóm (có thể kèm @tên bot).";

export const GROUPS_DESCRIPTION =
  "Bot nhắn việc vào nhóm Zalo của một bộ phận hoặc của cả xã. Tin nhắc riêng một người (nhắc tên, đề nghị lùi hạn, bản tin) không bao giờ vào nhóm. Gỡ nhóm chỉ làm được ở đây.";

export const COMMUNE_WIDE = "Toàn xã";

export const UNIT_REMOVED_WARNING = "Bộ phận đã bị xoá — chọn lại phạm vi";

export const CODE_ONCE =
  "Mã chỉ hiện một lần và dùng được một lần. Đóng hộp này là mã không hiện lại — cần thì tạo mã mới.";

export const CODE_EXPIRED = "Mã đã hết hạn. Đóng hộp này rồi tạo mã mới.";

/** Why "Tạo mã gắn nhóm" is disabled — the same causes comms answers 409 for, said BEFORE the click. */
export const CHANNEL_OFF_REASON =
  "Xã chưa bật kênh nhắc việc qua Zalo. Hãy bật kênh và chọn loại tin trước khi gắn nhóm.";
export const NO_BOT_REASON =
  "Chưa có bot Zalo nào phục vụ xã, nên chưa gắn được nhóm. Nhập mã bot riêng ở phần “Con bot của xã”, hoặc chờ ViHAT bật bot chung.";
export const NO_GROUP_KIND_REASON =
  "Xã chưa bật loại tin nào dùng được cho nhóm. Hãy chọn ở phần “Việc gì thì nhắn qua Zalo” — giao việc, quá hạn, chờ duyệt, phân công phản ánh…";

export const SCOPE_HINT = "Nhóm của một bộ phận chỉ nhận việc do bộ phận ấy đang giữ.";

export const KINDS_HINT = "Chỉ có các loại xã đang bật ở phần “Việc gì thì nhắn qua Zalo”.";

export const UNLINK_CONSEQUENCE = "Bot thôi nhắn việc vào nhóm này. Muốn nhận lại thì tạo mã gắn nhóm mới.";

export const UNLINKED_TOAST = "Đã gỡ nhóm.";

/** Edit dialog: stored kinds the commune has since switched off — kept unless a box is changed. */
export function droppedKindsText(names: readonly string[]): string {
  return `Nhóm còn nhận loại xã đã tắt ở phần “Việc gì thì nhắn qua Zalo”: ${names.join(", ")}. Đổi ô loại tin ở đây thì các loại ấy bị bỏ khỏi nhóm.`;
}

export const GENERIC_FAILURE = "Chưa lưu được. Vui lòng thử lại.";

export const UNLINK_REASON_MAX = 200;

/**
 * The kinds a group may tick NOW: comms' `supported_kinds` (the group kinds, its order) ∩ the kinds the
 * commune has switched on in §2. Channel off = none (comms: `kind_not_enabled` for every kind).
 */
export function offerableKinds(supported: readonly string[], channelEnabled: boolean, enabled: readonly string[]): string[] {
  if (!channelEnabled) return [];
  return supported.filter((k) => enabled.includes(k));
}

/** One checkbox: a §2 row, restricted to the kinds a group may tick now. */
export type GroupKindRow = { code: string; label: string; kinds: readonly string[] };
export type GroupKindSection = { title: string; rows: readonly GroupKindRow[] };

/**
 * §2's rows and groups, each restricted to `offerable`; a row with nothing offerable is not drawn, a
 * section with no row neither. "Văn bản, đơn thư quá hạn" stays ONE box for its three kinds, as in §2.
 */
export function groupKindSections(offerable: readonly string[]): GroupKindSection[] {
  return ZALO_EVENT_GROUPS.map((g) => ({
    title: g.title,
    rows: g.events
      .map((ev) => ({ code: ev.code, label: ev.label, kinds: rowKinds(ev).filter((k) => offerable.includes(k)) }))
      .filter((r) => r.kinds.length > 0),
  })).filter((s) => s.rows.length > 0);
}

/** A box is ticked when any of its offerable kinds is selected; ticking sets all of them alike. */
export function isRowChecked(row: GroupKindRow, selected: readonly string[]): boolean {
  return row.kinds.some((k) => selected.includes(k));
}

/** `order` = `supported_kinds`: the body never depends on click order. */
export function toggleGroupKinds(selected: readonly string[], row: GroupKindRow, on: boolean, order: readonly string[]): string[] {
  const next = on ? [...selected, ...row.kinds] : selected.filter((k) => !row.kinds.includes(k));
  return order.filter((k) => next.includes(k));
}

const ALL_ROWS: readonly ZaloEventRow[] = ZALO_EVENT_GROUPS.flatMap((g) => g.events);

/**
 * The names of a group's kinds, by §2 row (a row named once even when it carries three kinds). A kind no
 * row names is shown as its code rather than hidden: what a group receives is never understated.
 */
export function kindNames(kinds: readonly string[]): string[] {
  const names: string[] = [];
  const named = new Set<string>();
  for (const row of ALL_ROWS) {
    const ks = rowKinds(row);
    if (ks.some((k) => kinds.includes(k))) {
      names.push(row.label);
      for (const k of ks) named.add(k);
    }
  }
  for (const k of kinds) if (!named.has(k)) names.push(k);
  return names;
}

/** "Bộ phận Một cửa" / "Toàn xã"; a unit with no name identity could read is said so, never guessed. */
export function scopeText(g: Pick<comms_zaloGroupOut, "org_unit_id" | "org_unit_name">): string {
  if (g.org_unit_id === null) return COMMUNE_WIDE;
  return g.org_unit_name.trim() !== "" ? g.org_unit_name : "Bộ phận không đọc được tên";
}

/** "Gắn lúc … · bởi Nguyễn Văn A (CB-00123)" — `at` already formatted. The name may be empty. */
export function linkedText(at: string, g: Pick<comms_zaloGroupOut, "linked_by" | "linked_by_name">): string {
  const who = g.linked_by_name.trim() !== "" ? `${g.linked_by_name} (${g.linked_by})` : g.linked_by;
  return `Gắn lúc ${at} · bởi ${who}`;
}

/** "Còn 09:41" until expiry; null once expired. */
export function countdownText(expiresAt: string, now: number): string | null {
  const left = Math.floor((Date.parse(expiresAt) - now) / 1000);
  if (!Number.isFinite(left) || left <= 0) return null;
  const mm = String(Math.floor(left / 60)).padStart(2, "0");
  const ss = String(left % 60).padStart(2, "0");
  return `Còn ${mm}:${ss}`;
}

/** The scope select's value: "" = the whole commune, else the unit id. */
export type ScopeValue = string;

/**
 * `kindsTouched`: whether staff changed a box in THIS dialog. An edit that did not touch the kinds sends
 * no `kinds` key, so the stored list — including kinds the commune has since switched off in §2, which
 * the boxes cannot show — is kept as is rather than silently narrowed by a scope-only edit.
 */
export type GroupDraft = { scope: ScopeValue; kinds: string[]; kindsTouched: boolean };

/** The edit dialog's start: the stored scope, and the stored kinds a group may STILL tick. */
export function draftFromGroup(g: comms_zaloGroupOut, offerable: readonly string[]): GroupDraft {
  return { scope: g.org_unit_id ?? "", kinds: g.kinds.filter((k) => offerable.includes(k)), kindsTouched: false };
}

/** Stored kinds the commune has since switched off in §2: kept unless the boxes are changed — the dialog says so. */
export function droppedKinds(g: comms_zaloGroupOut, offerable: readonly string[]): string[] {
  return g.kinds.filter((k) => !offerable.includes(k));
}

/** Where a refusal is drawn. */
export type RefusalField = "scope" | "kinds" | "reason" | "form";
export type Refusal = { field: RefusalField; text: string };

/**
 * comms' refusal codes (zalo_groups.go @reply), each to the field it is about and a sentence in the
 * commune's words. Own keys only: `"toString" in GROUP_REFUSALS` is true and would draw a function.
 */
const GROUP_REFUSALS: Readonly<Record<string, Refusal>> = {
  org_unit_invalid: { field: "scope", text: "Bộ phận đã chọn không hợp lệ. Hãy chọn lại phạm vi." },
  org_unit_not_live: {
    field: "scope",
    text: "Bộ phận này không còn trong sơ đồ tổ chức của xã. Hãy chọn bộ phận khác hoặc Toàn xã.",
  },
  kinds_required: { field: "kinds", text: "Cần chọn ít nhất một loại tin cho nhóm." },
  unknown_kind: { field: "kinds", text: "Có loại tin không gửi vào nhóm được. Chỉ chọn trong danh sách dưới đây." },
  kind_not_enabled: {
    field: "kinds",
    text: "Có loại tin xã chưa bật ở phần “Việc gì thì nhắn qua Zalo”. Hãy bật loại ấy trước, hoặc bỏ chọn nó ở đây.",
  },
  reason_required: { field: "reason", text: `Cần nhập lý do gỡ nhóm, tối đa ${UNLINK_REASON_MAX} ký tự.` },
  nothing_to_change: { field: "form", text: "Chưa có gì thay đổi." },
  not_found: { field: "form", text: "Nhóm này không còn được gắn với xã — có thể vừa được gỡ. Danh sách đã được tải lại." },
  zalo_bot_not_configured: { field: "form", text: NO_BOT_REASON },
  zalo_channel_disabled: { field: "form", text: CHANNEL_OFF_REASON },
  org_units_unavailable: {
    field: "form",
    text: "Tạm thời không kiểm được bộ phận của xã. Vui lòng thử lại sau ít phút.",
  },
};

/**
 * A refused group write → where and what to draw. An unknown code: the server's own sentence when it
 * wrote one, the generic sentence when it did not (`unexplained` = `LOI_KHONG_RO`, passed in so this file
 * stays free of the transport).
 */
export function refusalFor(code: string | undefined, serverText: string, unexplained: string): Refusal {
  if (code !== undefined && Object.prototype.hasOwnProperty.call(GROUP_REFUSALS, code)) {
    return GROUP_REFUSALS[code]!;
  }
  return { field: "form", text: serverText === unexplained || serverText.trim() === "" ? GENERIC_FAILURE : serverText };
}

/** Client check before a create: at least one kind (comms: kinds_required). */
export function checkDraft(d: GroupDraft): Refusal | null {
  return d.kinds.length === 0 ? GROUP_REFUSALS.kinds_required! : null;
}

/**
 * The PATCH body: only what changed. Scope compared as stored (null ↔ ""); kinds only when a box was
 * touched, then compared as SETS — comms normalises the order, so a reorder is not a change. Nothing
 * changed → a refusal, nothing sent.
 */
export function buildPatch(g: comms_zaloGroupOut, d: GroupDraft): { ok: true; patch: comms_zaloGroupPatchIn } | { ok: false; refusal: Refusal } {
  const patch: comms_zaloGroupPatchIn = {};
  const stored = g.org_unit_id ?? "";
  if (d.scope !== stored) patch.org_unit_id = d.scope === "" ? null : d.scope;
  if (d.kindsTouched) {
    const kindsRefusal = checkDraft(d);
    if (kindsRefusal !== null) return { ok: false, refusal: kindsRefusal };
    const sameKinds = d.kinds.length === g.kinds.length && d.kinds.every((k) => g.kinds.includes(k));
    if (!sameKinds) patch.kinds = [...d.kinds];
  }
  if (patch.org_unit_id === undefined && patch.kinds === undefined) {
    return { ok: false, refusal: GROUP_REFUSALS.nothing_to_change! };
  }
  return { ok: true, patch };
}

/** The unlink reason, trimmed, or why not (comms: required, ≤ 200 characters, one line). */
export function checkUnlinkReason(raw: string): { ok: true; reason: string } | { ok: false; refusal: Refusal } {
  const reason = raw.trim();
  if (reason === "" || [...reason].length > UNLINK_REASON_MAX) return { ok: false, refusal: GROUP_REFUSALS.reason_required! };
  return { ok: true, reason };
}
