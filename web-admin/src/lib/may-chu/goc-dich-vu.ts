import "server-only";

import type { DichVuAPI } from "@/lib/api/dinh-tuyen.gen";

/**
 * Where each Go service's REST port lives, as seen from INSIDE the cluster.
 *
 * WHY THESE ARE ENVIRONMENT VARIABLES AND THAT IS ALLOWED: an internal origin is a PLATFORM
 * constant — the same for every commune — so rule 8 invariant 5 permits it. What it must never
 * be is `NEXT_PUBLIC_*`: that ships into the browser bundle, and the browser has no business
 * knowing the cluster's service names. This module is `server-only` so importing it from a
 * client component fails the BUILD rather than leaking at runtime.
 *
 * WHY A DEFAULT IS ACCEPTABLE HERE WHEN RULE 1 FORBIDS DEFAULTS: the default names a SERVICE,
 * never a commune. Which commune a request belongs to is still decided by the Go edge from the
 * `Host` header this app forwards untouched; an unset variable can at worst point at the wrong
 * port, which answers with a connection error, not with another commune's data. The default is
 * the k8s Service name and REST port (`deploy/base/<service>/service.yaml`), decided by the user.
 *
 * WHY THE TYPE IS A MAPPED TYPE OVER `DichVuAPI`: the route table is generated from the
 * contract (`dinh-tuyen.gen.ts`). The day a sixth service gains a REST route, this object stops
 * compiling until someone says where that service lives — instead of the gateway silently
 * having no origin for it.
 */
const BIEN_GOC: { readonly [K in DichVuAPI]: { readonly bien: string; readonly macDinh: string } } = {
  identity: { bien: "IDENTITY_HTTP_ADDR", macDinh: "http://identity:8080" },
  documents: { bien: "DOCUMENTS_HTTP_ADDR", macDinh: "http://documents:8080" },
  petitions: { bien: "PETITIONS_HTTP_ADDR", macDinh: "http://petitions:8080" },
  finance: { bien: "FINANCE_HTTP_ADDR", macDinh: "http://finance:8080" },
  comms: { bien: "COMMS_HTTP_ADDR", macDinh: "http://comms:8080" },
};

/**
 * The origin to connect to for `dichVu`. Read on every call rather than once at module load:
 * a bad value must fail the first REQUEST that needs it, naming the variable, rather than
 * crash `next start` for every route including the health probe.
 *
 * THE ERROR NAMES THE VARIABLE AND NEVER ITS VALUE. A malformed origin is exactly the kind of
 * value that has credentials pasted into it (`http://user:pass@…`), and an error message goes
 * to logs (rule 3, rule 8 invariant 3).
 */
export function gocDichVu(dichVu: DichVuAPI): URL {
  const { bien, macDinh } = BIEN_GOC[dichVu];
  const tho = (process.env[bien] ?? "").trim();
  const giaTri = tho === "" ? macDinh : tho;

  let goc: URL;
  try {
    goc = new URL(giaTri);
  } catch {
    throw new Error(`${bien} không phải một địa chỉ hợp lệ`);
  }
  if (goc.protocol !== "http:" && goc.protocol !== "https:") {
    throw new Error(`${bien} phải dùng http hoặc https`);
  }
  if (goc.username !== "" || goc.password !== "") {
    // Credentials in an origin would be sent as Basic auth on every forwarded call AND sit in
    // a ConfigMap, which is not a Secret (rule 11 invariant 7).
    throw new Error(`${bien} không được chứa thông tin đăng nhập`);
  }
  if (goc.pathname !== "/" || goc.search !== "" || goc.hash !== "") {
    // A path prefix would silently rewrite every forwarded path; the gateway promises to
    // forward paths UNCHANGED, so a prefix is a configuration error, not a feature.
    throw new Error(`${bien} chỉ được là origin (scheme://host:port), không kèm đường dẫn hay truy vấn`);
  }
  return goc;
}
