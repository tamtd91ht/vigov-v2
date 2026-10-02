"use client";

import {
  CheckCheck,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Clock,
  Construction,
  Eye,
  Info,
  Megaphone,
  MousePointerClick,
  Pin,
  Plus,
  Send,
  Trash2,
  UserPlus,
  Users,
  X,
} from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants, LEGACY_BUTTON_CLASS } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { Notice } from "@/components/ui/notice";
import { PageHeader } from "@/components/ui/page-header";
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
  CHO_DANH_SACH_NGUOI_NHAN,
  CHO_NUT_GO,
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
  PHAM_VI_DANG_HIEN,
  PHAN_CHUA_DUNG,
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
 * Sổ Thông báo nội bộ — `docs/ui-ux/08-thong-bao.md` §2 (bố cục), §3 (thẻ), §4 (chi tiết),
 * §5 (biểu mẫu soạn).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐIỀU QUAN TRỌNG NHẤT CỦA MÀN NÀY: **KHÔNG CÓ Ô CHỌN BỘ PHẬN**, và sự vắng mặt ấy là một câu
 * trả lời chứ không phải một phần còn thiếu.
 *
 * `POST /api/v1/announcements` trả **501 `not_implemented`** cho mọi thân mang `org_unit_ids`.
 * Nở một bộ phận thành danh sách cán bộ là dữ liệu của `identity` và chưa có RPC nào làm việc ấy.
 * Vẽ năm con chip bộ phận như §5 mô tả sẽ là vẽ đúng năm nút mà mọi lần bấm đều hỏng — và hỏng
 * sau khi cán bộ đã gõ xong cả nội dung. Lý do đầy đủ nằm ở `PHAN_CHUA_DUNG`, hiện ngay đầu màn.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ CỔNG QUYỀN Ở CLIENT, và đó là một quyết định đã ghi — xem `PHAN_CHUA_DUNG`.
 * `service-comms` kiểm `announcement.create` trên TỪNG lời gọi; tài khoản thiếu khoá nhận nguyên
 * câu 403 của máy chủ ra màn hình. Ẩn một nút chưa bao giờ là biện pháp (luật 5, cấm #1).
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
 * bộ phận — người nhận chọn theo từng người (bộ phận trả 501, xem `PHAN_CHUA_DUNG`).
 */
export function docDanhBaNguoiNhan(): Promise<KetQua<identity_danhBaChonNguoiRa>> {
  return layDanhBaChonNguoi();
}

/** Bao nhiêu thẻ một trang. Mỗi thẻ mang cả toàn văn nội dung, nên trang mỏng. */
const SO_THE_MOI_TRANG = 10;

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

  return (
    <>
      {/* The `<h1>` moved here from `page.tsx`: the primary button sits on the title row (spec §5)
          and the state it toggles — whether the form is open — lives in this component. */}
      <PageHeader
        icon={Megaphone}
        title={TIEU_DE_MAN}
        subtitle={<span className="mo-ta-trang m-0 max-w-none text-[13px] text-ink-500">{MO_TA_MAN}</span>}
        actions={
          <Button
            type="button"
            variant="primary"
            icon={<Glyph icon={dangMoBieuMau ? X : Plus} />}
            aria-expanded={dangMoBieuMau}
            onClick={() => {
              datDangMoBieuMau(!dangMoBieuMau);
              datLoiBieuMau(null);
            }}
          >
            {NHAN_NUT_SOAN}
          </Button>
        }
      />

      {/* `[&>*]:my-0`: the section's `gap` is the one spacing between blocks (spec §6.9). */}
      <section
        className="man-thong-bao mt-0 flex min-w-0 flex-col gap-4 [&>*]:my-0"
        aria-labelledby="tieu-de-so-thong-bao"
      >
        <KhoiChuaDung />

        {dangMoBieuMau && (
          <FormSoanThongBao
            // Khoá dựng lại: mỗi lần GHI XONG là một biểu mẫu mới, một khoá chống trùng mới.
            key={`bieu-mau|${lanGhiXong}`}
            dangGui={dangGui}
            loi={loiBieuMau}
            danhBa={danhBa}
            huy={() => {
              datDangMoBieuMau(false);
              datLoiBieuMau(null);
            }}
            phatHanh={phatHanh}
          />
        )}

        {/* §2: list 2/3, detail 1/3 from 1024px; stacked (list, then detail) below that. */}
        <div className="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)] lg:items-start">
          <div className="flex min-w-0 flex-col gap-3">
            <h2 id="tieu-de-so-thong-bao" className="m-0 text-[15px] leading-snug font-semibold text-ink-900">
              Danh sách thông báo
            </h2>
            <p className="ghi-chu m-0 flex items-start gap-1.5 text-[13px] text-ink-500">
              <Glyph icon={Info} className="mt-0.5 size-4 shrink-0" />
              <span>{PHAM_VI_DANG_HIEN}</span>
            </p>

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

            {so.pha === "xong" && (
              <>
                <DanhSachThongBao thongBao={dsTrongTrang} dangChon={dangChon} chon={datDangChon} />
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
              </>
            )}
          </div>

          {so.pha === "xong" && (
            <div className="min-w-0 lg:sticky lg:top-4">
              <ChiTietThongBao thongBao={theDangChon} />
            </div>
          )}
        </div>
      </section>
    </>
  );
}

