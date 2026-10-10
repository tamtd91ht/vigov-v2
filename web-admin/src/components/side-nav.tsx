import * as PopoverPrimitive from "@radix-ui/react-popover";
import { PanelLeftClose, PanelLeftOpen } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { PENDING_HOVER_TEXT } from "@/components/ui/pending-feature";
import { Tooltip } from "@/components/ui/tooltip";

import { MenuIcon } from "./menu-icons";
import {
  activeChildRoute,
  activeMenuRoute,
  isMenuParent,
  parentRoute,
  type MenuParent,
  type MucMenu,
  type NhomMenu,
} from "./muc-menu";

/**
 * The LEFT sidebar — module navigation, vertical, icon + word, grouped under the headings of `NHOM_MENU`.
 * Shape AND look from the owner's Giải ngân spec 01 (ADR 0068 §Sửa đổi 07/10/2026 lần 6 #2, #3, #8),
 * itself transcribed from the prototype's `AppSidebar.tsx`: navy, full viewport height, the
 * "VG · ViGov · Điều hành số cấp xã" brand block on top, groups with headings, the active item lit with
 * a 3px bar on the left edge, collapsible to a 64px icon strip, a version footer.
 *
 * WHY "ViGov" IS PRINTED HERE AND NOWHERE ELSE: ADR 0068 §13 kept the product name off every staff
 * screen; lần 6 #3 lifted that for THIS block only, on the owner's "Theo spec". The commune's own
 * identity stays in the header (`commune-identity.tsx`), read at runtime from `Host`.
 *
 * WHAT THIS FILE DOES NOT DECIDE: which items exist, their order and who sees them. `groups` arrives
 * already filtered by `locMenu` (`muc-menu.ts`). Hiding an item is UX; every route still checks its key
 * on the server (rule 5, forbidden #1).
 *
 * COLLAPSED = ICONS ONLY, BUT NEVER NAMELESS: the item's word stays in the DOM, visually hidden, so the
 * link keeps its accessible name; the tooltip only repeats it for a mouse (`tooltip.tsx`). Group headings
 * become thin dividers, one per group, as the prototype draws them.
 *
 * A PARENT ROW, COLLAPSED: the prototype hides the children and keeps the parent icon, which reaches the
 * first child only. Here the parent icon opens a flyout listing its children, so every child screen stays
 * one press away at either width — collapsing changes the look, never what a person can reach.
 *
 * AN ITEM WITH NO SCREEN YET (`duong: null`): muted, not a link, not focusable, hover says
 * "Tính năng đang phát triển" — and NO "?" beside it (lần 6 #11: the "?" left the sidebar; it stays on
 * unbuilt controls inside a screen, §14).
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

/** Brand block words — spec 01 verbatim (ADR 0068 lần 6 #8). Platform constants, never per commune. */
export const PRODUCT_MARK = "VG";
export const PRODUCT_NAME = "ViGov";
export const PRODUCT_TAGLINE = "Điều hành số cấp xã";

/**
 * The footer line, spec 01 minus "Môi trường phát triển" (lần 6 #8: that line was false on a commune's
 * live site). Written here rather than read from `package.json`: importing that file into a client
 * component ships the whole dependency list, with versions, to every browser — the same reason the
 * server sends no `X-Powered-By`. Bump it with the package version.
 */
