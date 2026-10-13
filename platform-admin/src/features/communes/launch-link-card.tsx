"use client";

import { Copy, Download, Printer } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";
import { createPortal, flushSync } from "react-dom";

import { downloadQrPng, QrCode } from "@/components/qr-code";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { getLaunchLinks, type LaunchLink, type LaunchLinks, type LaunchLinkUnavailable } from "@/lib/api";
import { launchLinkError } from "@/lib/errors";

/**
 * "Mã QR mở Mini App" on a commune's page (owner 04/10/2026, ADR 0073 #5). Owner 13/10/2026: the card
 * shows exactly ONE QR — the link the server picks for this commune — and NOTHING about which app it
 * opens: no App ID, no domain, no source label, no sentence about a shared or transitional app. The
 * people who print and scan it must see the commune's app as the commune's own. The server sends at
 * most one link; when it sends none, its reasons are shown instead of a silently empty card. One
 * exception, owner 13/10/2026 (ADR 0070 bổ sung): `own_mini_app_unpublished` is said under the QR,
 * because this card is the operator's and the operator is who marks the app published. The QR is
 * drawn in the browser by `components/qr-code.tsx` (SVG, no third-party service). Shown only with
 * `ops.qr.issue`.
 *
 * NOT TRAILED, by the owner's decision (ADR 0048 §30/09 #9): issuing a QR is not a write. The link is
 * fetched when the card mounts; the detail page remounts it when the running own App ID changes.
 *
 * "Tải ảnh PNG" and "In / Lưu PDF" (owner 13/10/2026, reversing the earlier "copy the link only"):
 * a commune prints this code on paper, and handing a printer a URL to turn into a QR again was a
 * second, unreviewed encoder outside this console. Both actions draw from the one `qrMatrix` path in
 * `qr-code.tsx`: the PNG on a canvas, the print sheet as the same SVG. The PNG's file name carries the
 * commune's opaque id only — never a name or a domain a person could be read from.
 */

export const SHARED_APP_PAGE = "/mini-app-dung-chung";

/** `<html>` carries this class only while the print dialog is open; `globals.css` then prints the
 *  QR sheet alone. Without it a plain Ctrl+P prints the page as it is. */
export const QR_PRINT_CLASS = "qr-printing";

export const SCAN_INSTRUCTION = "Quét mã bằng Zalo để mở ứng dụng của xã";

/** operator_launch.go `codeOwnMiniAppUnpublished`. */
export const OWN_MINI_APP_UNPUBLISHED = "own_mini_app_unpublished";

/** Operator card only; never on the print sheet, which carries no app wording. */
export const OWN_APP_UNPUBLISHED_NOTICE =
  "Mini App riêng của xã chưa được đánh dấu đã phát hành trên Zalo, nên mã QR này đang mở Mini App dùng chung. Khi Zalo đã duyệt và đã phát hành app riêng, chọn “Đánh dấu đã phát hành” ở mục Mini App riêng của xã; mã QR sẽ chuyển sang app riêng và cần tải, in lại.";

/** `qr-mini-app-<commune id>.png`. The id is an opaque ULID (rule 1 inv. 2): no personal data, and
 *  stable across a renamed commune. Anything outside [a-z0-9-] is dropped so no path or odd character
 *  reaches the save dialog. */
export function qrFileName(communeId: string): string {
  return `qr-mini-app-${communeId.toLowerCase().replace(/[^a-z0-9-]/g, "")}.png`;
}

export type LaunchLinkState =
  | { status: "loading" }
  | { status: "ready"; links: LaunchLinks }
  | { status: "error"; message: string; toSharedAppPage: boolean };

export type LaunchLinkActions = {
  onCopy: (url: string) => void;
  onDownload: (url: string) => void;
  onPrint: (url: string) => void;
};

