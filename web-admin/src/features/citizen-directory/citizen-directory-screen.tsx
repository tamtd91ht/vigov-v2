"use client";

import { PageHeader } from "@/components/ui/page-header";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { CITIZEN_READ_PERMISSION, CITIZEN_UPDATE_PERMISSION, quyetDinhTheoKhoa } from "@/lib/quyen";

import { CitizenDirectoryWorkspace } from "./citizen-directory-workspace";
import { CITIZEN_DIRECTORY_SUBTITLE, CITIZEN_DIRECTORY_TITLE, NO_READ_PERMISSION } from "./citizen-directory-words";

/**
 * `/nguoi-dan` — the gate and the frame (chapter 16, ADR 0097). The page (`app/nguoi-dan/page.tsx`)
 * supplies the shell; this decides what the session may see.
 *
 * THE TITLE IS DRAWN IN EVERY STATE, as the staff directory does: while the session is being read, or
 * when it is refused, the header is drawn here without actions, above the gate's own sentence. Once the
 * gate opens the workspace draws the header itself, with its buttons.
 *
 * Gate: `citizen.read`, the key every read route declares. The buttons, the row click, the trash icons
 * and "Xem số đầy đủ" ride on `citizen.update`, decided once here (`canEdit`) and passed down. Both are
 * convenience: the server checks each key on every call (rule 5, forbidden #1).
 */
export function CitizenDirectoryScreen() {
  const phien = usePhien();
  const open = phien !== null && quyetDinhTheoKhoa(phien, CITIZEN_READ_PERMISSION).hien;
  const canEdit = phien !== null && quyetDinhTheoKhoa(phien, CITIZEN_UPDATE_PERMISSION).hien;
  return (
    <div className="min-w-0">
      {!open && <PageHeader title={CITIZEN_DIRECTORY_TITLE} subtitle={CITIZEN_DIRECTORY_SUBTITLE} className="mb-5" />}
      <CongQuyen khoa={CITIZEN_READ_PERMISSION} cauThieuQuyen={NO_READ_PERMISSION}>
        <CitizenDirectoryWorkspace canEdit={canEdit} />
      </CongQuyen>
    </div>
  );
}
