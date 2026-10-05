"use client";

import { ExternalLink, Link2Off, MessageCircle, QrCode, RotateCw, Send } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { Skeleton } from "@/components/ui/skeleton";
import { formatVietnamDateTime } from "@/features/noi-dung/nhan-noi-dung";
import type { KetQua } from "@/lib/api/goi";
import {
  createPairingCode,
  getCurrentZaloLink,
  sendZaloTestMessage,
  unlinkCurrentZalo,
  type ZaloLinkCurrent,
  type ZaloPairingCode,
} from "@/lib/api/zalo";

import {
  CARD_DESCRIPTION,
  CARD_TITLE,
  CHANNEL_OFF,
  CHANNEL_OFF_LINKED,
  CODE_EXPIRED,
  formatCountdown,
  LINKED_NOW,
  NOT_LINKED,
  PAIR_STEPS,
  POLL_INTERVAL_MS,
  safeHttpsHref,
  secondsLeft,
  shouldPoll,
  TEST_SENT,
  UNLINK_CONSEQUENCE,
  UNLINK_QUESTION,
  UNLINKED,
} from "./personal-zalo-model";

/**
 * "Nhận nhắc việc qua Zalo" — the signed-in staff member pairs THEIR OWN Zalo chat with the shared bot
 * (ADR 0074). Everything is the session's: no staff code, no id, no chat id is sent or shown.
 *
 * Pairing: ask for an 8-character code (alive 10 minutes), send it to the bot from Zalo, and this card
 * polls `GET zalo-links/current` every 5 s while the code is alive, stopping the moment it reads
 * `linked` — or the code expires, or the person leaves the page.
 *
 * Commune channel off: pairing and the test send are disabled with the reason. UNLINKING STAYS
 * AVAILABLE — taking one's own chat out of the system is never something the commune's switch blocks.
 */

type Message = { ok: boolean; text: string } | null;

export function PersonalZaloCard() {
  const [link, setLink] = useState<KetQua<ZaloLinkCurrent> | null>(null);
  const [code, setCode] = useState<ZaloPairingCode | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [busy, setBusy] = useState<"code" | "test" | "unlink" | null>(null);
  const [confirmUnlink, setConfirmUnlink] = useState(false);
  const [message, setMessage] = useState<Message>(null);

  const read = useCallback(() => {
    getCurrentZaloLink().then(setLink);
  }, []);

  useEffect(read, [read]);

  const linked = link !== null && link.ok && link.duLieu.linked;
  const polling = shouldPoll(code, linked, now);

  // One timer drives both the countdown (each second) and the poll (every POLL_INTERVAL_MS) — and it
  // exists only while a code is alive, so an idle page sends nothing.
  useEffect(() => {
    if (!polling) return;
    let ticks = 0;
    const perPoll = Math.max(1, Math.round(POLL_INTERVAL_MS / 1000));
    const id = setInterval(() => {
      setNow(Date.now());
      ticks += 1;
      if (ticks % perPoll !== 0) return;
      getCurrentZaloLink().then((r) => {
        // A failed poll is not shown: the next one retries, and the code's own countdown still runs.
        if (r.ok && r.duLieu.linked) {
          setLink(r);
          setCode(null);
          setMessage({ ok: true, text: LINKED_NOW });
        }
      });
    }, 1000);
    return () => clearInterval(id);
  }, [polling]);

  async function requestCode() {
    setBusy("code");
    setMessage(null);
    const r = await createPairingCode();
    setBusy(null);
    if (!r.ok) return setMessage({ ok: false, text: r.thongBao });
    setNow(Date.now());
    setCode(r.duLieu);
  }

  async function sendTest() {
    setBusy("test");
    setMessage(null);
    const r = await sendZaloTestMessage();
    setBusy(null);
    setMessage(r.ok ? { ok: true, text: TEST_SENT } : { ok: false, text: r.thongBao });
  }

  async function unlink() {
    setBusy("unlink");
    setMessage(null);
    const r = await unlinkCurrentZalo();
    setBusy(null);
    setConfirmUnlink(false);
    if (!r.ok) return setMessage({ ok: false, text: r.thongBao });
    setMessage({ ok: true, text: UNLINKED });
    read();
  }

  return (
    <PersonalZaloView
      link={link}
      code={code}
      now={now}
      busy={busy}
      confirmUnlink={confirmUnlink}
      message={message}
      onRetry={() => {
        setLink(null);
        read();
      }}
      onRequestCode={() => void requestCode()}
      onTest={() => void sendTest()}
      onAskUnlink={() => setConfirmUnlink(true)}
      onCancelUnlink={() => setConfirmUnlink(false)}
      onUnlink={() => void unlink()}
    />
  );
}

