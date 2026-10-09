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

// Dự thảo = tangerine, Đã ký = leaf (spec 05 §B: the record's two states, prototype chip colours).
const MEETING_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon }>> = {
  [TRANG_THAI_DU_THAO]: { tone: "warning", icon: FilePen },
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
type ConclusionLook = { tone: BadgeTone; icon: LucideIcon; className?: string };

// Spec 05 §B colours: Chưa giao on the page colour, Không phát sinh a 10% grey — both quieter than
// the tone's own neutral fill. Đang thực hiện brand, Hoàn thành leaf, Quá hạn danger = the tones.
const CONCLUSION_LOOK: Readonly<Record<string, ConclusionLook>> = {
  "chua-giao": { tone: "neutral", icon: CircleDashed, className: "border-line bg-canvas text-ink-muted" },
  "dang-thuc-hien": { tone: "info", icon: Loader },
  "qua-han": { tone: "danger", icon: AlarmClock },
  "hoan-thanh": { tone: "success", icon: CircleCheck },
};

const NO_TASK_LOOK: ConclusionLook = {
  tone: "neutral",
  icon: CircleMinus,
  className: "border-line bg-ink-muted/10 text-ink-muted",
};

/** The pill sits on the progress line: 20px high, 10.5px word (spec 05 §B). */
export function ConclusionStatusBadge({
  conclusion,
  children,
}: {
  conclusion: petitions_ketLuanRa;
  children: ReactNode;
}) {
  const look = conclusion.no_task ? NO_TASK_LOOK : CONCLUSION_LOOK[conclusion.status];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon} className={cn("h-5 text-[10.5px]", look?.className)}>
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
        // The prototype's tile (`MeetingMinutes.tsx:134`): brand at 12%, 24px, 11px bold.
        "conclusion-ordinal grid size-6 shrink-0 place-items-center rounded-full bg-brand/12 text-[11px] font-bold text-brand tabular-nums",
        className,
      )}
    >
      {n}
    </span>
  );
}
