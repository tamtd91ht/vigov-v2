/**
 * RUNTIME PROFILE — which backend this ONE launch talks to (owner, 14/10/2026; ADR 0046 §Sửa đổi 14/10/2026 #9).
 *
 * One bundle, one App ID, two host tables: prod (`SERVICE_API_HOSTS`, `__VIGOV_API_HOST__`) and dev
 * (`SERVICE_API_HOSTS_DEV`, `__VIGOV_API_HOST_DEV__`). A launch link carrying `pf=<code>` whose SHA-256 equals
 * `DEV_PROFILE_CODE_SHA256` selects dev; ANYTHING else — no `pf`, an empty one, a wrong one, a webview without
 * `crypto.subtle`, an exception — is prod. Never the other way round: an error must not send a citizen to dev.
 *
 * WHY A HASH AND NOT A WORD: the owner asked that the link not say "dev" and that the code be hard to guess. Only
 * the digest ships in the bundle; the code itself lives on the machine running `npm run qr`
 * (`scripts/commune-qr.mjs`, `.env.qr.local`). A 24-character random code makes the digest useless to
 * brute-force. The dev hostnames themselves ARE in the bundle — a host table cannot be hidden from the device
 * that calls it.
 *
 * ⚠ THIS STEERS WHERE REQUESTS GO, IT GRANTS NOTHING. Dev has its own database and its own sessions; a citizen
 * on dev sees dev data only, and a prod session never travels to a dev host because the profile is fixed
 * before the first call and never changes within the launch.
 *
 * PER LAUNCH, NOT REMEMBERED (owner, 14/10/2026): nothing is stored. Reopening the app without the link is prod.
 */
import { thamSo, thamSoMoApp } from "./launch-params";

export type RuntimeProfile = "prod" | "dev";

/** The launch-link parameter. Short and neutral on purpose — it must not name what it selects. */
export const PROFILE_PARAM = "pf";

/** SHA-256 (hex) of the dev profile code. Rotate = new code in `.env.qr.local`, new digest here, new bundle. */
export const DEV_PROFILE_CODE_SHA256 = "b4e0899ed5ef732109c1b2d7ae4e66299e4522cf81f286d45068bca6dbe0b805";

let active: RuntimeProfile = "prod";
let settled: Promise<RuntimeProfile> | null = null;

/** The profile of this launch. `prod` until `resolveRuntimeProfile` has settled — and `main.tsx` waits for it. */
export function activeProfile(): RuntimeProfile {
  return active;
}

async function sha256Hex(text: string): Promise<string> {
  const bytes = new TextEncoder().encode(text);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, "0")).join("");
}

/** PURE decision, testable without a launch: `dev` only for a code whose digest matches. Never rejects. */
export async function profileFor(
  params: Record<string, string>,
  expectedSha256: string = DEV_PROFILE_CODE_SHA256,
  digest: (text: string) => Promise<string> = sha256Hex,
): Promise<RuntimeProfile> {
  const code = params[PROFILE_PARAM] ?? "";
  if (code === "" || expectedSha256 === "") return "prod";
  try {
    return (await digest(code)) === expectedSha256 ? "dev" : "prod";
  } catch {
    return "prod";
  }
}

/**
 * Decide the profile ONCE per launch, before the first render (`main.tsx`). A second call returns the first
 * answer: the profile must not change while a session opened against one backend is alive.
 */
export function resolveRuntimeProfile(): Promise<RuntimeProfile> {
  if (settled === null) {
    settled = profileFor(thamSo(thamSoMoApp())).then((p) => {
      active = p;
      return p;
    });
  }
  return settled;
}

/** Tests only: forget the decision of this launch. */
export function resetRuntimeProfileForTest(profile: RuntimeProfile = "prod"): void {
  active = profile;
  settled = null;
}
