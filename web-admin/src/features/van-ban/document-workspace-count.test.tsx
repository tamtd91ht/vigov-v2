// @vitest-environment jsdom
//
// jsdom: the tab's count is READ from the server after mount and re-read after a booking — effects and a
// submitted form, which a markup string cannot show.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai";
import { PETITION_CREATE_PERMISSION, PETITION_READ_PERMISSION } from "@/lib/quyen";

import { DocumentWorkspace } from "./document-workspace";

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

/** A fake server; `count` answers the count route (a number, or a refusal status). */
function stubServer(count: { n: number } | { refuse: number }): string[] {
  const urls: string[] = [];
  let booked = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      urls.push(url);
      const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
      if (url === "/api/v1/sessions/current") {
        return json({
          sid: "s",
          expires_at: "2099-01-01T00:00:00Z",
          staff: { code: "CB-00009", full_name: "Cán bộ X", position: "" },
          role: { code: "r", name: "R", is_leader: false },
          permissions: [PETITION_READ_PERMISSION, PETITION_CREATE_PERMISSION],
          must_change_password: false,
        });
      }
      if (url.startsWith("/api/v1/citizen-letter-counts")) {
        if ("refuse" in count) return json({ code: "forbidden", message: "Không có quyền.", trace_id: "" }, count.refuse);
        return json({ count: count.n + booked });
      }
      if (url === "/api/v1/citizen-letters" && init?.method === "POST") {
        booked += 1;
        return json({ id: "NEW", number: 9, year: 2026 }, 201);
      }
      if (url.startsWith("/api/v1/citizen-letters")) return json({ items: [], next_cursor: "", has_more: false });
      if (url.startsWith("/api/v1/org-units")) return json({ items: [] });
      if (url.startsWith("/api/v1/staff-directory")) return json({ items: [] });
      return new Response("{}", { status: 404 });
    }),
  );
  return urls;
}

async function mountWorkspace(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() =>
    r.render(
      <PhienProvider>
        <DocumentWorkspace />
      </PhienProvider>,
    ),
  );
  for (let i = 0; i < 5; i++) await act(async () => {});
  return host;
}

const letterTab = (el: HTMLElement) => el.querySelector("#tab-van-ban-petitions")!.textContent;

describe("tab “Đơn thư công dân (N)” (ADR 0084 #7)", () => {
  it("the count of the WHOLE register: scope all, no filter — the bare count route", async () => {
    const urls = stubServer({ n: 8 });
    const el = await mountWorkspace();
    expect(letterTab(el)).toBe("Đơn thư công dân (8)");
    expect(urls.filter((u) => u.startsWith("/api/v1/citizen-letter-counts"))).toEqual(["/api/v1/citizen-letter-counts"]);
  });

  it("DENIED / error: the label without a count — never a 0 that was not counted", async () => {
    stubServer({ refuse: 403 });
    const el = await mountWorkspace();
    expect(letterTab(el)).toBe("Đơn thư công dân");
  });

  it("a booking re-reads the count", async () => {
    const urls = stubServer({ n: 8 });
    const el = await mountWorkspace();
    const book = [...el.querySelectorAll("button")].find((b) => b.textContent?.trim() === "Vào sổ đơn thư")!;
    act(() => book.click());
    const summary = document.querySelector<HTMLTextAreaElement>("#don-thu-noi-dung")!;
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(summary, "Đường thôn bị ngập");
      summary.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => summary.form!.requestSubmit());
    for (let i = 0; i < 5; i++) await act(async () => {});
    expect(urls.filter((u) => u.startsWith("/api/v1/citizen-letter-counts"))).toHaveLength(2);
    expect(letterTab(el)).toBe("Đơn thư công dân (9)");
  });
});
