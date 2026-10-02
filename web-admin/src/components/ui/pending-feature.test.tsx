// @vitest-environment jsdom
//
// jsdom for this file: the "?" must OPEN something on click and CLOSE on Escape, and a placeholder
// must never reach the network. Those are events and focus, which a markup string cannot show.

import { BarChart3, Download } from "lucide-react";
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import {
  PENDING_HOVER_TEXT,
  PHASE_2_NOTE,
  PendingButton,
  PendingCell,
  PendingColumnHeader,
  PendingFeature,
  PendingField,
  PendingMarker,
  PendingSection,
  PendingStatCard,
  PendingTab,
  pendingMarkerLabel,
  type PendingFeatureInfo,
} from "./pending-feature";
import { Segmented } from "./segmented";
import { TabList } from "./tabs";

const INFO: PendingFeatureInfo = {
  ten: "Xuất Excel",
  viSao: "Máy chủ chưa có tuyến xuất sổ ra tệp, nên nút này chưa làm được gì.",
};

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  // Radix measures the trigger to place the popover; jsdom has no ResizeObserver.
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
  vi.restoreAllMocks();
});

function mount(node: ReactNode): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(node));
  return host;
}

function marker(el: ParentNode): HTMLButtonElement {
  const b = el.querySelector<HTMLButtonElement>("button[data-pending-marker]");
  if (b === null) throw new Error("no '?' marker");
  return b;
}

/** The open description, wherever Radix portalled it. */
function dialog(): HTMLElement | null {
  return document.body.querySelector<HTMLElement>('[role="dialog"]');
}

describe("PendingMarker", () => {
  it("is a focusable button whose accessible name says the whole sentence (touch has no hover)", () => {
    const el = mount(<PendingMarker info={INFO} />);
    const b = marker(el);
    expect(b.type).toBe("button");
    expect(b.disabled).toBe(false);
    expect(b.getAttribute("aria-label")).toBe(pendingMarkerLabel("Xuất Excel"));
    expect(b.getAttribute("aria-label")).toBe("Xuất Excel — tính năng đang phát triển. Bấm để xem mô tả");
    expect(b.textContent).toBe("?");
    b.focus();
    expect(document.activeElement).toBe(b);
  });

  it("click opens the description (name + why); Escape closes it and focus returns to '?'", async () => {
    const el = mount(<PendingMarker info={INFO} />);
    const b = marker(el);
    expect(dialog()).toBeNull();
    // Keyboard path: focus the "?", press it (Enter/Space on a native button is a click).
    b.focus();
    act(() => b.click());
    const d = dialog();
    expect(d).not.toBeNull();
    expect(d!.textContent).toContain("Xuất Excel");
    expect(d!.textContent).toContain(INFO.viSao);
    expect(d!.textContent).toContain(PENDING_HOVER_TEXT);
    expect(d!.textContent).not.toContain(PHASE_2_NOTE);
    // Named by its own title, not by the trigger's long sentence.
    const labelledBy = d!.getAttribute("aria-labelledby");
    expect(labelledBy).toBeTruthy();
    expect(document.getElementById(labelledBy!)?.textContent).toBe("Xuất Excel");
    expect(b.getAttribute("aria-expanded")).toBe("true");
    // Focus moves INTO the description, so a keyboard user reads it and reaches "Đóng".
    expect(d!.contains(document.activeElement)).toBe(true);

    act(() => {
      d!.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    });
    // Radix returns focus on the next tick, after the content unmounts.
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(dialog()).toBeNull();
    expect(document.activeElement).toBe(b);
  });

  it("focus shows the hover text 'Tính năng đang phát triển' (the tooltip repeats, never replaces, the name)", () => {
    const el = mount(<PendingMarker info={INFO} />);
    act(() => marker(el).focus());
    const tip = document.body.querySelector('[role="tooltip"]');
    expect(tip?.textContent).toBe(PENDING_HOVER_TEXT);
  });

  it("nameInHover: the tooltip names the part, for a spot where its label is not visible", () => {
    const el = mount(<PendingMarker info={INFO} nameInHover />);
    act(() => marker(el).focus());
    expect(document.body.querySelector('[role="tooltip"]')?.textContent).toBe(`Xuất Excel — ${PENDING_HOVER_TEXT}`);
  });

  it("the 'Đóng' button closes it too", () => {
    const el = mount(<PendingMarker info={INFO} />);
    act(() => marker(el).click());
    const close = [...dialog()!.querySelectorAll("button")].find((x) => x.textContent === "Đóng");
    expect(close).toBeDefined();
    act(() => close!.click());
    expect(dialog()).toBeNull();
  });

  it("a Phase-2 part says so in its description — a phase, never a date", () => {
    const el = mount(<PendingMarker info={INFO} phase2 />);
    act(() => marker(el).click());
    expect(dialog()!.textContent).toContain(PHASE_2_NOTE);
    expect(dialog()!.textContent).not.toMatch(/Sắp có|\d{1,2}\/\d{4}/);
  });

  it("opening, reading and closing never calls the network nor stores anything", () => {
    const fetchSpy = vi.fn();
    vi.stubGlobal("fetch", fetchSpy);
    const setItem = vi.spyOn(Storage.prototype, "setItem");
    const el = mount(<PendingMarker info={INFO} phase2 />);
    act(() => marker(el).click());
    act(() => {
      dialog()!.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    });
    act(() => marker(el).click());
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(setItem).not.toHaveBeenCalled();
    expect(document.cookie).toBe("");
    vi.unstubAllGlobals();
  });
});

