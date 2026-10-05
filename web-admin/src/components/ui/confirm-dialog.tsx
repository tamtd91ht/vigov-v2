import { TriangleAlert, type LucideIcon } from "lucide-react";
import type { ComponentProps, ElementType, ReactNode } from "react";

import { cn } from "@/lib/cn";

/**
 * Confirmation box — spec v2 §7 "Hộp xác nhận": the title is the SPECIFIC question ("Xoá Lê Văn
 * Cường khỏi danh bạ?"), then the consequence, then the buttons — the confirm button names the
 * action, never "OK".
 *
 * A FRAME AROUND THE SCREEN'S EXISTING CONFIRM FLOW, NOT A NEW ONE. It is not modal and holds no
 * state: the screen's own `<form onSubmit>` (or `<div>`), its fields, its buttons and its handlers
 * are passed in unchanged; `as` keeps the element, every other prop (`onSubmit`, `aria-label`, `id`)
 * passes through. So adopting it cannot change when the action runs or what it sends.
 *
 * Guide §5: a small modal-like box — white, 12px radius, the navy `--shadow-md`, no border.
 *
 * No hooks, no portal: it renders where the screen already rendered its confirm block, server
 * rendering included (`react-dom/server` tests read its text).
 */
type ConfirmElement = "form" | "div" | "section";

export type ConfirmDialogProps<E extends ConfirmElement = "div"> = {
  as?: E;
  /** The specific question. */
  title: ReactNode;
  titleAs?: "h2" | "h3" | "h4";
  /** `danger` for a destructive action (red icon), `default` otherwise. */
  tone?: "danger" | "default";
  icon?: LucideIcon;
  /** The consequence sentence(s) and any fields of the existing flow. */
  children?: ReactNode;
  /** The existing buttons, confirm first. Right-aligned. */
  actions: ReactNode;
  className?: string;
} & Omit<ComponentProps<E>, "title" | "children" | "className">;

export function ConfirmDialog<E extends ConfirmElement = "div">({
  as,
  title,
  titleAs: Heading = "h3",
  tone = "default",
  icon,
  children,
  actions,
  className,
  ...props
}: ConfirmDialogProps<E>) {
  const Element = (as ?? "div") as ElementType;
  const Icon = icon ?? TriangleAlert;
  return (
    <Element
      className={cn("rounded-popover bg-surface p-4 shadow-md", className)}
      {...props}
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
        <div className="min-w-0 flex-1">
          <Heading className="m-0 text-base leading-snug font-semibold text-ink-900">{title}</Heading>
          {children !== undefined && (
            <div className="mt-1.5 flex flex-col gap-3 text-sm text-ink-700">{children}</div>
          )}
        </div>
      </div>
      <div className="mt-4 flex flex-wrap items-center justify-end gap-2">{actions}</div>
    </Element>
  );
}
