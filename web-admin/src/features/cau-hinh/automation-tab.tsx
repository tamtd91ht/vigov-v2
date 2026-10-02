"use client";

import { BellRing, History, Play, Save } from "lucide-react";
import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CardHeader, CardTitle } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { NoAccess } from "@/components/ui/no-access";
import { Notice } from "@/components/ui/notice";
import { PendingSection } from "@/components/ui/pending-feature";
import { SkeletonRows } from "@/components/ui/skeleton";
import { BusyLabel } from "@/features/danh-ba/busy-label";

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
  AUTOMATION_RECIPIENTS,
  AUTOMATION_TITLE,
  ENABLE_LABEL,
  NO_RUNS_YET,
  RUN_NOW_BUTTON,
  RUN_NOW_NEEDS_ON,
  RUN_NOW_SENDING,
  SAVE_BUTTON,
  SAVED_SENTENCE,
  SAVING,
  WEEKDAYS,
  cadenceSentence,
  count,
  draftFrom,
  jobWords,
  outcomeLabel,
  runRequestSentence,
  settingBody,
  stateSentence,
  timezoneLabel,
  triggerLabel,
  when,
  workKindLabel,
  type AutomationDraft,
} from "./automation-form";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";
import { automationTabDecision } from "./quyen-tab";

/**
 * "Cấu hình → Tự động hoá" (§9, ADR 0058). All three routes declare `admin.sla`, the read included, so
 * the tab hides as a whole without it — convenience; the server refuses on every request.
 *
 * TWO JOBS OF §9 ARE NOT LIVE CARDS: `Tính lại số liệu Tổng quan` is dropped (ADR 0053: the overview
 * counts live, there is nothing to precompute) and gets NO placeholder — a spot for it would announce
 * a job the owner refused (ADR 0068 §14). `Gửi báo cáo định kỳ` waits for `/bao-cao` (ADR 0058 §4)
 * and sits in its spec position, the last card, disabled with the "?" (`PendingReportJobCard`).
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
    <section className="tab-danh-muc flex min-w-0 flex-col gap-4 [&>*]:my-0" aria-labelledby="tieu-de-tu-dong-hoa">
      <div className="min-w-0 overflow-hidden rounded-card border border-line bg-surface shadow-sm">
        <CardHeader className="m-0">
          <div className="min-w-0 flex-1 basis-64">
            <CardTitle as="h2" id="tieu-de-tu-dong-hoa" className="flex items-center gap-2">
              <BellRing aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
              {AUTOMATION_TITLE}
            </CardTitle>
            <p className="ghi-chu m-0 mt-1 text-[13px] text-ink-500">{AUTOMATION_GUIDANCE}</p>
          </div>
        </CardHeader>
        <div className="p-4">
          <Notice tone="info" className="ghi-chu">
            {AUTOMATION_RECIPIENTS}
          </Notice>
        </div>
      </div>
      {loaded === null ? (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải cấu hình tự động hoá…
          </p>
          <SkeletonRows rows={3} className="rounded-card border border-line bg-surface" />
        </>
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

/** The `Gửi báo cáo định kỳ` entry — looked up by name so a renamed entry fails a test, not a screen. */
const REPORT_JOB = PHAN_CHUA_DUNG.find((p) => p.ten === "Gửi báo cáo định kỳ");

/**
 * Spec §9's fifth card, "Gửi báo cáo định kỳ", in the shape of a job card: its spec description and
 * a DISABLED on/off switch, with the "?" in the header. No server call, nothing to save.
 */
function PendingReportJobCard() {
  if (REPORT_JOB === undefined) return null;
  return (
    <PendingSection info={REPORT_JOB} titleAs="h3" className="m-0">
      <div className="flex flex-col gap-3">
        <p className="m-0 text-[13px] text-ink-500">Báo cáo tuần vào đầu tuần, báo cáo tháng vào ngày mùng 1.</p>
        <div className="o-nhap o-chon m-0">
          <input id="tu-dong-hoa-bao-cao-dinh-ky-bat" type="checkbox" role="switch" disabled />
          <label htmlFor="tu-dong-hoa-bao-cao-dinh-ky-bat">{ENABLE_LABEL}</label>
        </div>
      </div>
    </PendingSection>
  );
}

