"use client";

import { CheckCircle2, Link2, LoaderCircle, RefreshCw, TriangleAlert, X } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { PendingMarker } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import type { KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";
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

import {
  CLOSE_LABEL,
  DAU_GACH,
  DOWNLOAD_IMAGES_PART,
  formatVietnamDateTime,
  INTERVAL_15_MIN_PART,
  KEY_IN_USE_PART,
  pendingContentPart,
  PORTAL_CATEGORY_COUNT_PART,
} from "./nhan-noi-dung";
import {
  API_URL_PLACEHOLDER,
  CANCEL_LABEL,
  categoriesBody,
  CATEGORIES_EMPTY,
  CATEGORIES_LIMIT_REACHED,
  CATEGORIES_LOADING,
  CATEGORIES_NEED_SETTINGS,
  CATEGORIES_NEED_UPDATE,
  CATEGORIES_TITLE,
  CATEGORIES_UNREACHABLE,
  choicesFromTree,
  CONFIGURE_LABEL,
  CONNECT_LABEL,
  DEFAULT_TARGET_KIND,
  DOWNLOAD_IMAGES_NOTE,
  ENABLED_LABEL,
  formFromSettings,
  groupChoices,
  INTERVAL_15_MIN_LABEL,
  intervalLabel,
  intervalOptions,
  KEEP_SOURCE_LABEL,
  KEY_IN_USE_PREFIX,
  KEY_KEEP_PLACEHOLDER,
  KEY_SAVED_HINT,
  keyHint,
  keyRequirement,
  LOADING_SETTINGS,
  MAX_ITEMS_HINT,
  MAX_ITEMS_PER_RUN_MAX,
  NOT_CONNECTED,
  outcomeLabel,
  pickMany,
  PORTAL_SYNC_DESCRIPTION,
  PORTAL_SYNC_TITLE,
  portalSyncStatusExplainer,
  PUBLISH_DIRECT_LABEL,
  PUBLISH_MODE_DIRECT,
  PUBLISH_MODE_REVIEW,
  publishModeLabel,
  REFETCH_DELAYS_MS,
  retryWaitLabel,
  RUN_NOW_LABEL,
  RUN_REFUSED_TOAST,
  RUN_STARTED_TOAST,
  runErrorLine,
  runIsUnfinished,
  runNowBlockedReason,
  SAVE_CONFIG_LABEL,
  SELECTED_CATEGORIES_MAX,
  selectAllUpToLimit,
  selectedCount,
  selectedCountLabel,
  SETTINGS_SAVED,
  settingsBody,
  settingsFormError,
  skippedCount,
  TARGET_KIND_OPTIONS,
  WINDOW_DAYS_HINT,
  WINDOW_DAYS_MAX,
  type CategoryChoice,
  type SettingsField,
  type SettingsForm,
} from "./portal-sync";
import { OverlayDialog } from "./overlay-dialog";

/**
 * §3 — the card "Đồng bộ tin từ Cổng thông tin điện tử" (ADR 0067 §2), drawn as the prototype's
 * `ContentSourcePanel` (`vigov-require/apps/admin/src/components/content/ContentSourcePanel.tsx:92-180`):
 * the title with its link icon, ONE status line (`Đang bật · {n} chuyên mục · Mỗi 6 giờ · chờ duyệt`), the
 * last run's line, and on the right `Đồng bộ ngay` (outline, only once connected) and `Cấu hình` /
 * `Nối Cổng thông tin` (primary). The run history, `Tải lại`, the pending-queue link, the state's
 * explainer sentence and the result chip are gone (owner D3, 09/10/2026).
 *
 * THE KEY NEVER COMES BACK. The settings response says `api_key_set` and nothing more; the key box is
 * empty on every open, and its text lives only in the form's state until the PUT, then is dropped.
 *
 * THE CATEGORY TREE IS ASKED ONLY WHEN `Cấu hình` IS OPEN. Each read is an outbound call to the commune's
 * portal (ADR 0067 §2: never copied), so the status line's `{n} chuyên mục` holds a disabled "?" where the
 * number would be (`PHAN_CHUA_DUNG`, ADR 0068 §14). The count is shown inside the form, from the live tree.
 *
 * `⟳ Đồng bộ ngay` AND `Cấu hình` ARE DRAWN ONLY WITH `content.update` (`canEdit`, as the prototype's
 * `ContentSourcePanel canEdit`): every call behind them declares that key — the run, both PUTs, and the
 * category tree, a GET under the write key (owner 02/10/2026, D3). Hiding is convenience: `service-comms`
 * checks the key on every call and a 403 sentence reaches the card verbatim (rule 5, forbidden #1).
 *
 * AFTER A 202 THE CARD RE-READS ITSELF THREE TIMES (`REFETCH_DELAYS_MS`: 4 s, 15 s, 45 s), then stops — the
 * toast tells the officer to reopen the screen later for a longer run. Never an open-ended poll.
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

/**
 * The config dialog's width — the prototype's `sm:max-w-[46rem]` (`ContentSourcePanel.tsx:315`), never
 * wider than the screen less a 0.5rem margin each side. `p-5` as the content editor's dialog.
 */
const CONFIG_DIALOG_CLASS = "w-[min(46rem,calc(100vw-1rem))] p-5";

/** The prototype's checkbox rows (`ContentSourcePanel.tsx:521`) and their native box. */
const CHECK_ROW = "flex items-center gap-2.5 text-[12.5px]";
const CHECKBOX = "accent-brand m-0 size-3.5 shrink-0";

export function PortalSyncCard({
  api = PORTAL_SYNC_API,
  refetchDelaysMs = REFETCH_DELAYS_MS,
  canEdit,
}: {
  api?: PortalSyncApi;
  /** When the card re-reads itself after a 202. Tests pass shorter ones. */
  refetchDelaysMs?: readonly number[];
  /** `content.update` held (`canEditContent`). Required: no default, so no caller forgets to decide. */
  canEdit: boolean;
}) {
  const [settings, setSettings] = useState<KetQua<comms_portalSyncSettingsOut> | null>(null);
  const [runs, setRuns] = useState<KetQua<page_Result_comms_portalRunOut> | null>(null);
  const [configOpen, setConfigOpen] = useState(false);

  // ONE KEY PER PRESS: a double click, or a retry after a lost answer, reuses it and `core/idem` turns
  // it into one run. Renewed only after a 202 — a refused attempt is released by the server.
  const [runKey, setRunKey] = useState(khoaChongTrungMoi);
  const [starting, setStarting] = useState(false);
  // Counts the 202s: each one schedules its own three re-reads, and a new press cancels the old ones.
  const [startedRuns, setStartedRuns] = useState(0);

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
  }, [api]);

  useEffect(() => {
    if (startedRuns === 0) return;
    let gone = false;
    const timers = refetchDelaysMs.map((ms) =>
      setTimeout(() => {
        // `last_run_at` and the newest run both move when a run ends: the whole card is read again.
        api.getSettings().then((r) => {
          if (!gone) setSettings(r);
        });
        api.listRuns().then((r) => {
          if (!gone) setRuns(r);
        });
      }, ms),
    );
    return () => {
      gone = true;
      timers.forEach(clearTimeout);
    };
  }, [api, startedRuns, refetchDelaysMs]);

  const latest: comms_portalRunOut | undefined = runs !== null && runs.ok ? runs.duLieu.items[0] : undefined;

  function runNow(): void {
    setStarting(true);
    api.startRun(runKey).then((r) => {
      setStarting(false);
      if (!r.ok) {
        // 409 portal_sync_in_progress / not_configured, 503 — the prototype's toast, with the server's own
        // sentence under it. On 503 portal_sync_busy the Retry-After wait is said too — and NOTHING is
        // scheduled: the officer presses again (same key, released by the server).
        const wait = r.retryAfterSeconds === undefined ? "" : ` ${retryWaitLabel(r.retryAfterSeconds)}`;
        toast.error(RUN_REFUSED_TOAST, { description: `${r.thongBao}${wait}` });
        return;
      }
      setRunKey(khoaChongTrungMoi());
      toast.success(RUN_STARTED_TOAST);
      // Read once now: the new run row disables the button at once.
      api.listRuns().then(setRuns);
      setStartedRuns((n) => n + 1);
    });
  }

  if (settings === null) {
    return (
      <div className="mb-5">
        <p role="status" className="an-thi-giac">
          {LOADING_SETTINGS}
        </p>
        <Skeleton className="h-24 w-full rounded-card" />
      </div>
    );
  }

  const s = settings.ok ? settings.duLieu : null;
  const connected = s !== null && s.configured;
  const blocked = s === null ? null : runNowBlockedReason(s, latest);
  const runDisabled = s === null || starting || blocked !== null || runs === null;

  return (
    <section
      className="portal-sync-card mb-5 min-w-0 rounded-card border border-line bg-white p-4 shadow-card"
      aria-labelledby="portal-sync-title"
    >
      <div className="flex min-w-0 flex-wrap items-start gap-3">
        <div className="min-w-0 flex-1 basis-72 [&_p]:m-0">
          <h2 id="portal-sync-title" className="m-0 flex items-center gap-2 text-[13px] font-bold text-navy">
            <Link2 aria-hidden="true" focusable="false" className="size-4 shrink-0" />
            {PORTAL_SYNC_TITLE}
          </h2>

          {!settings.ok && (
            <p className="thong-bao-loi mt-1" role="alert">
              {settings.thongBao}
            </p>
          )}

          {s !== null &&
            (connected ? (
              <p className="mt-1 text-[12px] text-ink-muted" data-testid="portal-sync-status-line">
                <span
                  className={cn("font-semibold", s.is_enabled ? "text-leaf" : "text-ink-muted")}
                  data-testid="portal-sync-status"
                >
                  {s.is_enabled ? "Đang bật" : "Đang tắt"}
                </span>
                {" · "}
                <CategoryCountPending />
                {" · "}
                {intervalLabel(s.interval_hours)}
                {" · "}
                {publishModeLabel(s.publish_mode)}
              </p>
            ) : (
              <p className="mt-1 text-[12px] text-ink-muted">{NOT_CONNECTED}</p>
            ))}

          {latest !== undefined && <LastRun run={latest} />}
          {runs !== null && !runs.ok && (
            <p className="thong-bao-loi mt-1" role="alert">
              {runs.thongBao}
            </p>
          )}
          {/* Why `Đồng bộ ngay` is off (a run in flight, no platform key) — only where that button is drawn. Not a
              visible line (the prototype draws none): the button's accessible description, and its `title`. */}
          {canEdit && connected && blocked !== null && (
            <p className="an-thi-giac" id="portal-sync-run-blocked">
              {blocked}
            </p>
          )}
        </div>

        {canEdit && (
          <div className="flex items-center gap-2">
            {connected && (
              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={runDisabled}
                aria-describedby={blocked !== null ? "portal-sync-run-blocked" : undefined}
                title={blocked ?? undefined}
                icon={
                  starting ? (
                    <LoaderCircle aria-hidden="true" focusable="false" className="size-3.5 animate-spin" />
                  ) : (
                    <RefreshCw aria-hidden="true" focusable="false" className="size-3.5" />
                  )
                }
                onClick={runNow}
              >
                {RUN_NOW_LABEL}
              </Button>
            )}
            <Button
              type="button"
              variant="primary"
              size="sm"
              aria-haspopup="dialog"
              aria-expanded={configOpen}
              disabled={s === null}
              onClick={() => setConfigOpen(true)}
            >
              {connected ? CONFIGURE_LABEL : CONNECT_LABEL}
            </Button>
          </div>
        )}
      </div>

      {canEdit && configOpen && s !== null && (
        <OverlayDialog
          titleId="portal-sync-config-title"
          onDismiss={() => setConfigOpen(false)}
          className={CONFIG_DIALOG_CLASS}
        >
          <PortalSyncConfig
            settings={s}
            api={api}
            close={() => setConfigOpen(false)}
            saved={(next) => setSettings({ ok: true, duLieu: next })}
          />
        </OverlayDialog>
      )}
    </section>
  );
}

