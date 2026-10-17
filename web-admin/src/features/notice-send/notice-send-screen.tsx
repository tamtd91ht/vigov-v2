"use client";

import { Loader2, Send } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useRef, useState, type ChangeEvent, type KeyboardEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { PageHeader } from "@/components/ui/page-header";
import { PendingMarker } from "@/components/ui/pending-feature";
import { Segmented } from "@/components/ui/segmented";
import { Skeleton } from "@/components/ui/skeleton";
import { Tab, TabList } from "@/components/ui/tabs";
import { Tooltip } from "@/components/ui/tooltip";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { countCitizenContacts, listCitizenContactTags, NO_CITIZEN_CONTACT_FILTER, type CitizenContactTag } from "@/lib/api/citizen-contacts";
import { listCitizenGroups, type CitizenGroup } from "@/lib/api/citizen-groups";
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi"; // vi-name-ok: existing export of danh-ba-chon-nguoi.ts, imported not declared (rule 12 inv 3)
import { layDanhMucBoPhan } from "@/lib/api/danh-muc"; // vi-name-ok: existing export of danh-muc.ts, imported not declared (rule 12 inv 3)
// vi-name-ok: the existing result type of lib/api/goi.ts, imported, not a new name
import type { KetQua } from "@/lib/api/goi";
import {
  createNoticeCampaign,
  getNoticeCampaign,
  listNoticeCampaigns,
  listNoticeRecipients,
  listNoticeAttributes,
  listNoticeTemplates,
  NOTICE_ERROR,
  previewNoticeCampaign,
  type NoticeAttribute,
  type NoticeAttributeAudience,
  type NoticeAudienceKind,
  type NoticeCampaign,
  type NoticeCampaignPage,
  type NoticeDraft,
  type NoticePreview,
  type NoticeRecipient,
  type NoticeRecipientMapping,
  type NoticeTemplates,
  type SmsContentMode,
  type SmsNoticeTemplate,
} from "@/lib/api/notice-campaigns";
import type { identity_boPhanRa, identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";
import { CITIZEN_READ_PERMISSION, NOTICE_SEND_PERMISSION, quyetDinhTheoKhoa, ZALO_CHANNEL_PERMISSION } from "@/lib/quyen";

import {
  allowedAttributeChoice,
  ATTRIBUTE_HELP,
  ATTRIBUTE_HELP_LABEL,
  ATTRIBUTE_MAPPING_COLUMNS,
  ATTRIBUTE_PICK,
  attributeAudience,
  attributeAudienceOfWire,
  attributeMappingBody,
  attributeMappingRows,
  audienceBody,
  audienceKindLabel,
  AUDIENCE_KINDS,
  AUDIENCE_LABEL,
  BACK_TO_HISTORY,
  CAMPAIGN_TITLE_LABEL,
  campaignSubline,
  CHANNEL_LABEL,
  CHANNEL_PICK_LABEL,
  CHANNEL_WIRE,
  countStaff,
  groupByOA,
  groupChipLabel,
  HISTORY_COLUMNS,
  HISTORY_EMPTY,
  historyTabLabel,
  invalidCountText,
  isStaffAudience,
  LOAD_MORE,
  MAPPING_CONTENT_COLUMN,
  MAPPING_OTHER_PARAMS_SHARED,
  MAPPING_PARAM_COLUMNS,
  MAPPING_PARAMS_SHARED,
  MAPPING_PHONE,
  MAPPING_SHARED_CONTENT,
  MAPPING_TITLE,
  MAPPING_VARIABLE_COLUMNS,
  mappingVariableLabel,
  missingLabel,
  missingParamsText,
  moneyOrNotPriced,
  NO_CITIZEN_READ,
  NO_RECIPIENTS,
  NO_SEND_PERMISSION,
  NO_TEMPLATES,
  NOTICE_SEND_TITLE,
  OA_REAUTH_REASON,
  OA_REAUTH_TAG,
  OPEN_CAMPAIGN_HINT,
  OPEN_SMS_CONFIG,
  OPEN_ZALO_CONFIG,
  PARAM_SOURCE_LABEL,
  PARAM_SOURCES,
  paramLabel,
  PARAMS_SOURCE_TITLE,
  paramSourceLegend,
  paramsLine,
  PICK_GROUP,
  PICK_TAG,
  PICK_TEMPLATE_FIRST,
  PICK_UNIT,
  PICKER_EMPTY,
  PREVIEW_BUTTON,
  PREVIEW_EMPTY,
  PREVIEW_STALE,
  PREVIEW_TITLE,
  queuedToast,
  RECIPIENT_COLUMNS,
  recipientSource,
  recipientStatusLabel,
  recipientStatusTone,
  recipientsText,
  recipientTimeText,
  RECIPIENTS_EMPTY,
  RECIPIENTS_TITLE,
  ROW_COST,
  ROW_DUPLICATE,
  ROW_INVALID,
  ROW_RECIPIENTS,
  ROW_SKIPPED,
  ROW_UNIT_PRICE,
  SEND_BUTTON,
  SEND_NOTE,
  SENDING_POLL_MS,
  SMS_ACCOUNT_PICK_LABEL,
  SMS_CONFIG_HREF,
  smsAccountOptionLabel,
  smsAccounts,
  smsSubline,
  statusLabel,
  statusTone,
  TAB_COMPOSE,
  tagChipLabel,
  templateKey,
  templateName,
  templateSourceId,
  ZALO_CONFIG_HREF,
  ZNS_CONTENT,
  znsSubline,
  type AudienceKind,
  type Channel,
  type ParamSource,
  type PickedTemplate,
} from "./notice-send-form";
import { OAStatusCard, useCommuneOA } from "./oa-status-card";
import {
  buildMapping,
  buildRows,
  charCounterText,
  columnLabel,
  CONTENT_COLUMN_LABEL,
  FILE_HINT,
  FILE_LABEL,
  fileSummary,
  guessPhoneColumn,
  importErrors,
  importTitle,
  maskedPreview,
  MAX_CONTENT_LENGTH,
  PARAM_SOURCE_SHARED,
  paramSourceColumn,
  PHONE_COLUMN_LABEL,
  PICK_COLUMN,
  readRecipientFile,
  SHARED_CONTENT_LABEL,
  SMS_MODE_LABEL,
  SMS_MODE_LEGEND,
  SMS_NO_TEMPLATE_NOTE,
  SOURCE_KINDS,
  SOURCE_LABEL,
  SOURCE_LEGEND,
  splitTypedPhones,
  TYPED_HINT,
  TYPED_LABEL,
  typedCountText,
  typedPhonesError,
  VARIABLES_HELP,
  VARIABLES_HELP_LABEL,
  VARIABLES_TITLE,
  variableToken,
  detectVariables,
  type ImportedSource,
  type ImportedTable,
  type RecipientSourceKind,
  type SmsImportComposition,
} from "./recipient-import";

/**
 * `/gui-thong-bao` — "Gửi tin ZNS / SMS", the prototype's `NoticeWorkspace`, on the real routes of
 * ADR 0099 (`service-comms/internal/http/notice_campaigns.go`).
 *
 * THE GATE is `notice.send` (`CongQuyen`), with the title drawn in every state as `/nguoi-dan` does. Every
 * notice route declares that key, so the templates, the preview, the send and the history all read with
 * it. Two parts read with OTHER keys, and say so when the session lacks them instead of failing:
 *   citizen.read  groups, tags and the step-1 citizen recipient count
 *   admin.zalo    the OA card (`zalo-oas`)
 * A route the session surely lacks the key for is NOT called (the `CongQuyen` rule: no certain 403). The
 * server still checks every one (agent rule 5): hiding is UX.
 *
 * The order is the prototype's on purpose: audience first, then template — a template written for staff
 * and one written for citizens address people differently, and picking the template first is how
 * "kính mời đồng chí" reaches a whole commune.
 */
export function NoticeSendScreen() {
  const phien = usePhien();
  const open = phien !== null && quyetDinhTheoKhoa(phien, NOTICE_SEND_PERMISSION).hien;
  const canReadCitizens = phien !== null && quyetDinhTheoKhoa(phien, CITIZEN_READ_PERMISSION).hien;
  const canManageZalo = phien !== null && quyetDinhTheoKhoa(phien, ZALO_CHANNEL_PERMISSION).hien;
  return (
    <div className="min-w-0">
      {!open && <PageHeader title={NOTICE_SEND_TITLE} className="mb-5" />}
      <CongQuyen khoa={NOTICE_SEND_PERMISSION} cauThieuQuyen={NO_SEND_PERMISSION}>
        <NoticeSendWorkspace canReadCitizens={canReadCitizens} canManageZalo={canManageZalo} />
      </CongQuyen>
    </div>
  );
}

type Step = "audience" | "template" | "content";
type TabId = "compose" | "history";
const PANEL_ID = "notice-send-panel";
const tabDomId = (id: TabId) => `notice-send-tab-${id}`;

const CARD = "border-line shadow-card rounded-card border border-solid bg-white";
const MUTED = "text-ink-muted m-0 text-[12.5px]";
const LINK = "text-brand text-[12.5px] font-semibold underline-offset-2 hover:underline";
const ALERT = "text-danger m-0 text-[12.5px] font-medium";

/** A preview, tied to the composition it was asked for; `result` null = in flight. */
type PreviewState = { key: string; result: KetQua<NoticePreview> | null };
type History = { items: NoticeCampaign[]; next: string };

function firstPage(r: KetQua<NoticeCampaignPage>): KetQua<History> {
  return r.ok ? { ok: true, duLieu: { items: r.duLieu.items, next: r.duLieu.next_cursor } } : r;
}

function importedSourceOf(
  kind: RecipientSourceKind,
  phones: readonly string[],
  table: ImportedTable | null,
  phoneColumn: number | null,
): ImportedSource | null {
  if (kind === "typed") return { kind: "typed", phones };
  if (kind === "file" && table !== null) return { kind: "file", table, phoneColumn };
  return null;
}

/** A typed list has no columns: its SMS text is always the shared one, variables refused (S3). */
function smsCompositionOf(
  smsFree: boolean,
  kind: RecipientSourceKind,
  mode: SmsContentMode,
  contentColumn: number | null,
  content: string,
  variableColumns: Readonly<Record<number, number | null>>,
): SmsImportComposition | null {
  if (!smsFree) return null;
  return { mode: kind === "typed" ? "noi-dung-chung" : mode, contentColumn, content, variableColumns };
}

function NoticeSendWorkspace({ canReadCitizens, canManageZalo }: { canReadCitizens: boolean; canManageZalo: boolean }) {
  const [tab, setTab] = useState<TabId>("compose");
  const [step, setStep] = useState<Step>("audience");
  const [kind, setKind] = useState<AudienceKind>("citizen_all");
  const [ids, setIds] = useState<string[]>([]);
  const [tags, setTags] = useState<string[]>([]);
  const [channel, setChannel] = useState<Channel>("zns");
  const [smsAccountId, setSmsAccountId] = useState("");
  const [picked, setPicked] = useState<PickedTemplate | null>(null);
  const [title, setTitle] = useState("");
  const [params, setParams] = useState<Record<string, string>>({});
  /** Directory only (ADR 0101 #12): a key present = "Theo thuộc tính", its value the code ("" = not picked). */
  const [attributeChoice, setAttributeChoice] = useState<Record<string, string>>({});
  const [attributeHelpOpen, setAttributeHelpOpen] = useState(false);

  // The imported sources (ADR 0099 §Sửa đổi S1–S4). The file's cells live HERE, in memory, and nowhere else.
  const [sourceKind, setSourceKind] = useState<RecipientSourceKind>("directory");
  const [typedText, setTypedText] = useState("");
  const [table, setTable] = useState<ImportedTable | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [reading, setReading] = useState(false);
  const [phoneColumn, setPhoneColumn] = useState<number | null>(null);
  const [smsMode, setSmsMode] = useState<SmsContentMode>("cot-trong-tep");
  const [contentColumn, setContentColumn] = useState<number | null>(null);
  const [sharedContent, setSharedContent] = useState("");
  const [variableColumns, setVariableColumns] = useState<Record<number, number | null>>({});
  const [paramColumns, setParamColumns] = useState<Record<string, number | null>>({});
  const [openCampaign, setOpenCampaign] = useState<NoticeCampaign | null>(null);

  const [groups, setGroups] = useState<KetQua<{ items: CitizenGroup[] }> | null>(null);
  const [tagList, setTagList] = useState<KetQua<{ items: CitizenContactTag[] }> | null>(null);
  const [units, setUnits] = useState<KetQua<{ items: identity_boPhanRa[] }> | null>(null);
  const [staff, setStaff] = useState<KetQua<{ items: identity_canBoChonNguoiRa[] }> | null>(null);
  const [count, setCount] = useState<{ key: string; result: KetQua<number> } | null>(null);
  const [templates, setTemplates] = useState<KetQua<NoticeTemplates> | null>(null);
  const [attributes, setAttributes] = useState<Partial<Record<NoticeAttributeAudience, KetQua<NoticeAttribute[]>>>>({});

  const [preview, setPreview] = useState<PreviewState | null>(null);
  /** The latest preview asked for: an older answer arriving late must not overwrite a newer one. */
  const previewSeq = useRef(0);
  const [sending, setSending] = useState(false);
  /**
   * The Idempotency-Key of the send, kept for the SAME composition (title included): a retry after a
   * refusal or a lost answer — or a second press — replays the first send instead of writing a second
   * campaign and paying for every message twice. A changed composition gets a new key.
   */
  const [sendKey, setSendKey] = useState<{ of: string; key: string } | null>(null);

  const [history, setHistory] = useState<KetQua<History> | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreRefused, setMoreRefused] = useState<string | null>(null);

  const oa = useCommuneOA(canManageZalo);

  // Every template of every OA and SMS account: ONE call, readable with notice.send (ADR 0099 #4).
  useEffect(() => {
    let gone = false;
    listNoticeTemplates().then((r) => {
      if (!gone) setTemplates(r);
    });
    return () => {
      gone = true;
    };
  }, []);

  // Groups and tags: only with citizen.read — every directory read route declares it.
  useEffect(() => {
    if (!canReadCitizens) return;
    let gone = false;
    listCitizenGroups().then((r) => {
      if (!gone) setGroups(r);
    });
    listCitizenContactTags().then((r) => {
      if (!gone) setTagList(r);
    });
    return () => {
      gone = true;
    };
  }, [canReadCitizens]);

  // Units and the staff directory: read the first time a staff audience is picked, then kept.
  const wantUnits = kind === "staff_unit" && units === null;
  useEffect(() => {
    if (!wantUnits) return;
    let gone = false;
    layDanhMucBoPhan().then((r) => {
      if (!gone) setUnits(r);
    });
    return () => {
      gone = true;
    };
  }, [wantUnits]);

  const wantStaff = isStaffAudience(kind) && staff === null;
  useEffect(() => {
    if (!wantStaff) return;
    let gone = false;
    layDanhBaChonNguoi().then((r) => {
      if (!gone) setStaff(r);
    });
    return () => {
      gone = true;
    };
  }, [wantStaff]);

  // The attributes a parameter may take, read once per audience group the first time a template is picked
  // for the directory — the imported sources map columns, not attributes.
  const attrAudience = attributeAudience(kind);
  const wantAttributes = sourceKind === "directory" && picked !== null && attributes[attrAudience] === undefined;
  useEffect(() => {
    if (!wantAttributes) return;
    let gone = false;
    listNoticeAttributes(attrAudience).then((r) => {
      if (!gone) setAttributes((cur) => ({ ...cur, [attrAudience]: r.ok ? { ok: true, duLieu: r.duLieu.items } : r }));
    });
    return () => {
      gone = true;
    };
  }, [wantAttributes, attrAudience]);
  const attributeList = attributes[attrAudience];

  const tagRows = tagList !== null && tagList.ok ? tagList.duLieu.items : [];
  const source = recipientSource(kind, ids, tags, tagRows, canReadCitizens);
  const countKey = source.type === "citizen-count" ? `group:${source.groupId}` : null;
  useEffect(() => {
    if (countKey === null) return;
    const groupId = countKey.slice("group:".length);
    let gone = false;
    countCitizenContacts({ ...NO_CITIZEN_CONTACT_FILTER, groupId }).then((result) => {
      if (!gone) setCount({ key: countKey, result });
    });
    return () => {
      gone = true;
    };
  }, [countKey]);

  // ---- the history ----------------------------------------------------------------------------------

  useEffect(() => {
    let gone = false;
    listNoticeCampaigns().then((r) => {
      if (!gone) setHistory(firstPage(r));
    });
    return () => {
      gone = true;
    };
  }, []);

  /** After a send: the first page again — the new campaign is the newest row. */
  async function reloadHistory() {
    const r = await listNoticeCampaigns();
    setMoreRefused(null);
    setHistory(firstPage(r));
  }

  async function loadMore() {
    if (history === null || !history.ok || history.duLieu.next === "") return;
    setLoadingMore(true);
    const r = await listNoticeCampaigns(history.duLieu.next);
    setLoadingMore(false);
    if (!r.ok) {
      setMoreRefused(r.thongBao);
      return;
    }
    setMoreRefused(null);
    setHistory((cur) =>
      cur !== null && cur.ok
        ? { ok: true, duLieu: { items: [...cur.duLieu.items, ...r.duLieu.items], next: r.duLieu.next_cursor } }
        : cur,
    );
  }

  // A campaign still sending is re-read every 10 s while the tab is open — one by one, so the pages
  // already loaded stay where they are.
  const sendingIds = history !== null && history.ok ? history.duLieu.items.filter((c) => c.status === "dang-gui").map((c) => c.id).join(",") : "";
  useEffect(() => {
    if (tab !== "history" || sendingIds === "") return;
    const timer = setInterval(() => {
      for (const id of sendingIds.split(",")) {
        getNoticeCampaign(id).then((r) => {
          if (!r.ok) return;
          setHistory((cur) =>
            cur !== null && cur.ok
              ? { ok: true, duLieu: { ...cur.duLieu, items: cur.duLieu.items.map((c) => (c.id === r.duLieu.id ? r.duLieu : c)) } }
              : cur,
          );
        });
      }
    }, SENDING_POLL_MS);
    return () => clearInterval(timer);
  }, [tab, sendingIds]);

  // ---- composition ----------------------------------------------------------------------------------

  // ---- the imported sources ---------------------------------------------------------------------------

  const imported = sourceKind !== "directory";
  /** SMS to an imported list is free text on an account, not a template (S3): step 2 picks no card. */
  const smsFree = imported && channel === "sms";
  const typedPhones = useMemo(() => (sourceKind === "typed" ? splitTypedPhones(typedText) : []), [sourceKind, typedText]);
  const importedSource = importedSourceOf(sourceKind, typedPhones, table, phoneColumn);
  const smsComposition = smsCompositionOf(smsFree, sourceKind, smsMode, contentColumn, sharedContent, variableColumns);
  const rows = useMemo(() => {
    const src = importedSourceOf(sourceKind, typedPhones, table, phoneColumn);
    if (src === null) return [];
    const sms = smsCompositionOf(smsFree, sourceKind, smsMode, contentColumn, sharedContent, variableColumns);
    return buildRows(src, channel, sms, sourceKind === "file" && channel === "zns" ? paramColumns : {});
  }, [sourceKind, typedPhones, table, phoneColumn, smsFree, channel, smsMode, contentColumn, sharedContent, variableColumns, paramColumns]);
  /** Stringified once per change of the rows, not on every keystroke elsewhere (10.000 rows). */
  const rowsKey = useMemo(() => JSON.stringify(rows), [rows]);
  const errors: string[] = !imported
    ? []
    : importedSource === null
      ? [fileError ?? "Chọn tệp danh sách."]
      : importErrors(importedSource, channel, smsComposition, rows);

  const freeAccounts = templates !== null && templates.ok ? smsAccounts(templates.duLieu.sms) : [];
  const freeAccount = freeAccounts.find((a) => a.sms_account_id === smsAccountId) ?? freeAccounts[0] ?? null;

  function wireKind(): NoticeAudienceKind {
    return sourceKind === "typed" ? "danh-sach-so" : "tep-nhap";
  }

  function draftOf(p: PickedTemplate | null, values: Readonly<Record<string, string>>): NoticeDraft | null {
    if (smsFree) {
      if (freeAccount === null || smsComposition === null) return null;
      const d: NoticeDraft = {
        channel: "sms",
        sourceId: freeAccount.sms_account_id,
        templateRef: "",
        audience: { kind: wireKind(), ids: [], tags: [] },
        params: {},
        recipients: rows,
        smsContentMode: smsComposition.mode,
      };
      if (smsComposition.mode === "noi-dung-chung") d.content = smsComposition.content;
      const mapping = importedSource === null ? undefined : buildMapping(importedSource, "sms", smsComposition, {});
      if (mapping !== undefined) d.mapping = mapping;
      return d;
    }
    if (p === null) return null;
    const fields = p.t.params.map((x) => x.field);
    // Directory only: a parameter fed by each recipient's attribute goes up "" in `params` — never a shared
    // value the officer typed before switching the row.
    const attributeMapping = imported ? {} : attributeMappingBody(fields, attributeChoice, kind);
    const d: NoticeDraft = {
      channel: CHANNEL_WIRE[p.channel],
      sourceId: templateSourceId(p),
      templateRef: p.t.template_ref,
      audience: imported ? { kind: wireKind(), ids: [], tags: [] } : audienceBody(kind, ids, tags),
      // Exactly the template's fields, each one present: an empty value is what comms reports as missing.
      // A parameter taken from a file column goes up empty here and filled in each row's `values` (S4).
      params: Object.fromEntries(fields.map((f) => [f, f in attributeMapping ? "" : (values[f] ?? "")])),
    };
    if (Object.keys(attributeMapping).length > 0) d.attributeMapping = attributeMapping;
    if (imported) d.recipients = rows;
    const mapping =
      importedSource === null ? undefined : buildMapping(importedSource, p.channel, null, p.channel === "zns" ? paramColumns : {});
    if (mapping !== undefined) d.mapping = mapping;
    return d;
  }

  /** The composition's identity: the draft without its rows, plus the rows' own key. */
  function keyOf(d: NoticeDraft): string {
    return JSON.stringify({ ...d, recipients: undefined }) + (d.recipients === undefined ? "" : rowsKey);
  }

  const currentDraft = draftOf(picked, params);
  const draftKey = currentDraft === null ? "" : keyOf(currentDraft);

  async function runPreview(p: PickedTemplate | null = picked, values: Readonly<Record<string, string>> = params) {
    if (errors.length > 0) return;
    const d = draftOf(p, values);
    if (d === null) return;
    const key = keyOf(d);
    const seq = ++previewSeq.current;
    setPreview({ key, result: null });
    const result = await previewNoticeCampaign(d);
    if (seq === previewSeq.current) setPreview({ key, result });
  }

  function clearPreview() {
    previewSeq.current++;
    setPreview(null);
  }

  function pickKind(next: AudienceKind) {
    setKind(next);
    setAttributeChoice((cur) => allowedAttributeChoice(cur, next));
    setIds([]);
    setTags([]);
    setPicked(null);
    clearPreview();
  }

  function pickChannel(next: Channel) {
    setChannel(next);
    setPicked(null);
    setParamColumns({});
    clearPreview();
  }

  function pickSource(next: RecipientSourceKind) {
    setSourceKind(next);
    setPicked(null);
    setParamColumns({});
    clearPreview();
  }

  /** The file is read here and dropped from the input at once: nothing keeps a handle on it (S2). */
  async function onFile(e: ChangeEvent<HTMLInputElement>) {
    const input = e.target;
    const file = input.files?.[0];
    if (file === undefined) return;
    setReading(true);
    const r = await readRecipientFile(file);
    input.value = "";
    setReading(false);
    clearPreview();
    setContentColumn(null);
    setVariableColumns({});
    setParamColumns({});
    if (!r.ok) {
      setTable(null);
      setPhoneColumn(null);
      setFileError(r.error);
      return;
    }
    setFileError(null);
    setTable(r.table);
    setPhoneColumn(guessPhoneColumn(r.table.headers));
  }

  function pickTemplate(p: PickedTemplate) {
    setPicked(p);
    setParams({});
    setParamColumns({});
    setAttributeChoice({});
    setTitle(templateName(p));
    setStep("content");
    // Prototype: picking a template runs the preview at once; a changed parameter is previewed again by
    // pressing "Xem trước". An imported list still in error is not previewed (`runPreview` refuses).
    void runPreview(p, {});
  }

  function defaultTitle(): string {
    if (picked !== null && !smsFree) return templateName(picked);
    return smsComposition === null ? "" : importTitle(smsComposition, sourceKind === "typed" ? "typed" : "file");
  }

  async function send() {
    const d = draftOf(picked, params);
    if (d === null) return;
    const finalTitle = title.trim() !== "" ? title.trim() : defaultTitle();
    const of = keyOf(d) + JSON.stringify(finalTitle);
    const key = sendKey !== null && sendKey.of === of ? sendKey.key : crypto.randomUUID();
    setSendKey({ of, key });
    setSending(true);
    const r = await createNoticeCampaign(d, finalTitle, key);
    setSending(false);
    if (!r.ok) {
      toast.error(r.code === NOTICE_ERROR.noRecipients ? NO_RECIPIENTS : r.thongBao);
      return;
    }
    toast.success(queuedToast(r.duLieu.recipient_count));
    void reloadHistory();
  }

  function onTabKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (e.key !== "ArrowRight" && e.key !== "ArrowLeft" && e.key !== "Home" && e.key !== "End") return;
    e.preventDefault();
    const next: TabId = e.key === "Home" ? "compose" : e.key === "End" ? "history" : tab === "compose" ? "history" : "compose";
    setTab(next);
    document.getElementById(tabDomId(next))?.focus();
  }

  /** The step-1 figure, or a "?" with its reason — never a number nobody counted. */
  function recipients(): ReactNode {
    const pending = (reason: string) => <PendingMarker info={{ ten: ROW_RECIPIENTS, viSao: reason }} side="right" />;
    const loading = <span className="text-ink-muted">…</span>;
    if (sourceKind === "typed") return typedCountText(typedPhones.length);
    if (sourceKind === "file") return table === null ? "—" : `${table.rows.length.toLocaleString("vi-VN")} dòng`;
    switch (source.type) {
      case "unknown":
        return pending(source.reason);
      case "tag":
        return recipientsText(source.count);
      case "citizen-count":
        if (count === null || count.key !== countKey) return loading;
        return count.result.ok ? recipientsText(count.result.duLieu) : pending(count.result.thongBao);
      case "staff":
        if (staff === null) return loading;
        return staff.ok ? recipientsText(countStaff(staff.duLieu.items, source.unitIds)) : pending(staff.thongBao);
    }
  }

  const loadedCount = history !== null && history.ok ? history.duLieu.items.length : null;
  const tabs: readonly { id: TabId; label: string }[] = [
    { id: "compose", label: TAB_COMPOSE },
    { id: "history", label: historyTabLabel(loadedCount) },
  ];

  const shown = preview !== null && preview.result !== null && preview.result.ok ? preview.result.duLieu : null;
  const stale = preview !== null && preview.key !== draftKey;

  return (
    <div className="min-w-0">
      <PageHeader
        title={NOTICE_SEND_TITLE}
        className="mb-5"
        subtitle={
          <p className="m-0 max-w-2xl">
            Tin thương hiệu gửi ra ngoài qua Zalo ZNS hoặc tin nhắn SMS, tới điện thoại của bà con và cán bộ — khác với{" "}
            <b>Thông báo nội bộ</b>, thứ chỉ hiện trong phần mềm. Tin ở đây tốn phí và không rút lại được, nên luôn phải xem
            trước.
          </p>
        }
      />

      <OAStatusCard canManage={canManageZalo} current={oa} />

      <TabList aria-label="Phần của màn Gửi tin ZNS / SMS" onKeyDown={onTabKeyDown}>
        {tabs.map((t) => (
          <Tab
            key={t.id}
            id={tabDomId(t.id)}
            selected={t.id === tab}
            aria-controls={PANEL_ID}
            tabIndex={t.id === tab ? 0 : -1}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </Tab>
        ))}
      </TabList>

      <div role="tabpanel" id={PANEL_ID} aria-labelledby={tabDomId(tab)} className="mt-4 min-w-0">
        {tab === "compose" ? (
          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_22rem]">
            <div className="flex min-w-0 flex-col gap-4">
              <StepCard
                index={1}
                title="Gửi cho ai"
                done={step !== "audience"}
                open={step === "audience"}
                onOpen={() => setStep("audience")}
                summary={imported ? SOURCE_LABEL[sourceKind] : AUDIENCE_LABEL[kind]}
              >
                <Segmented
                  legend={SOURCE_LEGEND}
                  name="notice-source"
                  mode="buttons"
                  value={sourceKind}
                  options={SOURCE_KINDS.map((k) => ({ value: k, label: SOURCE_LABEL[k] }))}
                  onChange={(v) => pickSource(v as RecipientSourceKind)}
                  className="mb-4"
                />

                {sourceKind === "typed" && (
                  <TypedPhones
                    text={typedText}
                    count={typedPhones.length}
                    // An empty box says nothing yet; the over-limit sentence shows as soon as it is true.
                    error={typedPhones.length > 0 ? (typedPhonesError(typedPhones) ?? undefined) : undefined}
                    onChange={setTypedText}
                  />
                )}

                {sourceKind === "file" && (
                  <FileImport
                    table={table}
                    error={fileError}
                    reading={reading}
                    phoneColumn={phoneColumn}
                    onFile={(e) => void onFile(e)}
                    onPhoneColumn={setPhoneColumn}
                  />
                )}

                {sourceKind === "directory" && (
                <div className="flex flex-wrap gap-1.5">
                  {AUDIENCE_KINDS.map((k) => (
                    <Chip key={k} label={AUDIENCE_LABEL[k]} active={kind === k} onClick={() => pickKind(k)} />
                  ))}
                </div>
                )}

                {sourceKind === "directory" && kind === "citizen_group" &&
                  (canReadCitizens ? (
                    <Picker
                      label={PICK_GROUP}
                      state={groups}
                      options={(g) => g.items.map((row) => ({ id: row.id, label: groupChipLabel(row) }))}
                      selected={ids}
                      onToggle={setIds}
                    />
                  ) : (
                    <p className={cn(MUTED, "mt-4")} data-no-citizen-read="">
                      {NO_CITIZEN_READ}
                    </p>
                  ))}

                {sourceKind === "directory" && kind === "citizen_tag" &&
                  (canReadCitizens ? (
                    <Picker
                      label={PICK_TAG}
                      state={tagList}
                      options={(t) => t.items.map((row) => ({ id: row.tag, label: tagChipLabel(row) }))}
                      selected={tags}
                      onToggle={setTags}
                    />
                  ) : (
                    <p className={cn(MUTED, "mt-4")} data-no-citizen-read="">
                      {NO_CITIZEN_READ}
                    </p>
                  ))}

                {sourceKind === "directory" && kind === "staff_unit" && (
                  <Picker
                    label={PICK_UNIT}
                    state={units}
                    options={(u) => u.items.map((row) => ({ id: row.id, label: row.name }))}
                    selected={ids}
                    onToggle={setIds}
                  />
                )}

                <p className="m-0 mt-4 flex items-center gap-2 text-[12.5px]" data-row="recipients">
                  <span className="text-ink-muted">{ROW_RECIPIENTS}:</span>
                  <span className="text-ink font-semibold tabular-nums">{recipients()}</span>
                </p>

                <Button type="button" variant="primary" className="mt-4" onClick={() => setStep("template")}>
                  Tiếp tục
                </Button>
              </StepCard>

              <StepCard
                index={2}
                title="Mẫu tin"
                done={(smsFree ? freeAccount !== null : picked !== null) && step === "content"}
                open={step === "template"}
                onOpen={() => setStep("template")}
                summary={
                  smsFree
                    ? freeAccount === null
                      ? "Chưa chọn"
                      : `${CHANNEL_LABEL.sms} · ${smsAccountOptionLabel(freeAccount)}`
                    : picked === null
                      ? "Chưa chọn"
                      : templateName(picked)
                }
              >
                <p className="text-navy m-0 mb-1.5 text-[13px] font-semibold">{CHANNEL_PICK_LABEL}</p>
                <div className="mb-4 flex flex-wrap gap-1.5">
                  {(Object.keys(CHANNEL_LABEL) as Channel[]).map((c) => (
                    <Chip key={c} label={CHANNEL_LABEL[c]} active={channel === c} onClick={() => pickChannel(c)} />
                  ))}
                </div>
                {smsFree ? (
                  <FreeSmsAccount
                    templates={templates}
                    account={freeAccount}
                    accounts={freeAccounts}
                    onAccount={(id) => {
                      setSmsAccountId(id);
                      clearPreview();
                    }}
                    onNext={() => setStep("content")}
                  />
                ) : (
                <TemplateList
                  channel={channel}
                  templates={templates}
                  picked={picked}
                  smsAccountId={smsAccountId}
                  onSmsAccount={(id) => {
                    setSmsAccountId(id);
                    if (picked?.channel === "sms") {
                      setPicked(null);
                      clearPreview();
                    }
                  }}
                  onPick={pickTemplate}
                />
                )}
              </StepCard>

              <StepCard
                index={3}
                title="Nội dung"
                done={false}
                open={step === "content"}
                onOpen={() => setStep("content")}
                summary={title || "Chưa điền"}
              >
                {currentDraft === null ? (
                  <p className={MUTED}>{PICK_TEMPLATE_FIRST}</p>
                ) : (
                  <div className="flex flex-col gap-4">
                    <Field label={CAMPAIGN_TITLE_LABEL} htmlFor="notice-title" grow="auto">
                      <input
                        id="notice-title"
                        value={title}
                        maxLength={200}
                        autoComplete="off"
                        placeholder={title === "" ? defaultTitle() : undefined}
                        onChange={(e) => setTitle(e.target.value)}
                      />
                    </Field>
                    {smsFree && smsComposition !== null && (
                      <SmsImportContent
                        sourceKind={sourceKind === "typed" ? "typed" : "file"}
                        table={table}
                        composition={smsComposition}
                        onMode={setSmsMode}
                        onContentColumn={setContentColumn}
                        onContent={setSharedContent}
                        onVariableColumn={(n, col) => setVariableColumns((cur) => ({ ...cur, [n]: col }))}
                      />
                    )}
                    {!smsFree && !imported && picked !== null && picked.t.params.length > 0 && (
                      <div className="flex flex-col gap-1.5">
                        <div className="flex items-center gap-1.5">
                          <span className="text-navy text-[13px] leading-tight font-semibold">{PARAMS_SOURCE_TITLE}</span>
                          <Tooltip content={ATTRIBUTE_HELP}>
                            <button
                              type="button"
                              aria-label={ATTRIBUTE_HELP_LABEL}
                              aria-expanded={attributeHelpOpen}
                              aria-controls="notice-attribute-help"
                              data-attribute-help=""
                              onClick={() => setAttributeHelpOpen((o) => !o)}
                              className="border-line text-ink-muted hover:text-brand grid size-[18px] cursor-pointer place-items-center rounded-full border border-solid bg-white p-0 [font-family:inherit] text-[11px] font-bold"
                            >
                              ?
                            </button>
                          </Tooltip>
                        </div>
                        {attributeHelpOpen && (
                          <p id="notice-attribute-help" className={MUTED} data-attribute-help-text="">
                            {ATTRIBUTE_HELP}
                          </p>
                        )}
                      </div>
                    )}
                    {!smsFree &&
                      picked !== null &&
                      picked.t.params.map((p, i) => {
                        const fromFile = sourceKind === "file" && picked.channel === "zns" && table !== null;
                        const column = fromFile ? (paramColumns[p.field] ?? null) : null;
                        const paramSource: ParamSource = !imported && p.field in attributeChoice ? "attribute" : "shared";
                        return (
                          <div key={p.field} className="flex flex-wrap items-end gap-3" data-param={p.field}>
                            {!imported && (
                              <Segmented
                                legend={paramSourceLegend(paramLabel(picked.channel, p))}
                                name={`notice-param-source-${i}`}
                                mode="buttons"
                                value={paramSource}
                                options={PARAM_SOURCES.map((v) => ({ value: v, label: PARAM_SOURCE_LABEL[v] }))}
                                onChange={(v) =>
                                  setAttributeChoice((cur) => {
                                    const next = { ...cur };
                                    if (v === "attribute") next[p.field] = cur[p.field] ?? "";
                                    else delete next[p.field];
                                    return next;
                                  })
                                }
                              />
                            )}
                            {paramSource === "attribute" && (
                              <Field label={paramLabel(picked.channel, p)} htmlFor={`notice-param-attr-${i}`} kind="select" grow="auto" className="min-w-0">
                                <select
                                  id={`notice-param-attr-${i}`}
                                  value={attributeChoice[p.field] ?? ""}
                                  disabled={attributeList === undefined || !attributeList.ok}
                                  onChange={(e) => {
                                    const code = e.target.value;
                                    setAttributeChoice((cur) => ({ ...cur, [p.field]: code }));
                                  }}
                                >
                                  <option value="">{attributeList === undefined ? "…" : ATTRIBUTE_PICK}</option>
                                  {attributeList !== undefined &&
                                    attributeList.ok &&
                                    attributeList.duLieu.map((a) => (
                                      <option key={a.code} value={a.code}>
                                        {a.label}
                                      </option>
                                    ))}
                                </select>
                              </Field>
                            )}
                            {paramSource === "attribute" && attributeList !== undefined && !attributeList.ok && (
                              <p className={ALERT} role="alert" data-attributes-refused="">
                                {attributeList.thongBao}
                              </p>
                            )}
                            {fromFile && (
                              <Field label={`Nguồn · ${paramLabel(picked.channel, p)}`} htmlFor={`notice-param-src-${i}`} kind="select" grow="auto" className="min-w-0">
                                <select
                                  id={`notice-param-src-${i}`}
                                  value={column === null ? "" : String(column)}
                                  onChange={(e) => {
                                    const v = e.target.value;
                                    setParamColumns((cur) => ({ ...cur, [p.field]: v === "" ? null : Number(v) }));
                                  }}
                                >
                                  <option value="">{PARAM_SOURCE_SHARED}</option>
                                  {table.headers.map((_, col) => (
                                    <option key={col} value={String(col)}>
                                      {paramSourceColumn(table.headers, col)}
                                    </option>
                                  ))}
                                </select>
                              </Field>
                            )}
                            {column === null && paramSource === "shared" && (
                              <Field label={paramLabel(picked.channel, p)} htmlFor={`notice-param-${i}`} grow="auto">
                                <input
                                  id={`notice-param-${i}`}
                                  value={params[p.field] ?? ""}
                                  maxLength={p.max_length > 0 ? p.max_length : undefined}
                                  autoComplete="off"
                                  onChange={(e) => setParams((cur) => ({ ...cur, [p.field]: e.target.value }))}
                                />
                              </Field>
                            )}
                          </div>
                        );
                      })}
                    {errors.length > 0 && (
                      <ul className="m-0 flex list-none flex-col gap-1 p-0" data-import-errors="">
                        {errors.map((e) => (
                          <li key={e} className={ALERT} role="alert">
                            {e}
                          </li>
                        ))}
                      </ul>
                    )}
                    <div>
                      <Button
                        type="button"
                        variant="outline"
                        disabled={errors.length > 0 || (preview !== null && preview.result === null)}
                        icon={preview !== null && preview.result === null ? <Loader2 aria-hidden="true" focusable="false" className="animate-spin" /> : undefined}
                        onClick={() => void runPreview()}
                      >
                        {PREVIEW_BUTTON}
                      </Button>
                    </div>
                  </div>
                )}
              </StepCard>
            </div>

            <aside className={cn(CARD, "h-fit p-5")} aria-labelledby="notice-preview-title" data-preview="">
              <h2 id="notice-preview-title" className="text-navy m-0 text-[14px] font-bold">
                {PREVIEW_TITLE}
              </h2>
              {currentDraft === null || preview === null ? (
                <p className={cn(MUTED, "mt-3")}>{PREVIEW_EMPTY}</p>
              ) : preview.result === null && shown === null ? (
                <div className="mt-3 flex flex-col gap-2" role="status" aria-label="Đang xem trước">
                  <Skeleton className="h-16 w-full" />
                  <Skeleton className="h-5 w-full" />
                  <Skeleton className="h-5 w-full" />
                </div>
              ) : preview.result !== null && !preview.result.ok ? (
                <p className={cn(ALERT, "mt-3")} role="alert" data-preview-refused="">
                  {preview.result.thongBao}
                </p>
              ) : shown !== null ? (
                <>
                  <p
                    className="border-line bg-surface text-ink m-0 mt-3 rounded-[10px] border border-solid p-3 text-[12.5px] whitespace-pre-wrap"
                    data-preview-content=""
                  >
                    {shown.content ?? ZNS_CONTENT}
                  </p>
                  <dl className="m-0 mt-4 flex flex-col gap-2 text-[12.5px]">
                    <Row label={ROW_RECIPIENTS} value={recipientsText(shown.recipient_count)} dataName="preview-recipients" />
                    {shown.skipped_count > 0 && (
                      <Row label={ROW_SKIPPED} value={recipientsText(shown.skipped_count)} tone="warn" dataName="preview-skipped" />
                    )}
                    {imported && (
                      <>
                        <Row
                          label={ROW_INVALID}
                          value={(shown.invalid_count ?? 0).toLocaleString("vi-VN")}
                          tone={(shown.invalid_count ?? 0) > 0 ? "warn" : undefined}
                          dataName="preview-invalid"
                        />
                        <Row label={ROW_DUPLICATE} value={(shown.duplicate_count ?? 0).toLocaleString("vi-VN")} dataName="preview-duplicate" />
                      </>
                    )}
                    <Row label={ROW_UNIT_PRICE} value={moneyOrNotPriced(shown.unit_price)} dataName="preview-unit-price" />
                    <Row label={ROW_COST} value={moneyOrNotPriced(shown.estimated_cost)} strong dataName="preview-cost" />
                  </dl>
                  {shown.missing_params.length > 0 && (
                    <p className="border-tangerine/30 bg-tangerine/8 text-navy m-0 mt-3 rounded-[10px] border border-solid p-3 text-[12px]" data-missing-params="">
                      {missingParamsText(shown.missing_params.map((f) => missingLabel(channel, f)))}
                    </p>
                  )}
                  {stale && (
                    <p className="text-tangerine m-0 mt-3 text-[12px] font-medium" data-preview-stale="">
                      {PREVIEW_STALE}
                    </p>
                  )}
                  <Button
                    type="button"
                    variant="primary"
                    className="mt-4 w-full"
                    data-send=""
                    disabled={sending || stale || preview.result === null || shown.recipient_count === 0 || shown.missing_params.length > 0}
                    aria-busy={sending}
                    icon={
                      sending ? (
                        <Loader2 aria-hidden="true" focusable="false" className="animate-spin" />
                      ) : (
                        <Send aria-hidden="true" focusable="false" />
                      )
                    }
                    onClick={() => void send()}
                  >
                    {SEND_BUTTON}
                  </Button>
                  <p className="text-ink-muted m-0 mt-2 text-[11.5px]">{SEND_NOTE}</p>
                </>
              ) : null}
            </aside>
          </div>
        ) : (
          openCampaign !== null ? (
            <CampaignRecipients campaign={openCampaign} onBack={() => setOpenCampaign(null)} />
          ) : (
            <HistoryTable
              history={history}
              loadingMore={loadingMore}
              moreRefused={moreRefused}
              onMore={() => void loadMore()}
              onOpen={setOpenCampaign}
            />
          )
        )}
      </div>
    </div>
  );
}

