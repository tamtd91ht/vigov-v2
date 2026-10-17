import { flattenMenu, locMenu, NHOM_MENU } from "@/components/muc-menu";
import { cacTabHien, SYSTEM_TABS, TAB_CAU_HINH } from "@/features/cau-hinh/thanh-tab-cau-hinh";
import { coQuyen } from "@/lib/quyen";

import type { GuideAction, GuideSection } from "./guide-content";

/** The read session as the tab gates take it — `null` = not read yet. */
export type GuideSession = Parameters<typeof cacTabHien>[1];

/**
 * Which parts of the guide this account sees (owner 17/10/2026: a menu the account cannot open is
 * HIDDEN in the guide too, with every button into it).
 *
 * ONE SOURCE FOR "CAN OPEN": the side menu's own filter (`locMenu` over `NHOM_MENU`). A second list of
 * keys here would drift from the menu the day an item's key changes — and the guide would then send a
 * reader to a screen the menu hides, or hide one it shows.
 *
 * Tabs of `/cau-hinh` and `/he-thong` use those screens' own gate functions (`cacTabHien`), for the same
 * reason. They need the read session; before it is read a gated tab link is not shown, so links can
 * only APPEAR once the session arrives, never appear and vanish.
 *
 * CONVENIENCE, NOT PROTECTION: every screen and every server route checks again (rule 5, forbidden #1).
 */
export type GuideAccess = {
  /** Menu routes visible to this account. */
  readonly routes: ReadonlySet<string>;
  /** `null` = session not read yet: no button needing an extra key is shown. */
  readonly permissions: readonly string[] | null;
  readonly configTabs: ReadonlySet<string>;
  readonly systemTabs: ReadonlySet<string>;
};

/**
 * `permissions` follows `locMenu`'s three states: `null` = not read yet, and then only the sections tied
 * to no menu show — the rule that keeps the sidebar from showing an item and then withdrawing it.
 */
export function guideAccess(permissions: readonly string[] | null, session: GuideSession): GuideAccess {
  const routes = new Set<string>();
  for (const m of flattenMenu(locMenu(NHOM_MENU, permissions))) {
    if (m.duong !== null) routes.add(m.duong);
  }
  return {
    routes,
    permissions,
    configTabs: new Set(cacTabHien(TAB_CAU_HINH, session).map((t) => t.ma)),
    systemTabs: new Set(cacTabHien(SYSTEM_TABS, session).map((t) => t.ma)),
  };
}

export function canSeeSection(section: GuideSection, access: GuideAccess): boolean {
  return section.routes === null || section.routes.some((r) => access.routes.has(r));
}

export function canSeeAction(action: GuideAction, access: GuideAccess): boolean {
  if (action.route !== null && !access.routes.has(action.route)) return false;
  if (action.permission !== undefined && (access.permissions === null || !coQuyen(access.permissions, action.permission))) {
    return false;
  }
  if (action.configTab !== undefined && !access.configTabs.has(action.configTab)) return false;
  if (action.systemTab !== undefined && !access.systemTabs.has(action.systemTab)) return false;
  return true;
}

/** The sections this account sees, each with only the buttons it can use — in guide order. */
export function visibleGuide(sections: readonly GuideSection[], access: GuideAccess): GuideSection[] {
  return sections
    .filter((s) => canSeeSection(s, access))
    .map((s) => ({ ...s, actions: s.actions.filter((a) => canSeeAction(a, access)) }));
}
