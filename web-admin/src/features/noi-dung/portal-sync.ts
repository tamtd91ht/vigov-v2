/**
 * Labels and pure rules of the §3 card "Đồng bộ tin từ Cổng thông tin điện tử" (ADR 0067 §2). The
 * component is `portal-sync-card.tsx`; the routes are `lib/api/portal-sync.ts`.
 *
 * WHAT IS DELIBERATELY NOT HERE:
 *   - The api_url rule (https, host ending `.gov.vn`, port 443, no query). The server owns it and answers
 *     400 with a sentence naming every condition; the screen gives a HINT, never a second copy of the
 *     check that could drift from `internal/portal.CheckURL`.
 *   - Six target kinds. §3 of the spec lists six; ADR 0067 §2 "Chế độ đăng" #2 allows exactly three
 *     (`tin-tuc` · `su-kien` · `thong-bao`), and the server refuses any other with a 400.
 *   - A text of the portal's own error. A run's `error_summary` holds a category and an error CLASS
 *     (`timeout`, `http-404`, …) — no article, no URL, no key, no personal data (rule 3). The class is
 *     shown as the server wrote it, as §3's sample `Các dự án: RemoteProtocolError` does.
 */

import type {
  comms_portalCategoriesIn,
  comms_portalCategoryIn,
  comms_portalCategoryTreeOut,
  comms_portalRunErrorOut,
  comms_portalRunOut,
  comms_portalSyncSettingsIn,
  comms_portalSyncSettingsOut,
} from "@/lib/api/schema.gen";

/* ── Card ──────────────────────────────────────────────────────────────────────────────────────── */

export const PORTAL_SYNC_TITLE = "Đồng bộ tin từ Cổng thông tin điện tử";
/** The config dialog's one sentence — verbatim, prototype `ContentSourcePanel.tsx:319`. */
export const PORTAL_SYNC_DESCRIPTION = "Tin đã đăng trên Cổng của xã sẽ tự về sổ tin này, khỏi phải gõ lại.";
/** The dialog's two footer buttons (prototype `ContentSourcePanel.tsx:556-563`). */
export const CANCEL_LABEL = "Huỷ";
export const SAVE_CONFIG_LABEL = "Lưu cấu hình";
export const RUN_NOW_LABEL = "Đồng bộ ngay";
export const CONFIGURE_LABEL = "Cấu hình";
/** The primary button before the commune connected anything (prototype `ContentSourcePanel.tsx:170`). */
export const CONNECT_LABEL = "Nối Cổng thông tin";
/** The card's line before the commune connected anything — verbatim, prototype `ContentSourcePanel.tsx:126`. */
export const NOT_CONNECTED =
  "Chưa nối. Xã nào đã có Cổng thông tin riêng thì khai báo ở đây để khỏi phải đăng tin hai lần.";
export const LOADING_SETTINGS = "Đang tải cấu hình đồng bộ…";

/**
 * The four states of the chip. ORDER MATTERS: without the platform's encryption key nothing can be
 * saved or run (503), whatever the commune configured — so that state wins over every other.
 */
export type PortalSyncStatus = "missing-encryption" | "not-configured" | "enabled" | "disabled";

export function portalSyncStatus(s: comms_portalSyncSettingsOut): PortalSyncStatus {
  if (!s.encryption_configured) return "missing-encryption";
  if (!s.configured) return "not-configured";
  return s.is_enabled ? "enabled" : "disabled";
}

const STATUS_LABEL: Readonly<Record<PortalSyncStatus, string>> = {
  "missing-encryption": "Thiếu khoá mã hoá",
  "not-configured": "Chưa cấu hình",
  enabled: "Đang bật",
  disabled: "Đang tắt",
};

export function portalSyncStatusLabel(st: PortalSyncStatus): string {
  return STATUS_LABEL[st];
}

/**
 * What a state means and what to do. No longer printed under the card's status line (owner D3,
 * 09/10/2026: the prototype's line is `Đang bật|Đang tắt · …` alone); the settings form still says the
 * `missing-encryption` one, since without it nothing can be saved.
 */
