"use client";

import { Copy } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { QrCode } from "@/components/qr-code";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { SkeletonRows } from "@/components/ui/skeleton";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { getLaunchLink, type LaunchLink } from "@/lib/api";
import { launchLinkError } from "@/lib/errors";

/**
 * "Mã QR mở Mini App" on a commune's page (owner 04/10/2026, ADR 0073 #5): the shared Mini App's
 * launch link for the commune's PRIMARY domain, drawn as a QR in the browser by the existing
 * `components/qr-code.tsx` (SVG, no third-party service). Shown only with `ops.qr.issue`.
 *
 * NOT TRAILED, by the owner's decision (ADR 0048 §30/09 #9): issuing a QR is not a write. The link is
 * fetched once when the card mounts.
 *
 * NO "download PNG": the QR component draws SVG only and has no export; a download added here would
 * be a second QR path nobody reviewed. The URL is shown with a copy button, so the link itself can
 * be handed to whoever prints.
 */

export const SHARED_APP_PAGE = "/mini-app-dung-chung";

export type LaunchLinkState =
  | { status: "loading" }
  | { status: "ready"; link: LaunchLink }
  | { status: "error"; message: string; toSharedAppPage: boolean };

export function LaunchLinkView({
  state,
  onCopy,
  copyNotice,
}: {
  state: LaunchLinkState;
  onCopy: () => void;
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
  const { link } = state;
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
      <QrCode value={link.url} label={`Mã QR mở Mini App cho tên miền ${link.domain}`} />
      <div className="flex min-w-0 flex-1 flex-col gap-3">
        <dl className="m-0 flex flex-col gap-3">
          <div className="flex min-w-0 flex-col gap-1">
            <dt className="text-xs font-medium text-ink-500">Tên miền chính của xã</dt>
            <dd className="m-0 font-mono text-sm break-all text-ink-900">{link.domain}</dd>
          </div>
          <div className="flex min-w-0 flex-col gap-1">
            <dt className="text-xs font-medium text-ink-500">App ID Mini App dùng chung</dt>
            <dd className="m-0 font-mono text-sm break-all text-ink-900">{link.app_id}</dd>
          </div>
          <div className="flex min-w-0 flex-col gap-1">
            <dt className="text-xs font-medium text-ink-500">Liên kết</dt>
            <dd className="m-0 font-mono text-[13px] break-all text-ink-900">{link.url}</dd>
          </div>
        </dl>
        <div>
          <Button type="button" variant="secondary" onClick={onCopy} icon={<Copy aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
            Chép liên kết
          </Button>
        </div>
        <p className="m-0 min-h-5 text-[13px] text-ink-500" role="status" aria-live="polite">
          {copyNotice}
        </p>
        <p className="m-0 text-[13px] text-ink-500">
          Mã QR mở Mini App dùng chung và đưa người dân vào đúng xã này. Nếu App ID dùng chung được đổi, mã đã in sẽ
          không còn dùng được và cần in lại.
        </p>
      </div>
    </div>
  );
}

export function LaunchLinkCard({ communeId }: { communeId: string }) {
  const guarded = useGuardedError();
  const [state, setState] = useState<LaunchLinkState>({ status: "loading" });
  const [copyNotice, setCopyNotice] = useState("");

  useEffect(() => {
    let alive = true;
    getLaunchLink(communeId).then(
      (link) => {
        if (alive) setState({ status: "ready", link });
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

  async function copy() {
    if (state.status !== "ready") return;
    try {
      await navigator.clipboard.writeText(state.link.url);
      setCopyNotice("Đã chép liên kết.");
    } catch {
      setCopyNotice("Trình duyệt không cho chép. Hãy chọn và chép liên kết bằng tay.");
    }
  }

  return <LaunchLinkView state={state} onCopy={copy} copyNotice={copyNotice} />;
}
