/**
 * Words and decisions of the block "Thời hạn giải quyết đơn thư" (tab Thời hạn xử lý; ADR 0084 #3,
 * ADR 0085 B and câu 2–4). PURE: no network, no DOM.
 *
 * NO DEADLINE IS COMPUTED HERE. The screen shows and edits a commune's figure; `identity` counts the days
 * (ADR 0085 câu 5) and fixes each letter's deadline once, at the act that fixes it (rule 10, invariant 2).
 *
 * ⚠ `SLOTS` IS A HAND COPY of the unit lock in `service-identity/internal/domain/citizen_letter_deadline.go`
 * (`RequiredCitizenLetterUnit`). It exists for ONE reason: the GET returns only rules that exist, so for a
 * pair with no rule yet nothing tells the screen which rows to draw or which unit to send on create. For
 * every pair that HAS a rule the server's `required_unit` wins over this table. A drift here is caught by
 * the server — the POST answers 400 with the lock's own sentence — never silently stored.
 */

import type { identity_citizenLetterDeadlineRuleOut } from "@/lib/api/schema.gen";

/** One possible (letter type, deadline kind) and the one unit the lock allows for it. */
export type DeadlineSlot = {
  readonly letterType: string;
  readonly deadlineKind: string;
  readonly unit: string;
};

/**
 * The six pairs, in the order the owner listed the types (TASK-10): kiến nghị-phản ánh and đề nghị have
 * only "Hạn xử lý đơn" (ADR 0085 câu 3); khiếu nại and tố cáo have both deadlines (ADR 0064).
 */
export const SLOTS: readonly DeadlineSlot[] = [
  {
    letterType: "kien-nghi-phan-anh",
    deadlineKind: "xu-ly-don",
    unit: "ngay-lam-viec",
  },
  { letterType: "khieu-nai", deadlineKind: "xu-ly-don", unit: "ngay-lich" },
  { letterType: "khieu-nai", deadlineKind: "giai-quyet", unit: "ngay-lich" },
  { letterType: "to-cao", deadlineKind: "xu-ly-don", unit: "ngay-lam-viec" },
  { letterType: "to-cao", deadlineKind: "giai-quyet", unit: "ngay-lich" },
  { letterType: "de-nghi", deadlineKind: "xu-ly-don", unit: "ngay-lam-viec" },
];

export const BLOCK_TITLE = "Thời hạn giải quyết đơn thư";
export const BLOCK_HELP =
  "Đơn vào sổ tự lấy hạn theo loại đơn; để trống thì đơn không đặt hạn.";
export const NOT_SET = "Không đặt hạn";
export const COLUMN_LETTER_TYPE = "Loại đơn";
export const COLUMN_DEADLINE_KIND = "Loại hạn";
export const COLUMN_DEADLINE = "Thời hạn";
export const EDIT_DEADLINE_TITLE = "Sửa thời hạn";
export const REMOVE_DEADLINE_TITLE = "Xoá thời hạn";
export const AMOUNT_LABEL = "Số ngày";
export const LOADING_LABEL = "Đang tải thời hạn giải quyết đơn thư…";
export const LOAD_ERROR_TITLE = "Chưa tải được thời hạn đơn thư";
export const REMOVED_DEADLINE =
  "Đã xoá thời hạn. Đơn loại này vào sổ từ nay không đặt hạn.";

/** Local refusal of the amount box — the server refuses it too ("số ngày phải lớn hơn 0"). */
export const AMOUNT_NOT_POSITIVE = "Số ngày phải là một số nguyên lớn hơn 0.";
/** The open edit changed nothing: nothing to send (the server refuses `{}` as well). */
export const NOTHING_CHANGED = "Chưa có con số nào được sửa.";

const KIND_LABELS: Readonly<Record<string, string>> = {
  "xu-ly-don": "Hạn xử lý đơn",
  "giai-quyet": "Hạn giải quyết (từ ngày thụ lý)",
};