const STATUS_EXPLAINER: Readonly<Record<PortalSyncStatus, string>> = {
  "missing-encryption":
    "Nền tảng chưa có khoá mã hoá bí mật, nên chưa lưu hay dùng được mã bảo mật của Cổng. " +
    "Hãy báo đơn vị vận hành hệ thống.",
  "not-configured": "Xã chưa lưu cấu hình đồng bộ. Bấm “Cấu hình” để nhập địa chỉ API và mã bảo mật của Cổng.",
  enabled: "",
  disabled: "Lịch đồng bộ đang tắt — hệ thống không tự chạy. Vẫn chạy tay được bằng “⟳ Đồng bộ ngay”.",
};

export function portalSyncStatusExplainer(st: PortalSyncStatus): string {
  return STATUS_EXPLAINER[st];
}

/**
 * The prototype's interval names (`ContentSourcePanel.tsx:32-38`) for the hours it shares with the server;
 * every other hour of 0..24 reads `Mỗi N giờ`. Interval 0 is "manual only" (ADR 0067 §2 "Ghi" #4). The
 * card's status line and the dialog's select both read this, so the two never name one value twice.
 */
export function intervalLabel(hours: number): string {
  if (hours === 0) return "Chỉ chạy khi bấm tay";
  if (hours === 1) return "Mỗi giờ";
  if (hours === 24) return "Mỗi ngày";
  return `Mỗi ${hours} giờ`;
}

/**
 * The prototype's set (`ContentSourcePanel.tsx:32-38`) in the server's whole hours: manual, hourly, every
 * 6 hours, daily. Its `Mỗi 15 phút` has no hour value — the card draws it as a disabled option with a "?"
 * (`INTERVAL_15_MIN_PART`).
 */
export const INTERVAL_OPTIONS: readonly number[] = [0, 1, 6, 24];

/** The label of the prototype's 15-minute option, drawn disabled. */
export const INTERVAL_15_MIN_LABEL = "Mỗi 15 phút";

/**
 * The select's hour options. A stored interval outside the prototype's set (any of the server's 0..24) is
 * added in its place: dropping it would show another option as selected, and the next `Lưu cấu hình` would
 * silently change the commune's schedule.
 */
export function intervalOptions(current: number): number[] {
  const out = [...INTERVAL_OPTIONS];
  if (!out.includes(current)) out.push(current);
  return out.sort((a, b) => a - b);
}

export const PUBLISH_MODE_REVIEW = "cho-duyet";
export const PUBLISH_MODE_DIRECT = "dang-thang";

/** The meta line's spelling (§3: `đăng thẳng` / `chờ duyệt`, lower case). */
export function publishModeLabel(mode: string): string {
  if (mode === PUBLISH_MODE_REVIEW) return "chờ duyệt";
  if (mode === PUBLISH_MODE_DIRECT) return "đăng thẳng";
  return mode;
}

/**
 * The prototype's checkbox (`ContentSourcePanel.tsx:528-534`) in place of a two-option picker: ticked =
 * `dang-thang`, unticked = `cho-duyet`. A commune that never ticked it stays on `cho-duyet`, the default
 * of ADR 0067 §2 "Chế độ đăng" #1 (the prototype ticks it by default — that is the point ADR 0067 overrides).
 */
export const PUBLISH_DIRECT_LABEL = "Tin về thì đăng thẳng lên Mini App";
export const ENABLED_LABEL = "Bật đồng bộ tự động";
export const KEEP_SOURCE_LABEL = "Ghi rõ nguồn tin gốc";
/** Its note, verbatim (`ContentSourcePanel.tsx:542-545`); the checkbox itself is a "?" (`PHAN_CHUA_DUNG`). */
export const DOWNLOAD_IMAGES_NOTE =
  "Ảnh trên Cổng phục vụ qua http, mà Mini App chạy trên https sẽ chặn. Tắt mục này thì tin về không có ảnh.";

/* ── Runs ──────────────────────────────────────────────────────────────────────────────────────── */

