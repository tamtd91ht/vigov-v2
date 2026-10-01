import { TenantListEmpty } from "@/features/tenants/tenant-list-empty";

export const metadata = { title: "Danh sách xã — ViGov Khu vận hành nền tảng" };

/**
 * `/xa` — the commune directory (ADR 0048 §01/10 #4). TODO(TASK-06b): load it via `listTenants`.
 * Until then it shows the empty state, which says the list is NOT LOADED — never "no communes",
 * which would be a false statement about the platform.
 */
export default function TenantListPage() {
  return (
    <>
      <h1 className="page-title">Danh sách xã</h1>
      <TenantListEmpty />
    </>
  );
}
