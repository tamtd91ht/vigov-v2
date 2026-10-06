"use client";

import { MessageCircle, Save, Users } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { ErrorState } from "@/components/ui/error-state";
import { NoAccess } from "@/components/ui/no-access";
import { Skeleton } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { formatVietnamDateTime } from "@/features/noi-dung/nhan-noi-dung";
import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import {
  getZaloChannelSettings,
  listZaloLinkedStaff,
  saveZaloChannelSettings,
  ZALO_REMINDER_KINDS,
  type ZaloChannelSettings,
  type ZaloLinkedStaff,
} from "@/lib/api/zalo";

import { zaloChannelTabDecision } from "./quyen-tab";
import {
  buildChange,
  draftFromSettings,
  KIND_LABEL,
  NO_LINKED_STAFF,
  QUIET_HINT,
  SAVED_SENTENCE,
  toggleKind,
  ZALO_TAB_DESCRIPTION,
  ZALO_TAB_TITLE,
  type ZaloChannelDraft,
  type ZaloChannelField,
} from "./zalo-channel-form";

/**
 * "Cấu hình → Kênh Zalo" (ADR 0074 #6): the commune's switch, which reminder kinds go out, quiet
 * hours, the overdue cadence — and who of this commune is paired. One key, `admin.lookup`, for read
 * and write (like Máy chủ thư), so the tab hides as a whole without it (convenience; comms refuses).
 *
 * The bot itself (token, webhook) is NOT here: it is the platform's, set in platform-admin (ADR 0074 #3).
 * The linked-staff list is names, codes and a date — never a Zalo chat id (comms never returns one).
 */
export function ZaloChannelTab() {
  const phien = usePhien();
  const decision = phien === null ? null : zaloChannelTabDecision(phien);
  const allowed = decision !== null && decision.hien;

  const [loaded, setLoaded] = useState<KetQua<ZaloChannelSettings> | null>(null);
  const [draft, setDraft] = useState<ZaloChannelDraft | null>(null);
  const [staff, setStaff] = useState<KetQua<ZaloLinkedStaff[]> | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState<{ ok: boolean; text: string; field?: ZaloChannelField } | null>(null);

  useEffect(() => {
    if (!allowed) return;
    let gone = false;
    getZaloChannelSettings().then((r) => {
      if (gone) return;
      setLoaded(r);
      if (r.ok) setDraft(draftFromSettings(r.duLieu));
    });
    listZaloLinkedStaff().then((r) => {
      if (!gone) setStaff(r);
    });
    return () => {
      gone = true;
    };
  }, [allowed]);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p className="thong-bao-loi" role="alert">
        {decision.thongBao}
      </p>
    ) : (
      <div className="flex min-w-0 flex-col items-center pb-10">
        <NoAccess className="pb-4" />
        <p className="m-0 max-w-md px-4 text-center text-[13px] text-ink-500">
          Tài khoản của bạn không có quyền cấu hình kênh Zalo, nên tab này không hiển thị.
        </p>
      </div>
    );
  }
  if (loaded === null)
    return (
      <div className="page--form flex flex-col gap-3 rounded-card border border-line bg-surface p-4">
        <p role="status" className="an-thi-giac">
          Đang tải cấu hình kênh Zalo…
        </p>
        <Skeleton className="h-5 w-48" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-2/3" />
      </div>
    );
  if (!loaded.ok) return <ErrorState role="alert" title="Chưa tải được cấu hình kênh Zalo" message={loaded.thongBao} />;
  if (draft === null) return null;

  async function save() {
    if (draft === null || saving) return;
    const built = buildChange(draft);
    if (!built.ok) return setSaveMessage({ ok: false, text: built.text, field: built.field });
    setSaving(true);
    setSaveMessage(null);
    const r = await saveZaloChannelSettings(built.change);
    setSaving(false);
    if (!r.ok) return setSaveMessage({ ok: false, text: r.thongBao });
    setLoaded(r);
    setDraft(draftFromSettings(r.duLieu));
    setSaveMessage({ ok: true, text: SAVED_SENTENCE });
  }

  return (
    <ZaloChannelView
      saved={loaded.duLieu}
      draft={draft}
      setDraft={setDraft}
      saving={saving}
      saveMessage={saveMessage}
      onSave={() => void save()}
      staff={staff}
    />
  );
}

