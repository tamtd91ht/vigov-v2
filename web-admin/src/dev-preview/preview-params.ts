/**
 * Query words of the dev-only preview, read on the server by its pages (a plain module, not a client one:
 * a server component importing a value from a "use client" file receives a reference, not the value).
 * Route-parameter words in Vietnamese, as every URL of this app (rule 12 #2).
 */

/** `?modal=` of `/xem-thu/giai-ngan` — one per dialog of the list screen. */
export const PREVIEW_MODALS = ["hang-muc", "them-du-an", "nhap-excel", "them-nguon", "chi-tiet-nguon", "xoa-nhieu"] as const;
export type PreviewModal = (typeof PREVIEW_MODALS)[number];

/** `?tab=` of `/xem-thu/giai-ngan/du-an` — the four tabs, `vuong-mac` (the default) first. */
export const PREVIEW_TAB_NAMES = ["vuong-mac", "chung-tu", "bieu-do", "trao-doi"] as const;
export type PreviewTab = (typeof PREVIEW_TAB_NAMES)[number];

export type PreviewSearchParams = Promise<Record<string, string | string[] | undefined>>;

function first(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

/** An unknown word opens nothing — never a guessed dialog. */
export function previewModal(value: string | string[] | undefined): PreviewModal | null {
  const v = first(value);
  return PREVIEW_MODALS.find((m) => m === v) ?? null;
}

export function previewTab(value: string | string[] | undefined): PreviewTab {
  const v = first(value);
  return PREVIEW_TAB_NAMES.find((t) => t === v) ?? "vuong-mac";
}

/** `?menu=day-du`: the sidebar with every menu item, for a side-by-side with the prototype's full menu. */
export function previewFullMenu(value: string | string[] | undefined): boolean {
  return first(value) === "day-du";
}

/**
 * Shell-state words, read by `PreviewShell` on the client (every preview page has the shell, so no page
 * needs to forward them). Each makes ONE screenshot state deterministic:
 *   `?sidebar=thu-gon`      the sidebar collapsed to icons (any other value, or none: expanded)
 *   `?toast=1`              one success toast, kept on screen
 *   `?menu-tai-khoan=1`     the account menu open
 *   `?chuong=1`             the notification bell's panel open
 * Anything else opens nothing — the same rule as `previewModal`.
 */
export type PreviewShellState = {
  readonly sidebarCollapsed: boolean;
  readonly toast: boolean;
  readonly accountMenu: boolean;
  readonly bellPanel: boolean;
};

/** The toast `?toast=1` shows — a Giải ngân success sentence, only to have one toast on screen. */
export const PREVIEW_TOAST = "Đã thêm hạng mục.";

export function previewShellState(get: (name: string) => string | null): PreviewShellState {
  return {
    sidebarCollapsed: get("sidebar") === "thu-gon",
    toast: get("toast") === "1",
    accountMenu: get("menu-tai-khoan") === "1",
    bellPanel: get("chuong") === "1",
  };
}
