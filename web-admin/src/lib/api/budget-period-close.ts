/**
 * Budget period close (`chốt kỳ ngân sách`) routes — `service-finance/internal/http/routes.go`,
 * the `budget-period-closes` block. Decided by the user 30/09/2026 (kb/00-foundation/
 * ubiquitous-language.md, rows "Chốt kỳ ngân sách" and "Mở chốt").
 *
 * TYPES COME FROM THE CONTRACT (`schema.gen.ts`), never hand-copied (rule 2, invariant 7).
 *
 * WHAT A CLOSE LOCKS IS DECIDED BY THE SERVER, inside the transaction of every guarded write, and
 * answered as 409 `budget_period_closed` with a Vietnamese sentence naming the period and the close
 * code. This file does not know those rules; the screen shows the server sentence verbatim.
 *
 * NO `tenant_id` anywhere: the commune comes from `Host` at the edge (rule 1, forbidden #2).
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./goi"; // vi-name-ok: existing shared call helpers of goi.ts, not renamed (rule 12 invariant 3)
import type {
  finance_budgetPeriodCloseIn,
  finance_budgetPeriodCloseOut,
  finance_budgetPeriodClosesOut,
  finance_budgetPeriodReopeningIn,
  finance_get_budget_period_closes,
  finance_post_budget_period_closes,
  finance_post_budget_period_closes_by_code_reopening,
} from "./schema.gen";

/** Read path for one year's close history. `year` is required by the handler — it never defaults. */
export function budgetPeriodClosesPath(year: number): string {
  const path: finance_get_budget_period_closes["duongDan"] = "/api/v1/budget-period-closes";
  const query = new URLSearchParams();
  const key: keyof finance_get_budget_period_closes["truyVan"] = "year";
  query.set(key, String(year));
  return `${path}?${query.toString()}`;
}

/** GET /api/v1/budget-period-closes?year=YYYY — active AND reopened closes of the year. */
export function listBudgetPeriodCloses(
  year: number,
): Promise<KetQua<finance_budgetPeriodClosesOut>> {
  return docJSON<finance_budgetPeriodClosesOut>(budgetPeriodClosesPath(year));
}

/**
 * POST /api/v1/budget-period-closes — close a month (`month` 1..12) or the whole year (`month`
 * absent). 201.
 *
 * `Idempotency-Key` REQUIRED (`idem.Required(idem.DongKhiHong)`): a close declares a commune's
 * figures final. The key is a PARAMETER, created when the form opens and kept across a retry after
 * failure — created here, every retry would be a new key and protect nothing.
 *
 * `month` is OMITTED for a year close rather than sent as `null`: absent is what the route
 * documents for "whole year", and one spelling leaves nothing to interpret.
 */
export function closeBudgetPeriod(
  body: finance_budgetPeriodCloseIn,
  idempotencyKey: string,
): Promise<KetQua<finance_budgetPeriodCloseOut>> {
  const path: finance_post_budget_period_closes["duongDan"] = "/api/v1/budget-period-closes";

  // Field by field, never a spread: a spread is how a stray field reaches the server the day
  // somebody passes a whole close object in.
  const sent: finance_budgetPeriodCloseIn = { year: body.year };
  if (body.month !== undefined && body.month !== null) sent.month = body.month;

  return docThanLoiGoi<finance_budgetPeriodCloseOut>(
    goiGhi(path, "POST", sent, 201, { "Idempotency-Key": idempotencyKey }),
  );
}

/**
 * POST /api/v1/budget-period-closes/{code}/reopening — reopen one close with a required reason.
 * 200 with the updated close. The close row is written once and never deleted.
 *
 * The reason travels in the BODY, never in the URL: free text about a public body's budget in a
 * URL stays in every access log and intermediate cache.
 */
export function reopenBudgetPeriodClose(
  code: string,
  reason: string,
  idempotencyKey: string,
): Promise<KetQua<finance_budgetPeriodCloseOut>> {
  const template: finance_post_budget_period_closes_by_code_reopening["duongDan"] =
    "/api/v1/budget-period-closes/{code}/reopening";
  const sent: finance_budgetPeriodReopeningIn = { reason };

  return docThanLoiGoi<finance_budgetPeriodCloseOut>(
    goiGhi(template.replace("{code}", encodeURIComponent(code)), "POST", sent, 200, {
      "Idempotency-Key": idempotencyKey,
    }),
  );
}
