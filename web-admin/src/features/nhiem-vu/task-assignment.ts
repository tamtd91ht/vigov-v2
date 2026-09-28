/**
 * §5.7 "Giao việc, chuyển việc" — the pure half of the drawer block (`task-assignment-block.tsx`).
 *
 * ONE ACT, ONE CALL (owner decision 28/09/2026 + user decisions of this session): "Chuyển tiếp" is
 * the SAME task handed to another unit or person, and the two `theo-van-ban` fields of §5.4 — lead
 * unit and monitor — are edited in the SAME `POST /api/v1/tasks/{ma}/assignment`, behind
 * `task.assign`. What the server does with it (`service-petitions/internal/domain/task_assignment.go`):
 *
 *   unit or assignee changed      status back to `moi-giao` — the new holder acknowledges again
 *   only lead unit / monitor      status unchanged
 *   the deadline                  NEVER changes (ADR 0038: only an approved extension moves it)
 *   terminal task                 409 `task_state` — hence no block on a terminal task
 *
 * No rule above is RE-DECIDED here. This file only decides what to SEND and what to SAY; the server
 * decides everything again, and its refusal is shown verbatim.
 */

import type {
  identity_boPhanRa,
  petitions_nhiemVuRa,
  petitions_taskAssignmentIn,
} from "@/lib/api/schema.gen";

import { coKhoiVanBanChiDao, ketThuc, nhanTrangThai } from "./nhan-nhiem-vu";
import type { BangNhanTrangThai } from "./nhan-nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import type { QuyenNhiemVu } from "./nhan-nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)

/** What the block's four fields and the note hold. Codes and ids, exactly as the task carries them. */
export type AssignmentForm = {
  /** `bo_phan` id; `""` = `— Chưa xác định —`. */
  readonly unit: string;
  /** Staff BUSINESS code `CB-…`; `""` = `— Để bộ phận tự phân công —`. */
  readonly assignee: string;
  /** `bo_phan` id; `theo-van-ban` only. */
  readonly leadUnit: string;
  /** Staff business code; `theo-van-ban` only. */
  readonly monitor: string;
  readonly note: string;
};

/** The form as it opens: the task as read, nothing changed, no note. */
export function assignmentFormFromTask(task: petitions_nhiemVuRa): AssignmentForm {
  return {
    unit: task.unit,
    assignee: task.assignee,
    leadUnit: task.lead_unit,
    monitor: task.monitor,
    note: "",
  };
}

/**
 * Picking `— Chưa xác định —` for a task that HAS a unit. The server refuses it (400, "giao lại
 * phải nêu bộ phận nhận việc" — handing work to nobody is not a hand-over), so the button is off and
 * a sentence says why before the server has to.
 */
export function unitWouldBeCleared(form: AssignmentForm, task: petitions_nhiemVuRa): boolean {
  return task.unit !== "" && form.unit.trim() === "";
}

/**
 * The body of `POST …/assignment`, or `null` (button disabled).
 *
 * ONLY THE FIELDS THAT DIFFER from the task as read. Not a convenience: every staff code SENT is
 * checked with identity (`TaskAssignmentChange.StaffCodes`), so resending an unchanged monitor who
 * has since left the commune would refuse an unrelated change of unit — with a sentence about a
 * person the officer did not touch.
 *
 *   unit        sent when it differs; never sent as "" (`null` instead — see `unitWouldBeCleared`)
 *   assignee    sent when it differs, "" included: `— Để bộ phận tự phân công —` is a real choice
 *   lead unit   `theo-van-ban` only; "" clears it
 *   monitor     `theo-van-ban` only; "" clears it
 *   note        trimmed, only with at least one of the four — a note alone changes nothing (400)
 *
 * On another type the two §5.4 fields are never sent, whatever the form holds: §7.3 drops them for
 * `co-ban`, and a stale value there must not ride along.
 */
export function assignmentBody(
  form: AssignmentForm,
  task: petitions_nhiemVuRa,
): petitions_taskAssignmentIn | null {
  if (unitWouldBeCleared(form, task)) return null;

  const body: petitions_taskAssignmentIn = {};
  const unit = form.unit.trim();
  if (unit !== task.unit) body.unit = unit;
  const assignee = form.assignee.trim();
  if (assignee !== task.assignee) body.assignee = assignee;
  if (coKhoiVanBanChiDao(task.type)) {
    const leadUnit = form.leadUnit.trim();
    if (leadUnit !== task.lead_unit) body.lead_unit = leadUnit;
    const monitor = form.monitor.trim();
    if (monitor !== task.monitor) body.monitor = monitor;
  }
  if (Object.keys(body).length === 0) return null;

  const note = form.note.trim();
  if (note !== "") body.note = note;
  return body;
}

