// @vitest-environment jsdom
//
// jsdom: the screen is driven through its buttons — which call each press sends, with which body, and
// which controls an account without `content.update` never sees.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { DisasterAlert, DisasterAlertPage } from "@/lib/api/disaster-alerts";
import type { KetQua } from "@/lib/api/goi";

import {
  ADD_BUTTON,
  CONTACTS_PUBLIC_HINT,
  DELETE_BUTTON,
  EMPTY_TITLE,
  END_BUTTON,
  LOAD_FAILED_TITLE,
  READ_ONLY_NOTE,
  SAVE_BUTTON,
} from "./alert-form";
import { DisasterAlertsScreen, type DisasterAlertsApi } from "./disaster-alerts-tab";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const NOW = Date.parse("2026-10-11T03:00:00Z");

function sessionWith(permissions: readonly string[]): PhienDaDoc {
  return {
    ok: true,
    duLieu: {
      sid: "01J000000000000000000SID",
      expires_at: "2026-10-11T12:00:00Z",
      staff: { code: "CB001", full_name: "Cán bộ thử", position: "Chuyên viên" },
      permissions: [...permissions],
    },
  } as PhienDaDoc;
}

function alert(over: Partial<DisasterAlert>): DisasterAlert {
  return {
    id: "01JALERT0000000000000000A1",
    kind: "bao",
    title: "Bão số 7",
    detail: "Chằng chống nhà cửa.",
    effective_from: "2026-10-11T01:00:00Z",
    in_force: true,
    danger_zone: [],
    evacuation_points: [],
    emergency_contacts: [],
    created_at: "2026-10-11T00:00:00Z",
    created_by: "CB-001",
    updated_at: "2026-10-11T00:00:00Z",
    updated_by: "CB-001",
    ...over,
  };
}

const ROWS: DisasterAlert[] = [
  alert({ id: "A1", title: "Bão số 7" }),
  alert({ id: "A2", kind: "lu", title: "Lũ sông Lam", in_force: false, effective_from: "2026-10-12T00:00:00Z" }),
  alert({ id: "A3", kind: "chay", title: "Cháy rừng", in_force: false, ended_at: "2026-10-10T00:00:00Z", ended_by: "CB-002" }),
];

function page(items: DisasterAlert[]): KetQua<DisasterAlertPage> {
  return { ok: true, duLieu: { items, next_cursor: "", has_more: false } };
}

function fakeApi(over: Partial<DisasterAlertsApi> = {}): DisasterAlertsApi {
  return {
    list: vi.fn(async () => page(ROWS)),
    get: vi.fn(async (id: string) => ({ ok: true as const, duLieu: ROWS.find((r) => r.id === id)! })),
    create: vi.fn(async () => ({ ok: true as const, duLieu: ROWS[0]! })),
    update: vi.fn(async () => ({ ok: true as const, duLieu: ROWS[0]! })),
    end: vi.fn(async () => ({ ok: true as const, duLieu: ROWS[0]! })),
    remove: vi.fn(async () => ({ ok: true as const, data: null })),
    ...over,
  };
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

async function mount(permissions: readonly string[], api: DisasterAlertsApi): Promise<void> {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(<DisasterAlertsScreen session={sessionWith(permissions)} api={api} now={() => NOW} />);
  });
}

function buttons(label: string): HTMLButtonElement[] {
  return Array.from(host!.querySelectorAll("button")).filter(
    (b) => b.textContent === label || (b.getAttribute("aria-label") ?? "").startsWith(`${label} — `),
  );
}

function type(el: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement, value: string): void {
  const proto = Object.getPrototypeOf(el) as object;
  Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
  act(() => {
    el.dispatchEvent(new Event(el instanceof HTMLSelectElement ? "change" : "input", { bubbles: true }));
  });
}

describe("permissions — hiding is convenience, the server still checks", () => {
  it("content.read only: the list, no write control, the read-only note", async () => {
    await mount(["content.read"], fakeApi());
    expect(host!.querySelectorAll('[data-testid="alert-row"]')).toHaveLength(3);
    expect(buttons(ADD_BUTTON)).toHaveLength(0);
    expect(buttons("Sửa")).toHaveLength(0);
    expect(buttons(END_BUTTON)).toHaveLength(0);
    expect(buttons(DELETE_BUTTON)).toHaveLength(0);
    expect(host!.textContent).toContain(READ_ONLY_NOTE);
  });

  it("content.update: add, and per row edit / end early / delete where they apply", async () => {
    await mount(["content.read", "content.update"], fakeApi());
    expect(buttons(ADD_BUTTON)).toHaveLength(1);
    expect(host!.textContent).not.toContain(READ_ONLY_NOTE);
    // A3 was ended early: no edit (409 at the server), no second ending — delete stays.
    expect(buttons("Sửa").map((b) => b.getAttribute("aria-label"))).toEqual(["Sửa — Bão số 7", "Sửa — Lũ sông Lam"]);
    expect(buttons(END_BUTTON).map((b) => b.getAttribute("aria-label"))).toEqual([
      "Kết thúc sớm — Bão số 7",
      "Kết thúc sớm — Lũ sông Lam",
    ]);
    expect(buttons(DELETE_BUTTON)).toHaveLength(3);
  });
});

