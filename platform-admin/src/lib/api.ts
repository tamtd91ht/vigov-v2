/**
 * The platform console talks to the platform service and to NOTHING ELSE.
 *
 * This is the whole security model of this application, and it is enforced by ARCHITECTURE
 * rather than by a permission flag:
 *
 *   The vendor operating this console cannot read a commune's petitions, documents or citizen
 *   data because THERE IS NO CLIENT FOR THOSE SERVICES — not because a flag is switched off.
 *   A flag can be flipped; adding a client has to be written, and would be caught in review.
 *
 * See kb/10-decisions/0003-platform-admin-metadata-only.md.
 *
 * If you find yourself wanting to import a business service client here, STOP. That is the
 * moment the decision is being reversed, and it is a decision for the customer, not for code.
 *
 * HOW CALLS LEAVE THE BROWSER: same-origin RELATIVE paths, `/api/v1/...`. This app's own server
 * forwards them to service-platform (`lib/server/gateway.ts`), whose address is a server-only
 * setting. There is no `NEXT_PUBLIC_*` base URL — ADR 0048 STOP CONDITION #5, rule 8 inv. 4.
 *
 * TYPES ARE HAND-WRITTEN, DELIBERATELY AND TEMPORARILY. The operator routes are kept OUT of
 * kb/20-contracts/openapi.json on purpose (a route there is routable on a commune's host — ADR
 * 0048 §01/10 #6c, `service-platform/internal/http/operator_routes.go` header), so there is no
 * generated operator contract to import yet. Each shape below mirrors one Go struct, named in its
 * comment; when an operator contract is generated, these are DELETED in favour of it — never kept
 * beside it (the drift agent rule 6 exists to prevent).
 *
 * CREDENTIALS: passwords, TOTP codes, recovery codes and the TOTP secret travel ONLY in JSON
 * bodies of POST/PUT — never in a path or a query, never in storage. The session cookie is
 * HttpOnly and set by the server; nothing here reads or writes it.
 *
 * NO RETRIES. A write repeated by a client library is a second write the person never asked for:
 * a second commune, a second recovery-code batch voiding the first, a second sign-in attempt
 * counted against the per-IP limit. Every failure surfaces once, as an `ApiError`.
 */

/** Same-origin prefix of every call. Relative on purpose: no host, no build-time constant. */
export const API_PREFIX = "/api/v1";

// --- wire shapes (hand-written; see the header) -----------------------------------------------

/** operator_sessions.go `operatorSessionView`. */
export type OperatorSession = { operator_code: string; expires_at: string };

/** operator_sessions.go `operatorEnrollmentView`. Both strings are the TOTP secret — never logged. */
export type EnrollmentStart = { operator_code: string; provisioning_uri: string; manual_entry_key: string };

/** operator_sessions.go `operatorEnrollmentCompletedView`. */
export type EnrollmentCompleted = { operator_code: string; expires_at: string; recovery_codes: string[] };

/** operator_sessions.go `operatorWhoAmIView`. */
export type CurrentOperator = { operator_code: string; permission_keys: string[] };

/** operator_sessions.go `operatorRecoveryCodesView`. */
export type RecoveryCodes = { recovery_codes: string[] };

/** operator_communes.go `communeView` — registry metadata ONLY (ADR 0003, ADR 0048 §30/09 #5). */
export type CommuneSummary = {
  id: string;
  name: string;
  province: string;
  active: boolean;
  /** Primary first. */
  domains: string[];
};

/** operator_communes.go `communePageView`. */
export type CommunePage = { items: CommuneSummary[]; next_cursor: string; has_more: boolean };

/**
 * operator_mini_app_secrets.go `secretStatusView`: metadata about an App ID's secret, NEVER the value
 * (owner 05/10/2026, ADR 0070 §Sửa đổi #3). `set_at` / `set_by` only with `da_dat`. `khong_ro` = identity
 * did not answer; it says nothing about whether a secret exists.
 */
export type MiniAppSecretStatus = {
  status: "da_dat" | "chua_dat" | "khong_ro";
  set_at?: string;
  set_by?: string;
};

/**
 * operator_communes.go `miniAppView`. Soft-deleted App IDs are never listed (ADR 0070 §Sửa đổi #2).
 * `secret` is absent on the attach answer only (one row just created); every detail carries it.
 */
