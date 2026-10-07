import { notFound } from "next/navigation";

import { devPreviewEnabled } from "@/dev-preview/preview-gate";
import {
  previewFullMenu,
  previewTaskModal,
  previewTaskSelect,
  previewTaskView,
  type PreviewSearchParams,
} from "@/dev-preview/preview-params";
import { PreviewShell } from "@/dev-preview/preview-shell";
import { PREVIEW_TASK_PERMISSIONS } from "@/dev-preview/shell.fixture";
import { TaskPreview } from "@/dev-preview/tasks-preview";
import { parseOpenTask } from "@/features/nhiem-vu/task-link";

/**
 * `/xem-thu/nhiem-vu` — DEV-ONLY screenshot preview of Quản lý nhiệm vụ (ADR 0068 lần 6 #10): the real
 * shell and the real `SoNhiemVu` on fixture data, no backend, no login.
 *   `?che-do=kanban|danh-sach|so-theo-doi`        the view (Kanban by default, as on the real screen)
 *   `?modal=giao-viec|nhap-excel|xoa-nhieu`       one dialog (`xoa-nhieu` ticks two tasks first)
 *   `?chon=2`                                     two tasks ticked — the bulk-delete bar
 *   `?task=NV105`                                 the detail open on a task — the REAL page's own word
 *   `?menu=day-du`, `?sidebar=thu-gon`, `?toast=1`, `?menu-tai-khoan=1`, `?chuong=1`  as every preview
 * In a production build this page is a 404 (`preview-gate.ts`).
 */
export const dynamic = "force-dynamic";

export const metadata = { title: "Xem thử · Quản lý nhiệm vụ", robots: { index: false, follow: false } };

export default async function TaskPreviewPage({ searchParams }: { searchParams: PreviewSearchParams }) {
  if (!devPreviewEnabled()) notFound();
  const q = await searchParams;
  const modal = previewTaskModal(q.modal);
  return (
    <PreviewShell fullMenu={previewFullMenu(q.menu)} permissions={PREVIEW_TASK_PERMISSIONS} person="lanh-dao">
      <TaskPreview
        view={previewTaskView(q["che-do"])}
        modal={modal}
        select={previewTaskSelect(q.chon, modal)}
        openTask={parseOpenTask(q)}
      />
    </PreviewShell>
  );
}
