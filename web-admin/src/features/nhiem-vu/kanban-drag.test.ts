import type { ClientRect, UniqueIdentifier } from "@dnd-kit/core";
import { describe, expect, it, vi } from "vitest";

import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";
import { QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU } from "@/lib/quyen";

import {
  columnKeyboardCoordinates,
  dragTargets,
  draggedTask,
  dropOnKanban,
  dropTarget,
  kanbanAnnouncements,
  type KanbanDragGate,
} from "./kanban-drag";
import { BANG_NHAN_MAC_DINH, quyenNhiemVu } from "./nhan-nhiem-vu";
import { serverTransitions } from "./task-transitions.fixture";

/**
 * The drag's decisions (@dnd-kit, 06/10/2026), without a browser: which columns take a card, and
 * that a drop calls `move` exactly once for an allowed column and never otherwise.
 */

const ASSIGNEE = "CB-2026-3H8N2W";
const OTHER = "CB-2026-0P4X1Z";

function at(status: string, patch: Partial<petitions_nhiemVuRa> = {}): petitions_nhiemVuRa {
  return {
    code: "NV19",
    child_count: 0,
    extension_count: 0,
    pending_extension: false,
    allowed_transitions: serverTransitions(status),
    updated_at: "2026-06-01T02:00:00Z",
    type: "co-ban",
    bloc: "",
    priority: "",
    title: "Rà soát hồ sơ tồn đọng",
    description: "",
    status,
    source: "truc-tiep",
    source_id: "",
    unit: "",
    assignee: ASSIGNEE,
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
    ...patch,
  };
}

const UPDATE_ONLY = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU]);
const UPDATE_AND_APPROVE = quyenNhiemVu([QUYEN_CAP_NHAT_NHIEM_VU, QUYEN_DUYET_HOAN_THANH_NHIEM_VU]);
const NO_KEYS = quyenNhiemVu([]);

function gate(patch: Partial<KanbanDragGate> = {}): KanbanDragGate {
  return { permissions: UPDATE_ONLY, staffCode: OTHER, pending: null, move: vi.fn(), ...patch };
}

describe("dragTargets — the drawer's list, nothing more", () => {
  it("`task.update`: the server's moves for the status", () => {
    expect(dragTargets(at("dang-thuc-hien"), gate())).toEqual(["cho-duyet", "hoan-thanh", "tam-dung"]);
  });

  it("the assignee without `task.update` may drag their own card", () => {
    expect(dragTargets(at("dang-thuc-hien"), gate({ permissions: NO_KEYS, staffCode: ASSIGNEE }))).toEqual([
      "cho-duyet",
      "hoan-thanh",
      "tam-dung",
    ]);
  });

  it("DENIED — no key, not the assignee: no column at all", () => {
    expect(dragTargets(at("dang-thuc-hien"), gate({ permissions: NO_KEYS }))).toEqual([]);
  });

  it("read-only board, or a move waiting for the server: no column", () => {
    expect(dragTargets(at("dang-thuc-hien"), null)).toEqual([]);
    expect(dragTargets(at("dang-thuc-hien"), gate({ pending: { code: "NV20", target: "cho-duyet" } }))).toEqual(
      [],
    );
  });

  it("the return step (`cho-duyet` → `dang-thuc-hien`) is never a drop target — it needs the drawer's reason", () => {
    const targets = dragTargets(at("cho-duyet"), gate({ permissions: UPDATE_AND_APPROVE }));
    expect(targets).toEqual(["hoan-thanh"]);
    expect(targets).not.toContain("dang-thuc-hien");
  });
});

describe("dropOnKanban — one call for an allowed column, none otherwise", () => {
  it("allowed column: `move(task, target, \"drag\")` exactly once", () => {
    const g = gate();
    const task = at("dang-thuc-hien");
    expect(dropOnKanban(task, "cho-duyet", g)).toBe(true);
    expect(g.move).toHaveBeenCalledTimes(1);
    expect(g.move).toHaveBeenCalledWith(task, "cho-duyet", "drag");
  });

  it("a column the lifecycle does not allow: nothing is sent", () => {
    const g = gate();
    expect(dropOnKanban(at("dang-thuc-hien"), "moi-giao", g)).toBe(false);
    // The card's own column is no move either.
    expect(dropOnKanban(at("dang-thuc-hien"), "dang-thuc-hien", g)).toBe(false);
    expect(g.move).not.toHaveBeenCalled();
  });

  it("dropped outside every column: nothing is sent", () => {
    const g = gate();
    expect(dropOnKanban(at("dang-thuc-hien"), null, g)).toBe(false);
    expect(dropOnKanban(at("dang-thuc-hien"), undefined, g)).toBe(false);
    expect(g.move).not.toHaveBeenCalled();
  });

  it("the return step dropped on `dang-thuc-hien`: nothing is sent — the reason lives in the drawer", () => {
    const g = gate({ permissions: UPDATE_AND_APPROVE });
    expect(dropOnKanban(at("cho-duyet"), "dang-thuc-hien", g)).toBe(false);
    expect(g.move).not.toHaveBeenCalled();
  });

  it("denied account, pending move, read-only board, no task: nothing is sent", () => {
    const denied = gate({ permissions: NO_KEYS });
    const pending = gate({ pending: { code: "NV20", target: "cho-duyet" } });
    expect(dropOnKanban(at("dang-thuc-hien"), "cho-duyet", denied)).toBe(false);
    expect(dropOnKanban(at("dang-thuc-hien"), "cho-duyet", pending)).toBe(false);
    expect(dropOnKanban(at("dang-thuc-hien"), "cho-duyet", null)).toBe(false);
    expect(dropOnKanban(null, "cho-duyet", gate())).toBe(false);
    expect(denied.move).not.toHaveBeenCalled();
    expect(pending.move).not.toHaveBeenCalled();
  });

  it("an unknown id over the board (not a status) is no column", () => {
    expect(dropTarget(at("dang-thuc-hien"), "overlay:NV19", gate())).toBeNull();
  });
});

