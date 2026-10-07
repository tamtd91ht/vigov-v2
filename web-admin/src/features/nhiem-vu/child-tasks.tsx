"use client";

import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { PendingMarker } from "@/components/ui/pending-feature";
import type { KetQua } from "@/lib/api/goi";
import { laySoNhiemVu } from "@/lib/api/nhiem-vu";
import type { LocNhiemVu } from "@/lib/api/nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import type { page_Result_petitions_nhiemVuRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  CHILD_TASK_WEIGHT_NONE,
  CHILD_TASK_WEIGHT_PENDING,
  CHILD_TASKS_TITLE,
  childTasksHeading,
  mergeChildPages,
  NHAN_XEM_THEM_NHAT_KY_NHIEM_VU,
} from "./nhan-nhiem-vu";
import { CHECKBOX_CLASS, SECTION_CLASS, SECTION_TITLE_CLASS } from "./task-spec";

/**
 * §5.10 — the drawer's `Nhiệm vụ con` block (ADR 0037, ad7f821), drawn as spec 07 §6d: a card titled
 * `Nhiệm vụ con (k/n hoàn thành)`, one bordered row per child — the completion tick and the weight at
 * the prototype's place, both a DISABLED "?" control (ADR 0076 #2: no weight is stored, and completing
 * a child is a status move made in its own detail). The title opens the child's detail.
 *
 * DRAWN ONLY WHEN THERE ARE CHILDREN (spec 07 §6d): an empty card says nothing. `+ Thêm việc con`
 * (kept, owner 07/10/2026 #6) is the drawer's own button under this block.
 *
 * THE LIST COMES FROM `GET /api/v1/tasks?parent=NV19`, the server's direct LIVE children — never
 * filtered out of the page the register happens to show (that page is sliced by the screen's filters).
 *
 * `Việc cha` (the parent field and its link) is GONE from the drawer (owner 07/10/2026 #6); the
 * `parent` support of `PATCH /api/v1/tasks/{ma}` stays in the API client.
 */

/** Rows per page. The server accepts 1–100. */
const CHILD_PAGE_SIZE = 20;

/**
 * The query of the children block — ONLY `parent`, never the register's filters. Exported so the
 * one decision that matters (no screen filter leaks in) is checked without a DOM.
 */
export function childTasksQuery(parentCode: string, cursor: string | null): LocNhiemVu {
  return { parent: parentCode, limit: CHILD_PAGE_SIZE, cursor };
}

type ChildPage =
  | {
      key: string;
      ok: true;
      rows: readonly petitions_nhiemVuRa[];
      cursor: string;
      more: boolean;
    }
  | { key: string; ok: false; message: string };

function toPage(key: string, result: KetQua<page_Result_petitions_nhiemVuRa>): ChildPage {
  return result.ok
    ? {
        key,
        ok: true,
        rows: result.duLieu.items,
        cursor: result.duLieu.next_cursor,
        more: result.duLieu.has_more,
      }
    : { key, ok: false, message: result.thongBao };
}

export function ChildTasks({
  parentCode,
  refreshKey,
  openTask,
}: {
  parentCode: string;
  /** Changes when the drawer re-reads (open, every write) — a child may have been added. */
  refreshKey: number;
  /** Open the child's drawer. The row is a register row (no `documents`) — `chuyenDrawer` knows. */
  openTask: (task: petitions_nhiemVuRa) => void;
}) {
  const [page, setPage] = useState<ChildPage | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreError, setMoreError] = useState<{ key: string; message: string } | null>(null);
  const key = `${parentCode}|${refreshKey}`;

  useEffect(() => {
    let dropped = false;
    laySoNhiemVu(childTasksQuery(parentCode, null)).then((result) => {
      if (!dropped) setPage(toPage(key, result));
    });
    return () => {
      dropped = true;
    };
  }, [parentCode, key]);

  // An answer for another parent (or from before the last write) is never drawn as this one's.
  const current = page !== null && page.key === key ? page : null;

  function loadMore(): void {
    if (current === null || !current.ok || !current.more || current.cursor === "") return;
    const before = current;
    setLoadingMore(true);
    laySoNhiemVu(childTasksQuery(parentCode, before.cursor)).then((result) => {
      setLoadingMore(false);
      if (!result.ok) {
        setMoreError({ key: before.key, message: result.thongBao });
        return;
      }
      setMoreError(null);
      setPage((p) =>
        p === null || p.key !== before.key || !p.ok
          ? p
          : {
              ...p,
              rows: mergeChildPages(p.rows, result.duLieu.items),
              cursor: result.duLieu.next_cursor,
              more: result.duLieu.has_more,
            },
      );
    });
  }

  return (
    <ChildTaskList
      load={
        current === null
          ? { phase: "loading" }
          : current.ok
            ? { phase: "done", rows: current.rows, more: current.more && current.cursor !== "" }
            : { phase: "error", message: current.message }
      }
      openTask={openTask}
      loadingMore={loadingMore}
      moreError={moreError !== null && moreError.key === key ? moreError.message : null}
      loadMore={loadMore}
    />
  );
}

