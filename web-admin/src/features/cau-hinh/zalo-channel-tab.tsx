"use client";

import { Check, Copy, Loader2, Pencil, Plus, RefreshCw, TriangleAlert, Undo2, Unlink, X } from "lucide-react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { NoAccess } from "@/components/ui/no-access";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { formatVietnamDateTime } from "@/features/noi-dung/nhan-noi-dung";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { cn } from "@/lib/cn";
// vi-name-ok: the existing unit-list reader of lib/api/danh-muc.ts, imported, not a new name
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
// vi-name-ok: the existing staff-directory reader of lib/api/danh-ba-chon-nguoi.ts, imported, not a new name
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
// vi-name-ok: the existing result type and sentence of lib/api/goi.ts, imported, not a new name
import { LOI_KHONG_RO, type KetQua } from "@/lib/api/goi";
import type { identity_danhSachBoPhanRa } from "@/lib/api/schema.gen";
import {
  checkCommuneZaloBot,
  createZaloGroupPairingCode,
  editZaloGroup,
  getCommuneZaloBot,
  getZaloChannelSettings,
  listZaloGroups,
  listZaloLinkedStaff,
  registerCommuneZaloWebhook,
  retireCommuneZaloBot,
  saveCommuneZaloBot,
  saveZaloChannelSettings,
  unlinkZaloGroup,
  type comms_groupPairingCodeOut,
  type comms_zaloGroupList,
  type comms_zaloGroupOut,
  type CommuneZaloBotCurrent,
  type ZaloChannelSettings,
  type ZaloLinkedStaff,
} from "@/lib/api/zalo";

// vi-name-ok: the existing org-chart tree helpers of cay-bo-phan.ts, imported, not new names
import { dungCay, traiPhang } from "./cay-bo-phan";
import { CommuneLaunchQrSection } from "./commune-launch-qr";
import { ConfigDialog } from "./config-dialog";
import { selectCls } from "./config-ui";
import { zaloChannelTabDecision } from "./quyen-tab";
import {
  botDescription,
  botPill,
  botTokenHint,
  buildBotChange,
  buildChange,
  CHANNEL_DESCRIPTION,
  CHECK_FAILED_TOAST,
  checkResultText,
  checkRetireReason,
  dayOptions,
  draftFromSettings,
  DUE_SOON_HINT,
  DUE_SOON_MIN,
  DUE_SOON_NO_LEAD,
  DUE_SOON_OUT_OF_RANGE,
  endedLinksText,
  EVENT_NOT_OFFERED,
  EVENTS_HINT,
  hourOptions,
  isOutcomeOk,
  joinStaffLinks,
  lastCheckText,
  NO_LINKED_STAFF,
  PLATFORM_NOT_READY,
  QUIET_HINT,
  relinkSentence,
  RETIRE_REASON_MAX,
  rowKinds,
  SAVE_FAILED_TOAST,
  SAVED_TOAST,
  toggleKind,
  WEBHOOK_SECRET_ONCE,
  webhookRequestFailedText,
  webhookResultText,
  WHEN_DESCRIPTION,
  ZALO_EVENT_GROUPS,
  type BotDraft,
  type BotSaveKind,
  type DirectoryPerson,
  type UnitName,
  type ZaloChannelDraft,
} from "./zalo-channel-form";
import {
  buildPatch,
  CHANNEL_OFF_REASON,
  checkDraft,
  checkUnlinkReason,
  CODE_EXPIRED,
  CODE_ONCE,
  COMMUNE_WIDE,
  countdownText,
  draftFromGroup,
  droppedKinds,
  droppedKindsText,
  groupKindSections,
  GROUPS_DESCRIPTION,
  GROUPS_HOW_IT_WORKS,
  isRowChecked,
  kindNames,
  KINDS_HINT,
  linkedText,
  NO_BOT_REASON,
  NO_GROUP_KIND_REASON,
  offerableKinds,
  refusalFor,
  SCOPE_HINT,
  scopeText,
  toggleGroupKinds,
  UNIT_REMOVED_WARNING,
  UNLINK_CONSEQUENCE,
  UNLINK_REASON_MAX,
  UNLINKED_TOAST,
  type GroupDraft,
  type GroupKindSection,
  type Refusal,
  type RefusalField,
} from "./zalo-groups-form";

type Directory = KetQua<{ items: DirectoryPerson[] }>;
/** `GET /api/v1/org-units` as read: the people join needs `UnitName`, the group scope picker the tree. */
type Units = KetQua<identity_danhSachBoPhanRa>;

/**
 * "Cấu hình → Kênh Zalo" (ADR 0074 #6), laid out by spec 11 and the prototype's `ZaloChannelPanel`
 * (ADR 0079). One key, `admin.zalo` (since 15/10/2026; was `admin.lookup`), for read and write, so the tab hides as a
 * whole without it (convenience; comms refuses).
 *
 * EVERY CHANGE SAVES AT ONCE (spec 11): the whole settings row is PUT — the server's PUT replaces the
 * row — and a refusal puts the screen back to what is stored, so it never shows a state that was not
 * saved. All controls are disabled while one save is in flight: two overlapping full-row PUTs would
 * let the slower one overwrite the other's change.
 *
 * WHAT THE SERVER CANNOT DO YET is drawn at its spec position, disabled, with "?" (ADR 0068 §14): the
 * events no producer sends (ADR 0079 Q3). "Sắp đến hạn: nhắc trước" is a real field since ADR 0079 lô 5
 * Q13 — it narrows the ZALO copy only; the bell keeps the SLA table's column. The linked list is names,
 * codes and a date — never a Zalo chat id.
 *
 * §3 "Con bot của xã" is `CommuneBotSection`: its own reads and writes, independent of the autosave.
 */
