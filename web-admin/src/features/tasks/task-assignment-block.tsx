"use client";

import { useState, type FormEvent } from "react";

import { StaffCombobox } from "@/components/staff-combobox";
import type { KetQua } from "@/lib/api/request";
import type {
  identity_boPhanRa,
  identity_danhBaChonNguoiRa,
  petitions_nhiemVuRa,
  petitions_taskAssignmentIn,
} from "@/lib/api/schema.gen";

import {
  CHUA_XAC_DINH,
  DE_BO_PHAN_TU_PHAN_CONG,
  coKhoiVanBanChiDao,
  docDanhBaChonNguoi,
  nhanTrongOChonCanBo,
} from "./task-labels";
import type { BangNhanTrangThai } from "./task-labels"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import {
  ASSIGNMENT_ASSIGNEE_LABEL,
  ASSIGNMENT_BLOCK_ID,
  ASSIGNMENT_BUTTON,
  ASSIGNMENT_LEAD_UNIT_LABEL,
  ASSIGNMENT_MONITOR_LABEL,
  ASSIGNMENT_NO_CHANGE,
  ASSIGNMENT_NOTE_LABEL,
  ASSIGNMENT_NOTE_MAX,
  ASSIGNMENT_SENDING,
  ASSIGNMENT_TITLE,
  ASSIGNMENT_UNIT_FIELD_ID,
  ASSIGNMENT_UNIT_LABEL,
  ASSIGNMENT_UNIT_REQUIRED,
  assignmentBody,
  assignmentDirectoryError,
  assignmentDoneText,
  assignmentEffectNote,
  assignmentFormFromTask,
  unitOptions,
  unitWouldBeCleared,
  type AssignmentForm,
} from "./task-assignment";

