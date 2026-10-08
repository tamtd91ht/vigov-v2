/**
 * Words and decisions of the CITIZEN-LETTER register (`Đơn thư công dân` and `Báo cáo` tabs), PURE:
 * no network, no DOM, and "now" is always PASSED IN — a function reading the clock cannot be tested at
 * the one moment that matters, just before and just after a deadline.
 *
 * THE BUSINESS RULES ARE THE CUSTOMER'S, THE PRESENTATION THE PROTOTYPE'S (ADR 0078 #1). Where they
 * differ the rule wins, drawn in the prototype's place and style:
 *   - the DATA keeps C3's ten statuses (TT 05/2021); the screen DRAWS the prototype's display groups,
 *     which the server derives and sends as `status_group` (ADR 0084 #2); assignment is an ATTRIBUTE;
 *   - types are C4's four codes;
 *   - the sender is optional (C7);
 *   - deadlines are NEVER computed here — the server fills them at booking / admission by the commune's
 *     rule per type (ADR 0084 #3, ADR 0085 B) and the clerk may still change them; unset reads "Không đặt
 *     hạn" / "Không đặt" (rule 10); lateness is DERIVED from the stored instant against `now`, never stored;
 *   - the phone is shown exactly as the server masked it, the address never (ADR 0078 #4, rule 3).
 *
 * ⚠ THE LABEL TABLES ARE HAND COPIES of `service-documents/internal/domain/citizen_letter.go` — the
 * contract carries `status` and `letter_type` as plain strings (ledger `apidoc-sinh-enum-cho-truong`).
 * An unknown code therefore falls to a sentence that SAYS it has no label, never to a guess.
 */

import { nhanThoiDiem } from "@/features/phan-anh/nhan-phieu";
import type {
  documents_bookLetterIn,
  documents_citizenLetterItemOut,
  documents_citizenLetterOut,
  documents_duplicateCheckIn,
  documents_letterResultIn,
  documents_letterRoutingIn,
  documents_senderCorrectionIn,
} from "@/lib/api/schema.gen";
import { PETITION_CREATE_PERMISSION, PETITION_READ_PERMISSION, QUYEN_TAO_NHIEM_VU } from "@/lib/quyen";

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

/* ---- statuses (C3) — the TT 05/2021 codes the DATA keeps ----------------------------------- */

/** Labels of C3's ten codes (glossary / ledger C3) — the step select and the log's pills. */
const STATUS_LABEL: Readonly<Record<string, string>> = {
  "moi-vao-so": "Mới vào sổ",
  "dang-xu-ly-don": "Đang xử lý đơn",
  "thu-ly": "Thụ lý",
  "dang-giai-quyet": "Đang giải quyết",
  "da-giai-quyet": "Đã giải quyết",
  "khong-thu-ly": "Không thụ lý",
  "huong-dan": "Hướng dẫn",
  "chuyen-don": "Chuyển đơn",
  "luu-don": "Lưu đơn",
  "dinh-chi": "Đình chỉ",
};

export function letterStatusLabel(code: string): string {
  const label = STATUS_LABEL[code];
  if (label !== undefined) return label;
  return code === "" ? "Không ghi trạng thái" : `${code} (mã trạng thái chưa có nhãn trên màn hình này)`;
}

/* ---- display groups (ADR 0084 #2) — what the screen DRAWS ---------------------------------- */

/**
 * The prototype's seven statuses less "Chờ phân công" (ADR 0084 #5): label, chip and "đang ở đây"
 * colours (`document-display.ts:52-84, 166-175`) and the hint under the strip (`:156-164`), word for word.
 *
 * `steps` is the half of ADR 0084 §2's table this screen needs: the TT 05 codes a STATUS MOVE can land
 * on inside the group. The server's `letterGroupRules` (`service-documents/internal/domain/
 * citizen_letter.go`) stays the one copy that DERIVES a letter's group (`status_group`); this one only
 * sorts the server's `next_statuses` arrows onto chips, because the contract carries those as TT 05
 * codes. The first two groups have none: a letter enters them by booking and by routing, never by a
 * status move. An arrow this table does not know lands on no chip — it is never guessed onto one.
 */
