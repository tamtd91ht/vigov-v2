// @vitest-environment jsdom
//
// The whole screen (`SoNoiDung`), mounted, against a fake server: what the prototype's `ContentWorkspace`
// does on a press — the six tabs and their default, the page size, search as you type, the row's
// `Đăng`/`Gỡ` (owner D1, 09/10/2026), the add dialog's default type and its toasts.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { PhienProvider } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing export (rule 12 inv 3)
import type { comms_noiDungRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract type

import {
  CATEGORY_ADDED_TOAST,
  CATEGORY_DIALOG_DESCRIPTION,
  CREATED_DRAFT_TOAST,
  PUBLISH_TOGGLE_FAILED,
  PUBLISHED_TOAST,
  SAVE_FAILED_TOAST,
  TITLE_REQUIRED,
  UNPUBLISHED_TOAST,
} from "./nhan-noi-dung";
import { SoNoiDung } from "./so-noi-dung"; // vi-name-ok: existing export (rule 12 inv 3)

const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  T.success.mockClear();
  T.error.mockClear();
  T.info.mockClear();
});

function item(id: string, title: string, status = "dang-hien"): comms_noiDungRa { // vi-name-ok: generated contract type
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
    status,
    source: "dong-bo-cong",
    source_url: "",
    source_ref: "",
    hand_edited: false,
    author_code: "CB-2026-7K3M9Q",
    created_at: "2026-09-14T02:00:00Z",
    updated_at: "2026-09-14T09:35:00Z",
  } as comms_noiDungRa; // vi-name-ok: generated contract type
}

const SHOWING = item("01JNDA", "Lịch tiêm phòng tháng 10");
const HIDDEN = item("01JNDB", "Thông báo nghỉ lễ", "an");

