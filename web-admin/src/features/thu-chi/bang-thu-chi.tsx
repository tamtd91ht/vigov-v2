"use client";

import {
  Calculator,
  ChevronDown,
  ChevronRight,
  Database,
  FilePlus2,
  FileSpreadsheet,
  Gauge,
  Info,
  Layers,
  ListPlus,
  LockKeyhole,
  Pencil,
  Plus,
  Star,
  Trash2,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { useEffect, useId, useRef, useState, type FormEvent, type ReactNode } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass, Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import { Skeleton, SkeletonRows } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { listBudgetPeriodCloses } from "@/lib/api/budget-period-close";
import type { KetQua } from "@/lib/api/goi";
import type {
  finance_bangDayDuRa,
  finance_budgetPeriodClosesOut,
  finance_bangRa,
  finance_chiSoNamRa,
  finance_chiSoRa,
  finance_cotRa,
  finance_dongRa,
  finance_tomTatRa,
} from "@/lib/api/schema.gen";
import {
  datDongTong,
  doiCachTinh,
  goBang,
  goKhoanMuc,
  layBang,
  layChiSoNganSach,
  suaBang,
  suaKhoanMuc,
  themKhoanMuc,
  type CachTinhChon,
  type LoaiBang,
  type SuaDongVao,
} from "@/lib/api/thu-chi";
import { cn } from "@/lib/cn";
import { compactDong } from "@/lib/compact-dong";
import { danhSachNam, namTheoDongHoMay } from "@/lib/nam";
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_XAC_NHAN_NGAN_SACH } from "@/lib/quyen";

import { HopDotThuChi } from "./dot-thu-chi";
import { FormGoKemLyDo } from "./form-go-ly-do";
import { BudgetSheetHeaderActions } from "./header-actions";
import { LapBang } from "./lap-bang";
import {
  CACH_TINH_CHON,
  CANH_BAO_GO_BANG,
  CANH_BAO_GO_KHOAN_MUC,
  canhBaoDoiCachTinh,
  cauQuyDoi,
  cellNameEdit,
  cellValueDraft,
  cellValueEdit,
  cotSo,
  donViCuaBang,
  dongSangChuoi,
  dungCay,
  editKeyIntent,
  formatSheetAmount,
  GHI_CHU_CHENH_LECH,
  isUnresolvedPercentColumn,
  lineOrderBody,
  lyDoKhongTinh,
  moiDongCoCon,
  NHAN_CHENH_LECH,
  nhanBoDem,
  nhanCachTinh,
  nhanChiSo,
  nhanDatDongTong,
  nhanGoKhoanMuc,
  nhanNutDot,
  nhanSoTien,
  nhanSoTienChiSo,
  nhanTab,
  nhanSuaO,
  nhanThemCon,
  NHAN_SUA_TEN,
  NO_REPORT_YET,
  O_KHONG_TINH_DUOC,
  O_TRONG,
  PARENT_SUMS_CHILDREN,
  percentCellRounded,
  phangCay,
  subtitleParts,
  suaDuocOSo,
  type CellEditOutcome,
  type DongHien,
  type DonViHien,
} from "./nhan-thu-chi";
import { DanhSachKhongTinh, OTien } from "./o-tien";
import { sheetLockReason } from "./period-close";
import { BudgetPeriodClosePanel, type ClosesView } from "./period-close-panel";
import { FormSuaBang } from "./sua-bang";

// Giữ đường nhập cũ cho phía gọi và bài kiểm: hộp gỡ nay nằm ở tệp riêng vì hộp `⇄` cũng dùng nó.
export { FormGoKemLyDo };

/** Decorative icon inside a button or a line of text: hidden from assistive tech, never focusable. */
function Glyph({ icon: Icon, className }: { icon: LucideIcon; className?: string }) {
  return <Icon aria-hidden="true" focusable="false" strokeWidth={1.8} className={className} />;
}

/**
 * Màn "Thu - Chi ngân sách" (`docs/ui-ux/07-thu-chi-ngan-sach.md`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BỐ CỤC THEO PROTOTYPE (`vigov-require/apps/admin/src/components/budget/FiscalReportPanel.tsx`,
 * ADR 0068 lần 5), từ trên xuống: thanh chọn bảng (năm · `Chi ngân sách 2026` · `Thu ngân sách 2026`
 * bên trái; `Nạp từ Excel` · `Lập bảng` · `Sửa thông tin bảng` · `Gỡ` bên phải) → thẻ báo cáo (tiêu
 * đề, dòng phụ, câu dòng tổng, một ô cho mỗi cột) → thanh công cụ của cây → bảng cây. Tên khoản mục
 * và ô số SỬA NGAY TRONG Ô như prototype (quyết định 09/10/2026); thêm, sửa bảng, gỡ, đổi cách tính,
 * TT / thứ tự hiển thị, các đợt là HỘP THOẠI.
 *
 * KHÁC PROTOTYPE CÓ LÝ DO: `Nạp từ Excel` nạp ngay như prototype (`budget-import-button.tsx`, ADR 0081
 * #6), và biểu mẫu `Lập bảng` vẫn đứng cạnh nó cho xã không có tệp; prototype không có năm nên có ô
 * năm — năm ấy cũng là năm tệp được nạp vào;
 * thẻ ba chỉ số của năm và khối chốt kỳ — hai thứ của CẢ NĂM, không của một bảng — nằm dưới bảng.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * BA KHOÁ QUYỀN, VÀ CHỖ DỄ GẮN NHẦM NHẤT ĐƯỢC GHI RA: `budget.read` mở cả màn (cổng ở trang),
 * `budget.update` mở LẬP BẢNG · THÊM · SỬA, `budget.confirm` mở **GỠ** và **ĐÁNH DẤU DÒNG TỔNG**.
 * Hai thao tác sau không đứng sau `budget.update` vì chúng đổi những con số đã được đọc trên một
 * màn hình và đã đi lên cấp trên (`routes.go:821,909,943`).
 *
 * Ẩn một nút theo quyền là TRẢI NGHIỆM, không phải biện pháp: máy chủ kiểm `(tenant_id, role,
 * permission)` trên từng yêu cầu, và gọi thẳng tuyến bằng chính cookie phiên vẫn 403 (luật 5 cấm #1).
 */

type TrangThai<T> = { pha: "dangTai" } | { pha: "loi"; thongBao: string } | { pha: "xong"; duLieu: T };

function trangThaiTu<T>(
  daTai: { khoa: string; kq: KetQua<T> } | null,
  khoa: string,
): TrangThai<T> {
  if (daTai === null || daTai.khoa !== khoa) return { pha: "dangTai" };
  return daTai.kq.ok
    ? { pha: "xong", duLieu: daTai.kq.duLieu }
    : { pha: "loi", thongBao: daTai.kq.thongBao };
}

/** Hộp thoại đang mở. Một tại một thời điểm — hai hộp cùng mở là hai lần xác nhận chồng nhau. */
type DangMo =
  | { kieu: "them"; chaId: string; tenCha: string; thuTuGoiY: number }
  | { kieu: "goDong"; dong: finance_dongRa }
  | { kieu: "goBang" }
  | { kieu: "suaBang" }
  | { kieu: "createSheet" }
  | { kieu: "cachTinh"; dong: finance_dongRa; den: CachTinhChon }
  // TT and display order: the two fields the in-cell edit does not cover.
  | { kieu: "lineOrder"; dong: finance_dongRa }
  // Hộp `⇄` GIỮ cột và đơn vị lúc mở: sau mỗi lần ghi đợt bảng được đọc lại, và trong lúc đọc lại
  // hộp không được biến mất cùng thông báo "Đã ghi đợt" của nó.
  | { kieu: "dot"; dong: finance_dongRa; cot: readonly finance_cotRa[]; donVi: DonViHien };