/** Step 2's list for the picked channel: loading, refused (the server's sentence), empty, or cards. */
function TemplateList({
  channel,
  templates,
  picked,
  smsAccountId,
  onSmsAccount,
  onPick,
}: {
  channel: Channel;
  templates: KetQua<NoticeTemplates> | null;
  picked: PickedTemplate | null;
  smsAccountId: string;
  onSmsAccount: (id: string) => void;
  onPick: (p: PickedTemplate) => void;
}) {
  if (templates === null) return <Skeleton className="h-24 w-full" />;
  if (!templates.ok) {
    return (
      <p className={ALERT} role="alert">
        {templates.thongBao}
      </p>
    );
  }
  const pickedKey = picked === null ? null : templateKey(picked);
  const empty = (href: string, label: string) => (
    <div className="flex flex-col gap-2" data-templates-empty="">
      <p className={MUTED}>{NO_TEMPLATES}</p>
      <Link href={href} className={LINK}>
        {label}
      </Link>
    </div>
  );

  if (channel === "zns") {
    const groups = groupByOA(templates.duLieu.zns);
    if (groups.length === 0) return empty(ZALO_CONFIG_HREF, OPEN_ZALO_CONFIG);
    const several = groups.length >= 2;
    return (
      <div className="flex flex-col gap-4">
        {groups.map((g) => (
          <div key={g.oaId} className="flex flex-col gap-2" data-oa-group={g.oaId}>
            {several && <p className="text-navy m-0 text-[12.5px] font-semibold">{g.oaName}</p>}
            {g.items.map((t) => {
              const p: PickedTemplate = { channel: "zns", t };
              const usable = !t.oa_needs_reauthorization;
              return (
                <TemplateCard
                  key={templateKey(p)}
                  id={templateKey(p)}
                  selected={pickedKey === templateKey(p)}
                  disabled={!usable}
                  disabledReason={usable ? undefined : OA_REAUTH_REASON}
                  name={t.template_name}
                  tag={usable ? undefined : OA_REAUTH_TAG}
                  body={paramsLine(t.params.map((x) => x.name))}
                  subline={znsSubline(t, several)}
                  onClick={() => onPick(p)}
                />
              );
            })}
          </div>
        ))}
      </div>
    );
  }

  const accounts = smsAccounts(templates.duLieu.sms);
  if (accounts.length === 0) return empty(SMS_CONFIG_HREF, OPEN_SMS_CONFIG);
  const account = accounts.some((a) => a.sms_account_id === smsAccountId) ? smsAccountId : accounts[0]!.sms_account_id;
  const list = templates.duLieu.sms.filter((t) => t.sms_account_id === account);
  return (
    <div className="flex flex-col gap-3">
      {accounts.length >= 2 && (
        <Field label={SMS_ACCOUNT_PICK_LABEL} htmlFor="notice-sms-account" kind="select" grow="auto" className="min-w-0">
          <select id="notice-sms-account" value={account} onChange={(e) => onSmsAccount(e.target.value)}>
            {accounts.map((a) => (
              <option key={a.sms_account_id} value={a.sms_account_id}>
                {smsAccountOptionLabel(a)}
              </option>
            ))}
          </select>
        </Field>
      )}
      <div className="flex flex-col gap-2">
        {list.map((t) => {
          const p: PickedTemplate = { channel: "sms", t };
          return (
            <TemplateCard
              key={templateKey(p)}
              id={templateKey(p)}
              selected={pickedKey === templateKey(p)}
              disabled={false}
              body={t.content}
              subline={smsSubline(t)}
              onClick={() => onPick(p)}
            />
          );
        })}
      </div>
    </div>
  );
}

