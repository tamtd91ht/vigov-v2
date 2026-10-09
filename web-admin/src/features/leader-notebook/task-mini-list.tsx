import { CHUA_PHAN_CONG, extensionCountText, lateText, nhanCanBoNgan, nhanNgay, nhanTrangThai, O_TRONG, tinhTrangHan } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing exports of the register's wording module, imported unchanged (rule 12 inv 3)
import type { BangNhanTrangThai } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing type of the status labels
import { SpecStatusBadge } from "@/features/nhiem-vu/so-nhiem-vu";
import { statusChipClass } from "@/features/nhiem-vu/task-spec";
import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing staff-directory lookup type
import type { petitions_deNghiChoDuyetRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract types

import { CHILD_OF_PREFIX } from "./notebook-queries";

/**
 * `TaskMiniList` — the one row list the three columns share (spec 03 §5). Presentation only: rows
 * arrive already selected and ordered by the server; nothing here filters, sorts or counts.
 *
 * EVERY WORD AND FIGURE OF THE DEADLINE LINE COMES FROM THE REGISTER'S OWN FUNCTIONS (`nhanNgay`,
 * `tinhTrangHan`, `lateText`, `extensionCountText`, `nhanCanBoNgan`): the same task must read
 * "trễ 6 ngày" here and on `/nhiem-vu`, and two copies of the arithmetic would one day disagree by a day.
 *
 * A ROW IS A BUTTON that opens the task's detail drawer IN PLACE, on this page (prototype
 * `LeaderNotebook.tsx:161-186`, owner 09/10/2026) — the register's own drawer through
 * `TaskDetailHost`, never a second copy (`leader-notebook.tsx`).
 */

/** The text of a row, in parts — the late, extension and parent parts are drawn apart, so not pre-joined. */
export type TaskRowText = {
  readonly title: string;
  /** `{assignee} · hạn {d/M/yyyy}` — or `Chưa phân công · hạn —`. */
  readonly meta: string;
  /** `trễ 6 ngày` · `trễ dưới 1 ngày`, or `""` when not late or without a deadline. */
  readonly late: string;
  /** `đã gia hạn 2 lần`, or `""` when never extended. */
  readonly extensions: string;
  /** The parent's register code (`NV19`), or `""` for a top-level task. */
  readonly parent: string;
};

export function taskRowText(task: petitions_nhiemVuRa, directory: DanhBaTheoMa | null, now: Date): TaskRowText {
  const due = tinhTrangHan(task.due_at, now);
  const dueDate = due.loai === "khong-han" ? O_TRONG : nhanNgay(task.due_at);
  return {
    title: task.title,
    meta: `${nhanCanBoNgan(task.assignee, directory, CHUA_PHAN_CONG)} · hạn ${dueDate}`,
    late: due.loai === "tre" ? lateText(due.soNgay) : "",
    extensions: extensionCountText(task.extension_count) ?? "",
    parent: task.parent,
  };
}

/*
 * The prototype's row: a full-width button, `px-4 py-3`, a rule under every row but the last, the title
 * in bold and ONE muted meta line under it. The rule sits on the `<li>` (the list keeps its accessible
 * name); the look is the prototype's. No preflight here: the button resets its own chrome (`m-0`,
 * `border-0`, `bg-transparent`, `[font-family:inherit]`). The prototype's `hover:bg-surface` is the page
 * grey — this app's `bg-canvas` (`task-spec.ts`).
 */
const ROW_BUTTON =
  "m-0 block w-full cursor-pointer border-0 bg-transparent px-4 py-3 text-left [font-family:inherit] hover:bg-canvas " +
  "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500";
const ROW_ITEM = "border-line border-b last:border-b-0";
const ROW_TITLE = "text-navy block text-[12.8px] font-semibold";
const ROW_META = "text-ink-muted mt-1 block text-[11.5px]";

export function TaskMiniList({
  tasks,
  directory,
  statusLabels,
  now,
  label,
  onOpen,
}: {
  tasks: readonly petitions_nhiemVuRa[];
  /** Staff directory by code; `null` = not loaded — rows then show the code. */
  directory: DanhBaTheoMa | null;
  /** The status words of the drawer this row opens — a `tam-dung` row shows its badge in them. */
  statusLabels: BangNhanTrangThai;
  now: Date;
  /** Accessible name of the list (the column or group title). */
  label: string;
  onOpen: (task: petitions_nhiemVuRa) => void;
}) {
  return (
    <ul aria-label={label} className="m-0 p-0 [list-style:none]">
      {tasks.map((task) => {
        const row = taskRowText(task, directory, now);
        return (
          <li key={task.code} className={`task-mini-row ${ROW_ITEM}`}>
            <button type="button" aria-haspopup="dialog" className={ROW_BUTTON} onClick={() => onOpen(task)}>
              <span className={ROW_TITLE}>{row.title}</span>
              <span className={ROW_META}>
                {row.meta}
                {row.late !== "" && <span className="text-danger font-semibold"> · {row.late}</span>}
                {row.extensions !== "" && <span className="text-tangerine"> · {row.extensions}</span>}
                {/* Not in the prototype: ADR 0071 puts sub-tasks in these lists, so the row says whose. */}
                {row.parent !== "" && (
                  <>
                    {` · ${CHILD_OF_PREFIX} `}
                    <span className="text-navy font-medium">{row.parent}</span>
                  </>
                )}
              </span>
              {/* Not in the prototype: the register's word-only badge (ADR 0082 #7), for `tam-dung` only —
                  a paused task is not "on its way", and lateness is already said on the line above. */}
              {task.status === "tam-dung" && (
                <span className="mt-1.5 block">
                  <SpecStatusBadge
                    state={{ label: nhanTrangThai(statusLabels, task.status), chip: statusChipClass(task.status), tone: "status" }}
                    status={task.status}
                    className="h-5 text-[10.5px]"
                  />
                </span>
              )}
            </button>
          </li>
        );
      })}
    </ul>
  );
}

/** `hạn 20/6/2026 → 15/7/2026` — the deadline now, then the one asked for. No deadline now: `hạn — → …`. */
export function extensionDeadlineText(request: petitions_deNghiChoDuyetRa): string {
  return `hạn ${nhanNgay(request.task_due_at)} → ${nhanNgay(request.new_due_at)}`;
}

/**
 * The `Duyệt lùi hạn` group: one row per pending request. Clicking opens the TASK's drawer — its
 * extension block carries the request with its `Duyệt` / `Từ chối` buttons (`task-extension-block.tsx`).
 */
export function ExtensionMiniList({
  requests,
  directory,
  label,
  onOpen,
}: {
  requests: readonly petitions_deNghiChoDuyetRa[];
  directory: DanhBaTheoMa | null;
  label: string;
  onOpen: (request: petitions_deNghiChoDuyetRa) => void;
}) {
  return (
    <ul aria-label={label} className="m-0 p-0 [list-style:none]">
      {requests.map((r) => (
        <li key={r.id} className={`extension-mini-row ${ROW_ITEM}`}>
          <button type="button" aria-haspopup="dialog" className={ROW_BUTTON} onClick={() => onOpen(r)}>
            <span className={ROW_TITLE}>{r.task_title}</span>
            <span className={ROW_META}>
              {nhanCanBoNgan(r.requested_by, directory, O_TRONG)} đề nghị · {extensionDeadlineText(r)}
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}