/** Accessible name of a child's (disabled) completion tick — which child, among twenty. */
export function childTickLabel(code: string): string {
  return `Đã hoàn thành ${code}`;
}

export type ChildTasksLoad =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "done"; rows: readonly petitions_nhiemVuRa[]; more: boolean };

/**
 * Rendering only — exported for `renderToStaticMarkup`.
 *
 * A FAILED READ IS NOT DRAWN AS "NO CHILDREN": that would tell a clerk a parent can be completed or
 * deleted, and the server would then refuse with the list of children. The server's sentence shows
 * verbatim, `role="alert"`, in the card. Loading or no children → nothing at all.
 */
export function ChildTaskList({
  load,
  openTask,
  loadingMore,
  moreError,
  loadMore,
}: {
  load: ChildTasksLoad;
  openTask: (task: petitions_nhiemVuRa) => void;
  loadingMore: boolean;
  moreError: string | null;
  loadMore: () => void;
}) {
  if (load.phase === "loading") return null;
  if (load.phase === "done" && load.rows.length === 0) return null;
  // The prototype's `(k/n hoàn thành)` only when every page is in hand: a count over the first page
  // of a longer list is a figure nobody can check.
  const heading =
    load.phase === "done" && !load.more
      ? childTasksHeading(load.rows.filter((t) => t.status === "hoan-thanh").length, load.rows.length)
      : CHILD_TASKS_TITLE;
  return (
    <section className={SECTION_CLASS} aria-labelledby="tieu-de-viec-con">
      <h3 id="tieu-de-viec-con" className={cn(SECTION_TITLE_CLASS, "flex items-center gap-1.5")}>
        {heading}
        {/* ONE "?" for the tick and the weight of every row — not one per row (ADR 0068 §14). */}
        {load.phase === "done" && <PendingMarker info={CHILD_TASK_WEIGHT_PENDING} />}
      </h3>
      {load.phase === "error" && (
        <p className="thong-bao-loi m-0" role="alert">
          {load.message}
        </p>
      )}
      {load.phase === "done" && (
        <ul className="m-0 list-none p-0">
          {load.rows.map((t) => {
            const done = t.status === "hoan-thanh";
            return (
              <li
                key={t.code}
                className="border-line mb-2 flex items-start gap-2.5 rounded-[9px] border bg-white px-3 py-2.5"
              >
                {/* The prototype's completion tick — DISABLED (`CHILD_TASK_WEIGHT_PENDING`). */}
                <input
                  type="checkbox"
                  className={cn(CHECKBOX_CLASS, "mt-0.5")}
                  disabled
                  readOnly
                  checked={done}
                  aria-label={childTickLabel(t.code)}
                />
                <span className="min-w-0 text-[12.5px]">
                  <button
                    type="button"
                    className={cn(
                      "cursor-pointer border-0 bg-transparent p-0 text-left text-[12.5px] [font-family:inherit] hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
                      done ? "text-ink-muted line-through" : "text-ink",
                    )}
                    aria-label={`Mở ${t.code}: ${t.title}`}
                    onClick={() => openTask(t)}
                  >
                    {t.title}
                  </button>
                  <span className="text-ink-muted ml-1.5 text-[11px]">({CHILD_TASK_WEIGHT_NONE})</span>
                </span>
              </li>
            );
          })}
        </ul>
      )}
      {load.phase === "done" && moreError !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {moreError}
        </p>
      )}
      {load.phase === "done" && load.more && (
        <Button type="button" variant="outline" size="sm" disabled={loadingMore} onClick={loadMore}>
          {NHAN_XEM_THEM_NHAT_KY_NHIEM_VU}
        </Button>
      )}
    </section>
  );
}
