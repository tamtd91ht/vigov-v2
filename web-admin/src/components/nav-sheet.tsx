"use client";

import { KeyRound, Menu, X } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { PendingMarker } from "@/components/ui/pending-feature";
import { NutDangXuat } from "@/features/auth/nut-dang-xuat";
import { DUONG_DAN_DOI_MAT_KHAU } from "@/features/mat-khau/bat-doi-mat-khau";
import type { KhoiNguoiDung } from "@/features/phien/khoi-nguoi-dung";

import { MenuIcon } from "./menu-icons";
import {
  activeChildRoute,
  dangChon,
  isMenuParent,
  parentRoute,
  PENDING_SCREENS,
  type MucMenu,
  type NhomMenu,
} from "./muc-menu";
import { RolePill } from "./role-pill";
import { userInitials } from "./user-initials";

/**
 * The narrow-screen navigation: a "menu" button in the header opening a full-height sheet, below 768px
 * (`globals.css`, `.header-narrow`), where the left sidebar (`side-nav.tsx`) is hidden.
 *
 * WHY 768px: below it the page is one column (`15-phu-luc §7`), and a 240px sidebar would leave a phone
 * too little room for the content. From 768px the sidebar fits beside the page and is the one place for
 * navigation (owner, 05/10/2026).
 *
 * WHY TEXT LABELS HERE, like the expanded sidebar: a small screen is a touch screen, and touch has no
 * hover, so no tooltip ever shows (`tooltip.tsx`). An icon with no visible word would be a guess.
 *
 * The right-side header items collapse into it (person, role, change password, sign out, the Phase-2
 * search placeholder); the bell stays in the header at every width, so an unread count is never one
 * tap further away.
 *
 * NATIVE `<dialog>` + `showModal()`, like `ui/large-dialog.tsx`: the page behind becomes inert, Tab
 * cannot leave the sheet, Esc fires `cancel`. MOUNTED = OPEN; closing unmounts it and focus returns to
 * the menu button. jsdom has no `showModal`, so the `open` attribute is the fallback there.
 */
export const NAV_SHEET_OPEN_LABEL = "Mở menu";
export const NAV_SHEET_CLOSE_LABEL = "Đóng menu";
export const NAV_SHEET_TITLE = "Menu";

export type NavSheetProps = {
  groups: readonly NhomMenu[];
  pathname: string;
  person: KhoiNguoiDung;
  roleName: string | null;
  /** The Phase-2 search placeholder, drawn by the header so its one sentence has one owner. */
  search: ReactNode;
};

export function NavSheet(props: NavSheetProps) {
  const [open, setOpen] = useState(false);

  return (
    <div className="header-narrow">
      <button
        type="button"
        className="header-icon-button"
        aria-label={NAV_SHEET_OPEN_LABEL}
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
      >
        <Menu aria-hidden="true" focusable="false" strokeWidth={1.8} />
      </button>
      {open && <NavSheetDialog {...props} onClose={() => setOpen(false)} />}
    </div>
  );
}

function NavSheetDialog({ onClose, ...view }: NavSheetProps & { onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (el === null) return;
    // Focus goes back to the menu button AFTER the dialog is gone: while it is still modal the page
    // behind is inert and a `focus()` there is silently refused.
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    if (typeof el.showModal === "function") {
      if (!el.open) el.showModal();
    } else {
      el.setAttribute("open", "");
    }
    return () => {
      if (opener !== null && opener.isConnected) opener.focus();
    };
  }, []);

  return (
    <dialog
      ref={ref}
      className="nav-sheet"
      aria-labelledby="nav-sheet-title"
      aria-modal="true"
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      // A press on the backdrop lands on the <dialog> element itself, never on its content.
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <NavSheetContent {...view} onNavigate={onClose} />
    </dialog>
  );
}

/**
 * What the sheet DRAWS — pure, exported so a Node test can render it without opening a dialog.
 * `onNavigate` closes the sheet when a link is followed: the App Router keeps this component mounted
 * across a client navigation, so without it the sheet would stay open over the new page.
 */
