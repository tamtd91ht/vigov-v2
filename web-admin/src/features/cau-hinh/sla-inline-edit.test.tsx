// @vitest-environment jsdom
//
// jsdom: the in-place edit of tab "Thời hạn xử lý" (spec 08) is keys and clicks — Enter saves, Esc
// cancels, the pencil opens THAT row — which a markup string cannot show.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { identity_danhSachSLARa, identity_dongSLARa } from "@/lib/api/schema.gen";

import { banTuDong } from "./sua-thoi-han";
// vi-name-ok: imports the existing exports of tab-thoi-han-xu-ly.tsx unchanged (rule 12 invariant 3)
import { ManThoiHanXuLy, type InlineEdit, type ThaoTacThoiHan } from "./tab-thoi-han-xu-ly";

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

function actions(startEdit: (d: identity_dongSLARa) => void = () => {}): ThaoTacThoiHan {
  return { gieoThoiHan: () => {}, suaThoiHan: startEdit };
}

function mount(rowActions: ThaoTacThoiHan, edit: InlineEdit | null): void {
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
      />,
    ),
  );
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
