import {
  Archive,
  BadgeCheck,
  FilePen,
  Loader,
  LoaderCircle,
  MailCheck,
  MailX,
  type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import { Badge, type BadgeTone } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/cn";

/**
 * Presentation shared by the Thông báo nội bộ screen (ADR 0068, spec v2). Nothing here reads data
 * or decides anything: it maps a CODE the server already sent to an icon and a tone. The WORD in
 * every pill is still the one `nhan-thong-bao.ts` returns, passed verbatim as `children`.
 *
 * ICON BY CODE, NEVER BY LABEL: a label is text somebody may reword; the code is the contract.
 * ICON + WORD, never colour alone (spec §7).
 */

/** Decorative icon inside a button or a line of text: hidden from assistive tech, never focusable. */
export function Glyph({ icon: Icon, className }: { icon: LucideIcon; className?: string }) {
  return <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className={className} />;
}

/**
 * Record status pill. Only drawn where `coChipTrangThai` says so — `da-phat-hanh` has no pill.
 * `da-go` is a withdrawal, not something late: neutral, never red (spec §2 "màu mang ý nghĩa").
 * An unknown code keeps the neutral pill and its raw word.
 */
const STATUS_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon }>> = {
  nhap: { tone: "neutral", icon: FilePen },
  "da-go": { tone: "neutral", icon: Archive },
};

export function AnnouncementStatusBadge({ code, children }: { code: string; children: ReactNode }) {
  const look = STATUS_LOOK[code];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon}>
      {children}
    </Badge>
  );
}

/** Email delivery pill, keyed on `email_status`. Today `nhanTrangThaiThu` never returns a word. */
const EMAIL_LOOK: Readonly<Record<string, { tone: BadgeTone; icon: LucideIcon }>> = {
  "dang-gui": { tone: "info", icon: Loader },
  "da-gui": { tone: "success", icon: MailCheck },
  loi: { tone: "danger", icon: MailX },
};

export function EmailStatusBadge({ code, children }: { code: string; children: ReactNode }) {
  const look = EMAIL_LOOK[code];
  return (
    <Badge tone={look?.tone ?? "neutral"} icon={look?.icon}>
      {children}
    </Badge>
  );
}

/** The orange "Bắt buộc xác nhận" pill of §3 — warning tone, its own icon shape. */
export function AckRequiredBadge({ children }: { children: ReactNode }) {
  return (
    <Badge tone="warning" icon={BadgeCheck}>
      {children}
    </Badge>
  );
}

/**
 * Content of a submit button that KEEPS ITS WIDTH while busy (spec v2 §8b "Nút đang xử lý"): the
 * idle icon + label and the spinner + busy words share one grid cell, and the idle one only turns
 * `invisible`. Not busy → the busy layer is not rendered at all, so the markup carries the label
 * once and nothing else.
 */
export function SubmitContent({
  busy,
  icon,
  label,
  busyText,
}: {
  busy: boolean;
  icon: LucideIcon;
  label: string;
  busyText: string;
}) {
  return (
    <span className="inline-grid [&>*]:[grid-area:1/1]">
      <span className={cn("inline-flex items-center justify-center gap-2", busy && "invisible")}>
        <Glyph icon={icon} />
        {label}
      </span>
      {busy && (
        <span className="inline-flex items-center justify-center gap-2">
          <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin" />
          {busyText}
        </span>
      )}
    </span>
  );
}

/** Thin 2px bar on top of the list while a page is (re)loading (spec v2 §8b). Decorative. */
export function LoadingBar() {
  return (
    <div aria-hidden="true" className="h-0.5 w-full overflow-hidden bg-brand-50">
      <div className="h-full w-1/3 bg-brand-500 motion-safe:animate-pulse" />
    </div>
  );
}

/**
 * First-load placeholder (spec v2 §8b): grey blocks in the shape of announcement cards, so the
 * layout does not jump when the page arrives. Decorative — the screen's own `role="status"`
 * sentence is what announces the load.
 */
export function AnnouncementCardsSkeleton({ cards = 3 }: { cards?: number }) {
  return (
    <div aria-hidden="true" className="flex flex-col gap-3">
      {Array.from({ length: cards }, (_, i) => (
        <div key={i} className="flex flex-col gap-3 rounded-card border border-line bg-surface p-4 shadow-sm">
          <div className="flex items-center gap-3">
            <Skeleton className="h-3.5 min-w-0 flex-1" />
            <Skeleton className="h-[22px] w-28 shrink-0 rounded-full" />
          </div>
          <Skeleton className="w-4/5" />
          <Skeleton className="w-3/5" />
          <Skeleton className="w-2/5" />
        </div>
      ))}
    </div>
  );
}
