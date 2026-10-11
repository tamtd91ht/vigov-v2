/**
 * The words, the form state and the pure rules of the "Cảnh báo thiên tai" tab (ADR 0091 §2).
 *
 * THE CLIENT CHECKS ARE CONVENIENCE. They mirror `service-comms/internal/domain/disaster_alert.go` so the
 * officer sees a problem next to the box instead of after Lưu; the server is where the refusal is real,
 * and its 400 sentence reaches the form (`localizeAlertRefusal` only swaps the field token for its label).
 *
 * THE DANGER ZONE IS A PICTURE, NOT A MAP (ADR 0091 §2): x and y are 0..100 over the illustration the Mini
 * App draws (`AlertPage.tsx:164-199`), never longitude/latitude. Nothing here sends a coordinate anywhere.
 */

import type { comms_DangerZonePoint, comms_EmergencyContact, comms_EvacuationPoint } from "@/lib/api/schema.gen";
import type { CreateDisasterAlertBody, DisasterAlert, UpdateDisasterAlertBody } from "@/lib/api/disaster-alerts";
import { instantToLocalInput, localInputToInstant } from "@/features/noi-dung/nhan-noi-dung";

/* ── The six kinds (ADR 0091 §2; `domain.DisasterKind`) ────────────────────────────────────────── */

export const ALERT_KINDS: readonly { readonly code: string; readonly label: string }[] = [
  { code: "bao", label: "Bão" },
  { code: "lu", label: "Lũ" },
  { code: "sat-lo", label: "Sạt lở" },
  { code: "mua-lon", label: "Mưa lớn" },
  { code: "dich-benh", label: "Dịch bệnh" },
  { code: "chay", label: "Cháy" },
];

/** The kind's word; an unknown code is shown as it came — never guessed into one of the six. */
export function kindLabel(code: string): string {
  return ALERT_KINDS.find((k) => k.code === code)?.label ?? code;
}

/* ── Bounds — `domain/disaster_alert.go:108-122` ──────────────────────────────────────────────── */

export const TITLE_MAX = 255;
export const HEADLINE_MAX = 255;
export const LEVEL_MAX = 100;
export const DETAIL_MAX = 10000;
export const ZONE_MIN_POINTS = 3;
export const ZONE_MAX_POINTS = 200;
export const LIST_MAX_ROWS = 50;
export const COORD_MIN = 0;
export const COORD_MAX = 100;

/* ── Words on the screen ───────────────────────────────────────────────────────────────────────── */

export const TAB_TITLE = "Cảnh báo thiên tai";
export const TAB_DESCRIPTION =
  "Cảnh báo hiện trên Zalo Mini App của xã từ lúc có hiệu lực, kèm băng đỏ ở trang chủ. Đăng cảnh báo không " +
  "gửi tin nhắn Zalo cho người dân.";
export const ADD_BUTTON = "Đăng cảnh báo";
export const LOADING = "Đang tải danh sách cảnh báo…";
export const LOAD_FAILED_TITLE = "Chưa tải được danh sách cảnh báo";
export const EMPTY_TITLE = "Chưa có cảnh báo thiên tai nào.";
export const EMPTY_HINT_EDITOR = "Bấm “Đăng cảnh báo” khi cần báo cho người dân.";
export const READ_ONLY_NOTE =
  "Bạn chỉ có quyền xem (content.read). Đăng, sửa, kết thúc hay xoá cảnh báo cần quyền content.update.";

export const COLUMN_KIND = "Loại";
export const COLUMN_TITLE = "Tiêu đề";
export const COLUMN_WINDOW = "Hiệu lực";
export const COLUMN_STATUS = "Trạng thái";
export const COLUMN_ACTIONS = "Thao tác";

export const EDIT_BUTTON = "Sửa";
export const END_BUTTON = "Kết thúc sớm";
export const DELETE_BUTTON = "Xoá";
export const CLOSE_LABEL = "Đóng";
export const CANCEL_BUTTON = "Huỷ";
export const SAVE_BUTTON = "Lưu";
export const PREV_PAGE = "Trang trước";
export const NEXT_PAGE = "Trang sau";

