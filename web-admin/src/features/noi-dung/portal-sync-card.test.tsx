// @vitest-environment jsdom
//
// jsdom for this file: these tests PRESS the buttons and let the poll timer run — which request a click
// sends, and when the polling stops, is the card's whole job. Same reasoning as `category-admin.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  comms_portalCategoryTreeOut,
  comms_portalRunOut,
  comms_portalSyncSettingsOut,
  page_Result_comms_portalRunOut,
} from "@/lib/api/schema.gen";

import { POLL_GAVE_UP, POLL_MAX_ATTEMPTS, KEY_NEW_URL_HINT, KEY_SAVED_HINT, MISSING_ON_PORTAL } from "./portal-sync";
import { PortalSyncCard, type PortalSyncApi } from "./portal-sync-card";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
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
    getCategories: vi.fn(async (): Promise<KetQua<comms_portalCategoryTreeOut>> => ({ ok: true, duLieu: TREE })),
    saveCategories: vi.fn(async () => ({ ok: true as const, duLieu: { items: [] } })),
    listRuns: vi.fn(async () => over.runs ?? page([run()])),
    startRun: vi.fn(async (): Promise<KetQua<null>> => ({ ok: true, duLieu: null })),
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

async function mount(api: Fakes, pollIntervalMs = 1000): Promise<void> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(<PortalSyncCard api={api} pollIntervalMs={pollIntervalMs} />);
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

describe("status chip and meta line", () => {
  it.each([
    [{}, "Đang bật"],
    [{ is_enabled: false }, "Đang tắt"],
    [{ configured: false }, "Chưa cấu hình"],
    [{ encryption_configured: false }, "Thiếu khoá mã hoá"],
  ] as const)("%o → %s", async (over, label) => {
    await mount(fakes({ settings: { ok: true, duLieu: settingsOut(over) } }));
    expect(chip()).toBe(label);
  });

  it("meta line: interval and publish mode", async () => {
    await mount(fakes());
    expect(text()).toContain("Mỗi 6 giờ · đăng thẳng");
  });
});

describe("denied — the server's 403 reaches the card verbatim", () => {
  it("no chip, the sentence, and the run button off", async () => {
    const sentence = "Bạn không có quyền thực hiện thao tác này.";
    await mount(fakes({ settings: { ok: false, thongBao: sentence }, runs: { ok: false, thongBao: sentence } }));
    expect(chip()).toBeUndefined();
    expect(host!.querySelector('[role="alert"]')?.textContent).toBe(sentence);
    expect(button("⟳ Đồng bộ ngay").disabled).toBe(true);
    expect(button("Cấu hình").disabled).toBe(true);
  });
});

describe("last run and error block", () => {
  it("time, outcome, counts and one line per category error — no article, no URL", async () => {
    await mount(fakes());
    const last = host!.querySelector('[data-testid="portal-sync-last-run"]')!.textContent!;
    expect(last).toContain("Chạy lần cuối 08:09 14/09/2026");
    expect(last).toContain("Một phần");
    expect(last).toContain("1 tin mới · bỏ qua 3563 · lỗi 2");
    const errors = Array.from(host!.querySelectorAll('[aria-label="Lỗi của lượt đồng bộ"] li')).map((l) => l.textContent);
    expect(errors).toEqual(["Các dự án: timeout", "Hoạt động của các doanh nghiệp: connect (2 lần)"]);
    expect(text()).not.toContain("https://");
  });

  it("history: trigger, actor as business code or Hệ thống", async () => {
    await mount(fakes({ runs: page([run({ id: "A", trigger_kind: "chay-tay", actor: "CB-00123" }), run({ id: "B" })]) }));
    const rows = Array.from(host!.querySelectorAll("details tbody tr")).map((r) => r.textContent ?? "");
    expect(rows[0]).toContain("Chạy tay");
    expect(rows[0]).toContain("CB-00123");
    expect(rows[1]).toContain("Theo lịch");
    expect(rows[1]).toContain("Hệ thống");
  });

  it("no run yet says so", async () => {
    await mount(fakes({ runs: page([]) }));
    expect(text()).toContain("Chưa chạy lượt đồng bộ nào.");
  });
});

