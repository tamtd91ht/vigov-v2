"use client";

import { Loader2, UserPlus } from "lucide-react";
import { useState, type FormEvent } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { luaChonCanBo } from "@/features/bien-ban/nhan-bien-ban";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_boPhanRa,
  identity_danhBaChonNguoiRa,
  petitions_nhiemVuRa,
  petitions_taskAssignmentIn,
} from "@/lib/api/schema.gen";

import {
  CHUA_XAC_DINH,
  DE_BO_PHAN_TU_PHAN_CONG,
  docDanhBaChonNguoi,
  nhanTrongOChonCanBo,
  taskStaffOptionLabel,
} from "./nhan-nhiem-vu";
import type { BangNhanTrangThai } from "./nhan-nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import {
  ASSIGNMENT_ASSIGNEE_LABEL,
  ASSIGNMENT_BLOCK_ID,
  ASSIGNMENT_NOTE_LABEL,
  ASSIGNMENT_NOTE_PLACEHOLDER,
  ASSIGNMENT_NOTE_MAX,
  ASSIGNMENT_UNIT_FIELD_ID,
  ASSIGNMENT_UNIT_LABEL,
  ASSIGNMENT_UNIT_REQUIRED,
  assignmentBody,
  assignmentDirectoryError,
  assignmentFormFromTask,
  assignmentOutcomeText,
  assignmentSubmitLabel,
  ASSIGNMENT_NOTHING_CHOSEN,
  UNIT_HAS_NO_STAFF,
  staffInUnit,
  unitOptions,
  unitWouldBeCleared,
  type AssignmentForm,
} from "./task-assignment";
import { INPUT_CLASS } from "./task-spec";

/** The prototype's `HandoverFields` select (`HandoverFields.tsx:6-7`). */
const HANDOVER_SELECT_CLASS =
  "border-line focus-visible:ring-ring/50 mt-1 h-9 w-full rounded-md border bg-white px-2.5 text-[12.5px] text-ink [font-family:inherit] outline-none focus-visible:ring-[3px]";
const HANDOVER_LABEL_CLASS = "text-ink block text-[11.5px] font-medium";

/**
 * §5.7 "Giao việc, chuyển việc" in the drawer — spec 07 §6e: a page-grey box holding the two
 * `HandoverFields` selects (unit, then the person — filtered by that unit), the reason, and
 * `[UserPlus] Giao việc` (`Chuyển việc` once a reason is typed). `POST /api/v1/tasks/{ma}/assignment`,
 * `task.assign` — ONE route for both words (ADR 0076 #4c keeps the 28/09 rule on this route: the task
 * goes back to "Chưa thực hiện" for the new holder, which the hint line under the button says).
 *
 * THE CALLER DECIDES WHETHER IT IS DRAWN (`canShowAssignment`) and draws its heading (the drawer's
 * `Section`, id `tieu-de-giao-viec-chuyen-viec`). No second gate here.
 *
 * NO OPTIMISTIC UPDATE: on success the drawer re-reads the task and the timeline, the form restarts
 * from the task the server returned, and a toast says what happened ("Đã giao việc." / "Đã chuyển
 * việc và ghi vết."). A refusal keeps the choice; its sentence — the server's, verbatim — is the
 * error toast. Pressing with nothing to hand over says so by toast (spec 07 §6e).
 */
export function TaskAssignmentBlock({
  task,
  units,
  directory,
  labels,
  reassign,
}: {
  task: petitions_nhiemVuRa;
  /** The commune's units — the same catalogue the `Giao việc mới` form lists. */
  units: readonly identity_boPhanRa[];
  /** The whole-commune staff directory as read (`null` = still loading). */
  directory: KetQua<identity_danhBaChonNguoiRa> | null;
  labels: BangNhanTrangThai;
  reassign: (body: petitions_taskAssignmentIn) => Promise<KetQua<petitions_nhiemVuRa>>;
}) {
  const [form, setForm] = useState<AssignmentForm>(() => assignmentFormFromTask(task));
  const [sending, setSending] = useState(false);

  function submit(): void {
    if (sending) return;
    if (unitWouldBeCleared(form, task)) {
      toast.error(ASSIGNMENT_UNIT_REQUIRED);
      return;
    }
    const body = assignmentBody(form, task);
    if (body === null) {
      toast.error(ASSIGNMENT_NOTHING_CHOSEN);
      return;
    }
    setSending(true);
    reassign(body).then((result) => {
      setSending(false);
      if (!result.ok) {
        toast.error(result.thongBao);
        return;
      }
      toast.success(assignmentOutcomeText(body));
      setForm(assignmentFormFromTask(result.duLieu));
    });
  }

  return (
    <TaskAssignmentForm
      task={task}
      units={units}
      directory={directory}
      labels={labels}
      form={form}
      change={(patch) => setForm((f) => ({ ...f, ...patch }))}
      sending={sending}
      submit={submit}
    />
  );
}

