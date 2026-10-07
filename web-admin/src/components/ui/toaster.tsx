"use client";

import { Toaster as SonnerToaster } from "sonner";

/**
 * The ONE toast region of the app (ADR 0068 §Sửa đổi 07/10/2026 lần 6 #4), mounted once in the root
 * layout. Screens call `toast.success(…)` / `toast.error(…)` from `sonner`; they never mount a second
 * region — two regions would show each toast twice and announce it twice.
 *
 * Position and look are the prototype's (`vigov-require/apps/admin/src/components/providers/
 * AppProviders.tsx`: `bottom-center`, `richColors`, `closeButton`). The two labels sonner would
 * otherwise read out in English are given in Vietnamese: a screen reader announces the region and
 * the close button by these strings.
 *
 * A toast REPORTS an outcome; it never replaces an error shown in place. A form's validation error
 * stays under its field (lần 6 #4), because a toast disappears and the field does not.
 *
 * sonner inserts its stylesheet as a `<style>` element at runtime — the reason it was refused in
 * ADR 0068 §4, lifted by lần 6 #4. A strict CSP will need to allow it (`tools/security_debt.json`,
 * `missing-security-headers`).
 */
export function Toaster() {
  return (
    <SonnerToaster
      position="bottom-center"
      richColors
      closeButton
      containerAriaLabel="Thông báo"
      toastOptions={{ closeButtonAriaLabel: "Đóng thông báo" }}
    />
  );
}