/** Pure rendering, exported so the tests read the markup. */
export function ZaloChannelView({
  saved,
  draft,
  setDraft,
  saving,
  saveMessage,
  onSave,
  staff,
}: {
  saved: ZaloChannelSettings;
  draft: ZaloChannelDraft;
  setDraft: (d: ZaloChannelDraft) => void;
  saving: boolean;
  saveMessage: { ok: boolean; text: string; field?: ZaloChannelField } | null;
  onSave: () => void;
  staff: KetQua<ZaloLinkedStaff[]> | null;
}) {
  const lateChosen = draft.kinds.includes("qua-han");
  const fieldError = (f: ZaloChannelField) => (saveMessage !== null && !saveMessage.ok && saveMessage.field === f ? saveMessage.text : null);
  const formError = saveMessage !== null && !saveMessage.ok && saveMessage.field === undefined ? saveMessage.text : null;

  return (
    // Presentation only follows the prototype's `ZaloChannelPanel` (ADR 0068 lần 5): white cards with the
    // title INSIDE (14px bold, no header bar), 20px padding, 20px apart. Fields, calls and rules are
    // unchanged (ADR 0074).
    <div className="page--form flex min-w-0 flex-col gap-5">
      <section
        className="m-0 min-w-0 overflow-hidden rounded-card border border-line bg-surface shadow-sm"
        aria-labelledby="tieu-de-kenh-zalo"
      >
        <div className="px-5 pt-5">
          <div className="min-w-0">
            <h2 id="tieu-de-kenh-zalo" className="m-0 flex items-center gap-2 text-sm font-bold text-ink-900">
              <MessageCircle aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0 text-brand-600" />
              {ZALO_TAB_TITLE}
            </h2>
            <p className="m-0 mt-1 max-w-xl text-[13px] text-ink-500">{ZALO_TAB_DESCRIPTION}</p>
            {saved.updated_at !== undefined && (
              <p className="m-0 mt-1 text-xs text-ink-500">
                Cập nhật {formatVietnamDateTime(saved.updated_at)}
                {saved.updated_by ? ` — ${saved.updated_by}` : ""}
              </p>
            )}
          </div>
        </div>
        <form
          className="form-danh-muc m-0 border-0 bg-transparent p-5"
          aria-label="Cấu hình kênh Zalo"
          onSubmit={(e) => {
            e.preventDefault();
            onSave();
          }}
        >
          <fieldset disabled={saving}>
            <div className="flex min-w-0 flex-col gap-4 [&>*]:m-0">
              <label className="inline-flex min-h-10 cursor-pointer items-center gap-2 font-semibold">
                <input
                  type="checkbox"
                  name="is_enabled"
                  checked={draft.isEnabled}
                  onChange={(e) => setDraft({ ...draft, isEnabled: e.target.checked })}
                />{" "}
                Bật nhắc việc qua Zalo cho cán bộ của xã
              </label>

              <fieldset
                className="flex flex-col gap-1 rounded-xl border border-line px-3 pt-1 pb-3"
                aria-describedby={fieldError("kinds") ? "loi-loai-zalo" : undefined}
              >
                <legend className="px-1 text-xs font-semibold text-ink-700">Loại nhắc việc gửi qua Zalo</legend>
                {ZALO_REMINDER_KINDS.map((k) => (
                  <label key={k} className="inline-flex min-h-10 cursor-pointer items-center gap-2">
                    <input
                      type="checkbox"
                      name="kinds"
                      value={k}
                      checked={draft.kinds.includes(k)}
                      onChange={(e) => setDraft(toggleKind(draft, k, e.target.checked))}
                    />{" "}
                    {KIND_LABEL[k]}
                  </label>
                ))}
                {fieldError("kinds") && (
                  <p id="loi-loai-zalo" className="thong-bao-loi m-0" role="alert">
                    {fieldError("kinds")}
                  </p>
                )}
              </fieldset>

              {lateChosen && (
                <div className="grid min-w-0 gap-4 sm:grid-cols-2 [&>*]:m-0">
                  <div className="o-nhap">
                    <label htmlFor="o-zalo-qua-han-bat-dau">Bắt đầu nhắc sau khi quá hạn (ngày)</label>
                    <input
                      id="o-zalo-qua-han-bat-dau"
                      name="overdue_start_after_days"
                      type="number"
                      inputMode="numeric"
                      min={1}
                      step={1}
                      required
                      value={draft.lateStartDays}
                      aria-invalid={fieldError("lateStartDays") ? true : undefined}
                      onChange={(e) => setDraft({ ...draft, lateStartDays: e.target.value })}
                    />
                    {fieldError("lateStartDays") && (
                      <p className="thong-bao-loi" role="alert">
                        {fieldError("lateStartDays")}
                      </p>
                    )}
                  </div>
                  <div className="o-nhap">
                    <label htmlFor="o-zalo-qua-han-lap">Nhắc lại sau mỗi (ngày)</label>
                    <input
                      id="o-zalo-qua-han-lap"
                      name="overdue_repeat_every_days"
                      type="number"
                      inputMode="numeric"
                      min={1}
                      step={1}
                      required
                      value={draft.lateRepeatDays}
                      aria-invalid={fieldError("lateRepeatDays") ? true : undefined}
                      onChange={(e) => setDraft({ ...draft, lateRepeatDays: e.target.value })}
                    />
                    {fieldError("lateRepeatDays") && (
                      <p className="thong-bao-loi" role="alert">
                        {fieldError("lateRepeatDays")}
                      </p>
                    )}
                  </div>
                </div>
              )}

              <fieldset className="flex flex-col gap-2 rounded-xl border border-line px-3 pt-1 pb-3">
                <legend className="px-1 text-xs font-semibold text-ink-700">Giờ yên lặng</legend>
                <div className="grid min-w-0 gap-4 sm:grid-cols-2 [&>*]:m-0">
                  <div className="o-nhap">
                    <label htmlFor="o-zalo-yen-lang-tu">Từ</label>
                    <input
                      id="o-zalo-yen-lang-tu"
                      name="quiet_start"
                      type="time"
                      value={draft.quietStart}
                      onChange={(e) => setDraft({ ...draft, quietStart: e.target.value })}
                    />
                  </div>
                  <div className="o-nhap">
                    <label htmlFor="o-zalo-yen-lang-den">Đến</label>
                    <input
                      id="o-zalo-yen-lang-den"
                      name="quiet_end"
                      type="time"
                      value={draft.quietEnd}
                      onChange={(e) => setDraft({ ...draft, quietEnd: e.target.value })}
                    />
                  </div>
                </div>
                <p className="ghi-chu m-0">{QUIET_HINT}</p>
                {fieldError("quiet") && (
                  <p className="thong-bao-loi m-0" role="alert">
                    {fieldError("quiet")}
                  </p>
                )}
              </fieldset>

              <div className="flex justify-end">
                <Button
                  type="submit"
                  variant="primary"
                  icon={saving ? undefined : <Save aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                  aria-busy={saving}
                >
                  <BusyLabel busy={saving} label="Lưu cấu hình" busyText={BUSY_SAVING} />
                </Button>
              </div>
            </div>
          </fieldset>
          {saveMessage !== null && saveMessage.ok && (
            <p role="status" className="mt-3 text-sm font-medium text-success-600">
              {saveMessage.text}
            </p>
          )}
          {formError !== null && (
            <p className="thong-bao-loi mt-3" role="alert">
              {formError}
            </p>
          )}
        </form>
      </section>

      <LinkedStaffSection staff={staff} />
    </div>
  );
}

function LinkedStaffSection({ staff }: { staff: KetQua<ZaloLinkedStaff[]> | null }) {
  return (
    <section
      className="m-0 min-w-0 overflow-hidden rounded-card border border-line bg-surface shadow-sm"
      aria-labelledby="tieu-de-can-bo-zalo"
    >
      <div className="flex flex-wrap items-center gap-3 border-b border-line px-5 py-3">
        <h2 id="tieu-de-can-bo-zalo" className="m-0 flex items-center gap-2 text-sm font-bold text-ink-900">
          <Users aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0 text-brand-600" />
          Cán bộ đã ghép nối Zalo
          {staff !== null && staff.ok && staff.duLieu.length > 0 && (
            <span className="text-[13px] font-normal text-ink-500">({staff.duLieu.length})</span>
          )}
        </h2>
      </div>
      {staff === null ? (
        <div className="flex flex-col gap-2 p-4">
          <p role="status" className="an-thi-giac">
            Đang tải danh sách cán bộ đã ghép nối…
          </p>
          <Skeleton className="h-5 w-full" />
          <Skeleton className="h-5 w-2/3" />
        </div>
      ) : !staff.ok ? (
        <p className="thong-bao-loi m-4" role="alert">
          {staff.thongBao}
        </p>
      ) : staff.duLieu.length === 0 ? (
        <p className="m-0 p-4 text-sm text-ink-500">{NO_LINKED_STAFF}</p>
      ) : (
        <TableScroll aria-label="Cán bộ đã ghép nối Zalo">
          <table className={DATA_TABLE_CLASS}>
            <caption className="an-thi-giac">Cán bộ của xã đã ghép nối Zalo</caption>
            <thead>
              <tr>
                <th scope="col">Mã cán bộ</th>
                <th scope="col">Họ và tên</th>
                <th scope="col">Ghép nối lúc</th>
              </tr>
            </thead>
            <tbody>
              {staff.duLieu.map((s) => (
                <tr key={s.staff_code}>
                  <td className="font-mono">{s.staff_code}</td>
                  <td>{s.staff_name}</td>
                  <td>{formatVietnamDateTime(s.linked_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableScroll>
      )}
    </section>
  );
}
