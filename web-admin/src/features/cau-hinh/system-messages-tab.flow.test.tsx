// @vitest-environment jsdom
//
// jsdom: adding, switching and deleting are behaviour — clicked, and read from the requests sent, not
// from markup. The markup itself is pinned in `system-messages-tab.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { SystemMessage } from "@/lib/api/system-messages";

const H = vi.hoisted(() => ({ phien: null as PhienDaDoc }));
vi.mock("@/features/phien/phien-hien-tai", () => ({ usePhien: () => H.phien }));
const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));

const { SystemMessagesTab } = await import("./system-messages-tab");

function session(permissions: string[]): PhienDaDoc {
  return { ok: true, duLieu: { staff: { full_name: "A", position: "B" }, role: null, permissions } } as unknown as PhienDaDoc;
}

const SHIPPED: SystemMessage = {
  code: "feedback.reason_required",
  group_code: "phan-anh",
  origin: "shipped",
  description: "Khi bỏ trống lý do",
  default_text: "Phải ghi rõ lý do.",
  current_text: "Xin ghi lý do.",
  override_text: "Xin ghi lý do.",
  overridden: true,
  is_active: true,
};
const SHARED: SystemMessage = {
  code: "chung.loi-chao",
  group_code: "chung",
  origin: "commune",
  description: "",
  current_text: "Xã xin chào.",
  overridden: false,
  is_active: true,
};
const FINANCE_COMMUNE: SystemMessage = { ...SHARED, code: "giai-ngan.nhac", group_code: "giai-ngan", current_text: "Nhắc." };

type Call = { method: string; url: string; headers: Headers; body?: unknown };
let calls: Call[] = [];
let lists: Record<string, SystemMessage[]> = {};
let writeReply: { status: number; body?: unknown } = { status: 200 };

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  calls = [];
  lists = { petitions: [SHIPPED, SHARED], finance: [FINANCE_COMMUNE], reporting: [] };
  writeReply = { status: 200 };
  T.success.mockReset();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init: RequestInit = {}) => {
      const method = String(init.method ?? "GET");
      calls.push({
        method,
        url,
        headers: new Headers(init.headers),
        body: typeof init.body === "string" ? (JSON.parse(init.body) as unknown) : undefined,
      });
      if (method === "GET") {
        const service = /\/api\/v1\/(\w+)-system-messages$/.exec(url)![1]!;
        return new Response(JSON.stringify({ items: lists[service] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      return writeReply.body === undefined
        ? new Response(null, { status: writeReply.status })
        : new Response(JSON.stringify(writeReply.body), {
            status: writeReply.status,
            headers: { "Content-Type": "application/json" },
          });
    }),
  );
});

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  H.phien = null;
  vi.unstubAllGlobals();
});

async function settle() {
  for (let i = 0; i < 6; i++) await act(async () => {});
}

async function mount(): Promise<HTMLDivElement> {
  host = document.createElement("div");
  document.body.append(host);
  const r = createRoot(host);
  root = r;
  act(() => r.render(<SystemMessagesTab />));
  await settle();
  return host;
}