const OUTCOME_LABEL: Readonly<Record<string, string>> = {
  "thanh-cong": "Thành công",
  "mot-phan": "Một phần",
  "that-bai": "Thất bại",
};

/** `""` is a run still in progress (portal_sync.go:147). An unknown code is shown verbatim. */
export function outcomeLabel(outcome: string): string {
  if (outcome === "") return "Đang chạy";
  return OUTCOME_LABEL[outcome] ?? outcome;
}

/** A run with no `finished_at` is still running (or crashed, until the server's reaper closes it). */
export function runIsUnfinished(run: comms_portalRunOut | undefined): boolean {
  return run !== undefined && run.finished_at === null;
}

/** §3's `bỏ qua {n}`: both kinds of skip — held by a live item, or by a soft-deleted one (never re-imported). */
export function skippedCount(run: comms_portalRunOut): number {
  return run.skipped_existing_count + run.skipped_deleted_count;
}

/** `{n} tin mới · bỏ qua {n} · lỗi {n}` — the counts of one run. */
export function runCountsLine(run: comms_portalRunOut): string {
  const parts = [`${run.imported_count} tin mới`, `bỏ qua ${skippedCount(run)}`];
  if (run.failed_count > 0) parts.push(`lỗi ${run.failed_count}`);
  return parts.join(" · ");
}

/** An error entry with no category is about the run as a whole (an interrupted run, a failed tree read). */
export const WHOLE_RUN = "Cả lượt";

/** `{Tên chuyên mục}: {mã lỗi}` (§3), plus the count when it happened more than once. */
export function runErrorLine(e: comms_portalRunErrorOut): string {
  const where = e.category_name === "" ? WHOLE_RUN : e.category_name;
  return e.count > 1 ? `${where}: ${e.error} (${e.count} lần)` : `${where}: ${e.error}`;
}

/* ── After `⟳ Đồng bộ ngay`: three re-reads, then silence ─────────────────────────────────────── */

/** The toasts of `Đồng bộ ngay` — verbatim, prototype `ContentSourcePanel.tsx:84-88`. */
export const RUN_STARTED_TOAST = "Đang lấy tin về. Vài chục giây nữa mở lại màn hình này để xem kết quả.";
export const RUN_REFUSED_TOAST = "Không xếp được lượt đồng bộ.";

/**
 * After a 202 the card re-reads itself (settings + newest run) at these delays, then stops (owner,
 * 09/10/2026 — replaces the old 5-second polling). NOT FOREVER: a run on a slow portal can take many
 * minutes, and an open tab polling until it ends is a request every few seconds per open admin screen of
 * every commune. Three reads cover a normal run; a longer one is seen the next time the screen opens,
 * which is what the toast tells the officer.
 */
export const REFETCH_DELAYS_MS: readonly number[] = [4000, 15000, 45000];

/**
 * The wait after 503 `portal_sync_busy`, from the server's `Retry-After` (60 s today). Said, never acted
 * on: the screen does not retry by itself — the officer presses again.
 */
export function retryWaitLabel(seconds: number): string {
  if (seconds < 60) return `Có thể bấm lại sau khoảng ${seconds} giây.`;
  return `Có thể bấm lại sau khoảng ${Math.ceil(seconds / 60)} phút.`;
}

/** Why `⟳ Đồng bộ ngay` is off, or `null` when it is on. UX only — the server answers 409/503 anyway. */
export function runNowBlockedReason(
  s: comms_portalSyncSettingsOut,
  latest: comms_portalRunOut | undefined,
): string | null {
  if (!s.encryption_configured) return "Chưa chạy được: nền tảng thiếu khoá mã hoá bí mật.";
  if (!s.configured) return "Chưa chạy được: xã chưa lưu cấu hình đồng bộ.";
  if (runIsUnfinished(latest)) return "Đang có một lượt đồng bộ chạy. Hãy chờ lượt ấy xong.";
  return null;
}

/* ── Settings form ─────────────────────────────────────────────────────────────────────────────── */

