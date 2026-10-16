"use client";

import { ExternalLink, Loader2, RefreshCw, TriangleAlert, Unlink } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { controlClass } from "@/components/ui/field";
import { PendingButton, type PendingFeatureInfo } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import { formatVietnamDateTime } from "@/features/noi-dung/nhan-noi-dung";
import { cn } from "@/lib/cn";
// vi-name-ok: the existing result type and sentence of lib/api/goi.ts, imported, not a new name
import { LOI_KHONG_RO, type KetQua } from "@/lib/api/goi";
import {
  attachZaloOATemplate,
  checkCommuneZaloOA,
  createZaloOAAuthorizationLink,
  disconnectCommuneZaloOA,
  getCommuneZaloOA,
  listZaloOATemplates,
  syncZaloOATemplates,
  ZALO_OA_NOT_CONFIGURED,
  type comms_communeZaloOACurrentOut,
  type comms_zaloOATemplateList,
  type comms_zaloOATemplateOut,
} from "@/lib/api/zalo-oa";

import { ConfigDialog } from "./config-dialog";
import { ConfigTable, selectCls } from "./config-ui";
import { SAVE_FAILED_TOAST, SAVED_TOAST } from "./zalo-channel-form";
import {
  attachBlockedReason,
  attachedText,
  AUTHORIZE_INSTRUCTION,
  authorizedText,
  callbackOutcomeText,
  canAttach,
  checkDisconnectReason,
  DISCONNECT_CONSEQUENCE,
  DISCONNECT_REASON_MAX,
  DISCONNECTED_TOAST,
  isOaOutcomeOk,
  LINK_OPENED_TOAST,
  MILESTONES,
  milestoneLabel,
  missingText,
  NO_MILESTONE_OPTION,
  NO_TEMPLATES,
  OA_CHECK_FAILED_TOAST,
  OA_DESCRIPTION,
  OA_NOT_CONFIGURED,
  oaCheckResultText,
  oaLastCheckText,
  oaPill,
  outcomeFromSearch,
  PARAMS_RULE,
  reauthorizationText,
  searchWithoutOutcome,
  staleAttachments,
  staleText,
  syncedText,
  syncResultText,
  TEMPLATES_DESCRIPTION,
  unattachedMilestones,
  unattachedText,
  zaloQualityLabel,
  zaloStatusLabel,
} from "./zalo-oa-form";

/**
 * "OA Zalo của xã" and "Mẫu tin ZBS" (ADR 0098 khối A), two sections of Cấu hình → Kênh Zalo placed
 * right after "Con bot của xã". NO PROTOTYPE (ADR 0098 §Xung đột): built on `CommuneBotSection`'s shape —
 * own reads and writes, explicit buttons, a confirmation with a reason for the destructive one.
 *
 * Mounted only inside `ZaloChannelTab`'s `admin.zalo` gate, so no second gate; comms checks the key
 * again on every route (rule 5 forbidden #1).
 *
 * SECRETS: no route returns a token, and nothing here has a field for one. The AUTHORISATION LINK
 * carries a one-time `state`: it exists only between the 201 and `window.open` in `authorize()` — no
 * state, no storage, no `title`/`aria-label`, no log, never rendered (rule 8, ADR 0098 #5).
 */

// Same card as the other sections of the tab (`zalo-channel-tab.tsx` SECTION / TITLE / DESCRIPTION).
const SECTION = "border-line shadow-card rounded-card m-0 min-w-0 border border-solid bg-white p-5";
const TITLE = "text-navy m-0 text-[14px] font-bold";
const DESCRIPTION = "text-ink-muted m-0 mt-1 max-w-2xl text-[12.5px]";
const FIELD_LABEL = "text-navy m-0 block text-[13px] leading-5 font-semibold";
const HINT = "text-ink-muted m-0 text-[12px]";
const WARNING_BOX =
  "border-tangerine/30 bg-tangerine/8 text-navy m-0 rounded-[10px] border border-solid px-4 py-3 text-[12.5px]";
const DANGER_OUTLINE = "border-danger/30 text-danger hover:not-disabled:bg-danger/5 bg-white";
const milestoneSelectCls = cn(selectCls, "w-full min-w-44 pr-8");

