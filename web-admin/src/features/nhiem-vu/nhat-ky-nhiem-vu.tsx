"use client";

import { ArrowRight, Loader2, MessageSquare, Paperclip, Send, UserCheck } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { PendingMarker } from "@/components/ui/pending-feature";
import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import { addTaskLogEntry, layNhatKyNhiemVu } from "@/lib/api/nhiem-vu";
import type {
  page_Result_petitions_nhatKyNhiemVuRa,
  petitions_nhatKyNhiemVuRa,
  petitions_taskAttachmentOut,
} from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import { ATTACH_ACCEPT, ATTACH_BUTTON, ATTACH_INPUT_LABEL, anyInFlight, storedIds } from "./task-attachments";
import {
  AttachmentRemoveDialog,
  PickedFileChips,
  TimelineAttachments,
  useAttachmentUploads,
} from "./task-attachments-ui";
import {
  CHUA_GIAO_BO_PHAN,
  CHUA_PHAN_CONG,
  DANG_TAI_NHAT_KY_NHIEM_VU,
  LOG_ENTRY_BUTTON,
  LOG_ENTRY_DONE,
  LOG_ENTRY_LABEL,
  LOG_ENTRY_MAX,
  LOG_ENTRY_PLACEHOLDER,
  NHAN_XEM_THEM_NHAT_KY_NHIEM_VU,
  NHAT_KY_RONG,
  TIEU_DE_NHAT_KY_NHIEM_VU,
  gopTrangNhatKy,
  hienDongNhatKy,
  laTrangThaiNhiemVu,
  logEntryNote,
  nhanCanBoNgan,
  nhanTrangThai,
  type BangNhanTrangThai,
} from "./nhan-nhiem-vu";
import { TEXTAREA_CLASS, staffInitials } from "./task-spec";

/**
 * Nhật ký & Trao đổi của MỘT nhiệm vụ — spec 08 / prototype `TaskActivityPanel`: the right column of
 * the drawer. A composer fixed at the top (`Đã làm được gì, còn vướng gì…`, `[Đính kèm] … [Ghi nhật
 * ký]`), then the timeline, newest first, scrolling on its own: an initials avatar, the name and time,
 * a `Chuyển sang “X”` pill when the row changed the status, a hand-over box, the note, its files.
 *
 * CÙNG KHUÔN `features/phan-anh/nhat-ky-phieu.tsx`, KHÔNG DÙNG CHUNG THÀNH PHẦN: hai hợp đồng khác
 * hình dạng (dòng phiếu có `action`; dòng nhiệm vụ không có, còn nhãn trạng thái đi qua
 * `BangNhanTrangThai`). Gộp hai cái thành một là một thành phần rẽ nhánh theo loại hồ sơ ở mọi dòng.
 *
 * GHI CHÚ HIỆN BẰNG TEXT NODE CỦA REACT, KHÔNG BAO GIỜ `dangerouslySetInnerHTML`: đó là chữ tự do do
 * cán bộ gõ, và một thẻ `<img onerror>` trong đó sẽ chạy trên máy mọi người mở drawer.
 */

/** Bao nhiêu dòng một lần tải. Máy chủ nhận 1–100. */
const SO_DONG_MOI_TRANG = 20;

/** Spec 08: the files not attached to any entry need a task-level file list (BACKEND DEPENDENCY). */
export const LOOSE_FILES_PENDING = {
  ten: "Tệp chưa gắn với ghi chép nào",
  viSao:
    "Nhật ký chỉ đọc được tệp gắn theo từng dòng ghi. Danh sách tệp của nhiệm vụ không thuộc dòng nào " +
    "cần máy chủ trả thêm một tuyến đọc riêng.",
} as const;

type TrangDoc =
  | {
      khoa: string;
      ok: true;
      dong: readonly petitions_nhatKyNhiemVuRa[];
      conTro: string;
      conNua: boolean;
    }
  | { khoa: string; ok: false; thongBao: string };

function tuKetQua(khoa: string, kq: KetQua<page_Result_petitions_nhatKyNhiemVuRa>): TrangDoc {
  return kq.ok
    ? {
        khoa,
        ok: true,
        dong: kq.duLieu.items,
        conTro: kq.duLieu.next_cursor,
        conNua: kq.duLieu.has_more,
      }
    : { khoa, ok: false, thongBao: kq.thongBao };
}

