"use client";

import { useEffect, useRef, type ReactNode } from "react";

import { cn } from "@/lib/cn";

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
 * LAYOUT CONTRACT: a flex column capped at 90dvh with no scroll of its own; the caller puts an
 * `overflow-y-auto` region inside so the header and the buttons stay in sight.
 */
export function ModalDialog({
  titleId,
  onDismiss,
  size = "md",
  className,
  children,
}: {
  /** Id of the heading inside — the dialog's accessible name. */
  titleId: string;
  /** Esc, or the browser closing the dialog. The parent decides whether it closes. */
  onDismiss: () => void;
  size?: "md" | "lg";
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
        "m-auto box-border max-h-[90dvh] w-[calc(100vw-2rem)] overflow-hidden rounded-card border border-solid border-line bg-surface p-6 text-ink-900 shadow-md open:flex open:flex-col open:gap-4",
        size === "lg" ? "max-w-[800px]" : "max-w-[500px]",
        "backdrop:bg-brand-600/50",
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

/** The dialog's title + one line under it — the prototype's `DialogHeader`. */
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
    <div className="flex shrink-0 flex-col gap-1.5">
      <h2 id={titleId} tabIndex={-1} className="m-0 text-lg leading-snug font-semibold text-ink-900">
        {title}
      </h2>
      {description !== undefined && <p className="m-0 text-sm text-ink-500">{description}</p>}
    </div>
  );
}
