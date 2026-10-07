import type { CauHinhXaHienThi } from "@/lib/cau-hinh-xa-hien-thi";

/**
 * The commune's identity at the left end of the WHITE header on every signed-in page (spec 01, ADR 0068
 * §Sửa đổi 07/10/2026 lần 6 #2, #11): one line "ỦY BAN NHÂN DÂN <COMMUNE>", the province under it.
 * The screen belongs to the commune's People's Committee — the product name lives only in the sidebar's
 * brand block (lần 6 #3).
 *
 * TWO ELEMENTS ON ONE LINE, NEVER ONE BUILT STRING (lần 6 #11, ADR 0068 §7): "Ủy ban nhân dân" is the
 * KIND of body — a product constant — and `displayName` is data the commune declares, printed VERBATIM.
 * A prefix glued on here ("UBND " + name) is wrong at the first commune that declared its name with the
 * prefix already in it. The capitals of spec 01 are CSS `uppercase` on both elements: the DOM keeps the
 * name exactly as declared, so a screen reader and a copy-paste get the commune's own spelling.
 *
 * `displayName` and `parentAuthority` are read at runtime from the commune resolved from `Host`
 * (`CauHinhXaProvider`, `lib/tenant-config.ts`) — never a constant (rule 1, invariant 10). The
 * "Thăng Bình" / "Đà Nẵng" of spec 01 are its sample data. `parentAuthority` "" means the commune
 * declared none: the line is left out, never guessed.
 *
 * NO LOGO TILE HERE ANY MORE: spec 01's header has none, and the sidebar carries the brand tile
 * (lần 6 #8). The commune's uploaded logo still draws on the sign-in screen (`CommuneLogoImage`).
 */
export const AUTHORITY_KIND = "Ủy ban nhân dân";

export function CommuneIdentity({ commune }: { commune: CauHinhXaHienThi }) {
  return (
    <div className="commune-identity min-w-0 leading-tight">
      {/* line-clamp, not nowrap: a long commune name keeps every character on two lines at most
          rather than being cut off in the 64px header. */}
      <p className="m-0 line-clamp-2 text-sm font-bold tracking-wide text-navy uppercase [overflow-wrap:anywhere]">
        <span className="commune-authority-kind">{AUTHORITY_KIND}</span> <span className="ten-co-quan">{commune.displayName}</span>
      </p>
      {commune.parentAuthority !== "" && (
        <p className="co-quan-cap-tren m-0 truncate text-[11.5px] text-ink-muted">{commune.parentAuthority}</p>
      )}
    </div>
  );
}

/**
 * The commune's own logo, drawn by the sign-in screen's tile (`app/dang-nhap/khoi-thuong-hieu.tsx`):
 * `object-fit: contain` (`.commune-logo`), so a logo is never cropped and its transparent background
 * shows the tile's surface rather than a white box.
 *
 * `alt=""`: the commune's name is printed as text right beside it. An alt naming the body would make a
 * screen reader read the name twice in a row; the image adds nothing a listener lacks.
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
