"use client";

import { Check, Loader2, RefreshCw, X } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { NoAccess } from "@/components/ui/no-access";
import { PendingMarker, type PendingFeatureInfo } from "@/components/ui/pending-feature";
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
import {
  getZaloChannelSettings,
  listZaloLinkedStaff,
  saveZaloChannelSettings,
  type ZaloChannelSettings,
  type ZaloLinkedStaff,
} from "@/lib/api/zalo";

import { selectCls } from "./config-ui";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";
import { zaloChannelTabDecision } from "./quyen-tab";
import {
  buildChange,
  CHANNEL_DESCRIPTION,
  dayOptions,
  draftFromSettings,
  EVENTS_HINT,
  hourOptions,
  joinStaffLinks,
  NO_LINKED_STAFF,
  QUIET_HINT,
  SAVE_FAILED_TOAST,
  SAVED_TOAST,
  toggleKind,
  WHEN_DESCRIPTION,
  ZALO_EVENT_GROUPS,
  type DirectoryPerson,
  type UnitName,
  type ZaloChannelDraft,
} from "./zalo-channel-form";

type Directory = KetQua<{ items: DirectoryPerson[] }>;
type Units = KetQua<{ items: UnitName[] }>;

/**
 * "Cấu hình → Kênh Zalo" (ADR 0074 #6), laid out by spec 11 and the prototype's `ZaloChannelPanel`
 * (ADR 0079). One key, `admin.lookup`, for read and write (like Máy chủ thư), so the tab hides as a
 * whole without it (convenience; comms refuses).
 *
 * EVERY CHANGE SAVES AT ONCE (spec 11): the whole settings row is PUT — the server's PUT replaces the
 * row — and a refusal puts the screen back to what is stored, so it never shows a state that was not
 * saved. All controls are disabled while one save is in flight: two overlapping full-row PUTs would
 * let the slower one overwrite the other's change.
 *
 * WHAT THE SERVER CANNOT DO YET is drawn at its spec position, disabled, with "?" (ADR 0068 §14): the
 * commune's own bot, the "nhắc trước" threshold, the events no producer sends, the staff total and the
 * not-yet-linked list. The linked list is names, codes and a date — never a Zalo chat id.
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

  useEffect(() => {
    if (!allowed) return;
    let gone = false;
    getZaloChannelSettings().then((r) => {
      if (gone) return;
      setLoaded(r);
      if (r.ok) setDraft(draftFromSettings(r.duLieu));
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
    setSaving(true);
    const r = await saveZaloChannelSettings(built.change);
    setSaving(false);
    if (!r.ok) {
      setDraft(draftFromSettings(saved));
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
      onChange={(d) => void apply(d)}
      saving={saving}
      held={held}
      people={peopleState(staff, directory, units)}
    />
  );
}

function pendingEntry(name: string): PendingFeatureInfo {
  const entry = PHAN_CHUA_DUNG.find((p) => p.ten === name);
  if (entry === undefined) throw new Error(`PHAN_CHUA_DUNG has no entry "${name}"`);
  return entry;
}

const BOT_ENTRY = pendingEntry("Con bot của xã");
const DUE_SOON_ENTRY = pendingEntry("Nhắc trước khi đến hạn qua Zalo");
const MORE_EVENTS_ENTRY = pendingEntry("Thêm loại việc nhắn qua Zalo");

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

export function peopleState(staff: KetQua<ZaloLinkedStaff[]> | null, directory: Directory | null, units: Units | null): PeopleState {
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
  onChange,
  saving,
  held,
  people,
}: {
  draft: ZaloChannelDraft;
  onChange: (d: ZaloChannelDraft) => void;
  saving: boolean;
  held: string | null;
  people: PeopleState;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-5">
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
          <ZaloField
            label="Sắp đến hạn: nhắc trước"
            htmlFor="zalo-due-soon-days"
            marker={<PendingMarker info={DUE_SOON_ENTRY} />}
          >
            <select id="zalo-due-soon-days" className={daySelectCls} disabled>
              <option value="" />
            </select>
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
                  if (kind === null) {
                    // The "?" sits OUTSIDE the label: a button inside a label joins its accessible name.
                    return (
                      <div key={ev.code} className="flex items-start gap-1.5" data-pending="">
                        <label className="flex cursor-not-allowed items-start gap-2.5 opacity-60">
                          <input type="checkbox" className={CHECKBOX} data-event={ev.code} checked={false} disabled readOnly />
                          {text}
                        </label>
                        <PendingMarker info={MORE_EVENTS_ENTRY} className="mt-0.5" />
                      </div>
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
                        onChange={(e) => onChange(toggleKind(draft, kind, e.target.checked))}
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

      <BotSection />

      <LinkedStaffSection people={people} />

      {saving && (
        <p role="status" className="text-ink-muted m-0 flex items-center gap-1.5 text-[12px]">
          <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />
          Đang lưu…
        </p>
      )}
    </div>
  );
}

/**
 * §3 — ZaloChannelPanel.tsx:337-510. The commune's own bot is decided (ADR 0079 #4) but comms has no
 * route for it yet, so the whole section is its shape, disabled, under ONE "?". No input has a
 * `name` and no button an action: nothing here can send a token anywhere.
 */
