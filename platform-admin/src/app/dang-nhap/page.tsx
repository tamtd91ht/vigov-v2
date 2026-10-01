import { SignInFlow } from "@/features/auth/sign-in-flow";

export const metadata = { title: "Đăng nhập — ViGov Khu vận hành nền tảng" };

/** `/dang-nhap` — the only public path (`lib/route-access.ts`). */
export default function SignInPage() {
  return (
    <div className="sign-in-page">
      <section className="brand-column" aria-label="ViGov">
        <div className="brand-mark" aria-hidden="true">
          VG
        </div>
        <p className="brand-name">ViGov</p>
        <p className="brand-subtitle">KHU VẬN HÀNH NỀN TẢNG</p>
        <hr className="brand-rule" />
        {/* No "every action is audited" claim: listing communes and issuing a QR are deliberately
            not audited (ADR 0048 §30/09 #5, #9), and a sentence on a sign-in page is a promise. */}
        <p className="brand-note">Dành cho người vận hành của ViHAT.</p>
      </section>
      <main className="form-column">
        <SignInFlow />
      </main>
    </div>
  );
}
