"use client";

import { Crosshair, Loader2, Search } from "lucide-react";
import { useEffect, useId, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { cn } from "@/lib/cn";
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
  EDIT_TITLE,
  INDUSTRIES,
  MASKED_NOTE,
  MY_LOCATION_BUTTON,
  MY_LOCATION_DONE,
  MY_LOCATION_FAILED,
  MY_LOCATION_OUTSIDE,
  MY_LOCATION_TITLE,
  MY_LOCATION_UNSUPPORTED,
  NO_CHANGE,
  PIN_OUTSIDE_FRAME,
  PLACE_SEARCH_PLACEHOLDER,
  SAVE_FAILED,
  STATUS_OPTIONS,
  industryText,
  pendingPart,
} from "./labels";
import { insideFrame, round6, type MapFrame } from "./map-logic";
import { PositionPicker } from "./position-picker";
import { HelpHint } from "./side-panel";

export type AssetDialogMode =
  | { readonly kind: "create"; readonly assetTypeCode: string; readonly idempotencyKey: string }
  | { readonly kind: "edit"; readonly asset: comms_mapAssetOut };

/* Prototype `AssetFormDialog.tsx` (spec 07): shadcn `Label` (14px/500) above, control 6px below. */
const LABEL = "block text-sm leading-none font-medium text-ink-900";
const INPUT = cn(controlClass, "mt-1.5 h-9 rounded-md border-line bg-white px-3");
/** The prototype's `SELECT_CLASS`, on the app's control frame. */
const SELECT = cn(controlClass, "mt-1.5 h-9 w-full rounded-md border-line bg-white px-3 text-[13px] md:text-[13px]");
const TEXTAREA = cn(controlClass, "mt-1.5 h-auto rounded-md border-line bg-white px-3 py-2");
/** Vĩ độ / Kinh độ under the pin map (prototype `LocationPicker.tsx`). */
const COORD_LABEL = "block text-[11px] leading-none font-medium text-ink-900";
const COORD_INPUT = cn(controlClass, "mt-1 h-8 rounded-md border-line bg-white text-[12px] md:text-[12px]");

/**
 * "Thêm đối tượng lên bản đồ" / "Sửa đối tượng trên bản đồ" — prototype `AssetFormDialog.tsx` +
 * `LocationPicker.tsx` (spec 07), in the app's `ModalDialog` (the prototype's `Dialog`, with its ✕).
 * The rules live in `asset-form.ts`; this file draws them. Server refusals (409 tax code taken, 422 group
 * unavailable / custom values invalid) keep the server's own sentence in the form, under the
 * prototype's "Không lưu được." toast.
 *
 * Kept against the prototype (hard rules / owner 11/10/2026):
 *   - the position stays REQUIRED and inside the commune frame (ADR 0072): no "Bỏ toạ độ";
 *   - masked personal fields cannot be edited (rule 3);
 *   - no commune's places or coordinates as placeholders (rule 1), and no "Trùng mã là cập nhật…" under
 *     Mã số thuế — this server answers 409 on a taken tax code, so that sentence would be false;
 *   - "Ngày thành lập" stays (the record has it, and nothing else edits it);
 *   - the place search needs a geocoder this system does not have: disabled, the reason in its hint.
 */
