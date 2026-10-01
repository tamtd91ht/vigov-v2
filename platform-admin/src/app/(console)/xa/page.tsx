import { CommuneList } from "@/features/communes/commune-list";

export const metadata = { title: "Danh sách xã — ViGov Khu vận hành nền tảng" };

/** `/xa` — the commune directory (ADR 0048 §01/10 #4). */
export default function CommuneListPage() {
  return (
    <>
      <h1 className="page-title">Danh sách xã</h1>
      <CommuneList />
    </>
  );
}
