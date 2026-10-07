"use client";

import { useEffect, useState } from "react";

import { BATCH_DELETE_BUTTON, selectLabel } from "@/features/nhiem-vu/batch-delete";
import { NHAN_CHE_DO_DANH_SACH, NHAN_CHE_DO_KANBAN } from "@/features/nhiem-vu/nhan-nhiem-vu";
import { SoNhiemVu } from "@/features/nhiem-vu/so-nhiem-vu";
import { IMPORT_OPEN_BUTTON } from "@/features/nhiem-vu/task-import";
import { REGISTER_VIEW_LABEL } from "@/features/nhiem-vu/task-register";
import { taskTabsStorageKey } from "@/features/nhiem-vu/task-tabs";

import type { TaskPreviewModal, TaskPreviewView } from "./preview-params";
import { usePressWhenReady } from "./preview-shell";
import { previewSession } from "./shell.fixture";
import { PREVIEW_SELECTED_CODES, previewTaskExtensionHistory } from "./tasks.fixture";

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
 * SCREENSHOT WORDS read here (the shared `preview-params.ts` belongs to every preview), each acting
 * on the REAL screen once it has drawn — never a copy of a component:
 *   `?cuon=phai`            every sideways-scrolling table scrolled to its right end (Sổ theo dõi's
 *                           13 columns, the list) — `cuon=cuoi` scrolls the open detail to its bottom;
 *   `?loai=theo-van-ban`    with `?modal=giao-viec`: the create dialog's type set to that code (800px);
 *   `?soan=1`               with `?task=`: the first pressable status chip pressed (the compose box);
 *   `?sua=1`                with `?task=`: the information block's `Sửa` pressed (edit mode).
 * A task with no pending extension request (its request form): `?task=NV106`.
 *
 * `GET /api/v1/tasks/{code}/extensions` (Lịch sử gia hạn) is answered HERE from `tasks.fixture.ts`
 * (a read; the shared answering machine has no route for it): a wrapper over the installed fixture
 * fetch, set once in the first render — after `PreviewShell` installed its own — delegating every
 * other call unchanged.
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
    installExtensionHistoryAnswer();
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

  const words = useScreenshotWords();
  usePressWhenReady(
    words.compose && openTask !== null ? 'dialog[open] ol[aria-label="Các bước của vòng đời nhiệm vụ"] button:not([disabled])' : null,
  );
  usePressWhenReady(
    words.edit && openTask !== null ? 'dialog[open] section[aria-labelledby="task-detail-info"] button[aria-label^="Sửa"]:not([disabled])' : null,
  );
  useSetTypeWhenReady(modal === "giao-viec" ? words.type : null);
  useScrollWhenReady(words.scroll);

  return <SoNhiemVu openTask={openTask} />;
}

type ScreenshotWords = {
  readonly scroll: "phai" | "cuoi" | null;
  readonly type: string | null;
  readonly compose: boolean;
  readonly edit: boolean;
};

/** The words above, read once on the client (the server render draws the plain screen). */
function useScreenshotWords(): ScreenshotWords {
  const [words] = useState<ScreenshotWords>(() => {
    if (typeof window === "undefined") return { scroll: null, type: null, compose: false, edit: false };
    const q = new URLSearchParams(window.location.search);
    const scroll = q.get("cuon");
    return {
      scroll: scroll === "phai" || scroll === "cuoi" ? scroll : null,
      type: q.get("loai"),
      compose: q.get("soan") === "1",
      edit: q.get("sua") === "1",
    };
  });
  return words;
}

/** Sets the create dialog's `Loại nhiệm vụ` through the native setter + `change`, as a click would. */
function useSetTypeWhenReady(code: string | null): void {
  useEffect(() => {
    if (code === null) return;
    const timer = window.setInterval(() => {
      const select = document.querySelector<HTMLSelectElement>("dialog[open] #giao-loai");
      if (select === null || ![...select.options].some((o) => o.value === code)) return;
      window.clearInterval(timer);
      Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")!.set!.call(select, code);
      select.dispatchEvent(new Event("change", { bubbles: true }));
    }, 100);
    const stop = window.setTimeout(() => window.clearInterval(timer), 10_000);
    return () => {
      window.clearInterval(timer);
      window.clearTimeout(stop);
    };
  }, [code]);
}

/**
 * `phai`: every sideways-scrolling table region to its right end, once it holds rows. `cuoi`: every
 * vertically scrolling box inside the open detail to its bottom. Repeated for 3 s — late reads
 * (documents, the timeline) grow the boxes after the first pass.
 */
function useScrollWhenReady(mode: "phai" | "cuoi" | null): void {
  useEffect(() => {
    if (mode === null) return;
    const timer = window.setInterval(() => {
      if (mode === "phai") {
        for (const el of document.querySelectorAll<HTMLElement>('[role="region"].overflow-x-auto')) {
          if (el.querySelector("tbody tr") !== null) el.scrollLeft = el.scrollWidth;
        }
        return;
      }
      for (const el of document.querySelectorAll<HTMLElement>("dialog[open] *")) {
        if (el.scrollHeight > el.clientHeight + 1 && /(auto|scroll)/.test(getComputedStyle(el).overflowY)) {
          el.scrollTop = el.scrollHeight;
        }
      }
    }, 250);
    const stop = window.setTimeout(() => window.clearInterval(timer), 3_000);
    return () => {
      window.clearInterval(timer);
      window.clearTimeout(stop);
    };
  }, [mode]);
}

let historyAnswerInstalled = false;

/** See the block comment: answers the extension-history read from the fixture, delegates the rest. */
function installExtensionHistoryAnswer(): void {
  if (historyAnswerInstalled) return;
  historyAnswerInstalled = true;
  const next = window.fetch.bind(window);
  window.fetch = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
    const url = new URL(raw, window.location.origin);
    const m = /^\/api\/v1\/tasks\/([^/]+)\/extensions$/.exec(url.pathname);
    const method = (init?.method ?? (input instanceof Request ? input.method : "GET")).toUpperCase();
    if (m === null || method !== "GET") return next(input, init);
    const body = JSON.stringify(previewTaskExtensionHistory(decodeURIComponent(m[1]!)));
    return Promise.resolve(new Response(body, { status: 200, headers: { "Content-Type": "application/json" } }));
  };
}
