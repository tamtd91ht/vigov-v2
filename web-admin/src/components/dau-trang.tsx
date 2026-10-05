"use client";

import { Search } from "lucide-react";
import { usePathname } from "next/navigation";

import { NutDangXuat } from "@/features/auth/nut-dang-xuat";
import { khoiNguoiDung } from "@/features/phien/khoi-nguoi-dung";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { useCauHinhXa } from "./cau-hinh-xa";
import { CommuneBanner } from "./commune-banner";
import { CommuneIdentity } from "./commune-identity";
import { locMenu, NHOM_MENU } from "./muc-menu";
import { NavSheet } from "./nav-sheet";
import { NotificationBell } from "./notification-bell";
import { PendingMarker, type PendingFeatureInfo } from "./ui/pending-feature";
import { sessionRoleName } from "./role-pill";
import { SideNav } from "./side-nav";
import { useSidebarCollapsed } from "./sidebar-state";
import { UserMenu } from "./user-menu";

/**
 * The signed-in shell's chrome: the navy header (68px, sticky — ADR 0068 §Sửa đổi 05/10/2026 (lần 2);
 * guide §7, §8.9), the commune's banner strip, and the LEFT sidebar carrying the module navigation
 * (owner, 05/10/2026: navigation back on the left, vertical — `side-nav.tsx`). The header no longer
 * carries module buttons: navigation lives in one place.
 *
 * HEADER LEFT: the commune's identity — "Ủy ban nhân dân", the commune's name verbatim, its logo when it
 * uploaded one (`CommuneIdentity`, ADR 0069). Never a product or vendor name (ADR 0068 §13). The name is
 * read at runtime from the commune's configuration, never a constant.
 *
 * HEADER RIGHT: the Phase-2 search placeholder, the bell, a divider, the person block opening their own
 * menu. Below 768px the sidebar is hidden and these (except the bell) collapse into the nav sheet.
 *
 * WHO SEES WHICH MENU ITEM is `locMenu`'s decision on the session's permissions, unchanged by where the
 * menu is drawn. `null` permissions = session NOT READ YET (three states, not two): only items needing no
 * key show, so the menu never shows an item and then withdraws it. The sidebar FRAME is drawn even then,
 * so the page body does not jump sideways when the session arrives.
 *
 * The person block appears only once the session is read (`khoiNguoiDung`) — no fallback name. Until
 * then, or when it cannot be read, the bare sign-out control stays: it is the way out for a person at
 * the wrong machine, and it needs no name.
 *
 * `navigation={false}`: the page draws no menu and no sidebar (`/doi-mat-khau`, where a forced change
 * must come first and every other screen would answer 403). Identity, bell and person block stay.
 *
 * ORDER OF SIBLINGS = the grid's order (`globals.css`, `.khung-trang`): header, banner, sidebar, then the
 * page's `<main>`. The banner follows the header as its SIBLING, not inside it: the header is sticky, and
 * a 112 px picture inside it would stay over every scrolled page.
 */
export function DauTrang({ navigation = true }: { navigation?: boolean }) {
  const xa = useCauHinhXa();
  const phien = usePhien();
  const pathname = usePathname() ?? "/";
  const nguoi = khoiNguoiDung(phien);
  const roleName = sessionRoleName(phien);

  const permissions = phien === null ? null : phien.ok ? phien.duLieu.permissions : [];
  const groups = navigation ? locMenu(NHOM_MENU, permissions) : [];
  const sidebar = useSidebarCollapsed();

  return (
    <>
      <header className="dau-trang">
        <div className="header-start">
          <CommuneIdentity commune={xa} />
        </div>
        <div className="header-end">
          <div className="header-wide">
            <SystemSearchPlaceholder />
          </div>
          {nguoi.hien && <NotificationBell />}
          <div className="header-wide">
            <span className="header-divider" aria-hidden="true" />
            {nguoi.hien ? (
              <UserMenu fullName={nguoi.hoTen} position={nguoi.chucVu} roleName={roleName} />
            ) : (
              <div className="header-logout">
                <NutDangXuat />
              </div>
            )}
          </div>
          <NavSheet groups={groups} pathname={pathname} person={nguoi} roleName={roleName} search={<SystemSearchPlaceholder inSheet />} />
        </div>
      </header>
      <CommuneBanner src={xa.webAdminBannerUrl} />
      {navigation && <SideNav groups={groups} pathname={pathname} collapsed={sidebar.collapsed} onToggle={sidebar.toggle} />}
    </>
  );
}

/**
 * Description behind the search placeholder's "?". A constant HERE, not a `PHAN_CHUA_DUNG` entry: the
 * shell is no screen of its own — `tools/tien_do_san_pham.py` counts `PHAN_CHUA_DUNG` per
 * `features/<dir>` and maps each page to the dirs it imports, so an entry in any feature would be
 * counted under that one screen, and an entry in every feature would count it eleven times. Like the
 * menu's `PENDING_SCREENS` (`muc-menu.ts`), the shell keeps its own one sentence.
 *
 * TRUE TODAY: the contract has staff search (`/api/v1/staff/searches`) and per-register filters, but
 * no route that searches across modules (tasks, documents, citizen reports) for one query.
 */
export const SYSTEM_SEARCH_PENDING: PendingFeatureInfo = {
  ten: "Tìm kiếm toàn hệ thống",
  viSao:
    "Một ô tìm được cùng lúc nhiệm vụ, văn bản và phản ánh, kết quả lọc theo quyền của từng người. " +
    "Hệ thống chưa tìm chung được trên các phân hệ; hiện mỗi sổ có bộ lọc riêng trên màn của nó.",
};

/**
 * The Phase-2 system-wide search (`docs/ui-ux/00` §3.1, `ROADMAP_PHASE2.md` item 1), drawn per ADR 0068
 * §14 as the control it will be, DISABLED, with its "?". In the navy header it is the guide's search
 * ICON (a field there would crowd the commune's name); in the narrow-screen sheet it is the
 * disabled field. Either way: a DISABLED native control, never a `<form>` or `role="search"` — there is
 * nothing to submit, and a search landmark that searches nothing misleads a screen-reader user. Nothing
 * here calls a server or stores anything.
 */
function SystemSearchPlaceholder({ inSheet = false }: { inSheet?: boolean }) {
  if (!inSheet) {
    return (
      <span className="header-module-pending" data-pending="">
        <button type="button" disabled aria-label={SYSTEM_SEARCH_PENDING.ten} className="header-icon-button is-disabled">
          <Search aria-hidden="true" focusable="false" strokeWidth={1.8} />
        </button>
        <PendingMarker info={SYSTEM_SEARCH_PENDING} phase2 placement="corner" side="bottom" nameInHover />
      </span>
    );
  }
  return (
    <div className="relative flex min-w-0 items-center" data-pending="">
      <Search
        aria-hidden="true"
        focusable="false"
        strokeWidth={1.8}
        className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-400"
      />
      <input
        type="search"
        disabled
        aria-label={SYSTEM_SEARCH_PENDING.ten}
        placeholder="Tìm nhiệm vụ, văn bản, phản ánh…"
        className="h-9 w-full min-w-0 cursor-not-allowed rounded-pill border-0 bg-canvas pr-9 pl-9 text-sm text-ink-500 placeholder:text-ink-400"
      />
      <PendingMarker info={SYSTEM_SEARCH_PENDING} phase2 placement="end" side="bottom" />
    </div>
  );
}