describe("⟳ Đồng bộ ngay", () => {
  it("is off while the newest run is unfinished", async () => {
    await mount(fakes({ runs: page([RUNNING]) }));
    expect(button("⟳ Đồng bộ ngay").disabled).toBe(true);
    expect(text()).toContain("Đang có một lượt đồng bộ chạy");
  });

  it("is off before the commune configured anything", async () => {
    await mount(fakes({ settings: { ok: true, duLieu: settingsOut({ configured: false, api_key_set: false }) } }));
    expect(button("⟳ Đồng bộ ngay").disabled).toBe(true);
  });

  it("polls a BOUNDED number of times, then stops and says so", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const api = fakes();
    await mount(api);
    api.listRuns.mockImplementation(async () => page([RUNNING]));

    await click(button("⟳ Đồng bộ ngay"));
    expect(api.startRun).toHaveBeenCalledTimes(1);
    expect(button("⟳ Đồng bộ ngay").disabled).toBe(true);
    const afterStart = api.listRuns.mock.calls.length; // mount + the read right after the 202

    for (let i = 0; i < POLL_MAX_ATTEMPTS + 5; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000);
      });
    }
    expect(api.listRuns.mock.calls.length - afterStart).toBe(POLL_MAX_ATTEMPTS);
    expect(text()).toContain(POLL_GAVE_UP);

    // Nothing more, however long the tab stays open.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(api.listRuns.mock.calls.length - afterStart).toBe(POLL_MAX_ATTEMPTS);
  });

  it("stops polling as soon as the run has finished, and re-reads the settings", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const api = fakes();
    await mount(api);
    api.listRuns.mockImplementationOnce(async () => page([RUNNING])); // right after the 202
    api.listRuns.mockImplementationOnce(async () => page([RUNNING])); // poll 1
    api.listRuns.mockImplementation(async () => page([run({ id: "R2", outcome: "thanh-cong", error_summary: [] })]));
    const settingsBefore = api.getSettings.mock.calls.length;

    await click(button("⟳ Đồng bộ ngay"));
    for (let i = 0; i < 6; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1000);
      });
    }
    // mount + after-202 + poll 1 + poll 2 (finished) — then silence.
    expect(api.listRuns).toHaveBeenCalledTimes(4);
    expect(api.getSettings.mock.calls.length).toBe(settingsBefore + 1);
    expect(button("⟳ Đồng bộ ngay").disabled).toBe(false);
    expect(text()).not.toContain(POLL_GAVE_UP);
  });

  it("a refusal shows the server's sentence; the retry reuses the SAME key; a 202 renews it", async () => {
    const api = fakes();
    const sentence = "Đang có một lượt đồng bộ của xã. Hãy chờ lượt ấy xong rồi chạy lại.";
    api.startRun.mockImplementationOnce(async () => ({ ok: false, thongBao: sentence }));
    await mount(api);
    await click(button("⟳ Đồng bộ ngay"));
    expect(host!.querySelector('[role="alert"]')?.textContent).toBe(sentence);
    await click(button("⟳ Đồng bộ ngay"));
    const [k1, k2] = api.startRun.mock.calls.map((c) => c[0]);
    expect(k1).toBe(k2);
    expect(k1).not.toBe("");
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

  it("the key box is empty and password-typed; the hint says a key is stored", async () => {
    await openConfig(fakes());
    expect(keyBox().value).toBe("");
    expect(keyBox().type).toBe("password");
    expect(keyBox().autocomplete).toBe("new-password");
    expect(host!.querySelector('[data-testid="portal-sync-key-hint"]')?.textContent).toBe(KEY_SAVED_HINT);
    expect(saveBtn().disabled).toBe(false);
  });

  it("changing the address requires the key again — Lưu off until typed, then sent", async () => {
    const api = fakes();
    await openConfig(api);
    type(urlBox(), "https://khac.danang.gov.vn/api");
    expect(host!.querySelector('[data-testid="portal-sync-key-hint"]')?.textContent).toBe(KEY_NEW_URL_HINT);
    expect(saveBtn().disabled).toBe(true);

    type(keyBox(), "k3y-moi");
    expect(saveBtn().disabled).toBe(false);
    await act(async () => {
      saveBtn().form!.requestSubmit();
    });
    expect(api.saveSettings).toHaveBeenCalledTimes(1);
    expect(api.saveSettings.mock.calls[0]![0]).toMatchObject({
      api_url: "https://khac.danang.gov.vn/api",
      api_key: "k3y-moi",
    });
  });

  it("a save that keeps the address sends no key at all", async () => {
    const api = fakes();
    await openConfig(api);
    type(host!.querySelector<HTMLSelectElement>("#portal-sync-interval")!, "0");
    await act(async () => {
      saveBtn().form!.requestSubmit();
    });
    const body = api.saveSettings.mock.calls[0]![0];
    expect("api_key" in body).toBe(false);
    expect(body.interval_hours).toBe(0);
  });

  it("the server's 422 is shown verbatim and the typed key stays for the retry", async () => {
    const api = fakes();
    const sentence = "Đã đổi địa chỉ API thì phải nhập lại mã bảo mật. Mã cũ không được gửi tới một địa chỉ chưa từng dùng nó.";
    api.saveSettings.mockImplementationOnce(async () => ({ ok: false, thongBao: sentence }));
    await openConfig(api);
    type(keyBox(), "abc");
    await act(async () => {
      saveBtn().form!.requestSubmit();
    });
    expect(host!.querySelector("form [role='alert']")?.textContent).toBe(sentence);
    expect(keyBox().value).toBe("abc");
  });

  it("after a successful save the key box is empty again", async () => {
    const api = fakes();
    await openConfig(api);
    type(keyBox(), "abc");
    await act(async () => {
      saveBtn().form!.requestSubmit();
    });
    expect(keyBox().value).toBe("");
    expect(text()).toContain("Đã lưu cấu hình đồng bộ.");
  });

  it("without the platform key, Lưu is off and the reason is said", async () => {
    await openConfig(fakes({ settings: { ok: true, duLieu: settingsOut({ encryption_configured: false }) } }));
    expect(saveBtn().disabled).toBe(true);
    expect(host!.querySelector("form [role='alert']")?.textContent).toContain("khoá mã hoá");
  });

  it("publish mode radios explain both choices; Chờ duyệt is marked default", async () => {
    await openConfig(fakes());
    const form = host!.querySelector("form")!.textContent!;
    expect(form).toContain("Chờ duyệt (mặc định)");
    expect(form).toContain("bà con chưa thấy");
    expect(form).toContain("hiện ngay cho bà con");
  });
});

