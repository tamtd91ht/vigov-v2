// GENERATED — do not edit, source deploy/hosts.yaml. Regenerate: `go run ./tools/ingress` (or `make kb`).
//
// API hosts for exactly the services `DichVuViGov` in dia-chi-vigov.ts lists. One bundle carries
// both tables; the second is chosen only by a launch link carrying the dev profile code
// (src/lib/runtime-profile.ts, owner 14/10/2026).

// Environment `prod` (`mini_app_environment`).
export const SERVICE_API_HOSTS = {
  petitions: "https://petitions.api.vigov.vn",
  identity: "https://identity.api.vigov.vn",
  comms: "https://comms.api.vigov.vn",
} as const;

// Environment `dev` (`mini_app_dev_environment`).
export const SERVICE_API_HOSTS_DEV = {
  petitions: "https://petitions-dev.api.vigov.vn",
  identity: "https://identity-dev.api.vigov.vn",
  comms: "https://comms-dev.api.vigov.vn",
} as const;