/** Pure rendering, exported so tests read every state's markup. */
export function PersonalZaloView({
  link,
  code,
  now,
  busy,
  confirmUnlink,
  message,
  onRetry,
  onRequestCode,
  onTest,
  onAskUnlink,
  onCancelUnlink,
  onUnlink,
}: {
  link: KetQua<ZaloLinkCurrent> | null;
  code: ZaloPairingCode | null;
  now: number;
  busy: "code" | "test" | "unlink" | null;
  confirmUnlink: boolean;
  message: Message;
  onRetry: () => void;
  onRequestCode: () => void;
  onTest: () => void;
  onAskUnlink: () => void;
  onCancelUnlink: () => void;
  onUnlink: () => void;
}) {
  return (
    <Card as="section" aria-labelledby="zalo-ca-nhan-title" className="page--form">
      <CardHeader>
        <div className="min-w-0 flex-1 basis-64">
          <CardTitle id="zalo-ca-nhan-title" className="flex items-center gap-2">
            <MessageCircle aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
            {CARD_TITLE}
          </CardTitle>
          <p className="m-0 mt-1 text-[13px] text-ink-500">{CARD_DESCRIPTION}</p>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {link === null ? (
          <>
            <p role="status" className="an-thi-giac">
              Đang tải trạng thái ghép nối Zalo…
            </p>
            <Skeleton className="h-5 w-56" />
            <Skeleton className="h-9 w-40" />
          </>
        ) : !link.ok ? (
          <ErrorState role="alert" title="Chưa tải được trạng thái ghép nối Zalo" message={link.thongBao} onRetry={onRetry} />
        ) : (
          <Body
            link={link.duLieu}
            code={code}
            now={now}
            busy={busy}
            confirmUnlink={confirmUnlink}
            onRequestCode={onRequestCode}
            onTest={onTest}
            onAskUnlink={onAskUnlink}
            onCancelUnlink={onCancelUnlink}
            onUnlink={onUnlink}
          />
        )}
        {message !== null &&
          (message.ok ? (
            <p role="status" className="m-0 text-sm font-medium text-success-600">
              {message.text}
            </p>
          ) : (
            <p className="thong-bao-loi m-0" role="alert">
              {message.text}
            </p>
          ))}
      </CardContent>
    </Card>
  );
}