/** Types into a React-controlled input or textarea. */
function type(el: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")!.set!;
  act(() => {
    setter.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function button(scope: ParentNode, label: string): HTMLButtonElement {
  const b = [...scope.querySelectorAll("button")].find((x) => x.textContent?.trim() === label);
  expect(b, `button "${label}"`).toBeDefined();
  return b!;
}

function cardOf(el: HTMLElement, code: string): HTMLElement {
  return el.querySelector<HTMLElement>(`article[aria-label="${code}"]`)!;
}

const writes = () => calls.filter((c) => c.method !== "GET");
const gets = (module: string) => calls.filter((c) => c.method === "GET" && c.url.includes(`/${module}-system-messages`));

describe("denied case", () => {
  it("without admin.lookup: the tab's sentence, NO request at all, no add button", async () => {
    H.phien = session(["task.read"]);
    const el = await mount();
    expect(el.textContent).toContain("không có quyền sửa lời hệ thống");
    expect(el.textContent).not.toContain("Thêm câu mới");
    expect(calls).toEqual([]);
  });
});

describe("sections", () => {
  it("'Dùng chung' appears when petitions lists a `chung` sentence, sorted between Báo cáo and Giải ngân", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    const titles = [...el.querySelectorAll("h3")].map((h) => h.textContent);
    expect(titles).toEqual(["Báo cáo điều hành", "Dùng chung", "Theo dõi giải ngân", "Phản ánh của người dân"]);
    const shared = [...el.querySelectorAll("section section")].find((s) => s.querySelector("h3")?.textContent === "Dùng chung")!;
    expect(shared.querySelector('article[aria-label="chung.loi-chao"]')).not.toBeNull();
    expect(shared.querySelector('article[aria-label="feedback.reason_required"]')).toBeNull();
  });

  it("no `chung` sentence → no 'Dùng chung' section", async () => {
    lists.petitions = [SHIPPED];
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    expect([...el.querySelectorAll("h3")].map((h) => h.textContent)).not.toContain("Dùng chung");
  });
});

describe("Thêm câu mới", () => {
  it("opens the form; empty → spec 07's sentence in place and NOTHING sent; filled → POST with Idempotency-Key, toast, form closes, petitions re-read", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    expect(el.querySelector('form[aria-label="Thêm câu hệ thống của xã"]')).toBeNull();
    act(() => button(el, "Thêm câu mới").click());
    const formEl = el.querySelector<HTMLFormElement>('form[aria-label="Thêm câu hệ thống của xã"]')!;
    expect(formEl).not.toBeNull();

    act(() => button(formEl, "Thêm câu").click());
    await settle();
    expect(formEl.textContent).toContain("Cần cả mã và nội dung câu.");
    expect(writes()).toEqual([]);

    type(el.querySelector<HTMLInputElement>("#loi-he-thong-them-ma")!, "chung.cam-on");
    type(el.querySelector<HTMLTextAreaElement>("#loi-he-thong-them-noi-dung")!, " Cảm ơn ông/bà. ");
    writeReply = { status: 201, body: { ...SHARED, code: "chung.cam-on", current_text: "Cảm ơn ông/bà." } };
    const petitionsReads = gets("petitions").length;
    act(() => button(el.querySelector('form[aria-label="Thêm câu hệ thống của xã"]')!, "Thêm câu").click());
    await settle();

    const post = writes()[0]!;
    expect(post.method).toBe("POST");
    expect(post.url).toBe("/api/v1/petitions-system-messages");
    expect(post.headers.get("Idempotency-Key")).toMatch(/^[0-9a-f-]{36}$/);
    expect(post.body).toEqual({ group_code: "chung", code: "chung.cam-on", text: "Cảm ơn ông/bà." });
    expect(T.success).toHaveBeenCalledWith("Đã thêm câu mới.");
    expect(el.querySelector('form[aria-label="Thêm câu hệ thống của xã"]')).toBeNull();
    expect(gets("petitions").length).toBe(petitionsReads + 1);
  });

  it("a code without a known prefix: the prefix sentence in place, nothing sent; no group select exists", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    act(() => button(el, "Thêm câu mới").click());
    const formEl = () => el.querySelector('form[aria-label="Thêm câu hệ thống của xã"]')!;
    expect(formEl().querySelector("select")).toBeNull();
    type(el.querySelector<HTMLInputElement>("#loi-he-thong-them-ma")!, "bao-cao.x");
    type(el.querySelector<HTMLTextAreaElement>("#loi-he-thong-them-noi-dung")!, "Câu.");
    act(() => button(formEl(), "Thêm câu").click());
    await settle();
    expect(formEl().textContent).toContain("Mã câu phải bắt đầu bằng chung., phan-anh. hoặc giai-ngan.");
    expect(writes()).toEqual([]);
  });

  it("a retry after a refusal reuses the SAME Idempotency-Key; a 'giai-ngan.' code goes to finance", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    act(() => button(el, "Thêm câu mới").click());
    expect(el.querySelector<HTMLInputElement>("#loi-he-thong-them-ma")!.placeholder).toBe("chung.loi-chao");
    type(el.querySelector<HTMLInputElement>("#loi-he-thong-them-ma")!, "giai-ngan.nhac");
    type(el.querySelector<HTMLTextAreaElement>("#loi-he-thong-them-noi-dung")!, "Nhắc.");
    writeReply = { status: 409, body: { code: "message_code_taken", message: "Mã này đã được dùng cho một câu khác." } };
    const formEl = () => el.querySelector('form[aria-label="Thêm câu hệ thống của xã"]')!;
    act(() => button(formEl(), "Thêm câu").click());
    await settle();
    expect(formEl().textContent).toContain("Mã này đã được dùng cho một câu khác.");
    act(() => button(formEl(), "Thêm câu").click());
    await settle();
    const [a, b] = writes();
    expect(a!.url).toBe("/api/v1/finance-system-messages");
    expect(a!.body).toEqual({ group_code: "giai-ngan", code: "giai-ngan.nhac", text: "Nhắc." });
    expect(a!.headers.get("Idempotency-Key")).toBe(b!.headers.get("Idempotency-Key"));
  });
});

