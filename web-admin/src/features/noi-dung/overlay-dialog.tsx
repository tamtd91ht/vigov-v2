"use client";

import { useEffect, useRef, type ReactNode } from "react";

/**
 * The overlay of §7 ("Modal `Thêm nội dung`") and §3 ("modal cấu hình đồng bộ"): a native `<dialog>`
 * opened with `showModal()`.
 *
 * NATIVE, NOT A HAND-MADE TRAP. `showModal()` puts the dialog in the top layer and makes the rest of the
 * page inert, so Tab cannot leave it and a screen reader cannot read behind it; Esc fires `cancel`. A
 * `div role="dialog"` would need all three written by hand, and the one forgotten is the one that leaks
 * focus behind an open form. The repo's other dialogs (`task-import-dialog.tsx`, `dot-thu-chi.tsx`) are
 * IN-PAGE sections, so there was no overlay to reuse.
 *
 * MOUNTED = OPEN. The parent renders this only while the form is open; unmounting closes it. Focus goes
 * back to whatever opened it (`+ Thêm nội dung`, the row's `✎`, `Cấu hình`) — not to the top of the
 * page, which at 320px is a long scroll away from where the officer was.
 *
 * ESC ASKS, IT DOES NOT FORCE: `cancel` is prevented and `onDismiss` decides. The forms refuse while a
 * save is in flight, for the same reason their `Huỷ` is disabled then — closing would hide the outcome.
 *
 * jsdom has no `showModal` (the tests render to a string or into jsdom): the `open` attribute is the
 * fallback there, so the content is still in the document to assert on.
 */
export function OverlayDialog({
  titleId,
  onDismiss,
  children,
}: {
  /** Id of the heading inside — the dialog's accessible name. */
  titleId: string;
  /** Esc, or the browser closing the dialog. The parent decides whether it closes. */
  onDismiss: () => void;
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
      // The row's `✎` may have been re-rendered by the reload after a save; only a node still in the
      // page can take focus back.
      if (opener !== null && opener.isConnected) opener.focus();
    };
  }, []);

  return (
    <dialog
      ref={ref}
      className="overlay-dialog"
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault();
        onDismiss();
      }}
    >
      {children}
    </dialog>
  );
}
