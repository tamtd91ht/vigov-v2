"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

import { useOperator } from "@/features/operator/operator-context";
import { ApiError, signOut } from "@/lib/api";
import { NETWORK_ERROR } from "@/lib/errors";
import { goTo } from "@/lib/navigate";
import { SIGN_IN_PATH } from "@/lib/route-access";

/**
 * Console navigation and the signed-in operator's account menu. Wave 1 (ADR 0048 §01/10 #4) has
 * one section with screens.
 *
 * Wave-2 sections are NOT listed as disabled entries: a menu of greyed-out promises reads as a
 * product that does not work. Later cards add entries as their screens land.
 *
 * Showing an entry is UX, not authorisation: service-platform checks `ops.*` on every call.
 */
export const NAV_ITEMS: readonly { label: string; href: string }[] = [{ label: "Danh sách xã", href: "/xa" }];

export const ACCOUNT_ITEMS: readonly { label: string; href: string }[] = [
  { label: "Đổi mật khẩu", href: "/tai-khoan/doi-mat-khau" },
  { label: "Tạo lại mã khôi phục", href: "/tai-khoan/ma-khoi-phuc" },
];

export const SIGN_OUT_UNAVAILABLE =
  "Chưa đăng xuất được: hệ thống xác thực tạm không phản hồi. Vui lòng thử lại sau ít phút.";

export function isCurrent(href: string, pathname: string): boolean {
  return pathname === href || pathname.startsWith(href + "/");
}

function NavList({ items, pathname }: { items: readonly { label: string; href: string }[]; pathname: string }) {
  return (
    <ul className="sidebar-list">
      {items.map((item) => {
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
  );
}

export function Sidebar() {
  const pathname = usePathname() ?? "/";
  const operator = useOperator();
  const [signingOut, setSigningOut] = useState(false);
  const [signOutError, setSignOutError] = useState<string | null>(null);

  async function doSignOut() {
    if (signingOut) return;
    setSigningOut(true);
    setSignOutError(null);
    try {
      await signOut();
    } catch (err) {
      // 401: the session was already gone — the person is signed out, which is what they asked.
      // Anything else: the session may still be live, so say so instead of pretending.
      if (!(err instanceof ApiError && err.status === 401)) {
        setSignOutError(
          err instanceof ApiError && err.status === 0
            ? NETWORK_ERROR
            : err instanceof ApiError && err.status === 503
              ? SIGN_OUT_UNAVAILABLE
              : "Chưa đăng xuất được. Vui lòng thử lại.",
        );
        setSigningOut(false);
        return;
      }
    }
    goTo(SIGN_IN_PATH);
  }

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
      <NavList items={NAV_ITEMS} pathname={pathname} />
      <section className="sidebar-account" aria-label="Tài khoản">
        <p className="sidebar-operator">
          <span className="sidebar-operator-label">Người vận hành</span>
          <span className="sidebar-operator-code">
            {operator.status === "ready" ? operator.operator.operator_code : operator.status === "loading" ? "…" : "—"}
          </span>
        </p>
        {operator.status === "error" ? (
          <p className="sidebar-alert" role="alert">
            {operator.message}
          </p>
        ) : null}
        <NavList items={ACCOUNT_ITEMS} pathname={pathname} />
        <button type="button" className="sidebar-signout" onClick={doSignOut} disabled={signingOut} aria-busy={signingOut}>
          {signingOut ? "Đang đăng xuất…" : "Đăng xuất"}
        </button>
        {signOutError ? (
          <p className="sidebar-alert" role="alert">
            {signOutError}
          </p>
        ) : null}
      </section>
    </nav>
  );
}
