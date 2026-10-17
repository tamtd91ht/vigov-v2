"use client";

import { ExternalLink, Loader2, RefreshCw, TriangleAlert, Unlink } from "lucide-react";
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
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
  listCommuneZaloOAs,
  listZaloOATemplates,
  syncZaloOATemplates,
  ZALO_OA_NOT_CONFIGURED,
  type comms_communeZaloOAListOut,
  type comms_communeZaloOAOut,
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
  CONNECT_FIRST_LABEL,
  CONNECT_MORE_LABEL,
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
  oaListPill,
  oaPill,
  outcomeFromSearch,
  PARAMS_RULE,
  PICK_OA_LABEL,
  REAUTHORIZE_LABEL,
  reauthorizationText,
  searchWithoutOutcome,
  staleAttachments,
  staleText,
  syncedText,
  syncResultText,
  templateCountText,
  TEMPLATES_DESCRIPTION,
  unattachedMilestones,
  unattachedText,
  zaloQualityLabel,
  zaloStatusLabel,
} from "./zalo-oa-form";

/**
 * "OA Zalo của xã" and "Mẫu tin ZBS" (ADR 0098 khối A; SEVERAL OAs per commune since ADR 0099 #1), two
 * sections of Cấu hình → Kênh Zalo placed right after "Con bot của xã". NO PROTOTYPE (ADR 0098 §Xung đột):
 * built on `CommuneBotSection`'s shape — own reads and writes, explicit buttons, a confirmation with a
 * reason for the destructive one.
 *
 * Mounted only inside `ZaloChannelTab`'s `admin.zalo` gate, so no second gate; comms checks the key
 * again on every route (rule 5 forbidden #1).
 *
 * SECRETS: no route returns a token, and nothing here has a field for one. The AUTHORISATION LINK
 * carries a one-time `state`: it exists only between the 201 and `window.open` in
 * `openZaloOAAuthorization()` — no state, no storage, no `title`/`aria-label`, no log, never rendered
 * (rule 8, ADR 0098 #5).
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
const OA_CARD = "border-line m-0 min-w-0 rounded-[10px] border border-solid p-4";
const milestoneSelectCls = cn(selectCls, "w-full min-w-44 pr-8");
const AUTHORIZE_HINT_ID = "zalo-oa-authorize-hint";

/** ADR 0068 §14: what the "?" says when the platform has not declared its shared Zalo app. */
const NOT_CONFIGURED_INFO = (ten: string): PendingFeatureInfo => ({ ten, viSao: OA_NOT_CONFIGURED });

export type ZaloOAState = {
  current: KetQua<comms_communeZaloOAListOut> | null;
  reload: () => Promise<void>;
};

/**
 * The commune's OAs, read once the tab is allowed, and the callback's `?oa=<outcome>` handled ONCE.
 *
 * WHY HERE AND NOT IN THE SECTION: both sections hang on it — "Mẫu tin ZBS" is drawn only when the
 * commune has at least one OA, and reads the templates of each — and they sit in two slots of
 * `ZaloChannelView`. A hook in `ZaloChannelTab` is the one owner.
 *
 * THE OUTCOME IS READ ONCE, THEN REMOVED FROM THE URL (`history.replaceState`): a reload or a copied link
 * must not toast "Đã nối OA" again, and it is never kept in state. The ref guards React's development
 * double-invoke of effects. The list is read AFTER, so a `da-noi` shows the OA that was just written.
 */
export function useZaloOA(allowed: boolean): ZaloOAState {
  const [current, setCurrent] = useState<KetQua<comms_communeZaloOAListOut> | null>(null);
  const outcomeRead = useRef(false);

  const reload = useCallback(async () => {
    setCurrent(await listCommuneZaloOAs());
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
    listCommuneZaloOAs().then((r) => {
      if (!gone) setCurrent(r);
    });
    return () => {
      gone = true;
    };
  }, [allowed]);

  return { current, reload };
}

