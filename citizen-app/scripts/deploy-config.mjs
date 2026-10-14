/**
 * Non-secret, platform-wide settings of the deploy step (`deploy.mjs`). Nothing here reaches the bundle:
 * only `deploy.mjs` imports this file (`dich-den.test.mjs` pins that no file under `src/` and not
 * `vite.config.ts` does).
 */

/** service-platform origin the deploy step asks for the target's App ID. Platform-wide constant, not secret, not per commune (owner 06/10/2026). */
export const VIGOV_PLATFORM_API_HOST = "https://platform.api.vigov.vn";

/**
 * The same, for the DEV environment — read only by `npm run qr -- --profile=dev` (owner 14/10/2026): a dev QR must
 * carry the App ID and launch code the DEV database holds. `<service>-dev.<api_suffix>` per deploy/hosts.yaml.
 */
export const VIGOV_PLATFORM_API_HOST_DEV = "https://platform-dev.api.vigov.vn";
