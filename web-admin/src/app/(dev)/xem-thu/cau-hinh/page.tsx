import { notFound } from "next/navigation";

import { devPreviewEnabled } from "@/dev-preview/preview-gate";
import { previewFullMenu, type PreviewSearchParams } from "@/dev-preview/preview-params";
import { PreviewShell } from "@/dev-preview/preview-shell";
import { SettingsPreview, type SettingsTabId } from "@/dev-preview/settings-preview";
import { ConfigPageHeader } from "@/features/cau-hinh/config-page-header";
import {
  TAB_CAU_HINH, // vi-name-ok: existing export of thanh-tab-cau-hinh.ts, imported not declared (rule 12 inv 3)
} from "@/features/cau-hinh/thanh-tab-cau-hinh";
import {
  ASSET_READ_PERMISSION,
  AUDIT_READ_PERMISSION,
  QUYEN_CAU_HINH_THOI_HAN, // vi-name-ok: existing constant of lib/quyen.ts, imported not declared
  QUYEN_QUAN_LY_DANH_MUC, // vi-name-ok: existing constant of lib/quyen.ts, imported not declared
  QUYEN_QUAN_LY_SO_DO, // vi-name-ok: existing constant of lib/quyen.ts, imported not declared
  QUYEN_XEM_NHIEM_VU, // vi-name-ok: existing constant of lib/quyen.ts, imported not declared
  QUYEN_XEM_VAN_BAN, // vi-name-ok: existing constant of lib/quyen.ts, imported not declared
} from "@/lib/quyen";

/**
 * `/xem-thu/cau-hinh` — DEV-ONLY screenshot preview of Cấu hình hệ thống (ADR 0068 lần 6 #10): the real
 * shell, the real page header and the real `KhungTabCauHinh` on fixture data (`settings.fixture.ts`),
 * no backend, no login.
 *   `?tab=<id>`             one tab of `TAB_CAU_HINH` (e.g. `thoi-han-xu-ly`), pressed on the real bar;
 *                           absent or unknown: the screen's own first tab
 *   `?state=loading|empty`  every read of the screen held unsettled / answered with empty lists
 *   `?quyen=none`           a session holding no `admin.*` / `asset.read` key: the read-only screen
 *                           (gated tabs hidden, write buttons hidden or disabled)
 *   `?menu=day-du`, `?sidebar=thu-gon`, `?toast=1`, `?menu-tai-khoan=1`, `?chuong=1`  as every preview
 * In a production build this page is a 404 (`preview-gate.ts`).
 */
export const dynamic = "force-dynamic";

export const metadata = { title: "Xem thử · Cấu hình hệ thống", robots: { index: false, follow: false } };

/** Every key a Cấu hình tab gates on (`quyen-tab.ts`, `thanh-tab-cau-hinh.ts`) — real `quyen` keys (rule 5, 3c). */
const SETTINGS_PERMISSIONS: readonly string[] = [
  QUYEN_QUAN_LY_SO_DO,
  QUYEN_QUAN_LY_DANH_MUC,
  QUYEN_CAU_HINH_THOI_HAN,
  ASSET_READ_PERMISSION,
  AUDIT_READ_PERMISSION,
];

/** `?quyen=none`: an officer with ordinary work keys and no configuration key at all. */
const NO_SETTINGS_PERMISSIONS: readonly string[] = [QUYEN_XEM_NHIEM_VU, QUYEN_XEM_VAN_BAN];

function first(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

/** An unknown word presses nothing — never a guessed tab. */
function previewSettingsTab(value: string | string[] | undefined): SettingsTabId | null {
  const v = first(value);
  return TAB_CAU_HINH.find((t) => t.ma === v)?.ma ?? null;
}

export default async function SettingsPreviewPage({ searchParams }: { searchParams: PreviewSearchParams }) {
  if (!devPreviewEnabled()) notFound();
  const q = await searchParams;
  const permissions = first(q.quyen) === "none" ? NO_SETTINGS_PERMISSIONS : SETTINGS_PERMISSIONS;
  return (
    <PreviewShell fullMenu={previewFullMenu(q.menu)} permissions={permissions} person="lanh-dao">
      {/* The real page's header and wrapper (`app/cau-hinh/page.tsx`), so the screenshot is that page. */}
      <ConfigPageHeader />
      <div className="min-w-0">
        <SettingsPreview tab={previewSettingsTab(q.tab)} />
      </div>
    </PreviewShell>
  );
}