/** ADR 0068 §14: what the "?" says when the platform has not declared its shared Zalo app. */
const NOT_CONFIGURED_INFO = (ten: string): PendingFeatureInfo => ({ ten, viSao: OA_NOT_CONFIGURED });

export type ZaloOAState = {
  current: KetQua<comms_communeZaloOACurrentOut> | null;
  reload: () => Promise<void>;
};

/**
 * The OA row, read once the tab is allowed, and the callback's `?oa=<outcome>` handled ONCE.
 *
 * WHY HERE AND NOT IN THE SECTION: both sections hang on it — "Mẫu tin ZBS" is drawn only when
 * `has_oa` — and they sit in two slots of `ZaloChannelView`. A hook in `ZaloChannelTab` is the one owner.
 *
 * THE OUTCOME IS READ ONCE, THEN REMOVED FROM THE URL (`history.replaceState`): a reload or a copied link
 * must not toast "Đã nối OA" again, and it is never kept in state. The ref guards React's development
 * double-invoke of effects. The OA is read AFTER, so a `da-noi` shows the row that was just written.
 */
export function useZaloOA(allowed: boolean): ZaloOAState {
  const [current, setCurrent] = useState<KetQua<comms_communeZaloOACurrentOut> | null>(null);
  const outcomeRead = useRef(false);

  const reload = useCallback(async () => {
    setCurrent(await getCommuneZaloOA());
  }, []);

  useEffect(() => {
    if (!allowed) return;
    if (!outcomeRead.current && typeof window !== "undefined") {
      outcomeRead.current = true;
      const { pathname, search, hash } = window.location;
      const outcome = outcomeFromSearch(search);
      if (outcome !== null) {
        const shown = callbackOutcomeText(outcome);
        if (shown.ok) toast.success(shown.text);
        else toast.error(shown.text);
        window.history.replaceState(window.history.state, "", `${pathname}${searchWithoutOutcome(search)}${hash}`);
      }
    }
    let gone = false;
    getCommuneZaloOA().then((r) => {
      if (!gone) setCurrent(r);
    });
    return () => {
      gone = true;
    };
  }, [allowed]);

  return { current, reload };
}

type OABusy = "link" | "check" | "disconnect" | null;

