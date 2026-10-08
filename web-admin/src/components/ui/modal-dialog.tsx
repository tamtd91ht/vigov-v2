"use client";

import { X } from "lucide-react";
import { useEffect, useRef, type ReactNode } from "react";

import { cn } from "@/lib/cn";

import { IconButton } from "./icon-button";

/**
 * A CENTRED modal for a short form over a list — the prototype's `Dialog` (`Giao việc mới`,
 * `Nhập từ Excel`, `Xoá đã chọn`; ADR 0068 §Sửa đổi 06/10/2026 lần 5). `LargeDialog` is the
 * right-pinned record panel; this is the box a form opens in. Two widths, the prototype's own:
 * `md` 500px, `lg` 800px (the create form of a `Theo văn bản` task carries three document lists).
 *
 * NATIVE `<dialog>` + `showModal()`, same reasoning as `LargeDialog`: the top layer makes the page
 * behind inert, so Tab cannot leave the box, and a second modal opened from inside a `LargeDialog`
 * stacks above it without a hand-made trap. Esc ASKS (`onDismiss`), it does not force.
 *
 * MOUNTED = OPEN; unmounting returns focus to whatever opened it. jsdom has no `showModal`: the
 * `open` attribute is the fallback there, so tests can still read the content.
 *
 * CENTRED WHEREVER IT IS MOUNTED: see `m-auto!` below — the parent's child-margin utilities cannot
 * pin it to the top of the window.
 *
 * LAYOUT CONTRACT: a flex column capped at 90dvh with no scroll of its own; the caller puts an
 * `overflow-y-auto` region inside so the header and the buttons stay in sight.
 *
 * THE ✕ IS DRAWN BY DEFAULT, as the prototype's `DialogContent` does on every dialog
 * (`vigov-require/apps/admin/src/components/ui/dialog.tsx:53,83-95`: `showCloseButton = true`, ghost
 * `icon-sm`, `absolute top-2 right-2`). The owner's decision of ADR 0068 lần 6 #1 (the spec's dialogs
 * for the whole app) is what adds it to every ModalDialog at once. It calls `onDismiss`, exactly as
 * Esc does — so a dialog that refuses Esc while it saves (`onDismiss={() => !busy && onClose()}`)
 * refuses the ✕ the same way, with no change on its side. `closeDisabled` additionally greys it out
 * for such a dialog; `showClose={false}` is the opt-out for one that draws its own close control.
 */
