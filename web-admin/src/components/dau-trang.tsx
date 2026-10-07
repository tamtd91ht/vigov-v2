"use client";

import { Search } from "lucide-react";
import { usePathname } from "next/navigation";

import { NutDangXuat } from "@/features/auth/nut-dang-xuat";
import { khoiNguoiDung } from "@/features/phien/khoi-nguoi-dung";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { cn } from "@/lib/cn";

import { useCauHinhXa } from "./cau-hinh-xa";
import { CommuneIdentity } from "./commune-identity";
import { locMenu, NHOM_MENU } from "./muc-menu";
import { NavSheet } from "./nav-sheet";
import { NotificationBell } from "./notification-bell";
import { controlClass } from "./ui/field";
import { PendingMarker, type PendingFeatureInfo } from "./ui/pending-feature";
import { sessionRoleName } from "./role-pill";
import { SideNav } from "./side-nav";
import { useSidebarCollapsed } from "./sidebar-state";
import { UserMenu } from "./user-menu";

/**
 * The signed-in shell's chrome (spec 01, ADR 0068 §Sửa đổi 07/10/2026 lần 6 #2): the navy LEFT sidebar
 * running the full height of the window (`side-nav.tsx`), and beside it, on top of the page, the WHITE
 * 64px sticky header. Navigation lives in one place, the sidebar; the header carries who and where.
 *
 * HEADER LEFT: the commune's identity — "Ủy ban nhân dân" + the commune's name, verbatim, upper-cased by
 * CSS, the province under it (`CommuneIdentity`). Read at runtime from the commune's configuration,
 * never a constant. No product name here: "ViGov" lives only in the sidebar's brand block (lần 6 #3).
 *
 * HEADER MIDDLE: the system-wide search field, always visible from 768px — DISABLED with its "?",
 * because no route searches across modules yet (lần 5 #5, lần 6 #11: never a fake control).
 *
 * HEADER RIGHT: the bell, then the person block opening their own menu. Below 768px the sidebar is
 * hidden and the search and the person block move into the nav sheet; the bell stays.
 *
 * NO BANNER STRIP: the commune's web-admin banner under the header (ADR 0069 #5) is gone from the shell
 * — the prototype has none (lần 6 #8). The upload stays in Cấu hình until the owner decides (lần 6 f).
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
 * PLACEMENT is the shell grid's (`globals.css`, `.khung-trang`): the sidebar takes the first column over
 * every row, the header and the page's `<main>` share the second. The sidebar comes FIRST in the DOM, as
 * in the prototype, so Tab reaches the menu before the header's controls.
 *
 * `backdrop-blur` on the header: spec 01's `bg-white/95 backdrop-blur`, allowed by lần 6 #3 for this one
 * element (and the modal overlay) — `ui-foundation.test.ts` pins that it is nowhere else.
 */
export function DauTrang({ navigation = true, menuPath }: { navigation?: boolean; menuPath?: string }) {
  const xa = useCauHinhXa();
  const phien = usePhien();
  const routerPath = usePathname() ?? "/";
  // `menuPath`: the path the menu marks as current, when it is not the URL's. ONLY the dev preview passes
  // it — its URL is `/xem-thu/<real path>` and its menu must light the real item (`dev-preview/`).
  const pathname = menuPath ?? routerPath;
  const nguoi = khoiNguoiDung(phien);
  const roleName = sessionRoleName(phien);

  const permissions = phien === null ? null : phien.ok ? phien.duLieu.permissions : [];
  const groups = navigation ? locMenu(NHOM_MENU, permissions) : [];
  const sidebar = useSidebarCollapsed();

  return (
    <>
      {navigation && <SideNav groups={groups} pathname={pathname} collapsed={sidebar.collapsed} onToggle={sidebar.toggle} />}
      <header className="dau-trang sticky top-0 z-30 flex h-16 min-w-0 items-center gap-3 border-b border-line bg-white/95 px-4 backdrop-blur md:gap-5 md:px-7">
        <CommuneIdentity commune={xa} />
        <div className="header-search relative ml-2 hidden w-full max-w-100 min-w-40 md:block">
          <SystemSearchPlaceholder />
        </div>
        <div className="header-end ml-auto flex shrink-0 items-center gap-2">
          {nguoi.hien && <NotificationBell />}
          <div className="header-wide">
            {nguoi.hien ? (
              <UserMenu fullName={nguoi.hoTen} position={nguoi.chucVu} roleName={roleName} />
            ) : (
              <div className="header-logout">
                <NutDangXuat />
              </div>
            )}
          </div>
          <NavSheet groups={groups} pathname={pathname} person={nguoi} roleName={roleName} search={<SystemSearchPlaceholder />} />
        </div>
      </header>
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
 * The Phase-2 system-wide search (`docs/ui-ux/00` §3.1, `ROADMAP_PHASE2.md` item 1) in spec 01's
 * position and shape — the field with its magnifier, `h-10`, page-colour fill, 13px — drawn per ADR 0068
 * §14 DISABLED, with its "?". The same field in the header and in the narrow-screen sheet.
 *
 * A DISABLED native control, never a `<form>` or `role="search"` — there is nothing to submit, and a
 * search landmark that searches nothing misleads a screen-reader user. Nothing here calls a server or
 * stores anything. `bg-canvas`: the spec's `bg-surface` is the PAGE colour, which this app names
 * `canvas` (`globals.css`, `@theme`).
 */
function SystemSearchPlaceholder() {
  return (
    <div className="relative flex min-w-0 items-center" data-pending="">
      <Search
        aria-hidden="true"
        focusable="false"
        strokeWidth={1.8}
        className="pointer-events-none absolute top-1/2 left-3 z-[1] size-4 -translate-y-1/2 text-ink-muted"
      />
      <input
        type="search"
        disabled
        aria-label={SYSTEM_SEARCH_PENDING.ten}
        placeholder="Tìm nhiệm vụ, văn bản, phản ánh…"
        className={cn(controlClass, "h-10 bg-canvas pr-9 pl-9 text-[13px] disabled:bg-canvas md:text-[13px]")}
      />
      <PendingMarker info={SYSTEM_SEARCH_PENDING} phase2 placement="end" side="bottom" />
    </div>
  );
}
