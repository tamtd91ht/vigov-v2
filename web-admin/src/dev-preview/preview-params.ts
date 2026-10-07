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

/**
 * `?che-do=` of `/xem-thu/nhiem-vu` — the screen's own three view codes (`CheDoXem` of `SoNhiemVu`).
 * The real screen keeps its view in state, not in the URL, so the preview presses its real switch.
 */
export const TASK_PREVIEW_VIEWS = ["kanban", "danh-sach", "so-theo-doi"] as const;
export type TaskPreviewView = (typeof TASK_PREVIEW_VIEWS)[number];

/** `?modal=` of `/xem-thu/nhiem-vu` — Giao việc mới, the Excel import, the bulk-delete confirm. */
export const TASK_PREVIEW_MODALS = ["giao-viec", "nhap-excel", "xoa-nhieu"] as const;
export type TaskPreviewModal = (typeof TASK_PREVIEW_MODALS)[number];

export type PreviewSearchParams = Promise<Record<string, string | string[] | undefined>>;

/** Unknown or absent: Kanban, the real screen's own default. */
export function previewTaskView(value: string | string[] | undefined): TaskPreviewView {
  const v = first(value);
  return TASK_PREVIEW_VIEWS.find((x) => x === v) ?? "kanban";
}

export function previewTaskModal(value: string | string[] | undefined): TaskPreviewModal | null {
  const v = first(value);
  return TASK_PREVIEW_MODALS.find((x) => x === v) ?? null;
}

/**
 * `?chon=2` — tick two tasks so the bulk-delete bar shows. Only `2` is a word: the fixture names
 * exactly two codes to tick (`PREVIEW_SELECTED_CODES`), and any other count would be a guess.
 * `?modal=xoa-nhieu` implies it — the confirm has nothing to confirm without a selection.
 */
export function previewTaskSelect(value: string | string[] | undefined, modal: TaskPreviewModal | null): boolean {
  return first(value) === "2" || modal === "xoa-nhieu";
}

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
