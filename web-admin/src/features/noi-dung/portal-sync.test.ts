import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { afterEach, describe, expect, it, vi } from "vitest";

import {
  getPortalCategories,
  parseRetryAfter,
  portalSyncRunsPath,
  savePortalCategories,
  savePortalSyncSettings,
  startPortalSyncRun,
} from "@/lib/api/portal-sync";
import type {
  comms_portalCategoryTreeOut,
  comms_portalRunOut,
  comms_portalSyncSettingsOut,
} from "@/lib/api/schema.gen";

import {
  categoriesBody,
  choicesFromTree,
  formFromSettings,
  intervalLabel,
  keyHint,
  keyRequirement,
  API_URL_REQUIRED,
  CATEGORIES_REQUIRED_TO_ENABLE,
  groupChoices,
  KEY_FIRST_HINT,
  KEY_NEW_URL_HINT,
  KEY_REQUIRED,
  KEY_SAVED_HINT,
  MAX_ITEMS_ERROR,
  MAX_ITEMS_PER_RUN_MAX,
  pickMany,
  PORTAL_SYNC_DESCRIPTION,
  outcomeLabel,
  portalSyncStatus,
  REFETCH_DELAYS_MS,
  portalSyncStatusLabel,
  runCountsLine,
  runErrorLine,
  retryWaitLabel,
  runNowBlockedReason,
  SELECTED_CATEGORIES_MAX,
  selectAllUpToLimit,
  selectedCountLabel,
  settingsBody,
  settingsFormError,
  TARGET_KIND_OPTIONS,
  WINDOW_DAYS_ERROR,
  WINDOW_DAYS_MAX,
  type CategoryChoice,
} from "./portal-sync";

function settingsOut(over: Partial<comms_portalSyncSettingsOut> = {}): comms_portalSyncSettingsOut {
  return {
    configured: true,
    encryption_configured: true,
    provider: "cttdt-danang",
    api_url: "https://xa.danang.gov.vn/api",
    api_key_set: true,
    publish_mode: "cho-duyet",
    interval_hours: 6,
    window_days: 90,
    max_items_per_run: 100,
    keep_source_credit: true,
    is_enabled: true,
    last_run_at: null,
    updated_by: "CB-00123",
    ...over,
  };
}

function run(over: Partial<comms_portalRunOut> = {}): comms_portalRunOut {
  return {
    id: "R1",
    trigger_kind: "chay-tay",
    actor: "CB-00123",
    started_at: "2026-09-14T01:09:00Z",
    finished_at: "2026-09-14T01:10:00Z",
    outcome: "thanh-cong",
    fetched_count: 3600,
    imported_count: 1,
    skipped_existing_count: 3500,
    skipped_deleted_count: 63,
    failed_count: 0,
    error_summary: [],
    ...over,
  };
}

describe("status chip", () => {
  it("four states, the missing platform key winning over everything", () => {
    expect(portalSyncStatus(settingsOut())).toBe("enabled");
    expect(portalSyncStatus(settingsOut({ is_enabled: false }))).toBe("disabled");
    expect(portalSyncStatus(settingsOut({ configured: false }))).toBe("not-configured");
    expect(portalSyncStatus(settingsOut({ configured: false, encryption_configured: false }))).toBe(
      "missing-encryption",
    );
    expect(portalSyncStatus(settingsOut({ encryption_configured: false }))).toBe("missing-encryption");
    expect(
      (["enabled", "disabled", "not-configured", "missing-encryption"] as const).map(portalSyncStatusLabel),
    ).toEqual(["Đang bật", "Đang tắt", "Chưa cấu hình", "Thiếu khoá mã hoá"]);
  });

  it("the prototype's interval names where it has one; every other hour is `Mỗi N giờ`", () => {
    expect(intervalLabel(0)).toBe("Chỉ chạy khi bấm tay");
    expect(intervalLabel(1)).toBe("Mỗi giờ");
    expect(intervalLabel(6)).toBe("Mỗi 6 giờ");
    expect(intervalLabel(24)).toBe("Mỗi ngày");
    expect(intervalLabel(12)).toBe("Mỗi 12 giờ");
  });

  it("the dialog's sentence is the prototype's, verbatim", () => {
    expect(PORTAL_SYNC_DESCRIPTION).toBe("Tin đã đăng trên Cổng của xã sẽ tự về sổ tin này, khỏi phải gõ lại.");
  });
});

