"use client";

import { CircleCheck, Send, TriangleAlert } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba"; // vi-name-ok: existing idempotency-key helper (rule 12 invariant 3)
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { Skeleton } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing shared result type of goi.ts (rule 12 invariant 3)
import { getProjectIssues, recordProjectIssue, resolveProjectIssue } from "@/lib/api/project-discussion";
import type { finance_projectIssueOut, finance_projectIssuesOut, identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { nhanMocKhoa } from "./nhan-ghi-giai-ngan";
import { TrackingTaskPending } from "./pending-parts";
import { ISSUE_PLACEHOLDER, ISSUES_DENIED, staffLabel } from "./project-discussion-labels";
import type { PeopleCatalogue } from "./project-people";
import { DeniedNote, Glyph } from "./project-ui";

/**
 * §8.1 "Vướng mắc" — the prototype's one-box form (`IssueForm.tsx`) and its timeline
 * (`BudgetItemDetail.tsx:521-601`): who · when · `Đã gỡ` chip · the text, struck through once resolved,
 * and `Đã gỡ xong` on the open ones.
 *
 * THE TIMELINE IS THE SERVER'S LIST, RE-READ AFTER EVERY WRITE — never a row patched in. The tab's
 * `(N)` and the timeline come from the same read, so they cannot disagree.
 *
 * `budget.update` RECORDS AND RESOLVES; `budget.read` sees the timeline. Hiding the form is UX only:
 * both routes check their key on every call (rule 5, forbidden #1).
 *
 * A RESOLVED ISSUE STAYS, greyed (prototype `:537-541`): the answer to "why is this project late" is
 * in what was unblocked, not only in what is blocked. Resolution is one-way — no reopen control.
 */

/** The project's issues as the page holds them. `openCount` is the server's, not a count of `items`. */
export type ProjectIssueList =
  | { readonly phase: "loading" }
  | { readonly phase: "error"; readonly message: string }
  | { readonly phase: "ready"; readonly items: readonly finance_projectIssueOut[]; readonly openCount: number };

/** Reads the timeline once per `reloadKey`; "loading" is DERIVED from the stored key (`useProjectVouchers`). */
export function useProjectIssues(projectId: string, reloadKey: string): ProjectIssueList {
  const key = `${projectId}|${reloadKey}`;
  const [loaded, setLoaded] = useState<{ key: string; result: KetQua<finance_projectIssuesOut> } | null>(null);

  useEffect(() => {
    let cancelled = false;
    getProjectIssues(projectId).then((result) => {
      if (!cancelled) setLoaded({ key, result });
    });
    return () => {
      cancelled = true;
    };
  }, [projectId, key]);

  if (loaded === null || loaded.key !== key) return { phase: "loading" };
  if (!loaded.result.ok) return { phase: "error", message: loaded.result.thongBao };
  return { phase: "ready", items: loaded.result.duLieu.items, openCount: loaded.result.duLieu.open_count };
}

export function ProjectIssuesPanel({
  projectId,
  issues,
  staff,
  canRecord,
  onChanged,
}: {
  projectId: string;
  issues: ProjectIssueList;
  /** The staff directory (`useProjectPeople().staff`): codes → names. */
  staff: PeopleCatalogue<identity_canBoChonNguoiRa>;
  /** `budget.update` held: the form and the `Đã gỡ xong` buttons. */
  canRecord: boolean;
  /** After a write, and as the list's retry: the page re-reads the timeline. */
  onChanged: () => void;
}) {
  const [resolving, setResolving] = useState<string | null>(null);
  const [resolveError, setResolveError] = useState<string | null>(null);

  function resolve(issue: finance_projectIssueOut): void {
    setResolving(issue.id);
    setResolveError(null);
    resolveProjectIssue(issue.id).then((r) => {
      setResolving(null);
      // The server's sentence verbatim — its 409 says the issue was already resolved and what to do.
      if (!r.ok) setResolveError(r.thongBao);
      // Re-read on failure too: a 409 means somebody else resolved it, and the timeline should say so.
      onChanged();
    });
  }

  return (
    <div className="flex min-w-0 flex-col gap-3" data-issues-panel="">
      {canRecord ? <IssueForm projectId={projectId} onRecorded={onChanged} /> : <DeniedNote>{ISSUES_DENIED}</DeniedNote>}

      {resolveError !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {resolveError}
        </p>
      )}

      {issues.phase === "loading" && (
        <div>
          <p role="status" className="an-thi-giac">
            Đang tải vướng mắc…
          </p>
          <div aria-hidden="true" className="flex flex-col gap-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        </div>
      )}

      {/* A failed read is NOT an empty timeline: "chưa có vướng mắc" would be a claim with no ground. */}
      {issues.phase === "error" && (
        <ErrorState
          title="Chưa tải được vướng mắc của dự án"
          message={<span role="alert">{issues.message}</span>}
          onRetry={onChanged}
        />
      )}

      {issues.phase === "ready" && (
        <IssueTimeline
          items={issues.items}
          staff={staff}
          canRecord={canRecord}
          resolving={resolving}
          onResolve={resolve}
        />
      )}
    </div>
  );
}