describe("the list", () => {
  it("kind, title, window and the three badges", async () => {
    await mount(["content.read"], fakeApi());
    const rows = Array.from(host!.querySelectorAll('[data-testid="alert-row"]')).map((r) => r.textContent ?? "");
    expect(rows[0]).toContain("Bão");
    expect(rows[0]).toContain("Đang hiệu lực");
    expect(rows[0]).toContain("Từ 08:00 11/10/2026 — tới khi kết thúc");
    expect(rows[1]).toContain("Lũ");
    expect(rows[1]).toContain("Sắp hiệu lực");
    expect(rows[2]).toContain("Cháy");
    expect(rows[2]).toContain("Đã kết thúc");
  });

  it("empty and error states", async () => {
    await mount(["content.read"], fakeApi({ list: vi.fn(async () => page([])) }));
    expect(host!.textContent).toContain(EMPTY_TITLE);
    act(() => root!.unmount());
    host!.remove();
    await mount(["content.read"], fakeApi({ list: vi.fn(async () => ({ ok: false as const, thongBao: "Bạn không có quyền." })) }));
    expect(host!.textContent).toContain(LOAD_FAILED_TITLE);
    expect(host!.textContent).toContain("Bạn không có quyền.");
  });
});

describe("posting an alert", () => {
  it("Lưu on an empty form sends nothing and names the missing fields", async () => {
    const api = fakeApi();
    await mount(["content.read", "content.update"], api);
    act(() => buttons(ADD_BUTTON)[0]!.click());
    await act(async () => {
      buttons(SAVE_BUTTON)[0]!.form!.requestSubmit();
    });
    expect(api.create).not.toHaveBeenCalled();
    expect(host!.textContent).toContain("Vui lòng chọn loại cảnh báo.");
    expect(host!.textContent).toContain("Vui lòng nhập tiêu đề.");
  });

  it("the emergency-number list says, verbatim, that its numbers are public", async () => {
    await mount(["content.read", "content.update"], fakeApi());
    act(() => buttons(ADD_BUTTON)[0]!.click());
    expect(host!.querySelector('[data-testid="contacts-public-hint"]')!.textContent).toBe(
      "Số này hiện công khai cho mọi người dân — nên dùng số trực, đường dây nóng hoặc số cơ quan.",
    );
    expect(CONTACTS_PUBLIC_HINT).toContain("công khai");
  });

  it("a title carrying markup is shown as text, never parsed as HTML", async () => {
    const raw = '<img src=x onerror="alert(1)">Bão';
    await mount(["content.read"], fakeApi({ list: vi.fn(async () => page([alert({ id: "X1", title: raw })])) }));
    expect(host!.querySelector("img")).toBeNull();
    expect(host!.textContent).toContain(raw);
  });

  it("a filled form posts the contract's body with an Idempotency-Key, and the same key on a retry", async () => {
    const create = vi
      .fn<DisasterAlertsApi["create"]>()
      .mockResolvedValueOnce({ ok: false, thongBao: "Không kết nối được máy chủ. Vui lòng thử lại." })
      .mockResolvedValueOnce({ ok: true, duLieu: ROWS[0]! });
    const api = fakeApi({ create });
    await mount(["content.read", "content.update"], api);
    act(() => buttons(ADD_BUTTON)[0]!.click());
    type(host!.querySelector<HTMLSelectElement>("#alert-kind")!, "sat-lo");
    type(host!.querySelector<HTMLInputElement>("#alert-title")!, "Sạt lở đồi thôn 3");
    type(host!.querySelector<HTMLTextAreaElement>("#alert-detail")!, "Không đi qua đường đồi.");
    type(host!.querySelector<HTMLInputElement>("#alert-from")!, "2026-10-11T09:30");
    await act(async () => {
      buttons(SAVE_BUTTON)[0]!.form!.requestSubmit();
    });
    expect(create).toHaveBeenCalledTimes(1);
    const [body, key] = create.mock.calls[0]!;
    expect(body).toEqual({
      kind: "sat-lo",
      title: "Sạt lở đồi thôn 3",
      detail: "Không đi qua đường đồi.",
      effective_from: "2026-10-11T09:30:00+07:00",
    });
    expect(key).toMatch(/^[0-9a-f-]{36}$/);
    expect(host!.textContent).toContain("Không kết nối được máy chủ.");
    await act(async () => {
      buttons(SAVE_BUTTON)[0]!.form!.requestSubmit();
    });
    expect(create).toHaveBeenCalledTimes(2);
    expect(create.mock.calls[1]![1]).toBe(key);
  });
});

describe("ending early and deleting", () => {
  it("Kết thúc sớm asks first, then calls the ending route with the row's id", async () => {
    const api = fakeApi();
    await mount(["content.read", "content.update"], api);
    act(() => buttons(END_BUTTON)[0]!.click());
    expect(host!.textContent).toContain("Kết thúc sớm cảnh báo này?");
    expect(api.end).not.toHaveBeenCalled();
    const confirm = Array.from(host!.querySelectorAll("dialog button")).find((b) => b.textContent === END_BUTTON) as HTMLButtonElement;
    await act(async () => {
      confirm.click();
    });
    expect(api.end).toHaveBeenCalledWith("A1");
  });

  it("delete needs a reason, and sends it trimmed", async () => {
    const api = fakeApi();
    await mount(["content.read", "content.update"], api);
    act(() => buttons(DELETE_BUTTON)[1]!.click());
    const submit = Array.from(host!.querySelectorAll("dialog button")).find((b) => b.textContent === DELETE_BUTTON) as HTMLButtonElement;
    expect(submit.disabled).toBe(true);
    type(host!.querySelector<HTMLTextAreaElement>("#disaster-alert-delete-reason")!, "  Đăng nhầm  ");
    expect(submit.disabled).toBe(false);
    await act(async () => {
      submit.form!.requestSubmit();
    });
    expect(api.remove).toHaveBeenCalledWith("A2", "Đăng nhầm");
  });
});
