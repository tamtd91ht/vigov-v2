// @vitest-environment jsdom
//
// jsdom: the screen is driven for real — the session is faked, the network is a router over `fetch`,
// and what is checked is what the screen SENDS and what it SHOWS. Typing in the search box and pressing
// the trash icon are events; a static render cannot show them.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { CitizenContact } from "@/lib/api/citizen-contacts";

import { SEARCH_DELAY_MS, SEARCH_INPUT_ID } from "./citizen-contact-list";
import { DELETE_REASON_INPUT_ID } from "./citizen-contact-delete-dialog";
import { CitizenDirectoryScreen } from "./citizen-directory-screen";
import {
  ADD_CONTACT_BUTTON,
  CITIZEN_DIRECTORY_TITLE,
  DELETE_CONFIRM_BUTTON,
  DELETE_REASON_REQUIRED,
  EMPTY_DIRECTORY,
  IMPORT_OPEN_BUTTON,
  NO_READ_PERMISSION,
  ZALO_COLUMN_TOOLTIP,
  deleteContactLabel,
} from "./citizen-directory-words";

const H = vi.hoisted(() => ({ session: null as PhienDaDoc }));

vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.session }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

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

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "Nguyễn Văn Hùng", position: "Chuyên viên" }, role: null, permissions } } as unknown as PhienDaDoc;
}

/** Fake data only (rule 3, invariant 5): the agreed `0900000xxx` numbers, masked as the server masks them. */
const ROWS: CitizenContact[] = [
  {
    id: "01JCONTACTA",
    full_name: "Nguyễn Văn A",
    phone_masked: "09****0001",
    residential_unit_id: "01JTHON1",
    residential_unit_name: "Thôn 1",
    tags: ["tổ trưởng"],
    group_ids: [],
    is_active: true,
    source: "nhap-tay",
    has_own_app_session: true,
    created_at: "2026-10-16T01:00:00Z",
  },
  {
    id: "01JCONTACTB",
    full_name: "Trần Thị B",
    phone_masked: "09****0002",
    residential_unit_id: "",
    residential_unit_name: "",
    tags: [],
    group_ids: [],
    is_active: true,
    source: "nhap-excel",
    has_own_app_session: false,
    created_at: "2026-10-16T01:00:00Z",
  },
];

type Call = { url: string; init: RequestInit | undefined };
const calls: Call[] = [];

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

/** The server, by path: every read of the screen has an answer; every write is recorded. */
function serve(rows: CitizenContact[]) {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/citizen-groups") return json({ items: [] });
      if (url === "/api/v1/citizen-contact-tags") return json({ items: [{ tag: "tổ trưởng", count: 1 }] });
      if (url === "/api/v1/residential-units") return json({ items: [] });
      if (url.startsWith("/api/v1/citizen-contacts?")) return json({ items: rows, next_cursor: "", has_more: false });
      if (url === "/api/v1/citizen-contact-searches") return json({ items: rows.slice(0, 1), next_cursor: "", has_more: false });
      if (url === "/api/v1/citizen-contact-count-queries") return json({ count: rows.length });
      if (url.endsWith("/deletion")) return new Response(null, { status: 204 });
      return json({ code: "not_found", message: "Không có tuyến này.", trace_id: "" }, 404);
    }),
  );
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  calls.length = 0;
  serve(ROWS);
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

const settle = () => act(async () => {});

async function mount(s: PhienDaDoc): Promise<HTMLDivElement> {
  H.session = s;
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(<CitizenDirectoryScreen />);
  });
  await settle();
  await settle();
  return host;
}

const citizenCalls = () => calls.filter((c) => c.url.includes("citizen"));
const bodyOf = (c: Call) => JSON.parse(String(c.init?.body)) as Record<string, unknown>;
const buttonNamed = (el: HTMLElement, text: string) => [...el.querySelectorAll("button")].find((b) => b.textContent === text) ?? null;

describe("the gate (citizen.read)", () => {
  it("DENIED — no citizen.read: the title, the sentence naming the right, NO read of any directory route", async () => {
    const el = await mount(session(["task.read", "citizen.update"]));
    expect(el.textContent).toContain(CITIZEN_DIRECTORY_TITLE);
    expect(el.textContent).toContain(NO_READ_PERMISSION);
    expect(el.querySelector("table")).toBeNull();
    expect(citizenCalls()).toHaveLength(0);
    expect(buttonNamed(el, ADD_CONTACT_BUTTON)).toBeNull();
  });

  it("session not read yet: the checking sentence, nothing fetched", async () => {
    const el = await mount(null);
    expect(el.querySelector('[role="status"]')?.textContent).toContain("Đang kiểm tra");
    expect(citizenCalls()).toHaveLength(0);
  });
});

