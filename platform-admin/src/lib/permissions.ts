import { OPS_KEYS } from "@/lib/api";

/**
 * Which controls to SHOW. UX only, never security (rule 5 forbidden #1): service-platform checks
 * the `ops.*` keys of every call (`opauth.RequireKey`), and a hidden button that is called anyway
 * gets a 403. Each predicate mirrors the guard of one route in operator_routes.go, and an operator
 * whose keys are not loaded yet holds none — a control appears only once the keys say so.
 */

export function hasAll(keys: readonly string[], ...required: string[]): boolean {
  return required.every((k) => keys.includes(k));
}

/** POST /communes: ops.tenant.manage AND ops.domain.manage. */
export const canCreateCommune = (keys: readonly string[]) =>
  hasAll(keys, OPS_KEYS.tenantManage, OPS_KEYS.domainManage);

/** POST /communes/{id}/domains, PUT …/primary-domain: ops.domain.manage. */
export const canManageDomains = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.domainManage);

/** PUT …/name, PUT …/activation: ops.tenant.manage. */
export const canManageCommune = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.tenantManage);

/** POST …/mini-apps: ops.mini_app.manage. */
export const canManageMiniApps = (keys: readonly string[]) => hasAll(keys, OPS_KEYS.miniAppManage);
