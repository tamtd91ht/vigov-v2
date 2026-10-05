"use client";

import { useEffect, useRef, type ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * The LARGE detail dialog over a list screen (ADR 0068 §Sửa đổi 05/10/2026 #3): a native `<dialog>`
 * opened with `showModal()`, ~76rem wide and 92% of the viewport tall from 768px, the whole screen
 * below it (docs/ui-ux/15 §5.2). Nhiệm vụ is the pilot; Đơn thư and Phản ánh follow in their own
 * rounds, which is why it lives in `components/ui` and knows nothing about tasks.
 *
 * NATIVE, NOT A HAND-MADE TRAP — same reasoning as `features/noi-dung/overlay-dialog.tsx`, which this
 * deliberately does not modify (other screens size their forms by `.overlay-dialog`): `showModal()`
 * puts the box in the top layer and makes the page behind it inert, so Tab cannot leave it and a
 * screen reader cannot read the list behind; Esc fires `cancel`.
 *
 * MOUNTED = OPEN. The parent renders this only while a record is open; unmounting closes it, and
 * focus returns to whatever opened it (the row's `Mở NV19`), which a reload may have re-rendered —
 * only a node still in the page takes focus back.
 *
 * ESC ASKS, IT DOES NOT FORCE: `cancel` is prevented and `onDismiss` decides, as the ✕ does.
 *
 * LAYOUT CONTRACT: the dialog is a flex column with no scroll of its own (`overflow-hidden`); the
 * caller's content puts its own `overflow-y-auto` region inside, so a header can stay put while the
 * body scrolls. Flat navy backdrop, no blur (ADR 0068 §11); the 200ms entrance is zeroed under
 * `prefers-reduced-motion` by the base layer of `globals.css`.
 *
 * jsdom has no `showModal`: the `open` attribute is the fallback there, so tests can still assert on
 * the content.
 */
export function LargeDialog({
  titleId,
  onDismiss,
  className,
  children,
}: {
  /** Id of the heading inside — the dialog's accessible name. */
  titleId: string;
  /** Esc, or the browser closing the dialog. The parent decides whether it closes. */
  onDismiss: () => void;
  className?: string;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const el = ref.current;
    if (el === null) return;
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    if (typeof el.showModal === "function") {
      if (!el.open) el.showModal();
    } else {
      el.setAttribute("open", "");
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
        "m-auto box-border overflow-hidden border-0 bg-canvas p-0 text-ink-900 open:flex open:flex-col",
        // Below 768px: the whole screen. The UA caps a modal at `100% - 2em`; lift both caps.
        "h-dvh max-h-none w-screen max-w-none rounded-none",
        "md:h-[92vh] md:w-[min(76rem,96vw)] md:rounded-[var(--r-xl)] md:border md:border-solid md:border-line md:shadow-md",
        "backdrop:bg-brand-600/50",
        "motion-safe:animate-[menu-in_200ms_ease-out]",
        className,
      )}
      onCancel={(e) => {
        e.preventDefault();
        onDismiss();
      }}
    >
      {children}
    </dialog>
  );
}