function LaunchLinkItem({ link, communeName, actions }: { link: LaunchLink; communeName: string; actions: LaunchLinkActions }) {
  return (
    <div className="flex min-w-0 flex-col gap-4 sm:flex-row sm:items-start">
      <QrCode value={link.url} label={`Mã QR mở ứng dụng của ${communeName}`} />
      <div className="flex min-w-0 flex-1 flex-col gap-3">
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="primary" onClick={() => actions.onDownload(link.url)} icon={<Download aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
            Tải ảnh PNG
          </Button>
          <Button type="button" variant="secondary" onClick={() => actions.onPrint(link.url)} icon={<Printer aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
            In / Lưu PDF
          </Button>
          <Button type="button" variant="secondary" onClick={() => actions.onCopy(link.url)} icon={<Copy aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
            Chép liên kết
          </Button>
        </div>
        <dl className="m-0 flex flex-col gap-1">
          <dt className="text-xs font-medium text-ink-500">Liên kết</dt>
          <dd className="m-0 font-mono text-[13px] break-all text-ink-900">{link.url}</dd>
        </dl>
        <p className="m-0 text-[13px] text-ink-500">
          Người dân quét mã bằng Zalo để mở ứng dụng của xã. Khi ứng dụng của xã thay đổi, mã đã in có thể không còn dùng được; khi đó cần tải và in lại mã trên trang này.
        </p>
      </div>
    </div>
  );
}

/** The A4 sheet `window.print()` prints: the commune's name as the page shows it, the QR, one line.
 *  Mounted into `<body>` only while printing (see `LaunchLinkCard`); `hidden print:flex` keeps it off
 *  the screen even for that moment. */
export function QrPrintSheet({ communeName, url }: { communeName: string; url: string }) {
  return (
    <div className="qr-print-sheet hidden flex-col items-center gap-8 pt-[10mm] text-center text-black print:flex">
      <h1 className="m-0 text-[22pt] leading-tight font-bold">{communeName}</h1>
      <QrCode value={url} label={`Mã QR mở ứng dụng của ${communeName}`} className="w-[120mm] max-w-none rounded-none border-0" />
      <p className="m-0 text-[16pt]">{SCAN_INSTRUCTION}</p>
    </div>
  );
}

function UnavailableItem({ item }: { item: LaunchLinkUnavailable }) {
  // The server's code is mapped to the console's own Vietnamese next step; its message is the
  // fallback for a code this console does not know yet. No source label (owner 13/10/2026). The
  // link to the shared-app page stays: it is the operator's only way to fix that refusal.
  const known = launchLinkError({ code: item.code });
  return (
    <li className="flex min-w-0 flex-col gap-2 rounded-lg border border-dashed border-line p-4">
      <p className="m-0 text-sm text-ink-700">Chưa tạo được mã QR. {known?.text ?? item.message}</p>
      {known?.toSharedAppPage ? (
        <Link href={SHARED_APP_PAGE} className="text-sm font-semibold text-brand-700 underline underline-offset-2">
          Mở trang Mini App dùng chung
        </Link>
      ) : null}
    </li>
  );
}

export function LaunchLinkView({
  state,
  communeName,
  actions,
  notice,
}: {
  state: LaunchLinkState;
  communeName: string;
  actions: LaunchLinkActions;
  notice: string;
}) {
  if (state.status === "loading") {
    return (
      <div>
        <p role="status" className="sr-only">
          Đang tạo liên kết mở Mini App…
        </p>
        <SkeletonRows rows={2} columns={2} />
      </div>
    );
  }
  if (state.status === "error") {
    return (
      <div className="flex flex-col items-center gap-2">
        <ErrorState role="alert" title="Chưa tạo được mã QR" message={state.message} className="py-6" />
        {state.toSharedAppPage ? (
          <Link href={SHARED_APP_PAGE} className="text-sm font-semibold text-brand-700 underline underline-offset-2">
            Mở trang Mini App dùng chung
          </Link>
        ) : null}
      </div>
    );
  }
  // One QR, the server's choice (owner 13/10/2026). A second entry, should a server ever send one,
  // is not shown: two codes on the card is exactly the choice the owner took away.
  const link = state.links.links[0];
  const { unavailable } = state.links;
  // The one reason shown BESIDE a link: the QR is the shared one because the commune's own app is not
  // marked published (ADR 0070 bổ sung 13/10/2026). Only the operator route sends it, and only the
  // operator can repair it — without the line the shared QR reads as a fault.
  const unpublished = link !== undefined && unavailable.some((u) => u.code === OWN_MINI_APP_UNPUBLISHED);
  return (
    <div className="flex flex-col gap-3">
      {link !== undefined ? (
        <>
          <LaunchLinkItem link={link} communeName={communeName} actions={actions} />
          {unpublished ? (
            <Notice tone="info">
              {OWN_APP_UNPUBLISHED_NOTICE}
            </Notice>
          ) : null}
        </>
      ) : unavailable.length > 0 ? (
        <ul className="m-0 flex list-none flex-col gap-3 p-0">
          {unavailable.map((u) => (
            <UnavailableItem key={u.source + u.code} item={u} />
          ))}
        </ul>
      ) : (
        <p className="m-0 text-sm text-ink-700">Chưa tạo được mã QR. Máy chủ không trả về liên kết nào; báo nhóm nền tảng.</p>
      )}
      <p className="m-0 min-h-5 text-[13px] text-ink-500" role="status" aria-live="polite">
        {notice}
      </p>
    </div>
  );
}

export function LaunchLinkCard({ communeId, communeName }: { communeId: string; communeName: string }) {
  const guarded = useGuardedError();
  const [state, setState] = useState<LaunchLinkState>({ status: "loading" });
  const [notice, setNotice] = useState("");
  const [printUrl, setPrintUrl] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    getLaunchLinks(communeId).then(
      (links) => {
        if (alive) setState({ status: "ready", links });
      },
      (err: unknown) => {
        if (!alive) return;
        let toSharedAppPage = false;
        const message = guarded(err, "tạo mã QR mở Mini App", (e) => {
          const known = launchLinkError(e);
          if (known) toSharedAppPage = known.toSharedAppPage;
          return known?.text ?? null;
        });
        if (message !== null) setState({ status: "error", message, toSharedAppPage });
      },
    );
    return () => {
      alive = false;
    };
  }, [communeId, guarded]);

  async function copy(url: string) {
    try {
      await navigator.clipboard.writeText(url);
      setNotice("Đã chép liên kết.");
    } catch {
      setNotice("Trình duyệt không cho chép. Hãy chọn và chép liên kết bằng tay.");
    }
  }

  async function download(url: string) {
    try {
      await downloadQrPng(url, qrFileName(communeId));
      setNotice("Đã tải ảnh mã QR.");
    } catch {
      setNotice("Trình duyệt không tạo được ảnh mã QR. Hãy dùng nút In / Lưu PDF.");
    }
  }

  // From the click, not from an effect: an effect calling window.print() runs twice under React's
  // development double-invoke and opens the dialog twice. flushSync mounts the sheet before the
  // dialog snapshots the page; `afterprint` takes it down again whether the user printed or cancelled.
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
      <LaunchLinkView state={state} communeName={communeName} actions={{ onCopy: copy, onDownload: download, onPrint: print }} notice={notice} />
      {printUrl !== null ? createPortal(<QrPrintSheet communeName={communeName} url={printUrl} />, document.body) : null}
    </>
  );
}
