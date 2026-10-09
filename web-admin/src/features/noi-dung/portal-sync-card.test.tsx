// @vitest-environment jsdom
//
// jsdom for this file: these tests PRESS the buttons and let the re-read timers run — which request a
// click sends, and when the re-reads stop, is the card's whole job. Same reasoning as `category-admin.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";
import type { KetQua } from "@/lib/api/goi";
import type { CategoryTreeResult, StartRunResult } from "@/lib/api/portal-sync";
import type {
  comms_portalCategoryTreeOut,
  comms_portalRunOut,
  comms_portalSyncSettingsOut,
  page_Result_comms_portalRunOut,
} from "@/lib/api/schema.gen";

const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));

import {
  API_URL_REQUIRED,
  CATEGORIES_LIMIT_REACHED,
  CATEGORIES_NEED_SETTINGS,
  CATEGORIES_NEED_UPDATE,
  CATEGORIES_REQUIRED_TO_ENABLE,
  CATEGORIES_UNREACHABLE,
  DOWNLOAD_IMAGES_NOTE,
  KEY_FIRST_HINT,
  KEY_NEW_URL_HINT,
  KEY_REQUIRED,
  KEY_SAVED_HINT,
  MAX_ITEMS_ERROR,
  NOT_CONNECTED,
  PORTAL_SYNC_DESCRIPTION,
  RUN_REFUSED_TOAST,
  RUN_STARTED_TOAST,
  SETTINGS_SAVED,
  WINDOW_DAYS_ERROR,
} from "./portal-sync";
import {
  DOWNLOAD_IMAGES_PART,
  INTERVAL_15_MIN_PART,
  KEY_IN_USE_PART,
  pendingContentPart,
  PORTAL_CATEGORY_COUNT_PART,
} from "./nhan-noi-dung";
import { PortalSyncCard, type PortalSyncApi } from "./portal-sync-card";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  T.success.mockClear();
  T.error.mockClear();
  T.info.mockClear();
});

function settingsOut(over: Partial<comms_portalSyncSettingsOut> = {}): comms_portalSyncSettingsOut {
  return {
    configured: true,
    encryption_configured: true,
    provider: "cttdt-danang",
    api_url: "https://xa.danang.gov.vn/api",
    api_key_set: true,
    publish_mode: "dang-thang",
    interval_hours: 6,
    window_days: 90,
    max_items_per_run: 100,
    keep_source_credit: true,
    is_enabled: true,
    last_run_at: "2026-09-14T01:09:00Z",
    updated_by: "CB-00123",
    ...over,
  };
}

function run(over: Partial<comms_portalRunOut> = {}): comms_portalRunOut {
  return {
    id: "R1",
    trigger_kind: "theo-lich",
    actor: "system",
    started_at: "2026-09-14T01:09:00Z",
    finished_at: "2026-09-14T01:10:00Z",
    outcome: "mot-phan",
    fetched_count: 3600,
    imported_count: 1,
    skipped_existing_count: 3500,
    skipped_deleted_count: 63,
    failed_count: 2,
    error_summary: [
      { category_external_id: "7", category_name: "Các dự án", error: "timeout", count: 1 },
      { category_external_id: "8", category_name: "Hoạt động của các doanh nghiệp", error: "connect", count: 2 },
    ],
    ...over,
  };
}

const RUNNING = run({ id: "R2", trigger_kind: "chay-tay", actor: "CB-00123", finished_at: null, outcome: "", imported_count: 0, error_summary: [] });

function page(items: comms_portalRunOut[]): KetQua<page_Result_comms_portalRunOut> {
  return { ok: true, duLieu: { items, next_cursor: "", has_more: false } };
}

const TREE: comms_portalCategoryTreeOut = {
  items: [
    { external_id: "1", name: "Danh mục", parent_id: "", parent_name: "", is_selected: false, target_kind: "" },
    { external_id: "2", name: "Chuyển đổi số", parent_id: "1", parent_name: "Danh mục", is_selected: true, target_kind: "tin-tuc" },
  ],
  missing: [{ id: "X", external_id: "9", name: "Mục cũ", target_kind: "su-kien", is_selected: true }],
};

type Fakes = { [K in keyof PortalSyncApi]: ReturnType<typeof vi.fn<PortalSyncApi[K]>> };

function fakes(over: Partial<{ settings: KetQua<comms_portalSyncSettingsOut>; runs: KetQua<page_Result_comms_portalRunOut> }> = {}): Fakes {
  const settings = over.settings ?? { ok: true, duLieu: settingsOut() };
  return {
    getSettings: vi.fn(async () => settings),
    saveSettings: vi.fn(async () => settings),
    getCategories: vi.fn(async (): Promise<CategoryTreeResult> => ({ ok: true, duLieu: TREE })),
    saveCategories: vi.fn(async () => ({ ok: true as const, duLieu: { items: [] } })),
    listRuns: vi.fn(async () => over.runs ?? page([run()])),
    startRun: vi.fn(async (): Promise<StartRunResult> => ({ ok: true, duLieu: null })),
  };
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.useRealTimers();
});

async function mount(api: Fakes, canEdit = true, refetchDelaysMs: readonly number[] = [4000, 15000, 45000]): Promise<void> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(<PortalSyncCard api={api} refetchDelaysMs={refetchDelaysMs} canEdit={canEdit} />);
  });
}

