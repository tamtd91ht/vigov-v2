import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { PendingMarker } from "@/components/ui/pending-feature";
import { Tooltip } from "@/components/ui/tooltip";

import { menuIcon } from "./menu-icons";
import { dangChon, PENDING_SCREENS, type MucMenu, type NhomMenu } from "./muc-menu";

/**
 * The LEFT sidebar — module navigation, vertical, icon + word, grouped under the headings of `NHOM_MENU`
 * (owner, 05/10/2026: "giữ ô menu bên trái … giữ cả nút icon nhưng là nằm dọc bên trái thay vì nằm ngang
 * phía trên, hãy xem prototype"). It replaces the header's icon row of ADR 0068 §Sửa đổi 05/10/2026
 * (lần 2) #6; navigation lives in ONE place. Shape from the prototype (`vigov-require` `AppSidebar.tsx`:
 * groups with headings, active item, collapse to icons with the choice remembered); look from the
 * OMICALL tokens of lần 2 — white surface, navy ink, the cyan accent only as a FILL.
 *
 * WHAT THIS FILE DOES NOT DECIDE: which items exist, their order and who sees them. `groups` arrives
 * already filtered by `locMenu` (`muc-menu.ts`). Hiding an item is UX; every route still checks its key
 * on the server (rule 5, forbidden #1).
 *
 * COLLAPSED = ICONS ONLY, BUT NEVER NAMELESS: the item's word stays in the DOM, visually hidden, so the
 * link keeps its accessible name; the tooltip only repeats it for a mouse (`tooltip.tsx`). Group headings
 * become thin dividers, so the business areas still read as runs.
 *
 * Below 768px this is not drawn (`globals.css`, `.side-nav`): the header's menu button opens the nav
 * sheet with the same items and words (`nav-sheet.tsx`).
 *
 * PURE FUNCTION OF ITS PROPS (no session, no router, no storage), so every branch renders in a plain
 * Node test. The collapse state is `sidebar-state.ts`'s, read by the shell (`dau-trang.tsx`).
 */
export const SIDEBAR_COLLAPSE_LABEL = "Thu gọn menu";
export const SIDEBAR_EXPAND_LABEL = "Mở rộng menu";
export const SIDE_NAV_LABEL = "Điều hướng chính";

export type SideNavProps = {
  groups: readonly NhomMenu[];
  pathname: string;
  collapsed: boolean;
  onToggle: () => void;
};

/** Tooltips of a collapsed sidebar open to its right, over the page. */
const TOOLTIP_SIDE = "right" as const;

export function SideNav({ groups, pathname, collapsed, onToggle }: SideNavProps) {
  const toggleLabel = collapsed ? SIDEBAR_EXPAND_LABEL : SIDEBAR_COLLAPSE_LABEL;
  const ToggleIcon = collapsed ? PanelLeftOpen : PanelLeftClose;
  return (
    <div className={collapsed ? "side-nav is-collapsed" : "side-nav"}>
      {/* The outer box stretches the full height of the page body (one white strip); the inner one sticks
          under the header and scrolls on its own, so a long page never carries the menu away. */}
      <div className="side-nav-inner">
        <div className="side-nav-head">
          <Tooltip content={toggleLabel} side={TOOLTIP_SIDE} enabled={collapsed}>
            <button type="button" className="side-nav-toggle" aria-label={toggleLabel} aria-expanded={!collapsed} onClick={onToggle}>
              <ToggleIcon aria-hidden="true" focusable="false" strokeWidth={1.8} />
            </button>
          </Tooltip>
        </div>
        {groups.length > 0 && (
          <nav className="side-nav-nav" aria-label={SIDE_NAV_LABEL}>
            {groups.map((g, i) => (
              <div key={g.ten === "" ? `untitled-${i}` : g.ten} className="side-nav-group">
                {collapsed || g.ten === "" ? (
                  i > 0 && <span className="side-nav-divider" aria-hidden="true" />
                ) : (
                  <p className="side-nav-group-label">{g.ten}</p>
                )}
                <ul aria-label={g.ten === "" ? undefined : g.ten}>
                  {g.muc.map((m) => {
                    const Icon = menuIcon(m.nhan);
                    return (
                      <li key={m.nhan} className={m.duong === null ? "side-nav-item is-pending" : "side-nav-item"}>
                        <SideNavItem
                          item={m}
                          pathname={pathname}
                          collapsed={collapsed}
                          icon={<Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                        />
                      </li>
                    );
                  })}
                </ul>
              </div>
            ))}
          </nav>
        )}
      </div>
    </div>
  );
}

function SideNavItem({ item, pathname, collapsed, icon }: { item: MucMenu; pathname: string; collapsed: boolean; icon: ReactNode }) {
  const label = <span className={collapsed ? "an-thi-giac" : "side-nav-label"}>{item.nhan}</span>;

  if (item.duong === null) {
    // An item listed before its screen exists (ADR 0068 §14): NOT a link and NOT a `<button disabled>` —
    // both take focus and then do nothing. The one focusable thing is the "?", whose accessible name
    // already names the item; collapsed, its hover also says the item's name, since no word is visible.
    const info = PENDING_SCREENS[item.nhan];
    return (
      <span className="side-nav-pending">
        <span aria-disabled="true" className="side-nav-link">
          {icon}
          {label}
        </span>
        {info !== undefined && (
          <PendingMarker
            info={info}
            side={TOOLTIP_SIDE}
            placement={collapsed ? "corner" : "inline"}
            nameInHover={collapsed}
            className={collapsed ? undefined : "ml-auto"}
          />
        )}
      </span>
    );
  }

  const active = dangChon(item.duong, pathname);
  return (
    <Tooltip content={item.nhan} side={TOOLTIP_SIDE} enabled={collapsed}>
      <Link href={item.duong} aria-current={active ? "page" : undefined} className={active ? "side-nav-link is-active" : "side-nav-link"}>
        {icon}
        {label}
      </Link>
    </Tooltip>
  );
}