type GroupLook = {
  readonly code: string;
  readonly label: string;
  /** Pill: `bg-{c}/12 text-{c} border-{c}/25`. */
  readonly chip: string;
  /** The strip's "đang ở đây" chip. */
  readonly active: string;
  readonly hint: string;
  readonly steps: readonly string[];
};

const GROUPS: readonly GroupLook[] = [
  {
    code: "moi-vao-so",
    label: "Mới vào sổ",
    chip: "border-line bg-ink-muted/12 text-ink",
    active: "bg-ink-muted text-white",
    hint: "Đã vào sổ, chưa giao cho bộ phận nào.",
    steps: [],
  },
  {
    code: "da-phan-cong",
    label: "Đã phân công",
    chip: "border-brand/25 bg-brand/12 text-brand",
    active: "bg-brand text-white",
    hint: "Đã giao cho bộ phận, chờ bộ phận bắt tay vào việc.",
    steps: [],
  },
  {
    code: "dang-xu-ly",
    label: "Đang xử lý",
    chip: "border-teal/25 bg-teal/12 text-teal",
    active: "bg-teal text-white",
    hint: "Bộ phận đang xử lý, trong hạn giải quyết.",
    steps: ["dang-xu-ly-don", "thu-ly", "dang-giai-quyet"],
  },
  {
    code: "da-giai-quyet",
    label: "Đã giải quyết",
    chip: "border-leaf/25 bg-leaf/12 text-leaf",
    active: "bg-leaf text-white",
    hint: "Đã giải quyết xong và trả lời công dân.",
    steps: ["da-giai-quyet"],
  },
  {
    code: "chuyen-cap-tren",
    label: "Chuyển cấp trên",
    chip: "border-violet/25 bg-violet/12 text-violet",
    active: "bg-violet text-white",
    hint: "Vượt thẩm quyền xã, đã chuyển lên cấp trên.",
    steps: ["chuyen-don"],
  },
  {
    code: "luu-khong-thu-ly",
    label: "Lưu, không thụ lý",
    chip: "border-line bg-ink/12 text-ink",
    active: "bg-ink text-white",
    hint: "Không thuộc thẩm quyền hoặc không đủ điều kiện thụ lý, đã lưu.",
    steps: ["khong-thu-ly", "huong-dan", "luu-don", "dinh-chi"],
  },
];

/** The six groups in the filter's order — the server's `LetterStatusGroups()` order, the prototype's. */
export const LETTER_STATUS_GROUPS: readonly { code: string; label: string }[] = GROUPS.map(({ code, label }) => ({ code, label }));

/** The strip's main row and its "Rẽ nhánh:" row (`document-display.ts:135-146`). */
export const LETTER_GROUP_MAIN_FLOW: readonly string[] = ["moi-vao-so", "da-phan-cong", "dang-xu-ly", "da-giai-quyet"];
export const LETTER_GROUP_BRANCHES: readonly string[] = ["chuyen-cap-tren", "luu-khong-thu-ly"];

function group(code: string): GroupLook | undefined {
  return GROUPS.find((g) => g.code === code);
}

export function letterGroupLabel(code: string): string {
  const found = group(code);
  if (found !== undefined) return found.label;
  return code === "" ? "Không ghi trạng thái" : `${code} (mã nhóm trạng thái chưa có nhãn trên màn hình này)`;
}

/** The pill's colour classes; an unknown code gets the neutral grey. */
export function letterGroupChip(code: string): string {
  return group(code)?.chip ?? "border-line bg-ink-muted/12 text-ink";
}

export function letterGroupActiveTone(code: string): string {
  return group(code)?.active ?? "bg-ink-muted text-white";
}

export function letterGroupHint(code: string): string {
  return group(code)?.hint ?? "";
}