function button(label: string): HTMLButtonElement {
  const b = Array.from(host!.querySelectorAll("button")).find((x) => x.textContent === label);
  if (b === undefined) throw new Error(`no button "${label}"`);
  return b;
}

async function click(el: HTMLElement): Promise<void> {
  await act(async () => {
    el.click();
  });
}

function type(el: HTMLInputElement | HTMLSelectElement, value: string): void {
  const proto = Object.getPrototypeOf(el) as object;
  Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
  act(() => {
    el.dispatchEvent(new Event(el instanceof HTMLSelectElement ? "change" : "input", { bubbles: true }));
  });
}

const chip = () => host!.querySelector('[data-testid="portal-sync-status"]')?.textContent;
const text = () => host!.textContent ?? "";
const has = (label: string) => Array.from(host!.querySelectorAll("button")).some((b) => b.textContent === label);

describe("status line (prototype `ContentSourcePanel.tsx:107-129`)", () => {
  it.each([
    [{}, "Đang bật", "text-leaf"],
    [{ is_enabled: false }, "Đang tắt", "text-ink-muted"],
    // Without the platform key the commune's own state is still what the line says; the run button's
    // reason names the key.
    [{ encryption_configured: false }, "Đang bật", "text-leaf"],
  ] as const)("%o → %s", async (over, label, colour) => {
    await mount(fakes({ settings: { ok: true, duLieu: settingsOut(over) } }));
    expect(chip()).toBe(label);
    expect(host!.querySelector('[data-testid="portal-sync-status"]')!.className).toContain(colour);
  });

  it("one line: state · {?} chuyên mục · interval · publish mode", async () => {
    await mount(fakes());
    const line = host!.querySelector('[data-testid="portal-sync-status-line"]')!.textContent;
    expect(line).toBe("Đang bật · ?chuyên mục · Mỗi 6 giờ · đăng thẳng");
  });

  it("the category count is a disabled '?' in the number's place, its reason the PHAN_CHUA_DUNG entry (MA-02)", async () => {
    await mount(fakes());
    const marker = host!.querySelector(`[aria-label="${pendingMarkerLabel(PORTAL_CATEGORY_COUNT_PART)}"]`);
    expect(marker).not.toBeNull();
    expect(marker!.closest('[aria-disabled="true"]')?.textContent).toContain("chuyên mục");
  });

  it("not connected: the prototype's sentence, `Nối Cổng thông tin`, no `Đồng bộ ngay`, no figures", async () => {
    await mount(fakes({ settings: { ok: true, duLieu: settingsOut({ configured: false, api_key_set: false }) } }));
    expect(text()).toContain(NOT_CONNECTED);
    expect(chip()).toBeUndefined();
    expect(text()).not.toContain("chuyên mục");
    expect(has("Nối Cổng thông tin")).toBe(true);
    expect(has("Cấu hình")).toBe(false);
    expect(has("Đồng bộ ngay")).toBe(false);
  });

  it("connected: `Cấu hình` and `Đồng bộ ngay`", async () => {
    await mount(fakes());
    expect(has("Cấu hình")).toBe(true);
    expect(has("Nối Cổng thông tin")).toBe(false);
    expect(has("Đồng bộ ngay")).toBe(true);
  });

  it("D3: no history, no `Tải lại`, no pending-queue link, no explainer sentence, no result chip", async () => {
    await mount(fakes({ settings: { ok: true, duLieu: settingsOut({ is_enabled: false, publish_mode: "cho-duyet" }) } }));
    expect(host!.querySelector("details")).toBeNull();
    expect(text()).not.toContain("Lịch sử đồng bộ");
    expect(has("Tải lại")).toBe(false);
    expect(text()).not.toContain("Xem các tin chờ duyệt");
    expect(text()).not.toContain("Lịch đồng bộ đang tắt");
    expect(text()).not.toContain("Một phần");
    expect(host!.querySelector(".chip")).toBeNull();
  });

  it("loading: a skeleton block and the status sentence for a screen reader", async () => {
    const api = fakes();
    api.getSettings.mockImplementation(() => new Promise(() => {}));
    await mount(api);
    expect(host!.querySelector('[role="status"]')?.textContent).toBe("Đang tải cấu hình đồng bộ…");
    expect(host!.querySelector(".h-24")).not.toBeNull();
    expect(host!.querySelector("section")).toBeNull();
  });
});

describe("denied — the server's 403 reaches the card verbatim", () => {
  it("no status, the sentence, and `Cấu hình` off", async () => {
    const sentence = "Bạn không có quyền thực hiện thao tác này.";
    await mount(fakes({ settings: { ok: false, thongBao: sentence }, runs: { ok: false, thongBao: sentence } }));
    expect(chip()).toBeUndefined();
    expect(host!.querySelector('[role="alert"]')?.textContent).toBe(sentence);
    expect(button("Nối Cổng thông tin").disabled).toBe(true);
  });
});

