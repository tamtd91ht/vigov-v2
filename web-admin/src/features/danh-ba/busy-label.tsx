import { LoaderCircle } from "lucide-react";

/** Busy words of the three submit buttons of this screen (spec v2 §8b "Nút đang xử lý"). */
export const BUSY_SAVING = "Đang lưu…";
export const BUSY_DELETING = "Đang xoá…";
export const BUSY_PUBLISHING = "Đang công khai…";

/**
 * A submit button's content: its label, or — while the request runs — a spinner and the busy words.
 *
 * THE BUTTON KEEPS ITS WIDTH: while busy, the label stays in the same grid cell, `invisible`, under
 * the busy words, so the button is as wide as before and nothing beside it jumps. `invisible` also
 * drops the label from the accessible name, so a screen reader hears only "Đang lưu…".
 *
 * NOT BUSY → THE BARE LABEL STRING, no wrapper: the markup is exactly what it was before this
 * existed (`<button …>Công khai lên Mini App</button>`).
 *
 * No hooks: the caller still owns `disabled` and `aria-busy`; this only draws.
 */
export function BusyLabel({ busy, label, busyText }: { busy: boolean; label: string; busyText: string }) {
  if (!busy) return label;
  return (
    <span className="grid [&>*]:col-start-1 [&>*]:row-start-1">
      <span className="invisible">{label}</span>
      <span className="inline-flex items-center justify-center gap-2">
        <LoaderCircle aria-hidden="true" focusable="false" className="size-4 shrink-0 animate-spin" />
        {busyText}
      </span>
    </span>
  );
}