describe("the list (citizen.read only)", () => {
  it("numbers are drawn MASKED, the Zalo column says Có / —, no header buttons, no row action, Danh sách (n) from the count route", async () => {
    const el = await mount(session(["citizen.read"]));
    expect(el.textContent).toContain("09****0001");
    expect(el.textContent).toContain("09****0002");
    expect(el.textContent).not.toMatch(/0900000001|0900000002/);
    const rows = [...el.querySelectorAll("tbody tr")];
    expect(rows).toHaveLength(2);
    expect(rows[0]!.textContent).toContain("Có");
    expect(rows[0]!.textContent).toContain("Thôn 1");
    expect(rows[1]!.textContent).toContain("—");
    expect(buttonNamed(el, ADD_CONTACT_BUTTON)).toBeNull();
    expect(buttonNamed(el, IMPORT_OPEN_BUTTON)).toBeNull();
    expect(el.querySelector(`button[aria-label="${deleteContactLabel("Nguyễn Văn A")}"]`)).toBeNull();
    expect(el.querySelector('[role="tab"]')?.textContent).toBe("Danh sách (2)");
    expect(el.querySelector(`[title="${ZALO_COLUMN_TOOLTIP}"]`)).not.toBeNull();
    // The first page is the plain GET, with the page size and no cursor.
    expect(calls.find((c) => c.url.startsWith("/api/v1/citizen-contacts?"))?.url).toBe("/api/v1/citizen-contacts?limit=50");
    // The residential units are read only for an editor.
    expect(calls.find((c) => c.url === "/api/v1/residential-units")).toBeUndefined();
  });

  it("the directory's own empty sentence when the commune has nobody", async () => {
    serve([]);
    const el = await mount(session(["citizen.read"]));
    expect(el.textContent).toContain(EMPTY_DIRECTORY);
  });

  it("typing a keyword: after the pause, POST /citizen-contact-searches with the words IN THE BODY — never on a URL", async () => {
    const el = await mount(session(["citizen.read"]));
    const input = el.querySelector<HTMLInputElement>(`#${SEARCH_INPUT_ID}`)!;
    const setValue = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
    await act(async () => {
      setValue.call(input, "0001");
      input.dispatchEvent(new Event("input", { bubbles: true }));
    });
    expect(calls.find((c) => c.url === "/api/v1/citizen-contact-searches")).toBeUndefined();
    await act(async () => {
      await new Promise((r) => setTimeout(r, SEARCH_DELAY_MS + 50));
    });
    await settle();
    const search = calls.find((c) => c.url === "/api/v1/citizen-contact-searches");
    expect(search).toBeDefined();
    expect(search!.init?.method).toBe("POST");
    expect(bodyOf(search!)).toMatchObject({ q: "0001", cursor: "", limit: 50 });
    const count = calls.filter((c) => c.url === "/api/v1/citizen-contact-count-queries").at(-1)!;
    expect(bodyOf(count)).toMatchObject({ q: "0001" });
    for (const c of calls) expect(c.url).not.toContain("0001");
    expect([...el.querySelectorAll("tbody tr")]).toHaveLength(1);
  });
});

describe("the trash icon (citizen.update) — a reason is MANDATORY (ADR 0097 D7)", () => {
  it("pressing Xoá with no reason: the sentence under the field, NOTHING sent; with a reason: POST …/deletion {reason}", async () => {
    const el = await mount(session(["citizen.read", "citizen.update"]));
    expect(buttonNamed(el, ADD_CONTACT_BUTTON)).not.toBeNull();
    expect(buttonNamed(el, IMPORT_OPEN_BUTTON)).not.toBeNull();
    await act(async () => el.querySelector<HTMLButtonElement>(`button[aria-label="${deleteContactLabel("Nguyễn Văn A")}"]`)!.click());
    expect(el.textContent).toContain("Xoá Nguyễn Văn A khỏi danh bạ?");

    await act(async () => buttonNamed(el, DELETE_CONFIRM_BUTTON)!.click());
    await settle();
    // The search box keeps an (empty) alert region in the DOM; the reason's is the one with the sentence.
    expect([...el.querySelectorAll('[role="alert"]')].map((a) => a.textContent)).toContain(DELETE_REASON_REQUIRED);
    expect(calls.find((c) => c.url.endsWith("/deletion"))).toBeUndefined();

    const reason = el.querySelector<HTMLInputElement>(`#${DELETE_REASON_INPUT_ID}`)!;
    const setValue = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
    await act(async () => {
      setValue.call(reason, "Nhập trùng");
      reason.dispatchEvent(new Event("input", { bubbles: true }));
    });
    await act(async () => buttonNamed(el, DELETE_CONFIRM_BUTTON)!.click());
    await settle();
    const del = calls.find((c) => c.url.endsWith("/deletion"));
    expect(del?.url).toBe("/api/v1/citizen-contacts/01JCONTACTA/deletion");
    expect(del?.init?.method).toBe("POST");
    expect(bodyOf(del!)).toEqual({ reason: "Nhập trùng" });
    expect(el.textContent).not.toContain("Xoá Nguyễn Văn A khỏi danh bạ?");
  });
});
