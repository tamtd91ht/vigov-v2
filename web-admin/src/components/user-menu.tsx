"use client";

import * as PopoverPrimitive from "@radix-ui/react-popover";
import { ChevronDown, KeyRound, UserRound } from "lucide-react";
import Link from "next/link";

import { NutDangXuat } from "@/features/auth/nut-dang-xuat";
import { DUONG_DAN_DOI_MAT_KHAU } from "@/features/mat-khau/bat-doi-mat-khau";

import { RolePill } from "./role-pill";
import { userInitials } from "./user-initials";

/** `/ca-nhan` — the signed-in person's own settings page. */
export const PERSONAL_PAGE_PATH = "/ca-nhan";

/**
 * The header's person block — avatar + name (15/500) + caret (guide §7, §8.9) — opening the person's own
 * menu: who they are, the role they act with, `Đổi mật khẩu`, `Đăng xuất`. These are the same controls
 * the old topbar drew inline; only where they sit moved.
 *
 * Drawn ONLY with a read session (`khoiNguoiDung`): no fallback name, no fallback role — the caller
 * draws the bare sign-out control instead (`dau-trang.tsx`).
 *
 * A POPOVER, NOT A `DropdownMenu`: sign-out reports its failure as a sentence inside the block
 * (`NutDangXuat`), and a menu closes on select — the person would never read why they are still signed
 * in. `modal` stays false: a modal Radix popover injects a `<style>` element (ADR 0068 §4).
 *
 * Composition mirrors the prototype's `AppTopbar` account dropdown (ADR 0068 lần 5): the trigger shows
 * the name with the POSITION under it (the prototype's `title`); the menu opens with name + a second
 * line, a divider, `Hồ sơ cá nhân` → `/ca-nhan`, then sign-out. Two departures, both deliberate:
 * - the second line in the menu is the position, not the e-mail address the prototype prints there —
 *   a work address is personal data (rule 3) and `khoiNguoiDung` never carries it;
 * - `Đổi mật khẩu` stays as a menu item: the prototype has no change-password screen at all, and this
 *   link is the only voluntary way into `/doi-mat-khau`.
 * The role pill stays inside the menu, so the person can still see which authority they act with.
 */
export function UserMenu({ fullName, position, roleName }: { fullName: string; position: string; roleName: string | null }) {
  const initials = userInitials(fullName);
  return (
    <PopoverPrimitive.Root>
      <PopoverPrimitive.Trigger asChild>
        <button type="button" className="header-user">
          {initials !== "" && (
            <span className="topbar-avatar" aria-hidden="true">
              {initials}
            </span>
          )}
          <span className="header-user-text">
            <span className="ho-ten">{fullName}</span>
            {position !== "" && <span className="header-user-role">{position}</span>}
          </span>
          <ChevronDown aria-hidden="true" focusable="false" strokeWidth={1.8} className="header-user-caret" />
        </button>
      </PopoverPrimitive.Trigger>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Content
          align="end"
          sideOffset={8}
          collisionPadding={8}
          aria-label="Tài khoản"
          className="user-menu"
        >
          <div className="user-menu-person">
            <p className="ho-ten">{fullName}</p>
            <p className="chuc-vu">{position}</p>
          </div>
          {roleName !== null && <RolePill roleName={roleName} />}
          <div className="user-menu-actions">
            {/* The voluntary way into `/doi-mat-khau` — without it the page opens only when the server
                forces a change. It is the signed-in person's own password; no identity travels in the link. */}
            {/* The person's own page (`/ca-nhan`: Zalo reminders, ADR 0074). Same rule as below: no
                identity in the link — the page reads everything from the session. */}
            <PopoverPrimitive.Close asChild>
              <Link className="user-menu-item" href={PERSONAL_PAGE_PATH}>
                <UserRound aria-hidden="true" focusable="false" strokeWidth={1.8} />
                Hồ sơ cá nhân
              </Link>
            </PopoverPrimitive.Close>
            <PopoverPrimitive.Close asChild>
              <Link className="user-menu-item" href={DUONG_DAN_DOI_MAT_KHAU}>
                <KeyRound aria-hidden="true" focusable="false" strokeWidth={1.8} />
                Đổi mật khẩu
              </Link>
            </PopoverPrimitive.Close>
            <NutDangXuat />
          </div>
        </PopoverPrimitive.Content>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  );
}
