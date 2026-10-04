"use client";

import { Crosshair, TriangleAlert } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Notice } from "@/components/ui/notice";
import { putMapFrame, resetMapFrame } from "@/lib/api/map-frame";
import type { comms_mapFrameOut } from "@/lib/api/schema.gen";

import { formatCoordinate, parseCoordinate } from "./asset-form";
import {
  BAD_COORDINATE,
  FRAME_FORM_NOTE,
  FRAME_FORM_TITLE,
  FRAME_FROM_POINTS_BUTTON,
  FRAME_RADIUS_UNUSUAL,
  FRAME_RESET_LEAD,
  FRAME_RESET_NO_DEFAULT_KNOWN,
  FRAME_SOURCE_COMMUNE,
  FRAME_SOURCE_DEFAULT,
} from "./labels";
import {
  FRAME_RADIUS_MAX_KM,
  FRAME_RADIUS_MIN_KM,
  radiusUnusual,
  type FrameHints,
  type FrameValues,
  type MapFrame,
} from "./map-logic";
import { NoticeConfirm } from "./notice-confirm";

const KM = new Intl.NumberFormat("vi-VN", { maximumFractionDigits: 1 });

/**
 * Radius as typed → km, or `null` outside 0.1–50 km or with more than one decimal (the column is
 * `numeric(4,1)`, ADR 0072 K2). Accepts a decimal comma. The server re-checks; its sentence wins.
 */
export function parseRadius(text: string): number | null {
  const t = text.trim().replace(",", ".");
  if (!/^\d{1,2}(\.\d)?$/.test(t)) return null;
  const r = Number(t);
  return r >= FRAME_RADIUS_MIN_KM && r <= FRAME_RADIUS_MAX_KM ? r : null;
}

export const RADIUS_ERROR = `Bán kính phải lớn hơn 0 và không quá ${FRAME_RADIUS_MAX_KM} km, tối đa một chữ số thập phân (ví dụ ${KM.format(FRAME_RADIUS_MIN_KM)}).`;

/** "Đề xuất: 10 km (thường 3–20 km)" — the server's figures; nothing when it sent none. */
export function radiusHint(hints: FrameHints): string {
  const range = hints.usualKm === null ? "" : `${KM.format(hints.usualKm[0])}–${KM.format(hints.usualKm[1])} km`;
  if (hints.recommendedKm === null) return range === "" ? "" : `Thường ${range}.`;
  const rec = `Đề xuất: ${KM.format(hints.recommendedKm)} km`;
  return range === "" ? `${rec}.` : `${rec} (thường ${range}).`;
}

function describeFrame(f: FrameValues): string {
  return `tâm ${formatCoordinate(f.centerLat)}, ${formatCoordinate(f.centerLng)} · bán kính ${KM.format(f.radiusKm)} km`;
}

/** "Đang dùng khung của xã" / "… mặc định do nhà cung cấp đặt", and the default's values when known. */
function SourceLines({ current, defaultFrame }: { current: MapFrame | null; defaultFrame: FrameValues | null }) {
  return (
    <>
      {current !== null && (
        <p className="m-0 text-[13px] font-medium text-ink-900 sm:col-span-3" data-frame-source={current.source}>
          {current.source === "commune" ? FRAME_SOURCE_COMMUNE : FRAME_SOURCE_DEFAULT}
        </p>
      )}
      {defaultFrame !== null && (
        <p className="m-0 text-[13px] text-ink-500 sm:col-span-3" data-frame-default="">
          {`Khung mặc định của nhà cung cấp: ${describeFrame(defaultFrame)}.`}
        </p>
      )}
    </>
  );
}

type Draft = { readonly lat: number; readonly lng: number; readonly radiusKm: number };

/**
 * Set / change the commune's own map frame (ADR 0072 H3, K4) — `admin.lookup` only; the caller does not
 * render it for anyone else, and the server refuses them anyway (rule 5, #1). Audited by the server.
 *
 * "Lưu" checks the fields, then opens the LEGAL NOTICE step (`NoticeConfirm`); nothing is sent before
 * the officer ticks it and presses "Xác nhận thay đổi". A radius outside the usual range shows a warning
 * and still saves (K2: the officer decides and answers for it).
 *
 * Prefill: the frame in effect (the commune's or the default), else the default carried by the reply,
 * else the recommended radius — every figure from the server, none from this bundle.
 *
 * "Lấy tâm từ các đối tượng đã có" fills the centre with the MEAN of the assets already filed — a
 * suggestion the officer still saves.
 */
