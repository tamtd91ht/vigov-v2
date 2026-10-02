"use client";

import {
  CircleCheck,
  LockKeyhole,
  LockKeyholeOpen,
  Pencil,
  Plus,
  ReceiptText,
  Trash2,
  TriangleAlert,
} from "lucide-react";
import { useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { Card, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { Notice } from "@/components/ui/notice";
import { PendingCell, PendingColumnHeader } from "@/components/ui/pending-feature";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import type { KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";
import {
  goChungTu,
  khoaChungTu,
  moKhoaChungTu,
  suaChungTu,
  themChungTu,
  xacNhanChungTu,
} from "@/lib/api/giai-ngan";
import type { finance_chungTuRa } from "@/lib/api/schema.gen";

import { FormLyDo } from "./form-ly-do";
import { nhanNgay, nhanTien } from "./nhan-du-an";
import {
  CANH_BAO_KHOA,
  CANH_BAO_SUA_VE_NHAP,
  CAU_THIEU_QUYEN_GHI,
  CAU_THIEU_QUYEN_XAC_NHAN,
  CHUNG_TU_DA_XAC_NHAN,
  DAU_GACH,
  DOI_TAC_TOI_DA,
  FORM_CHUNG_TU_TRONG,
  NOI_DUNG_CHUNG_TU_TOI_DA,
  nhanMocKhoa,
  nhanTrangThaiChungTu,
  SO_CHUNG_TU_TOI_DA,
  thaoTacChungTu,
  thanSuaChungTu,
  thanThemChungTu,
  pendingPart,
  type GiaTriFormChungTu,
} from "./nhan-ghi-giai-ngan";
import { VOUCHER_FUNDING_COLUMN } from "./pending-parts";
import { DeniedNote, Glyph, VoucherStatusBadge } from "./project-ui";

/**
 * Tab "Chứng từ" của §8.2 — sáu tuyến ghi của vòng đời chứng từ giải ngân.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * ⚠ BẢNG NÀY CHỈ CHỨA CHỨNG TỪ CỦA CHÍNH PHIÊN LÀM VIỆC NÀY, VÀ ĐÓ KHÔNG PHẢI MỘT LỰA CHỌN.
 *
 * Hợp đồng có sáu tuyến GHI chứng từ và **không có tuyến nào ĐỌC danh sách chứng từ** — máy chủ nói
 * thẳng đó là chủ ý (`service-finance/internal/http/chung_tu_giai_ngan.go`, khối đầu tệp). Bốn
 * tuyến `POST` · `PATCH` · `confirmation` · `lockout` trả về nguyên hàng, nên màn hình giữ lại được
 * đúng những hàng nó vừa chạm vào. Tải lại trang là bảng trống.
 *
 * Câu ấy được NÓI RA trên màn (`PHAN_CHUA_DUNG_GHI`, mục đầu tiên) chứ không giấu ở đây: một bảng
 * trống không kèm lời giải thích đọc thành "dự án này chưa chi đồng nào" — một khẳng định về tiền
 * của xã mà màn hình không có căn cứ nào để đưa ra.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * HAI KHOÁ, CHIA THEO TRỤC NGUY HIỂM (§8.2): `budget.update` NHẬP và SỬA; `budget.confirm` XÁC
 * NHẬN, KHOÁ, MỞ KHOÁ và GỠ. Đây là cách xã tách người nhập liệu khỏi người chịu trách nhiệm, nên
 * bốn nút dưới không bao giờ được đi chung một cổng với hai nút trên.
 *
 * KHÔNG GHI LOG GÌ: nội dung chi, đối tác và lý do gỡ là chữ cán bộ vừa gõ về tiền công quỹ.
 */

/** Ô của biểu mẫu, đổ từ một chứng từ máy chủ vừa trả về. */
export function giaTriTuChungTu(ct: finance_chungTuRa): GiaTriFormChungTu {
  return {
    ngayChi: ct.payment_date,
    // SỐ THÔ, KHÔNG ĐỊNH DẠNG — xem lý lẽ ở `giaTriTuDuAn`: ô nhập phải chứa đúng con số máy chủ
    // giữ, để phép so "có đổi không" không thấy khác ở một lần Lưu không sửa gì. Một `PATCH` thừa ở
    // tuyến này bóc mất chữ xác nhận của lãnh đạo (ADR 0036).
    soTien: String(ct.amount),
    noiDung: ct.description,
    doiTac: ct.counterparty ?? "",
    soChungTu: ct.voucher_no ?? "",
  };
}

/**
 * Biểu mẫu `+ Ghi nhận khoản chi` §8.2, dùng cho cả THÊM và SỬA.
 *
 * ⚠ KHỐI CẢNH BÁO ADR 0036 HIỆN NGAY TRÊN CÁC Ô, KHÔNG PHẢI SAU KHI LƯU. Sửa một chứng từ đang
 * `Đã xác nhận` sẽ kéo nó VỀ `Kế toán nhập` và xoá dấu người xác nhận. Máy chủ làm việc ấy trong im
 * lặng và trả 200; người phát hiện ra sau là người đi tìm chữ ký ấy lúc quyết toán.
 *
 * KHÔNG CÓ Ô `Trạng thái`: vòng đời đi qua các tuyến xác nhận / khoá / mở khoá, mỗi bước một vết
 * kiểm toán. Máy chủ trả 400 cho một thân mang `status`, và kiểu `ThemChungTuVao` đã loại nó ra ở
 * tầng biên dịch.
 *
 * KHÔNG CÓ Ô `Nguồn vốn`: chưa có tuyến nào đọc danh mục nguồn vốn — xem `PHAN_CHUA_DUNG_GHI`.
 */
export function FormChungTu({
  tieuDeForm,
  giaTriDau,
  trangThaiHienTai,
  dangGui,
  loi,
  huy,
  luu,
}: {
  tieuDeForm: string;
  giaTriDau: GiaTriFormChungTu;
  /** Trạng thái của chứng từ đang SỬA. `undefined` khi đang THÊM. */
  trangThaiHienTai?: string;
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (gt: GiaTriFormChungTu, khoaChongTrung: string) => void;
}) {
  const [gt, datGT] = useState<GiaTriFormChungTu>(giaTriDau);
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    luu(gt, khoaChongTrung);
  }

  return (
    <Card as="form" onSubmit={guiNgay} aria-label={tieuDeForm}>
      <CardHeader>
        <CardTitle as="h4">{tieuDeForm}</CardTitle>
      </CardHeader>

      <div className="flex min-w-0 flex-col gap-4 p-4">
        {trangThaiHienTai === CHUNG_TU_DA_XAC_NHAN && (
          <Notice tone="legal" icon={TriangleAlert}>
            {CANH_BAO_SUA_VE_NHAP}
          </Notice>
        )}

        {/* Labels above, 40px controls, two columns from 640px (spec §6.3, §6.5). */}
        <div className="grid min-w-0 gap-4 sm:grid-cols-2">
          {/* HAI NGÀY KHÁC NHAU, và xã nhập chứng từ tuần trước vào sáng thứ Hai là chuyện thường —
              mọi biểu đồ luỹ kế của §4 xếp theo ngày CHI, không theo ngày gõ. */}
          <Field
            label="Ngày chi *"
            htmlFor="ngay-chi-chung-tu"
            grow="auto"
            hint="Ngày tiền thực sự chi ra, không phải ngày gõ vào sổ."
          >
            <input
              id="ngay-chi-chung-tu"
              name="ngay-chi-chung-tu"
              type="date"
              value={gt.ngayChi}
              onChange={(e) => datGT({ ...gt, ngayChi: e.target.value })}
            />
          </Field>

          <Field label="Số tiền (đồng) *" htmlFor="so-tien-chung-tu" grow="auto">
            <input
              id="so-tien-chung-tu"
              name="so-tien-chung-tu"
              value={gt.soTien}
              inputMode="numeric"
              autoComplete="off"
              placeholder="30.000.000"
              className="tabular-nums"
              onChange={(e) => datGT({ ...gt, soTien: e.target.value })}
            />
          </Field>

          <Field label="Nội dung *" htmlFor="noi-dung-chung-tu" grow="auto" className="sm:col-span-2">
            <textarea
              id="noi-dung-chung-tu"
              name="noi-dung-chung-tu"
              rows={2}
              value={gt.noiDung}
              maxLength={NOI_DUNG_CHUNG_TU_TOI_DA}
              placeholder="Thanh toán đợt 3"
              className="py-2"
              onChange={(e) => datGT({ ...gt, noiDung: e.target.value })}
            />
          </Field>

          <Field label="Đối tác" htmlFor="doi-tac-chung-tu" grow="auto">
            <input
              id="doi-tac-chung-tu"
              name="doi-tac-chung-tu"
              value={gt.doiTac}
              maxLength={DOI_TAC_TOI_DA}
              autoComplete="off"
              onChange={(e) => datGT({ ...gt, doiTac: e.target.value })}
            />
          </Field>

          <Field label="Số chứng từ" htmlFor="so-chung-tu" grow="auto">
            <input
              id="so-chung-tu"
              name="so-chung-tu"
              value={gt.soChungTu}
              maxLength={SO_CHUNG_TU_TOI_DA}
              autoComplete="off"
              onChange={(e) => datGT({ ...gt, soChungTu: e.target.value })}
            />
          </Field>
        </div>

        {loi !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {loi}
          </p>
        )}
      </div>

      <CardFooter className="justify-end">
        <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
          Huỷ
        </Button>
        <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui || undefined}>
          <BusyLabel busy={dangGui} label="Lưu chứng từ" busyText={BUSY_SAVING} />
        </Button>
      </CardFooter>
    </Card>
  );
}

