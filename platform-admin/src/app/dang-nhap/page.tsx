import { ServerCog } from "lucide-react";

import { SignInFlow } from "@/features/auth/sign-in-flow";

export const metadata = { title: "Đăng nhập — ViGov Khu vận hành nền tảng" };

/**
 * `/dang-nhap` — the only public path (`lib/route-access.ts`).
 *
 * Two columns from 768px like web-admin's sign-in (brand left, a 420px card right), stacked below.
 * The brand column carries ONLY what this page said before the redesign — product, area, audience.
 * web-admin's slogan, module tiles and "Kết nối được mã hoá" line are not copied: each is a claim,
 * and a sentence on a sign-in page is a promise. Nor the red–gold strip: that is the identity accent
 * of a commune's own console (ADR 0068 §2), and this is the vendor's console, not a public authority's.
 */
export default function SignInPage() {
  return (
    <div className="flex min-h-dvh flex-col bg-canvas md:flex-row">
      <section aria-label="ViGov" className="flex flex-col gap-6 px-6 pt-10 pb-4 md:w-[52%] md:justify-center md:px-16 md:py-14">
        <div className="flex items-center gap-3">
          <span
            aria-hidden="true"
            className="grid size-12 shrink-0 place-items-center rounded-xl border border-brand-100 bg-brand-50 text-brand-600 md:size-14"
          >
            <ServerCog className="size-[26px]" strokeWidth={1.8} focusable="false" />
          </span>
          <div>
            <p className="m-0 text-[22px] leading-tight font-bold text-ink-900 md:text-2xl">ViGov</p>
            <p className="m-0 mt-0.5 text-[13px] text-ink-500">Khu vận hành nền tảng</p>
          </div>
        </div>
        {/* No "every action is audited" claim: listing communes and issuing a QR are deliberately
            not audited (ADR 0048 §30/09 #5, #9), and a sentence on a sign-in page is a promise. */}
        <p className="m-0 max-w-lg text-[26px] leading-tight font-bold tracking-[-0.01em] text-ink-900 md:text-[34px]">
          Dành cho người vận hành của <span className="text-brand-600">ViHAT</span>.
        </p>
      </section>
      <main className="flex flex-1 items-center justify-center px-4 pt-4 pb-10 md:w-[48%] md:p-10">
        <SignInFlow />
      </main>
    </div>
  );
}
