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
 *
 * HOW CALLS LEAVE THE BROWSER: same-origin RELATIVE paths, `/api/v1/...`. This app's own server
 * forwards them to service-platform (`lib/server/gateway.ts`), whose address is a server-only
 * setting. There is no `NEXT_PUBLIC_*` base URL — ADR 0048 STOP CONDITION #5, rule 8 inv. 4.
 *
 * TYPES: everything below is a PLACEHOLDER until service-platform publishes the operator
 * contract (TASK-05). TODO(TASK-06b): replace these hand-written shapes with the types generated
 * from that contract — never keep a hand copy beside a generated one.
 */

/** Same-origin prefix of every call. Relative on purpose: no host, no build-time constant. */
export const API_PREFIX = "/api/v1";

/** Metadata only: name, status, domain. Never business content (ADR 0003). */
export type TenantSummary = {
  tenantId: string;
  host: string;
  displayName: string;
  active: boolean;
};

/** Second factor of an operator sign-in (ADR 0048 §28/09 #10): TOTP, or a one-time recovery code. */
export type SecondFactor = { kind: "totp"; code: string } | { kind: "recovery"; code: string };

export type SignInInput = {
  email: string;
  password: string;
  secondFactor: SecondFactor;
};

/** Thrown by every call whose endpoint does not exist yet. Never a faked success. */
export class NotWiredError extends Error {
  constructor(what: string) {
    super(`${what}: not wired yet — TODO(TASK-06b)`);
    this.name = "NotWiredError";
  }
}

export async function listTenants(): Promise<TenantSummary[]> {
  // TODO(TASK-06b): the operator tenant-list route of service-platform (TASK-05), under API_PREFIX.
  throw new NotWiredError("listTenants");
}

export async function signIn(_input: SignInInput): Promise<void> {
  // TODO(TASK-06b): the operator sign-in route of service-platform (TASK-05), under API_PREFIX.
  // The session cookie is set by the server (HttpOnly, host-only); nothing here touches it.
  throw new NotWiredError("signIn");
}
