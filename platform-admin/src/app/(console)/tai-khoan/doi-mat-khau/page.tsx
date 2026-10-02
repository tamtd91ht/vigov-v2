import { KeyRound } from "lucide-react";

import { PageHeader } from "@/components/ui/page-header";
import { ChangePasswordForm } from "@/features/account/change-password-form";

export const metadata = { title: "Đổi mật khẩu — ViGov Khu vận hành nền tảng" };

/** A form page: one centred card at a readable width. */
export default function ChangePasswordPage() {
  return (
    <div className="mx-auto w-full max-w-[36rem] min-w-0">
      <PageHeader icon={KeyRound} title="Đổi mật khẩu" />
      <ChangePasswordForm />
    </div>
  );
}
