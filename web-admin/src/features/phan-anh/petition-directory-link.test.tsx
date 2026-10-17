// @vitest-environment jsdom
//
// jsdom: "Đồng bộ danh bạ" reads the network on mount and on a press, opens a dialog and re-reads after a
// save — events and state, which a static render cannot show. The network is a router over `fetch`;
// what is checked is what the drawer SENDS and what it SHOWS (ADR 0102).

import type { AnchorHTMLAttributes } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { PREFILL_UNVERIFIED_NOTE } from "@/features/citizen-directory/citizen-contact-dialog";
import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import {
  ADD_TO_DIRECTORY,
  DIRECTORY_MATCH_FAILED,
  PetitionDirectoryLink,
  VIEW_IN_DIRECTORY,
} from "./petition-directory-link";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
// A plain anchor: the App Router's Link needs a router this test does not mount.
vi.mock("next/link", () => ({
  default: ({ href, ...rest }: AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) => <a href={href} {...rest} />,
}));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const ALL_KEYS = ["feedback.read", "citizen.read", "feedback.unmask", "citizen.update"];

/** The drawer's copy is the LIST row: masked. Fake data only (rule 3, invariant 5). */
const PETITION = {
  code: "PA-7K2Q9X",
  anonymous: false,
  reporter_name: "Nguyễn V. A.",
  reporter_phone: "09****0001",
  address: "Ngõ 5, Thôn 3",
  residential_unit_id: "01JTHON3",
} as unknown as petitions_phieuPhanAnhRa;

/** The DETAIL route's answer to a `feedback.unmask` holder: the sender in full, the incident place beside it. */
const DETAIL = { ...PETITION, reporter_name: "Nguyễn Văn A", reporter_phone: "0900000001" };

type Call = { url: string; init: RequestInit | undefined };
const calls: Call[] = [];
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

let matchAnswers: Response[] = [];

function serve() {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init });
      if (url === "/api/v1/citizen-reports/PA-7K2Q9X/directory-match") return matchAnswers.shift() ?? json({ status: "not_applicable", contact_id: null, phone_unverified: false });
      if (url === "/api/v1/citizen-reports/PA-7K2Q9X") return json(DETAIL);
      if (url === "/api/v1/citizen-groups") return json({ items: [] });
      if (url.startsWith("/api/v1/citizen-fields")) return json({ items: [] });
      if (url === "/api/v1/citizen-contacts" && init?.method === "POST") return json({ id: "01JCONTACTNEW0000000000000" }, 201);
      return json({ code: "not_found", message: "Không có tuyến này.", trace_id: "" }, 404);
    }),
  );
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  calls.length = 0;
  matchAnswers = [];
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

async function mount(permissions: string[], petition = PETITION): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(<PetitionDirectoryLink petition={petition} permissions={permissions} />);
  });
  await settle();
  return host;
}

const matchCalls = () => calls.filter((c) => c.url.endsWith("/directory-match"));
const button = (el: HTMLElement, text: string, type = "button") =>
  [...el.querySelectorAll<HTMLButtonElement>(`button[type="${type}"]`)].find((b) => b.textContent?.includes(text)) ?? null;
const link = (el: HTMLElement, text: string) => [...el.querySelectorAll("a")].find((a) => a.textContent?.includes(text)) ?? null;
const match = (status: string, contactId: string | null, unverified = false) =>
  json({ status, contact_id: contactId, phone_unverified: unverified });

describe("gating (ADR 0102 #5) — the DENIED cases first", () => {
  it.each([
    ["no feedback.unmask", ["feedback.read", "citizen.read", "citizen.update"]],
    ["no citizen.update", ["feedback.read", "citizen.read", "feedback.unmask"]],
    ["no citizen.read", ["feedback.read", "feedback.unmask", "citizen.update"]],
    ["no feedback.read", ["citizen.read", "feedback.unmask", "citizen.update"]],
  ])("%s: nothing drawn, NOTHING requested", async (_name, permissions) => {
    matchAnswers = [match("not_found", null)];
    const el = await mount(permissions);
    expect(el.innerHTML).toBe("");
    expect(calls).toHaveLength(0);
  });

  it("all four keys: one match request, a GET, for this petition's lookup code", async () => {
    matchAnswers = [match("not_applicable", null)];
    await mount(ALL_KEYS);
    expect(matchCalls()).toHaveLength(1);
    expect(matchCalls()[0]!.init?.method).toBe("GET");
  });
});

