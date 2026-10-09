// @vitest-environment jsdom
//
// The whole screen (`SoNoiDung`), mounted, against a fake server: what the page does AROUND the delete
// dialog — the button's gate, closing, reloading the same page, the line above the table. The dialog's
// own rules (reason, counter, in-flight) are `content-delete-dialog.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing export (rule 12 inv 3)
import type { comms_noiDungRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract type

import {
  CONTENT_DELETE_GONE,
  CONTENT_DELETE_SUBMIT,
  CONTENT_DELETED,
  contentDeleteAriaLabel,
} from "./nhan-noi-dung";
import { SoNoiDung } from "./so-noi-dung"; // vi-name-ok: existing export (rule 12 inv 3)

const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));

beforeEach(() => {
  T.success.mockClear();
  T.error.mockClear();
  T.info.mockClear();
});

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

function item(id: string, title: string): comms_noiDungRa { // vi-name-ok: generated contract type
  return {
    id,
    type: "tin-tuc",
    category_id: "",
    title,
    summary: "",
    image_url: "",
    has_image: false,
    published_on: "2026-09-14",
    view_count: 0,
    status: "dang-hien",
    source: "thu-cong",
    source_url: "",
    source_ref: "",
    hand_edited: false,
    author_code: "CB-2026-7K3M9Q",
    created_at: "2026-09-14T02:00:00Z",
    updated_at: "2026-09-14T09:35:00Z",
  } as comms_noiDungRa; // vi-name-ok: generated contract type
}

const A = item("01JNDA", "Lịch tiêm phòng tháng 10");
const B = item("01JNDB", "Thông báo nghỉ lễ");

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

type FakeServer = {
  listReads: () => string[];
  deletes: () => { path: string; body: unknown }[];
};

/** `pages` answers each list read in turn (the last one repeats); `deleteAnswer` answers the DELETE. */
function fakeServer(permissions: string[], pages: comms_noiDungRa[][], deleteAnswer: Response): FakeServer { // vi-name-ok: generated contract type
  let reads = 0;
  const f = vi.fn(async (path: string, init?: RequestInit): Promise<Response> => {
    const method = init?.method ?? "GET";
    if (path === "/api/v1/sessions/current") {
      return json(200, { permissions, must_change_password: false });
    }
    if (method === "GET" && path.startsWith("/api/v1/content-items")) {
      const page = pages[Math.min(reads, pages.length - 1)] ?? [];
      reads += 1;
      return json(200, { items: page, has_more: false, next_cursor: "" });
    }
    if (method === "DELETE") return deleteAnswer.clone();
    if (path.startsWith("/api/v1/content-categories") || path.startsWith("/api/v1/commune-staff")) {
      return json(200, { items: [] });
    }
    // The portal sync card's reads: not this test's concern.
    return json(404, { code: "not_found", message: "Không có." });
  });
  vi.stubGlobal("fetch", f);
  return {
    listReads: () =>
      f.mock.calls
        .filter(([p, i]) => (i?.method ?? "GET") === "GET" && p.startsWith("/api/v1/content-items"))
        .map(([p]) => p),
    deletes: () =>
      f.mock.calls
        .filter(([, i]) => i?.method === "DELETE")
        .map(([p, i]) => ({ path: p, body: JSON.parse(String(i?.body)) as unknown })),
  };
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

async function settle(): Promise<void> {
  for (let i = 0; i < 5; i += 1) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

async function mountScreen(): Promise<void> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <PhienProvider>
        <SoNoiDung />
      </PhienProvider>,
    ),
  );
  await settle();
}

function byLabel(label: string): HTMLButtonElement | undefined {
  return Array.from(host!.querySelectorAll("button")).find((b) => b.getAttribute("aria-label") === label);
}

