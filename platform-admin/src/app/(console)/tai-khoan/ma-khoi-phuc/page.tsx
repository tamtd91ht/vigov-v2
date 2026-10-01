import { RegenerateRecoveryCodes } from "@/features/account/regenerate-recovery-codes";

export const metadata = { title: "Tạo lại mã khôi phục — ViGov Khu vận hành nền tảng" };

export default function RecoveryCodesPage() {
  return (
    <>
      <h1 className="page-title">Tạo lại mã khôi phục</h1>
      <RegenerateRecoveryCodes />
    </>
  );
}
