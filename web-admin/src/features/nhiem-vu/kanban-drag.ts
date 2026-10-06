import type {
  Announcements,
  ClientRect,
  CollisionDetection,
  KeyboardCoordinateGetter,
  ScreenReaderInstructions,
  UniqueIdentifier,
} from "@dnd-kit/core";
import { pointerWithin, rectIntersection } from "@dnd-kit/core";

import type { petitions_nhiemVuRa } from "@/lib/api/schema.gen";

/** A point on screen, in px (dnd-kit does not export its own name for it). */
type Coordinates = { readonly x: number; readonly y: number };

import {
  clickableTransitions,
  nhanTrangThai,
  type BangNhanTrangThai, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
  type QuyenNhiemVu, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
  type TrangThaiNhiemVu, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
} from "./nhan-nhiem-vu";

/**
 * Dragging a Kanban card (@dnd-kit, 06/10/2026) — the decisions, kept out of the component so they
 * are tested without a browser.
 *
 * WHY NOT NATIVE HTML5 DRAG-AND-DROP ANY MORE: the card's body is a `<button>` (it opens the task),
 * and browsers often refuse to start a native drag from a button; native drag also never fires on a
 * touch screen. Staff reported "kéo thả chưa được". @dnd-kit drives the drag from mouse, touch and
 * keyboard events, which every one of those surfaces delivers — the prototype's library.
 *
 * THE GATE IS UNCHANGED: a card may go only where `clickableTransitions` says (the drawer's list,
 * reason moves excluded), and the drop calls the SAME `move.move(task, target, "drag")` the menu
 * calls. Hiding is convenience — the route checks again (rule 5).
 */

/** What the drag needs from `KanbanMove` — structural, so this module never imports the screen. */
export type KanbanDragGate = {
  readonly permissions: QuyenNhiemVu;
  readonly staffCode: string;
  readonly pending: { readonly code: string; readonly target: TrangThaiNhiemVu } | null;
  readonly move: (task: petitions_nhiemVuRa, target: TrangThaiNhiemVu, via: "drag") => void;
};

/**
 * The columns this card may be dropped on. Empty — read-only board, a move waiting for the server,
 * or nothing this account may do — means the card offers no drag at all: a gesture that can only
 * be refused is not offered.
 */
export function dragTargets(
  task: petitions_nhiemVuRa,
  gate: KanbanDragGate | null,
): TrangThaiNhiemVu[] {
  if (gate === null || gate.pending !== null) return [];
  return clickableTransitions(task, gate.permissions, gate.staffCode);
}

/** The column a drop over `overId` moves the card to, or `null` (no column, or one it may not take). */
export function dropTarget(
  task: petitions_nhiemVuRa,
  overId: UniqueIdentifier | null | undefined,
  gate: KanbanDragGate | null,
): TrangThaiNhiemVu | null {
  if (overId === null || overId === undefined) return null;
  return dragTargets(task, gate).find((t) => t === String(overId)) ?? null;
}

/**
 * The drop. Calls `move` ONCE when the column is allowed, never otherwise — a disallowed column or
 * no column at all leaves the card where it is, with no request sent. Returns whether it called.
 */
export function dropOnKanban(
  task: petitions_nhiemVuRa | null,
  overId: UniqueIdentifier | null | undefined,
  gate: KanbanDragGate | null,
): boolean {
  if (task === null || gate === null) return false;
  const target = dropTarget(task, overId, gate);
  if (target === null) return false;
  gate.move(task, target, "drag");
  return true;
}

/** Reads the task the board attached to a draggable (`useDraggable({ data: { task } })`). */
export function draggedTask(data: Record<string, unknown> | undefined): petitions_nhiemVuRa | null {
  const task = data?.task;
  return typeof task === "object" && task !== null ? (task as petitions_nhiemVuRa) : null;
}

/**
 * Pointer drag: the column under the pointer, and only that one — a pointer in the gap between two
 * columns drops nowhere. A rectangle-overlap fallback would hand the drop to a NEIGHBOURING allowed
 * column while the pointer sits on a refused one. Keyboard drag has no pointer: the moved card's
 * rectangle decides (`columnKeyboardCoordinates` centres it on a column first).
 */
export const kanbanCollision: CollisionDetection = (args) =>
  args.pointerCoordinates !== null ? pointerWithin(args) : rectIntersection(args);

function centre(r: ClientRect): Coordinates {
  return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
}

