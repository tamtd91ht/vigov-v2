"use client";

import { Copy } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { QrCode } from "@/components/qr-code";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { SkeletonRows } from "@/components/ui/skeleton";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { getLaunchLinks, type LaunchLink, type LaunchLinks, type LaunchLinkUnavailable } from "@/lib/api";
import { launchLinkError } from "@/lib/errors";

/**
 * "Mã QR mở Mini App" on a commune's page (owner 04/10/2026, ADR 0073 #5; owner 06/10/2026: "QR từ
 * đâu thì mở app từ đó"): EVERY link the commune has, side by side, so the operator chooses which QR to
 * print — the shared ViHAT app's (for the commune's PRIMARY domain), which every active commune can
 * print while its own app waits for Zalo, and the commune's own running app's when it has one. The
 * server builds both; a link it could not build is listed with its reason instead of silently missing.
 * Each is drawn as a QR in the browser by the existing `components/qr-code.tsx` (SVG, no third-party
 * service). Shown only with `ops.qr.issue`.
 *
 * NOT TRAILED, by the owner's decision (ADR 0048 §30/09 #9): issuing a QR is not a write. The links
 * are fetched when the card mounts; the detail page remounts it when the running own App ID changes.
 *
 * NO "download PNG": the QR component draws SVG only and has no export; a download added here would
 * be a second QR path nobody reviewed. Each URL is shown with a copy button, so the link itself can
 * be handed to whoever prints.
 */

export const SHARED_APP_PAGE = "/mini-app-dung-chung";

/**
 * Which app a QR opens, from the server's `source`: `chung` = the shared ViHAT app, `rieng` = the
 * commune's own running app. An unknown value is shown as it is, never guessed.
 */
export function launchSourceLabel(source: string): string {
  switch (source) {
    case "chung":
      return "Mở bằng app ViHAT";
    case "rieng":
      return "Mở bằng app riêng của xã";
    default:
      return `Nguồn không xác định: ${source}`;
  }
}

export type LaunchLinkState =
  | { status: "loading" }
  | { status: "ready"; links: LaunchLinks }
  | { status: "error"; message: string; toSharedAppPage: boolean };

function LaunchLinkItem({ link, onCopy }: { link: LaunchLink; onCopy: (url: string) => void }) {
  const own = link.source === "rieng";
  const label = launchSourceLabel(link.source);
  return (
    <li className="flex min-w-0 flex-1 flex-col gap-3 rounded-lg border border-line p-4">
      <h3 className="m-0 text-sm font-semibold text-ink-900">{label}</h3>
      <QrCode value={link.url} label={`Mã QR: ${label}`} />
      <dl className="m-0 flex flex-col gap-3">
        {/* A `rieng` link does not carry the domain (the own app has its commune built in), and the
            server sends "" when the commune has no primary domain: show the row only when there is one. */}
        {link.domain !== "" ? (
          <div className="flex min-w-0 flex-col gap-1">
            <dt className="text-xs font-medium text-ink-500">Tên miền chính của xã</dt>
            <dd className="m-0 font-mono text-sm break-all text-ink-900">{link.domain}</dd>
          </div>
        ) : null}
        <div className="flex min-w-0 flex-col gap-1">
          <dt className="text-xs font-medium text-ink-500">{own ? "App ID Mini App riêng của xã" : "App ID Mini App dùng chung"}</dt>
          <dd className="m-0 font-mono text-sm break-all text-ink-900">{link.app_id}</dd>
        </div>
        <div className="flex min-w-0 flex-col gap-1">
          <dt className="text-xs font-medium text-ink-500">Liên kết</dt>
          <dd className="m-0 font-mono text-[13px] break-all text-ink-900">{link.url}</dd>
        </div>
      </dl>
      <div>
        <Button type="button" variant="secondary" onClick={() => onCopy(link.url)} icon={<Copy aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
          Chép liên kết
        </Button>
      </div>
      <p className="m-0 text-[13px] text-ink-500">
        {own
          ? "Mã QR mở thẳng Mini App riêng của xã. Nếu xã đổi hoặc gỡ App ID này, mã đã in sẽ không còn dùng được và cần in lại."
          : "Mã QR mở app ViHAT (Mini App dùng chung) và đưa người dân vào đúng xã này. Nếu App ID dùng chung được đổi, mã đã in sẽ không còn dùng được và cần in lại."}
      </p>
    </li>
  );
}

function UnavailableItem({ item }: { item: LaunchLinkUnavailable }) {
  // The server's code is mapped to the console's own Vietnamese next step; its message is the
  // fallback for a code this console does not know yet.
  const known = launchLinkError({ code: item.code });
  return (
    <li className="flex min-w-0 flex-1 flex-col gap-2 rounded-lg border border-dashed border-line p-4">
      <h3 className="m-0 text-sm font-semibold text-ink-900">{launchSourceLabel(item.source)}</h3>
      <p className="m-0 text-sm text-ink-700">Chưa tạo được mã QR này. {known?.text ?? item.message}</p>
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
  onCopy,
  copyNotice,
}: {
  state: LaunchLinkState;
  onCopy: (url: string) => void;
  copyNotice: string;
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
  const { links, unavailable } = state.links;
  return (
    <div className="flex flex-col gap-3">
      <ul className="m-0 flex list-none flex-col gap-4 p-0 lg:flex-row lg:items-start">
        {links.map((l) => (
          <LaunchLinkItem key={l.source + l.app_id} link={l} onCopy={onCopy} />
        ))}
        {unavailable.map((u) => (
          <UnavailableItem key={u.source} item={u} />
        ))}
      </ul>
      <p className="m-0 min-h-5 text-[13px] text-ink-500" role="status" aria-live="polite">
        {copyNotice}
      </p>
    </div>
  );
}

export function LaunchLinkCard({ communeId }: { communeId: string }) {
  const guarded = useGuardedError();
  const [state, setState] = useState<LaunchLinkState>({ status: "loading" });
  const [copyNotice, setCopyNotice] = useState("");

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
      setCopyNotice("Đã chép liên kết.");
    } catch {
      setCopyNotice("Trình duyệt không cho chép. Hãy chọn và chép liên kết bằng tay.");
    }
  }

  return <LaunchLinkView state={state} onCopy={copy} copyNotice={copyNotice} />;
}
