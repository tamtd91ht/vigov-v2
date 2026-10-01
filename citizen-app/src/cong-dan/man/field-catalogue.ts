/**
 * THE COMMUNE'S FIELD CATALOGUE, READ THE SAME WAY BY BOTH SEND SCREENS — `GET /api/v1/my-citizen-report-fields`
 * (`api/goi-vigov.ts` `citizenReportFields`). The shared app (`GuiPhanAnhScreen.tsx`) and the commune's own app
 * (`PhanAnhAppXa.tsx`) draw it differently and say it in different voices ("bạn" / "bà con", ADR 0050 #6), but
 * what an answer MEANS is one table, here, so the two cannot drift on it.
 *
 * There is NO built-in list anywhere (ADR 0060 §3): a failed or empty catalogue is said in words, with "Thử lại"
 * where pressing it can help. Twelve names the commune may not use would be a petition routed to nobody.
 *
 * PURE: no React, no network, no words. Each screen maps the session-shaped answers to its own next step.
 */
import type { citizenReportFields } from "../api/goi-vigov";
import type { CitizenField } from "../api/hop-dong-phan-anh";

/** The failures both screens show on the field step itself. `closed` is the commune app's only. */
export type CatalogueFailure = "unavailable" | "network" | "server" | "closed";

/** The step's state. `F` lets a screen add a failure of its own without widening the other's switch. */
export type Catalogue<F extends string = CatalogueFailure> =
  | { readonly kind: "loading" }
  | { readonly kind: "failed"; readonly failure: F }
  | { readonly kind: "ready"; readonly fields: readonly CitizenField[] };

export type CatalogueAnswer = Awaited<ReturnType<typeof citizenReportFields>>;

/**
 * One catalogue answer, session-neutral. The session-shaped kinds are kept apart because the two screens do
 * different things with them: the commune app sends all three through its gate; the shared app closes the
 * channel, says the session expired, or asks for the phone.
 */
export type CatalogueReading =
  | { readonly kind: "ready"; readonly fields: readonly CitizenField[] }
  | { readonly kind: "failed"; readonly failure: "unavailable" | "network" | "server" }
  /** No ViGov session on this phone — nothing was sent. */
  | { readonly kind: "no-session" }
  /** No ViGov address in this build — nothing was sent. */
  | { readonly kind: "not-configured" }
  /** 503 other than `field_catalogue_unavailable` — the commune's intake is not set up. */
  | { readonly kind: "intake-closed" }
  /** 401 — the session cannot be used. */
  | { readonly kind: "expired" }
  /** 403 `chua_xac_thuc_so`. The route needs a session only, so this is not expected; it is still read. */
  | { readonly kind: "phone-required" };

export function readCatalogueAnswer(kq: CatalogueAnswer): CatalogueReading {
  switch (kq.kieu) {
    case "xong":
      return { kind: "ready", fields: kq.fields };
    case "chua-co-phien":
      return { kind: "no-session" };
    case "chua-cau-hinh":
      return { kind: "not-configured" };
    case "kenh-chua-mo":
      return { kind: "intake-closed" };
    case "het-phien":
      return { kind: "expired" };
    case "can-xac-thuc-so":
      return { kind: "phone-required" };
    case "field-catalogue-unavailable":
      return { kind: "failed", failure: "unavailable" };
    case "loi-mang":
      return { kind: "failed", failure: "network" };
    default:
      return { kind: "failed", failure: "server" };
  }
}

/** The codes the commune offers now, or `null` while not known (loading / failed). */
export function offeredCodes(c: Catalogue<string>): readonly string[] | null {
  return c.kind === "ready" ? c.fields.map((f) => f.code) : null;
}

/** The label of `code` in a loaded catalogue, or "" — never the code itself, which the citizen never reads. */
export function fieldLabelOf(c: Catalogue<string>, code: string): string {
  if (c.kind !== "ready" || code === "") return "";
  return c.fields.find((f) => f.code === code)?.label ?? "";
}