export type MiniApp = {
  app_id: string;
  mode: string;
  active: boolean;
  created_at: string;
  created_by: string;
  secret?: MiniAppSecretStatus;
};

/** operator_mini_app_secrets.go `unboundSecretView`: a secret identity still holds for a removed App ID. */
export type UnboundSecret = { app_id: string; secret: MiniAppSecretStatus };

/**
 * operator_communes.go `communeDetailView`. `unbound_secrets`: null = identity could not be asked
 * (unknown), [] = none.
 */
export type CommuneDetail = CommuneSummary & { mini_apps: MiniApp[]; unbound_secrets: UnboundSecret[] | null };

/** operator_communes.go `provinceView`. */
export type Province = { id: string; name: string };

/** Second factor of an operator sign-in (ADR 0048 §28/09 #10): TOTP, or a one-time recovery code. */
export type SecondFactor = { kind: "totp"; code: string } | { kind: "recovery"; code: string };

export type SignInInput = { email: string; password: string; secondFactor: SecondFactor };

/**
 * The `ops.*` keys (service-platform/internal/opauth/opauth.go `Key…`, the CLOSED set `decidedKeys`):
 * the six of ADR 0048 §28/09 #3, the seventh of ADR 0073 #3 and the eighth of ADR 0074 #3
 * (`service-identity/migrations/0025_operator_zalo_bot_key.sql`). A key outside this set grants the
 * console nothing — the same closed set the server's `opauth.AnyKey` reads.
 */
export const OPS_KEYS = {
  tenantManage: "ops.tenant.manage",
  domainManage: "ops.domain.manage",
  profileManage: "ops.profile.manage",
  miniAppManage: "ops.mini_app.manage",
  uploadPolicyManage: "ops.upload_policy.manage",
  qrIssue: "ops.qr.issue",
  petitionFieldManage: "ops.petition_field.manage",
  zaloBotManage: "ops.zalo_bot.manage",
} as const;

export const ALL_OPS_KEYS: readonly string[] = Object.values(OPS_KEYS);

// --- wave 2 (ADR 0073) wire shapes ------------------------------------------------------------

/** operator_platform.go `uploadPolicyView`. `max_files_per_subject` null = deliberately no count limit. */
export type UploadPolicy = {
  purpose: string;
  max_bytes: number;
  allowed_mime_types: string[];
  max_files_per_subject: number | null;
  updated_at: string;
  updated_by: string;
  /** Types this purpose's pipeline handles; empty = a purpose no PUT can change. */
  mime_choices: string[];
  max_bytes_cap: number;
};

/** PUT /upload-policies/{purpose} body (operator_platform.go `uploadPolicyBody`). Every key always sent. */
export type UploadPolicyChange = {
  max_bytes: number;
  allowed_mime_types: string[];
  max_files_per_subject: number | null;
  reason: string;
};

/** operator_launch.go `sharedMiniAppView`. */
export type SharedMiniApp = { app_id: string; created_at: string; created_by: string };

/**
 * operator_launch.go `launchLinkView`. `source` says which app the link opens: `rieng` = the
 * commune's own running app (`domain` may then be ""), `chung` = the shared app (ADR 0070 §Sửa đổi #4).
 */
export type LaunchLink = { url: string; domain: string; app_id: string; source: "rieng" | "chung" };

/**
 * operator_map_frame_default.go `mapFrameDefaultView` (ADR 0072 amendment 2, K1–K2). Without a default:
 * `configured` false and the three hints only. `bounds` is [minLng, minLat, maxLng, maxLat].
 */
export type MapFrameDefault = {
  configured: boolean;
  center_lat?: number;
  center_lng?: number;
  radius_km?: number;
  bounds?: [number, number, number, number];
  updated_at?: string;
  updated_by?: string;
  recommended_radius_km: number;
  /** [min, max] of the usual band; outside it the PUT needs `acknowledged_unusual: true`. */
  usual_radius_km: [number, number];
  max_radius_km: number;
};

/**
 * PUT …/map-frame-default body (operator_map_frame_default.go `mapFrameDefaultBody`).
 * `acknowledged_unusual` is sent only when the radius is outside the usual band and the operator ticked
 * the confirmation — absent reads as false on the server.
 */
