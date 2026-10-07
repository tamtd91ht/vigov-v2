"use client";

import { Loader2, Paperclip } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { luaChonCanBo } from "@/features/bien-ban/nhan-bien-ban";
import type { KetQua } from "@/lib/api/goi";
import type { StatusMoveExtras } from "@/lib/api/nhiem-vu";
import type {
  identity_boPhanRa,
  identity_danhBaChonNguoiRa,
  petitions_nhatKyNhiemVuRa,
  petitions_nhiemVuRa,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  LY_DO_TRA_LAI_TOI_DA,
  NHAN_LY_DO_TRA_LAI,
  NHAN_NUT_HUY,
  NHAN_NUT_TRA_LAI,
  O_TRONG,
  REOPEN_BUTTON,
  REOPEN_REASON_LABEL,
  TRANG_THAI_CHINH,
  TRANG_THAI_RE_NHANH,
  CAU_THIEU_QUYEN_DUYET_HOAN_THANH,
  CHUA_XAC_DINH,
  DE_BO_PHAN_TU_PHAN_CONG,
  canMoveTask,
  clickableTransitions,
  docDanhBaChonNguoi,
  ghiChuTraLai,
  lacksApprovalFor,
  nhanTrangThai,
  nhanTrongOChonCanBo,
  reasonMove,
  reopenNote,
  yeuCauTraLai,
  type ReasonMove,
} from "./nhan-nhiem-vu";
import type { BangNhanTrangThai, QuyenNhiemVu } from "./nhan-nhiem-vu"; // vi-name-ok: existing types, imported not declared (rule 12 inv 3)
import { ASSIGNMENT_STEPPER_HINT, ASSIGNMENT_UNIT_FIELD_ID, staffInUnit, unitOptions } from "./task-assignment";
import { ATTACH_ACCEPT, ATTACH_INPUT_LABEL, anyInFlight, storedIds } from "./task-attachments";
import { PickedFileChips, useAttachmentUploads } from "./task-attachments-ui";
import { TEXTAREA_CLASS, statusActiveClass, taskStatusHint } from "./task-spec";

/**
 * §5.2 — the status pipeline at the head of the task detail, laid out as the prototype
 * (`vigov-require/apps/admin/src/components/tasks/TaskStatusPipeline.tsx:168-357`): the main flow
 * as chips, a `Rẽ nhánh:` row, the sentence explaining the current status, and — after a chip is
 * pressed — a compose box that asks for the note before anything is sent.
 *
 * A CHIP IS PRESSABLE ONLY WHEN THE MOVE IS ONE THIS ACCOUNT MAY TAKE (ADR 0068 §Sửa đổi 05/10 #3):
 * the plain moves are `clickableTransitions` — THE SAME LIST the Kanban menu uses, so the two
 * surfaces cannot offer different steps — and the two moves carrying a mandatory reason (return
 * from review, reopen) are `reasonMove`. Every other chip is a DISABLED button with the spec's title
 * (`STEP_NOT_REACHABLE`), the current one filled with its status colour (spec 07 §2). Convenience,
 * not protection: `POST /api/v1/tasks/{code}/status` decides again on the row (rule 5, forbidden #1).
 *
 * THE REASON MOVES NEVER SEND AN EMPTY NOTE (ADR 0065 NV2): their box labels the field as the
 * reason, marks it required, and `Xác nhận` stays disabled until `yeuCauTraLai` accepts it — the same
 * trim the server applies. The target of those two moves comes from `yeuCauTraLai`, not from the
 * chip, so a chip wired to the wrong code cannot send a forward move with a return reason.
 *
 * ĐỔI CHIỀU CÓ CHỦ Ý 07/10/2026 (ADR 0076 lần 2: the drawer follows spec 07): `Chờ duyệt` sits on
 * the main flow ONLY while it is the current status, as the prototype draws it; a move into it is
 * still on the Kanban's menu and drop. The hint under the strip is spec 10's `TASK_STATUS_HINT`,
 * verbatim (#3). Outcomes are TOASTS (lần 6 #4).
 *
 * TIME IN STATUS is derived from the task's timeline, as the prototype does (`statusEnteredAt`): the
 * oldest row of the newest run of rows in the current status. Older rows not loaded and the whole page
 * in that status → unknown, a dash — a guess would read as a fact.
 *
 * THE COMPOSE BOX CARRIES THE HAND-OVER AND FILES (prototype `TaskStatusPipeline.tsx:261-355`,
 * `handover` / `attachments` of `…/status`, cebe0d47): unit and person (for `task.assign` or the
 * assignee — the server decides again), `Đính kèm ảnh, tệp` (ADR 0052 uploads, only STORED ids go);
 * `Xác nhận` becomes `Cập nhật và giao việc` when the holder changes. One transaction on the server.
 *
 * NO OPTIMISTIC UPDATE: the chip stays where it is until the server answers; the caller refreshes
 * the task on success.
 */

