import { UsersRound } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { OperatorAccountsScreen } from "@/features/owner/operator-accounts-screen";

export const metadata = { title: "Phân quyền — ViGov Khu vận hành nền tảng" };

/** `/phan-quyen` — operator accounts and their `ops.*` keys, owner only (ADR 0048 §Sửa đổi 13/10/2026). */
export default function OperatorAccountsPage() {
  return (
    <>
      <PageHeader
        icon={UsersRound}
        title="Phân quyền"
        subtitle="Tài khoản vận hành: tạo tài khoản, cấp hoặc thu hồi quyền, đặt lại mật khẩu tạm."
      />
      <OperatorAccountsScreen />
    </>
  );
}