export function BangThuChi() {
  // Năm neo đọc MỘT lần khi component gắn vào: đọc lại sẽ làm danh sách năm nhảy ngay giữa phiên
  // làm việc của một cán bộ trực đêm 31/12.
  const [namGoc] = useState(namTheoDongHoMay);
  const [nam, datNam] = useState(namGoc);
  const [loai, datLoai] = useState<LoaiBang>("chi");

  /**
   * Bộ đếm lần tải. Mỗi lần ghi xong thì tăng một, và cả hai lời gọi đọc chạy lại.
   *
   * ĐỌC LẠI CẢ BẢNG SAU MỖI LẦN GHI, KHÔNG VÁ TẠI CHỖ. Con số của một dòng cha là TỔNG các con và
   * do máy chủ suy ra; vá một dòng vào danh sách đang giữ sẽ để nguyên tổng của cha nó — một con
   * số cũ nằm ngay trên con số mới, và không có gì nói ra rằng nó cũ. Chính vì thế tuyến ghi cũng
   * không gửi `values` về (`dongRaMot`).
   */
  const [lanTai, datLanTai] = useState(0);

  const [daTaiBang, datDaTaiBang] = useState<{ khoa: string; kq: KetQua<finance_bangDayDuRa> } | null>(
    null,
  );
  const [daTaiChiSo, datDaTaiChiSo] = useState<{
    khoa: string;
    kq: KetQua<finance_chiSoNamRa>;
  } | null>(null);

  // Close history of the selected year. Read on the same reload counter as the sheet: a close or a
  // reopen changes which buttons are live, and a write refused with 409 may mean the list is stale.
  const [loadedCloses, setLoadedCloses] = useState<{
    key: string;
    result: KetQua<finance_budgetPeriodClosesOut>;
  } | null>(null);

  const [thuGon, datThuGon] = useState<ReadonlySet<string>>(new Set());
  const [dangMo, datDangMo] = useState<DangMo | null>(null);
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  const [dangGui, datDangGui] = useState(false);

  const khoaBang = `${nam}|${loai}|${lanTai}`;
  const khoaChiSo = `${nam}|${lanTai}`;
  const closesKey = `${nam}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    layBang({ nam, loai }).then((kq) => {
      if (!bo) datDaTaiBang({ khoa: khoaBang, kq });
    });
    return () => {
      bo = true;
    };
  }, [nam, loai, khoaBang]);

  useEffect(() => {
    let bo = false;
    layChiSoNganSach(nam).then((kq) => {
      if (!bo) datDaTaiChiSo({ khoa: khoaChiSo, kq });
    });
    return () => {
      bo = true;
    };
  }, [nam, khoaChiSo]);

  useEffect(() => {
    let dropped = false;
    listBudgetPeriodCloses(nam).then((result) => {
      if (!dropped) setLoadedCloses({ key: closesKey, result });
    });
    return () => {
      dropped = true;
    };
  }, [nam, closesKey]);

  const phien = usePhien();
  // FAIL CLOSED: chưa đọc xong phiên, hoặc đọc hỏng, thì KHÔNG có quyền nào — "chưa rõ" không
  // được hành xử như "có" (luật 1, cấm #1).
  const dsQuyen: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const coGhi = coQuyen(dsQuyen, QUYEN_GHI_NGAN_SACH);
  const coXacNhan = coQuyen(dsQuyen, QUYEN_XAC_NHAN_NGAN_SACH);

  const bang = trangThaiTu(daTaiBang, khoaBang);
  const chiSo = trangThaiTu(daTaiChiSo, khoaChiSo);
  const closesView: ClosesView =
    loadedCloses === null || loadedCloses.key !== closesKey
      ? { phase: "loading" }
      : loadedCloses.result.ok
        ? { phase: "ready", closes: loadedCloses.result.duLieu.closes }
        : { phase: "error", message: loadedCloses.result.thongBao };
  // DISPLAY ONLY. Not loaded yet, or not readable: nothing is drawn as locked, and the server still
  // refuses a write into a closed period with a 409 shown verbatim.
  const closes = closesView.phase === "ready" ? closesView.closes : [];
  const sheetLock = bang.pha === "xong" ? sheetLockReason(closes, bang.duLieu.sheet.year) : null;

  /** Một lần ghi xong: đóng hộp thoại đang mở, xoá thông báo cũ, và đọc lại từ máy chủ. */
  function xong(kq: KetQua<unknown>): void {
    datDangGui(false);
    if (!kq.ok) {
      // NGUYÊN VĂN câu máy chủ: 409 của tuyến này mang đúng quy tắc nghiệp vụ đã từ chối ("khoản
      // mục có dòng con thì không gõ số vào cha"), và viết lại nó ở client là dựng bản sao thứ hai
      // của một quy tắc rồi để nó trôi. A dialog that is open shows it INSIDE itself — the page
      // behind a modal is inert and dimmed, so a sentence printed there is a sentence nobody reads.
      datLoiGhi(kq.thongBao);
      return;
    }
    datLoiGhi(null);
    datDangMo(null);
    datLanTai((n) => n + 1);
  }

  /**
   * One in-cell save (name or one figure). Resolves to the server's refusal, VERBATIM, or `null` on
   * success — the cell prints the refusal under its own box ("errors in place"), so it is NOT also
   * printed above the table. Success re-reads everything, as every write does (see `lanTai`).
   */
  function saveLine(id: string, body: SuaDongVao): Promise<string | null> {
    datDangGui(true);
    return suaKhoanMuc(id, body).then((kq) => {
      if (!kq.ok) {
        datDangGui(false);
        return kq.thongBao;
      }
      xong(kq);
      return null;
    });
  }

  /** Closes the open dialog — never while its request is in flight (the outcome would be hidden). */
  function closeDialog(): void {
    if (dangGui) return;
    datDangMo(null);
    datLoiGhi(null);
  }

  /** Switching year or sheet drops every half-done edit: it belonged to the other sheet. */
  function resetEdits(): void {
    datDangMo(null);
    datLoiGhi(null);
  }

  return (
    // `mt-0`: `.man-giai-ngan` (legacy layer) adds a 2rem top margin for pages whose header sits
    // apart; here the header's own `mb-4` is the gap (spec 02 "Khung").
    <section className="man-giai-ngan mt-0 flex min-w-0 flex-col gap-3" aria-labelledby="tieu-de-thu-chi">
      {/* The page's `<h1>` already says this; the heading stays as the region's accessible name. */}
      <h2 id="tieu-de-thu-chi" className="an-thi-giac">
        Thu - Chi ngân sách xã
      </h2>

      <SheetSelectionBar
        year={nam}
        anchorYear={namGoc}
        onYearChange={(n) => {
          datNam(n);
          resetEdits();
        }}
        kind={loai}
        onKindChange={(l) => {
          datLoai(l);
          resetEdits();
        }}
        sheetState={bang.pha === "xong" ? "ready" : bang.pha === "loi" ? "missing" : "loading"}
        canRecord={coGhi}
        canConfirm={coXacNhan}
        sheetLock={sheetLock}
        busy={dangGui}
        onCreate={() => {
          resetEdits();
          datDangMo({ kieu: "createSheet" });
        }}
        onEdit={() => {
          resetEdits();
          datDangMo({ kieu: "suaBang" });
        }}
        onRemove={() => {
          resetEdits();
          datDangMo({ kieu: "goBang" });
        }}
        onImported={(firstKind) => {
          // The file may have created or REPLACED either sheet of the year: every read starts over, and
          // the first sheet loaded is selected, as the prototype does (`FiscalReportPanel.tsx:141`).
          resetEdits();
          if (firstKind !== null) datLoai(firstKind);
          datLanTai((n) => n + 1);
        }}
      />

      <div
        role="tabpanel"
        id={`bang-ngan-sach-${loai}`}
        aria-labelledby={`tab-ngan-sach-${loai}`}
        tabIndex={0}
        className="flex min-w-0 flex-col gap-3 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500"
      >
        {/* A write refused while NO dialog is open (the headline star). */}
        {loiGhi !== null && dangMo === null && (
          <p className="thong-bao-loi m-0" role="alert">
            {loiGhi}
          </p>
        )}

        {bang.pha === "dangTai" && (
          <Card>
            <p role="status" className="an-thi-giac">
              Đang tải bảng ngân sách…
            </p>
            <SkeletonRows rows={6} />
          </Card>
        )}

        {bang.pha === "loi" && (
          <Card>
            <CardContent className="flex flex-col gap-3">
              {/* KHÔNG ĐOÁN VÌ SAO KHÔNG ĐỌC ĐƯỢC: hiện NGUYÊN câu máy chủ. 404 ở tuyến này thường
                  là "xã chưa lập bảng", nhưng phân biệt bằng cách đọc câu chữ là dựng lại đúng thứ
                  `goi.ts` cố ý giấu đi. The server's sentence is shown VERBATIM per ADR 0035 §A — a
                  muted note, not a red alarm, because the usual cause is "not set up yet". */}
              <Notice tone="neutral" icon={Database} role="alert">
                {bang.thongBao}
              </Notice>
              {/* The prototype's empty sentence, verbatim (owner decision 09/10/2026). */}
              <p className="m-0 text-[12.5px] text-ink-muted">{NO_REPORT_YET}</p>
            </CardContent>
          </Card>
        )}

        {bang.pha === "xong" && (
          <BangDayDu
            duLieu={bang.duLieu}
            thuGon={thuGon}
            datThuGon={datThuGon}
            coGhi={coGhi}
            coXacNhan={coXacNhan}
            sheetLock={sheetLock}
            dangGui={dangGui}
            saveLine={saveLine}
            openLineOrder={(dong) => {
              resetEdits();
              datDangMo({ kieu: "lineOrder", dong });
            }}
            moThem={(chaId, tenCha, thuTuGoiY) => {
              resetEdits();
              datDangMo({ kieu: "them", chaId, tenCha, thuTuGoiY });
            }}
            moGoDong={(dong) => {
              resetEdits();
              datDangMo({ kieu: "goDong", dong });
            }}
            datTong={(dong) => {
              datDangGui(true);
              datDongTong(dong.id).then(xong);
            }}
            moCachTinh={(dong, den) => {
              resetEdits();
              datDangMo({ kieu: "cachTinh", dong, den });
            }}
            moDot={(dong) => {
              resetEdits();
              datDangMo({
                kieu: "dot",
                dong,
                cot: cotSo(bang.duLieu.columns),
                donVi: donViCuaBang(bang.duLieu.sheet),
              });
            }}
          />
        )}
      </div>

      {/* THẺ BA CHỈ SỐ NẰM NGOÀI THANH CHỌN BẢNG, có chủ ý: con số chênh lệch cần CẢ HAI bảng, nên
          đặt nó trong phần của một bảng sẽ nói rằng con số ấy thuộc về bảng ấy. Dưới bảng, vì
          prototype không có khối này và phần trên là của bảng đang chọn. */}
      {chiSo.pha === "dangTai" && (
        <Card>
          <p role="status" className="an-thi-giac">
            Đang tải chỉ số ngân sách…
          </p>
          <div aria-hidden="true" className="grid gap-3 p-4 sm:grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }, (_, i) => (
              <div key={i} className="flex flex-col gap-3 rounded-control border border-line p-3">
                <Skeleton className="w-24" />
                <Skeleton className="h-6 w-32" />
              </div>
            ))}
          </div>
        </Card>
      )}
      {chiSo.pha === "loi" && (
        <Card>
          <ErrorState
            title="Chưa tải được chỉ số ngân sách"
            message={<span role="alert">{chiSo.thongBao}</span>}
            onRetry={() => datLanTai((n) => n + 1)}
          />
        </Card>
      )}
      {chiSo.pha === "xong" && <TheChiSoNam chiSo={chiSo.duLieu} />}

      {/* THE CLOSE BLOCK COVERS A PERIOD OF THE COMMUNE'S BUDGET — both sheets of that year — so it
          sits with the year's indicators, outside the selected sheet. */}
      <BudgetPeriodClosePanel
        // A new year is a new set of forms: a key kept from 2025 must not close 2026.
        key={nam}
        year={nam}
        view={closesView}
        canConfirm={coXacNhan}
        onChanged={() => datLanTai((n) => n + 1)}
      />

      {/* ── các hộp thoại (prototype: `window.prompt` / `window.confirm` / `Dialog`) ──────────── */}

      {dangMo !== null && dangMo.kieu === "createSheet" && coGhi && (
        <LapBang
          // ĐỔI NĂM HOẶC ĐỔI BẢNG THÌ DỰNG LẠI BIỂU MẪU, bằng `key`: bộ cột điền sẵn phụ thuộc cả năm
          // lẫn loại bảng, nên giữ lại bộ cột cũ là lập bảng thu bằng bộ cột của bảng chi.
          key={`${nam}|${loai}`}
          nam={nam}
          loai={loai}
          dangGui={dangGui}
          datDangGui={datDangGui}
          xong={xong}
          onClose={closeDialog}
          serverError={loiGhi}
        />
      )}

      {dangMo !== null && dangMo.kieu === "suaBang" && bang.pha === "xong" && (
        <FormSuaBang
          bang={bang.duLieu.sheet}
          dangGui={dangGui}
          huy={closeDialog}
          serverError={loiGhi}
          luu={(than) => {
            datDangGui(true);
            suaBang(bang.duLieu.sheet.id, than).then(xong);
          }}
        />
      )}

      {dangMo !== null && dangMo.kieu === "them" && bang.pha === "xong" && (
        <FormThemKhoanMuc
          tenCha={dangMo.tenCha}
          thuTuGoiY={dangMo.thuTuGoiY}
          dangGui={dangGui}
          huy={closeDialog}
          serverError={loiGhi}
          luu={(no, ten, thuTu, khoa) => {
            datDangGui(true);
            themKhoanMuc(
              {
                sheet_id: bang.duLieu.sheet.id,
                parent_id: dangMo.chaId === "" ? undefined : dangMo.chaId,
                no,
                name: ten,
                order: thuTu,
              },
              khoa,
            ).then(xong);
          }}
        />
      )}

      {dangMo !== null && dangMo.kieu === "lineOrder" && (
        <LineOrderDialog
          dong={dangMo.dong}
          dangGui={dangGui}
          huy={closeDialog}
          serverError={loiGhi}
          luu={(body) => {
            datDangGui(true);
            suaKhoanMuc(dangMo.dong.id, body).then(xong);
          }}
        />
      )}

      {dangMo !== null && dangMo.kieu === "cachTinh" && (
        <ConfirmModal formId={CONFIRM_FORM_ID} onDismiss={closeDialog} error={loiGhi}>
          <FormDoiCachTinh
            formId={CONFIRM_FORM_ID}
            className="shadow-none"
            ten={dangMo.dong.name}
            den={dangMo.den}
            dangGui={dangGui}
            huy={closeDialog}
            luu={() => {
              datDangGui(true);
              doiCachTinh(dangMo.dong.id, dangMo.den).then(xong);
            }}
          />
        </ConfirmModal>
      )}

      {dangMo !== null && dangMo.kieu === "goDong" && (
        <ConfirmModal formId={CONFIRM_FORM_ID} onDismiss={closeDialog} error={loiGhi}>
          <FormGoKemLyDo
            formId={CONFIRM_FORM_ID}
            className="shadow-none"
            tieuDe={`Gỡ khoản mục ${dangMo.dong.name}?`}
            submitLabel="Gỡ khoản mục"
            canhBao={CANH_BAO_GO_KHOAN_MUC}
            dangGui={dangGui}
            huy={closeDialog}
            luu={(lyDo) => {
              datDangGui(true);
              goKhoanMuc(dangMo.dong.id, lyDo).then(xong);
            }}
          />
        </ConfirmModal>
      )}

      {dangMo !== null && dangMo.kieu === "goBang" && bang.pha === "xong" && (
        <ConfirmModal formId={CONFIRM_FORM_ID} onDismiss={closeDialog} error={loiGhi}>
          <FormGoKemLyDo
            formId={CONFIRM_FORM_ID}
            className="shadow-none"
            tieuDe={`Gỡ bảng ${bang.duLieu.sheet.title}?`}
            submitLabel="Gỡ bảng"
            canhBao={CANH_BAO_GO_BANG}
            dangGui={dangGui}
            huy={closeDialog}
            luu={(lyDo) => {
              datDangGui(true);
              goBang(bang.duLieu.sheet.id, lyDo).then(xong);
            }}
          />
        </ConfirmModal>
      )}

      {dangMo !== null && dangMo.kieu === "dot" && (
        <HopDotThuChi
          // Đổi khoản mục thì dựng lại hộp: danh sách, khoá chống trùng và biểu mẫu là của MỘT dòng.
          key={dangMo.dong.id}
          khoanMucId={dangMo.dong.id}
          tenKhoanMuc={dangMo.dong.name}
          cot={dangMo.cot}
          donVi={dangMo.donVi}
          coGhi={coGhi}
          coXacNhan={coXacNhan}
          closes={closes}
          sheetYear={nam}
          dong={() => datDangMo(null)}
          daDoiSoLieu={() => datLanTai((n) => n + 1)}
        />
      )}
    </section>
  );
}

/** Id of the confirm box's `<form>` inside `ConfirmModal` — one confirm is open at a time. */
const CONFIRM_FORM_ID = "hop-xac-nhan-thu-chi";

/**
 * A confirm box opened as the prototype's `window.confirm` — a centred modal, the page behind inert.
 * The box itself (`ConfirmDialog` inside `FormGoKemLyDo` / `FormDoiCachTinh`) is unchanged; the
 * server's refusal is printed under it, inside the modal.
 */
function ConfirmModal({
  formId,
  onDismiss,
  error,
  children,
}: {
  formId: string;
  onDismiss: () => void;
  error: string | null;
  children: ReactNode;
}) {
  return (
    <ModalDialog titleId={formId} onDismiss={onDismiss} className="gap-0 p-0">
      {children}
      {error !== null && (
        <p className="thong-bao-loi m-0 px-4 pb-4" role="alert">
          {error}
        </p>
      )}
    </ModalDialog>
  );
}

/**
 * One sheet button of the selection bar — the prototype's period button (`FiscalReportPanel.tsx:162-174`):
 * separate bordered buttons, the selected one solid navy. Native `<button>` chrome is off by hand
 * (`[font-family:inherit]`, explicit colours): Tailwind's preflight is not loaded in this app.
 */
const SHEET_BUTTON =
  "cursor-pointer rounded-[8px] border border-solid px-3 py-1.5 [font-family:inherit] text-[12.5px] leading-tight font-semibold focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500";

/**
 * The prototype's first card (`FiscalReportPanel.tsx:159-234`): which sheet is on screen on the left,
 * the sheet's own actions on the right.
 *
 * LEFT — the year select, then the two sheets of that year as separate buttons (`Chi ngân sách 2026`,
 * `Thu ngân sách 2026`). The prototype lists the IMPORTED reports and has no year (the file carries
 * it); this contract reads one sheet per year and kind, so the year is a control and both kinds are
 * always offered — a kind with no sheet yet opens on its empty state. The buttons keep TAB semantics
 * (`tablist` / `tab` / `aria-selected`) because they drive the `tabpanel` below.
 *
 * RIGHT — `Nạp từ Excel` at the prototype's place (ADR 0081 #6: loads the selected year's sheets from the
 * finance office's file); `Lập bảng` beside it while the sheet does not exist (the form for a commune
 * without the file); `Sửa thông tin bảng` and `Gỡ` once it does — an imported sheet included. Gates: `budget.update` for the first three,
 * `budget.confirm` for `Gỡ` (`routes.go:821`). A YEAR close keeps edit and removal VISIBLE and
 * disabled with the reason as tooltip — a vanished button reads as "you lack the right".
 */
export function SheetSelectionBar({
  year,
  anchorYear,
  onYearChange,
  kind,
  onKindChange,
  sheetState,
  canRecord,
  canConfirm,
  sheetLock,
  busy,
  onCreate,
  onEdit,
  onRemove,
  onImported,
}: {
  year: number;
  anchorYear: number;
  onYearChange: (year: number) => void;
  kind: LoaiBang;
  onKindChange: (kind: LoaiBang) => void;
  /** `missing` = the sheet could not be read (usually: not created yet). */
  sheetState: "loading" | "ready" | "missing";
  canRecord: boolean;
  canConfirm: boolean;
  sheetLock: string | null;
  busy: boolean;
  onCreate: () => void;
  onEdit: () => void;
  onRemove: () => void;
  /** After `Nạp từ Excel` loaded the file: the board re-reads everything and selects `firstKind`. */
  onImported: (firstKind: LoaiBang | null) => void;
}) {
  const locked = sheetLock !== null;
  const lockTitle = sheetLock ?? undefined;
  return (
    <Card className="flex min-w-0 flex-wrap items-center gap-2 px-4 py-3">
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <Field label="Năm ngân sách" hideLabel htmlFor="nam-ngan-sach-thu-chi" kind="select" grow="auto">
          <select id="nam-ngan-sach-thu-chi" value={year} onChange={(e) => onYearChange(Number(e.target.value))}>
            {danhSachNam(anchorYear).map((n) => (
              <option key={n} value={n}>
                Năm ngân sách {n}
              </option>
            ))}
          </select>
        </Field>
        <div role="tablist" aria-label="Chọn bảng thu hoặc bảng chi" className="flex flex-wrap gap-1.5">
          {(["chi", "thu"] as const).map((l) => (
            <button
              key={l}
              type="button"
              role="tab"
              id={`tab-ngan-sach-${l}`}
              aria-selected={kind === l}
              aria-controls={`bang-ngan-sach-${l}`}
              className={cn(
                SHEET_BUTTON,
                kind === l ? "border-navy bg-navy text-white" : "border-line bg-surface text-ink-muted",
              )}
              onClick={() => onKindChange(l)}
            >
              {nhanTab(l, year)}
            </button>
          ))}
        </div>
      </div>

      {(canRecord || canConfirm) && (
        <div className="ml-auto flex min-w-0 flex-wrap items-center gap-2">
          {canRecord && <BudgetSheetHeaderActions year={year} busy={busy} onImported={onImported} />}
          {canRecord && sheetState === "missing" && (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              icon={<Glyph icon={FilePlus2} />}
              disabled={busy}
              onClick={onCreate}
            >
              Lập bảng
            </Button>
          )}
          {canRecord && sheetState === "ready" && (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              icon={<Glyph icon={Pencil} />}
              disabled={busy || locked}
              title={lockTitle}
              onClick={onEdit}
            >
              Sửa thông tin bảng
            </Button>
          )}
          {canConfirm && sheetState === "ready" && (
            // The prototype's `Button size="sm" variant="outline"` — the normal text colour, not red:
            // the reason dialog behind it is where the consequence is said. The accessible name says
            // WHAT is removed.
            <Button
              type="button"
              variant="outline"
              size="sm"
              icon={<Glyph icon={Trash2} />}
              aria-label="Gỡ bảng"
              disabled={busy || locked}
              title={lockTitle ?? "Gỡ bảng"}
              onClick={onRemove}
            >
              Gỡ
            </Button>
          )}
        </div>
      )}
    </Card>
  );
}

/**
 * Thẻ ba chỉ số của năm (§9 quy tắc 6) — thẻ nuôi `/tong-quan` và `/bao-cao`.
 *
 * THẺ KHÔNG BAO GIỜ BIẾN MẤT VÀ KHÔNG BAO GIỜ HIỆN `0`. Tuyến này trả 200 kèm
 * `unavailable_reason` chứ không 404 khi xã chưa có bảng, đúng để thẻ ở lại và nói ra việc còn
 * phải làm. Đọc một `null` thành `0` là báo lên lãnh đạo một con số cân đối mà xã chưa từng khai.
 *
 * CẢ HAI SỐ THU ĐỀU HIỆN, MỖI SỐ GỌI ĐÚNG TÊN (ADR 0035 §A): xã cần một số để báo cáo thu ngân
 * sách, và một số để biết mình còn được giữ bao nhiêu.
 *
 * SỐ TIỀN Ở THẺ NÀY TÍNH BẰNG ĐỒNG: thẻ nằm ngoài hai bảng và con số chênh lệch đọc từ CẢ HAI bảng,
 * hai bảng có thể mang hai đơn vị khác nhau. Ô in dạng gọn (`compactDong`, "853,3 tỷ đồng"); con số
 * đầy đủ kèm chữ "đồng" nằm ở `title` của ô. (The table and the summary tiles print in the sheet's
 * unit instead — this card spans two sheets, so it keeps đồng.)
 */
export function TheChiSoNam({ chiSo }: { chiSo: finance_chiSoNamRa }) {
  const balance = chiSo.balance;
  const balanceText = nhanSoTienChiSo(balance, "dong");
  return (
    <Card className="flex min-w-0 flex-col px-4 py-3">
      <h3 className="m-0 inline-flex min-w-0 items-center gap-2 text-[14px] leading-snug font-bold text-navy">
        <Glyph icon={Gauge} className="size-4 shrink-0 text-brand-600" />
        Chỉ số ngân sách năm {chiSo.year}
      </h3>
      <dl className={cn(FIGURE_TILES, "mt-2")}>
        <OChiSo chi={chiSo.revenue_achievement} />
        <OChiSo chi={chiSo.expenditure_achievement} />
        <FigureTile label={NHAN_CHENH_LECH} exact={exactDong(balance.amount)}>
          {balance.amount !== null || balanceText === O_TRONG ? (
            compactOrText(balance.amount, balanceText)
          ) : (
            // The server's reason, at body size.
            <span className={SENTENCE_TEXT}>{balanceText}</span>
          )}
        </FigureTile>
        {/* TỔNG THU mang câu lý do cho MỌI trường hợp không đưa ra được con số (chưa có dòng tổng,
            cột trống, tổng quá lớn) — nên `null` kèm câu là "không tính được", còn `null` không câu
            vẫn là `—`. */}
        {chiSo.revenue_totals.map((o) => {
          const reason = lyDoKhongTinh(o.unavailable_reason);
          return (
            <FigureTile key={o.column_id} label={o.name} exact={reason === null ? exactDong(o.value) : undefined}>
              {reason === null ? (
                compactOrText(o.value, nhanSoTien(o.value, "dong"))
              ) : (
                <OTien chu={nhanSoTien(o.value, "dong")} lyDo={reason} hienLyDo />
              )}
            </FigureTile>
          );
        })}
      </dl>
      <p className="m-0 mt-2 inline-flex items-start gap-1.5 text-[11.5px] text-ink-muted">
        <Glyph icon={Info} className="mt-0.5 size-3.5 shrink-0" />
        <span>
          {NHAN_CHENH_LECH}: {GHI_CHU_CHENH_LECH}
        </span>
      </p>
    </Card>
  );
}

/** The exact amount in đồng for a tile's hover, or nothing when there is no readable figure. */
function exactDong(amount: number | null): string | undefined {
  return amount !== null && Number.isSafeInteger(amount) ? `${dongSangChuoi(amount, "dong")} đồng` : undefined;
}

/**
 * A tile's figure: the compact form for a readable amount, else the same words the table uses (`—`,
 * "Không đọc được") — never "0".
 */
function compactOrText(amount: number | null, text: string): string {
  return amount !== null && Number.isSafeInteger(amount) ? compactDong(amount) : text;
}

/**
 * Grid of figure tiles — the prototype's `grid gap-3 sm:grid-cols-2 lg:grid-cols-4`. Tailwind's
 * `grid-cols-N` is `minmax(0, 1fr)`, so a long label truncates inside its tile instead of widening it.
 */
const FIGURE_TILES = "m-0 grid min-w-0 gap-3 sm:grid-cols-2 lg:grid-cols-4";

/** A server SENTENCE in place of a figure: body size, so it stays readable in a narrow tile. */
const SENTENCE_TEXT = "block text-[13px] leading-snug font-medium text-ink-700";

/**
 * One tile, the prototype's (`FiscalReportPanel.tsx:267-277`): a page-colour box with a hairline, the
 * label (11px, muted) above the figure (18px bold navy). Tiles of a row are equal in height (grid
 * stretch). `exact` is the full amount, on hover of the whole tile.
 *
 * `<dd>` CARRIES NO `title`: the tests read `<dd …>…</dd>` back, and a "—" must stay a bare "—".
 */
function FigureTile({ label, exact, children }: { label: string; exact?: string; children: ReactNode }) {
  return (
    <div title={exact} className="flex min-w-0 flex-col rounded-[8px] border border-solid border-line bg-canvas px-3 py-2">
      <dt title={label} className="truncate text-[11px] text-ink-muted">
        {label}
      </dt>
      <dd className="m-0 min-w-0 text-[18px] leading-tight font-bold break-words text-navy tabular-nums">
        {children}
      </dd>
    </div>
  );
}

function OChiSo({ chi }: { chi: finance_chiSoRa }) {
  // TÊN CHỈ SỐ LẤY TỪ MÁY CHỦ, không gõ lại: `ChiSoDatDuToan` đặt tên theo loại bảng, và một bản
  // sao ở client sẽ gọi bảng thu là "Chi đạt dự toán" vào ngày ai đó đổi thứ tự. `nhanChiSo` prints
  // the server's reason when there is no figure; that sentence is drawn at body size.
  return (
    <FigureTile label={chi.name}>
      {isFigureOrDash(chi) ? nhanChiSo(chi) : <span className={SENTENCE_TEXT}>{nhanChiSo(chi)}</span>}
    </FigureTile>
  );
}

/** True when `nhanChiSo` prints a figure or `—`, false when it prints the server's sentence. */
function isFigureOrDash(indicator: finance_chiSoRa): boolean {
  return indicator.basis_points !== null || nhanChiSo(indicator) === O_TRONG;
}

/** Header cell of the tree table — the prototype's `px-3 py-2 font-semibold` (`:347-363`). */
const TH = "px-3 py-2 font-semibold";

/** Thẻ báo cáo (§2) + thanh công cụ (§4.3) + bảng cây (§4.1), in the prototype's order. */
export function BangDayDu({
  duLieu,
  thuGon,
  datThuGon,
  coGhi,
  coXacNhan,
  sheetLock,
  dangGui,
  saveLine,
  openLineOrder,
  moThem,
  moGoDong,
  datTong,
  moCachTinh,
  moDot,
}: {
  duLieu: finance_bangDayDuRa;
  thuGon: ReadonlySet<string>;
  datThuGon: (t: ReadonlySet<string>) => void;
  coGhi: boolean;
  coXacNhan: boolean;
  /**
   * Why this sheet cannot be edited (an active YEAR close), or `null`. Edit controls stay VISIBLE
   * and are disabled, with the reason printed once above the table: a control that vanishes reads
   * as "you lack the right", which is a different and wrong message.
   */
  sheetLock: string | null;
  dangGui: boolean;
  /** One in-cell save; resolves to the server's refusal or `null` — see `BangThuChi.saveLine`. */
  saveLine: (id: string, body: SuaDongVao) => Promise<string | null>;
  /** Opens the TT / display-order dialog of a line. */
  openLineOrder: (dong: finance_dongRa) => void;
  moThem: (chaId: string, tenCha: string, thuTuGoiY: number) => void;
  moGoDong: (dong: finance_dongRa) => void;
  datTong: (dong: finance_dongRa) => void;
  moCachTinh: (dong: finance_dongRa, den: CachTinhChon) => void;
  moDot: (dong: finance_dongRa) => void;
}) {
  const cay = dungCay(duLieu.lines);
  const dongHien = phangCay(cay, thuGon);
  const donVi = donViCuaBang(duLieu.sheet);
  const locked = sheetLock !== null;
  const lockTitle = sheetLock ?? undefined;
  const headlineId = duLieu.summary.headline_line_id ?? "";
  const headline = duLieu.lines.find((l) => l.id === headlineId);

  return (
    <>
      {locked && (
        <Notice tone="legal" icon={LockKeyhole} role="note">
          {sheetLock}
        </Notice>
      )}
      <TheTomTat
        bang={duLieu.sheet}
        tomTat={duLieu.summary}
        columns={duLieu.columns}
        headline={headline}
        soKhoanMuc={duLieu.lines.length}
        donVi={donVi}
      />

      {/* Toolbar of the tree (§4.3, prototype `:284-343`): the two view buttons and the count on
          the left, `Thêm khoản mục cấp cao nhất` at the right — at the TOP of a long table, where
          people look for it. Every word kept. */}
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="secondary"
          size="sm"
          icon={<Glyph icon={ChevronRight} />}
          onClick={() => datThuGon(moiDongCoCon(duLieu.lines))}
        >
          Chỉ xem mục lớn
        </Button>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          icon={<Glyph icon={ChevronDown} />}
          onClick={() => datThuGon(new Set())}
        >
          Mở hết chi tiết
        </Button>
        <span className="min-w-0 text-xs text-ink-500 tabular-nums">
          {nhanBoDem(dongHien.length, duLieu.lines.length)}
        </span>
        {coGhi && (
          <Button
            type="button"
            variant="secondary"
            size="sm"
            className="ml-auto"
            icon={<Glyph icon={Layers} />}
            disabled={dangGui || locked}
            title={lockTitle}
            onClick={() => moThem("", "", thuTuKeTiep(duLieu.lines, ""))}
          >
            Thêm khoản mục cấp cao nhất
          </Button>
        )}
      </div>

      {/* The prototype's table frame (`:345-388`): a rounded hairline box that scrolls sideways
          INSIDE itself. Its own markup, not the shared `.bang-danh-muc` / `data-table` look (14px,
          `nowrap`, 48px rows), which is the shape of a register, not of the finance office's form.
          `border-collapse` by hand: without preflight a table keeps `border-spacing: 2px`, and the
          row hairlines would not join. */}
      <div
        role="region"
        tabIndex={0}
        aria-label={`Khoản mục của ${duLieu.sheet.title}`}
        className="min-w-0 overflow-x-auto rounded-[10px] border border-solid border-line bg-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500"
      >
        <table className="w-full border-collapse text-[12.5px]">
          <caption className="an-thi-giac">{duLieu.sheet.title}</caption>
          <thead className="border-b border-line text-ink-muted">
            <tr>
              <th scope="col" className="w-8">
                <span className="an-thi-giac">Mở, thu gọn và dòng tổng</span>
              </th>
              <th scope="col" className={cn(TH, "w-16 text-left")}>
                TT
              </th>
              <th scope="col" className={cn(TH, "min-w-[22rem] text-left")}>
                Nội dung
              </th>
              {duLieu.columns.map((c) => (
                // Figure columns right-aligned so digits line up down the column; the label keeps the
                // file's own line breaks (`whitespace-pre-line`).
                <th key={c.id} scope="col" className={cn(TH, "w-32 text-right whitespace-pre-line")}>
                  {c.name}
                  {/* Chú thích công thức của cột `%` — chữ HIỆN RÕ chứ không chỉ `title`, vì màn
                      cảm ứng không rê chuột được; nhỏ hơn nhãn cột (spec 03 A12). Chỉ là chữ: tỷ lệ
                      tính từ hai toán hạng, không từ chuỗi này. */}
                  {c.type === "phan_tram" && (c.formula ?? "").trim() !== "" && (
                    <>
                      <br />
                      <span className="text-[10.5px] font-normal">{c.formula}</span>
                    </>
                  )}
                </th>
              ))}
              <th scope="col" className={cn(TH, "w-44 text-left")}>
                Cách tính
              </th>
            </tr>
          </thead>
          <tbody>
            {dongHien.map((d) => (
              <DongKhoanMuc
                key={d.dong.id}
                hien={d}
                cot={duLieu.columns}
                donVi={donVi}
                dongTongId={headlineId}
                coGhi={coGhi}
                coXacNhan={coXacNhan}
                sheetLock={sheetLock}
                dangGui={dangGui}
                moRongDoi={() => {
                  const moi = new Set(thuGon);
                  if (moi.has(d.dong.id)) moi.delete(d.dong.id);
                  else moi.add(d.dong.id);
                  datThuGon(moi);
                }}
                saveLine={(body) => saveLine(d.dong.id, body)}
                openLineOrder={() => openLineOrder(d.dong)}
                them={() => moThem(d.dong.id, d.dong.name, thuTuKeTiep(duLieu.lines, d.dong.id))}
                go={() => moGoDong(d.dong)}
                datTong={() => datTong(d.dong)}
                doiCachTinh={(den) => moCachTinh(d.dong, den)}
                moDot={() => moDot(d.dong)}
              />
            ))}
          </tbody>
        </table>
      </div>
      <DanhSachKhongTinh tieuDe="Ô không tính được con số" o={oKhongTinhCuaCay(duLieu)} />
      <UnresolvedPercentColumns columns={duLieu.columns} />
    </>
  );
}

/**
 * Cột `%` cũ mà máy chủ chưa rõ tử số / mẫu số — nói MỘT LẦN ở mức cột, hiện rõ dưới bảng.
 *
 * Mỗi ô của cột ấy đã mang câu của máy chủ, nhưng hàng chục ô cùng một câu không nói ra điều duy
 * nhất cán bộ cần biết: lỗi nằm ở CỘT, không ở số liệu của dòng nào, và gõ lại số không sửa được.
 */
function UnresolvedPercentColumns({ columns }: { columns: readonly finance_cotRa[] }) {
  const unresolved = columns.filter(isUnresolvedPercentColumn);
  if (unresolved.length === 0) return null;
  // Red on purpose: a column whose ratio cannot be computed is wrong and needs the sheet fixed.
  return (
    <div className="thong-bao-loi m-0 rounded-xl border border-danger-200 bg-danger-50 px-4 py-3">
      <p className="m-0 inline-flex items-center gap-1.5 font-semibold">
        <Glyph icon={TriangleAlert} className="size-4 shrink-0" />
        Cột phần trăm chưa tính được tỷ lệ ({unresolved.length})
      </p>
      <ul className="m-0 mt-1 pl-5">
        {unresolved.map((c) => (
          <li key={c.id}>
            <strong>{c.name}</strong>: cột lập trước khi hệ thống lưu rõ cột tử số và cột mẫu số, và
            công thức cũ{(c.formula ?? "").trim() === "" ? "" : ` “${c.formula}”`} không xác định
            chắc chắn được hai cột ấy — tỷ lệ trên từng dòng không tính được. Hệ thống không đoán
            từ công thức.
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * Mọi ô không tính được của bảng, theo thứ tự máy chủ gửi dòng và thứ tự cột. Đọc từ `lines`, không
 * từ cây đang vẽ: một dòng đang thu gọn vẫn phải được nói ra, vì tổng của cha nó vừa hiện "Không
 * tính được" và người đọc cần biết phải mở dòng nào.
 */
function oKhongTinhCuaCay(duLieu: finance_bangDayDuRa): { khoa: string; noi: string; lyDo: string }[] {
  const ra: { khoa: string; noi: string; lyDo: string }[] = [];
  for (const d of duLieu.lines) {
    for (const c of duLieu.columns) {
      // CHỈ Ô TIỀN. Ô `%` không tính được là chuyện thường (mẫu số trống ở một dòng chưa khai) và
      // mang câu ngay trong ô; đưa chúng vào đây là chôn một tổng tràn số — thứ phải sửa — dưới hàng
      // chục dòng "ô mẫu số đang trống". Cột `%` hỏng ở mức cột có khối riêng bên dưới.
      if (c.type !== "so") continue;
      const lyDo = lyDoKhongTinh(d.unavailable_reasons?.[c.id]);
      if (lyDo === null) continue;
      const ten = d.no === "" ? d.name : `${d.no}. ${d.name}`;
      ra.push({ khoa: `${d.id}|${c.id}`, noi: `${ten} — ${c.name}`, lyDo });
    }
  }
  return ra;
}

/** Thứ tự gợi ý cho dòng mới: sau dòng cuối cùng cùng cha. Người nhập vẫn sửa được. */
function thuTuKeTiep(dong: readonly finance_dongRa[], chaId: string): number {
  let lonNhat = 0;
  for (const d of dong) {
    if ((d.parent_id ?? "") !== chaId) continue;
    if (d.order > lonNhat) lonNhat = d.order;
  }
  return lonNhat + 1;
}

/**
 * What one summary tile shows for one column (prototype `:266-278`: one tile per report column, the
 * value read from the headline line).
 *
 * A MONEY column reads the server's summary cell — it carries the "không tính được" reason when the
 * sum overflowed — and falls back to the headline line itself only when the server sent no cell for
 * that column. A `%` column reads the headline line's server-computed ratio. Nothing is computed here.
 */
function summaryFigure(
  column: finance_cotRa,
  tomTat: finance_tomTatRa,
  headline: finance_dongRa | undefined,
  donVi: DonViHien,
): { text: string; reason: string | null; exact?: string } {
  if (column.type !== "so") {
    return headline === undefined ? { text: O_TRONG, reason: null } : percentCellRounded(headline, column.id);
  }
  const cell = tomTat.cells.find((o) => o.column_id === column.id);
  const value = cell !== undefined ? cell.value : (headline?.values[column.id] ?? null);
  const reason = lyDoKhongTinh(
    cell !== undefined ? cell.unavailable_reason : headline?.unavailable_reasons?.[column.id],
  );
  return {
    text: formatSheetAmount(value, donVi.ma),
    reason,
    // The exact figure in the sheet's unit, on hover: the tile shows it rounded to one decimal.
    exact:
      reason === null && value !== null && Number.isSafeInteger(value)
        ? `${nhanSoTien(value, donVi.ma)} ${donVi.nhan.toLowerCase()}`
        : undefined,
  };
}

/**
 * Thẻ báo cáo của §2 — the prototype's second card (`FiscalReportPanel.tsx:240-282`): title, the
 * sub-line (`Đơn vị tính` · `Luỹ kế đến` · `N khoản mục` · source file), the unit note, the headline
 * sentence naming the starred line, then ONE TILE PER COLUMN of the sheet, the `%` column included.
 *
 * `unavailable_reason` CỦA THẺ TÓM TẮT LÀ MỘT CÂU, KHÔNG PHẢI MỘT Ô TRỐNG. Chưa ai đánh dấu dòng
 * tổng, hoặc hai dòng cùng nhận là dòng tổng, thì không có gì để đọc — và câu của máy chủ nói ra
 * việc phải làm. Điền đại bằng "dòng đầu tiên" là đúng cái đoán mà `is_headline` sinh ra để từ
 * chối: bảng thu có hai dòng cấp cao lồng nhau, bảng chi có `Tổng số` đứng ngang hàng A…E.
 *
 * SỐ Ở Ô IN THEO ĐƠN VỊ CỦA BẢNG, như bảng bên dưới (quyết định 09/10/2026), làm tròn một chữ số lẻ;
 * con số chính xác nằm ở `title` của ô.
 */
export function TheTomTat({
  bang,
  tomTat,
  columns,
  headline,
  soKhoanMuc,
  donVi,
}: {
  bang: finance_bangRa;
  tomTat: finance_tomTatRa;
  /** Every column of the sheet, in order — one tile each. */
  columns: readonly finance_cotRa[];
  /** The starred line, when it is among the lines read. */
  headline?: finance_dongRa;
  soKhoanMuc: number;
  donVi: DonViHien;
}) {
  const sourceFile = (bang.source_file ?? "").trim();
  const reason = (tomTat.unavailable_reason ?? "").trim();
  return (
    <Card className="flex min-w-0 flex-col px-4 py-3">
      <h3 className="m-0 min-w-0 text-[14px] leading-snug font-bold break-words text-navy">{bang.title}</h3>
      <p className="m-0 mt-0.5 flex min-w-0 flex-wrap gap-x-3 gap-y-0.5 text-[11.5px] text-ink-muted tabular-nums">
        {subtitleParts(bang, soKhoanMuc).map((part) => (
          <span key={part}>{part}</span>
        ))}
        {sourceFile !== "" && (
          <span className="inline-flex min-w-0 items-center gap-1 break-all">
            <Glyph icon={FileSpreadsheet} className="size-3 shrink-0" />
            {sourceFile}
          </span>
        )}
      </p>
      {/* ĐƠN VỊ CŨ CHƯA ÁNH XẠ ĐƯỢC: nói NỔI BẬT, vì số đang in bằng đồng trong khi tờ giấy của
          xã in theo đơn vị khác — người đọc so hai bản sẽ thấy lệch hàng nghìn lần. */}
      {donVi.canhBao !== null && (
        <p className="thong-bao-loi m-0 mt-1 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3" role="alert">
          {donVi.canhBao}
          {donVi.nhanCu !== null && ` Đơn vị đang lưu: "${donVi.nhanCu}".`}
        </p>
      )}
      <p className="m-0 mt-1 text-[11.5px] text-ink-muted">{cauQuyDoi(donVi)}</p>

      {reason !== "" ? (
        <EmptyState icon={Star} tone="neutral" title={reason} className="py-4" />
      ) : (
        <>
          <p className="m-0 mt-3 text-xs text-ink-500">
            {headline !== undefined ? (
              <>
                Con số tổng lấy từ dòng <span className="font-semibold text-ink-900">{headline.name}</span>. Bấm
                ngôi sao ở đầu một dòng khác để đổi.
              </>
            ) : (
              "Con số tổng lấy từ dòng được đánh sao. Bấm ngôi sao ở đầu một dòng khác để đổi."
            )}
          </p>
          <dl className={cn(FIGURE_TILES, "mt-1.5")}>
            {columns.map((c) => {
              const figure = summaryFigure(c, tomTat, headline, donVi);
              return (
                <FigureTile key={c.id} label={c.name} exact={figure.exact}>
                  {figure.reason === null ? (
                    figure.text
                  ) : (
                    <OTien chu={figure.text} lyDo={figure.reason} hienLyDo />
                  )}
                </FigureTile>
              );
            })}
          </dl>
        </>
      )}
    </Card>
  );
}

/* ── in-cell edit ─────────────────────────────────────────────────────────────────────────── */

/** The name box's field key; a figure box is `value:<column id>`. */
const NAME_FIELD = "name";

type OpenCell = { field: string; draft: string; error: string | null; saving: boolean };

/**
 * The one box a row may have open (prototype: `editing` / `renaming`). Enter or leaving the box saves,
 * Esc cancels — the decision of WHAT is sent is the pure `cellNameEdit` / `cellValueEdit`.
 *
 * TWO REFS GUARD THE DOUBLE SAVE THE PROTOTYPE HAS: Enter saves, and the box then loses focus as it
 * closes (or goes read-only), which would save a second time. `live` is false once the box is closed
 * (Esc, or a save that succeeded); `inFlight` covers the time between Enter and the answer.
 */
function useCellEditor(save: (body: SuaDongVao) => Promise<string | null>) {
  const [open, setOpen] = useState<OpenCell | null>(null);
  const live = useRef(false);
  const inFlight = useRef(false);

  function start(field: string, draft: string): void {
    live.current = true;
    setOpen({ field, draft, error: null, saving: false });
  }

  function close(): void {
    live.current = false;
    setOpen(null);
  }

  function setDraft(draft: string): void {
    // Typing clears the last refusal: the sentence was about the previous text.
    setOpen((o) => (o === null ? o : { ...o, draft, error: null }));
  }

  async function commit(outcome: CellEditOutcome): Promise<void> {
    if (!live.current || inFlight.current) return;
    if (outcome.kind === "unchanged") {
      close();
      return;
    }
    if (outcome.kind === "invalid") {
      setOpen((o) => (o === null ? o : { ...o, error: outcome.message }));
      return;
    }
    inFlight.current = true;
    setOpen((o) => (o === null ? o : { ...o, saving: true, error: null }));
    const error = await save(outcome.body);
    inFlight.current = false;
    if (error === null) close();
    else setOpen((o) => (o === null ? o : { ...o, saving: false, error }));
  }

  return { open, start, close, setDraft, commit };
}

/**
 * The input of an open cell — the prototype's `Input h-7 text-[12.5px]`. A refusal (the client's or
 * the server's, verbatim) is printed UNDER the box, in the cell, and the box stays open: Enter retries,
 * Esc gives up. Leaving the box while a refusal is shown does not resend it.
 */
function CellInput({
  label,
  cell,
  numeric,
  onDraft,
  onSave,
  onCancel,
}: {
  label: string;
  cell: OpenCell;
  numeric?: boolean;
  onDraft: (value: string) => void;
  onSave: () => void;
  onCancel: () => void;
}) {
  const errorId = useId();
  return (
    <>
      <input
        // Focus moves into the box: it exists only because the officer just clicked the value it replaces.
        autoFocus
        type="text"
        aria-label={label}
        aria-invalid={cell.error !== null || undefined}
        aria-describedby={cell.error !== null ? errorId : undefined}
        aria-busy={cell.saving || undefined}
        readOnly={cell.saving}
        inputMode={numeric ? "decimal" : undefined}
        autoComplete="off"
        value={cell.draft}
        className={cn(
          controlClass,
          "h-7 text-[12.5px] md:text-[12.5px]",
          numeric && "text-right tabular-nums",
        )}
        onChange={(e) => onDraft(e.target.value)}
        onKeyDown={(e) => {
          const intent = editKeyIntent(e.key);
          if (intent === null) return;
          e.preventDefault();
          if (intent === "save") onSave();
          else onCancel();
        }}
        onBlur={() => {
          if (cell.error === null) onSave();
        }}
      />
      {cell.error !== null && (
        <p id={errorId} role="alert" className="m-0 mt-1 text-left text-[11.5px] font-normal whitespace-normal text-danger">
          {cell.error}
        </p>
      )}
    </>
  );
}

/** A bare icon action of the row (prototype `:645-705`): no frame, muted, darker on hover. */
function RowIconButton({
  icon,
  label,
  title,
  disabled,
  danger = false,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  title: string;
  disabled: boolean;
  danger?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={title}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "inline-flex shrink-0 cursor-pointer items-center justify-center rounded-sm border-0 bg-transparent p-0 text-ink-muted",
        "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
        "disabled:cursor-not-allowed disabled:opacity-50",
        danger ? "hover:not-disabled:text-danger" : "hover:not-disabled:text-navy",
      )}
    >
      <Glyph icon={icon} className="size-4" />
    </button>
  );
}

/** "Dòng đang là con số tổng" — the read-only star's tooltip and accessible name. */
const HEADLINE_LINE_LABEL = "Dòng đang là con số tổng";

/** The live star's tooltip, the prototype's (`:501`). Its accessible name stays `nhanDatDongTong`. */
const HEADLINE_STAR_TITLE = "Đặt làm con số tổng của báo cáo";

/** Tooltip of the row's TT / display-order button, and that dialog's title. */
export const LINE_ORDER_TITLE = "Sửa số thứ tự và thứ tự hiển thị";

/** The bare in-cell toggles (expand, star): no frame, no fixed box — the icon is the target. */
const TREE_ICON =
  "inline-flex shrink-0 cursor-pointer items-center justify-center border-0 bg-transparent p-0 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500 disabled:cursor-not-allowed disabled:opacity-50";

/**
 * A name or figure drawn as plain text that becomes a box on click (prototype `:544-554`, `:586-599`):
 * no link colour, no underline — the hover tint is the only cue. Font and colour inherited by hand:
 * without preflight a `<button>` draws in the browser's own font and `buttontext` black.
 */
const EDITABLE_TEXT =
  "-mx-1 cursor-pointer rounded border-0 bg-transparent px-1 py-0 [font:inherit] text-inherit hover:not-disabled:bg-canvas focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500 disabled:cursor-not-allowed";

/** The `Cách tính` select, the prototype's compact one (`:616`); `min-h-7` beats the legacy 36px floor. */
const METHOD_SELECT =
  "h-7 min-h-7 min-w-0 flex-1 rounded-md border border-solid border-line bg-surface bg-[position:right_0.25rem_center] py-0 pr-5 pl-1.5 text-[11.5px] disabled:bg-canvas disabled:text-ink-muted";

/** Một dòng của cây khoản mục (§4.1), in the prototype's columns. */
export function DongKhoanMuc({
  hien,
  cot,
  donVi,
  dongTongId,
  coGhi,
  coXacNhan,
  sheetLock,
  dangGui,
  moRongDoi,
  saveLine,
  openLineOrder,
  them,
  go,
  datTong,
  doiCachTinh,
  moDot,
}: {
  hien: DongHien;
  cot: readonly finance_cotRa[];
  donVi: DonViHien;
  dongTongId: string;
  coGhi: boolean;
  coXacNhan: boolean;
  /** See `BangDayDu`: disables every EDIT control of the row; `⇄` stays open to read entries. */
  sheetLock: string | null;
  dangGui: boolean;
  moRongDoi: () => void;
  /** Saves ONE field of this line; resolves to the server's refusal or `null`. */
  saveLine: (body: SuaDongVao) => Promise<string | null>;
  openLineOrder: () => void;
  them: () => void;
  go: () => void;
  datTong: () => void;
  doiCachTinh: (den: CachTinhChon) => void;
  moDot: () => void;
}) {
  const d = hien.dong;
  const laDongTong = d.id === dongTongId;
  // A PARENT always sums its children — on the tree drawn, or by the server's word. Its select is
  // disabled on `Cộng khoản mục con`, and it has no entries: both would be a 409 (`routes.go:1081`,
  // `:1204`).
  const isParent = hien.coCon || d.method === "children";
  const editDisabled = dangGui || sheetLock !== null;
  const lockTitle = sheetLock ?? undefined;
  // The form's two top levels are its headings: tinted and bold, as on the paper form (prototype
  // `depth <= 1`). Officers check the screen against the printed copy.
  const heading = hien.cap <= 1;
  // A figure the SERVER computes (sum of the children, or of the entries) is green, as the
  // prototype's `text-leaf` — the cue that typing into it is refused.
  const computed = d.method !== "manual";
  const cell = useCellEditor(saveLine);
  const open = cell.open;

  return (
    <tr className={cn("border-b border-line last:border-b-0", heading && "bg-surface-muted")}>
      <td className="px-1 py-1.5 align-top">
        <span className="flex items-center gap-0.5">
          {hien.coCon ? (
            <button
              type="button"
              className={cn(TREE_ICON, "text-ink-muted hover:text-navy")}
              aria-expanded={hien.moRong}
              aria-label={hien.moRong ? `Thu gọn ${d.name}` : `Mở chi tiết ${d.name}`}
              onClick={moRongDoi}
            >
              <Glyph icon={hien.moRong ? ChevronDown : ChevronRight} className="size-3.5" />
            </button>
          ) : (
            // Same width as the toggle, so the stars line up down the column.
            <span aria-hidden="true" className="inline-block size-3.5 shrink-0" />
          )}
          {/* The star sits on TOP-LEVEL lines only, as in the prototype: the headline is one of the
              form's top rows (`Tổng số`, `Tổng thu nội địa`). A nested line already starred still
              shows its read-only mark. */}
          {coXacNhan && hien.cap === 0 ? (
            <button
              type="button"
              className={cn(TREE_ICON, laDongTong ? "text-tangerine" : "text-ink-muted hover:not-disabled:text-tangerine")}
              aria-label={nhanDatDongTong(d.name)}
              aria-pressed={laDongTong}
              disabled={editDisabled}
              title={lockTitle ?? HEADLINE_STAR_TITLE}
              onClick={datTong}
            >
              <Glyph icon={Star} className={cn("size-3.5", laDongTong && "fill-current")} />
            </button>
          ) : (
            // Không có quyền đổi thì ngôi sao vẫn phải ĐỌC ĐƯỢC: dòng nào đang là con số tổng là
            // thông tin ai xem bảng cũng cần, kể cả người không đổi được nó.
            laDongTong && (
              <span title={HEADLINE_LINE_LABEL} className="inline-flex shrink-0 text-tangerine">
                <Glyph icon={Star} className="size-3.5 fill-current" />
                <span className="an-thi-giac">{HEADLINE_LINE_LABEL}</span>
              </span>
            )
          )}
        </span>
      </td>
      {/* TT in the body font, muted (prototype `:523`); an empty TT is an empty cell. */}
      <td className="px-3 py-1.5 align-top text-ink-muted">{d.no}</td>
      {/* Thụt lề theo cấp NGAY TRONG cột Nội dung (§4.1, prototype `0.75 + depth × 1.25rem`).
          `paddingInlineStart` (không phải `paddingLeft`) để không hỏng nếu giao diện có ngày chạy ở
          chiều ngược lại. */}
      <td
        className={cn("px-3 py-1.5 align-top", heading && "font-semibold text-navy")}
        style={{ paddingInlineStart: `${0.75 + hien.cap * 1.25}rem` }}
      >
        {open !== null && open.field === NAME_FIELD ? (
          <CellInput
            label={`Tên khoản mục ${d.name}`}
            cell={open}
            onDraft={cell.setDraft}
            onSave={() => void cell.commit(cellNameEdit(d, open.draft))}
            onCancel={cell.close}
          />
        ) : coGhi ? (
          <button
            type="button"
            className={cn(EDITABLE_TEXT, "text-left")}
            disabled={editDisabled}
            title={lockTitle ?? NHAN_SUA_TEN}
            onClick={() => cell.start(NAME_FIELD, d.name)}
          >
            {d.name}
          </button>
        ) : (
          d.name
        )}
      </td>

      {cot.map((c) => {
        if (c.type === "so") {
          const value = d.values[c.id] ?? null;
          const shown = formatSheetAmount(value, donVi.ma);
          const lyDo = lyDoKhongTinh(d.unavailable_reasons?.[c.id]);
          const field = `value:${c.id}`;
          return (
            <td
              key={c.id}
              className={cn(
                "px-3 py-1.5 text-right align-top tabular-nums",
                heading && "font-semibold",
                computed && lyDo === null && "text-leaf",
              )}
            >
              {open !== null && open.field === field ? (
                <CellInput
                  label={`${c.name} của ${d.name}`}
                  cell={open}
                  numeric
                  onDraft={cell.setDraft}
                  onSave={() => void cell.commit(cellValueEdit(d, c, open.draft, donVi.ma))}
                  onCancel={cell.close}
                />
              ) : coGhi && suaDuocOSo(d.method) ? (
                // Only a `manual` row: the others' figures are sums the server computes, and it
                // refuses a typed one (409). The box opens with the EXACT figure, not the rounded one.
                <button
                  type="button"
                  className={cn(EDITABLE_TEXT, "w-full text-right tabular-nums")}
                  aria-label={nhanSuaO(c.name, d.name, lyDo === null ? shown : O_KHONG_TINH_DUOC)}
                  disabled={editDisabled}
                  title={lockTitle}
                  onClick={() => cell.start(field, cellValueDraft(d, c.id, donVi.ma))}
                >
                  <OTien chu={shown} lyDo={lyDo} />
                </button>
              ) : (
                <OTien chu={shown} lyDo={lyDo} />
              )}
            </td>
          );
        }
        // Cột phần trăm: tỷ lệ MÁY CHỦ tính cho dòng này (`percent_basis_points`), không chia lại ở
        // đây. Không có tỷ lệ thì máy chủ gửi câu lý do, và ô vẽ đúng dấu "Không tính được".
        const pc = percentCellRounded(d, c.id);
        return (
          <td key={c.id} className={cn("px-3 py-1.5 text-right align-top tabular-nums", heading && "font-semibold")}>
            <OTien chu={pc.text} lyDo={pc.reason} />
          </td>
        );
      })}

      {/* `Cách tính` holds the row's controls, as in the prototype: the mode, then the entries (⇄),
          add a child, remove — and, last, TT / display order. Icon-only, each with its full name as
          `aria-label` and tooltip — or, under a year close, the lock reason as the tooltip. */}
      <td className="px-3 py-1.5 align-top">
        <span className="flex items-center gap-1">
          {coGhi ? (
            // CHỌN KHÔNG GỬI NGAY: đổi cách tính đổi con số đang hiện, nên lựa chọn mở một hộp cảnh
            // báo (`FormDoiCachTinh`) và chỉ gửi khi cán bộ xác nhận. Ô chọn vẫn hiện chế độ ĐANG LƯU
            // cho tới khi bảng được đọc lại.
            <select
              aria-label={`Cách tính của ${d.name}`}
              value={isParent ? "children" : d.method}
              disabled={isParent || editDisabled}
              title={isParent ? PARENT_SUMS_CHILDREN : lockTitle}
              className={METHOD_SELECT}
              onChange={(e) => {
                const den = e.target.value;
                if ((den === "manual" || den === "entries") && den !== d.method) doiCachTinh(den);
              }}
            >
              {CACH_TINH_CHON.map((o) => (
                // `children` is never sent (400): shown, not choosable, on a leaf.
                <option key={o.ma} value={o.ma} disabled={o.ma === "children" && !isParent}>
                  {o.nhan}
                </option>
              ))}
            </select>
          ) : (
            <span className="min-w-0 flex-1 text-[11.5px] text-ink-muted">{nhanCachTinh(d.method)}</span>
          )}
          <RowIconButton
            icon={ListPlus}
            label={isParent ? PARENT_SUMS_CHILDREN : nhanNutDot(d.name)}
            title={isParent ? PARENT_SUMS_CHILDREN : nhanNutDot(d.name)}
            disabled={isParent || dangGui}
            onClick={moDot}
          />
          {coGhi && (
            <RowIconButton
              icon={Plus}
              label={nhanThemCon(d.name)}
              title={lockTitle ?? nhanThemCon(d.name)}
              disabled={editDisabled}
              onClick={them}
            />
          )}
          {coXacNhan && (
            <RowIconButton
              icon={Trash2}
              danger
              label={nhanGoKhoanMuc(d.name)}
              title={lockTitle ?? nhanGoKhoanMuc(d.name)}
              disabled={editDisabled}
              onClick={go}
            />
          )}
          {coGhi && (
            <RowIconButton
              icon={Pencil}
              label={`${LINE_ORDER_TITLE} của ${d.name}`}
              title={lockTitle ?? LINE_ORDER_TITLE}
              disabled={editDisabled}
              onClick={openLineOrder}
            />
          )}
        </span>
      </td>
    </tr>
  );
}

/**
 * The TT / display-order dialog of one line — the two fields of the former row form that the in-cell
 * edit does not cover, with the same `Field` + `o-nhap` inputs. Sends only what changed
 * (`lineOrderBody`); the server's refusal is printed inside.
 */
export function LineOrderDialog({
  dong,
  dangGui,
  huy,
  luu,
  serverError = null,
}: {
  dong: finance_dongRa;
  dangGui: boolean;
  huy: () => void;
  luu: (body: SuaDongVao) => void;
  serverError?: string | null;
}) {
  const [error, setError] = useState<string | null>(null);
  const shown = error ?? serverError;
  return (
    <ModalDialog
      titleId={LINE_ORDER_TITLE_ID}
      closeDisabled={dangGui}
      onDismiss={() => {
        if (!dangGui) huy();
      }}
    >
      <ModalDialogHeader titleId={LINE_ORDER_TITLE_ID} title={LINE_ORDER_TITLE} description={dong.name} />
      <form
        aria-labelledby={LINE_ORDER_TITLE_ID}
        className="flex min-h-0 min-w-0 flex-col gap-4"
        onSubmit={(e: FormEvent<HTMLFormElement>) => {
          e.preventDefault();
          const fd = new FormData(e.currentTarget);
          const built = lineOrderBody(dong, String(fd.get("no") ?? ""), String(fd.get("order") ?? ""));
          if (!built.ok) {
            setError(built.thongBao);
            return;
          }
          setError(null);
          luu(built.than);
        }}
      >
        <div className="grid min-w-0 gap-4 sm:grid-cols-2">
          <Field label="Số thứ tự (TT)" htmlFor="line-order-no" grow="auto">
            <input id="line-order-no" name="no" className="o-nhap" type="text" maxLength={32} defaultValue={dong.no} />
          </Field>
          <Field label="Thứ tự hiển thị" htmlFor="line-order-order" grow="auto">
            <input
              id="line-order-order"
              name="order"
              className="o-nhap"
              type="number"
              step={1}
              required
              defaultValue={dong.order}
            />
          </Field>
        </div>
        {shown !== null && (
          <p className="thong-bao-loi m-0" role="alert">
            {shown}
          </p>
        )}
        <div className="flex shrink-0 flex-wrap justify-end gap-2">
          <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
            Huỷ
          </Button>
          <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui || undefined}>
            <BusyLabel busy={dangGui} label="Lưu" busyText={BUSY_SAVING} />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}

/** Heading id of the TT / display-order dialog. */
const LINE_ORDER_TITLE_ID = "tieu-de-thu-tu-khoan-muc";

/**
 * Biểu mẫu thêm một khoản mục — dùng cho cả `Thêm khoản mục cấp cao nhất` lẫn `＋` dòng con. A centred
 * dialog, where the prototype asks with `window.prompt` (`Tên khoản mục cấp cao nhất:` / `Tên khoản
 * mục con:`); `tenCha` empty means a top-level line.
 */
export function FormThemKhoanMuc({
  tenCha,
  thuTuGoiY,
  dangGui,
  huy,
  luu,
  serverError = null,
}: {
  tenCha: string;
  thuTuGoiY: number;
  dangGui: boolean;
  luu: (no: string, ten: string, thuTu: number, khoaChongTrung: string) => void;
  huy: () => void;
  /** The server's refusal of the last submit, verbatim — shown inside the dialog. */
  serverError?: string | null;
}) {
  /**
   * Khoá chống trùng sinh MỘT LẦN lúc mở biểu mẫu, không lúc gửi.
   *
   * Sinh lúc gửi thì mỗi lần bấm là một khoá mới, tức là không chống được gì — mà tuyến này đòi
   * khoá đúng vì hai dòng cùng tên dưới một cha là chuyện có thật trong biểu mẫu ngân sách.
   */
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  return (
    <ModalDialog
      titleId={ADD_LINE_TITLE_ID}
      onDismiss={() => {
        if (!dangGui) huy();
      }}
    >
      <ModalDialogHeader
        titleId={ADD_LINE_TITLE_ID}
        title={tenCha === "" ? "Thêm khoản mục cấp cao nhất" : nhanThemCon(tenCha)}
        description={
          "Khoản mục mới bắt đầu ở Nhập trực tiếp. Khi chưa có khoản mục con, có thể đổi sang Cộng " +
          "theo đợt ở cột Cách tính; khi có khoản mục con đầu tiên, nó thành dòng cộng con."
        }
      />
      <form
        aria-labelledby={ADD_LINE_TITLE_ID}
        className="flex min-h-0 min-w-0 flex-col gap-4"
        onSubmit={(e: FormEvent<HTMLFormElement>) => {
          e.preventDefault();
          const fd = new FormData(e.currentTarget);
          luu(
            String(fd.get("no") ?? ""),
            String(fd.get("name") ?? ""),
            Number(fd.get("order") ?? thuTuGoiY),
            khoaChongTrung,
          );
        }}
      >
        <div className="grid min-h-0 min-w-0 gap-4 overflow-y-auto sm:grid-cols-2">
          <Field label="Nội dung khoản mục" htmlFor="them-name" grow="auto" className="sm:col-span-2">
            <input id="them-name" name="name" className="o-nhap" type="text" required />
          </Field>
          <Field label="Số thứ tự (TT)" htmlFor="them-no" grow="auto">
            <input id="them-no" name="no" className="o-nhap" type="text" maxLength={32} />
          </Field>
          <Field label="Thứ tự hiển thị" htmlFor="them-order" grow="auto">
            <input
              id="them-order"
              name="order"
              className="o-nhap"
              type="number"
              step={1}
              defaultValue={thuTuGoiY}
            />
          </Field>
          {serverError !== null && (
            <p className="thong-bao-loi m-0 sm:col-span-2" role="alert">
              {serverError}
            </p>
          )}
        </div>
        <div className="flex shrink-0 flex-wrap justify-end gap-2">
          <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
            Huỷ
          </Button>
          <Button
            type="submit"
            variant="primary"
            icon={<Glyph icon={Plus} />}
            disabled={dangGui}
            aria-busy={dangGui || undefined}
          >
            <BusyLabel busy={dangGui} label="Thêm khoản mục" busyText={BUSY_SAVING} />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}

/** Heading id of the add-line dialog. */
const ADD_LINE_TITLE_ID = "tieu-de-them-khoan-muc";

/**
 * Hộp xác nhận ĐỔI CÁCH TÍNH (§4.2) — nói đúng điều máy chủ sẽ làm với con số trước khi gửi.
 *
 * Không có ô lý do: đây là một lần sửa (`budget.update`), không phải một lần gỡ, và máy chủ ghi
 * vết của nó như mọi lần sửa khoản mục.
 */
export function FormDoiCachTinh({
  ten,
  den,
  dangGui,
  huy,
  luu,
  formId,
  className,
}: {
  ten: string;
  den: CachTinhChon;
  dangGui: boolean;
  huy: () => void;
  luu: () => void;
  /** `id` of the `<form>` — a surrounding `ModalDialog` names itself after it. */
  formId?: string;
  className?: string;
}) {
  // A confirm box (spec v2 §7): the specific question, the consequence, the button naming the act.
  // Not red — changing how a line is computed destroys nothing; the consequence sentence says what.
  return (
    <ConfirmDialog
      as="form"
      id={formId}
      className={className}
      icon={Calculator}
      title={`Đổi cách tính của ${ten} sang ${nhanCachTinh(den)}?`}
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        luu();
      }}
      actions={
        <>
          <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui || undefined}>
            <BusyLabel busy={dangGui} label="Đổi cách tính" busyText={BUSY_SAVING} />
          </Button>
          <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
            Huỷ
          </Button>
        </>
      }
    >
      <p className="m-0">{canhBaoDoiCachTinh(den)}</p>
    </ConfirmDialog>
  );
}
