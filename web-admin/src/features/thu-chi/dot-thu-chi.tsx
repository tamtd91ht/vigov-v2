"use client";

import { CircleCheck, ListOrdered, LockKeyhole, PencilLine, Plus, Trash2 } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import type { KetQua } from "@/lib/api/goi";
import { cn } from "@/lib/cn";
import type {
  finance_budgetPeriodCloseOut,
  finance_cotRa,
  finance_danhSachDotRa,
  finance_dotRa,
} from "@/lib/api/schema.gen";
import { ghiDot, goDot, layDot } from "@/lib/api/thu-chi";

import { FormGoKemLyDo } from "./form-go-ly-do";
import {
  cauDieuKienDot,
  DO_DAI_TOI_DA_DOT,
  DOT_TRONG,
  dungThanDot,
  khoaSauLanGhi,
  lyDoKhongTinh,
  MO_TA_HOP_DOT,
  nhanNgayLuyKe,
  nhanSoTien,
  tieuDeHopDot,
  type DonViHien,
  type NhapDot,
} from "./nhan-thu-chi";
import { DanhSachKhongTinh, OTien } from "./o-tien";
import { closedMonthsHint, entryLockReason, sheetLockReason } from "./period-close";

/**
 * Hộp "Các đợt thu, chi" (§5) của MỘT khoản mục lá.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BA KHOÁ QUYỀN, ĐÚNG NHƯ MÁY CHỦ CHIA (`routes.go:1193-1244`): đọc danh sách là `budget.read`
 * (ai mở được màn này cũng xem được), `+ Ghi đợt` là `budget.update`, GỠ một đợt là
 * `budget.confirm` — gỡ một đợt đổi con số của một dòng `entries`, con số có thể đã báo lên trên.
 * Ẩn nút là trải nghiệm; máy chủ vẫn kiểm từng lời gọi (luật 5 cấm #1).
 *
 * `counterparty` ("Đơn vị, cá nhân") VỀ ĐÃ CHE (`privacy.MaskName`) cho MỌI người gọi — không có
 * khoá xem đầy đủ nào (luật 3, câu hỏi mở #27). Màn hình in NGUYÊN chuỗi đã che, không thử khôi
 * phục, và không đưa nó vào URL, bộ nhớ trình duyệt hay console.
 *
 * SAU MỖI LẦN GHI HAY GỠ, ĐỌC LẠI CẢ HAI: danh sách đợt, và cả BẢNG — tổng của dòng `entries`, tổng
 * của cha nó, và hai chỉ số của năm đều đổi theo, và chúng do máy chủ suy ra.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function HopDotThuChi({
  khoanMucId,
  tenKhoanMuc,
  method,
  cot,
  donVi,
  coGhi,
  coXacNhan,
  closes,
  sheetYear,
  dong,
  daDoiSoLieu,
}: {
  khoanMucId: string;
  tenKhoanMuc: string;
  method: string;
  /** CHỈ cột số của bảng. */
  cot: readonly finance_cotRa[];
  donVi: DonViHien;
  coGhi: boolean;
  coXacNhan: boolean;
  /** ACTIVE and reopened closes of the sheet year — display only, the server decides (409). */
  closes: readonly finance_budgetPeriodCloseOut[];
  sheetYear: number;
  dong: () => void;
  /** Một đợt vừa được ghi hoặc gỡ: bảng và chỉ số phải đọc lại. */
  daDoiSoLieu: () => void;
}) {
  const [lanTai, datLanTai] = useState(0);
  const [daTai, datDaTai] = useState<{
    lan: number;
    kq: KetQua<finance_danhSachDotRa>;
  } | null>(null);

  /**
   * Khoá chống trùng của LẦN GỬI HIỆN TẠI. Sinh lúc hộp mở; giữ nguyên khi gửi lại sau lỗi; thay
   * mới sau một lần ghi thành công — xem `khoaSauLanGhi`.
   */
  const [khoa, datKhoa] = useState(khoaChongTrungMoi);
  const [dangGui, datDangGui] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const [thongBao, datThongBao] = useState<string | null>(null);
  const [dangGo, datDangGo] = useState<finance_dotRa | null>(null);

  useEffect(() => {
    let bo = false;
    layDot(khoanMucId).then((kq) => {
      if (!bo) datDaTai({ lan: lanTai, kq });
    });
    return () => {
      bo = true;
    };
  }, [khoanMucId, lanTai]);

  const danhSach: TrangThaiDot =
    daTai === null || daTai.lan !== lanTai
      ? { pha: "dangTai" }
      : daTai.kq.ok
        ? { pha: "xong", duLieu: daTai.kq.duLieu }
        : { pha: "loi", thongBao: daTai.kq.thongBao };

  function sauLanGhi(kq: KetQua<unknown>, cauXong: string): boolean {
    datDangGui(false);
    if (!kq.ok) {
      datLoi(kq.thongBao);
      datThongBao(null);
      return false;
    }
    datLoi(null);
    datThongBao(cauXong);
    datLanTai((n) => n + 1);
    daDoiSoLieu();
    return true;
  }

  // A YEAR close refuses every entry on this sheet, so the form is replaced by the reason instead
  // of offering a submit that can only answer 409. A MONTH close only names its months: entries
  // dated elsewhere are still accepted, and an adjustment entry is how a closed month is corrected.
  const sheetLock = sheetLockReason(closes, sheetYear);
  const monthsHint = closedMonthsHint(closes, sheetYear);

  // The prototype's `FiscalEntriesDialog`: a centred modal 52rem wide, title = the line, the entry form
  // on top and the list under it. Esc and the dialog's own ✕ (ModalDialog draws it, ADR 0068 lần 6)
  // both close it; nothing is lost — every entry is saved by its own submit.
  return (
    <ModalDialog titleId="tieu-de-hop-dot" size="lg" className="max-w-[52rem]" onDismiss={dong}>
      <div className="min-w-0 shrink-0">
        <ModalDialogHeader titleId="tieu-de-hop-dot" title={tieuDeHopDot(tenKhoanMuc)} />
      </div>

      <div className="flex min-h-0 min-w-0 flex-col gap-4 overflow-y-auto">
      <NoiDungHopDot
        method={method}
        cot={cot}
        donVi={donVi}
        danhSach={danhSach}
        coXacNhan={coXacNhan}
        closes={closes}
        sheetYear={sheetYear}
        dangGui={dangGui}
        moGo={(d) => {
          datDangGo(d);
          datLoi(null);
          datThongBao(null);
        }}
      >
        {loi !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {loi}
          </p>
        )}
        {thongBao !== null && (
          <p role="status" className="m-0 inline-flex items-center gap-1.5 text-sm text-success-600">
            <CircleCheck aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0" />
            {thongBao}
          </p>
        )}

        {dangGo !== null && (
          <FormGoKemLyDo
            idTruong="go-dot-reason"
            tieuDe={`Gỡ đợt ngày ${nhanNgayLuyKe(dangGo.date)}?`}
            submitLabel="Gỡ đợt"
            canhBao={
              "Gỡ một đợt đổi ngay con số của khoản mục nếu khoản mục đang Cộng theo đợt, và cả hai " +
              "chỉ số của năm. Đợt vẫn được giữ kèm người gỡ và lý do."
            }
            dangGui={dangGui}
            huy={() => datDangGo(null)}
            luu={(lyDo) => {
              datDangGui(true);
              goDot(dangGo.id, lyDo).then((kq) => {
                if (sauLanGhi(kq, "Đã gỡ đợt.")) datDangGo(null);
              });
            }}
          />
        )}

        {coGhi && sheetLock !== null && (
          <Notice tone="legal" icon={LockKeyhole}>
            {sheetLock}
          </Notice>
        )}
        {coGhi && sheetLock === null && monthsHint !== null && (
          <p className="m-0 inline-flex items-center gap-1.5 text-xs text-ink-500">
            <LockKeyhole aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3.5 shrink-0" />
            {monthsHint}
          </p>
        )}

        {coGhi && sheetLock === null && (
          <FormGhiDot
            cot={cot}
            donVi={donVi}
            dangGui={dangGui}
            gui={(nhap, bieuMau) => {
              const dung = dungThanDot(nhap, cot, donVi.ma);
              if (!dung.ok) {
                datLoi(dung.thongBao);
                datThongBao(null);
                return;
              }
              datDangGui(true);
              ghiDot(khoanMucId, dung.than, khoa).then((kq) => {
                const ok = sauLanGhi(kq, "Đã ghi đợt.");
                datKhoa((k) => khoaSauLanGhi(k, ok, khoaChongTrungMoi));
                if (ok) bieuMau.reset();
              });
            }}
          />
        )}
      </NoiDungHopDot>
      </div>
    </ModalDialog>
  );
}

