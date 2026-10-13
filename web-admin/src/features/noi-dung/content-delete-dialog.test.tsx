// @vitest-environment jsdom
//
// jsdom for this file: these tests PRESS the buttons — which call a click sends, and what the dialog
// does with each answer, is the whole job. Same reasoning as `category-admin.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { deleteContentItem } from "@/lib/api/noi-dung";
import type { CallResult } from "@/lib/api/task-attachments";
import type { comms_noiDungRa } from "@/lib/api/schema.gen";

import { ContentDeleteDialog } from "./content-delete-dialog";
import { SMALL_DIALOG_CLASS } from "./overlay-dialog";
import { CONTENT_DELETE_NOTE, CONTENT_DELETE_SUBMIT, NHAN_NUT_HUY } from "./nhan-noi-dung";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const ITEM = {
  id: "01JND1",
  type: "tin-tuc",
  category_id: "",
  title: "Xã tổ chức hội nghị tổng kết công tác chuyển đổi số",
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
} as comms_noiDungRa;

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.unstubAllGlobals();
});

type Spies = {
  remove: ReturnType<typeof vi.fn<(id: string) => Promise<CallResult<null>>>>;
  deleted: ReturnType<typeof vi.fn<() => void>>;
  gone: ReturnType<typeof vi.fn<() => void>>;
  close: ReturnType<typeof vi.fn<() => void>>;
};

function mount(remove: Spies["remove"]): Spies {
  const spies: Spies = { remove, deleted: vi.fn(), gone: vi.fn(), close: vi.fn() };
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <ContentDeleteDialog
        item={ITEM}
        remove={spies.remove}
        deleted={spies.deleted}
        gone={spies.gone}
        close={spies.close}
      />,
    ),
  );
  return spies;
}

function answering(r: CallResult<null>): Spies["remove"] {
  return vi.fn(async () => r);
}

function button(label: string): HTMLButtonElement {
  const b = Array.from(host!.querySelectorAll("button")).find((x) => x.textContent === label);
  if (b === undefined) throw new Error(`no button "${label}"`);
  return b;
}

async function submit(): Promise<void> {
  await act(async () => {
    button(CONTENT_DELETE_SUBMIT).form!.requestSubmit();
  });
}

function pressEsc(): void {
  act(() => {
    host!.querySelector("dialog")!.dispatchEvent(new Event("cancel", { cancelable: true }));
  });
}

describe("the dialog says what it deletes and what it keeps — and asks nothing else", () => {
  it("the item's title and the soft-delete sentence; NO reason box, no counter (sheet web-admin-131026)", () => {
    mount(answering({ ok: true, data: null }));
    const text = host!.textContent ?? "";
    expect(text).toContain(ITEM.title);
    expect(text).toContain(CONTENT_DELETE_NOTE);
    expect(CONTENT_DELETE_NOTE).toContain("vẫn được lưu");
    expect(host!.querySelector("textarea")).toBeNull();
    expect(host!.querySelector("#content-delete-reason")).toBeNull();
    expect(text).not.toContain("/500");
    expect(text).not.toContain("Lý do");
  });

  it("`Xoá` is on from the start: nothing to fill in", () => {
    mount(answering({ ok: true, data: null }));
    expect(button(CONTENT_DELETE_SUBMIT).disabled).toBe(false);
  });

  it("the box is sized to its content, not the 48rem default that left a white strip on the right", () => {
    mount(answering({ ok: true, data: null }));
    const dialog = host!.querySelector("dialog")!;
    expect(dialog.className).toContain(SMALL_DIALOG_CLASS);
  });
});

describe("the three outcomes", () => {
  it("204 → `deleted`, called for this item and nothing more; nothing else fires", async () => {
    const spies = mount(answering({ ok: true, data: null }));
    await submit();
    expect(spies.remove).toHaveBeenCalledTimes(1);
    expect(spies.remove.mock.calls[0]).toEqual(["01JND1"]);
    expect(spies.deleted).toHaveBeenCalledTimes(1);
    expect(spies.gone).not.toHaveBeenCalled();
    expect(spies.close).not.toHaveBeenCalled();
  });

  it("404 → `gone` (already deleted elsewhere): the parent closes and reloads", async () => {
    const spies = mount(answering({ ok: false, status: 404, message: "Không tìm thấy nội dung này." }));
    await submit();
    expect(spies.gone).toHaveBeenCalledTimes(1);
    expect(spies.deleted).not.toHaveBeenCalled();
  });

  for (const r of [
    { status: 403, message: "Bạn không có quyền thực hiện thao tác này." },
    { status: 0, message: "Không kết nối được máy chủ. Vui lòng thử lại." },
  ]) {
    it(`${r.status} → the server's sentence, dialog stays open, \`Xoá\` can be pressed again`, async () => {
      const spies = mount(answering({ ok: false, ...r }));
      await submit();
      expect(host!.querySelector('[role="alert"]')?.textContent).toBe(r.message);
      expect(button(CONTENT_DELETE_SUBMIT).disabled).toBe(false);
      expect(spies.deleted).not.toHaveBeenCalled();
      expect(spies.gone).not.toHaveBeenCalled();
      expect(spies.close).not.toHaveBeenCalled();
    });
  }
});

describe("while the call is in flight", () => {
  it("Esc and `Huỷ` are refused, both buttons are off; after the answer Esc closes again", async () => {
    let answer: (r: CallResult<null>) => void = () => {};
    const spies = mount(
      vi.fn(() => new Promise<CallResult<null>>((resolve) => {
        answer = resolve;
      })),
    );
    await submit();

    expect(button(NHAN_NUT_HUY).disabled).toBe(true);
    expect(button(CONTENT_DELETE_SUBMIT).disabled).toBe(true);
    pressEsc();
    expect(spies.close).not.toHaveBeenCalled();

    await act(async () => {
      answer({ ok: false, status: 500, message: "Đã xảy ra lỗi. Vui lòng thử lại." });
    });
    pressEsc();
    expect(spies.close).toHaveBeenCalledTimes(1);
  });

  it("`Huỷ` closes when nothing is in flight, without calling the server", async () => {
    const spies = mount(answering({ ok: true, data: null }));
    await act(async () => {
      button(NHAN_NUT_HUY).click();
    });
    expect(spies.close).toHaveBeenCalledTimes(1);
    expect(spies.remove).not.toHaveBeenCalled();
  });
});

describe("wired to the real client: one DELETE, no body", () => {
  it("one DELETE to the item's path, nothing in the query, no body (the server records its own reason)", async () => {
    const fetchSpy = vi.fn(async (_path: string, _init?: RequestInit) => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchSpy);
    const spies = mount(vi.fn((id: string) => deleteContentItem(id)));
    await submit();

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [path, init] = fetchSpy.mock.calls[0] as [string, RequestInit];
    expect(path).toBe("/api/v1/content-items/01JND1");
    expect(path).not.toContain("?");
    expect(init.method).toBe("DELETE");
    expect(init.body).toBeUndefined();
    expect(spies.deleted).toHaveBeenCalledTimes(1);
  });
});
