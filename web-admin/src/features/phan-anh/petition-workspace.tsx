"use client";

import { MessageSquareWarning } from "lucide-react";
import { useState } from "react";

import { PageHeader } from "@/components/ui/page-header";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { drillDownKey, NO_DRILL_DOWN, type DrillDown } from "@/lib/drill-down";
import { coQuyen, QUYEN_XEM_PHAN_ANH } from "@/lib/quyen";

import { PETITION_INTAKE_PERMISSION, PETITION_PAGE_SUBTITLE, PETITION_PAGE_TITLE } from "./nhan-phieu";
import { SoPhanAnh } from "./so-phan-anh";
import { StaffIntakeButton } from "./staff-intake";
import { TraCuuPhieu } from "./tra-cuu-phieu";

/** What an account without `feedback.read` is told, naming the key it lacks. */
export const PETITION_READ_DENIED =
  "Tài khoản của bạn không có quyền xem phản ánh của người dân (feedback.read), nên " +
  "phần này không hiển thị. Liên hệ quản trị viên của đơn vị nếu bạn cần quyền này.";

/**
 * The Phản ánh screen — the prototype's `FeedbackWorkspace` frame (ADR 0068 lần 5): the page header
 * with `+ Nhập hộ phản ánh` on the right of the title row, then the register (KPI cards, the three
 * tabs, the filter row, the cards).
 *
 * THE HEADER IS DRAWN FOR EVERY ACCOUNT, the register only behind `feedback.read` (`CongQuyen`), so an
 * account without the key still sees which screen it is on. The intake button is gated by
 * `feedback.create` inside `StaffIntakeButton` — UX only, the server checks both keys (rule 5).
 *
 * NO "tác phong cán bộ đi luồng riêng" LINE (prototype `FeedbackWorkspace.tsx:125-131`): telling an
 * account without `feedback.restricted` that such petitions exist is exactly what rule 4, forbidden #2
 * forbids. The register is silent about them.
 *
 * A booked petition bumps `bookings`, which the register folds into its read key: the new petition
 * shows without rebuilding the register (its filters and page stay).
 */
export function PetitionWorkspace({
  drillDown = NO_DRILL_DOWN,
}: {
  /** Overview filter, read on the SERVER (`app/phan-anh/page.tsx`). */
  drillDown?: DrillDown<"citizen-reports">;
}) {
  const phien = usePhien();
  // FAIL CLOSED: an unread or failed session holds no key (rule 1, forbidden #1).
  const permissions: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const [bookings, setBookings] = useState(0);

  return (
    <>
      <PageHeader
        icon={MessageSquareWarning}
        title={PETITION_PAGE_TITLE}
        subtitle={PETITION_PAGE_SUBTITLE}
        actions={
          coQuyen(permissions, PETITION_INTAKE_PERMISSION) ? (
            <StaffIntakeButton permissions={permissions} onBooked={() => setBookings((n) => n + 1)} />
          ) : undefined
        }
      />
      <CongQuyen khoa={QUYEN_XEM_PHAN_ANH} cauThieuQuyen={PETITION_READ_DENIED}>
        {/* One spacing between the register and the lookup (spec §6.9). */}
        <div className="flex min-w-0 flex-col gap-6">
          <SoPhanAnh key={drillDownKey(drillDown)} drillDown={drillDown} reloadSignal={bookings} />
          <TraCuuPhieu />
        </div>
      </CongQuyen>
    </>
  );
}
