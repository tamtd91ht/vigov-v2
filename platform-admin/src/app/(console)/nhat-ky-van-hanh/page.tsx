import { ScrollText } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { OperatorLogScreen } from "@/features/operator-log/operator-log";

export const metadata = { title: "Nhật ký vận hành — ViGov Khu vận hành nền tảng" };

/** `/nhat-ky-van-hanh` — the operator log, every commune plus platform-wide changes (ADR 0073 #2). */
export default function OperatorLogPage() {
  return (
    <>
      <PageHeader
        icon={ScrollText}
        title="Nhật ký vận hành"
        subtitle="Thao tác của người vận hành trên các xã và thay đổi cấu hình chung, mới nhất trước."
      />
      <OperatorLogScreen />
    </>
  );
}
