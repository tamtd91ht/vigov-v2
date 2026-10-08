// @vitest-environment jsdom
//
// jsdom: the in-place edit of tab "Thời hạn xử lý" (spec 08) is keys and clicks — Enter saves, Esc
// cancels, the pencil opens THAT row — which a markup string cannot show.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { identity_danhSachSLARa, identity_dongSLARa } from "@/lib/api/schema.gen";

import { banTuDong, newAddDraft } from "./sua-thoi-han";
// vi-name-ok: imports the existing exports of tab-thoi-han-xu-ly.tsx unchanged (rule 12 invariant 3)
import {
  ManThoiHanXuLy,
  type AddForm,
  type InlineEdit,
  type InlineRemove,
  type ThaoTacThoiHan,
} from "./tab-thoi-han-xu-ly";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const ROW: identity_dongSLARa = {
  id: "01J0000000000000000000SLA",
  work_kind: "phan-anh",
  field: "an-ninh-trat-tu",
  is_default: false,
  acknowledge_hours: 2,
  resolve_hours: 16,
  due_soon_hours: 4,
  escalate_leader_hours: 8,
  escalate_president_hours: 16,
  unassigned_hold_hours: 8,
};

const TABLE = { ok: true as const, duLieu: { items: [ROW], problems: [] } satisfies identity_danhSachSLARa };

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

function actions(
  startEdit: (d: identity_dongSLARa) => void = () => {},
  more: Partial<ThaoTacThoiHan> = {},
): ThaoTacThoiHan {
  return { gieoThoiHan: () => {}, suaThoiHan: startEdit, toggleAdd: () => {}, startRemove: () => {}, ...more };
}

function mount(
  rowActions: ThaoTacThoiHan,
  edit: InlineEdit | null,
  extra: { add?: AddForm | null; remove?: InlineRemove | null } = {},
): void {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <ManThoiHanXuLy
        du={{ thoiHan: TABLE }}
        coQuyenGhi
        thaoTac={rowActions}
        loiMayChuNgoaiForm=""
        dangGui={false}
        edit={edit}
        add={extra.add ?? null}
        remove={extra.remove ?? null}
      />,
    ),
  );
}

function addOf(more: Partial<AddForm>): AddForm {
  return {
    draft: newAddDraft(),
    localError: "",
    serverError: "",
    busy: false,
    setDraft: () => {},
    onSubmit: () => {},
    onCancel: () => {},
    ...more,
  };
}

function removeOf(more: Partial<InlineRemove>): InlineRemove {
  return {
    rowId: ROW.id,
    reason: "",
    localError: "",
    serverError: "",
    busy: false,
    setReason: () => {},
    onConfirm: () => {},
    onCancel: () => {},
    ...more,
  };
}

/** Sets a native control's value the way React's change tracking sees it, then fires the event. */
function setValue(el: HTMLInputElement | HTMLSelectElement, value: string, event: "input" | "change"): void {
  const proto = el instanceof HTMLSelectElement ? HTMLSelectElement.prototype : HTMLInputElement.prototype;
  act(() => {
    Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event(event, { bubbles: true }));
  });
}

function editOf(more: Partial<InlineEdit>): InlineEdit {
  return {
    rowId: ROW.id,
    draft: banTuDong(ROW),
    localError: "",
    serverError: "",
    busy: false,
    setDraft: () => {},
    onSave: () => {},
    onCancel: () => {},
    ...more,
  };
}

function box(name: string): HTMLInputElement {
  const el = host!.querySelector<HTMLInputElement>(`input[name="${name}"]`);
  if (el === null) throw new Error(`no input ${name}`);
  return el;
}

function press(el: HTMLElement, key: string): void {
  act(() => {
    el.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  });
}

