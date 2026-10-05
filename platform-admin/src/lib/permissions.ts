import { ALL_OPS_KEYS, OPS_KEYS } from "@/lib/api";

/**
 * Which controls to SHOW. UX only, never security (rule 5 forbidden #1): service-platform checks
 * the `ops.*` keys of every call (`opauth.RequireKey`), and a hidden button that is called anyway
 * gets a 403. Each predicate mirrors the guard of one route in operator_routes.go, and an operator
 * whose keys are not loaded yet holds none — a control appears only once the keys say so.
 */

export function hasAll(keys: readonly string[], ...required: string[]): boolean {
  return required.every((k) => keys.includes(k));
}

/**
 * The console's pure READS — commune list and detail, provinces, upload limits, the shared Mini App,
 * tier-1 petition fields, the operator log: ANY one decided key (ADR 0073 #1, `opauth.AnyKey`). Only
 * a key of the closed set counts, like the server's: a string it does not know grants nothing, and a
 * session holding no key still reads nothing.
 */
export const canReadConsole = (keys: readonly string[]) => keys.some((k) => ALL_OPS_KEYS.includes(k));

/** POST /communes: ops.tenant.manage AND ops.domain.manage. */
export const canCreateCommune = (keys: readonly string[]) =>
  hasAll(keys, OPS_KEYS.tenantManage, OPS_KEYS.domainManage);

/** POST /communes/{id}/domains, PUT …/primary-domain: ops.domain.manage. */
export const canManageDomains = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.domainManage);

/** PUT …/name, PUT …/activation: ops.tenant.manage. */
export const canManageCommune = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.tenantManage);

/** Every commune Mini App write, and PUT /shared-mini-app: ops.mini_app.manage. */
export const canManageMiniApps = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.miniAppManage);

/** PUT /upload-policies/{purpose}: ops.upload_policy.manage. */
export const canManageUploadPolicies = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.uploadPolicyManage);

/** GET /communes/{id}/mini-app-launch-link: ops.qr.issue (a read with its OWN key, not AnyKey). */
export const canIssueQr = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.qrIssue);

/**
 * Every `zalo-bots/shared…` route, the READS INCLUDED: ops.zalo_bot.manage (ADR 0074 #3). Unlike the
 * other console sections this one is not open to any `ops.*` key — so its menu entry and its page
 * follow this key alone.
 */
export const canManageZaloBot = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.zaloBotManage);

/** POST /petition-fields, PUT …/{code}, PUT …/{code}/activation: ops.petition_field.manage. */
export const canManagePetitionFields = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.petitionFieldManage);
