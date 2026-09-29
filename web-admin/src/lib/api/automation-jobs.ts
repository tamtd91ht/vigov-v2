/**
 * "Cấu hình → Tự động hoá" (`docs/ui-ux/14-cau-hinh.md §9`, ADR 0058) — three routes of
 * `service-identity`, all `admin.sla`:
 *
 *   GET  /api/v1/automation-jobs              the three jobs, their settings, last run per kind of work
 *   PUT  /api/v1/automation-jobs/{job}        save one job's switch and cadence → 200 the job
 *   POST /api/v1/automation-jobs/{job}/runs   "Chạy ngay" — MARKS the job; 202 the job, nothing ran yet
 *
 * NO Idempotency-Key: neither write declares one. PUT sets an absolute state, and a repeated "run now"
 * press only moves the same job-level mark (ADR 0058 §7) — both land in the same state when repeated.
 *
 * REFUSALS ARE THE SERVER'S SENTENCES, verbatim (`goi.ts`): 400 names the bound that was broken, 409
 * `automation_job_disabled` says to switch the job on first, 409 `automation_job_changed` says someone
 * else saved it. No branch on `code` here or on the screen.
 *
 * NO `tenant_id`, RELATIVE PATHS, `credentials: same-origin`: the three rules of `goi.ts`.
 */

import { docJSON, docThanLoiGoi, goiGhi } from "./request";
import type { KetQua } from "./request";
import type {
  identity_automationJobOut,
  identity_automationJobsOut,
  identity_automationSettingIn,
  identity_get_automation_jobs,
  identity_post_automation_jobs_by_job_runs,
  identity_put_automation_jobs_by_job,
} from "./schema.gen";

export type AutomationJob = identity_automationJobOut;
export type AutomationSetting = identity_automationSettingIn;

const LIST_PATH = "/api/v1/automation-jobs" satisfies identity_get_automation_jobs["duongDan"];
const JOB_TEMPLATE = "/api/v1/automation-jobs/{job}" satisfies identity_put_automation_jobs_by_job["duongDan"];
const RUNS_TEMPLATE =
  "/api/v1/automation-jobs/{job}/runs" satisfies identity_post_automation_jobs_by_job_runs["duongDan"];

function withJob(template: string, job: string): string {
  return template.replace("{job}", encodeURIComponent(job));
}

export async function listAutomationJobs(): Promise<KetQua<readonly AutomationJob[]>> {
  const r = await docJSON<identity_automationJobsOut>(LIST_PATH);
  if (!r.ok) return r;
  return { ok: true, duLieu: Array.isArray(r.duLieu.items) ? r.duLieu.items : [] };
}

/** Save one job. The body is built by `settingBody` (`features/cau-hinh/automation-form.ts`). */
export function saveAutomationJob(job: string, body: AutomationSetting): Promise<KetQua<AutomationJob>> {
  return docThanLoiGoi<AutomationJob>(goiGhi(withJob(JOB_TEMPLATE, job), "PUT", body, 200));
}

/** "Chạy ngay". No body. 202 carries the job with its new `run_requested_at`. */
export function requestAutomationRun(job: string): Promise<KetQua<AutomationJob>> {
  return docThanLoiGoi<AutomationJob>(goiGhi(withJob(RUNS_TEMPLATE, job), "POST", undefined, 202));
}
