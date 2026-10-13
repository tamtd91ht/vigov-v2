"use client";

import { Loader2 } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { TABLE_CLASS, TABLE_FRAME_CLASS, TD_CLASS, TH_CLASS } from "@/features/nhiem-vu/task-spec";
import { cn } from "@/lib/cn";

import { formatCount } from "./figures";

/**
 * THE ONE FRAME of Tổng quan's "rows behind a figure" dialog — the prototype's generic `DrillDialog`
 * (`vigov-require/apps/admin/src/components/reports/DrillDialog.tsx:37-129`): title, "N <entity> đằng
 * sau con số này.", Mã · Nội dung · Bộ phận · Người xử lý · Hạn · `Mở`, the loading / error / empty
 * states. Every register's dialog (`task-drill.tsx`, `record-drill.tsx`) draws through it, so the four
 * lists cannot drift into four looks.
 *
 * It renders TEXT ONLY from the rows: a petition's content is a citizen's words (rule 3) and goes into a
 * cell, never into an attribute, a title or a URL — the `open` control is the caller's, named by code.
 */

export const DRILL_TITLE_ID = "tieu-de-danh-sach-con-so";
export const DRILL_LOADING = "Đang lấy danh sách…";
export const DRILL_EMPTY = "Không có bản ghi nào.";
export const DRILL_OPEN = "Mở";
export const DRILL_MORE = "Xem thêm";

/** Rows per read — the server's own cap (`core/page.MaxLimit`); `Xem thêm` reads on with its cursor. */
export const DRILL_PAGE_SIZE = 100;

/** The prototype's description, `{n} {entity} đằng sau con số này.` — n from the server, never guessed. */
export function drillDescriptionOf(noun: string, count: number | null): string {
  return count === null ? DRILL_LOADING : `${formatCount(count)} ${noun} đằng sau con số này.`;
}

/** The prototype's six columns; a register may rename them (Giải ngân: Tên, Người phụ trách, Hạn giải ngân). */
export type DrillHeaders = readonly [string, string, string, string, string];

export const DEFAULT_DRILL_HEADERS: DrillHeaders = ["Mã", "Nội dung", "Bộ phận", "Người xử lý", "Hạn"];

/** One row, already formatted by its register. */
export type DrillRow = {
  readonly key: string;
  readonly code: string;
  readonly title: string;
  readonly unit: string;
  readonly assignee: string;
  readonly due: string;
  /** The `Mở` control — a button (task drawer) or a link (the register's screen). */
  readonly open: ReactNode;
};

export type DrillBody =
  | { readonly kind: "loading" }
  | { readonly kind: "error"; readonly message: string }
  | { readonly kind: "rows"; readonly rows: readonly DrillRow[] };

/** The `Mở` look, shared by the button and the link forms. */
export const DRILL_OPEN_CLASS =
  "text-brand inline-flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[12px] [font-family:inherit] focus-visible:outline-2 focus-visible:outline-brand-500";

export function DrillTableDialog({
  title,
  description,
  headers = DEFAULT_DRILL_HEADERS,
  body,
  hasMore,
  loadingMore,
  onMore,
  onClose,
}: {
  title: string;
  description: string;
  headers?: DrillHeaders;
  body: DrillBody;
  hasMore: boolean;
  loadingMore: boolean;
  onMore: () => void;
  onClose: () => void;
}) {
  return (
    <ModalDialog titleId={DRILL_TITLE_ID} size="lg" className="max-w-5xl" onDismiss={onClose}>
      <ModalDialogHeader titleId={DRILL_TITLE_ID} title={title} description={description} />
      {body.kind === "loading" ? (
        <p role="status" className="text-ink-muted m-0 flex items-center gap-2 py-10 text-[13px]">
          <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
          {DRILL_LOADING}
        </p>
      ) : body.kind === "error" ? (
        <p role="alert" className="thong-bao-loi m-0">
          {body.message}
        </p>
      ) : (
        <div
          role="region"
          aria-label={`Danh sách: ${title}`}
          tabIndex={0}
          className={cn(TABLE_FRAME_CLASS, "max-h-[56vh] min-h-0 overflow-auto")}
        >
          <table className={TABLE_CLASS}>
            <thead>
              <tr className="border-line border-b">
                {headers.map((h) => (
                  <th key={h} scope="col" className={TH_CLASS}>
                    {h}
                  </th>
                ))}
                <th scope="col" className={TH_CLASS}>
                  <span className="an-thi-giac">Thao tác</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {body.rows.length === 0 ? (
                <tr>
                  <td colSpan={6} className="text-ink-muted py-10 text-center">
                    {DRILL_EMPTY}
                  </td>
                </tr>
              ) : (
                body.rows.map((r) => (
                  <tr key={r.key} className="border-line border-b last:border-b-0">
                    <td className={TD_CLASS}>
                      <code className="text-[11.5px]">{r.code === "" ? "—" : r.code}</code>
                    </td>
                    <td className={cn(TD_CLASS, "text-navy max-w-[26rem] font-medium whitespace-normal")}>
                      <span className="line-clamp-2">{r.title}</span>
                    </td>
                    <td className={cn(TD_CLASS, "text-ink-muted")}>{r.unit}</td>
                    <td className={cn(TD_CLASS, "text-ink-muted")}>{r.assignee}</td>
                    <td className={TD_CLASS}>{r.due}</td>
                    <td className={TD_CLASS}>{r.open}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
      {hasMore && (
        <div>
          <Button type="button" variant="outline" size="sm" disabled={loadingMore} aria-busy={loadingMore || undefined} onClick={onMore}>
            {DRILL_MORE}
          </Button>
        </div>
      )}
    </ModalDialog>
  );
}
