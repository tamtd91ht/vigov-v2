/** Exported so the test pins the wording: the sentence IS the behaviour of this screen. */
export const TENANT_LIST_NOT_LOADED = "Chưa tải được danh sách xã.";
export const TENANT_LIST_NOT_LOADED_DETAIL =
  "Màn hình này chưa được nối với dịch vụ nền tảng. Danh sách sẽ hiện tại đây khi chức năng được mở.";

export function TenantListEmpty() {
  return (
    <section className="empty-state" aria-live="polite">
      <p className="empty-state-title">{TENANT_LIST_NOT_LOADED}</p>
      <p className="empty-state-detail">{TENANT_LIST_NOT_LOADED_DETAIL}</p>
    </section>
  );
}
