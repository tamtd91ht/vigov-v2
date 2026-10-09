"use client";

import { ArrowRightLeft, ChevronDown, LockKeyhole, Send, UserCheck } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { AttachmentPicker } from "@/features/nhiem-vu/task-attachments-ui";
import { ATTACH_WAIT_NOTE, anyInFlight, storedIds } from "@/features/nhiem-vu/task-attachments";
import { cn } from "@/lib/cn";
import { khoaSauLanGhi } from "@/features/thu-chi/nhan-thu-chi";
import type { KetQua } from "@/lib/api/goi";
import { ghiNhatKyPhieu, layNhatKyPhieu } from "@/lib/api/phieu-phan-anh";
import type {
  page_Result_petitions_nhatKyPhieuRa,
  petitions_nhatKyPhieuRa,
  petitions_taskAttachmentOut,
} from "@/lib/api/schema.gen";

import {
  DANG_TAI_NHAT_KY,
  dateTimeLabel,
  GOI_Y_GHI_NHAT_KY,
  laDongPhanCong,
  LOG_INTERNAL_NOTE,
  LOG_WRITTEN_TOAST,
  logActorLabel,
  loiGhiChuNhatKy,
  NHAC_DU_LIEU_CA_NHAN,
  NHAN_NUT_GHI_NHAT_KY,
  NHAN_O_GHI_NHAT_KY,
  NHAN_XEM_THEM_NHAT_KY,
  NHAT_KY_RONG,
  nhanBoPhan,
  nhanCanBoXuLy,
  nhanThaoTacNhatKy,
  nhanTrangThai,
  TIEU_DE_NHAT_KY,
  type DanhBaTheoMa,
} from "./nhan-phieu";
import { buttonClass, Glyph, TEXTAREA_CLASS } from "./petition-ui";
import {
  LogAttachmentRemoveDialog,
  PetitionLogAttachmentList,
  usePetitionLogAttachments,
} from "./petition-log-attachments";

/**
 * Nhật ký xử lý của MỘT phiếu (§8.7) — dòng thời gian mới nhất trước, "Xem thêm" theo con trỏ, và
 * nút `Ghi nhật ký`.
 *
 * NHẬT KÝ LÀ BẢN GHI NGHIỆP VỤ, KHÔNG PHẢI `audit_log`: cán bộ trong xã đọc nó để biết ai đã làm gì.
 * Máy chủ tự ghi một dòng ở mỗi thao tác xử lý; dòng `ghi-chu` là dòng cán bộ tự viết.
 *
 * NỘI DUNG GHI CHÚ HIỆN BẰNG TEXT NODE CỦA REACT, KHÔNG BAO GIỜ `dangerouslySetInnerHTML`: đó là chữ
 * tự do do người gõ, và một thẻ `<img onerror>` trong đó sẽ chạy trên máy của mọi cán bộ mở phiếu.
 * Xuống dòng giữ bằng CSS (`.ghi-chu-nhat-ky`), không bằng cách chèn `<br>`.
 */

/** Bao nhiêu dòng một lần tải. Máy chủ nhận 1–100, mặc định 20. */
const SO_DONG_MOI_TRANG = 20;

type TrangDoc =
  | { khoa: string; ok: true; dong: readonly petitions_nhatKyPhieuRa[]; conTro: string; conNua: boolean }
  | { khoa: string; ok: false; thongBao: string };

