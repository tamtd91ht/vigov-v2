/**
 * Words and decisions of the CITIZEN-LETTER register (`Đơn thư công dân` and `Báo cáo` tabs), PURE:
 * no network, no DOM, and "now" is always PASSED IN — a function reading the clock cannot be tested at
 * the one moment that matters, just before and just after a deadline.
 *
 * THE BUSINESS RULES ARE THE CUSTOMER'S, THE PRESENTATION THE PROTOTYPE'S (ADR 0078 #1). Where they
 * differ the rule wins, drawn in the prototype's place and style:
 *   - statuses are C3's ten (TT 05/2021), not the prototype's seven; assignment is an ATTRIBUTE;
 *   - types are C4's four codes;
 *   - the sender is optional (C7);
 *   - deadlines are NEVER computed here — both are null today and read "Không đặt hạn" (ADR 0078 #3,
 *     rule 10); lateness, when a deadline exists, is DERIVED from it against `now`, never stored;
 *   - the phone is shown exactly as the server masked it, the address never (ADR 0078 #4, rule 3).
 *
 * ⚠ THE LABEL TABLES ARE HAND COPIES of `service-documents/internal/domain/citizen_letter.go` — the
 * contract carries `status` and `letter_type` as plain strings (ledger `apidoc-sinh-enum-cho-truong`).
 * An unknown code therefore falls to a sentence that SAYS it has no label, never to a guess.
 */

import { nhanNgay } from "@/features/cau-hinh/nhan-lich-lam-viec";
import { nhanThoiDiem } from "@/features/phan-anh/nhan-phieu";
import type {
  documents_bookLetterIn,
  documents_citizenLetterItemOut,
  documents_citizenLetterOut,
  documents_duplicateCheckIn,
  documents_letterRoutingIn,
  documents_senderCorrectionIn,
} from "@/lib/api/schema.gen";
import { PETITION_CREATE_PERMISSION, PETITION_READ_PERMISSION } from "@/lib/quyen";

/* ---- types (C4) --------------------------------------------------------------------------- */

/** The four letter types in the prototype's select order (`document-display.ts:209-214`), our codes. */
export const LETTER_TYPES: readonly { code: string; label: string }[] = [
  { code: "khieu-nai", label: "Khiếu nại" },
  { code: "to-cao", label: "Tố cáo" },
  { code: "kien-nghi-phan-anh", label: "Kiến nghị, phản ánh" },
  { code: "de-nghi", label: "Đề nghị" },
];

/** The booking form's default type — the prototype's default (`PetitionEntryForm.tsx:57`). */
export const DEFAULT_LETTER_TYPE = "kien-nghi-phan-anh";

/** A denunciation: its sender is protected (Luật Tố cáo 2018 Đ.8). */
export const DENUNCIATION_TYPE = "to-cao";

export function letterTypeLabel(code: string): string {
  const found = LETTER_TYPES.find((t) => t.code === code);
  if (found !== undefined) return found.label;
  return code === "" ? "Không ghi loại đơn" : `${code} (mã loại đơn chưa có nhãn trên màn hình này)`;
}

/* ---- statuses (C3) ------------------------------------------------------------------------ */

/**
 * The ten statuses: label (glossary / ledger C3), the prototype's chip colours by the nearest step of
 * ITS flow (`document-display.ts:52-84, 167-175`), and one hint line under the strip
 * (`PetitionDetailDrawer.tsx:431-433`).
 */
type StatusLook = {
  readonly label: string;
  /** Pill: `bg-{c}/12 text-{c} border-{c}/25`. */
  readonly chip: string;
  /** The strip's "đang ở đây" chip. */
  readonly active: string;
  readonly hint: string;
};

