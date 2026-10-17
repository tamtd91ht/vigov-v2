/**
 * Notice campaigns (ADR 0099 #4–#9) — web-admin "Gửi tin ZNS / SMS" (`/gui-thong-bao`). Five routes, all
 * `notice.send` (`service-comms/internal/http/notice_campaigns.go`):
 *
 *   GET  /api/v1/notice-templates            what a campaign can be composed from (every OA, every SMS account)
 *   POST /api/v1/notice-campaign-previews    "Xem trước" — counts, content, estimate; writes nothing
 *   POST /api/v1/notice-campaigns            "Gửi thông báo" — Idempotency-Key REQUIRED
 *   GET  /api/v1/notice-campaigns            the "Đã gửi" history, newest first, `?cursor=` pages
 *   GET  /api/v1/notice-campaigns/{id}       one campaign (its progress while it sends)
 *
 * NO PHONE NUMBER travels either way: a request names an AUDIENCE (a kind plus group / tag / unit ids),
 * never a person; a reply carries counts, never recipients (rule 3). No tenant, no staff code in any
 * request: the commune is the `Host`'s, the actor the session's (rule 1 forbidden #2).
 *
 * MONEY IS DECIMAL TEXT ("3500.00", an ESTIMATE, VAT excluded) and stays text here: the screen formats the
 * string, never a float (`formatMoney` in the screen's form module).
 *
 * Enum VALUES are the stored Vietnamese-without-diacritics strings (ADR 0011).
 */

import { docJSON, docThanLoiGoi, goiGhi, thamSoTheoHopDong } from "./goi";
import type { KetQua } from "./goi";
import type {
  comms_get_notice_campaigns,
  comms_get_notice_campaigns_by_id,
  comms_get_notice_templates,
  comms_noticeCampaignIn,
  comms_noticeCampaignListOut as gen_noticeCampaignListOut,
  comms_noticeCampaignOut as gen_noticeCampaignOut,
  comms_noticePreviewIn,
  comms_noticePreviewOut,
  comms_noticeTemplateParamOut,
  comms_noticeTemplatesOut,
  comms_post_notice_campaign_previews,
  comms_post_notice_campaigns,
  comms_smsNoticeTemplateOut,
  comms_znsNoticeTemplateOut,
} from "./schema.gen";

// Every shape is `schema.gen.ts`'s (agent rule 6). The ONLY additions narrow three server strings to the
// closed sets the screen labels or sends — a value outside them is a red `tsc` at the label tables.

/** `channel` on the wire (comms `domain.NoticeChannelZNS` / `NoticeChannelSMS`). */
export type NoticeChannel = "zalo-zns" | "sms";

/** `audience.kind` — the five chips of step 1, as comms stores them. */
export type NoticeAudienceKind =
  | "nguoi-dan-tat-ca"
  | "nguoi-dan-theo-nhom"
  | "nguoi-dan-theo-nhan"
  | "can-bo-tat-ca"
  | "can-bo-theo-bo-phan";

/** A campaign's state (migration 0032). `da-gui` = the PROVIDER accepted the messages (ADR 0099 #9). */
export type NoticeCampaignStatus = "dang-gui" | "da-gui" | "gui-mot-phan" | "that-bai";

export type NoticeTemplateParam = comms_noticeTemplateParamOut;
export type ZnsNoticeTemplate = comms_znsNoticeTemplateOut;
export type SmsNoticeTemplate = comms_smsNoticeTemplateOut;
export type NoticeTemplates = comms_noticeTemplatesOut;
export type NoticePreview = comms_noticePreviewOut;

export type NoticeCampaign = Omit<gen_noticeCampaignOut, "status"> & { status: NoticeCampaignStatus };
export type NoticeCampaignPage = Omit<gen_noticeCampaignListOut, "items"> & { items: NoticeCampaign[] };

/**
 * What the officer composed. Exactly one source: `oaId` for ZNS, `smsAccountId` for SMS — the other is
 * not sent at all (comms refuses both). `params` is keyed by the template parameters' `field`: the Zalo
 * parameter NAME for ZNS, the slot POSITION ("1", "2", …) for SMS.
 */
export type NoticeDraft = {
  channel: NoticeChannel;
  sourceId: string;
  templateRef: string;
  audience: { kind: NoticeAudienceKind; ids: readonly string[]; tags: readonly string[] };
  params: Readonly<Record<string, string>>;
};

