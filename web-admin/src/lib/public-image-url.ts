/**
 * The one gate a server-issued PUBLIC image URL passes before it becomes an `<img src>`: the
 * commune's logo and web-admin banner (ADR 0069), read from `GET /api/v1/communes/current` and
 * `GET /api/v1/commune-branding`.
 *
 * "" MEANS "NO IMAGE", and that is the only fallback there is: the caller then draws the building
 * icon / no banner strip (ADR 0069 #7). Never a default picture — a public body displaying an image
 * it never issued is the failure ADR 0069 exists to prevent.
 *
 * WHY THE SERVER'S STRING IS STILL PARSED: identity answers "" while platform is down (commit
 * d93851b9), but an older identity during a rollout answers no key at all, and a value that is not an
 * absolute http(s) URL (`javascript:`, `data:`, a relative path that would hit this app's own
 * `/api` proxy) has no business in a `src`. Anything else becomes "", the same as "not set".
 * The URL carries `t_<tenant_id>` in its object key (ADR 0052, accepted by ADR 0069 #8); it is only
 * ever placed in `src`, never logged or shown as text.
 */
export function publicImageUrl(raw: unknown): string {
  if (typeof raw !== "string" || raw === "") return "";
  let u: URL;
  try {
    u = new URL(raw);
  } catch {
    return "";
  }
  return u.protocol === "https:" || u.protocol === "http:" ? u.href : "";
}
