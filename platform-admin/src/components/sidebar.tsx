"use client";

import { KeyRound, Landmark, LifeBuoy, PanelLeftClose, PanelLeftOpen, ServerCog, type LucideIcon } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

import { cn } from "@/lib/cn";

/**
 * Console navigation — drawn like web-admin's sidebar (`web-admin/src/components/sidebar-view.tsx`):
 * white, 240px, an icon per item, the active item filled `--brand-50` with a 3px bar. Wave 1
 * (ADR 0048 §01/10 #4) has one section with screens, plus the operator's own account pages.
 *
 * Wave-2 sections are NOT listed as disabled entries: a menu of greyed-out promises reads as a
 * product that does not work. Later cards add entries as their screens land.
 *
 * Showing an entry is UX, not authorisation: service-platform checks `ops.*` on every call.
 *
 * COLLAPSING (from 1024px): kept in component state only, NOT persisted. web-admin keeps it in
 * browser storage; this console stores nothing in the browser (`console.test.tsx` scans for it), and
 * a menu that re-opens on a full reload costs nothing. The layout keeps this component mounted
 * across client navigations, so the choice survives moving between screens. Collapsing hides the
 * labels from the EYE only: the DOM, the tab order and every accessible name stay.
 *
 * Below 1024px the bar is a strip across the top whose items scroll inside it, never the page.
 */
export type NavItem = { label: string; href: string; icon: LucideIcon };

export const NAV_ITEMS: readonly NavItem[] = [{ label: "Danh sách xã", href: "/xa", icon: Landmark }];

export const ACCOUNT_ITEMS: readonly NavItem[] = [
  { label: "Đổi mật khẩu", href: "/tai-khoan/doi-mat-khau", icon: KeyRound },
  { label: "Tạo lại mã khôi phục", href: "/tai-khoan/ma-khoi-phuc", icon: LifeBuoy },
];

export const COLLAPSE_LABEL = "Thu gọn menu";
export const EXPAND_LABEL = "Mở rộng menu";

export function isCurrent(href: string, pathname: string): boolean {
  return pathname === href || pathname.startsWith(href + "/");
}

function NavGroup({
  title,
  items,
  pathname,
  collapsed,
}: {
  title: string | null;
  items: readonly NavItem[];
  pathname: string;
  collapsed: boolean;
}) {
  return (
    <div className="flex shrink-0 items-center gap-1 lg:block">
      {title !== null ? (
        <p
          className={cn(
            "m-0 hidden px-2 pt-3.5 pb-1.5 text-[11px] font-semibold tracking-[0.06em] text-ink-500 uppercase lg:block",
            collapsed && "lg:sr-only",
          )}
        >
          {title}
        </p>
      ) : null}
      <ul className="m-0 flex list-none gap-1 p-0 lg:flex-col lg:gap-0.5">
        {items.map((item) => {
          const current = isCurrent(item.href, pathname);
          const Icon = item.icon;
          return (
            <li key={item.href}>
              <Link
                href={item.href}
                aria-current={current ? "page" : undefined}
                // A tooltip for the eye when the label is hidden; the label itself stays the name.
                title={collapsed ? item.label : undefined}
                className={cn(
                  "relative flex min-h-9 items-center gap-3 rounded-[8px] px-3 text-[13.5px] font-medium whitespace-nowrap text-ink-700 no-underline",
                  "transition-[background-color,color] duration-150",
                  "[&_svg]:size-[18px] [&_svg]:shrink-0 [&_svg]:text-ink-500",
                  current
                    ? "bg-brand-50 font-semibold text-brand-700 shadow-[inset_3px_0_0_var(--brand-600)] [&_svg]:text-brand-600"
                    : "hover:bg-surface-muted hover:text-ink-900 hover:[&_svg]:text-brand-600",
                  collapsed && "lg:justify-center lg:px-0",
                )}
              >
                <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />
                <span className={cn("min-w-0 overflow-hidden text-ellipsis", collapsed && "lg:sr-only")}>{item.label}</span>
              </Link>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export function Sidebar() {
  const pathname = usePathname() ?? "/";
  const [collapsed, setCollapsed] = useState(false);
  const toggleLabel = collapsed ? EXPAND_LABEL : COLLAPSE_LABEL;
  const ToggleIcon = collapsed ? PanelLeftOpen : PanelLeftClose;

  return (
    <nav
      aria-label="Điều hướng chính"
      className={cn(
        "relative flex min-w-0 flex-col border-b border-line bg-surface text-ink-700",
        "lg:sticky lg:top-0 lg:h-dvh lg:self-start lg:border-r lg:border-b-0",
        "transition-[width] duration-250",
        collapsed ? "lg:w-[var(--sidebar-w-collapsed)]" : "lg:w-[var(--sidebar-w)]",
      )}
    >
      <div className={cn("flex items-center gap-3 px-4 pt-3.5 pb-2 lg:px-5 lg:pt-5 lg:pb-4", collapsed && "lg:justify-center lg:px-0")}>
        <span
          aria-hidden="true"
          className="grid size-10 shrink-0 place-items-center rounded-xl border border-brand-100 bg-brand-50 text-brand-600"
        >
          <ServerCog className="size-[22px]" strokeWidth={1.8} focusable="false" />
        </span>
        <span className={cn("flex min-w-0 flex-col", collapsed && "lg:sr-only")}>
          <span className="text-[15px] leading-tight font-bold text-ink-900">ViGov</span>
          <span className="text-xs leading-snug text-ink-500">Khu vận hành nền tảng</span>
        </span>
      </div>

      <div className="flex min-w-0 gap-1 overflow-x-auto px-3 pb-2 lg:min-h-0 lg:flex-1 lg:flex-col lg:gap-0 lg:overflow-x-hidden lg:overflow-y-auto lg:pb-3">
        <NavGroup title={null} items={NAV_ITEMS} pathname={pathname} collapsed={collapsed} />
        <NavGroup title="Tài khoản" items={ACCOUNT_ITEMS} pathname={pathname} collapsed={collapsed} />
      </div>

      {/* Collapsing means nothing on the top strip, so its control exists from 1024px only. */}
      <div className="hidden shrink-0 border-t border-line p-3 lg:block">
        <button
          type="button"
          onClick={() => setCollapsed((c) => !c)}
          aria-expanded={!collapsed}
          // The label says what pressing it WILL do, not the current state.
          aria-label={toggleLabel}
          title={collapsed ? toggleLabel : undefined}
          className={cn(
            "m-0 flex min-h-10 w-full cursor-pointer items-center gap-2.5 rounded-[8px] border-0 bg-transparent px-3 [font-family:inherit] text-[13px] text-ink-500",
            "hover:bg-surface-muted hover:text-ink-900",
            collapsed && "justify-center px-0",
          )}
        >
          <ToggleIcon aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0" />
          <span className={cn(collapsed && "sr-only")}>{toggleLabel}</span>
        </button>
      </div>
    </nav>
  );
}
