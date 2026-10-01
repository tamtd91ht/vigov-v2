import Link from "next/link";

import { CreateCommunePage } from "@/features/communes/create-commune-form";

export const metadata = { title: "Tạo xã — ViGov Khu vận hành nền tảng" };

/** `/xa/moi` — create a commune (ADR 0048 §01/10 #1, #4). */
export default function NewCommunePage() {
  return (
    <>
      <p className="breadcrumb">
        <Link href="/xa">Danh sách xã</Link>
      </p>
      <h1 className="page-title">Tạo xã</h1>
      <CreateCommunePage />
    </>
  );
}
