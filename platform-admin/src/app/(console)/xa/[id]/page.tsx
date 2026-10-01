import Link from "next/link";

import { CommuneDetailScreen } from "@/features/communes/commune-detail";

export const metadata = { title: "Thông tin xã — ViGov Khu vận hành nền tảng" };

/**
 * `/xa/[id]` — one commune. The id is the path's, passed as it is: service-platform answers a
 * malformed id exactly like an unknown one (404 `commune_not_found`), so nothing is checked here.
 */
export default async function CommunePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <>
      <p className="breadcrumb">
        <Link href="/xa">Danh sách xã</Link>
      </p>
      <CommuneDetailScreen communeId={id} />
    </>
  );
}
