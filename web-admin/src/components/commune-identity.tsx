import { Landmark } from "lucide-react";

import type { CauHinhXaHienThi } from "@/lib/cau-hinh-xa-hien-thi";

/**
 * The commune's identity block: emblem tile, "Ủy ban nhân dân", the commune's name, its province.
 * It sits at the top of the sidebar, where the product name used to be (owner decision 02/10/2026:
 * the screen belongs to the commune's People's Committee, not to the software vendor). Pages with no
 * sidebar print it in the topbar instead, so no signed-in page is without it.
 *
 * `displayName` IS PRINTED VERBATIM, NEVER BUILT ("UBND " + name): the correct name of the body is
 * data the commune declares, and a prefix built here is wrong at the first commune that declared
 * its name with "UBND" already in it. "Ủy ban nhân dân" above it is the KIND of body — a product
 * constant (ADR 0068 §7) — not part of the name.
 *
 * `parentAuthority` "" means the commune declared none: the line is left out, never guessed
 * (`lib/tenant-config.ts`).
 *
 * THE TILE IS AN ICON, NOT A LOGO, ON PURPOSE. The commune's own logo lives in service-platform's
 * display profile (`ho_so_hien_thi_xa.logo_url`), which `GET /api/v1/communes/current` does not
 * return and no screen can set yet. Drawing a state emblem without an official file, or another
 * body's mark, is a public authority displaying something it never issued. The tile is replaced
 * the day the contract carries the logo.
 */
export const AUTHORITY_KIND = "Ủy ban nhân dân";

export function CommuneIdentity({ commune }: { commune: CauHinhXaHienThi }) {
  return (
    <div className="commune-identity">
      <span className="commune-emblem" aria-hidden="true">
        <Landmark focusable="false" strokeWidth={1.8} />
      </span>
      <div className="commune-identity-text">
        <p className="commune-authority-kind">{AUTHORITY_KIND}</p>
        <p className="ten-co-quan">{commune.displayName}</p>
        {commune.parentAuthority !== "" && <p className="co-quan-cap-tren">{commune.parentAuthority}</p>}
      </div>
    </div>
  );
}
