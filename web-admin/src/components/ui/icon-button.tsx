import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

import { Button, type ButtonProps } from "./button";

/**
 * Icon-only button — spec §4: "Nút chỉ có icon bắt buộc có `aria-label` và `title`".
 *
 * `label` IS REQUIRED BY THE TYPE and feeds BOTH attributes, so neither can be forgotten: the
 * accessible name for a screen reader and the hover tooltip for a sighted user are the same
 * Vietnamese words. `aria-label`, `title` and text `children` are removed from the accepted props,
 * so a caller cannot set one and let the other drift.
 *
 * The icon itself is decorative; pass it `aria-hidden`.
 */
export type IconButtonProps = Omit<ButtonProps, "aria-label" | "title" | "children" | "icon" | "variant"> & {
  /** Vietnamese action name, e.g. "Sửa", "Xoá khỏi danh bạ". Becomes `aria-label` and `title`. */
  label: string;
  /** The icon element, e.g. `<Pencil aria-hidden />`. */
  children: ReactNode;
  variant?: "icon" | "secondary" | "danger" | "ghost";
  /**
   * Declared `never`, not merely omitted: TypeScript does not run excess-property checks on
   * HYPHENATED JSX attributes, so an `Omit` alone still accepted `aria-label="…"` (measured by
   * the type test in `ui.test.tsx`). The label is `label`, once.
   */
  "aria-label"?: never;
  "aria-labelledby"?: never;
};

export function IconButton({ label, children, variant = "icon", size = "sm", className, ...props }: IconButtonProps) {
  return (
    <Button
      {...props}
      variant={variant}
      size={size}
      // AFTER the spread, so nothing a caller passes can replace the accessible name or the tooltip.
      aria-label={label}
      title={label}
      className={variant === "icon" ? className : cn("px-0", sizeSquare[size], className)}
    >
      {children}
    </Button>
  );
}

const sizeSquare = { sm: "w-[34px]", md: "w-10", lg: "w-[46px]" } as const;
