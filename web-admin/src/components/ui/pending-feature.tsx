"use client";

import * as PopoverPrimitive from "@radix-ui/react-popover";
import type { LucideIcon } from "lucide-react";
import { useId, type ReactElement, type ReactNode } from "react";

import { cn } from "@/lib/cn";

import { Button, type ButtonSize, type ButtonVariant } from "./button";
import { Card, CardContent, CardHeader, CardTitle } from "./card";
import { controlClass } from "./field";
import { Tab } from "./tabs";
import { Tooltip } from "./tooltip";

/**
 * Placeholder for a part of the spec that is not built yet — ADR 0068 §14 (owner, 02/10/2026).
 *
 * The part is drawn AT ITS SPEC POSITION, AS THE CONTROL IT WILL BE (button, tab, segment, column,
 * card, field, menu item), DISABLED, with a "?" next to it. Hover/focus on "?" says "Tính năng đang
 * phát triển"; pressing it opens the description — the same `{ten, viSao}` sentence the screen's
 * `PHAN_CHUA_DUNG` block used to print.
 *
 * NOTHING HERE CALLS A SERVER OR STORES ANYTHING: a placeholder is presentation, not a feature
 * (ADR 0068 §1). It has no `onClick` to pass, on purpose — there is nothing for it to do.
 *
 * WHY THE "?" IS ITS OWN BUTTON, NEVER A TOOLTIP ON THE DISABLED CONTROL: a disabled control gets no
 * focus and no pointer events in most browsers, and touch has no hover (`tooltip.tsx`). So the "?" is
 * the one focusable thing, and its `aria-label` alone already says the whole sentence; the tooltip
 * only repeats it for a mouse.
 *
 * WHAT IS NOT A PLACEHOLDER: something the owner decided NOT to build (ADR 0062, ADR 0068 §14) gets
 * no spot at all — a placeholder for it announces a feature the authority refused.
 *
 * USES HOOKS (Radix, `useId`): never render it from a component that a mocked-React test runner
 * calls as a plain function (`danh-ba-lien-he.luong.test.tsx`); render it from a child component.
 */

/**
 * One unbuilt part. SAME SHAPE AS EVERY `PHAN_CHUA_DUNG` ENTRY, so an entry is passed as-is — never
 * copied into a second sentence that would drift from the first.
 */
export type PendingFeatureInfo = {
  // vi-name-ok: mirrors the existing `PhanChuaDung` shape of every `PHAN_CHUA_DUNG` array, so entries pass without mapping
  readonly ten: string;
  // vi-name-ok: mirrors the existing `PhanChuaDung` shape of every `PHAN_CHUA_DUNG` array, so entries pass without mapping
  readonly viSao: string;
};

/** Hover/focus text — owner's words, ADR 0068 §14. */
export const PENDING_HOVER_TEXT = "Tính năng đang phát triển";

/**
 * Line added to the description of a Phase-2 part. States which phase, never a date: a public
 * authority's screen saying "coming soon" is a promise with a date nobody set (ADR 0068 §6).
 */
export const PHASE_2_NOTE = "Phần này thuộc giai đoạn 2 của dự án.";

/** Accessible name of the "?" — the whole sentence, because touch and screen readers get no tooltip. */
export function pendingMarkerLabel(name: string): string {
  return `${name} — tính năng đang phát triển. Bấm để xem mô tả`;
}

type Side = "top" | "right" | "bottom" | "left";

type CommonProps = {
  info: PendingFeatureInfo;
  /** Belongs to Phase 2 (`ROADMAP_PHASE2`): the description says so. */
  phase2?: boolean;
  /** Where the tooltip and the description open. */
  side?: Side;
};

export type PendingMarkerProps = CommonProps & {
  /**
   * `inline` sits in the text flow (labels, headers). `corner` pins it to the top-right corner of a
   * `relative` parent (a disabled control wrapped by `PendingFeature`). `end` pins it inside the right
   * padding of a `relative` parent, vertically centred (`PendingButton`, `PendingTab`).
   */
  placement?: "inline" | "corner" | "end";
  /**
   * Tooltip reads "<ten> — Tính năng đang phát triển". For a spot where the control's label is not
   * visible (collapsed sidebar, icon-only control), so the hover still names what it is.
   */
  nameInHover?: boolean;
  className?: string;
};

const PLACEMENT: Record<NonNullable<PendingMarkerProps["placement"]>, string> = {
  inline: "relative",
  corner: "absolute -top-2 -right-2 z-[1]",
  end: "absolute top-1/2 right-2 z-[1] -translate-y-1/2",
};

