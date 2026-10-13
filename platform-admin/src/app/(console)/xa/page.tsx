import { Landmark } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { Notice } from "@/components/ui/notice";
import { CommuneList, CommuneListActions } from "@/features/communes/commune-list";
import { DELETED_NOTICE } from "@/features/owner/owner-model";

export const metadata = { title: "Danh sách xã — ViGov Khu vận hành nền tảng" };

/**
 * `/xa` — the commune directory (ADR 0048 §01/10 #4). `?deleted=1` is where a commune deletion lands
 * (`features/owner/commune-deletion.tsx`): a fixed sentence, nothing from the URL is printed, so a
 * crafted link can show no text of its own choosing.
 */
export default async function CommuneListPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const deleted = (await searchParams).deleted === "1";
  return (
    <>
      <PageHeader
        icon={Landmark}
        title="Danh sách xã"
        subtitle="Các xã trên nền tảng: tỉnh, thành phố, trạng thái và tên miền."
        actions={<CommuneListActions />}
      />
      {deleted ? (
        <Notice tone="info" role="status" className="mb-4">
          <p>{DELETED_NOTICE}</p>
        </Notice>
      ) : null}
      <CommuneList />
    </>
  );
}