describe("`content.update` gates the two write controls (prototype `ContentSourcePanel canEdit`)", () => {
  it("DENIED: no `Đồng bộ ngay`, no `Cấu hình`, no run-blocked hint — the status and the last run stay", async () => {
    await mount(fakes({ runs: page([RUNNING]) }), false);
    expect(has("Đồng bộ ngay")).toBe(false);
    expect(has("Cấu hình")).toBe(false);
    // The only button left is the "?" of the category count — it explains, it writes nothing.
    expect(Array.from(host!.querySelectorAll("button")).every((b) => b.hasAttribute("data-pending-marker"))).toBe(true);
    expect(host!.querySelector("#portal-sync-run-blocked")).toBeNull();
    expect(host!.querySelector("dialog")).toBeNull();
    expect(chip()).toBe("Đang bật");
    expect(host!.querySelector('[data-testid="portal-sync-last-run"]')).not.toBeNull();
  });

  it("ALLOWED: both are there, and `Cấu hình` opens the settings in an overlay `<dialog>` named by its heading", async () => {
    await mount(fakes());
    expect(has("Đồng bộ ngay")).toBe(true);
    expect(host!.querySelector("dialog")).toBeNull();
    await click(button("Cấu hình"));
    const dialog = host!.querySelector("dialog")!;
    expect(dialog.className).toContain("overlay-dialog");
    // The prototype's `sm:max-w-[46rem]`, never wider than the screen.
    expect(dialog.className).toContain("w-[min(46rem,calc(100vw-1rem))]");
    expect(dialog.hasAttribute("open")).toBe(true);
    expect(dialog.getAttribute("aria-labelledby")).toBe("portal-sync-config-title");
    expect(dialog.querySelector("#portal-sync-config-title")).not.toBeNull();
    expect(dialog.querySelector("#portal-sync-config")).not.toBeNull();
  });

  it("`Nối Cổng thông tin` opens the same settings", async () => {
    await mount(fakes({ settings: { ok: true, duLieu: settingsOut({ configured: false, api_key_set: false }) } }));
    await click(button("Nối Cổng thông tin"));
    expect(host!.querySelector("dialog #portal-sync-config")).not.toBeNull();
  });

  it("Esc (`cancel`) closes the overlay", async () => {
    await mount(fakes());
    await click(button("Cấu hình"));
    await act(async () => {
      host!.querySelector("dialog")!.dispatchEvent(new Event("cancel", { cancelable: true }));
    });
    expect(host!.querySelector("dialog")).toBeNull();
  });
});

describe("last run line (prototype `ContentSourcePanel.tsx:131-149`)", () => {
  it("dd/MM/yyyy HH:mm, new items, skipped (both kinds), the errors in red — no article, no URL", async () => {
    await mount(fakes());
    const last = host!.querySelector('[data-testid="portal-sync-last-run"]')!;
    expect(last.textContent).toContain("Chạy lần cuối 14/09/2026 08:09");
    expect(last.textContent).toContain("1 tin mới");
    expect(last.textContent).toContain("bỏ qua 3563");
    const errors = host!.querySelector('[data-testid="portal-sync-run-errors"]')!;
    expect(errors.textContent).toBe("Các dự án: timeout; Hoạt động của các doanh nghiệp: connect (2 lần)");
    expect(errors.className).toContain("text-danger");
    expect(text()).not.toContain("https://");
  });

  it("a clean run draws no error part", async () => {
    await mount(fakes({ runs: page([run({ error_summary: [], failed_count: 0 })]) }));
    expect(host!.querySelector('[data-testid="portal-sync-run-errors"]')).toBeNull();
  });

  it("a run still going says so instead of `0 tin mới`", async () => {
    await mount(fakes({ runs: page([RUNNING]) }));
    const last = host!.querySelector('[data-testid="portal-sync-last-run"]')!.textContent!;
    expect(last).toContain("Đang chạy");
    expect(last).not.toContain("tin mới");
  });

  it("no run yet: no line at all", async () => {
    await mount(fakes({ runs: page([]) }));
    expect(host!.querySelector('[data-testid="portal-sync-last-run"]')).toBeNull();
  });
});

