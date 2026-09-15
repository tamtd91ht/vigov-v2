/**
 * Per-commune configuration, resolved at RUNTIME from the request Host.
 *
 * THE RULE THAT SHAPES THIS WHOLE APP: a commune-specific value is never baked into the
 * bundle. `NEXT_PUBLIC_*` is substituted at BUILD time, and one bundle cannot carry the names
 * of 200+ communes. Baking it in means building and hosting separately per commune — exactly
 * the packaging model this project rejected.
 *
 * | Value kind                         | Where it lives                |
 * | ---------------------------------- | ----------------------------- |
 * | Platform constant (API base URL)   | NEXT_PUBLIC_*                 |
 * | Commune name, parent authority,    | HERE — fetched at runtime     |
 * | logo, map centre, SLA, catalogues  | from the platform service     |
 */

export type TenantConfig = {
  /** Opaque ULID. Never the administrative code, domain, or commune name. */
  tenantId: string;
  host: string;
  /** Display name at this moment in time. An attribute, not an identifier. */
  displayName: string;
  parentAuthority: string;
  active: boolean;
};

/**
 * Resolve the commune for an incoming Host, server-side.
 *
 * A Host matching no commune must yield `null`, and the caller must return 404 — never a
 * fallback commune, and never a 400 that reveals which communes exist.
 */
export async function resolveTenant(_host: string): Promise<TenantConfig | null> {
  // TODO(skeleton): call the platform service, cache with a short TTL.
  // This sits on the path of every request at 200+ communes, so it must be cached — and
  // invalidated when a commune is renamed or its domain is reassigned, never polled.
  throw new Error("not implemented");
}
