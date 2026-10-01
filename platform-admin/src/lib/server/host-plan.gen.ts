// GENERATED — do not edit, source deploy/hosts.yaml. Regenerate: `go run ./tools/ingress` (or `make kb`).
//
// Parents of each environment's commune web hosts (`*.<root>`), most specific first: a host is
// cut by the first root it ends with — the same list and order core/config uses.
export const PLATFORM_WEB_ROOTS: readonly string[] = ["stg.vigov.vn", "vigov.vn"];
