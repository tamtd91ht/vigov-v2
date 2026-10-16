"use client";

import { ExternalLink, Loader2, TriangleAlert } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { AUTHORIZE_INSTRUCTION, OA_NOT_CONFIGURED, reauthorizationText } from "@/features/cau-hinh/zalo-oa-form";
import { openZaloOAAuthorization } from "@/features/cau-hinh/zalo-oa-section";
import { formatVietnamDateTime } from "@/features/noi-dung/nhan-noi-dung";
// vi-name-ok: the existing result type of lib/api/goi.ts, imported, not a new name
import type { KetQua } from "@/lib/api/goi";
import { getCommuneZaloOA, type comms_communeZaloOACurrentOut } from "@/lib/api/zalo-oa";
import { cn } from "@/lib/cn";

import { OA_CARD_TITLE, OA_MANAGE, OA_NOT_LINKED, oaSenderText, OPEN_ZALO_CONFIG, ZALO_CONFIG_HREF } from "./notice-send-form";

/**
 * The commune's OA as seen from `/gui-thong-bao` — which OA a ZNS would leave from, or why none can.
 * Only an account with `admin.zalo` reads it: `GET zalo-oas/current` declares that key, so for anyone
 * else the call is a certain 403. Without the key the card shows the way to Cấu hình and nothing red.
 * `null` = not read yet (or not readable).
 */
export function useCommuneOA(canManage: boolean): KetQua<comms_communeZaloOACurrentOut> | null {
  const [current, setCurrent] = useState<KetQua<comms_communeZaloOACurrentOut> | null>(null);
  useEffect(() => {
    if (!canManage) return;
    let gone = false;
    getCommuneZaloOA().then((r) => {
      if (!gone) setCurrent(r);
    });
    return () => {
      gone = true;
    };
  }, [canManage]);
  return canManage ? current : null;
}

const CARD_TOP = "border-line shadow-card rounded-card mb-4 border border-solid bg-white px-5 py-3.5";
const CARD_STEP = "border-line rounded-[10px] border border-solid bg-white px-4 py-3";
const TEXT = "text-ink-muted m-0 text-[12.5px]";
const WARNING = "border-tangerine/30 bg-tangerine/8 text-navy m-0 rounded-[10px] border border-solid px-3 py-2 text-[12.5px]";
const LINK = "text-brand text-[12.5px] font-semibold underline-offset-2 hover:underline";

/**
 * Four states: not configured on the platform · no OA · OA needing re-authorisation · OA connected.
 * "Kết nối OA" / "Uỷ quyền lại" call the SAME function as Cấu hình → Kênh Zalo (`openZaloOAAuthorization`);
 * the link never reaches state or the DOM here either. Zalo sends the browser back to Cấu hình when
 * the commune's administrator is done (ADR 0098 #4) — this card does not re-read on its own.
 */
export function OAStatusCard({
  canManage,
  current,
  where = "top",
}: {
  canManage: boolean;
  current: KetQua<comms_communeZaloOACurrentOut> | null;
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
  const oa = now.has_oa ? now.oa : null;
  const reauth = oa?.needs_reauthorization ?? null;

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

  if (!now.configured) {
    return (
      <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="not-configured">
        <p role="status" className={WARNING}>
          {OA_NOT_CONFIGURED}
        </p>
      </section>
    );
  }
  if (oa === null) {
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
  if (reauth !== null) {
    return (
      <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="reauthorize">
        <div className="flex flex-wrap items-start gap-3">
          <p className={cn(WARNING, "flex min-w-0 flex-1 items-start gap-2")} role="alert">
            <TriangleAlert aria-hidden="true" focusable="false" className="mt-0.5 size-4 shrink-0" />
            <span>{`${oa.oa_name}: ${reauthorizationText(reauth.reason)}`}</span>
          </p>
          {authorizeButton("Uỷ quyền lại")}
        </div>
        {hint}
      </section>
    );
  }
  return (
    <section className={CARD} aria-label={OA_CARD_TITLE} data-oa-card="linked">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <p className={cn(TEXT, "text-navy font-semibold")} data-oa-sender="">
          {oaSenderText(oa.oa_name, formatVietnamDateTime(oa.authorized_at))}
        </p>
        {configLink(OA_MANAGE)}
      </div>
    </section>
  );
}