describe("runs", () => {
  it("outcome labels, an unfinished run included", () => {
    expect(outcomeLabel("thanh-cong")).toBe("Thành công");
    expect(outcomeLabel("mot-phan")).toBe("Một phần");
    expect(outcomeLabel("that-bai")).toBe("Thất bại");
    expect(outcomeLabel("")).toBe("Đang chạy");
  });

  it("§3's counts: new items, both kinds of skip summed, failures only when any", () => {
    expect(runCountsLine(run())).toBe("1 tin mới · bỏ qua 3563");
    expect(runCountsLine(run({ failed_count: 2 }))).toBe("1 tin mới · bỏ qua 3563 · lỗi 2");
  });

  it("an error line is a category and a class — a run-level one says so", () => {
    expect(runErrorLine({ category_external_id: "7", category_name: "Các dự án", error: "timeout", count: 1 })).toBe(
      "Các dự án: timeout",
    );
    expect(runErrorLine({ category_external_id: "", category_name: "", error: "interrupted", count: 3 })).toBe(
      "Cả lượt: interrupted (3 lần)",
    );
  });

  it("the run button is off while a run is unfinished, and before the commune can run at all", () => {
    expect(runNowBlockedReason(settingsOut(), run())).toBeNull();
    expect(runNowBlockedReason(settingsOut(), undefined)).toBeNull();
    expect(runNowBlockedReason(settingsOut(), run({ finished_at: null, outcome: "" }))).toContain("Đang có");
    expect(runNowBlockedReason(settingsOut({ configured: false }), undefined)).toContain("chưa lưu");
    expect(runNowBlockedReason(settingsOut({ encryption_configured: false }), undefined)).toContain("khoá mã hoá");
  });

  it("after a run is started the card re-reads itself THREE times — 4 s, 15 s, 45 s — then never again", () => {
    expect(REFETCH_DELAYS_MS).toEqual([4000, 15000, 45000]);
  });
});

