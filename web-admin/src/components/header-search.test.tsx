// @vitest-environment jsdom
//
// jsdom: the header search box's "?" must open its description on click and reach no server.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { PHASE_2_NOTE, pendingMarkerLabel } from "./ui/pending-feature";

// The session is "not read yet" and no provider runs, so the header itself sends nothing: any fetch
// seen below would come from the placeholder.
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => null }));

const { CauHinhXaProvider } = await import("./cau-hinh-xa");
const { DauTrang, SYSTEM_SEARCH_PENDING } = await import("./dau-trang");

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  if (!("ResizeObserver" in globalThis)) {
    (globalThis as unknown as { ResizeObserver: unknown }).ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    };
  }
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

function mount(): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <CauHinhXaProvider giaTri={{ displayName: "Xã Tân Phú", parentAuthority: "Tỉnh Đồng Nai", logoUrl: "", webAdminBannerUrl: "" }}>
        <DauTrang />
      </CauHinhXaProvider>,
    ),
  );
  return host;
}

describe("DauTrang — system-wide search placeholder (00 §3.1, Phase 2, ADR 0068 §14)", () => {
  // Presentation pin moved 05/10/2026 (ADR 0068 §Sửa đổi lần 2): in the navy header the placeholder is
  // the guide's search ICON, disabled; the disabled FIELD lives in the narrow-screen sheet (nav-sheet).
  it("is a DISABLED search icon button with the spec's name, inside no form, no search landmark", () => {
    const el = mount();
    const button = el.querySelector<HTMLButtonElement>('header button[aria-label="Tìm kiếm toàn hệ thống"]');
    expect(button).not.toBeNull();
    expect(button!.disabled).toBe(true);
    expect(button!.closest("form")).toBeNull();
    expect(el.querySelector('[role="search"]')).toBeNull();
  });

  it("in the narrow-screen sheet: the disabled field with the spec's placeholder", () => {
    const el = mount();
    act(() => el.querySelector<HTMLButtonElement>('button[aria-label="Mở menu"]')!.click());
    const input = el.querySelector<HTMLInputElement>('dialog input[aria-label="Tìm kiếm toàn hệ thống"]');
    expect(input).not.toBeNull();
    expect(input!.disabled).toBe(true);
    expect(input!.type).toBe("search");
    expect(input!.placeholder).toBe("Tìm nhiệm vụ, văn bản, phản ánh…");
    expect(input!.closest("form")).toBeNull();
  });

  it("its '?' opens the description, says Phase 2, and calls no server", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const el = mount();
    const marker = el.querySelector<HTMLButtonElement>(
      `button[aria-label="${pendingMarkerLabel("Tìm kiếm toàn hệ thống")}"]`,
    );
    expect(marker).not.toBeNull();
    act(() => marker!.click());
    const dialog = document.body.querySelector('[role="dialog"]');
    expect(dialog?.textContent).toContain(SYSTEM_SEARCH_PENDING.viSao);
    expect(dialog?.textContent).toContain(PHASE_2_NOTE);
    expect(fetchSpy).not.toHaveBeenCalled();
  });
});