export type MapFrameDefaultChange = {
  center_lat: number;
  center_lng: number;
  radius_km: number;
  acknowledged_unusual?: true;
  reason: string;
};

/** operator_petition_fields.go `petitionFieldView`. */
export type PetitionField = {
  code: string;
  default_label: string;
  sort_order: number;
  icon: string;
  tone: string;
  active: boolean;
};

/** operator_petition_fields.go `petitionFieldListView`: every code, retired ones included. */
export type PetitionFieldList = { items: PetitionField[]; tones: string[] };

/** The editable part of a tier-1 code. The code itself is never in an edit body (ADR 0060 §4). */
export type PetitionFieldPresentation = { defaultLabel: string; sortOrder: number; icon: string; tone: string };

/** operator_platform.go `operatorAuditEntryView`. `before`/`after` differ per action — shown, never parsed. */
export type OperatorAuditEntry = {
  at: string;
  actor: string;
  action: string;
  commune: { id: string; name: string } | null;
  subject: string;
  before: unknown;
  after: unknown;
  reason: string;
};

/** operator_platform.go `operatorAuditPageView` — no total, by core/page's rule. */
export type OperatorAuditPage = { items: OperatorAuditEntry[]; next_cursor: string; has_more: boolean };

/** RFC 3339 instants, half-open [from, to). Either may be absent. */
export type OperatorAuditQuery = { from?: string; to?: string; limit?: number; cursor?: string };

// --- Zalo Bot (ADR 0074) wire shapes ------------------------------------------------------------
//
// Mirrors of the operator routes `zalo-bots/shared…` that service-platform forwards to comms'
// ZaloBotOperatorService (`proto/vigov/comms/v1/zalo_bot_operator.proto`). THE TOKEN IS NEVER IN A
// RESPONSE SHAPE: it is write-only (ADR 0074 #4), so no type below has a field that could carry it.
// No staff chat id either — the communes view is counts only (ADR 0003).

/**
 * The class of a Zalo Bot API call (`ZaloBotCallOutcome`), as the stored enum VALUES — Vietnamese
 * without diacritics, ADR 0011, `service-comms/migrations/0018_zalo_bot.sql`. Never refined with
 * Zalo's own text. A value outside this list (a newer server) is shown as a failure of unknown
 * cause, never as success.
 */
export type ZaloBotOutcome =
  | "thanh-cong"
  | "chua-cau-hinh"
  | "token-bi-tu-choi"
  | "gioi-han-tan-suat"
  | "khong-kha-dung"
  | "bi-tu-choi"
  | "phan-hoi-sai-dang";

/** `SharedZaloBotStatus`: metadata about the live token, never the token. `set_by` is a `VH-…` code. */
export type SharedZaloBotStatus = {
  set_at: string;
  set_by: string;
  has_token: boolean;
  bot_name: string;
  chat_url: string;
};

/** GET /zalo-bots/shared. `bot` and `last_check` are absent when `configured` is false. */
export type SharedZaloBot = {
  configured: boolean;
  bot?: SharedZaloBotStatus;
  /** Absent when the token in force was never checked. */
  last_check?: { checked_at: string; outcome: ZaloBotOutcome };
};

/**
 * PUT /zalo-bots/shared answer: the GET shape plus `ended_link_count` — staff links of the OLD bot
 * account soft-deleted in the same transaction (0 when the token is the same bot's).
 */
export type SharedZaloBotSaved = SharedZaloBot & { ended_link_count?: number };

/**
 * PUT /zalo-bots/shared body. `expected_relink_count` is sent ONLY on the second attempt, after the
 * operator confirmed the 409 `can-xac-nhan-ghep-lai` count (ADR 0074, owner 05/10/2026): it says "I
 * have seen that N staff must pair again", so a count that changed in between is refused again.
 */
export type SharedZaloBotChange = {
  token: string;
  bot_name: string;
  chat_url: string;
  expected_relink_count?: number;
};

/** POST /zalo-bots/shared/check. `account_name` only with `ok`. */
export type ZaloBotCheck = { outcome: ZaloBotOutcome; account_name?: string };

/**
 * GET /zalo-bots/shared/webhook — what Zalo reports (`getWebhookInfo`) beside what comms holds. The
 * Zalo fields are meaningful only with `ok`; `secret_set_*` come from comms' rows (absent = never set).
 */