/** The "?" button + its tooltip + its description. Every variant below is built on it. */
export function PendingMarker({
  info,
  phase2 = false,
  side = "top",
  placement = "inline",
  nameInHover = false,
  className,
}: PendingMarkerProps) {
  const titleId = useId();
  return (
    // `modal` stays false (the default): a modal Radix popover locks page scroll through
    // `react-remove-scroll`, which injects a `<style>` element — what ADR 0068 §4 keeps out.
    <PopoverPrimitive.Root>
      <Tooltip content={nameInHover ? `${info.ten} — ${PENDING_HOVER_TEXT}` : PENDING_HOVER_TEXT} side={side}>
        <PopoverPrimitive.Trigger asChild>
          <button
            type="button"
            aria-label={pendingMarkerLabel(info.ten)}
            data-pending-marker=""
            className={cn(
              "inline-grid size-[18px] shrink-0 cursor-help place-items-center rounded-full border border-solid border-line-strong bg-surface p-0",
              "[font-family:inherit] text-[11px] leading-none font-bold text-ink-500",
              "transition-[border-color,color] duration-150 hover:border-brand-500 hover:text-brand-700",
              "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
              "data-[state=open]:border-brand-500 data-[state=open]:text-brand-700",
              // A ~42px hit area around an 18px mark: older staff and touch (spec §4, 44px targets),
              // without the mark itself crowding the control it sits on.
              "before:absolute before:-inset-3 before:content-['']",
              PLACEMENT[placement],
              className,
            )}
          >
            <span aria-hidden="true">?</span>
          </button>
        </PopoverPrimitive.Trigger>
      </Tooltip>
      <PopoverPrimitive.Portal>
        <PopoverPrimitive.Content
          side={side}
          sideOffset={8}
          collisionPadding={8}
          aria-labelledby={titleId}
          className={cn(
            "z-[60] w-80 max-w-[calc(100vw-1rem)] rounded-xl border border-line bg-surface p-4 text-sm text-ink-700 shadow-lg",
            "origin-(--radix-popover-content-transform-origin) animate-[menu-in_160ms_ease-out]",
            "focus-visible:outline-none",
          )}
        >
          <p id={titleId} className="m-0 text-[15px] leading-snug font-semibold text-ink-900">
            {info.ten}
          </p>
          <p className="m-0 mt-1 text-xs font-semibold text-ink-500">{PENDING_HOVER_TEXT}</p>
          <p className="m-0 mt-2 leading-relaxed">{info.viSao}</p>
          {phase2 && <p className="m-0 mt-2 leading-relaxed text-ink-500">{PHASE_2_NOTE}</p>}
          <div className="mt-3 flex justify-end">
            <PopoverPrimitive.Close asChild>
              <Button type="button" variant="secondary" size="sm">
                Đóng
              </Button>
            </PopoverPrimitive.Close>
          </div>
        </PopoverPrimitive.Content>
      </PopoverPrimitive.Portal>
    </PopoverPrimitive.Root>
  );
}

/**
 * Generic wrapper: the screen draws its own control ALREADY DISABLED (`disabled` on a native control,
 * `aria-disabled` on anything else) and this pins the "?" to its top-right corner. For a control
 * shape the variants below do not cover.
 */
export function PendingFeature({
  children,
  className,
  ...marker
}: CommonProps & { children: ReactElement; className?: string }) {
  return (
    <span className={cn("relative inline-flex max-w-full", className)} data-pending="">
      {children}
      <PendingMarker {...marker} placement="corner" />
    </span>
  );
}

/** A PageHeader / toolbar button. The label defaults to `info.ten`. */
export function PendingButton({
  info,
  phase2,
  side,
  variant = "secondary",
  size = "md",
  icon,
  children,
  className,
}: CommonProps & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** Leading icon, e.g. `<Download aria-hidden="true" />`. */
  icon?: ReactNode;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <span className={cn("relative inline-flex", className)} data-pending="">
      {/* `pr-9` leaves room for the "?" inside the button's frame: a button cannot contain a button. */}
      <Button type="button" variant={variant} size={size} icon={icon} disabled className="pr-9">
        {children ?? info.ten}
      </Button>
      <PendingMarker info={info} phase2={phase2} side={side} placement="end" />
    </span>
  );
}

/**
 * A tab in an existing `TabList`. Drawn with `Tab`, `aria-selected={false}`, `disabled`. A screen
 * with its own arrow-key handling must skip `[role=tab]:disabled` when it moves focus.
 */
export function PendingTab({
  info,
  phase2,
  side = "bottom",
  icon,
  children,
  className,
}: CommonProps & { icon?: LucideIcon; children?: ReactNode; className?: string }) {
  return (
    <span className={cn("relative inline-flex", className)} data-pending="">
      <Tab
        selected={false}
        icon={icon}
        disabled
        aria-disabled="true"
        tabIndex={-1}
        className="cursor-not-allowed pr-9 opacity-60 hover:bg-transparent hover:text-ink-500"
      >
        {children ?? info.ten}
      </Tab>
      <PendingMarker info={info} phase2={phase2} side={side} placement="end" />
    </span>
  );
}

