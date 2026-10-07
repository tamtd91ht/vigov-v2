"use client";

import { useEffect, useState } from "react";

import { DOCUMENT_PANEL_ID, documentTabDomId, type DocumentTabId } from "@/features/van-ban/document-pending";
import { DocumentWorkspace } from "@/features/van-ban/document-workspace";
import { NUT_CAP_SO, NUT_VAO_SO } from "@/features/van-ban/nhan-van-ban";
import { idNutXem } from "@/features/van-ban/so-van-ban-den";

import { setDocumentPreviewState } from "./documents.fixture";
import type { DocumentPreviewModal, DocumentPreviewStateWord, DocumentPreviewTab } from "./preview-params";
import { usePressWhenReady } from "./preview-shell";

/** `?tab=` words → the REAL tab id of `DocumentWorkspace`. */
const TAB_ID: Record<DocumentPreviewTab, DocumentTabId> = {
  den: "incoming",
  di: "outgoing",
  "don-thu": "petitions",
  "bao-cao": "report",
};

/** `?modal=` words → the tab that owns the dialog, and the label of the REAL header button opening it. */
const MODAL: Record<DocumentPreviewModal, { tab: DocumentPreviewTab; button: string }> = {
  "vao-so-den": { tab: "den", button: NUT_VAO_SO },
  "cap-so-di": { tab: "di", button: NUT_CAP_SO },
};

/**
 * The REAL `DocumentWorkspace`, exactly as `app/van-ban/page.tsx` mounts it (no Overview filter), fed by
 * the fixture answering machine. Every state a screenshot needs is reached by pressing the screen's OWN
 * controls — its tab, its dialogs and its detail live in its state, and the preview never copies a
 * component:
 *   `tab`      presses the real `[role=tab]` (none pressed: the screen's own default tab);
 *   `modal`    presses the header button of the dialog's tab — and that tab first;
 *   `drawer`   presses the row's number button in Văn bản đến — and that tab first;
 *   `state`    sets what the two REGISTER reads answer, during the first render — before the
 *              registers' effects fetch (`documents.fixture.ts`);
 *   `scrollRight`  pushes the open register's own scroller fully right, once its rows are drawn;
 *   `foldOpen` presses the intake dialog's real "Thông tin thêm" toggle;
 *   `measure`  writes the open dialog's height and the window's to `<body data-preview-measure>`.
 */
export function DocumentsPreview({
  tab,
  modal,
  drawer,
  state,
  scrollRight = false,
  foldOpen = false,
  measure = false,
}: {
  tab: DocumentPreviewTab | null;
  modal: DocumentPreviewModal | null;
  drawer: string | null;
  state: DocumentPreviewStateWord | null;
  scrollRight?: boolean;
  foldOpen?: boolean;
  measure?: boolean;
}) {
  useState(() => {
    setDocumentPreviewState(state);
    return true;
  });

  const pressedTab = modal !== null ? MODAL[modal].tab : drawer !== null ? "den" : tab;
  usePressWhenReady(pressedTab === null ? null : `#${documentTabDomId(TAB_ID[pressedTab])}`);
  usePressWhenReady(modal === null ? null : 'button[aria-haspopup="dialog"]', modal === null ? undefined : MODAL[modal].button);
  usePressWhenReady(drawer === null ? null : `[id="${idNutXem(drawer)}"]`);
  usePressWhenReady(foldOpen ? '[aria-controls="thong-tin-them-van-ban-den"][aria-expanded="false"]' : null);
  useWhenReady(scrollRight ? `#${DOCUMENT_PANEL_ID} [role="region"]` : null, (el) => {
    if (el.querySelector("tbody tr") === null) return false;
    el.scrollLeft = el.scrollWidth;
    return true;
  });
  useWhenReady(measure ? "dialog[open]" : null, (el) => {
    // Wait for the drawer's timeline (or any dialog's content) to be drawn before measuring.
    if (el.querySelector('section[aria-labelledby="tieu-de-dong-thoi-gian"] ol, section[aria-labelledby="tieu-de-dong-thoi-gian"] p') === null && el.querySelector("form") === null) return false;
    document.body.dataset.previewMeasure = JSON.stringify({
      innerHeight: window.innerHeight,
      dialogHeight: Math.round(el.getBoundingClientRect().height),
    });
    return true;
  });

  return <DocumentWorkspace />;
}

/** Runs `act` on the first element matching `selector` until it returns true. Gives up after 10 s. */
function useWhenReady(selector: string | null, act: (el: HTMLElement) => boolean): void {
  useEffect(() => {
    if (selector === null) return;
    const timer = window.setInterval(() => {
      const el = document.querySelector<HTMLElement>(selector);
      if (el !== null && act(el)) window.clearInterval(timer);
    }, 150);
    const stop = window.setTimeout(() => window.clearInterval(timer), 10_000);
    return () => {
      window.clearInterval(timer);
      window.clearTimeout(stop);
    };
    // `act` is a fresh closure each render; the selector alone decides when to (re)start.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selector]);
}
