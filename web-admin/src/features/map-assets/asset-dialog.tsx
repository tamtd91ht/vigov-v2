"use client";

import { LocateFixed } from "lucide-react";
import { useEffect, useId, useState } from "react";

import { Button } from "@/components/ui/button";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { OverlayDialog } from "@/features/noi-dung/overlay-dialog";
import type { KetQua } from "@/lib/api/goi";
import { createMapAsset, updateMapAsset } from "@/lib/api/map-assets";
import { listMapFields } from "@/lib/api/map-field-schemas";
import type {
  comms_loaiTaiNguyenRa,
  comms_mapAssetOut,
  comms_mapFieldSchemaOut,
  identity_thonToDanPhoRa,
} from "@/lib/api/schema.gen";

import {
  createBody,
  draftFromAsset,
  formFields,
  formatCoordinate,
  newAssetDraft,
  parseCoordinate,
  updateBody,
  type AssetDraft,
} from "./asset-form";
import {
  ADD_DESCRIPTION,
  ADD_SUBMIT,
  ADD_TITLE,
  EDIT_SUBMIT,
  INDUSTRIES,
  MASKED_NOTE,
  MY_LOCATION_BUTTON,
  MY_LOCATION_FAILED,
  MY_LOCATION_OUTSIDE,
  NO_CHANGE,
  PIN_HINT,
  PIN_OUTSIDE_FRAME,
  STATUS_OPTIONS,
} from "./labels";
import { insideFrame, round6, type MapFrame } from "./map-logic";
import { PositionPicker } from "./position-picker";

export type AssetDialogMode =
  | { readonly kind: "create"; readonly assetTypeCode: string; readonly idempotencyKey: string }
  | { readonly kind: "edit"; readonly asset: comms_mapAssetOut };

/**
 * "Thêm đối tượng lên bản đồ" / edit (spec §8), in the shared native `<dialog>` (`OverlayDialog`).
 * The rules live in `asset-form.ts`; this file draws them. Server refusals (409 tax code taken, 422
 * group unavailable / custom values invalid) are shown as the server's own sentence (`goi.ts`).
 */