describe("⟳ Đồng bộ ngay", () => {
  it("is off while the newest run is unfinished; the reason is its accessible description, not a visible line", async () => {
    await mount(fakes({ runs: page([RUNNING]) }));
    const run = button("Đồng bộ ngay");
    expect(run.disabled).toBe(true);
    const reason = host!.querySelector("#portal-sync-run-blocked")!;
    expect(reason.textContent).toContain("Đang có một lượt đồng bộ chạy");
    // Screen-reader only (the prototype draws no line), tied to the button, and on hover.
    expect(reason.className).toBe("an-thi-giac");
    expect(run.getAttribute("aria-describedby")).toBe("portal-sync-run-blocked");
    expect(run.title).toContain("Đang có một lượt đồng bộ chạy");
  });

  it("on: no reason element, no description, no title", async () => {
    await mount(fakes());
    const run = button("Đồng bộ ngay");
    expect(run.disabled).toBe(false);
    expect(host!.querySelector("#portal-sync-run-blocked")).toBeNull();
    expect(run.hasAttribute("aria-describedby")).toBe(false);
    expect(run.hasAttribute("title")).toBe(false);
  });

  it("202: the prototype's toast, then the card re-reads itself at 4 s, 15 s and 45 s — and never again", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const api = fakes();
    await mount(api);
    const settingsBefore = api.getSettings.mock.calls.length;

    await click(button("Đồng bộ ngay"));
    expect(api.startRun).toHaveBeenCalledTimes(1);
    expect(T.success).toHaveBeenCalledWith(RUN_STARTED_TOAST);
    const runsAfterStart = api.listRuns.mock.calls.length; // mount + the read right after the 202

    const advance = async (ms: number) => {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(ms);
      });
    };
    await advance(3999);
    expect(api.getSettings.mock.calls.length - settingsBefore).toBe(0);
    await advance(1);
    expect(api.getSettings.mock.calls.length - settingsBefore).toBe(1);
    await advance(11_000);
    expect(api.getSettings.mock.calls.length - settingsBefore).toBe(2);
    await advance(30_000);
    expect(api.getSettings.mock.calls.length - settingsBefore).toBe(3);
    expect(api.listRuns.mock.calls.length - runsAfterStart).toBe(3);

    // Nothing more, however long the tab stays open.
    await advance(10 * 60_000);
    expect(api.getSettings.mock.calls.length - settingsBefore).toBe(3);
    expect(api.listRuns.mock.calls.length - runsAfterStart).toBe(3);
  });

  it("a refusal: the prototype's error toast with the server's sentence; the retry reuses the SAME key", async () => {
    const api = fakes();
    const sentence = "Đang có một lượt đồng bộ của xã. Hãy chờ lượt ấy xong rồi chạy lại.";
    api.startRun.mockImplementationOnce(async () => ({ ok: false, thongBao: sentence }));
    await mount(api);
    await click(button("Đồng bộ ngay"));
    expect(T.error).toHaveBeenCalledWith(RUN_REFUSED_TOAST, { description: sentence });
    expect(T.success).not.toHaveBeenCalled();
    await click(button("Đồng bộ ngay"));
    const [k1, k2] = api.startRun.mock.calls.map((c) => c[0]);
    expect(k1).toBe(k2);
    expect(k1).not.toBe("");
  });

  it("503 portal_sync_busy: the toast carries the Retry-After wait — and NOTHING is retried or re-read", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const api = fakes();
    const sentence = "Hệ thống đang chạy đồng bộ cho các xã khác. Hãy thử lại sau ít phút.";
    api.startRun.mockImplementation(async () => ({ ok: false, thongBao: sentence, retryAfterSeconds: 60 }));
    await mount(api);
    await click(button("Đồng bộ ngay"));
    expect(T.error).toHaveBeenCalledWith(RUN_REFUSED_TOAST, {
      description: `${sentence} Có thể bấm lại sau khoảng 1 phút.`,
    });
    const runsBefore = api.listRuns.mock.calls.length;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5 * 60_000);
    });
    expect(api.startRun).toHaveBeenCalledTimes(1);
    expect(api.listRuns.mock.calls.length).toBe(runsBefore);
    // The officer may press again; the button is not held.
    expect(button("Đồng bộ ngay").disabled).toBe(false);
  });
});

describe("Cấu hình — the dialog (prototype `ContentSourcePanel.tsx:313-566`)", () => {
  async function openConfig(api: Fakes): Promise<void> {
    await mount(api);
    await click(button("Cấu hình"));
  }

  it("one title, one sentence, no inner heading repeated; a scrolling body; `Huỷ` · `Lưu cấu hình`", async () => {
    await openConfig(fakes());
    const dialog = host!.querySelector("dialog")!;
    expect(dialog.querySelector("h3")?.textContent).toBe("Đồng bộ tin từ Cổng thông tin điện tử");
    expect(dialog.querySelectorAll("h3, h4")).toHaveLength(1);
    expect(dialog.textContent!.split(PORTAL_SYNC_DESCRIPTION)).toHaveLength(2);
    expect(dialog.querySelector(".max-h-\\[62vh\\].overflow-y-auto")).not.toBeNull();
    const footer = Array.from(dialog.querySelectorAll("button")).filter((b) => b.closest(".justify-end") !== null);
    expect(footer.map((b) => b.textContent)).toEqual(["Huỷ", "Lưu cấu hình"]);
    // ONE save button: the categories have no save of their own any more.
    expect(has("Lưu chuyên mục")).toBe(false);
    // The two-option publish picker and its explainers are gone.
    expect(dialog.querySelector('input[type="radio"]')).toBeNull();
    expect(dialog.textContent).not.toContain("Chờ duyệt (mặc định)");
  });

  it("the checkbox box, in the prototype's order; `Tải ảnh về kho của xã` is a disabled '?' with its note", async () => {
    await openConfig(fakes());
    const rows = Array.from(host!.querySelectorAll(".rounded-\\[10px\\].space-y-2 > :is(label, div)")).map((r) => r.textContent);
    expect(rows).toEqual([
      "Bật đồng bộ tự động",
      "Tin về thì đăng thẳng lên Mini App",
      "Tải ảnh về kho của xã?",
      "Ghi rõ nguồn tin gốc",
    ]);
    const images = host!.querySelector<HTMLInputElement>("#portal-sync-download-images")!;
    expect(images.disabled).toBe(true);
    expect(host!.querySelector(`[aria-label="${pendingMarkerLabel(DOWNLOAD_IMAGES_PART)}"]`)).not.toBeNull();
    expect(text()).toContain(DOWNLOAD_IMAGES_NOTE);
  });

  it("`Tin về thì đăng thẳng` is the publish mode: ticked = dang-thang, unticked = cho-duyet", async () => {
    const api = fakes({ settings: { ok: true, duLieu: settingsOut({ publish_mode: "cho-duyet" }) } });
    await openConfig(api);
    const direct = host!.querySelector<HTMLInputElement>("#portal-sync-direct")!;
    expect(direct.checked).toBe(false);
    await click(direct);
    await click(button("Lưu cấu hình"));
    expect(api.saveSettings.mock.calls[0]![0].publish_mode).toBe("dang-thang");
  });

  it("the three numbers side by side with the prototype's labels and unit hints", async () => {
    await openConfig(fakes());
    const grid = host!.querySelector("#portal-sync-interval")!.closest(".grid")!;
    expect(grid.className).toContain("sm:grid-cols-3");
    expect(Array.from(grid.querySelectorAll("label")).map((l) => l.textContent)).toEqual([
      "Nhịp đồng bộ",
      "Chỉ lấy tin trong",
      "Tối đa mỗi lần",
    ]);
    expect(grid.textContent).toContain("ngày trở lại");
    expect(grid.textContent).toContain("tin");
  });

  it("the interval options are the prototype's set; `Mỗi 15 phút` is a disabled \"?\" (whole hours only)", async () => {
    await openConfig(fakes());
    const options = Array.from(host!.querySelectorAll<HTMLOptionElement>("#portal-sync-interval option"));
    expect(options.map((o) => o.textContent)).toEqual([
      "Chỉ chạy khi bấm tay",
      "Mỗi 15 phút ?",
      "Mỗi giờ",
      "Mỗi 6 giờ",
      "Mỗi ngày",
    ]);
    expect(options.map((o) => o.value)).toEqual(["0", "", "1", "6", "24"]);
    const quarter = options[1]!;
    expect(quarter.disabled).toBe(true);
    expect(quarter.title).toBe(pendingContentPart(INTERVAL_15_MIN_PART).viSao);
    expect(host!.querySelector<HTMLSelectElement>("#portal-sync-interval")!.value).toBe("6");
  });

  it("a stored interval outside the set is kept as its own option — never silently changed", async () => {
    const api = fakes({ settings: { ok: true, duLieu: settingsOut({ interval_hours: 12 }) } });
    await openConfig(api);
    const select = host!.querySelector<HTMLSelectElement>("#portal-sync-interval")!;
    expect(Array.from(select.options).map((o) => o.textContent)).toEqual([
      "Chỉ chạy khi bấm tay",
      "Mỗi 15 phút ?",
      "Mỗi giờ",
      "Mỗi 6 giờ",
      "Mỗi 12 giờ",
      "Mỗi ngày",
    ]);
    expect(select.value).toBe("12");
    // The card's line names it with the same labeller.
    expect(host!.querySelector('[data-testid="portal-sync-status-line"]')!.textContent).toContain("Mỗi 12 giờ");
    await click(button("Lưu cấu hình"));
    expect(api.saveSettings.mock.calls[0]![0].interval_hours).toBe(12);
  });
});

