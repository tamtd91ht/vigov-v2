"use client";

import { X, type LucideIcon } from "lucide-react";
import { useEffect, useId, useRef, type ReactNode } from "react";

import { cn } from "@/lib/cn";

import { Button } from "./ui/button";

/**
 * A modal built on the native `<dialog>` with `showModal()`: the browser traps focus inside it,
 * makes the page behind inert, closes it on Escape and returns focus to the control that opened
 * it — none of which a hand-rolled overlay gets right by default.
 *
 * LOOK: web-admin's `ModalDialog` + `ModalDialogHeader` (`web-admin/src/components/ui/modal-dialog.tsx`,
 * spec 00 §5, ADR 0068 lần 6): shadcn's DialogContent — 500px, `rounded-xl`, `p-4`, a `ring-1` hairline,
 * 14px text, the title 16px/500 with no icon tile, a ✕ at the top right, and the overlay the dialog's
 * own `::backdrop` (`bg-black/10` + `backdrop-blur-xs`; lần 6 #3 lifted §11's blur ban). Copied, not
 * shared: change both. The frame is drawn here; the behaviour (open/close, Escape, what the buttons
 * do) is the caller's, unchanged.
 *
 * THE ✕ CALLS `onClose`, EXACTLY AS ESCAPE DOES. Every caller passes `() => !busy && close()`, so a
 * dialog that refuses Escape while it saves refuses the ✕ the same way; nothing new can be dismissed.
 * It is LAST in the box so Tab reaches it after the form's own buttons.
 *
 * THE TITLE TAKES THE OPENING FOCUS (`tabIndex={-1}`, first focusable element in the box, so
 * `showModal()` lands there): a screen reader starts by reading what the box is, and a confirm box
 * never opens with its destructive button already focused — before 13/10/2026 the bot relink box did.
 * A programmatic target, not a control, so it draws no ring.
 *
 * `tone` and `icon` ARE STILL ACCEPTED AND IGNORED, as `PageHeader` does with `icon`: web-admin's
 * dialogs carry no icon tile, and every call site here passes one. A destructive dialog is told by its
 * question, its gold warning and its red confirm button.
 *
 * WHY THE NATIVE ELEMENT AND NOT A RADIX DIALOG: Radix positions and locks scroll with inline `style`
 * attributes, which this console's CSP refuses (`style-src` has no 'unsafe-inline', `lib/csp.ts`).
 * `<dialog>` and its `::backdrop` are styled entirely by classes.
 */
export function Dialog({
  open,
  title,
  onClose,
  children,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
  /** Ignored since 13/10/2026 (no icon tile) — kept so existing callers compile. */
  tone?: "danger" | "default";
  /** Ignored since 13/10/2026 (no icon tile) — kept so existing callers compile. */
  icon?: LucideIcon;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) {
      // jsdom has no `showModal`; the attribute is the fallback there, so tests still read the content.
      if (typeof el.showModal === "function") el.showModal();
      else el.setAttribute("open", "");
    }
    if (!open && el.open) el.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className={cn(
        // `open:` only: a `flex` applying while closed would override the UA's `display: none`.
        "m-auto box-border max-h-[90dvh] w-[calc(100vw-2rem)] max-w-[500px] overflow-y-auto",
        "rounded-xl border-0 bg-popover p-4 text-sm text-popover-foreground ring-1 ring-foreground/10",
        "open:flex open:flex-col open:gap-4",
        "backdrop:bg-black/10 backdrop:backdrop-blur-xs",
      )}
      aria-labelledby={titleId}
      onCancel={(e) => {
        // Only Escape AT the dialog — a file input's picker fires a bubbling `cancel` of its own.
        if (e.target !== e.currentTarget) return;
        // Let the parent decide, so its state and the element never disagree.
        e.preventDefault();
        onClose();
      }}
    >
      {/* `pr-8` keeps a long question clear of the ✕. */}
      <h2
        id={titleId}
        tabIndex={-1}
        className="m-0 pr-8 text-base leading-snug font-medium [overflow-wrap:anywhere] text-popover-foreground outline-none"
      >
        {title}
      </h2>
      {open ? <div className="flex min-w-0 flex-col gap-4 [&_p]:m-0">{children}</div> : null}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        aria-label="Đóng"
        title="Đóng"
        className="absolute top-2 right-2 size-7 px-0"
        onClick={onClose}
      >
        <X aria-hidden="true" focusable="false" className="size-4" />
      </Button>
    </dialog>
  );
}

/**
 * The dialog's button row, right-aligned, in web-admin's order: "Huỷ" first, the action LAST (the
 * prototype's DialogFooter). Callers pass them in that order; this only lays them out.
 */
export function DialogActions({ children }: { children: ReactNode }) {
  return <div className="flex flex-wrap items-center justify-end gap-2">{children}</div>;
}
