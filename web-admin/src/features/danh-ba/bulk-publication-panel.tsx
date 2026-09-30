"use client";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  BULK_CLOSE,
  BULK_CONSENT_LABEL,
  BULK_INTRO,
  BULK_NO_CANDIDATES,
  BULK_REPLAYED,
  BULK_RESULT_TITLE,
  BULK_SELECT_LABEL,
  BULK_SOURCE_NOTE,
  BULK_SUBMIT,
  BULK_TITLE,
  bulkConsentAria,
  bulkCountText,
  bulkListRows,
  bulkSelectAria,
  bulkSummaryText,
  type BulkResultLine,
  type BulkSelection,
} from "./bulk-publication";
import { CANH_BAO_CONG_KHAI } from "./cong-khai";

/** What the last submit produced — shown until the panel is closed or the selection changes. */
export type BulkOutcome =
  | { readonly kind: "lines"; readonly lines: readonly BulkResultLine[] }
  | { readonly kind: "replayed" };

/**
 * The "Công khai nhiều người" panel (user decision 30/09/2026).
 *
 * PRESENTATION ONLY, same pattern as `HopCongKhai`: values in through props, changes out through
 * callbacks, the network call at the call site.
 *
 * A LIST, NOT A TABLE: at the smallest width (320px) the register table scrolls sideways, and a
 * tick box scrolled out of view is a tick nobody sees. Each row wraps: name and position, then its
 * two boxes. The consent box exists only once the row is selected, and starts unticked.
 *
 * NO PHONE NUMBER IS DRAWN HERE — not in the list, not in the results. Name, position and staff
 * code are enough to tell people apart; the numbers are in the register right below.
 */
export function BulkPublicationPanel({
  selection,
  pageRows,
  onSelect,
  onUnselect,
  onSetConsent,
  error,
  sending,
  outcome,
  onSubmit,
  onClose,
}: {
  selection: BulkSelection;
  pageRows: readonly identity_canBoTomTat[];
  onSelect: (cb: identity_canBoTomTat) => void;
  onUnselect: (id: string) => void;
  onSetConsent: (id: string, consentAsked: boolean) => void;
  error: string;
  sending: boolean;
  outcome: BulkOutcome | null;
  onSubmit: () => void;
  onClose: () => void;
}) {
  const rows = bulkListRows(selection, pageRows);
  const consented = selection.filter((s) => s.consentAsked).length;

  return (
    <form
      className="form-danh-muc"
      aria-labelledby="bulk-publication-title"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit();
      }}
    >
      <h4 id="bulk-publication-title">{BULK_TITLE}</h4>

      <p>{BULK_INTRO}</p>
      <p className="canh-bao-pham-vi">{CANH_BAO_CONG_KHAI}</p>
      <p className="ghi-chu">{BULK_SOURCE_NOTE}</p>

      {rows.length === 0 ? (
        <p className="trang-thai-rong">{BULK_NO_CANDIDATES}</p>
      ) : (
        <ul>
          {rows.map(({ selected, candidate }) => {
            const id = selected?.id ?? candidate?.id ?? "";
            const name = selected?.fullName ?? candidate?.full_name ?? "";
            const code = selected?.code ?? candidate?.code ?? "";
            return (
              <li key={id}>
                <div className="o-nhap">
                  <label htmlFor={`bulk-select-${id}`}>
                    <input
                      id={`bulk-select-${id}`}
                      type="checkbox"
                      checked={selected !== null}
                      aria-label={bulkSelectAria(name)}
                      disabled={sending}
                      onChange={(e) => {
                        if (!e.target.checked) onUnselect(id);
                        else if (candidate !== null) onSelect(candidate);
                      }}
                    />{" "}
                    {BULK_SELECT_LABEL}: <strong>{name}</strong> ({code})
                    {candidate !== null && candidate.position !== "" && <> — {candidate.position}</>}
                  </label>
                </div>
                {selected !== null && (
                  <div className="o-nhap">
                    <label htmlFor={`bulk-consent-${id}`}>
                      <input
                        id={`bulk-consent-${id}`}
                        type="checkbox"
                        checked={selected.consentAsked}
                        aria-label={bulkConsentAria(name)}
                        disabled={sending}
                        onChange={(e) => onSetConsent(id, e.target.checked)}
                      />{" "}
                      {BULK_CONSENT_LABEL}
                    </label>
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}

      <p role="status">{bulkCountText(selection.length, consented)}</p>

      {/* A local refusal, or the server's sentence verbatim (400 `invalid_request`, 403…). */}
      {error !== "" && (
        <p className="thong-bao-loi" role="alert">
          {error}
        </p>
      )}

      {outcome !== null && <BulkOutcomeView outcome={outcome} />}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={sending || consented === 0}>
          {BULK_SUBMIT}
        </button>
        <button type="button" className="nut-phu" onClick={onClose} disabled={sending}>
          {BULK_CLOSE}
        </button>
      </div>
    </form>
  );
}

export function BulkOutcomeView({ outcome }: { outcome: BulkOutcome }) {
  if (outcome.kind === "replayed") return <p role="status">{BULK_REPLAYED}</p>;
  return (
    <section aria-labelledby="bulk-publication-result-title">
      <h5 id="bulk-publication-result-title">{BULK_RESULT_TITLE}</h5>
      <p role="status">{bulkSummaryText(outcome.lines)}</p>
      <ul>
        {outcome.lines.map((l) => (
          <li key={l.id}>
            {l.who}: <strong>{l.text}</strong>
          </li>
        ))}
      </ul>
    </section>
  );
}
