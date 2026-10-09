"use client";

import { BookUser, Newspaper, type LucideIcon } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { KhungQuyen } from "@/features/quyen/cong-quyen";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { cn } from "@/lib/cn";

import {
  landingTab,
  MINI_APP_TABS,
  miniAppTabHref,
  NO_MINI_APP_ACCESS,
  visibleTabs,
  type MiniAppTab,
} from "./mini-app-tabs";

const TAB_ICON: Record<MiniAppTab, LucideIcon> = { "noi-dung": Newspaper, "danh-ba": BookUser };

/**
 * The padding under the tab band — the prototype's `p-7` of each tab's screen, drawn once here for both
 * tabs. Below 768px the page's own padding at that width (`.than-trang`: 16px, 40px at the bottom).
 */
export const TAB_BODY_CLASS = "min-w-0 px-4 pt-4 pb-10 md:p-7";

/**
 * `/mini-app` — the prototype's `MiniAppWorkspace` (`vigov-require` `apps/admin/src/components/content/
 * MiniAppWorkspace.tsx`, ADR 0068 lần 5): a strip of two tabs on top, then the open tab's whole screen.
 *
 * TABS ARE LINKS, NOT CLIENT STATE: the prototype keeps the open tab in `?tab=` (`router.replace`); a
 * `<Link>` to `/mini-app?tab=…` writes the same address, and the server page re-reads it. The panel of
 * the other tab is never mounted, so a tab the account may not open calls nothing.
 *
 * WHICH TAB SHOWS (`landingTab`): the one asked for when this account holds its key, else the first it
 * does hold. Until the session is read nothing is drawn but the gate's status line — drawing the content
 * tab first and swapping it out a moment later would fire its reads for an account that may lack the key.
 * A session that could not be read shows the server's sentence (`KhungQuyen`), never a guess.
 */
export function MiniAppWorkspace({
  requested,
  content,
  directory,
}: {
  requested: MiniAppTab;
  /** The content tab's screen — rendered only when that tab is the one shown. */
  content: ReactNode;
  /** The directory tab's screen. */
  directory: ReactNode;
}) {
  const session = usePhien();
  return <MiniAppFrame session={session} requested={requested} content={content} directory={directory} />;
}

/** Presentational half, session passed in — so the denied and unread cases render in a test. */
export function MiniAppFrame({
  session,
  requested,
  content,
  directory,
}: {
  session: ReturnType<typeof usePhien>;
  requested: MiniAppTab;
  content: ReactNode;
  directory: ReactNode;
}) {
  // The page's `main` has no padding of its own on this route (the tab band is flush); a screen with no
  // band gets the body's padding here instead.
  if (session === null || !session.ok) {
    return (
      <div className={TAB_BODY_CLASS}>
        <KhungQuyen
          quyetDinh={session === null ? null : { hien: false, vi: "khong-doc-duoc", thongBao: session.thongBao }}
          cauThieuQuyen={NO_MINI_APP_ACCESS}
        >
          {null}
        </KhungQuyen>
      </div>
    );
  }

  const visible = visibleTabs(session.duLieu.permissions);
  const shown = landingTab(requested, visible);
  if (shown === null) {
    return (
      <div className={TAB_BODY_CLASS}>
        <KhungQuyen quyetDinh={{ hien: false, vi: "khong-du-quyen" }} cauThieuQuyen={NO_MINI_APP_ACCESS}>
          {null}
        </KhungQuyen>
      </div>
    );
  }

  return (
    <div className="flex min-w-0 flex-col">
      {/* Prototype (`MiniAppWorkspace.tsx:39-68`): a FULL-WIDTH white band `px-7 pt-5`, flush with the
          header — the page's `main` drops its padding for it (`app/mini-app/page.tsx`) — holding `flex
          gap-1` tabs `rounded-t-[10px] border border-b-0 px-4 py-2.5 text-[13px]` over a one-pixel line.
          The open tab is white with the line's border, the others borderless. Only the tabs this account
          may open are drawn. 16px side padding below 768px, as the page's own padding there. */}
      <nav aria-label="Ngăn của màn Nội dung Mini App" className="min-w-0 bg-white px-4 pt-5 md:px-7">
        <div className="flex min-w-0 gap-1 overflow-x-auto">
          {MINI_APP_TABS.filter((t) => visible.includes(t.key)).map((t) => {
            const active = t.key === shown;
            const Icon = TAB_ICON[t.key];
            return (
              <Link
                key={t.key}
                href={miniAppTabHref(t.key)}
                aria-current={active ? "page" : undefined}
                scroll={false}
                className={cn(
                  "flex shrink-0 items-center gap-2 rounded-t-[10px] border border-b-0 px-4 py-2.5 text-[13px] font-semibold whitespace-nowrap no-underline transition-colors",
                  active ? "border-line bg-white text-navy" : "border-transparent text-ink-muted hover:text-navy",
                )}
              >
                <Icon aria-hidden="true" focusable="false" className="size-4" />
                {t.label}
              </Link>
            );
          })}
        </div>
        <div className="border-t border-line" />
      </nav>

      {/* Each tab's screen in the prototype opens with its own `p-7`. It is drawn HERE, once, for both
          tabs — so neither tab's component adds a padding of its own. 16px / 40px bottom below 768px,
          the page's padding at that width (`.than-trang`). */}
      <div className={TAB_BODY_CLASS} data-testid="mini-app-tab-body">
        {shown === "noi-dung" ? content : directory}
      </div>
    </div>
  );
}