export function NavSheetContent({
  groups,
  pathname,
  person,
  roleName,
  search,
  onNavigate,
}: NavSheetProps & { onNavigate: () => void }) {
  return (
    <div className="nav-sheet-body">
      <div className="nav-sheet-head">
        <h2 id="nav-sheet-title">{NAV_SHEET_TITLE}</h2>
        <button type="button" className="nav-sheet-close" aria-label={NAV_SHEET_CLOSE_LABEL} onClick={onNavigate}>
          <X aria-hidden="true" focusable="false" strokeWidth={1.8} />
        </button>
      </div>

      {groups.length > 0 && (
        <nav className="nav-sheet-nav" aria-label="Điều hướng chính">
          {groups.map((g, i) => (
            <div key={g.ten === "" ? `untitled-${i}` : g.ten} className="nav-sheet-group">
              {g.ten !== "" && <p className="nav-sheet-group-label">{g.ten}</p>}
              <ul aria-label={g.ten === "" ? undefined : g.ten}>
                {g.muc.map((m) => {
                  if (!isMenuParent(m)) {
                    return <NavSheetItem key={m.nhan} item={m} active={dangChon(m.duong, pathname)} onNavigate={onNavigate} />;
                  }
                  // Same shape as the expanded sidebar: the parent row, its children indented under it.
                  // Never `aria-current` on the row — the current page is the child.
                  const current = activeChildRoute(m, pathname);
                  const href = parentRoute(m);
                  const rowClass = current === null ? "nav-sheet-link nav-sheet-parent" : "nav-sheet-link nav-sheet-parent is-open";
                  const row = (
                    <>
                      <MenuIcon label={m.nhan} />
                      <span className="nav-sheet-label">{m.nhan}</span>
                    </>
                  );
                  return (
                    <li key={m.nhan} className="nav-sheet-item has-children">
                      {href === null ? (
                        <span className={rowClass}>{row}</span>
                      ) : (
                        <Link href={href} className={rowClass} onClick={onNavigate}>
                          {row}
                        </Link>
                      )}
                      <ul className="nav-sheet-children" aria-label={m.nhan}>
                        {m.children.map((c) => (
                          <NavSheetItem
                            key={c.nhan}
                            item={c}
                            active={c.duong !== null && c.duong === current}
                            onNavigate={onNavigate}
                          />
                        ))}
                      </ul>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </nav>
      )}

      <div className="nav-sheet-account">
        {search}
        {person.hien && (
          <div className="nav-sheet-person">
            {userInitials(person.hoTen) !== "" && (
              <span className="topbar-avatar" aria-hidden="true">
                {userInitials(person.hoTen)}
              </span>
            )}
            <div className="min-w-0">
              <p className="ho-ten">{person.hoTen}</p>
              <p className="chuc-vu">{person.chucVu}</p>
            </div>
          </div>
        )}
        {roleName !== null && <RolePill roleName={roleName} />}
        {person.hien && (
          <Link className="nav-sheet-link" href={DUONG_DAN_DOI_MAT_KHAU} onClick={onNavigate}>
            <KeyRound aria-hidden="true" focusable="false" strokeWidth={1.8} />
            <span className="nav-sheet-label">Đổi mật khẩu</span>
          </Link>
        )}
        <NutDangXuat />
      </div>
    </div>
  );
}

/** One screen row of the sheet — a link, or the disabled placeholder with its "?" (ADR 0068 §14). */
function NavSheetItem({ item, active, onNavigate }: { item: MucMenu; active: boolean; onNavigate: () => void }) {
  if (item.duong === null) {
    const info = PENDING_SCREENS[item.nhan];
    return (
      <li className="nav-sheet-item is-pending">
        <span aria-disabled="true" className="nav-sheet-link">
          <MenuIcon label={item.nhan} />
          <span className="nav-sheet-label">{item.nhan}</span>
        </span>
        {info !== undefined && <PendingMarker info={info} side="bottom" className="ml-auto" />}
      </li>
    );
  }
  return (
    <li className="nav-sheet-item">
      <Link
        href={item.duong}
        aria-current={active ? "page" : undefined}
        className={active ? "nav-sheet-link is-active" : "nav-sheet-link"}
        onClick={onNavigate}
      >
        <MenuIcon label={item.nhan} />
        <span className="nav-sheet-label">{item.nhan}</span>
      </Link>
    </li>
  );
}