export type TrangThaiDot =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duLieu: finance_danhSachDotRa };

/**
 * Phần hiển thị của hộp: mô tả, điều kiện cộng, danh sách đợt. Tách khỏi phần gọi mạng để bài
 * kiểm vẽ được từng trạng thái — đặc biệt nhánh KHÔNG có `budget.confirm`.
 *
 * `children` là biểu mẫu và các thông báo, đặt giữa phần mô tả và danh sách.
 */
export function NoiDungHopDot({
  method,
  cot,
  donVi,
  danhSach,
  coXacNhan,
  closes,
  sheetYear,
  dangGui,
  moGo,
  children,
}: {
  method: string;
  cot: readonly finance_cotRa[];
  donVi: DonViHien;
  danhSach: TrangThaiDot;
  coXacNhan: boolean;
  closes: readonly finance_budgetPeriodCloseOut[];
  sheetYear: number;
  dangGui: boolean;
  moGo: (d: finance_dotRa) => void;
  children?: ReactNode;
}) {
  // Cách tính theo DANH SÁCH vừa đọc nếu có — mới hơn bản của bảng lúc mở hộp.
  const methodHien = danhSach.pha === "xong" ? danhSach.duLieu.method : method;

  return (
    <>
      <div className="flex min-w-0 flex-col gap-1">
        <p className="m-0 text-sm text-ink-700">{MO_TA_HOP_DOT}</p>
        <p className="m-0 text-xs text-ink-500">{cauDieuKienDot(methodHien)}</p>
        <p className="m-0 text-xs text-ink-500">Số tiền theo đơn vị của bảng: {donVi.nhan}.</p>
      </div>

      {children}

      <h4 className="m-0 text-sm font-semibold text-ink-900">Các đợt đã ghi</h4>
      {danhSach.pha === "dangTai" && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải các đợt…
          </p>
          <SkeletonRows rows={3} className="-mx-4" />
        </>
      )}
      {danhSach.pha === "loi" && (
        <p className="thong-bao-loi m-0" role="alert">
          {danhSach.thongBao}
        </p>
      )}
      {danhSach.pha === "xong" && danhSach.duLieu.entries.length === 0 && (
        <EmptyState icon={ListOrdered} title={DOT_TRONG} className="py-6" />
      )}
      {danhSach.pha === "xong" && danhSach.duLieu.entries.length > 0 && (
        <TableScroll aria-label="Các đợt đã ghi" className="rounded-xl">
          <table className={cn("bang-danh-muc", DATA_TABLE_CLASS)}>
            <thead>
              <tr>
                {/* The prototype's four columns: Ngày · Nội dung (with "Đơn vị, cá nhân · Số chứng
                    từ" under it) · one per money column · the remove action. */}
                <th scope="col">Ngày</th>
                <th scope="col">Nội dung</th>
                {cot.map((c) => (
                  <th key={c.id} scope="col" className="text-right">
                    {c.name}
                  </th>
                ))}
                {coXacNhan && (
                  <th scope="col">
                    <span className="an-thi-giac">Thao tác</span>
                  </th>
                )}
              </tr>
            </thead>
            <tbody>
              {danhSach.duLieu.entries.map((d) => {
                const lock = entryLockReason(closes, d.date, sheetYear);
                const adjustment = d.adjustment_reason ?? "";
                // ĐÃ CHE Ở MÁY CHỦ (`privacy.MaskName`) — in nguyên, không khôi phục (luật 3).
                const counterparty = d.counterparty ?? "";
                const documentNo = d.document_no ?? "";
                return (
                <tr key={d.id}>
                  <td className="align-top tabular-nums">{nhanNgayLuyKe(d.date)}</td>
                  <td className="min-w-[14rem] align-top whitespace-normal">
                    {adjustment !== "" && (
                      <>
                        <Badge tone="warning" icon={PencilLine}>
                          Điều chỉnh
                        </Badge>{" "}
                      </>
                    )}
                    {d.content}
                    {adjustment !== "" && (
                      <>
                        <br />
                        <span className="ghi-chu">Lý do điều chỉnh: {adjustment}</span>
                      </>
                    )}
                    {(counterparty !== "" || documentNo !== "") && (
                      <span className="block text-xs text-ink-500">
                        {[counterparty, documentNo].filter((s) => s !== "").join(" · ")}
                      </span>
                    )}
                  </td>
                  {cot.map((c) => (
                    <td key={c.id} className="text-right align-top tabular-nums">
                      <OTien
                        chu={nhanSoTien(d.values[c.id] ?? null, donVi.ma)}
                        lyDo={lyDoKhongTinh(d.unavailable_reasons?.[c.id])}
                      />
                    </td>
                  ))}
                  {coXacNhan && (
                    <td className="text-right align-top">
                      {lock === null ? (
                        // A removal keeps its WORDS (spec v2 §7) and the red outline.
                        <Button
                          type="button"
                          variant="danger"
                          size="sm"
                          icon={<Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                          disabled={dangGui}
                          aria-label={`Gỡ đợt ngày ${nhanNgayLuyKe(d.date)}`}
                          onClick={() => moGo(d)}
                        >
                          Gỡ đợt
                        </Button>
                      ) : (
                        // No remove button on a closed period: it could only answer 409. The
                        // reason is printed, not hidden in a tooltip — touch screens cannot hover.
                        <span className="ghi-chu inline-flex items-start gap-1 text-left whitespace-normal">
                          <LockKeyhole aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-0.5 size-3.5 shrink-0" />
                          {lock}
                        </span>
                      )}
                    </td>
                  )}
                </tr>
                );
              })}
            </tbody>
          </table>
        </TableScroll>
      )}
      {/* Số tiền của một đợt ghi TRƯỚC khi trần hạ xuống 2^53 − 1 có thể vượt trần: máy chủ gửi
          `null` kèm câu, và đợt ấy phải TÌM ĐƯỢC để gỡ — nên nói ra ngày và cột của nó. */}
      {danhSach.pha === "xong" && (
        <DanhSachKhongTinh
          tieuDe="Số tiền không đọc chính xác được"
          o={danhSach.duLieu.entries.flatMap((d) =>
            cot.flatMap((c) => {
              const lyDo = lyDoKhongTinh(d.unavailable_reasons?.[c.id]);
              return lyDo === null
                ? []
                : [{ khoa: `${d.id}|${c.id}`, noi: `Đợt ngày ${nhanNgayLuyKe(d.date)} — ${c.name}`, lyDo }];
            }),
          )}
        />
      )}
    </>
  );
}