function tuKetQua(khoa: string, kq: KetQua<page_Result_petitions_nhatKyPhieuRa>): TrangDoc {
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

export function NhatKyPhieu({
  maTraCuu,
  tenBoPhan,
  danhBa,
  coNutGhi,
  lanLamMoi = 0,
  sessionStaffCode = "",
  mayResolve = false,
}: {
  maTraCuu: string;
  tenBoPhan: ReadonlyMap<string, string>;
  /** Danh bạ tra theo mã cán bộ, cho ô `Phụ trách` của dòng `phan-cong`. `null` = chưa có. */
  danhBa: DanhBaTheoMa | null;
  /**
   * Có vẽ nút `Ghi nhật ký` không. CHỈ LÀ TIỆN DỤNG: người gọi truyền `feedback.read` của phiên.
   * Máy chủ mới quyết ai ghi được (người được phân công, hoặc có `feedback.resolve` / `.assign` /
   * `.classify`) và câu 403 của nó ra nguyên văn.
   */
  coNutGhi: boolean;
  /** Tăng lên sau mỗi thao tác xử lý thành công — máy chủ vừa ghi thêm một dòng. */
  lanLamMoi?: number;
  /**
   * `phien.staff.code` (`CB-…`), `""` while the session is unread. A row's files may be removed by the
   * officer who WROTE that row (its `actor_code` — the uploader: a file is attached by the one who
   * uploaded it) or by a `feedback.resolve` holder (ADR 0088 §2; the server decides again).
   */
  sessionStaffCode?: string;
  /** The session holds `feedback.resolve`. Default `false`: an unknown session holds no key. */
  mayResolve?: boolean;
}) {
  const [lanTaiLai, datLanTaiLai] = useState(0);
  const [doc, datDoc] = useState<TrangDoc | null>(null);
  const [dangTaiThem, datDangTaiThem] = useState(false);
  const [loiThem, datLoiThem] = useState<string | null>(null);

  const [noiDung, datNoiDung] = useState("");
  /**
   * Khoá chống trùng của BẢN NHÁP đang gõ. Giữ nguyên qua mọi lần gửi lại sau lỗi (kể cả lỗi mạng mà
   * lần đầu thật ra đã tới máy chủ); thay mới sau một lần 201 — `khoaSauLanGhi`.
   */
  const [khoa, datKhoa] = useState(khoaChongTrungMoi);
  const [dangGui, datDangGui] = useState(false);
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  // `📎 Đính kèm` (§8.7). Only STORED files go with the entry; the entry waits while any still moves.
  const tep = usePetitionLogAttachments(maTraCuu);
  const choTep = anyInFlight(tep.items);
  const [removing, setRemoving] = useState<petitions_taskAttachmentOut | null>(null);

  const khoaDoc = `${maTraCuu}|${lanLamMoi}|${lanTaiLai}`;

  useEffect(() => {
    let bo = false;
    layNhatKyPhieu(maTraCuu, { limit: SO_DONG_MOI_TRANG }).then((kq) => {
      if (!bo) datDoc(tuKetQua(khoaDoc, kq));
    });
    return () => {
      bo = true;
    };
  }, [maTraCuu, khoaDoc]);

  // Kết quả của một lần đọc CŨ (phiếu khác, trước lần làm mới) không được vẽ như của lần này.
  const hienTai = doc !== null && doc.khoa === khoaDoc ? doc : null;

  function xemThem(): void {
    if (hienTai === null || !hienTai.ok || !hienTai.conNua || hienTai.conTro === "") return;
    const truoc = hienTai;
    datDangTaiThem(true);
    layNhatKyPhieu(maTraCuu, { limit: SO_DONG_MOI_TRANG, cursor: truoc.conTro }).then((kq) => {
      datDangTaiThem(false);
      if (!kq.ok) {
        datLoiThem(kq.thongBao);
        return;
      }
      datLoiThem(null);
      datDoc((d) =>
        d === null || d.khoa !== truoc.khoa || !d.ok
          ? d
          : {
              ...d,
              dong: [...d.dong, ...kq.duLieu.items],
              conTro: kq.duLieu.next_cursor,
              conNua: kq.duLieu.has_more,
            },
      );
    });
  }

  function gui(): void {
    if (choTep) return;
    datDangGui(true);
    ghiNhatKyPhieu(maTraCuu, noiDung, khoa, storedIds(tep.items)).then((kq) => {
      datDangGui(false);
      datKhoa((k) => khoaSauLanGhi(k, kq.ok, khoaChongTrungMoi));
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ — 403 nói đúng vì sao tài khoản này không ghi được vào phiếu này.
        datLoiGhi(kq.thongBao);
        return;
      }
      datLoiGhi(null);
      datNoiDung("");
      tep.clear();
      datLanTaiLai((n) => n + 1);
      toast.success(LOG_WRITTEN_TOAST);
    });
  }

  // THE PROTOTYPE'S PANEL (`FeedbackActivityPanel.tsx:154-326`): a header block with the title and the
  // ALWAYS-OPEN entry box, then the rows. Flat — the aside is the frame, no card inside it.
  return (
    <section aria-labelledby={`tieu-de-nhat-ky-${maTraCuu}`} className="flex min-h-0 flex-col">
      <div className="shrink-0 border-b border-solid border-line px-4 py-3">
        <div className="flex items-center gap-2">
          <h3 id={`tieu-de-nhat-ky-${maTraCuu}`} className="m-0 text-[12.5px] font-bold text-navy">
            {TIEU_DE_NHAT_KY}
          </h3>
        </div>
        {/* The log is ALWAYS internal (ADR 0041 §Sửa đổi 09/10/2026): one plain statement for the whole
            panel replaces the prototype's per-row `Nội bộ` pill — no row could ever read otherwise. */}
        <p className="m-0 mt-1 text-[11px] text-ink-muted" data-log-internal="">
          {LOG_INTERNAL_NOTE}
        </p>
        {coNutGhi && (
          <BieuMauGhiNhatKy
            id={`bieu-mau-nhat-ky-${maTraCuu}`}
            noiDung={noiDung}
            datNoiDung={datNoiDung}
            dangGui={dangGui}
            loi={loiGhi}
            gui={gui}
            choTep={choTep}
            dinhKem={
              <AttachmentPicker
                fieldId={`bieu-mau-nhat-ky-${maTraCuu}`}
                items={tep.items}
                disabled={dangGui}
                onAdd={tep.add}
                onRetry={tep.retry}
                onRemove={tep.remove}
              />
            }
          />
        )}
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-3 px-4 py-3 [&>p]:m-0">
      {hienTai === null && (
        <p role="status" className="text-sm text-ink-500">
          {DANG_TAI_NHAT_KY}
        </p>
      )}
      {hienTai !== null && !hienTai.ok && (
        <p className="thong-bao-loi" role="alert">
          {hienTai.thongBao}
        </p>
      )}
      {hienTai !== null && hienTai.ok && (
        <>
          <DanhSachNhatKy
            dong={hienTai.dong}
            tenBoPhan={tenBoPhan}
            danhBa={danhBa}
            maTraCuu={maTraCuu}
            mayRemoveFiles={(row) => mayRemoveRowFiles(row, sessionStaffCode, mayResolve)}
            onRemoveFile={setRemoving}
          />
          {loiThem !== null && (
            <p className="thong-bao-loi" role="alert">
              {loiThem}
            </p>
          )}
          {/* Hai điều kiện, không một: `has_more` nói còn dòng, `next_cursor` là đường đi tới đó. */}
          {hienTai.conNua && hienTai.conTro !== "" && (
            <div>
              <button
                type="button"
                className={buttonClass("secondary", "sm")}
                disabled={dangTaiThem}
                onClick={xemThem}
              >
                <Glyph icon={ChevronDown} />
                {NHAN_XEM_THEM_NHAT_KY}
              </button>
            </div>
          )}
        </>
      )}
      </div>
      {removing !== null && (
        <LogAttachmentRemoveDialog
          lookupCode={maTraCuu}
          file={removing}
          onClose={() => setRemoving(null)}
          // The row's file list is the server's: read the log again.
          onRemoved={() => datLanTaiLai((n) => n + 1)}
        />
      )}
    </section>
  );
}

