import "server-only";

import { PLATFORM_WEB_ROOTS } from "./host-plan.gen";

/**
 * The host this console's gateway names as `Host` on EVERY call to service-platform.
 *
 * WHY PINNED, NOT RELAYED: service-platform's outer mux (`service-platform/cmd/server/
 * operator_edge.go`, `buildOuter`) sends a request whose Host equals OPERATOR_HOST to the operator
 * chain and EVERY OTHER Host to the COMMUNE chain. Relaying the client's Host would let anyone who
 * reaches this pod with `Host: <commune host>` drive platform's commune REST surface — from a pod
 * that may sit inside TRUSTED_PROXY_CIDRS. This console only ever speaks to the operator area, so the
 * upstream Host is a constant of the deployment, never a property of the request.
 *
 * NAME AND MEANING: the same `OPERATOR_HOST` that `core/config` reads for platform's edge (rule 11
 * inv. 2: one name per role). Both pods must carry the same value; a mismatch sends every call to
 * the commune chain, which 404s the operator routes — wrong, but closed.
 *
 * SHAPE: the rules of `parseOperatorHost` (`core/config/operator.go`) — REFUSED, NEVER REPAIRED.
 * A value Go would refuse at start is refused here too, so the two sides cannot disagree about what
 * the host is. The commune-shape check under vigov.vn is repeated on purpose: here it is the line
 * that stops this gateway from being pointed at a commune's host by configuration.
 *
 * NO DEFAULT, read on every call (the `platform-origin.ts` precedent): unset or bad → 503 at the
 * first request, naming the variable.
 */
export const OPERATOR_HOST_VAR = "OPERATOR_HOST";

export type OperatorHostResult =
  | { ok: true; host: string }
  | { ok: false; reason: "unset" | "malformed"; detail: string };

/** `admin` / `admin-stg` — the labels ADR 0046 reserves for the vendor console (`operatorConsoleLabels`). */
const CONSOLE_LABELS = new Set(["admin", "admin-stg"]);
// The platform's web roots are GENERATED from deploy/hosts.yaml (owner, 01/10/2026) — the same
// source and order core/config's `platformWebRoots` is generated with, so the two sides cannot
// disagree about which domains are the platform's own.

const LABEL = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
const IPV4 = /^\d{1,3}(?:\.\d{1,3}){3}$/;

function malformed(why: string): OperatorHostResult {
  // The variable is named, the value is not: this text reaches logs, and the task's fail-closed
  // pattern is the same for every gateway variable.
  return { ok: false, reason: "malformed", detail: `${OPERATOR_HOST_VAR} ${why}` };
}

export function operatorHost(): OperatorHostResult {
  const raw = process.env[OPERATOR_HOST_VAR];
  const h = (raw ?? "").trim();
  if (h === "") {
    // Unset and blank alike: this console exists only where the operator area is on, so "off" is
    // not a state it can serve in.
    return { ok: false, reason: "unset", detail: `${OPERATOR_HOST_VAR} chưa được đặt` };
  }
  if (h.includes("://") || h.includes("/")) return malformed("phải là tên host trần, không scheme, không đường dẫn");
  if (h.includes(":") || h.includes("[")) return malformed("không được kèm cổng (hay là địa chỉ IPv6)");
  if (h !== h.toLowerCase()) return malformed("phải viết thường");
  if (h.endsWith(".")) return malformed("không được kết thúc bằng dấu chấm");
  if (IPV4.test(h)) return malformed("là địa chỉ IP, khu vận hành cần tên host");
  if (h.length > 253) return malformed("dài quá 253 ký tự");

  const labels = h.split(".");
  if (labels.length < 2) return malformed("không phải tên host đầy đủ");
  if (!labels.every((l) => LABEL.test(l))) return malformed("có nhãn không hợp lệ");

  if (PLATFORM_WEB_ROOTS.includes(h)) return malformed("là tên miền gốc của nền tảng");
  // Most specific root first (the generated order): "admin.stg.vigov.vn" also ends in ".vigov.vn".
  for (const root of PLATFORM_WEB_ROOTS) {
    const suffix = `.${root}`;
    if (h.endsWith(suffix)) {
      if (!CONSOLE_LABELS.has(h.slice(0, -suffix.length))) {
        return malformed("có dạng host của xã hoặc dịch vụ — khu vận hành không bao giờ dùng chung host với xã");
      }
      break;
    }
  }
  return { ok: true, host: h };
}