function Body({
  link,
  code,
  now,
  busy,
  confirmUnlink,
  onRequestCode,
  onTest,
  onAskUnlink,
  onCancelUnlink,
  onUnlink,
}: {
  link: ZaloLinkCurrent;
  code: ZaloPairingCode | null;
  now: number;
  busy: "code" | "test" | "unlink" | null;
  confirmUnlink: boolean;
  onRequestCode: () => void;
  onTest: () => void;
  onAskUnlink: () => void;
  onCancelUnlink: () => void;
  onUnlink: () => void;
}) {
  const off = !link.channel_enabled;

  if (link.linked) {
    return (
      <>
        <div className="flex flex-wrap items-center gap-2 text-sm text-ink-900">
          <Badge tone="success">Đã ghép nối</Badge>
          <span>
            {link.bot_name ? `Với bot “${link.bot_name}”` : "Với bot của hệ thống"}
            {link.linked_at ? ` từ ${formatVietnamDateTime(link.linked_at)}` : ""}
          </span>
        </div>
        {off && <Notice tone="neutral">{CHANNEL_OFF_LINKED}</Notice>}
        {confirmUnlink ? (
          <ConfirmDialog
            title={UNLINK_QUESTION}
            tone="danger"
            icon={Link2Off}
            actions={
              <>
                <Button type="button" variant="danger" onClick={onUnlink} disabled={busy !== null} aria-busy={busy === "unlink"}>
                  {busy === "unlink" ? "Đang gỡ…" : "Gỡ ghép nối"}
                </Button>
                <Button type="button" variant="secondary" onClick={onCancelUnlink} disabled={busy !== null}>
                  Huỷ
                </Button>
              </>
            }
          >
            <p className="m-0">{UNLINK_CONSEQUENCE}</p>
          </ConfirmDialog>
        ) : (
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="secondary"
              onClick={onTest}
              disabled={off || busy !== null}
              aria-busy={busy === "test"}
              icon={<Send aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            >
              {busy === "test" ? "Đang gửi…" : "Gửi tin thử"}
            </Button>
            <Button
              type="button"
              variant="danger"
              onClick={onAskUnlink}
              disabled={busy !== null}
              icon={<Link2Off aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            >
              Gỡ ghép nối
            </Button>
          </div>
        )}
      </>
    );
  }

  if (off) {
    return (
      <>
        <Notice tone="neutral">{CHANNEL_OFF}</Notice>
        <div>
          <Button type="button" variant="primary" disabled icon={<QrCode aria-hidden="true" focusable="false" strokeWidth={1.8} />}>
            Lấy mã ghép nối
          </Button>
        </div>
      </>
    );
  }

  const left = code === null ? 0 : secondsLeft(code.expires_at, now);

  if (code !== null && left > 0) {
    const href = safeHttpsHref(code.chat_url) ?? safeHttpsHref(link.chat_url);
    return (
      <div className="flex flex-col gap-4">
        <div className="flex flex-col items-start gap-1">
          <span className="text-xs font-semibold text-ink-500">Mã ghép nối của bạn</span>
          <output
            aria-label={`Mã ghép nối: ${code.code.split("").join(" ")}`}
            className="font-mono text-[32px] leading-tight font-bold tracking-[0.2em] break-all text-ink-900 select-all"
          >
            {code.code}
          </output>
          <span className="text-[13px] text-ink-500" aria-live="off">
            Còn hiệu lực <span className="tabular-nums">{formatCountdown(left)}</span>
          </span>
        </div>
        <ol className="m-0 flex flex-col gap-1 pl-5 text-sm text-ink-700">
          {PAIR_STEPS.map((s) => (
            <li key={s}>{s}</li>
          ))}
        </ol>
        <div className="flex flex-wrap gap-2">
          {href !== null && (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="nut-chinh inline-flex h-9 items-center gap-2 rounded-control border border-cta bg-cta px-4 text-[15px] text-white no-underline hover:bg-cta-hover"
            >
              <ExternalLink aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-5" />
              Mở Zalo
            </a>
          )}
          <Button
            type="button"
            variant="secondary"
            onClick={onRequestCode}
            disabled={busy !== null}
            icon={<RotateCw aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          >
            Lấy mã mới
          </Button>
        </div>
        <p role="status" className="an-thi-giac">
          Đang chờ bot nhận mã…
        </p>
      </div>
    );
  }

  return (
    <>
      <p className="m-0 text-sm text-ink-700">{code !== null ? CODE_EXPIRED : NOT_LINKED}</p>
      <div>
        <Button
          type="button"
          variant="primary"
          onClick={onRequestCode}
          disabled={busy !== null}
          aria-busy={busy === "code"}
          icon={<QrCode aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          {busy === "code" ? "Đang lấy mã…" : code !== null ? "Lấy mã mới" : "Lấy mã ghép nối"}
        </Button>
      </div>
    </>
  );
}
