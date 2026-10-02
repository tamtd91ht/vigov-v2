import { ScanLine } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { PendingButton, PendingMarker, PendingTab } from "@/components/ui/pending-feature";
import { Tab, TabList } from "@/components/ui/tabs";

import { PHAN_CHUA_DUNG, type PendingPart } from "./nhan-van-ban";

/**
 * The parts of `docs/ui-ux/05-van-ban-don-thu.md` the Văn bản & Đơn thư screen cannot build yet,
 * drawn at their spec position as the control they will be, disabled, with a "?" (ADR 0068 §14).
 * NOTHING HERE CALLS A SERVER: there is no handler to pass, on purpose.
 *
 * Descriptions come from `PHAN_CHUA_DUNG` by name, never a second sentence written here: two copies
 * of one reason drift, and the stale one is what a staff member reads.
 */
function part(name: string): PendingPart {
  const found = PHAN_CHUA_DUNG.find((p) => p.ten === name);
  // A renamed entry must fail loudly in tests, not draw a "?" that explains nothing.
  if (found === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry named "${name}"`);
  return found;
}

export const DOCUMENT_TAB_ID = "tab-van-ban";
export const DOCUMENT_PANEL_ID = "khung-tab-van-ban";

/**
 * Spec §2 tab bar under the PageHeader. The live tab is the two registers that exist today
 * (incoming, then outgoing — still one after the other inside the panel, as before); "Đơn thư công
 * dân" and "Báo cáo" are disabled tabs with "?". With a single live tab there is no arrow-key
 * handling to write, and the disabled tabs are out of the tab order (`PendingTab`).
 *
 * No icons on the pending tabs: this renders from a server page, and `PendingTab` is a client
 * component — a component-valued prop cannot cross that boundary.
 */
export function DocumentTabs({ children }: { children: ReactNode }) {
  return (
    <>
      <TabList aria-label="Phần của màn Văn bản & đơn thư" className="mb-4">
        <Tab selected id={DOCUMENT_TAB_ID} aria-controls={DOCUMENT_PANEL_ID}>
          Văn bản
        </Tab>
        <PendingTab info={part("Đơn thư công dân")} />
        <PendingTab info={part("Báo cáo")} />
      </TabList>
      <div role="tabpanel" id={DOCUMENT_PANEL_ID} aria-labelledby={DOCUMENT_TAB_ID} className="min-w-0">
        {children}
      </div>
    </>
  );
}

/** Spec §2 / §3.4: the "⬚ Quét & OCR" PageHeader button, disabled. */
export function ScanOcrButton() {
  return <PendingButton info={part("Quét & OCR")} icon={<ScanLine aria-hidden="true" />} />;
}

/**
 * Target statuses of the incoming-document lifecycle the customer settled on 30/09/2026 (Nghị định
 * 30/2020: Đã vào sổ → Chờ trình/phân luồng → Đã chuyển xử lý → Đang xử lý → Hoàn thành) — the four
 * after "Đã vào sổ". NOT the petition-letter labels the spec drew (§3.5): that decision says incoming
 * documents do not borrow them. Labels only — no status code is sent anywhere from here.
 */
const STATUS_TARGETS = ["Chờ trình/phân luồng", "Đã chuyển xử lý", "Đang xử lý", "Hoàn thành"] as const;

/**
 * Spec §3.5 status row: four buttons labelled "chuyển sang", disabled, ONE "?" for the row — four
 * identical markers would say the same sentence four times and crowd the row at 320px.
 */
export function StatusChangeRow() {
  return (
    <div role="group" aria-labelledby="nhan-chuyen-trang-thai" className="flex min-w-0 flex-col gap-2" data-pending="">
      <div className="flex items-center gap-1.5">
        <span id="nhan-chuyen-trang-thai" className="text-xs font-semibold text-ink-500">
          Chuyển trạng thái
        </span>
        <PendingMarker info={part("Chuyển trạng thái văn bản đến")} />
      </div>
      <div className="grid min-w-0 grid-cols-2 gap-2 sm:grid-cols-4">
        {STATUS_TARGETS.map((s) => (
          // The spec draws the status above "chuyển sang"; the name reads in speaking order.
          <Button
            key={s}
            type="button"
            variant="secondary"
            disabled
            aria-label={`Chuyển sang ${s}`}
            className="h-auto min-w-0 flex-col gap-0.5 py-2"
          >
            <span className="text-sm font-semibold whitespace-normal">{s}</span>
            <span className="text-xs font-normal text-ink-500">chuyển sang</span>
          </Button>
        ))}
      </div>
    </div>
  );
}