export function FrameForm({
  current,
  defaultFrame,
  hints,
  suggestedCentre,
  onSaved,
  onReloaded,
  onCancel,
  titleId,
}: {
  current: MapFrame | null;
  defaultFrame: FrameValues | null;
  hints: FrameHints;
  /** Mean of the existing points, or `null` when there are none (the button is then not drawn). */
  suggestedCentre: { lat: number; lng: number } | null;
  onSaved: (out: comms_mapFrameOut) => void;
  onReloaded: (out: comms_mapFrameOut) => void;
  onCancel?: () => void;
  titleId: string;
}) {
  const start: FrameValues | null = current ?? defaultFrame;
  const [lat, setLat] = useState(start === null ? "" : formatCoordinate(start.centerLat));
  const [lng, setLng] = useState(start === null ? "" : formatCoordinate(start.centerLng));
  const [radius, setRadius] = useState(
    start !== null ? String(start.radiusKm) : hints.recommendedKm === null ? "" : String(hints.recommendedKm),
  );
  const [error, setError] = useState("");
  const [draft, setDraft] = useState<Draft | null>(null);

  const typed = parseRadius(radius);
  const unusual = typed !== null && radiusUnusual(typed, hints.usualKm);
  const hint = radiusHint(hints);

  function submit() {
    const pLat = parseCoordinate(lat, 90);
    const pLng = parseCoordinate(lng, 180);
    if (pLat === null || pLng === null) return setError(BAD_COORDINATE);
    const r = parseRadius(radius);
    if (r === null) return setError(RADIUS_ERROR);
    setError("");
    setDraft({ lat: pLat, lng: pLng, radiusKm: r });
  }

  return (
    <>
      <form
        className="grid min-w-0 gap-4 sm:grid-cols-3 [&>*]:m-0"
        aria-labelledby={titleId}
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        <h3 id={titleId} className="text-[15px] font-semibold text-ink-900 sm:col-span-3">
          {FRAME_FORM_TITLE}
        </h3>
        <p className="text-[13px] text-ink-500 sm:col-span-3">{FRAME_FORM_NOTE}</p>
        <SourceLines current={current} defaultFrame={defaultFrame} />
        <div className="o-nhap">
          <label htmlFor="frame-center-lat">Vĩ độ tâm</label>
          <input id="frame-center-lat" name="center_lat" inputMode="decimal" value={lat} onChange={(e) => setLat(e.target.value)} />
        </div>
        <div className="o-nhap">
          <label htmlFor="frame-center-lng">Kinh độ tâm</label>
          <input id="frame-center-lng" name="center_lng" inputMode="decimal" value={lng} onChange={(e) => setLng(e.target.value)} />
        </div>
        <div className="o-nhap">
          <label htmlFor="frame-radius">Bán kính (km)</label>
          <input
            id="frame-radius"
            name="radius_km"
            inputMode="decimal"
            value={radius}
            onChange={(e) => setRadius(e.target.value)}
            aria-describedby={unusual ? "frame-radius-hint frame-radius-warning" : "frame-radius-hint"}
          />
          <p id="frame-radius-hint" className="ghi-chu">
            {`Từ ${KM.format(FRAME_RADIUS_MIN_KM)} đến ${KM.format(FRAME_RADIUS_MAX_KM)} km, bước ${KM.format(0.1)} km.`}
            {hint !== "" && ` ${hint}`}
          </p>
        </div>
        {unusual && (
          <Notice tone="legal" icon={TriangleAlert} id="frame-radius-warning" className="sm:col-span-3" data-radius-warning="">
            {FRAME_RADIUS_UNUSUAL}
          </Notice>
        )}
        {suggestedCentre !== null && (
          <p className="sm:col-span-3">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              icon={<Crosshair aria-hidden="true" focusable="false" strokeWidth={1.8} />}
              onClick={() => {
                setLat(formatCoordinate(suggestedCentre.lat));
                setLng(formatCoordinate(suggestedCentre.lng));
              }}
            >
              {FRAME_FROM_POINTS_BUTTON}
            </Button>
          </p>
        )}
        {error !== "" && (
          <p className="thong-bao-loi sm:col-span-3" role="alert">
            {error}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2 sm:col-span-3">
          <Button type="submit" variant="primary" disabled={draft !== null}>
            Lưu khung bản đồ
          </Button>
          {onCancel !== undefined && (
            <Button type="button" variant="secondary" onClick={onCancel} disabled={draft !== null}>
              Huỷ
            </Button>
          )}
        </div>
      </form>
      {draft !== null && (
        <NoticeConfirm
          titleId={`${titleId}-notice`}
          serverNoticeVersion={hints.noticeVersion}
          send={(v) => putMapFrame({ center_lat: draft.lat, center_lng: draft.lng, radius_km: draft.radiusKm, notice_version: v })}
          onDone={(out) => {
            setDraft(null);
            onSaved(out);
          }}
          onReloaded={onReloaded}
          onCancel={() => setDraft(null)}
        />
      )}
    </>
  );
}

/**
 * "Về mặc định" (ADR 0072 K4): the commune stops using its own frame — kept, never deleted — and the
 * platform default applies. Through the same legal notice step; the answer is the frame now in effect,
 * which may be `configured:false` when the platform has no default (the screen then shows "no frame").
 */
export function ResetFrameDialog({
  defaultFrame,
  hints,
  onDone,
  onReloaded,
  onCancel,
}: {
  defaultFrame: FrameValues | null;
  hints: FrameHints;
  onDone: (out: comms_mapFrameOut) => void;
  onReloaded: (out: comms_mapFrameOut) => void;
  onCancel: () => void;
}) {
  return (
    <NoticeConfirm
      titleId="frame-reset-notice"
      serverNoticeVersion={hints.noticeVersion}
      lead={
        <div className="flex flex-col gap-1 text-[13px] text-ink-700" data-frame-reset-lead="">
          <p className="m-0">{FRAME_RESET_LEAD}</p>
          <p className="m-0">
            {defaultFrame === null ? FRAME_RESET_NO_DEFAULT_KNOWN : `Khung mặc định: ${describeFrame(defaultFrame)}.`}
          </p>
        </div>
      }
      send={(v) => resetMapFrame(v)}
      onDone={onDone}
      onReloaded={onReloaded}
      onCancel={onCancel}
    />
  );
}