function byText(text: string): HTMLButtonElement {
  const b = Array.from(host!.querySelectorAll("button")).find((x) => x.textContent === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

function typeReason(value: string): void {
  const el = host!.querySelector<HTMLTextAreaElement>("#content-delete-reason")!;
  Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(el, value);
  act(() => {
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function deleteRow(title: string, reason: string): Promise<void> {
  await act(async () => {
    byLabel(contentDeleteAriaLabel(title))!.click();
  });
  typeReason(reason);
  await act(async () => {
    byText(CONTENT_DELETE_SUBMIT).form!.requestSubmit();
  });
  await settle();
}


describe("the delete button follows `content.update`", () => {
  it("DENIED (`content.read` only): the rows are there, no delete button on any of them", async () => {
    fakeServer(["content.read"], [[A, B]], new Response(null, { status: 204 }));
    await mountScreen();
    expect(host!.textContent).toContain(A.title);
    expect(byLabel(contentDeleteAriaLabel(A.title))).toBeUndefined();
    expect(byLabel(contentDeleteAriaLabel(B.title))).toBeUndefined();
  });

  it("ALLOWED: one per row", async () => {
    fakeServer(["content.read", "content.update"], [[A, B]], new Response(null, { status: 204 }));
    await mountScreen();
    expect(byLabel(contentDeleteAriaLabel(A.title))).toBeDefined();
    expect(byLabel(contentDeleteAriaLabel(B.title))).toBeDefined();
  });
});

describe("after the server answered", () => {
  it("204: the dialog closes, the same page is read again, the row is gone, the success TOAST shows", async () => {
    const s = fakeServer(["content.read", "content.update"], [[A, B], [B]], new Response(null, { status: 204 }));
    await mountScreen();
    expect(s.listReads()).toHaveLength(1);

    await deleteRow(A.title, "  Đăng trùng với bài ngày 14/9  ");

    expect(s.deletes()).toEqual([
      { path: "/api/v1/content-items/01JNDA", body: { reason: "Đăng trùng với bài ngày 14/9" } },
    ]);
    expect(host!.querySelector("#content-delete-reason")).toBeNull();
    expect(s.listReads()).toHaveLength(2);
    expect(s.listReads()[1]).toBe(s.listReads()[0]);
    expect(host!.textContent).not.toContain(A.title);
    expect(host!.textContent).toContain(B.title);
    expect(T.success).toHaveBeenCalledWith(CONTENT_DELETED);
    // The prototype's wording, verbatim (`ContentWorkspace.tsx:156`) — the act itself stays a soft delete.
    expect(CONTENT_DELETED).toBe("Đã gỡ khỏi Mini App.");
    // A toast, not a line above the table (prototype; owner 09/10/2026 row 34).
    expect(host!.textContent).not.toContain(CONTENT_DELETED);
  });

  it("404: the dialog closes, the officer is told it is no longer there, the list is read again", async () => {
    const s = fakeServer(
      ["content.read", "content.update"],
      [[A, B], [B]],
      json(404, { code: "not_found", message: "Không tìm thấy nội dung này." }),
    );
    await mountScreen();

    await deleteRow(A.title, "Không còn phù hợp");

    expect(host!.querySelector("#content-delete-reason")).toBeNull();
    expect(s.listReads()).toHaveLength(2);
    expect(T.info).toHaveBeenCalledWith(CONTENT_DELETE_GONE);
  });

  it("400: the dialog stays with the reason and the server's sentence; the list is NOT read again", async () => {
    const sentence = "Lý do xoá quá dài (tối đa 500 ký tự).";
    const s = fakeServer(
      ["content.read", "content.update"],
      [[A, B]],
      json(400, { code: "invalid_request", message: sentence }),
    );
    await mountScreen();

    await deleteRow(A.title, "Đăng nhầm");

    expect(host!.querySelector<HTMLTextAreaElement>("#content-delete-reason")?.value).toBe("Đăng nhầm");
    expect(host!.querySelector("dialog [role=\"alert\"]")?.textContent).toBe(sentence);
    expect(s.listReads()).toHaveLength(1);
    expect(T.success).not.toHaveBeenCalled();
  });
});
