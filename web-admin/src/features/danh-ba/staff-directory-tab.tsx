"use client";

import { CongQuyen } from "@/features/quyen/cong-quyen";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { QUYEN_QUAN_LY_NGUOI_DUNG, quyetDinhTheoKhoa } from "@/lib/quyen";

import { DanhBaLienHe } from "./danh-ba-lien-he";
import { DirectoryHeading } from "./directory-heading";
import { CAU_THIEU_QUYEN } from "./nhan-danh-ba";

/**
 * The `Danh bạ cán bộ` tab of `/mini-app` — the prototype's `StaffDirectoryWorkspace`. The tab frame
 * (`features/mini-app/`) supplies the `p-7`; nothing here adds padding of its own.
 *
 * THE TITLE IS DRAWN IN EVERY STATE. Once the gate opens, the screen draws the header itself, with its
 * two actions (they open dialogs the screen owns). While the session is being read, or when it
 * refuses, the header is drawn here, without actions, above the gate's own sentence.
 *
 * Gate: `admin.user`, the key `GET /api/v1/staff` declares (see `app/mini-app/page.tsx`). Convenience
 * only — the server checks the key on every request (rule 5, forbidden #1).
 */
export function StaffDirectoryTab() {
  const phien = usePhien();
  const open = phien !== null && quyetDinhTheoKhoa(phien, QUYEN_QUAN_LY_NGUOI_DUNG).hien;
  return (
    <div className="min-w-0">
      {!open && <DirectoryHeading />}
      <CongQuyen khoa={QUYEN_QUAN_LY_NGUOI_DUNG} cauThieuQuyen={CAU_THIEU_QUYEN}>
        <DanhBaLienHe />
      </CongQuyen>
    </div>
  );
}