export const APP_VERSION_LABEL = "Phiên bản 0.1.0";

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
  // One lit row for the whole menu, never one per matching item (`activeMenuRoute`).
  const active = activeMenuRoute(groups, pathname);
  const toggle = (
    <Tooltip content={toggleLabel} side={TOOLTIP_SIDE} enabled={collapsed}>
      <button
        type="button"
        className="side-nav-toggle"
        aria-label={toggleLabel}
        aria-expanded={!collapsed}
        title={collapsed ? undefined : toggleLabel}
        onClick={onToggle}
      >
        <ToggleIcon aria-hidden="true" focusable="false" strokeWidth={ICON_STROKE} />
      </button>
    </Tooltip>
  );
  return (
    <aside className={collapsed ? "side-nav is-collapsed" : "side-nav"}>
      {/* Expanded: the toggle sits at the right end of the brand row. Collapsed: the row keeps only the
          tile, and the toggle moves under it (spec 01). */}
      <SidebarBrand collapsed={collapsed}>{collapsed ? null : toggle}</SidebarBrand>
      {collapsed && toggle}
      {groups.length > 0 && (
        <nav className="side-nav-nav" aria-label={SIDE_NAV_LABEL}>
          {groups.map((g, i) => (
            <div key={g.ten === "" ? `untitled-${i}` : g.ten} className="side-nav-group">
              {collapsed || g.ten === "" ? (
                <span className="side-nav-divider" aria-hidden="true" />
              ) : (
                <p className="side-nav-group-label">{g.ten}</p>
              )}
              <ul aria-label={g.ten === "" ? undefined : g.ten}>
                {g.muc.map((m) =>
                  isMenuParent(m) ? (
                    <li key={m.nhan} className="side-nav-item has-children">
                      {collapsed ? <SideNavParentFlyout parent={m} pathname={pathname} active={active} /> : <SideNavParent parent={m} pathname={pathname} active={active} />}
                    </li>
                  ) : (
                    <li key={m.nhan} className={m.duong === null ? "side-nav-item is-pending" : "side-nav-item"}>
                      <SideNavItem item={m} collapsed={collapsed} active={m.duong !== null && m.duong === active} />
                    </li>
                  ),
                )}
              </ul>
            </div>
          ))}
        </nav>
      )}
      {/* The prototype drops the footer when collapsed: a 64px strip has no room for the words. */}
      {!collapsed && <p className="side-nav-footer">{APP_VERSION_LABEL}</p>}
    </aside>
  );
}

/**
 * The "VG · ViGov · Điều hành số cấp xã" block (spec 01). Shared with the narrow-screen sheet
 * (`nav-sheet.tsx`) so the two never drift; `children` is the control at the row's right end (the
 * collapse toggle here, the close button there). Decorative words, no link: the prototype's block is
 * not a link either, and "home" already has its menu item.
 */
export function SidebarBrand({ collapsed = false, children }: { collapsed?: boolean; children?: ReactNode }) {
  return (
    <div className={collapsed ? "side-nav-brand is-collapsed" : "side-nav-brand"}>
      <span className="side-nav-brand-mark" aria-hidden="true">
        {PRODUCT_MARK}
      </span>
      {!collapsed && (
        <span className="side-nav-brand-text">
          <span className="side-nav-brand-name">{PRODUCT_NAME}</span>
          <span className="side-nav-brand-tagline">{PRODUCT_TAGLINE}</span>
        </span>
      )}
      {children}
    </div>
  );
}

/**
 * Expanded parent: the parent row, then its children indented under it (prototype `AppSidebar.tsx`).
 * The row leads to the first visible child, as the prototype's parent link does. It is NEVER
 * `aria-current`: the current page is the child, and announcing two current pages is announcing none.
 * When a child is current the row is marked `is-open`, so the eye finds the branch.
 */
/** The child of `parent` that is the menu-wide active route, or `null` — a branch is lit only when its
 *  child IS the one lit row, not merely when a child matches by prefix. */
function currentChild(parent: MenuParent, pathname: string, active: string | null): string | null {
  const current = activeChildRoute(parent, pathname);
  return current !== null && current === active ? current : null;
}

function SideNavParent({ parent, pathname, active }: { parent: MenuParent; pathname: string; active: string | null }) {
  const current = currentChild(parent, pathname, active);
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
function SideNavParentFlyout({ parent, pathname, active }: { parent: MenuParent; pathname: string; active: string | null }) {
  const current = currentChild(parent, pathname, active);
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
    // An item listed before its screen exists: NOT a link and NOT a `<button disabled>` — both take
    // focus and then do nothing. Its word stays (visible, or hidden when collapsed) so a screen reader
    // still reads the full menu; the native `title` carries the owner's hover sentence (lần 6 #11).
    // Collapsed, the title also names the item, since no word is visible.
    return (
      <span
        aria-disabled="true"
        className="side-nav-link"
        title={collapsed ? `${item.nhan} — ${PENDING_HOVER_TEXT}` : PENDING_HOVER_TEXT}
      >
        {icon}
        {label}
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
