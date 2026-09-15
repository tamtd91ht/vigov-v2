/**
 * The platform console talks to the platform service and to NOTHING ELSE.
 *
 * This is the whole security model of this application, and it is enforced by ARCHITECTURE
 * rather than by a permission flag:
 *
 *   The vendor operating this console cannot read a commune's petitions, documents or citizen
 *   data because THERE IS NO CLIENT FOR THOSE SERVICES — not because a flag is switched off.
 *   A flag can be flipped; adding a client has to be written, and would be caught in review.
 *
 * See kb/10-decisions/0003-platform-admin-metadata-only.md.
 *
 * If you find yourself wanting to import a business service client here, STOP. That is the
 * moment the decision is being reversed, and it is a decision for the customer, not for code.
 */

const PLATFORM_API = process.env.NEXT_PUBLIC_PLATFORM_API ?? "";

/** Metadata only: name, status, domain, quota. Never business content. */
export type TenantSummary = {
  tenantId: string;
  host: string;
  displayName: string;
  active: boolean;
  /** Counts arrive via events from the business services. Never by querying their data. */
  counts?: { staff: number; openPetitions: number };
};

export async function listTenants(): Promise<TenantSummary[]> {
  // TODO(skeleton): GET ${PLATFORM_API}/tenants
  void PLATFORM_API;
  throw new Error("not implemented");
}
