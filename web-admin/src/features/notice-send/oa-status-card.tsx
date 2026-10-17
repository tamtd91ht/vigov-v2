"use client";

import { ExternalLink, Loader2, TriangleAlert } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { AUTHORIZE_INSTRUCTION, OA_NOT_CONFIGURED, reauthorizationText } from "@/features/cau-hinh/zalo-oa-form";
import { openZaloOAAuthorization } from "@/features/cau-hinh/zalo-oa-section";
// vi-name-ok: the existing result type of lib/api/goi.ts, imported, not a new name
import type { KetQua } from "@/lib/api/goi";
import { listCommuneZaloOAs, type comms_communeZaloOAListOut, type comms_communeZaloOAOut } from "@/lib/api/zalo-oa";
import { cn } from "@/lib/cn";

import { OA_CARD_TITLE, OA_MANAGE, OA_NOT_LINKED, oaSenderText, OPEN_ZALO_CONFIG, ZALO_CONFIG_HREF } from "./notice-send-form";

/**
 * The commune's OAs as seen from `/gui-thong-bao` — which OAs a ZNS could leave from, or why none can
 * (ADR 0099 #1: several per commune). Only an account with `admin.zalo` reads it: `GET zalo-oas` declares
 * that key, so for anyone else the call is a certain 403. Without the key the card shows the way to
 * Cấu hình and nothing red. `null` = not read yet (or not readable).
 */
export function useCommuneOA(canManage: boolean): KetQua<comms_communeZaloOAListOut> | null {
  const [current, setCurrent] = useState<KetQua<comms_communeZaloOAListOut> | null>(null);
  useEffect(() => {
    if (!canManage) return;
    let gone = false;
    listCommuneZaloOAs().then((r) => {
      if (!gone) setCurrent(r);
    });
    return () => {
      gone = true;
    };
  }, [canManage]);
  return canManage ? current : null;
}

/**
 * The OAs a ZNS can leave from NOW: the platform's app declared, and the OA not waiting for
 * re-authorisation (Zalo refuses its token) — the names this card lists.
 */
export function sendingOAs(list: comms_communeZaloOAListOut): comms_communeZaloOAOut[] {
  return list.configured ? list.items.filter((o) => o.needs_reauthorization === null) : [];
}

const CARD_TOP = "border-line shadow-card rounded-card mb-4 border border-solid bg-white px-5 py-3.5";
const CARD_STEP = "border-line rounded-[10px] border border-solid bg-white px-4 py-3";
const TEXT = "text-ink-muted m-0 text-[12.5px]";
const WARNING = "border-tangerine/30 bg-tangerine/8 text-navy m-0 rounded-[10px] border border-solid px-3 py-2 text-[12.5px]";
const LINK = "text-brand text-[12.5px] font-semibold underline-offset-2 hover:underline";

/**
 * Four states: not configured on the platform · no OA · no OA able to send (every one needs
 * re-authorisation) · at least one OA able to send — the last one still warns, by name, about any OA
 * needing re-authorisation. "Kết nối OA" / "Uỷ quyền lại" call the SAME function as Cấu hình → Kênh Zalo
 * (`openZaloOAAuthorization`); the link never reaches state or the DOM here either. Zalo sends the browser
 * back to Cấu hình when the commune's administrator is done (ADR 0098 #4) — this card does not re-read on
 * its own. The OA a campaign leaves from is the one of the template picked in step 2, not this card.
 */
