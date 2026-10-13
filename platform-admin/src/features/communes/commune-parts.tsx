import { Badge } from "@/components/ui/badge";

/**
 * Small presentational pieces shared by the commune screens. No data loading here, so each is
 * rendered to a string in tests.
 */

/**
 * Text plus an icon of its own shape, never colour alone (skills/accessibility-elderly #6). The tone
 * follows the `active` flag the server sends, never the wording.
 */
export function StatusBadge({ active }: { active: boolean }) {
  return active ? <Badge tone="success">Đang hoạt động</Badge> : <Badge tone="neutral">Ngừng hoạt động</Badge>;
}

const DATE_TIME = new Intl.DateTimeFormat("vi-VN", {
  // The business time zone, stated: a format call without it prints the server's or the
  // browser's zone, and the vitest TZ pin (UTC) exists to catch exactly that.
  timeZone: "Asia/Ho_Chi_Minh",
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : DATE_TIME.format(d);
}

/**
 * `dd/MM/yyyy HH:mm`, date first — vi-VN's own order puts the time first ("10:05 13/10/2026"), and the
 * owner's wording for the published mark reads date first. Same zone as `formatDateTime`.
 */
export function formatDateFirst(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const p: Record<string, string> = {};
  for (const part of DATE_TIME.formatToParts(d)) p[part.type] = part.value;
  return `${p.day}/${p.month}/${p.year} ${p.hour}:${p.minute}`;
}

/**
 * `mini_app.mode` (service-platform/internal/domain/mini_app.go `CheDo…`, ADR 0044). Only `rieng`
 * is ever bound to a commune; an unknown value is shown as it is, never guessed.
 */
export function miniAppModeLabel(mode: string): string {
  if (mode === "rieng") return "Mini App riêng của xã";
  return mode;
}
