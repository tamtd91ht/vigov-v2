/**
 * The owner's Giải ngân spec blocks as class strings (spec 00 §3–§5, ADR 0068 lần 6), shared by the
 * screens of this feature so a panel, a select or a table row is written once.
 *
 * TWO DIFFERENCES FROM THE SPEC TEXT, BOTH DELIBERATE:
 *   - The spec's `bg-surface` (page grey) is this app's `bg-canvas`; our `bg-surface` is white.
 *   - This app runs WITHOUT Tailwind preflight (`globals.css`), so headings, paragraphs and lists keep
 *     the browser's margins and a `<th>` its bold centred text unless a class says otherwise. The
 *     strings below add `m-0`, `text-left`, `font-medium` where the prototype relied on preflight —
 *     without them the same classes render a different picture.
 */

/** Section card (spec 00 §4 "Card / Section"). */
export const PANEL_CLASS = "border-line shadow-card rounded-card border border-solid bg-white p-4";

/** Its `<h2>` (spec 00 §3 "Tiêu đề section"). */
export const PANEL_TITLE_CLASS = "text-navy m-0 mb-3 text-[13px] font-bold";

/** Native select of the page (spec 00 §4 "Select native", prototype `SELECT_CLASS`). */
export const SELECT_CLASS =
  "border-line focus-visible:ring-ring/50 h-9 rounded-md border border-solid bg-white px-3 text-[12.5px] text-ink [font-family:inherit] outline-none focus-visible:ring-[3px]";

/** Native checkbox + its label (spec 00 §4 "Checkbox native"). */
export const CHECKBOX_CLASS = "accent-brand m-0 size-3.5";
export const CHECKBOX_LABEL_CLASS = "text-ink flex items-center gap-2 text-[12.5px]";

/** shadcn `Table` defaults (spec 00 §5 "Table"), the prototype's `components/ui/table.tsx`. */
export const TABLE_CLASS = "w-full caption-bottom border-collapse text-sm";
export const TH_CLASS = "h-10 px-2 text-left align-middle font-medium whitespace-nowrap text-foreground";
export const TR_CLASS = "border-line border-b hover:bg-muted/50";
export const TD_CLASS = "p-2 align-middle whitespace-nowrap";

/** Header of a secondary table inside a dialog or a tab (spec 00 §3 "Header cột bảng phụ"). */
export const SUB_TABLE_HEAD_ROW_CLASS = "text-ink-muted border-line border-b text-left text-[10.5px] uppercase";

/** Progress track (spec 00 §4 "Thanh tiến độ"). */
export const TRACK_CLASS = "bg-[#EDF0F3]";