export type ZaloBotWebhook = {
  outcome: ZaloBotOutcome;
  url: string;
  url_matches: boolean;
  secret_set_at?: string;
  secret_set_by?: string;
  zalo_updated_at?: string;
};

/** PUT /zalo-bots/shared/webhook. `set_at` / `set_by` only with `ok`; `url` whenever Zalo was called. */
export type ZaloBotWebhookSet = { outcome: ZaloBotOutcome; url: string; set_at?: string; set_by?: string };

/** One commune's uptake of the shared bot — counts and a switch, nothing about a person. */
export type ZaloBotCommuneStats = {
  tenant_id: string;
  commune_name?: string;
  channel_enabled: boolean;
  linked_staff_count: number;
};

/** passwordRejectionView.problem (operator_sessions.go). */
export type PasswordProblem = "empty" | "not_utf8" | "too_short" | "too_long" | "same_as_current";

export type PasswordRejection = { problem: PasswordProblem | ""; minLength: number; maxLength: number };

// --- errors ----------------------------------------------------------------------------------

/**
 * Every refusal, in the server's one error shape (`core/httpx` `{code, message, trace_id}`), plus
 * what two routes add: `Retry-After` on 429 and the rule a new password failed on 422.
 *
 * `status` 0 means the request never got an answer (network down, gateway unreachable from the
 * browser). It is NOT a 401: an operator whose network blipped is not signed out.
 */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly traceId: string,
    readonly retryAfterSeconds: number | null = null,
    readonly passwordRejection: PasswordRejection | null = null,
    /** 409 `can-xac-nhan-ghep-lai` of PUT /zalo-bots/shared: live staff links a new bot account ends. */
    readonly relinkCount: number | null = null,
    /** 422 `token-khong-dung-duoc` of PUT /zalo-bots/shared: the class of Zalo's answer to the new token. */
    readonly zaloOutcome: string | null = null,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/** `Retry-After` in whole seconds (core/ratelimit sends that form only). Anything else is unknown. */
export function parseRetryAfter(value: string | null): number | null {
  if (value === null || !/^\d+$/.test(value.trim())) return null;
  const n = Number(value.trim());
  return Number.isSafeInteger(n) && n > 0 ? n : null;
}

function asString(v: unknown): string {
  return typeof v === "string" ? v : "";
}

function asNumber(v: unknown): number {
  return typeof v === "number" && Number.isFinite(v) ? v : 0;
}

async function errorFrom(res: Response): Promise<ApiError> {
  let body: Record<string, unknown> = {};
  try {
    const parsed: unknown = await res.json();
    if (parsed !== null && typeof parsed === "object") body = parsed as Record<string, unknown>;
  } catch {
    // A body that is not the error shape (a proxy page, an empty 5xx): keep the status, name no code.
  }
  const code = asString(body.code) || "unexpected_response";
  const rejection =
    code === "new_password_rejected"
      ? {
          problem: asString(body.problem) as PasswordProblem | "",
          minLength: asNumber(body.min_length),
          maxLength: asNumber(body.max_length),
        }
      : null;
  // A count that is not a whole non-negative number is not a count the operator can confirm: null,
  // and the screen then shows the refusal instead of a confirmation box with an invented figure.
  const relink =
    code === "can-xac-nhan-ghep-lai" &&
    typeof body.live_link_count === "number" &&
    Number.isSafeInteger(body.live_link_count) &&
    body.live_link_count >= 0
      ? body.live_link_count
      : null;
  const zaloOutcome = code === "token-khong-dung-duoc" ? asString(body.outcome) || null : null;
  return new ApiError(
    res.status,
    code,
    asString(body.message),
    asString(body.trace_id),
    res.status === 429 ? parseRetryAfter(res.headers.get("Retry-After")) : null,
    rejection,
    relink,
    zaloOutcome,
  );
}

type Method = "GET" | "POST" | "PUT" | "DELETE";

/**
 * One call, no retry. `path` is built by the functions below from fixed segments plus
 * `encodeURIComponent`-escaped ids — never from user text spliced in raw.
 */
