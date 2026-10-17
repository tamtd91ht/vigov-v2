// @vitest-environment jsdom
//
// jsdom: "Xem phản ánh" (ADR 0102) is a press that opens a dialog, reads a page, and pages on a press; the
// `?id=` deep link opens the edit dialog on mount. The session is faked, the network is a router over
// `fetch`, and what is checked is what the screen SENDS and what it SHOWS.

import type { AnchorHTMLAttributes, ReactNode } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { CitizenContact } from "@/lib/api/citizen-contacts";
import type { petitions_contactPetitionOut } from "@/lib/api/petition-directory-link";

import { citizenContactDeepLinkId } from "./citizen-contact-deep-link";
import { CitizenDirectoryScreen } from "./citizen-directory-screen";
import { CONTACT_EDIT_TITLE } from "./citizen-directory-words";
import {
  CitizenPetitionsDialog,
  NO_PETITIONS,
  PETITIONS_LOADING,
  VIEW_PETITIONS_BUTTON,
  viewPetitionsLabel,
} from "./citizen-petitions-dialog";

const H = vi.hoisted(() => ({ session: null as PhienDaDoc }));

vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.session }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
// A plain anchor: the App Router's Link needs a router this test does not mount.
vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

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

/** A real ULID shape (Crockford base-32: no I, L, O, U) — the deep link accepts nothing else. */
const CONTACT_ID = "01JCQ7ZK3M8XW5T2V9RBNHGF4D";

/** Fake data only (rule 3, invariant 5): the agreed `0900000xxx` numbers, masked as the server masks them. */
const ROW: CitizenContact = {
  id: CONTACT_ID,
  full_name: "Nguyễn Văn A",
  phone_masked: "09****0001",
  residential_unit_id: "",
  residential_unit_name: "",
  tags: [],
  group_ids: [],
  is_active: true,
  source: "nhap-tay",
  has_own_app_session: false,
  created_at: "2026-10-16T01:00:00Z",
  attributes: {},
};

const petition = (n: number): petitions_contactPetitionOut => ({
  lookup_code: `PA-${String(n).padStart(4, "0")}`,
  title: `Phản ánh số ${n}`,
  status: "dang-xu-ly",
  received_at: "2026-10-16T01:00:00Z",
});

type Call = { url: string; init: RequestInit | undefined };
const calls: Call[] = [];
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

/** `byContact`: the answer of the person's petition list, per requested page. */
let byContact: (page: number) => Response = () => json({ items: [], total: 0, page: 1, page_size: 10 });

function serve() {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/citizen-groups") return json({ items: [] });
      if (url === "/api/v1/citizen-contact-tags") return json({ items: [] });
      if (url === "/api/v1/residential-units") return json({ items: [] });
      if (url.startsWith("/api/v1/citizen-fields")) return json({ items: [] });
      if (url.startsWith("/api/v1/citizen-contacts?")) return json({ items: [ROW], next_cursor: "", has_more: false });
      if (url === "/api/v1/citizen-contact-count-queries") return json({ count: 1 });
      if (url === `/api/v1/citizen-contacts/${CONTACT_ID}`)
        return json({ ...ROW, address: "", note: "", import_batch_id: "", updated_at: ROW.created_at, email_masked: "" });
      if (url.startsWith(`/api/v1/citizen-report-lists/by-contact/${CONTACT_ID}?`)) {
        return byContact(Number(new URL(url, "http://x").searchParams.get("page")));
      }
      return json({ code: "not_found", message: "Không có tuyến này.", trace_id: "" }, 404);
    }),
  );
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  calls.length = 0;
  byContact = () => json({ items: [], total: 0, page: 1, page_size: 10 });
  serve();
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

async function render(node: ReactNode): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(node);
  });
  await settle();
  await settle();
  return host;
}

const listCalls = () => calls.filter((c) => c.url.includes("/citizen-report-lists/by-contact/"));
const buttonNamed = (el: HTMLElement, text: string) => [...el.querySelectorAll("button")].find((b) => b.textContent?.includes(text)) ?? null;

describe("the `?id=` deep link — a ULID or nothing", () => {
  it("accepts a row id; ignores a phone number, a short code, a list, nothing", () => {
    expect(citizenContactDeepLinkId(CONTACT_ID)).toBe(CONTACT_ID);
    expect(citizenContactDeepLinkId("0900000001")).toBeNull();
    expect(citizenContactDeepLinkId("PA-1")).toBeNull();
    expect(citizenContactDeepLinkId([CONTACT_ID, CONTACT_ID])).toBeNull();
    expect(citizenContactDeepLinkId(undefined)).toBeNull();
  });

  it("an editor: the edit dialog of THAT row opens, its detail read by id, the mask from the detail", async () => {
    const el = await mountScreen(["citizen.read", "citizen.update"], CONTACT_ID);
    expect(el.querySelector("dialog h2")?.textContent).toBe(CONTACT_EDIT_TITLE);
    expect(calls.some((c) => c.url === `/api/v1/citizen-contacts/${CONTACT_ID}`)).toBe(true);
    expect(el.querySelector<HTMLInputElement>("#nguoi-dan-so-dien-thoai")?.value).toBe("09****0001");
    expect(el.querySelector<HTMLInputElement>("#nguoi-dan-ho-ten")?.value).toBe("Nguyễn Văn A");
  });

  it("DENIED — no citizen.update: the link opens nothing (same rule as a row click)", async () => {
    const el = await mountScreen(["citizen.read"], CONTACT_ID);
    expect(el.querySelector("dialog")).toBeNull();
    expect(calls.some((c) => c.url === `/api/v1/citizen-contacts/${CONTACT_ID}`)).toBe(false);
  });
});