export function NhatKyNhiemVu({
  maNhiemVu,
  nhanTT,
  danhBa,
  tenBoPhan,
  lanLamMoi,
  canWrite,
  onHistory,
}: {
  maNhiemVu: string;
  nhanTT: BangNhanTrangThai;
  /** Danh bạ chọn người tra theo mã. `null` = chưa có (đang tải / tải hỏng) — dòng hiện mã. */
  danhBa: DanhBaTheoMa | null;
  tenBoPhan: ReadonlyMap<string, string>;
  /**
   * Đổi ở mỗi lần mở drawer và sau mỗi lần ghi thành công (`DrawerNhiemVu.luotDoc`) — máy chủ vừa
   * ghi thêm một dòng, nên trang đang hiện đã cũ.
   */
  lanLamMoi: number;
  /**
   * Show the manual entry form — `canWriteLogEntry`. REQUIRED: a caller that forgets it gets a red
   * `tsc`, not a form that silently appears for every reader (or vanishes for every writer).
   */
  canWrite: boolean;
  /**
   * Told the rows loaded so far, newest first, and whether they reach the task's first row
   * (`complete`) — the status strip derives the time in the current status from them (prototype
   * `TaskStatusPipeline.tsx:62-75`) instead of reading the same route a second time.
   */
  onHistory?: (rows: readonly petitions_nhatKyNhiemVuRa[], complete: boolean) => void;
}) {
  const [doc, datDoc] = useState<TrangDoc | null>(null);
  const [removing, setRemoving] = useState<petitions_taskAttachmentOut | null>(null);
  const [dangTaiThem, datDangTaiThem] = useState(false);
  const [loiThem, datLoiThem] = useState<{ khoa: string; thongBao: string } | null>(null);
  // Bumped after a 201: the server appended a row, so the page on screen is stale. Part of the read
  // key, so the first page is read again — never a local splice of the reply into the list.
  const [written, setWritten] = useState(0);

  const khoaDoc = `${maNhiemVu}|${lanLamMoi}|${written}`;

  useEffect(() => {
    let bo = false;
    layNhatKyNhiemVu(maNhiemVu, null, SO_DONG_MOI_TRANG).then((kq) => {
      if (!bo) datDoc(tuKetQua(khoaDoc, kq));
    });
    return () => {
      bo = true;
    };
  }, [maNhiemVu, khoaDoc]);

  // Câu trả lời của một lần đọc CŨ (nhiệm vụ khác, trước lần làm mới) không được vẽ như của lần
  // này — đổi nhiệm vụ là về lại "đang tải", không bao giờ hiện nhật ký của việc vừa đóng.
  const hienTai = doc !== null && doc.khoa === khoaDoc ? doc : null;
  const loiThemHienTai = loiThem !== null && loiThem.khoa === khoaDoc ? loiThem.thongBao : null;

  useEffect(() => {
    if (hienTai !== null && hienTai.ok) onHistory?.(hienTai.dong, !hienTai.conNua);
  }, [hienTai, onHistory]);

  function xemThem(): void {
    if (hienTai === null || !hienTai.ok || !hienTai.conNua || hienTai.conTro === "") return;
    const truoc = hienTai;
    datDangTaiThem(true);
    layNhatKyNhiemVu(maNhiemVu, truoc.conTro, SO_DONG_MOI_TRANG).then((kq) => {
      datDangTaiThem(false);
      if (!kq.ok) {
        datLoiThem({ khoa: truoc.khoa, thongBao: kq.thongBao });
        return;
      }
      datLoiThem(null);
      datDoc((d) =>
        d === null || d.khoa !== truoc.khoa || !d.ok
          ? d
          : {
              ...d,
              dong: gopTrangNhatKy(d.dong, kq.duLieu.items),
              conTro: kq.duLieu.next_cursor,
              conNua: kq.duLieu.has_more,
            },
      );
    });
  }

  return (
    <KhoiNhatKyNhiemVu
      maNhiemVu={maNhiemVu}
      tai={
        hienTai === null
          ? { pha: "dangTai" }
          : hienTai.ok
            ? { pha: "xong", dong: hienTai.dong, conNua: hienTai.conNua && hienTai.conTro !== "" }
            : { pha: "loi", thongBao: hienTai.thongBao }
      }
      nhanTT={nhanTT}
      danhBa={danhBa}
      tenBoPhan={tenBoPhan}
      loiThem={loiThemHienTai}
      dangTaiThem={dangTaiThem}
      xemThem={xemThem}
      form={
        canWrite ? (
          <TaskLogEntryForm taskCode={maNhiemVu} onWritten={() => setWritten((n) => n + 1)} />
        ) : null
      }
      // The same gate as the composer: the server decides again (uploader or `task.update`).
      onRemoveFile={canWrite ? setRemoving : undefined}
      dialog={
        removing !== null ? (
          <AttachmentRemoveDialog
            taskCode={maNhiemVu}
            file={removing}
            onClose={() => setRemoving(null)}
            // The row's file list is the server's: read the log again, never splice it locally.
            onRemoved={() => setWritten((n) => n + 1)}
          />
        ) : null
      }
    />
  );
}

