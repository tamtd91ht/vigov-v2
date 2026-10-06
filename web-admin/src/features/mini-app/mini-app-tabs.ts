import { QUYEN_QUAN_LY_NGUOI_DUNG, QUYEN_XEM_NOI_DUNG } from "@/lib/quyen";

/**
 * The two tabs of `/mini-app` — "Quản trị nội dung Mini App" — and the rules that pick one.
 *
 * WHY ONE SCREEN (ADR 0068 lần 5, 06/10/2026): the prototype (`vigov-require` `apps/admin`
 * `MiniAppWorkspace.tsx`) puts the content book and the staff directory under one screen, two tabs,
 * the open tab kept in `?tab=`. The owner asked for the prototype's structure; `/noi-dung` and
 * `/danh-ba` stay as redirects because they sit in bookmarks and chat messages.
 *
 * EACH TAB KEEPS ITS OWN KEY, AND THE KEYS ARE NOT MERGED: `content.read` is what every content read
 * route declares; the directory reads `GET /api/v1/staff`, which declares `admin.user` (`app/mini-app/
 * page.tsx`). One key for both tabs would show one of them to accounts the server refuses on the first
 * call (rule 5, invariant 3b). The MENU ITEM opens on EITHER key (`MINI_APP_MENU_KEYS`), exactly like
 * "Cấu hình" (`KHOA_MO_CAU_HINH`): it is the door to a screen whose tabs gate themselves.
 *
 * Hiding a tab is convenience, never the control: the server checks the key on every call.
 */

export type MiniAppTab = "noi-dung" | "danh-ba";

export type MiniAppTabSpec = {
  readonly key: MiniAppTab;
  /** The prototype's label, verbatim. */
  readonly label: string;
  /** The ONE key that shows the tab — the key its read route declares. */
  readonly permission: string;
};

/** Prototype order: Nội dung first, Danh bạ cán bộ second. */
export const MINI_APP_TABS: readonly MiniAppTabSpec[] = [
  { key: "noi-dung", label: "Nội dung", permission: QUYEN_XEM_NOI_DUNG },
  { key: "danh-ba", label: "Danh bạ cán bộ", permission: QUYEN_QUAN_LY_NGUOI_DUNG },
];

/**
 * The keys that open the menu item "Nội dung Mini App": one per tab, any one is enough. A closed list
 * of real `quyen` keys (rule 5, 3c), derived from the tabs so the door and the rooms cannot drift.
 */
export const MINI_APP_MENU_KEYS: readonly string[] = MINI_APP_TABS.map((t) => t.permission);

export const MINI_APP_PATH = "/mini-app";

/** The tab the URL asks for. Anything but exactly `danh-ba` (absent, repeated, unknown) is `noi-dung`. */
export function requestedTab(tab: string | string[] | undefined): MiniAppTab {
  return tab === "danh-ba" ? "danh-ba" : "noi-dung";
}

/** Link of one tab — the prototype writes `/mini-app?tab=<key>` for both. */
export function miniAppTabHref(tab: MiniAppTab): string {
  return `${MINI_APP_PATH}?tab=${tab}`;
}

/** Tabs this account may open, in prototype order. Exact key match (`coQuyen` semantics), no prefix. */
export function visibleTabs(permissions: readonly string[]): MiniAppTab[] {
  return MINI_APP_TABS.filter((t) => permissions.includes(t.permission)).map((t) => t.key);
}

/**
 * The tab actually shown: the one asked for when the account may open it, otherwise the first one it
 * may open — so a person holding only `admin.user` who follows a bare `/mini-app` link lands on the
 * directory, not on a refusal. `null` = neither tab is open to this account.
 */
export function landingTab(requested: MiniAppTab, visible: readonly MiniAppTab[]): MiniAppTab | null {
  if (visible.includes(requested)) return requested;
  return visible[0] ?? null;
}

/**
 * Where an old `/noi-dung` link goes: `/mini-app` with every query parameter kept except `tab` (that
 * page has one tab, the content tab, which is `/mini-app`'s default). Kept so a link that carried a
 * parameter still carries it; `/mini-app` reads only `tab` today.
 */
export function legacyContentRedirect(params: Readonly<Record<string, string | string[] | undefined>>): string {
  const query = new URLSearchParams();
  for (const [name, value] of Object.entries(params)) {
    if (name === "tab" || value === undefined) continue;
    for (const v of Array.isArray(value) ? value : [value]) query.append(name, v);
  }
  const s = query.toString();
  return s === "" ? MINI_APP_PATH : `${MINI_APP_PATH}?${s}`;
}

/**
 * Where an old `/danh-ba` link goes. No parameter is carried over: the directory's search text never
 * lives in a URL (rule 3, forbidden #4 — `features/danh-ba/khong-ro-ri-nguon.test.ts`), so a query on
 * an old link is something this app never wrote and must not start reading now.
 */
export const LEGACY_DIRECTORY_REDIRECT = miniAppTabHref("danh-ba");

/** Sentence when the account holds neither key — names both, so the admin knows what to grant. */
export const NO_MINI_APP_ACCESS =
  "Tài khoản của bạn không có quyền xem nội dung Mini App (content.read) hay quản lý người dùng " +
  "(admin.user), nên màn này không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";