describe("Tắt / Bật lại", () => {
  it("a commune sentence: PATCH …/{code} { is_active: false }; the card turns 'Bật lại', dimmed, 'Đang tắt'", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    writeReply = { status: 200, body: { ...SHARED, is_active: false } };
    act(() => button(cardOf(el, "chung.loi-chao"), "Tắt").click());
    await settle();
    expect(writes()[0]).toMatchObject({
      method: "PATCH",
      url: "/api/v1/petitions-system-messages/chung.loi-chao",
      body: { is_active: false },
    });
    const card = cardOf(el, "chung.loi-chao");
    expect(card.className).toContain("opacity-60");
    expect(card.textContent).toContain("Đang tắt");
    expect(button(card, "Bật lại")).toBeDefined();
    expect(T.success).toHaveBeenCalledWith("Đã tắt câu này.");
  });

  it("a reworded shipped sentence: PATCH …/{code}/override; the box keeps the commune's words", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    writeReply = {
      status: 200,
      body: { ...SHIPPED, is_active: false, current_text: SHIPPED.default_text },
    };
    act(() => button(cardOf(el, "feedback.reason_required"), "Tắt").click());
    await settle();
    expect(writes()[0]).toMatchObject({
      method: "PATCH",
      url: "/api/v1/petitions-system-messages/feedback.reason_required/override",
      body: { is_active: false },
    });
    const card = cardOf(el, "feedback.reason_required");
    expect(card.textContent).toContain("Đang tắt — dùng lời gốc");
    expect(card.querySelector("textarea")!.value).toBe("Xin ghi lý do.");
  });
});

describe("Xoá (câu xã tự thêm)", () => {
  it("asks for a REASON; empty → refused in place, nothing sent; given → DELETE with { reason }, toast, finance re-read", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    act(() => button(cardOf(el, "giai-ngan.nhac"), "Xoá").click());
    const step = () => cardOf(el, "giai-ngan.nhac").querySelector<HTMLFormElement>('form[aria-label="Xoá câu giai-ngan.nhac"]')!;
    expect(step()).not.toBeNull();

    act(() => step().requestSubmit());
    await settle();
    expect(step().textContent).toContain("Hãy nêu lý do xoá câu này.");
    expect(writes()).toEqual([]);

    type(step().querySelector("input")!, " Không dùng nữa. ");
    writeReply = { status: 204 };
    const financeReads = gets("finance").length;
    act(() => step().requestSubmit());
    await settle();
    expect(writes()[0]).toMatchObject({
      method: "DELETE",
      url: "/api/v1/finance-system-messages/giai-ngan.nhac",
      body: { reason: "Không dùng nữa." },
    });
    expect(T.success).toHaveBeenCalledWith("Đã xoá câu này.");
    expect(gets("finance").length).toBe(financeReads + 1);
  });

  it("Huỷ closes the step without sending anything", async () => {
    H.phien = session(["admin.lookup"]);
    const el = await mount();
    act(() => button(cardOf(el, "giai-ngan.nhac"), "Xoá").click());
    act(() => button(cardOf(el, "giai-ngan.nhac"), "Huỷ").click());
    expect(cardOf(el, "giai-ngan.nhac").querySelector('form[aria-label="Xoá câu giai-ngan.nhac"]')).toBeNull();
    expect(writes()).toEqual([]);
  });
});