export function AssetDialog({
  mode,
  types,
  units,
  frame,
  styleUrl,
  onSaved,
  onCancel,
}: {
  mode: AssetDialogMode;
  types: readonly comms_loaiTaiNguyenRa[];
  units: readonly identity_thonToDanPhoRa[];
  frame: MapFrame | null;
  styleUrl: string | null;
  onSaved: (saved: comms_mapAssetOut, created: boolean) => void;
  onCancel: () => void;
}) {
  const titleId = useId();
  const [draft, setDraft] = useState<AssetDraft>(() =>
    mode.kind === "create"
      ? newAssetDraft(mode.assetTypeCode, frame === null ? null : { lat: frame.centerLat, lng: frame.centerLng })
      : draftFromAsset(mode.asset),
  );
  const [fields, setFields] = useState<KetQua<{ items: readonly comms_mapFieldSchemaOut[] }> | null>(null);
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  const masked = mode.kind === "edit" && mode.asset.masked;
  const bounds = frame === null ? null : frame.bounds;

  useEffect(() => {
    if (draft.assetTypeCode === "") return;
    let gone = false;
    listMapFields(draft.assetTypeCode).then((r) => {
      if (!gone) setFields(r);
    });
    return () => {
      gone = true;
    };
  }, [draft.assetTypeCode]);

  const fieldItems = fields !== null && fields.ok ? formFields(fields.duLieu.items) : [];
  const set = (patch: Partial<AssetDraft>) => setDraft((d) => ({ ...d, ...patch }));

  async function submit() {
    if (sending) return;
    if (fields !== null && !fields.ok) return setError(fields.thongBao);
    const all = fields !== null && fields.ok ? fields.duLieu.items : [];
    setError("");
    if (mode.kind === "create") {
      const b = createBody(draft, all, bounds);
      if (b.kind !== "send") return setError(b.kind === "error" ? b.message : NO_CHANGE);
      setSending(true);
      const r = await createMapAsset(b.body, mode.idempotencyKey);
      setSending(false);
      if (!r.ok) return setError(r.thongBao);
      return onSaved(r.duLieu, true);
    }
    const b = updateBody(mode.asset, draft, all, bounds);
    if (b.kind !== "send") return setError(b.kind === "error" ? b.message : NO_CHANGE);
    setSending(true);
    const r = await updateMapAsset(mode.asset.id, b.body);
    setSending(false);
    if (!r.ok) return setError(r.thongBao);
    onSaved(r.duLieu, false);
  }

  function myLocation() {
    if (frame === null || typeof navigator === "undefined" || navigator.geolocation === undefined) {
      return setError(MY_LOCATION_FAILED);
    }
    navigator.geolocation.getCurrentPosition(
      (p) => {
        const { latitude, longitude } = p.coords;
        if (!insideFrame(frame.bounds, longitude, latitude)) return setError(MY_LOCATION_OUTSIDE);
        setError("");
        set({ lat: formatCoordinate(latitude), lng: formatCoordinate(longitude) });
      },
      () => setError(MY_LOCATION_FAILED),
      { enableHighAccuracy: true, timeout: 10000 },
    );
  }

  const pLat = parseCoordinate(draft.lat, 90);
  const pLng = parseCoordinate(draft.lng, 180);
  const typedOutside = bounds !== null && pLat !== null && pLng !== null && !insideFrame(bounds, pLng, pLat);
  const activeTypes = types.filter((t) => t.active || t.code === draft.assetTypeCode);
  const pickableUnits = units.filter((u) => u.active || u.id === draft.residentialUnitId);

  return (
    <OverlayDialog titleId={titleId} onDismiss={() => !sending && onCancel()}>
      <form
        className="asset-form grid min-w-0 gap-4 sm:grid-cols-2 [&>*]:m-0"
        aria-labelledby={titleId}
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <div className="sm:col-span-2">
          <h2 id={titleId} className="m-0 text-lg font-semibold text-ink-900">
            {mode.kind === "create" ? ADD_TITLE : `Sửa ${mode.asset.name}`}
          </h2>
          <p className="m-0 mt-1 text-[13px] text-ink-500">{ADD_DESCRIPTION}</p>
        </div>

        <div className="o-nhap">
          <label htmlFor="asset-group">Nhóm *</label>
          <select id="asset-group" name="asset_type_code" value={draft.assetTypeCode} onChange={(e) => set({ assetTypeCode: e.target.value })}>
            <option value="">— Chọn nhóm —</option>
            {activeTypes.map((t) => (
              <option key={t.id} value={t.code}>
                {t.label}
              </option>
            ))}
          </select>
        </div>
        <div className="o-nhap">
          <label htmlFor="asset-status">Trạng thái</label>
          <select id="asset-status" name="status" value={draft.status} onChange={(e) => set({ status: e.target.value })}>
            {STATUS_OPTIONS.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </div>
        <div className="o-nhap sm:col-span-2">
          <label htmlFor="asset-name">Tên *</label>
          <input id="asset-name" name="name" value={draft.name} onChange={(e) => set({ name: e.target.value })} />
        </div>
        <div className="o-nhap">
          <label htmlFor="asset-address">Địa chỉ</label>
          <input id="asset-address" name="address" value={draft.address} onChange={(e) => set({ address: e.target.value })} />
        </div>
        <div className="o-nhap">
          <label htmlFor="asset-unit">Thôn / Tổ dân phố</label>
          <select
            id="asset-unit"
            name="residential_unit_id"
            value={draft.residentialUnitId}
            onChange={(e) => set({ residentialUnitId: e.target.value })}
          >
            <option value="">— Chưa xác định —</option>
            {pickableUnits.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </select>
        </div>

        <fieldset className="m-0 flex min-w-0 flex-col gap-2 rounded-xl border border-line p-3 sm:col-span-2 [&>*]:my-0">
          <legend className="px-1 text-xs font-semibold text-ink-700">Vị trí trên bản đồ *</legend>
          {styleUrl !== null && frame !== null && (
            <>
              <p className="ghi-chu text-[13px] text-ink-500">{PIN_HINT}</p>
              <PositionPicker
                styleUrl={styleUrl}
                frame={frame}
                lat={pLat}
                lng={pLng}
                onPick={(lat, lng) => {
                  setError("");
                  set({ lat: formatCoordinate(lat), lng: formatCoordinate(lng) });
                }}
                onOutside={() => setError(PIN_OUTSIDE_FRAME)}
              />
            </>
          )}
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
            <div className="o-nhap m-0">
              <label htmlFor="asset-lat">Vĩ độ</label>
              <input
                id="asset-lat"
                name="lat"
                inputMode="decimal"
                value={draft.lat}
                onChange={(e) => set({ lat: e.target.value })}
                onBlur={() => pLat !== null && set({ lat: round6(pLat).toFixed(6) })}
              />
            </div>
            <div className="o-nhap m-0">
              <label htmlFor="asset-lng">Kinh độ</label>
              <input
                id="asset-lng"
                name="lng"
                inputMode="decimal"
                value={draft.lng}
                onChange={(e) => set({ lng: e.target.value })}
                onBlur={() => pLng !== null && set({ lng: round6(pLng).toFixed(6) })}
              />
            </div>
            {frame !== null && (
              <Button
                type="button"
                variant="secondary"
                icon={<LocateFixed aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                onClick={myLocation}
              >
                {MY_LOCATION_BUTTON}
              </Button>
            )}
          </div>
          {typedOutside && (
            <p className="thong-bao-loi" role="alert">
              {PIN_OUTSIDE_FRAME}
            </p>
          )}
        </fieldset>

        {masked && <p className="ghi-chu text-[13px] text-ink-500 sm:col-span-2">{MASKED_NOTE}</p>}
        <div className="o-nhap">
          <label htmlFor="asset-representative">Người đại diện</label>
          <input
            id="asset-representative"
            name="representative"
            value={draft.representative}
            disabled={masked}
            onChange={(e) => set({ representative: e.target.value })}
          />
        </div>
        <div className="o-nhap">
          <label htmlFor="asset-phone">Điện thoại</label>
          <input
            id="asset-phone"
            name="phone"
            type="tel"
            inputMode="tel"
            value={draft.phone}
            disabled={masked}
            onChange={(e) => set({ phone: e.target.value })}
          />
        </div>
        <div className="o-nhap">
          <label htmlFor="asset-tax-code">Mã số thuế</label>
          <input
            id="asset-tax-code"
            name="tax_code"
            value={draft.taxCode}
            disabled={masked}
            onChange={(e) => set({ taxCode: e.target.value })}
          />
        </div>
        <div className="o-nhap">
          <label htmlFor="asset-industry">Ngành nghề</label>
          <select id="asset-industry" name="industry_code" value={draft.industryCode} onChange={(e) => set({ industryCode: e.target.value })}>
            <option value="">— Chưa phân ngành —</option>
            {INDUSTRIES.map((i) => (
              <option key={i.code} value={i.code}>
                {`${i.code} · ${i.label}`}
              </option>
            ))}
          </select>
        </div>
        <div className="o-nhap">
          <label htmlFor="asset-employees">Số lao động</label>
          <input
            id="asset-employees"
            name="employee_count"
            inputMode="numeric"
            value={draft.employeeCount}
            onChange={(e) => set({ employeeCount: e.target.value })}
          />
        </div>
        <div className="o-nhap">
          <label htmlFor="asset-established">Ngày thành lập</label>
          <input
            id="asset-established"
            name="established_on"
            type="date"
            value={draft.establishedOn}
            onChange={(e) => set({ establishedOn: e.target.value })}
          />
        </div>
        <div className="o-nhap sm:col-span-2">
          <label htmlFor="asset-description">Mô tả</label>
          <textarea
            id="asset-description"
            name="description"
            value={draft.description}
            onChange={(e) => set({ description: e.target.value })}
          />
        </div>

        {fields !== null && !fields.ok && (
          <p className="thong-bao-loi sm:col-span-2" role="alert">
            {fields.thongBao}
          </p>
        )}
        {fieldItems.map((f) => (
          <CustomField
            key={f.id}
            field={f}
            value={draft.custom[f.field_code] ?? ""}
            onChange={(v) => setDraft((d) => ({ ...d, custom: { ...d.custom, [f.field_code]: v } }))}
          />
        ))}

        {error !== "" && (
          <p className="thong-bao-loi sm:col-span-2" role="alert">
            {error}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2 sm:col-span-2">
          <Button type="button" variant="secondary" onClick={onCancel} disabled={sending}>
            Huỷ
          </Button>
          <Button type="submit" variant="primary" disabled={sending} aria-busy={sending}>
            <BusyLabel busy={sending} label={mode.kind === "create" ? ADD_SUBMIT : EDIT_SUBMIT} busyText={BUSY_SAVING} />
          </Button>
        </div>
      </form>
    </OverlayDialog>
  );
}

/** One custom field, drawn by its `value_type` (`docs/ui-ux/10` §10, `truong_tuy_bien_ban_do`). */
export function CustomField({
  field,
  value,
  onChange,
}: {
  field: comms_mapFieldSchemaOut;
  value: string;
  onChange: (v: string) => void;
}) {
  const id = `asset-custom-${field.field_code}`;
  const label = field.is_required ? `${field.label} *` : field.label;
  let control;
  switch (field.value_type) {
    case "chon":
      control = (
        <select id={id} value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">— Chọn —</option>
          {field.options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      );
      break;
    case "dung-sai":
      control = (
        <select id={id} value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">—</option>
          <option value="true">Có</option>
          <option value="false">Không</option>
        </select>
      );
      break;
    case "ngay":
      control = <input id={id} type="date" value={value} onChange={(e) => onChange(e.target.value)} />;
      break;
    case "so-nguyen":
    case "so-thap-phan":
      control = (
        <input
          id={id}
          inputMode={field.value_type === "so-nguyen" ? "numeric" : "decimal"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      );
      break;
    default:
      control = <input id={id} value={value} onChange={(e) => onChange(e.target.value)} />;
  }
  return (
    <div className="o-nhap" data-custom-field={field.field_code}>
      <label htmlFor={id}>{label}</label>
      {control}
    </div>
  );
}
