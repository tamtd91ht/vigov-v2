"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { CauHinhXaProvider } from "@/components/cau-hinh-xa";
import { DauTrang } from "@/components/dau-trang";
import { SIDEBAR_STORAGE_KEY } from "@/components/sidebar-state";
import { PhienProvider } from "@/features/phien/phien-hien-tai";

import { installFixtureFetch } from "./fixture-fetch";
import { DEV_PREVIEW_PREFIX } from "./preview-gate";
import { PREVIEW_TOAST, previewShellState } from "./preview-params";
import { fullMenuPermissions, PREVIEW_COMMUNE, PREVIEW_PERMISSIONS, previewSession } from "./shell.fixture";

/**
 * The REAL signed-in shell — `khung-trang`, `DauTrang` (sidebar + header), `<main class="than-trang">` —
 * exactly as every real page composes it (`app/giai-ngan/page.tsx`), around a preview body. The commune
 * comes from the fixture instead of `Host`, the session from the fixture answering machine
 * (`fixture-fetch.ts`), read by the REAL `PhienProvider` through the real client.
 *
 * THE ANSWERING MACHINE IS INSTALLED DURING THE FIRST RENDER, not in an effect: children's effects run
 * before their parent's, and the Giải ngân components fetch in theirs — an effect here would be too late
 * for the first requests. Installing is idempotent and does nothing on the server.
 *
 * The menu marks the REAL path as current: the URL is `/xem-thu/giai-ngan`, the item is `/giai-ngan`.
 *
 * SHELL STATES for screenshots (`previewShellState`): the sidebar's collapse is the REAL store's — its
 * remembered choice is written during this first render, before the sidebar subscribes and reads it, so
 * the preview never draws the other state from a choice left behind by an earlier visit (the store's
 * server snapshot is "expanded", so the collapsed state lands in the hydration pass, as on real pages).
 * The account menu and the bell are opened by pressing their REAL buttons; the toast goes through the
 * REAL `Toaster` of the root layout. Real pages read none of these words.
 */
export function PreviewShell({ fullMenu, children }: { fullMenu: boolean; children: ReactNode }) {
  const params = useSearchParams();
  const shell = previewShellState((name) => params.get(name));
  useState(() => {
    installFixtureFetch(previewSession(fullMenu ? fullMenuPermissions() : PREVIEW_PERMISSIONS));
    if (typeof window !== "undefined") {
      try {
        window.localStorage.setItem(SIDEBAR_STORAGE_KEY, shell.sidebarCollapsed ? "1" : "0");
      } catch {
        // Storage blocked: the sidebar simply stays expanded.
      }
    }
    return true;
  });
  usePressWhenReady(shell.accountMenu ? "button.header-user" : null);
  usePressWhenReady(shell.bellPanel ? "button.nut-chuong" : null);
  usePreviewToast(shell.toast);
  const path = usePathname() ?? DEV_PREVIEW_PREFIX;
  const menuPath = path.slice(DEV_PREVIEW_PREFIX.length) || "/";
  return (
    <CauHinhXaProvider giaTri={PREVIEW_COMMUNE}>
      <PhienProvider>
        <div className="khung-trang">
          <DauTrang menuPath={menuPath} />
          <main className="than-trang">{children}</main>
        </div>
      </PhienProvider>
    </CauHinhXaProvider>
  );
}

/**
 * Presses a control once it appears — how the preview opens a dialog or a tab that only the REAL
 * component's own button can open (the dialog is not exported, the tab state is internal), so a
 * screenshot URL is deterministic without copying the component. `selector` finds the candidates,
 * `text` (optional) picks the one whose words match. Gives up after 10 s.
 */
export function usePressWhenReady(selector: string | null, text?: string): void {
  useEffect(() => {
    if (selector === null) return;
    let done = false;
    const timer = window.setInterval(() => {
      const el = [...document.querySelectorAll<HTMLElement>(selector)].find(
        (c) => text === undefined || c.textContent?.trim() === text,
      );
      if (el === undefined || done) return;
      done = true;
      window.clearInterval(timer);
      el.click();
    }, 100);
    const stop = window.setTimeout(() => window.clearInterval(timer), 10_000);
    return () => {
      window.clearInterval(timer);
      window.clearTimeout(stop);
    };
  }, [selector, text]);
}

/**
 * Shows one success toast, kept on screen until closed (`duration: Infinity`) so a screenshot taken
 * seconds later still has it. Delayed: the root layout's `Toaster` is a LATER sibling of the page, so
 * its subscription effect runs after this one — a toast fired at once would reach no region.
 */
function usePreviewToast(show: boolean): void {
  useEffect(() => {
    if (!show) return;
    const timer = window.setTimeout(() => toast.success(PREVIEW_TOAST, { duration: Number.POSITIVE_INFINITY }), 300);
    return () => window.clearTimeout(timer);
  }, [show]);
}