export const CREATE_TITLE = "Đăng cảnh báo thiên tai";
export const EDIT_TITLE = "Sửa cảnh báo thiên tai";
export const FORM_DESCRIPTION =
  "Người dân thấy cảnh báo trên Mini App từ thời điểm có hiệu lực. Vùng nguy hiểm chỉ là hình minh hoạ — hãy " +
  "ghi rõ thôn, xóm bị ảnh hưởng trong nội dung chi tiết.";

export const LABEL_KIND = "Loại cảnh báo";
export const KIND_PLACEHOLDER = "— Chọn loại —";
export const LABEL_TITLE = "Tiêu đề";
export const LABEL_LEVEL = "Mức độ";
export const LEVEL_HINT = "Ghi theo cấp độ rủi ro thiên tai, ví dụ “Cấp 3”. Để trống nếu chưa có.";
export const LABEL_HEADLINE = "Tóm tắt hiển thị ở băng trang chủ";
export const HEADLINE_HINT = "Một dòng trên băng đỏ của trang chủ Mini App. Để trống thì băng hiện tiêu đề.";
export const LABEL_DETAIL = "Nội dung chi tiết";
export const LABEL_FROM = "Có hiệu lực từ";
export const LABEL_TO = "Đến (tuỳ chọn)";
export const TIME_HINT = "Giờ Việt Nam (GMT+7). Để trống ô “Đến” thì cảnh báo còn hiệu lực tới khi kết thúc sớm.";
export const LABEL_ZONE = "Vùng nguy hiểm";
export const ZONE_HINT =
  "Bấm lên ô lưới để thêm điểm, kéo một điểm để dời. Cần ít nhất 3 điểm, hoặc để trống nếu không có vùng. " +
  "Lưới 0–100 là khung minh hoạ, không phải toạ độ bản đồ.";
export const ZONE_REMOVE_LAST = "Xoá điểm cuối";
export const ZONE_CLEAR = "Xoá vùng";
export const ZONE_ADD_POINT = "Thêm điểm";
export const ZONE_POINTS_LEGEND = "Các điểm của vùng (nhập bằng bàn phím)";
export const ZONE_CANVAS_LABEL = "Khung vẽ vùng nguy hiểm, lưới 0 đến 100";
export const LABEL_EVACUATION = "Điểm sơ tán";
export const LABEL_CONTACTS = "Số điện thoại khẩn cấp";
/** Owner's sentence, verbatim (11/10/2026): the list is public on the Mini App, without an account. */
export const CONTACTS_PUBLIC_HINT =
  "Số này hiện công khai cho mọi người dân — nên dùng số trực, đường dây nóng hoặc số cơ quan.";
export const ADD_EVACUATION = "Thêm điểm sơ tán";
export const ADD_CONTACT = "Thêm số khẩn cấp";
export const REMOVE_ROW = "Bỏ dòng";
export const EVAC_NAME = "Tên";
export const EVAC_ADDRESS = "Địa chỉ";
export const EVAC_PHONE = "Số điện thoại";
export const EVAC_CAPACITY = "Sức chứa";
export const CONTACT_ROLE = "Vai trò";
export const CONTACT_NAME = "Họ tên";
export const CONTACT_PHONE = "Số điện thoại";

export const CREATED_TOAST = "Đã đăng cảnh báo.";
export const SAVED_TOAST = "Đã lưu thay đổi.";
export const ENDED_TOAST = "Đã kết thúc cảnh báo. Mini App không còn hiện cảnh báo này.";
export const DELETED_TOAST = "Đã xoá cảnh báo.";
export const GONE_TOAST = "Cảnh báo này không còn nữa — danh sách đã được tải lại.";
export const SAVE_FAILED_TOAST = "Không lưu được. Vui lòng thử lại.";
export const NOTHING_CHANGED = "Chưa có ô nào thay đổi, nên không có gì để lưu.";
export const ENDED_NOT_EDITABLE =
  "Cảnh báo đã kết thúc nên không sửa được nữa — đó là bản ghi những gì người dân đã được báo. Hãy đăng cảnh báo mới.";

