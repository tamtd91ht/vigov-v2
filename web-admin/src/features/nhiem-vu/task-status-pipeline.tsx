"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";

import type { KetQua } from "@/lib/api/goi";
import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  CAU_THIEU_QUYEN_DUYET_HOAN_THANH,
  LY_DO_TRA_LAI_TOI_DA,
  NHAN_LY_DO_TRA_LAI,
  NHAN_NUT_HUY,
  NHAN_NUT_TRA_LAI,
  O_TRONG,
  REOPEN_BUTTON,
  REOPEN_REASON_LABEL,
  TRANG_THAI_CHINH,
  TRANG_THAI_RE_NHANH,
  canMoveTask,
  cauGiaiThichTrangThai,
  clickableTransitions,
  ghiChuTraLai,
  lacksApprovalFor,
  nhanTrangThai,
  reasonMove,
  reopenNote,
  yeuCauTraLai,
  type ReasonMove,
} from "./nhan-nhiem-vu";
import type { BangNhanTrangThai, QuyenNhiemVu } from "./nhan-nhiem-vu"; // vi-name-ok: existing types, imported not declared (rule 12 inv 3)
import { ASSIGNMENT_STEPPER_HINT, ASSIGNMENT_UNIT_FIELD_ID } from "./task-assignment";

/**
 * §5.2 — the status pipeline at the head of the task detail, laid out as the prototype
 * (`vigov-require/apps/admin/src/components/tasks/TaskStatusPipeline.tsx:168-357`): the main flow
 * as chips, a `Rẽ nhánh:` row, the sentence explaining the current status, and — after a chip is
 * pressed — a compose box that asks for the note before anything is sent.
 *
 * A CHIP IS A BUTTON ONLY WHEN THE MOVE IS ONE THIS ACCOUNT MAY TAKE (ADR 0068 §Sửa đổi 05/10 #3):
 * the plain moves are `clickableTransitions` — THE SAME LIST the Kanban menu uses, so the two
 * surfaces cannot offer different steps — and the two moves carrying a mandatory reason (return
 * from review, reopen) are `reasonMove`. Every other chip is text. Convenience, not protection:
 * `POST /api/v1/tasks/{code}/status` decides again on the row (rule 5, forbidden #1).
 *
 * THE REASON MOVES NEVER SEND AN EMPTY NOTE (ADR 0065 NV2): their box labels the field as the
 * reason, marks it required, and `Xác nhận` stays disabled until `yeuCauTraLai` accepts it — the same
 * trim the server applies. The target of those two moves comes from `yeuCauTraLai`, not from the
 * chip, so a chip wired to the wrong code cannot send a forward move with a return reason.
 *
 * `Chờ duyệt` sits on the main flow only when it is the current status or a move this account may
 * take. The prototype hides it outright (`TaskStatusPipeline.tsx:33-37`) for its commune; here the
 * step is still in the server's lifecycle, and hiding a move the server lists would take it away
 * from the drawer while the Kanban still offers it.
 *
 * TIME IN STATUS IS A DASH: the last status change lives in the timeline, which this strip does not
 * read. A `0` there would read as "just moved" — the opposite of "unknown".
 *
 * NO OPTIMISTIC UPDATE: the chip stays where it is until the server answers; the caller refreshes
 * the task on success. Attachments and the hand-over fields of the prototype's box are NOT drawn —
 * they belong to a later card.
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

/** Chip frame — two lines, as the prototype: the label, then the time in status. */
const CHIP_FRAME =
  "inline-flex min-w-[6.5rem] flex-col items-start rounded-control border border-solid px-3 py-1.5 text-left [font-family:inherit]";
const CHIP_LOOK = {
  current: "border-transparent bg-brand-600 text-white",
  clickable:
    "cursor-pointer border-line-strong bg-surface text-ink-900 transition-[background-color,border-color] duration-150 hover:border-accent-500 hover:bg-accent-50 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500 disabled:cursor-not-allowed disabled:opacity-60",
  muted: "border-line bg-surface text-ink-500",
} as const;

type StepLook = "current" | "before" | "after" | "branch";

