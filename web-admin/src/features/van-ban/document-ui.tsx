import {
  AlarmClock,
  Archive,
  ArrowUpRight,
  CircleCheck,
  ClipboardCheck,
  Inbox,
  Loader,
  Zap,
  type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import { Badge, type BadgeTone } from "@/components/ui/badge";
import { cn } from "@/lib/cn";

// vi-name-ok: existing exports of nhan-van-ban.ts, imported unchanged (rule 12, invariant 3)
import { nhanHanVanBan, type HanVanBan } from "./nhan-van-ban";

/**
 * Presentation shared by the two document registers and the incoming-document panel (ADR 0068,
 * spec v2 §7–§8b). Nothing here reads data or decides anything: it maps a CODE the server already
 * sent to an icon and a tone, and draws placeholders. Hook-free, so every piece renders under
 * `react-dom/server` and can be called as a plain function.
 */

/**
 * Icon and tone per status CODE of `service-documents/internal/domain/van_ban.go:33` — never per
 * label, so rewording a label cannot change which icon it gets. ICON + WORD, never colour alone.
 *
 * Red is deliberately absent: overdue is not a status, it is derived from `due_at` against the
 * clock at render time (rule 10, invariant 3) and drawn by `DeadlineMark`. An unknown code falls to
 * the neutral pill and still shows `nhanTrangThai`'s fallback sentence verbatim.
 */
const STATUS_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon }>> = {
  "moi-vao-so": { tone: "info", icon: Inbox },
  "da-phan-cong": { tone: "info", icon: ClipboardCheck },
  "dang-xu-ly": { tone: "info", icon: Loader },
  "da-giai-quyet": { tone: "success", icon: CircleCheck },
  "chuyen-cap-tren": { tone: "neutral", icon: ArrowUpRight },
  "luu-khong-thu-ly": { tone: "neutral", icon: Archive },
};

/** Status pill. `children` is `nhanTrangThai(status)`, passed verbatim. */
export function DocumentStatusBadge({ status, children }: { status: string; children: ReactNode }) {
  const look = STATUS_LOOK[status];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon}>
      {children}
    </Badge>
  );
}

/**
 * Urgency pill (Decree 30/2020 levels, `van_ban.go:66`). Amber with `Zap` for the three levels above
 * "Thường"; neutral for "Thường" and for "" — "no urgency written" is a real answer, not a gap.
 */
export function UrgencyBadge({ urgency, children }: { urgency: string; children: ReactNode }) {
  const raised = urgency === "khan" || urgency === "thuong-khan" || urgency === "hoa-toc";
  return (
    <Badge tone={raised ? "warning" : "neutral"} icon={raised ? Zap : undefined}>
      {children}
    </Badge>
  );
}

/**
 * The deadline of one incoming document. Past the deadline: a red pill with `AlarmClock` carrying the
 * SAME words `nhanHanVanBan` always wrote ("Quá hạn · hạn xử lý …") — the words say it, the colour
 * and the icon are the second and third signal. Otherwise the plain sentence.
 */
export function DeadlineMark({ deadline }: { deadline: HanVanBan }) {
  if (deadline.loai === "quaHan") {
    return (
      <Badge tone="danger" icon={AlarmClock}>
        {nhanHanVanBan(deadline)}
      </Badge>
    );
  }
  return <span className="whitespace-nowrap tabular-nums">{nhanHanVanBan(deadline)}</span>;
}

/**
 * A register table inside its card (spec §6.7, v2 §8.1): flush with the card, its OWN scroller both
 * ways so the header row stays put (`sticky` inside the scroller), 48px rows, thin horizontal rules
 * only — `.bang-danh-muc` already draws those. Same shape as the Nhiệm vụ list.
 */
export const REGISTER_TABLE_SCROLLER = cn(
  "bang-cuon max-h-[70vh] overflow-auto rounded-none border-0 shadow-none",
  "[&_thead_th]:sticky [&_thead_th]:top-0 [&_thead_th]:z-[1] [&_thead_th]:shadow-[inset_0_-1px_0_var(--line)]",
  "[&_tbody_td]:h-12",
);

/**
 * First-load placeholder (spec §8b): grey bars in the shape of register rows. LOCAL ON PURPOSE — a
 * shared `Skeleton` is being built by another work item; this one is replaced by it then.
 * Decorative: the screen's own `role="status"` sentence is what announces the load.
 */
export function RegisterRowsSkeleton({ rows = 5 }: { rows?: number }) {
  return (
    <div aria-hidden="true" className="divide-y divide-line">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="flex h-12 items-center gap-4 px-4">
          <span className="h-3 w-12 shrink-0 rounded bg-line motion-safe:animate-pulse" />
          <span className="h-3 w-20 shrink-0 rounded bg-line motion-safe:animate-pulse" />
          <span className="h-3 min-w-0 flex-1 rounded bg-line motion-safe:animate-pulse" />
          <span className="hidden h-3 w-28 shrink-0 rounded bg-line motion-safe:animate-pulse sm:block" />
          <span className="h-[22px] w-24 shrink-0 rounded-full bg-line motion-safe:animate-pulse" />
        </div>
      ))}
    </div>
  );
}

/**
 * How a register puts its header buttons and its body on the page. The prototype draws the create
 * button in the PAGE header, above the tab bar (`DocumentWorkspace.tsx`), while the button's state —
 * which dialog is open, the `document.create` gate — lives in the register. So the page passes this
 * function down and the register calls it with its own buttons and body: the page decides where they
 * go, the register keeps its state. Without a page (tests, a register on its own) it is `plainFrame`.
 */
export type RegisterFrame = (headerActions: ReactNode, body: ReactNode) => ReactNode;

export const plainFrame: RegisterFrame = (headerActions, body) => (
  <>
    {headerActions}
    {body}
  </>
);

/** Decorative icon inside a button or a line of text: hidden from assistive tech, never focusable. */
export function Glyph({ icon: Icon, className }: { icon: LucideIcon; className?: string }) {
  return <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className={className} />;
}
