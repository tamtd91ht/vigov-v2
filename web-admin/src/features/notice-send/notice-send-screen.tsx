"use client";

import { Send } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type KeyboardEvent, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { PageHeader } from "@/components/ui/page-header";
import { PendingButton, PendingMarker, type PendingFeatureInfo } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import { Tab, TabList } from "@/components/ui/tabs";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { CongQuyen } from "@/features/quyen/cong-quyen";
import { countCitizenContacts, listCitizenContactTags, NO_CITIZEN_CONTACT_FILTER, type CitizenContactTag } from "@/lib/api/citizen-contacts";
import { listCitizenGroups, type CitizenGroup } from "@/lib/api/citizen-groups";
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi"; // vi-name-ok: existing export of danh-ba-chon-nguoi.ts, imported not declared (rule 12 inv 3)
import { layDanhMucBoPhan } from "@/lib/api/danh-muc"; // vi-name-ok: existing export of danh-muc.ts, imported not declared (rule 12 inv 3)
// vi-name-ok: the existing result type of lib/api/goi.ts, imported, not a new name
import type { KetQua } from "@/lib/api/goi";
import type { identity_boPhanRa, identity_canBoChonNguoiRa } from "@/lib/api/schema.gen";
import { listZaloOATemplates, type comms_zaloOATemplateList, type comms_zaloOATemplateOut } from "@/lib/api/zalo-oa";
import { cn } from "@/lib/cn";
import { CITIZEN_READ_PERMISSION, NOTICE_SEND_PERMISSION, quyetDinhTheoKhoa, ZALO_CHANNEL_PERMISSION } from "@/lib/quyen";

import {
  AUDIENCE_KINDS,
  AUDIENCE_LABEL,
  CAMPAIGN_TITLE_LABEL,
  CHANNEL_LABEL,
  CHANNEL_PICK_LABEL,
  CONTENT_PENDING,
  COST_PENDING,
  countStaff,
  groupChipLabel,
  HISTORY_EMPTY,
  HISTORY_PENDING,
  isStaffAudience,
  isTemplateUsable,
  missingParams,
  missingParamsText,
  NO_CITIZEN_READ,
  NO_SEND_PERMISSION,
  NO_TEMPLATE_PERMISSION,
  NO_TEMPLATES,
  NOTICE_SEND_TITLE,
  OPEN_ZALO_CONFIG,
  paramsLine,
  PICK_GROUP,
  PICK_TAG,
  PICK_TEMPLATE_FIRST,
  PICK_UNIT,
  PICKER_EMPTY,
  PREVIEW_EMPTY,
  PREVIEW_PENDING,
  PREVIEW_TITLE,
  PRICE_PENDING,
  recipientPendingInfo,
  recipientSource,
  recipientsText,
  ROW_COST,
  ROW_RECIPIENTS,
  ROW_UNIT_PRICE,
  SEND_BUTTON,
  SEND_NOTE,
  SEND_PENDING,
  SMS_PENDING,
  TAB_COMPOSE,
  TAB_HISTORY,
  tagChipLabel,
  templateSubline,
  zaloStatusText,
  ZALO_CONFIG_HREF,
  type AudienceKind,
  type Channel,
} from "./notice-send-form";
import { OAStatusCard, sendingOAs, useCommuneOA } from "./oa-status-card";

