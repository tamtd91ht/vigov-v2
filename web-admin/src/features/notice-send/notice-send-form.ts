/**
 * "Gửi tin ZNS / SMS" (`/gui-thong-bao`) — the pure half: the words, the audience labels, and the small
 * decisions the screen makes before any route exists to make them on the server. The prototype is
 * `vigov-require/apps/admin/src/components/citizens/NoticeWorkspace.tsx`; every string below that has
 * a counterpart there is its text, verbatim.
 *
 * WHAT IS REAL AND WHAT IS "?" (owner, 17/10/2026: the prototype's screen first, the send later):
 *   real  — the groups, tags and units to choose from; the recipient count where the server can count
 *           it without double-counting; the OA's ZBS templates (ADR 0098) and their parameter names
 *   "?"   — preview, price, cost, send, history (no route), SMS (eSMS, next round), and any count the
 *           server cannot give exactly (ADR 0068 §14: drawn in place, never faked)
 *
 * NO PERSONAL DATA ON THIS SCREEN (rule 3): no number, no name of any recipient. The prototype's
 * "Ví dụ người nhận" block (`full_name · phone`) is dropped, not replaced.
 */

import type { PendingFeatureInfo } from "@/components/ui/pending-feature";
import type { CitizenContactTag } from "@/lib/api/citizen-contacts";
import type { CitizenGroup } from "@/lib/api/citizen-groups";
import type { identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";
import { zaloStatusLabel } from "@/features/cau-hinh/zalo-oa-form";

// ---- header and gate (prototype :82-98) -----------------------------------------------------------

export const NOTICE_SEND_TITLE = "Gửi tin ZNS / SMS";

export const NO_SEND_PERMISSION = "Tài khoản của bạn không có quyền gửi thông báo. Liên hệ Chánh Văn phòng nếu cần.";

export const TAB_COMPOSE = "Soạn tin";
/** No count: there is no history route, and "(0)" would be a figure nobody counted. */
export const TAB_HISTORY = "Đã gửi";

// ---- step 1: "Gửi cho ai" (prototype :30-36) ---------------------------------------------------------

export type AudienceKind = "citizen_all" | "citizen_group" | "citizen_tag" | "staff_all" | "staff_unit";

export const AUDIENCE_LABEL: Readonly<Record<AudienceKind, string>> = {
  citizen_all: "Toàn bộ người dân",
  citizen_group: "Người dân theo nhóm",
  citizen_tag: "Người dân theo nhãn",
  staff_all: "Toàn bộ cán bộ",
  staff_unit: "Cán bộ theo bộ phận",
};

export const AUDIENCE_KINDS = Object.keys(AUDIENCE_LABEL) as AudienceKind[];

export function isStaffAudience(kind: AudienceKind): boolean {
  return kind === "staff_all" || kind === "staff_unit";
}

/** Prototype `Picker` labels. */
export const PICK_GROUP = "Chọn nhóm";
export const PICK_TAG = "Chọn nhãn";
export const PICK_UNIT = "Chọn bộ phận";
export const PICKER_EMPTY = "Chưa có mục nào.";

/** Without `citizen.read` the group and tag lists are not readable (`citizen-groups.ts`, `citizen-contacts.ts`). */
export const NO_CITIZEN_READ = "Cần quyền Xem danh bạ người dân để liệt kê nhóm, nhãn và đếm người nhận.";

/** "<tên> (<số thành viên>)" — the prototype's chip; `member_count` is the server's live count. */
export function groupChipLabel(g: Pick<CitizenGroup, "name" | "member_count">): string {
  return `${g.name} (${g.member_count})`;
}

export function tagChipLabel(t: CitizenContactTag): string {
  return `${t.tag} (${t.count})`;
}

// ---- the recipient count -----------------------------------------------------------------------------

/**
 * Where the number of recipients comes from, decided once from the choice — never summed by hand.
 *
 * WHY TWO GROUPS (OR TWO TAGS) ARE "?": one person can sit in several groups and carry several tags. The
 * count route takes ONE group (`identity_citizenContactCountQueryIn.group_id`) and no tag at all, so
 * adding two answers counts the shared people twice — a figure the screen would be inventing.
 * Staff units do add up: a staff member has exactly one `department_id`.
 */
export type RecipientSource =
  | { type: "citizen-count"; groupId: string }
  | { type: "tag"; count: number }
  | { type: "staff"; unitIds: readonly string[] | null }
  | { type: "unknown"; reason: string };

export const COUNT_NEEDS_CITIZEN_READ = "Cần quyền Xem danh bạ người dân để đếm số người nhận.";
export const COUNT_OVERLAP =
  "Một người có thể ở nhiều nhóm hoặc mang nhiều nhãn. Máy chủ chưa đếm được nhiều nhóm, nhãn cùng lúc mà không tính trùng, nên chưa có số.";

export function recipientSource(
  kind: AudienceKind,
  ids: readonly string[],
  tags: readonly string[],
  tagList: readonly CitizenContactTag[],
  canReadCitizens: boolean,
): RecipientSource {
  if (kind === "staff_all") return { type: "staff", unitIds: null };
  if (kind === "staff_unit") return { type: "staff", unitIds: ids };
  if (!canReadCitizens) return { type: "unknown", reason: COUNT_NEEDS_CITIZEN_READ };
  if (kind === "citizen_all") return { type: "citizen-count", groupId: "" };
  if (kind === "citizen_group") {
    return ids.length === 1 ? { type: "citizen-count", groupId: ids[0]! } : { type: "unknown", reason: COUNT_OVERLAP };
  }
  if (tags.length !== 1) return { type: "unknown", reason: COUNT_OVERLAP };
  const row = tagList.find((t) => t.tag === tags[0]);
  return row === undefined ? { type: "unknown", reason: COUNT_OVERLAP } : { type: "tag", count: row.count };
}

/** Staff in the directory, all of them or those whose ONE unit is among the picked ones. */
export function countStaff(items: readonly Pick<identity_canBoChonNguoiRa, "department_id">[], unitIds: readonly string[] | null): number {
  if (unitIds === null) return items.length;
  const picked = new Set(unitIds);
  return items.filter((s) => s.department_id !== "" && picked.has(s.department_id)).length;
}

export function recipientsText(n: number): string {
  return `${n.toLocaleString("vi-VN")} người`;
}

// ---- step 2: "Mẫu tin" -----------------------------------------------------------------------------

export type Channel = "zns" | "sms";

export const CHANNEL_LABEL: Readonly<Record<Channel, string>> = { zns: "Zalo ZNS", sms: "Tin nhắn SMS" };
export const CHANNEL_PICK_LABEL = "Kênh gửi";

/** Prototype :190-191. */
export const NO_TEMPLATES =
  "Chưa có mẫu tin nào cho đối tượng này. Mẫu phải đăng ký với nhà cung cấp trước khi gửi được.";

export const NO_TEMPLATE_PERMISSION = "Bạn chưa có quyền xem mẫu tin của OA. Cần quyền Cấu hình kênh Zalo.";

export const OPEN_ZALO_CONFIG = "Mở Cấu hình → Kênh Zalo";
/** `?tab=` is the Cấu hình screen's starting tab, the same one the OA callback returns to (ADR 0098). */
export const ZALO_CONFIG_HREF = "/cau-hinh?tab=kenh-zalo";

/** "Zalo ZNS · Đang bật" — the prototype's channel line, with Zalo's status in place of a price. */
export function templateSubline(status: string): string {
  return `${CHANNEL_LABEL.zns} · ${zaloStatusLabel(status)}`;
}

/** The tag beside an unusable template's name — Zalo's status in the officer's words (Cấu hình's table). */
export function zaloStatusText(status: string): string {
  return zaloStatusLabel(status);
}

/** Only an ENABLE template can carry a message (ADR 0098 #8). */
export function isTemplateUsable(status: string): boolean {
  return status === "ENABLE";
}

export function paramsLine(names: readonly string[]): string {
  return names.length === 0 ? "Mẫu không có tham số." : `Tham số: ${names.join(", ")}`;
}

export const SMS_PENDING: PendingFeatureInfo = {
  ten: "Mẫu tin SMS",
  viSao: "Gửi SMS qua eSMS đang được tích hợp.",
};

// ---- step 3: "Nội dung" (prototype :242-281) ----------------------------------------------------------

export const PICK_TEMPLATE_FIRST = "Chọn mẫu tin ở bước trên đã.";
export const CAMPAIGN_TITLE_LABEL = "Tên đợt gửi";

export const PREVIEW_PENDING: PendingFeatureInfo = {
  ten: "Xem trước",
  viSao: "Máy chủ chưa có tuyến xem trước tin, nên chưa ghép được nội dung thật, số người nhận và chi phí.",
};

/** Parameters still empty — computed here from the fields, in the template's order. */
export function missingParams(names: readonly string[], values: Readonly<Record<string, string>>): string[] {
  return names.filter((n) => (values[n] ?? "").trim() === "");
}

export function missingParamsText(names: readonly string[]): string {
  return `Còn chỗ trống chưa điền: ${names.join(", ")}.`;
}

// ---- the "Xem trước" aside (prototype :287-393) -----------------------------------------------------

export const PREVIEW_TITLE = "Xem trước";
export const PREVIEW_EMPTY =
  "Chọn đối tượng và mẫu tin, hệ thống sẽ hiện nội dung thật, số người nhận và chi phí dự kiến ở đây.";
export const ROW_RECIPIENTS = "Người nhận";
export const ROW_UNIT_PRICE = "Đơn giá";
export const ROW_COST = "Chi phí dự kiến";
export const SEND_BUTTON = "Gửi thông báo";

export const CONTENT_PENDING: PendingFeatureInfo = {
  ten: "Nội dung tin",
  viSao: "Zalo không trả thân mẫu ZBS, và máy chủ chưa có tuyến ghép nội dung thật của tin.",
};

export const PRICE_PENDING: PendingFeatureInfo = {
  ten: "Đơn giá",
  viSao: "Chưa có bảng giá tin ZNS / SMS theo xã, và chưa chốt bên nào trả phí.",
};

export const COST_PENDING: PendingFeatureInfo = {
  ten: "Chi phí dự kiến",
  viSao: "Chi phí = đơn giá × số người nhận. Chưa có đơn giá nên chưa tính được.",
};

/** The reason on the send button — and, in other words, the sentence under it. */
export const SEND_PENDING: PendingFeatureInfo = {
  ten: SEND_BUTTON,
  viSao: "Máy chủ chưa có tuyến gửi đợt tin. Tin ZNS sẽ đi từ OA của xã; SMS qua eSMS.",
};

/** Replaces the prototype's "Đường gửi sang eSMS chưa nối…" (:384-385) — that sentence describes a queue that does not exist here. */
export const SEND_NOTE = "Chưa gửi được: máy chủ chưa có tuyến gửi đợt tin. Tin ZNS sẽ đi từ OA của xã; SMS qua eSMS.";

export function recipientPendingInfo(reason: string): PendingFeatureInfo {
  return { ten: ROW_RECIPIENTS, viSao: reason };
}

// ---- the "Đã gửi" tab (prototype :398-457) ---------------------------------------------------------

export const HISTORY_EMPTY = "Chưa có đợt gửi nào.";

export const HISTORY_PENDING: PendingFeatureInfo = {
  ten: "Đã gửi",
  viSao: "Máy chủ chưa có tuyến lịch sử đợt gửi, nên chưa liệt kê được các đợt đã gửi.",
};

// ---- the OA card --------------------------------------------------------------------------------------

export const OA_CARD_TITLE = "OA Zalo của xã";
export const OA_NOT_LINKED = "Xã chưa nối OA Zalo. Tin ZNS chỉ gửi được sau khi nối.";
export const OA_MANAGE = "Quản lý OA";

/**
 * "Gửi từ OA: <tên>, <tên>" — every OA of the commune that can send now (ADR 0099 #1; which one a campaign
 * leaves from is picked at send time, D-6c). An OA's name is an authority's, not personal data.
 */
export function oaSenderText(oaNames: readonly string[]): string {
  return `Gửi từ OA: ${oaNames.join(", ")}`;
}