describe("settings form — the key is write-only", () => {
  it("the form never starts with a key, whatever the server says", () => {
    expect(formFromSettings(settingsOut({ api_key_set: true })).api_key).toBe("");
  });

  it("first save requires the key; a stored key may be kept blank; a NEW address requires it again", () => {
    const s = settingsOut();
    const f = formFromSettings(s);
    expect(keyRequirement(s, f)).toBeNull();
    expect(keyHint(s, f)).toBe(KEY_SAVED_HINT);
    expect(KEY_SAVED_HINT).toBe("Để trống nếu không đổi.");
    expect(settingsFormError(s, f, 1)).toBeNull();

    const moved = { ...f, api_url: "https://khac.danang.gov.vn/api" };
    expect(keyRequirement(s, moved)).toBe("new-url");
    expect(keyHint(s, moved)).toBe(KEY_NEW_URL_HINT);
    expect(settingsFormError(s, moved, null)).toEqual({ field: "api_key", text: KEY_NEW_URL_HINT });
    expect(settingsFormError(s, { ...moved, api_key: "abc" }, null)).toBeNull();

    // Trimming only — same rule as domain.PortalAPIURLChanged.
    expect(keyRequirement(s, { ...f, api_url: `  ${s.api_url} ` })).toBeNull();

    const fresh = settingsOut({ configured: false, api_key_set: false, api_url: "" });
    const ff = { ...formFromSettings(fresh), api_url: "https://xa.danang.gov.vn/api" };
    expect(keyRequirement(fresh, ff)).toBe("first");
    expect(keyHint(fresh, ff)).toBe(KEY_FIRST_HINT);
    expect(KEY_FIRST_HINT).toBe("Do đơn vị vận hành Cổng cấp.");
    expect(settingsFormError(fresh, ff, 0)).toEqual({ field: "api_key", text: KEY_REQUIRED });
    expect(KEY_REQUIRED).toBe("Nhập mã bảo mật do Cổng cấp.");
  });

  it("the body is the whole form; the key only when typed, untrimmed", () => {
    const f = formFromSettings(settingsOut());
    const b = settingsBody({ ...f, api_url: " https://xa.danang.gov.vn/api " });
    expect(b).toEqual({
      api_url: "https://xa.danang.gov.vn/api",
      publish_mode: "cho-duyet",
      interval_hours: 6,
      window_days: 90,
      max_items_per_run: 100,
      keep_source_credit: true,
      is_enabled: true,
    });
    expect("api_key" in b).toBe(false);
    expect(settingsBody({ ...f, api_key: "k3y" }).api_key).toBe("k3y");
  });

  it("a half-typed number holds Lưu with a sentence under its box, never sent as 0", () => {
    const s = settingsOut();
    const f = formFromSettings(s);
    expect(settingsFormError(s, { ...f, window_days: "" }, 1)).toEqual({ field: "window_days", text: WINDOW_DAYS_ERROR });
    expect(settingsFormError(s, { ...f, max_items_per_run: "0" }, 1)).toEqual({ field: "max_items_per_run", text: MAX_ITEMS_ERROR });
    expect(settingsFormError(s, { ...f, api_url: "  " }, 1)).toEqual({ field: "api_url", text: API_URL_REQUIRED });
    expect(API_URL_REQUIRED).toBe("Nhập địa chỉ API của Cổng.");
  });

  it("switching sync on with no category ticked is refused — unless the tree could not be read", () => {
    const s = settingsOut();
    const on = { ...formFromSettings(s), is_enabled: true };
    expect(settingsFormError(s, on, 0)).toEqual({ field: "categories", text: CATEGORIES_REQUIRED_TO_ENABLE });
    expect(CATEGORIES_REQUIRED_TO_ENABLE).toBe("Chọn ít nhất một chuyên mục trước khi bật đồng bộ.");
    expect(settingsFormError(s, on, 2)).toBeNull();
    // Unknown (the portal did not answer): the server decides, nothing is guessed here.
    expect(settingsFormError(s, on, null)).toBeNull();
    expect(settingsFormError(s, { ...on, is_enabled: false }, 0)).toBeNull();
  });

  it("the owner's ceilings hold Lưu with a sentence naming the range: window 1..90, items 1..100", () => {
    const s = settingsOut();
    const f = formFromSettings(s);
    expect(settingsFormError(s, { ...f, window_days: "90", max_items_per_run: "100" }, 1)).toBeNull();
    expect(settingsFormError(s, { ...f, window_days: "1", max_items_per_run: "1" }, 1)).toBeNull();
    expect(settingsFormError(s, { ...f, window_days: "91" }, 1)?.text).toBe(WINDOW_DAYS_ERROR);
    expect(settingsFormError(s, { ...f, window_days: "0" }, 1)?.text).toBe(WINDOW_DAYS_ERROR);
    expect(settingsFormError(s, { ...f, max_items_per_run: "101" }, 1)?.text).toBe(MAX_ITEMS_ERROR);
    expect(WINDOW_DAYS_ERROR).toContain("từ 1 tới 90");
    expect(MAX_ITEMS_ERROR).toContain("từ 1 tới 100");
    // Never sent past a ceiling: the body carries null, not 91 (and Lưu is off anyway).
    expect(settingsBody({ ...f, window_days: "91" }).window_days).toBeNull();
  });

  it("the ceilings are service-comms' own constants, not a second copy that could drift", () => {
    const go = readFileSync(
      fileURLToPath(new URL("../../../../service-comms/internal/domain/portal_sync.go", import.meta.url)),
      "utf8",
    );
    const constant = (name: string) => Number(new RegExp(`${name}\\s*=\\s*(\\d+)`).exec(go)?.[1]);
    expect(constant("PortalWindowDaysMax")).toBe(WINDOW_DAYS_MAX);
    expect(constant("PortalMaxItemsMax")).toBe(MAX_ITEMS_PER_RUN_MAX);
    expect(constant("PortalSelectedCategoriesMax")).toBe(SELECTED_CATEGORIES_MAX);
  });
});