/**
 * What the form holds. Numbers are the boxes' TEXT, parsed on save, so a half-typed number is not
 * silently turned into 0.
 *
 * `api_key` STARTS EMPTY, ALWAYS. The server never returns the key (only `api_key_set`), and the form
 * never invents a placeholder value that could be sent back as if it were one.
 */
export type SettingsForm = {
  api_url: string;
  api_key: string;
  publish_mode: string;
  interval_hours: number;
  window_days: string;
  max_items_per_run: string;
  keep_source_credit: boolean;
  is_enabled: boolean;
};

/** The form starts from the server's values — on a first save those ARE the owner's defaults (6 h, 90 days, 100). */
export function formFromSettings(s: comms_portalSyncSettingsOut): SettingsForm {
  return {
    api_url: s.api_url,
    api_key: "",
    publish_mode: s.publish_mode === "" ? PUBLISH_MODE_REVIEW : s.publish_mode,
    interval_hours: s.interval_hours,
    window_days: String(s.window_days),
    max_items_per_run: String(s.max_items_per_run),
    keep_source_credit: s.keep_source_credit,
    is_enabled: s.is_enabled,
  };
}

/**
 * When the key MUST be typed: `first` (nothing stored) or `new-url` (ADR 0067 §2 decision 5 — the stored
 * key never follows the configuration to another address). Compared after trimming, exactly as
 * `domain.PortalAPIURLChanged` does. `null`: blank keeps the stored key.
 */
export type KeyRequirement = "first" | "new-url" | null;

export function keyRequirement(s: comms_portalSyncSettingsOut, form: SettingsForm): KeyRequirement {
  if (!s.configured || !s.api_key_set) return "first";
  if (form.api_url.trim() !== s.api_url.trim()) return "new-url";
  return null;
}

/*
 * The prototype's hints (`ContentSourcePanel.tsx:337-341`). Its configured hint reads `Đang dùng {hint}.
 * Để trống nếu không đổi.`; `{hint}` is drawn as a disabled "?" (`KEY_IN_USE_PART`): the settings response
 * carries `api_key_set` and nothing derived from the key (backend gap — no `secret_hint` field exists), and
 * a hint built here would have to come from the key itself, which never comes back.
 */
export const KEY_IN_USE_PREFIX = "Đang dùng";
export const KEY_SAVED_HINT = "Để trống nếu không đổi.";
export const KEY_FIRST_HINT = "Do đơn vị vận hành Cổng cấp.";
/** Stricter than the prototype, and kept: ADR 0067 §2 decision 5 — the stored key never follows a new address. */
export const KEY_NEW_URL_HINT =
  "Đã đổi địa chỉ API thì phải nhập lại mã bảo mật: mã cũ không được gửi tới một địa chỉ chưa từng dùng nó.";
export const KEY_KEEP_PLACEHOLDER = "Giữ nguyên mã cũ";
/**
 * The address box's placeholder: the shape of a Đà Nẵng portal address with `<tên-xã>` where the
 * prototype names one commune — one bundle serves every commune (rule 1, invariant 10).
 */
export const API_URL_PLACEHOLDER = "https://<tên-xã>.danang.gov.vn/DesktopModules/cttdt/api/apichiase";

/** The prototype's three refusals on `Lưu cấu hình` (`ContentSourcePanel.tsx:273-285`), verbatim. */
export const API_URL_REQUIRED = "Nhập địa chỉ API của Cổng.";
export const KEY_REQUIRED = "Nhập mã bảo mật do Cổng cấp.";
export const CATEGORIES_REQUIRED_TO_ENABLE = "Chọn ít nhất một chuyên mục trước khi bật đồng bộ.";

export function keyHint(s: comms_portalSyncSettingsOut, form: SettingsForm): string {
  const req = keyRequirement(s, form);
  if (req === "first") return KEY_FIRST_HINT;
  if (req === "new-url") return KEY_NEW_URL_HINT;
  return KEY_SAVED_HINT;
}

