import { Bot } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { ZaloBotScreen } from "@/features/zalo-bot/zalo-bot-screen";

export const metadata = { title: "Zalo Bot — ViGov Khu vận hành nền tảng" };

/** `/zalo-bot` — the platform's shared Zalo Bot, staff reminder channel (ADR 0074). */
export default function ZaloBotPage() {
  return (
    <>
      <PageHeader
        icon={Bot}
        title="Zalo Bot"
        subtitle="Bot dùng chung nhắc việc cán bộ qua Zalo: token, webhook và số cán bộ đã ghép nối ở từng xã."
      />
      <ZaloBotScreen />
    </>
  );
}
