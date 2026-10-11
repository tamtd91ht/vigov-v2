"use client";

import { BadgeCheck, Building2, CircleSlash, Loader2, MapPin, Pencil, Phone, Trash2, User, X, type LucideIcon } from "lucide-react";
import { useId, useState, type ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import type { KetQua } from "@/lib/api/goi";
import type { comms_mapAssetOut, comms_mapFieldSchemaOut, JsonValue } from "@/lib/api/schema.gen";

import {
  DELETE_BUTTON,
  DELETE_CONFIRM,
  DELETE_CONFIRM_BUTTON,
  DELETE_REASON_LABEL,
  EDIT_BUTTON,
  UNVERIFIED_CHIP,
  UNVERIFY_BUTTON,
  VERIFIED_CHIP,
  VERIFY_BUTTON,
  industryText,
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
 * "YYYY-MM-DD" → "d/M/yyyy" (prototype `formatDay`). Read from the string, never through `Date`: a
 * calendar day has no time zone, and `new Date("2020-03-05")` is UTC midnight — the day before west of it.
 */
export function formatDay(iso: string): string {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(iso);
  return m === null ? iso : `${Number(m[3])}/${Number(m[2])}/${m[1]}`;
}

/**
 * Below `xl` a bottom sheet (a 320px column next to the left column and the map does not fit at
 * 1024px); from `xl` the prototype's fixed `w-80` column that narrows the map. One frame for every state.
 */
const PANEL_FRAME =
  "detail-panel fixed inset-x-0 bottom-0 z-40 flex max-h-[65vh] min-w-0 flex-col overflow-y-auto rounded-t-2xl border border-line bg-white shadow-lg xl:static xl:z-auto xl:max-h-none xl:w-80 xl:shrink-0 xl:rounded-none xl:border-0 xl:border-l xl:shadow-none";

/** Verification badge — prototype `VERIFICATION_STYLES`. Two states: the server stores a boolean. */
const VERIFIED_BADGE = "border-leaf/25 bg-leaf/12 text-leaf";
const UNVERIFIED_BADGE = "border-line bg-ink-muted/12 text-ink-muted";
const STATUS_BADGE = "border-line bg-navy/8 text-navy";

export function verificationBadgeClass(verified: boolean): string {
  return verified ? VERIFIED_BADGE : UNVERIFIED_BADGE;
}

/**
 * The detail of one asset — prototype `AssetCard.tsx` (spec 05): header (swatch, name, group, close), two
 * badges (verification, status), icon rows for address / representative / phone / tax code, label–value
 * rows, description, coordinates; the write actions in a footer. Opened by a CLICK, never hover-only.
 *
 * Kept against the prototype (hard rules / owner 11/10/2026):
 *   - personal fields EXACTLY as the server returned them (masked for `asset.read` alone; the server
 *     decides and audits the full read) and the phone as plain text — a `tel:` link would put the
 *     number in a URL (rule 3, forbidden #4);
 *   - the inline delete box carries a REQUIRED reason: a soft delete records why (rule 7);
 *   - "Bỏ xác minh" in the Xác minh slot once verified — the only way to undo a confirmation.
 */
export function DetailPanel({
  detail,
  groupLabel,
  unitName,
  fields,
  canUpdate,
  busy,
  actionError = "",
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
  /** A verification change is in flight. */
  busy: boolean;
  actionError?: string;
  onClose: () => void;
  onEdit: () => void;
  onToggleVerified: () => void;
  /** Soft delete with the officer's reason. `true` once deleted (the screen closes the panel). */
  onDelete: (reason: string) => Promise<boolean>;
}) {
  const titleId = useId();
  if (detail === "loading") {
    return (
      <aside className={cn(PANEL_FRAME, "p-4")} aria-busy="true">
        <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin text-ink-muted" />
        <p role="status" className="an-thi-giac">
          Đang tải thông tin đối tượng…
        </p>
      </aside>
    );
  }
  const a = detail.ok ? detail.duLieu : null;
  return (
    <aside className={PANEL_FRAME} aria-labelledby={titleId}>
      <header className="flex items-start gap-2 border-b border-line p-4">
        {a !== null && (
          <span aria-hidden="true" className={cn("mt-0.5 inline-block size-3 shrink-0 rounded-full", groupSwatchClass(a.asset_type_code))} />
        )}
        <div className="min-w-0 flex-1">
          <h2 id={titleId} className="m-0 text-[15px] leading-snug font-bold break-words text-navy">
            {a !== null ? a.name : "Không mở được đối tượng"}
          </h2>
          {a !== null && <p className="m-0 text-[12px] text-ink-muted">{groupLabel(a.asset_type_code)}</p>}
        </div>
        <Button type="button" variant="ghost" size="sm" className="h-7 px-2" aria-label="Đóng thông tin đối tượng" onClick={onClose}>
          <X aria-hidden="true" focusable="false" className="size-4" />
        </Button>
      </header>
      {!detail.ok && (
        <p className="thong-bao-loi m-4" role="alert">
          {detail.thongBao}
        </p>
      )}
      {a !== null && <AssetFacts a={a} unitName={unitName} fields={fields} />}
      {a !== null && canUpdate && (
        <PanelFooter
          verified={a.verified}
          busy={busy}
          onEdit={onEdit}
          onToggleVerified={onToggleVerified}
          onDelete={onDelete}
        />
      )}
      {actionError !== "" && (
        <p className="thong-bao-loi mx-4 mb-4" role="alert">
          {actionError}
        </p>
      )}
    </aside>
  );
}

/** Prototype footer: [Sửa][Xác minh], then "Xoá khỏi bản đồ" that turns into the inline confirmation. */
function PanelFooter({
  verified,
  busy,
  onEdit,
  onToggleVerified,
  onDelete,
}: {
  verified: boolean;
  busy: boolean;
  onEdit: () => void;
  onToggleVerified: () => void;
  onDelete: (reason: string) => Promise<boolean>;
}) {
  const reasonId = useId();
  const [confirming, setConfirming] = useState(false);
  const [reason, setReason] = useState("");
  const [sending, setSending] = useState(false);
  const clean = reason.trim();

  async function confirm() {
    if (sending || clean === "") return;
    setSending(true);
    const done = await onDelete(clean);
    // Deleted → the screen unmounts this panel; refused → stay, the reason still typed, to retry.
    if (!done) setSending(false);
  }

  return (
    <footer className="mt-auto space-y-2 border-t border-line p-4">
      <div className="flex gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="min-w-0 flex-1"
          icon={<Pencil aria-hidden="true" focusable="false" className="size-3.5" />}
          onClick={onEdit}
          disabled={busy || sending}
        >
          {EDIT_BUTTON}
        </Button>
        <Button
          type="button"
          variant={verified ? "outline" : "primary"}
          size="sm"
          className="min-w-0 flex-1"
          aria-busy={busy}
          icon={
            busy ? (
              <Loader2 aria-hidden="true" focusable="false" className="size-3.5 animate-spin" />
            ) : verified ? (
              <CircleSlash aria-hidden="true" focusable="false" className="size-3.5" />
            ) : (
              <BadgeCheck aria-hidden="true" focusable="false" className="size-3.5" />
            )
          }
          onClick={onToggleVerified}
          disabled={busy || sending}
        >
          {verified ? UNVERIFY_BUTTON : VERIFY_BUTTON}
        </Button>
      </div>
      {confirming ? (
        <div data-delete-confirm="" className="space-y-2 rounded-[9px] border border-danger/30 bg-danger/6 p-2.5">
          <p className="m-0 text-[12px] text-danger">{DELETE_CONFIRM}</p>
          <div>
            <label htmlFor={reasonId} className="block text-[11.5px] font-medium text-ink-900">
              {DELETE_REASON_LABEL}
            </label>
            <textarea
              id={reasonId}
              name="reason"
              required
              rows={2}
              // The officer asked for this box: the reason is the next thing to type.
              autoFocus
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              disabled={sending}
              className="mt-1 block w-full rounded-md border border-line bg-white px-2.5 py-1.5 [font-family:inherit] text-base text-ink-900 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 md:text-[12.5px]"
            />
          </div>
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="min-w-0 flex-1 text-danger hover:not-disabled:text-danger"
              disabled={sending || clean === ""}
              aria-busy={sending}
              icon={sending ? <Loader2 aria-hidden="true" focusable="false" className="size-3.5 animate-spin" /> : undefined}
              onClick={() => void confirm()}
            >
              {DELETE_CONFIRM_BUTTON}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="min-w-0 flex-1"
              disabled={sending}
              onClick={() => {
                setConfirming(false);
                setReason("");
              }}
            >
              Huỷ
            </Button>
          </div>
        </div>
      ) : (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="w-full text-danger hover:not-disabled:text-danger"
          icon={<Trash2 aria-hidden="true" focusable="false" className="size-3.5" />}
          onClick={() => setConfirming(true)}
          disabled={busy}
        >
          {DELETE_BUTTON}
        </Button>
      )}
    </footer>
  );
}

/** Icon row of the card (prototype `Row`): icon, small label, value under it. */
function IconRow({ icon: Icon, label, children }: { icon: LucideIcon; label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 gap-2">
      <Icon aria-hidden="true" focusable="false" className="mt-0.5 size-3.5 shrink-0 text-ink-muted" />
      <div className="min-w-0">
        <dt className="text-[11.5px] text-ink-muted">{label}</dt>
        <dd className="m-0 break-words text-ink">{children}</dd>
      </div>
    </div>
  );
}

/** Label left, value right (prototype `Field`). */
function FactRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 items-baseline justify-between gap-3">
      <dt className="shrink-0 text-[11.5px] text-ink-muted">{label}</dt>
      <dd className="m-0 min-w-0 text-right font-medium break-words text-navy">{children}</dd>
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
        <Badge data-badge="" icon={null} className={verificationBadgeClass(a.verified)}>
          {a.verified ? VERIFIED_CHIP : UNVERIFIED_CHIP}
        </Badge>
        <Badge data-badge="" icon={null} className={STATUS_BADGE}>
          {statusLabel(a.status)}
        </Badge>
      </div>
      <dl className="m-0 space-y-2.5 p-4 text-[12.5px]">
        <IconRow icon={MapPin} label="Địa chỉ">
          {present(a.address) ? a.address : "—"}
          {unit !== "" && <span className="text-ink-muted">{` · ${unit}`}</span>}
        </IconRow>
        {present(a.representative) && (
          <IconRow icon={User} label="Người đại diện">
            {a.representative}
          </IconRow>
        )}
        {present(a.phone) && (
          <IconRow icon={Phone} label="Điện thoại">
            {a.phone}
          </IconRow>
        )}
        {present(a.tax_code) && (
          <IconRow icon={Building2} label="Mã số thuế">
            <code className="text-[11.5px]">{a.tax_code}</code>
          </IconRow>
        )}
        {present(a.industry_code) && <FactRow label="Ngành nghề">{industryText(a.industry_code)}</FactRow>}
        {a.employee_count !== null && a.employee_count !== undefined && (
          <FactRow label="Số lao động">{String(a.employee_count)}</FactRow>
        )}
        {present(a.established_on) && <FactRow label="Thành lập">{formatDay(a.established_on)}</FactRow>}
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
            <dt className="text-[11.5px] text-ink-muted">Mô tả</dt>
            <dd className="m-0 mt-0.5 break-words whitespace-pre-line text-ink">{a.description}</dd>
          </div>
        )}
        <FactRow label="Toạ độ">
          <code className="text-[11px]">{`${a.lat.toFixed(5)}, ${a.lng.toFixed(5)}`}</code>
        </FactRow>
      </dl>
    </>
  );
}
