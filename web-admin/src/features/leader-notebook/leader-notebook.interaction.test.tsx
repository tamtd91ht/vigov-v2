// @vitest-environment jsdom
//
// jsdom for this file: what it pins — a row click opening the register's drawer ON this page, the
// address bar left alone, one drawer at a time, the columns re-read after a write and on close — are
// events, history and network order, none of which a string render can show.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { STATUS_COMPOSE_ID, statusMoveName } from "@/features/nhiem-vu/task-status-pipeline";
import { serverTransitions } from "@/features/nhiem-vu/task-transitions.fixture";
import type { petitions_deNghiChoDuyetRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract types
import { QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU, QUYEN_XEM_NHIEM_VU } from "@/lib/quyen"; // vi-name-ok: existing permission constants

import { LeaderNotebook } from "./leader-notebook";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const S = vi.hoisted(() => ({ permissions: [] as string[] }));

vi.mock("@/features/phien/phien-hien-tai", async (original) => ({
  ...(await original<typeof import("@/features/phien/phien-hien-tai")>()),
  usePhien: () => ({ ok: true, duLieu: { staff: { code: "CB-2026-7K3M9Q" }, permissions: S.permissions } }),
}));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

function task(patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    type: "co-ban",
    bloc: "",
    priority: "",
    title: "Xử lý tồn đọng giải phóng mặt bằng",
    description: "",
    status: "dang-thuc-hien",
    allowed_transitions: serverTransitions(patch.status ?? "dang-thuc-hien"),
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: "",
    assigner: "CB-2026-7K3M9Q",
    due_at: "2026-09-10T16:59:59Z",
    original_due_at: "2026-09-10T16:59:59Z",
    completed_at: null,
    progress: 0,
    result_summary: "",
    note: "",
    leader_approved: false,
    superior_acknowledged: false,
    parent: "",
    child_count: 0,
    extension_count: 0,
    pending_extension: false,
    created_by: "CB-2026-7K3M9Q",
    created_at: "2026-09-01T02:00:00Z",
    updated_at: "2026-09-01T02:00:00Z",
    ...patch,
  };
}

const OVERDUE = task();
const WAITING = task({ code: "NV30", title: "Nghiệm thu đường liên thôn", status: "cho-duyet" });
const ASSIGNED = task({ code: "NV40", title: "Khảo sát camera an ninh", status: "tam-dung" });
const EXTENDED = task({ code: "NV21", title: "Báo cáo tổng kết" });
const REQUEST: petitions_deNghiChoDuyetRa = {
  id: "01JDENGHI0001",
  task_code: "NV21",
  task_title: "Báo cáo tổng kết",
  task_due_at: "2026-06-20T16:59:59Z",
  task_assigner: "",
  new_due_at: "2026-07-15T16:59:59Z",
  reason: "Chờ số liệu",
  requested_by: "CB-2026-7K3M9Q",
  requested_at: "2026-06-18T02:20:00Z",
};

/** Every request, as `METHOD path?query`, in order. */
let calls: string[] = [];

function stubServer(): void {
  const json = (body: unknown) =>
    new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
  const page = (items: unknown[]) => json({ items, next_cursor: "", has_more: false });
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string, init?: RequestInit) => {
      const url = new URL(path, "http://localhost");
      const method = (init?.method ?? "GET").toUpperCase();
      calls.push(`${method} ${url.pathname}${url.search}`);
      const q = url.searchParams;
      if (method === "POST" && url.pathname === "/api/v1/tasks/NV19/status") {
        return json({ ...OVERDUE, status: "hoan-thanh", allowed_transitions: serverTransitions("hoan-thanh") });
      }
      if (url.pathname === "/api/v1/tasks") {
        if (q.get("metric") === "overdue") return page([OVERDUE]);
        if (q.get("status") === "cho-duyet") return page([WAITING]);
        if (q.get("scope") === "assigned-by-me") return page([ASSIGNED]);
        return page([]);
      }
      for (const t of [OVERDUE, WAITING, ASSIGNED, EXTENDED]) {
        if (url.pathname === `/api/v1/tasks/${t.code}`) return json({ ...t, documents: [] });
      }
      if (url.pathname === "/api/v1/task-counts") return json({ by_status: [{ status: "dang-thuc-hien", count: 1 }] });
      if (url.pathname === "/api/v1/task-extensions") return page(q.has("task") ? [] : [REQUEST]);
      if (url.pathname === "/api/v1/task-extension-counts") return json({ count: 1 });
      return page([]);
    }),
  );
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

async function mount(): Promise<void> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => root!.render(<LeaderNotebook />));
  await settle();
}

async function settle(): Promise<void> {
  for (let i = 0; i < 15; i++) await act(async () => {});
}

