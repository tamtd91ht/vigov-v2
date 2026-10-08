"use client";

import { Mail } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { PageHeader } from "@/components/ui/page-header";
import { NO_DRILL_DOWN, type DrillDown } from "@/lib/drill-down";

import {
  DOCUMENT_PANEL_ID,
  DocumentTabBar,
  documentTabDomId,
  type DocumentTabId,
} from "./document-pending";
import type { RegisterFrame } from "./document-ui";
import { LetterRegister } from "./letter-register";
import { LetterReport } from "./letter-report";
import { SoVanBanDen } from "./so-van-ban-den";
import { SoVanBanDi } from "./so-van-ban-di";

/** The page's title and subtitle — the prototype's subtitle, word for word. */
export const DOCUMENT_PAGE_TITLE = "Văn bản & đơn thư";
export const DOCUMENT_PAGE_SUBTITLE = "Vào sổ, phân công xử lý và theo dõi hạn giải quyết.";
/** The prototype's H1 (`DocumentWorkspace.tsx:108`) — its page holds only the petition tabs. */
export const LETTER_PAGE_TITLE = "Đơn thư công dân";

/**
 * The H1 for the active tab (owner request v2 §4.1): the two tabs the prototype has (`Đơn thư công dân`,
 * `Báo cáo`) carry its title; `Văn bản đến` / `Văn bản đi`, which the prototype does not have, keep the
 * page's own title — "Đơn thư công dân" above an incoming-document register would name the wrong book.
 */
export function documentPageTitle(tab: DocumentTabId): string {
  return tab === "petitions" || tab === "report" ? LETTER_PAGE_TITLE : DOCUMENT_PAGE_TITLE;
}

/**
 * The tab the page opens on (ADR 0078 #5, an ASSUMPTION awaiting the owner): `Đơn thư công dân`, as in
 * the prototype (`DocumentWorkspace.tsx:156`, `defaultValue="don-thu"`) — EXCEPT when the address
 * carries an Overview filter (`metric=…`, valid or not): that filter applies to `Văn bản đến`, and
 * opening another tab would hide it (or hide the "invalid filter" sentence) behind a tab nobody chose.
 */
export function initialDocumentTab(drillDown: DrillDown<"incoming-documents">): DocumentTabId {
  return drillDown.kind === "none" ? "petitions" : "incoming";
}

/**
 * Header-button look of the prototype (`DocumentWorkspace.tsx:126-143`): 40px tall, 13px words, 12px
 * gap with the "hoặc" between them. Given here, by descendant selector, so the disabled "?" buttons of
 * the petition tab (drawn by the shared `PendingButton`, which sizes itself) take the same height
 * without a variant added to a shared component. The "?" marker keeps its own size.
 */
const HEADER_ACTIONS_CLASS =
  "flex flex-wrap items-center gap-3 [&_button:not([data-pending-marker])]:h-10 [&_button:not([data-pending-marker])]:text-[13px]";

/**
 * The Văn bản & Đơn thư screen — the prototype's `DocumentWorkspace` frame (`DocumentWorkspace.tsx:
 * 105-166`): the page header (22px navy title, one subtitle line, the ACTIVE tab's 40px buttons on the
 * right), the tab bar, the active tab's body 16px under it.
 *
 * WHY THE HEADER BUTTONS CHANGE WITH THE TAB: in the prototype the header pair belongs to the petition
 * register, its main tab. Here each register has a working create button of its own, so each tab
 * puts its own buttons in the header (`RegisterFrame`): Văn bản đến `[+ Vào sổ văn bản đến]`, Văn bản đi
 * `[+ Cấp số văn bản đi]`, Đơn thư and Báo cáo the prototype's `[+ Vào sổ đơn thư] hoặc [Nhập từ
 * Excel ?]` (the import has no route, ADR 0078 #6). No "Quét & OCR": the prototype has none (#5).
 *
 * Tab order is the prototype's with `Văn bản đi` after `Văn bản đến`; the opening tab is
 * `initialDocumentTab`. The tab is NOT in the URL (see `app/van-ban/page.tsx`). Only the selected tab is
 * mounted, as in the prototype: switching tabs starts the other register from its first page.
 */
export function DocumentWorkspace({
  drillDown = NO_DRILL_DOWN,
}: {
  /** Overview filter, read on the SERVER (`app/van-ban/page.tsx`) — it applies to Văn bản đến. */
  drillDown?: DrillDown<"incoming-documents">;
}) {
  const [tab, setTab] = useState<DocumentTabId>(() => initialDocumentTab(drillDown));
  // Each tab's body renders its own frame, so switching tabs REMOUNTS the tab bar with it and the
  // pressed tab's node is gone. Focus is put back on the newly selected tab once the new tree is
  // committed — only after a switch the user asked for, never on arrival.
  const focusSelectedTab = useRef(false);
  useEffect(() => {
    if (!focusSelectedTab.current) return;
    focusSelectedTab.current = false;
    document.getElementById(documentTabDomId(tab))?.focus();
  }, [tab]);
  const selectTab = (id: DocumentTabId) => {
    focusSelectedTab.current = true;
    setTab(id);
  };

  const frame: RegisterFrame = (headerActions, body, subtitleExtra) => (
    <>
      <PageHeader
        icon={Mail}
        title={documentPageTitle(tab)}
        subtitle={
          subtitleExtra === undefined || subtitleExtra === null ? (
            DOCUMENT_PAGE_SUBTITLE
          ) : (
            <>
              {DOCUMENT_PAGE_SUBTITLE}
              {subtitleExtra}
            </>
          )
        }
        className="mb-5"
        actions={headerActions === null ? undefined : <div className={HEADER_ACTIONS_CLASS}>{headerActions}</div>}
      />
      <DocumentTabBar selected={tab} onSelect={selectTab} />
      <div
        role="tabpanel"
        id={DOCUMENT_PANEL_ID}
        aria-labelledby={documentTabDomId(tab)}
        className="mt-4 min-w-0"
      >
        {body}
      </div>
    </>
  );

  let content: ReactNode;
  switch (tab) {
    case "incoming":
      // A new overview filter rebuilds the whole workspace (the page keys it by filter).
      content = <SoVanBanDen drillDown={drillDown} frame={frame} />;
      break;
    case "outgoing":
      content = <SoVanBanDi frame={frame} />;
      break;
    case "petitions":
      content = <LetterRegister frame={frame} />;
      break;
    default:
      content = <LetterReport frame={frame} />;
  }
  return content;
}