export const END_CONFIRM_TITLE = "Kết thúc sớm cảnh báo này?";
export const END_CONFIRM_NOTE =
  "Cảnh báo được gỡ khỏi Mini App ngay. Hệ thống ghi lại ai kết thúc và lúc nào; sau đó không sửa được nữa.";
export const DELETE_TITLE = "Xoá cảnh báo";
export const DELETE_NOTE =
  "Xoá là xoá mềm: cảnh báo được gỡ khỏi Mini App, nhưng bản ghi và lý do xoá vẫn được lưu.";
export const DELETE_REASON_LABEL = "Lý do xoá";

/* ── Status of one alert ───────────────────────────────────────────────────────────────────────── */

export type AlertStatus = "in-force" | "upcoming" | "ended";

/**
 * The badge. `in_force` is the SERVER's derivation at the reply (never stored); the rest splits on the
 * row's own times. "Đã kết thúc" covers both an alert ended early and one whose end time has passed —
 * residents see neither.
 */
export function alertStatus(a: Pick<DisasterAlert, "in_force" | "ended_at" | "effective_from">, now: number): AlertStatus {
  if (a.in_force) return "in-force";
  if (a.ended_at === undefined || a.ended_at === null || a.ended_at === "") {
    const from = Date.parse(a.effective_from);
    if (!Number.isNaN(from) && from > now) return "upcoming";
  }
  return "ended";
}

export const STATUS_LABEL: Record<AlertStatus, string> = {
  "in-force": "Đang hiệu lực",
  upcoming: "Sắp hiệu lực",
  ended: "Đã kết thúc",
};

/** Ended EARLY (by staff) — the server answers 409 to an edit or a second ending. */
export function isEndedEarly(a: Pick<DisasterAlert, "ended_at">): boolean {
  return typeof a.ended_at === "string" && a.ended_at !== "";
}

/** `Kết thúc sớm` makes sense while residents see it or are about to. */
export function canEndEarly(a: DisasterAlert, now: number): boolean {
  const s = alertStatus(a, now);
  return !isEndedEarly(a) && (s === "in-force" || s === "upcoming");
}

/** `HH:mm dd/MM/yyyy`, Vietnam time; "" when unreadable. */
export function formatInstant(instant: string | null | undefined): string {
  const local = instantToLocalInput(instant);
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(local);
  return m === null ? "" : `${m[4]}:${m[5]} ${m[3]}/${m[2]}/${m[1]}`;
}

/** The list's `Hiệu lực` cell. */
export function validityText(a: Pick<DisasterAlert, "effective_from" | "effective_to">): string {
  const from = formatInstant(a.effective_from);
  const to = formatInstant(a.effective_to);
  return to === "" ? `Từ ${from} — tới khi kết thúc` : `${from} – ${to}`;
}

/* ── The form state ────────────────────────────────────────────────────────────────────────────── */

export type EvacuationRow = { readonly name: string; readonly address: string; readonly phone: string; readonly capacity: string };
export type ContactRow = { readonly role: string; readonly name: string; readonly phone: string };

export type AlertFormValues = {
  readonly kind: string;
  readonly title: string;
  readonly level: string;
  readonly headline: string;
  readonly detail: string;
  /** `datetime-local` values; the `+07:00` is added once, in the body builders. */
  readonly fromLocal: string;
  readonly toLocal: string;
  readonly zone: readonly comms_DangerZonePoint[];
  readonly evacuation: readonly EvacuationRow[];
  readonly contacts: readonly ContactRow[];
};

/** No kind preselected: the officer chooses one of the six, a default would be the system choosing. */
export const EMPTY_FORM: AlertFormValues = {
  kind: "",
  title: "",
  level: "",
  headline: "",
  detail: "",
  fromLocal: "",
  toLocal: "",
  zone: [],
  evacuation: [],
  contacts: [],
};

