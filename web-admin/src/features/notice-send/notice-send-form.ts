/**
 * "Gửi tin ZNS / SMS" (`/gui-thong-bao`) — the pure half: the words, the audience labels and their wire
 * values, the template cards' lines, money as text. The prototype is
 * `vigov-require/apps/admin/src/components/citizens/NoticeWorkspace.tsx`; every string below that has a
 * counterpart there is its text, verbatim.
 *
 * WHAT IS REAL (ADR 0099 #4–#9, `service-comms/internal/http/notice_campaigns.go`): the templates of every
 * OA and every SMS account, the preview (recipients deduplicated by number, skipped, content, price,
 * estimated cost, missing parameters), the send, the history. What stays "?" is ONE thing: the step-1
 * recipient figure the client cannot count without double-counting (two groups, two tags) — the aside
 * uses the preview's figure, which the server deduplicates.
 *
 * NO PERSONAL DATA ON THIS SCREEN (rule 3): no number, no name of any recipient. The prototype's
 * "Ví dụ người nhận" block (`full_name · phone`) is dropped, not replaced (ADR 0099 "Lệch prototype").
 */

import type { CitizenContactTag } from "@/lib/api/citizen-contacts";
import type { CitizenGroup } from "@/lib/api/citizen-groups";
import type {
  NoticeAudienceKind,
  NoticeCampaign,
  NoticeChannel,
  NoticeTemplateParam,
  SmsNoticeTemplate,
  ZnsNoticeTemplate,
} from "@/lib/api/notice-campaigns";
import type { identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";

// ---- header and gate (prototype :82-98) -----------------------------------------------------------

export const NOTICE_SEND_TITLE = "Gửi tin ZNS / SMS";

export const NO_SEND_PERMISSION = "Tài khoản của bạn không có quyền gửi thông báo. Liên hệ Chánh Văn phòng nếu cần.";

export const TAB_COMPOSE = "Soạn tin";

/** Prototype `Đã gửi (${n})`; `n` = the rows loaded so far ("Tải thêm" brings the rest). No count before the first read. */
export function historyTabLabel(loaded: number | null): string {
  return loaded === null ? "Đã gửi" : `Đã gửi (${loaded})`;
}

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

/** The chip → the `audience.kind` comms stores (ADR 0011). */
export const AUDIENCE_WIRE: Readonly<Record<AudienceKind, NoticeAudienceKind>> = {
  citizen_all: "nguoi-dan-tat-ca",
  citizen_group: "nguoi-dan-theo-nhom",
  citizen_tag: "nguoi-dan-theo-nhan",
  staff_all: "can-bo-tat-ca",
  staff_unit: "can-bo-theo-bo-phan",
};

/** The history's "Đối tượng" column: the chip's label for a stored kind; an unknown kind is shown as stored. */
export function audienceKindLabel(wire: string): string {
  const k = AUDIENCE_KINDS.find((key) => AUDIENCE_WIRE[key] === wire);
  return k === undefined ? wire : AUDIENCE_LABEL[k];
}

/** The audience as the routes take it: exactly the list the kind uses is sent non-empty. */
export function audienceBody(kind: AudienceKind, ids: readonly string[], tags: readonly string[]) {
  return {
    kind: AUDIENCE_WIRE[kind],
    ids: kind === "citizen_group" || kind === "staff_unit" ? [...ids] : [],
    tags: kind === "citizen_tag" ? [...tags] : [],
  };
}

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

// ---- the step-1 recipient count ----------------------------------------------------------------------

/**
 * Where the step-1 number of recipients comes from, decided once from the choice — never summed by hand.
 * The aside uses the PREVIEW's figure instead (deduplicated by number, "ngừng liên hệ" skipped).
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
  "Một người có thể ở nhiều nhóm hoặc mang nhiều nhãn. Số chính xác (đã bỏ trùng) hiện ở ô Xem trước.";

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

// ---- money: decimal TEXT, never a float -------------------------------------------------------------

const DECIMAL = /^(\d+)(?:\.(\d+))?$/;

/**
 * "3500.00" → "3.500 đ", "1234567.50" → "1.234.567,5 đ" — vi-VN grouping done on the STRING: comms sends
 * decimal text precisely so that no float rounds an amount of public money. A string that is not a
 * plain decimal is shown as it came, followed by "đ", rather than turned into a figure nobody sent.
 */
export function formatMoney(decimal: string): string {
  const m = DECIMAL.exec(decimal);
  if (m === null) return `${decimal} đ`;
  const whole = m[1]!.replace(/^0+(?=\d)/, "").replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  const frac = (m[2] ?? "").replace(/0+$/, "");
  return frac === "" ? `${whole} đ` : `${whole},${frac} đ`;
}

export const NOT_PRICED = "Chưa tính được";

/** A nullable decimal: formatted, or "Chưa tính được" (SMS while a slot is empty, or eSMS's estimate did not answer). */
export function moneyOrNotPriced(decimal: string | null): string {
  return decimal === null ? NOT_PRICED : formatMoney(decimal);
}

// ---- step 2: "Mẫu tin" -----------------------------------------------------------------------------

export type Channel = "zns" | "sms";

export const CHANNEL_LABEL: Readonly<Record<Channel, string>> = { zns: "Zalo ZNS", sms: "Tin nhắn SMS" };
export const CHANNEL_WIRE: Readonly<Record<Channel, NoticeChannel>> = { zns: "zalo-zns", sms: "sms" };
export const CHANNEL_PICK_LABEL = "Kênh gửi";
export const SMS_ACCOUNT_PICK_LABEL = "Tài khoản SMS";

/** Prototype :190-191. */
export const NO_TEMPLATES =
  "Chưa có mẫu tin nào cho đối tượng này. Mẫu phải đăng ký với nhà cung cấp trước khi gửi được.";

export const OPEN_ZALO_CONFIG = "Mở Cấu hình → Kênh Zalo";
export const OPEN_SMS_CONFIG = "Mở Cấu hình → Kênh SMS";
/** `?tab=` is the Cấu hình screen's starting tab, the same one the OA callback returns to (ADR 0098). */
export const ZALO_CONFIG_HREF = "/cau-hinh?tab=kenh-zalo";
export const SMS_CONFIG_HREF = "/cau-hinh?tab=kenh-sms";

/** The template an officer picked, with the source it leaves from. */
export type PickedTemplate =
  | { readonly channel: "zns"; readonly t: ZnsNoticeTemplate }
  | { readonly channel: "sms"; readonly t: SmsNoticeTemplate };

/** One identity per card: a template ref is only unique within its source. */
export function templateKey(p: PickedTemplate): string {
  return p.channel === "zns" ? `zns:${p.t.oa_id}:${p.t.template_ref}` : `sms:${p.t.sms_account_id}:${p.t.template_ref}`;
}

export function templateSourceId(p: PickedTemplate): string {
  return p.channel === "zns" ? p.t.oa_id : p.t.sms_account_id;
}

const SMS_TITLE_MAX = 60;

/** The card's name and the campaign's default title. An SMS template has no name: its text, cut. */
export function templateName(p: PickedTemplate): string {
  if (p.channel === "zns") return p.t.template_name;
  const text = p.t.content.trim();
  return text.length <= SMS_TITLE_MAX ? text : `${text.slice(0, SMS_TITLE_MAX).trimEnd()}…`;
}

/** The ZNS card's body line — the parameters, in place of a body Zalo does not return. */
export function paramsLine(names: readonly string[]): string {
  return names.length === 0 ? "Mẫu không có tham số." : `Tham số: ${names.join(", ")}`;
}

/** The ZNS card's channel line, prototype `Zalo ZNS · 200 đ một tin`; the OA named when the commune has several. */
export function znsSubline(t: ZnsNoticeTemplate, nameOA: boolean): string {
  const price = t.unit_price === null ? SMS_PRICE_AT_PREVIEW : `${formatMoney(t.unit_price)} một tin`;
  return [CHANNEL_LABEL.zns, ...(nameOA ? [t.oa_name] : []), price].join(" · ");
}

/** eSMS prices a FILLED text, so the card cannot carry a price. */
export const SMS_PRICE_AT_PREVIEW = "Đơn giá tính khi xem trước";

export function smsSubline(t: SmsNoticeTemplate): string {
  return `${CHANNEL_LABEL.sms} · ${t.account_label} · ${t.brandname} · ${SMS_PRICE_AT_PREVIEW}`;
}

export function smsAccountOptionLabel(t: Pick<SmsNoticeTemplate, "account_label" | "brandname">): string {
  return `${t.account_label} · ${t.brandname}`;
}

/** The tag beside a template whose OA must be authorised again, and why it cannot be picked. */
export const OA_REAUTH_TAG = "OA cần uỷ quyền lại";
export const OA_REAUTH_REASON = "Mẫu này chưa gửi được: OA cần được uỷ quyền lại. Vào Cấu hình → Kênh Zalo để uỷ quyền lại.";

/** ZNS templates grouped by OA, OAs in the order the server listed them. */
export function groupByOA(list: readonly ZnsNoticeTemplate[]): { oaId: string; oaName: string; items: ZnsNoticeTemplate[] }[] {
  const groups: { oaId: string; oaName: string; items: ZnsNoticeTemplate[] }[] = [];
  for (const t of list) {
    const g = groups.find((x) => x.oaId === t.oa_id);
    if (g === undefined) groups.push({ oaId: t.oa_id, oaName: t.oa_name, items: [t] });
    else g.items.push(t);
  }
  return groups;
}

/** The SMS accounts that have templates, in the server's order — the filter's options. */
export function smsAccounts(list: readonly SmsNoticeTemplate[]): SmsNoticeTemplate[] {
  const seen = new Set<string>();
  return list.filter((t) => (seen.has(t.sms_account_id) ? false : (seen.add(t.sms_account_id), true)));
}

// ---- step 3: "Nội dung" (prototype :242-281) ----------------------------------------------------------

export const PICK_TEMPLATE_FIRST = "Chọn mẫu tin ở bước trên đã.";
export const CAMPAIGN_TITLE_LABEL = "Tên đợt gửi";
export const PREVIEW_BUTTON = "Xem trước";

/** A field's label: ZNS the Zalo parameter name; SMS "Ô 1 (≤20 ký tự)" — eSMS slots have no meaningful name. */
export function paramLabel(channel: Channel, p: NoticeTemplateParam): string {
  if (channel === "zns") return p.name;
  return p.max_length > 0 ? `Ô ${p.field} (≤${p.max_length} ký tự)` : `Ô ${p.field}`;
}

/** A missing key as the officer reads it: ZNS the name, SMS "Ô 1". */
export function missingLabel(channel: Channel, field: string): string {
  return channel === "zns" ? field : `Ô ${field}`;
}

export function missingParamsText(labels: readonly string[]): string {
  return `Còn chỗ trống chưa điền: ${labels.join(", ")}.`;
}

// ---- the "Xem trước" aside (prototype :287-393) -----------------------------------------------------

export const PREVIEW_TITLE = "Xem trước";
export const PREVIEW_EMPTY =
  "Chọn đối tượng và mẫu tin, hệ thống sẽ hiện nội dung thật, số người nhận và chi phí dự kiến ở đây.";
/** ZNS: Zalo holds the approved body; comms returns `content: null`. */
export const ZNS_CONTENT = "Nội dung theo mẫu đã duyệt trên Zalo";
export const ROW_RECIPIENTS = "Người nhận";
export const ROW_SKIPPED = "Bỏ qua (ngừng liên hệ / thiếu số)";
export const ROW_UNIT_PRICE = "Đơn giá";
export const ROW_COST = "Chi phí dự kiến";
export const SEND_BUTTON = "Gửi thông báo";
/** Replaces the prototype's "Đường gửi sang eSMS chưa nối…" (:384-385) — the route is wired now. */
export const SEND_NOTE = "Tin đi từ OA/tài khoản đã chọn; đợt đã gửi không rút lại được.";
/** The composition changed after the preview shown: what would be sent is not what was previewed. */
export const PREVIEW_STALE = "Nội dung đã đổi sau lần xem trước. Bấm Xem trước lại trước khi gửi.";

/** Prototype toast, verbatim. */
export function queuedToast(n: number): string {
  return `Đã xếp hàng gửi cho ${n} người.`;
}
/** Prototype's sentence for 409 `no_recipients`. Every other refusal shows the server's sentence. */
export const NO_RECIPIENTS = "Không có người nhận nào.";

// ---- the "Đã gửi" tab (prototype :398-457) ---------------------------------------------------------

export const HISTORY_EMPTY = "Chưa có đợt gửi nào.";
export const LOAD_MORE = "Tải thêm";
export const HISTORY_COLUMNS = ["Đợt gửi", "Đối tượng", "Người nhận", "Chi phí", "Trạng thái"] as const;

/** How often a `dang-gui` row is re-read while the tab is open. */
export const SENDING_POLL_MS = 10_000;

/**
 * The status chip. `dang-gui` reads "Đang gửi", not the prototype's "Đã xếp hàng": the background job is
 * already sending, and the counts move while the officer watches.
 */
export function statusLabel(c: Pick<NoticeCampaign, "status" | "sent_count" | "recipient_count">): string {
  switch (c.status) {
    case "dang-gui":
      return "Đang gửi";
    case "da-gui":
      return "Đã gửi";
    case "gui-mot-phan":
      return `Gửi một phần · ${c.sent_count}/${c.recipient_count}`;
    case "that-bai":
      return "Hỏng";
  }
}

export type StatusTone = "success" | "danger" | "warn" | "neutral";

export function statusTone(status: NoticeCampaign["status"]): StatusTone {
  return status === "da-gui" ? "success" : status === "that-bai" ? "danger" : status === "gui-mot-phan" ? "warn" : "neutral";
}

/** "<mẫu> · <người gửi>" — the business code when identity could not name the sender. */
export function campaignSubline(c: Pick<NoticeCampaign, "template_name" | "created_by" | "created_by_name">): string {
  return `${c.template_name} · ${c.created_by_name !== "" ? c.created_by_name : c.created_by}`;
}

// ---- the OA card --------------------------------------------------------------------------------------

export const OA_CARD_TITLE = "OA Zalo của xã";
export const OA_NOT_LINKED = "Xã chưa nối OA Zalo. Tin ZNS chỉ gửi được sau khi nối.";
export const OA_MANAGE = "Quản lý OA";

/**
 * "Gửi từ OA: <tên>, <tên>" — every OA of the commune that can send now (ADR 0099 #1; which one a campaign
 * leaves from is the template picked in step 2). An OA's name is an authority's, not personal data.
 */
export function oaSenderText(oaNames: readonly string[]): string {
  return `Gửi từ OA: ${oaNames.join(", ")}`;
}
