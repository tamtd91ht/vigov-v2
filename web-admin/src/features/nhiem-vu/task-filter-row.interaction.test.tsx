// @vitest-environment jsdom
//
// jsdom: the filter row holds the search debounce (an effect), so it can no longer be called as a
// plain function. Clicks on the scope group and the two toggles are read from `datLoc`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { PHAM_VI_CUA_TOI, PHAM_VI_TOAN_XA, SCOPE_RELATED_LABEL } from "./nhan-nhiem-vu";
import { HangLoc, SCOPE_GROUP_LABEL, SCOPE_OPTIONS } from "./so-nhiem-vu";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

type RowProps = Parameters<typeof HangLoc>[0];

const CATALOGUE: RowProps["danhMuc"] = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };

let root: Root | null = null;
let host: HTMLDivElement | null = null;

function unmount(): void {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
}

afterEach(unmount);

function mount(loc: RowProps["loc"], onFilter: (l: unknown) => void, danhMuc: RowProps["danhMuc"] = CATALOGUE): void {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <HangLoc loc={loc} tim={loc.tim ?? ""} datTim={() => {}} datLoc={onFilter} danhMuc={danhMuc} danhBa={null} />,
    ),
  );
}

describe("Mức ưu tiên / Loại / Khối list ACTIVE rows only (prototype `TaskWorkspace.tsx:223-255`)", () => {
  const row = (code: string, active: boolean) => ({
    id: `01J${code}`,
    code,
    label: `nhãn ${code}`,
    is_default: false,
    active,
    order: 1,
    source: "xa",
    tier: 3,
    color: null,
  });
  const WITH_RETIRED: RowProps["danhMuc"] = {
    loai: [{ ...row("co-ban", true), requires_directive: false }, { ...row("loai-cu", false), requires_directive: false }],
    mucUuTien: [row("khan", true), row("uu-tien-cu", false)],
    khoi: [row("khoi-uy-ban", true), row("khoi-cu", false)],
    boPhan: [],
  };
  const values = (id: string) =>
    Array.from(host!.querySelectorAll<HTMLOptionElement>(`#${id} option`)).map((o) => o.value);

  it("a retired row is not offered", () => {
    mount({}, () => {}, WITH_RETIRED);
    expect(values("loc-uu-tien")).toEqual(["", "khan"]);
    expect(values("loc-loai")).toEqual(["", "co-ban"]);
    expect(values("loc-khoi")).toEqual(["", "khoi-uy-ban"]);
  });

  it("…except the one the query already holds, so the select never shows another value", () => {
    mount({ mucUuTien: "uu-tien-cu" }, () => {}, WITH_RETIRED);
    expect(values("loc-uu-tien")).toEqual(["", "khan", "uu-tien-cu"]);
  });
});

function scopeButtons(): HTMLButtonElement[] {
  return Array.from(
    host!.querySelectorAll<HTMLButtonElement>(`[role="group"][aria-label="${SCOPE_GROUP_LABEL}"] button`),
  );
}

it("the scope group is named as the prototype's `ScopeFilter`", () => {
  expect(SCOPE_GROUP_LABEL).toBe("Lọc nhanh theo người xử lý");
});

describe("scope — THREE options (owner 07/10/2026 #4), values `` / `mine` / `related`, never a staff code", () => {
  it("exactly three buttons, spec words and hints, the current one pressed", () => {
    mount({ phamVi: "mine" }, () => {});
    const buttons = scopeButtons();
    expect(buttons.map((b) => b.textContent)).toEqual([PHAM_VI_TOAN_XA, PHAM_VI_CUA_TOI, SCOPE_RELATED_LABEL]);
    expect(buttons.map((b) => b.getAttribute("aria-pressed"))).toEqual(["false", "true", "false"]);
    expect(buttons.map((b) => b.title)).toEqual(SCOPE_OPTIONS.map((o) => o.hint));
    // `Tôi đã giao` is not an option of this page.
    expect(host!.textContent).not.toContain("Tôi đã giao");
  });

  it("a press sends the scope VALUE only — `Toàn xã` is ABSENT, never a code", () => {
    const onFilter = vi.fn();
    mount({}, onFilter);
    const [all, mine, related] = scopeButtons();
    act(() => related!.click());
    act(() => mine!.click());
    act(() => all!.click());
    expect(onFilter.mock.calls.map((c) => (c[0] as { phamVi?: string }).phamVi)).toEqual(["related", "mine", undefined]);
  });
});

describe("`Chỉ việc quá hạn` and `Sắp đến hạn` exclude each other; off = ABSENT, never `false`", () => {
  function press(loc: RowProps["loc"], id: string): unknown {
    const onFilter = vi.fn();
    mount(loc, onFilter);
    act(() => host!.querySelector<HTMLButtonElement>(`#${id}`)!.click());
    unmount();
    return onFilter.mock.calls.at(-1)![0];
  }

  it("each toggle turns the other off", () => {
    expect(press({ dueSoon: true }, "loc-qua-han")).toEqual({ chiTreHan: true, dueSoon: undefined });
    expect(press({ chiTreHan: true }, "loc-sap-den-han")).toEqual({ chiTreHan: undefined, dueSoon: true });
    expect(press({ chiTreHan: true }, "loc-qua-han")).toEqual({ chiTreHan: undefined, dueSoon: undefined });
  });
});