export function ModalDialog({
  titleId,
  onDismiss,
  size = "md",
  closeLabel = "Đóng",
  showClose = true,
  closeDisabled = false,
  initialFocusId,
  className,
  children,
}: {
  /** Id of the heading inside — the dialog's accessible name. */
  titleId: string;
  /** Esc, the ✕, or the browser closing the dialog. The parent decides whether it closes. */
  onDismiss: () => void;
  size?: "md" | "lg";
  /** The ✕'s `aria-label` / `title`. */
  closeLabel?: string;
  /** `false` only for a dialog that draws its own close control — two ✕ in one box read as two actions. */
  showClose?: boolean;
  /** Greys the ✕ out, for a dialog whose `onDismiss` already refuses while a send is in flight. */
  closeDisabled?: boolean;
  /**
   * Id of a control INSIDE the box that takes the opening focus instead of the title — for a dialog
   * whose whole point is typing into one field (the prototype's `autoFocus` on `Thêm hạng mục`).
   * React's own `autoFocus` cannot do it here: it fires on mount, BEFORE this effect's `showModal()`,
   * which then moves focus to the box's first focusable element (the title). Absent, not found
   * inside the box, or disabled → the title keeps it, as for every other dialog.
   */
  initialFocusId?: string;
  className?: string;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  // Read once, at opening: the effect below runs on mount only, as the opening itself does.
  const initialFocus = useRef(initialFocusId);

  useEffect(() => {
    const el = ref.current;
    if (el === null) return;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    if (typeof el.showModal === "function") {
      if (!el.open) el.showModal();
    } else {
      el.setAttribute("open", "");
    }
    const id = initialFocus.current;
    if (id !== undefined) {
      const target = document.getElementById(id);
      // Only inside THIS box: an id elsewhere on the page sits behind the inert backdrop.
      if (target !== null && el.contains(target) && !target.matches(":disabled")) target.focus();
    }
    return () => {
      if (opener !== null && opener.isConnected) opener.focus();
    };
  }, []);

  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      aria-modal="true"
      className={cn(
        // `open:` only: a `flex` that applied while closed would override the UA's `display: none`.
        //
        // `m-auto!` (important), NOT `m-auto`: the vertical centring IS the auto margin (the UA puts a
        // modal dialog at `position: fixed; inset-block: 0; height: fit-content`). Pages wrap their
        // children in `[&>*]:my-0`, and that rule (`.\[\&\>\*\]\:my-0>*{margin-block:0}`) is emitted
        // AFTER `.m-auto` at equal specificity — so every dialog rendered as such a child was pinned
        // to the top edge of the window (Nhiệm vụ's `Giao việc mới`, prod 07/10/2026). A parent's
        // spacing rule has no business moving a top-layer box; important makes that structural.
        //
        // Look = shadcn's DialogContent (spec 00 §5): `rounded-xl bg-popover p-4 ring-1
        // ring-foreground/10 gap-4`, 14px text. The overlay is the dialog's own `::backdrop`
        // (`bg-black/10` + `backdrop-blur-xs`, ADR 0068 lần 6 #3 lifts §11's blur ban) — the native
        // element has no separate overlay node, and Radix's Dialog, which has one, injects a
        // `<style>` this app refuses (ADR 0068 §4).
        "m-auto! box-border max-h-[90dvh] w-[calc(100vw-2rem)] overflow-hidden rounded-xl border-0 bg-popover p-4 text-sm text-popover-foreground ring-1 ring-foreground/10 open:flex open:flex-col open:gap-4",
        size === "lg" ? "max-w-[800px]" : "max-w-[500px]",
        "backdrop:bg-black/10 backdrop:backdrop-blur-xs",
        className,
      )}
      onCancel={(e) => {
        e.preventDefault();
        onDismiss();
      }}
    >
      {children}
      {/* LAST in the box, as in the prototype: the dialog's first focusable element stays the title (see
          `ModalDialogHeader`), and Tab reaches the ✕ after the form's own buttons. */}
      {showClose && (
        <IconButton
          type="button"
          variant="ghost"
          label={closeLabel}
          size="sm"
          className="absolute top-2 right-2"
          disabled={closeDisabled}
          onClick={onDismiss}
        >
          <X aria-hidden="true" focusable="false" className="size-4" />
        </IconButton>
      )}
    </dialog>
  );
}

/**
 * The dialog's title + one line under it — the prototype's `DialogHeader`.
 *
 * THE TITLE TAKES THE OPENING FOCUS (`tabIndex={-1}`, first focusable element in the box, so
 * `showModal()` lands there): a screen reader starts by reading what the box is, and a confirm box never
 * opens with its destructive button already focused (a dialog may name one field instead —
 * `ModalDialog`'s `initialFocusId`). It is a programmatic focus target, not a
 * control, so it draws no ring (`outline-none`): Chrome matches `:focus-visible` on it after
 * `showModal()`, and the global ring (`globals.css`, `:focus-visible`) painted a blue box round the
 * title of every dialog (ADR 0068 lần 6 review, T1-15). Real controls keep that ring.
 */
export function ModalDialogHeader({
  titleId,
  title,
  description,
}: {
  titleId: string;
  title: ReactNode;
  description?: ReactNode;
}) {
  return (
    <div className="flex shrink-0 flex-col gap-2">
      <h2 id={titleId} tabIndex={-1} className="m-0 font-heading outline-none text-base leading-none font-medium text-popover-foreground">
        {title}
      </h2>
      {description !== undefined && <p className="m-0 text-sm text-muted-foreground">{description}</p>}
    </div>
  );
}