/**
 * Những phần đặc tả đòi mà hợp đồng hoặc lượt làm này không có — HIỆN LÊN ĐẦU MÀN, không giấu
 * trong chú thích mã.
 *
 * `<details>` chứ không phải một khối luôn mở: một bức tường chữ trên đầu màn hình là bức tường
 * người ta học cách không đọc.
 */
export function KhoiChuaDung() {
  // COLLAPSED, GREY (spec v2 §8.1, ADR 0068): a list of what is not built is not an alarm. The words
  // stay verbatim and stay in the HTML while closed — `<details>` only folds them.
  return (
    <details className="khoi-chua-khai group m-0">
      <summary className="flex cursor-pointer list-none items-center gap-2 [&::-webkit-details-marker]:hidden">
        <Glyph icon={Construction} className="size-[18px] shrink-0 text-ink-500" />
        <span className="font-semibold text-ink-700">
          {PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và lý
          do
        </span>
        <Glyph
          icon={ChevronDown}
          className="ml-auto size-4 shrink-0 text-ink-500 transition-transform group-open:rotate-180"
        />
      </summary>
      <dl className="danh-sach-truong">
        {PHAN_CHUA_DUNG.map((p) => (
          <div key={p.ten}>
            <dt>{p.ten}</dt>
            <dd>{p.viSao}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}

/**
 * Cột trái §2 — danh sách thẻ, mới nhất ở trên, thẻ ghim được nâng lên đầu TRANG.
 *
 * THE SELECTED CARD IS MARKED BY `aria-current`, and its blue border is DRAWN FROM that attribute
 * (`aria-[current=true]:` utilities): one source for "selected", readable by a screen reader, with
 * no second class that could disagree with it.
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
    // The screen has no filter, so "empty" here is only ever the empty register — never "empty under
    // these filters".
    return (
      <Card>
        <EmptyState
          icon={Megaphone}
          title={SO_RONG}
          description={`Bấm “${NHAN_NUT_SOAN}” để soạn thông báo đầu tiên.`}
        />
      </Card>
    );
  }

  return (
    <>
      <p className="ghi-chu m-0 flex items-start gap-1.5 text-[13px] text-ink-500">
        <Glyph icon={Pin} className="mt-0.5 size-4 shrink-0" />
        <span>{GHI_CHU_GHIM_TRONG_TRANG}</span>
      </p>
      <ul aria-label="Danh sách thông báo nội bộ" className="m-0 flex list-none flex-col gap-3 p-0">
        {thongBao.map((tb) => (
          <li key={tb.id}>
            <TheThongBao thongBao={tb} dangChon={tb.id === dangChon} chon={chon} />
          </li>
        ))}
      </ul>
    </>
  );
}

/** Một thẻ §3: tiêu đề · chip · trích nội dung · mốc · bộ đếm · chip thư. */
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

  return (
    <Card
      className="flex flex-col gap-3 p-4 transition-[border-color,box-shadow] aria-[current=true]:border-brand-500 aria-[current=true]:shadow-[0_0_0_3px_var(--brand-100)]"
      aria-current={dangChon ? "true" : undefined}
    >
      <div className="flex min-w-0 flex-wrap items-start gap-2">
        <h3 className="m-0 min-w-0 flex-1 basis-60 text-[15px] leading-snug font-semibold break-words text-ink-900">
          {thongBao.title}
        </h3>
        <div className="flex flex-wrap items-center gap-1.5">
          {/* Dấu ghim §3: icon + chữ, không còn một ký hiệu đứng một mình. */}
          {thongBao.pinned && (
            <Badge tone="info" icon={Pin}>
              {PINNED_LABEL}
            </Badge>
          )}
          {thongBao.ack_required && <AckRequiredBadge>{CHIP_BAT_BUOC_XAC_NHAN}</AckRequiredBadge>}
          {coChipTrangThai(thongBao.status) && (
            <AnnouncementStatusBadge code={thongBao.status}>{nhanTrangThai(thongBao.status)}</AnnouncementStatusBadge>
          )}
        </div>
      </div>

      {/* `line-clamp-2` on top of the character cut of `trichNoiDung` (§3 "2 dòng, cắt bớt"). */}
      <p className="m-0 line-clamp-2 text-sm text-ink-700">{trichNoiDung(thongBao.body)}</p>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-[13px] text-ink-500">
        <span className="inline-flex items-center gap-1.5">
          <Glyph icon={Clock} className="size-4 shrink-0" />
          {mocThe(thongBao)}
        </span>
        {boDem !== null && (
          <span className="inline-flex items-center gap-1.5">
            <Glyph icon={CheckCheck} className="size-4 shrink-0" />
            {boDem}
          </span>
        )}
        {chipThu !== null && <EmailStatusBadge code={thongBao.email_status}>{chipThu}</EmailStatusBadge>}
        <Button
          type="button"
          variant="secondary"
          size="sm"
          className="ml-auto"
          icon={<Glyph icon={Eye} />}
          onClick={() => chon(thongBao.id)}
        >
          Xem chi tiết
        </Button>
      </div>
    </Card>
  );
}

/**
 * Cột phải §4 — toàn văn nội dung của thẻ đang chọn.
 *
 * TOÀN VĂN LẤY TỪ CHÍNH PHẢN HỒI DANH SÁCH, không từ một tuyến chi tiết: `body` đi kèm mỗi thẻ vì
 * không có tuyến chi tiết nào, và máy chủ cố ý KHÔNG cắt bớt nó ở đó (nếu cắt, cột này sẽ hiện một
 * thông báo cụt mà không dòng nào nói ra).
 *
 * HAI KHỐI CỦA §4 KHÔNG CÓ Ở ĐÂY — `BỘ PHẬN NHẬN` và `NGƯỜI NHẬN (12)`. Chỗ của chúng là một dòng
 * chữ nói vì sao, không phải một danh sách rỗng: một danh sách rỗng nói với cán bộ rằng thông báo
 * này không có người nhận nào, trong khi sự thật là màn hình không đọc được danh sách ấy.
 */
export function ChiTietThongBao({ thongBao }: { thongBao: comms_thongBaoRa | null }) {
  if (thongBao === null) {
    return (
      <Card>
        <EmptyState icon={MousePointerClick} tone="neutral" title={CHUA_CHON_THONG_BAO} />
      </Card>
    );
  }

  const boDem = nhanBoDemXacNhan(thongBao);

  return (
    <Card aria-labelledby="tieu-de-chi-tiet-thong-bao">
      <CardHeader>
        <CardTitle as="h3" id="tieu-de-chi-tiet-thong-bao" className="min-w-0 flex-1 break-words">
          {thongBao.title}
        </CardTitle>
      </CardHeader>

      <CardContent className="flex flex-col gap-4">
        {/* `whitespace-pre-line`: the line breaks the author typed are kept. Toàn văn, không cắt. */}
        <p className="m-0 text-sm leading-relaxed break-words whitespace-pre-line text-ink-900">
          {thongBao.body}
        </p>

        <dl className="danh-sach-truong m-0 grid gap-3 [&_dd]:m-0 [&_dd]:text-sm [&_dd]:text-ink-900 [&_dt]:text-xs [&_dt]:font-semibold [&_dt]:text-ink-500">
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
            <dt>Thư điện tử</dt>
            {/* HAI SỰ THẬT, KHÔNG MỘT: `email_requested` là điều đã được yêu cầu, `email_status` là
                điều đã xảy ra. Gộp chúng lại sẽ làm "chưa gửi được" không phân biệt được với "không
                ai yêu cầu gửi thư". */}
            <dd>
              {thongBao.email_requested ? "Có yêu cầu gửi" : "Không yêu cầu gửi"} ·{" "}
              {nhanTrangThaiThu(thongBao.email_status) ?? "Chưa gửi"}
            </dd>
          </div>
        </dl>

        {/* CHỖ CỦA KHỐI NGƯỜI NHẬN VÀ NÚT GỠ — hai dòng chữ nói rõ vì sao, không phải hai nút mờ.
            Một nút mờ nói "bạn không có quyền"; sự thật là màn hình chưa dựng, và hai câu ấy không
            được lẫn vào nhau. */}
        <Notice tone="neutral" icon={Users}>
          {CHO_DANH_SACH_NGUOI_NHAN}
        </Notice>
        <Notice tone="neutral" icon={Trash2}>
          {CHO_NUT_GO}
        </Notice>
      </CardContent>
    </Card>
  );
}

/**
 * Biểu mẫu "Soạn thông báo" §5.
 *
 * ĐẶC TẢ GỌI NÓ LÀ MODAL. Ở đây nó là một thẻ nằm trong trang — KHÔNG phải một lớp phủ: đổi sang
 * lớp phủ là đổi cách mở/đóng biểu mẫu (bẫy focus, Esc, bấm ra ngoài), tức đổi hành vi chứ không
 * chỉ đổi hình (ADR 0068 §1).
 *
 * HAI THỨ CỦA §5 KHÔNG CÓ Ở ĐÂY: ô chọn bộ phận (máy chủ trả 501) và nút `Lưu nháp` (không có
 * tuyến). Ô chọn người nhận CÓ, trên danh bạ chọn người (`docDanhBaNguoiNhan`), nhưng không có email
 * như §5 vẽ vì danh bạ ấy không trả email. Lý do từng cái nằm ở `PHAN_CHUA_DUNG`, hiện ngay đầu màn.
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
  // §5: ô này **mặc định bật**. Cột `gui_thu_dien_tu` ghi lại ĐIỀU ĐÃ ĐƯỢC YÊU CẦU, nên giá trị
  // mặc định của đặc tả được giữ nguyên — kèm một câu ngay dưới nói rằng chưa có thư nào đi.
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
    <Card as="form" onSubmit={guiNgay} aria-labelledby="tieu-de-soan-thong-bao">
      <CardHeader>
        <span
          aria-hidden="true"
          className="grid size-9 shrink-0 place-items-center rounded-lg bg-brand-50 text-brand-600"
        >
          <Megaphone className="size-[18px]" strokeWidth={1.8} focusable="false" />
        </span>
        <div className="min-w-0 flex-1">
          <CardTitle as="h3" id="tieu-de-soan-thong-bao">
            Soạn thông báo
          </CardTitle>
          <p className={HINT}>{MO_TA_SOAN}</p>
        </div>
      </CardHeader>

      {/* `[&_.o-nhap]:mb-0`: the `gap` is the one spacing between fields; the legacy 1rem bottom
          margin of `.o-nhap` would double it. Labels stay above their controls (`.o-nhap`). */}
      <CardContent className="flex flex-col gap-4 [&_.o-nhap]:mb-0">
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

      {/* The picker: ONE row — the select grows, the add button keeps its width beside it, bottoms
          aligned (both 40px). It wraps under the select only when the row is too narrow for both. */}
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
        // No select to label (loading, failed, empty directory): the caption and the ONE sentence that
        // says why, in the same place the picker would be.
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
        <label htmlFor="nguoi-nhan-thong-bao">Gửi thêm đích danh *</label>
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

      {/* Three options side by side from 768px, each a tile: the box + its words, and the one
          sentence that bounds what the option does today right under it. The className sits AFTER
          `checked` on each box: tests read `id="…"[^>]*checked` inside the tag. */}
      <div className="grid gap-3 md:grid-cols-3">
        <div className={OPTION_TILE}>
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
          <p className={HINT}>{CANH_BAO_GHIM_TRONG_TRANG}</p>
        </div>

        <div className={OPTION_TILE}>
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
        </div>

        <div className={OPTION_TILE}>
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
          <p className={HINT}>{CANH_BAO_CHUA_GUI_THU}</p>
        </div>
      </div>

      {loi !== null && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}
      </CardContent>

      {/* Huỷ / Phát hành right-aligned at the foot (spec §6.5); Phát hành is the form's one solid
          button and stays LAST. Native submit `<button>` with `type` first, as before. */}
      <CardFooter className="justify-end">
        <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </Button>
        <button
          type="submit"
          className={cn(LEGACY_BUTTON_CLASS.primary, buttonVariants({ variant: "primary" }))}
          disabled={dangGui || !duDieuKien}
          aria-busy={dangGui}
        >
          <SubmitContent busy={dangGui} icon={Send} label={NHAN_NUT_PHAT_HANH} busyText={PUBLISH_BUSY_LABEL} />
        </button>
      </CardFooter>
    </Card>
  );
}

/** One helper line under a control (spec §3 "Chú thích": 12px, `--ink-500`). */
const HINT = "ghi-chu mt-1.5 mb-0 text-xs text-ink-500";

/** Frame of one checkbox option of the compose form. */
const OPTION_TILE = "flex min-w-0 flex-col rounded-xl border border-line bg-surface-muted p-3";
const OPTION_LABEL = "flex cursor-pointer items-start gap-2 text-sm font-semibold text-ink-900";
const OPTION_BOX = "mt-0.5 size-4 shrink-0 cursor-pointer accent-brand-600";