const STATUS: Readonly<Record<string, StatusLook>> = {
  "moi-vao-so": {
    label: "Mới vào sổ",
    chip: "border-line bg-ink-muted/12 text-ink",
    active: "bg-ink-muted text-white",
    hint: "Đã vào sổ, chưa xem xét, xử lý đơn.",
  },
  "dang-xu-ly-don": {
    label: "Đang xử lý đơn",
    chip: "border-tangerine/25 bg-tangerine/12 text-tangerine",
    active: "bg-tangerine text-white",
    hint: "Đang xem xét để thụ lý, hướng dẫn, chuyển đơn hoặc lưu đơn.",
  },
  "thu-ly": {
    label: "Thụ lý",
    chip: "border-brand/25 bg-brand/12 text-brand",
    active: "bg-brand text-white",
    hint: "Đã thụ lý đơn để giải quyết.",
  },
  "dang-giai-quyet": {
    label: "Đang giải quyết",
    chip: "border-teal/25 bg-teal/12 text-teal",
    active: "bg-teal text-white",
    hint: "Đang giải quyết đơn. Ghi kết quả giải quyết trước khi chuyển sang Đã giải quyết.",
  },
  "da-giai-quyet": {
    label: "Đã giải quyết",
    chip: "border-leaf/25 bg-leaf/12 text-leaf",
    active: "bg-leaf text-white",
    hint: "Đã giải quyết xong, kết quả đã được ghi.",
  },
  "khong-thu-ly": {
    label: "Không thụ lý",
    chip: "border-line bg-ink/12 text-ink",
    active: "bg-ink text-white",
    hint: "Đơn không đủ điều kiện thụ lý.",
  },
  "huong-dan": {
    label: "Hướng dẫn",
    chip: "border-violet/25 bg-violet/12 text-violet",
    active: "bg-violet text-white",
    hint: "Đã hướng dẫn người gửi đến cơ quan có thẩm quyền.",
  },
  "chuyen-don": {
    label: "Chuyển đơn",
    chip: "border-violet/25 bg-violet/12 text-violet",
    active: "bg-violet text-white",
    hint: "Đã chuyển đơn đến cơ quan có thẩm quyền giải quyết.",
  },
  "luu-don": {
    label: "Lưu đơn",
    chip: "border-line bg-ink/12 text-ink",
    active: "bg-ink text-white",
    hint: "Đơn đã được lưu, không xử lý tiếp.",
  },
  "dinh-chi": {
    label: "Đình chỉ",
    chip: "border-line bg-ink/12 text-ink",
    active: "bg-ink text-white",
    hint: "Việc giải quyết đơn đã bị đình chỉ.",
  },
};

/** The ten codes, for the status filter — in the strip's reading order. */
export const LETTER_STATUS_CODES: readonly string[] = [
  "moi-vao-so",
  "dang-xu-ly-don",
  "thu-ly",
  "dang-giai-quyet",
  "da-giai-quyet",
  "khong-thu-ly",
  "huong-dan",
  "chuyen-don",
  "luu-don",
  "dinh-chi",
];

/** The strip's main row (ADR 0078 C3): the path a letter takes when it is admitted and resolved. */
export const LETTER_MAIN_FLOW: readonly string[] = [
  "moi-vao-so",
  "dang-xu-ly-don",
  "thu-ly",
  "dang-giai-quyet",
  "da-giai-quyet",
];

/** The strip's "Rẽ nhánh:" row — the other ends (four in processing, one in resolution). */
export const LETTER_BRANCHES: readonly string[] = ["khong-thu-ly", "huong-dan", "chuyen-don", "luu-don", "dinh-chi"];

export function letterStatusLabel(code: string): string {
  const look = STATUS[code];
  if (look !== undefined) return look.label;
  return code === "" ? "Không ghi trạng thái" : `${code} (mã trạng thái chưa có nhãn trên màn hình này)`;
}

/** The pill's colour classes; an unknown code gets the neutral grey. */
export function letterStatusChip(code: string): string {
  return STATUS[code]?.chip ?? "border-line bg-ink-muted/12 text-ink";
}

export function letterStatusActiveTone(code: string): string {
  return STATUS[code]?.active ?? "bg-ink-muted text-white";
}

export function letterStatusHint(code: string): string {
  return STATUS[code]?.hint ?? "";
}

/** One chip of the status strip. */
export type StatusChipModel = {
  readonly code: string;
  readonly label: string;
  readonly current: boolean;
  readonly clickable: boolean;
};

/**
 * The strip: the main row, and the branch row with ONLY the branches that are the current status or a
 * move the server allows from it (ADR 0078, card W2). A chip is CLICKABLE only when the server's
 * `next_statuses` holds it AND the viewer may work on the letter — the server decides the arrows; this
 * never draws one of its own.
 */
export function statusStrip(
  current: string,
  nextStatuses: readonly string[],
  mayWork: boolean,
): { main: StatusChipModel[]; branches: StatusChipModel[] } {
  const chip = (code: string): StatusChipModel => ({
    code,
    label: letterStatusLabel(code),
    current: code === current,
    clickable: mayWork && code !== current && nextStatuses.includes(code),
  });
  return {
    main: LETTER_MAIN_FLOW.map(chip),
    branches: LETTER_BRANCHES.filter((code) => code === current || nextStatuses.includes(code)).map(chip),
  };
}

/** The result form is open only in these two statuses (`ErrLetterResultNotAllowed`). */
export const RESULT_EDITABLE_STATUSES: readonly string[] = ["thu-ly", "dang-giai-quyet"];

