"use client";

import { KeyboardSensor, MouseSensor, TouchSensor, useSensor, useSensors, type Announcements } from "@dnd-kit/core";

/**
 * The drag set-up shared by the two views of Thuộc tính danh bạ — the ordered list and the 2-column layout
 * preview (ADR 0101 #3, #3c). One place, so the two views cannot drift into different sensors or different
 * spoken instructions for the same act.
 */

/** The drag instructions a screen reader hears on a handle (dnd-kit's own are English). */
export const DRAG_INSTRUCTIONS = {
  draggable:
    "Nhấn Space hoặc Enter để nhấc thuộc tính, dùng mũi tên để di chuyển, nhấn Space hoặc Enter lần nữa để thả, Esc để huỷ. Cũng có thể dùng nút Lên / Xuống ở mỗi thuộc tính.",
};

export function orderAnnouncements(labelOf: (code: string) => string): Announcements {
  return {
    onDragStart: ({ active }) => `Đã nhấc thuộc tính ${labelOf(String(active.id))}.`,
    onDragOver: ({ active, over }) =>
      over === null ? `Thuộc tính ${labelOf(String(active.id))} không ở trên ô nào.` : `Đang ở vị trí của ${labelOf(String(over.id))}.`,
    onDragEnd: ({ active, over }) =>
      over === null ? `Đã thả ${labelOf(String(active.id))}, thứ tự không đổi.` : `Đã thả ${labelOf(String(active.id))} vào vị trí của ${labelOf(String(over.id))}.`,
    onDragCancel: ({ active }) => `Đã huỷ kéo ${labelOf(String(active.id))}, thứ tự không đổi.`,
  };
}

/** Mouse, touch and keyboard. */
export function useOrderSensors() {
  return useSensors(
    // Mouse, not Pointer: a pointer sensor would race the touch sensor for one finger (`so-nhiem-vu.tsx`).
    useSensor(MouseSensor, { activationConstraint: { distance: 6 } }),
    useSensor(TouchSensor, { activationConstraint: { delay: 200, tolerance: 6 } }),
    useSensor(KeyboardSensor),
  );
}
