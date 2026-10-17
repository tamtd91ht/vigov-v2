// @vitest-environment jsdom
//
// jsdom for this file: `?create=1` is an effect on mount and a `history.replaceState` — neither shows in
// a string render. Same mounting as `task-detail-dialog.test.tsx`.

import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import { QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_TAO_NHIEM_VU } from "@/lib/quyen";

import { SoNhiemVu } from "./so-nhiem-vu";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

let fakeSession: PhienDaDoc = null;
vi.mock("@/features/phien/phien-hien-tai", async (original) => ({
  ...(await original<typeof import("@/features/phien/phien-hien-tai")>()),
  usePhien: () => fakeSession,
}));

function sessionWith(permissions: readonly string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { code: "CB-2026-7K3M9Q" }, permissions: [...permissions] } } as unknown as PhienDaDoc;
}

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

async function mount(node: ReactNode): Promise<void> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() => root!.render(node));
  for (let i = 0; i < 12; i++) await act(async () => {});
}

beforeEach(() => {
  // Every read answers an empty page: the register settles with no task, so the only dialog that can
  // appear is the create form.
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      const url = new URL(path, "http://localhost");
      const body = url.pathname === "/api/v1/task-counts" ? { by_status: [] } : { items: [], next_cursor: "", has_more: false };
      return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
    }),
  );
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  fakeSession = null;
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});

describe("/nhiem-vu `?create=1` — `Giao việc mới` opened once, under `task.create`", () => {
  it("ALLOWED: the form opens, and `create` leaves the address bar", async () => {
    fakeSession = sessionWith([QUYEN_TAO_NHIEM_VU]);
    window.history.replaceState(null, "", "/nhiem-vu?create=1");
    await mount(<SoNhiemVu openCreateOnLoad />);
    expect(document.querySelector("dialog")).not.toBeNull();
    expect(window.location.search).toBe("");
  });

  it("DENIED: without `task.create` no form opens (the button is not drawn either)", async () => {
    fakeSession = sessionWith([QUYEN_CAP_NHAT_NHIEM_VU]);
    window.history.replaceState(null, "", "/nhiem-vu?create=1");
    await mount(<SoNhiemVu openCreateOnLoad />);
    expect(document.querySelector("dialog")).toBeNull();
    expect(Array.from(document.querySelectorAll("button")).some((b) => b.textContent?.trim() === "Giao việc mới")).toBe(false);
  });

  it("session still unread: waits — nothing opens and the parameter stays", async () => {
    fakeSession = null;
    window.history.replaceState(null, "", "/nhiem-vu?create=1");
    await mount(<SoNhiemVu openCreateOnLoad />);
    expect(document.querySelector("dialog")).toBeNull();
    expect(window.location.search).toBe("?create=1");
  });

  it("no parameter: nothing opens", async () => {
    fakeSession = sessionWith([QUYEN_TAO_NHIEM_VU]);
    window.history.replaceState(null, "", "/nhiem-vu");
    await mount(<SoNhiemVu />);
    expect(document.querySelector("dialog")).toBeNull();
  });

  it("coexists with `?task=`: dropping `create` keeps `task`", async () => {
    fakeSession = sessionWith([QUYEN_TAO_NHIEM_VU]);
    window.history.replaceState(null, "", "/nhiem-vu?task=NV19&create=1");
    await mount(<SoNhiemVu openCreateOnLoad />);
    expect(new URLSearchParams(window.location.search).get("task")).toBe("NV19");
    expect(new URLSearchParams(window.location.search).has("create")).toBe(false);
  });
});