/**
 * The prototype's last-run line (`ContentSourcePanel.tsx:131-149`): `Chạy lần cuối {dd/MM/yyyy HH:mm}` ·
 * `{n} tin mới` · `bỏ qua {n}` · the errors in red. From the newest run of the history route — the settings
 * carry only `last_run_at`, not the counts.
 *
 * A RUN STILL GOING HAS NO COUNTS YET: it says `Đang chạy` instead of a `0 tin mới` that would read as a
 * finished run that found nothing.
 */
export function LastRun({ run }: { run: comms_portalRunOut }) {
  return (
    <p
      className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11.5px] text-ink-muted"
      data-testid="portal-sync-last-run"
    >
      <span>Chạy lần cuối {run.started_at === null ? DAU_GACH : formatVietnamDateTime(run.started_at)}</span>
      {runIsUnfinished(run) ? (
        <span>{outcomeLabel(run.outcome)}</span>
      ) : (
        <>
          <span className="flex items-center gap-1 text-leaf">
            <CheckCircle2 aria-hidden="true" focusable="false" className="size-3" />
            {run.imported_count} tin mới
          </span>
          <span>bỏ qua {skippedCount(run)}</span>
          {run.error_summary.length > 0 && (
            <span className="flex items-center gap-1 font-semibold text-danger" data-testid="portal-sync-run-errors">
              <TriangleAlert aria-hidden="true" focusable="false" className="size-3" />
              {run.error_summary.map(runErrorLine).join("; ")}
            </span>
          )}
        </>
      )}
    </p>
  );
}

