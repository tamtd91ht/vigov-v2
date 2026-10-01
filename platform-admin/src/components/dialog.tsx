"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";

/**
 * A modal built on the native `<dialog>` with `showModal()`: the browser traps focus inside it,
 * makes the page behind inert, closes it on Escape and returns focus to the control that opened
 * it — none of which a hand-rolled overlay gets right by default.
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
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className="dialog"
      aria-labelledby={titleId}
      onCancel={(e) => {
        // Escape: let the parent decide, so its state and the element never disagree.
        e.preventDefault();
        onClose();
      }}
    >
      <h2 id={titleId} className="dialog-title">
        {title}
      </h2>
      {open ? children : null}
    </dialog>
  );
}