export const EMPTY_EVACUATION_ROW: EvacuationRow = { name: "", address: "", phone: "", capacity: "" };
export const EMPTY_CONTACT_ROW: ContactRow = { role: "", name: "", phone: "" };

export function formFromAlert(a: DisasterAlert): AlertFormValues {
  return {
    kind: a.kind,
    title: a.title,
    level: a.level ?? "",
    headline: a.headline ?? "",
    detail: a.detail,
    fromLocal: instantToLocalInput(a.effective_from),
    toLocal: instantToLocalInput(a.effective_to),
    zone: a.danger_zone.map((p) => ({ x: p.x, y: p.y })),
    evacuation: a.evacuation_points.map((p) => ({
      name: p.name,
      address: p.address ?? "",
      phone: p.phone ?? "",
      capacity: p.capacity === undefined || p.capacity === null ? "" : String(p.capacity),
    })),
    contacts: a.emergency_contacts.map((c) => ({ role: c.role, name: c.name, phone: c.phone })),
  };
}

/* ── Client-side checks ────────────────────────────────────────────────────────────────────────── */

export type AlertFormField = "kind" | "title" | "level" | "headline" | "detail" | "from" | "to" | "zone" | "evacuation" | "contacts";
export type AlertFormErrors = Partial<Record<AlertFormField, string>>;

const CONTROL_CHAR = /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/;
const LOCAL_INPUT = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2})?$/;

function runes(s: string): number {
  return [...s].length;
}

/** Whole non-negative number as typed, or `null`. "" is not a number — the caller decides what it means. */
export function parseCapacity(raw: string): number | null {
  const s = raw.trim();
  if (!/^\d{1,9}$/.test(s)) return null;
  return Number(s);
}

/** The zone rule: none, or 3..200 points, each inside 0..100. `null` = fine. */
export function zoneProblem(zone: readonly comms_DangerZonePoint[]): string | null {
  if (zone.length === 0) return null;
  if (zone.length < ZONE_MIN_POINTS) return `Vùng nguy hiểm cần ít nhất ${ZONE_MIN_POINTS} điểm (đang có ${zone.length}), hoặc xoá vùng.`;
  if (zone.length > ZONE_MAX_POINTS) return `Vùng nguy hiểm tối đa ${ZONE_MAX_POINTS} điểm.`;
  const bad = zone.findIndex(
    (p) => !Number.isFinite(p.x) || !Number.isFinite(p.y) || p.x < COORD_MIN || p.x > COORD_MAX || p.y < COORD_MIN || p.y > COORD_MAX,
  );
  return bad < 0 ? null : `Điểm thứ ${bad + 1} của vùng phải có X, Y từ ${COORD_MIN} đến ${COORD_MAX}.`;
}

