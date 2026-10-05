// @vitest-environment jsdom
//
// jsdom: record tabs on the task panel are clicks, history moves and sessionStorage — none of
// which a string render can show. Same set-up as `task-detail-dialog.test.tsx`.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import { QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_TAO_NHIEM_VU, QUYEN_XOA_NHIEM_VU } from "@/lib/quyen";

import { SoNhiemVu } from "./so-nhiem-vu";
import { taskTabsStorageKey } from "./task-tabs";
import { serverTransitions } from "./task-transitions.fixture";

const STAFF = "CB-2026-7K3M9Q";

vi.mock("@/features/phien/phien-hien-tai", async (original) => ({
  ...(await original<typeof import("@/features/phien/phien-hien-tai")>()),
  usePhien: () => ({
    ok: true,
    duLieu: { staff: { code: "CB-2026-7K3M9Q" }, permissions: ["task.read", QUYEN_TAO_NHIEM_VU, QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_XOA_NHIEM_VU] },
  }),
}));

// Each case mounts the whole register and opens several tasks, one read per opening: 1–6s under
// a full parallel run, over vitest's 5s default. A slow case, not a loop.
vi.setConfig({ testTimeout: 30_000 });

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

function task(code: string, title: string): petitions_nhiemVuRa {
  return {
    code,
    child_count: 0,
    allowed_transitions: serverTransitions("moi-giao"),
    updated_at: "2026-06-01T02:00:00Z",
    type: "co-ban",
    bloc: "",
    priority: "",
    title,
    description: "",
    status: "moi-giao",
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: "",
    assigner: STAFF,
    due_at: "2026-10-20T23:59:59+07:00",
    original_due_at: "2026-10-20T23:59:59+07:00",
    completed_at: null,
    progress: 0,
    result_summary: "",
    note: "",
    leader_approved: false,
    superior_acknowledged: false,
    parent: "",
    created_by: "CB-2026-VANTHU",
    created_at: "2026-06-01T02:00:00Z",
  };
}

const A = task("NV19", "Rà soát danh sách hộ nghèo quý III");
const B = task("NV20", "Kiểm tra công trình thuỷ lợi");
const C = task("NV21", "Tổng hợp báo cáo tháng");

/** The register lists `listed`; the detail route answers every task in `readable`, else 404. */
function stubServer(listed: readonly petitions_nhiemVuRa[], readable = listed): void {
  const json = (body: unknown, status = 200) =>
    new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      const url = new URL(path, "http://localhost");
      const m = /^\/api\/v1\/tasks\/([^/]+)$/.exec(url.pathname);
      if (m !== null) {
        const t = readable.find((x) => x.code === decodeURIComponent(m[1]!));
        return t === undefined
          ? json({ code: "not_found", message: "Không tìm thấy nhiệm vụ." }, 404)
          : json({ ...t, documents: [] });
      }
      if (url.pathname === "/api/v1/tasks" && !url.searchParams.has("parent")) {
        const status = url.searchParams.get("status");
        return json({ items: status === null || status === "moi-giao" ? listed : [], next_cursor: "", has_more: false });
      }
      if (url.pathname === "/api/v1/task-counts") return json({ by_status: [] });
      return json({ items: [], next_cursor: "", has_more: false });
    }),
  );
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

function mount(node: ReactNode): void {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() => root!.render(node));
}

async function settle(): Promise<void> {
  for (let i = 0; i < 12; i++) await act(async () => {});
}

