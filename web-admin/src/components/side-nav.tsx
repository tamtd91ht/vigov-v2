import * as PopoverPrimitive from "@radix-ui/react-popover";
import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import Link from "next/link";

import { PendingMarker } from "@/components/ui/pending-feature";
import { Tooltip } from "@/components/ui/tooltip";

import { MenuIcon } from "./menu-icons";
import {
  activeChildRoute,
  dangChon,
  isMenuParent,
  parentRoute,
  PENDING_SCREENS,
  type MenuParent,
  type MucMenu,
  type NhomMenu,
} from "./muc-menu";

/**
 * The LEFT sidebar — module navigation, vertical, icon + word, grouped under the headings of `NHOM_MENU`
 * (owner, 05/10/2026: "giữ ô menu bên trái … giữ cả nút icon nhưng là nằm dọc bên trái thay vì nằm ngang
 * phía trên, hãy xem prototype"). It replaces the header's icon row of ADR 0068 §Sửa đổi 05/10/2026
 * (lần 2) #6; navigation lives in ONE place. Shape from the prototype (`vigov-require` `AppSidebar.tsx`:
 * groups with headings, active item, one level of children indented under their parent behind a guide
 * line, collapse to icons with the choice remembered); look from the OMICALL tokens of lần 2 — white
 * surface, navy ink, the cyan accent only as a FILL.
 *
 * WHAT THIS FILE DOES NOT DECIDE: which items exist, their order and who sees them. `groups` arrives
 * already filtered by `locMenu` (`muc-menu.ts`). Hiding an item is UX; every route still checks its key
 * on the server (rule 5, forbidden #1).
 *
 * COLLAPSED = ICONS ONLY, BUT NEVER NAMELESS: the item's word stays in the DOM, visually hidden, so the
 * link keeps its accessible name; the tooltip only repeats it for a mouse (`tooltip.tsx`). Group headings
 * become thin dividers, so the groups still read as runs.
 *
 * A PARENT ROW, COLLAPSED: the prototype hides the children and keeps the parent icon, which reaches the
 * first child only. Here the parent icon opens a flyout listing its children, so every child screen stays
 * one press away at either width — collapsing changes the look, never what a person can reach.
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

/** Tooltips and the flyout of a collapsed sidebar open to its right, over the page. */
const TOOLTIP_SIDE = "right" as const;

const ICON_STROKE = 1.8;

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
              <ToggleIcon aria-hidden="true" focusable="false" strokeWidth={ICON_STROKE} />
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
                  {g.muc.map((m) =>
                    isMenuParent(m) ? (
                      <li key={m.nhan} className="side-nav-item has-children">
                        {collapsed ? <SideNavParentFlyout parent={m} pathname={pathname} /> : <SideNavParent parent={m} pathname={pathname} />}
                      </li>
                    ) : (
                      <li key={m.nhan} className={m.duong === null ? "side-nav-item is-pending" : "side-nav-item"}>
                        <SideNavItem item={m} collapsed={collapsed} active={dangChon(m.duong, pathname)} />
                      </li>
                    ),
                  )}
                </ul>
              </div>
            ))}
          </nav>
        )}
      </div>
    </div>
  );
}

/**
 * Expanded parent: the parent row, then its children indented under it (prototype `AppSidebar.tsx`).
 * The row leads to the first visible child, as the prototype's parent link does. It is NEVER
 * `aria-current`: the current page is the child, and announcing two current pages is announcing none.
 * When a child is current the row is marked `is-open`, so the eye finds the branch.
 */
function SideNavParent({ parent, pathname }: { parent: MenuParent; pathname: string }) {
  const current = activeChildRoute(parent, pathname);
  const href = parentRoute(parent);
  const rowClass = current === null ? "side-nav-link side-nav-parent" : "side-nav-link side-nav-parent is-open";
  const row = (
    <>
      <MenuIcon label={parent.nhan} />
      <span className="side-nav-label">{parent.nhan}</span>
    </>
  );
  return (
    <>
      {href === null ? (
        <span className={rowClass}>{row}</span>
      ) : (
        <Link href={href} className={rowClass}>
          {row}
        </Link>
      )}
      <ul className="side-nav-children" aria-label={parent.nhan}>
        {parent.children.map((c) => (
          <li key={c.nhan} className={c.duong === null ? "side-nav-item is-child is-pending" : "side-nav-item is-child"}>
            <SideNavItem item={c} collapsed={false} active={c.duong !== null && c.duong === current} />
          </li>
        ))}
      </ul>
    </>
  );
}

/**
 * Collapsed parent: its icon, named (hidden word + tooltip), opening a flyout to the right that lists the
 * children with their words. Lit when one of its children is the current page — it is the only row left
 * to carry that mark. A popover, not a hover menu: hover does not exist on touch, and a keyboard must
 * reach every child. `modal` stays false (ADR 0068 §4, as `user-menu.tsx`).
 */
function SideNavParentFlyout({ parent, pathname }: { parent: MenuParent; pathname: string }) {
  const current = activeChildRoute(parent, pathname);
  return (
    <PopoverPrimitive.Root>
      <Tooltip content={parent.nhan} side={TOOLTIP_SIDE}>
        <PopoverPrimitive.Trigger asChild>
          <button type="button" className={current === null ? "side-nav-link side-nav-parent" : "side-nav-link side-nav-parent is-active"}>
            <MenuIcon label={parent.nhan} />
            <span className="an-thi-giac">{parent.nhan}</span>
          </button>
        </PopoverPrimitive.Trigger>
      </Tooltip>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Content
          side={TOOLTIP_SIDE}
          align="start"
          sideOffset={8}
          collisionPadding={8}
          aria-label={parent.nhan}
          className="side-nav-flyout"
        >
          <p className="side-nav-flyout-title">{parent.nhan}</p>
          <ul>
            {parent.children.map((c) => (
              <li key={c.nhan} className={c.duong === null ? "side-nav-item is-pending" : "side-nav-item"}>
                {c.duong === null ? (
                  <SideNavItem item={c} collapsed={false} active={false} />
                ) : (
                  <PopoverPrimitive.Close asChild>
                    <Link
                      href={c.duong}
                      aria-current={c.duong === current ? "page" : undefined}
                      className={c.duong === current ? "side-nav-link is-active" : "side-nav-link"}
                    >
                      <MenuIcon label={c.nhan} />
                      <span className="side-nav-label">{c.nhan}</span>
                    </Link>
                  </PopoverPrimitive.Close>
                )}
              </li>
            ))}
          </ul>
        </PopoverPrimitive.Content>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  );
}

function SideNavItem({ item, collapsed, active }: { item: MucMenu; collapsed: boolean; active: boolean }) {
  const icon = <MenuIcon label={item.nhan} />;
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

  return (
    <Tooltip content={item.nhan} side={TOOLTIP_SIDE} enabled={collapsed}>
      <Link href={item.duong} aria-current={active ? "page" : undefined} className={active ? "side-nav-link is-active" : "side-nav-link"}>
        {icon}
        {label}
      </Link>
    </Tooltip>
  );
}
