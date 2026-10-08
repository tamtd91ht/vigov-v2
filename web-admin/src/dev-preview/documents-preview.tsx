"use client";

import { useEffect, useState } from "react";

import { DOCUMENT_PANEL_ID, documentTabDomId, type DocumentTabId } from "@/features/van-ban/document-pending";
import { DocumentWorkspace } from "@/features/van-ban/document-workspace";
import { LETTER_ENTRY_SUBMIT } from "@/features/van-ban/letter-entry-dialog";
import { letterRowButtonId } from "@/features/van-ban/letter-register";
import { NUT_CAP_SO, NUT_VAO_SO } from "@/features/van-ban/nhan-van-ban";
import { idNutXem } from "@/features/van-ban/so-van-ban-den";

import { setDocumentPreviewState } from "./documents.fixture";
import { PREVIEW_LETTER_PREFIX } from "./letters.fixture";
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
  "vao-so-don": { tab: "don-thu", button: LETTER_ENTRY_SUBMIT },
};

/** What `?dup=1` types into the booking dialog — a fixture sender and summary, nothing real. */
const DUPLICATE_SENDER = "Hoàng Văn E";
const DUPLICATE_SUMMARY = "Phản ánh mương thoát nước tổ 3 bị tắc, nước tràn vào nhà dân";

/** Sets a React-controlled field the way a keystroke does: the native setter, then an `input` event. */
function typeInto(el: HTMLInputElement | HTMLTextAreaElement, value: string): void {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  Object.getOwnPropertyDescriptor(proto, "value")?.set?.call(el, value);
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

/**
 * The REAL `DocumentWorkspace`, exactly as `app/van-ban/page.tsx` mounts it (no Overview filter), fed by
 * the fixture answering machine. Every state a screenshot needs is reached by pressing the screen's OWN
 * controls — its tab, its dialogs and its detail live in its state, and the preview never copies a
 * component:
 *   `tab`      presses the real `[role=tab]` (none pressed: the screen's own default tab);
 *   `modal`    presses the header button of the dialog's tab — and that tab first;
 *   `drawer`   presses the row's number button in Văn bản đến — and that tab first; a citizen-letter
 *              id (`PREVIEW_LETTER_PREFIX`) does the same on the Đơn thư tab;
 *   `duplicate` types a fixture name and summary into the open booking dialog (`?dup=1`);
 *   `state`    sets what the two REGISTER reads answer, during the first render — before the
 *              registers' effects fetch (`documents.fixture.ts`);
 *   `scrollRight`  pushes the open register's own scroller fully right, once its rows are drawn;
 *   `foldOpen` presses the intake dialog's real "Thông tin thêm" toggle;
 *   `measure`  writes the open dialog's height and the window's to `<body data-preview-measure>`, and
 *              the PAGE's scroll width against the window's to `<body data-preview-page-width>` (a
 *              page that scrolls sideways at 1440px is a defect even when every table scrolls inside).
 */
export function DocumentsPreview({
  tab,
  modal,
  drawer,
  state,
  scrollRight = false,
  foldOpen = false,
  measure = false,
  duplicate = false,
  statusStep = false,
  editSender = false,
  statusFilter = false,
  scrollEnd = false,
}: {
  tab: DocumentPreviewTab | null;
  modal: DocumentPreviewModal | null;
  drawer: string | null;
  state: DocumentPreviewStateWord | null;
  scrollRight?: boolean;
  foldOpen?: boolean;
  measure?: boolean;
  duplicate?: boolean;
  /** `?buoc=1` and the three words after it: `previewDocumentFlag`. */
  statusStep?: boolean;
  editSender?: boolean;
  statusFilter?: boolean;
  scrollEnd?: boolean;
}) {
  useState(() => {
    setDocumentPreviewState(state);
    return true;
  });

  const letterDrawer = drawer !== null && drawer.startsWith(PREVIEW_LETTER_PREFIX);
  const pressedTab = modal !== null ? MODAL[modal].tab : drawer !== null ? (letterDrawer ? "don-thu" : "den") : tab;
  usePressWhenReady(pressedTab === null ? null : `#${documentTabDomId(TAB_ID[pressedTab])}`);
  usePressWhenReady(modal === null ? null : 'button[aria-haspopup="dialog"]', modal === null ? undefined : MODAL[modal].button);
  usePressWhenReady(drawer === null ? null : `[id="${letterDrawer ? letterRowButtonId(drawer) : idNutXem(drawer)}"]`);
  useWhenReady(duplicate && modal === "vao-so-don" ? "dialog[open] #don-thu-nguoi-gui" : null, (el) => {
    const summary = document.querySelector<HTMLTextAreaElement>("dialog[open] #don-thu-noi-dung");
    if (summary === null) return false;
    typeInto(el as HTMLInputElement, DUPLICATE_SENDER);
    typeInto(summary, DUPLICATE_SUMMARY);
    return true;
  });
  usePressWhenReady(statusStep ? 'dialog[open] [aria-label="Các bước của đơn thư"] button:not(:disabled)' : null);
  usePressWhenReady(editSender ? "dialog[open] button" : null, editSender ? "Sửa thông tin người gửi" : undefined);
  useWhenReady(statusFilter ? "#don-thu-loc-trang-thai" : null, (el) => {
    const select = el as HTMLSelectElement;
    Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, "value")?.set?.call(select, "dinh-chi");
    select.dispatchEvent(new Event("change", { bubbles: true }));
    return true;
  });
  useWhenReady(
    scrollEnd ? 'dialog[open] section[aria-labelledby="tieu-de-ket-qua-don-thu"], #' + DOCUMENT_PANEL_ID + ' [role="img"]' : null,
    (el) => {
      // Scroll the nearest SCROLLING ancestor to its end — never `scrollIntoView`, which also moves a
      // document whose shell is fixed-height and leaves a blank band at the top.
      for (let p = el.parentElement; p !== null; p = p.parentElement) {
        const y = getComputedStyle(p).overflowY;
        if ((y === "auto" || y === "scroll") && p.scrollHeight > p.clientHeight) {
          p.scrollTop = p.scrollHeight;
          return true;
        }
      }
      window.scrollTo(0, document.documentElement.scrollHeight);
      return true;
    },
  );
  useWhenReady(measure ? `#${DOCUMENT_PANEL_ID}` : null, (el) => {
    // Once the tab has drawn something more than its loading bars, record the page's width.
    if (el.querySelector("table, [role='alert'], h2 + div") === null) return false;
    // The OUTERMOST elements reaching past the window's right edge — where a sideways page scroll
    // comes from (an element whose parent already overflows is not listed again). A table CLIPPED by
    // its own scroller is listed too; it is harmless when `scrollWidth` equals the window's width.
    const edge = document.documentElement.clientWidth + 1;
    const offenders = [...document.body.querySelectorAll<HTMLElement>("*")]
      .filter((n) => n.getBoundingClientRect().right > edge && (n.parentElement?.getBoundingClientRect().right ?? 0) <= edge)
      .slice(0, 6)
      .map((n) => `${n.tagName.toLowerCase()}.${String(n.className).slice(0, 80)} → ${Math.round(n.getBoundingClientRect().right)}`);
    document.body.dataset.previewPageWidth = JSON.stringify({
      scrollWidth: document.documentElement.scrollWidth,
      innerWidth: window.innerWidth,
      clientWidth: document.documentElement.clientWidth,
      offenders,
    });
    return true;
  });
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