describe("Retry-After after 503 portal_sync_busy", () => {
  it("delta-seconds only; an HTTP-date, a blank or a zero shows no wait", () => {
    expect(parseRetryAfter("60")).toBe(60);
    expect(parseRetryAfter(" 120 ")).toBe(120);
    expect(parseRetryAfter(null)).toBeUndefined();
    expect(parseRetryAfter("")).toBeUndefined();
    expect(parseRetryAfter("0")).toBeUndefined();
    expect(parseRetryAfter("Wed, 21 Oct 2026 07:28:00 GMT")).toBeUndefined();
  });

  it("the wait is said in seconds under a minute, in whole minutes rounded up after", () => {
    expect(retryWaitLabel(60)).toBe("Có thể bấm lại sau khoảng 1 phút.");
    expect(retryWaitLabel(90)).toBe("Có thể bấm lại sau khoảng 2 phút.");
    expect(retryWaitLabel(30)).toBe("Có thể bấm lại sau khoảng 30 giây.");
  });
});

const TREE: comms_portalCategoryTreeOut = {
  items: [
    { external_id: "1", name: "Danh mục", parent_id: "", parent_name: "", is_selected: false, target_kind: "" },
    { external_id: "2", name: "Chuyển đổi số", parent_id: "1", parent_name: "Danh mục", is_selected: true, target_kind: "tin-tuc" },
    { external_id: "3", name: "Kinh tế", parent_id: "", parent_name: "", is_selected: true, target_kind: "thong-bao" },
    { external_id: "4", name: "Nông thôn mới", parent_id: "3", parent_name: "Kinh tế", is_selected: false, target_kind: "" },
  ],
  missing: [{ id: "X", external_id: "9", name: "Mục cũ", target_kind: "su-kien", is_selected: true }],
};

