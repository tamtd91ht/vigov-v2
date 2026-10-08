import { notFound } from "next/navigation";

import { DocumentsPreview } from "@/dev-preview/documents-preview";
import { devPreviewEnabled } from "@/dev-preview/preview-gate";
import {
  previewDocumentDrawer,
  previewDocumentDuplicate,
  previewDocumentFlag,
  previewDocumentFoldOpen,
  previewDocumentMeasure,
  previewDocumentModal,
  previewDocumentScrollRight,
  previewDocumentState,
  previewDocumentTab,
  previewFullMenu,
  type PreviewSearchParams,
} from "@/dev-preview/preview-params";
import { PreviewShell } from "@/dev-preview/preview-shell";
import { PREVIEW_DOCUMENT_PERMISSIONS } from "@/dev-preview/shell.fixture";

/**
 * `/xem-thu/van-ban` — DEV-ONLY screenshot preview of Văn bản & Đơn thư (ADR 0068 lần 6 #10, ADR 0078):
 * the real shell and the real `DocumentWorkspace` on fixture data, no backend, no login.
 *   `?tab=den|di|don-thu|bao-cao`            the tab (none: the screen's own default)
 *   `?modal=vao-so-den|cap-so-di|vao-so-don` the intake / issue / letter-booking dialog (its tab first)
 *   `?dup=1`                                 with `modal=vao-so-don`: a name + summary typed, so the
 *                                            duplicate warning shows
 *   `?buoc=1`, `?sua-nguoi-gui=1`, `?loc=1`, `?cuon=1`  letter-tab states, see `previewDocumentFlag`
 *   `?drawer=<id>`                          the detail of one incoming document (e.g. the fixture's
 *                                            `01PREVIEWVBDEN000000000011`, three routings) or of one
 *                                            citizen letter (`01PREVIEWDONTHU…`, `letters.fixture.ts`)
 *   `?state=loading|empty|error`             what the two registers' reads answer
 *   `?scroll=right`, `?them=1`, `?do-cao=1`  see `preview-params.ts`
 *   `?menu=day-du`, `?sidebar=thu-gon`, `?toast=1`, `?menu-tai-khoan=1`, `?chuong=1`  as every preview
 * In a production build this page is a 404 (`preview-gate.ts`).
 */
export const dynamic = "force-dynamic";

export const metadata = { title: "Xem thử · Văn bản & đơn thư", robots: { index: false, follow: false } };

export default async function DocumentsPreviewPage({ searchParams }: { searchParams: PreviewSearchParams }) {
  if (!devPreviewEnabled()) notFound();
  const q = await searchParams;
  return (
    <PreviewShell fullMenu={previewFullMenu(q.menu)} permissions={PREVIEW_DOCUMENT_PERMISSIONS} person="lanh-dao">
      <DocumentsPreview
        tab={previewDocumentTab(q.tab)}
        modal={previewDocumentModal(q.modal)}
        drawer={previewDocumentDrawer(q.drawer)}
        state={previewDocumentState(q.state)}
        scrollRight={previewDocumentScrollRight(q.scroll)}
        foldOpen={previewDocumentFoldOpen(q.them)}
        measure={previewDocumentMeasure(q["do-cao"])}
        duplicate={previewDocumentDuplicate(q.dup)}
        statusStep={previewDocumentFlag(q.buoc)}
        editSender={previewDocumentFlag(q["sua-nguoi-gui"])}
        statusFilter={previewDocumentFlag(q.loc)}
        scrollEnd={previewDocumentFlag(q.cuon)}
      />
    </PreviewShell>
  );
}
