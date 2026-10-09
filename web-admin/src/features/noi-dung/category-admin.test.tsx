// @vitest-environment jsdom
//
// jsdom for this file: these tests PRESS the buttons — the dialogs' whole job is which body a click
// sends, and a string render cannot click. Same reasoning as `rich-text.test.ts`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { pendingMarkerLabel } from "@/components/ui/pending-feature";

import type { KetQua } from "@/lib/api/goi";
import type { UpdateCategoryIn } from "@/lib/api/noi-dung";
import type { comms_danhMucRa } from "@/lib/api/schema.gen";

const T = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }));
vi.mock("sonner", () => ({ toast: T }));

import { CategoryAdmin, CategoryImportPending } from "./category-admin";
import {
  CATEGORY_EMPTY,
  CATEGORY_FROM_PORTAL_PART,
  CATEGORY_IMPORT_PART,
  CATEGORY_ITEM_COUNT_PART,
  CATEGORY_DELETED_TOAST,
  CATEGORY_RENAMED_TOAST,
} from "./nhan-noi-dung";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

beforeEach(() => {
  T.success.mockClear();
});

function dm(over: Partial<comms_danhMucRa>): comms_danhMucRa {
  return {
    id: "A",
    name: "Tin xã",
    slug: "tin-xa",
    parent_id: "",
    order: 0,
    created_at: "2026-09-01T02:00:00Z",
    hidden: false,
    ...over,
  };
}

const TREE = [
  dm({ id: "A", name: "Tin xã", slug: "tin-xa" }),
  dm({ id: "B", name: "Thôn Một", slug: "thon-mot", parent_id: "A" }),
  dm({ id: "C", name: "Tổ 1", slug: "to-1", parent_id: "B" }),
  dm({ id: "D", name: "An ninh", slug: "an-ninh", order: 2 }),
];

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

type Calls = {
  update: ReturnType<typeof vi.fn<(id: string, body: UpdateCategoryIn) => Promise<KetQua<comms_danhMucRa>>>>;
  remove: ReturnType<typeof vi.fn<(id: string, reason: string) => Promise<KetQua<null>>>>;
  changed: ReturnType<typeof vi.fn<() => void>>;
};

function mount(
  answer: KetQua<unknown> = { ok: true, duLieu: null },
  categories: readonly comms_danhMucRa[] = TREE,
): Calls {
  const calls: Calls = {
    update: vi.fn(async () => answer as KetQua<comms_danhMucRa>),
    remove: vi.fn(async () => answer as KetQua<null>),
    changed: vi.fn(),
  };
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <CategoryAdmin
        categories={categories}
        update={calls.update}
        remove={calls.remove}
        changed={calls.changed}
      />,
    ),
  );
  return calls;
}

function button(label: string): HTMLButtonElement {
  const b = Array.from(host!.querySelectorAll("button")).find(
    (x) => x.getAttribute("aria-label") === label || x.textContent === label,
  );
  if (b === undefined) throw new Error(`no button "${label}"`);
  return b;
}

async function click(el: HTMLElement): Promise<void> {
  await act(async () => {
    el.click();
  });
}

/** Sets a controlled field the way a keyboard would, so React's onChange runs. */
function type(el: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement, value: string): void {
  const proto = Object.getPrototypeOf(el) as object;
  Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
  act(() => {
    el.dispatchEvent(new Event(el instanceof HTMLSelectElement ? "change" : "input", { bubbles: true }));
  });
}

async function submit(form: HTMLFormElement): Promise<void> {
  await act(async () => {
    form.requestSubmit();
  });
}

