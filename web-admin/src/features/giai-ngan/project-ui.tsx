import { CircleMinus, FilePen, Lock, ShieldCheck, TriangleAlert, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Badge, type BadgeTone } from "@/components/ui/badge";

import type { TienDoDuAn } from "./nhan-du-an"; // vi-name-ok: existing type of nhan-du-an.ts (rule 12, invariant 3)
// vi-name-ok: existing exports of nhan-ghi-giai-ngan.ts, imported unchanged (rule 12, invariant 3)
import { CHUNG_TU_DA_KHOA, CHUNG_TU_DA_XAC_NHAN, CHUNG_TU_KE_TOAN_NHAP } from "./nhan-ghi-giai-ngan";

/**
 * Presentation shared by the project list, the project detail and the voucher table (ADR 0068,
 * spec v2 §7). Nothing here reads data or decides anything: it maps a KIND or a status CODE the
 * code already computed to an icon and a tone. Hook-free, so it renders under `react-dom/server`.
 *
 * ICON + WORD, NEVER COLOUR ALONE: the words are still `nhanTienDo` / `nhanTrangThaiChungTu`,
 * passed in verbatim.
 */

/**
 * Progress pill of one project (spec 07 §Body 1). `children` is `nhanTienDo(t)`, verbatim.
 *
 *   cham          danger + `TriangleAlert` — only for a project the SERVER flagged late.
 *   bamSat        leaf, NO ICON, as the spec draws it. The words alone say it; nothing is colour-only.
 *   chuaBoTriVon  grey — no capital allocated is an accounting state, not a delay (`domain.LaCham`). A
 *                 state the prototype never shows (its ratio is never null), kept.
 */
export function ProgressBadge({ progress, children }: { progress: TienDoDuAn; children: ReactNode }) {
  if (progress.loai === "bamSat") {
    return (
      <span className="border-leaf/25 bg-leaf/12 text-leaf inline-flex h-5 w-fit shrink-0 items-center rounded-4xl border border-solid px-2 py-0.5 text-xs leading-none font-medium whitespace-nowrap">
        {children}
      </span>
    );
  }
  return progress.loai === "cham" ? (
    <Badge tone="danger" icon={TriangleAlert}>
      {children}
    </Badge>
  ) : (
    <Badge tone="neutral" icon={CircleMinus}>
      {children}
    </Badge>
  );
}

/**
 * Icon and tone per voucher status CODE (`nhan-ghi-giai-ngan.ts`), never per label. An unknown code
 * falls to the neutral pill and still shows the raw string `nhanTrangThaiChungTu` returns, so a
 * reader sees that something is off.
 */
const VOUCHER_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon; className?: string }>> = {
  // Spec 00 §6 `DISBURSEMENT_STATUS_META`: draft grey, confirmed teal, locked leaf with the Lock icon.
  [CHUNG_TU_KE_TOAN_NHAP]: { tone: "neutral", icon: FilePen },
  [CHUNG_TU_DA_XAC_NHAN]: { tone: "neutral", icon: ShieldCheck, className: "border-teal/25 bg-teal/12 text-teal" },
  [CHUNG_TU_DA_KHOA]: { tone: "success", icon: Lock },
};

/** Status pill of one voucher. `children` is `nhanTrangThaiChungTu(status)`, verbatim. */
export function VoucherStatusBadge({ status, children }: { status: string; children: ReactNode }) {
  const look = VOUCHER_LOOK[status];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon} className={look?.className} data-voucher-status={status}>
      {children}
    </Badge>
  );
}

/** Decorative icon inside a button or a line of text: hidden from assistive tech, never focusable. */
export function Glyph({ icon: Icon, className }: { icon: LucideIcon; className?: string }) {
  return <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className={className} />;
}
