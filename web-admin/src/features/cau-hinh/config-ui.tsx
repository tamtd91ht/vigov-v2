import type { ComponentProps, ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { DATA_TABLE_CLASS } from "@/components/ui/data-table";
import { controlClass } from "@/components/ui/field";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";

/**
 * The pieces every table-shaped tab of Cấu hình repeats — spec `02-khung-trang.md` "Mẫu dùng chung
 * cho các tab dạng bảng" (owner, 08/10/2026, ADR 0079): a grey add/edit row ABOVE the table (never a
 * modal), the table in a hairline frame, status chips, a right-aligned row of small outline actions.
 * Only what that list names lives here; a tab's own columns and words stay in the tab.
 *
 * TOKEN TRAP: the spec's `bg-surface` is the PAGE colour (#f4f8fb). In this app `bg-surface` is the
 * WHITE card and the page colour is `bg-background` (`globals.css`), so the spec's grey row is written
 * `bg-background` here.
 *
 * LEGACY RULES STILL REACH THIS MARKUP where no utility sets the same property (`globals.css` keeps
 * them in `@layer legacy`): `ui/button.tsx` always emits `nut-phu` / `nut-chinh`, whose
 * `min-height: var(--control-h)` (32px) turned the spec's 28px `size="sm"` button into 32px — hence
 * `SMALL_BUTTON_CLASS`; and `:where(p, div):has(> label + select)` makes any label+select wrapper a
 * flex column — hence `ConfigField` sets its own `display`. No class name of the legacy sheet is used
 * here, so none of its contextual 44px rules (`.o-thao-tac .nut-phu`, `.form-danh-muc …`) applies.
 */

/** Put on every `size="sm"` Button outside `RowActions`: undoes the legacy 32px `min-height`. */
export const SMALL_BUTTON_CLASS = "min-h-0";

/**
 * The spec's native select (spec 00 §4). The legacy sheet also draws a chevron as a background image
 * on every select and floors its width at 12rem: `formSelectCls` keeps the chevron clear of the text
 * (`pr-8`) and lets the select fit its grid cell (`min-w-0`).
 */
export const selectCls =
  "border-line focus-visible:ring-ring/50 h-9 rounded-md border bg-white px-3 text-[12.5px] outline-none focus-visible:ring-[3px]";

/** A select inside a `ConfigField` of a `ConfigFormRow`. */
export const formSelectCls = cn(selectCls, "mt-1 w-full min-w-0 pr-8");

/** A text / date / time input inside a `ConfigField` (spec: `mt-1 h-9 text-[12.5px]` over the Input frame). */
export const formInputCls = cn(controlClass, "mt-1 h-9 text-[12.5px]");

/**
 * The grey add/edit row. A `<form>`, so Enter in any field submits it (the spec's "Enter trong ô chính
 * để submit"). `columns` is the grid template FROM `sm:` up (e.g. `"sm:grid-cols-[10rem_1fr_auto]"`):
 * below 640px every field takes a full row, so 320px never scrolls sideways.
 *
 * NO MARGIN UTILITY HERE, ON PURPOSE. Tailwind v4 emits the parent's `space-y-*` under `:where()` (zero
 * specificity), so an `m-0` on this row beat it and the row touched the table below (VALIDATE round 2,
 * 08/10/2026). There was nothing for `m-0` to cancel: a `<form>` has no UA margin in standards mode and
 * the legacy sheet has no bare `form` rule. `config-ui.test.tsx` keeps it that way.
 */
export function ConfigFormRow({
  columns,
  className,
  ...props
}: ComponentProps<"form"> & { columns: string }) {
  return (
    <form
      className={cn(
        "border-line bg-background grid items-end gap-3 rounded-[10px] border border-solid p-3",
        columns,
        className,
      )}
      {...props}
    />
  );
}

/**
 * Label above one control (`text-[11.5px]`, shadcn Label `leading-none font-medium`). `block` beats the
 * legacy `:has(> label + select)` flex column on the wrapper; the label's own utilities beat that
 * rule's 12px/600 label. The control is passed as `children` with `formInputCls` / `formSelectCls`.
 */
export function ConfigField({
  label,
  htmlFor,
  className,
  children,
}: {
  label: ReactNode;
  htmlFor: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <div className={cn("block min-w-0", className)}>
      <label
        htmlFor={htmlFor}
        className="text-foreground m-0 flex items-center gap-2 text-[11.5px] leading-none font-medium select-none"
      >
        {label}
      </label>
      {children}
    </div>
  );
}

/**
 * The table frame: `border-line overflow-hidden rounded-[10px] border` around shadcn's Table. The cell
 * box (head `h-10 px-2 font-medium text-foreground`, cell `p-2 align-middle`, row hairline +
 * `hover:bg-muted/50`) is `DATA_TABLE_CLASS`'s, shared with every other table of the app; cells add
 * shadcn's `whitespace-nowrap`. The frame itself scrolls sideways, as a labelled focusable region: a
 * scroller a keyboard user cannot reach is a column they cannot read.
 */
export function ConfigTable({
  label,
  caption,
  children,
}: {
  /** Accessible name of the scroll region. */
  label: string;
  /** Visually hidden caption — what the rows are, for a screen reader. */
  caption?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div
      role="region"
      tabIndex={0}
      aria-label={label}
      className="border-line min-w-0 overflow-x-auto rounded-[10px] border border-solid"
    >
      <table className={cn(DATA_TABLE_CLASS, "w-full text-sm [&_td]:whitespace-nowrap")}>
        {caption !== undefined && <caption className="an-thi-giac">{caption}</caption>}
        {children}
      </table>
    </div>
  );
}

/**
 * "Đang dùng" / "Ngừng dùng" — the spec's two status chips. The shared Badge keeps its tone icon
 * (owner decision 08/10/2026): a status is never colour alone. ONE EXCEPTION, decided by the user on
 * 09/10/2026: the `/nguoi-dung` table draws its status and account chips text-only, like the prototype
 * (`Badge icon={null}`, `danh-ba-can-bo.tsx` `StatusCell`); every chip here keeps its icon.
 */
export function StatusBadge({ active }: { active: boolean }) {
  return active ? (
    <Badge tone="success">Đang dùng</Badge>
  ) : (
    <Badge tone="neutral" className="text-ink-muted">
      Ngừng dùng
    </Badge>
  );
}

/** The action cell's button row; its small buttons get `SMALL_BUTTON_CLASS` without each saying so. */
export function RowActions({ className, ...props }: ComponentProps<"div">) {
  return (
    <div className={cn("flex items-center justify-end gap-1.5 [&>button]:min-h-0", className)} {...props} />
  );
}

/**
 * First load of a tab or a table (spec 02 "Loading chung"): three 44px placeholder bars. `label` is
 * the live-region sentence — a skeleton is decorative and announces nothing by itself.
 */
export function ConfigLoading({ label }: { label?: string }) {
  return (
    <>
      {label !== undefined && (
        <p role="status" className="an-thi-giac">
          {label}
        </p>
      )}
      <div className="space-y-2" aria-hidden="true">
        <Skeleton className="bg-muted h-11 w-full rounded-md" />
        <Skeleton className="bg-muted h-11 w-full rounded-md" />
        <Skeleton className="bg-muted h-11 w-full rounded-md" />
      </div>
    </>
  );
}

/** The one full-width row of an empty table. The sentence may wrap: it is the only thing in the row. */
export function EmptyRow({ colSpan, children }: { colSpan: number; children: ReactNode }) {
  return (
    <tr>
      <td colSpan={colSpan} className="text-ink-muted py-10 text-center">
        <div className="whitespace-normal">{children}</div>
      </td>
    </tr>
  );
}