export function ZaloChannelTab() {
  const phien = usePhien();
  const decision = phien === null ? null : zaloChannelTabDecision(phien);
  const allowed = decision !== null && decision.hien;

  const [loaded, setLoaded] = useState<KetQua<ZaloChannelSettings> | null>(null);
  const [draft, setDraft] = useState<ZaloChannelDraft | null>(null);
  const [staff, setStaff] = useState<KetQua<ZaloLinkedStaff[]> | null>(null);
  const [directory, setDirectory] = useState<Directory | null>(null);
  const [units, setUnits] = useState<Units | null>(null);
  const [saving, setSaving] = useState(false);
  const [held, setHeld] = useState<string | null>(null);
  const [dueSoonError, setDueSoonError] = useState<string | null>(null);
  /**
   * Spec 11 §0. Its own state, not `loaded.platform_ready`: comms sends it on the GET only, and the PUT
   * reply that replaces `loaded` after every autosave does not carry it. null = not known — no warning
   * is drawn on a guess.
   */
  const [platformReady, setPlatformReady] = useState<boolean | null>(null);
  /** Bumped after a bot switch: comms ended every group link with it (ADR 0092), so the list is read again. */
  const [groupsVersion, setGroupsVersion] = useState(0);

  const readPlatformReady = useCallback(() => {
    // After a bot switch the answer may change. A failed re-read keeps what was shown.
    getZaloChannelSettings().then((r) => {
      if (r.ok) setPlatformReady(r.duLieu.platform_ready ?? null);
    });
  }, []);

  useEffect(() => {
    if (!allowed) return;
    let gone = false;
    getZaloChannelSettings().then((r) => {
      if (gone) return;
      setLoaded(r);
      if (r.ok) {
        setDraft(draftFromSettings(r.duLieu));
        setPlatformReady(r.duLieu.platform_ready ?? null);
      }
    });
    listZaloLinkedStaff().then((r) => {
      if (!gone) setStaff(r);
    });
    // Who has NOT linked yet = the commune's staff directory minus the links (AnyAuthenticated,
    // code · name · title · unit only). Unit names come from the unit list (also AnyAuthenticated).
    layDanhBaChonNguoi().then((r) => {
      if (!gone) setDirectory(r);
    });
    layDanhMucBoPhan().then((r) => {
      if (!gone) setUnits(r);
    });
    return () => {
      gone = true;
    };
  }, [allowed]);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p className="text-danger m-0 text-[12.5px] font-medium" role="alert">
        {decision.thongBao}
      </p>
    ) : (
      <div className="flex min-w-0 flex-col items-center pb-10">
        <NoAccess className="pb-4" />
        <p className="text-ink-muted m-0 max-w-md px-4 text-center text-[13px]">
          Tài khoản của bạn không có quyền cấu hình kênh Zalo, nên tab này không hiển thị.
        </p>
      </div>
    );
  }
  if (loaded === null)
    return (
      <>
        <p role="status" className="an-thi-giac">
          Đang tải cấu hình kênh Zalo…
        </p>
        <Skeleton className="h-64 w-full" />
      </>
    );
  if (!loaded.ok) return <ErrorState role="alert" title="Chưa tải được cấu hình kênh Zalo" message={loaded.thongBao} />;
  if (draft === null) return null;
  const saved = loaded.duLieu;

  async function apply(next: ZaloChannelDraft) {
    if (saving) return;
    const built = buildChange(next);
    if (!built.ok) {
      if (built.held) {
        // Half-way through picking the two cadence numbers: keep what was picked, send nothing yet.
        setDraft(next);
        setHeld(built.text);
        return;
      }
      setDraft(draftFromSettings(saved));
      setHeld(null);
      toast.error(built.text);
      return;
    }
    setDraft(next);
    setHeld(null);
    setDueSoonError(null);
    setSaving(true);
    const r = await saveZaloChannelSettings(built.change);
    setSaving(false);
    if (!r.ok) {
      setDraft(draftFromSettings(saved));
      if (r.code === DUE_SOON_OUT_OF_RANGE) {
        // A field error, in the server's words, under the field it is about (ADR 0079 §Giữ bất kể spec,
        // "Thông báo": "lỗi biểu mẫu hiện tại chỗ") — the select is back at the stored value, the sentence says why.
        setDueSoonError(r.thongBao);
        return;
      }
      // A refusal the server explained (a rule, a permission) is shown in its words; a lost connection
      // gets the spec's sentence.
      toast.error(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    setLoaded(r);
    setDraft(draftFromSettings(r.duLieu));
    toast.success(SAVED_TOAST);
  }

  return (
    <ZaloChannelView
      draft={draft}
      supported={saved.supported_events}
      platformReady={platformReady}
      onChange={(d) => void apply(d)}
      saving={saving}
      held={held}
      dueSoonError={dueSoonError}
      people={peopleState(staff, directory, units)}
      botSection={
        <CommuneBotSection
          onSwitched={() => {
            readPlatformReady();
            setGroupsVersion((v) => v + 1);
          }}
        />
      }
      groupSection={
        <ZaloGroupsSection
          key={groupsVersion}
          channelEnabled={saved.is_enabled}
          enabledKinds={saved.kinds}
          platformReady={platformReady}
          units={units}
        />
      }
      launchQrSection={<CommuneLaunchQrSection />}
    />
  );
}

/**
 * What "Ai đã ghép nối" and the §1 count can show:
 *   - `loading`  — a read still running;
 *   - `joined`   — directory + links: every staff member, linked or not, and "{linked}/{total}";
 *   - `linkedOnly` — the directory could NOT be read: fall back to the links alone, say why, and
 *     claim no total and no "not linked" — a list missing the unlinked must not read as "everyone";
 *   - `failed`   — the links themselves could not be read: nothing about pairing is known.
 */
export type PeopleState =
  | { state: "loading" }
  | { state: "joined"; rows: ReturnType<typeof joinStaffLinks>["rows"]; linked: number; total: number }
  | { state: "linkedOnly"; links: ZaloLinkedStaff[]; reason: string }
  | { state: "failed"; reason: string };

export function peopleState(
  staff: KetQua<ZaloLinkedStaff[]> | null,
  directory: Directory | null,
  units: KetQua<{ items: UnitName[] }> | null,
): PeopleState {
  if (staff === null || directory === null || units === null) return { state: "loading" };
  if (!staff.ok) return { state: "failed", reason: staff.thongBao };
  if (!directory.ok) return { state: "linkedOnly", links: staff.duLieu, reason: directory.thongBao };
  // Unit names are presentation only: without them the line shows the title alone.
  const j = joinStaffLinks(directory.duLieu.items, units.ok ? units.duLieu.items : [], staff.duLieu);
  return { state: "joined", ...j };
}

/*
 * NO `m-0` INSIDE A `space-y-*` PARENT: Tailwind v4 emits `space-y` under `:where()` (zero
 * specificity), so a child's own `m-0` wins and the gap collapses to nothing. Stacks here are
 * `flex flex-col gap-*` instead — a gap does not depend on any child's margin. Preflight is off, so
 * `<p>`/`<h3>` still need their UA margins cleared (`m-0`) — which is exactly why `space-y` cannot be used.
 */
const SECTION = "border-line shadow-card rounded-card m-0 min-w-0 border border-solid bg-white p-5";
const TITLE = "text-navy m-0 text-[14px] font-bold";
const DESCRIPTION = "text-ink-muted m-0 mt-1 max-w-2xl text-[12.5px]";
const FIELD_LABEL = "text-navy m-0 block text-[13px] leading-5 font-semibold";
/**
 * Preflight is off, so `border-solid` alone gives the OTHER three sides the UA `medium` (3px) width:
 * a one-sided hairline is always `border-0` + that side.
 */
const HAIRLINE_BOTTOM = "border-line border-0 border-b border-solid";
const HAIRLINE_TOP = "border-line border-0 border-t border-solid";
const FIELD_HINT = "text-ink-muted m-0 text-[12px]";
const CHECKBOX = "accent-brand m-0 mt-0.5 size-3.5 shrink-0";
const daySelectCls = cn(selectCls, "w-full min-w-0 pr-8");
const hourSelectCls = cn(selectCls, "min-w-0 pr-8");
const botInputCls = cn(controlClass, "bg-white");

/**
 * The prototype's `Field` (label, control, hint), 6px apart — `flex flex-col gap-1.5`, see the note on
 * `SECTION`. The label row is ALWAYS a fixed 20px row, marker or not, so the controls of one grid row
 * line up even when only one label carries a "?" (18px).
 */
function ZaloField({
  label,
  htmlFor,
  hint,
  marker,
  children,
}: {
  label: string;
  htmlFor?: string;
  hint?: string;
  marker?: ReactNode;
  children: ReactNode;
}) {
  const labelEl = htmlFor ? (
    <label htmlFor={htmlFor} className={FIELD_LABEL}>
      {label}
    </label>
  ) : (
    <p className={FIELD_LABEL}>{label}</p>
  );
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <div className="flex h-5 items-center gap-1.5" data-field-label-row="">
        {labelEl}
        {marker}
      </div>
      {children}
      {hint && <p className={FIELD_HINT}>{hint}</p>}
    </div>
  );
}

function DaySelect({
  id,
  name,
  min,
  value,
  zeroLabel,
  disabled,
  onChange,
}: {
  id: string;
  name: string;
  min: number;
  value: number | null;
  zeroLabel?: string;
  disabled: boolean;
  onChange: (days: number | null) => void;
}) {
  return (
    <select
      id={id}
      name={name}
      className={daySelectCls}
      value={value === null ? "" : String(value)}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value === "" ? null : Number(e.target.value))}
    >
      {value === null && <option value="">Chưa đặt</option>}
      {dayOptions(min, value).map((d) => (
        <option key={d} value={d}>
          {d === 0 && zeroLabel ? zeroLabel : `${d} ngày`}
        </option>
      ))}
    </select>
  );
}

function HourSelect({
  name,
  label,
  value,
  disabled,
  onChange,
}: {
  name: string;
  label: string;
  value: string;
  disabled: boolean;
  onChange: (v: string) => void;
}) {
  return (
    <select
      name={name}
      aria-label={label}
      className={hourSelectCls}
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value)}
    >
      {hourOptions(value).map((h) => (
        <option key={h} value={h}>
          {h}
        </option>
      ))}
    </select>
  );
}