describe("category tree", () => {
  it("only three target kinds (ADR 0067 §2), never Truyền thanh / Video / Banner", () => {
    expect(TARGET_KIND_OPTIONS.map((o) => o.value)).toEqual(["tin-tuc", "su-kien", "thong-bao"]);
  });

  it("groups by parent in the portal's order; no parent (or dropped by the portal) → `Khác`", () => {
    const groups = groupChoices(choicesFromTree(TREE));
    expect(groups.map((g) => [g.name, g.items.map((c) => c.external_id)])).toEqual([
      ["Khác", ["1", "3", "9"]],
      ["Danh mục", ["2"]],
      ["Kinh tế", ["4"]],
    ]);
    expect(choicesFromTree(TREE).find((c) => c.external_id === "9")?.on_portal).toBe(false);
    // The prototype's counter: ticked over the rows listed. The missing-but-selected one counts.
    expect(selectedCountLabel(choicesFromTree(TREE))).toBe("đã chọn 3/5");
  });

  it("two parents sharing a name stay two groups — the reason the list is grouped at all", () => {
    const groups = groupChoices([
      { external_id: "a", name: "Chuyển đổi số", parent_id: "P1", parent_name: "Tin tức", selected: false, target_kind: "", on_portal: true },
      { external_id: "b", name: "Chuyển đổi số", parent_id: "P2", parent_name: "Tin tức", selected: false, target_kind: "", on_portal: true },
    ]);
    expect(groups).toHaveLength(2);
  });

  it("`chọn cả mục` ticks a group up to the ceiling, keeping a kind already set; `bỏ cả mục` unticks only it", () => {
    const many: CategoryChoice[] = Array.from({ length: 32 }, (_, i) => ({
      external_id: String(i),
      name: `C${i}`,
      parent_id: "",
      parent_name: "",
      selected: i < 28,
      target_kind: i === 0 ? "thong-bao" : i < 28 ? "tin-tuc" : "",
      on_portal: true,
    }));
    const group = new Set(["0", "28", "29", "30", "31"]);
    const taken = pickMany(many, group, true, "tin-tuc");
    expect(taken.filter((c) => c.selected).length).toBe(SELECTED_CATEGORIES_MAX);
    expect(taken.find((c) => c.external_id === "0")?.target_kind).toBe("thong-bao");
    expect(taken.filter((c) => ["28", "29"].includes(c.external_id)).every((c) => c.selected && c.target_kind === "tin-tuc")).toBe(true);
    expect(taken.filter((c) => ["30", "31"].includes(c.external_id)).some((c) => c.selected)).toBe(false);
    const dropped = pickMany(taken, group, false, "tin-tuc");
    expect(dropped.filter((c) => group.has(c.external_id)).some((c) => c.selected)).toBe(false);
    expect(dropped.filter((c) => c.selected).length).toBe(27);
  });

  it("`Chọn tất cả` stops at 30, already-ticked ones counted first; missing rows are never ticked by it", () => {
    const many: CategoryChoice[] = Array.from({ length: 40 }, (_, i) => ({
      external_id: String(i),
      name: `C${i}`,
      parent_id: "",
      parent_name: "",
      selected: i >= 35, // five already ticked, at the END of the order
      target_kind: i >= 35 ? "su-kien" : "",
      on_portal: true,
    }));
    many.push({ external_id: "gone", name: "Gone", parent_id: "", parent_name: "", selected: false, target_kind: "", on_portal: false });
    const after = selectAllUpToLimit(many, "tin-tuc");
    expect(after.filter((c) => c.selected).length).toBe(SELECTED_CATEGORIES_MAX);
    expect(after.slice(0, 25).every((c) => c.selected && c.target_kind === "tin-tuc")).toBe(true);
    expect(after.slice(25, 35).some((c) => c.selected)).toBe(false);
    expect(after.slice(35, 40).every((c) => c.selected && c.target_kind === "su-kien")).toBe(true);
    expect(after.find((c) => c.external_id === "gone")?.selected).toBe(false);
    // Already at the ceiling: nothing moves.
    expect(selectAllUpToLimit(after, "tin-tuc")).toEqual(after);
  });

  it("the body holds ONLY what changed, each entry with a valid kind", () => {
    const initial = choicesFromTree(TREE);
    expect(categoriesBody(initial, initial)).toEqual({ categories: [] });

    const current = initial.map((c) => {
      if (c.external_id === "1") return { ...c, selected: true }; // newly ticked, no kind yet
      if (c.external_id === "2") return { ...c, selected: false }; // unticked
      if (c.external_id === "3") return { ...c, target_kind: "su-kien" }; // kind changed
      if (c.external_id === "9") return { ...c, selected: false }; // missing one unticked
      return c;
    });
    expect(categoriesBody(initial, current)).toEqual({
      categories: [
        { external_id: "1", name: "Danh mục", target_kind: "tin-tuc", is_selected: true },
        { external_id: "2", name: "Chuyển đổi số", target_kind: "tin-tuc", is_selected: false },
        { external_id: "3", name: "Kinh tế", target_kind: "su-kien", is_selected: true },
        { external_id: "9", name: "Mục cũ", target_kind: "su-kien", is_selected: false },
      ],
    });
  });
});

/* ── The wire ─────────────────────────────────────────────────────────────────────────────────── */