/**
 * Whether the block is drawn at all: `task.assign` of the SESSION and a task that is not finished.
 *
 * CONVENIENCE, NOT PROTECTION (rule 5, forbidden #1): the route checks `task.assign` and refuses a
 * finished task with 409 whatever this says (`CheckAssignable`, `task_assignment.go:121-126`).
 *
 * ĐỔI CHIỀU CÓ CHỦ Ý 28/09/2026: an old `chuyen-tiep` row used to be excluded too, as "terminal". The
 * server no longer treats it so — it moves on to `da-tiep-nhan` / `dang-thuc-hien`, and
 * `CheckAssignable` refuses only `hoan-thanh` — so hiding the block there hid an act the server allows.
 */
export function canShowAssignment(permissions: QuyenNhiemVu, status: string): boolean {
  return permissions.reassign && !ketThuc(status);
}

/** Server bound: the note IS a timeline line (`domain.NoiDungNhatKyToiDa`, `nhiem_vu_ghi.go:43`). */
export const ASSIGNMENT_NOTE_MAX = 5000;

/** Anchor of the block — the stepper's `Chuyển tiếp` branch leads here (user decision 2). */
export const ASSIGNMENT_BLOCK_ID = "giao-viec-chuyen-viec";
/** First field of the block: where focus lands from the stepper. */
export const ASSIGNMENT_UNIT_FIELD_ID = "giao-lai-bo-phan";

export const ASSIGNMENT_TITLE = "Giao việc, chuyển việc";
export const ASSIGNMENT_UNIT_LABEL = "Bộ phận";
export const ASSIGNMENT_ASSIGNEE_LABEL = "Người thực hiện";
export const ASSIGNMENT_LEAD_UNIT_LABEL = "Cơ quan chủ trì tham mưu";
export const ASSIGNMENT_MONITOR_LABEL = "Chuyên viên Văn phòng tham mưu / theo dõi";
/** §5.7, verbatim. */
export const ASSIGNMENT_NOTE_LABEL = "Lý do chuyển (bỏ trống nếu chỉ giao lần đầu)";
export const ASSIGNMENT_BUTTON = "Giao việc";
export const ASSIGNMENT_SENDING = "Đang giao…";

export const ASSIGNMENT_UNIT_REQUIRED =
  "Nhiệm vụ đã có bộ phận thực hiện thì giao lại phải chọn một bộ phận nhận việc, không để trống.";
export const ASSIGNMENT_NO_CHANGE =
  "Chưa có gì thay đổi so với phân công hiện tại, nên chưa có gì để giao.";

/** The stepper's `Chuyển tiếp` branch, as a control: its accessible name says where it leads. */
export const ASSIGNMENT_STEPPER_HINT = "đến khối Giao việc, chuyển việc";

/**
 * The staff directory could not be read. SAYS THE CONSEQUENCE: the two staff boxes keep only the
 * person already recorded (and "nobody"), while handing the task to a unit still works — the same
 * direction the create form takes when `identity` is down. The directory is read once per page,
 * so the way to retry is reloading the page.
 */
export function assignmentDirectoryError(message: string): string {
  return (
    `Không tải được danh bạ cán bộ: ${message} Ô người thực hiện và chuyên viên theo dõi chỉ còn ` +
    "người đang được ghi và lựa chọn trống; vẫn giao được cho bộ phận. Tải lại trang để thử đọc " +
    "lại danh bạ."
  );
}

/** Said after a success — the drawer re-reads, and this says what happened. */
export function assignmentDoneText(code: string): string {
  return `Đã giao lại ${code}.`;
}

/**
 * The sentence ALWAYS visible above the button: what the act does to the status and the deadline,
 * BEFORE the officer presses it. The status name comes from the COMMUNE'S label table (decision
 * #21) — a commune that renamed `moi-giao` must read its own word here, not "Mới giao".
 */
export function assignmentEffectNote(labels: BangNhanTrangThai, withLeadFields: boolean): string {
  const reset =
    "Đổi bộ phận hoặc người thực hiện thì nhiệm vụ trở về trạng thái " +
    `“${nhanTrangThai(labels, "moi-giao")}” để bên nhận tiếp nhận lại. Hạn xử lý giữ nguyên — ` +
    "hạn chỉ dịch qua đề nghị lùi hạn có người duyệt.";
  return withLeadFields
    ? `${reset} Chỉ đổi cơ quan chủ trì hoặc chuyên viên theo dõi thì trạng thái không đổi.`
    : reset;
}

/**
 * The unit options: the commune's units, PLUS the value the task currently holds when it is not
 * among them (a unit since removed from the catalogue), shown by its id. Without that line the
 * `<select>` would show `— Chưa xác định —` for a task that has a unit, and the officer would
 * believe it has none.
 */
export function unitOptions(
  units: readonly identity_boPhanRa[],
  current: string,
): readonly { readonly id: string; readonly name: string }[] {
  const options = units.map((u) => ({ id: u.id, name: u.name }));
  return current === "" || options.some((o) => o.id === current)
    ? options
    : [...options, { id: current, name: current }];
}
