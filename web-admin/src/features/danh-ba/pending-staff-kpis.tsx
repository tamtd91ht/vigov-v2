import { Smartphone, Users } from "lucide-react";

import { PendingStatCard } from "@/components/ui/pending-feature";

import { pendingPart } from "./nhan-danh-ba";

/**
 * Spec §2's two KPI cards that cannot be counted — TỔNG SỐ CÁN BỘ and ĐANG HIỆN TRÊN MINI APP — at the
 * head of the screen, disabled with the "?" of ADR 0068 §14. "—" instead of a figure: the contract
 * returns no total (`nhan-danh-ba.ts`, `PHAN_CHUA_DUNG`). No server call.
 *
 * ITS OWN COMPONENT, NOT INLINE IN `DanhBaLienHe`: the "?" uses hooks (Radix, `useId`), and
 * `danh-ba-lien-he.luong.test.tsx` calls `DanhBaLienHe` as a plain function under a mocked React. As
 * a child element this is never invoked there.
 */
export function PendingStaffKpis() {
  return (
    <div className="grid min-w-0 grid-cols-1 gap-3 min-[480px]:grid-cols-2 lg:max-w-2xl">
      <PendingStatCard info={pendingPart("Tổng số cán bộ")} icon={Users} />
      <PendingStatCard info={pendingPart("Đang hiện trên Mini App")} icon={Smartphone} />
    </div>
  );
}
