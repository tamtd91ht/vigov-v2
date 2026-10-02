"use client";

import { TriangleAlert, type LucideIcon } from "lucide-react";
import { useEffect, useId, useRef, type ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * A modal built on the native `<dialog>` with `showModal()`: the browser traps focus inside it,
 * makes the page behind inert, closes it on Escape and returns focus to the control that opened
 * it — none of which a hand-rolled overlay gets right by default.
 *
 * LOOK: web-admin's confirm box (`web-admin/src/components/ui/confirm-dialog.tsx`) — an icon tile,
 * the title as the specific question, then the screen's own content and buttons. The frame is
 * drawn here; the behaviour (open/close, Escape, what the buttons do) is unchanged.
 *
 * WHY THE NATIVE ELEMENT AND NOT A RADIX DIALOG: Radix positions and locks scroll with inline `style`
 * attributes, which this console's CSP refuses (`style-src` has no 'unsafe-inline', `lib/csp.ts`).
 * `<dialog>` and its `::backdrop` are styled entirely by classes. Flat tinted backdrop, no frosted glass (ADR 0068 §11).
 */
export function Dialog({
  open,
  title,
  onClose,
  children,
  tone = "default",
  icon,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
  /** `danger` for a destructive action (red icon tile), `default` otherwise. */
  tone?: "danger" | "default";
  icon?: LucideIcon;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const Icon = icon ?? TriangleAlert;

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      className={cn(
        "m-auto w-[min(32rem,calc(100vw-2rem))] max-h-[calc(100dvh-2rem)] overflow-y-auto",
        "rounded-card border border-line bg-surface p-5 text-ink-900 shadow-md",
        "backdrop:bg-[rgba(15,27,45,0.45)]",
      )}
      aria-labelledby={titleId}
      onCancel={(e) => {
        // Escape: let the parent decide, so its state and the element never disagree.
        e.preventDefault();
        onClose();
      }}
    >
      <div className="flex items-start gap-3">
        <span
          aria-hidden="true"
          className={cn(
            "grid size-9 shrink-0 place-items-center rounded-full",
            tone === "danger" ? "bg-danger-50 text-danger-600" : "bg-brand-50 text-brand-600",
          )}
        >
          <Icon className="size-[18px]" strokeWidth={1.8} focusable="false" />
        </span>
        <h2 id={titleId} className="m-0 min-w-0 self-center text-base leading-snug font-semibold [overflow-wrap:anywhere] text-ink-900">
          {title}
        </h2>
      </div>
      {open ? <div className="mt-3 flex flex-col gap-4 text-sm text-ink-700 [&_p]:m-0">{children}</div> : null}
    </dialog>
  );
}

/** The dialog's button row: confirm first, then cancel, right-aligned (web-admin confirm box). */
export function DialogActions({ children }: { children: ReactNode }) {
  return <div className="flex flex-wrap items-center justify-end gap-2 pt-1">{children}</div>;
}