const UNIT_LABELS: Readonly<Record<string, string>> = {
  "ngay-lam-viec": "ngày làm việc",
  "ngay-lich": "ngày",
};

/** An unknown code is shown AS a code, never guessed into a label. */
export function deadlineKindLabel(code: string): string {
  return KIND_LABELS[code] ?? code;
}

export function unitLabel(code: string): string {
  return UNIT_LABELS[code] ?? code;
}

/** One row of the block: a pair, its live rule if any, and the unit to show and send. */
export type DeadlineRow = {
  /** Stable React key and the edit's handle: the rule id when one exists, else the pair. */
  readonly key: string;
  readonly letterType: string;
  readonly deadlineKind: string;
  /** The server's `required_unit` when a rule exists and names one; else the slot's. */
  readonly unit: string;
  readonly rule: identity_citizenLetterDeadlineRuleOut | null;
};

function slotKey(letterType: string, deadlineKind: string): string {
  return `${letterType}/${deadlineKind}`;
}

/**
 * The six slots, each with its rule; then any rule matching NO slot (a pair the lock does not know —
 * restored from elsewhere, or a cell a legal review moved). Such a row is still drawn, with the server's
 * `problem`, so it can be cleared: dropping it would hide a rule that blocks booking.
 */
export function deadlineRows(
  rules: readonly identity_citizenLetterDeadlineRuleOut[],
): DeadlineRow[] {
  const used = new Set<string>();
  const rows: DeadlineRow[] = SLOTS.map((slot) => {
    const rule =
      rules.find(
        (r) =>
          r.letter_type === slot.letterType &&
          r.deadline_kind === slot.deadlineKind,
      ) ?? null;
    if (rule !== null) used.add(rule.id);
    return {
      key: rule?.id ?? slotKey(slot.letterType, slot.deadlineKind),
      letterType: slot.letterType,
      deadlineKind: slot.deadlineKind,
      unit:
        rule !== null && rule.required_unit !== ""
          ? rule.required_unit
          : slot.unit,
      rule,
    };
  });
  for (const rule of rules) {
    if (used.has(rule.id)) continue;
    rows.push({
      key: rule.id,
      letterType: rule.letter_type,
      deadlineKind: rule.deadline_kind,
      unit: rule.required_unit !== "" ? rule.required_unit : rule.unit,
      rule,
    });
  }
  return rows;
}

/** What a save sends: a POST for a pair without a rule, a PATCH for one with. */
export type ComposedSave =
  | {
      readonly ok: true;
      readonly action: "create";
      readonly body: {
        letter_type: string;
        deadline_kind: string;
        amount: number;
        unit: string;
      };
    }
  | {
      readonly ok: true;
      readonly action: "update";
      readonly id: string;
      readonly body: { amount?: number; unit?: string };
    }
  | { readonly ok: false; readonly error: string };

/**
 * The amount box → a request. A positive integer only; the 1..365 ceiling is the server's, and its
 * sentence comes back verbatim. On an existing rule whose stored unit the lock refuses (`problem`), the
 * required unit travels too: saving the row is how an administrator repairs it.
 */
export function composeSave(row: DeadlineRow, text: string): ComposedSave {
  const trimmed = text.trim();
  const value = trimmed === "" ? Number.NaN : Number(trimmed);
  if (!Number.isInteger(value) || value <= 0)
    return { ok: false, error: AMOUNT_NOT_POSITIVE };

  if (row.rule === null) {
    return {
      ok: true,
      action: "create",
      body: {
        letter_type: row.letterType,
        deadline_kind: row.deadlineKind,
        amount: value,
        unit: row.unit,
      },
    };
  }
  const body: { amount?: number; unit?: string } = {};
  if (value !== row.rule.amount) body.amount = value;
  if (row.rule.unit !== row.unit) body.unit = row.unit;
  if (Object.keys(body).length === 0)
    return { ok: false, error: NOTHING_CHANGED };
  return { ok: true, action: "update", id: row.rule.id, body };
}