function AutomationJobCard({ initial }: { initial: AutomationJob }) {
  const [job, setJob] = useState(initial);
  const [draft, setDraft] = useState<AutomationDraft>(() => draftFrom(initial));
  const [busy, setBusy] = useState<"" | "save" | "run">("");
  const [notice, setNotice] = useState<{ ok: boolean; text: string } | null>(null);

  async function save() {
    if (busy !== "") return;
    const b = settingBody(job, draft);
    if (!b.ok) {
      setNotice({ ok: false, text: b.message });
      return;
    }
    setBusy("save");
    setNotice(null);
    const r = await saveAutomationJob(job.job, b.body);
    setBusy("");
    if (!r.ok) {
      // The draft stays: the refusal (a bound, or someone else saved) is about what is on screen.
      setNotice({ ok: false, text: r.thongBao });
      return;
    }
    setJob(r.duLieu);
    setDraft(draftFrom(r.duLieu));
    setNotice({ ok: true, text: SAVED_SENTENCE });
  }

  async function runNow() {
    if (busy !== "") return;
    setBusy("run");
    setNotice(null);
    const r = await requestAutomationRun(job.job);
    setBusy("");
    if (!r.ok) {
      setNotice({ ok: false, text: r.thongBao });
      return;
    }
    // The card's run line is drawn from `run_requested_at` of the 202 body — see `runRequestSentence`.
    setJob(r.duLieu);
  }

  return (
    <AutomationJobCardView
      job={job}
      draft={draft}
      busy={busy}
      notice={notice}
      onDraft={setDraft}
      onSave={() => void save()}
      onRunNow={() => void runNow()}
    />
  );
}