async function call<T>(method: Method, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(API_PREFIX + path, {
      method,
      credentials: "same-origin",
      cache: "no-store",
      headers: body === undefined ? { Accept: "application/json" } : { Accept: "application/json", "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, "network_error", "", "");
  }
  if (!res.ok) throw await errorFrom(res);
  if (res.status === 204) return undefined as T;
  try {
    return (await res.json()) as T;
  } catch {
    throw new ApiError(res.status, "unexpected_response", "", "");
  }
}

const id = (v: string) => encodeURIComponent(v);

// --- sign-in and the signed-in operator -------------------------------------------------------

/**
 * Exactly one factor field is sent: the server answers both with 400 `second_factor_ambiguous`,
 * and an empty second key is still a key `DisallowUnknownFields` reads.
 */
export function signIn(input: SignInInput): Promise<OperatorSession> {
  const factor =
    input.secondFactor.kind === "totp"
      ? { totp_code: input.secondFactor.code }
      : { recovery_code: input.secondFactor.code };
  return call("POST", "/operator-sessions", { email: input.email, password: input.password, ...factor });
}

export function beginEnrollment(input: { email: string; temporaryPassword: string }): Promise<EnrollmentStart> {
  return call("POST", "/operator-enrollments", {
    email: input.email,
    temporary_password: input.temporaryPassword,
  });
}

export function completeEnrollment(input: {
  email: string;
  temporaryPassword: string;
  newPassword: string;
  totpCode: string;
}): Promise<EnrollmentCompleted> {
  return call("POST", "/operator-enrollments/completion", {
    email: input.email,
    temporary_password: input.temporaryPassword,
    new_password: input.newPassword,
    totp_code: input.totpCode,
  });
}

export function getCurrentOperator(): Promise<CurrentOperator> {
  return call("GET", "/operator-sessions/current");
}

export function signOut(): Promise<void> {
  return call("DELETE", "/operator-sessions/current");
}

/** 204 = every session of the account is revoked, this one included: sign in again. */
export function changePassword(input: { currentPassword: string; newPassword: string; totpCode: string }): Promise<void> {
  return call("PUT", "/operators/current/password", {
    current_password: input.currentPassword,
    new_password: input.newPassword,
    totp_code: input.totpCode,
  });
}

/** Voids the previous batch. */
export function regenerateRecoveryCodes(input: { totpCode: string }): Promise<RecoveryCodes> {
  return call("POST", "/operators/current/recovery-codes", { totp_code: input.totpCode });
}

// --- the commune registry --------------------------------------------------------------------

export function listCommunes(input: { limit?: number; cursor?: string } = {}): Promise<CommunePage> {
  const q = new URLSearchParams();
  if (input.limit !== undefined) q.set("limit", String(input.limit));
  if (input.cursor) q.set("cursor", input.cursor);
  const qs = q.toString();
  return call("GET", "/communes" + (qs ? "?" + qs : ""));
}

export function getCommune(communeId: string): Promise<CommuneDetail> {
  return call("GET", `/communes/${id(communeId)}`);
}

export async function listProvinces(): Promise<Province[]> {
  const res = await call<{ items: Province[] }>("GET", "/provinces");
  return res.items;
}

export function createCommune(input: { name: string; provinceId: string; primaryDomain: string }): Promise<CommuneDetail> {
  return call("POST", "/communes", {
    name: input.name,
    province_id: input.provinceId,
    primary_domain: input.primaryDomain,
  });
}

export function addDomain(communeId: string, domain: string): Promise<CommuneDetail> {
  return call("POST", `/communes/${id(communeId)}/domains`, { domain });
}

export function setPrimaryDomain(communeId: string, domain: string): Promise<CommuneDetail> {
  return call("PUT", `/communes/${id(communeId)}/primary-domain`, { domain });
}

export function correctName(communeId: string, input: { name: string; reason: string }): Promise<CommuneDetail> {
  return call("PUT", `/communes/${id(communeId)}/name`, { name: input.name, reason: input.reason });
}

export function setActivation(communeId: string, input: { active: boolean; reason: string }): Promise<CommuneDetail> {
  return call("PUT", `/communes/${id(communeId)}/activation`, { active: input.active, reason: input.reason });
}

/** `note` is sent only when given: an empty one is not a note. */
export function attachMiniApp(communeId: string, input: { appId: string; note?: string }): Promise<MiniApp> {
  const body: { app_id: string; note?: string } = { app_id: input.appId };
  if (input.note) body.note = input.note;
  return call("POST", `/communes/${id(communeId)}/mini-apps`, body);
}

/** Why identity did not retire a turned-off App ID's secret (operator_mini_app_secrets.go). */
export type SecretRetirementError = "identity_unavailable" | "session_not_live" | "forbidden" | "refused";

/**
 * operator_mini_app_secrets.go `miniAppChangeView`: the commune as it now stands, plus — whenever an
 * App ID was removed — whether its secret was retired.
 */
export type MiniAppChange = CommuneDetail & {
  secret_retired?: boolean;
  secret_retirement_error?: SecretRetirementError;
};

/** operator_mini_app_secrets.go `miniAppSecretView` — version metadata only, never the value. */
export type MiniAppSecretSet = { app_id: string; version: string; set_at: string; set_by: string };

/** operator_mini_app_secrets.go `miniAppSecretRetirementView`. `retired` false = nothing was live. */
export type MiniAppSecretRetirement = {
  app_id: string;
  retired: boolean;
  retired_version?: string;
  retired_at?: string;
  retired_by?: string;
};

/** One transaction: the new App ID bound, the old one turned off (ADR 0070 #1). */
export function replaceMiniApp(
  communeId: string,
  appId: string,
  input: { newAppId: string; reason: string },
): Promise<MiniAppChange> {
  return call("POST", `/communes/${id(communeId)}/mini-apps/${id(appId)}/replacement`, {
    new_app_id: input.newAppId,
    reason: input.reason,
  });
}

/**
 * Removes the commune's own App ID: a soft delete, permanent — it can never be attached again
 * (ADR 0070 §Sửa đổi 05/10/2026 #2). `active: true` is no longer accepted (422
 * `mini_app_reactivation_removed`), so this console never sends it.
 */
export function removeMiniApp(communeId: string, appId: string, input: { reason: string }): Promise<MiniAppChange> {
  return call("PUT", `/communes/${id(communeId)}/mini-apps/${id(appId)}/activation`, {
    active: false,
    reason: input.reason,
  });
}

/** The secret travels in the JSON body only — never a path, a query, storage or a log line. */
export function setMiniAppSecret(
  communeId: string,
  appId: string,
  input: { secret: string; reason: string },
): Promise<MiniAppSecretSet> {
  return call("PUT", `/communes/${id(communeId)}/mini-apps/${id(appId)}/secret`, {
    secret: input.secret,
    reason: input.reason,
  });
}

/** Also accepts a removed App ID — the ones listed in the detail's `unbound_secrets`. */
export function retireMiniAppSecret(communeId: string, appId: string, input: { reason: string }): Promise<MiniAppSecretRetirement> {
  return call("DELETE", `/communes/${id(communeId)}/mini-apps/${id(appId)}/secret`, { reason: input.reason });
}

// --- wave 2 (ADR 0073) ------------------------------------------------------------------------

export async function listUploadPolicies(): Promise<UploadPolicy[]> {
  const res = await call<{ items: UploadPolicy[] }>("GET", "/upload-policies");
  return res.items;
}

/** Platform-wide; the services reading the limits see it within their 60-second cache. */
export function changeUploadPolicy(purpose: string, body: UploadPolicyChange): Promise<UploadPolicy> {
  return call("PUT", `/upload-policies/${id(purpose)}`, body);
}

/** 404 `shared_mini_app_not_declared` when nothing is declared yet. */
export function getSharedMiniApp(): Promise<SharedMiniApp> {
  return call("GET", "/shared-mini-app");
}

/** First declaration and replacement are one act with one shape (operator_launch.go). */
export function declareSharedMiniApp(input: { appId: string; reason: string }): Promise<SharedMiniApp> {
  return call("PUT", "/shared-mini-app", { app_id: input.appId, reason: input.reason });
}

/** Not trailed by decision (ADR 0048 §30/09 #9); `ops.qr.issue` is the gate. */
export function getLaunchLink(communeId: string): Promise<LaunchLink> {
  return call("GET", `/communes/${id(communeId)}/mini-app-launch-link`);
}

/** Any `ops.*` key reads it (a read of commune metadata, ADR 0073 #1). */
export function getMapFrameDefault(communeId: string): Promise<MapFrameDefault> {
  return call("GET", `/communes/${id(communeId)}/map-frame-default`);
}

/** `ops.tenant.manage`. Create and replace are one act; identical values are a 200 with no new trail entry. */
export function setMapFrameDefault(communeId: string, body: MapFrameDefaultChange): Promise<MapFrameDefault> {
  return call("PUT", `/communes/${id(communeId)}/map-frame-default`, body);
}

export function listPetitionFields(): Promise<PetitionFieldList> {
  return call("GET", "/petition-fields");
}

export function createPetitionField(input: PetitionFieldPresentation & { code: string; reason: string }): Promise<PetitionField> {
  return call("POST", "/petition-fields", {
    code: input.code,
    default_label: input.defaultLabel,
    sort_order: input.sortOrder,
    icon: input.icon,
    tone: input.tone,
    reason: input.reason,
  });
}

/** No `code` key: the server refuses an unknown field, so a rename cannot even be asked. */
export function editPetitionField(code: string, input: PetitionFieldPresentation & { reason: string }): Promise<PetitionField> {
  return call("PUT", `/petition-fields/${id(code)}`, {
    default_label: input.defaultLabel,
    sort_order: input.sortOrder,
    icon: input.icon,
    tone: input.tone,
    reason: input.reason,
  });
}

export function setPetitionFieldActivation(code: string, input: { active: boolean; reason: string }): Promise<PetitionField> {
  return call("PUT", `/petition-fields/${id(code)}/activation`, { active: input.active, reason: input.reason });
}

// --- Zalo Bot (ADR 0074) — every route `ops.zalo_bot.manage`, reads included -------------------

export function getSharedZaloBot(): Promise<SharedZaloBot> {
  return call("GET", "/zalo-bots/shared");
}

/**
 * The token travels in the JSON body only — never a path, a query, storage or a log line — and is
 * never answered back. Built key by key: `expected_relink_count` is absent unless the operator
 * confirmed a count, because an absent key and a 0 mean different things to the server.
 */
export function setSharedZaloBot(change: SharedZaloBotChange): Promise<SharedZaloBotSaved> {
  const body: SharedZaloBotChange = { token: change.token, bot_name: change.bot_name, chat_url: change.chat_url };
  if (change.expected_relink_count !== undefined) body.expected_relink_count = change.expected_relink_count;
  return call("PUT", "/zalo-bots/shared", body);
}

/** Calls Zalo `getMe` once with the live token; the result becomes the bot's last check. */
export function checkSharedZaloBot(): Promise<ZaloBotCheck> {
  return call("POST", "/zalo-bots/shared/check");
}

/** Asks Zalo `getWebhookInfo`. A read: nothing is written. */
export function getSharedZaloBotWebhook(): Promise<ZaloBotWebhook> {
  return call("GET", "/zalo-bots/shared/webhook");
}

/** No body: comms builds the URL and a fresh secret itself (a caller-chosen URL could redirect every update). */
export function setSharedZaloBotWebhook(): Promise<ZaloBotWebhookSet> {
  return call("PUT", "/zalo-bots/shared/webhook");
}

/** Every commune's uptake of the shared bot, counts only. The server trails each read. */
export async function listZaloBotCommunes(): Promise<ZaloBotCommuneStats[]> {
  const res = await call<{ items: ZaloBotCommuneStats[] }>("GET", "/zalo-bots/shared/communes");
  return res.items;
}

function auditQueryString(q: OperatorAuditQuery): string {
  const p = new URLSearchParams();
  if (q.from) p.set("from", q.from);
  if (q.to) p.set("to", q.to);
  if (q.limit !== undefined) p.set("limit", String(q.limit));
  if (q.cursor) p.set("cursor", q.cursor);
  const s = p.toString();
  return s ? "?" + s : "";
}

/**
 * Every commune's operator acts plus the platform-wide changes. With `communeId`, that commune's
 * only — the commune is the PATH's, never a query parameter (operator_platform.go `parseLogQuery`).
 * Each page read is itself trailed by the server.
 */
export function listOperatorAuditEntries(q: OperatorAuditQuery, communeId?: string): Promise<OperatorAuditPage> {
  const base = communeId === undefined ? "/operator-audit-entries" : `/communes/${id(communeId)}/operator-audit-entries`;
  return call("GET", base + auditQueryString(q));
}
