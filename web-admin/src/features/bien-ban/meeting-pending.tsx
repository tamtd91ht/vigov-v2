import { CalendarClock } from "lucide-react";

import { PendingField, PendingMarker, type PendingFeatureInfo } from "@/components/ui/pending-feature";

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
 * Spec §4 last field, "Tệp đính kèm — bản scan biên bản": `PendingField`'s file kind — label above,
 * "?" beside the label, a disabled native file input with no `name` below.
 */
export function ScanAttachmentField() {
  return (
    <PendingField
      info={part("Tệp đính kèm")}
      id="tep-dinh-kem-bien-ban"
      label="Tệp đính kèm"
      kind="file"
      className="min-w-0 flex-none"
    />
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
      <PendingMarker info={part("Hạn gợi ý")} />
    </p>
  );
}