/**
 * The composer (spec 08) — `POST /api/v1/tasks/{ma}/log-entries`.
 *
 * THE IDEMPOTENCY KEY LIVES FOR ONE ENTRY: made when the form mounts, REUSED on a retry after a
 * failure (the first send may have reached the server and written the row), replaced only after a
 * 201. The table is append-only — a duplicate made by a double send can never be removed.
 *
 * A NOTE IS REQUIRED: the server refuses a blank note, so files alone cannot be sent yet (the prototype
 * allows it — BACKEND DEPENDENCY). Only STORED files go with the entry; the button waits while any
 * file still uploads or is checked (ADR 0052).
 *
 * NO OPTIMISTIC ROW: the entry appears when the timeline is read again after the 201. Outcomes are
 * toasts (spec 08: "Đã ghi vào nhật ký." / the server's sentence).
 */
export function TaskLogEntryForm({
  taskCode,
  onWritten,
}: {
  taskCode: string;
  onWritten: () => void;
}) {
  const [text, setText] = useState("");
  const [key, setKey] = useState(() => crypto.randomUUID());
  const [sending, setSending] = useState(false);
  const note = logEntryNote(text);
  const fieldId = `ghi-nhat-ky-${taskCode}`;
  const inputId = `${fieldId}-dinh-kem`;
  const fileInput = useRef<HTMLInputElement>(null);
  // `📎 Đính kèm` (A4). Only STORED files go with the entry; the entry waits while any still moves.
  const files = useAttachmentUploads(taskCode);
  const waiting = anyInFlight(files.items);

  return (
    <form
      className="m-0 mt-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (note === null || sending || waiting) return;
        setSending(true);
        addTaskLogEntry(taskCode, note, key, storedIds(files.items)).then((r) => {
          setSending(false);
          if (!r.ok) {
            // THE SERVER'S SENTENCE VERBATIM — the 403 names who may write.
            toast.error(r.thongBao);
            return;
          }
          setText("");
          files.clear();
          setKey(crypto.randomUUID());
          toast.success(LOG_ENTRY_DONE);
          onWritten();
        });
      }}
    >
      {/* No visible label (spec 08: no title above the box) — the accessible name stays. */}
      <label htmlFor={fieldId} className="an-thi-giac">
        {LOG_ENTRY_LABEL}
      </label>
      <textarea
        id={fieldId}
        name={fieldId}
        rows={3}
        maxLength={LOG_ENTRY_MAX}
        placeholder={LOG_ENTRY_PLACEHOLDER}
        value={text}
        className={TEXTAREA_CLASS}
        onChange={(e) => setText(e.target.value)}
      />
      <PickedFileChips items={files.items} disabled={sending} onRetry={files.retry} onRemove={files.remove} />
      <div className="mt-2 flex items-center gap-2">
        <input
          ref={fileInput}
          id={inputId}
          name={inputId}
          type="file"
          multiple
          accept={ATTACH_ACCEPT}
          className="hidden"
          aria-label={ATTACH_INPUT_LABEL}
          disabled={sending}
          onChange={(e) => {
            const picked = e.target.files === null ? [] : Array.from(e.target.files);
            e.target.value = ""; // the same file can be chosen again after a removal
            if (picked.length > 0) files.add(picked);
          }}
        />
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={sending}
          icon={<Paperclip aria-hidden="true" focusable="false" className="size-3.5" />}
          onClick={() => fileInput.current?.click()}
        >
          {ATTACH_BUTTON}
        </Button>
        <Button
          type="submit"
          variant="primary"
          size="sm"
          className="ml-auto"
          disabled={sending || waiting || note === null}
          aria-busy={sending || undefined}
          icon={
            sending ? (
              <Loader2 aria-hidden="true" focusable="false" className="size-3.5 animate-spin" />
            ) : (
              <Send aria-hidden="true" focusable="false" className="size-3.5" />
            )
          }
        >
          {LOG_ENTRY_BUTTON}
        </Button>
      </div>
    </form>
  );
}

