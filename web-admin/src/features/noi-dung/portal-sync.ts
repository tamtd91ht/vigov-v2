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
export const PORTAL_SYNC_DESCRIPTION =
  "Tin đã đăng trên Cổng thông tin của xã sẽ tự về sổ tin này, khỏi phải gõ lại.";
export const RUN_NOW_LABEL = "⟳ Đồng bộ ngay";
export const CONFIGURE_LABEL = "Cấu hình";
export const RELOAD_LABEL = "Tải lại";
export const HISTORY_LABEL = "Lịch sử đồng bộ";
export const LOADING_SETTINGS = "Đang tải cấu hình đồng bộ…";
export const NO_RUN_YET = "Chưa chạy lượt đồng bộ nào.";

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

/** `globals.css` has a green and a grey chip only; the TEXT says which of the three grey states it is. */
export function portalSyncStatusChipClass(st: PortalSyncStatus): string {
  return st === "enabled" ? "chip chip-hoat-dong" : "chip chip-ngung";
}

/** One sentence under the chip saying what the state means and what to do. */
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

/** Interval 0 is "manual only" (ADR 0067 §2 "Ghi" #4). */
export function intervalLabel(hours: number): string {
  return hours === 0 ? "Chỉ chạy tay" : `Mỗi ${hours} giờ`;
}

/** 0..24 — the select's options, in order. The server's bound (`PortalIntervalMax`), stated once. */
export const INTERVAL_OPTIONS: readonly number[] = Array.from({ length: 25 }, (_, i) => i);

export const PUBLISH_MODE_REVIEW = "cho-duyet";
export const PUBLISH_MODE_DIRECT = "dang-thang";

/** The meta line's spelling (§3: `đăng thẳng` / `chờ duyệt`, lower case). */
export function publishModeLabel(mode: string): string {
  if (mode === PUBLISH_MODE_REVIEW) return "chờ duyệt";
  if (mode === PUBLISH_MODE_DIRECT) return "đăng thẳng";
  return mode;
}

/**
 * The two radio options of the form, each with the sentence saying what it does to the residents.
 * `Chờ duyệt` first and marked default: ADR 0067 §2 "Chế độ đăng" #1.
 */
export const PUBLISH_MODE_OPTIONS: readonly { value: string; label: string; explainer: string }[] = [
  {
    value: PUBLISH_MODE_REVIEW,
    label: "Chờ duyệt (mặc định)",
    explainer:
      "Tin về từ Cổng nằm trong sổ ở trạng thái “Chờ duyệt”; bà con chưa thấy cho tới khi cán bộ mở bài, " +
      "tích “Đăng lên Mini App” rồi Lưu.",
  },
  {
    value: PUBLISH_MODE_DIRECT,
    label: "Đăng thẳng",
    explainer:
      "Tin về từ Cổng hiện ngay cho bà con trên Mini App, không qua bước duyệt. Chỉ chọn khi xã tin " +
      "mọi tin trên Cổng đều đăng được cho dân.",
  },
];

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

export function outcomeChipClass(outcome: string): string {
  return outcome === "thanh-cong" ? "chip chip-hoat-dong" : "chip chip-ngung";
}

const TRIGGER_LABEL: Readonly<Record<string, string>> = {
  "theo-lich": "Theo lịch",
  "chay-tay": "Chạy tay",
};

