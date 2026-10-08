"use client";

import { BellRing, Play } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { NoAccess } from "@/components/ui/no-access";
import { PendingMarker } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { cn } from "@/lib/cn";

import { usePhien } from "@/features/phien/phien-hien-tai";
import {
  listAutomationJobs,
  requestAutomationRun,
  saveAutomationJob,
  type AutomationJob,
} from "@/lib/api/automation-jobs";
import type { KetQua } from "@/lib/api/goi";

import {
  AUTOMATION_GUIDANCE,
  AUTOMATION_TITLE,
  RUN_NOW_BUTTON,
  RUN_NOW_SENDING,
  SAVE_CADENCE_BUTTON,
  WEEKDAYS,
  cadenceChanged,
  cadenceSavedToast,
  clockFieldLabel,
  count,
  draftFrom,
  intervalLabel,
  intervalOptions,
  jobWords,
  lastRunSentence,
  outcomeLabel,
  runRequestSentence,
  runRequestedToast,
  settingBody,
  switchWord,
  toggledToast,
  triggerLabel,
  when,
  workKindLabel,
  type AutomationDraft,
} from "./automation-form";
import { ConfigTable, SMALL_BUTTON_CLASS, selectCls } from "./config-ui";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";
import { automationTabDecision } from "./quyen-tab";

/**
 * "Cấu hình → Tự động hoá" (§9, ADR 0058). All three routes declare `admin.sla`, the read included, so
 * the tab hides as a whole without it — convenience; the server refuses on every request.
 *
 * TWO JOBS OF §9 ARE NOT LIVE CARDS: `Tính lại số liệu Tổng quan` is dropped (ADR 0053: the overview
 * counts live, there is nothing to precompute) and gets NO placeholder — a spot for it would announce
 * a job the owner refused (ADR 0068 §14). `Gửi báo cáo định kỳ` waits for `/bao-cao`'s export (ADR 0058 §4, ADR 0053 B4)
 * and sits in its spec position, the last card, disabled with the "?" (`PendingReportJobCard`).
 *
 * Drawn after `tmp/web/cau-hinh/vigov-cau-hinh-spec/09-tu-dong-hoa.md` and the prototype's
 * `AutomationTable.tsx` (ADR 0079). "Chạy ngay" and the last-runs table are not in either; ADR 0079
 * decision 3 keeps them, inside the spec's card.
 */
export function AutomationTab() {
  const phien = usePhien();
  const decision = phien === null ? null : automationTabDecision(phien);
  const allowed = decision !== null && decision.hien;

  const [loaded, setLoaded] = useState<KetQua<readonly AutomationJob[]> | null>(null);

  useEffect(() => {
    // Read only once the session says the key is held: a read that can only 403 is not sent.
    if (!allowed) return;
    let gone = false;
    void listAutomationJobs().then((r) => {
      if (!gone) setLoaded(r);
    });
    return () => {
      gone = true;
    };
  }, [allowed]);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p className="thong-bao-loi" role="alert">
        {decision.thongBao}
      </p>
    ) : (
      // Shared `NoAccess` (spec v2 §8b) + this tab's own sentence, verbatim, as its caption.
      <div className="khung-thieu-quyen flex min-w-0 flex-col items-center pb-10 [&>.trang-thai-rong]:m-0 [&>.trang-thai-rong]:max-w-md [&>.trang-thai-rong]:border-0 [&>.trang-thai-rong]:bg-transparent [&>.trang-thai-rong]:px-4 [&>.trang-thai-rong]:py-0 [&>.trang-thai-rong]:text-center [&>.trang-thai-rong]:text-[13px] [&>.trang-thai-rong]:text-ink-500">
        <NoAccess className="pb-4" />
        <p className="trang-thai-rong">
          Tài khoản của bạn không có quyền cấu hình tự động hoá, nên tab này không hiển thị.
        </p>
      </div>
    );
  }

  return <AutomationTabView loaded={loaded} />;
}