function BotSection() {
  return (
    <section className={SECTION} aria-labelledby="zalo-bot-title" data-pending="">
      <div className="flex flex-wrap items-start gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <h3 id="zalo-bot-title" className={TITLE}>
              Con bot của xã
            </h3>
            <PendingMarker info={BOT_ENTRY} />
          </div>
          <p className={DESCRIPTION}>
            Xã đang dùng bot chung của nền tảng. Để trống phần dưới là giữ nguyên như vậy — chỉ nhập mã bot khi xã muốn
            bot mang tên mình.
          </p>
        </div>
        <span className="bg-background text-ink-muted rounded-full px-3 py-1 text-[12px] font-semibold">Bot chung</span>
      </div>

      {/* A <form> only because a password input outside one makes the browser warn ("[DOM] Password field
          is not contained in a form"). Fully disabled, no action, submit prevented: nothing is sent. */}
      <form aria-label="Con bot của xã" className="m-0" onSubmit={(e) => e.preventDefault()}>
        <fieldset disabled className="m-0 min-w-0 border-0 p-0">
      <div className="mt-4 grid gap-4 sm:grid-cols-2">
        <ZaloField
          label="Mã bot của xã"
          htmlFor="zalo-bot-token"
          hint="Lấy trong Mini App “Zalo Bot Creator”. Để trống là dùng bot chung."
        >
          <input
            id="zalo-bot-token"
            type="password"
            autoComplete="off"
            placeholder="123456789:abc-xyz"
            className={botInputCls}
            disabled
          />
        </ZaloField>
        <ZaloField label="Tên bot" htmlFor="zalo-bot-name" hint="Tên hiển thị trong Zalo, bắt đầu bằng “Bot”.">
          <input id="zalo-bot-name" type="text" className={botInputCls} disabled />
        </ZaloField>
        <ZaloField
          label="Đường mở khung chat"
          htmlFor="zalo-bot-chat-url"
          hint="Link bot dạng https://zalo.me/<số>. Mã QR cán bộ quét được dựng từ đây."
        >
          <input id="zalo-bot-chat-url" type="text" placeholder="https://zalo.me/..." className={botInputCls} disabled />
        </ZaloField>
        <ZaloField
          label="Secret Token của webhook"
          htmlFor="zalo-bot-secret"
          hint="Chép sang ứng dụng Zalo nếu xã tự đăng ký webhook bằng tay."
        >
          <input id="zalo-bot-secret" type="text" className={cn(botInputCls, "font-mono")} disabled />
        </ZaloField>
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Button type="button" variant="primary" disabled>
          Lưu con bot
        </Button>
        <Button
          type="button"
          variant="outline"
          className="bg-white"
          icon={<RefreshCw aria-hidden="true" focusable="false" />}
          disabled
        >
          Kiểm tra kết nối
        </Button>
      </div>
        </fieldset>
      </form>
    </section>
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
