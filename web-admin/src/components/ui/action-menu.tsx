"use client";

import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { Ellipsis, type LucideIcon } from "lucide-react";

import { cn } from "@/lib/cn";

import { IconButton } from "./icon-button";

/**
 * Row action menu "⋯" — spec v2 §7: an `Ellipsis` icon button opening a list where EVERY item is
 * icon + text (important actions — publish, delete — are never icon-only), a separator before the
 * destructive item, the destructive item red and last. Arrow keys move, `Esc` closes, focus returns
 * to the trigger: all from Radix, none re-implemented here.
 *
 * DATA, NOT CHILDREN: the items are a plain array prop. A screen's test can therefore read which
 * actions a row offers (and that a denied permission removed one) from the unrendered element tree,
 * without opening a menu — the closed menu renders no items to the HTML.
 *
 * Each item calls the screen's EXISTING handler through `onSelect`; nothing here decides anything.
 *
 * `modal={false}`: a modal Radix menu locks page scroll through `react-remove-scroll`, which injects
 * a `<style>` element at runtime — the pattern ADR 0068 §4 keeps out (future strict CSP). A row
 * menu has no reason to lock the page anyway.
 *
 * USES HOOKS (inside Radix): never render it from a component that the mocked-React test runners
 * call as a plain function (`danh-ba-lien-he.luong.test.tsx`); render it from a child component
 * those runners only read props of.
 */
export type ActionMenuItem =
  | {
      readonly kind: "item";
      /** Stable React key. */
      readonly id: string;
      readonly label: string;
      readonly icon: LucideIcon;
      readonly onSelect: () => void;
      readonly tone?: "danger";
      /**
       * The action exists for this row but cannot run now (e.g. `Cấp tài khoản` without an email): the
       * item stays in the list, greyed, so the officer sees it and `hint` says why. Radix skips it on
       * arrow keys and never fires `onSelect`. UX only — the server still refuses the call.
       */
      readonly disabled?: boolean;
      /** A second, muted line under the label — the reason a disabled item cannot be chosen. */
      readonly hint?: string;
    }
  | { readonly kind: "separator"; readonly id: string };

export type ActionMenuProps = {
  /** Accessible name AND tooltip of the trigger, e.g. "Thao tác khác: Nguyễn Văn A". */
  label: string;
  items: readonly ActionMenuItem[];
  align?: "start" | "end";
  className?: string;
  /**
   * The trigger's look. Default `icon` (borderless). `secondary` is the outline square that sits among
   * a row's other `sm` outline buttons (`/nguoi-dung`), so the three read as one group.
   */
  triggerVariant?: "icon" | "secondary";
};

const ITEM = cn(
  "flex min-h-9 cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-ink-700 outline-none select-none",
  "data-[highlighted]:bg-brand-50 data-[highlighted]:text-brand-700",
  "data-[disabled]:cursor-not-allowed data-[disabled]:opacity-60",
  "[&_svg]:size-4 [&_svg]:shrink-0",
);

const ITEM_DANGER = "text-danger-600 data-[highlighted]:bg-danger-50 data-[highlighted]:text-danger-600";

export function ActionMenu({ label, items, align = "end", className, triggerVariant = "icon" }: ActionMenuProps) {
  return (
    <DropdownMenu.Root modal={false}>
      <DropdownMenu.Trigger asChild>
        <IconButton type="button" label={label} variant={triggerVariant} className={className}>
          <Ellipsis aria-hidden="true" />
        </IconButton>
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align={align}
          sideOffset={4}
          collisionPadding={8}
          className={cn(
            "z-[60] min-w-[14rem] max-w-[calc(100vw-1rem)] rounded-xl border border-line bg-surface p-1 shadow-lg",
            "origin-(--radix-dropdown-menu-content-transform-origin) animate-[menu-in_160ms_ease-out]",
          )}
        >
          {items.map((it) =>
            it.kind === "separator" ? (
              <DropdownMenu.Separator key={it.id} className="my-1 h-px bg-line" />
            ) : (
              <DropdownMenu.Item
                key={it.id}
                onSelect={it.onSelect}
                disabled={it.disabled}
                className={cn(ITEM, it.tone === "danger" && ITEM_DANGER)}
              >
                <it.icon aria-hidden="true" focusable="false" strokeWidth={1.8} />
                {it.hint === undefined ? (
                  it.label
                ) : (
                  <span className="flex min-w-0 flex-col">
                    <span>{it.label}</span>
                    <span className="text-ink-muted text-xs font-normal whitespace-normal">{it.hint}</span>
                  </span>
                )}
              </DropdownMenu.Item>
            ),
          )}
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  );
}
