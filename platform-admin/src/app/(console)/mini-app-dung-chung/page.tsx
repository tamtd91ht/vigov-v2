import { Smartphone } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { SharedMiniAppScreen } from "@/features/shared-mini-app/shared-mini-app-screen";

export const metadata = { title: "Mini App dùng chung — ViGov Khu vận hành nền tảng" };

/** `/mini-app-dung-chung` — the platform's shared Mini App (owner 04/10/2026, ADR 0073 #5). */
export default function SharedMiniAppPage() {
  return (
    <>
      <PageHeader
        icon={Smartphone}
        title="Mini App dùng chung"
        subtitle="App ID mà mã QR mở Mini App của mọi xã dùng chung."
      />
      <SharedMiniAppScreen />
    </>
  );
}
