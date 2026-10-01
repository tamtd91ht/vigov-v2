import type { ReactNode } from "react";

import { Sidebar } from "@/components/sidebar";

/**
 * Layout of every signed-in screen. It does NOT check the session: that is `src/proxy.ts`, which
 * runs before this renders and redirects to `/dang-nhap`. A check here would be a second copy.
 */
export default function ConsoleLayout({ children }: { children: ReactNode }) {
  return (
    <div className="console-frame">
      <Sidebar />
      <main className="page-body">{children}</main>
    </div>
  );
}