export function triggerLabel(kind: string): string {
  return TRIGGER_LABEL[kind] ?? kind;
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

/* ── Bounded polling after `⟳ Đồng bộ ngay` ───────────────────────────────────────────────────── */

/**
 * After a 202 the screen re-reads the history a FEW times, then stops. NOT FOREVER: a run on a slow
 * portal can take many minutes (100 articles, images through ClamAV), and an open tab polling until it
 * ends is a request every few seconds per open admin screen of every commune. Twelve reads five seconds
 * apart cover a normal run; after that the officer presses `Tải lại`.
 */
export const POLL_INTERVAL_MS = 5000;
export const POLL_MAX_ATTEMPTS = 12;
export const POLL_GAVE_UP =
  "Lượt đồng bộ vẫn đang chạy ở máy chủ. Màn hình đã thôi tự kiểm tra — bấm “Tải lại” để xem kết quả.";
export const RUN_STARTED = "Đã mở một lượt đồng bộ. Kết quả sẽ hiện ở đây khi lượt chạy xong.";

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

export const KEY_SAVED_HINT = "Đã lưu khoá. Để trống nếu không đổi.";
export const KEY_FIRST_HINT = "Chưa có mã bảo mật — bắt buộc nhập ở lần lưu đầu tiên.";
export const KEY_NEW_URL_HINT =
  "Đã đổi địa chỉ API thì phải nhập lại mã bảo mật: mã cũ không được gửi tới một địa chỉ chưa từng dùng nó.";
export const API_URL_HINT =
  "Bắt đầu bằng https://, tên máy kết thúc bằng .gov.vn — ví dụ https://<tên-xã>.danang.gov.vn/DesktopModules/cttdt/api/apichiase";

export function keyHint(s: comms_portalSyncSettingsOut, form: SettingsForm): string {
  const req = keyRequirement(s, form);
  if (req === "first") return KEY_FIRST_HINT;
  if (req === "new-url") return KEY_NEW_URL_HINT;
  return KEY_SAVED_HINT;
}

function positiveInt(text: string): number | null {
  const t = text.trim();
  if (!/^\d+$/.test(t)) return null;
  const n = Number.parseInt(t, 10);
  return n >= 1 ? n : null;
}

/**
 * The sentence that holds `Lưu cấu hình` off, or `null`. Only what the officer can see is missing — the
 * value rules (url shape, upper bounds) are the server's 400, shown verbatim.
 */
export function settingsFormError(s: comms_portalSyncSettingsOut, form: SettingsForm): string | null {
  if (form.api_url.trim() === "") return "Hãy nhập địa chỉ API của Cổng.";
  const req = keyRequirement(s, form);
  if (req !== null && form.api_key === "") return req === "first" ? KEY_FIRST_HINT : KEY_NEW_URL_HINT;
  if (positiveInt(form.window_days) === null) return "Số ngày lấy tin phải là một số nguyên từ 1 trở lên.";
  if (positiveInt(form.max_items_per_run) === null) return "Số tin tối đa mỗi lượt phải là một số nguyên từ 1 trở lên.";
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
    window_days: positiveInt(form.window_days),
    max_items_per_run: positiveInt(form.max_items_per_run),
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

export const MISSING_ON_PORTAL = "không còn trên cổng";
export const CATEGORIES_NEED_SETTINGS =
  "Lưu địa chỉ API và mã bảo mật trước — sau đó danh sách chuyên mục của Cổng sẽ hiện ở đây.";
export const CATEGORIES_LOADING = "Đang hỏi Cổng danh sách chuyên mục…";
export const CATEGORIES_EMPTY = "Cổng không trả về chuyên mục nào.";
export const CATEGORIES_NOTHING_CHANGED = "Không có chuyên mục nào thay đổi — chưa gửi gì.";
export const CATEGORIES_SAVED = "Đã lưu lựa chọn chuyên mục.";
export const SETTINGS_SAVED = "Đã lưu cấu hình đồng bộ.";

/** One row of the picker: a portal category (live) or a stored one the portal no longer lists. */
export type CategoryChoice = {
  external_id: string;
  name: string;
  parent_id: string;
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
      selected: m.is_selected,
      target_kind: m.target_kind,
      on_portal: false,
    });
  }
  return out;
}

/**
 * Order the choices as a tree: each root followed by its descendants, `depth` for the indent. A parent
 * the list does not hold makes its child a root, and a cycle in the portal's data cannot loop — every
 * row appears exactly once.
 */
export function orderAsTree(choices: readonly CategoryChoice[]): { choice: CategoryChoice; depth: number }[] {
  const ids = new Set(choices.map((c) => c.external_id));
  const children = new Map<string, CategoryChoice[]>();
  const roots: CategoryChoice[] = [];
  for (const c of choices) {
    if (c.parent_id === "" || c.parent_id === "0" || !ids.has(c.parent_id) || c.parent_id === c.external_id) {
      roots.push(c);
    } else {
      const list = children.get(c.parent_id) ?? [];
      list.push(c);
      children.set(c.parent_id, list);
    }
  }
  const out: { choice: CategoryChoice; depth: number }[] = [];
  const placed = new Set<string>();
  const walk = (c: CategoryChoice, depth: number) => {
    if (placed.has(c.external_id)) return;
    placed.add(c.external_id);
    out.push({ choice: c, depth });
    for (const k of children.get(c.external_id) ?? []) walk(k, depth + 1);
  };
  for (const r of roots) walk(r, 0);
  // Rows only reachable through a cycle: shown at the root rather than dropped.
  for (const c of choices) walk(c, 0);
  return out;
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

/** `đã chọn {n}/{m}` — counted over the categories the portal lists now. */
export function selectedCountLabel(choices: readonly CategoryChoice[]): string {
  const live = choices.filter((c) => c.on_portal);
  return `đã chọn ${live.filter((c) => c.selected).length}/${live.length}`;
}