/** Every problem the form can see, by field. `{}` = Lưu may go. */
export function validateAlertForm(v: AlertFormValues): AlertFormErrors {
  const e: AlertFormErrors = {};
  if (!ALERT_KINDS.some((k) => k.code === v.kind)) e.kind = "Vui lòng chọn loại cảnh báo.";
  const title = v.title.trim();
  if (title === "") e.title = "Vui lòng nhập tiêu đề.";
  else if (runes(title) > TITLE_MAX) e.title = `Tiêu đề tối đa ${TITLE_MAX} ký tự.`;
  if (runes(v.level.trim()) > LEVEL_MAX || CONTROL_CHAR.test(v.level)) e.level = `Mức độ tối đa ${LEVEL_MAX} ký tự, một dòng.`;
  if (runes(v.headline.trim()) > HEADLINE_MAX || /[\r\n]/.test(v.headline)) {
    e.headline = `Tóm tắt tối đa ${HEADLINE_MAX} ký tự, một dòng.`;
  }
  const detail = v.detail.trim();
  if (detail === "") e.detail = "Vui lòng nhập nội dung chi tiết.";
  else if (runes(detail) > DETAIL_MAX) e.detail = `Nội dung chi tiết tối đa ${DETAIL_MAX} ký tự.`;
  if (v.fromLocal === "") e.from = "Vui lòng chọn thời điểm bắt đầu có hiệu lực.";
  else if (!LOCAL_INPUT.test(v.fromLocal)) e.from = "Thời điểm bắt đầu không hợp lệ.";
  if (v.toLocal !== "") {
    if (!LOCAL_INPUT.test(v.toLocal)) e.to = "Thời điểm kết thúc không hợp lệ.";
    else if (e.from === undefined && localInputToInstant(v.toLocal) <= localInputToInstant(v.fromLocal)) {
      e.to = "Thời điểm “Đến” phải sau thời điểm bắt đầu có hiệu lực.";
    }
  }
  const zone = zoneProblem(v.zone);
  if (zone !== null) e.zone = zone;
  if (v.evacuation.length > LIST_MAX_ROWS) e.evacuation = `Tối đa ${LIST_MAX_ROWS} điểm sơ tán.`;
  else {
    const i = v.evacuation.findIndex((r) => r.name.trim() === "");
    const j = v.evacuation.findIndex((r) => r.capacity.trim() !== "" && parseCapacity(r.capacity) === null);
    if (i >= 0) e.evacuation = `Điểm sơ tán thứ ${i + 1} cần có tên.`;
    else if (j >= 0) e.evacuation = `Sức chứa của điểm sơ tán thứ ${j + 1} phải là số nguyên không âm.`;
  }
  if (v.contacts.length > LIST_MAX_ROWS) e.contacts = `Tối đa ${LIST_MAX_ROWS} số khẩn cấp.`;
  else {
    const i = v.contacts.findIndex((c) => c.role.trim() === "" || c.name.trim() === "" || c.phone.trim() === "");
    if (i >= 0) e.contacts = `Số khẩn cấp thứ ${i + 1} cần đủ vai trò, họ tên và số điện thoại.`;
  }
  return e;
}

export function hasErrors(e: AlertFormErrors): boolean {
  return Object.keys(e).length > 0;
}

/* ── Request bodies ────────────────────────────────────────────────────────────────────────────── */

function evacuationOut(rows: readonly EvacuationRow[]): comms_EvacuationPoint[] {
  return rows.map((r) => {
    const p: comms_EvacuationPoint = { name: r.name.trim() };
    const address = r.address.trim();
    const phone = r.phone.trim();
    const capacity = parseCapacity(r.capacity);
    if (address !== "") p.address = address;
    if (phone !== "") p.phone = phone;
    if (capacity !== null) p.capacity = capacity;
    return p;
  });
}

function contactsOut(rows: readonly ContactRow[]): comms_EmergencyContact[] {
  return rows.map((c) => ({ role: c.role.trim(), name: c.name.trim(), phone: c.phone.trim() }));
}

function zoneOut(zone: readonly comms_DangerZonePoint[]): comms_DangerZonePoint[] {
  return zone.map((p) => ({ x: p.x, y: p.y }));
}

/** POST body. Optional texts and lists only when they hold something. */
export function createBody(v: AlertFormValues): CreateDisasterAlertBody {
  const body: CreateDisasterAlertBody = {
    kind: v.kind,
    title: v.title.trim(),
    detail: v.detail.trim(),
    effective_from: localInputToInstant(v.fromLocal),
  };
  const level = v.level.trim();
  const headline = v.headline.trim();
  if (level !== "") body.level = level;
  if (headline !== "") body.headline = headline;
  if (v.toLocal !== "") body.effective_to = localInputToInstant(v.toLocal);
  if (v.zone.length > 0) body.danger_zone = zoneOut(v.zone);
  if (v.evacuation.length > 0) body.evacuation_points = evacuationOut(v.evacuation);
  if (v.contacts.length > 0) body.emergency_contacts = contactsOut(v.contacts);
  return body;
}

