"use client";

import { LockKeyhole, PencilLine, Plus, Trash2 } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS } from "@/components/ui/data-table";
import { controlClass } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import { Skeleton } from "@/components/ui/skeleton";
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
  DO_DAI_TOI_DA_DOT,
  DOT_TRONG,
  dungThanDot,
  ENTRY_RECORDED,
  ENTRY_REMOVED,
  khoaSauLanGhi,
  lyDoKhongTinh,
  MO_TA_HOP_DOT,
  nhanNgayLuyKe,
  nhanSoTien,
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

  /**
   * After a write: the server's refusal stays IN the dialog, verbatim; a success is the prototype's
   * toast (`FiscalEntriesDialog.tsx:88,251`, sonner as every other screen — ADR 0068 lần 6), and the
   * list and the sheet are read again.
   */
  function sauLanGhi(kq: KetQua<unknown>, cauXong: string): boolean {
    datDangGui(false);
    if (!kq.ok) {
      datLoi(kq.thongBao);
      return false;
    }
    datLoi(null);
    toast.success(cauXong);
    datLanTai((n) => n + 1);
    daDoiSoLieu();
    return true;
  }

  // A YEAR close refuses every entry on this sheet, so the form is replaced by the reason instead
  // of offering a submit that can only answer 409. A MONTH close only names its months: entries
  // dated elsewhere are still accepted, and an adjustment entry is how a closed month is corrected.
  const sheetLock = sheetLockReason(closes, sheetYear);
  const monthsHint = closedMonthsHint(closes, sheetYear);

  // The prototype's `FiscalEntriesDialog`: a centred modal 52rem wide, title = the line's name as it is,
  // the description under it, the entry form on top and the list under it. Esc and the dialog's own ✕
  // (ModalDialog draws it, ADR 0068 lần 6) both close it; nothing is lost — every entry is saved by its
  // own submit.
  return (
    <ModalDialog titleId="tieu-de-hop-dot" size="lg" className="max-w-[52rem]" onDismiss={dong}>
      <div className="min-w-0 shrink-0">
        <ModalDialogHeader titleId="tieu-de-hop-dot" title={tenKhoanMuc} description={MO_TA_HOP_DOT} />
      </div>

      <div className="flex min-h-0 min-w-0 flex-col gap-4 overflow-y-auto">
      <NoiDungHopDot
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
        }}
      >
        {loi !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {loi}
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
                if (sauLanGhi(kq, ENTRY_REMOVED)) datDangGo(null);
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
            dangGui={dangGui}
            gui={(nhap, bieuMau) => {
              const dung = dungThanDot(nhap, cot, donVi.ma);
              if (!dung.ok) {
                datLoi(dung.thongBao);
                return;
              }
              datDangGui(true);
              ghiDot(khoanMucId, dung.than, khoa).then((kq) => {
                const ok = sauLanGhi(kq, ENTRY_RECORDED);
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
 * Phần hiển thị của hộp: các thông báo, biểu mẫu, danh sách đợt. Tách khỏi phần gọi mạng để bài
 * kiểm vẽ được từng trạng thái — đặc biệt nhánh KHÔNG có `budget.confirm`.
 *
 * `children` là biểu mẫu và các thông báo, đặt trên danh sách. The description sits in the dialog's
 * header, as the prototype's `DialogDescription`; the list has no heading of its own (prototype).
 */
export function NoiDungHopDot({
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
  return (
    <>
      {children}

      {/* The prototype's list box (`FiscalEntriesDialog.tsx:189`): capped at 24rem, scrolling inside. */}
      <div role="region" aria-label="Các đợt đã ghi" tabIndex={0} className="max-h-[24rem] min-w-0 shrink-0 overflow-auto">
      {danhSach.pha === "dangTai" && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải các đợt…
          </p>
          <Skeleton className="h-32 w-full" />
        </>
      )}
      {danhSach.pha === "loi" && (
        <p className="thong-bao-loi m-0" role="alert">
          {danhSach.thongBao}
        </p>
      )}
      {danhSach.pha === "xong" && danhSach.duLieu.entries.length === 0 && (
        <p className="m-0 py-8 text-center text-[12.5px] text-ink-muted">{DOT_TRONG}</p>
      )}
      {danhSach.pha === "xong" && danhSach.duLieu.entries.length > 0 && (
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
                      <span className="block text-[11px] text-ink-muted">
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
                        // The prototype's bare red bin (`FiscalEntriesDialog.tsx:243-258`). It still
                        // opens the reason form: the server requires a reason (rule 7).
                        <button
                          type="button"
                          className="inline-flex cursor-pointer items-center border-0 bg-transparent p-0 text-danger focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500 disabled:cursor-not-allowed disabled:opacity-50"
                          disabled={dangGui}
                          aria-label={`Gỡ đợt ngày ${nhanNgayLuyKe(d.date)}`}
                          title="Gỡ đợt"
                          onClick={() => moGo(d)}
                        >
                          <Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-3.5" />
                        </button>
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
      )}
      </div>
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

/** The prototype's `Label className="text-[11.5px]"` (shadcn Label: medium weight, tight line). */
const ENTRY_LABEL = "block text-[11.5px] leading-none font-medium text-ink";

/** The prototype's `Input className="mt-1 h-9 text-[12.5px]"` — the shared frame, at the dialog's size. */
const ENTRY_INPUT = cn(controlClass, "mt-1 h-9 text-[12.5px] md:text-[12.5px]");

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
  dangGui,
  gui,
}: {
  /** CHỈ cột số. Their boxes read the sheet's unit — said once in the summary card's sub-line. */
  cot: readonly finance_cotRa[];
  dangGui: boolean;
  gui: (nhap: NhapDot, bieuMau: HTMLFormElement) => void;
}) {
  // Đọc đồng hồ MỘT lần lúc biểu mẫu dựng, không mỗi lần vẽ lại.
  const [ngayMacDinh] = useState(homNay);

  return (
    // The prototype's entry form (`FiscalEntriesDialog.tsx:106-186`): ONE tinted box, a 4-column grid
    // from 640px — Ngày | Nội dung (3) · Đơn vị, cá nhân (2) | Số chứng từ (2) · one box per money
    // column · `Ghi đợt` in the last cell. Plain label + input pairs, 11.5px labels and 36px boxes.
    <form
      aria-label="Ghi một đợt"
      className="grid min-w-0 gap-2 rounded-[10px] border border-solid border-line bg-canvas p-3 sm:grid-cols-4"
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
        <div>
          <label htmlFor="dot-date" className={ENTRY_LABEL}>
            Ngày
          </label>
          <input id="dot-date" name="date" className={ENTRY_INPUT} type="date" required defaultValue={ngayMacDinh} />
        </div>
        {/* No `required` on Nội dung: the browser's own bubble would replace the prototype's sentence
            ("Nhập nội dung đợt thu, chi.", `dungThanDot`). */}
        <div className="sm:col-span-3">
          <label htmlFor="dot-content" className={ENTRY_LABEL}>
            Nội dung
          </label>
          <input
            id="dot-content"
            name="content"
            className={ENTRY_INPUT}
            type="text"
            maxLength={DO_DAI_TOI_DA_DOT.content}
            placeholder="Thu tiền sử dụng đất đợt 2"
          />
        </div>
        <div className="sm:col-span-2">
          <label htmlFor="dot-counterparty" className={ENTRY_LABEL}>
            Đơn vị, cá nhân
          </label>
          <input
            id="dot-counterparty"
            name="counterparty"
            className={ENTRY_INPUT}
            type="text"
            maxLength={DO_DAI_TOI_DA_DOT.counterparty}
            autoComplete="off"
          />
        </div>
        <div className="sm:col-span-2">
          <label htmlFor="dot-document-no" className={ENTRY_LABEL}>
            Số chứng từ
          </label>
          <input
            id="dot-document-no"
            name="document_no"
            className={ENTRY_INPUT}
            type="text"
            maxLength={DO_DAI_TOI_DA_DOT.document_no}
          />
        </div>
        {/* KEPT, not in the prototype: an adjustment entry corrects a closed period (rule 7 — a
            closed period is never edited). SAME FIELD ORDER AS BEFORE (keyboard order is behaviour):
            the reason, then the money. */}
        <div className="sm:col-span-4">
          <label htmlFor="dot-adjustment-reason" className={ENTRY_LABEL}>
            Lý do điều chỉnh (chỉ điền khi đây là đợt điều chỉnh)
          </label>
          <textarea
            id="dot-adjustment-reason"
            name="adjustment_reason"
            className={cn(controlClass, "mt-1 h-auto py-2 text-[12.5px] md:text-[12.5px]")}
            rows={2}
            maxLength={DO_DAI_TOI_DA_DOT.adjustment_reason}
            aria-describedby="dot-adjustment-reason-hint"
          />
          <p id="dot-adjustment-reason-hint" className="m-0 mt-1 text-[11px] text-ink-muted">
            Đợt điều chỉnh sửa sai sót của một kỳ đã chốt và được ghi ở kỳ còn mở. Để trống nếu là đợt
            thu chi thông thường. Tối đa {DO_DAI_TOI_DA_DOT.adjustment_reason} ký tự.
          </p>
        </div>
        {cot.map((c) => (
          // The column's own label, as the prototype (`:155-157`).
          <div key={c.id}>
            <label htmlFor={`dot-gia-${c.id}`} className={ENTRY_LABEL}>
              {c.name}
            </label>
            <input
              id={`dot-gia-${c.id}`}
              name={`dot-gia:${c.id}`}
              className={cn(ENTRY_INPUT, "text-right tabular-nums")}
              type="text"
              inputMode="decimal"
              autoComplete="off"
            />
          </div>
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
