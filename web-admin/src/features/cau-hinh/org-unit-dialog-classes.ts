/**
 * The two pieces of shadcn's Dialog that `ModalDialog` does not draw, shared by the org chart's
 * add/edit dialog and its delete dialog (spec 03).
 *
 * PREFLIGHT IS OFF in this app (globals.css), so a border utility must name its style and zero the
 * other sides itself: `border-t` alone draws nothing, and `border-solid` alone draws the UA's
 * `medium` width on all four sides.
 */

/** shadcn `DialogFooter`: hairline on top, muted fill, flush with the dialog's bottom edge. */
export const DIALOG_FOOTER_CLASS =
  "border-border bg-muted/50 -mx-4 -mb-4 mt-2 flex shrink-0 flex-wrap justify-end gap-2 rounded-b-xl border-0 border-t border-solid p-4";

/**
 * shadcn `Label`. Its wrapper `div` carries `block`: the legacy `:has(> label + select)` rule would
 * otherwise turn it into a flex column with a 12px/600 label.
 */
export const DIALOG_LABEL_CLASS =
  "text-foreground m-0 flex items-center gap-2 text-sm leading-none font-medium select-none";