/** Step list label, so a screen reader announces "list, n items". */
export const STATUS_STEPS_LABEL = "Các bước của vòng đời nhiệm vụ";
/** Label of the branch row — the prototype's words. */
export const BRANCH_ROW_LABEL = "Rẽ nhánh:";
/** Under the strip, for an account that may not move this row (prototype `TaskStatusPipeline:359-363`). */
export const STATUS_MOVE_DENIED = "Bạn không có quyền đổi trạng thái nhiệm vụ này.";
/** The server's list is empty: said, so an empty strip is not read as a broken screen. */
export const STATUS_NO_EXIT = "Máy chủ không liệt kê lối ra nào khỏi trạng thái này.";
/** Label of the note field of a plain move — the prototype's words. */
export const STATUS_NOTE_LABEL = "Nội dung cập nhật";
export const STATUS_NOTE_PLACEHOLDER = "Ghi lại vì sao chuyển, đã làm được gì… (không bắt buộc)";
export const STATUS_CONFIRM_BUTTON = "Xác nhận";
/** Id of the compose box — the chip that opened it points at it (`aria-controls`). */
export const STATUS_COMPOSE_ID = "task-status-compose";

/** Accessible name of a chip that is a plain move. Contains the visible label (WCAG 2.5.3). */
export function statusMoveName(label: string): string {
  return `Chuyển sang ${label}`;
}

/** Accessible name of a chip that opens a reason move — the action first, then the visible label. */
export function reasonMoveName(kind: ReasonMove, label: string): string {
  return `${kind === "reopen" ? REOPEN_BUTTON : NHAN_NUT_TRA_LAI} — ${label}`;
}

/** Said after the server accepted a move. The label is the commune's (`nhanTT`). */
export function statusMoveDoneText(label: string): string {
  return `Đã chuyển sang “${label}”.`;
}

/** Which move a chip opens: a plain move (optional note) or a reason move (mandatory note). */
export type StatusMove = { readonly target: string; readonly kind: "plain" | ReasonMove };

/**
 * The move a chip for `code` opens for this account, or `null` (the chip is text). Exported so the
 * gate is tested without a DOM. Derived ONLY from `clickableTransitions` and `reasonMove` — no
 * third filter of its own.
 */
export function chipMove(
  task: Pick<petitions_nhiemVuRa, "assignee" | "status" | "allowed_transitions">,
  permissions: QuyenNhiemVu,
  staffCode: string,
  code: string,
): StatusMove | null {
  if (code === task.status) return null;
  if ((clickableTransitions(task, permissions, staffCode) as readonly string[]).includes(code)) {
    return { target: code, kind: "plain" };
  }
  const reason = reasonMove(task, permissions, staffCode);
  return reason !== null && code === "dang-thuc-hien" ? { target: code, kind: reason } : null;
}

/**
 * When the task entered its current status, from its timeline (newest first, as the server sends it):
 * the oldest row of the newest run of rows whose status is the current one. `complete` = the rows
 * reach the task's first row. Returns `null` when unknown: no rows yet, or the whole loaded page is
 * in the current status while older rows exist. No row at all and complete → `created_at`.
 */
