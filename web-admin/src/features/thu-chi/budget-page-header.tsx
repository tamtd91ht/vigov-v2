/**
 * The page header of Thu - Chi, as the prototype draws it (`app/(workspace)/giai-ngan/thu-chi/page.tsx`):
 * an 18px title and a 12px line under it, no icon, `mb-4`. Smaller than the shared `PageHeader` (22px /
 * 13px) ON PURPOSE — the spec keeps this page apart from the other Giải ngân pages (spec 02 "Khung").
 *
 * One component for the real page and the `/xem-thu/thu-chi` preview, so the screenshot shows the same
 * header the officer sees.
 *
 * `m-0` on both: Tailwind's preflight is off (`globals.css`), so an `<h1>` / `<p>` would keep the
 * browser's margins.
 */
export function BudgetPageHeader() {
  return (
    <header className="mb-4">
      <h1 className="m-0 text-[18px] font-bold text-navy">Thu - Chi ngân sách xã</h1>
      <p className="m-0 text-[12px] text-ink-muted">
        Nạp thẳng tệp Excel của Phòng Tài chính. Một tệp hai sheet nạp được cả thu lẫn chi trong một
        lần, khoản mục và cột sinh ra theo đúng tệp.
      </p>
    </header>
  );
}
