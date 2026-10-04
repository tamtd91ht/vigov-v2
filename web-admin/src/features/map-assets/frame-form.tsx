"use client";

import { Crosshair } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { putMapFrame } from "@/lib/api/map-frame";
import type { comms_mapFrameOut } from "@/lib/api/schema.gen";

import { formatCoordinate, parseCoordinate } from "./asset-form";
import { BAD_COORDINATE, FRAME_FORM_NOTE, FRAME_FORM_TITLE, FRAME_FROM_POINTS_BUTTON } from "./labels";
import { FRAME_RADIUS_MAX_KM, FRAME_RADIUS_MIN_KM, type MapFrame } from "./map-logic";

/** Radius as typed → km, or `null` outside the named bounds (the server re-checks; its sentence wins). */
export function parseRadius(text: string): number | null {
  const t = text.trim().replace(",", ".");
  if (!/^\d{1,2}(\.\d{1,3})?$/.test(t)) return null;
  const r = Number(t);
  return r >= FRAME_RADIUS_MIN_KM && r <= FRAME_RADIUS_MAX_KM ? r : null;
}

export const RADIUS_ERROR = `Bán kính phải từ ${FRAME_RADIUS_MIN_KM} đến ${FRAME_RADIUS_MAX_KM} km.`;

/**
 * Set / change the commune's map frame (ADR 0072 H3) — `admin.lookup` only; the caller does not render
 * it for anyone else, and the server refuses them anyway (rule 5, #1). Audited by the server.
 *
 * "Lấy tâm từ các đối tượng đã có" fills the centre with the MEAN of the assets already filed — a
 * suggestion the officer still saves; nothing is sent until they press Lưu.
 */
export function FrameForm({
  current,
  suggestedCentre,
  onSaved,
  onCancel,
  titleId,
}: {
  current: MapFrame | null;
  /** Mean of the existing points, or `null` when there are none (the button is then not drawn). */
  suggestedCentre: { lat: number; lng: number } | null;
  onSaved: (out: comms_mapFrameOut) => void;
  onCancel?: () => void;
  titleId: string;
}) {
  const [lat, setLat] = useState(current === null ? "" : formatCoordinate(current.centerLat));
  const [lng, setLng] = useState(current === null ? "" : formatCoordinate(current.centerLng));
  const [radius, setRadius] = useState(current === null ? "" : String(current.radiusKm));
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  async function submit() {
    if (sending) return;
    const pLat = parseCoordinate(lat, 90);
    const pLng = parseCoordinate(lng, 180);
    if (pLat === null || pLng === null) return setError(BAD_COORDINATE);
    const r = parseRadius(radius);
    if (r === null) return setError(RADIUS_ERROR);
    setError("");
    setSending(true);
    const res = await putMapFrame({ center_lat: pLat, center_lng: pLng, radius_km: r });
    setSending(false);
    if (!res.ok) return setError(res.thongBao);
    onSaved(res.duLieu);
  }

  return (
    <form
      className="grid min-w-0 gap-4 sm:grid-cols-3 [&>*]:m-0"
      aria-labelledby={titleId}
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      <h3 id={titleId} className="text-[15px] font-semibold text-ink-900 sm:col-span-3">
        {FRAME_FORM_TITLE}
      </h3>
      <p className="text-[13px] text-ink-500 sm:col-span-3">{FRAME_FORM_NOTE}</p>
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
          aria-describedby="frame-radius-hint"
        />
        <p id="frame-radius-hint" className="ghi-chu">{`Từ ${FRAME_RADIUS_MIN_KM} đến ${FRAME_RADIUS_MAX_KM} km.`}</p>
      </div>
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
        <Button type="submit" variant="primary" disabled={sending} aria-busy={sending}>
          <BusyLabel busy={sending} label="Lưu khung bản đồ" busyText={BUSY_SAVING} />
        </Button>
        {onCancel !== undefined && (
          <Button type="button" variant="secondary" onClick={onCancel} disabled={sending}>
            Huỷ
          </Button>
        )}
      </div>
    </form>
  );
}