export function statusEnteredAt(
  task: Pick<petitions_nhiemVuRa, "status" | "created_at">,
  rows: readonly Pick<petitions_nhatKyNhiemVuRa, "status" | "at">[],
  complete: boolean,
): string | null {
  let run = 0;
  while (run < rows.length && rows[run]!.status === task.status) run += 1;
  if (run === 0) return complete && rows.length === 0 ? task.created_at : null;
  if (run < rows.length) return rows[run - 1]!.at;
  return complete ? rows[run - 1]!.at : null;
}

/** Prototype `TaskStatusPipeline.tsx:77-86`: `n ngày m giờ`, `n giờ`, `vừa xong`; `null` = unknown. */
export function elapsedText(since: string | null, now: Date): string | null {
  if (since === null) return null;
  const ms = now.getTime() - new Date(since).getTime();
  if (Number.isNaN(ms) || ms < 0) return null;
  const hours = Math.floor(ms / 3_600_000);
  const days = Math.floor(hours / 24);
  if (days > 0) return `${days} ngày ${hours % 24} giờ`;
  if (hours > 0) return `${hours} giờ`;
  return "vừa xong";
}

/** The compose box's button when the holder changes too (prototype). */
export const STATUS_CONFIRM_HANDOVER = "Cập nhật và giao việc";
export const STATUS_ATTACH_BUTTON = "Đính kèm ảnh, tệp";

/** `title` of a step that is neither the current one nor a move this account may take (spec 07 §2). */
export const STEP_NOT_REACHABLE = "Không chuyển thẳng sang bước này được";
export const STEP_CURRENT = "Trạng thái hiện tại";

/** Chip frame — two lines, as the prototype: the label, then the time in status (spec 07 §2). */
const CHIP_FRAME =
  "inline-flex min-w-[6.5rem] flex-col items-start rounded-[8px] border px-3 py-1.5 text-left [font-family:inherit] transition-colors";
const CHIP_CLICKABLE =
  "border-line text-ink hover:bg-canvas cursor-pointer bg-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500 disabled:cursor-not-allowed disabled:opacity-60";
const CHIP_UNREACHABLE = "border-line/60 text-ink-muted/60 cursor-not-allowed bg-white";

type StepLook = "current" | "before" | "after" | "branch";

/** The two lines inside a chip. `data-step` stays on the LABEL, where tests and styles read it. */
function ChipBody({ label, step, time = null }: { label: string; step: StepLook; time?: string | null }) {
  const current = step === "current";
  return (
    <>
      <span
        className="block text-[12px] leading-tight font-semibold"
        aria-current={current ? "step" : undefined}
        data-step={step}
      >
        {label}
      </span>
      {/* Reserved even when empty so every chip is the same height (prototype). */}
      <span aria-hidden="true" className={cn("block text-[10.5px] tabular-nums", current ? "text-white/80" : "text-ink-muted")}>
        {current && time !== null ? time : O_TRONG}
      </span>
    </>
  );
}

function StatusChip({
  code,
  label,
  step,
  time,
  move,
  open,
  disabled,
  onOpen,
}: {
  code: string;
  label: string;
  step: StepLook;
  time: string | null;
  move: StatusMove | null;
  open: boolean;
  disabled: boolean;
  onOpen: (move: StatusMove) => void;
}) {
  if (move === null) {
    // The prototype draws every step as a button: the current one coloured, the others DISABLED with
    // a title saying why — never a control that can only be refused.
    const current = step === "current";
    return (
      <button
        type="button"
        disabled
        title={current ? STEP_CURRENT : STEP_NOT_REACHABLE}
        className={cn(CHIP_FRAME, current ? cn("border-transparent text-white", statusActiveClass(code)) : CHIP_UNREACHABLE)}
      >
        <ChipBody label={label} step={step} time={time} />
      </button>
    );
  }
  return (
    <button
      type="button"
      id={`task-status-chip-${code}`}
      className={cn(CHIP_FRAME, CHIP_CLICKABLE)}
      // The hover title says the same act as the accessible name: a return / reopen is never
      // announced as a plain move.
      title={move.kind === "plain" ? statusMoveName(label) : reasonMoveName(move.kind, label)}
      aria-label={move.kind === "plain" ? statusMoveName(label) : reasonMoveName(move.kind, label)}
      aria-expanded={open}
      aria-controls={open ? STATUS_COMPOSE_ID : undefined}
      disabled={disabled}
      onClick={() => onOpen(move)}
    >
      <ChipBody label={label} step={step} />
    </button>
  );
}

