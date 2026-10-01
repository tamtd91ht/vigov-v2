"use client";

import { useEffect, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { KetQua } from "@/lib/api/goi";
import {
  getPortalCategories,
  getPortalSyncSettings,
  listPortalSyncRuns,
  savePortalCategories,
  savePortalSyncSettings,
  startPortalSyncRun,
  type CategoryTreeResult,
  type StartRunResult,
} from "@/lib/api/portal-sync";
import type {
  comms_portalCategoriesIn,
  comms_portalCategoriesOut,
  comms_portalRunOut,
  comms_portalSyncSettingsIn,
  comms_portalSyncSettingsOut,
  page_Result_comms_portalRunOut,
} from "@/lib/api/schema.gen";

import { DAU_GACH, nhanMoc } from "./nhan-noi-dung";
import {
  API_URL_HINT,
  categoriesBody,
  CATEGORIES_EMPTY,
  CATEGORIES_LIMIT_REACHED,
  CATEGORIES_LOADING,
  CATEGORIES_NEED_SETTINGS,
  CATEGORIES_NEED_UPDATE,
  CATEGORIES_NOTHING_CHANGED,
  CATEGORIES_SAVED,
  choicesFromTree,
  CONFIGURE_LABEL,
  DEFAULT_TARGET_KIND,
  formFromSettings,
  HISTORY_LABEL,
  INTERVAL_OPTIONS,
  intervalLabel,
  keyHint,
  keyRequirement,
  LOADING_SETTINGS,
  MAX_ITEMS_HINT,
  MAX_ITEMS_PER_RUN_MAX,
  MISSING_ON_PORTAL,
  NO_RUN_YET,
  orderAsTree,
  outcomeChipClass,
  outcomeLabel,
  POLL_GAVE_UP,
  POLL_INTERVAL_MS,
  POLL_MAX_ATTEMPTS,
  PORTAL_SYNC_DESCRIPTION,
  PORTAL_SYNC_TITLE,
  portalSyncStatus,
  portalSyncStatusChipClass,
  portalSyncStatusExplainer,
  portalSyncStatusLabel,
  PUBLISH_MODE_OPTIONS,
  PUBLISH_MODE_REVIEW,
  publishModeLabel,
  RELOAD_LABEL,
  retryWaitLabel,
  RUN_NOW_LABEL,
  RUN_STARTED,
  runCountsLine,
  runErrorLine,
  runIsUnfinished,
  runNowBlockedReason,
  SELECTED_CATEGORIES_MAX,
  selectAllUpToLimit,
  selectedCount,
  selectedCountLabel,
  SETTINGS_SAVED,
  settingsBody,
  settingsFormError,
  SHOW_PENDING_LABEL,
  TARGET_KIND_OPTIONS,
  triggerLabel,
  WINDOW_DAYS_HINT,
  WINDOW_DAYS_MAX,
  type CategoryChoice,
  type SettingsForm,
} from "./portal-sync";

/**
 * §3 — the card "Đồng bộ tin từ Cổng thông tin điện tử" (ADR 0067 §2): chip, meta line, last run,
 * error block, `⟳ Đồng bộ ngay`, `Cấu hình`, and the run history.
 *
 * THE KEY NEVER COMES BACK. The settings response says `api_key_set` and nothing more; the key box is
 * empty on every open, and its text lives only in the form's state until the PUT, then is dropped.
 *
 * THE CATEGORY TREE IS ASKED ONLY WHEN `Cấu hình` IS OPEN. Each read is an outbound call to the commune's
 * portal (ADR 0067 §2: never copied), so the card's meta line does not show §3's `{n} chuyên mục` — see
 * `PHAN_CHUA_DUNG`. The count is shown inside the form, from the live tree.
 *
 * NO PERMISSION GATE ON THE CLIENT, same as the rest of this screen: `service-comms` checks
 * `content.read` / `content.update` on every call and a 403 sentence reaches the card verbatim (rule 5,
 * forbidden #1). The category tree is a GET under `content.update` (owner 02/10/2026, D3): an account
 * with `content.read` only opens `Cấu hình`, the tree's call answers 403, and the picker alone says which
 * right is missing — the settings, the history and the run button are untouched.
 *
 * `Cấu hình` IS AN IN-PAGE SECTION, NOT AN OVERLAY — the same reason as §7's form (`PHAN_CHUA_DUNG`,
 * item `Bố cục §2`).
 */

/** The six calls, injectable so the tests press the buttons against fakes. */
export type PortalSyncApi = {
  getSettings: () => Promise<KetQua<comms_portalSyncSettingsOut>>;
  saveSettings: (b: comms_portalSyncSettingsIn) => Promise<KetQua<comms_portalSyncSettingsOut>>;
  getCategories: () => Promise<CategoryTreeResult>;
  saveCategories: (b: comms_portalCategoriesIn) => Promise<KetQua<comms_portalCategoriesOut>>;
  listRuns: () => Promise<KetQua<page_Result_comms_portalRunOut>>;
  startRun: (idempotencyKey: string) => Promise<StartRunResult>;
};

export const PORTAL_SYNC_API: PortalSyncApi = {
  getSettings: getPortalSyncSettings,
  saveSettings: savePortalSyncSettings,
  getCategories: getPortalCategories,
  saveCategories: savePortalCategories,
  listRuns: listPortalSyncRuns,
  startRun: startPortalSyncRun,
};

export function PortalSyncCard({
  api = PORTAL_SYNC_API,
  pollIntervalMs = POLL_INTERVAL_MS,
  showPendingReview,
}: {
  api?: PortalSyncApi;
  pollIntervalMs?: number;
  /** Sets §6's `Trạng thái` filter to `Chờ duyệt`. Absent = no link (the card alone, in a test). */
  showPendingReview?: () => void;
}) {
  const [settings, setSettings] = useState<KetQua<comms_portalSyncSettingsOut> | null>(null);
  const [runs, setRuns] = useState<KetQua<page_Result_comms_portalRunOut> | null>(null);
  const [reload, setReload] = useState(0);
  const [configOpen, setConfigOpen] = useState(false);

  // ONE KEY PER PRESS: a double click, or a retry after a lost answer, reuses it and `core/idem` turns
  // it into one run. Renewed only after a 202 — a refused attempt is released by the server.
  const [runKey, setRunKey] = useState(khoaChongTrungMoi);
  const [starting, setStarting] = useState(false);
  // `wait`: the Retry-After sentence of a 503 portal_sync_busy, under the server's own sentence.
  const [startMessage, setStartMessage] = useState<{ ok: boolean; text: string; wait?: string } | null>(null);
  // `left` reads still owed after a 202; `null` = not polling. Reaching 0 with the run unfinished
  // shows POLL_GAVE_UP.
  const [poll, setPoll] = useState<{ left: number } | null>(null);

  useEffect(() => {
    let gone = false;
    api.getSettings().then((r) => {
      if (!gone) setSettings(r);
    });
    api.listRuns().then((r) => {
      if (!gone) setRuns(r);
    });
    return () => {
      gone = true;
    };
  }, [api, reload]);

  useEffect(() => {
    if (poll === null || poll.left <= 0) return;
    let gone = false;
    const t = setTimeout(() => {
      api.listRuns().then((r) => {
        if (gone) return;
        setRuns(r);
        if (r.ok && !runIsUnfinished(r.duLieu.items[0])) {
          setPoll(null);
          setStartMessage(null);
          // `last_run_at` moved: re-read the settings line too.
          api.getSettings().then((s) => {
            if (!gone) setSettings(s);
          });
        } else {
          setPoll({ left: poll.left - 1 });
        }
      });
    }, pollIntervalMs);
    return () => {
      gone = true;
      clearTimeout(t);
    };
  }, [api, poll, pollIntervalMs]);

  const latest: comms_portalRunOut | undefined = runs !== null && runs.ok ? runs.duLieu.items[0] : undefined;

  function runNow(): void {
    setStarting(true);
    setStartMessage(null);
    api.startRun(runKey).then((r) => {
      setStarting(false);
      if (!r.ok) {
        // 409 portal_sync_in_progress / not_configured, 503 — the server's sentence, verbatim. On 503
        // portal_sync_busy the Retry-After wait is said too — and NOTHING is scheduled: the officer
        // presses again (same key, released by the server).
        setStartMessage(
          r.retryAfterSeconds === undefined
            ? { ok: false, text: r.thongBao }
            : { ok: false, text: r.thongBao, wait: retryWaitLabel(r.retryAfterSeconds) },
        );
        return;
      }
      setRunKey(khoaChongTrungMoi());
      setStartMessage({ ok: true, text: RUN_STARTED });
      // Read once now (the new run row disables the button), then a bounded number of times.
      api.listRuns().then(setRuns);
      setPoll({ left: POLL_MAX_ATTEMPTS });
    });
  }

  function reloadAll(): void {
    setPoll(null);
    setStartMessage(null);
    setReload((n) => n + 1);
  }

  const s = settings !== null && settings.ok ? settings.duLieu : null;
  const status = s === null ? null : portalSyncStatus(s);
  const blocked = s === null ? null : runNowBlockedReason(s, latest);
  const runDisabled = s === null || starting || blocked !== null || runs === null;

  return (
    <section className="khoi-chi-tiet portal-sync-card" aria-labelledby="portal-sync-title">
      <div className="dau-khoi-chi-tiet">
        <h3 id="portal-sync-title">🔗 {PORTAL_SYNC_TITLE}</h3>
        <div className="cum-nut">
          <button type="button" className="nut-chinh" disabled={runDisabled} onClick={runNow}>
            {RUN_NOW_LABEL}
          </button>
          <button
            type="button"
            className="nut-phu"
            aria-expanded={configOpen}
            aria-controls="portal-sync-config"
            disabled={s === null}
            onClick={() => setConfigOpen(!configOpen)}
          >
            {CONFIGURE_LABEL}
          </button>
          <button type="button" className="nut-phu" onClick={reloadAll}>
            {RELOAD_LABEL}
          </button>
        </div>
      </div>

      {settings === null && <p role="status">{LOADING_SETTINGS}</p>}
      {settings !== null && !settings.ok && (
        <p className="thong-bao-loi" role="alert">
          {settings.thongBao}
        </p>
      )}

      {s !== null && status !== null && (
        <>
          <p>
            <span className={portalSyncStatusChipClass(status)} data-testid="portal-sync-status">
              {portalSyncStatusLabel(status)}
            </span>
            {s.configured && (
              <>
                {" "}
                · {intervalLabel(s.interval_hours)} · {publishModeLabel(s.publish_mode)}
              </>
            )}
          </p>
          {portalSyncStatusExplainer(status) !== "" && <p className="ghi-chu">{portalSyncStatusExplainer(status)}</p>}
          {/* Review mode puts every import in `Chờ duyệt`; the way to that queue is one press. No count:
              the list route has no total, and a number read off one page would be wrong. */}
          {showPendingReview !== undefined && s.configured && s.publish_mode === PUBLISH_MODE_REVIEW && (
            <p>
              <button type="button" className="nut-phu" onClick={showPendingReview}>
                {SHOW_PENDING_LABEL}
              </button>
            </p>
          )}
        </>
      )}

      {runs !== null && !runs.ok && (
        <p className="thong-bao-loi" role="alert">
          {runs.thongBao}
        </p>
      )}
      {runs !== null && runs.ok && <LastRun run={latest} />}

      {blocked !== null && s !== null && (
        <p className="ghi-chu" id="portal-sync-run-blocked">
          {blocked}
        </p>
      )}
      {startMessage !== null && (
        <p role={startMessage.ok ? "status" : "alert"} className={startMessage.ok ? undefined : "thong-bao-loi"}>
          {startMessage.text}
          {startMessage.wait !== undefined && (
            <span className="dong-phu" data-testid="portal-sync-retry-wait">
              {startMessage.wait}
            </span>
          )}
        </p>
      )}
      {poll !== null && poll.left <= 0 && <p role="status">{POLL_GAVE_UP}</p>}

      {runs !== null && runs.ok && runs.duLieu.items.length > 0 && <RunHistory runs={runs.duLieu.items} />}

      {configOpen && s !== null && (
        <PortalSyncConfig
          settings={s}
          api={api}
          close={() => setConfigOpen(false)}
          saved={(next) => setSettings({ ok: true, duLieu: next })}
        />
      )}
    </section>
  );
}

/** §3's `Chạy lần cuối {HH:mm dd/MM/yyyy}` + outcome + counts, and the red error block. */
export function LastRun({ run }: { run: comms_portalRunOut | undefined }) {
  if (run === undefined) return <p className="ghi-chu">{NO_RUN_YET}</p>;
  return (
    <div data-testid="portal-sync-last-run">
      <p>
        Chạy lần cuối {nhanMoc(run.started_at)}{" "}
        <span className={outcomeChipClass(run.outcome)}>{outcomeLabel(run.outcome)}</span>
        {!runIsUnfinished(run) && <> · {runCountsLine(run)}</>}
      </p>
      <RunErrors run={run} />
    </div>
  );
}

function RunErrors({ run }: { run: comms_portalRunOut }) {
  if (run.error_summary.length === 0) return null;
  return (
    <ul className="thong-bao-loi" aria-label="Lỗi của lượt đồng bộ">
      {run.error_summary.map((e, i) => (
        <li key={`${e.category_external_id}|${e.error}|${i}`}>{runErrorLine(e)}</li>
      ))}
    </ul>
  );
}

/** `Lịch sử đồng bộ` — the newest page of runs, collapsed by default. */
export function RunHistory({ runs }: { runs: readonly comms_portalRunOut[] }) {
  return (
    <details>
      <summary>
        {HISTORY_LABEL} ({runs.length} lượt gần nhất)
      </summary>
      <div className="bang-cuon">
        <table className="bang-can-bo">
          <caption className="an-thi-giac">{HISTORY_LABEL}</caption>
          <thead>
            <tr>
              <th scope="col">Bắt đầu</th>
              <th scope="col">Kiểu chạy</th>
              <th scope="col">Người chạy</th>
              <th scope="col">Kết quả</th>
              <th scope="col">Số tin</th>
              <th scope="col">Lỗi</th>
            </tr>
          </thead>
          <tbody>
            {runs.map((r) => (
              <tr key={r.id}>
                <td>
                  {nhanMoc(r.started_at)}
                  {r.finished_at !== null && <span className="dong-phu">xong {nhanMoc(r.finished_at)}</span>}
                </td>
                <td>{triggerLabel(r.trigger_kind)}</td>
                {/* `system` or the staff BUSINESS code (rule 6, invariant 8) — no name, no internal id. */}
                <td>{r.actor === "" ? DAU_GACH : r.actor === "system" ? "Hệ thống" : r.actor}</td>
                <td>
                  <span className={outcomeChipClass(r.outcome)}>{outcomeLabel(r.outcome)}</span>
                </td>
                <td>{runIsUnfinished(r) ? DAU_GACH : `đọc ${r.fetched_count} · ${runCountsLine(r)}`}</td>
                <td>
                  {r.error_summary.length === 0 ? (
                    DAU_GACH
                  ) : (
                    <ul>
                      {r.error_summary.map((e, i) => (
                        <li key={`${e.category_external_id}|${e.error}|${i}`}>{runErrorLine(e)}</li>
                      ))}
                    </ul>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </details>
  );
}

/** The `Cấu hình` section: the settings form, then the category picker (saved separately). */
export function PortalSyncConfig({
  settings,
  api,
  close,
  saved,
}: {
  settings: comms_portalSyncSettingsOut;
  api: PortalSyncApi;
  close: () => void;
  saved: (s: comms_portalSyncSettingsOut) => void;
}) {
  // Counts successful saves. It keys the form, so each save rebuilds it from the server's answer: the
  // key box empties and the hints follow `api_key_set`. A failed save keeps the form — and the typed
  // key — so the officer does not paste it twice after fixing the address.
  const [saves, setSaves] = useState(0);
  return (
    <div id="portal-sync-config">
      {saves > 0 && <p role="status">{SETTINGS_SAVED}</p>}
      <SettingsFormView
        key={saves}
        settings={settings}
        api={api}
        close={close}
        saved={(next) => {
          setSaves((n) => n + 1);
          saved(next);
        }}
      />
      {settings.configured ? (
        // Re-asked after each save: a new address is a different portal.
        <CategoryPicker api={api} key={`${saves}|${settings.api_url}`} />
      ) : (
        <p className="ghi-chu">{CATEGORIES_NEED_SETTINGS}</p>
      )}
    </div>
  );
}

function SettingsFormView({
  settings,
  api,
  close,
  saved,
}: {
  settings: comms_portalSyncSettingsOut;
  api: PortalSyncApi;
  close: () => void;
  saved: (s: comms_portalSyncSettingsOut) => void;
}) {
  const [form, setForm] = useState<SettingsForm>(() => formFromSettings(settings));
  const [sending, setSending] = useState(false);
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);

  const formError = settingsFormError(settings, form);
  const keyRequired = keyRequirement(settings, form) !== null;
  // Without the platform key the server answers 503 and writes nothing — the button says so up front.
  const canSave = formError === null && settings.encryption_configured && !sending;

  function submit(e: FormEvent): void {
    e.preventDefault();
    if (!canSave) return;
    setSending(true);
    api.saveSettings(settingsBody(form)).then((r) => {
      setSending(false);
      if (!r.ok) {
        // 422 api_key_required(_for_new_url), 400 url rule, 503 encryption — verbatim. The typed key
        // stays in the box so the officer does not paste it twice after fixing the address.
        setMessage({ ok: false, text: r.thongBao });
        return;
      }
      saved(r.duLieu);
    });
  }

  return (
    <form className="form-danh-muc" onSubmit={submit} aria-labelledby="portal-sync-form-title">
      <h4 id="portal-sync-form-title">{PORTAL_SYNC_TITLE}</h4>
      <p className="ghi-chu">{PORTAL_SYNC_DESCRIPTION}</p>

      <div className="o-nhap">
        <label htmlFor="portal-sync-api-url">Địa chỉ API của Cổng *</label>
        <input
          id="portal-sync-api-url"
          name="portal-sync-api-url"
          type="url"
          inputMode="url"
          value={form.api_url}
          placeholder="https://"
          autoComplete="off"
          maxLength={2048}
          aria-describedby="portal-sync-api-url-hint"
          onChange={(e) => setForm({ ...form, api_url: e.target.value })}
        />
        <p className="ghi-chu" id="portal-sync-api-url-hint">
          {API_URL_HINT}
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="portal-sync-api-key">Mã bảo mật{keyRequired ? " *" : ""}</label>
        {/* WRITE-ONLY: never prefilled, `new-password` so no browser offers a saved password here. */}
        <input
          id="portal-sync-api-key"
          name="portal-sync-api-key"
          type="password"
          value={form.api_key}
          placeholder={keyRequired ? "" : "Giữ nguyên mã cũ"}
          autoComplete="new-password"
          spellCheck={false}
          maxLength={512}
          aria-describedby="portal-sync-api-key-hint"
          onChange={(e) => setForm({ ...form, api_key: e.target.value })}
        />
        <p className="ghi-chu" id="portal-sync-api-key-hint" data-testid="portal-sync-key-hint">
          {keyHint(settings, form)}
        </p>
      </div>

      <fieldset className="o-nhap">
        <legend>Chế độ đăng</legend>
        {PUBLISH_MODE_OPTIONS.map((o) => (
          <div key={o.value}>
            <label htmlFor={`portal-sync-mode-${o.value}`}>
              <input
                id={`portal-sync-mode-${o.value}`}
                type="radio"
                name="portal-sync-mode"
                value={o.value}
                checked={form.publish_mode === o.value}
                onChange={() => setForm({ ...form, publish_mode: o.value })}
              />{" "}
              {o.label}
            </label>
            <p className="ghi-chu">{o.explainer}</p>
          </div>
        ))}
      </fieldset>

      <div className="o-chon">
        <label htmlFor="portal-sync-interval">Nhịp đồng bộ</label>
        <select
          id="portal-sync-interval"
          value={String(form.interval_hours)}
          onChange={(e) => setForm({ ...form, interval_hours: Number.parseInt(e.target.value, 10) })}
        >
          {INTERVAL_OPTIONS.map((h) => (
            <option key={h} value={String(h)}>
              {intervalLabel(h)}
            </option>
          ))}
        </select>
      </div>

      <div className="o-nhap">
        <label htmlFor="portal-sync-window">Lấy tin đăng trong bao nhiêu ngày gần nhất</label>
        <input
          id="portal-sync-window"
          type="number"
          inputMode="numeric"
          min={1}
          max={WINDOW_DAYS_MAX}
          step={1}
          value={form.window_days}
          aria-describedby="portal-sync-window-hint"
          onChange={(e) => setForm({ ...form, window_days: e.target.value })}
        />
        <p className="ghi-chu" id="portal-sync-window-hint">
          {WINDOW_DAYS_HINT}
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="portal-sync-max-items">Số tin tối đa mỗi lượt</label>
        <input
          id="portal-sync-max-items"
          type="number"
          inputMode="numeric"
          min={1}
          max={MAX_ITEMS_PER_RUN_MAX}
          step={1}
          value={form.max_items_per_run}
          aria-describedby="portal-sync-max-items-hint"
          onChange={(e) => setForm({ ...form, max_items_per_run: e.target.value })}
        />
        <p className="ghi-chu" id="portal-sync-max-items-hint">
          {MAX_ITEMS_HINT}
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="portal-sync-credit">
          <input
            id="portal-sync-credit"
            type="checkbox"
            checked={form.keep_source_credit}
            onChange={(e) => setForm({ ...form, keep_source_credit: e.target.checked })}
          />{" "}
          Giữ dòng ghi nguồn “Nguồn: …” của Cổng trong bài
        </label>
      </div>

      <div className="o-nhap">
        <label htmlFor="portal-sync-enabled">
          <input
            id="portal-sync-enabled"
            type="checkbox"
            checked={form.is_enabled}
            onChange={(e) => setForm({ ...form, is_enabled: e.target.checked })}
          />{" "}
          Bật đồng bộ theo lịch
        </label>
      </div>

      {!settings.encryption_configured && (
        <p className="thong-bao-loi" role="alert">
          {portalSyncStatusExplainer("missing-encryption")}
        </p>
      )}
      {formError !== null && (
        <p className="ghi-chu" data-testid="portal-sync-form-error">
          {formError}
        </p>
      )}
      {message !== null && (
        <p role={message.ok ? "status" : "alert"} className={message.ok ? undefined : "thong-bao-loi"}>
          {message.text}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={sending} onClick={close}>
          Đóng
        </button>
        <button type="submit" className="nut-chinh" disabled={!canSave}>
          Lưu cấu hình
        </button>
      </div>
    </form>
  );
}

/** The live category tree with ticks and a kind per ticked category; `Lưu chuyên mục` saves only changes. */
export function CategoryPicker({ api }: { api: PortalSyncApi }) {
  const [loaded, setLoaded] = useState<CategoryTreeResult | null>(null);
  const [initial, setInitial] = useState<CategoryChoice[]>([]);
  const [choices, setChoices] = useState<CategoryChoice[]>([]);
  const [sending, setSending] = useState(false);
  const [message, setMessage] = useState<{ ok: boolean; text: string } | null>(null);

  useEffect(() => {
    let gone = false;
    api.getCategories().then((r) => {
      if (gone) return;
      setLoaded(r);
      if (r.ok) {
        const c = choicesFromTree(r.duLieu);
        setInitial(c);
        setChoices(c);
      }
    });
    return () => {
      gone = true;
    };
  }, [api]);

  function update(id: string, change: Partial<CategoryChoice>): void {
    setMessage(null);
    setChoices((cs) => cs.map((c) => (c.external_id === id ? { ...c, ...change } : c)));
  }

  function tick(c: CategoryChoice, selected: boolean): void {
    update(c.external_id, {
      selected,
      target_kind: selected && c.target_kind === "" ? DEFAULT_TARGET_KIND : c.target_kind,
    });
  }

  function setAll(selected: boolean): void {
    setMessage(null);
    // Ticking all stops at the ceiling (30); unticking all clears the portal's rows.
    setChoices((cs) =>
      selected
        ? selectAllUpToLimit(cs, DEFAULT_TARGET_KIND)
        : cs.map((c) => (c.on_portal ? { ...c, selected: false } : c)),
    );
  }

  function save(e: FormEvent): void {
    e.preventDefault();
    const body = categoriesBody(initial, choices);
    if (body.categories.length === 0) {
      setMessage({ ok: false, text: CATEGORIES_NOTHING_CHANGED });
      return;
    }
    setSending(true);
    api.saveCategories(body).then((r) => {
      setSending(false);
      if (!r.ok) {
        setMessage({ ok: false, text: r.thongBao });
        return;
      }
      // What was just saved is the new baseline: a second press sends nothing.
      setInitial(choices);
      setMessage({ ok: true, text: CATEGORIES_SAVED });
    });
  }

  if (loaded === null) return <p role="status">{CATEGORIES_LOADING}</p>;
  if (!loaded.ok) {
    // 409 not configured, 502 portal_<class> — the server's sentence, which never quotes the portal.
    // 403: the same sentence, plus which right the tree needs (D3). Only this block; the form above stays.
    return (
      <div data-testid="portal-sync-categories-refused">
        {loaded.forbidden === true && <p className="ghi-chu">{CATEGORIES_NEED_UPDATE}</p>}
        <p className="thong-bao-loi" role="alert">
          {loaded.thongBao}
        </p>
      </div>
    );
  }

  const rows = orderAsTree(choices);
  // A HINT, not the rule: unticked boxes go off at 30 and the server's 422 is what refuses.
  const atLimit = selectedCount(choices) >= SELECTED_CATEGORIES_MAX;

  return (
    <form className="form-danh-muc" onSubmit={save} aria-labelledby="portal-sync-categories-title">
      <h4 id="portal-sync-categories-title">
        Chuyên mục lấy về <span className="dong-phu">{selectedCountLabel(choices)}</span>
      </h4>
      <p className="ghi-chu">
        Danh sách hỏi trực tiếp Cổng mỗi lần mở cấu hình. Mỗi chuyên mục đã chọn về sổ thành một loại nội
        dung: Tin tức, Sự kiện hoặc Thông báo.
      </p>

      {rows.length === 0 && <p className="trang-thai-rong">{CATEGORIES_EMPTY}</p>}
      {atLimit && (
        <p className="ghi-chu" role="status" data-testid="portal-sync-categories-limit">
          {CATEGORIES_LIMIT_REACHED}
        </p>
      )}

      {rows.length > 0 && (
        <>
          <div className="cum-nut">
            <button type="button" className="nut-phu" disabled={atLimit} onClick={() => setAll(true)}>
              Chọn tất cả
            </button>
            <button type="button" className="nut-phu" onClick={() => setAll(false)}>
              Bỏ chọn
            </button>
          </div>
          <ul className="category-list" aria-label="Cây chuyên mục của Cổng">
            {rows.map(({ choice: c, depth }) => {
              const tickId = `portal-category-${c.external_id}`;
              return (
                // The indent is the tree's depth, so it cannot be a class; same shape as `bang-thu-chi.tsx`.
                <li
                  key={c.external_id}
                  className="category-row"
                  style={{ paddingInlineStart: `${depth * 1.5}rem` }}
                >
                  <label htmlFor={tickId}>
                    <input
                      id={tickId}
                      type="checkbox"
                      checked={c.selected}
                      disabled={!c.selected && atLimit}
                      onChange={(e) => tick(c, e.target.checked)}
                    />{" "}
                    {c.name}
                  </label>
                  {!c.on_portal && <span className="chip chip-ngung"> {MISSING_ON_PORTAL}</span>}
                  {c.selected && (
                    <select
                      aria-label={`Loại nội dung của chuyên mục ${c.name}`}
                      value={c.target_kind === "" ? DEFAULT_TARGET_KIND : c.target_kind}
                      onChange={(e) => update(c.external_id, { target_kind: e.target.value })}
                    >
                      {TARGET_KIND_OPTIONS.map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </select>
                  )}
                </li>
              );
            })}
          </ul>
        </>
      )}

      {message !== null && (
        <p role={message.ok ? "status" : "alert"} className={message.ok ? undefined : "thong-bao-loi"}>
          {message.text}
        </p>
      )}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={sending || rows.length === 0}>
          Lưu chuyên mục
        </button>
      </div>
    </form>
  );
}
