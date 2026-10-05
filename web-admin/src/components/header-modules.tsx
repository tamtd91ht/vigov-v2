import { Settings } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { PendingMarker } from "@/components/ui/pending-feature";
import { Tooltip } from "@/components/ui/tooltip";

import { menuIcon } from "./menu-icons";
import { dangChon, PENDING_SCREENS, type MucMenu, type NhomMenu } from "./muc-menu";

/**
 * The navy header's module row — icon-only buttons with a tooltip, REPLACING the text sidebar (ADR 0068
 * §Sửa đổi 05/10/2026 (lần 2) #6; guide §7, §8.2).
 *
 * WHAT THIS FILE DOES NOT DECIDE: which items exist, their order and who sees them. `groups` arrives
 * already filtered by `locMenu` (`muc-menu.ts`), exactly as the sidebar received it — so moving from a
 * sidebar to a header changed no permission outcome. Hiding an icon is UX; every route still checks
 * its key on the server (rule 5, forbidden #1).
 *
 * ONE ICON PER MENU ITEM, NO SUB-MENU POPOVER. The menu model has no child level on purpose
 * (`muc-menu.ts`, the `MAU_MUC` note: a fourth field on an item silently drops it from the product
 * progress count), and "Biên bản họp" / "Sổ tay lãnh đạo" each carry their own key. Folding them under
 * "Nhiệm vụ" would add a click to every daily screen and invent a parent the model does not have. The
 * groups of `NHOM_MENU` stay visible as thin dividers between runs of icons.
 *
 * PURE FUNCTION OF ITS PROPS (no session, no router), so every branch renders in a plain Node test.
 */

/**
 * The item drawn as the header's right-side settings button (guide §7: "Settings" sits on the right),
 * instead of in the module row. It is the same `NHOM_MENU` entry, same route, same key set — moved,
 * not duplicated. In the narrow-screen sheet it stays in its group like every other item.
 */
export const SETTINGS_LABEL = "Cấu hình";

/** Splits the settings item out of the filtered groups. A group left empty disappears, like `locMenu`. */
export function splitSettings(groups: readonly NhomMenu[]): { modules: readonly NhomMenu[]; settings: MucMenu | null } {
  let settings: MucMenu | null = null;
  const modules = groups
    .map((g) => ({
      ten: g.ten,
      muc: g.muc.filter((m) => {
        if (m.nhan !== SETTINGS_LABEL) return true;
        settings = m;
        return false;
      }),
    }))
    .filter((g) => g.muc.length > 0);
  return { modules, settings };
}

/**
 * Tooltips of the header open BELOW it. Same z as every other overlay of the app (`tooltip.tsx`,
 * z-60), which already sits above the sticky header (z-30).
 */
const TOOLTIP_SIDE = "bottom" as const;

export function HeaderModules({ groups, pathname }: { groups: readonly NhomMenu[]; pathname: string }) {
  if (groups.length === 0) return null;
  return (
    <nav className="header-modules" aria-label="Điều hướng chính">
      {groups.map((g, i) => (
        <ul key={g.ten === "" ? `untitled-${i}` : g.ten} className="header-module-group" aria-label={g.ten === "" ? undefined : g.ten}>
          {g.muc.map((m) => {
            const Icon = menuIcon(m.nhan);
            return (
              <li key={m.nhan}>
                <ModuleButton item={m} pathname={pathname} icon={<Icon aria-hidden="true" focusable="false" strokeWidth={1.8} />} />
              </li>
            );
          })}
        </ul>
      ))}
    </nav>
  );
}

/** The right-side settings button — the `Cấu hình` item, drawn with the guide's settings icon. */
export function SettingsButton({ item, pathname }: { item: MucMenu; pathname: string }) {
  return <ModuleButton item={item} pathname={pathname} icon={<Settings aria-hidden="true" focusable="false" strokeWidth={1.8} />} />;
}

function ModuleButton({ item, pathname, icon }: { item: MucMenu; pathname: string; icon: ReactNode }) {
  if (item.duong === null) {
    // An item listed before its screen exists (ADR 0068 §14): NOT a link and NOT a `<button disabled>` —
    // both take keyboard focus and then do nothing. A span with `aria-disabled` stays out of the tab order;
    // the one focusable thing is the "?", whose accessible name already names the item, and whose hover
    // reads "<item> — Tính năng đang phát triển" because the icon has no visible label.
    const info = PENDING_SCREENS[item.nhan];
    return (
      <span className="header-module-pending">
        <span aria-disabled="true" className="header-icon-button is-disabled">
          {icon}
          <span className="an-thi-giac">{item.nhan}</span>
        </span>
        {info !== undefined && <PendingMarker info={info} side={TOOLTIP_SIDE} placement="corner" nameInHover />}
      </span>
    );
  }
  const active = dangChon(item.duong, pathname);
  return (
    <Tooltip content={item.nhan} side={TOOLTIP_SIDE}>
      {/* `aria-label` IS the visible name's only carrier: the tooltip repeats it for a mouse, and touch or
          a screen reader never sees a tooltip (`tooltip.tsx`). */}
      <Link
        href={item.duong}
        aria-label={item.nhan}
        aria-current={active ? "page" : undefined}
        className={active ? "header-icon-button is-active" : "header-icon-button"}
      >
        {icon}
      </Link>
    </Tooltip>
  );
}