function row(title: string): HTMLButtonElement {
  const b = Array.from(document.querySelectorAll<HTMLButtonElement>(".leader-notebook li > button")).find((x) =>
    x.textContent?.startsWith(title),
  );
  if (b === undefined) throw new Error(`no row "${title}"`);
  return b;
}

/** How many times the overdue column's list was read. */
function overdueReads(): number {
  return calls.filter((c) => c.startsWith("GET /api/v1/tasks?") && c.includes("metric=overdue")).length;
}

beforeEach(() => {
  S.permissions = [QUYEN_XEM_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU, QUYEN_CAP_NHAT_NHIEM_VU];
  window.history.replaceState(null, "", "/nhiem-vu/so-tay");
  stubServer();
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("a row opens the register's drawer HERE (owner 09/10/2026)", () => {
  it("click → the detail dialog on this page; the address bar is not touched", async () => {
    const push = vi.spyOn(window.history, "pushState");
    const replace = vi.spyOn(window.history, "replaceState");
    await mount();

    await act(async () => row(OVERDUE.title).click());
    await settle();

    const dialog = document.querySelector("dialog");
    expect(dialog).not.toBeNull();
    const title = document.getElementById(dialog!.getAttribute("aria-labelledby")!)!;
    expect(title.textContent).toBe(OVERDUE.title);
    expect(title.closest("header")?.textContent).toContain("NV19");
    expect(calls).toContain("GET /api/v1/tasks/NV19");
    expect(push).not.toHaveBeenCalled();
    expect(replace).not.toHaveBeenCalled();
    expect(window.location.pathname + window.location.search).toBe("/nhiem-vu/so-tay");
  });

  it("ONE drawer at a time: no record-tab strip, no 'Đóng tất cả'; opening another replaces it", async () => {
    await mount();
    await act(async () => row(OVERDUE.title).click());
    await settle();
    expect(document.querySelector('dialog [role="tablist"]')).toBeNull();
    expect(document.body.textContent).not.toContain("Đóng tất cả");

    await act(async () => row(ASSIGNED.title).click());
    await settle();
    expect(document.querySelectorAll("dialog")).toHaveLength(1);
    const dialog = document.querySelector("dialog")!;
    expect(document.getElementById(dialog.getAttribute("aria-labelledby")!)!.textContent).toBe(ASSIGNED.title);
  });

  it("an extension row opens its TASK's drawer", async () => {
    await mount();
    await act(async () => row(REQUEST.task_title).click());
    await settle();
    expect(calls).toContain("GET /api/v1/tasks/NV21");
    const dialog = document.querySelector("dialog")!;
    expect(document.getElementById(dialog.getAttribute("aria-labelledby")!)!.textContent).toBe(EXTENDED.title);
  });

  it("closing the drawer re-reads the columns and their counts", async () => {
    await mount();
    const before = overdueReads();
    const countsBefore = calls.filter((c) => c.startsWith("GET /api/v1/task-extension-counts")).length;
    await act(async () => row(OVERDUE.title).click());
    await settle();
    expect(overdueReads()).toBe(before);

    const close = document.querySelector<HTMLButtonElement>('dialog header button[aria-label="Đóng"]')!;
    await act(async () => close.click());
    await settle();
    expect(document.querySelector("dialog")).toBeNull();
    expect(overdueReads()).toBe(before + 1);
    expect(calls.filter((c) => c.startsWith("GET /api/v1/task-extension-counts")).length).toBe(countsBefore + 1);
    // The rows stayed on screen while re-read — no column flashed back to its loading state.
    expect(row(OVERDUE.title)).toBeTruthy();
  });

  it("a write in the drawer (status move) re-reads the columns while the drawer stays open", async () => {
    await mount();
    await act(async () => row(OVERDUE.title).click());
    await settle();
    const before = overdueReads();

    const chip = document.querySelector<HTMLButtonElement>(`button[aria-label="${statusMoveName("Hoàn thành")}"]`)!;
    await act(async () => chip.click());
    await act(async () => (document.getElementById(STATUS_COMPOSE_ID) as HTMLFormElement).requestSubmit());
    await settle();

    expect(calls).toContain("POST /api/v1/tasks/NV19/status");
    expect(overdueReads()).toBe(before + 1);
    expect(document.querySelector("dialog")).not.toBeNull();
  });

  it("DENIED: without `task.update` the drawer opens read-only — no status chip is pressable", async () => {
    S.permissions = [QUYEN_XEM_NHIEM_VU];
    await mount();
    await act(async () => row(OVERDUE.title).click());
    await settle();
    expect(document.querySelector("dialog")).not.toBeNull();
    expect(document.querySelector(`button[aria-label="${statusMoveName("Hoàn thành")}"]`)).toBeNull();
  });
});