describe("sửa tại chỗ — phím và nút", () => {
  it("bấm bút mở ĐÚNG dòng ấy", () => {
    const startEdit = vi.fn();
    mount(actions(startEdit), null);

    act(() => host!.querySelector<HTMLButtonElement>('button[title="Sửa thời hạn"]')!.click());
    expect(startEdit).toHaveBeenCalledWith(ROW);
  });

  it("Enter trong một ô số → lưu; Esc → huỷ", () => {
    const onSave = vi.fn();
    const onCancel = vi.fn();
    mount(actions(), editOf({ onSave, onCancel }));

    press(box("resolve_hours"), "Enter");
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();

    press(box("due_soon_hours"), "Escape");
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("nút Lưu / Huỷ gọi đúng việc", () => {
    const onSave = vi.fn();
    const onCancel = vi.fn();
    mount(actions(), editOf({ onSave, onCancel }));

    const buttons = [...host!.querySelectorAll("button")];
    act(() => buttons.find((b) => b.textContent === "Lưu")!.click());
    act(() => buttons.find((b) => b.textContent === "Huỷ")!.click());
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("gõ vào một ô chỉ đổi ĐÚNG khoá của ô ấy trong bản nháp", () => {
    const setDraft = vi.fn();
    mount(actions(), editOf({ setDraft }));

    const el = box("resolve_hours");
    act(() => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(el, "24");
      el.dispatchEvent(new Event("input", { bubbles: true }));
    });
    expect(setDraft).toHaveBeenCalledWith({ ...banTuDong(ROW), resolve_hours: "24" });
  });
});

describe("thêm thời hạn cho một lĩnh vực — nút và hàng thêm", () => {
  it("nút '+ Thêm thời hạn cho một lĩnh vực' gọi toggleAdd", () => {
    const toggleAdd = vi.fn();
    mount(actions(undefined, { toggleAdd }), null);

    const button = [...host!.querySelectorAll("button")].find((b) =>
      b.textContent?.includes("Thêm thời hạn cho một lĩnh vực"),
    )!;
    act(() => button.click());
    expect(toggleAdd).toHaveBeenCalledTimes(1);
  });

  it("đổi Loại việc → bản nháp mang loại mới và Lĩnh vực về rỗng", () => {
    const setDraft = vi.fn();
    mount(actions(), null, { add: addOf({ setDraft, draft: { ...newAddDraft(), field: "an-ninh-trat-tu" } }) });

    setValue(host!.querySelector<HTMLSelectElement>("#sla-add-kind")!, "van-ban-den", "change");
    expect(setDraft).toHaveBeenCalledWith({ ...newAddDraft(), workKind: "van-ban-den", field: "" });
  });

  it("gõ Xử lý xong chỉ đổi đúng ô ấy", () => {
    const setDraft = vi.fn();
    mount(actions(), null, { add: addOf({ setDraft }) });

    setValue(host!.querySelector<HTMLInputElement>("#sla-add-resolve")!, "24", "input");
    expect(setDraft).toHaveBeenCalledWith({ ...newAddDraft(), resolveHours: "24" });
  });

  it("bấm Thêm gửi biểu mẫu (onSubmit), Huỷ đóng (onCancel), Esc trong hàng cũng đóng", () => {
    const onSubmit = vi.fn();
    const onCancel = vi.fn();
    mount(actions(), null, { add: addOf({ onSubmit, onCancel }) });

    const form = host!.querySelector("form")!;
    const buttons = [...form.querySelectorAll("button")];
    act(() => buttons.find((b) => b.textContent === "Thêm")!.click());
    expect(onSubmit).toHaveBeenCalledTimes(1);

    act(() => buttons.find((b) => b.textContent === "Huỷ")!.click());
    expect(onCancel).toHaveBeenCalledTimes(1);

    press(host!.querySelector<HTMLInputElement>("#sla-add-acknowledge")!, "Escape");
    expect(onCancel).toHaveBeenCalledTimes(2);
  });
});

describe("xoá thời hạn riêng — thùng rác và bước lý do", () => {
  it("bấm thùng rác mở bước lý do của ĐÚNG dòng ấy", () => {
    const startRemove = vi.fn();
    mount(actions(undefined, { startRemove }), null);

    act(() => host!.querySelector<HTMLButtonElement>('button[title="Xoá thời hạn riêng"]')!.click());
    expect(startRemove).toHaveBeenCalledWith(ROW);
  });

  it("gõ lý do → setReason; Enter → xác nhận; Esc → huỷ; nút Xoá / Huỷ gọi đúng việc", () => {
    const setReason = vi.fn();
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    mount(actions(), null, { remove: removeOf({ setReason, onConfirm, onCancel }) });

    const reason = box("reason");
    setValue(reason, "Trùng với dòng mặc định", "input");
    expect(setReason).toHaveBeenCalledWith("Trùng với dòng mặc định");

    press(reason, "Enter");
    expect(onConfirm).toHaveBeenCalledTimes(1);
    press(reason, "Escape");
    expect(onCancel).toHaveBeenCalledTimes(1);

    const buttons = [...host!.querySelectorAll("button")];
    act(() => buttons.find((b) => b.textContent === "Xoá")!.click());
    act(() => buttons.find((b) => b.textContent === "Huỷ")!.click());
    expect(onConfirm).toHaveBeenCalledTimes(2);
    expect(onCancel).toHaveBeenCalledTimes(2);
  });
});