function json(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

type Call = { path: string; method: string; body: unknown };

function fakeServer(
  permissions: string[],
  opts: { patch?: Response; post?: Response; hasMore?: boolean } = {},
): () => Call[] {
  const f = vi.fn(async (path: string, init?: RequestInit): Promise<Response> => {
    const method = init?.method ?? "GET";
    if (path === "/api/v1/sessions/current") return json(200, { permissions, must_change_password: false });
    if (method === "GET" && path.startsWith("/api/v1/content-items")) {
      return json(200, { items: [SHOWING, HIDDEN], has_more: opts.hasMore ?? false, next_cursor: opts.hasMore ? "C2" : "" });
    }
    if (method === "PATCH") return (opts.patch ?? json(200, SHOWING)).clone();
    if (method === "POST" && path === "/api/v1/content-items") return (opts.post ?? json(201, SHOWING)).clone();
    if (method === "POST" && path === "/api/v1/content-categories") {
      return json(201, { id: "01JDMN", name: "Nông nghiệp", slug: "nong-nghiep", parent_id: "", order: 0, created_at: "2026-10-09T02:00:00Z", hidden: false });
    }
    if (path.startsWith("/api/v1/content-categories") || path.startsWith("/api/v1/commune-staff")) {
      return json(200, { items: [] });
    }
    return json(404, { code: "not_found", message: "Không có." });
  });
  vi.stubGlobal("fetch", f);
  return () =>
    f.mock.calls.map(([p, i]) => ({
      path: p,
      method: i?.method ?? "GET",
      body: i?.body === undefined ? undefined : (JSON.parse(String(i.body)) as unknown),
    }));
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
  vi.useRealTimers();
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

const listReads = (calls: Call[]) =>
  calls.filter((c) => c.method === "GET" && c.path.startsWith("/api/v1/content-items")).map((c) => c.path);

function byLabel(label: string): HTMLButtonElement | undefined {
  return Array.from(host!.querySelectorAll("button")).find((b) => b.getAttribute("aria-label") === label);
}

function byText(text: string): HTMLButtonElement {
  const b = Array.from(host!.querySelectorAll("button")).find((x) => x.textContent === text);
  if (b === undefined) throw new Error(`no button "${text}"`);
  return b;
}

function typeInto(el: HTMLInputElement, value: string): void {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(el, value);
  act(() => {
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

describe("opening the screen", () => {
  it("opens on `Tin tức` (no `Tất cả`), 25 rows a page", async () => {
    const calls = fakeServer(["content.read"]);
    await mountScreen();
    expect(listReads(calls())).toEqual(["/api/v1/content-items?type=tin-tuc&limit=25"]);
    expect(host!.querySelector('[role="tab"][aria-selected="true"]')?.textContent).toBe("Tin tức");
    expect(host!.querySelectorAll('[role="tab"]')).toHaveLength(6);
  });

  it("the page header: the prototype's title and sentence, no icon", async () => {
    fakeServer(["content.read"]);
    await mountScreen();
    const h1 = host!.querySelector("h1")!;
    expect(h1.textContent).toBe("Quản trị nội dung Mini App");
    expect(h1.querySelector("svg")).toBeNull();
    expect(h1.parentElement!.parentElement!.className).toBe("mb-5 flex flex-wrap items-end gap-4");
  });

  it("a tab change reads that type from page 1", async () => {
    const calls = fakeServer(["content.read"], { hasMore: true });
    await mountScreen();
    await act(async () => byText("Sau").click());
    await settle();
    expect(listReads(calls()).at(-1)).toBe("/api/v1/content-items?type=tin-tuc&limit=25&cursor=C2");
    await act(async () => byText("Video").click());
    await settle();
    expect(listReads(calls()).at(-1)).toBe("/api/v1/content-items?type=video&limit=25");
  });
});

describe("search filters as you type — one request per pause, never in the address bar", () => {
  it("no request while typing; one, page 1, 300 ms after the last key", async () => {
    const calls = fakeServer(["content.read"]);
    await mountScreen();
    const before = listReads(calls()).length;
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const box = host!.querySelector<HTMLInputElement>("#tim-noi-dung")!;
    typeInto(box, "lịch");
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200);
    });
    typeInto(box, "lịch tiêm");
    await act(async () => {
      await vi.advanceTimersByTimeAsync(299);
    });
    expect(listReads(calls()).length).toBe(before);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    vi.useRealTimers();
    await settle();
    const reads = listReads(calls());
    expect(reads.length).toBe(before + 1);
    expect(reads.at(-1)).toBe(`/api/v1/content-items?type=tin-tuc&q=${encodeURIComponent("lịch tiêm").replace(/%20/g, "+")}&limit=25`);
    expect(window.location.href).not.toContain("tiêm");
    expect(window.location.href).not.toContain(encodeURIComponent("tiêm"));
  });
});

describe("Đăng / Gỡ (owner D1) — PATCH body is `{publish}` ONLY", () => {
  it("`Gỡ` on a showing row: `{publish:false}`, the prototype's toast, the same page read again", async () => {
    const calls = fakeServer(["content.read", "content.update"]);
    await mountScreen();
    const readsBefore = listReads(calls()).length;
    await act(async () => byLabel(`Gỡ: ${SHOWING.title}`)!.click());
    await settle();
    const patches = calls().filter((c) => c.method === "PATCH");
    expect(patches).toEqual([{ path: "/api/v1/content-items/01JNDA", method: "PATCH", body: { publish: false } }]);
    expect(T.success).toHaveBeenCalledWith(UNPUBLISHED_TOAST);
    expect(listReads(calls()).length).toBe(readsBefore + 1);
    expect(listReads(calls()).at(-1)).toBe(listReads(calls())[0]);
  });

  it("`Đăng` on a hidden row: `{publish:true}` and `Đã đăng.`", async () => {
    const calls = fakeServer(["content.read", "content.update"]);
    await mountScreen();
    await act(async () => byLabel(`Đăng: ${HIDDEN.title}`)!.click());
    await settle();
    expect(calls().filter((c) => c.method === "PATCH")).toEqual([
      { path: "/api/v1/content-items/01JNDB", method: "PATCH", body: { publish: true } },
    ]);
    expect(T.success).toHaveBeenCalledWith(PUBLISHED_TOAST);
  });

  it("a refusal: the prototype's toast title ONLY (no description, `ContentWorkspace.tsx:149`); nothing re-read", async () => {
    const calls = fakeServer(["content.read", "content.update"], {
      patch: json(403, { code: "forbidden", message: "Bạn không có quyền thực hiện thao tác này." }),
    });
    await mountScreen();
    const readsBefore = listReads(calls()).length;
    await act(async () => byLabel(`Gỡ: ${SHOWING.title}`)!.click());
    await settle();
    expect(PUBLISH_TOGGLE_FAILED).toBe("Không đổi được trạng thái đăng.");
    expect(T.error).toHaveBeenCalledTimes(1);
    expect(T.error.mock.calls[0]).toEqual([PUBLISH_TOGGLE_FAILED]);
    expect(T.success).not.toHaveBeenCalled();
    expect(listReads(calls()).length).toBe(readsBefore);
  });

  it("DENIED (`content.read` only): no `Đăng`, no `Gỡ`, the title is not a button", async () => {
    fakeServer(["content.read"]);
    await mountScreen();
    expect(host!.textContent).toContain(SHOWING.title);
    expect(byLabel(`Gỡ: ${SHOWING.title}`)).toBeUndefined();
    expect(byLabel(`Đăng: ${HIDDEN.title}`)).toBeUndefined();
    expect(Array.from(host!.querySelectorAll("button")).some((b) => b.textContent === SHOWING.title)).toBe(false);
    expect(Array.from(host!.querySelectorAll("button")).some((b) => b.textContent === "Thêm nội dung")).toBe(false);
  });
});

describe("Thêm nội dung", () => {
  async function openAdd(): Promise<void> {
    await act(async () => byText("Thêm nội dung").click());
    await settle();
  }

  it("the new item's type is the tab being looked at", async () => {
    fakeServer(["content.read", "content.update"]);
    await mountScreen();
    await act(async () => byText("Truyền thanh").click());
    await settle();
    await openAdd();
    expect(host!.querySelector<HTMLSelectElement>("#loai-noi-dung")!.value).toBe("truyen-thanh");
    expect(host!.querySelector<HTMLSelectElement>("#loai-noi-dung")!.disabled).toBe(false);
  });

  it("Lưu with an empty title: `Vui lòng nhập tiêu đề` under the box, nothing sent", async () => {
    const calls = fakeServer(["content.read", "content.update"]);
    await mountScreen();
    await openAdd();
    await act(async () => {
      host!.querySelector<HTMLFormElement>("dialog form")!.requestSubmit();
    });
    expect(host!.querySelector("dialog")!.textContent).toContain(TITLE_REQUIRED);
    expect(calls().filter((c) => c.method === "POST")).toHaveLength(0);
  });

  it("a draft saved: `Đã lưu bản nháp.` and the dialog closes", async () => {
    const calls = fakeServer(["content.read", "content.update"]);
    await mountScreen();
    await openAdd();
    typeInto(host!.querySelector<HTMLInputElement>("#tieu-de-noi-dung")!, "Lịch tiếp dân");
    await act(async () => {
      host!.querySelector<HTMLFormElement>("dialog form")!.requestSubmit();
    });
    await settle();
    expect(calls().filter((c) => c.method === "POST")).toHaveLength(1);
    expect(T.success).toHaveBeenCalledWith(CREATED_DRAFT_TOAST);
    expect(host!.querySelector("dialog")).toBeNull();
  });

  it("a refusal: the error toast, and the server's sentence stays inline in the open dialog", async () => {
    const sentence = "Tiêu đề quá dài (tối đa 300 ký tự).";
    fakeServer(["content.read", "content.update"], { post: json(400, { code: "invalid_request", message: sentence }) });
    await mountScreen();
    await openAdd();
    typeInto(host!.querySelector<HTMLInputElement>("#tieu-de-noi-dung")!, "Lịch tiếp dân");
    await act(async () => {
      host!.querySelector<HTMLFormElement>("dialog form")!.requestSubmit();
    });
    await settle();
    expect(T.error).toHaveBeenCalledWith(SAVE_FAILED_TOAST);
    expect(host!.querySelector("dialog")!.textContent).toContain(sentence);
  });
});

describe("Danh mục tin — the prototype's `CategoryManagerDialog`", () => {
  async function openCategories(): Promise<void> {
    await act(async () => byText("Danh mục tin").click());
    await settle();
  }

  it("~44rem wide, a plain title and ONE sentence, the import box, the add row, then the list", async () => {
    fakeServer(["content.read", "content.update"]);
    await mountScreen();
    await openCategories();
    const dialog = host!.querySelector("dialog")!;
    expect(dialog.className).toContain("w-[min(44rem,calc(100vw-1rem))]");
    const title = dialog.querySelector("h3")!;
    expect(title.textContent).toBe("Danh mục tin");
    expect(title.querySelector("svg")).toBeNull();
    expect(dialog.textContent).toContain(CATEGORY_DIALOG_DESCRIPTION);
    expect(CATEGORY_DIALOG_DESCRIPTION).toBe("Bà con lọc tin theo danh mục này trên Mini App.");
    const text = dialog.textContent!;
    const at = (t: string) => text.indexOf(t);
    expect(at("Lấy đủ cây chuyên mục")).toBeGreaterThan(-1);
    expect(at("Lấy đủ cây chuyên mục")).toBeLessThan(at("Thêm danh mục"));
    expect(at("Thêm danh mục")).toBeLessThan(at("Chưa có danh mục nào. Thêm ở trên."));
  });

  it("`Thêm`: POST name/slug/parent — no order — the toast, and the dialog STAYS open", async () => {
    const calls = fakeServer(["content.read", "content.update"]);
    await mountScreen();
    await openCategories();
    typeInto(host!.querySelector<HTMLInputElement>("#ten-danh-muc")!, "Nông nghiệp");
    typeInto(host!.querySelector<HTMLInputElement>("#slug-danh-muc")!, "nong-nghiep");
    await act(async () => {
      host!.querySelector<HTMLInputElement>("#ten-danh-muc")!.form!.requestSubmit();
    });
    await settle();
    const post = calls().filter((c) => c.method === "POST" && c.path === "/api/v1/content-categories");
    expect(post).toHaveLength(1);
    expect(post[0]!.body).toEqual({ name: "Nông nghiệp", slug: "nong-nghiep", parent_id: "" });
    expect(T.success).toHaveBeenCalledWith(CATEGORY_ADDED_TOAST);
    expect(CATEGORY_ADDED_TOAST).toBe("Đã thêm danh mục.");
    expect(host!.querySelector("dialog")).not.toBeNull();
    expect(host!.querySelector<HTMLInputElement>("#ten-danh-muc")!.value).toBe("");
  });
});