export function AssetDialog({
  mode,
  types,
  units,
  frame,
  mapAvailable,
  onSaved,
  onCancel,
}: {
  mode: AssetDialogMode;
  types: readonly comms_loaiTaiNguyenRa[];
  units: readonly identity_thonToDanPhoRa[];
  frame: MapFrame | null;
  /** The page draws its map (self-hosted basemap configured, frame known, style loaded): show the pin map. */
  mapAvailable: boolean;
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

  const editing = mode.kind === "edit";
  const masked = editing && mode.asset.masked;
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
    const r =
      mode.kind === "create"
        ? await (async () => {
            const b = createBody(draft, all, bounds);
            if (b.kind !== "send") return b;
            setSending(true);
            return createMapAsset(b.body, mode.idempotencyKey);
          })()
        : await (async () => {
            const b = updateBody(mode.asset, draft, all, bounds);
            if (b.kind !== "send") return b;
            setSending(true);
            return updateMapAsset(mode.asset.id, b.body);
          })();
    setSending(false);
    if ("kind" in r) return setError(r.kind === "error" ? r.message : NO_CHANGE);
    if (!r.ok) {
      // The prototype's toast; the server's own sentence (409 tax code taken…) stays in the form to read.
      toast.error(SAVE_FAILED);
      return setError(r.thongBao);
    }
    onSaved(r.duLieu, mode.kind === "create");
  }

  function myLocation() {
    if (frame === null) return;
    if (typeof navigator === "undefined" || !navigator.geolocation) return void toast.error(MY_LOCATION_UNSUPPORTED);
    navigator.geolocation.getCurrentPosition(
      (p) => {
        const { latitude, longitude } = p.coords;
        if (!insideFrame(frame.bounds, longitude, latitude)) return void toast.error(MY_LOCATION_OUTSIDE);
        setError("");
        set({ lat: formatCoordinate(latitude), lng: formatCoordinate(longitude) });
        toast.success(MY_LOCATION_DONE);
      },
      () => toast.error(MY_LOCATION_FAILED),
      { enableHighAccuracy: true, timeout: 10000 },
    );
  }

  const pLat = parseCoordinate(draft.lat, 90);
  const pLng = parseCoordinate(draft.lng, 180);
  const typedOutside = bounds !== null && pLat !== null && pLng !== null && !insideFrame(bounds, pLng, pLat);
  const activeTypes = types.filter((t) => t.active || t.code === draft.assetTypeCode);
  const pickableUnits = units.filter((u) => u.active || u.id === draft.residentialUnitId);

  return (
    <ModalDialog
      titleId={titleId}
      onDismiss={() => !sending && onCancel()}
      closeDisabled={sending}
      size="lg"
      initialFocusId="asset-name"
      className="max-h-[86vh] max-w-2xl overflow-auto"
    >
      <ModalDialogHeader titleId={titleId} title={editing ? EDIT_TITLE : ADD_TITLE} description={ADD_DESCRIPTION} />
      <form
        className="asset-form grid min-w-0 gap-4 sm:grid-cols-2"
        aria-labelledby={titleId}
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <FormField id="asset-group" label="Nhóm">
          {/* Locked on edit (prototype): a record does not change group — its custom fields belong to it. */}
          <select
            id="asset-group"
            name="asset_type_code"
            className={SELECT}
            value={draft.assetTypeCode}
            disabled={editing}
            onChange={(e) => set({ assetTypeCode: e.target.value, custom: {} })}
          >
            {activeTypes.map((t) => (
              <option key={t.id} value={t.code}>
                {t.label}
              </option>
            ))}
          </select>
        </FormField>
        <FormField id="asset-status" label="Trạng thái">
          <select id="asset-status" name="status" className={SELECT} value={draft.status} onChange={(e) => set({ status: e.target.value })}>
            {STATUS_OPTIONS.map((s) => (
              <option key={s.value} value={s.value}>
                {s.label}
              </option>
            ))}
          </select>
        </FormField>
        <FormField id="asset-name" label="Tên" wide>
          <input id="asset-name" name="name" className={INPUT} value={draft.name} onChange={(e) => set({ name: e.target.value })} />
        </FormField>
        <FormField id="asset-address" label="Địa chỉ" wide>
          <input id="asset-address" name="address" className={INPUT} value={draft.address} onChange={(e) => set({ address: e.target.value })} />
        </FormField>
        <FormField id="asset-unit" label="Thôn / Tổ dân phố">
          <select
            id="asset-unit"
            name="residential_unit_id"
            className={SELECT}
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
        </FormField>

        <fieldset className="m-0 min-w-0 border-0 p-0 sm:col-span-2">
          <legend className={cn(LABEL, "p-0")}>Vị trí trên bản đồ</legend>
          <div className="mt-1.5 space-y-2">
            {mapAvailable && frame !== null && (
              <>
                <div className="flex items-center gap-1.5">
                  <div className="relative min-w-0 flex-1">
                    <Search
                      aria-hidden="true"
                      focusable="false"
                      className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-ink-muted"
                    />
                    <input
                      id="asset-place-search"
                      aria-label={pendingPart("Tìm địa điểm theo tên").ten}
                      disabled
                      placeholder={PLACE_SEARCH_PLACEHOLDER}
                      className={cn(INPUT, "mt-0 pl-8 text-[12.5px] md:text-[12.5px]")}
                    />
                  </div>
                  <HelpHint info={pendingPart("Tìm địa điểm theo tên")} />
                </div>
                <PositionPicker
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
            {/* The prototype's one row from `sm`; at 320px the two inputs would be ~65px wide beside the
                button, too narrow for "15.700000", so the button takes its own line there. */}
            <div data-coordinates="" className="grid grid-cols-2 items-end gap-2 sm:grid-cols-[1fr_1fr_auto]">
              <div className="min-w-0">
                <label htmlFor="asset-lat" className={COORD_LABEL}>
                  Vĩ độ
                </label>
                <input
                  id="asset-lat"
                  name="lat"
                  inputMode="decimal"
                  className={COORD_INPUT}
                  value={draft.lat}
                  onChange={(e) => set({ lat: e.target.value })}
                  onBlur={() => pLat !== null && set({ lat: round6(pLat).toFixed(6) })}
                />
              </div>
              <div className="min-w-0">
                <label htmlFor="asset-lng" className={COORD_LABEL}>
                  Kinh độ
                </label>
                <input
                  id="asset-lng"
                  name="lng"
                  inputMode="decimal"
                  className={COORD_INPUT}
                  value={draft.lng}
                  onChange={(e) => set({ lng: e.target.value })}
                  onBlur={() => pLng !== null && set({ lng: round6(pLng).toFixed(6) })}
                />
              </div>
              {frame !== null && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="col-span-2 sm:col-span-1"
                  title={MY_LOCATION_TITLE}
                  icon={<Crosshair aria-hidden="true" focusable="false" className="size-4" />}
                  onClick={myLocation}
                >
                  {MY_LOCATION_BUTTON}
                </Button>
              )}
            </div>
            {typedOutside && (
              <p className="thong-bao-loi m-0" role="alert">
                {PIN_OUTSIDE_FRAME}
              </p>
            )}
          </div>
        </fieldset>

        {masked && <p className="m-0 text-[13px] text-ink-muted sm:col-span-2">{MASKED_NOTE}</p>}
        <FormField id="asset-representative" label="Người đại diện">
          <input
            id="asset-representative"
            name="representative"
            className={INPUT}
            value={draft.representative}
            disabled={masked}
            onChange={(e) => set({ representative: e.target.value })}
          />
        </FormField>
        <FormField id="asset-phone" label="Điện thoại">
          <input
            id="asset-phone"
            name="phone"
            type="tel"
            inputMode="tel"
            className={INPUT}
            value={draft.phone}
            disabled={masked}
            onChange={(e) => set({ phone: e.target.value })}
          />
        </FormField>
        <FormField id="asset-tax-code" label="Mã số thuế">
          <input
            id="asset-tax-code"
            name="tax_code"
            className={INPUT}
            value={draft.taxCode}
            disabled={masked}
            onChange={(e) => set({ taxCode: e.target.value })}
          />
        </FormField>
        <FormField id="asset-industry" label="Ngành nghề">
          <select id="asset-industry" name="industry_code" className={SELECT} value={draft.industryCode} onChange={(e) => set({ industryCode: e.target.value })}>
            <option value="">— Chưa phân ngành —</option>
            {INDUSTRIES.map((i) => (
              <option key={i.code} value={i.code}>
                {industryText(i.code)}
              </option>
            ))}
          </select>
        </FormField>
        <FormField id="asset-employees" label="Số lao động">
          <input
            id="asset-employees"
            name="employee_count"
            inputMode="numeric"
            className={INPUT}
            value={draft.employeeCount}
            onChange={(e) => set({ employeeCount: e.target.value })}
          />
        </FormField>
        <FormField id="asset-established" label="Ngày thành lập">
          <input
            id="asset-established"
            name="established_on"
            type="date"
            className={INPUT}
            value={draft.establishedOn}
            onChange={(e) => set({ establishedOn: e.target.value })}
          />
        </FormField>

        {fields !== null && !fields.ok && (
          <p className="thong-bao-loi m-0 sm:col-span-2" role="alert">
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

        <FormField id="asset-description" label="Mô tả" wide>
          <textarea
            id="asset-description"
            name="description"
            rows={3}
            className={TEXTAREA}
            value={draft.description}
            onChange={(e) => set({ description: e.target.value })}
          />
        </FormField>

        {error !== "" && (
          <p className="thong-bao-loi m-0 sm:col-span-2" role="alert">
            {error}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2 sm:col-span-2">
          <Button type="button" variant="outline" onClick={onCancel} disabled={sending}>
            Huỷ
          </Button>
          <Button
            type="submit"
            variant="primary"
            disabled={sending}
            aria-busy={sending}
            icon={sending ? <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" /> : undefined}
          >
            {editing ? EDIT_SUBMIT : ADD_SUBMIT}
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}

/** Label above, control below (prototype `Label` + `mt-1.5`); `wide` spans both columns. */
function FormField({ id, label, wide = false, children }: { id: string; label: ReactNode; wide?: boolean; children: ReactNode }) {
  return (
    <div className={cn("min-w-0", wide && "sm:col-span-2")}>
      <label htmlFor={id} className={LABEL}>
        {label}
      </label>
      {children}
    </div>
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
  let control;
  switch (field.value_type) {
    case "chon":
      control = (
        <select id={id} className={SELECT} value={value} onChange={(e) => onChange(e.target.value)}>
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
        <select id={id} className={SELECT} value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">—</option>
          <option value="true">Có</option>
          <option value="false">Không</option>
        </select>
      );
      break;
    case "ngay":
      control = <input id={id} type="date" className={INPUT} value={value} onChange={(e) => onChange(e.target.value)} />;
      break;
    case "so-nguyen":
    case "so-thap-phan":
      control = (
        <input
          id={id}
          className={INPUT}
          inputMode={field.value_type === "so-nguyen" ? "numeric" : "decimal"}
          value={value}
          onChange={(e) => onChange(e.target.value)}
        />
      );
      break;
    default:
      control = <input id={id} className={INPUT} value={value} onChange={(e) => onChange(e.target.value)} />;
  }
  return (
    <div className="min-w-0" data-custom-field={field.field_code}>
      <label htmlFor={id} className={LABEL}>
        {field.label}
        {field.is_required && <span className="text-danger"> *</span>}
      </label>
      {control}
    </div>
  );
}
