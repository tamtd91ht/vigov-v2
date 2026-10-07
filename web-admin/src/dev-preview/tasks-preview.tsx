"use client";

import { useState } from "react";

import { BATCH_DELETE_BUTTON, selectLabel } from "@/features/nhiem-vu/batch-delete";
import { NHAN_CHE_DO_DANH_SACH, NHAN_CHE_DO_KANBAN } from "@/features/nhiem-vu/nhan-nhiem-vu";
import { SoNhiemVu } from "@/features/nhiem-vu/so-nhiem-vu";
import { IMPORT_OPEN_BUTTON } from "@/features/nhiem-vu/task-import";
import { REGISTER_VIEW_LABEL } from "@/features/nhiem-vu/task-register";
import { taskTabsStorageKey } from "@/features/nhiem-vu/task-tabs";

import type { TaskPreviewModal, TaskPreviewView } from "./preview-params";
import { usePressWhenReady } from "./preview-shell";
import { previewSession } from "./shell.fixture";
import { PREVIEW_SELECTED_CODES } from "./tasks.fixture";

/** `?che-do=` words → the label of the REAL view-switch button that shows that view. */
const VIEW_BUTTON: Record<TaskPreviewView, string> = {
  kanban: NHAN_CHE_DO_KANBAN,
  "danh-sach": NHAN_CHE_DO_DANH_SACH,
  "so-theo-doi": REGISTER_VIEW_LABEL,
};

/** The header's two buttons — each opens its dialog; both `aria-haspopup="dialog"`. */
const MODAL_BUTTON: Record<Exclude<TaskPreviewModal, "xoa-nhieu">, string> = {
  "giao-viec": "Giao việc mới",
  "nhap-excel": IMPORT_OPEN_BUTTON,
};

/**
 * The REAL `SoNhiemVu`, exactly as `app/nhiem-vu/page.tsx` mounts it (no drill-down; `openTask` from
 * `?task=`, read server-side by the preview page with the real `parseOpenTask`), fed by the fixture
 * answering machine. Every state a screenshot needs is reached by pressing the screen's OWN controls —
 * its view, dialogs and selection live in its state, and the preview never copies a component:
 *   `view`     presses the view switch (Kanban is the screen's default: nothing pressed);
 *   `modal`    presses `Giao việc mới` / `Nhập từ Excel` in the header, or — `xoa-nhieu` — the
 *              `Xoá đã chọn` button the selection brings up;
 *   `select`   ticks the two fixture tasks `PREVIEW_SELECTED_CODES` (the bulk-delete bar appears).
 *
 * RECORD TABS: the screen restores its open tabs from sessionStorage (`taskTabsStorageKey`, per host
 * and staff code). The preview clears its own fixture officer's key during the FIRST render — before
 * the screen's restore effect runs — so a screenshot never carries tabs left by an earlier visit.
 */
export function TaskPreview({
  view,
  modal,
  select,
  openTask,
}: {
  view: TaskPreviewView;
  modal: TaskPreviewModal | null;
  select: boolean;
  openTask: string | null;
}) {
  useState(() => {
    if (typeof window === "undefined") return true;
    const key = taskTabsStorageKey(window.location.host, previewSession([], "lanh-dao").staff.code);
    try {
      if (key !== null) window.sessionStorage.removeItem(key);
    } catch {
      // Storage blocked: nothing was restored from it either.
    }
    return true;
  });

  usePressWhenReady(view === "kanban" ? null : '[aria-label="Chế độ xem"] button', VIEW_BUTTON[view]);
  usePressWhenReady(
    modal === null || modal === "xoa-nhieu" ? null : 'button[aria-haspopup="dialog"]',
    modal === null || modal === "xoa-nhieu" ? undefined : MODAL_BUTTON[modal],
  );
  usePressWhenReady(select ? `input[type="checkbox"][aria-label="${selectLabel(PREVIEW_SELECTED_CODES[0])}"]` : null);
  usePressWhenReady(select ? `input[type="checkbox"][aria-label="${selectLabel(PREVIEW_SELECTED_CODES[1])}"]` : null);
  usePressWhenReady(modal === "xoa-nhieu" ? 'button[aria-haspopup="dialog"]' : null, BATCH_DELETE_BUTTON);

  return <SoNhiemVu openTask={openTask} />;
}
