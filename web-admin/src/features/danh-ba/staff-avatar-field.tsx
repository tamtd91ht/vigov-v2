import { PendingField } from "@/components/ui/pending-feature";

import { pendingPart } from "./nhan-danh-ba";

/**
 * Spec §5's `Ảnh đại diện` field of the staff form, between `Có Zalo` and `Thứ tự hiển thị`: a
 * disabled file input with the "?" of ADR 0068 §14, described by the Danh bạ `Ảnh đại diện` entry.
 * No `name`, no handler: nothing is submitted, nothing is uploaded.
 *
 * ITS OWN COMPONENT, passed to `BieuMauGhiCanBo` as an element: the "?" uses hooks (Radix, `useId`),
 * and `danh-ba-lien-he.luong.test.tsx` calls the screen as a plain function under a mocked React.
 */
export function StaffAvatarField() {
  return (
    <PendingField
      info={pendingPart("Ảnh đại diện")}
      id="o-anh-dai-dien-can-bo"
      label="Ảnh đại diện"
      kind="file"
      className="min-w-0 flex-none"
    />
  );
}