describe("the four answers of directory-match", () => {
  it("not_applicable (anonymous / no number): no button, no sentence", async () => {
    matchAnswers = [match("not_applicable", null)];
    const el = await mount(ALL_KEYS);
    expect(el.querySelector("button")).toBeNull();
    expect(el.querySelector("a")).toBeNull();
    expect(el.textContent).not.toContain(DIRECTORY_MATCH_FAILED);
  });

  it("found: “Xem trong danh bạ” links to /nguoi-dan?id=<row id> — the id, never a number", async () => {
    matchAnswers = [match("found", "01JCONTACTA000000000000000")];
    const el = await mount(ALL_KEYS);
    expect(link(el, VIEW_IN_DIRECTORY)?.getAttribute("href")).toBe("/nguoi-dan?id=01JCONTACTA000000000000000");
    expect(button(el, ADD_TO_DIRECTORY)).toBeNull();
  });

  it("not_found: “Thêm vào danh bạ”, and nothing else is read until it is pressed", async () => {
    matchAnswers = [match("not_found", null)];
    const el = await mount(ALL_KEYS);
    expect(button(el, ADD_TO_DIRECTORY)).not.toBeNull();
    expect(link(el, VIEW_IN_DIRECTORY)).toBeNull();
    // The detail (an audited unmasked read) is NOT fetched on render.
    expect(calls.map((c) => c.url)).toEqual(["/api/v1/citizen-reports/PA-7K2Q9X/directory-match"]);
  });

  it.each([
    ["503 identity unreachable", () => json({ code: "identity_unavailable", message: "Danh bạ tạm thời không trả lời.", trace_id: "" }, 503)],
    ["403", () => json({ code: "forbidden", message: "Không có quyền.", trace_id: "" }, 403)],
    ["an unknown status", () => match("maybe", null)],
  ])("%s: the muted sentence — never a guessed “Thêm vào danh bạ”", async (_name, answer) => {
    matchAnswers = [answer()];
    const el = await mount(ALL_KEYS);
    expect(el.textContent).toContain(DIRECTORY_MATCH_FAILED);
    expect(button(el, ADD_TO_DIRECTORY)).toBeNull();
    expect(link(el, VIEW_IN_DIRECTORY)).toBeNull();
  });
});

describe("“Thêm vào danh bạ” — the directory's own add dialog, prefilled (ADR 0102 #6, #7)", () => {
  async function pressAdd(unverified: boolean): Promise<HTMLDivElement> {
    matchAnswers = [match("not_found", null, unverified), match("found", "01JCONTACTNEW0000000000000")];
    const el = await mount(ALL_KEYS);
    await act(async () => button(el, ADD_TO_DIRECTORY)!.click());
    await settle();
    return el;
  }
  const value = (el: HTMLElement, id: string) => el.querySelector<HTMLInputElement | HTMLSelectElement>(`#${id}`)!.value;

  it("name and FULL number from the detail route; hamlet and address left empty (the incident place)", async () => {
    const el = await pressAdd(false);
    expect(calls.some((c) => c.url === "/api/v1/citizen-reports/PA-7K2Q9X")).toBe(true);
    expect(value(el, "nguoi-dan-ho-ten")).toBe("Nguyễn Văn A");
    expect(value(el, "nguoi-dan-so-dien-thoai")).toBe("0900000001");
    expect(value(el, "nguoi-dan-dia-chi")).toBe("");
    expect(value(el, "nguoi-dan-thon")).toBe("");
    expect(el.textContent).not.toContain("Thôn 3");
    expect(el.textContent).not.toContain(PREFILL_UNVERIFIED_NOTE);
  });

  it("phone_unverified: the fixed warning line in the dialog", async () => {
    const el = await pressAdd(true);
    expect(el.querySelector("[data-prefill-unverified]")?.textContent).toBe(PREFILL_UNVERIFIED_NOTE);
  });

  it("save: POST without the address or hamlet, then directory-match is asked AGAIN and the button flips", async () => {
    const el = await pressAdd(false);
    await act(async () => button(el, ADD_TO_DIRECTORY, "submit")!.click());
    await settle();
    const post = calls.find((c) => c.url === "/api/v1/citizen-contacts" && c.init?.method === "POST")!;
    expect(JSON.parse(String(post.init?.body))).toMatchObject({
      full_name: "Nguyễn Văn A",
      phone: "0900000001",
      address: "",
      residential_unit_id: "",
    });
    expect(matchCalls()).toHaveLength(2);
    expect(link(el, VIEW_IN_DIRECTORY)?.getAttribute("href")).toBe("/nguoi-dan?id=01JCONTACTNEW0000000000000");
  });

  it("no request of the whole flow carries the number on its URL (rule 3, forbidden #4)", async () => {
    const el = await pressAdd(true);
    await act(async () => button(el, ADD_TO_DIRECTORY, "submit")!.click());
    await settle();
    expect(calls.length).toBeGreaterThan(3);
    for (const c of calls) {
      expect(c.url).not.toContain("0900000001");
      expect(c.url).not.toContain("0001");
    }
  });
});
