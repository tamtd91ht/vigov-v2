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
 * THE TILE SHOWS THE COMMUNE'S OWN LOGO WHEN IT HAS ONE, THE BUILDING ICON OTHERWISE (ADR 0069 #7).
 * The logo is the file the commune itself uploaded in Cấu hình › Nhận diện xã, scanned and normalised
 * by platform to a 512 px square PNG that keeps its transparency; it arrives as `logoUrl` from the
 * public `GET /api/v1/communes/current`. `logoUrl` "" — none uploaded, or platform did not answer —
 * draws the icon: never a state emblem or any other picture the commune did not issue itself.
 */
export const AUTHORITY_KIND = "Ủy ban nhân dân";

export function CommuneIdentity({ commune }: { commune: CauHinhXaHienThi }) {
  return (
    <div className="commune-identity">
      <span className={commune.logoUrl !== "" ? "commune-emblem has-logo" : "commune-emblem"} aria-hidden="true">
        {commune.logoUrl !== "" ? (
          <CommuneLogoImage src={commune.logoUrl} />
        ) : (
          <Landmark focusable="false" strokeWidth={1.8} />
        )}
      </span>
      <div className="commune-identity-text">
        <p className="commune-authority-kind">{AUTHORITY_KIND}</p>
        <p className="ten-co-quan">{commune.displayName}</p>
        {commune.parentAuthority !== "" && <p className="co-quan-cap-tren">{commune.parentAuthority}</p>}
      </div>
    </div>
  );
}

/**
 * The logo itself, shared by the sidebar tile and the sign-in screen's tile so both draw it the
 * same way: `object-fit: contain` (`.commune-logo`), so a logo is never cropped and its transparent
 * background shows the tile's surface rather than a white box.
 *
 * `alt=""` AND THE TILE IS `aria-hidden`, ON PURPOSE: the commune's name is printed as text right
 * beside it in both places (and stays in the DOM when the sidebar collapses). An alt naming the body
 * would make a screen reader read the name twice in a row; the image adds nothing a listener lacks.
 *
 * A PLAIN `<img>`, NOT `next/image`: the optimiser needs every remote host declared in
 * `next.config.ts` AT BUILD TIME, and the public media host is deployment configuration read by
 * platform (`OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL`), not by this app — baking it in would bind one build
 * to one storage endpoint. The file is already normalised to 512 px server-side, so there is
 * nothing for the optimiser to do. `width`/`height` are the published size, so no layout shift.
 */
export function CommuneLogoImage({ src }: { src: string }) {
  return (
    // eslint-disable-next-line @next/next/no-img-element -- see the doc comment above
    <img className="commune-logo" src={src} alt="" width={512} height={512} decoding="async" referrerPolicy="no-referrer" />
  );
}
