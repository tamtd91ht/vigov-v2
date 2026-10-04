"use client";

import { MapPin, Pencil, TriangleAlert } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FieldError, FormMessage, TextAreaField, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { ApiError, getMapFrameDefault, setMapFrameDefault, type MapFrameDefault } from "@/lib/api";

import { formatDateTime } from "./commune-parts";
import {
  buildMapFrameDefaultChange,
  canSave,
  CONFIRM_LABEL,
  DEFAULT_NOTE,
  deviationWarning,
  formatKm,
  formFromView,
  mapFrameDefaultError,
  NOT_CONFIGURED,
  radiusHint,
  warningRadius,
  type MapFrameDefaultField,
  type MapFrameDefaultForm,
  type MapFrameDefaultHints,
} from "./map-frame-default-model";

/**
 * "Khung bản đồ mặc định (Bản đồ kinh tế số)" on a commune's page — ADR 0072 §"Sửa đổi 04/10/2026
 * (lần 2)", K1–K2. The platform's DEFAULT centre + radius for the commune; the commune's own frame,
 * set in its web-admin, wins over it (K3). Read with any `ops.*` key; the edit control is shown only
 * with `ops.tenant.manage` (the PUT's guard) — a hint, the server checks every call.
 *
 * NO MAP WIDGET: this console carries no map library, and adding one to preview a box would be a
 * third-party tile request from the vendor's console. The form is numbers only.
 */

export type MapFrameDefaultState =
  | { status: "loading" }
  | { status: "ready"; view: MapFrameDefault }
  | { status: "error"; message: string };