/**
 * The owner's CEILINGS (02/10/2026, D1) — `domain.PortalWindowDaysMax`, `PortalMaxItemsMax` and
 * `PortalSelectedCategoriesMax` in service-comms. Copied here ONLY as a hint (the inputs' `max`, the
 * `n/30` counter); the server is where a value is refused (400 / 422, shown verbatim).
 * `portal-sync.test.ts` reads the Go constants and turns red the day they move.
 */
export const WINDOW_DAYS_MAX = 90;
export const MAX_ITEMS_PER_RUN_MAX = 100;
export const SELECTED_CATEGORIES_MAX = 30;

/** The prototype's unit hints under the two number boxes (`ContentSourcePanel.tsx:497,509`). */
export const WINDOW_DAYS_HINT = "ngày trở lại";
export const MAX_ITEMS_HINT = "tin";
export const WINDOW_DAYS_ERROR = `Số ngày lấy tin phải là một số nguyên từ 1 tới ${WINDOW_DAYS_MAX}.`;
export const MAX_ITEMS_ERROR = `Số tin tối đa mỗi lượt phải là một số nguyên từ 1 tới ${MAX_ITEMS_PER_RUN_MAX}.`;

/** An integer in 1..max typed as text, or `null` (half-typed, zero, above the ceiling). */
function intInRange(text: string, max: number): number | null {
  const t = text.trim();
  if (!/^\d+$/.test(t)) return null;
  const n = Number.parseInt(t, 10);
  return n >= 1 && n <= max ? n : null;
}

/** Which box a refusal belongs to — the sentence is drawn under that box. */
export type SettingsField = "api_url" | "api_key" | "window_days" | "max_items_per_run" | "categories";

/**
 * The first refusal of `Lưu cấu hình`, or `null`: what the officer can see is missing or past a ceiling —
 * the url shape stays the server's 400, shown verbatim.
 *
 * `selected` is how many categories are ticked, or `null` when that is not known (the tree could not be
 * read): then the categories rule is the server's, not guessed here. A commune not yet configured has no
 * tree, so it passes `0` — the prototype refuses to switch sync on with nothing chosen, because a call to
 * the portal with an empty category list drags every record back.
 */
export function settingsFormError(
  s: comms_portalSyncSettingsOut,
  form: SettingsForm,
  selected: number | null,
): { field: SettingsField; text: string } | null {
  if (form.api_url.trim() === "") return { field: "api_url", text: API_URL_REQUIRED };
  const req = keyRequirement(s, form);
  if (req !== null && form.api_key === "") {
    return { field: "api_key", text: req === "first" ? KEY_REQUIRED : KEY_NEW_URL_HINT };
  }
  if (form.is_enabled && selected === 0) return { field: "categories", text: CATEGORIES_REQUIRED_TO_ENABLE };
  if (intInRange(form.window_days, WINDOW_DAYS_MAX) === null) return { field: "window_days", text: WINDOW_DAYS_ERROR };
  if (intInRange(form.max_items_per_run, MAX_ITEMS_PER_RUN_MAX) === null) {
    return { field: "max_items_per_run", text: MAX_ITEMS_ERROR };
  }
  return null;
}

/**
 * The PUT body. THE WHOLE FORM (PUT, not PATCH — routes_portal_sync.go:62), except the key, which goes
 * only when typed. Call after `settingsFormError` returned `null`.
 *
 * THE KEY IS NOT TRIMMED: a key with a space is refused by the server's shape check with a sentence; a
 * trimmed one would be a different key, saved without a word.
 */
export function settingsBody(form: SettingsForm): comms_portalSyncSettingsIn {
  const body: comms_portalSyncSettingsIn = {
    api_url: form.api_url.trim(),
    publish_mode: form.publish_mode,
    interval_hours: form.interval_hours,
    window_days: intInRange(form.window_days, WINDOW_DAYS_MAX),
    max_items_per_run: intInRange(form.max_items_per_run, MAX_ITEMS_PER_RUN_MAX),
    keep_source_credit: form.keep_source_credit,
    is_enabled: form.is_enabled,
  };
  if (form.api_key !== "") body.api_key = form.api_key;
  return body;
}