describe("Cấu hình — categories", () => {
  it("not configured: the portal is not asked, a sentence says why", async () => {
    const api = fakes({ settings: { ok: true, duLieu: settingsOut({ configured: false, api_key_set: false }) } });
    await mount(api);
    await click(button("Cấu hình"));
    expect(api.getCategories).not.toHaveBeenCalled();
    expect(text()).toContain("Lưu địa chỉ API và mã bảo mật trước");
  });

  it("the tree, the missing one flagged, and the body holds only the changes", async () => {
    const api = fakes();
    await mount(api);
    await click(button("Cấu hình"));
    expect(api.getCategories).toHaveBeenCalledTimes(1);
    expect(text()).toContain("đã chọn 1/2");
    expect(text()).toContain(MISSING_ON_PORTAL);

    await click(host!.querySelector<HTMLInputElement>("#portal-category-1")!);
    type(host!.querySelector<HTMLSelectElement>('[aria-label="Loại nội dung của chuyên mục Danh mục"]')!, "thong-bao");
    await click(host!.querySelector<HTMLInputElement>("#portal-category-9")!);

    await act(async () => {
      button("Lưu chuyên mục").form!.requestSubmit();
    });
    expect(api.saveCategories.mock.calls[0]![0]).toEqual({
      categories: [
        { external_id: "1", name: "Danh mục", target_kind: "thong-bao", is_selected: true },
        { external_id: "9", name: "Mục cũ", target_kind: "su-kien", is_selected: false },
      ],
    });

    // Saved state is the new baseline: pressing again sends nothing.
    await act(async () => {
      button("Lưu chuyên mục").form!.requestSubmit();
    });
    expect(api.saveCategories).toHaveBeenCalledTimes(1);
  });

  it("a portal failure (502) shows the server's sentence, not the portal's", async () => {
    const api = fakes();
    const sentence = "Cổng không trả lời kịp. Hãy thử lại sau.";
    api.getCategories.mockImplementation(async () => ({ ok: false, thongBao: sentence }));
    await mount(api);
    await click(button("Cấu hình"));
    expect(Array.from(host!.querySelectorAll('[role="alert"]')).map((a) => a.textContent)).toContain(sentence);
  });
});
