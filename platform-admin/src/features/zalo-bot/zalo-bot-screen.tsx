"use client";

import { Bot, KeyRound, Landmark, Lock, RefreshCw, ShieldCheck, TriangleAlert, Webhook } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FormMessage, TextField } from "@/components/form-parts";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { formatDateTime } from "@/features/communes/commune-parts";
import { useOperator } from "@/features/operator/operator-context";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import {
  ApiError,
  checkSharedZaloBot,
  getSharedZaloBot,
  getSharedZaloBotWebhook,
  listZaloBotCommunes,
  setSharedZaloBot,
  setSharedZaloBotWebhook,
  type SharedZaloBot,
  type SharedZaloBotChange,
  type SharedZaloBotSaved,
  type SharedZaloBotStatus,
  type ZaloBotCommuneStats,
  type ZaloBotWebhook,
} from "@/lib/api";
import { canManageZaloBot } from "@/lib/permissions";

import {
  BOT_NAME_HINT,
  buildBotChange,
  CHAT_URL_HINT,
  communeLabel,
  communeTotals,
  endedLinksText,
  isOk,
  outcomeText,
  RELINK_COUNT_MISSING,
  relinkQuestion,
  relinkStep,
  safeHttpsHref,
  TOKEN_HINT,
  webhookSetText,
  zaloBotError,
  type BotDraft,
  type BotField,
} from "./zalo-bot-model";

/**
 * `/zalo-bot` — the platform's ONE shared Zalo Bot (ADR 0074 #1): its token, its webhook, and how many
 * staff of each commune are paired. Every route behind it requires `ops.zalo_bot.manage`, reads
 * included (ADR 0074 #3), so an operator without the key gets a sentence and NO request is sent. That
 * is UX: service-platform checks the key on every call.
 *
 * THE TOKEN IS WRITE-ONLY (ADR 0074 #4): never prefilled, the input emptied the moment it is sent,
 * and the only trace on screen is "Đã đặt lúc … bởi VH-…". During the relink confirmation the token is
 * held in component memory only, and dropped on confirm, cancel or close.
 */

export type Load<T> = { status: "loading" } | { status: "ready"; data: T } | { status: "error"; message: string };

export type Result = { ok: boolean; text: string };

export const DENIED_TEXT =
  "Tài khoản vận hành của bạn không có quyền quản lý Zalo Bot (cần quyền ops.zalo_bot.manage). Nếu cần, đề nghị người quản lý tài khoản vận hành cấp quyền.";

export const NOT_CONFIGURED_TITLE = "Chưa đặt token cho bot dùng chung.";

export const WEBHOOK_NEEDS_TOKEN = "Đặt token cho bot trước, rồi mới kiểm tra và trỏ webhook được.";

export const WEBHOOK_ELSEWHERE =
  "Bot đang gửi tin nhắn tới một địa chỉ khác hệ thống, nên cán bộ gửi mã ghép nối sẽ không được nhận. Bấm “Trỏ webhook về hệ thống” để sửa.";

export const NO_COMMUNE_ROWS = "Chưa xã nào bật kênh Zalo hoặc có cán bộ ghép nối.";

// --- pure views (rendered to strings in tests) ---------------------------------------------------

function Item({ label, children, mono = false }: { label: string; children: ReactNode; mono?: boolean }) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <dt className="text-xs font-medium text-ink-500">{label}</dt>
      <dd className={`m-0 text-sm break-all text-ink-900 ${mono ? "font-mono" : ""}`}>{children}</dd>
    </div>
  );
}

function ResultLine({ result }: { result: Result | null }) {
  if (result === null) return null;
  return result.ok ? (
    <p role="status" className="m-0 text-sm font-medium text-success-600">
      {result.text}
    </p>
  ) : (
    <FormMessage text={result.text} />
  );
}

export function tokenLine(bot: SharedZaloBotStatus): string {
  return bot.has_token ? `Đã đặt lúc ${formatDateTime(bot.set_at)} bởi ${bot.set_by}` : "Chưa có token";
}

