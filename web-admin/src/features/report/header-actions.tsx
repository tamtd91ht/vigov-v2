"use client";

import { ExportPendingActions } from "@/features/dashboard/header-actions";
import { usePhien } from "@/features/phien/phien-hien-tai";

import { reportAccess } from "./report-access";

/**
 * `/bao-cao`'s export row (spec 13 §2): `[PDF][XLSX][PPTX]`, NOT built this round (ADR 0053 amendment
 * 04/10/2026, B4) — the same disabled group with "?" as `/tong-quan`. No "Trình chiếu" (spec §1).
 *
 * DRAWN ONLY WITH `report.read` + `report.export` (spec §9.4). A session not yet read, or unreadable,
 * draws nothing — fail closed. Hiding is convenience; the export route will check the key itself.
 */
export function ReportHeaderActions() {
  const session = usePhien();
  const permissions = session !== null && session.ok ? session.duLieu.permissions : [];
  return <ReportExportSlot permissions={permissions} />;
}

/** The decision alone, without the session hook — what the tests render. */
export function ReportExportSlot({ permissions }: { permissions: readonly string[] }) {
  return reportAccess(permissions).exportReport ? <ExportPendingActions /> : null;
}