/** The prototype's second chip when no unit holds the letter (`PetitionDetailDrawer.tsx:398-407`). */
export const ASSIGN_CHIP_LABEL = "Phân công";
export const ASSIGN_CHIP_SUB = "chọn bộ phận xử lý";

/** One chip of the status strip. */
export type StatusChipModel = {
  readonly group: string;
  readonly label: string;
  /** "đang ở đây" · "chuyển sang" · "—" · or the assign chip's "chọn bộ phận xử lý". */
  readonly sub: string;
  readonly current: boolean;
  readonly clickable: boolean;
  /** `move`: the composer posts one of `steps`. `assign`: the routing box — no status moves (C3). */
  readonly action: "move" | "assign" | "none";
  /** The server's allowed TT 05 targets that fall in this group, in the group's order. */
  readonly steps: readonly string[];
  /** The composer asks which TT 05 step: the group holds several (ADR 0084 §2). */
  readonly pickStep: boolean;
};

/**
 * The strip (ADR 0084 #2): the prototype's four steps and two branches, BOTH ROWS ALWAYS DRAWN
 * (`PetitionDetailDrawer.tsx:392-428`). A chip is reachable only when one of the SERVER's
 * `next_statuses` falls in its group and the viewer may work on the letter — the server decides the
 * arrows; this never draws one of its own.
 *
 * The CURRENT chip may itself be reachable: "Đang xử lý" holds three TT 05 steps, and Đang xử lý đơn →
 * Thụ lý → Đang giải quyết are moves inside it. Left unpressable, a letter could never be admitted.
 *
 * No unit holds the letter → the second chip is the prototype's "Phân công · chọn bộ phận xử lý": an
 * ASSIGNMENT (the routing box), pressable by whoever may route (`petition.create`) while the letter is open.
 */
export function statusStrip(input: {
  group: string;
  nextStatuses: readonly string[];
  /** A unit holds the letter (`holding_unit_id`). */
  held: boolean;
  closed: boolean;
  mayWork: boolean;
  /** May route (`petition.create`) — the "Phân công" chip. */
  canRoute: boolean;
}): { main: StatusChipModel[]; branches: StatusChipModel[] } {
  const chip = (code: string): StatusChipModel => {
    const look = group(code);
    if (code === "da-phan-cong" && !input.held) {
      const clickable = input.mayWork && input.canRoute && !input.closed;
      return { group: code, label: ASSIGN_CHIP_LABEL, sub: ASSIGN_CHIP_SUB, current: false, clickable, action: clickable ? "assign" : "none", steps: [], pickStep: false };
    }
    const steps = (look?.steps ?? []).filter((s) => input.nextStatuses.includes(s));
    const current = code === input.group;
    const clickable = input.mayWork && steps.length > 0;
    return {
      group: code,
      label: letterGroupLabel(code),
      sub: current ? "đang ở đây" : clickable ? "chuyển sang" : "—",
      current,
      clickable,
      action: clickable ? "move" : "none",
      steps,
      pickStep: (look?.steps.length ?? 0) > 1,
    };
  };
  return { main: LETTER_GROUP_MAIN_FLOW.map(chip), branches: LETTER_GROUP_BRANCHES.map(chip) };
}

/* ---- source (ADR 0084 #7) — the prototype's `SOURCE_META` (`document-display.ts:194-206`) ---- */

const SOURCES: Readonly<Record<string, { label: string; chip: string }>> = {
  "nhap-tay": { label: "Nhập tay", chip: "border-brand/25 bg-brand/12 text-brand" },
  "nhap-excel": { label: "Nhập từ Excel", chip: "border-line bg-ink-muted/12 text-ink" },
  "mini-app": { label: "Mini App", chip: "border-leaf/25 bg-leaf/12 text-leaf" },
  "thu-dien-tu": { label: "Thư điện tử", chip: "border-teal/25 bg-teal/12 text-teal" },
};

export function letterSourceLabel(code: string): string {
  const found = SOURCES[code];
  if (found !== undefined) return found.label;
  return code === "" ? "Không ghi nguồn" : `${code} (mã nguồn chưa có nhãn trên màn hình này)`;
}