/** The screen under a faked session. */
function mountScreen(permissions: string[], openContactId: string | null = null): Promise<HTMLDivElement> {
  H.session = session(permissions);
  return render(<CitizenDirectoryScreen openContactId={openContactId} />);
}

describe("“Xem phản ánh” on the directory (citizen.read + feedback.read)", () => {
  it("DENIED — no feedback.read: no row icon, no button in the edit dialog, no list request", async () => {
    const el = await mountScreen(["citizen.read", "citizen.update"], CONTACT_ID);
    expect(el.querySelector(`button[aria-label="${viewPetitionsLabel("Nguyễn Văn A")}"]`)).toBeNull();
    expect(buttonNamed(el, VIEW_PETITIONS_BUTTON)).toBeNull();
    expect(listCalls()).toHaveLength(0);
  });

  it("citizen.read + feedback.read only (no citizen.update): the row icon opens the list — the one way in for a reader", async () => {
    byContact = () => json({ items: [petition(1)], total: 1, page: 1, page_size: 10 });
    const el = await mountScreen(["citizen.read", "feedback.read"]);
    await act(async () => el.querySelector<HTMLButtonElement>(`button[aria-label="${viewPetitionsLabel("Nguyễn Văn A")}"]`)!.click());
    await settle();
    expect(listCalls().map((c) => c.url)).toEqual([`/api/v1/citizen-report-lists/by-contact/${CONTACT_ID}?page=1&page_size=10`]);
    expect(el.textContent).toContain("Phản ánh số 1");
  });

  it("an editor with feedback.read: “Xem phản ánh” in the edit dialog opens the same list", async () => {
    const el = await mountScreen(["citizen.read", "citizen.update", "feedback.read"], CONTACT_ID);
    await act(async () => buttonNamed(el, VIEW_PETITIONS_BUTTON)!.click());
    await settle();
    expect(listCalls()).toHaveLength(1);
    expect(el.textContent).toContain(NO_PETITIONS);
  });
});

describe("the petitions dialog", () => {
  it("loading: the skeleton and the status sentence while the page is read", async () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {})));
    const el = await render(<CitizenPetitionsDialog contactId={CONTACT_ID} fullName="Nguyễn Văn A" onClose={() => {}} />);
    expect(el.querySelector('[role="status"]')?.textContent).toBe(PETITIONS_LOADING);
    expect(el.querySelector(".skeleton-rows")).not.toBeNull();
  });

  it("empty: the sentence", async () => {
    const el = await render(<CitizenPetitionsDialog contactId={CONTACT_ID} fullName="Nguyễn Văn A" onClose={() => {}} />);
    expect(el.textContent).toContain(NO_PETITIONS);
  });

  it("error: the server's sentence, verbatim", async () => {
    byContact = () => json({ code: "identity_unavailable", message: "Chưa đối chiếu được với danh bạ, vui lòng thử lại.", trace_id: "" }, 503);
    const el = await render(<CitizenPetitionsDialog contactId={CONTACT_ID} fullName="Nguyễn Văn A" onClose={() => {}} />);
    expect(el.querySelector('[role="alert"]')?.textContent).toBe("Chưa đối chiếu được với danh bạ, vui lòng thử lại.");
  });

  it("a row: code, title, status LABEL, vi-VN date; it links to /phan-anh?id=<lookup code>", async () => {
    byContact = () => json({ items: [petition(7)], total: 1, page: 1, page_size: 10 });
    const el = await render(<CitizenPetitionsDialog contactId={CONTACT_ID} fullName="Nguyễn Văn A" onClose={() => {}} />);
    const a = el.querySelector<HTMLAnchorElement>("[data-contact-petitions] a")!;
    expect(a.getAttribute("href")).toBe("/phan-anh?id=PA-0007");
    expect(a.textContent).toContain("PA-0007");
    expect(a.textContent).toContain("Phản ánh số 7");
    expect(a.textContent).toContain("Đang xử lý");
    expect(a.textContent).toContain("16/10/2026");
    // No pager for a single page.
    expect(buttonNamed(el, "Sau")).toBeNull();
  });

  it("pagination: 25 petitions → “Sau” asks page=2&page_size=10 and draws that page; “Trước” goes back", async () => {
    byContact = (page) =>
      json({ items: Array.from({ length: 10 }, (_, i) => petition((page - 1) * 10 + i + 1)), total: 25, page, page_size: 10 });
    const el = await render(<CitizenPetitionsDialog contactId={CONTACT_ID} fullName="Nguyễn Văn A" onClose={() => {}} />);
    expect(el.textContent).toContain("Trang 1 / 3 · 25 phản ánh");
    expect(buttonNamed(el, "Trước")!.disabled).toBe(true);
    await act(async () => buttonNamed(el, "Sau")!.click());
    await settle();
    expect(listCalls().at(-1)!.url).toBe(`/api/v1/citizen-report-lists/by-contact/${CONTACT_ID}?page=2&page_size=10`);
    expect(el.textContent).toContain("Phản ánh số 11");
    expect(el.textContent).not.toContain("Phản ánh số 1 ");
    expect(el.textContent).toContain("Trang 2 / 3");
    await act(async () => buttonNamed(el, "Trước")!.click());
    await settle();
    expect(listCalls().at(-1)!.url).toContain("page=1&");
    // The person is named by the ROW id on every request — never a number.
    for (const c of calls) expect(c.url).not.toMatch(/09\d{8}/);
  });
});