/** Pure list rendering — exported so the loading, failure and empty states have tests. */
export function AutomationTabView({ loaded }: { loaded: KetQua<readonly AutomationJob[]> | null }) {
  return (
    // Spec 09 / prototype `AutomationTable.tsx:49-59`: the guidance as plain text, then one card per
    // job. No outer card, no visible tab title.
    <section className="min-w-0 space-y-3" aria-labelledby="tieu-de-tu-dong-hoa">
      <h2 id="tieu-de-tu-dong-hoa" className="an-thi-giac">
        {AUTOMATION_TITLE}
      </h2>
      {/* The guidance only: text the prototype lacks is dropped (owner, 08/10/2026). */}
      <p className="text-ink-muted max-w-2xl text-[12.5px]">{AUTOMATION_GUIDANCE}</p>
      {loaded === null ? (
        <div>
          <p role="status" className="an-thi-giac">
            Đang tải cấu hình tự động hoá…
          </p>
          <Skeleton className="rounded-card h-80 w-full" />
        </div>
      ) : !loaded.ok ? (
        // Kept as the one-line alert (not ErrorState): the tab's test pins this exact markup.
        <p className="thong-bao-loi" role="alert">
          {loaded.thongBao}
        </p>
      ) : loaded.duLieu.length === 0 ? (
        <EmptyState icon={BellRing} title="Chưa có việc tự động hoá nào." className="rounded-card border border-line bg-surface" />
      ) : (
        loaded.duLieu.map((j) => <AutomationJobCard key={j.job} initial={j} />)
      )}
      <PendingReportJobCard />
    </section>
  );
}

/** Spec 09's card frame; the fill is white while the job is on, the page colour while it is off. */
const CARD_CLASS = "border-line shadow-card rounded-card border border-solid p-4";

/**
 * TOKEN TRAP (`config-ui.tsx`): the spec's off-card `bg-surface` is the page colour #f4f8fb, which is
 * `bg-background` in this app — `bg-surface` here is white, and an off card would look on.
 */
const CARD_OFF_FILL = "bg-background";

/** Spec 09's field label, above its control. */
const FIELD_LABEL_CLASS = "text-ink-muted m-0 mb-1 block text-[11px] font-semibold";

/**
 * Spec 09's select: `selectCls` + `disabled:opacity-60`. `pr-8` keeps the legacy chevron clear and
 * `min-w-0` undoes the legacy 12rem floor on every select, as `formSelectCls` does — without it
 * "Cứ mỗi" draws ~190px wide for "15 phút".
 */
const CADENCE_SELECT_CLASS = cn(selectCls, "min-w-0 pr-8 disabled:opacity-60");

/** The `Gửi báo cáo định kỳ` entry — looked up by name so a renamed entry fails a test, not a screen. */
const REPORT_JOB = PHAN_CHUA_DUNG.find((p) => p.ten === "Gửi báo cáo định kỳ");

/**
 * Spec §9's fifth card, "Gửi báo cáo định kỳ", in the shape of a job card that is off: its spec
 * description and a DISABLED switch, with the "?" beside the title (ADR 0068 §14). No server call.
 */
function PendingReportJobCard() {
  if (REPORT_JOB === undefined) return null;
  return (
    <section className={cn(CARD_CLASS, CARD_OFF_FILL)} aria-labelledby="tu-dong-hoa-bao-cao-dinh-ky-ten" data-pending="">
      <div className="flex flex-wrap items-start gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <h3 id="tu-dong-hoa-bao-cao-dinh-ky-ten" className="text-navy m-0 text-[13px] font-bold">
              {REPORT_JOB.ten}
            </h3>
            <PendingMarker info={REPORT_JOB} />
          </div>
          <p className="text-ink-muted m-0 mt-0.5 text-[12px]">Báo cáo tuần vào đầu tuần, báo cáo tháng vào ngày mùng 1.</p>
        </div>
        <label className="m-0 flex shrink-0 items-center gap-2 text-[12.5px]">
          <Switch id="tu-dong-hoa-bao-cao-dinh-ky-bat" checked={false} disabled aria-labelledby="tu-dong-hoa-bao-cao-dinh-ky-ten" />
          {switchWord(false)}
        </label>
      </div>
    </section>
  );
}

export type AutomationBusy = "" | "toggle" | "save" | "run";

/** `configured: false` is OFF whatever `enabled` says: no saved row runs nothing (ADR 0058). */
function isOn(j: AutomationJob): boolean {
  return j.configured && j.enabled;
}

