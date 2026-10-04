import Link from "next/link";

import { CHUA_PHAN_CONG, lateText, nhanCanBoNgan, nhanNgay, nhanTrangThai, O_TRONG, tinhTrangHan } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing exports of the register's wording module, imported unchanged (rule 12 inv 3)
import type { BangNhanTrangThai } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing type of the commune's status labels
import { TaskStatusBadge } from "@/features/nhiem-vu/task-ui";
import { taskDetailHref } from "@/features/nhiem-vu/task-link";
import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing staff-directory lookup type
import type { petitions_deNghiChoDuyetRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract types

import { childOfText } from "./notebook-queries";

/**
 * `TaskMiniList` — the one row list the three columns share (spec 03 §5). Presentation only: rows
 * arrive already selected and ordered by the server; nothing here filters, sorts or counts.
 *
 * EVERY WORD AND FIGURE OF THE DEADLINE LINE COMES FROM THE REGISTER'S OWN FUNCTIONS (`nhanNgay`,
 * `tinhTrangHan`, `lateText`, `nhanCanBoNgan`): the same task must read "trễ 6 ngày" here and on
 * `/nhiem-vu`, and two copies of the arithmetic would one day disagree by a day.
 *
 * A ROW IS A LINK to `/nhiem-vu?task=<code>` (`task-link.ts`), which opens the register's own detail.
 * The detail is not rebuilt here — see `task-link.ts`.
 */

/** The text of a row, in parts — the late part is drawn red, so it is not pre-joined. */
export type TaskRowText = {
  readonly title: string;
  /** `{assignee} · hạn {d/M/yyyy}` — or `Chưa phân công · hạn —`. */
  readonly meta: string;
  /** `trễ 6 ngày` · `trễ dưới 1 ngày`, or `""` when not late or without a deadline. */
  readonly late: string;
  /** `việc con của NV19`, or `""` for a top-level task. */
  readonly childOf: string;
};

export function taskRowText(task: petitions_nhiemVuRa, directory: DanhBaTheoMa | null, now: Date): TaskRowText {
  const due = tinhTrangHan(task.due_at, now);
  const dueDate = due.loai === "khong-han" ? O_TRONG : nhanNgay(task.due_at);
  return {
    title: task.title,
    meta: `${nhanCanBoNgan(task.assignee, directory, CHUA_PHAN_CONG)} · hạn ${dueDate}`,
    late: due.loai === "tre" ? lateText(due.soNgay) : "",
    childOf: task.parent === "" ? "" : childOfText(task.parent),
  };
}

const ROW_LINK =
  "flex flex-col gap-1 px-4 py-3 text-inherit no-underline hover:bg-surface-muted " +
  "focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500";

export function TaskMiniList({
  tasks,
  directory,
  statusLabels,
  now,
  label,
}: {
  tasks: readonly petitions_nhiemVuRa[];
  /** Staff directory by code; `null` = not loaded — rows then show the code. */
  directory: DanhBaTheoMa | null;
  /** The commune's status labels — a `tam-dung` row shows its pill in the commune's own word. */
  statusLabels: BangNhanTrangThai;
  now: Date;
  /** Accessible name of the list (the column or group title). */
  label: string;
}) {
  return (
    <ul aria-label={label} className="m-0 divide-y divide-line p-0 [list-style:none]">
      {tasks.map((task) => {
        const row = taskRowText(task, directory, now);
        return (
          <li key={task.code} className="task-mini-row">
            <Link href={taskDetailHref(task.code)} className={ROW_LINK}>
              <span className="line-clamp-2 text-sm font-medium text-ink-900">{row.title}</span>
              <span className="text-xs text-ink-500">
                {row.meta}
                {row.late !== "" && (
                  <>
                    {" · "}
                    <span className="font-medium text-danger-600">{row.late}</span>
                  </>
                )}
              </span>
              {row.childOf !== "" && <span className="text-xs text-ink-500">{row.childOf}</span>}
              {task.status === "tam-dung" && (
                <span>
                  <TaskStatusBadge status={task.status}>{nhanTrangThai(statusLabels, task.status)}</TaskStatusBadge>
                </span>
              )}
            </Link>
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
 * The `Duyệt lùi hạn` group: one row per pending request. Clicking opens the TASK — the register's
 * detail carries the request block with its `Duyệt` / `Từ chối` buttons (`task-extension-block.tsx`).
 */
export function ExtensionMiniList({
  requests,
  directory,
  label,
}: {
  requests: readonly petitions_deNghiChoDuyetRa[];
  directory: DanhBaTheoMa | null;
  label: string;
}) {
  return (
    <ul aria-label={label} className="m-0 divide-y divide-line p-0 [list-style:none]">
      {requests.map((r) => (
        <li key={r.id} className="extension-mini-row">
          <Link href={taskDetailHref(r.task_code)} className={ROW_LINK}>
            <span className="line-clamp-2 text-sm font-medium text-ink-900">{r.task_title}</span>
            <span className="text-xs text-ink-500">
              {nhanCanBoNgan(r.requested_by, directory, O_TRONG)} đề nghị · {extensionDeadlineText(r)}
            </span>
          </Link>
        </li>
      ))}
    </ul>
  );
}