describe("lib/api/portal-sync — what goes on the wire", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  function stubFetch(status: number, body: unknown) {
    const f = vi.fn(async (_url: string, _init?: RequestInit) =>
      new Response(body === null ? null : JSON.stringify(body), { status }),
    );
    vi.stubGlobal("fetch", f);
    return f;
  }

  it("PUT settings omits a blank key and never carries a commune", async () => {
    const f = stubFetch(200, settingsOut());
    await savePortalSyncSettings({ api_url: "https://xa.danang.gov.vn/api", api_key: "", interval_hours: 0 });
    const [url, init] = f.mock.calls[0]!;
    expect(url).toBe("/api/v1/portal-sync/settings");
    expect(init?.method).toBe("PUT");
    const sent = JSON.parse(String(init?.body)) as Record<string, unknown>;
    expect("api_key" in sent).toBe(false);
    expect(sent.interval_hours).toBe(0);
    expect(JSON.stringify(sent)).not.toContain("tenant");
  });

  it("PUT settings passes a 422 sentence through verbatim", async () => {
    const sentence = "Đã đổi địa chỉ API thì phải nhập lại mã bảo mật.";
    stubFetch(422, { code: "api_key_required_for_new_url", message: sentence });
    const r = await savePortalSyncSettings({ api_url: "https://khac.gov.vn/api" });
    expect(r).toEqual({ ok: false, thongBao: sentence });
  });

  it("PUT categories rebuilds every entry field by field", async () => {
    const f = stubFetch(200, { items: [] });
    const extra = { external_id: "1", name: "A", target_kind: "tin-tuc", is_selected: true, status: "x" };
    await savePortalCategories({ categories: [extra] });
    expect(JSON.parse(String(f.mock.calls[0]![1]?.body))).toEqual({
      categories: [{ external_id: "1", name: "A", target_kind: "tin-tuc", is_selected: true }],
    });
  });

  it("GET categories: a 403 is flagged `forbidden` with the server's sentence; a 502 is not", async () => {
    const sentence = "Tài khoản của bạn không có quyền thực hiện thao tác này.";
    const f = stubFetch(403, { code: "forbidden", message: sentence });
    expect(await getPortalCategories()).toEqual({ ok: false, thongBao: sentence, forbidden: true });
    expect(f.mock.calls[0]![0]).toBe("/api/v1/portal-sync/categories");
    expect(f.mock.calls[0]![1]?.method).toBe("GET");

    stubFetch(502, { code: "portal_timeout", message: "Cổng không trả lời kịp. Hãy thử lại sau." });
    expect(await getPortalCategories()).toEqual({ ok: false, thongBao: "Cổng không trả lời kịp. Hãy thử lại sau." });
  });

  it("PUT categories: 422 too_many_categories reaches the screen verbatim", async () => {
    const sentence = "Mỗi xã chọn tối đa 30 chuyên mục Cổng. Hãy bỏ chọn bớt rồi lưu lại.";
    stubFetch(422, { code: "too_many_categories", message: sentence });
    expect(await savePortalCategories({ categories: [] })).toEqual({ ok: false, thongBao: sentence });
  });

  it("POST runs: 503 portal_sync_busy carries the sentence and the Retry-After seconds", async () => {
    const sentence = "Hệ thống đang chạy đồng bộ cho các xã khác. Hãy thử lại sau ít phút.";
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(JSON.stringify({ code: "portal_sync_busy", message: sentence }), {
          status: 503,
          headers: { "Retry-After": "60" },
        }),
      ),
    );
    expect(await startPortalSyncRun("k-1")).toEqual({ ok: false, thongBao: sentence, retryAfterSeconds: 60 });
  });

  it("POST runs: a 409 has no wait, and a 503 without Retry-After none either", async () => {
    const sentence = "Đang có một lượt đồng bộ của xã. Hãy chờ lượt ấy xong rồi chạy lại.";
    stubFetch(409, { code: "portal_sync_in_progress", message: sentence });
    expect(await startPortalSyncRun("k-1")).toEqual({ ok: false, thongBao: sentence });
    stubFetch(503, { code: "portal_sync_unavailable", message: "Bộ chạy đồng bộ đang khởi động lại." });
    expect(await startPortalSyncRun("k-1")).toEqual({ ok: false, thongBao: "Bộ chạy đồng bộ đang khởi động lại." });
  });

  it("POST runs carries the Idempotency-Key, no body, and 202 is success whatever the body", async () => {
    const f = stubFetch(202, { replayed: true });
    const r = await startPortalSyncRun("k-1");
    expect(r).toEqual({ ok: true, duLieu: null });
    const init = f.mock.calls[0]![1]!;
    expect(init.method).toBe("POST");
    expect(init.body).toBeUndefined();
    expect((init.headers as Record<string, string>)["Idempotency-Key"]).toBe("k-1");
  });

  it("the history asks newest-first by default — no sort sent", () => {
    expect(portalSyncRunsPath()).toBe("/api/v1/portal-sync/runs?limit=10");
  });

  it("the routes are the ones service-comms declares", () => {
    const go = readFileSync(
      fileURLToPath(new URL("../../../../service-comms/internal/http/routes_portal_sync.go", import.meta.url)),
      "utf8",
    );
    for (const r of [
      "GET /api/v1/portal-sync/settings",
      "PUT /api/v1/portal-sync/settings",
      "GET /api/v1/portal-sync/categories",
      "PUT /api/v1/portal-sync/categories",
      "GET /api/v1/portal-sync/runs",
      "POST /api/v1/portal-sync/runs",
    ]) {
      expect(go).toContain(`"${r}"`);
    }
  });
});
