import { Plus } from "lucide-react";

import { BackLink } from "@/components/back-link";
import { PageHeader } from "@/components/ui/page-header";
import { CreateCommunePage } from "@/features/communes/create-commune-form";

export const metadata = { title: "Tạo xã — ViGov Khu vận hành nền tảng" };

/** `/xa/moi` — create a commune (ADR 0048 §01/10 #1, #4). A form page: readable width (880px). */
export default function NewCommunePage() {
  return (
    <div className="max-w-[880px] min-w-0">
      <BackLink href="/xa">Danh sách xã</BackLink>
      <PageHeader icon={Plus} title="Tạo xã" />
      <CreateCommunePage />
    </div>
  );
}
