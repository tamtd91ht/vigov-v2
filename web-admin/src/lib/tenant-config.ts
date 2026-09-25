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

import { goiNoiBo } from "./may-chu/goi-noi-bo";

import type { identity_get_communes_current, identity_thongTinXa } from "./api/schema.gen";

export type TenantConfig = {
  /**
   * The commune's canonical domain AS THE REGISTRY HOLDS IT — not necessarily the string the
   * caller typed. Read from the contract rather than echoed back from the request, so that a
   * caller reaching a commune by an alias cannot make the application repeat the alias as if
   * it were the official one.
   */
  host: string;
  /** Display name at this moment in time. An attribute, not an identifier. */
  displayName: string;
  /**
   * The line the admin header prints under the commune name.
   *
   * THE CONTRACT CALLS THIS `province` AND THE DIFFERENCE IS DELIBERATE — do not "fix" it to
   * match. ADR 0017 decides it, and decides it in this direction: each layer names a value
   * after what IT knows. The contract knows the registry is holding a province
   * (`tinh_thanh`); this screen knows it is printing the line the design calls "cơ quan cấp
   * trên". The two coincide today and are not the same concept — a province is where the
   * commune IS, a superior authority is who it ANSWERS TO — so the day an administrative
   * reorganisation separates them, the cost lands here, in a presentation layer no archival
   * record points at, instead of in the contract. Renaming this field to `province` would
   * move that cost onto the surface that is hardest to take back, and would buy nothing.
   *
   * "" means the commune has not declared one (`service-identity/internal/http/xa.go`) —
   * render NOTHING on that line. Never a placeholder, never a guess: a guessed parent
   * authority printed under the name of a public body is that body stating something untrue
   * about itself.
   *
   * → `kb/10-decisions/0017-contract-field-naming.md`
   */
  parentAuthority: string;
};

/**
 * THERE IS NO `tenantId` FIELD, AND ADDING ONE IS THE MISTAKE THIS PARAGRAPH EXISTS TO STOP.
 *
 * The public route deliberately does not return one, and a server-side test pins that
 * (`service-identity/internal/http/xa_test.go`). Nothing here needs it either: the commune is
 * derived server-side from `Host` on EVERY request (rule 1, invariant 3). An internal
 * identifier the client holds is an identifier the client can send back up — and a client
 * naming its own commune is a client granting itself access (rule 1, forbidden #2).
 *
 * THERE IS NO `active` FIELD EITHER, and for a different reason: here it could only ever be
 * `true`. The edge answers 404 for a deactivated commune BEFORE any handler runs
 * (`core/httpx/edge.go:25`), so `GET /api/v1/communes/current` cannot reply 200 for one. A
 * field that is constant by construction invites a "this commune has merged" branch that never
 * executes, and a branch that never executes is a branch nobody notices going wrong. A merged
 * commune is a real state (rule 1, invariant 6) and answering it properly means the edge saying
 * something other than a bare 404 — a change in `core/httpx` that every service shares. STATED,
 * not half-built on this side.
 */