/* ── Category tree ─────────────────────────────────────────────────────────────────────────────── */

export const TARGET_KIND_OPTIONS: readonly { value: string; label: string }[] = [
  { value: "tin-tuc", label: "Tin tức" },
  { value: "su-kien", label: "Sự kiện" },
  { value: "thong-bao", label: "Thông báo" },
];

/** A newly ticked category starts as `Tin tức` — the server refuses an entry without a valid kind. */
export const DEFAULT_TARGET_KIND = "tin-tuc";

/** Verbatim, prototype `ContentSourcePanel.tsx:355-394`. */
export const CATEGORIES_TITLE = "Chuyên mục lấy về";
export const CATEGORIES_NEED_SETTINGS =
  "Lưu cấu hình một lần trước, rồi mở lại để chọn chuyên mục — danh sách này hỏi thẳng từ Cổng nên cần mã bảo mật.";
export const CATEGORIES_UNREACHABLE = "Không gọi được sang Cổng. Kiểm tra lại địa chỉ và mã bảo mật.";
/** For a screen reader beside the skeleton — the skeleton itself says nothing. */
export const CATEGORIES_LOADING = "Đang hỏi Cổng danh sách chuyên mục…";
export const CATEGORIES_EMPTY = "Cổng không trả về chuyên mục nào.";
/** Beside the server's 403 sentence when the tree needs `content.update` (D3) and the account lacks it. */
export const CATEGORIES_NEED_UPDATE = "Cần quyền sửa nội dung để xem cây chuyên mục.";
export const CATEGORIES_LIMIT_REACHED =
  `Đã chọn đủ ${SELECTED_CATEGORIES_MAX} chuyên mục — mức tối đa của mỗi xã. Bỏ chọn một chuyên mục để chọn chuyên mục khác.`;
/** The success toast of `Lưu cấu hình` — verbatim, prototype `ContentSourcePanel.tsx:300`. */
export const SETTINGS_SAVED = "Đã lưu cấu hình nguồn tin.";
/** The group of categories with no parent on the portal — the prototype's `group_name || "Khác"`. */
export const NO_PARENT_GROUP = "Khác";

/** One row of the picker: a portal category (live) or a stored one the portal no longer lists. */
export type CategoryChoice = {
  external_id: string;
  name: string;
  parent_id: string;
  /** The parent's name on the portal — the group heading. `""` for a root or a row the portal dropped. */
  parent_name: string;
  selected: boolean;
  target_kind: string;
  on_portal: boolean;
};

/** The picker's starting state, in the portal's order, then the missing ones. Keyed by `external_id`. */
export function choicesFromTree(tree: comms_portalCategoryTreeOut): CategoryChoice[] {
  const out: CategoryChoice[] = tree.items.map((n) => ({
    external_id: n.external_id,
    name: n.name,
    parent_id: n.parent_id,
    parent_name: n.parent_name,
    selected: n.is_selected,
    target_kind: n.target_kind,
    on_portal: true,
  }));
  const seen = new Set(out.map((c) => c.external_id));
  for (const m of tree.missing) {
    if (seen.has(m.external_id)) continue;
    out.push({
      external_id: m.external_id,
      name: m.name,
      parent_id: "",
      parent_name: "",
      selected: m.is_selected,
      target_kind: m.target_kind,
      on_portal: false,
    });
  }
  return out;
}

/**
 * The picker's groups, by parent, in the order the portal listed them (prototype `groupOptions`,
 * `ContentSourcePanel.tsx:53-67`). NOT FOR TIDINESS: a commune's portal has two categories both called
 * "Chuyển đổi số" under different parents, and a flat list shows two identical lines.
 *
 * KEYED BY THE PARENT'S ID, LABELLED BY ITS NAME: two parents sharing a name stay two groups. A row with
 * no parent — a root, or a stored row the portal no longer lists — goes under `Khác`.
 */
