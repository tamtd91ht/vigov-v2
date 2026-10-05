"use client";

import {
  BellRing,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Megaphone,
  Pin,
  Plus,
  Save,
  Send,
  Trash2,
  UserPlus,
} from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button, buttonVariants, LEGACY_BUTTON_CLASS } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { PageHeader } from "@/components/ui/page-header";
import { PendingButton, PendingMarker } from "@/components/ui/pending-feature";
import { Segmented } from "@/components/ui/segmented";
import { cn } from "@/lib/cn";
import {
  coTrangTruoc,
  sangTrangSau,
  TRANG_DAU,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi";
import type { KetQua } from "@/lib/api/goi";
import type {
  comms_thongBaoRa,
  identity_danhBaChonNguoiRa,
  page_Result_comms_thongBaoRa,
} from "@/lib/api/schema.gen";
import { laySoThongBao, phatHanhThongBao, type PhatHanhThongBaoVao } from "@/lib/api/thong-bao";

import {
  CANH_BAO_CHUA_GUI_THU,
  CANH_BAO_GHIM_TRONG_TRANG,
  CHIP_BAT_BUOC_XAC_NHAN,
  CHON_NGUOI_NHAN_RONG,
  CHUA_CHON_THONG_BAO,
  cauLoiDanhBa,
  coChipTrangThai,
  DANG_TAI_DANH_BA,
  DANG_TAI_SO,
  DANH_BA_NGUOI_NHAN_RONG,
  DAU_GACH,
  GHI_CHU_GHIM_TRONG_TRANG,
  GHI_CHU_NGUOI_NHAN,
  MA_CAN_BO_TOI_DA,
  MO_TA_MAN,
  MO_TA_SOAN,
  mocThe,
  nangGhimLenDau,
  NGUOI_NHAN_TOI_DA,
  NHAN_NUT_HUY,
  NHAN_NUT_PHAT_HANH,
  NHAN_CHON_NGUOI_NHAN,
  NHAN_NUT_SOAN,
  nhanBoDemXacNhan,
  nhanLuaChonNguoiNhan,
  nhanMoc,
  nhanTrangThai,
  nhanTrangThaiThu,
  NOI_DUNG_TOI_DA,
  NUT_THEM_NGUOI_NHAN,
  ACKNOWLEDGED_CHIP,
  pendingPart,
  RECIPIENT_UNITS_BLOCK_TITLE,
  RECIPIENT_UNITS_CHIP,
  RECIPIENT_UNITS_FIELD_LABEL,
  RECIPIENTS_EXTRA_LABEL,
  recipientsTitle,
  SAVE_DRAFT_LABEL,
  SCOPE_ALL,
  SCOPE_ALL_LABEL,
  SCOPE_LEGEND,
  SCOPE_MINE,
  SCOPE_MINE_LABEL,
  WITHDRAW_LABEL,
  PINNED_LABEL,
  PLACEHOLDER_TIEU_DE,
  PUBLISH_BUSY_LABEL,
  SO_RONG,
  tachMaNguoiNhan,
  themMaNguoiNhan,
  TIEU_DE_MAN,
  TIEU_DE_TOI_DA,
  trichNoiDung,
} from "./nhan-thong-bao";
import {
  AckRequiredBadge,
  AnnouncementCardsSkeleton,
  AnnouncementStatusBadge,
  EmailStatusBadge,
  Glyph,
  LoadingBar,
  SubmitContent,
} from "./announcement-ui";

/**
 * Sổ Thông báo nội bộ — composition mirrors the prototype `AnnouncementWorkspace` +
 * `AnnouncementForm` (`../vigov-require/apps/admin`, ADR 0068 §Sửa đổi 06/10/2026 lần 5): header row
 * [title · scope segments · `+ Soạn thông báo`], then a list of whole-card buttons beside a 22rem
 * detail panel, and the compose form in a DIALOG. Colours and type stay our tokens (lần 2).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐIỀU QUAN TRỌNG NHẤT CỦA MÀN NÀY: **KHÔNG CÓ Ô CHỌN BỘ PHẬN DÙNG ĐƯỢC**, và đó là một câu trả
 * lời chứ không phải một phần quên vẽ.
 *
 * `POST /api/v1/announcements` trả **501 `not_implemented`** cho mọi thân mang `org_unit_ids`.
 * Nở một bộ phận thành danh sách cán bộ là dữ liệu của `identity` và chưa có RPC nào làm việc ấy.
 * Vì thế chỗ của ô chọn bộ phận là MỘT chip bị vô hiệu kèm dấu "?" (ADR 0068 §14) — không phải năm
 * nút mà mọi lần bấm đều hỏng sau khi cán bộ đã gõ xong cả nội dung. Thân yêu cầu không bao giờ
 * mang `org_unit_ids`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * PHẦN CHƯA DỰNG (ADR 0068 §14): vẽ đúng chỗ prototype đặt nó, vô hiệu, dấu "?" mở mô tả lấy từ
 * `PHAN_CHUA_DUNG` (`pendingPart`). Không gọi máy chủ, không lưu gì.
 *
 * KHÔNG CÓ CỔNG QUYỀN Ở TRANG: `service-comms` kiểm `announcement.create` trên TỪNG lời gọi; tài
 * khoản thiếu khoá nhận nguyên câu 403 của máy chủ ra màn hình. Ẩn một nút chưa bao giờ là biện
 * pháp (luật 5, cấm #1).
 *
 * NỘI DUNG THÔNG BÁO LÀ CHỮ MỘT CÁN BỘ VỪA GÕ VÀ CÓ THỂ NHẮC TỚI HỒ SƠ CÔNG DÂN: không dòng nào ở
 * đây ghi nó vào log, vào tên tệp hay vào một URL (luật 3, cấm #1 và #4). Mã cán bộ người nhận
 * cũng không — nó là định danh của một con người.
 */

/**
 * Đọc danh bạ cho ô chọn người nhận.
 *
 * DANH BẠ CHỌN NGƯỜI, KHÔNG PHẢI SỔ QUẢN TRỊ: `GET /api/v1/staff-directory` là `AnyAuthenticated`
 * và chỉ trả mã · họ tên · chức vụ · bộ phận. `GET /api/v1/staff` đòi `admin.user` — một khoá quản
 * trị mà người soạn thông báo gần như không bao giờ cầm, và nó trả cả số di động cá nhân mà ô chọn
 * này không cần. KHÔNG truyền `permission`: ai trong xã cũng nhận được thông báo. Cả xã, không lọc
 * bộ phận — người nhận chọn theo từng người (bộ phận trả 501, xem `PHAN_CHUA_DUNG`, mục `byUnit`).
 */
export function docDanhBaNguoiNhan(): Promise<KetQua<identity_danhBaChonNguoiRa>> {
  return layDanhBaChonNguoi();
}

/** Bao nhiêu thẻ một trang. Mỗi thẻ mang cả toàn văn nội dung, nên trang mỏng. */
const SO_THE_MOI_TRANG = 10;

/** Id of the compose dialog's heading — its accessible name. */
const COMPOSE_TITLE_ID = "tieu-de-soan-thong-bao";

type TrangThaiTai<T> =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duLieu: T };