/** The one-box form: first line is the title, the rest the description — split by the SERVER. */
function IssueForm({ projectId, onRecorded }: { projectId: string; onRecorded: () => void }) {
  const [text, setText] = useState("");
  // Minted when the form opens and kept across a failed send: a retry must reuse it (`recordProjectIssue`).
  const [idempotencyKey, setIdempotencyKey] = useState(khoaChongTrungMoi);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const empty = text.trim() === "";

  function submit(e: FormEvent): void {
    e.preventDefault();
    if (empty || busy) return;
    setBusy(true);
    recordProjectIssue(projectId, text, idempotencyKey).then((r) => {
      setBusy(false);
      if (!r.ok) {
        setError(r.thongBao);
        return;
      }
      setError(null);
      setText("");
      setIdempotencyKey(khoaChongTrungMoi());
      onRecorded();
    });
  }

  return (
    <form onSubmit={submit} aria-label="Ghi nhận vướng mắc" className="flex min-w-0 flex-col gap-2">
      <label htmlFor="vuong-mac-moi" className="an-thi-giac">
        Vướng mắc mới
      </label>
      <textarea
        id="vuong-mac-moi"
        name="vuong-mac-moi"
        rows={2}
        value={text}
        placeholder={ISSUE_PLACEHOLDER}
        className={cn(controlClass, "h-auto min-h-16 w-full min-w-0 resize-y py-2")}
        onChange={(e) => setText(e.target.value)}
      />
      {error !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {error}
        </p>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <span className="m-0 text-xs text-ink-500">Dòng đầu là tiêu đề, các dòng sau là diễn giải.</span>
        <TrackingTaskPending />
        <Button
          type="submit"
          variant="primary"
          className="ml-auto"
          icon={busy ? undefined : <Glyph icon={Send} />}
          disabled={empty || busy}
          aria-busy={busy || undefined}
        >
          <BusyLabel busy={busy} label="Ghi nhận" busyText={BUSY_SAVING} />
        </Button>
      </div>
    </form>
  );
}

/** Newest first, as the server orders it. Presentational: renders in tests without the network. */
export function IssueTimeline({
  items,
  staff,
  canRecord,
  resolving,
  onResolve,
}: {
  items: readonly finance_projectIssueOut[];
  staff: PeopleCatalogue<identity_canBoChonNguoiRa>;
  canRecord: boolean;
  /** Id of the issue whose resolution is in flight: every `Đã gỡ xong` waits for it. */
  resolving: string | null;
  onResolve: (issue: finance_projectIssueOut) => void;
}) {
  if (items.length === 0) {
    return <EmptyState icon={TriangleAlert} title="Chưa ghi nhận vướng mắc nào ở dự án này." />;
  }
  return (
    <ol className="m-0 flex list-none flex-col gap-3 p-0" aria-label="Dòng thời gian vướng mắc">
      {items.map((issue) => (
        <li
          key={issue.id}
          data-issue={issue.id}
          data-resolved={issue.resolved ? "" : undefined}
          className={cn(
            "rounded-xl border border-line px-3 py-2.5",
            issue.resolved ? "bg-surface-subtle opacity-75" : "bg-surface",
          )}
        >
          <div className="mb-1 flex flex-wrap items-baseline gap-x-2 gap-y-1">
            <b className="text-[13px] text-ink-900">{staffLabel(issue.recorded_by, staff)}</b>
            <time dateTime={issue.recorded_at} className="text-xs text-ink-500 tabular-nums">
              {nhanMocKhoa(issue.recorded_at)}
            </time>
            {issue.resolved && (
              <Badge tone="success" icon={CircleCheck}>
                Đã gỡ
              </Badge>
            )}
          </div>
          <p className={cn("m-0 text-sm whitespace-pre-line text-ink-900", issue.resolved && "line-through")}>
            {issue.title}
          </p>
          {issue.description !== undefined && issue.description !== "" && (
            <p
              className={cn(
                "m-0 mt-1 text-[13px] whitespace-pre-line text-ink-500",
                issue.resolved && "line-through",
              )}
            >
              {issue.description}
            </p>
          )}
          {issue.resolved && (
            <p className="m-0 mt-1 text-xs text-ink-500" data-resolved-by="">
              Gỡ lúc {nhanMocKhoa(issue.resolved_at)}
              {issue.resolved_by !== undefined && issue.resolved_by !== "" && ` · ${staffLabel(issue.resolved_by, staff)}`}
            </p>
          )}
          {canRecord && !issue.resolved && (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="mt-2"
              icon={<Glyph icon={CircleCheck} />}
              disabled={resolving !== null}
              aria-busy={resolving === issue.id || undefined}
              onClick={() => onResolve(issue)}
            >
              Đã gỡ xong
            </Button>
          )}
        </li>
      ))}
    </ol>
  );
}
