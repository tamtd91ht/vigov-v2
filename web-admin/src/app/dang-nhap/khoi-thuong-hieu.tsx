"use client";

import { Landmark } from "lucide-react";

import { useCauHinhXa } from "@/components/cau-hinh-xa";
import { CommuneLogoImage } from "@/components/commune-identity";

/**
 * The identity block above the login card — the prototype's tile + two lines
 * (`(auth)/dang-nhap/page.tsx`, ADR 0068 lần 5). The slogan, the four module tiles and the
 * parent-authority row of the earlier two-column screen are gone: the prototype has none of them.
 *
 * NO PRODUCT NAME (ADR 0068 §13, owner 02/10/2026): where the prototype prints "ViGov" this prints
 * the commune's name, and the second line is "Hệ thống điều hành số" — the wording the owner
 * approved for this block, kept over the prototype's "Nền tảng điều hành số cấp xã".
 *
 * The commune's name comes down through server-built context, read at runtime from `Host`. This is
 * exactly where a `NEXT_PUBLIC_TEN_XA` would creep in — and one bundle cannot carry 300 names.
 * Printed VERBATIM: no upper-casing, no prefix — the unit's name is data the commune declared.
 */
export function KhoiThuongHieu() {
  const xa = useCauHinhXa();

  return (
    <div className="login-brand-head">
      {/* The commune's own logo when it uploaded one (ADR 0069 #4), drawn exactly as in the sidebar
          tile (`CommuneLogoImage`: contain, transparency kept, alt "" because the name is printed
          right beside it). None yet → the building icon, as in the sidebar (ADR 0069 #7): the
          prototype's "VG" initials were the product's mark, and ADR 0068 §13 removed the product. */}
      <div className={xa.logoUrl !== "" ? "logo-vigov has-logo" : "logo-vigov"} aria-hidden="true">
        {xa.logoUrl !== "" ? <CommuneLogoImage src={xa.logoUrl} /> : <Landmark focusable="false" strokeWidth={1.8} />}
      </div>
      <div className="min-w-0">
        <p className="ten-san-pham">{xa.displayName}</p>
        <p className="phu-de-san-pham">Hệ thống điều hành số</p>
      </div>
    </div>
  );
}