/**
 * Bảng chứng từ §8.2 với cột thao tác.
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * CỘT THAO TÁC LÀ GIAO CỦA HAI ĐIỀU, VÀ CẢ HAI ĐỀU CẦN:
 *
 *   1. TRẠNG THÁI cho phép gì (`thaoTacChungTu`) — vòng đời là một dây xích: chưa xác nhận thì
 *      chưa khoá được, đã khoá thì không sửa và không gỡ.
 *   2. TÀI KHOẢN có khoá nào — `budget.update` cho Sửa, `budget.confirm` cho bốn nút còn lại.
 *
 * Thiếu vế 1 thì màn hình vẽ những nút chắc chắn nhận 409. Thiếu vế 2 thì màn hình mở thao tác xác
 * nhận cho người chỉ được nhập liệu. Không vế nào thay được vế nào, và không vế nào là biện pháp:
 * máy chủ kiểm lại cả hai trên TỪNG yêu cầu (luật 5, cấm #1).
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 */
export function BangChungTu({
  ds,
  coGhi,
  coXacNhan,
  dangGui,
  moSua,
  moGo,
  moMoKhoa,
  xacNhan,
  khoa,
}: {
  ds: readonly finance_chungTuRa[];
  coGhi: boolean;
  coXacNhan: boolean;
  dangGui: boolean;
  moSua: (id: string) => void;
  moGo: (id: string) => void;
  moMoKhoa: (id: string) => void;
  xacNhan: (id: string) => void;
  khoa: (id: string) => void;
}) {
  if (ds.length === 0) {
    // The sentence is unchanged, split between the title and the one guidance line (spec §7).
    return (
      <EmptyState
        icon={ReceiptText}
        title="Chưa có chứng từ nào trong phiên làm việc này."
        description="Bảng này không đọc được chứng từ đã lưu từ trước — hợp đồng chưa có tuyến đọc danh sách chứng từ."
      />
    );
  }

  return (
    <TableScroll sticky aria-label="Chứng từ giải ngân vừa ghi" className="rounded-none border-0 shadow-none">
      <table className={cn("bang-danh-muc", DATA_TABLE_CLASS)}>
        <caption className="an-thi-giac">
          Chứng từ giải ngân đã ghi hoặc đã đổi trạng thái trong phiên làm việc này
        </caption>
        <thead>
          <tr>
            <th scope="col">Ngày chi</th>
            <th scope="col" className="text-right">
              Số tiền
            </th>
            {/* Spec §8.2's NGUỒN VỐN column — a "?" placeholder (ADR 0068 §14), "—" in every row. */}
            <PendingColumnHeader info={pendingPart(VOUCHER_FUNDING_COLUMN)}>Nguồn vốn</PendingColumnHeader>
            <th scope="col">Nội dung</th>
            <th scope="col">Chứng từ</th>
            <th scope="col">Trạng thái</th>
            <th scope="col" className="text-right">
              Thao tác
            </th>
          </tr>
        </thead>
        <tbody>
          {ds.map((ct) => {
            const cho = thaoTacChungTu(ct.status);
            const ngay = nhanNgay(ct.payment_date);
            return (
              <tr key={ct.id}>
                <td className="tabular-nums">{ngay}</td>
                {/* IN ĐÚNG CON SỐ MÁY CHỦ TRẢ. Không đổi đơn vị, không làm tròn: một con số sai ở
                    đây đi thẳng vào báo cáo ngân sách. */}
                <td className="text-right tabular-nums">{nhanTien(ct.amount)}</td>
                <PendingCell />
                <td className="whitespace-normal">
                  {ct.description}
                  {ct.counterparty !== undefined && ct.counterparty !== "" && (
                    <span className="dong-phu">{ct.counterparty}</span>
                  )}
                </td>
                <td>{ct.voucher_no === undefined || ct.voucher_no === "" ? DAU_GACH : ct.voucher_no}</td>
                <td>
                  <VoucherStatusBadge status={ct.status}>{nhanTrangThaiChungTu(ct.status)}</VoucherStatusBadge>
                  {/* MỐC KHOÁ VÀ SỐ LẦN MỞ KHOÁ ĐỀU HIỆN, và `unlock_count` là con số nói rằng đã
                      có những lần mở khoá KHÁC — bốn trường mở khoá chỉ mô tả lần gần nhất. */}
                  {ct.locked_at !== undefined && ct.locked_at !== "" && (
                    <span className="dong-phu mt-1 tabular-nums">Khoá lúc {nhanMocKhoa(ct.locked_at)}</span>
                  )}
                  {ct.unlock_count > 0 && (
                    <span className="dong-phu">Đã mở khoá {ct.unlock_count} lần</span>
                  )}
                </td>
                <td>
                  {/* Sửa is the everyday action: an icon with its name as label + tooltip. The
                      four `budget.confirm` actions keep their WORDS (spec v2 §7); Gỡ is the red
                      outline and sits last. Every label names the voucher by its DATE. */}
                  <span className="flex flex-wrap items-center justify-end gap-1.5">
                    {coGhi && cho.sua && (
                      <IconButton
                        type="button"
                        label={`Sửa chứng từ ngày ${ngay}`}
                        disabled={dangGui}
                        onClick={() => moSua(ct.id)}
                      >
                        <Pencil aria-hidden="true" />
                      </IconButton>
                    )}
                    {coXacNhan && cho.xacNhan && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        icon={<Glyph icon={CircleCheck} />}
                        disabled={dangGui}
                        onClick={() => xacNhan(ct.id)}
                        aria-label={`Xác nhận chứng từ ngày ${ngay}`}
                      >
                        Xác nhận
                      </Button>
                    )}
                    {coXacNhan && cho.khoa && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        icon={<Glyph icon={LockKeyhole} />}
                        disabled={dangGui}
                        onClick={() => khoa(ct.id)}
                        aria-label={`Khoá chứng từ ngày ${ngay}`}
                        title={CANH_BAO_KHOA}
                      >
                        Khoá
                      </Button>
                    )}
                    {coXacNhan && cho.moKhoa && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        icon={<Glyph icon={LockKeyholeOpen} />}
                        disabled={dangGui}
                        onClick={() => moMoKhoa(ct.id)}
                        aria-label={`Mở khoá chứng từ ngày ${ngay}`}
                      >
                        Mở khoá
                      </Button>
                    )}
                    {coXacNhan && cho.go && (
                      <Button
                        type="button"
                        variant="danger"
                        size="sm"
                        icon={<Glyph icon={Trash2} />}
                        disabled={dangGui}
                        onClick={() => moGo(ct.id)}
                        aria-label={`Gỡ chứng từ ngày ${ngay}`}
                      >
                        Gỡ
                      </Button>
                    )}
                  </span>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </TableScroll>
  );
}

type DangMo =
  | { kieu: "them" }
  | { kieu: "sua"; ct: finance_chungTuRa }
  | { kieu: "go"; ct: finance_chungTuRa }
  | { kieu: "moKhoa"; ct: finance_chungTuRa }
  | null;

/**
 * Khối "Chứng từ" của trang chi tiết dự án — nối cả sáu tuyến ghi.
 *
 * MỖI LẦN GHI XONG GỌI `daGhiXong`, và trang cha ĐỌC LẠI dự án: `disbursed_amount`,
 * `remaining_amount`, `disbursed_ratio` và `delay_score` đều SUY RA từ chứng từ, nên một chứng từ
 * mới làm cả bốn con số ấy cũ đi ngay lập tức. Vá tại chỗ ở trình duyệt là dựng câu trả lời thứ
 * hai cho cùng một câu hỏi.
 */
export function KhoiChungTu({
  duAnID,
  coGhi,
  coXacNhan,
  daGhiXong,
}: {
  duAnID: string;
  coGhi: boolean;
  coXacNhan: boolean;
  daGhiXong: () => void;
}) {
  const [ds, datDS] = useState<readonly finance_chungTuRa[]>([]);
  const [dangMo, datDangMo] = useState<DangMo>(null);
  const [dangGui, datDangGui] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const [lanGhiXong, datLanGhiXong] = useState(0);

  /** Một hàng vừa đổi: thay tại chỗ, GIỮ NGUYÊN thứ tự — thứ tự là thứ tự cán bộ vừa ghi. */
  function thayHang(moi: finance_chungTuRa): void {
    datDS((truoc) => truoc.map((c) => (c.id === moi.id ? moi : c)));
  }

  /**
   * Kết thúc MỘT lần ghi, dùng chung cho cả sáu tuyến.
   *
   * MỘT HÀM CHO SÁU TUYẾN vì sáu bản sao của đoạn này sẽ trôi, và bản trôi là bản quên gọi
   * `daGhiXong` — tức bảng chứng từ đúng còn bốn con số suy ra của dự án ở ngay trên thì cũ, mà
   * không có gì nói ra rằng chúng cũ.
   */
  function ketThuc<T>(kq: KetQua<T>, apDung: (duLieu: T) => void): void {
    datDangGui(false);
    if (!kq.ok) {
      // NGUYÊN VĂN câu máy chủ, kể cả 409 của vòng đời và 409 "người vừa khoá không tự mở lại
      // được". Những câu ấy mang tên thao tác và mang đường ra — nuốt chúng thành "Có lỗi xảy ra"
      // là lấy mất đúng thứ cán bộ cần để biết phải làm gì.
      datLoi(kq.thongBao);
      return;
    }
    datLoi(null);
    datDangMo(null);
    datLanGhiXong((n) => n + 1);
    apDung(kq.duLieu);
    daGhiXong();
  }

  function them(gt: GiaTriFormChungTu, khoa: string): void {
    const than = thanThemChungTu(duAnID, gt);
    if (!than.ok) {
      datLoi(than.cau);
      return;
    }
    datDangGui(true);
    themChungTu(than.than, khoa).then((kq) => {
      ketThuc(kq, (ct) => datDS((truoc) => [...truoc, ct]));
    });
  }

  function sua(ct: finance_chungTuRa, gt: GiaTriFormChungTu): void {
    const than = thanSuaChungTu(giaTriTuChungTu(ct), gt);
    if (!than.ok) {
      datLoi(than.cau);
      return;
    }
    datDangGui(true);
    suaChungTu(ct.id, than.than).then((kq) => {
      ketThuc(kq, thayHang);
    });
  }

  function xacNhan(id: string): void {
    datDangGui(true);
    xacNhanChungTu(id).then((kq) => {
      ketThuc(kq, thayHang);
    });
  }

  function khoa(id: string): void {
    datDangGui(true);
    khoaChungTu(id).then((kq) => {
      ketThuc(kq, thayHang);
    });
  }

  function moKhoa(ct: finance_chungTuRa, lyDo: string): void {
    datDangGui(true);
    moKhoaChungTu(ct.id, lyDo).then((kq) => {
      ketThuc(kq, thayHang);
    });
  }

  function go(ct: finance_chungTuRa, lyDo: string): void {
    datDangGui(true);
    goChungTu(ct.id, lyDo).then((kq) => {
      // 204 KHÔNG THÂN: hàng biến khỏi bảng. Nó vẫn còn ở CSDL kèm người gỡ và lý do (xoá mềm) —
      // chỉ là không còn gì để làm với nó ở màn hình này.
      ketThuc(kq, () => datDS((truoc) => truoc.filter((c) => c.id !== ct.id)));
    });
  }

  function timHang(id: string): finance_chungTuRa | undefined {
    return ds.find((c) => c.id === id);
  }

  return (
    <Card as="section" aria-labelledby="tieu-de-chung-tu">
      <CardHeader className="justify-between">
        <div className="min-w-0">
          <CardTitle as="h3" id="tieu-de-chung-tu" className="inline-flex items-center gap-2">
            <Glyph icon={ReceiptText} className="size-[18px] shrink-0 text-brand-600" />
            Chứng từ giải ngân
          </CardTitle>
          {/* VÒNG ĐỜI NÓI RA NGAY TRÊN BẢNG, không để cán bộ suy từ việc nút nào hiện nút nào không. */}
          <p className="m-0 mt-1 text-[13px] text-ink-500">
            Vòng đời: Kế toán nhập → Đã xác nhận → Đã khoá. Nhập và sửa cần quyền budget.update; xác
            nhận, khoá, mở khoá và gỡ cần quyền budget.confirm.
          </p>
        </div>
        {coGhi && (
          <Button
            type="button"
            variant="primary"
            icon={<Glyph icon={Plus} />}
            aria-expanded={dangMo?.kieu === "them"}
            onClick={() => {
              datLoi(null);
              datDangMo(dangMo?.kieu === "them" ? null : { kieu: "them" });
            }}
          >
            Ghi nhận khoản chi
          </Button>
        )}
      </CardHeader>

      <div className="flex min-w-0 flex-col gap-4 p-4 empty:hidden">
      {!coGhi && <DeniedNote>{CAU_THIEU_QUYEN_GHI}</DeniedNote>}

      {!coXacNhan && <DeniedNote>{CAU_THIEU_QUYEN_XAC_NHAN}</DeniedNote>}

      {dangMo?.kieu === "them" && coGhi && (
        <FormChungTu
          key={`them-chung-tu|${lanGhiXong}`}
          tieuDeForm="Ghi nhận khoản chi"
          giaTriDau={FORM_CHUNG_TU_TRONG}
          dangGui={dangGui}
          loi={loi}
          huy={() => {
            datDangMo(null);
            datLoi(null);
          }}
          luu={them}
        />
      )}

      {dangMo?.kieu === "sua" && coGhi && (
        <FormChungTu
          key={`sua-chung-tu|${dangMo.ct.id}|${lanGhiXong}`}
          tieuDeForm="Sửa chứng từ"
          giaTriDau={giaTriTuChungTu(dangMo.ct)}
          trangThaiHienTai={dangMo.ct.status}
          dangGui={dangGui}
          loi={loi}
          huy={() => {
            datDangMo(null);
            datLoi(null);
          }}
          luu={(gt) => {
            sua(dangMo.ct, gt);
          }}
        />
      )}

      {dangMo?.kieu === "go" && coXacNhan && (
        <FormLyDo
          tieuDe={`Gỡ chứng từ ngày ${nhanNgay(dangMo.ct.payment_date)}?`}
          busyText="Đang gỡ…"
          moTa={
            "Chứng từ được gỡ MỀM: hàng vẫn còn kèm người gỡ và lý do, nhưng nó thôi cộng vào số " +
            "đã giải ngân của dự án — tức là một con số đã báo cáo vừa thay đổi."
          }
          nhanNut="Gỡ chứng từ"
          dangGui={dangGui}
          loi={loi}
          huy={() => {
            datDangMo(null);
            datLoi(null);
          }}
          xacNhan={(lyDo) => {
            go(dangMo.ct, lyDo);
          }}
        />
      )}

      {dangMo?.kieu === "moKhoa" && coXacNhan && (
        <FormLyDo
          tieuDe={`Mở khoá chứng từ ngày ${nhanNgay(dangMo.ct.payment_date)}?`}
          tone="default"
          busyText="Đang mở khoá…"
          moTa={
            "Mở khoá đưa chứng từ về Đã xác nhận để sửa được. Lý do là bắt buộc, và NGƯỜI VỪA KHOÁ " +
            "không tự mở lại được — cần một cán bộ khác có quyền budget.confirm."
          }
          nhanNut="Mở khoá"
          dangGui={dangGui}
          loi={loi}
          huy={() => {
            datDangMo(null);
            datLoi(null);
          }}
          xacNhan={(lyDo) => {
            moKhoa(dangMo.ct, lyDo);
          }}
        />
      )}

      {/* LỖI CỦA MỘT THAO TÁC KHÔNG MỞ BIỂU MẪU NÀO (xác nhận, khoá) vẫn phải ra màn hình. */}
      {loi !== null && dangMo === null && (
        <p className="thong-bao-loi m-0" role="alert">
          {loi}
        </p>
      )}
      </div>

      <BangChungTu
        ds={ds}
        coGhi={coGhi}
        coXacNhan={coXacNhan}
        dangGui={dangGui}
        moSua={(id) => {
          const ct = timHang(id);
          if (ct === undefined) return;
          datLoi(null);
          datDangMo({ kieu: "sua", ct });
        }}
        moGo={(id) => {
          const ct = timHang(id);
          if (ct === undefined) return;
          datLoi(null);
          datDangMo({ kieu: "go", ct });
        }}
        moMoKhoa={(id) => {
          const ct = timHang(id);
          if (ct === undefined) return;
          datLoi(null);
          datDangMo({ kieu: "moKhoa", ct });
        }}
        xacNhan={xacNhan}
        khoa={khoa}
      />
    </Card>
  );
}