/** The prototype's template card: name (and a tag when it cannot be picked), body, channel line. */
function TemplateCard({
  id,
  selected,
  disabled,
  disabledReason,
  name,
  tag,
  body,
  subline,
  onClick,
}: {
  id: string;
  selected: boolean;
  disabled: boolean;
  disabledReason?: string;
  name?: string;
  tag?: string;
  body: string;
  subline: string;
  onClick: () => void;
}) {
  const reasonId = disabledReason === undefined ? undefined : `${id}-reason`;
  return (
    <button
      type="button"
      disabled={disabled}
      aria-pressed={selected}
      aria-describedby={reasonId}
      data-template-id={id}
      onClick={onClick}
      className={cn(
        "w-full cursor-pointer rounded-[10px] border border-solid p-3 text-left [font-family:inherit] transition-colors",
        selected ? "border-brand/40 bg-brand/5" : "border-line hover:border-brand/30 bg-white",
        disabled && "cursor-not-allowed opacity-50 hover:border-line",
      )}
    >
      {name !== undefined && (
        <span className="text-navy block text-[13px] font-semibold">
          {name}
          {tag !== undefined && <span className="text-tangerine ml-2 text-[11px] font-semibold">{tag}</span>}
        </span>
      )}
      <span className={cn("text-ink-muted line-clamp-2 block text-[12px] break-words", name !== undefined && "mt-1")}>{body}</span>
      <span className="text-ink-muted mt-1 block text-[11.5px]">{subline}</span>
      {reasonId !== undefined && (
        <span id={reasonId} className="text-ink-muted mt-1 block text-[11.5px]">
          {disabledReason}
        </span>
      )}
    </button>
  );
}