describe("delete — a soft delete with a mandatory reason", () => {
  it("the confirm button stays off until a reason is typed; the body is exactly {reason}", async () => {
    const calls = mount();
    await click(button("Xoá danh mục An ninh"));
    const confirm = button("Xoá danh mục");
    expect(confirm.disabled).toBe(true);

    const reason = host!.querySelector<HTMLTextAreaElement>("#xoa-danh-muc-D-ly-do")!;
    type(reason, "   ");
    expect(confirm.disabled).toBe(true);
    type(reason, "  Trùng với danh mục Tin xã  ");
    expect(confirm.disabled).toBe(false);

    await submit(confirm.form!);
    expect(calls.remove).toHaveBeenCalledTimes(1);
    expect(calls.remove).toHaveBeenCalledWith("D", "Trùng với danh mục Tin xã");
    expect(calls.update).not.toHaveBeenCalled();
    expect(calls.changed).toHaveBeenCalledTimes(1);
    // The prototype's toast, verbatim (`CategoryManagerDialog.tsx:226`).
    expect(T.success).toHaveBeenCalledWith(CATEGORY_DELETED_TOAST);
    expect(CATEGORY_DELETED_TOAST).toBe("Đã xoá danh mục.");
  });

  it("a refusal (409 category_not_empty) is shown verbatim and nothing reloads", async () => {
    const sentence =
      "Danh mục còn nội dung hoặc danh mục con nên không xoá được. " +
      "Hãy ẩn danh mục nếu không muốn bà con thấy nó trên Mini App.";
    const calls = mount({ ok: false, thongBao: sentence });
    await click(button("Xoá danh mục Tin xã"));
    type(host!.querySelector<HTMLTextAreaElement>("#xoa-danh-muc-A-ly-do")!, "Không dùng");
    await submit(button("Xoá danh mục").form!);
    expect(host!.querySelector('[role="alert"]')?.textContent).toBe(sentence);
    expect(calls.changed).not.toHaveBeenCalled();
    expect(T.success).not.toHaveBeenCalled();
  });

  it("the delete form has no `Ẩn thay vì xoá` — the prototype has none; the row's `Tắt` hides", async () => {
    mount();
    await click(button("Xoá danh mục An ninh"));
    expect(host!.textContent).not.toContain("Ẩn thay vì xoá");
    const form = host!.querySelector<HTMLTextAreaElement>("#xoa-danh-muc-D-ly-do")!.form!;
    expect(Array.from(form.querySelectorAll("button")).map((b) => b.textContent)).toEqual(["Huỷ", "Xoá danh mục"]);
  });
});

describe("Tắt / Bật", () => {
  it("one click, one-field PATCH, in both directions", async () => {
    const calls = mount({ ok: true, duLieu: null }, [
      dm({ id: "A", name: "Tin xã" }),
      dm({ id: "H", name: "Cũ", hidden: true }),
    ]);
    expect(button("Tắt danh mục Tin xã").textContent).toBe("Tắt");
    expect(button("Bật danh mục Cũ").textContent).toBe("Bật");
    await click(button("Tắt danh mục Tin xã"));
    await click(button("Bật danh mục Cũ"));
    expect(calls.update.mock.calls).toEqual([
      ["A", { hidden: true }],
      ["H", { hidden: false }],
    ]);
  });

  it("a turned-off row is dimmed as the prototype draws it — no badge repeats the state", () => {
    mount({ ok: true, duLieu: null }, [dm({ id: "A", name: "Tin xã" }), dm({ id: "H", name: "Cũ", hidden: true })]);
    const off = host!.querySelector<HTMLInputElement>("#ten-danh-muc-H")!;
    expect(off.className).toContain("line-through");
    expect(off.parentElement!.className).toContain("bg-canvas");
    const on = host!.querySelector<HTMLInputElement>("#ten-danh-muc-A")!;
    expect(on.className).not.toContain("line-through");
    expect(on.parentElement!.className).toContain("bg-surface");
    expect(host!.textContent).not.toContain("Đang hiện trên Mini App");
    expect(host!.textContent).not.toContain("Đã ẩn trên Mini App");
  });
});

