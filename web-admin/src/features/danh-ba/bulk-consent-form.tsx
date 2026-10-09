import { Button } from "@/components/ui/button";

import {
  BULK_CLOSE,
  BULK_CONSENT_LABEL,
  BULK_INTRO,
  BULK_NO_CANDIDATES,
  BULK_REPLAYED,
  BULK_RESULT_TITLE,
  BULK_SUBMIT,
  bulkConsentAria,
  bulkCountText,
  bulkSummaryText,
  type BulkResultLine,
  type BulkSelection,
} from "./bulk-publication";
import { BUSY_PUBLISHING, BusyLabel } from "./busy-label";
import { CANH_BAO_CONG_KHAI } from "./cong-khai";
import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";

/** What the last submit produced, when somebody was skipped — shown until the dialog closes. */
export type BulkOutcome =
  | { readonly kind: "lines"; readonly lines: readonly BulkResultLine[] }
  | { readonly kind: "replayed" };

/**
 * The consent step behind the selection bar's "Thêm vào danh bạ Mini App" (owner decision 09/10/2026,
 * keeping #12): every selected person who can be published, each with their OWN "Đã hỏi ý người
 * này" box, unticked. Nothing is published from the table itself.
 *
 * PRESENTATION ONLY, same pattern as `HopCongKhai`: values in through props, changes out through
 * callbacks, the network call at the call site — so `danh-ba-lien-he.luong.test.tsx` reads the props
 * the screen hands it, and this renders under `react-dom/server` with no hook.
 *
 * NO PHONE NUMBER IS DRAWN HERE — name and staff code tell people apart; the numbers are in the table.
 */
export function BulkConsentForm({
  selection,
  onSetConsent,
  error,
  sending,
  outcome,
  onSubmit,
  onClose,
}: {
  selection: BulkSelection;
  onSetConsent: (id: string, consentAsked: boolean) => void;
  error: string;
  sending: boolean;
  outcome: BulkOutcome | null;
  onSubmit: () => void;
  onClose: () => void;
}) {
  const consented = selection.filter((s) => s.consentAsked).length;
  const done = outcome !== null;

  return (
    <form
      className="m-0 flex min-w-0 flex-col gap-3"
      aria-label={BULK_SUBMIT}
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      {!done && selection.length > 0 && (
        <>
          <p className="m-0 text-[13px] text-ink-700">{BULK_INTRO}</p>
          <p className="canh-bao-pham-vi m-0">{CANH_BAO_CONG_KHAI}</p>
          <ul className="border-line m-0 max-h-60 list-none space-y-1.5 overflow-y-auto rounded-md border border-solid p-3 text-[12.5px]">
            {selection.map((s) => (
              <li key={s.id} className="flex flex-wrap items-center gap-x-3 gap-y-1">
                <span className="text-navy min-w-0 flex-1 font-medium">
                  {s.fullName} <span className="text-ink-muted font-normal">({s.code})</span>
                </span>
                <label htmlFor={`bulk-consent-${s.id}`} className="m-0 flex cursor-pointer items-center gap-2">
                  <input
                    id={`bulk-consent-${s.id}`}
                    type="checkbox"
                    checked={s.consentAsked}
                    aria-label={bulkConsentAria(s.fullName)}
                    disabled={sending}
                    onChange={(e) => onSetConsent(s.id, e.target.checked)}
                    className="accent-brand m-0 size-3.5"
                  />
                  {BULK_CONSENT_LABEL}
                </label>
              </li>
            ))}
          </ul>
          <p role="status" className="text-ink-muted m-0 text-[12px]">
            {bulkCountText(selection.length, consented)}
          </p>
        </>
      )}

      {!done && selection.length === 0 && <p className="m-0">{BULK_NO_CANDIDATES}</p>}

      {/* A local refusal, or the server's sentence verbatim (400 `invalid_request`, 403…). */}
      {error !== "" && (
        <p className="text-danger m-0 text-[12px] font-medium" role="alert">
          {error}
        </p>
      )}

      {outcome !== null && <BulkOutcomeView outcome={outcome} />}

      <div className="flex flex-wrap justify-end gap-2 pt-1">
        <Button type="button" variant="outline" onClick={onClose} disabled={sending}>
          {done ? BULK_CLOSE : NUT_HUY}
        </Button>
        {!done && selection.length > 0 && (
          <Button type="submit" variant="primary" disabled={sending || consented === 0} aria-busy={sending}>
            <BusyLabel busy={sending} label={BULK_SUBMIT} busyText={BUSY_PUBLISHING} />
          </Button>
        )}
      </div>
    </form>
  );
}

export function BulkOutcomeView({ outcome }: { outcome: BulkOutcome }) {
  if (outcome.kind === "replayed") return <p role="status">{BULK_REPLAYED}</p>;
  return (
    <section aria-labelledby="bulk-publication-result-title">
      <h5 id="bulk-publication-result-title" className="m-0 text-[13px] font-semibold">
        {BULK_RESULT_TITLE}
      </h5>
      <p role="status" className="m-0 mt-1">
        {bulkSummaryText(outcome.lines)}
      </p>
      <ul className="m-0 mt-1 pl-5">
        {outcome.lines.map((l) => (
          <li key={l.id}>
            {l.who}: <strong>{l.text}</strong>
          </li>
        ))}
      </ul>
    </section>
  );
}