/** Pure rendering of one job, exported so each state's markup has a test. */
export function AutomationJobCardView({
  job,
  draft,
  busy,
  notice,
  onDraft,
  onSave,
  onRunNow,
}: {
  job: AutomationJob;
  draft: AutomationDraft;
  busy: "" | "save" | "run";
  notice: { ok: boolean; text: string } | null;
  onDraft: (d: AutomationDraft) => void;
  onSave: () => void;
  onRunNow: () => void;
}) {
  const words = jobWords(job.job);
  const id = `tu-dong-hoa-${job.job}`;
  const requestLine = runRequestSentence(job);
  // "Run now" on a job that is off answers 409; the button is drawn only when the SAVED state is on.
  const canRunNow = job.configured && job.enabled;

  return (
    <article
      className="the-loi-he-thong m-0 flex min-w-0 flex-col gap-3 rounded-card border border-line bg-surface p-4 shadow-sm [&>*]:my-0"
      aria-labelledby={`${id}-ten`}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1 basis-64">
          <h3 id={`${id}-ten`} className="m-0 text-[15px] font-semibold text-ink-900">
            {words.title}
          </h3>
          {words.description !== "" && <p className="ghi-chu m-0 mt-1 text-[13px] text-ink-500">{words.description}</p>}
        </div>
        <p className="m-0">
          {/* Tone by the CODES (`configured`, `enabled`); icon + word, never colour alone. */}
          <Badge tone={job.configured && job.enabled ? "success" : "neutral"} className="whitespace-normal">
            {stateSentence(job)}
          </Badge>
        </p>
      </div>
      {job.configured && job.enabled && (
        <p className="ghi-chu text-[13px] text-ink-500">
          {cadenceSentence(job)} ({timezoneLabel(job.timezone)}). Bật từ {when(job.enabled_at)}.
        </p>
      )}

      <form
        className="form-danh-muc m-0"
        aria-label={`Cấu hình việc ${words.title}`}
        onSubmit={(e) => {
          e.preventDefault();
          onSave();
        }}
      >
        <fieldset disabled={busy !== ""} className="m-0 grid min-w-0 gap-4 border-0 p-0 sm:grid-cols-2 [&>*]:m-0 [&>.cum-nut]:col-span-full [&>.ghi-chu]:col-span-full">
          <div className="o-nhap o-chon col-span-full">
            <input
              id={`${id}-bat`}
              type="checkbox"
              role="switch"
              checked={draft.enabled}
              onChange={(e) => onDraft({ ...draft, enabled: e.target.checked })}
            />
            <label htmlFor={`${id}-bat`}>{ENABLE_LABEL}</label>
          </div>

          {job.schedule_kind === "interval" && (
            <div className="o-nhap">
              <label htmlFor={`${id}-nhip`}>Cứ mỗi (phút)</label>
              {/* `inputMode="numeric"`, not `type="number"`: a wheel scroll silently changes a number box. */}
              <input
                id={`${id}-nhip`}
                inputMode="numeric"
                value={draft.interval}
                onChange={(e) => onDraft({ ...draft, interval: e.target.value })}
                aria-describedby={`${id}-nhip-goi-y`}
              />
              <p className="ghi-chu" id={`${id}-nhip-goi-y`}>
                {job.min_interval_minutes === null
                  ? "Số phút giữa hai lượt quét."
                  : `Ít nhất ${job.min_interval_minutes} phút giữa hai lượt quét.`}
              </p>
            </div>
          )}

          {job.schedule_kind === "weekly" && (
            <div className="o-nhap">
              <label htmlFor={`${id}-thu`}>Vào</label>
              <select
                id={`${id}-thu`}
                value={draft.weekday}
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

          {(job.schedule_kind === "daily" || job.schedule_kind === "weekly") && (
            <div className="o-nhap">
              <label htmlFor={`${id}-gio`}>Lúc ({timezoneLabel(job.timezone)})</label>
              <input
                id={`${id}-gio`}
                type="time"
                step={60}
                value={draft.time}
                onChange={(e) => onDraft({ ...draft, time: e.target.value })}
              />
            </div>
          )}

          <div className="cum-nut flex flex-wrap justify-end gap-2">
            {canRunNow && (
              <Button
                type="button"
                variant="secondary"
                icon={busy === "run" ? undefined : <Play aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                onClick={onRunNow}
                aria-busy={busy === "run"}
              >
                <BusyLabel busy={busy === "run"} label={RUN_NOW_BUTTON} busyText={RUN_NOW_SENDING} />
              </Button>
            )}
            <Button
              type="submit"
              variant="primary"
              icon={busy === "save" ? undefined : <Save aria-hidden="true" focusable="false" strokeWidth={1.8} />}
              aria-busy={busy === "save"}
            >
              <BusyLabel busy={busy === "save"} label={SAVE_BUTTON} busyText={SAVING} />
            </Button>
          </div>
          {!canRunNow && <p className="ghi-chu">{RUN_NOW_NEEDS_ON}</p>}
        </fieldset>
      </form>

      {notice !== null &&
        (notice.ok ? (
          <p role="status" className="text-sm font-medium text-success-600">
            {notice.text}
          </p>
        ) : (
          // Server sentences (400 bound, 409 disabled / changed) arrive here verbatim.
          <p className="thong-bao-loi" role="alert">
            {notice.text}
          </p>
        ))}

      {requestLine !== null && <p className="ghi-chu text-[13px] text-ink-500">{requestLine}</p>}

      <h4 className="flex items-center gap-2 text-sm font-semibold text-ink-900">
        <History aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0 text-ink-500" />
        Lượt chạy gần nhất
      </h4>
      {job.last_runs.length === 0 ? (
        <p className="trang-thai-rong m-0">{NO_RUNS_YET}</p>
      ) : (
        <TableScroll sticky aria-label={`Lượt chạy gần nhất — ${words.title}`}>
          <table className={`bang-danh-muc ${DATA_TABLE_CLASS}`}>
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
          </table>
        </TableScroll>
      )}
    </article>
  );
}