export function TaskStatusPipeline({
  task,
  labels,
  permissions,
  staffCode,
  showAssignment,
  sending,
  move,
  history = null,
  now,
  units = [],
  directory = null,
}: {
  task: petitions_nhiemVuRa;
  /** The status words (the spec's on this screen). The strip keeps the lifecycle ORDER of §6, not `order`. */
  labels: BangNhanTrangThai;
  permissions: QuyenNhiemVu;
  /** `phien.staff.code` — `""` while the session is unread: then no row is this account's. */
  staffCode: string;
  /** §5.7 block shown: the `Chuyển tiếp` chip moves focus to it (no status call). */
  showAssignment: boolean;
  /** A write of the drawer is in flight. */
  sending: boolean;
  /** `POST /api/v1/tasks/{code}/status`. The caller refreshes the drawer and the register on success. */
  move: (target: string, note?: string, extra?: StatusMoveExtras) => Promise<KetQua<petitions_nhiemVuRa>>;
  /** The timeline rows loaded by the drawer's log (`null` = not yet) — for the time in status. */
  history?: { readonly rows: readonly petitions_nhatKyNhiemVuRa[]; readonly complete: boolean } | null;
  now: Date;
  /** The commune's units and the whole-commune directory — the compose box's hand-over fields. */
  units?: readonly identity_boPhanRa[];
  directory?: KetQua<identity_danhBaChonNguoiRa> | null;
}) {
  const [pending, setPending] = useState<StatusMove | null>(null);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [unit, setUnit] = useState(task.unit);
  const [assignee, setAssignee] = useState(task.assignee);
  const files = useAttachmentUploads(task.code);
  const fileInput = useRef<HTMLInputElement>(null);
  const field = useRef<HTMLTextAreaElement>(null);
  const steps = useRef<HTMLOListElement>(null);

  useEffect(() => {
    if (pending !== null) field.current?.focus();
  }, [pending]);

  const canMove = canMoveTask(permissions, task, staffCode);
  const moveOf = (code: string) => chipMove(task, permissions, staffCode, code);
  const currentStep = (TRANG_THAI_CHINH as readonly string[]).indexOf(task.status);
  // Spec 07 §2: `Chờ duyệt` is on the main flow ONLY while the task is in it (prototype
  // `TaskStatusPipeline.tsx:222-225`). A move into it stays on the Kanban's menu and drop.
  const flow = TRANG_THAI_CHINH.filter((code) => code !== "cho-duyet" || task.status === code);
  const branches = TRANG_THAI_RE_NHANH.filter(
    (code) =>
      task.status === code || (code === "chuyen-tiep" ? showAssignment : moveOf(code) !== null),
  );
  const disabled = sending || busy;
  const here =
    history === null ? null : elapsedText(statusEnteredAt(task, history.rows, history.complete), now);
  // The hand-over is for `task.assign` holders or the assignee (the route's rule; the server checks).
  const canHandover = permissions.reassign || (staffCode !== "" && staffCode === task.assignee);
  // A side counts only when it names a NEW holder: `handover` cannot clear a field (the client drops
  // an empty value), so an emptied select must not relabel the button as a hand-over.
  const unitChanged = unit !== task.unit && unit !== "";
  const assigneeChanged = assignee !== task.assignee && assignee !== "";
  const handedOver = canHandover && (unitChanged || assigneeChanged);
  const waiting = anyInFlight(files.items);

  function resetBox(): void {
    setNote("");
    setUnit(task.unit);
    setAssignee(task.assignee);
    files.clear();
  }

  function open(m: StatusMove): void {
    setPending(m);
    resetBox();
  }

  function cancel(): void {
    const opener = pending === null ? null : document.getElementById(`task-status-chip-${pending.target}`);
    setPending(null);
    resetBox();
    opener?.focus();
  }

  async function submit(e: FormEvent): Promise<void> {
    e.preventDefault();
    if (pending === null || disabled || waiting) return;
    let target = pending.target;
    let text = note.trim();
    if (pending.kind !== "plain") {
      const reason = yeuCauTraLai(note);
      if (reason === null) return;
      target = reason.trangThai;
      text = reason.ghiChu;
    }
    setBusy(true);
    const extra: StatusMoveExtras = {
      handover: handedOver
        ? { unit: unitChanged ? unit : undefined, assignee: assigneeChanged ? assignee : undefined }
        : undefined,
      attachments: storedIds(files.items),
    };
    const r = await move(target, text, extra);
    setBusy(false);
    if (!r.ok) {
      // Stays open, the typed note kept; the server's sentence verbatim (spec 07: a toast).
      toast.error(r.thongBao);
      return;
    }
    setPending(null);
    resetBox();
    toast.success(statusMoveDoneText(nhanTrangThai(labels, target)));
    // The box and its buttons are gone: focus must not fall to the top of the page.
    steps.current?.focus();
  }

  const reasonKind = pending !== null && pending.kind !== "plain" ? pending.kind : null;
  const reasonReady = reasonKind === null || yeuCauTraLai(note) !== null;
  // Same ids as the reason forms always had; a plain move's note has its own.
  const fieldId =
    reasonKind === "reopen" ? "ly-do-mo-lai" : reasonKind === "return" ? "ly-do-tra-lai" : "noi-dung-chuyen-trang-thai";

  return (
    <div className="border-line shrink-0 border-b bg-white px-5 py-3">
      <div className="flex flex-wrap items-center gap-1.5">
        <ol
          ref={steps}
          tabIndex={-1}
          aria-label={STATUS_STEPS_LABEL}
          className="m-0 flex list-none flex-wrap items-center gap-1.5 p-0 focus-visible:outline-2 focus-visible:outline-brand-500"
        >
          {flow.map((code) => {
            // "before" is POSITION in the main order, not a claim the step was passed — a task may go
            // from `dang-thuc-hien` straight to `hoan-thanh` (ADR 0065 NV1).
            const index = TRANG_THAI_CHINH.indexOf(code);
            const step: StepLook =
              code === task.status ? "current" : currentStep >= 0 && index < currentStep ? "before" : "after";
            const m = moveOf(code);
            return (
              <li key={code} className="flex">
                <StatusChip
                  code={code}
                  label={nhanTrangThai(labels, code)}
                  step={step}
                  time={here}
                  move={m}
                  open={pending !== null && m !== null && pending.target === code}
                  disabled={disabled}
                  onOpen={open}
                />
              </li>
            );
          })}
        </ol>
      </div>

      {branches.length > 0 && (
        <div className="mt-2 flex flex-wrap items-center gap-1.5">
          <span className="text-ink-muted mr-1 text-[11px]">{BRANCH_ROW_LABEL}</span>
          {branches.map((code) => {
            const label = nhanTrangThai(labels, code);
            const step: StepLook = code === task.status ? "current" : "branch";
            // `Chuyển tiếp` IS NO LONGER A STATUS MOVE (owner decision 28/09/2026; `…/status` answers
            // 400): with the §5.7 block shown, its chip moves FOCUS to that block — no call is made.
            if (code === "chuyen-tiep" && showAssignment && step !== "current") {
              return (
                <button
                  key={code}
                  type="button"
                  className={cn(CHIP_FRAME, CHIP_CLICKABLE)}
                  aria-controls={ASSIGNMENT_UNIT_FIELD_ID}
                  aria-label={`${label} — ${ASSIGNMENT_STEPPER_HINT}`}
                  onClick={() => document.getElementById(ASSIGNMENT_UNIT_FIELD_ID)?.focus()}
                >
                  <ChipBody label={label} step={step} />
                </button>
              );
            }
            const m = moveOf(code);
            return (
              <StatusChip
                key={code}
                code={code}
                label={label}
                step={step}
                time={here}
                move={m}
                open={pending !== null && m !== null && pending.target === code}
                disabled={disabled}
                onOpen={open}
              />
            );
          })}
        </div>
      )}

      {/* Spec 10 `TASK_STATUS_HINT`, verbatim (owner 07/10/2026 #3). */}
      {taskStatusHint(task.status) !== "" && (
        <p className="text-ink-muted m-0 mt-2 text-[11.5px]">{taskStatusHint(task.status)}</p>
      )}

      {pending !== null && (
        <form
          id={STATUS_COMPOSE_ID}
          aria-labelledby={`${STATUS_COMPOSE_ID}-title`}
          className="border-brand/35 border-l-brand mt-2.5 rounded-[10px] border border-l-4 bg-white p-3"
          onSubmit={submit}
        >
          <p id={`${STATUS_COMPOSE_ID}-title`} className="text-navy m-0 text-[12.5px] font-semibold">
            {reasonKind === "reopen"
              ? REOPEN_BUTTON
              : reasonKind === "return"
                ? NHAN_NUT_TRA_LAI
                : `Chuyển sang “${nhanTrangThai(labels, pending.target)}”`}
          </p>
          <p className="text-ink-muted m-0 mt-0.5 text-[11.5px]">
            {reasonKind === "reopen"
              ? reopenNote(labels)
              : reasonKind === "return"
                ? ghiChuTraLai(labels)
                : taskStatusHint(pending.target)}
          </p>
          {canHandover && (
            <ComposeHandover
              units={units}
              directory={directory}
              currentUnit={task.unit}
              unit={unit}
              assignee={assignee}
              setUnit={setUnit}
              setAssignee={setAssignee}
            />
          )}
          <div className="mt-2">
            <label htmlFor={fieldId} className="text-ink block text-[11.5px] font-medium">
              {reasonKind === "reopen"
                ? REOPEN_REASON_LABEL
                : reasonKind === "return"
                  ? NHAN_LY_DO_TRA_LAI
                  : STATUS_NOTE_LABEL}
            </label>
            <textarea
              ref={field}
              id={fieldId}
              name={fieldId}
              rows={2}
              required={reasonKind !== null}
              maxLength={LY_DO_TRA_LAI_TOI_DA}
              placeholder={reasonKind === null ? STATUS_NOTE_PLACEHOLDER : undefined}
              value={note}
              className={cn(TEXTAREA_CLASS, "mt-1")}
              onChange={(e) => setNote(e.target.value)}
            />
          </div>
          <PickedFileChips items={files.items} disabled={busy} onRetry={files.retry} onRemove={files.remove} />
          <div className="mt-2 flex items-center gap-2">
            <input
              ref={fileInput}
              id={`${STATUS_COMPOSE_ID}-tep`}
              name={`${STATUS_COMPOSE_ID}-tep`}
              type="file"
              multiple
              accept={ATTACH_ACCEPT}
              className="hidden"
              aria-label={ATTACH_INPUT_LABEL}
              disabled={busy}
              onChange={(e) => {
                const picked = e.target.files === null ? [] : Array.from(e.target.files);
                e.target.value = ""; // the same file can be chosen again after a removal
                if (picked.length > 0) files.add(picked);
              }}
            />
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              icon={<Paperclip aria-hidden="true" focusable="false" className="size-3.5" />}
              onClick={() => fileInput.current?.click()}
            >
              {STATUS_ATTACH_BUTTON}
            </Button>
            <Button type="button" variant="outline" size="sm" className="ml-auto" disabled={busy} onClick={cancel}>
              {NHAN_NUT_HUY}
            </Button>
            <Button
              type="submit"
              variant="primary"
              size="sm"
              disabled={disabled || waiting || !reasonReady}
              aria-busy={busy || undefined}
              icon={busy ? <Loader2 aria-hidden="true" focusable="false" className="size-3.5 animate-spin" /> : undefined}
            >
              {handedOver ? STATUS_CONFIRM_HANDOVER : STATUS_CONFIRM_BUTTON}
            </Button>
          </div>
        </form>
      )}

      {/* The denied and empty states keep their sentence (owner 07/10/2026 #13): a strip of disabled
          chips alone does not say WHY nothing can be pressed. */}
      {canMove && lacksApprovalFor(task, permissions, staffCode) && (
        <p className="text-ink-muted m-0 mt-2 text-[11px]">{CAU_THIEU_QUYEN_DUYET_HOAN_THANH}</p>
      )}
      {canMove && task.allowed_transitions.length === 0 && (
        <p className="text-ink-muted m-0 mt-2 text-[11px]">{STATUS_NO_EXIT}</p>
      )}
      {!canMove && <p className="text-ink-muted m-0 mt-2 text-[11px]">{STATUS_MOVE_DENIED}</p>}
    </div>
  );
}