function taiTu<T>(daTai: { khoa: string; kq: KetQua<T> } | null, khoa: string): TrangThaiTai<T> {
  if (daTai === null || daTai.khoa !== khoa) return { pha: "dangTai" };
  return daTai.kq.ok
    ? { pha: "xong", duLieu: daTai.kq.duLieu }
    : { pha: "loi", thongBao: daTai.kq.thongBao };
}

export function SoThongBao() {
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);
  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_comms_thongBaoRa>;
  } | null>(null);

  const [dangMoBieuMau, datDangMoBieuMau] = useState(false);
  const [dangGui, datDangGui] = useState(false);
  const [loiBieuMau, datLoiBieuMau] = useState<string | null>(null);

  // Id của thẻ đang chọn, KHÔNG phải cả bản ghi. Giữ bản ghi ở đây là giữ một bản sao thứ hai của
  // một hàng vừa tải: sau một lần phát hành, bản sao ấy là bản cũ, và cột phải sẽ hiện một con số
  // xác nhận không còn đúng trong khi danh sách bên trái đã mới.
  const [dangChon, datDangChon] = useState<string | null>(null);

  // Đếm số lần GHI THÀNH CÔNG. Nó đi vào `key` của biểu mẫu, nên một lần ghi xong là một lần biểu
  // mẫu dựng lại từ đầu: các ô trống trở lại VÀ một khoá chống trùng mới được sinh. Lần ghi HỎNG
  // thì không tăng — biểu mẫu giữ nguyên chữ đã gõ và giữ nguyên khoá cũ, đúng điều
  // `Idempotency-Key` sinh ra để làm.
  const [lanGhiXong, datLanGhiXong] = useState(0);

  // Danh bạ cho ô chọn người nhận — đọc MỘT lần cho cả màn, không theo mỗi lần mở biểu mẫu. Hỏng
  // thì biểu mẫu vẫn gửi được: ô gõ mã không phụ thuộc nó.
  const [danhBa, datDanhBa] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);

  const khoa = `${nganXep.hienTai ?? ""}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    docDanhBaNguoiNhan().then((kq) => {
      if (!bo) datDanhBa(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  useEffect(() => {
    let bo = false;
    laySoThongBao({ limit: SO_THE_MOI_TRANG, cursor: nganXep.hienTai }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [nganXep.hienTai, khoa]);

  const so = taiTu(daTai, khoa);

  function phatHanh(than: PhatHanhThongBaoVao, khoaChongTrung: string): void {
    datDangGui(true);
    phatHanhThongBao(than, khoaChongTrung).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ, kể cả câu 501. Câu ấy CHỈ ĐƯỜNG — nó nói đúng mục biểu mẫu còn
        // dùng được — và viết lại nó ở client là dựng bản sao thứ hai của một quy tắc nghiệp vụ.
        datLoiBieuMau(kq.thongBao);
        return;
      }
      datLoiBieuMau(null);
      datDangMoBieuMau(false);
      datLanGhiXong((n) => n + 1);
      // Về TRANG ĐẦU: thẻ mới nhất nằm trên cùng, và con trỏ của trang đang xem không còn nghĩa
      // sau khi một bản ghi chen vào đầu danh sách.
      datNganXep(TRANG_DAU);
      datLanTai((n) => n + 1);
    });
  }

  function closeForm(): void {
    datDangMoBieuMau(false);
    datLoiBieuMau(null);
  }

  // Thẻ đang chọn được TÌM LẠI trong dữ liệu vừa tải, không giữ riêng. Sang trang khác hoặc sau
  // một lần phát hành, thẻ ấy có thể không còn trong trang — khi đó cột phải quay về câu "Chọn một
  // thông báo để xem." thay vì hiện một bản ghi không còn nằm trong danh sách bên trái.
  const dsTrongTrang = so.pha === "xong" ? nangGhimLenDau(so.duLieu.items) : [];
  const theDangChon = dsTrongTrang.find((t) => t.id === dangChon) ?? null;

  // The previous page, while a re-read is in flight (after a publish, or "Tải lại"): drawn dimmed
  // and `inert`, never acted on (spec v2 §8b "Đang tải lại"). A DIFFERENT subtree from the fresh
  // list, so the fresh list mounts anew exactly as before.
  const previousPage =
    so.pha === "dangTai" && daTai !== null && daTai.kq.ok && daTai.kq.duLieu.items.length > 0
      ? nangGhimLenDau(daTai.kq.duLieu.items)
      : null;

  // The prototype does not page; this register does (cursor pages of 10). The two buttons are drawn
  // only when there IS another page, so a one-page register looks like the prototype's.
  const hasOtherPage = so.pha === "xong" && (coTrangTruoc(nganXep) || so.duLieu.has_more);

  return (
    <>
      {/* The prototype's header row: title + one line, then the scope segments, then the primary
          button, which OPENS the compose dialog. */}
      <PageHeader
        icon={Megaphone}
        title={TIEU_DE_MAN}
        subtitle={<span className="mo-ta-trang m-0 max-w-none text-[13px] text-ink-500">{MO_TA_MAN}</span>}
        actions={
          <>
            {/* `Cả sổ thông báo` is what the list shows and the only answerable segment; `Gửi cho tôi`
                is the disabled placeholder (ADR 0068 §14). Nothing is emitted: the read route takes
                no scope at all. */}
            <Segmented
              legend={SCOPE_LEGEND}
              name="announcement-scope"
              value={SCOPE_ALL}
              onChange={() => {}}
              options={[
                { value: SCOPE_MINE, label: SCOPE_MINE_LABEL, pending: pendingPart("scopeMine") },
                { value: SCOPE_ALL, label: SCOPE_ALL_LABEL },
              ]}
            />
            <Button
              type="button"
              variant="primary"
              icon={<Glyph icon={Plus} />}
              aria-haspopup="dialog"
              onClick={() => {
                datDangMoBieuMau(true);
                datLoiBieuMau(null);
              }}
            >
              {NHAN_NUT_SOAN}
            </Button>
          </>
        }
      />

      {/* `[&>*]:my-0`: the section's `gap` is the one spacing between blocks; legacy vertical
          margins on direct children would add to it unevenly. */}
      <section
        className="man-thong-bao mt-0 flex min-w-0 flex-col gap-3 [&>*]:my-0"
        aria-labelledby="tieu-de-so-thong-bao"
      >
        {/* The prototype has no visible list heading; this one stays for the outline (h1 → h2 → the
            detail's h3), read by assistive technology only. */}
        <h2 id="tieu-de-so-thong-bao" className="an-thi-giac">
          Danh sách thông báo
        </h2>

        {/* LOADING (spec v2 §8b). The sentence stays the live region; the eye gets a 2px bar and
            either the previous page dimmed (a re-read) or card-shaped placeholders (first read). */}
        {so.pha === "dangTai" && (
          <div className="flex min-w-0 flex-col gap-3">
            <LoadingBar />
            <p className="an-thi-giac" role="status">
              {DANG_TAI_SO}
            </p>
            {previousPage !== null ? (
              <div inert className="pointer-events-none opacity-60 transition-opacity">
                <DanhSachThongBao thongBao={previousPage} dangChon={dangChon} chon={() => {}} />
              </div>
            ) : (
              <AnnouncementCardsSkeleton />
            )}
          </div>
        )}

        {/* LOAD ERROR (spec v2 §8b): the server's sentence VERBATIM is the alert; `Tải lại` asks the
            same read again through the screen's existing re-read key (`lanTai`). */}
        {so.pha === "loi" && (
          <Card>
            <ErrorState
              role="alert"
              title="Chưa tải được danh sách thông báo"
              message={so.thongBao}
              onRetry={() => datLanTai((n) => n + 1)}
            />
          </Card>
        )}

        {/* Empty register: the prototype's one dashed box, no detail panel beside it. */}
        {so.pha === "xong" && dsTrongTrang.length === 0 && (
          <DanhSachThongBao thongBao={dsTrongTrang} dangChon={dangChon} chon={datDangChon} />
        )}

        {/* The prototype's grid: list, and a 22rem detail panel from 1024px; stacked below that. */}
        {so.pha === "xong" && dsTrongTrang.length > 0 && (
          <div className="grid min-w-0 gap-3 lg:grid-cols-[minmax(0,1fr)_22rem] lg:items-start">
            <div className="flex min-w-0 flex-col gap-2">
              <DanhSachThongBao thongBao={dsTrongTrang} dangChon={dangChon} chon={datDangChon} />
            </div>
            <div className="min-w-0 lg:sticky lg:top-4">
              <ChiTietThongBao thongBao={theDangChon} />
            </div>
          </div>
        )}

        {so.pha === "xong" && hasOtherPage && (
          <nav className="dieu-huong-trang m-0" aria-label="Phân trang danh sách thông báo">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              icon={<Glyph icon={ChevronLeft} />}
              disabled={!coTrangTruoc(nganXep)}
              onClick={() => datNganXep(veTrangTruoc(nganXep))}
            >
              Trang trước
            </Button>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi
              // tới đó.
              disabled={!so.duLieu.has_more || so.duLieu.next_cursor === ""}
              onClick={() => datNganXep(sangTrangSau(nganXep, so.duLieu.next_cursor))}
            >
              Trang sau
              <Glyph icon={ChevronRight} />
            </Button>
          </nav>
        )}
      </section>

      {dangMoBieuMau && (
        <FormSoanThongBao
          // Khoá dựng lại: mỗi lần GHI XONG là một biểu mẫu mới, một khoá chống trùng mới.
          key={`bieu-mau|${lanGhiXong}`}
          dangGui={dangGui}
          loi={loiBieuMau}
          danhBa={danhBa}
          huy={closeForm}
          phatHanh={phatHanh}
        />
      )}
    </>
  );
}

/**
 * The list — newest first, pinned cards lifted to the top of the PAGE.
 *
 * Empty → the prototype's dashed box. The screen has no filter, so "empty" is only ever the empty
 * register — never "empty under these filters".
 */
export function DanhSachThongBao({
  thongBao,
  dangChon,
  chon,
}: {
  thongBao: readonly comms_thongBaoRa[];
  dangChon: string | null;
  chon: (id: string) => void;
}) {
  if (thongBao.length === 0) {
    return (
      <p className="m-0 rounded-card border border-dashed border-line bg-surface px-6 py-16 text-center text-[13px] text-ink-500">
        {SO_RONG}
      </p>
    );
  }

  return (
    <>
      {/* Only where a pinned card is on the page: that is when "lifted within the page" matters. */}
      {thongBao.some((t) => t.pinned) && (
        <p className="ghi-chu m-0 flex items-start gap-1.5 text-xs text-ink-500">
          <Glyph icon={Pin} className="mt-0.5 size-3.5 shrink-0" />
          <span>{GHI_CHU_GHIM_TRONG_TRANG}</span>
        </p>
      )}
      <ul aria-label="Danh sách thông báo nội bộ" className="m-0 flex list-none flex-col gap-2 p-0">
        {thongBao.map((tb) => (
          <li key={tb.id}>
            <TheThongBao thongBao={tb} dangChon={tb.id === dangChon} chon={chon} />
          </li>
        ))}
      </ul>
    </>
  );
}

/**
 * One card, the prototype's shape: the WHOLE card is the button that opens the detail. Row 1: pin ·
 * title · chips; row 2: two-line excerpt; row 3: time · counter · mail word.
 *
 * THE SELECTED CARD IS MARKED BY `aria-current`, and its blue border is DRAWN FROM that attribute
 * (`aria-[current=true]:` utilities): one source for "selected", readable by a screen reader.
 *
 * Phrasing content only inside (`span`s): a `<button>` may not hold a heading or a block.
 */
export function TheThongBao({
  thongBao,
  dangChon,
  chon,
}: {
  thongBao: comms_thongBaoRa;
  dangChon: boolean;
  chon: (id: string) => void;
}) {
  const boDem = nhanBoDemXacNhan(thongBao);
  const chipThu = nhanTrangThaiThu(thongBao.email_status);
  const isIssued = thongBao.issued_at !== null && thongBao.issued_at !== "";

  return (
    <button
      type="button"
      aria-current={dangChon ? "true" : undefined}
      onClick={() => chon(thongBao.id)}
      className={cn(
        "block w-full cursor-pointer rounded-card border border-line bg-surface px-4 py-3 text-left [font-family:inherit] text-ink-900",
        "transition-[border-color,box-shadow] hover:border-brand-500",
        "aria-[current=true]:border-brand-500 aria-[current=true]:shadow-[0_0_0_2px_var(--brand-100)]",
        "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500",
      )}
    >
      <span className="flex min-w-0 flex-wrap items-center gap-2">
        {/* The prototype's pin is the icon alone; the word stays for assistive technology. */}
        {thongBao.pinned && (
          <>
            <Glyph icon={Pin} className="size-3.5 shrink-0 text-warning-600" />
            <span className="an-thi-giac">{PINNED_LABEL}</span>
          </>
        )}
        <span className="min-w-0 text-[13.5px] leading-snug font-bold break-words text-ink-900">
          {thongBao.title}
        </span>
        {coChipTrangThai(thongBao.status) && (
          <AnnouncementStatusBadge code={thongBao.status}>{nhanTrangThai(thongBao.status)}</AnnouncementStatusBadge>
        )}
        {thongBao.ack_required && <AckRequiredBadge>{CHIP_BAT_BUOC_XAC_NHAN}</AckRequiredBadge>}
      </span>

      {/* `line-clamp-2` on top of the character cut of `trichNoiDung` ("2 dòng, cắt bớt"). */}
      <span className="mt-1 line-clamp-2 block text-xs text-ink-500">{trichNoiDung(thongBao.body)}</span>

      <span className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-ink-500">
        {/* The prototype's "Soạn …" prefix when nothing has been issued yet (status `nhap`). */}
        <span>{isIssued ? mocThe(thongBao) : `Soạn ${mocThe(thongBao)}`}</span>
        {boDem !== null && <span>{boDem}</span>}
        {chipThu !== null && <EmailStatusBadge code={thongBao.email_status}>{chipThu}</EmailStatusBadge>}
      </span>
    </button>
  );
}

/** Small uppercase section label of the detail panel (the prototype's `Bộ phận nhận`, `Người nhận`). */
const SECTION_LABEL = "m-0 flex items-center gap-1.5 text-[11px] font-semibold uppercase text-ink-500";

/**
 * The detail panel (prototype `aside`, 22rem): title → full text → facts → `Bộ phận nhận` → action
 * row [`Xác nhận đã đọc`] [`Gỡ`] → `Người nhận (N)`.
 *
 * TOÀN VĂN LẤY TỪ CHÍNH PHẢN HỒI DANH SÁCH, không từ một tuyến chi tiết: `body` đi kèm mỗi thẻ vì
 * không có tuyến chi tiết nào, và máy chủ cố ý KHÔNG cắt bớt nó ở đó.
 *
 * PHẦN CHƯA DỰNG (ADR 0068 §14) at the prototype's positions: `Bộ phận nhận`, the two action
 * buttons, `Người nhận (N)`, and the mail status (beside the mail fact — the prototype shows it per
 * recipient, inside the list that is itself a placeholder). NEVER an empty list: an empty list tells
 * an officer this announcement reached nobody, while the truth is the screen cannot read the list.
 * The `(N)` is the server's own `recipient_count`, so the heading itself is a true figure.
 */
export function ChiTietThongBao({ thongBao }: { thongBao: comms_thongBaoRa | null }) {
  if (thongBao === null) {
    return (
      <Card as="aside" className="h-fit border border-line">
        <div className="flex flex-col items-center gap-2 px-4 py-10 text-center text-[12.5px] text-ink-500">
          <Glyph icon={BellRing} className="size-5 opacity-40" />
          <p className="m-0">{CHUA_CHON_THONG_BAO}</p>
        </div>
      </Card>
    );
  }

  const boDem = nhanBoDemXacNhan(thongBao);

  return (
    <Card as="aside" aria-labelledby="tieu-de-chi-tiet-thong-bao" className="h-fit border border-line p-4">
      <h3
        id="tieu-de-chi-tiet-thong-bao"
        className="m-0 text-sm leading-snug font-bold break-words text-ink-900"
      >
        {thongBao.title}
      </h3>
      {/* `whitespace-pre-line`: the line breaks the author typed are kept. Toàn văn, không cắt. */}
      <p className="m-0 mt-2 text-[12.5px] leading-relaxed break-words whitespace-pre-line text-ink-900">
        {thongBao.body}
      </p>

      <dl className="m-0 mt-3 grid grid-cols-2 gap-x-4 gap-y-2 [&_dd]:m-0 [&_dd]:text-xs [&_dd]:text-ink-900 [&_dt]:text-[11px] [&_dt]:font-semibold [&_dt]:text-ink-500">
        <div>
          <dt>Phát hành lúc</dt>
          <dd>{nhanMoc(thongBao.issued_at)}</dd>
        </div>
        <div>
          <dt>Người soạn</dt>
          {/* MÃ NGHIỆP VỤ (`CB-2026-7K3M9Q`), không phải họ tên và không phải id nội bộ (luật 6,
              bất biến 8). `service-comms` không sở hữu danh bạ cán bộ nên không có tên để nối. */}
          <dd>{thongBao.author_code === "" ? DAU_GACH : thongBao.author_code}</dd>
        </div>
        <div>
          <dt>Xác nhận đã đọc</dt>
          <dd>{boDem ?? "Không bắt buộc xác nhận"}</dd>
        </div>
        <div>
          <dt className="flex items-center gap-1.5">
            Thư điện tử
            {thongBao.email_requested && <PendingMarker info={pendingPart("emailStatus")} />}
          </dt>
          {/* HAI SỰ THẬT, KHÔNG MỘT: `email_requested` là điều đã được yêu cầu, `email_status` là
              điều đã xảy ra. Gộp chúng lại sẽ làm "chưa gửi được" không phân biệt được với "không
              ai yêu cầu gửi thư". */}
          <dd>
            {thongBao.email_requested ? "Có yêu cầu gửi" : "Không yêu cầu gửi"} ·{" "}
            {nhanTrangThaiThu(thongBao.email_status) ?? "Chưa gửi"}
          </dd>
        </div>
      </dl>

      <div className="mt-3">
        <p className={SECTION_LABEL}>
          {RECIPIENT_UNITS_BLOCK_TITLE}
          <PendingMarker info={pendingPart("byUnit")} />
        </p>
        <p className="m-0 mt-1 text-xs text-ink-500">{DAU_GACH}</p>
      </div>

      {/* The prototype's action row. `Xác nhận đã đọc` only where the announcement asks for it — the
          same flag that would decide the real button. */}
      <div className="mt-3 flex flex-wrap gap-2">
        {thongBao.ack_required && (
          <PendingButton
            info={pendingPart("acknowledge")}
            variant="primary"
            size="sm"
            icon={<Glyph icon={CheckCircle2} />}
          >
            {ACKNOWLEDGED_CHIP}
          </PendingButton>
        )}
        <PendingButton info={pendingPart("withdraw")} size="sm" icon={<Glyph icon={Trash2} />}>
          {WITHDRAW_LABEL}
        </PendingButton>
      </div>

      <div className="mt-4 border-t border-line pt-3">
        <p className={SECTION_LABEL}>
          {recipientsTitle(thongBao.recipient_count)}
          <PendingMarker info={pendingPart("recipients")} />
        </p>
      </div>
    </Card>
  );
}

/**
 * Biểu mẫu "Soạn thông báo" — the prototype's DIALOG (`AnnouncementForm`, 52rem): Tiêu đề → Nội
 * dung → Bộ phận nhận thông báo → Gửi thêm đích danh → one box of three options → Huỷ · Lưu nháp ·
 * Phát hành. Esc asks `huy`, as `Huỷ` does — except while a publish is in flight.
 *
 * HAI THỨ CHỈ LÀ CHỖ GIỮ: ô chọn bộ phận (máy chủ trả 501) và nút `Lưu nháp` (không có tuyến) — vô
 * hiệu, dấu "?" (ADR 0068 §14). Người nhận: ô chọn theo họ tên trên danh bạ chọn người
 * (`docDanhBaNguoiNhan`) THÊM mã vào ô gõ mã — ô gõ mã là thứ đi lên máy chủ và vẫn dùng được khi
 * danh bạ tải hỏng. Lý do từng cái nằm ở `PHAN_CHUA_DUNG`.
 */
export function FormSoanThongBao({
  dangGui,
  loi,
  danhBa,
  huy,
  phatHanh,
}: {
  dangGui: boolean;
  loi: string | null;
  /** `null` = đang tải. */
  danhBa: KetQua<identity_danhBaChonNguoiRa> | null;
  huy: () => void;
  phatHanh: (than: PhatHanhThongBaoVao, khoaChongTrung: string) => void;
}) {
  const [tieuDe, datTieuDe] = useState("");
  const [noiDung, datNoiDung] = useState("");
  const [nguoiNhan, datNguoiNhan] = useState("");
  const [maDangChon, datMaDangChon] = useState("");
  const [ghim, datGhim] = useState(false);
  const [batBuocXacNhan, datBatBuocXacNhan] = useState(false);
  // Ô này **mặc định bật**. Cột `gui_thu_dien_tu` ghi lại ĐIỀU ĐÃ ĐƯỢC YÊU CẦU, nên giá trị mặc
  // định được giữ nguyên — kèm một câu ngay dưới nói rằng chưa có thư nào đi.
  const [guiThu, datGuiThu] = useState(true);
  // Sinh ở chỗ MỞ biểu mẫu, không ở chỗ gửi: bấm lại sau một lỗi mạng phải dùng LẠI đúng khoá ấy,
  // vì lần gửi đầu có thể đã tới máy chủ và đã phát một thông báo đi khắp xã.
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  const tieuDeGon = tieuDe.trim();
  const noiDungGon = noiDung.trim();
  const dsNguoiNhan = tachMaNguoiNhan(nguoiNhan);

  // BA ĐIỀU KIỆN, KHÔNG HAI. Hợp đồng đánh dấu `title` và `body` bắt buộc, nhưng máy chủ còn từ
  // chối một thông báo KHÔNG CÓ NGƯỜI NHẬN — và đó là lần từ chối tốn nhất nếu để lọt: người soạn
  // gõ xong cả trang rồi mới biết. Máy chủ vẫn là nơi từ chối thật; nút tắt chỉ để không phải gõ lại.
  const duDieuKien = tieuDeGon !== "" && noiDungGon !== "" && dsNguoiNhan.length > 0;

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien) return;

    phatHanh(
      {
        title: tieuDeGon,
        body: noiDungGon,
        recipient_codes: dsNguoiNhan,
        pinned: ghim,
        ack_required: batBuocXacNhan,
        email_requested: guiThu,
      },
      khoaChongTrung,
    );
  }

  return (
    <ModalDialog
      titleId={COMPOSE_TITLE_ID}
      size="lg"
      className="max-w-[52rem]"
      onDismiss={() => (dangGui ? undefined : huy())}
    >
      <ModalDialogHeader titleId={COMPOSE_TITLE_ID} title={NHAN_NUT_SOAN} description={MO_TA_SOAN} />
      <form className="m-0 flex min-h-0 flex-col gap-4" onSubmit={guiNgay}>
        {/* The fields scroll between the header and the buttons (`ModalDialog` layout contract).
            `[&_.o-nhap]:mb-0`: the column `gap` is the one spacing between fields; the legacy 1rem
            bottom margin of `.o-nhap` would double it. Labels stay above their controls. */}
        <div className="flex min-h-0 flex-col gap-3 overflow-y-auto pr-1 [&_.o-nhap]:mb-0">
          <div className="o-nhap">
            <label htmlFor="tieu-de-thong-bao">Tiêu đề *</label>
            <input
              id="tieu-de-thong-bao"
              name="tieu-de-thong-bao"
              value={tieuDe}
              placeholder={PLACEHOLDER_TIEU_DE}
              maxLength={TIEU_DE_TOI_DA}
              autoComplete="off"
              onChange={(e) => datTieuDe(e.target.value)}
            />
          </div>

          <div className="o-nhap">
            <label htmlFor="noi-dung-thong-bao">Nội dung *</label>
            <textarea
              id="noi-dung-thong-bao"
              name="noi-dung-thong-bao"
              rows={6}
              value={noiDung}
              maxLength={NOI_DUNG_TOI_DA}
              onChange={(e) => datNoiDung(e.target.value)}
            />
          </div>

          {/* `Bộ phận nhận thông báo` — ONE disabled chip with its "?" (ADR 0068 §14), never the
              commune's unit names: those are the commune's data, and chips that each end in a 501
              after the whole text is typed are worse than none. Nothing here reaches the body. */}
          <div className="flex min-w-0 flex-col gap-1.5">
            <div className="flex items-center gap-1.5">
              <p className="m-0 text-xs leading-tight font-semibold text-ink-500">{RECIPIENT_UNITS_FIELD_LABEL}</p>
              <PendingMarker info={pendingPart("byUnit")} />
            </div>
            <div className="flex flex-wrap gap-1.5">
              <button
                type="button"
                disabled
                aria-pressed={false}
                className="inline-flex cursor-not-allowed items-center rounded-full border border-dashed border-line-strong bg-transparent px-3 py-1 [font-family:inherit] text-xs font-medium text-ink-500 opacity-60"
              >
                {RECIPIENT_UNITS_CHIP}
              </button>
            </div>
          </div>

          {/* The picker by name: ONE row — the select grows, the add button keeps its width beside it,
              bottoms aligned. It wraps under the select only when the row is too narrow for both. */}
          {danhBa !== null && danhBa.ok && danhBa.duLieu.items.length > 0 ? (
            <div className="flex min-w-0 flex-wrap items-end gap-2">
              <Field
                label={NHAN_CHON_NGUOI_NHAN}
                htmlFor="chon-nguoi-nhan-thong-bao"
                kind="select"
                grow="auto"
                className="min-w-0 flex-[1_1_260px]"
              >
                <select
                  id="chon-nguoi-nhan-thong-bao"
                  name="chon-nguoi-nhan-thong-bao"
                  value={maDangChon}
                  onChange={(e) => datMaDangChon(e.target.value)}
                >
                  <option value="">{CHON_NGUOI_NHAN_RONG}</option>
                  {danhBa.duLieu.items.map((cb) => (
                    <option key={cb.code} value={cb.code}>
                      {nhanLuaChonNguoiNhan(cb)}
                    </option>
                  ))}
                </select>
              </Field>
              <Button
                type="button"
                variant="secondary"
                icon={<Glyph icon={UserPlus} />}
                disabled={maDangChon === ""}
                onClick={() => {
                  datNguoiNhan(themMaNguoiNhan(nguoiNhan, maDangChon));
                  datMaDangChon("");
                }}
              >
                {NUT_THEM_NGUOI_NHAN}
              </Button>
            </div>
          ) : (
            // No select to label (loading, failed, empty directory): the caption and the ONE sentence
            // that says why, in the same place the picker would be.
            <div className="flex min-w-0 flex-col gap-1.5">
              <p className="m-0 text-xs leading-tight font-semibold text-ink-700">{NHAN_CHON_NGUOI_NHAN}</p>
              {danhBa === null && (
                <p className="m-0 text-[13px] text-ink-500" role="status">
                  {DANG_TAI_DANH_BA}
                </p>
              )}
              {danhBa !== null && !danhBa.ok && (
                <p className="thong-bao-loi m-0" role="alert">
                  {cauLoiDanhBa(danhBa.thongBao)}
                </p>
              )}
              {danhBa !== null && danhBa.ok && (
                <p className="m-0 text-[13px] text-ink-500">{DANH_BA_NGUOI_NHAN_RONG}</p>
              )}
            </div>
          )}

          <div className="o-nhap">
            <label htmlFor="nguoi-nhan-thong-bao">{RECIPIENTS_EXTRA_LABEL} *</label>
            <textarea
              id="nguoi-nhan-thong-bao"
              name="nguoi-nhan-thong-bao"
              rows={4}
              value={nguoiNhan}
              // KHÔNG CÓ `maxLength` TRÊN CẢ Ô: trần của máy chủ là hai trần khác nhau — 500 người và
              // 64 ký tự MỖI MÃ — nên một con số duy nhất trên cả ô sẽ cắt sai ở cả hai chiều. Máy chủ
              // từ chối bằng câu của nó, và câu ấy nói đúng chỗ sai.
              onChange={(e) => datNguoiNhan(e.target.value)}
            />
            <p className={HINT}>
              {GHI_CHU_NGUOI_NHAN} Tối đa {NGUOI_NHAN_TOI_DA} người, mỗi mã tối đa{" "}
              {MA_CAN_BO_TOI_DA} ký tự.
            </p>
          </div>

          {/* The prototype's one box of three options, one row each. The sentence that bounds what an
              option does today sits right under its row. The className sits AFTER `checked` on each
              box: tests read `id="…"[^>]*checked` inside the tag. */}
          <div className="flex flex-col gap-2 rounded-[10px] border border-line bg-surface-muted p-3">
            <div>
              <label htmlFor="ghim-thong-bao" className={OPTION_LABEL}>
                <input
                  id="ghim-thong-bao"
                  name="ghim-thong-bao"
                  type="checkbox"
                  checked={ghim}
                  onChange={(e) => datGhim(e.target.checked)}
                  className={OPTION_BOX}
                />{" "}
                Ghim lên đầu danh sách
              </label>
              <p className={OPTION_HINT}>{CANH_BAO_GHIM_TRONG_TRANG}</p>
            </div>

            <label htmlFor="bat-buoc-xac-nhan" className={OPTION_LABEL}>
              <input
                id="bat-buoc-xac-nhan"
                name="bat-buoc-xac-nhan"
                type="checkbox"
                checked={batBuocXacNhan}
                onChange={(e) => datBatBuocXacNhan(e.target.checked)}
                className={OPTION_BOX}
              />{" "}
              Bắt buộc xác nhận đã đọc
            </label>

            <div>
              <label htmlFor="gui-thu-dien-tu" className={OPTION_LABEL}>
                <input
                  id="gui-thu-dien-tu"
                  name="gui-thu-dien-tu"
                  type="checkbox"
                  checked={guiThu}
                  onChange={(e) => datGuiThu(e.target.checked)}
                  className={OPTION_BOX}
                />{" "}
                Gửi thư điện tử cho người nhận
              </label>
              <p className={OPTION_HINT}>{CANH_BAO_CHUA_GUI_THU}</p>
            </div>
          </div>

          {loi !== null && (
            <p className="thong-bao-loi m-0" role="alert">
              {loi}
            </p>
          )}
        </div>

        {/* The prototype's buttons, right-aligned: Huỷ · Lưu nháp · Phát hành; Phát hành stays LAST
            and is the form's one solid button. Native submit `<button>` with `type` first. */}
        <div className="flex shrink-0 flex-wrap justify-end gap-2">
          <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
            {NHAN_NUT_HUY}
          </Button>
          {/* `Lưu nháp` — a disabled placeholder (ADR 0068 §14). It is NOT a submit button: it can
              never publish. */}
          <PendingButton info={pendingPart("saveDraft")} icon={<Glyph icon={Save} />}>
            {SAVE_DRAFT_LABEL}
          </PendingButton>
          <button
            type="submit"
            className={cn(LEGACY_BUTTON_CLASS.primary, buttonVariants({ variant: "primary" }))}
            disabled={dangGui || !duDieuKien}
            aria-busy={dangGui}
          >
            <SubmitContent busy={dangGui} icon={Send} label={NHAN_NUT_PHAT_HANH} busyText={PUBLISH_BUSY_LABEL} />
          </button>
        </div>
      </form>
    </ModalDialog>
  );
}

/** One helper line under a control (spec §3 "Chú thích": 12px, `--ink-500`). */
const HINT = "ghi-chu mt-1.5 mb-0 text-xs text-ink-500";

/** One option row of the compose form's option box, and the sentence under it (indented to the word). */
const OPTION_LABEL = "flex cursor-pointer items-center gap-2 text-[13px] text-ink-900";
const OPTION_BOX = "size-4 shrink-0 cursor-pointer accent-brand-600";
const OPTION_HINT = "ghi-chu m-0 mt-1 pl-6 text-xs text-ink-500";