/** Arrow key → direction on screen; anything else is not a move. */
function arrowDirection(code: string): Coordinates | null {
  switch (code) {
    case "ArrowRight":
      return { x: 1, y: 0 };
    case "ArrowLeft":
      return { x: -1, y: 0 };
    case "ArrowDown":
      return { x: 0, y: 1 };
    case "ArrowUp":
      return { x: 0, y: -1 };
    default:
      return null;
  }
}

/**
 * Arrow keys jump the dragged card to the nearest column in that direction (side by side from
 * 768px, stacked below), instead of dnd-kit's default 25px nudge — ten key presses per column is
 * no keyboard path for anybody. No column that way: the card stays put.
 */
export const columnKeyboardCoordinates: KeyboardCoordinateGetter = (
  event,
  { currentCoordinates, context },
) => {
  const from = context.collisionRect;
  const dir = arrowDirection(event.code);
  if (from === null || dir === null) return undefined;
  event.preventDefault();
  const here = centre(from);
  let best: Coordinates | null = null;
  let bestDistance = Infinity;
  for (const rect of context.droppableRects.values()) {
    // The column the card already sits in is never "next" — on a stacked board its centre may
    // well lie below the card.
    const inside =
      here.x >= rect.left && here.x <= rect.right && here.y >= rect.top && here.y <= rect.bottom;
    if (inside) continue;
    const c = centre(rect);
    // Only columns lying in the arrow's direction.
    const along = (c.x - here.x) * dir.x + (c.y - here.y) * dir.y;
    if (along <= 1) continue;
    const distance = Math.hypot(c.x - here.x, c.y - here.y);
    if (distance < bestDistance) {
      bestDistance = distance;
      best = c;
    }
  }
  if (best === null) return undefined;
  return { x: currentCoordinates.x + best.x - here.x, y: currentCoordinates.y + best.y - here.y };
};

/** dnd-kit's hidden instructions, read when focus lands on a draggable card. */
export const KANBAN_DRAG_INSTRUCTIONS: ScreenReaderInstructions = {
  draggable:
    "Để kéo thẻ: nhấn phím cách hoặc Enter, dùng phím mũi tên để sang cột khác, nhấn phím cách " +
    "lần nữa để thả, hoặc Esc để huỷ. Cũng có thể dùng nút “Chuyển sang cột…” trên thẻ.",
};

/**
 * What a screen reader hears during a drag. Every sentence says whether the column TAKES the card —
 * the same fact the outline and `kanbanDropHint` show to sighted staff. A drop on an allowed column
 * says the card moves only once the server answers (no optimistic move).
 */
export function kanbanAnnouncements(
  labels: BangNhanTrangThai,
  gate: KanbanDragGate | null,
): Announcements {
  const allowedOver = (
    data: Record<string, unknown> | undefined,
    overId: UniqueIdentifier,
  ): boolean => {
    const task = draggedTask(data);
    return task !== null && dropTarget(task, overId, gate) !== null;
  };
  return {
    onDragStart: ({ active }) => `Đã nhấc thẻ ${String(active.id)}.`,
    onDragOver: ({ active, over }) => {
      const code = String(active.id);
      if (over === null) return `Thẻ ${code} không ở trên cột nào.`;
      const label = nhanTrangThai(labels, String(over.id));
      return allowedOver(active.data.current, over.id)
        ? `Thẻ ${code} đang ở trên cột “${label}”. Thả để chuyển sang cột này.`
        : `Thẻ ${code} đang ở trên cột “${label}” — không chuyển được sang cột này.`;
    },
    onDragEnd: ({ active, over }) => {
      const code = String(active.id);
      if (over === null) return `Đã thả thẻ ${code} ngoài các cột. Thẻ ở nguyên cột cũ.`;
      const label = nhanTrangThai(labels, String(over.id));
      return allowedOver(active.data.current, over.id)
        ? `Đã thả thẻ ${code} vào cột “${label}”. Thẻ chuyển khi máy chủ xác nhận.`
        : `Cột “${label}” không nhận thẻ ${code}. Thẻ ở nguyên cột cũ.`;
    },
    onDragCancel: ({ active }) => `Đã huỷ kéo. Thẻ ${String(active.id)} ở nguyên cột cũ.`,
  };
}

/**
 * `aria-roledescription` of a card that can be dragged — what a screen reader says instead of
 * "article". Only on a card this account may move: a denied card is never announced as draggable.
 */
export const KANBAN_CARD_ROLE = "thẻ kéo được";

/** Words under a column that will NOT take the dragged card — the outline is never the only signal. */
export const KANBAN_DROP_REFUSED_HINT = "Không chuyển được thẻ này sang cột này.";