describe("rename in place", () => {
  const nameBox = (id: string) => host!.querySelector<HTMLInputElement>(`#ten-danh-muc-${id}`)!;

  async function key(el: HTMLInputElement, k: string): Promise<KeyboardEvent> {
    const ev = new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true });
    await act(async () => {
      el.dispatchEvent(ev);
    });
    return ev;
  }

  it("Enter saves: a PATCH of `{name}` ONLY, trimmed — never slug, never hidden; a toast; the tree re-read", async () => {
    const calls = mount();
    const box = nameBox("B");
    act(() => box.focus());
    type(box, "  Thôn 1 ");
    await key(box, "Enter");
    expect(calls.update).toHaveBeenCalledTimes(1);
    expect(calls.update).toHaveBeenCalledWith("B", { name: "Thôn 1" });
    expect(T.success).toHaveBeenCalledWith(CATEGORY_RENAMED_TOAST);
    expect(calls.changed).toHaveBeenCalledTimes(1);
  });

  it("Esc puts the old name back, sends nothing, and does not close the dialog", async () => {
    const calls = mount();
    const box = nameBox("B");
    act(() => box.focus());
    type(box, "Sai tên");
    const ev = await key(box, "Escape");
    expect(ev.defaultPrevented).toBe(true);
    expect(box.value).toBe("Thôn Một");
    await act(async () => {
      box.blur();
    });
    expect(calls.update).not.toHaveBeenCalled();
  });

  it("unchanged or emptied: no PATCH, the name stays", async () => {
    const calls = mount();
    const box = nameBox("D");
    act(() => box.focus());
    await key(box, "Enter");
    act(() => box.focus());
    type(box, "   ");
    await key(box, "Enter");
    expect(calls.update).not.toHaveBeenCalled();
    expect(box.value).toBe("An ninh");
  });

  it("a refusal is shown verbatim under the row and the old name comes back", async () => {
    const sentence = "Tên danh mục đã có trong xã.";
    const calls = mount({ ok: false, thongBao: sentence });
    const box = nameBox("D");
    act(() => box.focus());
    type(box, "Tin xã");
    await key(box, "Enter");
    expect(calls.update).toHaveBeenCalledWith("D", { name: "Tin xã" });
    expect(host!.querySelector('[role="alert"]')?.textContent).toBe(sentence);
    expect(box.value).toBe("An ninh");
    expect(calls.changed).not.toHaveBeenCalled();
  });
});

describe("the list (prototype `CategoryManagerDialog.tsx:168-338`)", () => {
  it("grouped by parent, the no-parent group FIRST and named; no extra heading, no hide explainer, no slug line", () => {
    mount();
    const headings = Array.from(host!.querySelectorAll("p.uppercase")).map((p) => p.textContent);
    expect(headings).toEqual(["Đứng riêng, không thuộc mục cha nào", "Tin xã", "Thôn Một"]);
    expect(host!.querySelector("h3")).toBeNull();
    expect(host!.textContent).not.toContain("Danh mục tin của xã");
    expect(host!.textContent).not.toContain("bỏ nút lọc");
    expect(host!.textContent).not.toContain("Slug");
    expect(host!.querySelector(".max-h-\\[26rem\\].overflow-y-auto")).not.toBeNull();
  });

  it("each row: the name box, `? bài`, `Từ Cổng ?` — both disabled '?' — `Tắt`, the bin", () => {
    mount({ ok: true, duLieu: null }, [dm({ id: "A", name: "Tin xã" })]);
    const row = host!.querySelector("#ten-danh-muc-A")!.parentElement!;
    expect(row.querySelector(`[aria-label="${pendingMarkerLabel(CATEGORY_ITEM_COUNT_PART)}"]`)).not.toBeNull();
    expect(row.querySelector(`[aria-label="${pendingMarkerLabel(CATEGORY_FROM_PORTAL_PART)}"]`)).not.toBeNull();
    expect(row.textContent).toBe("?bàiTừ Cổng?Tắt");
    expect(row.querySelectorAll('[aria-disabled="true"][data-pending]')).toHaveLength(2);
    expect(button("Xoá danh mục Tin xã")).not.toBeNull();
    // No `Sửa` form any more: the name is edited in place.
    expect(Array.from(host!.querySelectorAll("button")).some((b) => b.textContent === "Sửa")).toBe(false);
  });

  it("empty: the prototype's sentence", () => {
    mount({ ok: true, duLieu: null }, []);
    expect(host!.textContent).toBe(CATEGORY_EMPTY);
    expect(CATEGORY_EMPTY).toBe("Chưa có danh mục nào. Thêm ở trên.");
  });
});

describe("`Lấy danh mục từ Cổng` — owner D4: in place, disabled, with its '?'", () => {
  it("the prototype's sentence and a disabled button with the DownloadCloud icon", () => {
    host = document.createElement("div");
    document.body.append(host);
    root = createRoot(host);
    act(() => root!.render(<CategoryImportPending />));
    const box = host.firstElementChild!;
    expect(box.className).toContain("bg-canvas");
    expect(box.textContent).toContain("Lấy đủ cây chuyên mục của Cổng về đây.");
    const b = Array.from(host.querySelectorAll("button")).find((x) => x.textContent === "Lấy danh mục từ Cổng")!;
    expect(b.disabled).toBe(true);
    expect(b.querySelector(".lucide-cloud-download, .lucide-download-cloud")).not.toBeNull();
    expect(host.querySelector(`[aria-label="${pendingMarkerLabel(CATEGORY_IMPORT_PART)}"]`)).not.toBeNull();
  });
});
