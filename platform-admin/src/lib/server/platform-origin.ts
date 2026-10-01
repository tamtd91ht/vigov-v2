import "server-only";

/**
 * Where service-platform's REST port lives, as seen from INSIDE the cluster.
 *
 * NAME: `PLATFORM_HTTP_ADDR`, the `<SERVICE>_HTTP_ADDR` convention web-admin already uses for
 * the same role (`web-admin/src/lib/may-chu/goc-dich-vu.ts`; ADR 0043) — one name per role
 * (rule 11 inv. 2). Platform-wide constant, so the environment is the right place (rule 8 inv. 5);
 * never `NEXT_PUBLIC_*` (ADR 0048 STOP #5), and this module is `server-only` so importing it from
 * a client component fails the BUILD.
 *
 * NO DEFAULT, UNLIKE web-admin. web-admin's defaults name a service that every commune deployment
 * runs. This console is deployed only where the operator area is switched on, and the address it
 * reaches holds every commune's directory: an unset value must refuse (503, `gateway.ts`), not
 * quietly try `http://platform:8080` and succeed against whatever answers there.
 *
 * Read on every call, not at module load: a bad value fails the first request that needs it,
 * naming the variable, instead of crashing `next start` for every page.
 */
export const PLATFORM_ORIGIN_VAR = "PLATFORM_HTTP_ADDR";

export type OriginResult =
  | { ok: true; origin: URL }
  | { ok: false; reason: "unset" | "malformed"; detail: string };

/**
 * The problem is named, the VALUE never is: a malformed origin is exactly where somebody pasted
 * credentials (`http://user:pass@…`), and this text goes to logs (rule 8 inv. 3).
 */
export function platformOrigin(): OriginResult {
  const raw = (process.env[PLATFORM_ORIGIN_VAR] ?? "").trim();
  if (raw === "") {
    return { ok: false, reason: "unset", detail: `${PLATFORM_ORIGIN_VAR} chưa được đặt` };
  }

  let origin: URL;
  try {
    origin = new URL(raw);
  } catch {
    return { ok: false, reason: "malformed", detail: `${PLATFORM_ORIGIN_VAR} không phải một địa chỉ hợp lệ` };
  }
  if (origin.protocol !== "http:" && origin.protocol !== "https:") {
    return { ok: false, reason: "malformed", detail: `${PLATFORM_ORIGIN_VAR} phải dùng http hoặc https` };
  }
  if (origin.username !== "" || origin.password !== "") {
    // Would be sent as Basic auth on every forwarded call, from a ConfigMap (rule 11 inv. 7).
    return { ok: false, reason: "malformed", detail: `${PLATFORM_ORIGIN_VAR} không được chứa thông tin đăng nhập` };
  }
  if (origin.pathname !== "/" || origin.search !== "" || origin.hash !== "") {
    // The gateway promises to forward paths UNCHANGED; a prefix would silently rewrite them.
    return {
      ok: false,
      reason: "malformed",
      detail: `${PLATFORM_ORIGIN_VAR} chỉ được là origin (scheme://host:port), không kèm đường dẫn hay truy vấn`,
    };
  }
  return { ok: true, origin };
}