describe("control shapes: the control is the real kind, disabled; only the '?' is live", () => {
  it("PendingButton: a native disabled <button> labelled with the part, then the '?'", () => {
    const el = mount(<PendingButton info={INFO} icon={<Download aria-hidden="true" />} />);
    const buttons = [...el.querySelectorAll("button")];
    expect(buttons).toHaveLength(2);
    expect(buttons[0]!.disabled).toBe(true);
    expect(buttons[0]!.textContent).toBe("Xuất Excel");
    expect(buttons[1]!.hasAttribute("data-pending-marker")).toBe(true);
    expect(buttons[1]!.disabled).toBe(false);
  });

  it("PendingButton: a custom label, still disabled", () => {
    const el = mount(
      <PendingButton info={INFO} variant="primary">
        Xuất tệp
      </PendingButton>,
    );
    const real = el.querySelector("button:not([data-pending-marker])") as HTMLButtonElement;
    expect(real.textContent).toBe("Xuất tệp");
    expect(real.disabled).toBe(true);
    expect(real.className).toContain("nut-chinh");
  });

  it("PendingTab: role=tab, never selected, disabled, out of the tab order", () => {
    const el = mount(
      <TabList aria-label="Phạm vi">
        <PendingTab info={{ ten: "Bản đồ", viSao: "Chưa chọn nhà cung cấp bản đồ." }} />
      </TabList>,
    );
    const tab = el.querySelector<HTMLButtonElement>('[role="tab"]')!;
    expect(tab.disabled).toBe(true);
    expect(tab.getAttribute("aria-selected")).toBe("false");
    expect(tab.getAttribute("aria-disabled")).toBe("true");
    expect(tab.tabIndex).toBe(-1);
    expect(tab.textContent).toBe("Bản đồ");
    expect(marker(el).getAttribute("aria-label")).toBe(pendingMarkerLabel("Bản đồ"));
  });

  it("Segmented (buttons): a pending option is disabled and NEVER emits; real ones still do", () => {
    const onChange = vi.fn();
    const el = mount(
      <Segmented
        legend="Kỳ"
        name="ky"
        mode="buttons"
        value="thang"
        onChange={onChange}
        options={[
          { value: "thang", label: "Tháng này" },
          { value: "tuy-chon", label: "Tuỳ chọn", pending: { ten: "Tuỳ chọn", viSao: "Chưa dựng." } },
        ]}
      />,
    );
    const pending = [...el.querySelectorAll("button")].find((b) => b.textContent === "Tuỳ chọn")!;
    expect(pending.disabled).toBe(true);
    act(() => pending.click());
    expect(onChange).not.toHaveBeenCalled();
    act(() => [...el.querySelectorAll("button")].find((b) => b.textContent === "Tháng này")!.click());
    expect(onChange).toHaveBeenCalledWith("thang");
    expect(marker(el).getAttribute("aria-label")).toBe(pendingMarkerLabel("Tuỳ chọn"));
  });

  it("Segmented (radio): a pending option is a disabled radio, unchecked", () => {
    const el = mount(
      <Segmented
        legend="Kỳ"
        name="ky2"
        value="thang"
        onChange={() => {}}
        options={[
          { value: "thang", label: "Tháng này" },
          { value: "tuy-chon", label: "Tuỳ chọn", pending: { ten: "Tuỳ chọn", viSao: "Chưa dựng." } },
        ]}
      />,
    );
    const radio = el.querySelector<HTMLInputElement>('input[value="tuy-chon"]')!;
    expect(radio.disabled).toBe(true);
    expect(radio.checked).toBe(false);
    expect(el.querySelectorAll("button[data-pending-marker]")).toHaveLength(1);
  });

  it("PendingColumnHeader + PendingCell: a real column header, a '—' cell that says why", () => {
    const el = mount(
      <table>
        <thead>
          <tr>
            <PendingColumnHeader info={{ ten: "Ảnh hiện trường", viSao: "Chưa dựng." }} />
          </tr>
        </thead>
        <tbody>
          <tr>
            <PendingCell />
          </tr>
        </tbody>
      </table>,
    );
    const th = el.querySelector("th")!;
    expect(th.getAttribute("scope")).toBe("col");
    expect(th.textContent).toContain("Ảnh hiện trường");
    expect(th.querySelector("button[data-pending-marker]")).not.toBeNull();
    const td = el.querySelector("td")!;
    expect(td.textContent).toContain("—");
    expect(td.textContent).toContain(PENDING_HOVER_TEXT);
  });

  it("PendingStatCard: StatCard's frame, '—' instead of a figure, the '?' by the label", () => {
    const el = mount(<PendingStatCard info={{ ten: "Tỷ lệ đúng hạn", viSao: "Chưa có số liệu." }} icon={BarChart3} />);
    expect(el.textContent).toContain("Tỷ lệ đúng hạn");
    expect(el.querySelector("p")!.textContent).toBe("—");
    expect(el.querySelectorAll("button")).toHaveLength(1);
  });

  it("PendingSection: a card with a heading and the '?' in its header", () => {
    const el = mount(<PendingSection info={{ ten: "Xếp hạng bộ phận", viSao: "Chưa dựng." }} titleAs="h3" />);
    expect(el.querySelector("section h3")!.textContent).toBe("Xếp hạng bộ phận");
    expect(el.querySelectorAll("button")).toHaveLength(1);
  });

  it("PendingField: label tied to a DISABLED control; the '?' is NOT inside the label", () => {
    for (const kind of ["input", "select", "textarea"] as const) {
      const el = mount(<PendingField info={{ ten: "Người ký", viSao: "Chưa dựng." }} id={`nguoi-ky-${kind}`} kind={kind} />);
      const label = el.querySelector("label")!;
      expect(label.htmlFor).toBe(`nguoi-ky-${kind}`);
      expect(label.querySelector("button")).toBeNull();
      const control = el.querySelector<HTMLInputElement>(`#nguoi-ky-${kind}`)!;
      expect(control.tagName.toLowerCase()).toBe(kind);
      expect(control.disabled).toBe(true);
      act(() => root?.unmount());
      host?.remove();
      root = null;
    }
  });

  it("PendingField file: a DISABLED file input with no name — nothing a form could submit", () => {
    const el = mount(<PendingField info={{ ten: "Ảnh", viSao: "Chưa dựng." }} id="anh" kind="file" />);
    const input = el.querySelector<HTMLInputElement>("#anh")!;
    expect(input.tagName.toLowerCase()).toBe("input");
    expect(input.type).toBe("file");
    expect(input.disabled).toBe(true);
    expect(input.name).toBe("");
    expect(el.querySelector("label")!.htmlFor).toBe("anh");
    expect(el.querySelector("label")!.querySelector("button")).toBeNull();
    expect(el.querySelector("button[data-pending-marker]")).not.toBeNull();
  });

  it("PendingFeature: wraps a control the screen drew disabled, '?' pinned to its corner", () => {
    const el = mount(
      <PendingFeature info={INFO}>
        <select disabled aria-label="Cột">
          <option>Cột</option>
        </select>
      </PendingFeature>,
    );
    expect(el.querySelector("select")!.disabled).toBe(true);
    expect(marker(el).className).toContain("absolute");
  });
});

describe("server render", () => {
  it("renders on the server with the '?' and no description until pressed", () => {
    const html = renderToStaticMarkup(<PendingButton info={INFO} />);
    expect(html).toContain("disabled");
    expect(html).toContain(`aria-label="${pendingMarkerLabel("Xuất Excel")}"`);
    expect(html).not.toContain(INFO.viSao);
  });
});