function AutomationJobCard({ initial }: { initial: AutomationJob }) {
  const [job, setJob] = useState(initial);
  const [draft, setDraft] = useState<AutomationDraft>(() => draftFrom(initial));
  // The switch as drawn: moves at the click, back to the SAVED state if the server refuses.
  const [on, setOn] = useState(() => isOn(initial));
  const [busy, setBusy] = useState<AutomationBusy>("");
  const [error, setError] = useState<string | null>(null);
  const title = jobWords(job.job).title;

  /** Spec 09: the switch saves at once, with the SAVED cadence — an unsaved edit is not sent with it. */
  async function toggle(next: boolean) {
    if (busy !== "") return;
    const b = settingBody(job, { ...draftFrom(job), enabled: next });
    if (!b.ok) {
      toast.error(b.message);
      return;
    }
    const before = job;
    setOn(next);
    setBusy("toggle");
    setError(null);
    const r = await saveAutomationJob(job.job, b.body);
    setBusy("");
    if (!r.ok) {
      setOn(isOn(before));
      toast.error(r.thongBao);
      return;
    }
    setJob(r.duLieu);
    setDraft(draftFrom(r.duLieu));
    setOn(isOn(r.duLieu));
    toast.success(toggledToast(title, next));
  }

  async function saveCadence() {
    if (busy !== "") return;
    const b = settingBody(job, { ...draft, enabled: isOn(job) });
    if (!b.ok) {
      setError(b.message);
      return;
    }
    const before = job;
    setBusy("save");
    setError(null);
    const r = await saveAutomationJob(job.job, b.body);
    setBusy("");
    if (!r.ok) {
      // The server's sentence in place, and the saved cadence back on screen (spec 09). A too-short
      // interval is not told apart: `saveAutomationJob` asks for no `code`, and the server's sentence
      // already names the bound ("nhịp nhắc việc phải từ 5 đến 10080 phút").
      setError(r.thongBao);
      setDraft(draftFrom(before));
      return;
    }
    setJob(r.duLieu);
    setDraft(draftFrom(r.duLieu));
    toast.success(cadenceSavedToast(title));
  }

  async function runNow() {
    if (busy !== "") return;
    setBusy("run");
    setError(null);
    const r = await requestAutomationRun(job.job);
    setBusy("");
    if (!r.ok) {
      setError(r.thongBao);
      return;
    }
    // The card's request line is drawn from `run_requested_at` of the 202 body — see `runRequestSentence`.
    setJob(r.duLieu);
    toast.success(runRequestedToast(title));
  }

  return (
    <AutomationJobCardView
      job={job}
      draft={draft}
      on={on}
      busy={busy}
      error={error}
      onToggle={(next) => void toggle(next)}
      onDraft={setDraft}
      onSave={() => void saveCadence()}
      onRunNow={() => void runNow()}
    />
  );
}