/**
 * PATCH body — ONLY what changed. Absent = leave alone at the server; `""` CLEARS `level`, `headline` and
 * `effective_to`; a list REPLACES the stored one whole (`[]` clears). Sending every field on each save
 * would rewrite the trail with "changes" nobody made.
 */
export function patchBody(before: AlertFormValues, after: AlertFormValues): UpdateDisasterAlertBody {
  const out: UpdateDisasterAlertBody = {};
  if (after.kind !== before.kind) out.kind = after.kind;
  if (after.title.trim() !== before.title.trim()) out.title = after.title.trim();
  if (after.level.trim() !== before.level.trim()) out.level = after.level.trim();
  if (after.headline.trim() !== before.headline.trim()) out.headline = after.headline.trim();
  if (after.detail.trim() !== before.detail.trim()) out.detail = after.detail.trim();
  if (after.fromLocal !== before.fromLocal) out.effective_from = localInputToInstant(after.fromLocal);
  if (after.toLocal !== before.toLocal) out.effective_to = after.toLocal === "" ? "" : localInputToInstant(after.toLocal);
  if (JSON.stringify(zoneOut(after.zone)) !== JSON.stringify(zoneOut(before.zone))) out.danger_zone = zoneOut(after.zone);
  if (JSON.stringify(evacuationOut(after.evacuation)) !== JSON.stringify(evacuationOut(before.evacuation))) {
    out.evacuation_points = evacuationOut(after.evacuation);
  }
  if (JSON.stringify(contactsOut(after.contacts)) !== JSON.stringify(contactsOut(before.contacts))) {
    out.emergency_contacts = contactsOut(after.contacts);
  }
  return out;
}

/* ── The danger-zone canvas ────────────────────────────────────────────────────────────────────── */

/** One decimal, clamped to the grid — what the server accepts and what the Mini App draws. */
export function clampCoord(n: number): number {
  if (!Number.isFinite(n)) return COORD_MIN;
  const c = Math.min(COORD_MAX, Math.max(COORD_MIN, n));
  return Math.round(c * 10) / 10;
}

/** A pointer position over the canvas's on-screen box → a grid point. A zero-size box gives `null`. */
export function toGridPoint(
  clientX: number,
  clientY: number,
  rect: { readonly left: number; readonly top: number; readonly width: number; readonly height: number },
): comms_DangerZonePoint | null {
  if (rect.width <= 0 || rect.height <= 0) return null;
  return {
    x: clampCoord(((clientX - rect.left) / rect.width) * COORD_MAX),
    y: clampCoord(((clientY - rect.top) / rect.height) * COORD_MAX),
  };
}

export function polygonPoints(zone: readonly comms_DangerZonePoint[]): string {
  return zone.map((p) => `${p.x},${p.y}`).join(" ");
}

/* ── The server's sentence, field token in Vietnamese ──────────────────────────────────────────── */

const FIELD_LABELS: Readonly<Record<string, string>> = {
  kind: LABEL_KIND,
  title: LABEL_TITLE,
  level: LABEL_LEVEL,
  headline: LABEL_HEADLINE,
  detail: LABEL_DETAIL,
  effective_from: LABEL_FROM,
  effective_to: "Đến",
  danger_zone: LABEL_ZONE,
  evacuation_points: LABEL_EVACUATION,
  emergency_contacts: LABEL_CONTACTS,
};

/**
 * The server names a field by its wire name in backticks ("`danger_zone` không hợp lệ: …"). This swaps
 * ONLY that token for the box's label; the rule in the sentence stays the server's, word for word. A
 * token not in the table is left as it came — a changed sentence degrades to the raw one, never to a guess.
 */
export function localizeAlertRefusal(message: string): string {
  const out = message.replace(/`([a-z_]+)`/g, (whole, name: string) => FIELD_LABELS[name] ?? whole);
  return out.charAt(0).toLocaleUpperCase("vi") + out.slice(1);
}
