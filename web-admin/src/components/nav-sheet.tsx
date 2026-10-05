"use client";

import { KeyRound, Menu, X } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { PendingMarker } from "@/components/ui/pending-feature";
import { NutDangXuat } from "@/features/auth/nut-dang-xuat";
import { DUONG_DAN_DOI_MAT_KHAU } from "@/features/mat-khau/bat-doi-mat-khau";
import type { KhoiNguoiDung } from "@/features/phien/khoi-nguoi-dung";

import { menuIcon } from "./menu-icons";
import { dangChon, PENDING_SCREENS, type NhomMenu } from "./muc-menu";
import { RolePill } from "./role-pill";
import { userInitials } from "./user-initials";

/**
 * The narrow-screen navigation: a "menu" button in the header opening a full-height sheet, below 1024px
 * (`globals.css`, `.header-narrow`).
 *
 * WHY TEXT LABELS HERE when the header row is icon-only: a small screen is a touch screen, and touch has
 * no hover, so no tooltip ever shows (`tooltip.tsx`). An icon with no visible word would be a guess.
 *
 * WHY 1024px AND NOT 768px: the header row carries 15 module icons + settings, the commune's name and
 * the person block — about 1000px at the tightest. Between 768 and 1023px it could only fit by
 * scrolling the icon row sideways, hiding half the menu with nothing saying so.
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
              <ul>
                {g.muc.map((m) => {
                  const Icon = menuIcon(m.nhan);
                  if (m.duong === null) {
                    const info = PENDING_SCREENS[m.nhan];
                    return (
                      <li key={m.nhan} className="nav-sheet-item is-pending">
                        <span aria-disabled="true" className="nav-sheet-link">
                          <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />
                          <span className="nav-sheet-label">{m.nhan}</span>
                        </span>
                        {info !== undefined && <PendingMarker info={info} side="bottom" className="ml-auto" />}
                      </li>
                    );
                  }
                  const active = dangChon(m.duong, pathname);
                  return (
                    <li key={m.nhan} className="nav-sheet-item">
                      <Link
                        href={m.duong}
                        aria-current={active ? "page" : undefined}
                        className={active ? "nav-sheet-link is-active" : "nav-sheet-link"}
                        onClick={onNavigate}
                      >
                        <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />
                        <span className="nav-sheet-label">{m.nhan}</span>
                      </Link>
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