export function CommuneZaloOASection({ state }: { state: ZaloOAState }) {
  const { current, reload } = state;
  const [busy, setBusy] = useState<OABusy>(null);
  /** comms' `zalo_oa_not_configured` sentence: "Kết nối OA" stays disabled with it until the page reloads. */
  const [linkRefused, setLinkRefused] = useState<string | null>(null);
  const [disconnectOpen, setDisconnectOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState<string | null>(null);
  const [revokeNotice, setRevokeNotice] = useState<string | null>(null);

  const heading = (
    <div className="min-w-0 flex-1">
      <h3 id="zalo-oa-title" className={TITLE}>
        OA Zalo của xã
      </h3>
      <p className={DESCRIPTION}>{OA_DESCRIPTION}</p>
    </div>
  );

  if (current === null) {
    return (
      <section className={SECTION} aria-labelledby="zalo-oa-title">
        {heading}
        <p role="status" className="an-thi-giac">
          Đang tải OA Zalo của xã…
        </p>
        <Skeleton className="mt-4 h-20 w-full" />
      </section>
    );
  }
  if (!current.ok) {
    return (
      <section className={SECTION} aria-labelledby="zalo-oa-title">
        {heading}
        <p className="text-danger m-0 mt-2 text-[12.5px] font-medium" role="alert">
          {current.thongBao}
        </p>
      </section>
    );
  }
  const now = current.duLieu;
  const oa = now.has_oa ? now.oa : null;
  const reauth = oa?.needs_reauthorization ?? null;
  const idle = busy === null;

  async function authorize() {
    setBusy("link");
    const r = await createZaloOAAuthorizationLink();
    setBusy(null);
    if (!r.ok) {
      if (r.code === ZALO_OA_NOT_CONFIGURED) {
        setLinkRefused(r.thongBao);
        return;
      }
      toast.error(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    // The only place the link exists: opened, then dropped with this scope. Never set into state.
    window.open(r.duLieu.url, "_blank", "noopener");
    toast.success(LINK_OPENED_TOAST);
  }

  async function check() {
    setBusy("check");
    const r = await checkCommuneZaloOA();
    setBusy(null);
    if (!r.ok) {
      toast.error(r.thongBao === LOI_KHONG_RO ? OA_CHECK_FAILED_TOAST : r.thongBao);
      return;
    }
    const text = oaCheckResultText(r.duLieu.result, r.duLieu.oa_name);
    if (isOaOutcomeOk(r.duLieu.result)) toast.success(text);
    else toast.error(text);
    await reload();
  }

  async function disconnect() {
    const checked = checkDisconnectReason(reason);
    if (!checked.ok) {
      setReasonError(checked.text);
      return;
    }
    setReasonError(null);
    setBusy("disconnect");
    const r = await disconnectCommuneZaloOA(checked.reason);
    setBusy(null);
    if (!r.ok) {
      // Kept in the box: the reason typed stays, the server's sentence next to it.
      setReasonError(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    setDisconnectOpen(false);
    setReason("");
    const detached = r.duLieu.detached_template_count;
    toast.success(detached > 0 ? `${DISCONNECTED_TOAST} Đã gỡ ${detached} mốc đã gắn mẫu.` : DISCONNECTED_TOAST);
    setRevokeNotice(r.duLieu.revoke_notice !== "" ? r.duLieu.revoke_notice : null);
    await reload();
  }

  const authorizeLabel = reauth !== null ? "Uỷ quyền lại" : "Kết nối OA";
  const authorizeButton = !now.configured ? (
    <PendingButton info={NOT_CONFIGURED_INFO(authorizeLabel)} variant="primary">
      {authorizeLabel}
    </PendingButton>
  ) : (
    <Button
      type="button"
      variant="primary"
      disabled={!idle || linkRefused !== null}
      aria-busy={busy === "link"}
      aria-describedby="zalo-oa-authorize-hint"
      icon={
        busy === "link" ? (
          <Loader2 aria-hidden="true" focusable="false" className="animate-spin" />
        ) : (
          <ExternalLink aria-hidden="true" focusable="false" />
        )
      }
      onClick={() => void authorize()}
    >
      {authorizeLabel}
    </Button>
  );
  // Shown when there is no OA, or when the OA must be authorised again; a healthy OA has nothing to authorise.
  const showAuthorize = oa === null || reauth !== null;

  return (
    <section className={SECTION} aria-labelledby="zalo-oa-title" data-zalo-oa="">
      <div className="flex flex-wrap items-start gap-3">
        {heading}
        {reauth !== null ? (
          <Badge tone="warning" icon={TriangleAlert} data-oa-pill="">
            {oaPill(now)}
          </Badge>
        ) : (
          <span
            className={cn(
              "rounded-full px-3 py-1 text-[12px] font-semibold",
              oa !== null ? "bg-violet/10 text-violet" : "bg-background text-ink-muted",
            )}
            data-oa-pill=""
          >
            {oaPill(now)}
          </span>
        )}
      </div>

      {!now.configured && (
        <p role="status" className={cn(WARNING_BOX, "mt-3")} data-oa-not-configured="">
          {OA_NOT_CONFIGURED}
        </p>
      )}

      {oa !== null && (
        <dl className="m-0 mt-4 grid gap-x-6 gap-y-2 text-[12.5px] sm:grid-cols-[auto_1fr]">
          <dt className="text-ink-muted">Tên OA</dt>
          <dd className="text-navy m-0 font-semibold" data-oa-name="">
            {oa.oa_name}
          </dd>
          <dt className="text-ink-muted">Mã OA</dt>
          <dd className="text-navy m-0 font-mono break-all" data-oa-id="">
            {oa.oa_id}
          </dd>
        </dl>
      )}

      {oa !== null && (
        <div className="mt-3 flex flex-col gap-1">
          <p className={HINT} data-oa-authorized="">
            {authorizedText(formatVietnamDateTime(oa.authorized_at), oa.authorized_by, oa.authorized_by_name)}
          </p>
          <p className={HINT} data-oa-refreshed="">
            {`Token làm mới lúc ${formatVietnamDateTime(oa.token_refreshed_at)}`}
          </p>
          {oa.last_check !== null && (
            <p className={HINT} data-oa-last-check="">
              {oaLastCheckText(formatVietnamDateTime(oa.last_check.at), oa.last_check.result)}
            </p>
          )}
        </div>
      )}

      {reauth !== null && (
        <p className={cn(WARNING_BOX, "mt-3")} role="alert" data-oa-reauth="">
          {`Từ ${formatVietnamDateTime(reauth.since)}: ${reauthorizationText(reauth.reason)}`}
        </p>
      )}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        {showAuthorize && authorizeButton}
        {oa !== null &&
          (!now.configured ? (
            <PendingButton info={NOT_CONFIGURED_INFO("Kiểm tra kết nối")} variant="outline">
              Kiểm tra kết nối
            </PendingButton>
          ) : (
            <Button
              type="button"
              variant="outline"
              className="bg-white"
              disabled={!idle}
              aria-busy={busy === "check"}
              icon={
                busy === "check" ? (
                  <Loader2 aria-hidden="true" focusable="false" className="animate-spin" />
                ) : (
                  <RefreshCw aria-hidden="true" focusable="false" />
                )
              }
              onClick={() => void check()}
            >
              Kiểm tra kết nối
            </Button>
          ))}
        {oa !== null &&
          (!now.configured ? (
            <PendingButton info={NOT_CONFIGURED_INFO("Ngắt OA")} variant="outline">
              Ngắt OA
            </PendingButton>
          ) : (
            <Button
              type="button"
              variant="outline"
              className={DANGER_OUTLINE}
              disabled={!idle}
              icon={<Unlink aria-hidden="true" focusable="false" />}
              onClick={() => {
                setReasonError(null);
                setDisconnectOpen(true);
              }}
            >
              Ngắt OA
            </Button>
          ))}
      </div>

      {showAuthorize && now.configured && (
        <p id="zalo-oa-authorize-hint" className={cn(HINT, "mt-2 max-w-2xl")}>
          {linkRefused ?? AUTHORIZE_INSTRUCTION}
        </p>
      )}

      {revokeNotice !== null && (
        <p role="status" className={cn(WARNING_BOX, "mt-3")} data-oa-revoke-notice="">
          {revokeNotice}
        </p>
      )}

      {disconnectOpen && oa !== null && (
        <ConfigDialog
          title="Ngắt OA Zalo của xã?"
          hideHeader
          onDismiss={() => {
            if (idle) setDisconnectOpen(false);
          }}
        >
          <ConfirmDialog
            as="form"
            aria-label="Ngắt OA Zalo của xã"
            icon={Unlink}
            tone="danger"
            title={`Ngắt OA “${oa.oa_name}” khỏi xã?`}
            className="m-0 p-0 shadow-none"
            onSubmit={(e) => {
              e.preventDefault();
              if (idle) void disconnect();
            }}
            actions={
              <>
                <Button type="submit" variant="primary" disabled={!idle} aria-busy={busy === "disconnect"}>
                  Ngắt OA
                </Button>
                <Button type="button" variant="outline" disabled={!idle} onClick={() => setDisconnectOpen(false)}>
                  {NUT_HUY}
                </Button>
              </>
            }
          >
            <p className="m-0" data-oa-consequence="">
              {DISCONNECT_CONSEQUENCE}
            </p>
            <div className="mt-3 flex flex-col gap-1.5">
              <label htmlFor="zalo-oa-disconnect-reason" className={FIELD_LABEL}>
                Lý do ngắt OA
              </label>
              <input
                id="zalo-oa-disconnect-reason"
                type="text"
                className={cn(controlClass, "bg-white")}
                maxLength={DISCONNECT_REASON_MAX}
                value={reason}
                disabled={!idle}
                aria-invalid={reasonError !== null}
                onChange={(e) => setReason(e.target.value)}
              />
              {reasonError !== null && (
                <p className="text-danger m-0 text-[12px] font-medium" role="alert">
                  {reasonError}
                </p>
              )}
            </div>
          </ConfirmDialog>
        </ConfigDialog>
      )}
    </section>
  );
}

// ---- "Mẫu tin ZBS" ---------------------------------------------------------------------------------

/**
 * The OA's ZBS templates and the milestone each is attached to (ADR 0098 #8–#10). Mounted only when the
 * commune HAS an OA. A milestone change is PATCHed AT ONCE, like the tab's autosave; a refusal puts the
 * select back to what is stored and toasts the server's sentence (`milestone_taken`,
 * `template_not_attachable`). One change in flight at a time: two overlapping PATCHes on the same
 * milestone would race on the server's one-template-one-milestone rule.
 */
export function ZaloOATemplatesSection() {
  const [list, setList] = useState<KetQua<comms_zaloOATemplateList> | null>(null);
  const [syncing, setSyncing] = useState(false);
  /** The template whose PATCH is in flight; every select is disabled meanwhile. */
  const [saving, setSaving] = useState<string | null>(null);

  const load = useCallback(async () => {
    setList(await listZaloOATemplates());
  }, []);

  useEffect(() => {
    let gone = false;
    listZaloOATemplates().then((r) => {
      if (!gone) setList(r);
    });
    return () => {
      gone = true;
    };
  }, []);

  const heading = (
    <div className="min-w-0 flex-1">
      <h3 id="zalo-oa-templates-title" className={TITLE}>
        Mẫu tin ZBS
      </h3>
      <p className={DESCRIPTION}>{TEMPLATES_DESCRIPTION}</p>
    </div>
  );

  if (list === null) {
    return (
      <section className={SECTION} aria-labelledby="zalo-oa-templates-title">
        {heading}
        <p role="status" className="an-thi-giac">
          Đang tải mẫu tin ZBS…
        </p>
        <Skeleton className="mt-4 h-24 w-full" />
      </section>
    );
  }
  if (!list.ok) {
    return (
      <section className={SECTION} aria-labelledby="zalo-oa-templates-title">
        {heading}
        <p className="text-danger m-0 mt-2 text-[12.5px] font-medium" role="alert">
          {list.thongBao}
        </p>
      </section>
    );
  }
  const data = list.duLieu;
  const items = data.items;

  async function sync() {
    setSyncing(true);
    const r = await syncZaloOATemplates();
    setSyncing(false);
    if (!r.ok) {
      toast.error(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    toast.success(syncResultText(r.duLieu));
    await load();
  }

  async function attach(t: comms_zaloOATemplateOut, milestone: string | null) {
    if (saving !== null || milestone === t.milestone) return;
    const before = data;
    // Shown at once (autosave); put back if the server refuses.
    setList({ ok: true, duLieu: { ...before, items: before.items.map((x) => (x.id === t.id ? { ...x, milestone } : x)) } });
    setSaving(t.id);
    const r = await attachZaloOATemplate(t.id, milestone);
    setSaving(null);
    if (!r.ok) {
      setList({ ok: true, duLieu: before });
      toast.error(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    setList({ ok: true, duLieu: { ...before, items: before.items.map((x) => (x.id === t.id ? r.duLieu : x)) } });
    toast.success(SAVED_TOAST);
  }

  const missing = unattachedMilestones(items);
  const stale = staleAttachments(items);
  const synced = syncedText(data.synced_at === null ? null : formatVietnamDateTime(data.synced_at));

  return (
    <section className={SECTION} aria-labelledby="zalo-oa-templates-title" data-zalo-oa-templates="">
      <div className="flex flex-wrap items-start gap-3">
        {heading}
        <div className="flex flex-col items-end gap-1">
          <Button
            type="button"
            variant="primary"
            disabled={syncing || saving !== null}
            aria-busy={syncing}
            icon={
              syncing ? (
                <Loader2 aria-hidden="true" focusable="false" className="animate-spin" />
              ) : (
                <RefreshCw aria-hidden="true" focusable="false" />
              )
            }
            onClick={() => void sync()}
          >
            Nạp mẫu từ OA
          </Button>
          {synced !== null && (
            <p className={HINT} data-synced-at="">
              {synced}
            </p>
          )}
        </div>
      </div>

      <p className={cn(HINT, "mt-2")} data-params-rule="">
        {PARAMS_RULE}
      </p>

      {items.length === 0 ? (
        <p className="text-ink-muted m-0 mt-4 text-[12.5px]" data-templates-empty="">
          {NO_TEMPLATES}
        </p>
      ) : (
        <div className="mt-4">
          <ConfigTable label="Mẫu tin ZBS của OA" caption="Mẫu tin ZBS nạp từ OA của xã và mốc đã gắn">
            <thead>
              <tr>
                <th scope="col">Tên mẫu</th>
                <th scope="col">Mã</th>
                <th scope="col">Trạng thái Zalo</th>
                <th scope="col">Chất lượng</th>
                <th scope="col">Tham số</th>
                <th scope="col">Mốc gắn</th>
              </tr>
            </thead>
            <tbody>
              {items.map((t) => (
                <TemplateRow
                  key={t.id}
                  t={t}
                  disabled={saving !== null || syncing}
                  busy={saving === t.id}
                  onAttach={(m) => void attach(t, m)}
                />
              ))}
            </tbody>
          </ConfigTable>
        </div>
      )}

      {(missing.length > 0 || stale.length > 0) && (
        <div className={cn(WARNING_BOX, "mt-3 flex flex-col gap-1")} role="status" data-template-warnings="">
          {missing.length > 0 && <p className="m-0" data-unattached="">{unattachedText(missing)}</p>}
          {stale.map((t) => (
            <p key={t.id} className="m-0" data-stale="">
              {staleText(t.milestone ?? "")}
            </p>
          ))}
        </div>
      )}
    </section>
  );
}

function TemplateRow({
  t,
  disabled,
  busy,
  onAttach,
}: {
  t: comms_zaloOATemplateOut;
  disabled: boolean;
  busy: boolean;
  onAttach: (milestone: string | null) => void;
}) {
  const attachable = canAttach(t);
  const blocked = attachBlockedReason(t);
  const missing = missingText(t);
  const selectId = `zalo-oa-template-${t.id}-milestone`;
  const reasonId = `${selectId}-reason`;
  /*
   * A template attached BEFORE it stopped being usable (Zalo disabled it, a sync found its parameters
   * changed) keeps its select open with exactly two choices — its milestone and "Không gắn" — so the
   * commune can free that milestone for another template. Fully disabling it would leave the milestone
   * held by a template that cannot send, and comms answers `milestone_taken` to every other one.
   */
  const detachOnly = !attachable && t.milestone !== null;
  const options = attachable ? MILESTONES : detachOnly ? MILESTONES.filter((m) => m.code === t.milestone) : [];
  return (
    <tr data-template-id={t.id}>
      <td className="text-navy font-semibold">{t.template_name}</td>
      <td className="font-mono">{t.template_id}</td>
      <td>
        <Badge tone={t.zalo_status === "ENABLE" ? "success" : t.zalo_status === "PENDING_REVIEW" ? "warning" : "neutral"}>
          {zaloStatusLabel(t.zalo_status)}
        </Badge>
      </td>
      <td>{zaloQualityLabel(t.zalo_quality)}</td>
      <td>
        <span className="block" data-params="">
          {t.params.length > 0 ? t.params.map((p) => p.name).join(", ") : "—"}
        </span>
        {missing !== null && (
          <span className="text-danger block font-medium line-through" data-missing="">
            {missing}
          </span>
        )}
      </td>
      <td className="!whitespace-normal">
        <span className="flex min-w-44 flex-col gap-1" title={blocked ?? undefined}>
          <label htmlFor={selectId} className="an-thi-giac">
            {`Mốc gắn cho mẫu ${t.template_name}`}
          </label>
          <select
            id={selectId}
            className={milestoneSelectCls}
            value={t.milestone ?? ""}
            disabled={disabled || (!attachable && !detachOnly)}
            aria-busy={busy}
            aria-describedby={blocked !== null ? reasonId : undefined}
            onChange={(e) => onAttach(e.target.value === "" ? null : e.target.value)}
          >
            <option value="">{NO_MILESTONE_OPTION}</option>
            {options.map((m) => (
              <option key={m.code} value={m.code}>
                {m.label}
              </option>
            ))}
            {/* A stored milestone this build does not know is still shown, never silently cleared. */}
            {t.milestone !== null && !MILESTONES.some((m) => m.code === t.milestone) && (
              <option value={t.milestone}>{milestoneLabel(t.milestone)}</option>
            )}
          </select>
          {blocked !== null && (
            <span id={reasonId} className="text-ink-muted text-[11.5px]" data-attach-blocked="">
              {blocked}
            </span>
          )}
          {t.milestone !== null && t.milestone_set_at !== null && t.milestone_set_by !== null && (
            <span className="text-ink-muted text-[11.5px]" data-attached-by="">
              {attachedText(formatVietnamDateTime(t.milestone_set_at), t.milestone_set_by, t.milestone_set_by_name)}
            </span>
          )}
        </span>
      </td>
    </tr>
  );
}
