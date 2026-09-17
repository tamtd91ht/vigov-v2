/**
 * Resolving which commune the citizen is acting with.
 *
 * THE CONTRADICTION TO UNDERSTAND BEFORE TOUCHING THIS FILE:
 *
 *   "Tell communes apart by domain" is right for the admin webs and DOES NOT APPLY here. A
 *   Mini App is identified by its platform App ID, not a domain; 200+ communes cannot be 200+
 *   registered apps. ONE app serves every commune, and the commune is resolved at runtime.
 *
 * See kb/00-foundation/multi-tenant-model.md.
 */

export type CommuneSource = "deeplink" | "profile" | "gps" | "manual";

export type ResolvedCommune = {
  tenantId: string;
  displayName: string;
  source: CommuneSource;
};

/**
 * Resolution order. The first that yields a commune wins.
 *
 * | Priority | Source   | Confidence                                      |
 * | -------- | -------- | ----------------------------------------------- |
 * | 1        | deeplink | High — QR at the commune office, links it sent  |
 * | 2        | profile  | High — what the citizen chose before            |
 * | 3        | gps      | Medium — a HINT only, never a decision          |
 * | 4        | manual   | Last resort, and ALWAYS available               |
 *
 * GPS suggests and never decides: locations can be spoofed, and urban boundaries run down the
 * middle of streets. A citizen standing on the wrong side of a road is not in another commune.
 */
export async function resolveCommune(): Promise<ResolvedCommune | null> {
  // TODO(skeleton): deeplink -> profile -> gps (suggest) -> manual picker
  throw new Error("not implemented");
}

/**
 * Switching commune is an EXPLICIT action, never automatic from GPS.
 *
 * And once a commune is selected its name must appear on every screen, with a final
 * confirmation before submitting anything. That is a business rule, not UI polish:
 * submitting to the wrong commune means the commune receives work outside its territory, has
 * to redirect or refuse, and the citizen waits for nothing and then loses trust.
 */
export async function switchCommune(_tenantId: string): Promise<void> {
  throw new Error("not implemented");
}
