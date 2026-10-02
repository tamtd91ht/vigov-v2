import { LayoutDashboard, ShieldX } from "lucide-react";
import Link from "next/link";

import { cn } from "@/lib/cn";

/**
 * "Không có quyền" — spec v2 §8b: `ShieldX`, one sentence, the person's current role, and a way
 * back to Tổng quan.
 *
 * WHEN IT IS DRAWN IS NOT DECIDED HERE. The screen shows it in the branch it already has for a
 * denied read (a 403 from the server, or `CongQuyen`'s denied case). Drawing it is UX; the route
 * behind the screen still refuses on its own (rule 5, forbidden #1).
 *
 * `roleName` comes from the session (`sessionRoleName` in `components/role-pill.tsx`). Absent or
 * blank = the line is left out, never a guessed role.
 *
 * The way back is a plain link, not a button, because it navigates. `homeHref` defaults to
 * `/tong-quan`; a screen whose users may lack `report.read` can point it elsewhere, since Tổng quan
 * would then refuse them too.
 */
export const NO_ACCESS_TITLE = "Bạn không có quyền xem mục này";
export const NO_ACCESS_HOME_LABEL = "Về Tổng quan";

export type NoAccessProps = {
  roleName?: string | null;
  homeHref?: string;
  className?: string;
  role?: "alert" | "status";
};

export function NoAccess({ roleName, homeHref = "/tong-quan", className, role }: NoAccessProps) {
  const roleLine = typeof roleName === "string" ? roleName.trim() : "";
  return (
    <div role={role} className={cn("no-access flex flex-col items-center gap-2 px-4 py-10 text-center", className)}>
      <span aria-hidden="true" className="mb-2 grid size-[72px] place-items-center rounded-full bg-surface-muted text-ink-500">
        <ShieldX className="size-8" strokeWidth={1.6} focusable="false" />
      </span>
      <p className="m-0 text-base font-semibold text-ink-900">{NO_ACCESS_TITLE}</p>
      {roleLine !== "" && (
        <p className="no-access-role m-0 text-[13px] text-ink-500">
          Vai trò hiện tại: <strong className="font-semibold text-ink-700">{roleLine}</strong>
        </p>
      )}
      <Link
        href={homeHref}
        className="nut-phu mt-2 inline-flex h-10 items-center gap-2 rounded-control border border-line-strong bg-surface px-4 text-sm font-semibold text-ink-700 no-underline hover:border-brand-100 hover:bg-brand-50 hover:text-brand-700 [&_svg]:size-[18px]"
      >
        <LayoutDashboard aria-hidden="true" focusable="false" strokeWidth={1.8} />
        {NO_ACCESS_HOME_LABEL}
      </Link>
    </div>
  );
}