/** Pure rendering, exported so the tests read the markup. */
export function ZaloChannelView({
  draft,
  supported,
  platformReady,
  onChange,
  saving,
  held,
  dueSoonError = null,
  people,
  botSection,
  groupSection = null,
  launchQrSection = null,
}: {
  draft: ZaloChannelDraft;
  /** The server's `supported_events`: the only kinds a box may be ticked for. */
  supported: readonly string[];
  /** Spec 11 §0: false draws the warning; true or unknown (null) draws nothing. */
  platformReady: boolean | null;
  onChange: (d: ZaloChannelDraft) => void;
  saving: boolean;
  held: string | null;
  /** comms' `due_soon_days_out_of_range` sentence, drawn under the "nhắc trước" select. */
  dueSoonError?: string | null;
  people: PeopleState;
  /** §3, stateful (`CommuneBotSection`) — a slot, so this view stays pure for the tests. */
  botSection: ReactNode;
  /** "Nhóm Zalo" (ADR 0092), stateful (`ZaloGroupsSection`) — a slot for the same reason. */
  groupSection?: ReactNode;
  /** "Mã QR mở Mini App của xã" (ADR 0070 §13/10), stateful (`CommuneLaunchQrSection`) — a slot for the same reason. */
  launchQrSection?: ReactNode;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-5">
      {/* §0 — ZaloChannelPanel.tsx:55-60. Preflight is off: `border` needs its `border-solid`. */}
      {platformReady === false && (
        <p
          role="status"
          className="border-tangerine/30 bg-tangerine/8 text-navy m-0 rounded-[10px] border border-solid px-4 py-3 text-[12.5px]"
        >
          {PLATFORM_NOT_READY}
        </p>
      )}

      {/* §1 — ZaloChannelPanel.tsx:62-109 */}
      <section className={SECTION} aria-labelledby="zalo-channel-title">
        <div className="flex flex-wrap items-start gap-4">
          <div className="min-w-0 flex-1">
            <h3 id="zalo-channel-title" className={TITLE}>
              Nhắc việc qua Zalo
            </h3>
            <p className="text-ink-muted m-0 mt-1 max-w-xl text-[12.5px]">{CHANNEL_DESCRIPTION}</p>
          </div>
          <Switch
            checked={draft.isEnabled}
            disabled={saving}
            onCheckedChange={(next) => onChange({ ...draft, isEnabled: next })}
            aria-label="Bật kênh Zalo"
          />
        </div>

        <div className="mt-5 grid gap-4 sm:grid-cols-2">
          <ZaloField label="Giờ yên tĩnh" hint={QUIET_HINT}>
            <div className="flex flex-wrap items-center gap-2 text-[12.5px]" role="group" aria-label="Giờ yên tĩnh">
              <HourSelect
                name="quiet_start"
                label="Giờ yên tĩnh bắt đầu"
                value={draft.quietStart}
                disabled={saving}
                onChange={(v) => onChange({ ...draft, quietStart: v })}
              />
              <span className="text-ink-muted">đến</span>
              <HourSelect
                name="quiet_end"
                label="Giờ yên tĩnh kết thúc"
                value={draft.quietEnd}
                disabled={saving}
                onChange={(v) => onChange({ ...draft, quietEnd: v })}
              />
            </div>
          </ZaloField>

          {/* "{linked}/{total}" only when the directory was read; without it a total would be a guess. */}
          <ZaloField label="Đã ghép nối">
            <p className="text-navy m-0 text-[15px] font-bold">
              {people.state === "joined"
                ? `${people.linked}/${people.total} cán bộ`
                : people.state === "linkedOnly"
                  ? `${people.links.length} cán bộ`
                  : people.state === "loading"
                    ? "…"
                    : "—"}
            </p>
          </ZaloField>
        </div>
      </section>

      {/* §2 — ZaloChannelPanel.tsx:111-204 */}
      <section className={SECTION} aria-labelledby="zalo-when-title">
        <h3 id="zalo-when-title" className={TITLE}>
          Nhắc khi nào
        </h3>
        <p className={DESCRIPTION}>{WHEN_DESCRIPTION}</p>

        <div className="mt-4 grid gap-4 sm:grid-cols-3">
          <ZaloField label="Sắp đến hạn: nhắc trước" htmlFor="zalo-due-soon-days" hint={DUE_SOON_HINT}>
            {/* Unlike the two overdue selects, the empty option stays offered once a number is picked:
                "no narrowing" is a choice a commune must be able to go back to, not an unset state. */}
            <select
              id="zalo-due-soon-days"
              name="due_soon_days"
              className={daySelectCls}
              value={draft.dueSoonDays === null ? "" : String(draft.dueSoonDays)}
              disabled={saving}
              aria-invalid={dueSoonError !== null}
              aria-describedby={dueSoonError !== null ? "zalo-due-soon-error" : undefined}
              onChange={(e) => onChange({ ...draft, dueSoonDays: e.target.value === "" ? null : Number(e.target.value) })}
            >
              <option value="">{DUE_SOON_NO_LEAD}</option>
              {dayOptions(DUE_SOON_MIN, draft.dueSoonDays).map((d) => (
                <option key={d} value={d}>{`${d} ngày`}</option>
              ))}
            </select>
            {dueSoonError !== null && (
              <p id="zalo-due-soon-error" className="text-danger m-0 text-[12px] font-medium" role="alert">
                {dueSoonError}
              </p>
            )}
          </ZaloField>
          <ZaloField label="Quá hạn: bắt đầu nhắc sau" htmlFor="zalo-overdue-start">
            <DaySelect
              id="zalo-overdue-start"
              name="overdue_start_after_days"
              min={0}
              zeroLabel="Ngay hôm quá hạn"
              value={draft.lateStartDays}
              disabled={saving}
              onChange={(v) => onChange({ ...draft, lateStartDays: v })}
            />
          </ZaloField>
          <ZaloField label="Quá hạn: nhắc lại mỗi" htmlFor="zalo-overdue-repeat">
            <DaySelect
              id="zalo-overdue-repeat"
              name="overdue_repeat_every_days"
              min={1}
              value={draft.lateRepeatDays}
              disabled={saving}
              onChange={(v) => onChange({ ...draft, lateRepeatDays: v })}
            />
          </ZaloField>
        </div>
        {held !== null && (
          <p role="status" className="text-ink-muted m-0 mt-2 text-[12px]">
            {held}
          </p>
        )}

        <p className="text-navy m-0 mt-5 mb-1 text-[13px] font-semibold">Việc gì thì nhắn qua Zalo</p>
        <p className="text-ink-muted m-0 mb-3 text-[12.5px]">{EVENTS_HINT}</p>
        <div className="grid gap-5 sm:grid-cols-2">
          {ZALO_EVENT_GROUPS.map((group) => (
            <div key={group.title} role="group" aria-label={group.title}>
              <p className="text-ink-muted m-0 mb-1.5 text-[11px] font-semibold tracking-wide uppercase">{group.title}</p>
              <div className="flex flex-col gap-1.5">
                {group.events.map((ev) => {
                  const text = (
                    <span>
                      <span className="text-navy block text-[12.5px] font-semibold">{ev.label}</span>
                      {ev.hint && <span className="text-ink-muted block text-[11.5px]">{ev.hint}</span>}
                    </span>
                  );
                  const kind = ev.kind;
                  const kinds = rowKinds(ev);
                  // A kind this server does not offer: the server decides what may be ticked.
                  if (!kinds.every((k) => supported.includes(k))) {
                    return (
                      <label
                        key={ev.code}
                        className="flex cursor-not-allowed items-start gap-2.5 opacity-60"
                        data-not-offered=""
                      >
                        <input type="checkbox" className={CHECKBOX} data-event={ev.code} checked={false} disabled readOnly />
                        <span>
                          <span className="text-navy block text-[12.5px] font-semibold">{ev.label}</span>
                          {ev.hint && <span className="text-ink-muted block text-[11.5px]">{ev.hint}</span>}
                          <span className="text-ink-muted block text-[11.5px]">{EVENT_NOT_OFFERED}</span>
                        </span>
                      </label>
                    );
                  }
                  return (
                    <label key={ev.code} className="flex cursor-pointer items-start gap-2.5">
                      <input
                        type="checkbox"
                        className={CHECKBOX}
                        data-event={ev.code}
                        checked={draft.kinds.includes(kind)}
                        disabled={saving}
                        onChange={(e) => onChange(toggleKind(draft, kinds, e.target.checked, supported))}
                      />
                      {text}
                    </label>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </section>

      {botSection}

      {groupSection}

      <LinkedStaffSection people={people} />

      {launchQrSection}

      {saving && (
        <p role="status" className="text-ink-muted m-0 flex items-center gap-1.5 text-[12px]">
          <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
          Đang lưu…
        </p>
      )}
    </div>
  );
}

type BotCurrent = CommuneZaloBotCurrent;

type BotBusy = "save" | "check" | "webhook" | "retire" | null;

/** A save waiting for "N cán bộ … phải ghép nối lại" to be confirmed (ADR 0079 Q1 #4). */
type PendingSwitch = { kind: Exclude<BotSaveKind, "edit">; draft: BotDraft };

function draftFromBot(c: BotCurrent): BotDraft {
  return { token: "", name: c.bot?.bot_name ?? "", chatUrl: c.bot?.chat_url ?? "" };
}

const DANGER_OUTLINE = "border-danger/30 text-danger hover:not-disabled:bg-danger/5 bg-white";

/**
 * §3 — ZaloChannelPanel.tsx:337-510, against `zalo-bots/current` (ADR 0079 Q1 #1–#5). Every action is
 * an explicit button, never the autosave: a bot switch ends every live link.
 *
 * SECRETS:
 *   - the TOKEN input is write-only: never filled from the server (nothing returns it), cleared after a
 *     save that went through, sent only when typed (`saveCommuneZaloBot`);
 *   - the WEBHOOK SECRET lives in `secret` only while its one-time box is open, and is dropped by
 *     "Tôi đã chép, đóng", by any other write, and by unmount. No `console`, no storage, no URL, no
 *     `aria-label` / `title` carries either (rule 3 forbidden #4, rule 8).
 *
 * Spec 11 §3's "Secret Token của webhook" field is THAT box (ADR 0079 Q1 #3: shown once, never read
 * back), not a standing input.
 */
function CommuneBotSection({ onSwitched }: { onSwitched: () => void }) {
  const [current, setCurrent] = useState<KetQua<BotCurrent> | null>(null);
  const [draft, setDraft] = useState<BotDraft>({ token: "", name: "", chatUrl: "" });
  const [busy, setBusy] = useState<BotBusy>(null);
  const [pendingSwitch, setPendingSwitch] = useState<PendingSwitch | null>(null);
  const [retireOpen, setRetireOpen] = useState(false);
  const [retireReason, setRetireReason] = useState("");
  const [retireError, setRetireError] = useState<string | null>(null);
  const [secret, setSecret] = useState<string | null>(null);
  const [revokeNotice, setRevokeNotice] = useState<string | null>(null);

  const load = useCallback(async (resetDraft: boolean) => {
    const r = await getCommuneZaloBot();
    setCurrent(r);
    if (r.ok && resetDraft) setDraft(draftFromBot(r.duLieu));
  }, []);

  useEffect(() => {
    let gone = false;
    getCommuneZaloBot().then((r) => {
      if (gone) return;
      setCurrent(r);
      if (r.ok) setDraft(draftFromBot(r.duLieu));
    });
    return () => {
      gone = true;
    };
  }, []);

  if (current === null) {
    return (
      <section className={SECTION} aria-labelledby="zalo-bot-title">
        <h3 id="zalo-bot-title" className={TITLE}>
          Con bot của xã
        </h3>
        <p role="status" className="an-thi-giac">
          Đang tải con bot của xã…
        </p>
        <Skeleton className="mt-4 h-24 w-full" />
      </section>
    );
  }
  if (!current.ok) {
    return (
      <section className={SECTION} aria-labelledby="zalo-bot-title">
        <h3 id="zalo-bot-title" className={TITLE}>
          Con bot của xã
        </h3>
        <p className="text-danger m-0 mt-2 text-[12.5px] font-medium" role="alert">
          {current.thongBao}
        </p>
      </section>
    );
  }
  const bot = current.duLieu;

  async function save(d: BotDraft) {
    const built = buildBotChange(d, bot.has_own_bot);
    if (!built.ok) {
      toast.error(built.text);
      return;
    }
    setPendingSwitch(null);
    setBusy("save");
    setSecret(null);
    const r = await saveCommuneZaloBot(built.body);
    setBusy(null);
    if (!r.ok) {
      // Prototype `ZaloChannelPanel.tsx:48-49`: "Lưu con bot" goes through the same save as the rest of
      // the tab, so the same two toasts (ADR 0079 Q19). A refusal the server explained keeps its words.
      toast.error(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    toast.success(`${SAVED_TOAST}${endedLinksText(r.duLieu.ended_link_count, r.duLieu.ended_group_count)}`);
    setRevokeNotice(r.duLieu.revoke_notice ?? null);
    await load(true);
    onSwitched();
  }

  function requestSave() {
    const built = buildBotChange(draft, bot.has_own_bot);
    if (!built.ok) {
      toast.error(built.text);
      return;
    }
    if (built.kind === "edit") {
      void save(draft);
      return;
    }
    setPendingSwitch({ kind: built.kind, draft });
  }

  async function check() {
    setBusy("check");
    const r = await checkCommuneZaloBot();
    setBusy(null);
    if (!r.ok) {
      // Prototype `ZaloChannelPanel.tsx:456` for a call that failed unexplained (ADR 0079 Q19).
      toast.error(r.thongBao === LOI_KHONG_RO ? CHECK_FAILED_TOAST : r.thongBao);
      return;
    }
    const text = checkResultText(r.duLieu.result, r.duLieu.account_name);
    if (isOutcomeOk(r.duLieu.result)) toast.success(text);
    else toast.error(text);
    await load(false);
  }

  async function registerWebhook() {
    setBusy("webhook");
    setSecret(null);
    const r = await registerCommuneZaloWebhook();
    setBusy(null);
    if (!r.ok) {
      toast.error(webhookRequestFailedText(r.thongBao));
      return;
    }
    const text = webhookResultText(r.duLieu.result);
    if (isOutcomeOk(r.duLieu.result)) toast.success(text);
    else toast.error(text);
    // Present only on the call that generated it (comms: not on a refusal, not when a pending one is reused).
    const s = r.duLieu.secret;
    if (typeof s === "string" && s !== "") setSecret(s);
    await load(false);
  }

  async function retire() {
    const checked = checkRetireReason(retireReason);
    if (!checked.ok) {
      setRetireError(checked.text);
      return;
    }
    setRetireError(null);
    setBusy("retire");
    setSecret(null);
    const r = await retireCommuneZaloBot(checked.reason);
    setBusy(null);
    if (!r.ok) {
      // Kept in the box: the reason typed stays, the server's sentence is next to it. Unexplained
      // failure: the prototype's save toast text (`ZaloChannelPanel.tsx:49`, ADR 0079 Q19).
      setRetireError(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    setRetireOpen(false);
    setRetireReason("");
    // Prototype `ZaloChannelPanel.tsx:491` → `:48`: "Quay về bot chung" is a save, toasted "Đã lưu."
    toast.success(
      r.duLieu.retired
        ? `${SAVED_TOAST}${endedLinksText(r.duLieu.ended_link_count, r.duLieu.ended_group_count)}`
        : "Xã đã không còn bot riêng nào đang dùng.",
    );
    setRevokeNotice(r.duLieu.revoke_notice ?? null);
    await load(true);
    onSwitched();
  }

  const idle = busy === null;
  const lastCheck = bot.bot?.last_check ?? null;
  return (
    <section className={SECTION} aria-labelledby="zalo-bot-title">
      <div className="flex flex-wrap items-start gap-3">
        <div className="min-w-0 flex-1">
          <h3 id="zalo-bot-title" className={TITLE}>
            Con bot của xã
          </h3>
          <p className={DESCRIPTION}>{botDescription(bot.has_own_bot)}</p>
        </div>
        <span
          className={cn(
            "rounded-full px-3 py-1 text-[12px] font-semibold",
            // Token trap (config-ui.tsx): the spec's `bg-surface` is the page grey = `bg-background` here.
            bot.has_own_bot ? "bg-violet/10 text-violet" : "bg-background text-ink-muted",
          )}
          data-bot-pill=""
        >
          {botPill(bot)}
        </span>
      </div>

      <form
        aria-label="Con bot của xã"
        className="m-0"
        onSubmit={(e) => {
          e.preventDefault();
          if (idle) requestSave();
        }}
      >
        <fieldset disabled={!idle} className="m-0 min-w-0 border-0 p-0">
          <div className="mt-4 grid gap-4 sm:grid-cols-2">
            <ZaloField label="Mã bot của xã" htmlFor="zalo-bot-token" hint={botTokenHint(bot.has_own_bot)}>
              {/* Write-only: never filled from the server; no `name`, so nothing submits it natively. */}
              <input
                id="zalo-bot-token"
                type="password"
                autoComplete="off"
                spellCheck={false}
                placeholder={bot.has_own_bot ? "••••••••" : "123456789:abc-xyz"}
                className={botInputCls}
                value={draft.token}
                onChange={(e) => setDraft({ ...draft, token: e.target.value })}
              />
            </ZaloField>
            <ZaloField label="Tên bot" htmlFor="zalo-bot-name" hint="Tên hiển thị trong Zalo, bắt đầu bằng “Bot”.">
              <input
                id="zalo-bot-name"
                type="text"
                className={botInputCls}
                value={draft.name}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              />
            </ZaloField>
            <ZaloField
              label="Đường mở khung chat"
              htmlFor="zalo-bot-chat-url"
              hint="Link bot dạng https://zalo.me/<số>. Mã QR cán bộ quét được dựng từ đây."
            >
              <input
                id="zalo-bot-chat-url"
                type="url"
                inputMode="url"
                placeholder="https://zalo.me/..."
                className={botInputCls}
                value={draft.chatUrl}
                onChange={(e) => setDraft({ ...draft, chatUrl: e.target.value })}
              />
            </ZaloField>
          </div>

          <div className="mt-4 flex flex-wrap items-center gap-2">
            <Button type="submit" variant="primary" aria-busy={busy === "save"}>
              Lưu con bot
            </Button>
            {/* Own bot only (owner 08/10/2026): comms checks the commune's OWN bot, 409 zalo_own_bot_missing otherwise. */}
            {bot.has_own_bot && (
              <Button
                type="button"
                variant="outline"
                className="bg-white"
                icon={
                  busy === "check" ? (
                    <Loader2 aria-hidden="true" focusable="false" className="animate-spin" />
                  ) : (
                    <RefreshCw aria-hidden="true" focusable="false" />
                  )
                }
                aria-busy={busy === "check"}
                onClick={() => void check()}
              >
                Kiểm tra kết nối
              </Button>
            )}
            {bot.has_own_bot && (
              <Button
                type="button"
                variant="outline"
                className="bg-white"
                aria-busy={busy === "webhook"}
                onClick={() => void registerWebhook()}
              >
                Đăng ký webhook
              </Button>
            )}
            {bot.has_own_bot && (
              <Button
                type="button"
                variant="outline"
                className={DANGER_OUTLINE}
                icon={<Undo2 aria-hidden="true" focusable="false" />}
                onClick={() => {
                  setRetireError(null);
                  setRetireOpen(true);
                }}
              >
                Quay về bot chung
              </Button>
            )}
          </div>
        </fieldset>
      </form>

      {bot.bot !== null && (
        <div className="mt-3 flex flex-col gap-1">
          {lastCheck !== null && (
            <p className="text-ink-muted m-0 text-[12px]" data-last-check="">
              {lastCheckText(formatVietnamDateTime(lastCheck.at), lastCheck.result)}
            </p>
          )}
          <p className="text-ink-muted m-0 text-[12px]" data-webhook-state="">
            {bot.bot.webhook_set_at !== null
              ? `Webhook đã đăng ký lúc ${formatVietnamDateTime(bot.bot.webhook_set_at)}.`
              : bot.bot.webhook_pending
                ? "Lần đăng ký webhook gần nhất chưa được Zalo xác nhận — bấm “Đăng ký webhook” lần nữa."
                : "Chưa đăng ký webhook."}
          </p>
        </div>
      )}

      {secret !== null && <WebhookSecretOnce secret={secret} onClose={() => setSecret(null)} />}

      {revokeNotice !== null && (
        <p
          role="status"
          className="border-tangerine/30 bg-tangerine/8 text-navy m-0 mt-3 rounded-[10px] border border-solid px-4 py-3 text-[12.5px]"
          data-revoke-notice=""
        >
          {revokeNotice}
        </p>
      )}

      {pendingSwitch !== null && (
        <ConfigDialog
          title={pendingSwitch.kind === "adopt" ? "Chuyển sang bot riêng của xã?" : "Thay mã bot của xã?"}
          hideHeader
          onDismiss={() => {
            if (idle) setPendingSwitch(null);
          }}
        >
          <ConfirmDialog
            icon={TriangleAlert}
            tone="danger"
            title={pendingSwitch.kind === "adopt" ? "Chuyển sang bot riêng của xã?" : "Thay mã bot của xã?"}
            className="m-0 p-0 shadow-none"
            actions={
              <>
                <Button
                  type="button"
                  variant="primary"
                  disabled={!idle}
                  aria-busy={busy === "save"}
                  onClick={() => void save(pendingSwitch.draft)}
                >
                  Lưu con bot
                </Button>
                <Button type="button" variant="outline" disabled={!idle} onClick={() => setPendingSwitch(null)}>
                  {NUT_HUY}
                </Button>
              </>
            }
          >
            <p className="m-0" data-relink="">
              {pendingSwitch.kind === "adopt"
                ? `Từ lúc lưu, mọi tin nhắc việc của xã đi bằng bot riêng. ${relinkSentence(bot.live_link_count, bot.live_group_count)}`
                : `Nếu mã mới là của một con bot khác, ${relinkSentence(bot.live_link_count, bot.live_group_count)}`}
            </p>
          </ConfirmDialog>
        </ConfigDialog>
      )}

      {retireOpen && (
        <ConfigDialog
          title="Quay về bot chung?"
          hideHeader
          onDismiss={() => {
            if (idle) setRetireOpen(false);
          }}
        >
          <ConfirmDialog
            as="form"
            aria-label="Quay về bot chung"
            icon={Undo2}
            tone="danger"
            title="Quay về bot chung?"
            className="m-0 p-0 shadow-none"
            onSubmit={(e) => {
              e.preventDefault();
              if (idle) void retire();
            }}
            actions={
              <>
                <Button type="submit" variant="primary" disabled={!idle} aria-busy={busy === "retire"}>
                  Quay về bot chung
                </Button>
                <Button type="button" variant="outline" disabled={!idle} onClick={() => setRetireOpen(false)}>
                  {NUT_HUY}
                </Button>
              </>
            }
          >
            <p className="m-0" data-relink="">
              {`Mọi tin nhắc việc của xã sẽ đi bằng bot chung của nền tảng. ${relinkSentence(bot.live_link_count, bot.live_group_count)}`}
            </p>
            <div className="mt-3 flex flex-col gap-1.5">
              <label htmlFor="zalo-bot-retire-reason" className={FIELD_LABEL}>
                Lý do quay về bot chung
              </label>
              {/* One line: comms refuses control characters, a newline included. */}
              <input
                id="zalo-bot-retire-reason"
                type="text"
                className={botInputCls}
                maxLength={RETIRE_REASON_MAX}
                value={retireReason}
                disabled={!idle}
                aria-invalid={retireError !== null}
                onChange={(e) => setRetireReason(e.target.value)}
              />
              {retireError !== null && (
                <p className="text-danger m-0 text-[12px] font-medium" role="alert">
                  {retireError}
                </p>
              )}
            </div>
          </ConfirmDialog>
        </ConfigDialog>
      )}
    </section>
  );
}

/**
 * The webhook secret, ONCE (ADR 0079 Q1 #3). A read-only box to copy from, the sentence that it will
 * not be shown again, and one explicit close. It does not close by itself.
 */
function WebhookSecretOnce({ secret, onClose }: { secret: string; onClose: () => void }) {
  return (
    <div
      className="border-line bg-background m-0 mt-4 flex flex-col gap-2 rounded-[10px] border border-solid p-3"
      data-webhook-secret=""
    >
      <label htmlFor="zalo-bot-secret" className={FIELD_LABEL}>
        Secret Token của webhook
      </label>
      <p className="text-navy m-0 text-[12.5px]" role="status">
        {WEBHOOK_SECRET_ONCE}
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <input
          id="zalo-bot-secret"
          type="text"
          readOnly
          autoComplete="off"
          spellCheck={false}
          className={cn(botInputCls, "min-w-0 flex-1 font-mono")}
          value={secret}
          onFocus={(e) => e.currentTarget.select()}
        />
        <Button
          type="button"
          variant="outline"
          className="bg-white"
          icon={<Copy aria-hidden="true" focusable="false" />}
          onClick={() => {
            const clip = typeof navigator === "undefined" ? undefined : navigator.clipboard;
            if (clip === undefined) {
              toast.error("Chưa chép được. Hãy chọn chữ trong ô rồi chép tay.");
              return;
            }
            clip.writeText(secret).then(
              () => toast.success("Đã chép."),
              () => toast.error("Chưa chép được. Hãy chọn chữ trong ô rồi chép tay."),
            );
          }}
        >
          Chép
        </Button>
      </div>
      <p className={FIELD_HINT}>Chép sang ứng dụng Zalo nếu xã tự đăng ký webhook bằng tay.</p>
      <div className="flex justify-end">
        <Button
          type="button"
          variant="primary"
          icon={<Check aria-hidden="true" focusable="false" />}
          onClick={onClose}
        >
          Tôi đã chép, đóng
        </Button>
      </div>
    </div>
  );
}

/** §4 — ZaloChannelPanel.tsx:208-262. */
function LinkedStaffSection({ people }: { people: PeopleState }) {
  const [onlyMissing, setOnlyMissing] = useState(false);
  // The filter only means something when the unlinked are known (the directory was read).
  const canFilter = people.state === "joined";
  const rows =
    people.state === "joined"
      ? people.rows.filter((r) => !onlyMissing || !r.linked)
      : people.state === "linkedOnly"
        ? people.links.map((s) => ({
            code: s.staff_code,
            name: s.staff_name,
            detail: `${s.staff_code} · Ghép nối lúc ${formatVietnamDateTime(s.linked_at)}`,
            linked: true,
          }))
        : [];
  return (
    <section
      className="border-line shadow-card rounded-card m-0 min-w-0 border border-solid bg-white"
      aria-labelledby="zalo-linked-title"
    >
      <div className={cn(HAIRLINE_BOTTOM, "flex flex-wrap items-center gap-3 px-5 py-3")}>
        <h3 id="zalo-linked-title" className={TITLE}>
          Ai đã ghép nối
        </h3>
        <label
          className={cn(
            "text-ink ml-auto flex items-center gap-2 text-[12.5px]",
            canFilter ? "cursor-pointer" : "cursor-not-allowed opacity-60",
          )}
        >
          <input
            type="checkbox"
            name="only_missing"
            className="accent-brand m-0 size-3.5"
            checked={canFilter && onlyMissing}
            disabled={!canFilter}
            onChange={(e) => setOnlyMissing(e.target.checked)}
          />
          Chỉ người chưa ghép nối
        </label>
      </div>
      {people.state === "loading" ? (
        <div className="flex flex-col gap-2 p-4">
          <p role="status" className="an-thi-giac">
            Đang tải danh sách cán bộ…
          </p>
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-9 w-full" />
        </div>
      ) : people.state === "failed" ? (
        <p className="text-danger m-0 px-5 py-3 text-[12.5px] font-medium" role="alert">
          {people.reason}
        </p>
      ) : (
        <>
          {people.state === "linkedOnly" && (
            // Fail closed: without the directory nobody can be called "not linked", so the list says
            // what it is instead of passing for the whole commune.
            <p className={cn(HAIRLINE_BOTTOM, "text-danger m-0 px-5 py-2.5 text-[12px]")} role="alert">
              Chưa đọc được danh bạ cán bộ ({people.reason}), nên dưới đây chỉ có người đã ghép nối.
            </p>
          )}
          {/* Filter on and nobody left: the prototype draws the empty table, no sentence (ZaloChannelPanel.tsx:227-255). */}
          {rows.length === 0 && !(people.state === "joined" && onlyMissing) ? (
            <p className="text-ink-muted m-0 px-5 py-3 text-[12.5px]">{NO_LINKED_STAFF}</p>
          ) : (
            <div className="max-h-96 overflow-y-auto">
              <table className="w-full border-collapse text-[12.5px]">
                <caption className="an-thi-giac">Cán bộ của xã và trạng thái ghép nối Zalo</caption>
                <tbody>
                  {rows.map((r) => (
                    <tr key={r.code} className={cn(HAIRLINE_BOTTOM, "last:border-b-0")} data-staff-code={r.code}>
                      <td className="px-5 py-2.5">
                        <p className="text-navy m-0 font-semibold">{r.name}</p>
                        {r.detail !== "" && <p className="text-ink-muted m-0 text-[11px]">{r.detail}</p>}
                      </td>
                      <td className="px-5 py-2.5 text-right">
                        {r.linked ? (
                          <span className="text-leaf inline-flex items-center gap-1 font-semibold">
                            <Check aria-hidden="true" focusable="false" className="size-3.5" />
                            Đã ghép nối
                          </span>
                        ) : (
                          <span className="text-ink-muted inline-flex items-center gap-1">
                            <X aria-hidden="true" focusable="false" className="size-3.5" />
                            Chưa
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
      {/* Spec 11 §4 "trong ViGov" → "trong hệ thống" (ADR 0068 §13). "Hồ sơ cá nhân" is the real
          menu item (`components/user-menu.tsx`, → /ca-nhan). */}
      <p className={cn(HAIRLINE_TOP, "text-ink-muted m-0 px-5 py-3 text-[12px]")}>
        Cán bộ tự ghép nối trong hệ thống: bấm tên mình ở góc trên, chọn “Hồ sơ cá nhân”. Không ai bật hộ được — Zalo
        chỉ cho bot nhắn cho người đã nhắn cho nó trước.
      </p>
    </section>
  );
}

// ---- "Nhóm Zalo" (ADR 0092) -----------------------------------------------------------------------

type GroupsState = KetQua<comms_zaloGroupList>;

type GroupDialog =
  | { mode: "create" }
  | { mode: "edit"; group: comms_zaloGroupOut }
  | { mode: "unlink"; group: comms_zaloGroupOut }
  | null;

/** One option of the scope select: "" = Toàn xã, else a unit id, indented by depth. */
type ScopeOption = { value: string; label: string; disabled?: boolean };

function scopeOptions(units: Units | null): ScopeOption[] {
  const tree = units !== null && units.ok ? traiPhang(dungCay(units.duLieu.items)) : [];
  return [
    { value: "", label: COMMUNE_WIDE },
    ...tree.map((row) => ({ value: row.bp.id, label: `${"— ".repeat(row.cap)}${row.bp.name}` })),
  ];
}

/**
 * "Nhóm Zalo" — ADR 0092 #1–#3, #8, against `zalo-groups` and `zalo-group-pairing-codes`. Its own reads
 * and writes, like `CommuneBotSection`; never the settings autosave. Same key as the tab (`admin.zalo`),
 * so it is only mounted inside the gate; comms checks it again.
 *
 * THE CODE IS A CREDENTIAL: it lives in the create dialog's state only, from the 201 until the dialog
 * closes — no storage, no URL, no `title`/`aria-label`, no log (rule 3 forbidden #4, rule 8). Closing the
 * dialog drops it and reads the list again (the group may already be linked by then).
 *
 * No chat id anywhere: comms returns none, and nothing here has a field for one.
 */
function ZaloGroupsSection({
  channelEnabled,
  enabledKinds,
  platformReady,
  units,
}: {
  channelEnabled: boolean;
  /** The commune's SAVED kinds (§2): a group may tick only these. */
  enabledKinds: readonly string[];
  platformReady: boolean | null;
  units: Units | null;
}) {
  const [groups, setGroups] = useState<GroupsState | null>(null);
  const [dialog, setDialog] = useState<GroupDialog>(null);

  const load = useCallback(async () => {
    setGroups(await listZaloGroups());
  }, []);

  useEffect(() => {
    let gone = false;
    listZaloGroups().then((r) => {
      if (!gone) setGroups(r);
    });
    return () => {
      gone = true;
    };
  }, []);

  const heading = (
    <div className="min-w-0 flex-1">
      <h3 id="zalo-groups-title" className={TITLE}>
        Nhóm Zalo
      </h3>
      <p className={DESCRIPTION}>{GROUPS_DESCRIPTION}</p>
    </div>
  );

  if (groups === null) {
    return (
      <section className={SECTION} aria-labelledby="zalo-groups-title">
        {heading}
        <p role="status" className="an-thi-giac">
          Đang tải các nhóm Zalo…
        </p>
        <Skeleton className="mt-4 h-16 w-full" />
      </section>
    );
  }
  if (!groups.ok) {
    return (
      <section className={SECTION} aria-labelledby="zalo-groups-title">
        {heading}
        <p className="text-danger m-0 mt-2 text-[12.5px] font-medium" role="alert">
          {groups.thongBao}
        </p>
      </section>
    );
  }
  const list = groups.duLieu;
  const order = list.supported_kinds;
  const offerable = offerableKinds(order, channelEnabled, enabledKinds);
  const sections = groupKindSections(offerable);
  // The same causes comms answers 409 / 422 for, said before the click. Unknown bot state (null) is not a no.
  const createBlocked = !channelEnabled
    ? CHANNEL_OFF_REASON
    : platformReady === false
      ? NO_BOT_REASON
      : sections.length === 0
        ? NO_GROUP_KIND_REASON
        : null;
  const options = scopeOptions(units);
  const unitsNote =
    units !== null && !units.ok
      ? `Chưa tải được danh sách bộ phận (${units.thongBao}), nên lúc này chỉ chọn được ${COMMUNE_WIDE}.`
      : null;

  function close(reload: boolean) {
    setDialog(null);
    if (reload) void load();
  }

  return (
    <section className={SECTION} aria-labelledby="zalo-groups-title">
      <div className="flex flex-wrap items-start gap-3">
        {heading}
        <Button
          type="button"
          variant="primary"
          icon={<Plus aria-hidden="true" focusable="false" />}
          disabled={createBlocked !== null}
          aria-describedby={createBlocked !== null ? "zalo-groups-blocked" : undefined}
          onClick={() => setDialog({ mode: "create" })}
        >
          Tạo mã gắn nhóm
        </Button>
      </div>
      {createBlocked !== null && (
        <p id="zalo-groups-blocked" className="text-ink-muted m-0 mt-2 text-[12px]" data-groups-blocked="">
          {createBlocked}
        </p>
      )}

      {list.items.length === 0 ? (
        <p className="text-ink-muted m-0 mt-4 text-[12.5px]" data-groups-empty="">
          {GROUPS_HOW_IT_WORKS}
        </p>
      ) : (
        <ul className="m-0 mt-4 flex list-none flex-col p-0">
          {list.items.map((g) => {
            const names = kindNames(g.kinds);
            return (
              <li
                key={g.id}
                className={cn(HAIRLINE_BOTTOM, "flex flex-wrap items-start gap-3 py-3 first:pt-0 last:border-b-0 last:pb-0")}
                data-group-id={g.id}
              >
                <div className="flex min-w-0 flex-1 basis-60 flex-col gap-0.5">
                  <p className="text-navy m-0 text-[13px] font-semibold" data-group-scope="">
                    {scopeText(g)}
                  </p>
                  {g.org_unit_removed && (
                    <p className="text-danger m-0 flex items-center gap-1 text-[12px] font-medium" data-unit-removed="">
                      <TriangleAlert aria-hidden="true" focusable="false" className="size-3.5 shrink-0" />
                      {UNIT_REMOVED_WARNING}
                    </p>
                  )}
                  <p className="text-ink m-0 text-[12px]" data-group-kinds="">
                    {`${names.length} loại tin: ${names.join(", ")}`}
                  </p>
                  <p className="text-ink-muted m-0 text-[11.5px]">{linkedText(formatVietnamDateTime(g.linked_at), g)}</p>
                </div>
                <div className="flex shrink-0 flex-wrap gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    className="bg-white"
                    icon={<Pencil aria-hidden="true" focusable="false" />}
                    onClick={() => setDialog({ mode: "edit", group: g })}
                  >
                    Sửa
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    className={DANGER_OUTLINE}
                    icon={<Unlink aria-hidden="true" focusable="false" />}
                    onClick={() => setDialog({ mode: "unlink", group: g })}
                  >
                    Gỡ
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {dialog?.mode === "create" && (
        <CreateGroupCodeDialog
          sections={sections}
          order={order}
          options={options}
          unitsNote={unitsNote}
          onClose={() => close(true)}
        />
      )}
      {dialog?.mode === "edit" && (
        <EditGroupDialog
          group={dialog.group}
          sections={sections}
          order={order}
          offerable={offerable}
          options={options}
          unitsNote={unitsNote}
          onClose={close}
        />
      )}
      {dialog?.mode === "unlink" && <UnlinkGroupDialog group={dialog.group} onClose={close} />}
    </section>
  );
}

type FieldErrors = Partial<Record<RefusalField, string>>;

function refusalErrors(r: Refusal): FieldErrors {
  return { [r.field]: r.text };
}

function FieldError({ field, text }: { field: RefusalField; text: string | undefined }) {
  if (text === undefined) return null;
  return (
    <p className="text-danger m-0 text-[12px] font-medium" role="alert" data-error={field}>
      {text}
    </p>
  );
}

/** The two fields a group has — scope and kinds — each with its refusal drawn under it. */
function GroupFields({
  idPrefix,
  draft,
  onChange,
  disabled,
  sections,
  order,
  options,
  unitsNote,
  errors,
}: {
  idPrefix: string;
  draft: GroupDraft;
  onChange: (d: GroupDraft) => void;
  disabled: boolean;
  sections: readonly GroupKindSection[];
  order: readonly string[];
  options: readonly ScopeOption[];
  unitsNote: string | null;
  errors: FieldErrors;
}) {
  return (
    <>
      <ZaloField label="Phạm vi" htmlFor={`${idPrefix}-scope`} hint={SCOPE_HINT}>
        <select
          id={`${idPrefix}-scope`}
          name="org_unit_id"
          className={daySelectCls}
          value={draft.scope}
          disabled={disabled}
          aria-invalid={errors.scope !== undefined}
          onChange={(e) => onChange({ ...draft, scope: e.target.value })}
        >
          {options.map((o) => (
            <option key={o.value} value={o.value} disabled={o.disabled}>
              {o.label}
            </option>
          ))}
        </select>
        {unitsNote !== null && <p className={FIELD_HINT}>{unitsNote}</p>}
        <FieldError field="scope" text={errors.scope} />
      </ZaloField>

      <div className="flex min-w-0 flex-col gap-1.5">
        <p className={FIELD_LABEL}>Loại tin</p>
        <p className={FIELD_HINT}>{KINDS_HINT}</p>
        <div className="grid gap-4 sm:grid-cols-2">
          {sections.map((s) => (
            <div key={s.title} role="group" aria-label={s.title}>
              <p className="text-ink-muted m-0 mb-1.5 text-[11px] font-semibold tracking-wide uppercase">{s.title}</p>
              <div className="flex flex-col gap-1.5">
                {s.rows.map((row) => (
                  <label key={row.code} className="flex cursor-pointer items-start gap-2.5">
                    <input
                      type="checkbox"
                      className={CHECKBOX}
                      data-group-kind={row.code}
                      checked={isRowChecked(row, draft.kinds)}
                      disabled={disabled}
                      onChange={(e) =>
                        onChange({
                          ...draft,
                          kinds: toggleGroupKinds(draft.kinds, row, e.target.checked, order),
                          kindsTouched: true,
                        })
                      }
                    />
                    <span className="text-navy text-[12.5px] font-semibold">{row.label}</span>
                  </label>
                ))}
              </div>
            </div>
          ))}
        </div>
        <FieldError field="kinds" text={errors.kinds} />
      </div>
    </>
  );
}

/**
 * "Tạo mã gắn nhóm": scope + kinds → POST → the code, big, with its countdown, a copy button and the
 * instruction. The code exists only while this dialog is open.
 */
function CreateGroupCodeDialog({
  sections,
  order,
  options,
  unitsNote,
  onClose,
}: {
  sections: readonly GroupKindSection[];
  order: readonly string[];
  options: readonly ScopeOption[];
  unitsNote: string | null;
  onClose: () => void;
}) {
  const [draft, setDraft] = useState<GroupDraft>({ scope: "", kinds: [], kindsTouched: false });
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [issued, setIssued] = useState<comms_groupPairingCodeOut | null>(null);

  async function submit() {
    const refused = checkDraft(draft);
    if (refused !== null) {
      setErrors(refusalErrors(refused));
      return;
    }
    setErrors({});
    setBusy(true);
    const r = await createZaloGroupPairingCode({ org_unit_id: draft.scope === "" ? null : draft.scope, kinds: draft.kinds });
    setBusy(false);
    if (!r.ok) {
      setErrors(refusalErrors(refusalFor(r.code, r.thongBao, LOI_KHONG_RO)));
      return;
    }
    setIssued(r.duLieu);
  }

  return (
    <ConfigDialog
      title="Tạo mã gắn nhóm"
      onDismiss={() => {
        if (!busy) onClose();
      }}
    >
      {issued === null ? (
        <form
          aria-label="Tạo mã gắn nhóm"
          className="m-0 flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (!busy) void submit();
          }}
        >
          <GroupFields
            idPrefix="zalo-group-new"
            draft={draft}
            onChange={setDraft}
            disabled={busy}
            sections={sections}
            order={order}
            options={options}
            unitsNote={unitsNote}
            errors={errors}
          />
          <FieldError field="form" text={errors.form} />
          <div className="flex flex-wrap justify-end gap-2">
            <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
              Tạo mã
            </Button>
            <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
              {NUT_HUY}
            </Button>
          </div>
        </form>
      ) : (
        <GroupCodeOnce issued={issued} onClose={onClose} />
      )}
    </ConfigDialog>
  );
}

const COPY_FAILED = "Chưa chép được. Hãy chọn mã rồi chép tay.";

/** The fresh code, ONCE: big, monospaced, a countdown to `expires_at`, "Chép", the instruction. */
function GroupCodeOnce({ issued, onClose }: { issued: comms_groupPairingCodeOut; onClose: () => void }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);
  const left = countdownText(issued.expires_at, now);
  return (
    <div className="flex flex-col gap-3" data-group-code-once="">
      <p className="text-navy m-0 text-[12.5px]">{GROUPS_HOW_IT_WORKS}</p>
      <div className="border-line bg-background flex flex-wrap items-center gap-3 rounded-[10px] border border-solid p-3">
        <output
          className={cn(
            "min-w-0 flex-1 font-mono text-[26px] font-bold tracking-[0.2em] break-all",
            left === null ? "text-ink-muted line-through" : "text-navy",
          )}
          data-group-code=""
        >
          {issued.code}
        </output>
        <Button
          type="button"
          variant="outline"
          className="bg-white"
          disabled={left === null}
          icon={<Copy aria-hidden="true" focusable="false" />}
          onClick={() => {
            const clip = typeof navigator === "undefined" ? undefined : navigator.clipboard;
            if (clip === undefined) {
              toast.error(COPY_FAILED);
              return;
            }
            clip.writeText(issued.code).then(
              () => toast.success("Đã chép."),
              () => toast.error(COPY_FAILED),
            );
          }}
        >
          Chép
        </Button>
      </div>
      <p
        className={cn("m-0 text-[12.5px] font-medium", left === null ? "text-danger" : "text-navy")}
        role="status"
        data-countdown=""
      >
        {left ?? CODE_EXPIRED}
      </p>
      <p className={FIELD_HINT}>{CODE_ONCE}</p>
      <div className="flex justify-end">
        <Button type="button" variant="primary" icon={<Check aria-hidden="true" focusable="false" />} onClick={onClose}>
          Xong
        </Button>
      </div>
    </div>
  );
}

/** "Sửa": the same two fields → PATCH with only what changed. */
function EditGroupDialog({
  group,
  sections,
  order,
  offerable,
  options,
  unitsNote,
  onClose,
}: {
  group: comms_zaloGroupOut;
  sections: readonly GroupKindSection[];
  order: readonly string[];
  offerable: readonly string[];
  options: readonly ScopeOption[];
  unitsNote: string | null;
  onClose: (reload: boolean) => void;
}) {
  const [draft, setDraft] = useState<GroupDraft>(() => draftFromGroup(group, offerable));
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});
  const dropped = droppedKinds(group, offerable);
  // A unit no longer in the tree stays the select's value (nothing moves unasked), shown for what it is
  // and not selectable again.
  const scopeChoices: readonly ScopeOption[] =
    group.org_unit_id !== null && !options.some((o) => o.value === group.org_unit_id)
      ? [
          ...options,
          {
            value: group.org_unit_id,
            label: `${scopeText(group)} (${group.org_unit_removed ? "đã xoá" : "không có trong danh sách"})`,
            disabled: true,
          },
        ]
      : options;

  async function submit() {
    const built = buildPatch(group, draft);
    if (!built.ok) {
      setErrors(refusalErrors(built.refusal));
      return;
    }
    setErrors({});
    setBusy(true);
    const r = await editZaloGroup(group.id, built.patch);
    setBusy(false);
    if (!r.ok) {
      const refusal = refusalFor(r.code, r.thongBao, LOI_KHONG_RO);
      if (r.code === "not_found") {
        toast.error(refusal.text);
        onClose(true);
        return;
      }
      setErrors(refusalErrors(refusal));
      return;
    }
    toast.success(SAVED_TOAST);
    onClose(true);
  }

  return (
    <ConfigDialog
      title="Sửa nhóm Zalo"
      description={scopeText(group)}
      onDismiss={() => {
        if (!busy) onClose(false);
      }}
    >
      <form
        aria-label="Sửa nhóm Zalo"
        className="m-0 flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (!busy) void submit();
        }}
      >
        {group.org_unit_removed && (
          <p className="text-danger m-0 text-[12.5px] font-medium" data-unit-removed="">
            {UNIT_REMOVED_WARNING}.
          </p>
        )}
        <GroupFields
          idPrefix={`zalo-group-${group.id}`}
          draft={draft}
          onChange={setDraft}
          disabled={busy}
          sections={sections}
          order={order}
          options={scopeChoices}
          unitsNote={unitsNote}
          errors={errors}
        />
        {dropped.length > 0 && (
          <p className={FIELD_HINT} data-dropped-kinds="">
            {droppedKindsText(kindNames(dropped))}
          </p>
        )}
        <FieldError field="form" text={errors.form} />
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            Lưu
          </Button>
          <Button type="button" variant="outline" disabled={busy} onClick={() => onClose(false)}>
            {NUT_HUY}
          </Button>
        </div>
      </form>
    </ConfigDialog>
  );
}

/** "Gỡ": the ONLY way a group is unlinked (ADR 0092 #8) — a required reason, then DELETE. */
function UnlinkGroupDialog({ group, onClose }: { group: comms_zaloGroupOut; onClose: (reload: boolean) => void }) {
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});
  const title = `Gỡ nhóm Zalo “${scopeText(group)}”?`;

  async function submit() {
    const checked = checkUnlinkReason(reason);
    if (!checked.ok) {
      setErrors(refusalErrors(checked.refusal));
      return;
    }
    setErrors({});
    setBusy(true);
    const r = await unlinkZaloGroup(group.id, checked.reason);
    setBusy(false);
    if (!r.ok) {
      const refusal = refusalFor(r.code, r.thongBao, LOI_KHONG_RO);
      if (r.code === "not_found") {
        toast.error(refusal.text);
        onClose(true);
        return;
      }
      setErrors(refusalErrors(refusal));
      return;
    }
    toast.success(UNLINKED_TOAST);
    onClose(true);
  }

  return (
    <ConfigDialog
      title={title}
      hideHeader
      onDismiss={() => {
        if (!busy) onClose(false);
      }}
    >
      <ConfirmDialog
        as="form"
        aria-label="Gỡ nhóm Zalo"
        icon={Unlink}
        tone="danger"
        title={title}
        className="m-0 p-0 shadow-none"
        onSubmit={(e) => {
          e.preventDefault();
          if (!busy) void submit();
        }}
        actions={
          <>
            <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
              Gỡ nhóm
            </Button>
            <Button type="button" variant="outline" disabled={busy} onClick={() => onClose(false)}>
              {NUT_HUY}
            </Button>
          </>
        }
      >
        <p className="m-0">{UNLINK_CONSEQUENCE}</p>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="zalo-group-unlink-reason" className={FIELD_LABEL}>
            Lý do gỡ nhóm
          </label>
          {/* One line: comms refuses control characters, a newline included. */}
          <input
            id="zalo-group-unlink-reason"
            type="text"
            className={botInputCls}
            maxLength={UNLINK_REASON_MAX}
            value={reason}
            disabled={busy}
            aria-invalid={errors.reason !== undefined}
            onChange={(e) => setReason(e.target.value)}
          />
          <FieldError field="reason" text={errors.reason} />
          <FieldError field="form" text={errors.form} />
        </div>
      </ConfirmDialog>
    </ConfigDialog>
  );
}