describe("draggedTask", () => {
  it("reads the task the card attached, and nothing else", () => {
    const task = at("moi-giao");
    expect(draggedTask({ task })).toBe(task);
    expect(draggedTask(undefined)).toBeNull();
    expect(draggedTask({})).toBeNull();
    expect(draggedTask({ task: "NV19" })).toBeNull();
  });
});

describe("announcements — Vietnamese, and say whether the column takes the card", () => {
  const a = kanbanAnnouncements(BANG_NHAN_MAC_DINH, gate());
  const active = (task: petitions_nhiemVuRa) =>
    ({ id: task.code, data: { current: { task } }, rect: { current: { initial: null, translated: null } } }) as never;
  const over = (id: UniqueIdentifier) => ({ id, rect: {} as ClientRect, disabled: false, data: { current: {} } }) as never;
  const task = at("dang-thuc-hien");

  it("start, over an allowed column, over a refused one, outside, cancelled", () => {
    expect(a.onDragStart({ active: active(task) })).toBe("Đã nhấc thẻ NV19.");
    expect(a.onDragOver({ active: active(task), over: over("cho-duyet") })).toContain("Thả để chuyển");
    expect(a.onDragOver({ active: active(task), over: over("moi-giao") })).toContain("không chuyển được");
    expect(a.onDragEnd({ active: active(task), over: null })).toContain("ở nguyên cột cũ");
    expect(a.onDragEnd({ active: active(task), over: over("cho-duyet") })).toContain("máy chủ xác nhận");
    expect(a.onDragEnd({ active: active(task), over: over("moi-giao") })).toContain("không nhận");
    expect(a.onDragCancel({ active: active(task), over: null })).toContain("Đã huỷ kéo");
  });
});

describe("columnKeyboardCoordinates — arrows jump a whole column", () => {
  const rect = (left: number, top: number, width = 200, height = 400): ClientRect => ({
    left,
    top,
    width,
    height,
    right: left + width,
    bottom: top + height,
  });
  const columns = new Map<UniqueIdentifier, ClientRect>([
    ["moi-giao", rect(0, 0)],
    ["da-tiep-nhan", rect(220, 0)],
    ["dang-thuc-hien", rect(440, 0)],
  ]);
  // The dragged card sits centred in the middle column.
  const card = rect(450, 100, 180, 80);
  const key = (code: string) => ({ code, preventDefault: () => {} }) as KeyboardEvent;
  const args = {
    active: "NV19",
    currentCoordinates: { x: 540, y: 140 },
    context: { collisionRect: card, droppableRects: columns } as never,
  };

  it("ArrowLeft lands on the centre of the column to the left", () => {
    // Card centre (540,140) → column centre (320,200): shift by (-220,+60).
    expect(columnKeyboardCoordinates(key("ArrowLeft"), args)).toEqual({ x: 320, y: 200 });
  });

  it("stacked board: ArrowDown skips the card's own column, whose centre lies below the card", () => {
    const stacked = new Map<UniqueIdentifier, ClientRect>([
      ["moi-giao", rect(0, 0, 300, 400)],
      ["da-tiep-nhan", rect(0, 420, 300, 400)],
    ]);
    const top = {
      active: "NV19",
      currentCoordinates: { x: 150, y: 60 },
      context: { collisionRect: rect(10, 20, 280, 80), droppableRects: stacked } as never,
    };
    // Card centre (150,60) → second column centre (150,620).
    expect(columnKeyboardCoordinates(key("ArrowDown"), top)).toEqual({ x: 150, y: 620 });
  });

  it("no column that way, or not an arrow: no move", () => {
    expect(columnKeyboardCoordinates(key("ArrowRight"), args)).toBeUndefined();
    expect(columnKeyboardCoordinates(key("KeyA"), args)).toBeUndefined();
  });
});