export function OAStatusCard({
  canManage,
  current,
  where = "top",
}: {
  canManage: boolean;
  current: KetQua<comms_communeZaloOAListOut> | null;
  /** The card is drawn twice when ZNS is picked with no usable OA (top of the page, and in step 2): one id each. */
  where?: "top" | "step";
}) {
  const [linking, setLinking] = useState(false);
  const [refused, setRefused] = useState<string | null>(null);
  const hintId = `notice-oa-authorize-hint-${where}`;
  const CARD = where === "top" ? CARD_TOP : CARD_STEP;

  const configLink = (label: string) => (
    <Link href={ZALO_CONFIG_HREF} className={LINK} data-oa-config-link="">
      {label}
    </Link>
  );

  if (!canManage) {
    return (
      <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="no-permission">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <h2 className="text-navy m-0 text-[13.5px] font-bold">{OA_CARD_TITLE}</h2>
          {configLink(OPEN_ZALO_CONFIG)}
        </div>
      </section>
    );
  }
  if (current === null) {
    return (
      <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="loading">
        <p role="status" className="an-thi-giac">
          Đang tải OA Zalo của xã…
        </p>
        <Skeleton className="h-6 w-full" />
      </section>
    );
  }
  if (!current.ok) {
    return (
      <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="error">
        <p className="text-danger m-0 text-[12.5px] font-medium" role="alert">
          {current.thongBao}
        </p>
      </section>
    );
  }

  const now = current.duLieu;
  const sending = sendingOAs(now);
  const needReauth = now.items.filter((o) => o.needs_reauthorization !== null);

  async function authorize() {
    setLinking(true);
    const r = await openZaloOAAuthorization();
    setLinking(false);
    if (r.refused !== null) setRefused(r.refused);
  }

  const authorizeButton = (label: string) => (
    <Button
      type="button"
      variant="primary"
      size="sm"
      disabled={linking || refused !== null}
      aria-busy={linking}
      aria-describedby={hintId}
      icon={
        linking ? (
          <Loader2 aria-hidden="true" focusable="false" className="animate-spin" />
        ) : (
          <ExternalLink aria-hidden="true" focusable="false" />
        )
      }
      onClick={() => void authorize()}
    >
      {label}
    </Button>
  );
  const hint = (
    <p id={hintId} className={cn(TEXT, "mt-2 text-[12px]")}>
      {refused ?? AUTHORIZE_INSTRUCTION}
    </p>
  );
  /** One line per OA waiting for re-authorisation, each naming its OA. */
  const reauthWarning = (
    <p className={cn(WARNING, "flex min-w-0 flex-1 items-start gap-2")} role="alert" data-oa-reauth="">
      <TriangleAlert aria-hidden="true" focusable="false" className="mt-0.5 size-4 shrink-0" />
      <span className="flex min-w-0 flex-col gap-0.5">
        {needReauth.map((o) => (
          <span key={o.id}>{`${o.oa_name}: ${reauthorizationText(o.needs_reauthorization?.reason ?? "")}`}</span>
        ))}
      </span>
    </p>
  );

  if (!now.configured) {
    return (
      <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="not-configured">
        <p role="status" className={WARNING}>
          {OA_NOT_CONFIGURED}
        </p>
      </section>
    );
  }
  if (now.items.length === 0) {
    return (
      <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="not-linked">
        <div className="flex flex-wrap items-center gap-3">
          <p className={cn(TEXT, "text-navy min-w-0 flex-1 font-semibold")}>{OA_NOT_LINKED}</p>
          {authorizeButton("Kết nối OA")}
        </div>
        {hint}
      </section>
    );
  }
  if (sending.length === 0) {
    return (
      <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="reauthorize">
        <div className="flex flex-wrap items-start gap-3">
          {reauthWarning}
          {authorizeButton("Uỷ quyền lại")}
        </div>
        {hint}
      </section>
    );
  }
  return (
    <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="linked">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <p className={cn(TEXT, "text-navy min-w-0 font-semibold break-words")} data-oa-sender="">
          {oaSenderText(sending.map((o) => o.oa_name))}
        </p>
        {configLink(OA_MANAGE)}
      </div>
      {needReauth.length > 0 && (
        <>
          <div className="mt-2 flex flex-wrap items-start gap-3">
            {reauthWarning}
            {authorizeButton("Uỷ quyền lại")}
          </div>
          {hint}
        </>
      )}
    </section>
  );
}
