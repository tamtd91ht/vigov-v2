"use client";

import { BadgeCheck, Building2, CircleSlash, MapPin, Pencil, Phone, Trash2, User, X, type LucideIcon } from "lucide-react";
import { useId, useState, type ReactNode } from "react";

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
 * The detail of one asset — prototype `AssetCard.tsx`: header (swatch, name, group, close), two badges
 * (verification, status), icon rows for address / representative / phone / tax code, label–value rows,
 * description, coordinates; the write actions in a footer. Beside the map from `xl`, a bottom sheet
 * below that (a 320px column next to the left column and the map does not fit at 1024px). Opened by a
 * CLICK, never hover-only. Personal fields are shown EXACTLY as the server returned them: masked for
 * `asset.read` alone, full for `asset.update` (the server decides and audits the full read; rule 3) —
 * and never as a `tel:` link, which would put the number in a URL (rule 3, forbidden #4).
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
  const a = detail !== "loading" && detail.ok ? detail.duLieu : null;
  return (
    <aside
      className="detail-panel fixed inset-x-0 bottom-0 z-40 flex max-h-[65vh] min-w-0 flex-col overflow-y-auto rounded-t-2xl border border-line bg-surface shadow-lg xl:static xl:z-auto xl:max-h-none xl:w-80 xl:shrink-0 xl:rounded-none xl:border-0 xl:border-l xl:shadow-none"
      aria-labelledby={titleId}
    >
      <header className="flex items-start gap-2 border-b border-line p-4">
        {a !== null && (
          <span aria-hidden="true" className={cn("mt-1 inline-block size-3 shrink-0 rounded-full", groupSwatchClass(a.asset_type_code))} />
        )}
        <div className="min-w-0 flex-1">
          <h2 id={titleId} className="m-0 text-[15px] leading-snug font-semibold break-words text-ink-900">
            {detail === "loading" ? "Đang tải…" : a !== null ? a.name : "Không mở được đối tượng"}
          </h2>
          {a !== null && <p className="m-0 text-xs text-ink-500">{groupLabel(a.asset_type_code)}</p>}
        </div>
        <Button
          type="button"
          variant="icon"
          size="sm"
          aria-label="Đóng thông tin đối tượng"
          icon={<X aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          onClick={onClose}
        />
      </header>
      {detail === "loading" && (
        <p role="status" className="m-0 p-4 text-[13px] text-ink-500">
          Đang tải thông tin đối tượng…
        </p>
      )}
      {detail !== "loading" && !detail.ok && (
        <p className="thong-bao-loi m-4" role="alert">
          {detail.thongBao}
        </p>
      )}
      {a !== null && <AssetFacts a={a} unitName={unitName} fields={fields} />}
      {a !== null && canUpdate && (
        <footer className="mt-auto flex flex-col gap-2 border-t border-line p-4">
          <div className="flex gap-2">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="min-w-0 flex-1"
              icon={<Pencil aria-hidden="true" />}
              onClick={onEdit}
              disabled={busy}
            >
              {EDIT_BUTTON}
            </Button>
            <Button
              type="button"
              variant={a.verified ? "secondary" : "primary"}
              size="sm"
              className="min-w-0 flex-1"
              icon={a.verified ? <CircleSlash aria-hidden="true" /> : <BadgeCheck aria-hidden="true" />}
              onClick={onToggleVerified}
              disabled={busy}
            >
              {a.verified ? UNVERIFY_BUTTON : VERIFY_BUTTON}
            </Button>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="w-full text-danger-600 hover:not-disabled:text-danger-600"
            icon={<Trash2 aria-hidden="true" />}
            onClick={onDelete}
            disabled={busy}
          >
            {DELETE_BUTTON}
          </Button>
        </footer>
      )}
      {actionError !== "" && (
        <p className="thong-bao-loi mx-4 mb-4" role="alert">
          {actionError}
        </p>
      )}
    </aside>
  );
}

/** Icon row of the card (prototype `Row`): icon, small label, value under it. */
function IconRow({ icon: Icon, label, children }: { icon: LucideIcon; label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 gap-2">
      <Icon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-ink-500" />
      <div className="min-w-0">
        <dt className="text-xs text-ink-500">{label}</dt>
        <dd className="m-0 break-words text-ink-900">{children}</dd>
      </div>
    </div>
  );
}

/** Label left, value right (prototype `Field`). */
function FactRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 items-baseline justify-between gap-3">
      <dt className="shrink-0 text-xs text-ink-500">{label}</dt>
      <dd className="m-0 min-w-0 text-right font-medium break-words text-ink-900">{children}</dd>
    </div>
  );
}

function present(v: string | null | undefined): v is string {
  return v !== null && v !== undefined && v !== "";
}

/** The card's body. Like the prototype, a row appears only when the record has the value (address always). */
export function AssetFacts({
  a,
  unitName,
  fields,
}: {
  a: comms_mapAssetOut;
  unitName: (id: string) => string;
  fields: readonly comms_mapFieldSchemaOut[];
}) {
  const byCode = new Map(fields.map((f) => [f.field_code, f]));
  const custom = Object.entries(a.custom_values ?? {});
  const unit = present(a.residential_unit_id) ? unitName(a.residential_unit_id) : "";
  return (
    <>
      <div className="flex flex-wrap gap-1.5 px-4 pt-3">
        <Badge tone={a.verified ? "success" : "neutral"}>{a.verified ? VERIFIED_CHIP : UNVERIFIED_CHIP}</Badge>
        <Badge tone="neutral">{statusLabel(a.status)}</Badge>
      </div>
      <dl className="m-0 flex flex-col gap-2.5 p-4 text-[13px]">
        <IconRow icon={MapPin} label="Địa chỉ">
          {present(a.address) ? a.address : "—"}
          {unit !== "" && <span className="text-ink-500">{` · ${unit}`}</span>}
        </IconRow>
        {present(a.representative) && (
          <IconRow icon={User} label="Người đại diện">
            {a.representative}
          </IconRow>
        )}
        {present(a.phone) && (
          <IconRow icon={Phone} label="Điện thoại">
            <span className="tabular-nums">{a.phone}</span>
          </IconRow>
        )}
        {present(a.tax_code) && (
          <IconRow icon={Building2} label="Mã số thuế">
            <code className="text-xs">{a.tax_code}</code>
          </IconRow>
        )}
        {present(a.industry_code) && <FactRow label="Ngành nghề">{industryLabel(a.industry_code)}</FactRow>}
        {a.employee_count !== null && a.employee_count !== undefined && (
          <FactRow label="Số lao động">{String(a.employee_count)}</FactRow>
        )}
        {present(a.established_on) && <FactRow label="Thành lập">{a.established_on}</FactRow>}
        {custom.map(([code, v]) => {
          const f = byCode.get(code);
          return (
            <FactRow key={code} label={f?.label ?? code}>
              {customValueText(f, v)}
            </FactRow>
          );
        })}
        {present(a.description) && (
          <div className="min-w-0">
            <dt className="text-xs text-ink-500">Mô tả</dt>
            <dd className="m-0 mt-0.5 break-words whitespace-pre-line text-ink-900">{a.description}</dd>
          </div>
        )}
        <FactRow label="Toạ độ">
          <code className="text-[11px]">{`${a.lat.toFixed(5)}, ${a.lng.toFixed(5)}`}</code>
        </FactRow>
      </dl>
    </>
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
