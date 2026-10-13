import { ServerCog } from "lucide-react";

import { SignInFlow } from "@/features/auth/sign-in-flow";

export const metadata = { title: "Đăng nhập — ViGov Khu vận hành nền tảng" };

/**
 * `/dang-nhap` — the only public path (`lib/route-access.ts`).
 *
 * web-admin's sign-in composition (`.trang-dang-nhap`, ADR 0068 lần 5): a flat page background and one
 * centred column — the identity block (a 44px `brand-50` tile, the name in navy, the area in small
 * upper case), then the card. Same shape at 320px as on a wide screen; the column only narrows.
 * The identity block carries ONLY what this page said before the redesign — product, area, audience.
 * web-admin's slogan, module tiles and "Kết nối được mã hoá" line are not copied: each is a claim,
 * and a sentence on a sign-in page is a promise. Nor the red–gold strip: that is the identity accent
 * of a commune's own console (ADR 0068 §2), and this is the vendor's console, not a public authority's.
 */
export default function SignInPage() {
  return (
    <main className="grid min-h-dvh place-items-center bg-canvas px-4 py-6">
      <div className="flex w-full max-w-[34rem] min-w-0 flex-col items-center">
        <section aria-label="ViGov" className="mb-7 flex w-full max-w-[25rem] min-w-0 flex-col gap-3">
          <div className="flex items-center gap-3">
            <span
              aria-hidden="true"
              className="grid size-11 shrink-0 place-items-center rounded-xl border border-solid border-brand-100 bg-brand-50 text-brand-600"
            >
              <ServerCog className="size-6" strokeWidth={1.8} focusable="false" />
            </span>
            <div className="min-w-0">
              <p className="m-0 text-[19px] leading-tight font-bold text-navy">ViGov</p>
              <p className="m-0 mt-0.5 text-xs font-medium tracking-[0.05em] text-ink-muted uppercase">Khu vận hành nền tảng</p>
            </div>
          </div>
          {/* No "every action is audited" claim: listing communes and issuing a QR are deliberately
              not audited (ADR 0048 §30/09 #5, #9), and a sentence on a sign-in page is a promise. */}
          <p className="m-0 text-[13px] text-ink">
            Dành cho người vận hành của <span className="font-semibold text-navy">ViHAT</span>.
          </p>
        </section>
        <SignInFlow />
      </div>
    </main>
  );
}