/**
 * The `Cấu hình` dialog's body — the prototype's `SourceDialog` (`ContentSourcePanel.tsx:313-566`): title and
 * one sentence, ONE scrolling column (address, key, `Chuyên mục lấy về`, the three numbers, the checkbox box),
 * then `Huỷ` · `Lưu cấu hình`.
 *
 * ONE SAVE BUTTON, TWO ROUTES, IN THIS ORDER: `PUT settings`, then `PUT categories` — the latter only when
 * the tree was read and something in it changed (`categoriesBody` sends only the changes). The first refusal
 * stops the chain and is shown verbatim. If the settings were written and the categories refused, the card
 * already shows the new settings and the dialog stays open on the categories' sentence: a retry re-sends the
 * same settings (a PUT of the whole form, harmless) and then the categories.
 *
 * A CHANGED ADDRESS SENDS NO CATEGORIES: the tree on screen was read from the OLD portal, and its ids mean
 * nothing to the new one. Reopening `Cấu hình` asks the new portal.
 *
 * A FAILED SAVE KEEPS THE FORM — and the typed key — so the officer does not paste it twice after fixing
 * the address.
 */
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
  const [form, setForm] = useState<SettingsForm>(() => formFromSettings(settings));
  const [tree, setTree] = useState<CategoryTreeResult | null>(null);
  const [initial, setInitial] = useState<CategoryChoice[]>([]);
  const [choices, setChoices] = useState<CategoryChoice[]>([]);
  const [sending, setSending] = useState(false);
  // The client-side refusal of the last press, drawn under its own box.
  const [refusal, setRefusal] = useState<{ field: SettingsField; text: string } | null>(null);
  // The server's sentence of the last press — 422 api_key_required(_for_new_url), 400 url rule, 503
  // encryption, 422 too_many_categories — verbatim.
  const [serverError, setServerError] = useState<string | null>(null);

  // Only a configured commune has a key the server can spend on the portal; otherwise nothing is asked.
  useEffect(() => {
    if (!settings.configured) return;
    let gone = false;
    api.getCategories().then((r) => {
      if (gone) return;
      setTree(r);
      if (r.ok) {
        const c = choicesFromTree(r.duLieu);
        setInitial(c);
        setChoices(c);
      }
    });
    return () => {
      gone = true;
    };
  }, [api, settings.configured]);

  const keyRequired = keyRequirement(settings, form) !== null;
  const errorOf = (field: SettingsField) => (refusal !== null && refusal.field === field ? refusal.text : undefined);

  function edit(next: SettingsForm): void {
    setForm(next);
    setRefusal(null);
  }

  function submit(e: FormEvent): void {
    e.preventDefault();
    // Without the platform key the server answers 503 and writes nothing — the button says so up front.
    if (sending || !settings.encryption_configured) return;
    const urlChanged = form.api_url.trim() !== settings.api_url.trim();
    const treeUsable = tree !== null && tree.ok && !urlChanged;
    const selected = !settings.configured ? 0 : treeUsable ? selectedCount(choices) : null;
    const err = settingsFormError(settings, form, selected);
    setRefusal(err);
    setServerError(null);
    if (err !== null) return;

    const catBody = treeUsable ? categoriesBody(initial, choices) : null;
    const finish = () => {
      setSending(false);
      toast.success(SETTINGS_SAVED);
      close();
    };
    setSending(true);
    api.saveSettings(settingsBody(form)).then((r) => {
      if (!r.ok) {
        setSending(false);
        setServerError(r.thongBao);
        return;
      }
      saved(r.duLieu);
      if (catBody === null || catBody.categories.length === 0) {
        finish();
        return;
      }
      api.saveCategories(catBody).then((c) => {
        if (!c.ok) {
          setSending(false);
          setServerError(c.thongBao);
          return;
        }
        // What was just saved is the new baseline, should the close be refused.
        setInitial(choices);
        finish();
      });
    });
  }

  return (
    <form id="portal-sync-config" className="m-0" onSubmit={submit} noValidate aria-labelledby="portal-sync-config-title">
      <div className="mb-4 flex items-start gap-3">
        <div className="min-w-0 flex-1">
          <h3 id="portal-sync-config-title" className="m-0 text-lg leading-snug font-semibold text-navy">
            {PORTAL_SYNC_TITLE}
          </h3>
          <p className="m-0 mt-1 text-[13px] text-ink-muted">{PORTAL_SYNC_DESCRIPTION}</p>
        </div>
        <IconButton type="button" label={CLOSE_LABEL} className="-mt-1 -mr-2 min-h-0" disabled={sending} onClick={close}>
          <X aria-hidden="true" focusable="false" />
        </IconButton>
      </div>

      <div className="max-h-[62vh] space-y-4 overflow-y-auto pr-1">
        <Field
          label="Địa chỉ API của Cổng"
          htmlFor="portal-sync-api-url"
          required
          grow="auto"
          error={errorOf("api_url")}
        >
          <input
            id="portal-sync-api-url"
            name="portal-sync-api-url"
            type="url"
            inputMode="url"
            value={form.api_url}
            placeholder={API_URL_PLACEHOLDER}
            autoComplete="off"
            maxLength={2048}
            onChange={(e) => edit({ ...form, api_url: e.target.value })}
          />
        </Field>

        <Field
          label="Mã bảo mật"
          htmlFor="portal-sync-api-key"
          required={keyRequired}
          grow="auto"
          hint={
            <span data-testid="portal-sync-key-hint">
              {keyRequirement(settings, form) === null ? <KeyInUseHint /> : keyHint(settings, form)}
            </span>
          }
          error={errorOf("api_key")}
        >
          {/* WRITE-ONLY: never prefilled. `new-password`, not the prototype's `off`: browsers ignore `off` on a
              password box and would offer the officer's OWN saved login here — which would then be sent as
              the commune's portal key. */}
          <input
            id="portal-sync-api-key"
            name="portal-sync-api-key"
            type="password"
            value={form.api_key}
            placeholder={keyRequired ? "" : KEY_KEEP_PLACEHOLDER}
            autoComplete="new-password"
            spellCheck={false}
            maxLength={512}
            onChange={(e) => edit({ ...form, api_key: e.target.value })}
          />
        </Field>

        <CategoryPicker
          configured={settings.configured}
          tree={tree}
          choices={choices}
          setChoices={(next) => {
            setChoices(next);
            setRefusal(null);
          }}
          error={errorOf("categories")}
        />

        {/* `sm:` so the three boxes stack at 320px instead of squeezing a select to a third of the screen. */}
        <div className="grid gap-3 sm:grid-cols-3">
          <Field label="Nhịp đồng bộ" htmlFor="portal-sync-interval" kind="select" grow="auto">
            <select
              id="portal-sync-interval"
              value={String(form.interval_hours)}
              onChange={(e) => edit({ ...form, interval_hours: Number.parseInt(e.target.value, 10) })}
            >
              {/* The prototype's set (`intervalOptions`); its `Mỗi 15 phút` disabled with a "?" — the server
                  counts whole hours. The reason is the PHAN_CHUA_DUNG entry, on hover. */}
              {intervalOptions(settings.interval_hours).map((h) => (
                <IntervalOption key={h} hours={h} />
              ))}
            </select>
          </Field>
          <Field
            label="Chỉ lấy tin trong"
            htmlFor="portal-sync-window"
            grow="auto"
            hint={WINDOW_DAYS_HINT}
            error={errorOf("window_days")}
          >
            <input
              id="portal-sync-window"
              type="number"
              inputMode="numeric"
              min={1}
              max={WINDOW_DAYS_MAX}
              step={1}
              value={form.window_days}
              onChange={(e) => edit({ ...form, window_days: e.target.value })}
            />
          </Field>
          <Field
            label="Tối đa mỗi lần"
            htmlFor="portal-sync-max-items"
            grow="auto"
            hint={MAX_ITEMS_HINT}
            error={errorOf("max_items_per_run")}
          >
            <input
              id="portal-sync-max-items"
              type="number"
              inputMode="numeric"
              min={1}
              max={MAX_ITEMS_PER_RUN_MAX}
              step={1}
              value={form.max_items_per_run}
              onChange={(e) => edit({ ...form, max_items_per_run: e.target.value })}
            />
          </Field>
        </div>

        <div className="space-y-2 rounded-[10px] border border-line p-3">
          <label htmlFor="portal-sync-enabled" className={CHECK_ROW}>
            <input
              id="portal-sync-enabled"
              type="checkbox"
              className={CHECKBOX}
              checked={form.is_enabled}
              onChange={(e) => edit({ ...form, is_enabled: e.target.checked })}
            />
            {ENABLED_LABEL}
          </label>
          <label htmlFor="portal-sync-direct" className={CHECK_ROW}>
            <input
              id="portal-sync-direct"
              type="checkbox"
              className={CHECKBOX}
              checked={form.publish_mode === PUBLISH_MODE_DIRECT}
              onChange={(e) =>
                edit({ ...form, publish_mode: e.target.checked ? PUBLISH_MODE_DIRECT : PUBLISH_MODE_REVIEW })
              }
            />
            {PUBLISH_DIRECT_LABEL}
          </label>
          <DownloadImagesPending />
          <label htmlFor="portal-sync-credit" className={CHECK_ROW}>
            <input
              id="portal-sync-credit"
              type="checkbox"
              className={CHECKBOX}
              checked={form.keep_source_credit}
              onChange={(e) => edit({ ...form, keep_source_credit: e.target.checked })}
            />
            {KEEP_SOURCE_LABEL}
          </label>
        </div>

        {!settings.encryption_configured && (
          <p className="m-0 text-[12px] font-medium text-danger" role="alert">
            {portalSyncStatusExplainer("missing-encryption")}
          </p>
        )}
        {serverError !== null && (
          <p className="m-0 text-[12px] font-medium text-danger" role="alert">
            {serverError}
          </p>
        )}
      </div>

      <div className="mt-4 flex justify-end gap-2">
        <Button type="button" variant="secondary" disabled={sending} onClick={close}>
          {CANCEL_LABEL}
        </Button>
        <Button
          type="submit"
          variant="primary"
          disabled={sending || !settings.encryption_configured}
          aria-busy={sending || undefined}
          icon={sending ? <LoaderCircle aria-hidden="true" focusable="false" className="animate-spin" /> : undefined}
        >
          {SAVE_CONFIG_LABEL}
        </Button>
      </div>
    </form>
  );
}