export function SharedBotView({
  state,
  onEdit,
  onCheck,
  checking,
  checkResult,
}: {
  state: Load<SharedZaloBot>;
  onEdit: () => void;
  onCheck: () => void;
  checking: boolean;
  checkResult: Result | null;
}) {
  if (state.status === "loading") {
    return (
      <Card>
        <p role="status" className="sr-only">
          Đang tải cấu hình bot…
        </p>
        <SkeletonRows rows={3} columns={2} />
      </Card>
    );
  }
  if (state.status === "error") {
    return (
      <Card as="section">
        <ErrorState role="alert" title="Chưa tải được cấu hình Zalo Bot" message={state.message} />
      </Card>
    );
  }
  const { bot, last_check: lastCheck } = state.data;
  const configured = state.data.configured && bot !== undefined;
  const href = safeHttpsHref(bot?.chat_url);
  return (
    <Card as="section" aria-labelledby="zalo-bot-title">
      <CardHeader>
        <Bot aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
        <CardTitle id="zalo-bot-title">Bot dùng chung</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {!configured ? (
          <EmptyState icon={Bot} tone="neutral" title={NOT_CONFIGURED_TITLE} className="py-6" />
        ) : (
          <dl className="m-0 grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
            <Item label="Tên bot">{bot.bot_name}</Item>
            <Item label="Liên kết trò chuyện">
              {href !== null ? (
                <a href={href} target="_blank" rel="noopener noreferrer" className="text-brand-700 underline underline-offset-2">
                  {bot.chat_url}
                </a>
              ) : (
                bot.chat_url
              )}
            </Item>
            <Item label="Token">{tokenLine(bot)}</Item>
            <Item label="Lần kiểm tra gần nhất">
              {lastCheck === undefined ? (
                "Chưa kiểm tra"
              ) : (
                <span className="flex flex-col gap-1">
                  <span>{formatDateTime(lastCheck.checked_at)}</span>
                  <Badge tone={isOk(lastCheck.outcome) ? "success" : "danger"}>
                    {isOk(lastCheck.outcome) ? "Kết nối tốt" : "Có lỗi"}
                  </Badge>
                  {!isOk(lastCheck.outcome) ? <span className="text-ink-700">{outcomeText(lastCheck.outcome)}</span> : null}
                </span>
              )}
            </Item>
          </dl>
        )}
        <div className="flex flex-wrap gap-2 border-t border-line pt-4">
          <Button
            type="button"
            variant={configured ? "secondary" : "primary"}
            onClick={onEdit}
            icon={<KeyRound aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          >
            {configured ? "Thay token" : "Đặt token"}
          </Button>
          {configured ? (
            <Button
              type="button"
              variant="secondary"
              onClick={onCheck}
              disabled={checking}
              aria-busy={checking}
              icon={<ShieldCheck aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            >
              {checking ? "Đang kiểm tra…" : "Kiểm tra kết nối"}
            </Button>
          ) : null}
        </div>
        <ResultLine result={checkResult} />
      </CardContent>
    </Card>
  );
}

export function WebhookView({
  configured,
  state,
  onRefresh,
  onPoint,
  pointing,
  pointResult,
}: {
  configured: boolean;
  state: Load<ZaloBotWebhook>;
  onRefresh: () => void;
  onPoint: () => void;
  pointing: boolean;
  pointResult: Result | null;
}) {
  return (
    <Card as="section" aria-labelledby="zalo-webhook-title">
      <CardHeader>
        <Webhook aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
        <CardTitle id="zalo-webhook-title">Webhook</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {!configured ? (
          <p className="m-0 text-sm text-ink-500">{WEBHOOK_NEEDS_TOKEN}</p>
        ) : state.status === "loading" ? (
          <>
            <p role="status" className="sr-only">
              Đang hỏi Zalo về webhook…
            </p>
            <SkeletonRows rows={2} columns={2} />
          </>
        ) : state.status === "error" ? (
          <FormMessage text={state.message} />
        ) : (
          <WebhookFacts webhook={state.data} />
        )}
        {configured ? (
          <div className="flex flex-wrap gap-2 border-t border-line pt-4">
            <Button
              type="button"
              variant="primary"
              onClick={onPoint}
              disabled={pointing}
              aria-busy={pointing}
              icon={<Webhook aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            >
              {pointing ? "Đang trỏ webhook…" : "Trỏ webhook về hệ thống"}
            </Button>
            <Button
              type="button"
              variant="secondary"
              onClick={onRefresh}
              disabled={pointing || state.status === "loading"}
              icon={<RefreshCw aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            >
              Kiểm tra webhook
            </Button>
          </div>
        ) : null}
        <ResultLine result={pointResult} />
      </CardContent>
    </Card>
  );
}

function WebhookFacts({ webhook }: { webhook: ZaloBotWebhook }) {
  const secret =
    webhook.secret_set_at !== undefined
      ? `Đặt lúc ${formatDateTime(webhook.secret_set_at)}${webhook.secret_set_by ? ` bởi ${webhook.secret_set_by}` : ""}`
      : "Chưa đặt";
  if (!isOk(webhook.outcome)) {
    return (
      <>
        <FormMessage text={outcomeText(webhook.outcome)} />
        <dl className="m-0 grid grid-cols-1 gap-3">
          <Item label="Khoá xác thực webhook">{secret}</Item>
        </dl>
      </>
    );
  }
  return (
    <>
      <dl className="m-0 grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-2">
        <Item label="Địa chỉ Zalo đang gọi" mono>
          {webhook.url === "" ? "Zalo chưa có webhook nào cho bot này" : webhook.url}
        </Item>
        <Item label="Đối chiếu">
          {webhook.url_matches ? (
            <Badge tone="success">Đúng địa chỉ của hệ thống</Badge>
          ) : (
            <Badge tone="danger">Không khớp địa chỉ của hệ thống</Badge>
          )}
        </Item>
        <Item label="Khoá xác thực webhook">{secret}</Item>
        {webhook.zalo_updated_at !== undefined ? (
          <Item label="Zalo cập nhật lần cuối">{formatDateTime(webhook.zalo_updated_at)}</Item>
        ) : null}
      </dl>
      {!webhook.url_matches && webhook.url !== "" ? (
        <Notice tone="legal" icon={TriangleAlert}>
          {WEBHOOK_ELSEWHERE}
        </Notice>
      ) : null}
    </>
  );
}

export function CommuneUptakeView({ state }: { state: Load<ZaloBotCommuneStats[]> }) {
  return (
    <Card as="section" aria-labelledby="zalo-communes-title">
      <CardHeader>
        <Landmark aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
        <CardTitle id="zalo-communes-title">Xã dùng bot</CardTitle>
        {state.status === "ready" && state.data.length > 0 ? (
          <span className="text-[13px] text-ink-500">
            {`${communeTotals(state.data).enabled} xã bật kênh · ${communeTotals(state.data).linked} cán bộ đã ghép nối`}
          </span>
        ) : null}
      </CardHeader>
      {state.status === "loading" ? (
        <>
          <p role="status" className="sr-only">
            Đang tải số liệu các xã…
          </p>
          <SkeletonRows rows={3} columns={3} />
        </>
      ) : state.status === "error" ? (
        <ErrorState role="alert" title="Chưa tải được số liệu các xã" message={state.message} />
      ) : state.data.length === 0 ? (
        <EmptyState icon={Landmark} tone="neutral" title={NO_COMMUNE_ROWS} className="py-6" />
      ) : (
        <TableScroll sticky aria-label="Xã dùng bot">
          <table className={DATA_TABLE_CLASS}>
            <caption className="sr-only">Kênh Zalo và số cán bộ đã ghép nối, theo xã</caption>
            <thead>
              <tr>
                <th scope="col">Xã</th>
                <th scope="col">Kênh Zalo</th>
                <th scope="col">Cán bộ đã ghép nối</th>
              </tr>
            </thead>
            <tbody>
              {state.data.map((row) => (
                <tr key={row.tenant_id}>
                  <td>{communeLabel(row)}</td>
                  <td>
                    {row.channel_enabled ? <Badge tone="success">Đang bật</Badge> : <Badge tone="neutral">Đang tắt</Badge>}
                  </td>
                  <td className="tabular-nums">{row.linked_staff_count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableScroll>
      )}
    </Card>
  );
}

/** The token form's fields; no saving, so tests render it and see the token box empty. */
export function TokenFormFields({
  draft,
  onDraft,
  busy,
  error,
}: {
  draft: BotDraft;
  onDraft: (d: BotDraft) => void;
  busy: boolean;
  error: { field: BotField; text: string } | null;
}) {
  return (
    <>
      <TextField
        label="Token của bot"
        hint={TOKEN_HINT}
        name="zalo_bot_token"
        type="password"
        autoComplete="off"
        autoCapitalize="none"
        spellCheck={false}
        value={draft.token}
        onChange={(e) => onDraft({ ...draft, token: e.target.value })}
        disabled={busy}
        error={error?.field === "token" ? error.text : null}
      />
      <TextField
        label="Tên bot"
        hint={BOT_NAME_HINT}
        name="bot_name"
        type="text"
        maxLength={100}
        value={draft.botName}
        onChange={(e) => onDraft({ ...draft, botName: e.target.value })}
        disabled={busy}
        error={error?.field === "botName" ? error.text : null}
      />
      <TextField
        label="Liên kết trò chuyện với bot"
        hint={CHAT_URL_HINT}
        name="chat_url"
        type="url"
        inputMode="url"
        maxLength={300}
        spellCheck={false}
        value={draft.chatUrl}
        onChange={(e) => onDraft({ ...draft, chatUrl: e.target.value })}
        disabled={busy}
        error={error?.field === "chatUrl" ? error.text : null}
      />
      <FormMessage text={error?.field === "form" ? error.text : null} />
    </>
  );
}

/** The relink step's body: the owner's question with the server's count. */
export function RelinkConfirmBody({ count }: { count: number }) {
  return (
    <Notice tone="legal" icon={TriangleAlert}>
      <p>{relinkQuestion(count)}</p>
      <p>Liên kết Zalo của các cán bộ ấy với bot cũ sẽ được gỡ (có ghi vết); họ ghép nối lại ở trang cá nhân của mình.</p>
    </Notice>
  );
}

// --- stateful parts --------------------------------------------------------------------------------

function TokenDialog({
  current,
  onClose,
  onSaved,
}: {
  current: SharedZaloBotStatus | null;
  onClose: () => void;
  onSaved: (b: SharedZaloBotSaved) => void;
}) {
  const guarded = useGuardedError();
  const [draft, setDraft] = useState<BotDraft>({ token: "", botName: current?.bot_name ?? "", chatUrl: current?.chat_url ?? "" });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ field: BotField; text: string } | null>(null);
  // The change awaiting the relink confirmation — the token lives here, never back in the input.
  const [pending, setPending] = useState<{ change: SharedZaloBotChange; count: number } | null>(null);

  async function send(change: SharedZaloBotChange) {
    setBusy(true);
    setError(null);
    try {
      const saved = await setSharedZaloBot(change);
      setPending(null);
      onSaved(saved);
    } catch (err) {
      const step = err instanceof ApiError ? relinkStep(err) : { kind: "none" as const };
      if (step.kind === "confirm") {
        // First ask, or asked again after a confirmation because the count moved: show the server's
        // count and wait for the operator. Never resend on their behalf.
        setPending({ change: { token: change.token, bot_name: change.bot_name, chat_url: change.chat_url }, count: step.count });
        return;
      }
      setPending(null);
      if (step.kind === "missing") {
        setError({ field: "form", text: RELINK_COUNT_MISSING });
        return;
      }
      let field: BotField = "form";
      const text = guarded(err, "đặt token Zalo Bot", (e) => {
        const known = zaloBotError(e);
        if (known) field = known.field;
        return known?.text ?? null;
      });
      if (text !== null) setError({ field, text });
    } finally {
      setBusy(false);
    }
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const built = buildBotChange(draft);
    // The token leaves the input the moment it is submitted, valid or not.
    setDraft((d) => ({ ...d, token: "" }));
    if (!built.ok) return setError({ field: built.field, text: built.text });
    void send(built.change);
  }

  function close() {
    if (busy) return;
    setPending(null);
    setDraft((d) => ({ ...d, token: "" }));
    onClose();
  }

  if (pending !== null) {
    return (
      <Dialog open title="Đổi sang tài khoản bot khác?" tone="danger" icon={TriangleAlert} onClose={close}>
        <RelinkConfirmBody count={pending.count} />
        <FormMessage text={error?.text ?? null} />
        <DialogActions>
          <Button
            type="button"
            variant="danger-solid"
            disabled={busy}
            aria-busy={busy}
            onClick={() => void send({ ...pending.change, expected_relink_count: pending.count })}
          >
            {busy ? "Đang lưu…" : "Tiếp tục đổi bot"}
          </Button>
          <Button type="button" variant="secondary" onClick={close} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </Dialog>
    );
  }

  return (
    <Dialog open title={current !== null ? "Thay token của bot dùng chung" : "Đặt token cho bot dùng chung"} icon={KeyRound} onClose={close}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <p>Token mới có hiệu lực ngay khi lưu. Nếu token thuộc một tài khoản bot khác, hệ thống sẽ hỏi lại trước khi đổi.</p>
        <TokenFormFields draft={draft} onDraft={setDraft} busy={busy} error={error} />
        <DialogActions>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : "Lưu token"}
          </Button>
          <Button type="button" variant="secondary" onClick={close} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

function ZaloBotWorkspace() {
  const guarded = useGuardedError();
  const [bot, setBot] = useState<Load<SharedZaloBot>>({ status: "loading" });
  const [webhook, setWebhook] = useState<Load<ZaloBotWebhook>>({ status: "loading" });
  const [communes, setCommunes] = useState<Load<ZaloBotCommuneStats[]>>({ status: "loading" });
  const [editing, setEditing] = useState(false);
  const [checking, setChecking] = useState(false);
  const [checkResult, setCheckResult] = useState<Result | null>(null);
  const [pointing, setPointing] = useState(false);
  const [pointResult, setPointResult] = useState<Result | null>(null);

  const configured = bot.status === "ready" && bot.data.configured;

  const fail = useCallback(
    (err: unknown, action: string) => guarded(err, action, (e) => zaloBotError(e)?.text ?? null),
    [guarded],
  );

  // The read alone, state set only in its callbacks — so the effect below may call it.
  const fetchWebhook = useCallback(() => {
    getSharedZaloBotWebhook().then(
      (data) => setWebhook({ status: "ready", data }),
      (err: unknown) => {
        const message = fail(err, "xem webhook của Zalo Bot");
        if (message !== null) setWebhook({ status: "error", message });
      },
    );
  }, [fail]);

  /** From a click: show loading again, then read. */
  const loadWebhook = useCallback(() => {
    setWebhook({ status: "loading" });
    fetchWebhook();
  }, [fetchWebhook]);

  useEffect(() => {
    let alive = true;
    getSharedZaloBot().then(
      (data) => alive && setBot({ status: "ready", data }),
      (err: unknown) => {
        if (!alive) return;
        const message = fail(err, "xem cấu hình Zalo Bot");
        if (message !== null) setBot({ status: "error", message });
      },
    );
    listZaloBotCommunes().then(
      (data) => alive && setCommunes({ status: "ready", data }),
      (err: unknown) => {
        if (!alive) return;
        const message = fail(err, "xem số liệu Zalo Bot của các xã");
        if (message !== null) setCommunes({ status: "error", message });
      },
    );
    return () => {
      alive = false;
    };
  }, [fail]);

  // The webhook read calls Zalo, so it runs only once a token exists — and again after a new one.
  useEffect(() => {
    // `webhook` starts as loading, so the first read needs no state change here.
    if (configured) fetchWebhook();
  }, [configured, fetchWebhook]);

  async function check() {
    setChecking(true);
    setCheckResult(null);
    try {
      const r = await checkSharedZaloBot();
      const ok = isOk(r.outcome);
      setCheckResult({
        ok,
        text: ok ? `Kết nối tốt. Zalo nhận ra bot${r.account_name ? ` “${r.account_name}”` : ""}.` : outcomeText(r.outcome),
      });
      // The check is now the bot's last check: re-read so the card shows it.
      getSharedZaloBot().then((data) => setBot({ status: "ready", data }), () => {});
    } catch (err) {
      const message = fail(err, "kiểm tra kết nối Zalo Bot");
      if (message !== null) setCheckResult({ ok: false, text: message });
    } finally {
      setChecking(false);
    }
  }

  async function point() {
    setPointing(true);
    setPointResult(null);
    try {
      const r = await setSharedZaloBotWebhook();
      setPointResult({ ok: isOk(r.outcome), text: webhookSetText(r.outcome) });
      loadWebhook();
    } catch (err) {
      const message = fail(err, "trỏ webhook của Zalo Bot");
      if (message !== null) setPointResult({ ok: false, text: message });
    } finally {
      setPointing(false);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <SharedBotView
        state={bot}
        onEdit={() => setEditing(true)}
        onCheck={() => void check()}
        checking={checking}
        checkResult={checkResult}
      />
      <WebhookView
        configured={configured}
        state={webhook}
        onRefresh={loadWebhook}
        onPoint={() => void point()}
        pointing={pointing}
        pointResult={pointResult}
      />
      <CommuneUptakeView state={communes} />
      {editing && bot.status === "ready" ? (
        <TokenDialog
          current={bot.data.configured ? (bot.data.bot ?? null) : null}
          onClose={() => setEditing(false)}
          onSaved={(data) => {
            setBot({ status: "ready", data });
            setEditing(false);
            const ended = endedLinksText(data.ended_link_count);
            setCheckResult({
              ok: true,
              text: `Đã lưu token mới.${ended !== null ? ` ${ended}` : ""} Nếu đây là tài khoản bot khác, bấm “Trỏ webhook về hệ thống”.`,
            });
            if (data.configured) loadWebhook();
          }}
        />
      ) : null}
    </div>
  );
}

/** Denied first: without the key nothing is requested at all. */
export function ZaloBotGate({ status, keys, message }: { status: "loading" | "ready" | "error"; keys: readonly string[]; message?: string }) {
  if (status === "loading") {
    return (
      <Card>
        <p role="status" className="sr-only">
          Đang kiểm tra quyền…
        </p>
        <SkeletonRows rows={3} columns={2} />
      </Card>
    );
  }
  if (status === "error") {
    return (
      <Card as="section">
        <ErrorState role="alert" title="Chưa đọc được quyền của tài khoản" message={message} />
      </Card>
    );
  }
  if (!canManageZaloBot(keys)) {
    return (
      <Card as="section">
        <EmptyState icon={Lock} tone="neutral" title="Không có quyền quản lý Zalo Bot" description={DENIED_TEXT} />
      </Card>
    );
  }
  return <ZaloBotWorkspace />;
}

export function ZaloBotScreen() {
  const op = useOperator();
  return (
    <ZaloBotGate
      status={op.status}
      keys={op.status === "ready" ? op.operator.permission_keys : []}
      message={op.status === "error" ? op.message : undefined}
    />
  );
}