/**
 * Whether THIS account is offered the remove control on a row's files (ADR 0088 §2): a
 * `feedback.resolve` holder, or the officer who wrote the row. FAIL CLOSED on the comparison: an empty
 * session code matches nothing — `"" === ""` would hand every code-less row to every account. Both
 * sides are business codes (`CB-…`). UX only; the server checks the uploader on the stored file.
 */
export function mayRemoveRowFiles(
  row: Pick<petitions_nhatKyPhieuRa, "actor_code">,
  sessionStaffCode: string,
  mayResolve: boolean,
): boolean {
  if (mayResolve) return true;
  return sessionStaffCode !== "" && row.actor_code === sessionStaffCode;
}

/**
 * Các dòng nhật ký, đúng thứ tự máy chủ trả (mới nhất trước). Tách ra để kiểm bằng HTML tĩnh.
 *
 * The server returns the actor as a STAFF CODE only; the cell reads `Full name (CB-…)` through the
 * directory the screen already read once (`danhBa`), and the bare code when the directory does not know
 * it — the code is what still names one person years later (rule 6, invariant 8). Empty reads "—".
 * Rows the citizen caused carry the marker `cong-dan` and read "Người dân" (`logActorLabel`).
 */
export function DanhSachNhatKy({
  dong,
  tenBoPhan,
  danhBa,
  maTraCuu,
  mayRemoveFiles = () => false,
  onRemoveFile,
}: {
  dong: readonly petitions_nhatKyPhieuRa[];
  tenBoPhan: ReadonlyMap<string, string>;
  danhBa: DanhBaTheoMa | null;
  /** The petition's lookup code — the download route of a row's files needs it. Absent = no files drawn. */
  maTraCuu?: string;
  /** Whether a row's files get a remove control. Default: none (fail closed). */
  mayRemoveFiles?: (row: petitions_nhatKyPhieuRa) => boolean;
  /** Opens the reason dialog for one file. Absent = no remove control anywhere. */
  onRemoveFile?: (file: petitions_taskAttachmentOut) => void;
}) {
  if (dong.length === 0) {
    return <p className="m-0 py-10 text-center text-[12px] text-ink-muted">{NHAT_KY_RONG}</p>;
  }

  // The prototype's rows (`FeedbackActivityPanel.tsx:241-323`): initials, the name and the time, the
  // status pill ONLY when the status changed, the hand-over lines, the note, the files. The act's own
  // label stays beside the time — "Tạo nhiệm vụ", "Người dân đánh giá" would otherwise read as nothing.
  return (
    <ol aria-label="Nhật ký xử lý, mới nhất trước" className="m-0 flex list-none flex-col gap-3 p-0">
      {dong.map((d, i) => {
        const who = logActorLabel(d.actor_code, danhBa);
        const older = dong[i + 1];
        // Rows come newest first, so the row BELOW is the one before. The oldest row of the page has no
        // predecessor here: its pill follows whether the act moves a status at all.
        const changed =
          d.status !== "" && (older !== undefined ? older.status !== d.status : !LOG_ACTS_KEEPING_STATUS.has(d.action));
        return (
          <li key={d.id} className="flex gap-2.5">
            <span
              aria-hidden="true"
              className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full bg-brand/12 text-[11px] font-bold text-brand"
            >
              {initials(who)}
            </span>
            <div className="min-w-0 flex-1 [&>p]:m-0">
              <div className="flex flex-wrap items-baseline gap-x-2">
                <b className="text-navy text-[12.5px]">{who}</b>
                <time dateTime={d.at} className="text-[11px] text-ink-muted tabular-nums">
                  {dateTimeLabel(d.at)}
                </time>
                <span className="text-[11px] text-ink-muted">{nhanThaoTacNhatKy(d.action)}</span>
              </div>
              {changed && (
                <span className="mt-1 inline-block rounded-full bg-brand/12 px-2 py-0.5 text-[11px] font-semibold text-brand">
                  {nhanTrangThai(d.status)}
                </span>
              )}
              {laDongPhanCong(d.action) && (
                <>
                  <p className="mt-1 flex items-start gap-1.5 text-[12px] text-navy">
                    <Glyph icon={ArrowRightLeft} className="mt-0.5 size-3 shrink-0 text-ink-muted" />
                    <span>
                      <span className="text-ink-muted">Bộ phận: </span>
                      <b className="font-semibold">{nhanBoPhan(d.unit, tenBoPhan)}</b>
                    </span>
                  </p>
                  {d.assignee !== "" && (
                    <p className="mt-0.5 flex items-start gap-1.5 text-[12px] text-navy">
                      <Glyph icon={UserCheck} className="mt-0.5 size-3 shrink-0 text-ink-muted" />
                      <span>
                        <span className="text-ink-muted">Phụ trách: </span>
                        <b className="font-semibold">{nhanCanBoXuLy(d.assignee, danhBa)}</b>
                      </span>
                    </p>
                  )}
                </>
              )}
              {/* The class stays exactly `ghi-chu-nhat-ky` (it keeps the line breaks; a test reads it). */}
              {d.note !== "" && (
                <div className="mt-1 text-[12.5px] [&>p]:m-0">
                  <p className="ghi-chu-nhat-ky">{d.note}</p>
                </div>
              )}
              {/* The row's files (§8.7): a download link each, asked for at the click — never prefetched. */}
              {maTraCuu !== undefined && (
                <PetitionLogAttachmentList
                  lookupCode={maTraCuu}
                  attachments={d.attachments ?? []}
                  onRemove={onRemoveFile !== undefined && mayRemoveFiles(d) ? onRemoveFile : undefined}
                />
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

/** Acts that never move the status — the pill fallback for the oldest row on a page. */
const LOG_ACTS_KEEPING_STATUS: ReadonlySet<string> = new Set(["ghi-chu", "danh-gia", "tao-nhiem-vu"]);

/**
 * Up to two initials of the actor's NAME (the code in brackets is left out), as the prototype's `Avatar`.
 * A bare code (`CB-00123`) gives its letters; nothing gives "?".
 */
function initials(label: string): string {
  const name = label.replace(/\s*\([^)]*\)\s*$/, "");
  const letters = name
    .split(/\s+/)
    .filter((w) => w !== "")
    .slice(-2)
    .map((w) => w[0] ?? "")
    .join("")
    .toUpperCase();
  return letters === "" ? "?" : letters;
}

/**
 * Biểu mẫu `Ghi nhật ký` — ALWAYS OPEN under the panel's title, as in the prototype
 * (`FeedbackActivityPanel.tsx:159-231`): a 3-row box, `Đính kèm`, `Ghi nhật ký`. Controlled from
 * outside (the draft and its idempotency key live in `NhatKyPhieu`), so a failed send keeps the text and
 * the retry is the same write.
 *
 * Nút gửi khoá khi trống hoặc quá 2000 ký tự — nhưng giới hạn thật ở máy chủ, và câu 400 của nó vẫn
 * ra nguyên văn nếu hai bên lệch nhau. No autofocus: the box is there whenever the drawer opens, and
 * stealing focus from the drawer's title would hide where the officer is.
 */
export function BieuMauGhiNhatKy({
  id,
  noiDung,
  datNoiDung,
  dangGui,
  loi,
  gui,
  dinhKem,
  choTep = false,
}: {
  id: string;
  noiDung: string;
  datNoiDung: (s: string) => void;
  dangGui: boolean;
  loi: string | null;
  gui: () => void;
  /** The `📎 Đính kèm` picker (`AttachmentPicker`), passed in by the block that owns the files. */
  dinhKem?: ReactNode;
  /** A chosen file is still uploading or being checked: the entry waits for it. */
  choTep?: boolean;
}) {
  const loiO = loiGhiChuNhatKy(noiDung);
  const idO = `${id}-noi-dung`;
  // Empty is "nothing to send" (the button locks), not an error to shout about.
  const tooLong = loiO !== null && noiDung !== "";

  return (
    <form
      id={id}
      className="m-0 mt-2 flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (loiO === null && !dangGui && !choTep) gui();
      }}
    >
      <label htmlFor={idO} className="an-thi-giac">
        {NHAN_O_GHI_NHAT_KY}
      </label>
      <textarea
        id={idO}
        name={idO}
        rows={3}
        className={cn(TEXTAREA_CLASS, "bg-white", tooLong && "border-danger-600")}
        value={noiDung}
        placeholder={GOI_Y_GHI_NHAT_KY}
        onChange={(e) => datNoiDung(e.target.value)}
        aria-describedby={`${idO}-nhac`}
      />
      {tooLong && (
        <p className="m-0 text-[11px] text-danger-600" aria-live="polite">
          {loiO}
        </p>
      )}
      <p className="m-0 inline-flex items-start gap-1 text-[11px] text-ink-muted" id={`${idO}-nhac`}>
        <Glyph icon={LockKeyhole} className="mt-0.5 size-3 shrink-0" />
        {NHAC_DU_LIEU_CA_NHAN}
      </p>
      {choTep && <p className="ghi-chu m-0">{ATTACH_WAIT_NOTE}</p>}
      {loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}
      <div className="flex flex-wrap items-start gap-2">
        <div className="min-w-0 flex-1">{dinhKem}</div>
        {/* Busy = THIS form's own send (`dangGui` is the log block's state, not the drawer's). */}
        <button
          type="submit"
          className={buttonClass("primary", "sm", "ml-auto")}
          disabled={dangGui || choTep || loiO !== null}
        >
          <Glyph icon={Send} className="size-3.5" />
          {NHAN_NUT_GHI_NHAT_KY}
        </button>
      </div>
    </form>
  );
}
