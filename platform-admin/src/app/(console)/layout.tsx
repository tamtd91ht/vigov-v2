import type { ReactNode } from "react";

import { Sidebar } from "@/components/sidebar";
import { OperatorProvider } from "@/features/operator/operator-context";

/**
 * Layout of every signed-in screen. It does NOT check the session: that is `src/proxy.ts`, which
 * runs before this renders and redirects to `/dang-nhap` when the cookie is absent, and
 * service-platform, which validates it on every call. `OperatorProvider` only loads who is signed
 * in, for display and permission HINTS.
 */
export default function ConsoleLayout({ children }: { children: ReactNode }) {
  return (
    <OperatorProvider>
      <div className="console-frame">
        <Sidebar />
        <main className="page-body">{children}</main>
      </div>
    </OperatorProvider>
  );
}