/** A table column header (`<th scope="col">`) for a column the spec has and the data does not. */
export function PendingColumnHeader({
  info,
  phase2,
  side = "bottom",
  children,
  className,
}: CommonProps & { children?: ReactNode; className?: string }) {
  return (
    <th scope="col" className={className} data-pending="">
      <span className="inline-flex items-center gap-1.5 text-ink-400">
        <span>{children ?? info.ten}</span>
        <PendingMarker info={info} phase2={phase2} side={side} />
      </span>
    </th>
  );
}

/**
 * Body cell under a `PendingColumnHeader`. "—" for the eye — never an empty cell (spec §7) — and the
 * real reason for a screen reader, so "—" is not read as "no data".
 */
export function PendingCell({ className }: { className?: string }) {
  return (
    <td className={cn("text-ink-400", className)} data-pending="">
      <span aria-hidden="true">—</span>
      <span className="sr-only">{PENDING_HOVER_TEXT}</span>
    </td>
  );
}

/** KPI card in `StatCard`'s shape: same frame and height, "—" instead of a figure. */
export function PendingStatCard({
  info,
  phase2,
  side,
  icon: Icon,
  label,
  className,
}: CommonProps & { icon: LucideIcon; label?: string; className?: string }) {
  return (
    <div
      className={cn("flex h-full min-w-0 flex-col gap-2 rounded-xl border border-line bg-surface p-4 shadow-sm", className)}
      data-pending=""
    >
      <div className="flex min-w-0 items-center gap-2">
        <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-lg bg-[#f1f4f8] text-ink-400">
          <Icon className="size-4" strokeWidth={1.8} focusable="false" />
        </span>
        <span className="min-w-0 truncate text-xs font-semibold text-ink-500">{label ?? info.ten}</span>
        <PendingMarker info={info} phase2={phase2} side={side} className="ml-auto" />
      </div>
      <p className="m-0 text-[26px] leading-none font-bold text-ink-400">—</p>
      <p className="m-0 text-xs text-ink-500">{PENDING_HOVER_TEXT}</p>
    </div>
  );
}

/**
 * A whole card / section of a screen (`Card` + header). `children` may hold DISABLED controls that
 * show the section's shape; without them the body is the one sentence.
 */
export function PendingSection({
  info,
  phase2,
  side,
  title,
  titleAs = "h2",
  children,
  className,
}: CommonProps & { title?: ReactNode; titleAs?: "h2" | "h3" | "h4"; children?: ReactNode; className?: string }) {
  return (
    <Card as="section" className={className} data-pending="">
      <CardHeader>
        <CardTitle as={titleAs} className="text-ink-500">
          {title ?? info.ten}
        </CardTitle>
        <PendingMarker info={info} phase2={phase2} side={side} />
      </CardHeader>
      <CardContent className="text-sm text-ink-500">{children ?? PENDING_HOVER_TEXT}</CardContent>
    </Card>
  );
}

/**
 * A form / filter field: label above (spec §6.4), a DISABLED native control, "?" beside the label.
 * Not drawn through `Field`: the "?" would sit inside the `<label>` and its sentence would become part
 * of the control's accessible name.
 */
export function PendingField({
  info,
  phase2,
  side,
  id,
  label,
  kind = "input",
  placeholder,
  className,
}: CommonProps & {
  /** `id` of the disabled control — unique on the page, as for any `<label htmlFor>`. */
  id: string;
  label?: ReactNode;
  /**
   * `file`: a disabled native file input with NO `name` — even with `disabled` removed by hand it
   * would contribute nothing to a submitted form, so a placeholder can never upload a file.
   */
  kind?: "input" | "select" | "textarea" | "file";
  placeholder?: string;
  className?: string;
}) {
  const control = cn(controlClass, kind === "textarea" && "h-auto min-h-20 py-2");
  return (
    <div className={cn("flex min-w-[180px] flex-[0_1_220px] flex-col gap-1.5", className)} data-pending="">
      <div className="flex items-center gap-1.5">
        <label htmlFor={id} className="text-xs leading-tight font-semibold text-ink-500">
          {label ?? info.ten}
        </label>
        <PendingMarker info={info} phase2={phase2} side={side} />
      </div>
      {kind === "select" ? (
        <select id={id} disabled className={control}>
          <option>{placeholder ?? ""}</option>
        </select>
      ) : kind === "textarea" ? (
        <textarea id={id} disabled placeholder={placeholder} className={control} />
      ) : kind === "file" ? (
        // Not `controlClass`: a native file input draws its own button, and a 40px bordered box
        // around it reads as a text box.
        <input id={id} type="file" disabled className="max-w-full cursor-not-allowed text-sm text-ink-500 opacity-60" />
      ) : (
        <input id={id} type="text" disabled placeholder={placeholder} className={control} />
      )}
    </div>
  );
}
