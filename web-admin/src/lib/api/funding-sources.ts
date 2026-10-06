/**
 * The four funding-source routes of the Giải ngân screen (`docs/ui-ux/06-giai-ngan.md` §6, migration
 * 0013) — exactly what `service-finance/internal/http/routes.go` declares:
 *
 *   GET  /api/v1/funding-sources?year=                         budget.read
 *   POST /api/v1/funding-sources                               budget.update + Idempotency-Key REQUIRED
 *   PUT  /api/v1/funding-sources/{id}/annual-amounts/{year}    budget.update
 *   GET  /api/v1/funding-sources/{id}/projects?year=           budget.read
 *
 * TYPES COME FROM THE CONTRACT (`schema.gen.ts`), never hand-copied (rule 2, invariant 7).
 *
 * EVERY FIGURE IS THE SERVER'S. Allocated, disbursed, the three ratios and the unattributed amount are
 * summed and divided in `service-finance`; nothing here or on the screen adds them up again — a second
 * sum in the browser is a second answer that drifts from the first the day a voucher state changes.
 *
 * NO DELETE, NO RENAME: a source is declared once and serves every year (decided 06/10/2026). There is
 * no route for either, so there is no function for either.
 *
 * NO `tenant_id` anywhere: the commune comes from `Host` at the edge (rule 1, forbidden #2).
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./goi"; // vi-name-ok: existing shared call helpers of goi.ts, not renamed (rule 12 invariant 3)
import type {
  finance_fundingSourceCreateIn,
  finance_fundingSourceOut,
  finance_fundingSourceProjectsOut,
  finance_fundingSourcesOut,
  finance_get_funding_sources,
  finance_get_funding_sources_by_id_projects,
  finance_grantedAmountIn,
  finance_grantedAmountOut,
  finance_post_funding_sources,
  finance_put_funding_sources_by_id_annual_amounts_by_year,
} from "./schema.gen";

/**
 * `?year=` for both read routes. REQUIRED here although the contract marks it optional: the handler
 * answers 400 without it and never defaults a year (`funding_sources.go`, `yearParam`) — a default
 * would decide which year's money is reported, invisibly.
 */
function yearQuery(year: number): string {
  const query = new URLSearchParams();
  const key: keyof finance_get_funding_sources["truyVan"] = "year";
  query.set(key, String(year));
  return query.toString();
}

/** Path of the year's cards. Split from the call so it is testable without a fake `fetch`. */
export function fundingSourcesPath(year: number): string {
  const path: finance_get_funding_sources["duongDan"] = "/api/v1/funding-sources";
  return `${path}?${yearQuery(year)}`;
}

/** GET /api/v1/funding-sources?year= — one card per source of the commune, plus §13 rule 6's amount. */
export function listFundingSources(year: number): Promise<KetQua<finance_fundingSourcesOut>> {
  return docJSON<finance_fundingSourcesOut>(fundingSourcesPath(year));
}

/** Path of the projects behind one card. `id` is encoded: it goes into the PATH. */
export function fundingSourceProjectsPath(id: string, year: number): string {
  const template: finance_get_funding_sources_by_id_projects["duongDan"] = "/api/v1/funding-sources/{id}/projects";
  return `${template.replace("{id}", encodeURIComponent(id))}?${yearQuery(year)}`;
}

/**
 * GET /api/v1/funding-sources/{id}/projects?year= — this source's share of each project of the year.
 * A source of another commune answers the same 404 as one that does not exist.
 */
export function listFundingSourceProjects(
  id: string,
  year: number,
): Promise<KetQua<finance_fundingSourceProjectsOut>> {
  return docJSON<finance_fundingSourceProjectsOut>(fundingSourceProjectsPath(id, year));
}

/**
 * POST /api/v1/funding-sources — add a source, with this year's granted amount when the clerk typed
 * one. 201 with the source as a card for `year`.
 *
 * `granted_amount` ABSENT means "not entered yet"; `0` means "granted nothing", recorded. The two are
 * different facts to the server (`fundingSourceCreateIn`), so a blank box must never become `0`.
 *
 * `Idempotency-Key` REQUIRED and a PARAMETER, created when the form opens and kept across a retry: a
 * key made here would be new on every retry and protect nothing. 409 `funding_source_name_taken` /
 * `funding_source_catalogue_full` come back as the server's Vietnamese sentence, verbatim.
 */
export function createFundingSource(
  body: finance_fundingSourceCreateIn,
  idempotencyKey: string,
): Promise<KetQua<finance_fundingSourceOut>> {
  const path: finance_post_funding_sources["duongDan"] = "/api/v1/funding-sources";
  // Field by field, never a spread: a spread is how a stray field reaches the server.
  const sent: finance_fundingSourceCreateIn = { name: body.name, year: body.year };
  if (body.granted_amount !== undefined && body.granted_amount !== null) sent.granted_amount = body.granted_amount;

  return docThanLoiGoi<finance_fundingSourceOut>(
    goiGhi(path, "POST", sent, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/**
 * PUT /api/v1/funding-sources/{id}/annual-amounts/{year} — record or correct one year's granted
 * amount. 200. The amount is REQUIRED (absent is 400, never read as 0). Setting the same figure twice
 * yields the same row, so the contract asks for no idempotency key.
 */
export function setGrantedAmount(
  id: string,
  year: number,
  grantedAmount: number,
): Promise<KetQua<finance_grantedAmountOut>> {
  const template: finance_put_funding_sources_by_id_annual_amounts_by_year["duongDan"] =
    "/api/v1/funding-sources/{id}/annual-amounts/{year}";
  const path = template.replace("{id}", encodeURIComponent(id)).replace("{year}", String(year));
  const sent: finance_grantedAmountIn = { granted_amount: grantedAmount };

  return docThanLoiGoi<finance_grantedAmountOut>(goiGhi(path, "PUT", sent, 200));
}
