// GENERATED — do not edit, source deploy/hosts.yaml. Regenerate: `go run ./tools/ingress` (or `make kb`).
//
// API hosts of environment `prod` (`mini_app_environment`) for exactly the services
// `DichVuViGov` in dia-chi-vigov.ts lists. The Mini App is built with prod hosts only (owner,
// 01/10/2026).
export const SERVICE_API_HOSTS = {
  petitions: "https://petitions.api.vigov.vn",
  identity: "https://identity.api.vigov.vn",
  comms: "https://comms.api.vigov.vn",
} as const;