function button(text: string): HTMLButtonElement {
  // `Mở NV19` = the Kanban card of NV19: since the prototype layout (06/10/2026) the whole card body
  // is the open button, named by its content rather than by a `Mở …` label.
  const card = /^Mở (\S+)$/.exec(text);
  if (card !== null) {
    const open = document.querySelector<HTMLButtonElement>(
      `article[aria-labelledby="the-nhiem-vu-${card[1]}"] button[aria-expanded]`,
    );
    if (open === null) throw new Error(`no card "${card[1]}"`);
    return open;
  }
  const b = Array.from(document.querySelectorAll("button")).find((x) => x.textContent?.trim() === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

const tabs = () => Array.from(document.querySelectorAll<HTMLButtonElement>('dialog [role="tab"][id^="task-record-tab"]'));
const activeTab = () => tabs().find((t) => t.getAttribute("aria-selected") === "true");
const heading = () => document.getElementById("tieu-de-chi-tiet-nhiem-vu")?.textContent ?? "";
const storageKey = () => taskTabsStorageKey(window.location.host, STAFF)!;

async function click(el: HTMLElement): Promise<void> {
  await act(async () => el.click());
  await settle();
}

async function hidePanel(): Promise<void> {
  await click(document.querySelector<HTMLButtonElement>('dialog button[aria-label="Đóng chi tiết nhiệm vụ"]')!);
}

beforeEach(() => {
  window.history.replaceState(null, "", "/nhiem-vu");
  window.sessionStorage.clear();
  vi.spyOn(window.history, "back").mockImplementation(() => {});
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  window.sessionStorage.clear();
});

describe("record tabs on the task panel", () => {
  it("open A, hide the panel, open B → two tabs, B active; the panel's ✕ kept A", async () => {
    stubServer([A, B]);
    mount(<SoNhiemVu />);
    await settle();
    await click(button("Mở NV19"));
    expect(tabs()).toHaveLength(1);
    await hidePanel();
    expect(document.querySelector("dialog")).toBeNull();

    const push = vi.spyOn(window.history, "pushState");
    await click(button("Mở NV20"));
    expect(tabs().map((t) => t.textContent)).toEqual([
      expect.stringContaining("[NV19]"),
      expect.stringContaining("[NV20]"),
    ]);
    expect(activeTab()!.textContent).toContain("[NV20]");
    expect(heading()).toContain("[NV20]");
    // Opening from the list still PUSHES one entry.
    expect(push).toHaveBeenCalledTimes(1);
    expect(window.location.search).toBe("?task=NV20");
  });

  it("switching to A shows A's detail and REPLACES `?task=` (one Back still closes)", async () => {
    stubServer([A, B]);
    mount(<SoNhiemVu />);
    await settle();
    await click(button("Mở NV19"));
    await hidePanel();
    await click(button("Mở NV20"));

    const push = vi.spyOn(window.history, "pushState");
    const replace = vi.spyOn(window.history, "replaceState");
    await click(tabs()[0]!);
    expect(activeTab()!.textContent).toContain("[NV19]");
    expect(heading()).toContain("[NV19]");
    expect(window.location.search).toBe("?task=NV19");
    expect(push).not.toHaveBeenCalled();
    expect(replace).toHaveBeenCalled();
    // The tabpanel is labelled by the active tab.
    expect(document.getElementById("task-record-panel")!.getAttribute("aria-labelledby")).toBe(activeTab()!.id);
  });

  it("✕ on the active tab activates the neighbour; the last ✕ closes the panel", async () => {
    stubServer([A, B, C]);
    mount(<SoNhiemVu />);
    await settle();
    for (const code of ["NV19", "NV20", "NV21"]) {
      await click(button(`Mở ${code}`));
      await hidePanel();
    }
    await click(button("Mở NV20"));
    expect(activeTab()!.textContent).toContain("[NV20]");

    await click(document.querySelector<HTMLButtonElement>('button[aria-label="Đóng tab NV20"]')!);
    // Right neighbour first.
    expect(activeTab()!.textContent).toContain("[NV21]");
    expect(heading()).toContain("[NV21]");
    expect(window.location.search).toBe("?task=NV21");
    await click(document.querySelector<HTMLButtonElement>('button[aria-label="Đóng tab NV21"]')!);
    // No right neighbour: the left one.
    expect(activeTab()!.textContent).toContain("[NV19]");
    await click(document.querySelector<HTMLButtonElement>('button[aria-label="Đóng tab NV19"]')!);
    expect(document.querySelector("dialog")).toBeNull();
    expect(window.history.back).toHaveBeenCalled();
  });

  it("Đóng tất cả closes every tab and the panel; the next opening starts a fresh strip", async () => {
    stubServer([A, B]);
    mount(<SoNhiemVu />);
    await settle();
    await click(button("Mở NV19"));
    await hidePanel();
    await click(button("Mở NV20"));
    await click(button("Đóng tất cả"));
    expect(document.querySelector("dialog")).toBeNull();
    await click(button("Mở NV19"));
    expect(tabs()).toHaveLength(1);
  });

  it("a tab the officer can no longer read shows the refusal in its content, and still closes", async () => {
    stubServer([A, B], [B]);
    window.sessionStorage.setItem(
      storageKey(),
      JSON.stringify([{ id: "NV19", data: { title: A.title, status: "moi-giao" }, used: 1 }]),
    );
    mount(<SoNhiemVu />);
    await settle();
    await click(button("Mở NV20"));
    await click(tabs()[0]!);
    expect(heading()).toContain("[NV19]");
    expect(document.querySelector('dialog [role="alert"]')?.textContent).toBeTruthy();
    await click(document.querySelector<HTMLButtonElement>('button[aria-label="Đóng tab NV19"]')!);
    expect(tabs()).toHaveLength(1);
    expect(heading()).toContain("[NV20]");
  });

  it("tabs survive in sessionStorage, keyed by host + staff code, holding codes and labels only", async () => {
    stubServer([A, B]);
    mount(<SoNhiemVu />);
    await settle();
    await click(button("Mở NV19"));
    await hidePanel();
    const raw = window.sessionStorage.getItem(storageKey());
    expect(raw).not.toBeNull();
    const stored = JSON.parse(raw!) as { id: string; data: Record<string, unknown> }[];
    expect(stored.map((t) => t.id)).toEqual(["NV19"]);
    expect(Object.keys(stored[0]!.data).sort()).toEqual(["status", "title"]);

    // A reload: the strip comes back under the next opening, panel hidden until then.
    act(() => root!.unmount());
    host!.remove();
    mount(<SoNhiemVu />);
    await settle();
    expect(document.querySelector("dialog")).toBeNull();
    await click(button("Mở NV20"));
    expect(tabs().map((t) => t.textContent)).toEqual([
      expect.stringContaining("[NV19]"),
      expect.stringContaining("[NV20]"),
    ]);
    expect(activeTab()!.textContent).toContain("[NV20]");
  });

  it("a stored entry that is not a register code is refused", async () => {
    stubServer([B]);
    window.sessionStorage.setItem(
      storageKey(),
      JSON.stringify([{ id: " ", data: { title: "x", status: "moi-giao" }, used: 1 }, { id: "NV9", data: { title: 7 }, used: 2 }]),
    );
    mount(<SoNhiemVu />);
    await settle();
    await click(button("Mở NV20"));
    expect(tabs()).toHaveLength(1);
  });

  it("a throwing sessionStorage costs the tabs' persistence, never the panel", async () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("SecurityError");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("QuotaExceededError");
    });
    stubServer([A, B]);
    mount(<SoNhiemVu />);
    await settle();
    await click(button("Mở NV19"));
    await hidePanel();
    await click(button("Mở NV20"));
    expect(tabs()).toHaveLength(2);
    expect(heading()).toContain("[NV20]");
  });

  it("`?task=` on load becomes the active tab, above the stored ones", async () => {
    stubServer([A, B]);
    window.sessionStorage.setItem(
      storageKey(),
      JSON.stringify([{ id: "NV19", data: { title: A.title, status: "moi-giao" }, used: 1 }]),
    );
    window.history.replaceState(null, "", "/nhiem-vu?task=NV20");
    mount(<SoNhiemVu openTask="NV20" />);
    await settle();
    expect(tabs()).toHaveLength(2);
    expect(activeTab()!.textContent).toContain("[NV20]");
  });

  it("← → on the strip switch tabs from the keyboard", async () => {
    stubServer([A, B]);
    mount(<SoNhiemVu />);
    await settle();
    await click(button("Mở NV19"));
    await hidePanel();
    await click(button("Mở NV20"));
    const t = activeTab()!;
    t.focus();
    await act(async () => {
      t.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft", bubbles: true }));
    });
    await settle();
    expect(activeTab()!.textContent).toContain("[NV19]");
    expect(document.activeElement).toBe(activeTab());
    expect(heading()).toContain("[NV19]");
  });
});
