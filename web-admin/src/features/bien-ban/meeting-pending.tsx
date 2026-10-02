import { CalendarClock } from "lucide-react";

import { PendingMarker, type PendingFeatureInfo } from "@/components/ui/pending-feature";

import { PHAN_CHUA_DUNG } from "./nhan-bien-ban";

/**
 * The parts of `docs/ui-ux/04-bien-ban-hop.md` the Biên bản screen cannot build yet, drawn at their
 * spec position as the control they will be, disabled, with a "?" (ADR 0068 §14). NOTHING HERE
 * CALLS A SERVER OR STORES ANYTHING: no handler, no `name`, no form state.
 *
 * Descriptions come from `PHAN_CHUA_DUNG` by the start of their name, never a second sentence
 * written here — two copies of one reason drift, and the stale one is what staff read.
 */
function part(prefix: string): PendingFeatureInfo {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten.startsWith(prefix));
  // A renamed entry must fail loudly in tests, not draw a "?" that explains nothing.
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry starting "${prefix}"`);
  return found;
}

/**
 * Spec §4 last field, "Tệp đính kèm — bản scan biên bản". `PendingField` has no file kind, so it is
 * drawn here in the same shape: label above, "?" beside the label (NEVER inside it — its sentence
 * would join the control's accessible name), a disabled native file input below.
 */
export function ScanAttachmentField() {
  return (
    <div className="flex min-w-0 flex-col gap-1.5" data-pending="">
      <div className="flex items-center gap-1.5">
        <label htmlFor="tep-dinh-kem-bien-ban" className="text-xs leading-tight font-semibold text-ink-500">
          Tệp đính kèm
        </label>
        <PendingMarker info={part("Tệp đính kèm")} />
      </div>
      <input
        id="tep-dinh-kem-bien-ban"
        type="file"
        disabled
        className="max-w-full cursor-not-allowed text-sm text-ink-500 opacity-60"
      />
    </div>
  );
}

/**
 * Spec §3 "Hạn hoàn thành — gợi ý từ ngày nêu trong kết luận": a disabled hint line with a "?".
 * It suggests nothing: the server refuses to guess a date on purpose (see the entry's reason), and
 * a client-side guesser would rebuild exactly that, where nobody can check it.
 */
export function SuggestedDeadlineHint() {
  return (
    <p className="m-0 inline-flex items-center gap-1.5 text-[13px] text-ink-400" aria-disabled="true" data-pending="">
      <CalendarClock aria-hidden="true" focusable="false" className="size-3.5 shrink-0" />
      Hạn gợi ý từ ngày nêu trong kết luận
      <PendingMarker info={part("HẠN GỢI Ý")} />
    </p>
  );
}