/* ---- permissions (C13/C14) — UX ONLY, the server decides --------------------------------- */

/** Book, route, correct the sender: `petition.create`. */
export function canBookLetters(permissions: readonly string[]): boolean {
  return permissions.includes(PETITION_CREATE_PERMISSION);
}

/**
 * Status / result / log: `petition.read` AND (the letter's assignee OR `petition.create`). The staff
 * code is the SESSION's; an empty one never matches an empty assignee.
 */
export function mayWorkOnLetter(permissions: readonly string[], staffCode: string, assigneeCode: string): boolean {
  if (!permissions.includes(PETITION_READ_PERMISSION)) return false;
  if (permissions.includes(PETITION_CREATE_PERMISSION)) return true;
  return staffCode !== "" && staffCode === assigneeCode;
}

/* ---- deadlines (rule 10) ------------------------------------------------------------------ */

/** The words of a letter with no deadline — the prototype's own state (`document-display.ts:228`). */
export const NO_DEADLINE = "Không đặt hạn";

type DueFields = Pick<documents_citizenLetterItemOut, "status" | "processing_due_at" | "resolution_due_at">;

/**
 * The STORED deadline that applies to the letter's current phase — `domain.ActiveDueAt`: processing
 * before admission, resolution after it, none once finished. Read, never computed.
 */
export function activeDue(letter: DueFields): string | null {
  switch (letter.status) {
    case "moi-vao-so":
    case "dang-xu-ly-don":
      return letter.processing_due_at;
    case "thu-ly":
    case "dang-giai-quyet":
      return letter.resolution_due_at;
    default:
      return null;
  }
}

export type DueTone = "overdue" | "normal" | "none";

/** The prototype's deadline colours (`document-display.ts:248-253`), less its "soon" tone (no count). */
export const DUE_TONE_CLASS: Readonly<Record<DueTone, string>> = {
  overdue: "font-semibold text-danger",
  normal: "text-ink",
  none: "text-ink-muted",
};

/**
 * The "Hạn giải quyết" words: "Không đặt hạn", "Hạn {dd/MM/yyyy}", or — open AND past the stored
 * instant — "Quá hạn · {dd/MM/yyyy}". A COMPARISON of two instants, never a deadline computed here
 * (rule 10, invariants 2–3).
 *
 * NO "Còn N ngày" / "Quá hạn N ngày" COUNT (owner decision 08/10/2026, R1): such a count is a duration,
 * and its unit — working hours (ADR 0007) or calendar days for complaints/denunciations (ADR 0064) — is
 * the server's to apply on the commune's calendar. A wall-clock count here would disagree with the
 * server's figure on every holiday; the incoming register refuses it for the same reason
 * (`nhan-van-ban.ts:11-16`).
 */
export function dueLabel(dueISO: string | null, closed: boolean, now: Date): { text: string; tone: DueTone } {
  if (dueISO === null || dueISO === "") return { text: NO_DEADLINE, tone: "none" };
  const due = new Date(dueISO);
  if (Number.isNaN(due.getTime())) return { text: dueISO, tone: "normal" };
  if (!closed && due.getTime() < now.getTime()) return { text: `Quá hạn · ${deadlineDate(dueISO)}`, tone: "overdue" };
  return { text: `Hạn ${deadlineDate(dueISO)}`, tone: "normal" };
}

/** The drawer's "Hạn xử lý" figure: the stored date, or "Không đặt hạn" — never a count. */
export function dueDateText(dueISO: string | null): string {
  if (dueISO === null || dueISO === "") return NO_DEADLINE;
  return Number.isNaN(new Date(dueISO).getTime()) ? dueISO : deadlineDate(dueISO);
}

/** Whether an OPEN letter is past its stored, phase-relevant deadline. Derived at render time. */
export function isPastDue(letter: DueFields & { is_closed: boolean }, now: Date): boolean {
  if (letter.is_closed) return false;
  const due = activeDue(letter);
  if (due === null || due === "") return false;
  const at = new Date(due).getTime();
  return !Number.isNaN(at) && at < now.getTime();
}

/** `dd/MM/yyyy` of a deadline instant, pinned to the Vietnamese zone through `nhanThoiDiem`. */
function deadlineDate(dueISO: string): string {
  const full = nhanThoiDiem(dueISO);
  return full.slice(full.lastIndexOf(" ") + 1);
}

/* ---- the sender (C7, ADR 0078 #4) --------------------------------------------------------- */