/**
 * §5.7 "Giao việc, chuyển việc" in the drawer — `POST /api/v1/tasks/{ma}/assignment`, `task.assign`.
 *
 * THE CALLER DECIDES WHETHER IT IS DRAWN (`canShowAssignment`: the key and a non-terminal task).
 * This component does not re-check: a second gate here is a second copy that drifts.
 *
 * NO OPTIMISTIC UPDATE. The fields keep what the officer chose until the server answers; on success
 * the drawer re-reads the task and the timeline (`reassign` resolves after the caller has queued
 * that), and the form restarts from the task the server returned. On refusal the choice stays and
 * the server's sentence is shown VERBATIM — "Cán bộ được chọn không nhận được việc…", the 409 of a
 * task that just ended, the 503 when identity could not be asked.
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
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);

  function submit(body: petitions_taskAssignmentIn): void {
    setSending(true);
    setError(null);
    setDone(null);
    reassign(body).then((result) => {
      setSending(false);
      if (!result.ok) {
        setError(result.thongBao);
        return;
      }
      setForm(assignmentFormFromTask(result.duLieu));
      setDone(assignmentDoneText(result.duLieu.code));
    });
  }

  return (
    <TaskAssignmentForm
      task={task}
      units={units}
      directory={directory}
      labels={labels}
      form={form}
      change={(patch) => {
        setDone(null);
        setForm((f) => ({ ...f, ...patch }));
      }}
      sending={sending}
      error={error}
      done={done}
      submit={submit}
    />
  );
}

/** Rendering only — exported for `renderToStaticMarkup` (no DOM in vitest). */
export function TaskAssignmentForm({
  task,
  units,
  directory,
  labels,
  form,
  change,
  sending,
  error,
  done,
  submit,
}: {
  task: petitions_nhiemVuRa;
  units: readonly identity_boPhanRa[];
  directory: KetQua<identity_danhBaChonNguoiRa> | null;
  labels: BangNhanTrangThai;
  form: AssignmentForm;
  change: (patch: Partial<AssignmentForm>) => void;
  sending: boolean;
  error: string | null;
  done: string | null;
  submit: (body: petitions_taskAssignmentIn) => void;
}) {
  const staff = docDanhBaChonNguoi(directory);
  const withLeadFields = coKhoiVanBanChiDao(task.type);
  const body = assignmentBody(form, task);
  const blocked = unitWouldBeCleared(form, task)
    ? ASSIGNMENT_UNIT_REQUIRED
    : body === null
      ? ASSIGNMENT_NO_CHANGE
      : null;

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (body !== null && !sending) submit(body);
  }

  return (
    <form
      id={ASSIGNMENT_BLOCK_ID}
      className="form-danh-muc"
      aria-labelledby="tieu-de-giao-viec-chuyen-viec"
      onSubmit={onSubmit}
    >
      <h4 id="tieu-de-giao-viec-chuyen-viec">{ASSIGNMENT_TITLE}</h4>
      <p className="ghi-chu">{assignmentEffectNote(labels, withLeadFields)}</p>

      <div className="o-chon">
        <label htmlFor={ASSIGNMENT_UNIT_FIELD_ID}>{ASSIGNMENT_UNIT_LABEL}</label>
        <select
          id={ASSIGNMENT_UNIT_FIELD_ID}
          value={form.unit}
          onChange={(e) => change({ unit: e.target.value })}
        >
          <option value="">{CHUA_XAC_DINH}</option>
          {unitOptions(units, task.unit).map((u) => (
            <option key={u.id} value={u.id}>
              {u.name}
            </option>
          ))}
        </select>
      </div>

      {staff.loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {assignmentDirectoryError(staff.loi)}
        </p>
      )}
      <StaffCombobox
        id="giao-lai-nguoi-thuc-hien"
        label={ASSIGNMENT_ASSIGNEE_LABEL}
        emptyLabel={nhanTrongOChonCanBo(staff, DE_BO_PHAN_TU_PHAN_CONG)}
        value={form.assignee}
        directory={staff.ds}
        disabled={staff.dangTai}
        onChange={(code) => change({ assignee: code })}
      />

      {/* §5.4's two `theo-van-ban` fields, edited HERE and in the same call (user decision 1). On
          another type they are neither drawn nor sent (`assignmentBody`). */}
      {withLeadFields && (
        <>
          <div className="o-chon">
            <label htmlFor="giao-lai-co-quan-chu-tri">{ASSIGNMENT_LEAD_UNIT_LABEL}</label>
            <select
              id="giao-lai-co-quan-chu-tri"
              value={form.leadUnit}
              onChange={(e) => change({ leadUnit: e.target.value })}
            >
              <option value="">{CHUA_XAC_DINH}</option>
              {unitOptions(units, task.lead_unit).map((u) => (
                <option key={u.id} value={u.id}>
                  {u.name}
                </option>
              ))}
            </select>
          </div>
          <StaffCombobox
            id="giao-lai-chuyen-vien"
            label={ASSIGNMENT_MONITOR_LABEL}
            emptyLabel={nhanTrongOChonCanBo(staff, CHUA_XAC_DINH)}
            value={form.monitor}
            directory={staff.ds}
            disabled={staff.dangTai}
            onChange={(code) => change({ monitor: code })}
          />
        </>
      )}

      <div className="o-nhap">
        <label htmlFor="giao-lai-ly-do">{ASSIGNMENT_NOTE_LABEL}</label>
        <input
          id="giao-lai-ly-do"
          name="giao-lai-ly-do"
          value={form.note}
          maxLength={ASSIGNMENT_NOTE_MAX}
          autoComplete="off"
          onChange={(e) => change({ note: e.target.value })}
        />
      </div>

      {/* NOT `role="alert"`: it shows on first render (nothing chosen yet), and it is the reason
          the button is off — so it sits right above the button and is linked to it. */}
      {blocked !== null && (
        <p id="giao-lai-ly-do-khoa" className="ghi-chu">
          {blocked}
        </p>
      )}
      {error !== null && (
        <p className="thong-bao-loi" role="alert">
          {error}
        </p>
      )}
      {done !== null && <p role="status">{done}</p>}

      <div className="cum-nut">
        <button
          type="submit"
          className="nut-chinh"
          disabled={sending || body === null}
          aria-describedby={blocked !== null ? "giao-lai-ly-do-khoa" : undefined}
        >
          {sending ? ASSIGNMENT_SENDING : ASSIGNMENT_BUTTON}
        </button>
      </div>
    </form>
  );
}