/** Ba pha của khối. `conNua` đã gộp hai điều kiện: `has_more` VÀ có `next_cursor` để đi tới. */
export type TaiNhatKyNhiemVu =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; dong: readonly petitions_nhatKyNhiemVuRa[]; conNua: boolean };

/** Spec 08: the pill of a row that changed the status — `Chuyển sang “{label}”`. */
export function statusPillText(label: string): string {
  return `Chuyển sang “${label}”`;
}

/**
 * Whether row `i` CHANGED the status: its status differs from the next OLDER row loaded. A row whose
 * older neighbour is not loaded (page boundary) or the first row ever (the creation) gets no pill —
 * "unknown" is never drawn as "changed". Derived only from the rows the server sent.
 */
export function rowChangedStatus(rows: readonly petitions_nhatKyNhiemVuRa[], i: number): boolean {
  const older = rows[i + 1];
  const row = rows[i];
  return row !== undefined && older !== undefined && older.status !== row.status;
}

/**
 * The line the SERVER writes into a timeline row when a move carries no note
 * (`service-petitions/internal/domain/nhiem_vu_ghi.go:594-596`, `NoiDungChuyenTrangThai`) — and appends,
 * after `; `, to a reassignment that also moved the status (`task_assignment.go:207-209`). Two CODES,
 * never labels. Matched only when BOTH are among the seven codes: an officer's own sentence that
 * happens to start the same way is never touched.
 */
const SERVER_STATUS_LINE = /(?:^|; )Chuyển trạng thái: (\S+) → (\S+)$/;

function serverStatusLine(note: string): { readonly at: number; readonly to: string } | null {
  const m = SERVER_STATUS_LINE.exec(note);
  if (m === null || m[1] === undefined || m[2] === undefined) return null;
  if (!laTrangThaiNhiemVu(m[1]) || !laTrangThaiNhiemVu(m[2])) return null;
  return { at: m.index, to: m[2] };
}

/**
 * The note AS DRAWN (ADR 0082 #4): the server's `Chuyển trạng thái: <code> → <code>` line is hidden —
 * the whole note when it is only that, the `; Chuyển trạng thái: …` tail of a reassignment line
 * otherwise. DISPLAY ONLY: the stored row is untouched (the timeline is append-only, rule 7); the
 * status pill says the same thing in the commune's words.
 */
export function visibleLogNote(note: string): string {
  const line = serverStatusLine(note);
  return line === null ? note : note.slice(0, line.at);
}

/**
 * Whether row `i` gets the status pill (prototype `TaskActivityPanel.tsx:119-122,307-311`: every entry
 * that recorded a status). A row carries the status AT ITS MOMENT, not a "status after" field, so a
 * change is read from the older neighbour (`rowChangedStatus`) — or, when that neighbour is not loaded
 * (page boundary), from the server's own transition line in the note, which names the code it moved to.
 */
export function rowShowsStatusPill(rows: readonly petitions_nhatKyNhiemVuRa[], i: number): boolean {
  const row = rows[i];
  if (row === undefined) return false;
  return rowChangedStatus(rows, i) || serverStatusLine(row.note)?.to === row.status;
}

/**
 * Phần vẽ, không đọc mạng — tách ra để kiểm bằng `renderToStaticMarkup`.
 *
 * TẢI HỎNG KHÔNG VẼ CÂU "Chưa có ghi chép nào.": một nhật ký rỗng vì đọc hỏng trông y hệt một nhiệm
 * vụ chưa ai đụng tới. Câu máy chủ ra NGUYÊN VĂN, `role="alert"`.
 */