/**
 * "Kết nối OA" / "Kết nối thêm OA" / "Uỷ quyền lại": asks comms for a one-time authorisation link and opens
 * it in a new tab. One call for all three — which OA it adds or renews is the one the OA administrator
 * picks at Zalo (ADR 0099 #1). SHARED with the OA card of `/gui-thong-bao`
 * (`features/notice-send/oa-status-card.tsx`) — one copy of the only code that ever holds the link, so the
 * rule below cannot drift between two screens.
 *
 * The link exists only between the 201 and `window.open`, inside this function: never returned, never set
 * into state, never logged (rule 8, ADR 0098 #5). Returns comms' `zalo_oa_not_configured` sentence when
 * the platform has not declared its shared app — the caller keeps its button disabled with it; every
 * other refusal is toasted here.
 */
export async function openZaloOAAuthorization(): Promise<{ refused: string | null }> {
  const r = await createZaloOAAuthorizationLink();
  if (!r.ok) {
    if (r.code === ZALO_OA_NOT_CONFIGURED) return { refused: r.thongBao };
    toast.error(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
    return { refused: null };
  }
  window.open(r.duLieu.url, "_blank", "noopener");
  toast.success(LINK_OPENED_TOAST);
  return { refused: null };
}

/** What is in flight: the link (no OA named), or a check / disconnect of ONE OA. One act at a time. */
type OABusy = { kind: "link" } | { kind: "check" | "disconnect"; id: string } | null;

export function CommuneZaloOASection({ state }: { state: ZaloOAState }) {
  const { current, reload } = state;
  const [busy, setBusy] = useState<OABusy>(null);
  /** comms' `zalo_oa_not_configured` sentence: every authorise button stays disabled with it until the page reloads. */
  const [linkRefused, setLinkRefused] = useState<string | null>(null);
  /** The OA whose "Ngắt OA" confirmation is open. */
  const [disconnecting, setDisconnecting] = useState<comms_communeZaloOAOut | null>(null);
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
  const oas = now.items;
  const idle = busy === null;

  async function authorize() {
    setBusy({ kind: "link" });
    const { refused } = await openZaloOAAuthorization();
    setBusy(null);
    if (refused !== null) setLinkRefused(refused);
  }

  async function check(oa: comms_communeZaloOAOut) {
    setBusy({ kind: "check", id: oa.id });
    const r = await checkCommuneZaloOA(oa.id);
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

  async function disconnect(oa: comms_communeZaloOAOut) {
    const checked = checkDisconnectReason(reason);
    if (!checked.ok) {
      setReasonError(checked.text);
      return;
    }
    setReasonError(null);
    setBusy({ kind: "disconnect", id: oa.id });
    const r = await disconnectCommuneZaloOA(oa.id, checked.reason);
    setBusy(null);
    if (!r.ok) {
      // Kept in the box: the reason typed stays, the server's sentence next to it.
      setReasonError(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    setDisconnecting(null);
    setReason("");
    const detached = r.duLieu.detached_template_count;
    toast.success(detached > 0 ? `${DISCONNECTED_TOAST} Đã gỡ ${detached} mốc đã gắn mẫu.` : DISCONNECTED_TOAST);
    setRevokeNotice(r.duLieu.revoke_notice !== "" ? r.duLieu.revoke_notice : null);
    await reload();
  }

  /** The one authorise button, in three places: the section's head, and each card needing re-authorisation. */
  const authorizeButton = (label: string) =>
    !now.configured ? (
      <PendingButton info={NOT_CONFIGURED_INFO(label)} variant="primary">
        {label}
      </PendingButton>
    ) : (
      <Button
        type="button"
        variant="primary"
        disabled={!idle || linkRefused !== null}
        aria-busy={busy?.kind === "link"}
        aria-describedby={AUTHORIZE_HINT_ID}
        icon={
          busy?.kind === "link" ? (
            <Loader2 aria-hidden="true" focusable="false" className="animate-spin" />
          ) : (
            <ExternalLink aria-hidden="true" focusable="false" />
          )
        }
        onClick={() => void authorize()}
      >
        {label}
      </Button>
    );

  return (
    <section className={SECTION} aria-labelledby="zalo-oa-title" data-zalo-oa="">
      <div className="flex flex-wrap items-start gap-3">
        {heading}
        <div className="flex flex-wrap items-center gap-2">
          <span
            className={cn(
              "rounded-full px-3 py-1 text-[12px] font-semibold",
              oas.length > 0 ? "bg-violet/10 text-violet" : "bg-background text-ink-muted",
            )}
            data-oa-pill=""
          >
            {oaListPill(oas)}
          </span>
          {authorizeButton(oas.length === 0 ? CONNECT_FIRST_LABEL : CONNECT_MORE_LABEL)}
        </div>
      </div>

      {!now.configured && (
        <p role="status" className={cn(WARNING_BOX, "mt-3")} data-oa-not-configured="">
          {OA_NOT_CONFIGURED}
        </p>
      )}

      {now.configured && (
        <p id={AUTHORIZE_HINT_ID} className={cn(HINT, "mt-2 max-w-2xl")}>
          {linkRefused ?? AUTHORIZE_INSTRUCTION}
        </p>
      )}

      {revokeNotice !== null && (
        <p role="status" className={cn(WARNING_BOX, "mt-3")} data-oa-revoke-notice="">
          {revokeNotice}
        </p>
      )}

      {oas.length > 0 && (
        <ul className="m-0 mt-4 flex list-none flex-col gap-3 p-0" aria-label="Các OA đang nối">
          {oas.map((oa) => {
            const reauth = oa.needs_reauthorization;
            const checking = busy?.kind === "check" && busy.id === oa.id;
            return (
              <li key={oa.id} className={OA_CARD} data-oa-row-id={oa.id}>
                <div className="flex flex-wrap items-start gap-2">
                  <h4 className="text-navy m-0 min-w-0 flex-1 text-[13.5px] font-bold break-words" data-oa-name="">
                    {oa.oa_name}
                  </h4>
                  {reauth !== null ? (
                    <Badge tone="warning" icon={TriangleAlert} data-oa-card-pill="">
                      {oaPill(oa)}
                    </Badge>
                  ) : (
                    <span className="bg-violet/10 text-violet rounded-full px-3 py-1 text-[12px] font-semibold" data-oa-card-pill="">
                      {oaPill(oa)}
                    </span>
                  )}
                </div>

                <dl className="m-0 mt-2 grid gap-x-6 gap-y-1 text-[12.5px] sm:grid-cols-[auto_1fr]">
                  <dt className="text-ink-muted">Mã OA</dt>
                  <dd className="text-navy m-0 font-mono break-all" data-oa-id="">
                    {oa.oa_id}
                  </dd>
                  <dt className="text-ink-muted">Mẫu tin</dt>
                  <dd className="text-navy m-0" data-oa-template-count="">
                    {templateCountText(oa)}
                  </dd>
                </dl>

                <div className="mt-2 flex flex-col gap-1">
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

                {reauth !== null && (
                  <p className={cn(WARNING_BOX, "mt-3")} role="alert" data-oa-reauth="">
                    {`Từ ${formatVietnamDateTime(reauth.since)}: ${reauthorizationText(reauth.reason)}`}
                  </p>
                )}

                <div className="mt-3 flex flex-wrap items-center gap-2">
                  {reauth !== null && authorizeButton(REAUTHORIZE_LABEL)}
                  {!now.configured ? (
                    <PendingButton info={NOT_CONFIGURED_INFO("Kiểm tra kết nối")} variant="outline">
                      Kiểm tra kết nối
                    </PendingButton>
                  ) : (
                    <Button
                      type="button"
                      variant="outline"
                      className="bg-white"
                      disabled={!idle}
                      aria-busy={checking}
                      icon={
                        checking ? (
                          <Loader2 aria-hidden="true" focusable="false" className="animate-spin" />
                        ) : (
                          <RefreshCw aria-hidden="true" focusable="false" />
                        )
                      }
                      onClick={() => void check(oa)}
                    >
                      Kiểm tra kết nối
                    </Button>
                  )}
                  {!now.configured ? (
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
                        setReason("");
                        setReasonError(null);
                        setDisconnecting(oa);
                      }}
                    >
                      Ngắt OA
                    </Button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {disconnecting !== null && (
        <ConfigDialog
          title="Ngắt OA Zalo của xã?"
          hideHeader
          onDismiss={() => {
            if (idle) setDisconnecting(null);
          }}
        >
          <ConfirmDialog
            as="form"
            aria-label="Ngắt OA Zalo của xã"
            icon={Unlink}
            tone="danger"
            title={`Ngắt OA “${disconnecting.oa_name}” khỏi xã?`}
            className="m-0 p-0 shadow-none"
            onSubmit={(e) => {
              e.preventDefault();
              if (idle) void disconnect(disconnecting);
            }}
            actions={
              <>
                <Button type="submit" variant="primary" disabled={!idle} aria-busy={busy?.kind === "disconnect"}>
                  Ngắt OA
                </Button>
                <Button type="button" variant="outline" disabled={!idle} onClick={() => setDisconnecting(null)}>
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
 * Each OA's ZBS templates and the milestone each is attached to (ADR 0098 #8–#10, ADR 0099 #1). Mounted
 * only when the commune HAS an OA, keyed by the parent on the OA ids, so connecting or disconnecting an OA
 * reads everything afresh.
 *
 * THE TEMPLATES OF EVERY OA ARE READ, not only the one shown: one milestone has ONE template in the whole
 * commune whichever OA it sits on, so "Chưa có mẫu cho …" is only true across all of them. It is NOT drawn
 * while any OA's list is unread or refused — a milestone attached on that OA would be reported missing.
 *
 * A milestone change is PATCHed AT ONCE, like the tab's autosave; a refusal puts the select back to what
 * is stored and toasts the server's sentence (`milestone_taken`, `template_not_attachable`). One change in
 * flight at a time: two overlapping PATCHes on the same milestone would race on the server's
 * one-template-one-milestone rule. `onChanged` re-reads the OA list so each card's counts follow.
 */
export function ZaloOATemplatesSection({
  oas,
  onChanged,
}: {
  oas: readonly comms_communeZaloOAOut[];
  onChanged?: () => Promise<void>;
}) {
  /** Per OA row id; an absent key = not read yet. */
  const [lists, setLists] = useState<Readonly<Record<string, KetQua<comms_zaloOATemplateList>>>>({});
  const [picked, setPicked] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  /** The template whose PATCH is in flight; every select is disabled meanwhile. */
  const [saving, setSaving] = useState<string | null>(null);

  const ids = oas.map((o) => o.id).join(",");
  useEffect(() => {
    let gone = false;
    for (const id of ids.split(",").filter((x) => x !== "")) {
      listZaloOATemplates(id).then((r) => {
        if (!gone) setLists((cur) => ({ ...cur, [id]: r }));
      });
    }
    return () => {
      gone = true;
    };
  }, [ids]);

  const heading = (
    <div className="min-w-0 flex-1">
      <h3 id="zalo-oa-templates-title" className={TITLE}>
        Mẫu tin ZBS
      </h3>
      <p className={DESCRIPTION}>{TEMPLATES_DESCRIPTION}</p>
    </div>
  );

  if (oas.length === 0) return null;
  const shown = oas.find((o) => o.id === picked) ?? oas[0]!;
  const list = lists[shown.id];

  async function sync() {
    const id = shown.id;
    setSyncing(true);
    const r = await syncZaloOATemplates(id);
    setSyncing(false);
    if (!r.ok) {
      toast.error(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    toast.success(syncResultText(r.duLieu));
    const fresh = await listZaloOATemplates(id);
    setLists((cur) => ({ ...cur, [id]: fresh }));
    await onChanged?.();
  }

  async function attach(oaId: string, before: comms_zaloOATemplateList, t: comms_zaloOATemplateOut, milestone: string | null) {
    if (saving !== null || milestone === t.milestone) return;
    const put = (data: comms_zaloOATemplateList) => setLists((cur) => ({ ...cur, [oaId]: { ok: true, duLieu: data } }));
    // Shown at once (autosave); put back if the server refuses.
    put({ ...before, items: before.items.map((x) => (x.id === t.id ? { ...x, milestone } : x)) });
    setSaving(t.id);
    const r = await attachZaloOATemplate(t.id, milestone);
    setSaving(null);
    if (!r.ok) {
      put(before);
      toast.error(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    put({ ...before, items: before.items.map((x) => (x.id === t.id ? r.duLieu : x)) });
    toast.success(SAVED_TOAST);
    await onChanged?.();
  }

  // Warnings across EVERY OA (see the comment above the component).
  const loaded = oas.map((o) => ({ oa: o, r: lists[o.id] }));
  const allRead = loaded.every((x) => x.r !== undefined && x.r.ok);
  const everyTemplate = loaded.flatMap((x) => (x.r !== undefined && x.r.ok ? x.r.duLieu.items : []));
  const missing = allRead ? unattachedMilestones(everyTemplate) : [];
  const stale = loaded.flatMap((x) =>
    x.r !== undefined && x.r.ok ? staleAttachments(x.r.duLieu.items).map((t) => ({ t, oaName: x.oa.oa_name })) : [],
  );
  const synced = list !== undefined && list.ok && list.duLieu.synced_at !== null ? syncedText(formatVietnamDateTime(list.duLieu.synced_at)) : null;

  let body: ReactNode;
  if (list === undefined) {
    body = (
      <>
        <p role="status" className="an-thi-giac">
          Đang tải mẫu tin ZBS…
        </p>
        <Skeleton className="mt-4 h-24 w-full" />
      </>
    );
  } else if (!list.ok) {
    body = (
      <p className="text-danger m-0 mt-4 text-[12.5px] font-medium" role="alert">
        {list.thongBao}
      </p>
    );
  } else if (list.duLieu.items.length === 0) {
    body = (
      <p className="text-ink-muted m-0 mt-4 text-[12.5px]" data-templates-empty="">
        {NO_TEMPLATES}
      </p>
    );
  } else {
    const data = list.duLieu;
    body = (
      <div className="mt-4">
        <ConfigTable label={`Mẫu tin ZBS của OA ${shown.oa_name}`} caption={`Mẫu tin ZBS nạp từ OA “${shown.oa_name}” và mốc đã gắn`}>
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
            {data.items.map((t) => (
              <TemplateRow
                key={t.id}
                t={t}
                disabled={saving !== null || syncing}
                busy={saving === t.id}
                onAttach={(m) => void attach(shown.id, data, t, m)}
              />
            ))}
          </tbody>
        </ConfigTable>
      </div>
    );
  }

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

      {oas.length > 1 && (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <label htmlFor="zalo-oa-templates-oa" className={FIELD_LABEL}>
            {PICK_OA_LABEL}
          </label>
          <select
            id="zalo-oa-templates-oa"
            className={cn(selectCls, "max-w-full min-w-56 pr-8")}
            value={shown.id}
            disabled={syncing || saving !== null}
            onChange={(e) => setPicked(e.target.value)}
          >
            {oas.map((o) => (
              <option key={o.id} value={o.id}>
                {o.oa_name}
              </option>
            ))}
          </select>
        </div>
      )}

      <p className={cn(HINT, "mt-2")} data-params-rule="">
        {PARAMS_RULE}
      </p>

      {body}

      {(missing.length > 0 || stale.length > 0) && (
        <div className={cn(WARNING_BOX, "mt-3 flex flex-col gap-1")} role="status" data-template-warnings="">
          {missing.length > 0 && <p className="m-0" data-unattached="">{unattachedText(missing)}</p>}
          {stale.map(({ t, oaName }) => (
            <p key={t.id} className="m-0" data-stale="">
              {staleText(t.milestone ?? "", oaName)}
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
