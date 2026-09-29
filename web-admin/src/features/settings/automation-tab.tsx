"use client";

import { useEffect, useState } from "react";

import { usePhien } from "@/features/session/current-session";
import {
  listAutomationJobs,
  requestAutomationRun,
  saveAutomationJob,
  type AutomationJob,
} from "@/lib/api/automation-jobs";
import type { KetQua } from "@/lib/api/request";

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
import { automationTabDecision } from "./tab-permissions";

/**
 * "Cấu hình → Tự động hoá" (§9, ADR 0058). All three routes declare `admin.sla`, the read included, so
 * the tab hides as a whole without it — convenience; the server refuses on every request.
 *
 * TWO JOBS OF §9 ARE NOT HERE, on purpose: `Tính lại số liệu Tổng quan` is dropped (ADR 0053: the
 * overview counts live, there is nothing to precompute) and `Gửi báo cáo định kỳ` waits for `/bao-cao`
 * (ADR 0058 §4). Both are said in `PHAN_CHUA_DUNG`, with the reason.
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
      <p className="trang-thai-rong">
        Tài khoản của bạn không có quyền cấu hình tự động hoá, nên tab này không hiển thị.
      </p>
    );
  }

  return <AutomationTabView loaded={loaded} />;
}

/** Pure list rendering — exported so the loading, failure and empty states have tests. */
export function AutomationTabView({ loaded }: { loaded: KetQua<readonly AutomationJob[]> | null }) {
  return (
    <section className="tab-danh-muc" aria-labelledby="tieu-de-tu-dong-hoa">
      <h2 id="tieu-de-tu-dong-hoa">{AUTOMATION_TITLE}</h2>
      <p className="ghi-chu">{AUTOMATION_GUIDANCE}</p>
      <p className="ghi-chu">{AUTOMATION_RECIPIENTS}</p>
      {loaded === null ? (
        <p role="status">Đang tải cấu hình tự động hoá…</p>
      ) : !loaded.ok ? (
        <p className="thong-bao-loi" role="alert">
          {loaded.thongBao}
        </p>
      ) : loaded.duLieu.length === 0 ? (
        <p className="trang-thai-rong">Chưa có việc tự động hoá nào.</p>
      ) : (
        loaded.duLieu.map((j) => <AutomationJobCard key={j.job} initial={j} />)
      )}
    </section>
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
    <article className="the-loi-he-thong" aria-labelledby={`${id}-ten`}>
      <h3 id={`${id}-ten`}>{words.title}</h3>
      {words.description !== "" && <p className="ghi-chu">{words.description}</p>}
      <p>
        <span className={job.configured && job.enabled ? "chip chip-hoat-dong" : "chip chip-ngung"}>
          {stateSentence(job)}
        </span>
      </p>
      {job.configured && job.enabled && (
        <p className="ghi-chu">
          {cadenceSentence(job)} ({timezoneLabel(job.timezone)}). Bật từ {when(job.enabled_at)}.
        </p>
      )}

      <form
        className="form-danh-muc"
        aria-label={`Cấu hình việc ${words.title}`}
        onSubmit={(e) => {
          e.preventDefault();
          onSave();
        }}
      >
        <fieldset disabled={busy !== ""}>
          <div className="o-nhap o-chon">
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

          <div className="cum-nut">
            <button type="submit" className="nut-chinh">
              {busy === "save" ? SAVING : SAVE_BUTTON}
            </button>
            {canRunNow && (
              <button type="button" className="nut-phu" onClick={onRunNow}>
                {busy === "run" ? RUN_NOW_SENDING : RUN_NOW_BUTTON}
              </button>
            )}
          </div>
          {!canRunNow && <p className="ghi-chu">{RUN_NOW_NEEDS_ON}</p>}
        </fieldset>
      </form>

      {notice !== null &&
        (notice.ok ? (
          <p role="status">{notice.text}</p>
        ) : (
          // Server sentences (400 bound, 409 disabled / changed) arrive here verbatim.
          <p className="thong-bao-loi" role="alert">
            {notice.text}
          </p>
        ))}

      {requestLine !== null && <p className="ghi-chu">{requestLine}</p>}

      <h4>Lượt chạy gần nhất</h4>
      {job.last_runs.length === 0 ? (
        <p className="trang-thai-rong">{NO_RUNS_YET}</p>
      ) : (
        <div className="bang-cuon" role="region" aria-label={`Lượt chạy gần nhất — ${words.title}`} tabIndex={0}>
          <table className="bang-danh-muc">
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
        </div>
      )}
    </article>
  );
}
