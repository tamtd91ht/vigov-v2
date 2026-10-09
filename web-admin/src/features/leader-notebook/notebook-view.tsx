import { CircleCheck, LoaderCircle, TriangleAlert, UserCheck, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { SkeletonRows } from "@/components/ui/skeleton";
import type { BangNhanTrangThai } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing type of the status labels
import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing staff-directory lookup type
import type { petitions_deNghiChoDuyetRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract types
import { cn } from "@/lib/cn";

import {
  APPROVAL_TITLE,
  ASSIGNED_TITLE,
  COLUMN_EMPTY,
  COMPLETION_GROUP_EMPTY,
  COMPLETION_GROUP_TITLE,
  COUNT_ERROR_PREFIX,
  EXTENSION_GROUP_EMPTY,
  EXTENSION_GROUP_TITLE,
  LOAD_ERROR_TITLE,
  LOADING_SENTENCE,
  MORE_LABEL,
  NO_FIGURE,
  OVERDUE_TITLE,
} from "./notebook-queries";
import { ExtensionMiniList, TaskMiniList } from "./task-mini-list";

/**
 * The three columns of the Sổ tay lãnh đạo — DRAWING ONLY, no network (`leader-notebook.tsx` reads
 * and owns the drawer a row opens). Split out so every state (loading, error, empty, hidden group)
 * renders under `react-dom/server`.
 *
 * EMPTY IS NEVER AN ERROR, AND AN ERROR IS NEVER EMPTY. "Không có việc nào." drawn because a read
 * failed is a leader believing the commune is on time. A failed read shows the server's sentence,
 * verbatim, with `Tải lại`.
 */

export type Loaded<T> =
  | { readonly phase: "loading" }
  | { readonly phase: "error"; readonly message: string }
  | { readonly phase: "done"; readonly value: T };

/** One paged list as the screen holds it. `hasMore` already means "has_more AND a cursor to follow". */
export type ListState<T> = {
  readonly load: Loaded<readonly T[]>;
  readonly hasMore: boolean;
  readonly loadingMore: boolean;
  readonly moreError: string | null;
  readonly onRetry: () => void;
  readonly onMore: () => void;
};

export type Section<T> = { readonly list: ListState<T>; readonly count: Loaded<number> };

export type NotebookViewProps = {
  readonly pastDue: Section<petitions_nhiemVuRa>;
  /** `null` = the session does not hold `task.approve`: the group is NOT drawn (not an empty state). */
  readonly completion: Section<petitions_nhiemVuRa> | null;
  readonly extensions: Section<petitions_deNghiChoDuyetRa>;
  readonly assigned: Section<petitions_nhiemVuRa>;
  readonly directory: DanhBaTheoMa | null;
  readonly statusLabels: BangNhanTrangThai;
  readonly now: Date;
  /** A task row was pressed: open its drawer on this page. */
  readonly onOpenTask: (task: petitions_nhiemVuRa) => void;
  /** An extension-request row was pressed: open its TASK's drawer. */
  readonly onOpenRequest: (request: petitions_deNghiChoDuyetRa) => void;
};

/** The figure for a badge: the server's count, or `—` while it is unknown. Never a row count. */
export function figure(count: Loaded<number>): string {
  return count.phase === "done" ? String(count.value) : NO_FIGURE;
}

/** The column-2 figure: the sum of the VISIBLE groups' server counts, once every one is known. */
export function approvalFigure(groups: readonly Loaded<number>[]): string {
  let total = 0;
  for (const g of groups) {
    if (g.phase !== "done") return NO_FIGURE;
    total += g.value;
  }
  return String(total);
}

/**
 * The prototype's grid (`LeaderNotebook.tsx:74`): three columns, ONE column at 1280px and below.
 * `items-start` (spec 03 A5, owner 09/10/2026): each column is as tall as its content, so a short
 * column does not stretch an empty white body down to the longest one.
 */
export const NOTEBOOK_GRID_CLASS = "leader-notebook grid min-w-0 grid-cols-3 items-start gap-4 max-[1280px]:grid-cols-1";

export function NotebookView({
  pastDue,
  completion,
  extensions,
  assigned,
  directory,
  statusLabels,
  now,
  onOpenTask,
  onOpenRequest,
}: NotebookViewProps) {
  const approvalGroups = completion === null ? [extensions.count] : [completion.count, extensions.count];
  const groupsDone = [extensions.list.load, ...(completion === null ? [] : [completion.list.load])];
  const approvalAllEmpty = groupsDone.every((l) => l.phase === "done" && l.value.length === 0);
  function taskList(title: string) {
    return function renderTasks(tasks: readonly petitions_nhiemVuRa[]) {
      return <TaskMiniList tasks={tasks} directory={directory} statusLabels={statusLabels} now={now} label={title} onOpen={onOpenTask} />;
    };
  }

  return (
    <div className={NOTEBOOK_GRID_CLASS}>
      <NotebookColumn
        id="notebook-overdue"
        icon={TriangleAlert}
        tone={TONE_DANGER}
        title={OVERDUE_TITLE}
        figure={figure(pastDue.count)}
        countError={pastDue.count.phase === "error" ? pastDue.count.message : null}
      >
        <ListBody state={pastDue.list} empty={<ColumnEmpty />} render={taskList(OVERDUE_TITLE)} />
      </NotebookColumn>

      <NotebookColumn
        id="notebook-approval"
        icon={CircleCheck}
        tone={TONE_VIOLET}
        title={APPROVAL_TITLE}
        figure={approvalFigure(approvalGroups)}
        countError={null}
      >
        {approvalAllEmpty ? (
          <ColumnEmpty />
        ) : (
          <>
            {completion !== null && (
              <Group id="notebook-completion" title={COMPLETION_GROUP_TITLE} count={completion.count}>
                <ListBody
                  state={completion.list}
                  empty={<GroupEmpty sentence={COMPLETION_GROUP_EMPTY} />}
                  render={taskList(COMPLETION_GROUP_TITLE)}
                />
              </Group>
            )}
            <Group id="notebook-extensions" title={EXTENSION_GROUP_TITLE} count={extensions.count}>
              <ListBody
                state={extensions.list}
                empty={<GroupEmpty sentence={EXTENSION_GROUP_EMPTY} />}
                render={(requests) => (
                  <ExtensionMiniList requests={requests} directory={directory} label={EXTENSION_GROUP_TITLE} onOpen={onOpenRequest} />
                )}
              />
            </Group>
          </>
        )}
      </NotebookColumn>

      <NotebookColumn
        id="notebook-assigned"
        icon={UserCheck}
        tone={TONE_BRAND}
        title={ASSIGNED_TITLE}
        figure={figure(assigned.count)}
        countError={assigned.count.phase === "error" ? assigned.count.message : null}
      >
        <ListBody state={assigned.list} empty={<ColumnEmpty />} render={taskList(ASSIGNED_TITLE)} />
      </NotebookColumn>
    </div>
  );
}

/* The prototype's tile tones (`LeaderNotebook.tsx:136-140`): danger · violet · brand. */
export const TONE_DANGER = "text-danger bg-danger/10 border-danger/25";
export const TONE_VIOLET = "text-violet bg-violet/10 border-violet/25";
export const TONE_BRAND = "text-brand bg-brand/10 border-brand/25";

/**
 * One column, the prototype's `NotebookColumn` (`LeaderNotebook.tsx:142-190`): a white card, a header
 * row `[tile] title ……… [count]` over a rule, then a body that scrolls past 560px. Drawn here rather
 * than through the shared `Card`: the prototype's frame (border, shadow) and its 14px navy title are
 * this screen's, and the shared component stays as every other screen has it. `m-0` where a heading or
 * paragraph would keep the browser's margins (no preflight).
 */
function NotebookColumn({
  id,
  icon: Icon,
  tone,
  title,
  figure: shown,
  countError,
  children,
}: {
  id: string;
  icon: LucideIcon;
  tone: string;
  title: string;
  figure: string;
  countError: string | null;
  children: ReactNode;
}) {
  return (
    <section aria-labelledby={id} className="border-line shadow-card rounded-card min-w-0 border bg-white">
      <header className="border-line flex items-center gap-2.5 border-b px-4 py-3">
        <span aria-hidden="true" className={cn("grid size-8 shrink-0 place-items-center rounded-[9px] border", tone)}>
          <Icon className="size-4" focusable="false" />
        </span>
        <h2 id={id} className="text-navy m-0 min-w-0 text-[14px] font-bold">
          {title}
        </h2>
        <span className="notebook-count border-line text-ink-muted ml-auto shrink-0 rounded-[10px] border px-2 py-0.5 text-[12px] font-semibold">
          {shown}
        </span>
      </header>
      {countError !== null && (
        <p className="thong-bao-loi m-0 px-4 pt-3" role="alert">
          {COUNT_ERROR_PREFIX} {countError}
        </p>
      )}
      <div className="max-h-[560px] overflow-y-auto">{children}</div>
    </section>
  );
}

/** A labelled group inside column 2, with its own server count (ADR 0071 — not in the prototype). */
function Group({ id, title, count, children }: { id: string; title: string; count: Loaded<number>; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="notebook-group border-b border-line last:border-b-0">
      <div className="flex items-center gap-2 bg-surface-muted px-4 py-2">
        <h3 id={id} className="m-0 flex-1 text-[13px] font-semibold text-ink-700">
          {title}
        </h3>
        <span className="notebook-group-count text-xs font-semibold text-ink-700">{figure(count)}</span>
      </div>
      {count.phase === "error" && (
        <p className="thong-bao-loi m-0 px-4 pt-2" role="alert">
          {COUNT_ERROR_PREFIX} {count.message}
        </p>
      )}
      {children}
    </section>
  );
}

/** The prototype's empty column: one centred muted line, no illustration. */
function ColumnEmpty() {
  return (
    <p className="column-empty text-ink-muted m-0 px-4 py-8 text-center text-[12.5px]" role="status">
      {COLUMN_EMPTY}
    </p>
  );
}

function GroupEmpty({ sentence }: { sentence: string }) {
  return <p className="group-empty m-0 px-4 py-3 text-[13px] text-ink-500">{sentence}</p>;
}

function ListBody<T>({
  state,
  empty,
  render,
}: {
  state: ListState<T>;
  empty: ReactNode;
  render: (items: readonly T[]) => ReactNode;
}) {
  const { load } = state;
  if (load.phase === "loading") {
    return (
      <div aria-busy="true">
        <p className="an-thi-giac" role="status">
          {LOADING_SENTENCE}
        </p>
        <SkeletonRows rows={4} columns={2} />
      </div>
    );
  }
  if (load.phase === "error") {
    return <ErrorState role="alert" title={LOAD_ERROR_TITLE} message={load.message} onRetry={state.onRetry} className="py-8" />;
  }
  if (load.value.length === 0) return <>{empty}</>;
  return (
    <>
      {render(load.value)}
      {state.moreError !== null && (
        <p className="thong-bao-loi m-0 px-4 py-2" role="alert">
          {state.moreError}
        </p>
      )}
      {state.hasMore && (
        <div className="px-4 py-3">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={state.loadingMore}
            onClick={state.onMore}
            icon={state.loadingMore ? <LoaderCircle aria-hidden="true" focusable="false" className="motion-safe:animate-spin" /> : undefined}
          >
            {MORE_LABEL}
          </Button>
        </div>
      )}
    </>
  );
}