describe("Cấu hình — settings", () => {
  async function openConfig(api: Fakes): Promise<void> {
    await mount(api);
    await click(button("Cấu hình"));
  }
  const keyBox = () => host!.querySelector<HTMLInputElement>("#portal-sync-api-key")!;
  const urlBox = () => host!.querySelector<HTMLInputElement>("#portal-sync-api-url")!;
  const saveBtn = () => button("Lưu cấu hình");
  const save = () => click(saveBtn());

  it("the key box is empty and password-typed; the hint says blank keeps it", async () => {
    await openConfig(fakes());
    expect(keyBox().value).toBe("");
    expect(keyBox().type).toBe("password");
    // `new-password`, not the prototype's `off`: browsers ignore `off` and would offer the officer's own login.
    expect(keyBox().autocomplete).toBe("new-password");
    expect(keyBox().placeholder).toBe("Giữ nguyên mã cũ");
    // The prototype's `Đang dùng {hint}. Để trống nếu không đổi.`, a disabled "?" in the hint's place.
    const hint = host!.querySelector('[data-testid="portal-sync-key-hint"]')!;
    expect(hint.textContent).toBe(`Đang dùng?. ${KEY_SAVED_HINT}`);
    expect(hint.querySelector(`[aria-label="${pendingMarkerLabel(KEY_IN_USE_PART)}"]`)).not.toBeNull();
    expect(saveBtn().disabled).toBe(false);
  });

  it("the address placeholder names no commune", async () => {
    await openConfig(fakes());
    expect(urlBox().placeholder).toContain("<tên-xã>");
  });

  it("an empty address is refused under its box, nothing sent", async () => {
    const api = fakes();
    await openConfig(api);
    type(urlBox(), "  ");
    await save();
    expect(api.saveSettings).not.toHaveBeenCalled();
    expect(urlBox().closest(".flex-col")!.querySelector('[role="alert"]')?.textContent).toBe(API_URL_REQUIRED);
  });

  it("first time: the key is required, its hint is the prototype's, the refusal is its own sentence", async () => {
    const api = fakes({ settings: { ok: true, duLieu: settingsOut({ configured: false, api_key_set: false, api_url: "", is_enabled: false }) } });
    await mount(api);
    await click(button("Nối Cổng thông tin"));
    expect(host!.querySelector('[data-testid="portal-sync-key-hint"]')?.textContent).toBe(KEY_FIRST_HINT);
    type(urlBox(), "https://xa.danang.gov.vn/api");
    await save();
    expect(api.saveSettings).not.toHaveBeenCalled();
    expect(keyBox().closest(".flex-col")!.querySelector('[role="alert"]')?.textContent).toBe(KEY_REQUIRED);
  });

  it("changing the address requires the key again — refused under the key box until typed, then sent", async () => {
    const api = fakes();
    await openConfig(api);
    type(urlBox(), "https://khac.danang.gov.vn/api");
    expect(host!.querySelector('[data-testid="portal-sync-key-hint"]')?.textContent).toBe(KEY_NEW_URL_HINT);
    await save();
    expect(api.saveSettings).not.toHaveBeenCalled();
    expect(keyBox().closest(".flex-col")!.querySelector('[role="alert"]')?.textContent).toBe(KEY_NEW_URL_HINT);

    type(keyBox(), "k3y-moi");
    await save();
    expect(api.saveSettings).toHaveBeenCalledTimes(1);
    expect(api.saveSettings.mock.calls[0]![0]).toMatchObject({
      api_url: "https://khac.danang.gov.vn/api",
      api_key: "k3y-moi",
    });
    // The tree on screen was read from the OLD portal: none of it is sent to the new one.
    expect(api.saveCategories).not.toHaveBeenCalled();
  });

  it("a save that keeps the address sends no key at all", async () => {
    const api = fakes();
    await openConfig(api);
    type(host!.querySelector<HTMLSelectElement>("#portal-sync-interval")!, "0");
    await save();
    const body = api.saveSettings.mock.calls[0]![0];
    expect("api_key" in body).toBe(false);
    expect(body.interval_hours).toBe(0);
  });

  it("the server's 422 is shown verbatim, the dialog stays and the typed key stays for the retry", async () => {
    const api = fakes();
    const sentence = "Đã đổi địa chỉ API thì phải nhập lại mã bảo mật. Mã cũ không được gửi tới một địa chỉ chưa từng dùng nó.";
    api.saveSettings.mockImplementationOnce(async () => ({ ok: false, thongBao: sentence }));
    await openConfig(api);
    type(keyBox(), "abc");
    await save();
    expect(Array.from(host!.querySelectorAll("form [role='alert']")).map((a) => a.textContent)).toContain(sentence);
    expect(keyBox().value).toBe("abc");
    expect(T.success).not.toHaveBeenCalled();
  });

  it("success: the prototype's toast, the dialog closes, the card shows the new settings", async () => {
    const api = fakes();
    api.saveSettings.mockImplementationOnce(async () => ({ ok: true, duLieu: settingsOut({ interval_hours: 24 }) }));
    await openConfig(api);
    await save();
    expect(T.success).toHaveBeenCalledWith(SETTINGS_SAVED);
    expect(SETTINGS_SAVED).toBe("Đã lưu cấu hình nguồn tin.");
    expect(host!.querySelector("dialog")).toBeNull();
    expect(host!.querySelector('[data-testid="portal-sync-status-line"]')?.textContent).toContain("Mỗi ngày");
  });

  it("without the platform key, Lưu is off and the reason is said", async () => {
    await openConfig(fakes({ settings: { ok: true, duLieu: settingsOut({ encryption_configured: false }) } }));
    expect(saveBtn().disabled).toBe(true);
    expect(host!.querySelector("form [role='alert']")?.textContent).toContain("khoá mã hoá");
  });

  it("window 1..90 and items 1..100: min/max on the inputs, a sentence under the box holds the save", async () => {
    const api = fakes();
    await openConfig(api);
    const win = host!.querySelector<HTMLInputElement>("#portal-sync-window")!;
    const max = host!.querySelector<HTMLInputElement>("#portal-sync-max-items")!;
    expect([win.min, win.max, max.min, max.max]).toEqual(["1", "90", "1", "100"]);

    type(win, "91");
    await save();
    expect(win.closest(".flex-col")!.querySelector('[role="alert"]')?.textContent).toBe(WINDOW_DAYS_ERROR);
    expect(api.saveSettings).not.toHaveBeenCalled();
    type(win, "30");
    type(max, "101");
    await save();
    expect(max.closest(".flex-col")!.querySelector('[role="alert"]')?.textContent).toBe(MAX_ITEMS_ERROR);
    type(max, "50");
    await save();
    expect(api.saveSettings.mock.calls[0]![0]).toMatchObject({ window_days: 30, max_items_per_run: 50 });
  });

  it("the server's 400 range sentence is shown verbatim", async () => {
    const api = fakes();
    const sentence = "dong_bo_cong: số ngày lấy tin phải từ 1 tới 90";
    api.saveSettings.mockImplementationOnce(async () => ({ ok: false, thongBao: sentence }));
    await openConfig(api);
    await save();
    expect(Array.from(host!.querySelectorAll("form [role='alert']")).map((a) => a.textContent)).toContain(sentence);
  });
});

