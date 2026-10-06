import { ContactRound } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { QUYEN_QUAN_LY_NGUOI_DUNG } from "@/lib/quyen";

import { DanhBaLienHe } from "./danh-ba-lien-he";
import { CAU_THIEU_QUYEN, MO_TA_TRANG, TIEU_DE_TRANG } from "./nhan-danh-ba";

/**
 * The `Danh bạ cán bộ` tab of `/mini-app` — the prototype's `StaffDirectoryWorkspace` header (`<h1>Danh bạ
 * cán bộ</h1>` + one line, actions on the right), then the screen (`DanhBaLienHe`).
 *
 * THE `<h1>` STAYS OUTSIDE THE GATE, so the tab keeps its title while the gate reads the session or when it
 * refuses. The header's right-hand buttons need the screen's state (the bulk panel, the session), so
 * `DanhBaLienHe` draws them and lifts them into this row from `lg` up (`lg:absolute` against the
 * `relative` wrapper here; the header reserves the room with `lg:pr-*`).
 *
 * Gate: `admin.user`, the key `GET /api/v1/staff` declares (see `app/mini-app/page.tsx`). Convenience only.
 */
export function StaffDirectoryTab() {
  return (
    <div className="relative min-w-0">
      <PageHeader icon={ContactRound} title={TIEU_DE_TRANG} subtitle={MO_TA_TRANG} className="lg:pr-[30rem]" />
      <CongQuyen khoa={QUYEN_QUAN_LY_NGUOI_DUNG} cauThieuQuyen={CAU_THIEU_QUYEN}>
        <DanhBaLienHe />
      </CongQuyen>
    </div>
  );
}