/** The two lines inside a chip. `data-step` stays on the LABEL, where tests and styles read it. */
function ChipBody({ label, step }: { label: string; step: StepLook }) {
  const current = step === "current";
  return (
    <>
      <span
        className="block text-[13px] leading-tight font-semibold"
        aria-current={current ? "step" : undefined}
        data-step={step}
      >
        {label}
      </span>
      {/* Reserved even when empty so every chip is the same height (prototype). */}
      <span aria-hidden="true" className={cn("block text-[11px] tabular-nums", current ? "text-white/80" : "text-ink-500")}>
        {O_TRONG}
      </span>
    </>
  );
}

function StatusChip({
  code,
  label,
  step,
  move,
  open,
  disabled,
  onOpen,
}: {
  code: string;
  label: string;
  step: StepLook;
  move: StatusMove | null;
  open: boolean;
  disabled: boolean;
  onOpen: (move: StatusMove) => void;
}) {
  if (move === null) {
    return (
      <span className={cn(CHIP_FRAME, step === "current" ? CHIP_LOOK.current : CHIP_LOOK.muted)}>
        <ChipBody label={label} step={step} />
      </span>
    );
  }
  return (
    <button
      type="button"
      id={`task-status-chip-${code}`}
      className={cn(CHIP_FRAME, CHIP_LOOK.clickable)}
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
}: {
  task: petitions_nhiemVuRa;
  /** The commune's labels (decision #21). The strip keeps the lifecycle ORDER of §6, not `order`. */
  labels: BangNhanTrangThai;
  permissions: QuyenNhiemVu;
  /** `phien.staff.code` — `""` while the session is unread: then no row is this account's. */
  staffCode: string;
  /** §5.7 block shown: the `Chuyển tiếp` chip moves focus to it (no status call). */
  showAssignment: boolean;
  /** A write of the drawer is in flight. */
  sending: boolean;
  /** `POST /api/v1/tasks/{code}/status`. The caller refreshes the drawer and the register on success. */
  move: (target: string, note?: string) => Promise<KetQua<petitions_nhiemVuRa>>;
}) {
  const [pending, setPending] = useState<StatusMove | null>(null);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);
  const field = useRef<HTMLTextAreaElement>(null);
  const steps = useRef<HTMLOListElement>(null);

  useEffect(() => {
    if (pending !== null) field.current?.focus();
  }, [pending]);

  const canMove = canMoveTask(permissions, task, staffCode);
  const moveOf = (code: string) => chipMove(task, permissions, staffCode, code);
  const currentStep = (TRANG_THAI_CHINH as readonly string[]).indexOf(task.status);
  const flow = TRANG_THAI_CHINH.filter(
    (code) => code !== "cho-duyet" || task.status === code || moveOf(code) !== null,
  );
  const branches = TRANG_THAI_RE_NHANH.filter(
    (code) =>
      task.status === code || (code === "chuyen-tiep" ? showAssignment : moveOf(code) !== null),
  );
  const disabled = sending || busy;

  function open(m: StatusMove): void {
    setPending(m);
    setNote("");
    setError(null);
    setDone(null);
  }

  function cancel(): void {
    const opener = pending === null ? null : document.getElementById(`task-status-chip-${pending.target}`);
    setPending(null);
    setNote("");
    setError(null);
    opener?.focus();
  }

  async function submit(e: FormEvent): Promise<void> {
    e.preventDefault();
    if (pending === null || disabled) return;
    let target = pending.target;
    let text = note.trim();
    if (pending.kind !== "plain") {
      const reason = yeuCauTraLai(note);
      if (reason === null) return;
      target = reason.trangThai;
      text = reason.ghiChu;
    }
    setBusy(true);
    setError(null);
    const r = await move(target, text);
    setBusy(false);
    if (!r.ok) {
      // Stays open, the typed note kept, the server's sentence verbatim.
      setError(r.thongBao);
      return;
    }
    setPending(null);
    setNote("");
    setDone(statusMoveDoneText(nhanTrangThai(labels, target)));
    // The box and its buttons are gone: focus must not fall to the top of the page.
    steps.current?.focus();
  }

  const reasonKind = pending !== null && pending.kind !== "plain" ? pending.kind : null;
  const reasonReady = reasonKind === null || yeuCauTraLai(note) !== null;
  // Same ids as the reason forms always had; a plain move's note has its own.
  const fieldId =
    reasonKind === "reopen" ? "ly-do-mo-lai" : reasonKind === "return" ? "ly-do-tra-lai" : "noi-dung-chuyen-trang-thai";
  const explanation = cauGiaiThichTrangThai(task.status);

  return (
    <div>
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
                move={m}
                open={pending !== null && m !== null && pending.target === code}
                disabled={disabled}
                onOpen={open}
              />
            </li>
          );
        })}
      </ol>

      {branches.length > 0 && (
        <div className="mt-2 flex flex-wrap items-center gap-1.5">
          <span className="mr-1 text-xs text-ink-500">{BRANCH_ROW_LABEL}</span>
          {branches.map((code) => {
            const label = nhanTrangThai(labels, code);
            const step: StepLook = code === task.status ? "current" : "branch";
            // `Chuyển tiếp` IS NO LONGER A STATUS MOVE (owner decision 28/09/2026; `…/status` answers
            // 400): with the §5.7 block shown, its chip moves FOCUS to that block — no call is made.
            if (code === "chuyen-tiep" && showAssignment) {
              return (
                <button
                  key={code}
                  type="button"
                  className={cn(CHIP_FRAME, step === "current" ? CHIP_LOOK.current : CHIP_LOOK.clickable)}
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
                move={m}
                open={pending !== null && m !== null && pending.target === code}
                disabled={disabled}
                onOpen={open}
              />
            );
          })}
        </div>
      )}

      {explanation !== "" && <p className="m-0 mt-2 text-sm text-ink-700">{explanation}</p>}

      {pending !== null && (
        <form
          id={STATUS_COMPOSE_ID}
          aria-labelledby={`${STATUS_COMPOSE_ID}-title`}
          className="mt-3 rounded-card border border-l-4 border-solid border-brand-500/35 border-l-brand-500 bg-surface p-3"
          onSubmit={submit}
        >
          <h4 id={`${STATUS_COMPOSE_ID}-title`} className="m-0 text-sm font-semibold text-ink-900">
            {reasonKind === "reopen"
              ? REOPEN_BUTTON
              : reasonKind === "return"
                ? NHAN_NUT_TRA_LAI
                : `Chuyển sang “${nhanTrangThai(labels, pending.target)}”`}
          </h4>
          <p className="ghi-chu">
            {reasonKind === "reopen"
              ? reopenNote(labels)
              : reasonKind === "return"
                ? ghiChuTraLai(labels)
                : cauGiaiThichTrangThai(pending.target)}
          </p>
          <div className="o-nhap">
            <label htmlFor={fieldId}>
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
              onChange={(e) => setNote(e.target.value)}
            />
          </div>
          {error !== null && (
            <p className="thong-bao-loi" role="alert">
              {error}
            </p>
          )}
          <div className="cum-nut">
            <button type="button" className="nut-phu" disabled={busy} onClick={cancel}>
              {NHAN_NUT_HUY}
            </button>
            <button type="submit" className="nut-chinh" disabled={disabled || !reasonReady}>
              {STATUS_CONFIRM_BUTTON}
            </button>
          </div>
        </form>
      )}

      {/* Always in the DOM: a live region inserted later is not always announced. */}
      <p className="m-0 mt-2 text-[13px] text-ink-700 empty:hidden" role="status">
        {done ?? ""}
      </p>

      {canMove && lacksApprovalFor(task, permissions, staffCode) && (
        <p className="ghi-chu">{CAU_THIEU_QUYEN_DUYET_HOAN_THANH}</p>
      )}
      {canMove && task.allowed_transitions.length === 0 && <p className="trang-thai-rong">{STATUS_NO_EXIT}</p>}
      {!canMove && <p className="m-0 mt-2 text-xs text-ink-500">{STATUS_MOVE_DENIED}</p>}
    </div>
  );
}