/** A denunciation on a list: the sender is protected, whoever looks. */
export const WITHHELD_SENDER = "Người gửi được giữ bí mật";
/** A denunciation's summary where it is withheld. */
export const WITHHELD_SUMMARY = "Nội dung đơn tố cáo được bảo mật";
/** C7's "Không rõ người gửi" — derived by the server (`sender_unknown`), never a stored flag. */
export const SENDER_UNKNOWN = "Không rõ người gửi";
/** No name is shown (none given, or a list row that cannot tell whether anything else was). */
export const NO_SENDER_NAME = "Không ghi họ tên";

export type SenderView =
  | { kind: "withheld" }
  | { kind: "unknown" }
  | { kind: "named"; name: string; phone: string | null };

/**
 * What the sender cell / line shows. The phone is the server's MASKED string, passed through as text —
 * never a `tel:` link and never unmasked (rule 3). The address is never in this shape at all.
 */
export function senderView(
  letter: Pick<documents_citizenLetterItemOut, "identity_withheld" | "sender_name" | "sender_phone"> & {
    sender_unknown?: boolean | null;
  },
): SenderView {
  if (letter.identity_withheld) return { kind: "withheld" };
  // ONLY the server's derived flag says "unknown" (the drawer has it). A list row does not carry the
  // address, so a row with no name and no phone may still have one: it reads "Không ghi họ tên", which
  // is true either way, instead of "Không rõ người gửi", which might not be.
  if (letter.sender_unknown === true) return { kind: "unknown" };
  return { kind: "named", name: letter.sender_name ?? NO_SENDER_NAME, phone: letter.sender_phone };
}

/** The address cell of the drawer: never the address itself. */
export function senderAddressText(letter: Pick<documents_citizenLetterOut, "identity_withheld" | "has_sender_address">): string {
  if (letter.identity_withheld) return WITHHELD_SENDER;
  return letter.has_sender_address ? "Có địa chỉ (không hiển thị)" : "—";
}

/* ---- "Số ngày xử lý" (C16/C17) ------------------------------------------------------------ */

/**
 * The server's `days_open` — counted on the server, the received day included, stopped when the letter
 * closed. A running count is bold navy; a stopped one muted with the reason it stopped
 * (`PetitionTable.tsx:87-107`): "đã giải quyết" for a resolved letter, "đã kết thúc" for any other end.
 */
export function daysOpenView(letter: Pick<documents_citizenLetterItemOut, "days_open" | "is_closed" | "status">): {
  text: string;
  stopped: boolean;
  note: string | null;
} {
  const text = `${letter.days_open} ngày`;
  if (!letter.is_closed) return { text, stopped: false, note: null };
  return { text, stopped: true, note: letter.status === "da-giai-quyet" ? "đã giải quyết" : "đã kết thúc" };
}

/* ---- numbers and dates -------------------------------------------------------------------- */

/** `7/2026` — the number never stands without its year: the series restarts every year. */
export function letterNumber(number: number, year: number): string {
  return `${number}/${year}`;
}

/** `dd/MM/yyyy` of a `YYYY-MM-DD` — cut as text, never through `Date` (a date has no zone). */
export function letterDate(iso: string): string {
  return iso === "" ? "—" : nhanNgay(iso);
}

/** A report figure: `null` → "—", never 0 (a 0 is a figure somebody reports upward). */
export function reportFigure(value: number | null, suffix = ""): string {
  return value === null ? "—" : `${value.toLocaleString("vi-VN")}${suffix}`;
}

/* ---- drafts → request bodies ------------------------------------------------------------- */

export type Built<T> = { ok: true; body: T } | { ok: false; error: string };

/** The booking form's draft. Strings throughout — browser inputs return strings. */
export type EntryDraft = {
  receivedDate: string;
  senderName: string;
  senderPhone: string;
  senderAddress: string;
  /** C7's "Không rõ người gửi": the three sender fields are cleared and disabled. */
  senderUnknown: boolean;
  letterType: string;
  summary: string;
  /** "Chuyển ngay cho bộ phận" — `holding_unit_id`. */
  holdingUnit: string;
  /** The duplicate the clerk CONFIRMED with "Gộp vào đơn này" (C11). Empty = none. */
  relatedLetterId: string;
};

export function emptyEntryDraft(today: string): EntryDraft {
  return {
    receivedDate: today,
    senderName: "",
    senderPhone: "",
    senderAddress: "",
    senderUnknown: false,
    letterType: DEFAULT_LETTER_TYPE,
    summary: "",
    holdingUnit: "",
    relatedLetterId: "",
  };
}

/** The ONE client check of the booking form (C7: the sender is optional). Everything else is the server's. */
export const SUMMARY_REQUIRED = "Cần nội dung đơn.";

