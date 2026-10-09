"use client";

import { NotebookTabs } from "lucide-react";
import { useEffect } from "react";

import { PageHeader } from "@/components/ui/page-header";
import { LeaderNotebook } from "@/features/leader-notebook/leader-notebook";
import { PAGE_SUBTITLE, PAGE_TITLE } from "@/features/leader-notebook/notebook-queries";

import { previewTask } from "./tasks.fixture";

/**
 * The REAL Sổ tay lãnh đạo body, exactly as `app/nhiem-vu/so-tay/page.tsx` composes it (header, then
 * `LeaderNotebook`), fed by the fixture answering machine. The fixture covers every row variant the
 * prototype and spec 03 §B draw: a late task (NV103), an extended one (NV107, `đã gia hạn 1 lần`), a
 * paused one (NV110, its badge), sub-tasks (NV111/NV112, `việc con của NV105`) and one extension request.
 *
 * `open` (`?mo=NV105`) presses that task's REAL row once it is drawn — the drawer opens in place, as a
 * click would; the screen itself has no address-bar word for it (the register's `?task=` is not this
 * page's).
 */
export function NotebookPreview({ open }: { open: string | null }) {
  usePressRowWhenReady(open);
  return (
    <>
      <PageHeader icon={NotebookTabs} title={PAGE_TITLE} subtitle={PAGE_SUBTITLE} className="mb-6" />
      <LeaderNotebook />
    </>
  );
}

function usePressRowWhenReady(code: string | null): void {
  useEffect(() => {
    const title = code === null ? null : (previewTask(code)?.title ?? null);
    if (title === null) return;
    let done = false;
    const timer = window.setInterval(() => {
      const row = [...document.querySelectorAll<HTMLButtonElement>(".leader-notebook li > button")].find((b) =>
        b.textContent?.startsWith(title),
      );
      if (row === undefined || done) return;
      done = true;
      window.clearInterval(timer);
      row.click();
    }, 100);
    const stop = window.setTimeout(() => window.clearInterval(timer), 10_000);
    return () => {
      window.clearInterval(timer);
      window.clearTimeout(stop);
    };
  }, [code]);
}