const STATUS_BADGE = { success: "success", danger: "danger", warn: "warning", neutral: "neutral" } as const;

/** The "Đã gửi" tab — prototype :398-457: five columns, newest first, "Tải thêm" while pages remain. */
function HistoryTable({
  history,
  loadingMore,
  moreRefused,
  onMore,
  onOpen,
}: {
  history: KetQua<History> | null;
  loadingMore: boolean;
  moreRefused: string | null;
  onMore: () => void;
  onOpen: (c: NoticeCampaign) => void;
}) {
  if (history === null) {
    return (
      <div className={cn(CARD, "p-5")} data-history="">
        <Skeleton className="h-24 w-full" />
      </div>
    );
  }
  if (!history.ok) {
    return (
      <div className={cn(CARD, "p-5")} data-history="">
        <p className={ALERT} role="alert">
          {history.thongBao}
        </p>
      </div>
    );
  }
  const { items, next } = history.duLieu;
  return (
    <div className={CARD} data-history="">
      {items.length === 0 ? (
        <p className="text-ink-muted m-0 p-8 text-center text-[12.5px]">{HISTORY_EMPTY}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[40rem] border-collapse text-[12.5px]">
            <thead>
              <tr className="text-ink-muted border-line border-b border-solid text-left text-[11px] uppercase">
                {HISTORY_COLUMNS.map((c) => (
                  <th key={c} scope="col" className="px-4 py-2.5 font-semibold">
                    {c}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={row.id} className="border-line border-b border-solid last:border-b-0" data-campaign-id={row.id}>
                  <td className="px-4 py-2.5">
                    <button
                      type="button"
                      title={OPEN_CAMPAIGN_HINT}
                      data-open-campaign={row.id}
                      onClick={() => onOpen(row)}
                      className="text-navy hover:text-brand m-0 cursor-pointer border-0 bg-transparent p-0 text-left [font-family:inherit] text-[12.5px] font-semibold break-words underline-offset-2 hover:underline"
                    >
                      {row.title}
                    </button>
                    <p className="text-ink-muted m-0 text-[11.5px]">{campaignSubline(row)}</p>
                  </td>
                  <td className="px-4 py-2.5">{audienceKindLabel(row.audience_kind)}</td>
                  <td className="px-4 py-2.5 tabular-nums">
                    {row.recipient_count.toLocaleString("vi-VN")}
                    {row.invalid_count > 0 && (
                      <p className="text-tangerine m-0 text-[11.5px]" data-invalid-count="">
                        {invalidCountText(row.invalid_count)}
                      </p>
                    )}
                  </td>
                  <td className="px-4 py-2.5 tabular-nums">{moneyOrNotPriced(row.estimated_cost)}</td>
                  <td className="px-4 py-2.5">
                    <Badge tone={STATUS_BADGE[statusTone(row.status)]} icon={null} data-status={row.status}>
                      {statusLabel(row)}
                    </Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {(next !== "" || moreRefused !== null) && (
        <div className="border-line flex flex-col items-center gap-2 border-t border-solid p-3">
          {moreRefused !== null && (
            <p className={ALERT} role="alert">
              {moreRefused}
            </p>
          )}
          {next !== "" && (
            <Button type="button" variant="outline" size="sm" disabled={loadingMore} aria-busy={loadingMore} onClick={onMore}>
              {LOAD_MORE}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

function StepCard({
  index,
  title,
  summary,
  done,
  open,
  onOpen,
  children,
}: {
  readonly index: number;
  readonly title: string;
  readonly summary: string;
  readonly done: boolean;
  readonly open: boolean;
  readonly onOpen: () => void;
  readonly children: ReactNode;
}) {
  return (
    <section className={CARD} data-step={index}>
      <button
        type="button"
        onClick={onOpen}
        aria-expanded={open}
        className="flex w-full cursor-pointer items-center gap-3 border-0 bg-transparent px-5 py-3 text-left [font-family:inherit]"
      >
        <span
          className={cn(
            "grid size-6 shrink-0 place-items-center rounded-full text-[12px] font-bold",
            open || done ? "bg-brand text-white" : "bg-surface text-ink-muted",
          )}
        >
          {index}
        </span>
        <span className="min-w-0 flex-1">
          <span className="text-navy block text-[13.5px] font-bold">{title}</span>
          {!open && <span className="text-ink-muted block truncate text-[12px]">{summary}</span>}
        </span>
      </button>
      {open && <div className="border-line border-t border-solid px-5 py-4">{children}</div>}
    </section>
  );
}

function Chip({ label, active, onClick }: { readonly label: string; readonly active: boolean; readonly onClick: () => void }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        "cursor-pointer rounded-full border border-solid px-3 py-1.5 [font-family:inherit] text-[12px] font-semibold",
        active ? "border-brand/30 bg-brand/10 text-brand" : "border-line text-ink-muted bg-white",
      )}
    >
      {label}
    </button>
  );
}

/** The prototype's `Picker`, fed by a server read: loading, refused (the server's sentence), empty, or chips. */
function Picker<T>({
  label,
  state,
  options,
  selected,
  onToggle,
}: {
  readonly label: string;
  readonly state: KetQua<T> | null;
  readonly options: (data: T) => { id: string; label: string }[];
  readonly selected: readonly string[];
  readonly onToggle: (next: string[]) => void;
}) {
  let body: ReactNode;
  if (state === null) body = <Skeleton className="h-7 w-full" />;
  else if (!state.ok)
    body = (
      <p className={ALERT} role="alert">
        {state.thongBao}
      </p>
    );
  else {
    const list = options(state.duLieu);
    body =
      list.length === 0 ? (
        <p className={MUTED}>{PICKER_EMPTY}</p>
      ) : (
        list.map((o) => {
          const on = selected.includes(o.id);
          return (
            <Chip
              key={o.id}
              label={o.label}
              active={on}
              onClick={() => onToggle(on ? selected.filter((id) => id !== o.id) : [...selected, o.id])}
            />
          );
        })
      );
  }
  return (
    <div className="mt-4">
      <p className="text-navy m-0 mb-1.5 text-[13px] font-semibold">{label}</p>
      <div className="flex flex-wrap gap-1.5">{body}</div>
    </div>
  );
}

function Row({
  label,
  value,
  strong,
  tone,
  dataName,
}: {
  label: string;
  value: ReactNode;
  strong?: boolean;
  tone?: "warn";
  dataName?: string;
}) {
  return (
    <div className="flex items-center justify-between gap-3" data-row={dataName}>
      <dt className="text-ink-muted">{label}</dt>
      <dd
        className={cn(
          "m-0 tabular-nums",
          strong ? "text-navy text-[14px] font-bold" : "text-ink font-semibold",
          tone === "warn" && "text-tangerine",
        )}
      >
        {value}
      </dd>
    </div>
  );
}

// ---- the imported sources (ADR 0099 §Sửa đổi S1–S4) ---------------------------------------------------

/** "Nhập số điện thoại": one per line or comma separated, the live count beside it. */
function TypedPhones({
  text,
  count,
  error,
  onChange,
}: {
  text: string;
  count: number;
  error: string | undefined;
  onChange: (v: string) => void;
}) {
  return (
    <div className="flex flex-col gap-1.5" data-typed-phones="">
      <Field label={TYPED_LABEL} htmlFor="notice-typed-phones" grow="auto" hint={TYPED_HINT} error={error}>
        <textarea
          id="notice-typed-phones"
          rows={6}
          value={text}
          autoComplete="off"
          spellCheck={false}
          className="py-2"
          onChange={(e) => onChange(e.target.value)}
        />
      </Field>
      <p className={MUTED} data-typed-count="">
        {typedCountText(count)}
      </p>
    </div>
  );
}

/**
 * "Nhập tệp": the picker, then the file's name and rows, the phone column, and the first rows with
 * every number MASKED. The `<input type="file">` is emptied after each read — nothing keeps the file.
 */
function FileImport({
  table,
  error,
  reading,
  phoneColumn,
  onFile,
  onPhoneColumn,
}: {
  table: ImportedTable | null;
  error: string | null;
  reading: boolean;
  phoneColumn: number | null;
  onFile: (e: ChangeEvent<HTMLInputElement>) => void;
  onPhoneColumn: (col: number | null) => void;
}) {
  return (
    <div className="flex flex-col gap-3" data-file-import="">
      <Field label={FILE_LABEL} htmlFor="notice-file" grow="auto" hint={FILE_HINT} error={error ?? undefined}>
        <input id="notice-file" type="file" accept=".xlsx,.csv" disabled={reading} aria-busy={reading} onChange={onFile} />
      </Field>
      {reading && <p className={MUTED}>Đang đọc tệp…</p>}
      {table !== null && (
        <>
          <p className="text-ink m-0 text-[12.5px] font-semibold" data-file-summary="">
            {fileSummary(table)}
          </p>
          <ColumnSelect id="notice-phone-column" label={PHONE_COLUMN_LABEL} headers={table.headers} value={phoneColumn} onChange={onPhoneColumn} />
          <div className="border-line overflow-x-auto rounded-[10px] border border-solid" data-file-preview="">
            <table className="w-full border-collapse text-[12px]">
              <thead>
                <tr className="text-ink-muted border-line border-b border-solid text-left">
                  {table.headers.map((_, i) => (
                    <th key={i} scope="col" className="px-3 py-2 font-semibold whitespace-nowrap">
                      {columnLabel(table.headers, i)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {maskedPreview(table, phoneColumn).map((r, ri) => (
                  <tr key={ri} className="border-line border-b border-solid last:border-b-0">
                    {r.map((c, ci) => (
                      <td key={ci} className="max-w-[16rem] truncate px-3 py-2 tabular-nums">
                        {c}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  );
}

function ColumnSelect({
  id,
  label,
  headers,
  value,
  onChange,
}: {
  id: string;
  label: ReactNode;
  headers: readonly string[];
  value: number | null;
  onChange: (col: number | null) => void;
}) {
  return (
    <Field label={label} htmlFor={id} kind="select" grow="auto" className="min-w-0">
      <select id={id} value={value === null ? "" : String(value)} onChange={(e) => onChange(e.target.value === "" ? null : Number(e.target.value))}>
        <option value="">{PICK_COLUMN}</option>
        {headers.map((_, i) => (
          <option key={i} value={String(i)}>
            {columnLabel(headers, i)}
          </option>
        ))}
      </select>
    </Field>
  );
}

/** Step 2 for SMS to an imported list: the account it leaves from — no template card (S3). */
function FreeSmsAccount({
  templates,
  account,
  accounts,
  onAccount,
  onNext,
}: {
  templates: KetQua<NoticeTemplates> | null;
  account: SmsNoticeTemplate | null;
  accounts: readonly SmsNoticeTemplate[];
  onAccount: (id: string) => void;
  onNext: () => void;
}) {
  if (templates === null) return <Skeleton className="h-16 w-full" />;
  if (!templates.ok) {
    return (
      <p className={ALERT} role="alert">
        {templates.thongBao}
      </p>
    );
  }
  if (account === null) {
    return (
      <div className="flex flex-col gap-2" data-templates-empty="">
        <p className={MUTED}>{NO_TEMPLATES}</p>
        <Link href={SMS_CONFIG_HREF} className={LINK}>
          {OPEN_SMS_CONFIG}
        </Link>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-3" data-free-sms="">
      <Field label={SMS_ACCOUNT_PICK_LABEL} htmlFor="notice-sms-account" kind="select" grow="auto" className="min-w-0">
        <select id="notice-sms-account" value={account.sms_account_id} onChange={(e) => onAccount(e.target.value)}>
          {accounts.map((a) => (
            <option key={a.sms_account_id} value={a.sms_account_id}>
              {smsAccountOptionLabel(a)}
            </option>
          ))}
        </select>
      </Field>
      <p className={MUTED}>{SMS_NO_TEMPLATE_NOTE}</p>
      <div>
        <Button type="button" variant="primary" onClick={onNext}>
          Tiếp tục
        </Button>
      </div>
    </div>
  );
}

/**
 * Step 3 for SMS to an imported list (S3): from a file, a column per row or one shared text; from a
 * typed list, the shared text only. The shared text may carry `{{variable_N}}` — each N found gets a
 * column select. The "?" explains it on hover AND on press: a tooltip never holds the only copy, and
 * there is no hover on a phone (`tooltip.tsx`).
 */
function SmsImportContent({
  sourceKind,
  table,
  composition,
  onMode,
  onContentColumn,
  onContent,
  onVariableColumn,
}: {
  sourceKind: "typed" | "file";
  table: ImportedTable | null;
  composition: SmsImportComposition;
  onMode: (m: SmsContentMode) => void;
  onContentColumn: (col: number | null) => void;
  onContent: (v: string) => void;
  onVariableColumn: (n: number, col: number | null) => void;
}) {
  const [helpOpen, setHelpOpen] = useState(false);
  const fileTable = sourceKind === "file" ? table : null;
  const shared = composition.mode === "noi-dung-chung";
  const vars = fileTable !== null && shared ? detectVariables(composition.content) : [];
  const length = composition.content.length;
  return (
    <div className="flex flex-col gap-3" data-sms-import="">
      {fileTable !== null && (
        <Segmented
          legend={SMS_MODE_LEGEND}
          showLegend
          name="notice-sms-mode"
          value={composition.mode}
          options={(Object.keys(SMS_MODE_LABEL) as SmsContentMode[]).map((m) => ({ value: m, label: SMS_MODE_LABEL[m] }))}
          onChange={(v) => onMode(v as SmsContentMode)}
        />
      )}
      {fileTable !== null && !shared && (
        <ColumnSelect
          id="notice-content-column"
          label={CONTENT_COLUMN_LABEL}
          headers={fileTable.headers}
          value={composition.contentColumn}
          onChange={onContentColumn}
        />
      )}
      {shared && (
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center gap-1.5">
            <label htmlFor="notice-sms-content" className="text-navy block text-[13px] leading-tight font-semibold">
              {SHARED_CONTENT_LABEL}
            </label>
            {fileTable !== null && (
              <Tooltip content={VARIABLES_HELP}>
                <button
                  type="button"
                  aria-label={VARIABLES_HELP_LABEL}
                  aria-expanded={helpOpen}
                  aria-controls="notice-variables-help"
                  data-variables-help=""
                  onClick={() => setHelpOpen((o) => !o)}
                  className="border-line text-ink-muted hover:text-brand grid size-[18px] cursor-pointer place-items-center rounded-full border border-solid bg-white p-0 [font-family:inherit] text-[11px] font-bold"
                >
                  ?
                </button>
              </Tooltip>
            )}
          </div>
          {helpOpen && fileTable !== null && (
            <p id="notice-variables-help" className={MUTED} data-variables-help-text="">
              {VARIABLES_HELP}
            </p>
          )}
          <Field label={SHARED_CONTENT_LABEL} hideLabel htmlFor="notice-sms-content-frame" grow="auto">
            <textarea
              id="notice-sms-content"
              rows={4}
              value={composition.content}
              autoComplete="off"
              className="py-2"
              onChange={(e) => onContent(e.target.value)}
            />
          </Field>
          <p className={cn(MUTED, length > MAX_CONTENT_LENGTH && "text-danger")} data-char-counter="">
            {charCounterText(length)}
          </p>
        </div>
      )}
      {vars.length > 0 && fileTable !== null && (
        <div className="flex flex-col gap-2" data-variables="">
          <p className="text-navy m-0 text-[13px] font-semibold">{VARIABLES_TITLE}</p>
          {vars.map((n) => (
            <ColumnSelect
              key={n}
              id={`notice-variable-${n}`}
              label={variableToken(n)}
              headers={fileTable.headers}
              value={composition.variableColumns[n] ?? null}
              onChange={(col) => onVariableColumn(n, col)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

// ---- one campaign's recipients (S5) -----------------------------------------------------------------

/**
 * One campaign's recipients: the number MASKED by comms, the result in words, the provider's error for a
 * failure, the time. Read page by page ("Tải thêm"); nothing here holds a full number.
 */
function CampaignRecipients({ campaign, onBack }: { campaign: NoticeCampaign; onBack: () => void }) {
  const [page, setPage] = useState<KetQua<{ items: NoticeRecipient[]; next: string }> | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreRefused, setMoreRefused] = useState<string | null>(null);
  const [attributes, setAttributes] = useState<NoticeAttribute[]>([]);

  // The labels of a directory campaign's mapped attributes. Not readable = the codes stand in: the code is
  // what the campaign stored, the label only a courtesy.
  const mapped = Object.keys(campaign.attribute_mapping ?? {}).length > 0;
  const attrAudience = attributeAudienceOfWire(campaign.audience_kind);
  useEffect(() => {
    if (!mapped || attrAudience === null) return;
    let gone = false;
    listNoticeAttributes(attrAudience).then((r) => {
      if (!gone && r.ok) setAttributes(r.duLieu.items);
    });
    return () => {
      gone = true;
    };
  }, [mapped, attrAudience]);
  const attributeRows = attributeMappingRows(campaign.channel === "sms" ? "sms" : "zalo-zns", campaign.attribute_mapping, attributes);

  useEffect(() => {
    let gone = false;
    listNoticeRecipients(campaign.id).then((r) => {
      if (!gone) setPage(r.ok ? { ok: true, duLieu: { items: r.duLieu.items, next: r.duLieu.next_cursor } } : r);
    });
    return () => {
      gone = true;
    };
  }, [campaign.id]);

  async function more() {
    if (page === null || !page.ok || page.duLieu.next === "") return;
    setLoadingMore(true);
    const r = await listNoticeRecipients(campaign.id, page.duLieu.next);
    setLoadingMore(false);
    if (!r.ok) {
      setMoreRefused(r.thongBao);
      return;
    }
    setMoreRefused(null);
    setPage((cur) =>
      cur !== null && cur.ok ? { ok: true, duLieu: { items: [...cur.duLieu.items, ...r.duLieu.items], next: r.duLieu.next_cursor } } : cur,
    );
  }

  return (
    <div className={CARD} data-campaign-recipients={campaign.id}>
      <div className="border-line flex flex-wrap items-center justify-between gap-3 border-b border-solid px-4 py-3">
        <div className="min-w-0">
          <p className="text-ink-muted m-0 text-[11.5px]">{RECIPIENTS_TITLE}</p>
          <h2 className="text-navy m-0 text-[14px] font-bold break-words">{campaign.title}</h2>
          <p className="text-ink-muted m-0 text-[11.5px]">
            {audienceKindLabel(campaign.audience_kind)} · {recipientsText(campaign.recipient_count)}
            {campaign.invalid_count > 0 ? ` · ${invalidCountText(campaign.invalid_count)}` : ""}
          </p>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={onBack}>
          {BACK_TO_HISTORY}
        </Button>
      </div>
      {((campaign.recipient_mapping !== null && campaign.recipient_mapping !== undefined) || attributeRows.length > 0) && (
        <MappingBlock campaign={campaign} mapping={campaign.recipient_mapping ?? null} attributeRows={attributeRows} />
      )}
      {page === null ? (
        <div className="p-5">
          <Skeleton className="h-24 w-full" />
        </div>
      ) : !page.ok ? (
        <p className={cn(ALERT, "p-5")} role="alert">
          {page.thongBao}
        </p>
      ) : page.duLieu.items.length === 0 ? (
        <p className="text-ink-muted m-0 p-8 text-center text-[12.5px]">{RECIPIENTS_EMPTY}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[36rem] border-collapse text-[12.5px]">
            <thead>
              <tr className="text-ink-muted border-line border-b border-solid text-left text-[11px] uppercase">
                {RECIPIENT_COLUMNS.map((c) => (
                  <th key={c} scope="col" className="px-4 py-2.5 font-semibold">
                    {c}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {page.duLieu.items.map((r) => (
                <tr key={r.id} className="border-line border-b border-solid last:border-b-0" data-recipient-id={r.id}>
                  <td className="px-4 py-2.5 tabular-nums">{r.phone_masked}</td>
                  <td className="px-4 py-2.5">
                    <Badge tone={STATUS_BADGE[recipientStatusTone(r.status)]} icon={null} data-recipient-status={r.status}>
                      {recipientStatusLabel(r.status)}
                    </Badge>
                  </td>
                  <td className="text-ink-muted px-4 py-2.5 break-words">{r.error_message}</td>
                  <td className="px-4 py-2.5 whitespace-nowrap tabular-nums">{recipientTimeText(r)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {page !== null && page.ok && (page.duLieu.next !== "" || moreRefused !== null) && (
        <div className="border-line flex flex-col items-center gap-2 border-t border-solid p-3">
          {moreRefused !== null && (
            <p className={ALERT} role="alert">
              {moreRefused}
            </p>
          )}
          {page.duLieu.next !== "" && (
            <Button type="button" variant="outline" size="sm" disabled={loadingMore} aria-busy={loadingMore} onClick={() => void more()}>
              {LOAD_MORE}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * "Thông tin map biến": which file column fed the phone, the content, each variable or parameter — and,
 * for a directory campaign, which attribute fed each parameter (ADR 0101 #12).
 */
function MappingBlock({
  campaign,
  mapping,
  attributeRows,
}: {
  campaign: NoticeCampaign;
  mapping: NoticeRecipientMapping | null;
  attributeRows: readonly (readonly [string, string])[];
}) {
  return (
    <section className="border-line flex flex-col gap-2 border-b border-solid px-4 py-3 text-[12.5px]" data-mapping="">
      <h3 className="text-navy m-0 text-[13px] font-bold">{MAPPING_TITLE}</h3>
      {mapping !== null && <FileMapping campaign={campaign} mapping={mapping} />}
      {attributeRows.length > 0 && <PairTable columns={ATTRIBUTE_MAPPING_COLUMNS} rows={attributeRows} name="attributes" />}
    </section>
  );
}

function FileMapping({ campaign, mapping }: { campaign: NoticeCampaign; mapping: NoticeRecipientMapping }) {
  const variables = Object.entries(mapping.variables ?? {});
  const params = Object.entries(mapping.params ?? {});
  const isSms = campaign.channel === "sms";
  return (
    <>
      <dl className="m-0 flex flex-col gap-1.5">
        <MappingRow label={MAPPING_PHONE} value={mapping.phone_column} name="phone" />
        {mapping.content_column !== undefined && <MappingRow label={MAPPING_CONTENT_COLUMN} value={mapping.content_column} name="content-column" />}
        {isSms && campaign.sms_content_mode === "noi-dung-chung" && campaign.content !== null && (
          <MappingRow label={MAPPING_SHARED_CONTENT} value={<span className="whitespace-pre-wrap">{campaign.content}</span>} name="shared-content" />
        )}
      </dl>
      {variables.length > 0 && <PairTable columns={MAPPING_VARIABLE_COLUMNS} rows={variables.map(([k, v]) => [mappingVariableLabel(k), v])} name="variables" />}
      {!isSms &&
        (params.length > 0 ? (
          <>
            <PairTable columns={MAPPING_PARAM_COLUMNS} rows={params} name="params" />
            <p className={MUTED}>{MAPPING_OTHER_PARAMS_SHARED}</p>
          </>
        ) : (
          <p className={MUTED}>{MAPPING_PARAMS_SHARED}</p>
        ))}
    </>
  );
}

function MappingRow({ label, value, name }: { label: string; value: ReactNode; name: string }) {
  return (
    <div className="flex flex-wrap gap-x-3" data-mapping-row={name}>
      <dt className="text-ink-muted">{label}:</dt>
      <dd className="text-ink m-0 font-semibold break-words">{value}</dd>
    </div>
  );
}

function PairTable({ columns, rows, name }: { columns: readonly [string, string]; rows: readonly (readonly [string, string])[]; name: string }) {
  return (
    <table className="border-line w-full max-w-md border-collapse border border-solid text-[12px]" data-mapping-table={name}>
      <thead>
        <tr className="text-ink-muted border-line border-b border-solid text-left">
          {columns.map((c) => (
            <th key={c} scope="col" className="px-3 py-1.5 font-semibold">
              {c}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map(([a, b]) => (
          <tr key={a} className="border-line border-b border-solid last:border-b-0">
            <td className="px-3 py-1.5">{a}</td>
            <td className="px-3 py-1.5">{b}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