export function KhoiNhatKyNhiemVu({
  maNhiemVu,
  tai,
  nhanTT,
  danhBa,
  tenBoPhan,
  loiThem,
  dangTaiThem,
  xemThem,
  form = null,
  onRemoveFile,
  dialog = null,
}: {
  maNhiemVu: string;
  tai: TaiNhatKyNhiemVu;
  nhanTT: BangNhanTrangThai;
  danhBa: DanhBaTheoMa | null;
  tenBoPhan: ReadonlyMap<string, string>;
  loiThem: string | null;
  dangTaiThem: boolean;
  xemThem: () => void;
  /** The composer, or `null` for an account that may only read (see `NhatKyNhiemVu`). */
  form?: ReactNode;
  /** Ask to remove a file of a row (opens the reason dialog). Absent = no remove control. */
  onRemoveFile?: (file: petitions_taskAttachmentOut) => void;
  /** The open removal dialog, if any. */
  dialog?: ReactNode;
}) {
  const idTieuDe = `tieu-de-nhat-ky-nhiem-vu-${maNhiemVu}`;
  return (
    <section className="flex min-h-0 flex-col lg:flex-1" aria-labelledby={idTieuDe}>
      <div className="border-line shrink-0 border-b px-4 py-3">
        <h3 id={idTieuDe} className="text-navy m-0 flex items-center gap-1.5 text-[12.5px] font-bold">
          <MessageSquare aria-hidden="true" focusable="false" className="size-4" />
          {TIEU_DE_NHAT_KY_NHIEM_VU}
        </h3>
        {form}
      </div>

      <div className="px-4 py-3 lg:min-h-0 lg:flex-1 lg:overflow-y-auto">
        <p className="text-ink-muted m-0 mb-3 flex items-center gap-1.5 text-[11px] font-semibold uppercase">
          {LOOSE_FILES_PENDING.ten}
          <PendingMarker info={LOOSE_FILES_PENDING} />
        </p>

        {tai.pha === "dangTai" && (
          <p role="status" className="text-ink-muted m-0 py-10 text-center text-[12px]">
            {DANG_TAI_NHAT_KY_NHIEM_VU}
          </p>
        )}
        {tai.pha === "loi" && (
          <p className="thong-bao-loi m-0" role="alert">
            {tai.thongBao}
          </p>
        )}
        {tai.pha === "xong" && tai.dong.length === 0 && (
          <p className="text-ink-muted m-0 py-10 text-center text-[12px]">{NHAT_KY_RONG}</p>
        )}
        {tai.pha === "xong" && tai.dong.length > 0 && (
          <ol className="m-0 list-none space-y-3 p-0" aria-label="Nhật ký nhiệm vụ, mới nhất trước">
            {tai.dong.map((d, i) => {
              const h = hienDongNhatKy(d, nhanTT, danhBa, tenBoPhan);
              const changed = rowShowsStatusPill(tai.dong, i);
              const note = visibleLogNote(h.ghiChu);
              const handover = handoverOf(
                tai.dong,
                i,
                (id) => (id === "" ? CHUA_GIAO_BO_PHAN : (tenBoPhan.get(id) ?? id)),
                (code) => nhanCanBoNgan(code, danhBa, CHUA_PHAN_CONG),
              );
              const handedOver = handover !== null;
              const initials = staffInitials(danhBa?.get(d.actor_code)?.full_name ?? "");
              return (
                <li key={h.id} className="flex gap-2.5">
                  <span
                    aria-hidden="true"
                    className="bg-brand/12 text-brand mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full text-[11px] font-bold"
                  >
                    {initials === "" ? "?" : initials}
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-baseline gap-x-2">
                      <b className="text-navy text-[12.5px]">{h.nguoi}</b>
                      <time dateTime={h.luc} className="text-ink-muted text-[11px]">
                        {h.thoiDiem}
                      </time>
                    </div>
                    {changed && (
                      <span className="bg-brand/12 text-brand mt-1 inline-block rounded-full px-2 py-0.5 text-[11px] font-semibold">
                        {statusPillText(nhanTrangThai(nhanTT, d.status))}
                      </span>
                    )}
                    {/* Prototype `TaskActivityPanel.tsx:317-347`: "Bộ phận: A → B" — only the side that
                        changed. The previous holder is the nearest OLDER assignment row loaded; with
                        none (the first assignment, or older rows not loaded) the line names the new
                        holder only — never an arrow with nothing before it. */}
                    {handover !== null && (
                      <div className="border-line bg-canvas mt-1 rounded-[8px] border px-2 py-1.5">
                        {handover.unit !== null && (
                          <HandoverLine icon="unit" label="Bộ phận" change={handover.unit} />
                        )}
                        {handover.assignee !== null && (
                          <HandoverLine icon="assignee" label="Người thực hiện" change={handover.assignee} />
                        )}
                      </div>
                    )}
                    {note !== "" && (
                      <p className={cn("m-0 text-[12.5px] whitespace-pre-line", changed || handedOver ? "mt-1" : "mt-0.5")}>
                        {note}
                      </p>
                    )}
                    {/* `attachments` is always present on a row (b37ec2d); `?? []` only guards a reply
                        from before that commit, which would otherwise crash the whole timeline. */}
                    <TimelineAttachments taskCode={maNhiemVu} attachments={d.attachments ?? []} onRemove={onRemoveFile} />
                  </div>
                </li>
              );
            })}
          </ol>
        )}
        {tai.pha === "xong" && loiThem !== null && (
          <p className="thong-bao-loi m-0 mt-2" role="alert">
            {loiThem}
          </p>
        )}
        {tai.pha === "xong" && tai.conNua && (
          <Button type="button" variant="outline" size="sm" className="mt-3" disabled={dangTaiThem} onClick={xemThem}>
            {NHAN_XEM_THEM_NHAT_KY_NHIEM_VU}
          </Button>
        )}
      </div>
      {dialog}
    </section>
  );
}

