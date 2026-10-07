"use client";

import { ImageIcon, Landmark, RotateCcw, Trash2, Upload } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { Button, buttonVariants, LEGACY_BUTTON_CLASS } from "@/components/ui/button";
import { CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ErrorState } from "@/components/ui/error-state";
import { NoAccess } from "@/components/ui/no-access";
import { Skeleton } from "@/components/ui/skeleton";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { cn } from "@/lib/cn";
import { getCommuneBranding, removeBrandingImage, type BrandingImageKind } from "@/lib/api/commune-branding";
import type { platform_brandingSettingsOut } from "@/lib/api/schema.gen";
import type { CallResult } from "@/lib/api/task-attachments";
import { publicImageUrl } from "@/lib/public-image-url";

import {
  BRANDING_ACCEPT,
  BRANDING_CANCEL_BUTTON,
  BRANDING_PICK_BUTTON,
  BRANDING_REPLACE_BUTTON,
  BRANDING_RETRY_BUTTON,
  BRANDING_TEXT,
  brandingInFlight,
  brandingStateText,
  brandingUpdatedText,
  retryBrandingCompletion,
  runBrandingUpload,
  type BrandingUploadState,
} from "./commune-branding";
import { brandingTabDecision } from "./quyen-tab";

/**
 * "Cấu hình → Nhận diện xã" (ADR 0069): the commune uploads its own logo and web-admin banner. One key,
 * `admin.org`, on all seven routes, read included — the tab hides as a whole without it (convenience;
 * the server refuses on every call, rule 5 #1). No approval step: a completed upload is current at once
 * (ADR 0069 #3), so after every change the page is refreshed (`router.refresh()`) and the server
 * re-reads `GET /api/v1/communes/current` — the sidebar tile and the banner strip update without a reload.
 */
export function CommuneBrandingTab() {
  const phien = usePhien();
  const router = useRouter();
  const decision = phien === null ? null : brandingTabDecision(phien);
  const allowed = decision !== null && decision.hien;

  const [loaded, setLoaded] = useState<CallResult<platform_brandingSettingsOut> | null>(null);
  const [cards, setCards] = useState<Record<BrandingImageKind, CardState>>({ logo: IDLE_CARD, banner: IDLE_CARD });

  useEffect(() => {
    if (!allowed) return;
    let gone = false;
    getCommuneBranding().then((r) => {
      if (!gone) setLoaded(r);
    });
    return () => {
      gone = true;
    };
  }, [allowed]);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p className="thong-bao-loi" role="alert">
        {decision.thongBao}
      </p>
    ) : (
      <div className="khung-thieu-quyen flex min-w-0 flex-col items-center pb-10 [&>.trang-thai-rong]:m-0 [&>.trang-thai-rong]:max-w-md [&>.trang-thai-rong]:border-0 [&>.trang-thai-rong]:bg-transparent [&>.trang-thai-rong]:px-4 [&>.trang-thai-rong]:py-0 [&>.trang-thai-rong]:text-center [&>.trang-thai-rong]:text-[13px] [&>.trang-thai-rong]:text-ink-500">
        <NoAccess className="pb-4" />
        <p className="trang-thai-rong">{BRANDING_DENIED}</p>
      </div>
    );
  }
  if (loaded === null)
    return (
      <div className="page--form flex flex-col gap-3 rounded-card border border-line bg-surface p-4">
        <p role="status" className="an-thi-giac">
          Đang tải nhận diện xã…
        </p>
        <Skeleton className="h-5 w-48" />
        <Skeleton className="h-24 w-24" />
        <Skeleton className="h-10 w-2/3" />
      </div>
    );
  if (!loaded.ok) return <ErrorState role="alert" title="Chưa tải được nhận diện xã" message={loaded.message} />;

  const patch = (kind: BrandingImageKind, p: Partial<CardState>) =>
    setCards((c) => ({ ...c, [kind]: { ...c[kind], ...p } }));

  /** After a change took effect: re-read the tab's own settings and the server-rendered shell. */
  async function afterChange(kind: BrandingImageKind, sentence: string) {
    patch(kind, { upload: { kind: "idle" }, notice: sentence });
    setLoaded(await getCommuneBranding());
    router.refresh();
  }

  async function pick(kind: BrandingImageKind, file: File) {
    patch(kind, { notice: "", removeError: "", confirming: false });
    const last = await runBrandingUpload(kind, file, () => crypto.randomUUID(), (s) => patch(kind, { upload: s }));
    if (last.kind === "done") await afterChange(kind, BRANDING_TEXT[kind].saved);
  }

  async function retry(kind: BrandingImageKind, id: string) {
    const last = await retryBrandingCompletion(kind, id, (s) => patch(kind, { upload: s }));
    if (last.kind === "done") await afterChange(kind, BRANDING_TEXT[kind].saved);
  }

  async function remove(kind: BrandingImageKind) {
    patch(kind, { removing: true, removeError: "", notice: "" });
    const r = await removeBrandingImage(kind);
    patch(kind, { removing: false, confirming: false });
    if (!r.ok) {
      patch(kind, { removeError: r.message });
      return;
    }
    await afterChange(kind, BRANDING_TEXT[kind].removed);
  }

  const updated = brandingUpdatedText(loaded.data.updated_at, loaded.data.updated_by);
  return (
    <div className="flex min-w-0 flex-col gap-4">
      {(["logo", "banner"] as const).map((kind) => (
        <BrandingCardView
          key={kind}
          kind={kind}
          currentUrl={publicImageUrl(kind === "logo" ? loaded.data.logo_public_url : loaded.data.web_admin_banner_public_url)}
          updatedText={updated}
          card={cards[kind]}
          onPick={(f) => void pick(kind, f)}
          onRetry={(id) => void retry(kind, id)}
          onAskRemove={() => patch(kind, { confirming: true, removeError: "", notice: "" })}
          onCancelRemove={() => patch(kind, { confirming: false })}
          onConfirmRemove={() => void remove(kind)}
        />
      ))}
    </div>
  );
}

