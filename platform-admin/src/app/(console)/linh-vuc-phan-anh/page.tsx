import { Tags } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { PetitionFieldScreen } from "@/features/petition-fields/petition-field-screen";

export const metadata = { title: "Lĩnh vực phản ánh — ViGov Khu vận hành nền tảng" };

/** `/linh-vuc-phan-anh` — tier-1 petition field codes (ADR 0060, ADR 0073 #3). */
export default function PetitionFieldPage() {
  return (
    <>
      <PageHeader icon={Tags} title="Lĩnh vực phản ánh" subtitle="Bộ mã lĩnh vực cấp 1, gồm cả mã đã ngừng dùng." />
      <PetitionFieldScreen />
    </>
  );
}
