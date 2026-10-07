/**
 * The colour tier of a disbursement ratio — the list's progress bar and the category table's `Tỷ lệ`.
 *
 * USER DECISION 07/10/2026: the screen colours ratios as the prototype does, with ITS thresholds
 * (`vigov-require/apps/admin/src/lib/budget-display.ts:62-67`, `disbursementColor`): ≥ 80% green,
 * ≥ 50% brand, ≥ 30% amber, below red. These are presentation tiers, not the late rule: whether a
 * project is LATE stays the server's `is_delayed` (red left edge, "Chậm x điểm"), from the commune's
 * own threshold. Changing a figure here recolours bars; it never relabels a project late.
 *
 * ONE constant, so a second screen that needs the tiers imports these numbers instead of copying them.
 */
export const PROGRESS_TONE_THRESHOLDS = { success: 80, brand: 50, warning: 30 } as const;

export type ProgressTone = "success" | "brand" | "warning" | "danger";

/** `hundredths` is the server's ratio unit (hundredths of a percent: 8000 ⇒ 80%). */
export function progressTone(hundredths: number): ProgressTone {
  const percent = hundredths / 100;
  if (percent >= PROGRESS_TONE_THRESHOLDS.success) return "success";
  if (percent >= PROGRESS_TONE_THRESHOLDS.brand) return "brand";
  if (percent >= PROGRESS_TONE_THRESHOLDS.warning) return "warning";
  return "danger";
}

/** Bar fill per tier. Literal class names so Tailwind finds them. */
export const PROGRESS_BAR_CLASS: Record<ProgressTone, string> = {
  success: "bg-success-500",
  brand: "bg-brand-500",
  warning: "bg-warning-500",
  danger: "bg-danger-500",
};

/** Figure colour per tier — the 600 shades, readable as text on white. */
export const PROGRESS_TEXT_CLASS: Record<ProgressTone, string> = {
  success: "text-success-600",
  brand: "text-brand-600",
  warning: "text-warning-600",
  danger: "text-danger-600",
};