describe("Cấu hình — categories", () => {
  const box = () => host!.querySelector('[data-testid="portal-sync-categories"]')!;

  it("not configured: the portal is not asked, the prototype's sentence says why", async () => {
    const api = fakes({ settings: { ok: true, duLieu: settingsOut({ configured: false, api_key_set: false }) } });
    await mount(api);
    await click(button("Nối Cổng thông tin"));
    expect(api.getCategories).not.toHaveBeenCalled();
    expect(box().textContent).toContain(CATEGORIES_NEED_SETTINGS);
    expect(has("Chọn tất cả")).toBe(false);
  });

  it("first time with sync switched on: refused inside the box — nothing can be ticked yet", async () => {
    const api = fakes({ settings: { ok: true, duLieu: settingsOut({ configured: false, api_key_set: false, api_url: "", is_enabled: true }) } });
    await mount(api);
    await click(button("Nối Cổng thông tin"));
    type(host!.querySelector<HTMLInputElement>("#portal-sync-api-url")!, "https://xa.danang.gov.vn/api");
    type(host!.querySelector<HTMLInputElement>("#portal-sync-api-key")!, "k3y");
    await click(button("Lưu cấu hình"));
    expect(api.saveSettings).not.toHaveBeenCalled();
    expect(box().querySelector('[role="alert"]')?.textContent).toBe(CATEGORIES_REQUIRED_TO_ENABLE);
  });

  it("loading: a skeleton inside the box", async () => {
    const api = fakes();
    api.getCategories.mockImplementation(() => new Promise(() => {}));
    await mount(api);
    await click(button("Cấu hình"));
    expect(box().querySelector(".h-24")).not.toBeNull();
  });

  it("grouped by parent: `Khác` for the roots, a group per parent, each with `chọn cả mục`; no 'không còn trên cổng' chip", async () => {
    await mount(fakes());
    await click(button("Cấu hình"));
    const groups = Array.from(box().querySelectorAll("p.uppercase")).map((g) => g.textContent);
    expect(groups).toEqual(["Khác", "Danh mục"]);
    expect(text()).toContain("đã chọn 2/3");
    expect(text()).not.toContain("không còn trên cổng");
    expect(Array.from(box().querySelectorAll("button")).map((b) => b.textContent)).toEqual([
      "Chọn tất cả",
      "Bỏ chọn",
      "chọn cả mục",
      // `Danh mục`'s one child is ticked: the group's button unticks.
      "bỏ cả mục",
    ]);
  });

  it("`chọn cả mục` ticks the whole group; a ticked one shows the kind select", async () => {
    await mount(fakes());
    await click(button("Cấu hình"));
    expect(host!.querySelector('[aria-label="Loại nội dung của chuyên mục Danh mục"]')).toBeNull();
    await click(button("chọn cả mục"));
    expect(host!.querySelector<HTMLInputElement>("#portal-category-1")!.checked).toBe(true);
    expect(host!.querySelector<HTMLSelectElement>('[aria-label="Loại nội dung của chuyên mục Danh mục"]')!.value).toBe("tin-tuc");
    expect(text()).toContain("đã chọn 3/3");
  });

  it("ONE save: PUT settings, THEN PUT categories with only the changes", async () => {
    const api = fakes();
    const order: string[] = [];
    api.saveSettings.mockImplementation(async (b) => {
      order.push("settings");
      return { ok: true, duLieu: settingsOut({ api_url: b.api_url }) };
    });
    api.saveCategories.mockImplementation(async () => {
      order.push("categories");
      return { ok: true as const, duLieu: { items: [] } };
    });
    await mount(api);
    await click(button("Cấu hình"));
    await click(host!.querySelector<HTMLInputElement>("#portal-category-1")!);
    type(host!.querySelector<HTMLSelectElement>('[aria-label="Loại nội dung của chuyên mục Danh mục"]')!, "thong-bao");
    await click(host!.querySelector<HTMLInputElement>("#portal-category-9")!);
    await click(button("Lưu cấu hình"));

    expect(order).toEqual(["settings", "categories"]);
    expect(api.saveCategories.mock.calls[0]![0]).toEqual({
      categories: [
        { external_id: "1", name: "Danh mục", target_kind: "thong-bao", is_selected: true },
        { external_id: "9", name: "Mục cũ", target_kind: "su-kien", is_selected: false },
      ],
    });
    expect(T.success).toHaveBeenCalledWith(SETTINGS_SAVED);
    expect(host!.querySelector("dialog")).toBeNull();
  });

  it("nothing ticked differently: only the settings are sent", async () => {
    const api = fakes();
    await mount(api);
    await click(button("Cấu hình"));
    await click(button("Lưu cấu hình"));
    expect(api.saveSettings).toHaveBeenCalledTimes(1);
    expect(api.saveCategories).not.toHaveBeenCalled();
  });

  it("the settings refused: the chain stops there, the categories are never sent", async () => {
    const api = fakes();
    api.saveSettings.mockImplementationOnce(async () => ({ ok: false, thongBao: "Từ chối." }));
    await mount(api);
    await click(button("Cấu hình"));
    await click(host!.querySelector<HTMLInputElement>("#portal-category-1")!);
    await click(button("Lưu cấu hình"));
    expect(api.saveCategories).not.toHaveBeenCalled();
    expect(Array.from(host!.querySelectorAll("form [role='alert']")).map((a) => a.textContent)).toContain("Từ chối.");
  });

  it("the server's 422 too_many_categories is shown verbatim; the dialog stays open", async () => {
    const api = fakes();
    const sentence = "Mỗi xã chọn tối đa 30 chuyên mục Cổng. Hãy bỏ chọn bớt rồi lưu lại.";
    api.saveCategories.mockImplementationOnce(async () => ({ ok: false, thongBao: sentence }));
    await mount(api);
    await click(button("Cấu hình"));
    await click(host!.querySelector<HTMLInputElement>("#portal-category-1")!);
    await click(button("Lưu cấu hình"));
    expect(Array.from(host!.querySelectorAll('[role="alert"]')).map((a) => a.textContent)).toContain(sentence);
    expect(host!.querySelector("dialog")).not.toBeNull();
    expect(T.success).not.toHaveBeenCalled();
  });

  it("a portal failure (502): the prototype's sentence in red inside the box", async () => {
    const api = fakes();
    api.getCategories.mockImplementation(async () => ({ ok: false, thongBao: "Cổng không trả lời kịp. Hãy thử lại sau." }));
    await mount(api);
    await click(button("Cấu hình"));
    const alert = box().querySelector('[role="alert"]')!;
    expect(alert.textContent).toBe(CATEGORIES_UNREACHABLE);
    expect(alert.className).toContain("text-danger");
    expect(text()).not.toContain(CATEGORIES_NEED_UPDATE);
  });

  it("DENIED (403, read-only account): the server's sentence + the right it needs, inside Cấu hình; the rest works", async () => {
    const api = fakes();
    const sentence = "Tài khoản của bạn không có quyền thực hiện thao tác này.";
    api.getCategories.mockImplementation(async () => ({ ok: false, thongBao: sentence, forbidden: true }));
    await mount(api);
    await click(button("Cấu hình"));
    const refused = host!.querySelector('[data-testid="portal-sync-categories-refused"]')!;
    expect(refused.closest("#portal-sync-config")).not.toBeNull();
    expect(refused.textContent).toContain(CATEGORIES_NEED_UPDATE);
    expect(refused.querySelector('[role="alert"]')?.textContent).toBe(sentence);
    // No picker, but the settings form, the chip, the history and the run button are all still there.
    expect(host!.querySelector("#portal-category-1")).toBeNull();
    expect(host!.querySelector("#portal-sync-api-url")).not.toBeNull();
    expect(chip()).toBe("Đang bật");
    expect(host!.querySelector('[data-testid="portal-sync-last-run"]')).not.toBeNull();
    expect(button("Đồng bộ ngay").disabled).toBe(false);
  });

  it("ALLOWED: the same card with content.update draws the picker and no permission sentence", async () => {
    await mount(fakes());
    await click(button("Cấu hình"));
    expect(host!.querySelector("#portal-category-1")).not.toBeNull();
    expect(text()).not.toContain(CATEGORIES_NEED_UPDATE);
  });

  it("at 30 selected: unticked boxes go off, ticked ones stay changeable, Chọn tất cả is off", async () => {
    const big: comms_portalCategoryTreeOut = {
      items: Array.from({ length: 32 }, (_, i) => ({
        external_id: `c${i}`,
        name: `Mục ${i}`,
        parent_id: "",
        parent_name: "",
        is_selected: i < 29,
        target_kind: i < 29 ? "tin-tuc" : "",
      })),
      missing: [],
    };
    const api = fakes();
    api.getCategories.mockImplementation(async () => ({ ok: true, duLieu: big }));
    await mount(api);
    await click(button("Cấu hình"));
    expect(text()).toContain("đã chọn 29/32");
    expect(host!.querySelector('[data-testid="portal-sync-categories-limit"]')).toBeNull();

    await click(host!.querySelector<HTMLInputElement>("#portal-category-c29")!);
    expect(text()).toContain("đã chọn 30/32");
    expect(host!.querySelector('[data-testid="portal-sync-categories-limit"]')?.textContent).toBe(CATEGORIES_LIMIT_REACHED);
    expect(host!.querySelector<HTMLInputElement>("#portal-category-c30")!.disabled).toBe(true);
    expect(host!.querySelector<HTMLInputElement>("#portal-category-c0")!.disabled).toBe(false);
    expect(button("Chọn tất cả").disabled).toBe(true);

    // Unticking one frees a slot.
    await click(host!.querySelector<HTMLInputElement>("#portal-category-c0")!);
    expect(host!.querySelector<HTMLInputElement>("#portal-category-c30")!.disabled).toBe(false);
  });

  it("Chọn tất cả on a 40-category portal ticks exactly 30, and the one save sends them", async () => {
    const big: comms_portalCategoryTreeOut = {
      items: Array.from({ length: 40 }, (_, i) => ({
        external_id: `c${i}`,
        name: `Mục ${i}`,
        parent_id: "",
        parent_name: "",
        is_selected: false,
        target_kind: "",
      })),
      missing: [],
    };
    const api = fakes();
    api.getCategories.mockImplementation(async () => ({ ok: true, duLieu: big }));
    await mount(api);
    await click(button("Cấu hình"));
    await click(button("Chọn tất cả"));
    expect(text()).toContain("đã chọn 30/40");
    await click(button("Lưu cấu hình"));
    expect(api.saveCategories.mock.calls[0]![0].categories.filter((c) => c.is_selected)).toHaveLength(30);
  });

  it("switching sync on with every category unticked is refused inside the box", async () => {
    const api = fakes();
    await mount(api);
    await click(button("Cấu hình"));
    await click(button("Bỏ chọn"));
    // The stored row the portal no longer lists is unticked by hand.
    await click(host!.querySelector<HTMLInputElement>("#portal-category-9")!);
    await click(button("Lưu cấu hình"));
    expect(api.saveSettings).not.toHaveBeenCalled();
    expect(box().querySelector('[role="alert"]')?.textContent).toBe(CATEGORIES_REQUIRED_TO_ENABLE);
  });
});