export function letterSourceChip(code: string): string {
  return SOURCES[code]?.chip ?? "border-line bg-ink-muted/12 text-ink";
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

/**
 * "Chuyển thành nhiệm vụ": `task.create` AND `petition.read` — the two keys of POST
 * /api/v1/citizen-letter-tasks. Convenience only: the server checks both.
 */
export function canRaiseLetterTask(permissions: readonly string[]): boolean {
  return permissions.includes(QUYEN_TAO_NHIEM_VU) && permissions.includes(PETITION_READ_PERMISSION);
}

/**
 * Whether the drawer draws the button: the keys, and NEVER for a denunciation — a task is read by every
 * officer with `task.read`, and the denouncer's protection (Luật Tố cáo 2018 Đ.8; C9, ADR 0084 #6) does not
 * survive that. The server's 422 `denunciation_no_task` is the backstop, not the gate.
 */
export function showsRaiseTask(letter: Pick<documents_citizenLetterOut, "letter_type">, canRaise: boolean): boolean {
  return canRaise && letter.letter_type !== DENUNCIATION_TYPE;
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
 * The "Hạn giải quyết" words: "Không đặt hạn", "Hạn {d/m/yyyy}", or — open AND past the stored
 * instant — "Quá hạn · {d/m/yyyy}". A COMPARISON of two instants, never a deadline computed here
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

/** The drawer figure's word for no deadline — the prototype's (`PetitionDetailDrawer.tsx:522`). */
export const DUE_NOT_SET = "Không đặt";

/**
 * The drawer's ONE "Hạn xử lý" figure (prototype `PetitionDetailDrawer.tsx:519-524`, its single
 * `sla.due_at`): the resolution deadline once the letter was admitted (`accepted_at`), the processing one
 * before — whichever phase the letter is in or ended in. Both are STORED by the server; this only picks.
 */
export function drawerDueAt(
  letter: Pick<documents_citizenLetterOut, "accepted_at" | "processing_due_at" | "resolution_due_at">,
): string | null {
  return (letter.accepted_at ?? "") !== "" ? letter.resolution_due_at : letter.processing_due_at;
}

/**
 * The statuses in which `accepted_at` is set — `service-documents/migrations/0006_citizen_letter.sql`,
 * CHECK `citizen_letter_accepted_at_matches_status`. The list row carries no `accepted_at`, so the
 * database's own binding stands in for it.
 */
const ADMITTED_STATUSES: readonly string[] = ["thu-ly", "dang-giai-quyet", "da-giai-quyet", "dinh-chi"];

/**
 * `drawerDueAt` for a LIST ROW: the same instant, with admission read from the status instead of
 * `accepted_at` (see `ADMITTED_STATUSES`). The list's "Hạn giải quyết", the drawer's chip row and its
 * "Hạn xử lý" figure all read through this rule, so one letter never shows two deadlines. It is also the
 * instant a task raised from the letter inherits (`CurrentStageDueAt`, documents).
 */
export function stageDueAt(letter: DueFields): string | null {
  return ADMITTED_STATUSES.includes(letter.status) ? letter.resolution_due_at : letter.processing_due_at;
}

/** The drawer's "Hạn xử lý" figure: the stored date, or "Không đặt" — never a count. */
export function dueDateText(dueISO: string | null): string {
  if (dueISO === null || dueISO === "") return DUE_NOT_SET;
  return Number.isNaN(new Date(dueISO).getTime()) ? dueISO : deadlineDate(dueISO);
}

/**
 * Whether the letter's phase HAS a deadline a clerk can set — mirror of `domain.LetterDueColumn`: the
 * four open phases; a finished letter has none (the server answers 409). UX only: the server decides.
 */
export function dueSettable(status: string): boolean {
  return ["moi-vao-so", "dang-xu-ly-don", "thu-ly", "dang-giai-quyet"].includes(status);
}

/** A stored deadline → the `YYYY-MM-DD` of a date input, read in the Vietnamese zone. `""` when none. */
export function dueInputValue(dueISO: string | null): string {
  if (dueISO === null || dueISO === "" || Number.isNaN(new Date(dueISO).getTime())) return "";
  const [dd, mm, yyyy] = deadlineDayPadded(dueISO).split("/");
  return `${yyyy}-${mm}-${dd}`;
}

/**
 * The date input → the instant PATCH …/deadline stores: 17:00 of that day. The prototype's own
 * deadline field sends `new Date(\`${dueOn}T17:00:00\`).toISOString()` (`DocumentEntryForm.tsx:118`) —
 * 17:00 in the BROWSER's zone. Pinned to `+07:00` here instead, so a machine set to the wrong zone
 * cannot shift a commitment made to a citizen; in Vietnam the two are the same instant. `null` = empty.
 */
export function dueInstantOf(date: string): string | null {
  return /^\d{4}-\d{2}-\d{2}$/.test(date) ? `${date}T17:00:00+07:00` : null;
}

/**
 * Whether an OPEN letter is past its stored deadline — the one `stageDueAt` shows, so a red row and the
 * "Quá hạn" words in it can never disagree. Derived at render time.
 */
export function isPastDue(letter: DueFields & { is_closed: boolean }, now: Date): boolean {
  if (letter.is_closed) return false;
  const due = stageDueAt(letter);
  if (due === null || due === "") return false;
  const at = new Date(due).getTime();
  return !Number.isNaN(at) && at < now.getTime();
}

/** `dd/MM/yyyy` of a deadline instant, pinned to the Vietnamese zone through `nhanThoiDiem`. */
function deadlineDayPadded(dueISO: string): string {
  const full = nhanThoiDiem(dueISO);
  return full.slice(full.lastIndexOf(" ") + 1);
}

/**
 * `d/m/yyyy` of a deadline instant — the prototype's `formatDay` (`toLocaleDateString("vi-VN")`, no zero
 * padding), but still read in the Vietnamese zone: the browser's own zone could move a deadline a day.
 */
function deadlineDate(dueISO: string): string {
  const [dd, mm, yyyy] = deadlineDayPadded(dueISO).split("/");
  return `${Number(dd)}/${Number(mm)}/${yyyy}`;
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

/**
 * `d/m/yyyy` of a `YYYY-MM-DD` — the prototype's `formatDay` (`23/8/2026`, no zero padding). Cut as text,
 * never through `Date` (a date has no zone). Anything not in the contract's shape is shown verbatim.
 */
export function letterDate(iso: string): string {
  if (iso === "") return "—";
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  return m === null ? iso : `${Number(m[3])}/${Number(m[2])}/${m[1]}`;
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

/** The reply's draft. Sent as typed; the server says what is missing. */
export type ResultDraft = { documentNo: string; documentDate: string; signer: string; issuer: string; summary: string };

/**
 * The two types whose result is an ISSUED DOCUMENT — number, date, signer, issuing body — on top of the
 * reply (ADR 0084 #2: C10 narrowed to complaints and denunciations). The server requires them; a
 * feedback letter or a request is answered by the one textarea alone.
 */
const RESULT_DOCUMENT_TYPES: readonly string[] = ["khieu-nai", "to-cao"];

export function resultNeedsDocument(letterType: string): boolean {
  return RESULT_DOCUMENT_TYPES.includes(letterType);
}

/** PUT …/result's body: the reply, plus the document fields only for the two types that have them. */
export function buildResult(draft: ResultDraft, letterType: string): documents_letterResultIn {
  const body: documents_letterResultIn = { result_summary: draft.summary.trim() };
  if (!resultNeedsDocument(letterType)) return body;
  return {
    result_document_no: draft.documentNo.trim(),
    result_document_date: draft.documentDate,
    result_signer: draft.signer.trim(),
    result_issuer: draft.issuer.trim(),
    ...body,
  };
}

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