export function groupChoices(choices: readonly CategoryChoice[]): { key: string; name: string; items: CategoryChoice[] }[] {
  const order: string[] = [];
  const groups = new Map<string, { key: string; name: string; items: CategoryChoice[] }>();
  for (const c of choices) {
    const hasParent = c.parent_id !== "" && c.parent_id !== "0" && c.parent_name !== "";
    const key = hasParent ? `p:${c.parent_id}` : "";
    let g = groups.get(key);
    if (g === undefined) {
      g = { key, name: hasParent ? c.parent_name : NO_PARENT_GROUP, items: [] };
      groups.set(key, g);
      order.push(key);
    }
    g.items.push(c);
  }
  return order.map((k) => groups.get(k)!);
}

/**
 * Tick or untick a batch (`chọn cả mục` / `bỏ cả mục`, prototype `pickMany`). Ticking stops at the
 * ceiling exactly as `Chọn tất cả` does; a row already ticked keeps the kind the officer set.
 */
export function pickMany(
  choices: readonly CategoryChoice[],
  ids: ReadonlySet<string>,
  take: boolean,
  defaultKind: string,
): CategoryChoice[] {
  if (!take) return choices.map((c) => (ids.has(c.external_id) ? { ...c, selected: false } : c));
  let n = selectedCount(choices);
  return choices.map((c) => {
    if (!ids.has(c.external_id) || c.selected || n >= SELECTED_CATEGORIES_MAX) return c;
    n++;
    return { ...c, selected: true, target_kind: c.target_kind === "" ? defaultKind : c.target_kind };
  });
}

/**
 * The PUT categories body: ONLY the rows that changed — a tick added or removed, or the kind of a
 * selected row changed. The server leaves rows not in the body as they are (routes_portal_sync.go:104),
 * so sending the 60 untouched ones would only file 60 checks for nothing.
 *
 * Every entry carries a valid kind, unticked ones included: the server checks the kind of every entry.
 */
export function categoriesBody(
  initial: readonly CategoryChoice[],
  current: readonly CategoryChoice[],
): comms_portalCategoriesIn {
  const before = new Map(initial.map((c) => [c.external_id, c]));
  const categories: comms_portalCategoryIn[] = [];
  for (const c of current) {
    const b = before.get(c.external_id);
    const wasSelected = b?.selected ?? false;
    const kind = c.target_kind === "" ? DEFAULT_TARGET_KIND : c.target_kind;
    const kindBefore = b === undefined || b.target_kind === "" ? DEFAULT_TARGET_KIND : b.target_kind;
    const changed = c.selected !== wasSelected || (c.selected && kind !== kindBefore);
    if (!changed) continue;
    categories.push({ external_id: c.external_id, name: c.name, target_kind: kind, is_selected: c.selected });
  }
  return { categories };
}

/**
 * How many are ticked — EVERY ticked row, the ones the portal no longer lists included: the server's
 * ceiling counts every stored selection (`domain.CheckPortalSelectedCount`), so a hidden one would make
 * `29/30` answer 422.
 */
export function selectedCount(choices: readonly CategoryChoice[]): number {
  return choices.filter((c) => c.selected).length;
}

/**
 * `đã chọn {x}/{y}` — the prototype's counter (`ContentSourcePanel.tsx:360-362`): ticked over the rows of
 * the list. The ceiling of 30 is said by `CATEGORIES_LIMIT_REACHED` when it is reached.
 */
export function selectedCountLabel(choices: readonly CategoryChoice[]): string {
  return `đã chọn ${selectedCount(choices)}/${choices.length}`;
}

/**
 * `Chọn tất cả` under the ceiling: ticks the portal's rows in the order given until 30 are ticked, the
 * already-ticked ones counted first. Never more — the hint is the client's; the server's 422 decides.
 */
export function selectAllUpToLimit(choices: readonly CategoryChoice[], defaultKind: string): CategoryChoice[] {
  let n = selectedCount(choices);
  return choices.map((c) => {
    if (!c.on_portal || c.selected || n >= SELECTED_CATEGORIES_MAX) return c;
    n++;
    return { ...c, selected: true, target_kind: c.target_kind === "" ? defaultKind : c.target_kind };
  });
}
