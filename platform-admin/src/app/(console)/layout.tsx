import type { ReactNode } from "react";

import { ConsoleShell } from "@/features/operator/console-shell";
import { OperatorProvider } from "@/features/operator/operator-context";

/**
 * Layout of every signed-in screen. It does NOT check the session: that is `src/proxy.ts`, which
 * runs before this renders and redirects to `/dang-nhap` when the cookie is absent, and
 * service-platform, which validates it on every call. `OperatorProvider` only loads who is signed
 * in, for display and permission HINTS; `ConsoleShell` draws the frame, or only the forced password
 * change while the session is restricted (ADR 0048 §13/10 #6).
 */
export default function ConsoleLayout({ children }: { children: ReactNode }) {
  return (
    <OperatorProvider>
      <ConsoleShell>{children}</ConsoleShell>
    </OperatorProvider>
  );
}
