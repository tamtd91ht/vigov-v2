"use client";

import { useEffect, useRef, useState } from "react";

import {
  KANBAN_MOVE_BUTTON,
  KANBAN_RETURN_NOTE,
  kanbanMoveButtonName,
  kanbanMoveItemLabel,
  type BangNhanTrangThai, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
  type TrangThaiNhiemVu, // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
} from "./nhan-nhiem-vu";

/** Where focus goes after a key inside the open menu; `close` = close and return to the button. */
export type MenuKeyResult = { readonly active: number; readonly close: boolean };

/**
 * WAI-ARIA menu button keys, pure so vitest can run them without a DOM. Arrows wrap (APG);
 * Home/End jump; Escape and Tab close. Enter and Space are not here: a `role="menuitem"` that is a
 * native `<button>` already activates on both.
 */
export function menuKey(active: number, key: string, count: number): MenuKeyResult | null {
  if (count === 0) return key === "Escape" || key === "Tab" ? { active: -1, close: true } : null;
  switch (key) {
    case "ArrowDown":
      return { active: (active + 1) % count, close: false };
    case "ArrowUp":
      return { active: (active - 1 + count) % count, close: false };
    case "Home":
      return { active: 0, close: false };
    case "End":
      return { active: count - 1, close: false };
    case "Escape":
    case "Tab":
      return { active, close: true };
    default:
      return null;
  }
}

/**
 * The keyboard and screen-reader path of a Kanban move — equal to dragging, not a fallback.
 *
 * Lists exactly `targets` (`clickableTransitions`, the drawer's list), and calls `onMove`, which
 * is the SAME function the drop handler calls. Choosing an item closes the menu and puts focus
 * back on the button; the card itself does not move until the server has answered.
 */
export function KanbanMoveMenu({
  code,
  targets,
  labels,
  disabled,
  showReturnNote,
  onMove,
}: {
  code: string;
  targets: readonly TrangThaiNhiemVu[];
  labels: BangNhanTrangThai;
  /** A move is in flight somewhere on the board. */
  disabled: boolean;
  /** This card is `cho-duyet` and the account may return it — the step lives in the drawer. */
  showReturnNote: boolean;
  onMove: (target: TrangThaiNhiemVu) => void;
}) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const buttonId = `nut-chuyen-cot-${code}`;
  const menuId = `menu-chuyen-cot-${code}`;

  useEffect(() => {
    if (open) itemRefs.current[active]?.focus();
  }, [open, active]);

  function openAt(i: number) {
    setActive(i);
    setOpen(true);
  }

  function close(returnFocus: boolean) {
    setOpen(false);
    if (returnFocus) buttonRef.current?.focus();
  }

  return (
    <div
      className="chuyen-cot"
      onBlur={(e) => {
        // Focus left the button and the menu altogether (click elsewhere, Tab away).
        if (open && !e.currentTarget.contains(e.relatedTarget as Node | null)) setOpen(false);
      }}
    >
      <button
        ref={buttonRef}
        id={buttonId}
        type="button"
        className="nut-phu"
        aria-label={kanbanMoveButtonName(code)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        disabled={disabled}
        onClick={() => (open ? close(false) : openAt(0))}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") {
            e.preventDefault();
            openAt(0);
          } else if (e.key === "ArrowUp") {
            e.preventDefault();
            openAt(targets.length - 1);
          }
        }}
      >
        {KANBAN_MOVE_BUTTON}
      </button>
      {open && (
        <>
          <ul id={menuId} role="menu" aria-labelledby={buttonId} className="menu-chuyen-cot">
            {targets.map((t, i) => (
              <li key={t} role="none">
                <button
                  ref={(el) => {
                    itemRefs.current[i] = el;
                  }}
                  type="button"
                  role="menuitem"
                  tabIndex={i === active ? 0 : -1}
                  disabled={disabled}
                  onKeyDown={(e) => {
                    const r = menuKey(active, e.key, targets.length);
                    if (r === null) return;
                    if (e.key !== "Tab") e.preventDefault();
                    if (r.close) close(e.key === "Escape");
                    else setActive(r.active);
                  }}
                  onClick={() => {
                    close(true);
                    onMove(t);
                  }}
                >
                  {kanbanMoveItemLabel(labels, t)}
                </button>
              </li>
            ))}
          </ul>
          {showReturnNote && <p className="ghi-chu">{KANBAN_RETURN_NOTE}</p>}
        </>
      )}
    </div>
  );
}