/** One side of a hand-over: `from` is `null` when the previous holder is not known from the rows. */
export type HandoverChange = { readonly from: string | null; readonly to: string };

/**
 * The hand-over a timeline row records, or `null`. A row carries `unit` / `assignee` ONLY when it
 * changed the assignment (`nhat_ky_nhiem_vu.go:52-57`) — the holder AFTER the act. The holder before
 * is the nearest OLDER loaded row that carries them; a side that did not change is left out (the
 * prototype draws only the side that changed). With no older assignment row loaded, both sides show
 * their new value with `from: null`.
 */
export function handoverOf(
  rows: readonly petitions_nhatKyNhiemVuRa[],
  i: number,
  unitName: (id: string) => string,
  staffName: (code: string) => string,
): { readonly unit: HandoverChange | null; readonly assignee: HandoverChange | null } | null {
  const row = rows[i];
  if (row === undefined || (row.unit === "" && row.assignee === "")) return null;
  const before = rows.slice(i + 1).find((r) => r.unit !== "" || r.assignee !== "");
  if (before === undefined) {
    return {
      unit: { from: null, to: unitName(row.unit) },
      assignee: { from: null, to: staffName(row.assignee) },
    };
  }
  const unit = before.unit === row.unit ? null : { from: unitName(before.unit), to: unitName(row.unit) };
  const assignee =
    before.assignee === row.assignee ? null : { from: staffName(before.assignee), to: staffName(row.assignee) };
  return unit === null && assignee === null ? null : { unit, assignee };
}

function HandoverLine({
  icon,
  label,
  change,
}: {
  icon: "unit" | "assignee";
  label: string;
  change: HandoverChange;
}) {
  const iconClass = "text-ink-muted mt-0.5 size-3 shrink-0";
  return (
    <p className={cn("m-0 flex items-start gap-1.5 text-[12px]", icon === "assignee" && "mt-0.5")}>
      {icon === "unit" ? (
        <ArrowRight aria-hidden="true" focusable="false" className={iconClass} />
      ) : (
        <UserCheck aria-hidden="true" focusable="false" className={iconClass} />
      )}
      <span>
        <span className="text-ink-muted">{label}: </span>
        {change.from !== null && (
          <>
            {change.from}
            <span className="text-ink-muted"> → </span>
          </>
        )}
        <b className="text-navy font-semibold">{change.to}</b>
      </span>
    </p>
  );
}
