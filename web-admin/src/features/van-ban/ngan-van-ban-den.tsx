"use client";

import { CircleCheck, FileText, LockKeyhole, X } from "lucide-react";
import { useCallback, type KeyboardEvent, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { Notice } from "@/components/ui/notice";
import { traTen, type BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { BusyLabel } from "@/features/danh-ba/busy-label";
import { nhanThoiDiem } from "@/features/phan-anh/nhan-phieu";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_danhSachLichSuChuyenRa,
  documents_lichSuChuyenRa,
  documents_vanBanDenRa,
} from "@/lib/api/schema.gen";

import {
  DAN_CHUYEN_XU_LY,
  DANG_TAI_CHI_TIET,
  DANG_TAI_LICH_SU,
  LICH_SU_RONG,
  LOI_THIEU_LY_DO_CHUYEN,
  NUT_DONG_CHI_TIET,
  NUT_XAC_NHAN_CHUYEN,
  O_CAN_BO_XU_LY,
  O_CO_QUAN_BAN_HANH,
  O_DEN_BO_PHAN,
  O_LY_DO_CHUYEN,
  TIEU_DE_DONG_THOI_GIAN,
  TIEU_DE_KHOI_CHUYEN,
  nhanBoPhanDangGiu,
  nhanCanBo,
  nhanDoKhan,
  nhanLoaiVanBan,
  nhanNgayCoThe,
  nhanTieuDeVanBanDen,
  nhanTrangThai,
  nhanTuBoPhan,
  trangThaiHanVanBan,
  type BanChuyen,
} from "./nhan-van-ban";
import { StatusChangeRow } from "./document-pending";
import { DeadlineMark, DocumentStatusBadge, Glyph, UrgencyBadge } from "./document-ui";

/**
 * Ngăn chi tiết MỘT văn bản đến — `docs/ui-ux/05-van-ban-don-thu.md §3.5`, phần áp được cho văn bản
 * đến: tiêu đề, trích yếu, hàng chip, các ô thông tin, dòng thời gian chuyển tiếp CHỈ ĐỌC, và khối
 * "Chuyển cho bộ phận khác".
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * MỘT KHỐI TRONG LUỒNG TRANG, KHÔNG PHẢI LỚP PHỦ — cùng khuôn `khoi-chi-tiet` mà ngăn chi tiết nhiệm
 * vụ và phiếu phản ánh đang dùng, vì ứng dụng chưa có thành phần lớp phủ nào để dùng lại. Vì không
 * phải hộp thoại nên KHÔNG có bẫy tiêu điểm: bẫy tiêu điểm trong một vùng không che phần còn lại của
 * trang là nhốt người dùng bàn phím khỏi một trang họ vẫn nhìn thấy. Thay vào đó: mở thì tiêu điểm
 * sang tiêu đề ngăn, `Esc` khi tiêu điểm ở trong ngăn thì đóng, đóng thì tiêu điểm về nút "Xem chi
 * tiết" của đúng dòng ấy (bên gọi làm việc ấy — nó biết dòng nào).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * BỐN NÚT ĐỔI TRẠNG THÁI LÀ CHỖ GIỮ VÔ HIỆU mang dấu "?" (`StatusChangeRow`, ADR 0068 §14): bộ trạng
 * thái riêng của văn bản đến đã chốt (30/09/2026) nhưng máy chủ chưa có tuyến đổi trạng thái. KHÔNG
 * CÓ "CHUYỂN THÀNH NHIỆM VỤ", có chủ ý: nó cần một hợp đồng giữa hai dịch vụ chưa có, và không nằm
 * trong bảng vị trí đã duyệt.
 *
 * KHÔNG CÓ NÚT NÀO SỬA HAY XOÁ MỘT DÒNG LỊCH SỬ: bảng lịch sử chỉ-thêm ở tầng CSDL (trigger
 * `lich_su_chuyen_chi_them`) và hợp đồng không có tuyến nào làm việc ấy (luật 7, cấm #5).
 */
export function NganVanBanDen({
  vb,
  lichSu,
  bayGio,
  traLoai,
  traBoPhan,
  coQuyenChuyen,
  ban,
  datBan,
  loi,
  dangGui,
  cauDaXong,
  tieuDiemChuyen,
  onGui,
  onDong,
}: {
  /** `null` = đang đọc. Lỗi hiện NGUYÊN câu máy chủ — 404 là một câu chung cho mọi ca. */
  vb: KetQua<documents_vanBanDenRa> | null;
  lichSu: KetQua<documents_danhSachLichSuChuyenRa> | null;
  /** Thời điểm hiện tại TRUYỀN VÀO: quá hạn là phép so sánh lúc vẽ, không phải trường lưu sẵn. */
  bayGio: Date;
  traLoai: BangTraDanhMuc;
  traBoPhan: BangTraDanhMuc;
  /** `document.route` — TIỆN DỤNG, không phải biện pháp: máy chủ kiểm lại trên từng yêu cầu. */
  coQuyenChuyen: boolean;
  ban: BanChuyen;
  datBan: (b: BanChuyen) => void;
  loi: string;
  dangGui: boolean;
  cauDaXong: string;
  /** Mở từ nút "Chuyển xử lý" của dòng: tiêu điểm vào ô bộ phận thay vì tiêu đề. */
  tieuDiemChuyen: boolean;
  onGui: () => void;
  onDong: () => void;
}) {
  const coVanBan = vb !== null && vb.ok;
  const tieuDe = coVanBan
    ? nhanTieuDeVanBanDen(vb.duLieu.number, vb.duLieu.year, vb.duLieu.received_date)
    : "Chi tiết văn bản đến";

  // TIÊU ĐIỂM LÚC MỞ, bằng ref gọi lại ỔN ĐỊNH. Một hàm viết thẳng trong JSX là hàm MỚI mỗi lần
  // vẽ, React gọi lại nó ở mỗi lần vẽ, và tiêu điểm bị giật về tiêu đề sau mỗi phím gõ vào ô lý do.
  // Khi mở để chuyển xử lý, ô bộ phận nhận tiêu điểm thay — xem `KhoiChuyenXuLy`.
  const tieuDiemTieuDe = useCallback(
    (el: HTMLHeadingElement | null) => {
      if (el !== null && !tieuDiemChuyen) el.focus();
    },
    [tieuDiemChuyen],
  );

  function banPhim(e: KeyboardEvent<HTMLElement>) {
    if (e.key === "Escape") {
      e.stopPropagation();
      onDong();
    }
  }

  return (
    <Card as="section" aria-labelledby="tieu-de-ngan-van-ban-den" onKeyDown={banPhim}>
      <CardHeader className="justify-between">
        <h3
          id="tieu-de-ngan-van-ban-den"
          className="m-0 min-w-0 flex-1 text-base leading-snug font-semibold text-ink-900 tabular-nums"
          tabIndex={-1}
          ref={tieuDiemTieuDe}
        >
          {tieuDe}
        </h3>
        <IconButton type="button" label={NUT_DONG_CHI_TIET} onClick={onDong}>
          <X aria-hidden="true" />
        </IconButton>
      </CardHeader>

      <CardContent className="flex min-w-0 flex-col gap-4 [&>*]:my-0">
        {vb === null && (
          <>
            <p role="status" className="an-thi-giac">
              {DANG_TAI_CHI_TIET}
            </p>
            {/* First-load placeholder (spec §8b), local: a shared Skeleton is being built elsewhere. */}
            <div aria-hidden="true" className="flex flex-col gap-3">
              <span className="h-4 w-3/4 rounded bg-line motion-safe:animate-pulse" />
              <span className="h-[26px] w-64 max-w-full rounded-full bg-line motion-safe:animate-pulse" />
              <span className="h-3 w-1/2 rounded bg-line motion-safe:animate-pulse" />
              <span className="h-3 w-2/3 rounded bg-line motion-safe:animate-pulse" />
            </div>
          </>
        )}
        {vb !== null && !vb.ok && (
          <p className="thong-bao-loi" role="alert">
            {vb.thongBao}
          </p>
        )}

        {coVanBan && (
          <>
            <ThongTinVanBanDen vb={vb.duLieu} bayGio={bayGio} traLoai={traLoai} traBoPhan={traBoPhan} />

            <DongThoiGianChuyen lichSu={lichSu} traBoPhan={traBoPhan} />

            {cauDaXong !== "" && (
              <p role="status" className="flex items-center gap-2 text-sm font-medium text-success-600">
                <Glyph icon={CircleCheck} className="size-[18px] shrink-0" />
                {cauDaXong}
              </p>
            )}

            {coQuyenChuyen && (
              <KhoiChuyenXuLy
                ban={ban}
                datBan={datBan}
                traBoPhan={traBoPhan}
                loi={loi}
                dangGui={dangGui}
                tieuDiem={tieuDiemChuyen}
                onGui={onGui}
              />
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}

/** Trích yếu, hàng chip, và các ô thông tin. Xuất ra để bài kiểm vẽ được từng phần. */
export function ThongTinVanBanDen({
  vb,
  bayGio,
  traLoai,
  traBoPhan,
}: {
  vb: documents_vanBanDenRa;
  bayGio: Date;
  traLoai: BangTraDanhMuc;
  traBoPhan: BangTraDanhMuc;
}) {
  // SUY RA LÚC VẼ, không lưu (luật 10, bất biến 3) — cùng hàm cột "Hạn xử lý" của bảng dùng.
  const han = trangThaiHanVanBan(vb.due_at, bayGio);
  const soKyHieu = vb.reference_no === undefined || vb.reference_no === "" ? "Không ghi" : vb.reference_no;

  return (
    <>
      <p className="m-0 text-[15px] leading-relaxed text-ink-900">{vb.summary}</p>

      {/* Spec §3.5 order: header text, then the status row, then the chips. Disabled placeholder. */}
      <StatusChangeRow />

      <p className="m-0 flex flex-wrap items-center gap-2" aria-label="Trạng thái và thuộc tính">
        <DocumentStatusBadge status={vb.status}>{nhanTrangThai(vb.status)}</DocumentStatusBadge>{" "}
        <Badge tone="neutral" icon={FileText}>
          {nhanLoaiVanBan(traTen(traLoai, vb.document_type))}
        </Badge>{" "}
        <UrgencyBadge urgency={vb.urgency ?? ""}>{nhanDoKhan(vb.urgency ?? "")}</UrgencyBadge>
      </p>

      {/* Label–value pairs, label above on a phone, beside from 640px (spec §8b "khối trạng thái"). */}
      <dl className="m-0 grid min-w-0 gap-x-4 gap-y-3 rounded-xl border border-line bg-surface-muted p-4 sm:grid-cols-[11rem_minmax(0,1fr)]">
        <Pair label={O_CO_QUAN_BAN_HANH}>{vb.issuing_body}</Pair>

        <Pair label="Số, ký hiệu · ngày văn bản">
          <span className="tabular-nums">
            {soKyHieu} · {nhanNgayCoThe(vb.document_date ?? "")}
          </span>
        </Pair>

        <Pair label="Bộ phận đang giữ">
          {nhanBoPhanDangGiu(traTen(traBoPhan, vb.holding_unit ?? ""))} ·{" "}
          {nhanCanBo(vb.assignee ?? "")}
        </Pair>

        <Pair label="Hạn xử lý">
          <DeadlineMark deadline={han} />
        </Pair>

        <Pair label="Người vào sổ">{vb.created_by}</Pair>
      </dl>
    </>
  );
}

/** One `<dt>`/`<dd>` pair of the panel's field list. */
function Pair({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-xs font-semibold text-ink-500 sm:pt-0.5">{label}</dt>
      <dd className="m-0 min-w-0 text-sm break-words text-ink-900">{children}</dd>
    </>
  );
}

/**
 * Dòng thời gian chuyển tiếp — CHỈ ĐỌC, GIỮ NGUYÊN THỨ TỰ MÁY CHỦ TRẢ (cũ nhất trước).
 *
 * Không sắp lại ở client: thứ tự là thứ tự các lần chuyển thật, và máy chủ đã xếp theo `routed_at`.
 * Người chuyển và cán bộ được giao hiện bằng MÃ CÁN BỘ — xem `nhanCanBo`.
 */
export function DongThoiGianChuyen({
  lichSu,
  traBoPhan,
}: {
  lichSu: KetQua<documents_danhSachLichSuChuyenRa> | null;
  traBoPhan: BangTraDanhMuc;
}) {
  return (
    <div aria-labelledby="tieu-de-dong-thoi-gian" className="flex min-w-0 flex-col gap-3">
      <h4 id="tieu-de-dong-thoi-gian" className="m-0 text-sm font-semibold text-ink-900">
        {TIEU_DE_DONG_THOI_GIAN}
      </h4>
      {lichSu === null && (
        <p role="status" className="m-0 text-[13px] text-ink-500">
          {DANG_TAI_LICH_SU}
        </p>
      )}
      {lichSu !== null && !lichSu.ok && (
        <p className="thong-bao-loi m-0" role="alert">
          {lichSu.thongBao}
        </p>
      )}
      {lichSu !== null && lichSu.ok && lichSu.duLieu.items.length === 0 && (
        <p className="m-0 text-[13px] text-ink-500 italic">{LICH_SU_RONG}</p>
      )}
      {lichSu !== null && lichSu.ok && lichSu.duLieu.items.length > 0 && (
        // A plain vertical list (oldest first, as the server ordered it): a rail on the left, one
        // dot per routing. Read-only — no control on any line (rule 7, forbidden #5).
        <ol className="m-0 flex list-none flex-col p-0">
          {lichSu.duLieu.items.map((d) => (
            <DongLichSu key={d.id} d={d} traBoPhan={traBoPhan} />
          ))}
        </ol>
      )}
    </div>
  );
}

function DongLichSu({ d, traBoPhan }: { d: documents_lichSuChuyenRa; traBoPhan: BangTraDanhMuc }) {
  return (
    <li className="relative flex min-w-0 flex-col gap-1.5 border-l-2 border-line pb-4 pl-5 text-sm text-ink-700 last:pb-0 [&>p]:m-0">
      <span
        aria-hidden="true"
        className="absolute top-1 -left-[7px] size-3 rounded-full border-2 border-surface bg-brand-500"
      />
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <strong className="font-semibold text-ink-900">{d.routed_by}</strong> ·{" "}
        <time dateTime={d.routed_at} className="text-ink-500 tabular-nums">
          {nhanThoiDiem(d.routed_at)}
        </time>
      </p>
      <p>
        <DocumentStatusBadge status={d.status}>{nhanTrangThai(d.status)}</DocumentStatusBadge>
      </p>
      <p>
        {nhanTuBoPhan(traTen(traBoPhan, d.from_unit ?? ""))} →{" "}
        {nhanBoPhanDangGiu(traTen(traBoPhan, d.to_unit))}
      </p>
      <p className="text-ink-500">Phụ trách: {nhanCanBo(d.assignee ?? "")}</p>
      <p className="break-words text-ink-900">{d.reason}</p>
    </li>
  );
}

/**
 * Khối "Chuyển cho bộ phận khác" — chuyển từ biểu mẫu trong trang của sổ vào đây, giữ nguyên ba ô
 * và phép kiểm (`guiChuyenVanBan` ở `thao-tac-van-ban.ts`).
 *
 * `reason` LÀ CHỮ TỰ DO CÓ THỂ NHẮC TÊN MỘT CÔNG DÂN: nó chỉ sống trong trạng thái React và trong
 * thân `POST`, không vào log, URL, `localStorage` hay một thuộc tính nào (luật 3).
 */
export function KhoiChuyenXuLy({
  ban,
  datBan,
  traBoPhan,
  loi,
  dangGui,
  tieuDiem,
  onGui,
}: {
  ban: BanChuyen;
  datBan: (b: BanChuyen) => void;
  traBoPhan: BangTraDanhMuc;
  loi: string;
  dangGui: boolean;
  tieuDiem: boolean;
  onGui: () => void;
}) {
  // Ổn định vì cùng lý do với tiêu đề ngăn: không giật tiêu điểm khỏi ô đang gõ.
  const tieuDiemO = useCallback(
    (el: HTMLSelectElement | null) => {
      if (el !== null && tieuDiem) el.focus();
    },
    [tieuDiem],
  );

  return (
    <form
      className="flex min-w-0 flex-col gap-4 border-t border-line pt-4"
      aria-labelledby="tieu-de-khoi-chuyen"
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <h4 id="tieu-de-khoi-chuyen" className="m-0 text-sm font-semibold text-ink-900">
        {TIEU_DE_KHOI_CHUYEN}
      </h4>
      <Notice tone="legal" icon={LockKeyhole}>
        {DAN_CHUYEN_XU_LY}
      </Notice>

      <div className="grid min-w-0 gap-4 sm:grid-cols-2">
      <Field label={O_DEN_BO_PHAN} htmlFor="o-den-bo-phan" kind="select" grow="auto">
        <select
          id="o-den-bo-phan"
          name="denBoPhan"
          value={ban.denBoPhan}
          ref={tieuDiemO}
          onChange={(e) => datBan({ ...ban, denBoPhan: e.target.value })}
        >
          <option value="">— Chọn bộ phận —</option>
          {traBoPhan.pha === "xong" &&
            [...traBoPhan.ten].map(([id, ten]) => (
              <option key={id} value={id}>
                {ten}
              </option>
            ))}
        </select>
      </Field>

      <Field label={O_CAN_BO_XU_LY} htmlFor="o-can-bo-xu-ly" grow="auto">
        {/* Ô CHỮ, KHÔNG PHẢI Ô CHỌN CÁN BỘ: hợp đồng nhận `assignee` là MÃ CÁN BỘ (`CB-00123`),
            và danh bạ cán bộ đòi khoá `admin.user` — một người có `document.route` chưa chắc
            đọc được danh bạ. Vẽ một ô chọn rỗng cho họ là vẽ một ô không bao giờ dùng được. */}
        <input
          id="o-can-bo-xu-ly"
          name="canBoXuLy"
          value={ban.canBoXuLy}
          autoComplete="off"
          onChange={(e) => datBan({ ...ban, canBoXuLy: e.target.value })}
          placeholder="Để trống nếu để bộ phận tự phân công"
        />
      </Field>
      </div>

      <Field label={O_LY_DO_CHUYEN} htmlFor="o-ly-do-chuyen" grow="auto">
        <input
          id="o-ly-do-chuyen"
          name="lyDoChuyen"
          required
          autoComplete="off"
          value={ban.lyDo}
          onChange={(e) => datBan({ ...ban, lyDo: e.target.value })}
          aria-invalid={loi === LOI_THIEU_LY_DO_CHUYEN}
        />
      </Field>

      {/* CÂU TỪ CHỐI RA NGUYÊN VĂN, dù từ phép kiểm ở client hay từ máy chủ. */}
      {loi !== "" && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}

      <div className="flex justify-end">
        <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui || undefined}>
          <BusyLabel busy={dangGui} label={NUT_XAC_NHAN_CHUYEN} busyText="Đang chuyển…" />
        </Button>
      </div>
    </form>
  );
}
