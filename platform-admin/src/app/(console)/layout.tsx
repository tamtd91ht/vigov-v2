import type { ReactNode } from "react";

import { Sidebar } from "@/components/sidebar";
import { Topbar } from "@/components/topbar";
import { OperatorProvider } from "@/features/operator/operator-context";

/**
 * Layout of every signed-in screen. It does NOT check the session: that is `src/proxy.ts`, which
 * runs before this renders and redirects to `/dang-nhap` when the cookie is absent, and
 * service-platform, which validates it on every call. `OperatorProvider` only loads who is signed
 * in, for display and permission HINTS.
 *
 * FRAME (web-admin's `.khung-trang`): a grid of sidebar + content column. `minmax(0, 1fr)`, not
 * `1fr`: a bare `1fr` track grows to its widest child, so one wide table would push the whole page
 * sideways; with `minmax(0, …)` the column shrinks and the table scrolls inside its own region.
 */
export default function ConsoleLayout({ children }: { children: ReactNode }) {
  return (
    <OperatorProvider>
      {/* Below 1024px the sidebar is a strip on its own row: `auto 1fr` rows, or a short page would
          share its spare height with the strip and the strip would grow. */}
      <div className="grid min-h-dvh grid-cols-[minmax(0,1fr)] grid-rows-[auto_1fr] lg:grid-cols-[auto_minmax(0,1fr)] lg:grid-rows-[1fr]">
        <Sidebar />
        <div className="flex min-w-0 flex-col">
          <Topbar />
          <main className="w-full min-w-0 max-w-[90rem] px-4 pt-6 pb-10 md:px-7">{children}</main>
        </div>
      </div>
    </OperatorProvider>
  );
}
