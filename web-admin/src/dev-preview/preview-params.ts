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

/**
 * `?tab=` of `/xem-thu/van-ban` — the four tabs of Văn bản & Đơn thư. The real screen keeps its tab in
 * state, not in the URL, so the preview presses the real `[role=tab]`. Absent or unknown: nothing is
 * pressed, and the screen opens on its own default tab.
 */
export const DOCUMENT_PREVIEW_TABS = ["den", "di", "don-thu", "bao-cao"] as const;
export type DocumentPreviewTab = (typeof DOCUMENT_PREVIEW_TABS)[number];

/**
 * `?modal=` of `/xem-thu/van-ban` — the intake of Văn bản đến, the issue of Văn bản đi, the booking of
 * a citizen letter (`vao-so-don`, on the Đơn thư tab).
 */
export const DOCUMENT_PREVIEW_MODALS = ["vao-so-den", "cap-so-di", "vao-so-don"] as const;
export type DocumentPreviewModal = (typeof DOCUMENT_PREVIEW_MODALS)[number];

/** `?state=` of `/xem-thu/van-ban` — what the two register reads answer (`documents.fixture.ts`). */
export const DOCUMENT_PREVIEW_STATES = ["loading", "empty", "error"] as const;
export type DocumentPreviewStateWord = (typeof DOCUMENT_PREVIEW_STATES)[number];

export function previewDocumentTab(value: string | string[] | undefined): DocumentPreviewTab | null {
  const v = first(value);
  return DOCUMENT_PREVIEW_TABS.find((x) => x === v) ?? null;
}

export function previewDocumentModal(value: string | string[] | undefined): DocumentPreviewModal | null {
  const v = first(value);
  return DOCUMENT_PREVIEW_MODALS.find((x) => x === v) ?? null;
}

export function previewDocumentState(value: string | string[] | undefined): DocumentPreviewStateWord | null {
  const v = first(value);
  return DOCUMENT_PREVIEW_STATES.find((x) => x === v) ?? null;
}

/**
 * Three screenshot helpers of `/xem-thu/van-ban`, each one fixed word:
 *   `?scroll=right`  the register's own horizontal scroller pushed fully right (the last columns)
 *   `?them=1`        the intake dialog's "Thông tin thêm" fold opened (with `?modal=vao-so-den`)
 *   `?do-cao=1`      once a dialog is open, its measured height and the window's are written to
 *                    `<body data-preview-measure>` — read with `--dump-dom`, never drawn
 */
export function previewDocumentScrollRight(value: string | string[] | undefined): boolean {
  return first(value) === "right";
}

export function previewDocumentFoldOpen(value: string | string[] | undefined): boolean {
  return first(value) === "1";
}

export function previewDocumentMeasure(value: string | string[] | undefined): boolean {
  return first(value) === "1";
}

/**
 * `?dup=1` (with `?modal=vao-so-don`) — types a sender's name and a summary into the REAL booking
 * dialog, so its real duplicate check runs and the warning box shows (fixture candidates).
 */
export function previewDocumentDuplicate(value: string | string[] | undefined): boolean {
  return first(value) === "1";
}

/**
 * Four more one-word screenshot states of the citizen-letter tabs, each `=1`, each pressing or setting
 * the REAL control:
 *   `?buoc=1`           in a letter drawer: the first clickable status chip — the status composer
 *   `?sua-nguoi-gui=1`  in a letter drawer: "Sửa thông tin người gửi" — the sender-correction form
 *   `?loc=1`            on the Đơn thư tab: status filter "Đình chỉ" (no fixture row) — "Bỏ 1 bộ lọc"
 *                       and the empty-by-filter sentence
 *   `?cuon=1`           scrolls to the end: the drawer's result form (with its save button), or the
 *                       report's monthly bars
 */
export function previewDocumentFlag(value: string | string[] | undefined): boolean {
  return first(value) === "1";
}

/**
 * `?drawer=<id>` — open the detail of one incoming document. Only an id SHAPE is accepted (the
 * fixtures' ULID-like ids): the value goes into a CSS selector, and anything else opens nothing.
 */
export function previewDocumentDrawer(value: string | string[] | undefined): string | null {
  const v = first(value);
  return v !== undefined && /^[0-9A-Z]{10,40}$/.test(v) ? v : null;
}

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