/**
 * `Chuyên mục lấy về` — the prototype's box (`ContentSourcePanel.tsx:353-477`): the counter and `Chọn tất cả`
 * · `Bỏ chọn` once the list is in, then the portal's categories GROUPED BY PARENT with `chọn cả mục` /
 * `bỏ cả mục` per group, and a kind select beside each ticked one. Nothing is saved from here: the dialog's
 * one `Lưu cấu hình` sends the changes.
 *
 * THE 30 CEILING: unticked boxes go off at 30 and `Chọn tất cả` stops there; the server's 422 is what
 * refuses (`CATEGORIES_LIMIT_REACHED` is a hint).
 */
export function CategoryPicker({
  configured,
  tree,
  choices,
  setChoices,
  error,
}: {
  configured: boolean;
  /** `null` while the portal is being asked. */
  tree: CategoryTreeResult | null;
  choices: readonly CategoryChoice[];
  setChoices: (next: CategoryChoice[]) => void;
  /** `Chọn ít nhất một chuyên mục…` after a press of `Lưu cấu hình`. */
  error?: string;
}) {
  const listed = configured && tree !== null && tree.ok && choices.length > 0;
  // A HINT, not the rule: unticked boxes go off at 30 and the server's 422 is what refuses.
  const atLimit = selectedCount(choices) >= SELECTED_CATEGORIES_MAX;

  function update(id: string, change: Partial<CategoryChoice>): void {
    setChoices(choices.map((c) => (c.external_id === id ? { ...c, ...change } : c)));
  }

  return (
    <div className="rounded-[10px] border border-line p-3" data-testid="portal-sync-categories">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <p className="m-0 text-[12.5px] font-semibold text-navy">{CATEGORIES_TITLE}</p>
        {listed && (
          <>
            <span className="text-[11.5px] text-ink-muted tabular-nums">{selectedCountLabel(choices)}</span>
            <div className="ml-auto flex gap-1.5">
              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={atLimit}
                onClick={() => setChoices(selectAllUpToLimit(choices, DEFAULT_TARGET_KIND))}
              >
                Chọn tất cả
              </Button>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                // Unticks the portal's rows; a stored row the portal no longer lists is unticked by hand.
                onClick={() => setChoices(choices.map((c) => (c.on_portal ? { ...c, selected: false } : c)))}
              >
                Bỏ chọn
              </Button>
            </div>
          </>
        )}
      </div>

      {!configured ? (
        <p className="m-0 text-[12px] text-ink-muted">{CATEGORIES_NEED_SETTINGS}</p>
      ) : tree === null ? (
        <>
          <p role="status" className="an-thi-giac">
            {CATEGORIES_LOADING}
          </p>
          <Skeleton className="h-24 w-full" />
        </>
      ) : !tree.ok ? (
        tree.forbidden === true ? (
          // 403: the account reads the register but may not spend the commune's portal key (D3). The
          // server's sentence and the right it needs; the rest of the dialog keeps working.
          <div data-testid="portal-sync-categories-refused">
            <p className="m-0 text-[12px] text-ink-muted">{CATEGORIES_NEED_UPDATE}</p>
            <p className="m-0 text-[12px] text-danger" role="alert">
              {tree.thongBao}
            </p>
          </div>
        ) : (
          <p className="m-0 text-[12px] text-danger" role="alert">
            {CATEGORIES_UNREACHABLE}
          </p>
        )
      ) : choices.length === 0 ? (
        <p className="m-0 text-[12px] text-ink-muted">{CATEGORIES_EMPTY}</p>
      ) : (
        <>
          {atLimit && (
            <p className="m-0 mb-2 text-[11.5px] text-ink-muted" role="status" data-testid="portal-sync-categories-limit">
              {CATEGORIES_LIMIT_REACHED}
            </p>
          )}
          <div className="max-h-72 space-y-1 overflow-y-auto" aria-label="Chuyên mục của Cổng" role="group">
            {groupChoices(choices).map((g) => {
              const ids = new Set(g.items.map((c) => c.external_id));
              const all = g.items.every((c) => c.selected);
              return (
                <div key={g.key}>
                  <div className="mt-2 mb-1 flex items-center gap-2 first:mt-0">
                    <p className="m-0 text-[11px] font-semibold tracking-wide text-ink-muted uppercase">{g.name}</p>
                    <button
                      type="button"
                      className="cursor-pointer border-0 bg-transparent p-0 [font-family:inherit] text-[11px] font-semibold text-navy hover:underline"
                      aria-label={`${all ? "Bỏ cả mục" : "Chọn cả mục"} ${g.name}`}
                      onClick={() => setChoices(pickMany(choices, ids, !all, DEFAULT_TARGET_KIND))}
                    >
                      {all ? "bỏ cả mục" : "chọn cả mục"}
                    </button>
                  </div>
                  {g.items.map((c) => {
                    const tickId = `portal-category-${c.external_id}`;
                    return (
                      <div key={c.external_id} className="flex items-center gap-2.5 py-0.5 pl-2 text-[12.5px]">
                        <input
                          id={tickId}
                          type="checkbox"
                          className={CHECKBOX}
                          checked={c.selected}
                          disabled={!c.selected && atLimit}
                          aria-label={`Lấy chuyên mục ${c.name} thuộc ${g.name}`}
                          onChange={(e) =>
                            update(c.external_id, {
                              selected: e.target.checked,
                              target_kind:
                                e.target.checked && c.target_kind === "" ? DEFAULT_TARGET_KIND : c.target_kind,
                            })
                          }
                        />
                        <label htmlFor={tickId} className="min-w-0 flex-1 truncate">
                          {c.name}
                        </label>
                        {c.selected && (
                          <select
                            className="h-7 shrink-0 rounded-md border border-solid border-line bg-white px-2 text-[11.5px]"
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
                      </div>
                    );
                  })}
                </div>
              );
            })}
          </div>
        </>
      )}

      {error !== undefined && (
        <p className="m-0 mt-2 text-[12px] font-medium text-danger" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}

/**
 * `Tải ảnh về kho của xã` and its note (prototype `ContentSourcePanel.tsx:535-545`), drawn disabled with a "?"
 * (ADR 0068 §14): the settings contract has no such field, so the box could only lie. A child component
 * because `PendingMarker` uses hooks.
 */
function DownloadImagesPending() {
  return (
    <>
      <div className={cn(CHECK_ROW, "text-ink-muted")} data-pending="" aria-disabled="true">
        <input id="portal-sync-download-images" type="checkbox" className={CHECKBOX} disabled />
        <label htmlFor="portal-sync-download-images">Tải ảnh về kho của xã</label>
        <PendingMarker info={pendingContentPart(DOWNLOAD_IMAGES_PART)} />
      </div>
      <p className="m-0 pl-7 text-[11px] text-ink-muted">{DOWNLOAD_IMAGES_NOTE}</p>
    </>
  );
}

/**
 * §3's `{n} chuyên mục` on the status line, with a disabled "?" where the number would be (ADR 0068 §14,
 * MA-02) — the reason is the `PHAN_CHUA_DUNG` entry, passed as-is. A child component because
 * `PendingMarker` uses hooks.
 */
function CategoryCountPending() {
  const info = pendingContentPart(PORTAL_CATEGORY_COUNT_PART);
  return (
    <span className="inline-flex items-center gap-1 align-middle" aria-disabled="true" data-pending="">
      <PendingMarker info={info} />
      chuyên mục
    </span>
  );
}

/**
 * One option of `Nhịp đồng bộ`. The prototype's 15-minute option sits between `Chỉ chạy khi bấm tay` and
 * `Mỗi giờ`, disabled and marked "?": an `<option>` can hold no button, so the mark is text and the reason
 * (the PHAN_CHUA_DUNG entry, as-is) is its `title`.
 */
function IntervalOption({ hours }: { hours: number }) {
  const option = (
    <option value={String(hours)}>
      {intervalLabel(hours)}
    </option>
  );
  if (hours !== 0) return option;
  return (
    <>
      {option}
      <option value="" disabled title={pendingContentPart(INTERVAL_15_MIN_PART).viSao} data-pending="">
        {`${INTERVAL_15_MIN_LABEL} ?`}
      </option>
    </>
  );
}

/**
 * The prototype's configured key hint, `Đang dùng {hint}. Để trống nếu không đổi.`, with a disabled "?" in
 * the `{hint}`'s place — the server returns `api_key_set` and nothing derived from the key. A child
 * component because `PendingMarker` uses hooks.
 */
function KeyInUseHint() {
  return (
    <span className="inline-flex flex-wrap items-center gap-1" data-pending="">
      {KEY_IN_USE_PREFIX}
      <PendingMarker info={pendingContentPart(KEY_IN_USE_PART)} />
      {`. ${KEY_SAVED_HINT}`}
    </span>
  );
}
