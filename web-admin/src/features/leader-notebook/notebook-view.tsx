import { Hourglass, LoaderCircle, TriangleAlert, UserCheck, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Badge, type BadgeTone } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { SkeletonRows } from "@/components/ui/skeleton";
import type { BangNhanTrangThai } from "@/features/nhiem-vu/nhan-nhiem-vu"; // vi-name-ok: existing type of the commune's status labels
import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing staff-directory lookup type
import type { petitions_deNghiChoDuyetRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract types
import { cn } from "@/lib/cn";

import {
  APPROVAL_EMPTY,
  APPROVAL_TITLE,
  ASSIGNED_EMPTY,
  ASSIGNED_TITLE,
  COMPLETION_GROUP_EMPTY,
  COMPLETION_GROUP_TITLE,
  COUNT_ERROR_PREFIX,
  EXTENSION_GROUP_EMPTY,
  EXTENSION_GROUP_TITLE,
  LOAD_ERROR_TITLE,
  LOADING_SENTENCE,
  MORE_LABEL,
  NO_FIGURE,
  OVERDUE_EMPTY,
  OVERDUE_TITLE,
} from "./notebook-queries";
import { ExtensionMiniList, TaskMiniList } from "./task-mini-list";

/**
 * The three columns of the Sổ tay lãnh đạo — DRAWING ONLY, no network (`leader-notebook.tsx` reads).
 * Split out so every state (loading, error, empty, hidden group) renders under `react-dom/server`.
 *
 * EMPTY IS NEVER AN ERROR, AND AN ERROR IS NEVER EMPTY. "Không có việc quá hạn." drawn because a read
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

export function NotebookView({ pastDue, completion, extensions, assigned, directory, statusLabels, now }: NotebookViewProps) {
  const approvalGroups = completion === null ? [extensions.count] : [completion.count, extensions.count];
  const groupsDone = [extensions.list.load, ...(completion === null ? [] : [completion.list.load])];
  const approvalAllEmpty = groupsDone.every((l) => l.phase === "done" && l.value.length === 0);

  return (
    <div className="leader-notebook grid min-w-0 grid-cols-1 gap-4 lg:grid-cols-3">
      <NotebookColumn
        id="notebook-overdue"
        icon={TriangleAlert}
        iconClass="bg-danger-50 text-danger-600"
        title={OVERDUE_TITLE}
        figure={figure(pastDue.count)}
        tone={pastDue.count.phase === "done" && pastDue.count.value > 0 ? "danger" : "neutral"}
        countError={pastDue.count.phase === "error" ? pastDue.count.message : null}
      >
        <ListBody
          state={pastDue.list}
          empty={<ColumnEmpty sentence={OVERDUE_EMPTY} />}
          render={(tasks) => (
            <TaskMiniList tasks={tasks} directory={directory} statusLabels={statusLabels} now={now} label={OVERDUE_TITLE} />
          )}
        />
      </NotebookColumn>

      <NotebookColumn
        id="notebook-approval"
        icon={Hourglass}
        iconClass="bg-warning-50 text-warning-600"
        title={APPROVAL_TITLE}
        figure={approvalFigure(approvalGroups)}
        tone="warning"
        countError={null}
      >
        {approvalAllEmpty ? (
          <ColumnEmpty sentence={APPROVAL_EMPTY} />
        ) : (
          <>
            {completion !== null && (
              <Group id="notebook-completion" title={COMPLETION_GROUP_TITLE} count={completion.count}>
                <ListBody
                  state={completion.list}
                  empty={<GroupEmpty sentence={COMPLETION_GROUP_EMPTY} />}
                  render={(tasks) => (
                    <TaskMiniList
                      tasks={tasks}
                      directory={directory}
                      statusLabels={statusLabels}
                      now={now}
                      label={COMPLETION_GROUP_TITLE}
                    />
                  )}
                />
              </Group>
            )}
            <Group id="notebook-extensions" title={EXTENSION_GROUP_TITLE} count={extensions.count}>
              <ListBody
                state={extensions.list}
                empty={<GroupEmpty sentence={EXTENSION_GROUP_EMPTY} />}
                render={(requests) => (
                  <ExtensionMiniList requests={requests} directory={directory} label={EXTENSION_GROUP_TITLE} />
                )}
              />
            </Group>
          </>
        )}
      </NotebookColumn>

      <NotebookColumn
        id="notebook-assigned"
        icon={UserCheck}
        iconClass="bg-brand-50 text-brand-600"
        title={ASSIGNED_TITLE}
        figure={figure(assigned.count)}
        tone="info"
        countError={assigned.count.phase === "error" ? assigned.count.message : null}
      >
        <ListBody
          state={assigned.list}
          empty={<ColumnEmpty sentence={ASSIGNED_EMPTY} />}
          render={(tasks) => (
            <TaskMiniList tasks={tasks} directory={directory} statusLabels={statusLabels} now={now} label={ASSIGNED_TITLE} />
          )}
        />
      </NotebookColumn>
    </div>
  );
}

/** A card of fixed height (~70vh) with its own scroll; single column under 1024px (spec 03 §2). */
function NotebookColumn({
  id,
  icon: Icon,
  iconClass,
  title,
  figure: shown,
  tone,
  countError,
  children,
}: {
  id: string;
  icon: LucideIcon;
  iconClass: string;
  title: string;
  figure: string;
  tone: BadgeTone;
  countError: string | null;
  children: ReactNode;
}) {
  return (
    <Card as="section" aria-labelledby={id} className="flex max-h-[70vh] flex-col lg:h-[70vh]">
      <CardHeader className="shrink-0 flex-nowrap">
        <span aria-hidden="true" className={cn("grid size-9 shrink-0 place-items-center rounded-lg", iconClass)}>
          <Icon className="size-[18px]" strokeWidth={1.8} focusable="false" />
        </span>
        <CardTitle id={id} className="min-w-0 flex-1">
          {title}
        </CardTitle>
        <Badge tone={tone} className="notebook-count">
          {shown}
        </Badge>
      </CardHeader>
      {countError !== null && (
        <p className="thong-bao-loi m-0 shrink-0 px-4 pt-3" role="alert">
          {COUNT_ERROR_PREFIX} {countError}
        </p>
      )}
      <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
    </Card>
  );
}

/** A labelled group inside column 2, with its own server count. */
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

function ColumnEmpty({ sentence }: { sentence: string }) {
  return <EmptyState tone="neutral" title={sentence} role="status" className="py-8" />;
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
