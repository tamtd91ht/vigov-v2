// @vitest-environment jsdom
//
// jsdom for this file: these tests PRESS the buttons — the dialogs' whole job is which body a click
// sends, and a string render cannot click. Same reasoning as `rich-text.test.ts`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type { UpdateCategoryIn } from "@/lib/api/noi-dung";
import type { comms_danhMucRa } from "@/lib/api/schema.gen";

import { CategoryAdmin } from "./category-admin";
import { KHONG_CO_GI_DOI } from "./nhan-noi-dung";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
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
  });

  it("`Ẩn thay vì xoá` sends {hidden: true} and no delete", async () => {
    const calls = mount();
    await click(button("Xoá danh mục An ninh"));
    await click(button("Ẩn thay vì xoá"));
    expect(calls.update).toHaveBeenCalledWith("D", { hidden: true });
    expect(calls.remove).not.toHaveBeenCalled();
  });
});

describe("hide / show", () => {
  it("one click, one-field PATCH, in both directions", async () => {
    const calls = mount({ ok: true, duLieu: null }, [
      dm({ id: "A", name: "Tin xã" }),
      dm({ id: "H", name: "Cũ", hidden: true }),
    ]);
    await click(button("Ẩn trên Mini App: Tin xã"));
    await click(button("Hiện lại trên Mini App: Cũ"));
    expect(calls.update.mock.calls).toEqual([
      ["A", { hidden: true }],
      ["H", { hidden: false }],
    ]);
  });
});

describe("edit", () => {
  it("sends ONLY what changed — never slug, never hidden", async () => {
    const calls = mount();
    await click(button("Sửa danh mục Thôn Một"));
    type(host!.querySelector<HTMLInputElement>("#sua-danh-muc-B-ten")!, "  Thôn 1 ");
    type(host!.querySelector<HTMLInputElement>("#sua-danh-muc-B-thu-tu")!, "5");
    await submit(host!.querySelector<HTMLFormElement>("form")!);
    expect(calls.update).toHaveBeenCalledWith("B", { name: "Thôn 1", order: 5 });
  });

  it("moving to the root sends `parent_id: \"\"`", async () => {
    const calls = mount();
    await click(button("Sửa danh mục Thôn Một"));
    type(host!.querySelector<HTMLSelectElement>("#sua-danh-muc-B-cha")!, "");
    await submit(host!.querySelector<HTMLFormElement>("form")!);
    expect(calls.update).toHaveBeenCalledWith("B", { parent_id: "" });
  });

  it("the parent select offers neither the category nor its descendants", async () => {
    mount();
    await click(button("Sửa danh mục Tin xã"));
    const values = Array.from(
      host!.querySelectorAll<HTMLOptionElement>("#sua-danh-muc-A-cha option"),
    ).map((o) => o.value);
    expect(values).toEqual(["", "D"]);
  });

  it("nothing changed → no PATCH, a sentence", async () => {
    const calls = mount();
    await click(button("Sửa danh mục An ninh"));
    await submit(host!.querySelector<HTMLFormElement>("form")!);
    expect(calls.update).not.toHaveBeenCalled();
    expect(host!.querySelector('[role="alert"]')?.textContent).toBe(KHONG_CO_GI_DOI);
  });

  it("a server refusal (409 category_cycle) is shown verbatim inside the form", async () => {
    const sentence = "Không đặt được danh mục cha là một danh mục con (hoặc cháu) của chính nó.";
    const calls = mount({ ok: false, thongBao: sentence });
    await click(button("Sửa danh mục An ninh"));
    type(host!.querySelector<HTMLSelectElement>("#sua-danh-muc-D-cha")!, "A");
    await submit(host!.querySelector<HTMLFormElement>("form")!);
    expect(calls.update).toHaveBeenCalledWith("D", { parent_id: "A" });
    expect(host!.querySelector("form [role='alert']")?.textContent).toBe(sentence);
    expect(calls.changed).not.toHaveBeenCalled();
  });

  it("the slug is shown and cannot be typed into", async () => {
    mount();
    await click(button("Sửa danh mục An ninh"));
    expect(host!.querySelector("form")?.textContent).toContain("an-ninh");
    expect(host!.querySelector("#sua-danh-muc-D-slug")).toBeNull();
  });
});
