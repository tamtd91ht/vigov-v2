import { ChangePasswordForm } from "@/features/account/change-password-form";

export const metadata = { title: "Đổi mật khẩu — ViGov Khu vận hành nền tảng" };

export default function ChangePasswordPage() {
  return (
    <>
      <h1 className="page-title">Đổi mật khẩu</h1>
      <ChangePasswordForm />
    </>
  );
}
