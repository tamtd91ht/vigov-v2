import { Landmark } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { CommuneList, CommuneListActions } from "@/features/communes/commune-list";

export const metadata = { title: "Danh sách xã — ViGov Khu vận hành nền tảng" };

/** `/xa` — the commune directory (ADR 0048 §01/10 #4). */
export default function CommuneListPage() {
  return (
    <>
      <PageHeader icon={Landmark} title="Danh sách xã" actions={<CommuneListActions />} />
      <CommuneList />
    </>
  );
}