/**
 * `/gui-thong-bao` — "Gửi tin ZNS / SMS", the prototype's `NoticeWorkspace` (owner, 17/10/2026: the
 * prototype's screen first, with the OA link of ADR 0098; the send itself later).
 *
 * THE GATE is `notice.send` (`CongQuyen`), with the title drawn in every state as `/nguoi-dan` does. The
 * parts behind the gate read with OTHER keys, and say so when the session lacks them instead of failing:
 *   citizen.read  groups, tags and the citizen recipient count
 *   admin.zalo    the OA card and the OAs' ZBS templates (`zalo-oas`, `zalo-oas/{id}/templates`)
 * A route the session surely lacks the key for is NOT called (the `CongQuyen` rule: no certain 403).
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
const TABS: readonly { id: TabId; label: string }[] = [
  { id: "compose", label: TAB_COMPOSE },
  { id: "history", label: TAB_HISTORY },
];
const PANEL_ID = "notice-send-panel";
const tabDomId = (id: TabId) => `notice-send-tab-${id}`;

const CARD = "border-line shadow-card rounded-card border border-solid bg-white";
const MUTED = "text-ink-muted m-0 text-[12.5px]";
const LINK = "text-brand text-[12.5px] font-semibold underline-offset-2 hover:underline";

function NoticeSendWorkspace({ canReadCitizens, canManageZalo }: { canReadCitizens: boolean; canManageZalo: boolean }) {
  const [tab, setTab] = useState<TabId>("compose");
  const [step, setStep] = useState<Step>("audience");
  const [kind, setKind] = useState<AudienceKind>("citizen_all");
  const [ids, setIds] = useState<string[]>([]);
  const [tags, setTags] = useState<string[]>([]);
  const [channel, setChannel] = useState<Channel>("zns");
  const [template, setTemplate] = useState<comms_zaloOATemplateOut | null>(null);
  const [title, setTitle] = useState("");
  const [params, setParams] = useState<Record<string, string>>({});

  const [groups, setGroups] = useState<KetQua<{ items: CitizenGroup[] }> | null>(null);
  const [tagList, setTagList] = useState<KetQua<{ items: CitizenContactTag[] }> | null>(null);
  const [units, setUnits] = useState<KetQua<{ items: identity_boPhanRa[] }> | null>(null);
  const [staff, setStaff] = useState<KetQua<{ items: identity_canBoChonNguoiRa[] }> | null>(null);
  const [count, setCount] = useState<{ key: string; result: KetQua<number> } | null>(null);
  const [templates, setTemplates] = useState<KetQua<comms_zaloOATemplateList> | null>(null);

  const oa = useCommuneOA(canManageZalo);
  // The OAs that can send now (ADR 0099 #1). Picking one at send time is D-6c; until then step 2 lists the
  // templates of all of them.
  const sendingOAIds = oa !== null && oa.ok ? sendingOAs(oa.duLieu).map((o) => o.id).join(",") : "";
  const oaUsable = sendingOAIds !== "";

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

  // The OAs' templates: only with admin.zalo, and only from OAs that can send. One refusal is shown as is
  // rather than a partial list that looks complete.
  useEffect(() => {
    if (!canManageZalo || sendingOAIds === "") return;
    let gone = false;
    Promise.all(sendingOAIds.split(",").map((id) => listZaloOATemplates(id))).then((rs) => {
      if (gone) return;
      const refused = rs.find((r) => !r.ok);
      setTemplates(refused ?? { ok: true, duLieu: { items: rs.flatMap((r) => (r.ok ? r.duLieu.items : [])), synced_at: null } });
    });
    return () => {
      gone = true;
    };
  }, [canManageZalo, sendingOAIds]);

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

  /** The recipient figure, or a "?" with its reason — never a number nobody counted. */
  function recipients(): ReactNode {
    const pending = (info: PendingFeatureInfo) => <PendingMarker info={info} side="left" />;
    const loading = <span className="text-ink-muted">…</span>;
    switch (source.type) {
      case "unknown":
        return pending(recipientPendingInfo(source.reason));
      case "tag":
        return recipientsText(source.count);
      case "citizen-count":
        if (count === null || count.key !== countKey) return loading;
        return count.result.ok ? recipientsText(count.result.duLieu) : pending(recipientPendingInfo(count.result.thongBao));
      case "staff":
        if (staff === null) return loading;
        return staff.ok ? recipientsText(countStaff(staff.duLieu.items, source.unitIds)) : pending(recipientPendingInfo(staff.thongBao));
    }
  }

  function pickKind(next: AudienceKind) {
    setKind(next);
    setIds([]);
    setTags([]);
    setTemplate(null);
  }

  function pickChannel(next: Channel) {
    setChannel(next);
    setTemplate(null);
  }

  function pickTemplate(t: comms_zaloOATemplateOut) {
    setTemplate(t);
    setParams({});
    setTitle(t.template_name);
    setStep("content");
  }

  function onTabKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    if (e.key !== "ArrowRight" && e.key !== "ArrowLeft" && e.key !== "Home" && e.key !== "End") return;
    e.preventDefault();
    const next: TabId = e.key === "Home" ? "compose" : e.key === "End" ? "history" : tab === "compose" ? "history" : "compose";
    setTab(next);
    document.getElementById(tabDomId(next))?.focus();
  }

  const paramNames = template?.params.map((p) => p.name) ?? [];
  const missing = missingParams(paramNames, params);

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

      <div className="flex items-center gap-2">
        <TabList aria-label="Phần của màn Gửi tin ZNS / SMS" onKeyDown={onTabKeyDown}>
          {TABS.map((t) => (
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
        <PendingMarker info={HISTORY_PENDING} side="bottom" />
      </div>

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
                summary={AUDIENCE_LABEL[kind]}
              >
                <div className="flex flex-wrap gap-1.5">
                  {AUDIENCE_KINDS.map((k) => (
                    <Chip key={k} label={AUDIENCE_LABEL[k]} active={kind === k} onClick={() => pickKind(k)} />
                  ))}
                </div>

                {kind === "citizen_group" &&
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

                {kind === "citizen_tag" &&
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

                {kind === "staff_unit" && (
                  <Picker
                    label={PICK_UNIT}
                    state={units}
                    options={(u) => u.items.map((row) => ({ id: row.id, label: row.name }))}
                    selected={ids}
                    onToggle={setIds}
                  />
                )}

                <Button
                  type="button"
                  variant="primary"
                  className="mt-4"
                  onClick={() => setStep("template")}
                >
                  Tiếp tục
                </Button>
              </StepCard>

              <StepCard
                index={2}
                title="Mẫu tin"
                done={template !== null && step === "content"}
                open={step === "template"}
                onOpen={() => setStep("template")}
                summary={template?.template_name ?? "Chưa chọn"}
              >
                <p className="text-navy m-0 mb-1.5 text-[13px] font-semibold">{CHANNEL_PICK_LABEL}</p>
                <div className="mb-4 flex flex-wrap gap-1.5">
                  {(Object.keys(CHANNEL_LABEL) as Channel[]).map((c) => (
                    <Chip key={c} label={CHANNEL_LABEL[c]} active={channel === c} onClick={() => pickChannel(c)} />
                  ))}
                </div>
                {channel === "sms" ? (
                  <div className="border-line flex items-center gap-2 rounded-[10px] border border-dashed p-3" data-sms-pending="">
                    <span className="text-ink-muted text-[12.5px]">{SMS_PENDING.ten}</span>
                    <PendingMarker info={SMS_PENDING} />
                  </div>
                ) : !canManageZalo ? (
                  <p className={MUTED} data-no-template-permission="">
                    {NO_TEMPLATE_PERMISSION}
                  </p>
                ) : oa === null ? (
                  <Skeleton className="h-24 w-full" />
                ) : !oaUsable ? (
                  <OAStatusCard canManage={canManageZalo} current={oa} where="step" />
                ) : templates === null ? (
                  <Skeleton className="h-24 w-full" />
                ) : !templates.ok ? (
                  <p className="text-danger m-0 text-[12.5px] font-medium" role="alert">
                    {templates.thongBao}
                  </p>
                ) : templates.duLieu.items.length === 0 ? (
                  <div className="flex flex-col gap-2" data-templates-empty="">
                    <p className={MUTED}>{NO_TEMPLATES}</p>
                    <Link href={ZALO_CONFIG_HREF} className={LINK}>
                      {OPEN_ZALO_CONFIG}
                    </Link>
                  </div>
                ) : (
                  <div className="flex flex-col gap-2">
                    {templates.duLieu.items.map((row) => {
                      const usable = isTemplateUsable(row.zalo_status);
                      return (
                        <button
                          key={row.id}
                          type="button"
                          disabled={!usable}
                          aria-pressed={template?.id === row.id}
                          data-template-id={row.id}
                          onClick={() => pickTemplate(row)}
                          className={cn(
                            "w-full cursor-pointer rounded-[10px] border border-solid p-3 text-left [font-family:inherit] transition-colors",
                            template?.id === row.id ? "border-brand/40 bg-brand/5" : "border-line hover:border-brand/30 bg-white",
                            !usable && "cursor-not-allowed opacity-50 hover:border-line",
                          )}
                        >
                          <span className="text-navy block text-[13px] font-semibold">
                            {row.template_name}
                            {!usable && (
                              <span className="text-tangerine ml-2 text-[11px] font-semibold">{zaloStatusText(row.zalo_status)}</span>
                            )}
                          </span>
                          <span className="text-ink-muted mt-1 line-clamp-2 block text-[12px]">
                            {paramsLine(row.params.map((p) => p.name))}
                          </span>
                          <span className="text-ink-muted mt-1 block text-[11.5px]">{templateSubline(row.zalo_status)}</span>
                        </button>
                      );
                    })}
                  </div>
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
                {template === null ? (
                  <p className={MUTED}>{PICK_TEMPLATE_FIRST}</p>
                ) : (
                  <div className="flex flex-col gap-4">
                    <Field label={CAMPAIGN_TITLE_LABEL} htmlFor="notice-title" grow="auto">
                      <input id="notice-title" value={title} autoComplete="off" onChange={(e) => setTitle(e.target.value)} />
                    </Field>
                    {paramNames.map((name, i) => (
                      <Field key={name} label={name} htmlFor={`notice-param-${i}`} grow="auto">
                        <input
                          id={`notice-param-${i}`}
                          value={params[name] ?? ""}
                          autoComplete="off"
                          onChange={(e) => setParams((cur) => ({ ...cur, [name]: e.target.value }))}
                        />
                      </Field>
                    ))}
                    <div>
                      <PendingButton info={PREVIEW_PENDING} variant="outline">
                        Xem trước
                      </PendingButton>
                    </div>
                  </div>
                )}
              </StepCard>
            </div>

            <aside className={cn(CARD, "h-fit p-5")} aria-labelledby="notice-preview-title">
              <h2 id="notice-preview-title" className="text-navy m-0 text-[14px] font-bold">
                {PREVIEW_TITLE}
              </h2>
              {template === null ? (
                <p className={cn(MUTED, "mt-3")}>{PREVIEW_EMPTY}</p>
              ) : (
                <>
                  <div className="border-line bg-surface mt-3 flex items-center gap-2 rounded-[10px] border border-solid p-3" data-content-pending="">
                    <span className="text-ink-muted text-[12.5px]">{CONTENT_PENDING.ten}</span>
                    <PendingMarker info={CONTENT_PENDING} side="left" />
                  </div>
                  <dl className="m-0 mt-4 flex flex-col gap-2 text-[12.5px]">
                    <Row label={ROW_RECIPIENTS} value={recipients()} dataName="recipients" />
                    <Row label={ROW_UNIT_PRICE} value={<PendingMarker info={PRICE_PENDING} side="left" />} />
                    <Row label={ROW_COST} value={<PendingMarker info={COST_PENDING} side="left" />} strong />
                  </dl>
                  {missing.length > 0 && (
                    <p className="border-tangerine/30 bg-tangerine/8 text-navy m-0 mt-3 rounded-[10px] border border-solid p-3 text-[12px]" data-missing-params="">
                      {missingParamsText(missing)}
                    </p>
                  )}
                  <span className="relative mt-4 flex" data-pending="" data-send="">
                    <Button type="button" variant="primary" disabled className="w-full pr-9" icon={<Send aria-hidden="true" focusable="false" />}>
                      {SEND_BUTTON}
                    </Button>
                    <PendingMarker info={SEND_PENDING} placement="end" side="left" />
                  </span>
                  <p className="text-ink-muted m-0 mt-2 text-[11.5px]">{SEND_NOTE}</p>
                </>
              )}
            </aside>
          </div>
        ) : (
          <div className={CARD} data-history="">
            <p className="text-ink-muted m-0 p-8 text-center text-[12.5px]">{HISTORY_EMPTY}</p>
          </div>
        )}
      </div>
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
        <span className="flex-1">
          <span className="text-navy block text-[13.5px] font-bold">{title}</span>
          {!open && <span className="text-ink-muted block text-[12px]">{summary}</span>}
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
      <p className="text-danger m-0 text-[12.5px] font-medium" role="alert">
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

function Row({ label, value, strong, dataName }: { label: string; value: ReactNode; strong?: boolean; dataName?: string }) {
  return (
    <div className="flex items-center justify-between gap-3" data-row={dataName}>
      <dt className="text-ink-muted">{label}</dt>
      <dd className={cn("m-0 tabular-nums", strong ? "text-navy text-[14px] font-bold" : "text-ink font-semibold")}>{value}</dd>
    </div>
  );
}