export const BRANDING_DENIED =
  "Tài khoản của bạn không có quyền quản lý nhận diện xã (admin.org), nên tab này không hiển thị.";

/** One card's UI state. `notice` = the last success sentence; `removeError` = the server's refusal. */
export type CardState = {
  readonly upload: BrandingUploadState;
  readonly confirming: boolean;
  readonly removing: boolean;
  readonly notice: string;
  readonly removeError: string;
};

export const IDLE_CARD: CardState = { upload: { kind: "idle" }, confirming: false, removing: false, notice: "", removeError: "" };

/**
 * One card, pure — exported so the tests read the markup of every state. No hooks, no context: the
 * tests render it with `renderToStaticMarkup`.
 */
export function BrandingCardView({
  kind,
  currentUrl,
  updatedText,
  card,
  onPick,
  onRetry,
  onAskRemove,
  onCancelRemove,
  onConfirmRemove,
}: {
  kind: BrandingImageKind;
  /** Already through `publicImageUrl`: "" = none. */
  currentUrl: string;
  updatedText: string;
  card: CardState;
  onPick: (f: File) => void;
  onRetry: (id: string) => void;
  onAskRemove: () => void;
  onCancelRemove: () => void;
  onConfirmRemove: () => void;
}) {
  const t = BRANDING_TEXT[kind];
  const busy = brandingInFlight(card.upload) || card.removing;
  const stateText = brandingStateText(card.upload);
  const failed = card.upload.kind === "refused" || card.upload.kind === "retry";
  const inputId = `nhan-dien-${kind}`;
  const titleId = `tieu-de-nhan-dien-${kind}`;

  return (
    <section
      className="page--form m-0 min-w-0 overflow-hidden rounded-card border border-line bg-surface shadow-sm"
      aria-labelledby={titleId}
    >
      <CardHeader className="m-0">
        <div className="min-w-0 flex-1 basis-64">
          <CardTitle as="h2" id={titleId} className="flex items-center gap-2">
            <ImageIcon aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
            {t.title}
          </CardTitle>
          <p className="ghi-chu m-0 mt-1 text-[13px] text-ink-500">{t.description}</p>
        </div>
      </CardHeader>

      <div className="flex min-w-0 flex-col gap-4 p-4 [&>*]:my-0">
        {currentUrl !== "" ? (
          kind === "logo" ? (
            // The tile shape of the sidebar, larger: contain, transparency kept on a neutral surface.
            <div className="grid size-28 place-items-center overflow-hidden rounded-xl border border-line bg-surface p-1">
              {/* eslint-disable-next-line @next/next/no-img-element -- public URL, see commune-identity.tsx */}
              <img
                src={currentUrl}
                alt={t.previewLabel}
                width={512}
                height={512}
                className="size-full object-contain"
                referrerPolicy="no-referrer"
              />
            </div>
          ) : (
            // eslint-disable-next-line @next/next/no-img-element -- public URL, see CommuneLogoImage in commune-identity.tsx
            <img
              src={currentUrl}
              alt={t.previewLabel}
              width={1600}
              height={112}
              className="block h-[72px] w-full rounded-card bg-surface-muted object-cover md:h-[112px]"
              referrerPolicy="no-referrer"
            />
          )
        ) : (
          <div className="flex items-center gap-3 rounded-xl border border-dashed border-line-strong p-3 text-sm text-ink-500">
            <span className="grid size-11 shrink-0 place-items-center rounded-xl bg-brand-50 text-brand-600" aria-hidden="true">
              {kind === "logo" ? (
                <Landmark focusable="false" strokeWidth={1.8} className="size-6" />
              ) : (
                <ImageIcon focusable="false" strokeWidth={1.8} className="size-6" />
              )}
            </span>
            <p className="m-0">{t.empty}</p>
          </div>
        )}

        {updatedText !== "" && <p className="ghi-chu m-0 text-xs text-ink-500">{updatedText}</p>}

        <div className="flex flex-wrap items-center gap-2">
          {/* The input FIRST, visually hidden but focusable; the label right after it is the visible
              button (`input:focus-visible + label`, as the content cover picker). */}
          <input
            id={inputId}
            name={inputId}
            type="file"
            accept={BRANDING_ACCEPT}
            className="an-thi-giac"
            aria-describedby={`${inputId}-goi-y`}
            disabled={busy}
            onChange={(e) => {
              const f = e.target.files?.[0];
              e.target.value = ""; // the same file can be chosen again after a refusal
              if (f !== undefined) onPick(f);
            }}
          />
          <label
            htmlFor={inputId}
            aria-disabled={busy || undefined}
            className={cn(
              LEGACY_BUTTON_CLASS.primary,
              buttonVariants({ variant: "primary", size: "md" }),
              busy && "pointer-events-none opacity-60",
            )}
          >
            <Upload aria-hidden="true" focusable="false" strokeWidth={1.8} />
            {currentUrl === "" ? BRANDING_PICK_BUTTON : BRANDING_REPLACE_BUTTON}
            <span className="an-thi-giac"> — {t.title}</span>
          </label>
          {card.upload.kind === "retry" && (
            <Button
              type="button"
              variant="secondary"
              icon={<RotateCcw aria-hidden="true" focusable="false" strokeWidth={1.8} />}
              onClick={() => card.upload.kind === "retry" && onRetry(card.upload.id)}
            >
              {BRANDING_RETRY_BUTTON}
            </Button>
          )}
          {currentUrl !== "" && !card.confirming && (
            <Button
              type="button"
              variant="danger"
              disabled={busy}
              icon={<Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} />}
              onClick={onAskRemove}
            >
              {t.removeButton}
            </Button>
          )}
        </div>
        <p className="ghi-chu m-0 text-xs text-ink-500" id={`${inputId}-goi-y`}>
          {t.hint}
        </p>

        {stateText !== "" && (
          <p role={failed ? "alert" : "status"} className={failed ? "thong-bao-loi" : "m-0 text-sm text-ink-700"}>
            {stateText}
          </p>
        )}
        {card.notice !== "" && (
          <p role="status" className="m-0 text-sm font-medium text-success-600">
            {card.notice}
          </p>
        )}

        {card.confirming && currentUrl !== "" && (
          <ConfirmDialog
            as="div"
            role="group"
            aria-label={t.confirmTitle}
            tone="danger"
            icon={Trash2}
            title={t.confirmTitle}
            actions={
              <>
                <Button
                  type="button"
                  variant="danger"
                  disabled={card.removing}
                  aria-busy={card.removing}
                  icon={<Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                  onClick={onConfirmRemove}
                >
                  {card.removing ? "Đang gỡ…" : t.removeButton}
                </Button>
                <Button type="button" variant="secondary" disabled={card.removing} onClick={onCancelRemove}>
                  {BRANDING_CANCEL_BUTTON}
                </Button>
              </>
            }
          >
            <p className="m-0">{t.confirmConsequence}</p>
          </ConfirmDialog>
        )}
        {card.removeError !== "" && (
          <p className="thong-bao-loi" role="alert">
            {card.removeError}
          </p>
        )}
      </div>
    </section>
  );
}