const TEMPLATES_PATH = "/api/v1/notice-templates" satisfies comms_get_notice_templates["duongDan"];
const PREVIEWS_PATH = "/api/v1/notice-campaign-previews" satisfies comms_post_notice_campaign_previews["duongDan"];
const CAMPAIGNS_PATH = "/api/v1/notice-campaigns" satisfies comms_post_notice_campaigns["duongDan"];
const CAMPAIGNS_LIST_PATH = "/api/v1/notice-campaigns" satisfies comms_get_notice_campaigns["duongDan"];
const CAMPAIGN_TEMPLATE = "/api/v1/notice-campaigns/{id}" satisfies comms_get_notice_campaigns_by_id["duongDan"];

/** Success statuses, as `notice_campaigns.go` declares them (`@reply`). */
export const NOTICE_STATUS = { previewed: 200, created: 201 } as const;

/**
 * The refusal codes of the two POSTs (`notice_campaigns.go` `fail`). The screen reads ONE of them —
 * `no_recipients`, drawn in the prototype's own sentence; every other refusal shows the server's sentence.
 */
export const NOTICE_ERROR = {
  invalid: "notice_invalid",
  notFound: "not_found",
  noRecipients: "no_recipients",
  templateNotSendable: "template_not_sendable",
  sourceUnavailable: "source_unavailable",
  missingParams: "missing_params",
  identityUnavailable: "identity_unavailable",
  encryptionMissing: "encryption_not_configured",
} as const;

/** GET notice-templates — every ENABLE ZBS template of every live OA, every SMS template of every live account. */
export function listNoticeTemplates(): Promise<KetQua<NoticeTemplates>> {
  return docJSON<NoticeTemplates>(TEMPLATES_PATH);
}

/** The preview body, built key by key (a `...draft` is how a stray field goes up). */
function previewBody(d: NoticeDraft): comms_noticePreviewIn {
  const body: comms_noticePreviewIn = {
    channel: d.channel,
    template_ref: d.templateRef,
    audience: { kind: d.audience.kind, ids: [...d.audience.ids], tags: [...d.audience.tags] },
    params: { ...d.params },
  };
  if (d.channel === "zalo-zns") body.oa_id = d.sourceId;
  else body.sms_account_id = d.sourceId;
  return body;
}

/** POST notice-campaign-previews — writes nothing; asked twice, it answers twice. */
export function previewNoticeCampaign(d: NoticeDraft): Promise<KetQua<NoticePreview>> {
  return docThanLoiGoi<NoticePreview>(
    goiGhi(PREVIEWS_PATH, "POST", previewBody(d), NOTICE_STATUS.previewed, undefined, { withCode: true }),
  );
}

/**
 * POST notice-campaigns — the campaign and every recipient row in one transaction; the messages go out
 * from a background job afterwards.
 *
 * `idempotencyKey` REQUIRED and a PARAMETER, kept by the caller across a retry of the SAME composition: a
 * new key per click would let a retried POST write a second campaign and send every message twice — real
 * money, and a citizen who receives the notice twice.
 */
export function createNoticeCampaign(
  d: NoticeDraft,
  title: string,
  idempotencyKey: string,
): Promise<KetQua<NoticeCampaign>> {
  const sent: comms_noticeCampaignIn = { title, ...previewBody(d) };
  return docThanLoiGoi<NoticeCampaign>(
    goiGhi(CAMPAIGNS_PATH, "POST", sent, NOTICE_STATUS.created, { "Idempotency-Key": idempotencyKey }, { withCode: true }),
  );
}

/** GET notice-campaigns — one page, newest first. `next_cursor` "" = the last page. An empty cursor is not sent. */
export function listNoticeCampaigns(cursor = ""): Promise<KetQua<NoticeCampaignPage>> {
  const q = new URLSearchParams();
  thamSoTheoHopDong<comms_get_notice_campaigns["truyVan"]>(q)("cursor", cursor);
  const qs = q.toString();
  return docJSON<NoticeCampaignPage>(qs === "" ? CAMPAIGNS_LIST_PATH : `${CAMPAIGNS_LIST_PATH}?${qs}`);
}

/** GET notice-campaigns/{id} — another commune's id is the same 404 as an unknown one. */
export function getNoticeCampaign(id: string): Promise<KetQua<NoticeCampaign>> {
  return docJSON<NoticeCampaign>(CAMPAIGN_TEMPLATE.replace("{id}", encodeURIComponent(id)));
}
