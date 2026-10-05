"use client";

import { Mail } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { PageHeader } from "@/components/ui/page-header";
import { NO_DRILL_DOWN, type DrillDown } from "@/lib/drill-down";
import { namTheoDongHoMay } from "@/lib/nam";

import {
  DOCUMENT_PANEL_ID,
  DocumentTabBar,
  PendingPetitionRegister,
  PendingPetitionReport,
  PetitionHeaderActions,
  documentTabDomId,
  type DocumentTabId,
} from "./document-pending";
import type { RegisterFrame } from "./document-ui";
import { SoVanBanDen } from "./so-van-ban-den";
import { SoVanBanDi } from "./so-van-ban-di";

/** The page's title and subtitle — the prototype's subtitle, word for word. */
export const DOCUMENT_PAGE_TITLE = "Văn bản & đơn thư";
export const DOCUMENT_PAGE_SUBTITLE = "Vào sổ, phân công xử lý và theo dõi hạn giải quyết.";

/**
 * The Văn bản & Đơn thư screen — the prototype's `DocumentWorkspace` frame (ADR 0068 lần 5): the page
 * header with the ACTIVE tab's buttons on the right, the tab bar, the active tab's body.
 *
 * WHY THE HEADER BUTTONS CHANGE WITH THE TAB: in the prototype the header pair belongs to the petition
 * register, its main tab. Here each register has a working create button of its own, so each tab
 * puts its own pair in the header (`RegisterFrame`): Văn bản đến `[+ Vào sổ văn bản đến] hoặc [Quét
 * & OCR ?]`, Văn bản đi `[+ Cấp số văn bản đi]`, Đơn thư and Báo cáo the prototype's `[+ Vào sổ đơn
 * thư ?] hoặc [Nhập từ Excel ?]`.
 *
 * THE FIRST TAB IS SELECTED ON ARRIVAL — `Văn bản đến`, a working register. The prototype opens on
 * `Đơn thư công dân` because its demo commune hid `Văn bản đến` (17/09/2026); neither applies here.
 * Only the selected tab is mounted, as in the prototype: switching tabs starts the other register
 * from its first page.
 */
export function DocumentWorkspace({
  drillDown = NO_DRILL_DOWN,
}: {
  /** Overview filter, read on the SERVER (`app/van-ban/page.tsx`) — it applies to Văn bản đến. */
  drillDown?: DrillDown<"incoming-documents">;
}) {
  const [tab, setTab] = useState<DocumentTabId>("incoming");
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
  // The report's year: the machine clock, read once — the same anchor every register uses.
  const [year] = useState(namTheoDongHoMay);

  const frame: RegisterFrame = (headerActions, body) => (
    <>
      <PageHeader
        icon={Mail}
        title={DOCUMENT_PAGE_TITLE}
        subtitle={DOCUMENT_PAGE_SUBTITLE}
        actions={headerActions === null ? undefined : <div className="flex flex-wrap items-center gap-3">{headerActions}</div>}
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
      content = frame(<PetitionHeaderActions />, <PendingPetitionRegister />);
      break;
    default:
      content = frame(<PetitionHeaderActions />, <PendingPetitionReport year={year} />);
  }
  return content;
}