/** Rendering only — exported for `renderToStaticMarkup` (no DOM in vitest). */
export function TaskAssignmentForm({
  task,
  units,
  directory,
  // Kept on the props: callers and tests pass it; the status note it fed is gone (review r2 F-1).
  labels: _labels,
  form,
  change,
  sending,
  submit,
}: {
  task: petitions_nhiemVuRa;
  units: readonly identity_boPhanRa[];
  directory: KetQua<identity_danhBaChonNguoiRa> | null;
  labels: BangNhanTrangThai;
  form: AssignmentForm;
  change: (patch: Partial<AssignmentForm>) => void;
  sending: boolean;
  submit: () => void;
}) {
  const staff = docDanhBaChonNguoi(directory);
  // Unit chosen → only its staff (spec 07 §6e); the person recorded today keeps a line even when
  // outside it, so the select never shows "nobody" for a task that has someone.
  const reachable = staffInUnit(staff.ds, form.unit);
  // `Họ tên — email_masked · Chức danh` (prototype `HandoverFields.tsx:87-92`, ADR 0082 #10).
  const people = luaChonCanBo(reachable, form.assignee, taskStaffOptionLabel);

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    submit();
  }

  return (
    <form
      id={ASSIGNMENT_BLOCK_ID}
      className="border-line bg-canvas m-0 space-y-3 rounded-[10px] border p-3"
      aria-labelledby="tieu-de-giao-viec-chuyen-viec"
      onSubmit={onSubmit}
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <label htmlFor={ASSIGNMENT_UNIT_FIELD_ID} className={HANDOVER_LABEL_CLASS}>
            {ASSIGNMENT_UNIT_LABEL}
          </label>
          <select
            id={ASSIGNMENT_UNIT_FIELD_ID}
            value={form.unit}
            className={HANDOVER_SELECT_CLASS}
            onChange={(e) => {
              const unit = e.target.value;
              // The person chosen is not in the new unit: drop them, rather than keep a name that just
              // vanished from the list below.
              const stays = form.assignee === "" || staffInUnit(staff.ds, unit).some((c) => c.code === form.assignee);
              change(stays ? { unit } : { unit, assignee: "" });
            }}
          >
            <option value="">{CHUA_XAC_DINH}</option>
            {unitOptions(units, task.unit).map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label htmlFor="giao-lai-nguoi-thuc-hien" className={HANDOVER_LABEL_CLASS}>
            {ASSIGNMENT_ASSIGNEE_LABEL}
          </label>
          <select
            id="giao-lai-nguoi-thuc-hien"
            value={form.assignee}
            disabled={staff.dangTai}
            className={HANDOVER_SELECT_CLASS}
            onChange={(e) => change({ assignee: e.target.value })}
          >
            <option value="">{nhanTrongOChonCanBo(staff, DE_BO_PHAN_TU_PHAN_CONG)}</option>
            {people.map((o) => (
              <option key={o.ma} value={o.ma}>
                {o.nhan}
              </option>
            ))}
          </select>
          {form.unit !== "" && !staff.dangTai && staff.loi === null && reachable.length === 0 && (
            <p className="text-tangerine m-0 mt-1 text-[11px]">{UNIT_HAS_NO_STAFF}</p>
          )}
        </div>
      </div>

      {staff.loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {assignmentDirectoryError(staff.loi)}
        </p>
      )}

      <div>
        <label htmlFor="giao-lai-ly-do" className={HANDOVER_LABEL_CLASS}>
          {ASSIGNMENT_NOTE_LABEL}
        </label>
        <input
          id="giao-lai-ly-do"
          name="giao-lai-ly-do"
          value={form.note}
          placeholder={ASSIGNMENT_NOTE_PLACEHOLDER}
          maxLength={ASSIGNMENT_NOTE_MAX}
          autoComplete="off"
          className={`${INPUT_CLASS} mt-1 text-[12.5px] md:text-[12.5px]`}
          onChange={(e) => change({ note: e.target.value })}
        />
      </div>

      <Button
        type="submit"
        variant="primary"
        disabled={sending}
        aria-busy={sending || undefined}
        icon={
          sending ? (
            <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
          ) : (
            <UserPlus aria-hidden="true" focusable="false" className="size-4" />
          )
        }
      >
        {assignmentSubmitLabel(form)}
      </Button>
      {/* No line about what the act does to the status: the prototype's box has none
          (TaskDetailDrawer.tsx:498-536), and the owner's 07/10 rule is "follow the prototype" (review
          r2 F-1). The 28/09 status reset still happens on the server (ADR 0076 #4c). */}
    </form>
  );
}
