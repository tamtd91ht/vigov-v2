import {
  AlarmClock,
  CircleCheck,
  CircleDashed,
  CircleMinus,
  FileCheck2,
  FilePen,
  Loader,
  type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import { Badge, type BadgeTone } from "@/components/ui/badge";
import type { petitions_bienBanRa, petitions_ketLuanRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { TRANG_THAI_DA_KY, TRANG_THAI_DU_THAO } from "./nhan-bien-ban";

/**
 * Presentation shared by the Biên bản họp screen (ADR 0068, spec §8.5). Nothing here reads data or
 * decides anything: it maps a status CODE the server already sent to an icon and a tone. The WORD
 * inside every pill is still the one `nhan-bien-ban.ts` returns, passed verbatim as `children`.
 *
 * ICON BY CODE, NEVER BY LABEL: a label is text somebody may reword; the code is the contract.
 * ICON + WORD, never colour alone (spec §7) — every tone below also has its own icon shape.
 */

const MEETING_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon }>> = {
  [TRANG_THAI_DU_THAO]: { tone: "neutral", icon: FilePen },
  [TRANG_THAI_DA_KY]: { tone: "success", icon: FileCheck2 },
};

/** Status pill of a meeting record. An unknown code keeps the neutral pill and its raw word. */
export function MeetingStatusBadge({ meeting, children }: { meeting: petitions_bienBanRa; children: ReactNode }) {
  const look = MEETING_LOOK[meeting.status];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon}>
      {children}
    </Badge>
  );
}

/**
 * The conclusion status is DERIVED BY THE SERVER (rule 10, invariant 3) — this only draws it.
 * `no_task` goes first, exactly as `nhanTrangThaiKetLuan` reads it: the server counts a "không phát
 * sinh nhiệm vụ" conclusion as done, and the pill must say why, with its own icon.
 *
 * Red (`danger`) only for `qua-han`: something actually late (spec §2, "màu mang ý nghĩa").
 */
const CONCLUSION_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon }>> = {
  "chua-giao": { tone: "neutral", icon: CircleDashed },
  "dang-thuc-hien": { tone: "info", icon: Loader },
  "qua-han": { tone: "danger", icon: AlarmClock },
  "hoan-thanh": { tone: "success", icon: CircleCheck },
};

const NO_TASK_LOOK = { tone: "success", icon: CircleMinus } as const;

export function ConclusionStatusBadge({
  conclusion,
  children,
}: {
  conclusion: petitions_ketLuanRa;
  children: ReactNode;
}) {
  const look = conclusion.no_task ? NO_TASK_LOOK : CONCLUSION_LOOK[conclusion.status];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon}>
      {children}
    </Badge>
  );
}

/**
 * The round ordinal tile of a conclusion (spec §2 "ô tròn số thứ tự").
 *
 * `n` MUST BE THE SERVER'S `ordinal` (`soThuTuKetLuan`), never a map index — see the block comment
 * of `so-bien-ban.tsx`. `conclusion-ordinal` is the stable hook the tests read the number through:
 * the utility classes beside it are look, and may change without a test going red for a wrong reason.
 */
export function ConclusionOrdinal({ n, className }: { n: number; className?: string }) {
  return (
    <span
      className={cn(
        "conclusion-ordinal inline-grid size-7 shrink-0 place-items-center rounded-full bg-brand-50 text-xs font-semibold text-brand-700 tabular-nums",
        className,
      )}
    >
      {n}
    </span>
  );
}

/**
 * First-load placeholder (spec §8b): grey blocks in the shape of meeting cards, so the layout does
 * not jump when the page arrives. LOCAL ON PURPOSE — a shared `Skeleton` is being built by another
 * work item; this one is replaced by it then. Decorative: the screen's own `role="status"` sentence
 * is what announces the load.
 */
export function MeetingCardsSkeleton({ cards = 3 }: { cards?: number }) {
  return (
    <div aria-hidden="true" className="flex flex-col gap-4">
      {Array.from({ length: cards }, (_, i) => (
        <div key={i} className="rounded-card border border-line bg-surface shadow-sm">
          <div className="flex items-center gap-3 border-b border-line px-4 py-3.5">
            <span className="size-9 shrink-0 rounded-lg bg-line motion-safe:animate-pulse" />
            <span className="h-3.5 min-w-0 flex-1 rounded bg-line motion-safe:animate-pulse" />
            <span className="h-[22px] w-20 shrink-0 rounded-full bg-line motion-safe:animate-pulse" />
          </div>
          <div className="flex flex-col gap-3 p-4">
            <span className="h-3 w-2/5 rounded bg-line motion-safe:animate-pulse" />
            <span className="h-3 w-4/5 rounded bg-line motion-safe:animate-pulse" />
            <span className="h-3 w-3/5 rounded bg-line motion-safe:animate-pulse" />
          </div>
        </div>
      ))}
    </div>
  );
}
