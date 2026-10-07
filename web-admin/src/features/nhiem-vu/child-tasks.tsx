"use client";

import { useEffect, useState, type FormEvent } from "react";

import { PendingMarker } from "@/components/ui/pending-feature";

import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import type { KetQua } from "@/lib/api/goi";
import { laySoNhiemVu } from "@/lib/api/nhiem-vu";
import type { LocNhiemVu } from "@/lib/api/nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import type {
  page_Result_petitions_nhiemVuRa,
  petitions_nhiemVuRa,
  petitions_suaNhiemVuVao,
} from "@/lib/api/schema.gen";

import {
  CHILD_TASK_WEIGHT_NONE,
  CHILD_TASK_WEIGHT_PENDING,
  CHILD_TASKS_EMPTY,
  CHILD_TASKS_LOADING,
  CHILD_TASKS_TITLE,
  childTasksHeading,
  CHUA_PHAN_CONG,
  DETACH_PARENT_BODY,
  NHAN_XEM_THEM_NHAT_KY_NHIEM_VU,
  PARENT_DETACH_BUTTON,
  PARENT_INPUT_LABEL,
  PARENT_NONE,
  PARENT_SAVE_BUTTON,
  PARENT_TITLE,
  mergeChildPages,
  nhanCanBoNgan,
  nhanTrangThai,
  oHan,
  parentPatchBody,
} from "./nhan-nhiem-vu";
import type { BangNhanTrangThai } from "./nhan-nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)

/**
 * §5.10 — the drawer's `Nhiệm vụ con` block and the `Việc cha` field (ADR 0037, ad7f821).
 *
 * THE LIST COMES FROM `GET /api/v1/tasks?parent=NV19`, the server's direct LIVE children. It is
 * never filtered out of the page the register happens to show: that page is sliced by the
 * screen's filters, and a child outside the slice would simply be missing from its parent.
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
  labels,
  directory,
  now,
  openTask,
}: {
  parentCode: string;
  /** Changes when the drawer re-reads (open, every write) — a child may have been added. */
  refreshKey: number;
  labels: BangNhanTrangThai;
  directory: DanhBaTheoMa | null;
  now: Date;
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
      labels={labels}
      directory={directory}
      now={now}
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
 * A FAILED READ IS NOT DRAWN AS "NO CHILDREN": that sentence would tell a clerk a parent can be
 * completed or deleted, and the server would then refuse with the list of children. The server's
 * sentence shows verbatim, `role="alert"`.
 */
export function ChildTaskList({
  load,
  labels,
  directory,
  now,
  openTask,
  loadingMore,
  moreError,
  loadMore,
}: {
  load: ChildTasksLoad;
  labels: BangNhanTrangThai;
  directory: DanhBaTheoMa | null;
  now: Date;
  openTask: (task: petitions_nhiemVuRa) => void;
  loadingMore: boolean;
  moreError: string | null;
  loadMore: () => void;
}) {
  // The prototype's `(k/n hoàn thành)` only when every page is in hand: a count over the first page
  // of a longer list is a figure nobody can check.
  const heading =
    load.phase === "done" && load.rows.length > 0 && !load.more
      ? childTasksHeading(load.rows.filter((t) => t.status === "hoan-thanh").length, load.rows.length)
      : CHILD_TASKS_TITLE;
  return (
    <div className="form-danh-muc" aria-labelledby="tieu-de-viec-con">
      <div className="flex items-center gap-2">
        <h4 id="tieu-de-viec-con">{heading}</h4>
        {/* ONE "?" for the tick and the weight of every row — not one per row (ADR 0068 §14). */}
        {load.phase === "done" && load.rows.length > 0 && <PendingMarker info={CHILD_TASK_WEIGHT_PENDING} />}
      </div>
      {load.phase === "loading" && <p role="status">{CHILD_TASKS_LOADING}</p>}
      {load.phase === "error" && (
        <p className="thong-bao-loi" role="alert">
          {load.message}
        </p>
      )}
      {load.phase === "done" && load.rows.length === 0 && (
        <p className="trang-thai-rong">{CHILD_TASKS_EMPTY}</p>
      )}
      {load.phase === "done" && load.rows.length > 0 && (
        <ul className="danh-sach-viec-con">
          {load.rows.map((t) => {
            const due = oHan(t.due_at, now);
            return (
              <li key={t.code}>
                <div className="flex items-start gap-2.5">
                  {/* The prototype's completion tick — DISABLED: completing a child is a status move
                      of that child, made in its own detail (see `CHILD_TASK_WEIGHT_PENDING`). */}
                  <input
                    type="checkbox"
                    className="mt-2.5"
                    disabled
                    readOnly
                    checked={t.status === "hoan-thanh"}
                    aria-label={childTickLabel(t.code)}
                  />
                  <button
                    type="button"
                    className="nut-phu"
                    onClick={() => openTask(t)}
                    aria-label={`Mở ${t.code}: ${t.title}`}
                  >
                    <span className="ma-muc">{t.code}</span> {t.title}
                  </button>
                  <span className="mt-2 text-xs whitespace-nowrap text-ink-500">({CHILD_TASK_WEIGHT_NONE})</span>
                </div>
                <p className="dong-phu">
                  <span className="chip chip-ngung">{nhanTrangThai(labels, t.status)}</span>{" "}
                  {nhanCanBoNgan(t.assignee, directory, CHUA_PHAN_CONG)} · Hạn {due.ngay}
                  {due.phanTre !== "" && <span className="nhan-lech"> {due.phanTre}</span>}
                </p>
              </li>
            );
          })}
        </ul>
      )}
      {load.phase === "done" && moreError !== null && (
        <p className="thong-bao-loi" role="alert">
          {moreError}
        </p>
      )}
      {load.phase === "done" && load.more && (
        <button type="button" className="nut-phu" disabled={loadingMore} onClick={loadMore}>
          {NHAN_XEM_THEM_NHAT_KY_NHIEM_VU}
        </button>
      )}
    </div>
  );
}

