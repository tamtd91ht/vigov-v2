"use client";

import { Download, Printer } from "lucide-react";
import { useEffect, useState } from "react";
import { createPortal, flushSync } from "react-dom";
import { toast } from "sonner";

import { useCauHinhXa } from "@/components/cau-hinh-xa";
import { downloadQrPng, QrCode } from "@/components/qr-code";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { getCommuneLaunchLink } from "@/lib/api/commune-launch-link";

/**
 * "Mã QR mở Mini App của xã" in Cấu hình → Kênh Zalo (owner 13/10/2026, ADR 0070 §Sửa đổi 13/10/2026):
 * exactly ONE QR — the link the server picks for this commune — and NOTHING about which app it opens:
 * no App ID, no raw link, no source label, no word about a shared app. The people who print and scan it
 * must see the commune's app as the commune's own. No "Chép liên kết" either (owner 13/10/2026): the
 * link carries ViHAT's App ID; copying it is the operator console's action only.
 *
 * Mounted only inside `ZaloChannelTab`'s `admin.zalo_channel` gate (since 15/10/2026; was `admin.lookup`),
 * so no second gate; platform checks the route's own key again. NOT TRAILED (ADR 0070 §13/10 #5): viewing, downloading and printing are
 * not writes.
 *
 * Mirrors `platform-admin/src/features/communes/launch-link-card.tsx` (bf1fa420) for the PNG and the
 * print sheet, so both surfaces print the same page.
 */

/** `<html>` carries this class only while the print dialog is open; `globals.css` then prints the QR
 *  sheet alone. Without it a plain Ctrl+P prints the page as it is. */
export const QR_PRINT_CLASS = "qr-printing";

export const SCAN_INSTRUCTION = "Quét mã bằng Zalo để mở ứng dụng của xã";

/** No commune name, no domain, no id: the browser holds no tenant id by design (`cau-hinh-xa-hien-thi.ts`),
 *  and a name or domain in a file name is how a file gets traced to a place. */
export const QR_FILE_NAME = "qr-mini-app.png";

// Same card as the other sections of the tab (`zalo-channel-tab.tsx` SECTION / TITLE / DESCRIPTION).
const SECTION = "border-line shadow-card rounded-card m-0 min-w-0 border border-solid bg-white p-5";
const TITLE = "text-navy m-0 text-[14px] font-bold";
const DESCRIPTION = "text-ink-muted m-0 mt-1 max-w-2xl text-[12.5px]";

export type LaunchQrState = { status: "loading" } | { status: "ready"; url: string } | { status: "refused"; message: string };

export type LaunchQrActions = {
  onDownload: (url: string) => void;
  onPrint: (url: string) => void;
};

/** Pure rendering, exported so the tests read the markup. */
export function CommuneLaunchQrView({
  state,
  communeName,
  actions,
}: {
  state: LaunchQrState;
  communeName: string;
  actions: LaunchQrActions;
}) {
  return (
    <section className={SECTION} aria-labelledby="zalo-launch-qr-title" data-launch-qr="">
      <h3 id="zalo-launch-qr-title" className={TITLE}>
        Mã QR mở Mini App của xã
      </h3>
      <p className={DESCRIPTION}>
        In mã này để người dân quét bằng Zalo và mở ứng dụng của xã.
      </p>
      {state.status === "loading" ? (
        <>
          <p role="status" className="an-thi-giac">
            Đang tạo mã QR…
          </p>
          <Skeleton className="mt-4 h-40 w-40" />
        </>
      ) : state.status === "refused" ? (
        <p className="text-danger m-0 mt-3 text-[12.5px] font-medium" role="alert">
          {state.message}
        </p>
      ) : (
        <div className="mt-4 flex min-w-0 flex-col gap-4 sm:flex-row sm:items-start">
          <QrCode value={state.url} label={`Mã QR mở ứng dụng của ${communeName}`} />
          <div className="flex min-w-0 flex-1 flex-col gap-3">
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="primary"
                icon={<Download aria-hidden="true" focusable="false" />}
                onClick={() => actions.onDownload(state.url)}
              >
                Tải ảnh PNG
              </Button>
              <Button
                type="button"
                variant="outline"
                className="bg-white"
                icon={<Printer aria-hidden="true" focusable="false" />}
                onClick={() => actions.onPrint(state.url)}
              >
                In / Lưu PDF
              </Button>
            </div>
            <p className="text-ink-muted m-0 text-[12px]">
              Khi ứng dụng của xã thay đổi, mã đã in có thể không còn dùng được; khi đó hãy tải và in lại mã ở đây.
            </p>
          </div>
        </div>
      )}
    </section>
  );
}

/** The A4 sheet `window.print()` prints: the commune's name, the QR, one line. Mounted into `<body>`
 *  only while printing; `hidden print:flex` keeps it off the screen even for that moment. */
export function QrPrintSheet({ communeName, url }: { communeName: string; url: string }) {
  return (
    <div className="qr-print-sheet hidden flex-col items-center gap-8 pt-[10mm] text-center text-black print:flex">
      <h1 className="m-0 text-[22pt] leading-tight font-bold">{communeName}</h1>
      <QrCode
        value={url}
        label={`Mã QR mở ứng dụng của ${communeName}`}
        className="w-[120mm] max-w-none rounded-none border-0"
      />
      <p className="m-0 text-[16pt]">{SCAN_INSTRUCTION}</p>
    </div>
  );
}

export function CommuneLaunchQrSection() {
  // The name the header shows, from the server-side commune config (rule 1 inv. 10) — never typed here.
  const communeName = useCauHinhXa().displayName;
  const [state, setState] = useState<LaunchQrState>({ status: "loading" });
  const [printUrl, setPrintUrl] = useState<string | null>(null);

  useEffect(() => {
    let gone = false;
    getCommuneLaunchLink().then((r) => {
      if (gone) return;
      setState(r.ok ? { status: "ready", url: r.data.url } : { status: "refused", message: r.message });
    });
    return () => {
      gone = true;
    };
  }, []);

  async function download(url: string) {
    try {
      await downloadQrPng(url, QR_FILE_NAME);
      toast.success("Đã tải ảnh mã QR.");
    } catch {
      toast.error("Trình duyệt không tạo được ảnh mã QR. Hãy dùng nút In / Lưu PDF.");
    }
  }

  // From the click, not from an effect: an effect calling window.print() runs twice under React's
  // development double-invoke and opens the dialog twice. flushSync mounts the sheet before the dialog
  // snapshots the page; `afterprint` takes it down again whether the user printed or cancelled.
  function print(url: string) {
    flushSync(() => setPrintUrl(url));
    const root = document.documentElement;
    root.classList.add(QR_PRINT_CLASS);
    window.addEventListener(
      "afterprint",
      () => {
        root.classList.remove(QR_PRINT_CLASS);
        setPrintUrl(null);
      },
      { once: true },
    );
    window.print();
  }

  return (
    <>
      <CommuneLaunchQrView
        state={state}
        communeName={communeName}
        actions={{ onDownload: (u) => void download(u), onPrint: print }}
      />
      {printUrl !== null ? createPortal(<QrPrintSheet communeName={communeName} url={printUrl} />, document.body) : null}
    </>
  );
}
