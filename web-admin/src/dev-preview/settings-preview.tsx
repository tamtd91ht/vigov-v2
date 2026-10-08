"use client";

import {
  KhungTabCauHinh, // vi-name-ok: existing component of khung-tab-cau-hinh.tsx, imported not declared (rule 12 inv 3)
} from "@/features/cau-hinh/khung-tab-cau-hinh";
import type {
  MaTabCauHinh, // vi-name-ok: existing type of thanh-tab-cau-hinh.ts, imported not declared (rule 12 inv 3)
} from "@/features/cau-hinh/thanh-tab-cau-hinh";

import { usePressWhenReady } from "./preview-shell";

/** One tab id of the Cấu hình bar (`TAB_CAU_HINH`). */
export type SettingsTabId = MaTabCauHinh;

/**
 * The REAL tab frame of Cấu hình (`KhungTabCauHinh`), exactly as `app/cau-hinh/page.tsx` mounts it, fed
 * by `settings.fixture.ts` through the shared answering machine (`fixture-fetch.ts`). The selected tab
 * lives in the frame's own state (it reads no `?tab=`), so `tab` presses the REAL tab button
 * (`#tab-cau-hinh-<id>`) once the bar has drawn — the bar appears only after the session is read, which
 * is why this waits instead of pressing at once.
 */
export function SettingsPreview({
  tab,
  press = null,
  pressTitle = null,
}: {
  tab: SettingsTabId | null;
  /** `?bam=<words>`: presses the REAL button whose text is exactly these words (opens a form row or dialog). */
  press?: string | null;
  /** `?bam-title=<title>`: presses the first REAL button with this `title` (an icon button: Pencil, Trash2). */
  pressTitle?: string | null;
}) {
  usePressWhenReady(tab === null ? null : `button[role="tab"]#tab-cau-hinh-${tab}`);
  usePressWhenReady(press === null ? null : "button", press ?? undefined);
  usePressWhenReady(pressTitle === null ? null : `button[title="${CSS.escape(pressTitle)}"]`);
  return <KhungTabCauHinh />;
}
