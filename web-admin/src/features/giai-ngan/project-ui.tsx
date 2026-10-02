import {
  CircleCheck,
  CircleMinus,
  FilePen,
  LockKeyhole,
  ShieldX,
  TrendingDown,
  type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import { Badge, type BadgeTone } from "@/components/ui/badge";
import { Notice } from "@/components/ui/notice";

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

const PROGRESS_LOOK: Readonly<Record<TienDoDuAn["loai"], { tone: BadgeTone; icon: LucideIcon }>> = {
  // No capital allocated is an accounting state, not a delay (`domain.LaCham`): grey, never red.
  chuaBoTriVon: { tone: "neutral", icon: CircleMinus },
  bamSat: { tone: "success", icon: CircleCheck },
  // Red only for a project the SERVER flagged as late against the commune's threshold.
  cham: { tone: "danger", icon: TrendingDown },
};

/** Progress pill of one project. `children` is `nhanTienDo(t)`, verbatim. */
export function ProgressBadge({ progress, children }: { progress: TienDoDuAn; children: ReactNode }) {
  const look = PROGRESS_LOOK[progress.loai];
  return (
    <Badge tone={look.tone} icon={look.icon}>
      {children}
    </Badge>
  );
}

/**
 * Icon and tone per voucher status CODE (`nhan-ghi-giai-ngan.ts`), never per label. An unknown code
 * falls to the neutral pill and still shows the raw string `nhanTrangThaiChungTu` returns, so a
 * reader sees that something is off.
 */
const VOUCHER_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon }>> = {
  // Entered, waiting for the person responsible to confirm it: amber = "chờ" (spec §7).
  [CHUNG_TU_KE_TOAN_NHAP]: { tone: "warning", icon: FilePen },
  [CHUNG_TU_DA_XAC_NHAN]: { tone: "success", icon: CircleCheck },
  [CHUNG_TU_DA_KHOA]: { tone: "info", icon: LockKeyhole },
};

/** Status pill of one voucher. `children` is `nhanTrangThaiChungTu(status)`, verbatim. */
export function VoucherStatusBadge({ status, children }: { status: string; children: ReactNode }) {
  const look = VOUCHER_LOOK[status];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon}>
      {children}
    </Badge>
  );
}

/**
 * "Your account lacks key X" — the existing sentence, drawn as a calm grey note with a shield, not
 * as a dashed empty-state box: it is a fact about the account, not about the data (spec v2 §8b).
 */
export function DeniedNote({ children }: { children: ReactNode }) {
  return (
    <Notice tone="neutral" icon={ShieldX}>
      {children}
    </Notice>
  );
}

/** Decorative icon inside a button or a line of text: hidden from assistive tech, never focusable. */
export function Glyph({ icon: Icon, className }: { icon: LucideIcon; className?: string }) {
  return <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className={className} />;
}
