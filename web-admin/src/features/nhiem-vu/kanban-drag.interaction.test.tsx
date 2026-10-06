// @vitest-environment jsdom
//
// jsdom: the @dnd-kit card listens to mouse and keyboard events, and the questions here — does a
// plain click still open the task, does a keyboard pick-up highlight only the allowed columns,
// does Escape send nothing — are about those events, which a string render cannot show.
// jsdom has no layout (every rect is 0×0), so no case here drops on a column: the drop decision is
// pure and tested in `kanban-drag.test.ts`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import type { page_Result_petitions_nhiemVuRa, petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import { QUYEN_CAP_NHAT_NHIEM_VU } from "@/lib/quyen";

import { KANBAN_DROP_REFUSED_HINT } from "./kanban-drag";
import { BANG_NHAN_MAC_DINH, TRANG_THAI_CHINH, kanbanDropHint, quyenNhiemVu } from "./nhan-nhiem-vu";
import { BangKanban, type CotKanban, type KanbanMove } from "./so-nhiem-vu";
import { serverTransitions } from "./task-transitions.fixture";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const OTHER = "CB-2026-0P4X1Z";

const TASK: petitions_nhiemVuRa = {
  code: "NV19",
  child_count: 0,
  allowed_transitions: serverTransitions("dang-thuc-hien"),
  updated_at: "2026-06-01T02:00:00Z",
  type: "co-ban",
  bloc: "",
  priority: "",
  title: "Rà soát hồ sơ tồn đọng",
  description: "",
  status: "dang-thuc-hien",
  source: "truc-tiep",
  source_id: "",
  unit: "",
  assignee: "CB-2026-3H8N2W",
  assigner: "",
  due_at: null,
  original_due_at: null,
  completed_at: null,
  progress: 0,
  result_summary: "",
  note: "",
  leader_approved: false,
  superior_acknowledged: false,
  parent: "",
  created_by: "CB-2026-VANTHU",
  created_at: "2026-06-01T02:00:00Z",
};

function page(items: readonly petitions_nhiemVuRa[]): page_Result_petitions_nhiemVuRa {
  return { items: [...items], next_cursor: "", has_more: false };
}

let root: Root | null = null;
let host: HTMLDivElement | null = null;

function mount(move: KanbanMove, open: (n: petitions_nhiemVuRa) => void): void {
  const cot: readonly CotKanban[] = TRANG_THAI_CHINH.map((ma) => ({
    ma,
    tai: { pha: "xong" as const, duLieu: page(ma === TASK.status ? [TASK] : []) },
  }));
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() =>
    root!.render(
      <BangKanban
        cot={cot}
        danhMuc={{ loai: [], mucUuTien: [], khoi: [], boPhan: [] }}
        nhanTT={BANG_NHAN_MAC_DINH}
        bayGio={new Date("2026-09-15T03:00:00Z")}
        maDangMo={null}
        moNhiemVu={open}
        counts={{ pha: "dangTai" }}
        move={move}
      />,
    ),
  );
}

function moveWith(permissions: readonly string[]): KanbanMove {
  return { permissions: quyenNhiemVu(permissions), staffCode: OTHER, pending: null, result: null, move: vi.fn() };
}

function card(): HTMLElement {
  const el = host!.querySelector<HTMLElement>("article.the-nhiem-vu");
  expect(el).not.toBeNull();
  return el!;
}

function openButton(): HTMLButtonElement {
  // The card body; the move-menu button also carries `aria-expanded`, with `aria-haspopup`.
  const el = card().querySelector<HTMLButtonElement>("button[aria-expanded]:not([aria-haspopup])");
  expect(el).not.toBeNull();
  return el!;
}

function column(status: string): HTMLElement {
  return host!.querySelector<HTMLElement>(`section[aria-labelledby="cot-kanban-${status}"]`)!;
}

/** dnd-kit's keyboard sensor starts listening for keys on the next tick. */
async function tick(): Promise<void> {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
}

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
  vi.restoreAllMocks();
});

describe("a draggable card still opens on a plain click", () => {
  it("ALLOWED account: mousedown + click without moving opens the task and moves nothing", () => {
    const open = vi.fn();
    const move = moveWith([QUYEN_CAP_NHAT_NHIEM_VU]);
    mount(move, open);
    expect(card().getAttribute("aria-roledescription")).not.toBeNull();
    act(() => {
      openButton().dispatchEvent(new MouseEvent("mousedown", { bubbles: true, button: 0, clientX: 10, clientY: 10 }));
      openButton().dispatchEvent(new MouseEvent("mouseup", { bubbles: true, button: 0, clientX: 10, clientY: 10 }));
      openButton().click();
    });
    expect(open).toHaveBeenCalledTimes(1);
    expect(open).toHaveBeenCalledWith(TASK);
    expect(move.move).not.toHaveBeenCalled();
  });

  it("DENIED account: no drag handle, and the click still opens the task", () => {
    const open = vi.fn();
    mount(moveWith([]), open);
    expect(card().getAttribute("aria-roledescription")).toBeNull();
    expect(card().hasAttribute("tabindex")).toBe(false);
    act(() => openButton().click());
    expect(open).toHaveBeenCalledTimes(1);
  });
});

describe("keyboard drag (a11y)", () => {
  it("Space on the card highlights ONLY the allowed columns, in words; Escape sends nothing", async () => {
    const move = moveWith([QUYEN_CAP_NHAT_NHIEM_VU]);
    mount(move, vi.fn());
    card().focus();
    act(() => {
      card().dispatchEvent(new KeyboardEvent("keydown", { bubbles: true, code: "Space", key: " " }));
    });
    await tick();

    // `dang-thuc-hien` → cho-duyet, hoan-thanh are columns; moi-giao, da-tiep-nhan are not moves.
    for (const allowed of ["cho-duyet", "hoan-thanh"]) {
      expect(column(allowed).className).toContain("cot-nhan-tha");
      expect(column(allowed).textContent).toContain(kanbanDropHint(BANG_NHAN_MAC_DINH, allowed));
    }
    for (const refused of ["moi-giao", "da-tiep-nhan"]) {
      expect(column(refused).className).toContain("cot-khong-nhan");
      expect(column(refused).textContent).toContain(KANBAN_DROP_REFUSED_HINT);
    }
    // The real card stays in its column, dimmed, while the copy is dragged.
    expect(column("dang-thuc-hien").querySelector("article.the-dang-keo")).not.toBeNull();

    act(() => {
      document.dispatchEvent(new KeyboardEvent("keydown", { bubbles: true, code: "Escape", key: "Escape" }));
    });
    await tick();
    expect(host!.querySelector(".cot-nhan-tha")).toBeNull();
    expect(host!.querySelector(".cot-khong-nhan")).toBeNull();
    expect(move.move).not.toHaveBeenCalled();
  });

  it("Space on the card's OWN controls does not pick the card up", async () => {
    mount(moveWith([QUYEN_CAP_NHAT_NHIEM_VU]), vi.fn());
    act(() => {
      openButton().dispatchEvent(new KeyboardEvent("keydown", { bubbles: true, code: "Space", key: " " }));
    });
    await tick();
    expect(host!.querySelector(".cot-nhan-tha")).toBeNull();
  });
});
