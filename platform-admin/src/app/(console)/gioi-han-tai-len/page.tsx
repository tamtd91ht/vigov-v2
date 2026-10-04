import { FileUp } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { UploadPolicyScreen } from "@/features/upload-policies/upload-policy-screen";

export const metadata = { title: "Giới hạn tải lên — ViGov Khu vận hành nền tảng" };

/** `/gioi-han-tai-len` — upload limits, platform-wide (ADR 0073 #5). */
export default function UploadPolicyPage() {
  return (
    <>
      <PageHeader icon={FileUp} title="Giới hạn tải lên" subtitle="Dung lượng, kiểu tệp và số tệp cho từng mục đích tải lên, áp dụng chung mọi xã." />
      <UploadPolicyScreen />
    </>
  );
}