type BookingBody = Omit<documents_bookLetterIn, "number" | "status" | "processing_due_at" | "resolution_due_at">;

export function buildBooking(draft: EntryDraft): Built<BookingBody> {
  const summary = draft.summary.trim();
  if (summary === "") return { ok: false, error: SUMMARY_REQUIRED };
  const body: BookingBody = { received_date: draft.receivedDate, letter_type: draft.letterType, summary };
  if (!draft.senderUnknown) {
    const name = draft.senderName.trim();
    const phone = draft.senderPhone.trim();
    const address = draft.senderAddress.trim();
    if (name !== "") body.sender_name = name;
    if (phone !== "") body.sender_phone = phone;
    if (address !== "") body.sender_address = address;
  }
  if (draft.relatedLetterId !== "") body.related_letter_id = draft.relatedLetterId;
  if (draft.holdingUnit !== "") body.holding_unit_id = draft.holdingUnit;
  return { ok: true, body };
}

/**
 * The duplicate check's BODY, or `null` when it should not run: the sender's name and the summary are
 * both typed (the prototype's thresholds, `> 1` and `> 10` characters) and the sender is not "unknown".
 */
export function duplicateQuery(draft: EntryDraft): documents_duplicateCheckIn | null {
  if (draft.senderUnknown) return null;
  const name = draft.senderName.trim();
  const summary = draft.summary.trim();
  if (name.length <= 1 || summary.length <= 10) return null;
  return { sender_name: name, summary, letter_type: draft.letterType };
}

/** The routing box's draft. */
export type RoutingDraft = { toUnit: string; assignee: string; reason: string };
export const EMPTY_ROUTING: RoutingDraft = { toUnit: "", assignee: "", reason: "" };
/** The prototype's sentence (`PetitionDetailDrawer.tsx:199`). */
export const ROUTING_INCOMPLETE = "Cần chọn bộ phận và ghi lý do chuyển.";

export function buildRouting(draft: RoutingDraft): Built<documents_letterRoutingIn> {
  const toUnit = draft.toUnit.trim();
  const reason = draft.reason.trim();
  if (toUnit === "" || reason === "") return { ok: false, error: ROUTING_INCOMPLETE };
  const body: documents_letterRoutingIn = { to_unit: toUnit, reason };
  const assignee = draft.assignee.trim();
  if (assignee !== "") body.assignee = assignee;
  return { ok: true, body };
}

/**
 * The sender-correction draft. THE FORM STARTS EMPTY (ADR 0078 #4): the full values are never sent to
 * this screen, so an empty field means "leave as is" and is NOT sent; "Không rõ người gửi" clears all
 * three (`null`), which is also how a Decree 13 anonymisation is recorded.
 */
export type SenderDraft = { name: string; phone: string; address: string; clearAll: boolean };
export const EMPTY_SENDER: SenderDraft = { name: "", phone: "", address: "", clearAll: false };
export const SENDER_NOTHING_TO_SAVE = "Nhập ít nhất một thông tin cần sửa, hoặc chọn “Không rõ người gửi”.";

export function buildSenderCorrection(draft: SenderDraft): Built<documents_senderCorrectionIn> {
  if (draft.clearAll) return { ok: true, body: { sender_name: null, sender_phone: null, sender_address: null } };
  const body: documents_senderCorrectionIn = {};
  if (draft.name.trim() !== "") body.sender_name = draft.name.trim();
  if (draft.phone.trim() !== "") body.sender_phone = draft.phone.trim();
  if (draft.address.trim() !== "") body.sender_address = draft.address.trim();
  if (Object.keys(body).length === 0) return { ok: false, error: SENDER_NOTHING_TO_SAVE };
  return { ok: true, body };
}

/** The result form's draft (C10). Sent as typed; the server says what is missing. */
export type ResultDraft = { documentNo: string; documentDate: string; signer: string; issuer: string; summary: string };

export function resultDraftOf(letter: documents_citizenLetterOut): ResultDraft {
  return {
    documentNo: letter.result_document_no ?? "",
    documentDate: letter.result_document_date ?? "",
    signer: letter.result_signer ?? "",
    issuer: letter.result_issuer ?? "",
    summary: letter.result_summary ?? "",
  };
}

/** Two initials of a name for the log's avatar (`PetitionDetailDrawer.tsx:876-892`); "?" when none. */
export function initialsOf(name: string): string {
  const letters = name
    .split(/\s+/)
    .filter((part) => part !== "" && !/^\(?CB-/.test(part))
    .slice(-2)
    .map((part) => part[0] ?? "")
    .join("")
    .toUpperCase();
  return letters === "" ? "?" : letters;
}
