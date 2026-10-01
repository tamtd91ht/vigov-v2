"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

/**
 * Console navigation. Wave 1 (ADR 0048 §01/10 #4) has one section with a screen.
 *
 * Wave-2 sections are NOT listed as disabled entries: they are not scheduled screens of this
 * shell, and a menu of greyed-out promises reads as a product that does not work. TASK-06b and
 * later cards add entries as their screens land.
 *
 * Showing an entry is UX, not authorisation: service-platform checks `ops.*` on every call.
 */
export const NAV_ITEMS: readonly { label: string; href: string }[] = [{ label: "Danh sách xã", href: "/xa" }];

export function isCurrent(href: string, pathname: string): boolean {
  return pathname === href || pathname.startsWith(href + "/");
}

export function Sidebar() {
  const pathname = usePathname() ?? "/";
  return (
    <nav className="sidebar" aria-label="Điều hướng chính">
      <div className="sidebar-top">
        <span className="sidebar-mark" aria-hidden="true">
          VG
        </span>
        <span className="sidebar-title">
          <span className="sidebar-product">ViGov</span>
          <span className="sidebar-subtitle">KHU VẬN HÀNH NỀN TẢNG</span>
        </span>
      </div>
      <ul className="sidebar-list">
        {NAV_ITEMS.map((item) => {
          const current = isCurrent(item.href, pathname);
          return (
            <li key={item.href}>
              <Link
                href={item.href}
                aria-current={current ? "page" : undefined}
                className={current ? "sidebar-link current" : "sidebar-link"}
              >
                {item.label}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
