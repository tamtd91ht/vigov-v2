import { LifeBuoy } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { RegenerateRecoveryCodes } from "@/features/account/regenerate-recovery-codes";

export const metadata = { title: "Tạo lại mã khôi phục — ViGov Khu vận hành nền tảng" };

/** A form page: one centred card at a readable width (wide enough for the code grid). */
export default function RecoveryCodesPage() {
  return (
    <div className="mx-auto w-full max-w-[40rem] min-w-0">
      <PageHeader icon={LifeBuoy} title="Tạo lại mã khôi phục" />
      <RegenerateRecoveryCodes />
    </div>
  );
}