/** Ngày hôm nay `YYYY-MM-DD` theo đồng hồ máy — giá trị điền sẵn của ô Ngày (§5). */
function homNay(): string {
  const d = new Date();
  const hai = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${hai(d.getMonth() + 1)}-${hai(d.getDate())}`;
}

/**
 * Biểu mẫu `+ Ghi đợt`: bốn trường của §5 và một ô cho MỖI CỘT SỐ của bảng.
 *
 * Ô tiền là ô CHỮ (`inputMode="decimal"`), không phải `type="number"`: ô số của trình duyệt đọc
 * theo ngôn ngữ của máy, nên `3.463.459,2` bị nó coi là rỗng — và một ô rỗng là "không có số" ở
 * đợt này. Chữ gõ vào được đọc bằng `docSoNhap` theo đơn vị của bảng, giống hệt ô sửa khoản mục.
 */
export function FormGhiDot({
  cot,
  donVi,
  dangGui,
  gui,
}: {
  cot: readonly finance_cotRa[];
  donVi: DonViHien;
  dangGui: boolean;
  gui: (nhap: NhapDot, bieuMau: HTMLFormElement) => void;
}) {
  // Đọc đồng hồ MỘT lần lúc biểu mẫu dựng, không mỗi lần vẽ lại.
  const [ngayMacDinh] = useState(homNay);

  return (
    // The prototype's entry form: ONE tinted box, a 4-column grid from 640px — Ngày | Nội dung (3) ·
    // Đơn vị, cá nhân (2) | Số chứng từ (2) · one box per money column · `Ghi đợt` in the last cell.
    <form
      aria-label="Ghi một đợt"
      className="grid min-w-0 gap-3 rounded-xl border border-line bg-surface-muted p-3 sm:grid-cols-4"
      onSubmit={(e) => {
        e.preventDefault();
        const bieuMau = e.currentTarget;
        const fd = new FormData(bieuMau);
        const gia: Record<string, string> = {};
        for (const c of cot) gia[c.id] = String(fd.get(`dot-gia:${c.id}`) ?? "");
        gui(
          {
            ngay: String(fd.get("date") ?? ""),
            noiDung: String(fd.get("content") ?? ""),
            doiTac: String(fd.get("counterparty") ?? ""),
            soChungTu: String(fd.get("document_no") ?? ""),
            adjustmentReason: String(fd.get("adjustment_reason") ?? ""),
            gia,
          },
          bieuMau,
        );
      }}
    >
        <Field label="Ngày" htmlFor="dot-date" grow="auto">
          <input
            id="dot-date"
            name="date"
            className="o-nhap"
            type="date"
            required
            defaultValue={ngayMacDinh}
          />
        </Field>
        <Field label="Nội dung" htmlFor="dot-content" grow="auto" className="sm:col-span-3">
          <input
            id="dot-content"
            name="content"
            className="o-nhap"
            type="text"
            required
            maxLength={DO_DAI_TOI_DA_DOT.content}
            placeholder="Thu tiền sử dụng đất đợt 2"
          />
        </Field>
        <Field label="Đơn vị, cá nhân" htmlFor="dot-counterparty" grow="auto" className="sm:col-span-2">
          <input
            id="dot-counterparty"
            name="counterparty"
            className="o-nhap"
            type="text"
            maxLength={DO_DAI_TOI_DA_DOT.counterparty}
            autoComplete="off"
          />
        </Field>
        <Field label="Số chứng từ" htmlFor="dot-document-no" grow="auto" className="sm:col-span-2">
          <input
            id="dot-document-no"
            name="document_no"
            className="o-nhap"
            type="text"
            maxLength={DO_DAI_TOI_DA_DOT.document_no}
          />
        </Field>
        {/* SAME FIELD ORDER AS BEFORE (keyboard order is behaviour): the reason, then the money. */}
        <Field
          label="Lý do điều chỉnh (chỉ điền khi đây là đợt điều chỉnh)"
          htmlFor="dot-adjustment-reason"
          grow="auto"
          className="sm:col-span-4"
          hint={
            <span id="dot-adjustment-reason-hint">
              Đợt điều chỉnh sửa sai sót của một kỳ đã chốt và được ghi ở kỳ còn mở. Để trống nếu là
              đợt thu chi thông thường. Tối đa {DO_DAI_TOI_DA_DOT.adjustment_reason} ký tự.
            </span>
          }
        >
          <textarea
            id="dot-adjustment-reason"
            name="adjustment_reason"
            className="o-nhap py-2"
            rows={2}
            maxLength={DO_DAI_TOI_DA_DOT.adjustment_reason}
            aria-describedby="dot-adjustment-reason-hint"
          />
        </Field>
        {cot.map((c) => (
          <Field key={c.id} label={`${c.name} (${donVi.nhan.toLowerCase()})`} htmlFor={`dot-gia-${c.id}`} grow="auto">
            <input
              id={`dot-gia-${c.id}`}
              name={`dot-gia:${c.id}`}
              className="o-nhap text-right tabular-nums"
              type="text"
              inputMode="decimal"
              autoComplete="off"
            />
          </Field>
        ))}
      <div className="flex min-w-0 items-end">
        <Button
          type="submit"
          variant="primary"
          className="w-full"
          icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          disabled={dangGui}
          aria-busy={dangGui || undefined}
        >
          <BusyLabel busy={dangGui} label="Ghi đợt" busyText={BUSY_SAVING} />
        </Button>
      </div>
    </form>
  );
}