/**
 * Resolve the commune for an incoming Host, server-side.
 *
 * A Host matching no commune yields `null`, and the caller returns 404 — never a fallback
 * commune, and never a 400 that reveals which communes exist.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * WHY THIS ASKS THE SERVER INSTEAD OF HOLDING A TABLE: the commune registry belongs to the
 * platform service (rule 2, invariant 1). A commune directory copied into the web tier is a
 * second source that drifts — and it drifts into the one screen where being wrong is an
 * incident: a public body's own name.
 *
 * WHY THE CALL RUNS ON THE SERVER. The usual objection to server-side calls in this app is
 * that they force the session cookie to be forwarded by hand, which is the easiest place to
 * forward it to the wrong host (`features/phien/phien-hien-tai.tsx`). It does not apply to
 * this one route: it is PUBLIC (`x-vigov-permission: public` — the sign-in screen must print
 * the commune name before any session exists), so nothing is forwarded and no cookie is in
 * scope here at all. And the answer is needed on the server regardless: `layCauHinhXa()` must
 * be able to call `notFound()` BEFORE any HTML is produced, and the commune name has to be
 * right on the first paint rather than appearing a moment later.
 *
 * WHY NO CACHE ACROSS REQUESTS, despite this sitting on the path of every page render. A
 * process-level cache is exactly the shape rule 1 forbids — one wrong key and commune A's
 * name renders on commune B's screen, with every test still green. A cache keyed by host
 * would avoid that, but there is no invalidation channel: an administrative renaming would
 * keep showing the old name for the length of the TTL, on the header of a public authority.
 * De-duplication WITHIN one request already happens, in request scope, in
 * `tenant.server.ts`. The cost is one extra HTTP call per page render, on the same host, to
 * a route that is a single registry lookup — and that is the cheap side of this trade.
 *
 * WHERE THE CALL GOES (decided 25/09/2026; supersedes the earlier "call the commune's own
 * public host over TLS"). The cluster sends every path — `/api` included — to THIS app, so the
 * public host leads straight back here, and in production that loop answered 307 → throw → 500
 * on `/dang-nhap`. The call now goes to identity's INTERNAL origin (`IDENTITY_HTTP_ADDR`, a
 * platform constant, `lib/may-chu/goc-dich-vu.ts`) with the COMMUNE'S host in the `Host`
 * header, because the Go edge resolves the commune from `r.Host` and nothing else
 * (`core/httpx/edge.go:24`). That is why this uses `node:http` and not `fetch`: undici
 * overwrites `Host` with the URL's host (`lib/may-chu/goi-noi-bo.ts`), which would ask identity
 * about "the internal service name" — 404 on every page.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export async function resolveTenant(host: string): Promise<TenantConfig | null> {
  const goc = gocAPI(host);
  // A Host this application cannot even parse is not a commune. The connection target no
  // longer comes from it (it is the internal origin), but the Host header sent below still
  // does — and the Host is a security input of the same weight as the token in a system told
  // apart by domain (`kb/00-foundation/multi-tenant-model.md`, §Ranh giới tin cậy). Only a
  // plain, normalised host name goes on the wire.
  if (goc === null) return null;

  const duongDan: identity_get_communes_current["duongDan"] = "/api/v1/communes/current";
  // No credentials and no other headers. The route is public, so there is nothing to
  // authorise; and no `tenant_id` travels anywhere — the commune IS the Host being asked.
  // No cache either: `node:http` has none, and no process-level one is added (see above).
  const phanHoi = await goiNoiBo("identity", { method: "GET", duongDan, host: goc.host });
  const status = phanHoi.statusCode ?? 0;

  // 404 IS THE ONLY "NO". The edge answers it identically for a host that was never a commune
  // and for one that has been deactivated, on purpose: telling them apart lets anyone probing
  // domains enumerate which communes exist (`core/httpx/edge.go`). This side does not try to
  // tell them apart either.
  if (status === 404) {
    phanHoi.resume();
    return null;
  }

  // ANYTHING ELSE IS A FAILURE TO ANSWER, NOT AN ANSWER OF "NO" — so it throws rather than
  // returning `null`. Returning `null` would render the 404 page, telling an operator during
  // an outage that a commune which does exist does not. Throwing fails closed just the same
  // (no page, no fallback commune) while staying honest about which of the two happened.
  //
  // A 3xx LANDS HERE TOO, and that is the point: `node:http` never follows redirects, and a
  // redirect followed would be a way to make the server read some other commune's
  // configuration and print it under this host.
  if (status !== 200) {
    phanHoi.resume();
    throw new Error(`không đọc được cấu hình xã: máy chủ trả ${status}`);
  }

  const khoi: Buffer[] = [];
  for await (const k of phanHoi) khoi.push(k as Buffer);
  const than = JSON.parse(Buffer.concat(khoi).toString("utf8")) as identity_thongTinXa;
  return {
    host: than.host,
    displayName: than.name,
    // The one place the contract's name and this layer's name are joined — ADR 0017.
    parentAuthority: than.province,
  };
}

/**
 * The request's own Host, normalised and validated; its `.host` is what goes into the `Host`
 * header of the call to identity. `null` when the Host is not a plain hostname — which is
 * refused, not repaired.
 *
 * The `https://` prefix is only a vehicle for the parser: the connection itself goes to
 * `IDENTITY_HTTP_ADDR`, never to this URL.
 *
 * WHY THE `URL` PARSER RATHER THAN A HAND-WRITTEN CHECK: the comparison below rejects anything
 * the parser reads as more than a host — a path (`xa.example/../other`), user info
 * (`someone@elsewhere`), a stray space — because for all of those `url.host` comes back
 * different from what was passed in. A regular expression written here would have to
 * anticipate each of those separately, and the one it forgets is the one that gets used.
 */
function gocAPI(host: string): URL | null {
  // The edge lowercases the host before looking it up; do the same so that two spellings of
  // one commune cannot become two different origins.
  const chuan = host.trim().toLowerCase();
  if (chuan === "") return null;

  let goc: URL;
  try {
    goc = new URL(`https://${chuan}`);
  } catch {
    return null;
  }
  return goc.host === chuan ? goc : null;
}