const COMPOSE_SELECT_CLASS =
  "border-line focus-visible:ring-ring/50 mt-1 h-9 w-full rounded-md border bg-white px-2.5 text-[12.5px] text-ink [font-family:inherit] outline-none focus-visible:ring-[3px]";

/**
 * The compose box's hand-over (prototype `HandoverFields`): the unit, then the person filtered by
 * it. Both start on the task's current holder; a change is what turns the move into a hand-over.
 */
function ComposeHandover({
  units,
  directory,
  currentUnit,
  unit,
  assignee,
  setUnit,
  setAssignee,
}: {
  units: readonly identity_boPhanRa[];
  directory: KetQua<identity_danhBaChonNguoiRa> | null;
  currentUnit: string;
  unit: string;
  assignee: string;
  setUnit: (v: string) => void;
  setAssignee: (v: string) => void;
}) {
  const staff = docDanhBaChonNguoi(directory);
  const people = luaChonCanBo(staffInUnit(staff.ds, unit), assignee);
  return (
    <div className="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2">
      <div>
        <label htmlFor={`${STATUS_COMPOSE_ID}-bo-phan`} className="text-ink block text-[11.5px] font-medium">
          Bộ phận
        </label>
        <select
          id={`${STATUS_COMPOSE_ID}-bo-phan`}
          value={unit}
          className={COMPOSE_SELECT_CLASS}
          onChange={(e) => {
            const next = e.target.value;
            const stays = assignee === "" || staffInUnit(staff.ds, next).some((c) => c.code === assignee);
            setUnit(next);
            if (!stays) setAssignee("");
          }}
        >
          <option value="">{CHUA_XAC_DINH}</option>
          {unitOptions(units, currentUnit).map((u) => (
            <option key={u.id} value={u.id}>
              {u.name}
            </option>
          ))}
        </select>
      </div>
      <div>
        <label htmlFor={`${STATUS_COMPOSE_ID}-nguoi`} className="text-ink block text-[11.5px] font-medium">
          Người thực hiện
        </label>
        <select
          id={`${STATUS_COMPOSE_ID}-nguoi`}
          value={assignee}
          disabled={staff.dangTai}
          className={COMPOSE_SELECT_CLASS}
          onChange={(e) => setAssignee(e.target.value)}
        >
          <option value="">{nhanTrongOChonCanBo(staff, DE_BO_PHAN_TU_PHAN_CONG)}</option>
          {people.map((o) => (
            <option key={o.ma} value={o.ma}>
              {o.nhan}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
