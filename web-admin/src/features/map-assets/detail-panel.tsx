"use client";

import { BadgeCheck, CircleSlash, Pencil, Trash2, X } from "lucide-react";
import { useId, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { BUSY_DELETING, BusyLabel } from "@/features/danh-ba/busy-label";
import { OverlayDialog } from "@/features/noi-dung/overlay-dialog";
import { cn } from "@/lib/cn";
import type { KetQua } from "@/lib/api/goi";
import { deleteMapAsset } from "@/lib/api/map-assets";
import type { comms_mapAssetOut, comms_mapFieldSchemaOut, JsonValue } from "@/lib/api/schema.gen";

import {
  DELETE_BUTTON,
  DELETE_EXPLANATION,
  DELETE_REASON_REQUIRED,
  EDIT_BUTTON,
  UNVERIFIED_CHIP,
  UNVERIFY_BUTTON,
  VERIFIED_CHIP,
  VERIFY_BUTTON,
  deleteTitle,
  industryLabel,
  statusLabel,
} from "./labels";
import { groupSwatchClass } from "./map-logic";

/** Text of one stored custom value, using the field's option label for `chon`. */
export function customValueText(field: comms_mapFieldSchemaOut | undefined, v: JsonValue): string {
  if (v === null || v === undefined) return "—";
  if (typeof v === "boolean") return v ? "Có" : "Không";
  if (field?.value_type === "chon") return field.options.find((o) => o.value === v)?.label ?? String(v);
  if (typeof v === "string" || typeof v === "number") return String(v);
  return "—";
}

/**
 * The detail of one asset (side panel on desktop, bottom sheet on a phone) — opened by a CLICK, never
 * hover-only. Personal fields are shown EXACTLY as the server returned them: masked for `asset.read`
 * alone, full for `asset.update` (the server decides and audits the full read; rule 3).
 */
export function DetailPanel({
  detail,
  groupLabel,
  unitName,
  fields,
  canUpdate,
  busy,
  actionError,
  onClose,
  onEdit,
  onToggleVerified,
  onDelete,
}: {
  detail: KetQua<comms_mapAssetOut> | "loading";
  groupLabel: (code: string) => string;
  unitName: (id: string) => string;
  fields: readonly comms_mapFieldSchemaOut[];
  canUpdate: boolean;
  busy: boolean;
  actionError: string;
  onClose: () => void;
  onEdit: () => void;
  onToggleVerified: () => void;
  onDelete: () => void;
}) {
  const titleId = useId();
  return (
    <aside
      className="detail-panel fixed inset-x-0 bottom-0 z-40 max-h-[65vh] overflow-y-auto rounded-t-2xl border border-line bg-surface p-4 shadow-lg md:static md:z-auto md:max-h-none md:w-[340px] md:shrink-0 md:rounded-xl md:shadow-sm"
      aria-labelledby={titleId}
    >
      <div className="flex items-start gap-2">
        <h2 id={titleId} className="m-0 min-w-0 flex-1 text-base font-semibold text-ink-900">
          {detail === "loading" ? "Đang tải…" : detail.ok ? detail.duLieu.name : "Không mở được đối tượng"}
        </h2>
        <Button
          type="button"
          variant="icon"
          size="sm"
          aria-label="Đóng thông tin đối tượng"
          icon={<X aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          onClick={onClose}
        />
      </div>
      {detail === "loading" && <p role="status">Đang tải thông tin đối tượng…</p>}
      {detail !== "loading" && !detail.ok && (
        <p className="thong-bao-loi" role="alert">
          {detail.thongBao}
        </p>
      )}
      {detail !== "loading" && detail.ok && (
        <AssetFacts
          a={detail.duLieu}
          groupLabel={groupLabel}
          unitName={unitName}
          fields={fields}
        />
      )}
      {detail !== "loading" && detail.ok && canUpdate && (
        <div className="mt-4 flex flex-wrap gap-2">
          <Button type="button" variant="secondary" size="sm" icon={<Pencil aria-hidden="true" />} onClick={onEdit} disabled={busy}>
            {EDIT_BUTTON}
          </Button>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            icon={detail.duLieu.verified ? <CircleSlash aria-hidden="true" /> : <BadgeCheck aria-hidden="true" />}
            onClick={onToggleVerified}
            disabled={busy}
          >
            {detail.duLieu.verified ? UNVERIFY_BUTTON : VERIFY_BUTTON}
          </Button>
          <Button type="button" variant="danger" size="sm" icon={<Trash2 aria-hidden="true" />} onClick={onDelete} disabled={busy}>
            {DELETE_BUTTON}
          </Button>
        </div>
      )}
      {actionError !== "" && (
        <p className="thong-bao-loi mt-2" role="alert">
          {actionError}
        </p>
      )}
    </aside>
  );
}

export function AssetFacts({
  a,
  groupLabel,
  unitName,
  fields,
}: {
  a: comms_mapAssetOut;
  groupLabel: (code: string) => string;
  unitName: (id: string) => string;
  fields: readonly comms_mapFieldSchemaOut[];
}) {
  const byCode = new Map(fields.map((f) => [f.field_code, f]));
  const rows: [string, string][] = [
    ["Trạng thái", statusLabel(a.status)],
    ["Địa chỉ", a.address ?? ""],
    ["Thôn / Tổ dân phố", a.residential_unit_id ? unitName(a.residential_unit_id) : ""],
    ["Người đại diện", a.representative ?? ""],
    ["Điện thoại", a.phone ?? ""],
    ["Mã số thuế", a.tax_code ?? ""],
    ["Ngành nghề", a.industry_code ? `${a.industry_code} · ${industryLabel(a.industry_code)}` : ""],
    ["Số lao động", a.employee_count === null || a.employee_count === undefined ? "" : String(a.employee_count)],
    ["Ngày thành lập", a.established_on ?? ""],
    ["Mô tả", a.description ?? ""],
  ];
  for (const [code, v] of Object.entries(a.custom_values ?? {})) {
    const f = byCode.get(code);
    rows.push([f?.label ?? code, customValueText(f, v)]);
  }
  return (
    <div className="mt-2 flex flex-col gap-3">
      <p className="m-0 flex flex-wrap items-center gap-2 text-[13px] text-ink-700">
        <span aria-hidden="true" className={cn("inline-block size-3 rounded-full", groupSwatchClass(a.asset_type_code))} />
        <span>{groupLabel(a.asset_type_code)}</span>
        <Badge tone={a.verified ? "success" : "neutral"}>{a.verified ? VERIFIED_CHIP : UNVERIFIED_CHIP}</Badge>
      </p>
      <dl className="m-0 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1.5 text-[13px]">
        {rows.map(([k, v]) => (
          <div key={k} className="contents">
            <dt className="text-ink-500">{k}</dt>
            <dd className="m-0 break-words text-ink-900">{v === "" ? "—" : v}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

/** Delete with a REQUIRED reason (soft delete, rule 7) — the reason goes into the record and its trail. */
export function DeleteAssetDialog({
  asset,
  onDeleted,
  onCancel,
}: {
  asset: { readonly id: string; readonly name: string };
  onDeleted: () => void;
  onCancel: () => void;
}) {
  const titleId = useId();
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);

  async function submit() {
    if (sending) return;
    const clean = reason.trim();
    if (clean === "") return setError(DELETE_REASON_REQUIRED);
    setSending(true);
    const r = await deleteMapAsset(asset.id, clean);
    setSending(false);
    if (!r.ok) return setError(r.thongBao);
    onDeleted();
  }

  return (
    <OverlayDialog titleId={titleId} onDismiss={() => !sending && onCancel()}>
      <form
        className="flex min-w-0 flex-col gap-3 [&>*]:m-0"
        aria-labelledby={titleId}
        noValidate
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <h2 id={titleId} className="text-lg font-semibold text-ink-900">
          {deleteTitle(asset.name)}
        </h2>
        <p className="text-[13px] text-ink-500">{DELETE_EXPLANATION}</p>
        <div className="o-nhap">
          <label htmlFor="asset-delete-reason">Lý do xoá</label>
          <textarea id="asset-delete-reason" name="reason" required value={reason} onChange={(e) => setReason(e.target.value)} />
        </div>
        {error !== "" && (
          <p className="thong-bao-loi" role="alert">
            {error}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="secondary" onClick={onCancel} disabled={sending}>
            Huỷ
          </Button>
          <Button type="submit" variant="danger" disabled={sending} aria-busy={sending}>
            <BusyLabel busy={sending} label="Xác nhận xoá" busyText={BUSY_DELETING} />
          </Button>
        </div>
      </form>
    </OverlayDialog>
  );
}