/** Pure rendering of one job (spec 09 "Thẻ job"), exported so each state's markup has a test. */
export function AutomationJobCardView({
  job,
  draft,
  on,
  busy,
  error,
  onToggle,
  onDraft,
  onSave,
  onRunNow,
}: {
  job: AutomationJob;
  draft: AutomationDraft;
  /** The switch as drawn. */
  on: boolean;
  busy: AutomationBusy;
  /** A refusal of "Lưu nhịp" or "Chạy ngay": the server's sentence, verbatim. */
  error: string | null;
  onToggle: (next: boolean) => void;
  onDraft: (d: AutomationDraft) => void;
  onSave: () => void;
  onRunNow: () => void;
}) {
  const words = jobWords(job.job);
  const id = `tu-dong-hoa-${job.job}`;
  const requestLine = runRequestSentence(job);
  const disabled = busy !== "";
  // "Run now" on a job that is off answers 409: drawn only when the SAVED state is on.
  const canRunNow = isOn(job);

  return (
    <section className={cn(CARD_CLASS, on ? "bg-white" : CARD_OFF_FILL)} aria-labelledby={`${id}-ten`}>
      <div className="flex flex-wrap items-start gap-3">
        <div className="min-w-0 flex-1">
          <h3 id={`${id}-ten`} className="text-navy m-0 text-[13px] font-bold">
            {words.title}
          </h3>
          {words.description !== "" && <p className="text-ink-muted m-0 mt-0.5 text-[12px]">{words.description}</p>}
          <p className="text-ink-muted m-0 mt-1 text-[11px]">{lastRunSentence(job)}</p>
        </div>
        <label className="m-0 flex shrink-0 items-center gap-2 text-[12.5px]">
          <Switch id={`${id}-bat`} checked={on} disabled={disabled} aria-labelledby={`${id}-ten`} onCheckedChange={onToggle} />
          {switchWord(on)}
        </label>
      </div>

      {on && (
        <form
          className="border-line m-0 mt-3 flex flex-wrap items-end gap-3 border-t pt-3"
          aria-label={`Nhịp chạy — ${words.title}`}
          onSubmit={(e) => {
            e.preventDefault();
            onSave();
          }}
        >
          {job.schedule_kind === "interval" && (
            <div className="block">
              <label htmlFor={`${id}-nhip`} className={FIELD_LABEL_CLASS}>
                Cứ mỗi
              </label>
              <select
                id={`${id}-nhip`}
                className={CADENCE_SELECT_CLASS}
                value={draft.interval}
                disabled={disabled}
                onChange={(e) => onDraft({ ...draft, interval: e.target.value })}
              >
                {draft.interval === "" && <option value="">— Chọn nhịp —</option>}
                {intervalOptions(job, draft.interval).map((m) => (
                  <option key={m} value={String(m)}>
                    {intervalLabel(m)}
                  </option>
                ))}
              </select>
            </div>
          )}

          {job.schedule_kind === "weekly" && (
            <div className="block">
              <label htmlFor={`${id}-thu`} className={FIELD_LABEL_CLASS}>
                Vào
              </label>
              <select
                id={`${id}-thu`}
                className={CADENCE_SELECT_CLASS}
                value={draft.weekday}
                disabled={disabled}
                onChange={(e) => onDraft({ ...draft, weekday: e.target.value })}
              >
                {draft.weekday === "" && <option value="">— Chọn thứ —</option>}
                {WEEKDAYS.map((w) => (
                  <option key={w.value} value={String(w.value)}>
                    {w.label}
                  </option>
                ))}
              </select>
            </div>
          )}

          {job.schedule_kind !== "interval" && (
            <div className="block">
              <label htmlFor={`${id}-gio`} className={FIELD_LABEL_CLASS}>
                {clockFieldLabel(job.timezone)}
              </label>
              <input
                id={`${id}-gio`}
                type="time"
                step={60}
                className={cn(selectCls, "disabled:opacity-60")}
                value={draft.time}
                disabled={disabled}
                onChange={(e) => onDraft({ ...draft, time: e.target.value })}
              />
            </div>
          )}

          {/* `ml-auto` on the group, not on "Lưu nhịp": the button comes and goes, "Chạy ngay" stays right. */}
          <div className="ml-auto flex flex-wrap items-center gap-2">
            {cadenceChanged(job, draft) && (
              <Button
                type="submit"
                variant="primary"
                size="sm"
                className={SMALL_BUTTON_CLASS}
                disabled={disabled}
                aria-busy={busy === "save"}
              >
                <BusyLabel busy={busy === "save"} label={SAVE_CADENCE_BUTTON} busyText={BUSY_SAVING} />
              </Button>
            )}
            {canRunNow && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className={SMALL_BUTTON_CLASS}
                icon={busy === "run" ? undefined : <Play aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                disabled={disabled}
                onClick={onRunNow}
                aria-busy={busy === "run"}
              >
                <BusyLabel busy={busy === "run"} label={RUN_NOW_BUTTON} busyText={RUN_NOW_SENDING} />
              </Button>
            )}
          </div>
        </form>
      )}

      {error !== null && (
        // Server sentences (400 bound, 409 disabled / changed) arrive here verbatim.
        <p className="thong-bao-loi m-0 mt-2" role="alert">
          {error}
        </p>
      )}
      {requestLine !== null && <p className="text-ink-muted m-0 mt-2 text-[11px]">{requestLine}</p>}

      {job.last_runs.length > 0 && (
        // ADR 0079 decision 3 (not in spec 09): the last run of each kind of work, compact, in the card.
        <div className="border-line mt-3 border-t pt-3">
          <h4 className="text-ink-muted m-0 mb-2 text-[11px] font-semibold">Lượt chạy gần nhất</h4>
          <ConfigTable label={`Lượt chạy gần nhất — ${words.title}`}>
            <thead>
              <tr>
                <th scope="col">Loại việc</th>
                <th scope="col">Bắt đầu lúc</th>
                <th scope="col">Cách chạy</th>
                <th scope="col">Kết quả</th>
                <th scope="col">Hồ sơ đã quét</th>
                <th scope="col">Thông báo đã gửi</th>
                <th scope="col">Không tìm được người nhận</th>
              </tr>
            </thead>
            <tbody>
              {job.last_runs.map((r) => (
                <tr key={r.run_id}>
                  <td>{workKindLabel(r.work_kind)}</td>
                  <td>{when(r.claimed_at)}</td>
                  <td>{triggerLabel(r.trigger)}</td>
                  <td>{outcomeLabel(r.outcome)}</td>
                  <td>{count(r.records_examined)}</td>
                  <td>{count(r.notices_delivered)}</td>
                  <td>{count(r.records_without_recipient)}</td>
                </tr>
              ))}
            </tbody>
          </ConfigTable>
        </div>
      )}
    </section>
  );
}