/** The read-only half, with no loading, so it is rendered in tests. */
export function MapFrameDefaultView({
  state,
  canManage,
  onEdit,
}: {
  state: MapFrameDefaultState;
  canManage: boolean;
  onEdit: () => void;
}) {
  if (state.status === "loading") {
    return (
      <div>
        <p role="status" className="sr-only">
          Đang tải khung bản đồ mặc định…
        </p>
        <SkeletonRows rows={2} columns={2} />
      </div>
    );
  }
  if (state.status === "error") {
    return <ErrorState role="alert" title="Chưa tải được khung bản đồ mặc định" message={state.message} className="py-6" />;
  }
  const v = state.view;
  const configured = v.configured && v.center_lat !== undefined && v.center_lng !== undefined && v.radius_km !== undefined;
  return (
    <>
      {configured ? (
        <dl className="m-0 grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-3">
          <Item label="Vĩ độ tâm" value={v.center_lat?.toFixed(6) ?? ""} mono />
          <Item label="Kinh độ tâm" value={v.center_lng?.toFixed(6) ?? ""} mono />
          <Item label="Bán kính" value={`${formatKm(v.radius_km ?? 0)} km`} />
          <Item label="Cập nhật lúc" value={v.updated_at ? formatDateTime(v.updated_at) : ""} />
          <Item label="Cập nhật bởi" value={v.updated_by ?? ""} mono />
        </dl>
      ) : (
        <p className="m-0 text-sm text-ink-700">{NOT_CONFIGURED}</p>
      )}
      <Notice tone="info">{DEFAULT_NOTE}</Notice>
      {canManage ? (
        <div className="border-t border-line pt-4">
          <Button
            type="button"
            variant="secondary"
            onClick={onEdit}
            icon={<Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          >
            {configured ? "Sửa khung mặc định" : "Đặt khung mặc định"}
          </Button>
        </div>
      ) : null}
    </>
  );
}

function Item({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <dt className="text-xs font-medium text-ink-500">{label}</dt>
      <dd className={mono ? "m-0 font-mono text-sm break-all text-ink-900" : "m-0 text-sm font-semibold text-ink-900"}>
        {value}
      </dd>
    </div>
  );
}

/** The dialog body, without saving, so the warning and the disabled Lưu are rendered in tests. */
export function MapFrameDefaultFormFields({
  hints,
  form,
  onChange,
  busy,
  error,
}: {
  hints: MapFrameDefaultHints;
  form: MapFrameDefaultForm;
  onChange: (f: MapFrameDefaultForm) => void;
  busy: boolean;
  error: { field: MapFrameDefaultField; text: string } | null;
}) {
  const at = (f: MapFrameDefaultField) => (error?.field === f ? error.text : null);
  const set = (patch: Partial<MapFrameDefaultForm>) => onChange({ ...form, ...patch });
  const warned = warningRadius(form, hints);

  return (
    <>
      <Notice tone="info">{DEFAULT_NOTE}</Notice>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <TextField
          label="Vĩ độ tâm"
          hint="Tối đa 6 chữ số thập phân, ví dụ 21.028511."
          name="center_lat"
          type="text"
          inputMode="decimal"
          autoComplete="off"
          value={form.lat}
          onChange={(e) => set({ lat: e.target.value })}
          disabled={busy}
          error={at("lat")}
        />
        <TextField
          label="Kinh độ tâm"
          hint="Tối đa 6 chữ số thập phân, ví dụ 105.804817."
          name="center_lng"
          type="text"
          inputMode="decimal"
          autoComplete="off"
          value={form.lng}
          onChange={(e) => set({ lng: e.target.value })}
          disabled={busy}
          error={at("lng")}
        />
      </div>
      <TextField
        label="Bán kính (km)"
        hint={`${radiusHint(hints)} Bước 0,1 km.`}
        name="radius_km"
        type="text"
        inputMode="decimal"
        autoComplete="off"
        value={form.radius}
        // A confirmation is for ONE radius: changing it clears both the tick and the server's ask.
        onChange={(e) => set({ radius: e.target.value, acknowledged: false, serverAskedConfirmation: false })}
        disabled={busy}
        error={at("radius")}
      />
      {warned !== null ? (
        <Notice tone="legal" icon={TriangleAlert} title="Cảnh báo:" role="group" aria-label="Cảnh báo bán kính">
          <p>{deviationWarning(warned, hints)}</p>
          <label className="flex min-h-9 cursor-pointer items-center gap-2.5 text-sm font-semibold text-ink-900">
            <input
              type="checkbox"
              name="acknowledged_unusual"
              checked={form.acknowledged}
              disabled={busy}
              onChange={(e) => set({ acknowledged: e.target.checked })}
              className="m-0 size-5 shrink-0 cursor-pointer accent-brand-600"
            />
            {CONFIRM_LABEL}
          </label>
          {at("acknowledge") ? <FieldError id="map-frame-ack-error" text={at("acknowledge") ?? ""} /> : null}
        </Notice>
      ) : null}
      <TextAreaField
        label="Lý do"
        hint="Bắt buộc, tối đa 500 ký tự. Lý do được ghi vào nhật ký vận hành."
        name="reason"
        rows={3}
        maxLength={500}
        value={form.reason}
        onChange={(e) => set({ reason: e.target.value })}
        disabled={busy}
        error={at("reason")}
      />
      <FormMessage
        text={error?.field === "form" || (error?.field === "acknowledge" && warned === null) ? error.text : null}
      />
    </>
  );
}

/** Lưu, disabled while a warning shows unticked. Exported so the disabled state is rendered in tests. */
export function MapFrameDefaultSaveButton({
  hints,
  form,
  busy,
}: {
  hints: MapFrameDefaultHints;
  form: MapFrameDefaultForm;
  busy: boolean;
}) {
  return (
    <Button type="submit" variant="primary" disabled={busy || !canSave(form, hints)} aria-busy={busy}>
      {busy ? "Đang lưu…" : "Lưu khung mặc định"}
    </Button>
  );
}

function MapFrameDefaultDialog({
  communeId,
  view,
  onClose,
  onSaved,
}: {
  communeId: string;
  view: MapFrameDefault;
  onClose: () => void;
  onSaved: (v: MapFrameDefault) => void;
}) {
  const guarded = useGuardedError();
  const [form, setForm] = useState<MapFrameDefaultForm>(() => formFromView(view));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ field: MapFrameDefaultField; text: string } | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const built = buildMapFrameDefaultChange(form, view);
    if (!built.ok) return setError({ field: built.field, text: built.text });
    setBusy(true);
    setError(null);
    try {
      onSaved(await setMapFrameDefault(communeId, built.body));
    } catch (err) {
      let field: MapFrameDefaultField = "form";
      const text = guarded(err, "đặt khung bản đồ mặc định", (e) => {
        const known = mapFrameDefaultError(e);
        if (known) field = known.field;
        return known?.text ?? null;
      });
      if (err instanceof ApiError && err.code === "radius_unusual_unconfirmed") {
        // The server's band decides: show the warning again, unticked, whatever this copy says.
        setForm((f) => ({ ...f, acknowledged: false, serverAskedConfirmation: true }));
      }
      if (text !== null) setError({ field, text });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open title="Khung bản đồ mặc định" icon={MapPin} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <MapFrameDefaultFormFields hints={view} form={form} onChange={setForm} busy={busy} error={error} />
        <DialogActions>
          <MapFrameDefaultSaveButton hints={view} form={form} busy={busy} />
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

export function MapFrameDefaultSection({ communeId, canManage }: { communeId: string; canManage: boolean }) {
  const guarded = useGuardedError();
  const [state, setState] = useState<MapFrameDefaultState>({ status: "loading" });
  const [editing, setEditing] = useState(false);

  useEffect(() => {
    let alive = true;
    getMapFrameDefault(communeId).then(
      (view) => {
        if (alive) setState({ status: "ready", view });
      },
      (err: unknown) => {
        if (!alive) return;
        const message = guarded(err, "xem khung bản đồ mặc định", (e) => mapFrameDefaultError(e)?.text ?? null);
        if (message !== null) setState({ status: "error", message });
      },
    );
    return () => {
      alive = false;
    };
  }, [communeId, guarded]);

  return (
    <>
      <MapFrameDefaultView state={state} canManage={canManage && state.status === "ready"} onEdit={() => setEditing(true)} />
      {editing && state.status === "ready" ? (
        <MapFrameDefaultDialog
          communeId={communeId}
          view={state.view}
          onClose={() => setEditing(false)}
          onSaved={(view) => {
            // The answer replaces what is on screen; the page never patches its own copy.
            setState({ status: "ready", view });
            setEditing(false);
          }}
        />
      ) : null}
    </>
  );
}