/**
 * `Việc cha` — shown to every reader; the change controls only with `task.update` (the key of
 * `PATCH /api/v1/tasks/{ma}`). Hiding them is convenience, not the control: the server checks the
 * key and the whole tree rule (rule 5, forbidden #1).
 *
 * WHY ITS OWN SMALL FORM AND NOT A FIELD OF `✎ Sửa`: that form sits on the §5.4 block, which only
 * the `Theo văn bản` type has. A parent field there would be unreachable for every basic task.
 *
 * THE SERVER'S 409 `task_tree` SENTENCE IS THE WHOLE ANSWER — unknown, another commune's, deleted,
 * a cycle, too deep (ADR 0037). It shows verbatim; the box keeps what was typed.
 */
export function ParentTaskField({
  code,
  parent,
  canEdit,
  save,
  openParent,
}: {
  code: string;
  /** Register code of the parent, `""` for a root. */
  parent: string;
  canEdit: boolean;
  /** `PATCH /api/v1/tasks/{code}` with `{ parent }`. The caller refreshes the drawer on success. */
  save: (body: petitions_suaNhiemVuVao) => Promise<KetQua<petitions_nhiemVuRa>>;
  /** Open the parent's drawer; the answer's sentence shows here when it fails. */
  openParent: (code: string) => Promise<KetQua<unknown>>;
}) {
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const body = parentPatchBody(input, parent);
  const inputId = `viec-cha-moi-${code}`;

  function send(b: petitions_suaNhiemVuVao): void {
    setSending(true);
    setError(null);
    save(b).then((result) => {
      setSending(false);
      if (!result.ok) {
        setError(result.thongBao);
        return;
      }
      setInput("");
    });
  }

  function submit(e: FormEvent): void {
    e.preventDefault();
    if (body !== null && !sending) send(body);
  }

  return (
    <div className="form-danh-muc" aria-labelledby={`tieu-de-viec-cha-${code}`}>
      <h4 id={`tieu-de-viec-cha-${code}`}>{PARENT_TITLE}</h4>
      {parent === "" ? (
        <p className="ghi-chu">{PARENT_NONE}</p>
      ) : (
        <p>
          <button
            type="button"
            className="nut-phu"
            onClick={() => {
              setError(null);
              openParent(parent).then((result) => {
                if (!result.ok) setError(result.thongBao);
              });
            }}
          >
            Mở việc cha {parent}
          </button>
        </p>
      )}

      {canEdit && (
        <form onSubmit={submit}>
          <div className="o-nhap">
            <label htmlFor={inputId}>{PARENT_INPUT_LABEL}</label>
            <input
              id={inputId}
              name={inputId}
              value={input}
              autoComplete="off"
              maxLength={100}
              onChange={(e) => setInput(e.target.value)}
            />
          </div>
          <div className="cum-nut">
            <button type="submit" className="nut-phu" disabled={sending || body === null}>
              {PARENT_SAVE_BUTTON}
            </button>
            {parent !== "" && (
              <button
                type="button"
                className="nut-phu"
                disabled={sending}
                onClick={() => send(DETACH_PARENT_BODY)}
              >
                {PARENT_DETACH_BUTTON}
              </button>
            )}
          </div>
        </form>
      )}

      {error !== null && (
        <p className="thong-bao-loi" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
