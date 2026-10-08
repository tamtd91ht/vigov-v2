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
  TableProperties,
  Trash2,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { useEffect, useState, type FormEvent, type ReactNode } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import { Skeleton, SkeletonRows } from "@/components/ui/skeleton";
import { Tab, TabList } from "@/components/ui/tabs";
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
  chonDuocCachTinh,
  cotSo,
  donViCuaBang,
  dongSangChuoi,
  dungCay,
  dungGiaSuaDong,
  GHI_CHU_CHENH_LECH,
  isUnresolvedPercentColumn,
  lyDoKhongTinh,
  moiDongCoCon,
  NHAN_CHENH_LECH,
  nhanBoDem,
  nhanCachTinh,
  nhanChiSo,
  nhanChuaCoBang,
  nhanDatDongTong,
  nhanGoKhoanMuc,
  nhanNutDot,
  nhanSoTien,
  nhanSoTienChiSo,
  nhanTab,
  nhanSuaO,
  nhanThemCon,
  NHAN_SUA_TEN,
  O_KHONG_TINH_DUOC,
  O_TRONG,
  percentCell,
  phangCay,
  subtitleParts,
  suaDuocOSo,
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
 * đề, dòng phụ, câu dòng tổng, các ô số) → thanh công cụ của cây → bảng cây. Thêm, sửa bảng, gỡ, đổi
 * cách tính, các đợt là HỘP THOẠI như prototype.
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
  const [dangSuaDong, datDangSuaDong] = useState<string | null>(null);
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
    datDangSuaDong(null);
    datLanTai((n) => n + 1);
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
    datDangSuaDong(null);
    datLoiGhi(null);
  }

  return (
    <section className="man-giai-ngan flex min-w-0 flex-col gap-3" aria-labelledby="tieu-de-thu-chi">
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
        {/* A write refused while NO dialog is open (the in-place row edit, the headline star). */}
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
            <CardContent className="flex flex-col gap-4">
              {/* KHÔNG ĐOÁN VÌ SAO KHÔNG ĐỌC ĐƯỢC: hiện NGUYÊN câu máy chủ. 404 ở tuyến này thường
                  là "xã chưa lập bảng", nhưng phân biệt bằng cách đọc câu chữ là dựng lại đúng thứ
                  `goi.ts` cố ý giấu đi — nên câu dưới nói cả hai khả năng và không khẳng định.
                  The server's sentence (e.g. "ngan_sach: không có bảng ngân sách này trong xã") is
                  shown VERBATIM per ADR 0035 §A — a muted note, not a red alarm, because the usual
                  cause is "not set up yet". Rewording it is a separate, open decision. */}
              <Notice tone="neutral" icon={Database} role="alert">
                {bang.thongBao}
              </Notice>
              <EmptyState
                icon={TableProperties}
                tone="neutral"
                title={nhanChuaCoBang(nam, loai)}
                className="py-4"
              />
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
            dangSuaDong={dangSuaDong}
            moSua={(id) => {
              datDangSuaDong(id);
              datDangMo(null);
            }}
            huySua={() => datDangSuaDong(null)}
            luuSua={(id, than) => {
              datDangGui(true);
              suaKhoanMuc(id, than).then(xong);
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
          method={dangMo.dong.method}
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
 * The prototype's first card (`FiscalReportPanel.tsx:159-234`): which sheet is on screen on the left,
 * the sheet's own actions on the right.
 *
 * LEFT — the year select, then the two sheets of that year as segment buttons (`Chi ngân sách 2026`,
 * `Thu ngân sách 2026`). The prototype lists the IMPORTED reports and has no year (the file carries
 * it); this contract reads one sheet per year and kind, so the year is a control and both kinds are
 * always offered — a kind with no sheet yet opens on its empty state.
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
        <TabList aria-label="Chọn bảng thu hoặc bảng chi" className="gap-1.5">
          {(["chi", "thu"] as const).map((l) => (
            <Tab
              key={l}
              id={`tab-ngan-sach-${l}`}
              selected={kind === l}
              aria-controls={`bang-ngan-sach-${l}`}
              className="h-8 px-3 text-[13px]"
              onClick={() => onKindChange(l)}
            >
              {nhanTab(l, year)}
            </Tab>
          ))}
        </TabList>
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
            // The prototype's word is `Gỡ`; the accessible name says WHAT is removed.
            <Button
              type="button"
              variant="danger"
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
 * đầy đủ kèm chữ "đồng" nằm ở `title` của ô.
 */
export function TheChiSoNam({ chiSo }: { chiSo: finance_chiSoNamRa }) {
  const balance = chiSo.balance;
  const balanceText = nhanSoTienChiSo(balance, "dong");
  return (
    <Card className="flex min-w-0 flex-col gap-2 px-4 py-3">
      <h3 className="m-0 inline-flex min-w-0 items-center gap-2 text-[15px] leading-snug font-bold text-ink-900">
        <Glyph icon={Gauge} className="size-[18px] shrink-0 text-brand-600" />
        Chỉ số ngân sách năm {chiSo.year}
      </h3>
      <dl className={FIGURE_TILES}>
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
      <p className="m-0 inline-flex items-start gap-1.5 text-xs text-ink-500">
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
 * One tile, in the prototype's order: label (small, muted) above the figure (bold). The figure's size
 * follows the TILE's width (`cqi` of the `@container` tile, never `vw`), so four tiles on a narrow
 * main column never clip a sum. Tiles of a row are equal in height (grid stretch). `exact` is the full
 * amount, on hover of the whole tile.
 *
 * `<dd>` CARRIES NO ATTRIBUTE: the tests read `<dd>…</dd>` back, and a "—" must stay a bare "—".
 */
function FigureTile({ label, exact, children }: { label: string; exact?: string; children: ReactNode }) {
  return (
    <div
      title={exact}
      className="@container flex min-w-0 flex-col gap-1 rounded-control border border-line bg-surface-muted px-3 py-2"
    >
      <dt title={label} className="truncate text-xs text-ink-500">
        {label}
      </dt>
      <dd className="m-0 min-w-0 text-[clamp(15px,10cqi,18px)] leading-tight font-bold text-ink-900 tabular-nums">
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

/** Thẻ báo cáo (§2) + thanh công cụ (§4.3) + bảng cây (§4.1), in the prototype's order. */
export function BangDayDu({
  duLieu,
  thuGon,
  datThuGon,
  coGhi,
  coXacNhan,
  sheetLock,
  dangGui,
  dangSuaDong,
  moSua,
  huySua,
  luuSua,
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
  dangSuaDong: string | null;
  moSua: (id: string) => void;
  huySua: () => void;
  luuSua: (id: string, than: SuaDongVao) => void;
  moThem: (chaId: string, tenCha: string, thuTuGoiY: number) => void;
  moGoDong: (dong: finance_dongRa) => void;
  datTong: (dong: finance_dongRa) => void;
  moCachTinh: (dong: finance_dongRa, den: CachTinhChon) => void;
  moDot: (dong: finance_dongRa) => void;
}) {
  const cay = dungCay(duLieu.lines);
  const dongHien = phangCay(cay, thuGon);
  const cot = cotSo(duLieu.columns);
  const donVi = donViCuaBang(duLieu.sheet);
  const locked = sheetLock !== null;
  const lockTitle = sheetLock ?? undefined;
  const headlineId = duLieu.summary.headline_line_id ?? "";
  const headlineName = duLieu.lines.find((l) => l.id === headlineId)?.name;

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
        soKhoanMuc={duLieu.lines.length}
        donVi={donVi}
        headlineName={headlineName}
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

      {/* The table scrolls sideways INSIDE its own box (`overflow-x: auto`); amounts stay whole,
          right-aligned and tabular — never shortened here, this is where a figure is checked. */}
      <TableScroll sticky aria-label={`Khoản mục của ${duLieu.sheet.title}`}>
        <table className={cn("bang-danh-muc", DATA_TABLE_CLASS)}>
          <caption className="an-thi-giac">{duLieu.sheet.title}</caption>
          <thead>
            <tr>
              <th scope="col" className="w-px px-1">
                <span className="an-thi-giac">Mở, thu gọn và dòng tổng</span>
              </th>
              <th scope="col">TT</th>
              <th scope="col">Nội dung</th>
              {duLieu.columns.map((c) => (
                // Figure columns right-aligned so digits line up down the column (spec §6.7).
                <th key={c.id} scope="col" className="text-right">
                  {c.name}
                  {/* Chú thích công thức của cột `%` — chữ HIỆN RÕ chứ không chỉ `title`, vì màn
                      cảm ứng không rê chuột được. Chỉ là chữ: tỷ lệ tính từ hai toán hạng, không
                      từ chuỗi này. */}
                  {c.type === "phan_tram" && (c.formula ?? "").trim() !== "" && (
                    <>
                      <br />
                      <span className="ghi-chu font-normal">{c.formula}</span>
                    </>
                  )}
                </th>
              ))}
              <th scope="col">Cách tính</th>
            </tr>
          </thead>
          <tbody>
            {dongHien.map((d) =>
              dangSuaDong === d.dong.id ? (
                <FormSuaDong
                  key={d.dong.id}
                  dong={d.dong}
                  cot={cot}
                  donVi={donVi}
                  soCotBang={duLieu.columns.length + 4}
                  dangGui={dangGui}
                  huy={huySua}
                  luu={luuSua}
                />
              ) : (
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
                  moSua={() => moSua(d.dong.id)}
                  them={() =>
                    moThem(d.dong.id, d.dong.name, thuTuKeTiep(duLieu.lines, d.dong.id))
                  }
                  go={() => moGoDong(d.dong)}
                  datTong={() => datTong(d.dong)}
                  doiCachTinh={(den) => moCachTinh(d.dong, den)}
                  moDot={() => moDot(d.dong)}
                />
              ),
            )}
          </tbody>
        </table>
      </TableScroll>
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
 * Thẻ báo cáo của §2 — the prototype's second card (`FiscalReportPanel.tsx:240-282`): title, the
 * sub-line (`Đơn vị tính` · `Luỹ kế đến` · `N khoản mục` · source file), the headline sentence naming
 * the starred line, then one tile per figure.
 *
 * `unavailable_reason` CỦA THẺ TÓM TẮT LÀ MỘT CÂU, KHÔNG PHẢI MỘT Ô TRỐNG. Chưa ai đánh dấu dòng
 * tổng, hoặc hai dòng cùng nhận là dòng tổng, thì không có gì để đọc — và câu của máy chủ nói ra
 * việc phải làm. Điền đại bằng "dòng đầu tiên" là đúng cái đoán mà `is_headline` sinh ra để từ
 * chối: bảng thu có hai dòng cấp cao lồng nhau, bảng chi có `Tổng số` đứng ngang hàng A…E.
 *
 * SỐ TIỀN Ở Ô IN DẠNG GỌN (`compactDong`); con số đầy đủ theo đơn vị của bảng nằm ở `title` của ô và
 * ở bảng bên dưới.
 */
export function TheTomTat({
  bang,
  tomTat,
  soKhoanMuc,
  donVi,
  headlineName,
}: {
  bang: finance_bangRa;
  tomTat: finance_tomTatRa;
  soKhoanMuc: number;
  donVi: DonViHien;
  /** Name of the starred line, when it is among the lines read. */
  headlineName?: string;
}) {
  const sourceFile = (bang.source_file ?? "").trim();
  const reason = (tomTat.unavailable_reason ?? "").trim();
  return (
    <Card className="flex min-w-0 flex-col gap-1 px-4 py-3">
      <h3 className="m-0 min-w-0 text-[15px] leading-snug font-bold break-words text-ink-900">{bang.title}</h3>
      <p className="m-0 flex min-w-0 flex-wrap gap-x-3 gap-y-0.5 text-xs text-ink-500 tabular-nums">
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
      <p className="m-0 text-xs text-ink-500">{cauQuyDoi(donVi)}</p>

      {reason !== "" ? (
        <EmptyState icon={Star} tone="neutral" title={reason} className="py-4" />
      ) : (
        <>
          <p className="m-0 mt-2 text-xs text-ink-500">
            {headlineName !== undefined ? (
              <>
                Con số tổng lấy từ dòng <span className="font-semibold text-ink-900">{headlineName}</span>. Bấm
                ngôi sao ở đầu một dòng khác để đổi.
              </>
            ) : (
              "Con số tổng lấy từ dòng được đánh sao. Bấm ngôi sao ở đầu một dòng khác để đổi."
            )}
          </p>
          <dl className={FIGURE_TILES}>
            {tomTat.cells.map((o) => {
              const cellReason = lyDoKhongTinh(o.unavailable_reason);
              const text = nhanSoTien(o.value, donVi.ma);
              return (
                <FigureTile
                  key={o.column_id}
                  label={o.name}
                  exact={
                    cellReason === null && o.value !== null && Number.isSafeInteger(o.value)
                      ? `${text} ${donVi.nhan.toLowerCase()}`
                      : undefined
                  }
                >
                  {cellReason === null ? (
                    compactOrText(o.value, text)
                  ) : (
                    <OTien chu={text} lyDo={cellReason} hienLyDo />
                  )}
                </FigureTile>
              );
            })}
            <OChiSo chi={tomTat.indicator} />
          </dl>
        </>
      )}
    </Card>
  );
}

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
  moSua,
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
  moSua: () => void;
  them: () => void;
  go: () => void;
  datTong: () => void;
  doiCachTinh: (den: CachTinhChon) => void;
  moDot: () => void;
}) {
  const d = hien.dong;
  const laDongTong = d.id === dongTongId;
  // Dòng LÁ: không có con trên cây đang vẽ và máy chủ không nói nó cộng con. Chỉ dòng lá có ô
  // chọn cách tính và có hộp đợt — dòng có con thì cả hai là 409 (`routes.go:1081`, `:1204`).
  const laLa = !hien.coCon && d.method !== "children";
  const editDisabled = dangGui || sheetLock !== null;
  const lockTitle = sheetLock ?? undefined;
  // The form's two top levels are its headings: tinted and bold, as on the paper form (prototype
  // `depth <= 1`). Officers check the screen against the printed copy.
  const heading = hien.cap <= 1;
  // A figure the SERVER computes (sum of the children, or of the entries) is green, as the
  // prototype's `text-leaf` — the cue that typing into it is refused.
  const computed = d.method !== "manual";

  return (
    <tr className={heading ? "bg-surface-muted" : undefined}>
      <td className="px-1 align-top">
        <span className="inline-flex items-center gap-0.5">
          {hien.coCon ? (
            <button type="button" className={TREE_ICON_BUTTON} aria-expanded={hien.moRong} onClick={moRongDoi}>
              <Glyph icon={hien.moRong ? ChevronDown : ChevronRight} className="size-4" />
              <span className="an-thi-giac">
                {hien.moRong ? `Thu gọn ${d.name}` : `Mở chi tiết ${d.name}`}
              </span>
            </button>
          ) : (
            // Same width as the toggle, so the stars line up down the column.
            <span aria-hidden="true" className="inline-block w-7 shrink-0" />
          )}
          {/* The star sits on TOP-LEVEL lines only, as in the prototype: the headline is one of the
              form's top rows (`Tổng số`, `Tổng thu nội địa`). A nested line already starred still
              shows its read-only mark. */}
          {coXacNhan && hien.cap === 0 ? (
            <button
              type="button"
              className={TREE_ICON_BUTTON}
              aria-label={nhanDatDongTong(d.name)}
              aria-pressed={laDongTong}
              disabled={editDisabled}
              title={lockTitle ?? nhanDatDongTong(d.name)}
              onClick={datTong}
            >
              <Glyph
                icon={Star}
                className={cn("size-4", laDongTong ? "fill-current text-warning-600" : "text-ink-400")}
              />
            </button>
          ) : (
            // Không có quyền đổi thì ngôi sao vẫn phải ĐỌC ĐƯỢC: dòng nào đang là con số tổng là
            // thông tin ai xem bảng cũng cần, kể cả người không đổi được nó.
            laDongTong && (
              <span title={HEADLINE_LINE_LABEL} className="inline-flex w-7 shrink-0 justify-center">
                <Glyph icon={Star} className="size-4 fill-current text-warning-600" />
                <span className="an-thi-giac">{HEADLINE_LINE_LABEL}</span>
              </span>
            )
          )}
        </span>
      </td>
      <td className="ma-muc align-top text-[13px] text-ink-500">{d.no === "" ? O_TRONG : d.no}</td>
      {/* Thụt lề theo cấp NGAY TRONG cột Nội dung (§4.1, prototype `0.75 + depth × 1.25rem`).
          `paddingInlineStart` (không phải `paddingLeft`) để không hỏng nếu giao diện có ngày chạy ở
          chiều ngược lại. The name wraps (min 22rem), so a long item never widens the table forever. */}
      <td
        className={cn("min-w-[22rem] align-top whitespace-normal", heading && "font-semibold text-ink-900")}
        style={{ paddingInlineStart: `${0.75 + hien.cap * 1.25}rem` }}
      >
        {coGhi ? (
          // The name IS the edit control (§4.1, `NHAN_SUA_TEN`): drawn as text with a link colour,
          // not as a bordered button on every row.
          <button
            type="button"
            className="cursor-pointer rounded-sm border-0 bg-transparent p-0 text-left [font-family:inherit] text-[length:inherit] [font-weight:inherit] text-brand-700 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500 disabled:cursor-not-allowed disabled:text-ink-700 disabled:no-underline"
            aria-label={NHAN_SUA_TEN}
            disabled={editDisabled}
            title={lockTitle}
            onClick={moSua}
          >
            {d.name}
          </button>
        ) : (
          d.name
        )}
      </td>

      {cot.map((c) => {
        if (c.type === "so") {
          const chu = nhanSoTien(d.values[c.id] ?? null, donVi.ma);
          const lyDo = lyDoKhongTinh(d.unavailable_reasons?.[c.id]);
          return (
            <td
              key={c.id}
              className={cn(
                "text-right align-top tabular-nums",
                heading && "font-semibold",
                computed && lyDo === null && "text-success-600",
              )}
            >
              {coGhi && suaDuocOSo(d.method) ? (
                // §4.1 "Các ô số — button, bấm để sửa tại chỗ" (NS-01): opens the SAME in-place form as
                // the name, where every figure of the row is a box. Only a `manual` row: the others'
                // figures are sums the server computes, and it refuses a typed one (409).
                <button
                  type="button"
                  className="cursor-pointer rounded-sm border-0 bg-transparent p-0 text-right [font-family:inherit] text-[length:inherit] [font-weight:inherit] text-brand-700 tabular-nums hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500 disabled:cursor-not-allowed disabled:text-ink-700 disabled:no-underline"
                  aria-label={nhanSuaO(c.name, d.name, lyDo === null ? chu : O_KHONG_TINH_DUOC)}
                  disabled={editDisabled}
                  title={lockTitle}
                  onClick={moSua}
                >
                  <OTien chu={chu} lyDo={lyDo} />
                </button>
              ) : (
                <OTien chu={chu} lyDo={lyDo} />
              )}
            </td>
          );
        }
        // Cột phần trăm: tỷ lệ MÁY CHỦ tính cho dòng này (`percent_basis_points`), không chia lại ở
        // đây. Không có tỷ lệ thì máy chủ gửi câu lý do, và ô vẽ đúng dấu "Không tính được".
        const cell = percentCell(d, c.id);
        return (
          <td key={c.id} className={cn("text-right align-top tabular-nums", heading && "font-semibold")}>
            <OTien chu={cell.text} lyDo={cell.reason} />
          </td>
        );
      })}

      {/* `Cách tính` holds the row's controls, as in the prototype: the mode, then the entries (⇄),
          add a child, remove. Icon-only, each with its full name as `aria-label` and tooltip — or,
          under a year close, the lock reason as the tooltip. */}
      <td className="nhan-trong align-top">
        <span className="flex items-center gap-1">
          {coGhi && chonDuocCachTinh(d.method, hien.coCon) ? (
            // CHỌN KHÔNG GỬI NGAY: đổi cách tính đổi con số đang hiện, nên lựa chọn mở một hộp cảnh
            // báo (`FormDoiCachTinh`) và chỉ gửi khi cán bộ xác nhận. Ô chọn vẫn hiện chế độ ĐANG LƯU
            // cho tới khi bảng được đọc lại.
            //
            // COMPACT 32px INSIDE THE CELL: the global select frame is 40px (`globals.css`), which
            // made this one row taller than every other. Utilities win over the legacy layer.
            <select
              aria-label={`Cách tính của ${d.name}`}
              value={d.method}
              disabled={editDisabled}
              title={lockTitle}
              className="h-8 min-h-8 py-0 pr-8 pl-2.5 text-[13px] not-italic"
              onChange={(e) => {
                const den = e.target.value;
                if ((den === "manual" || den === "entries") && den !== d.method) doiCachTinh(den);
              }}
            >
              {CACH_TINH_CHON.map((c) => (
                <option key={c.ma} value={c.ma}>
                  {c.nhan}
                </option>
              ))}
            </select>
          ) : (
            <span className="min-w-0">{nhanCachTinh(d.method)}</span>
          )}
          {laLa && (
            <Button
              type="button"
              variant="icon"
              size="sm"
              aria-label={nhanNutDot(d.name)}
              title={nhanNutDot(d.name)}
              disabled={dangGui}
              onClick={moDot}
            >
              <Glyph icon={ListPlus} />
            </Button>
          )}
          {coGhi && (
            <Button
              type="button"
              variant="icon"
              size="sm"
              aria-label={nhanThemCon(d.name)}
              disabled={editDisabled}
              title={lockTitle ?? nhanThemCon(d.name)}
              onClick={them}
            >
              <Glyph icon={Plus} />
            </Button>
          )}
          {coXacNhan && (
            <Button
              type="button"
              variant="icon"
              size="sm"
              className="hover:not-disabled:bg-danger-50 hover:not-disabled:text-danger-600"
              aria-label={nhanGoKhoanMuc(d.name)}
              disabled={editDisabled}
              title={lockTitle ?? nhanGoKhoanMuc(d.name)}
              onClick={go}
            >
              <Glyph icon={Trash2} />
            </Button>
          )}
        </span>
      </td>
    </tr>
  );
}

/** "Dòng đang là con số tổng" — the read-only star's tooltip and accessible name. */
const HEADLINE_LINE_LABEL = "Dòng đang là con số tổng";

/** The two small in-cell toggles of the tree (expand, headline star): 28px, no frame. */
const TREE_ICON_BUTTON = cn(
  "inline-flex size-7 shrink-0 cursor-pointer items-center justify-center rounded-md border-0 bg-transparent p-0 text-ink-500",
  "hover:not-disabled:bg-brand-50 hover:not-disabled:text-brand-700",
  "focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-brand-500",
  "disabled:cursor-not-allowed disabled:opacity-60",
);

/**
 * Dòng đang sửa: TT, tên, thứ tự và các ô số của một dòng.
 *
 * MỘT BIỂU MẪU KHÔNG KIỂM SOÁT (`defaultValue` + `FormData`), không phải một `useState` cho mỗi ô.
 * Bảng này có tới hàng chục dòng và mỗi dòng có tới bốn ô số; dựng state cho từng ô là dựng lại
 * đúng thứ trình duyệt đã làm sẵn, và React Compiler đang bật thì mỗi lần gõ một ký tự sẽ dựng
 * lại cả bảng.
 *
 * Ô SỐ CHỈ MỞ VỚI DÒNG KHÔNG CÓ CON. Máy chủ trả 409 kèm quy tắc của khách; khoá ô ở đây chỉ để
 * cán bộ không gõ xong rồi mới bị từ chối.
 */
export function FormSuaDong({
  dong,
  cot,
  donVi,
  soCotBang,
  dangGui,
  huy,
  luu,
}: {
  dong: finance_dongRa;
  /** CHỈ cột `so` — cột phần trăm không lưu giá trị nào (§9 quy tắc 3). */
  cot: readonly finance_cotRa[];
  donVi: DonViHien;
  soCotBang: number;
  dangGui: boolean;
  huy: () => void;
  luu: (id: string, than: SuaDongVao) => void;
}) {
  const [loiO, datLoiO] = useState<string | null>(null);
  const moO = suaDuocOSo(dong.method);

  return (
    <tr>
      {/* The row under edit opens in place, as a tinted panel spanning the table (§4.1). */}
      <td colSpan={soCotBang} className="bg-brand-50/40 whitespace-normal">
        <form
          aria-label={`Sửa khoản mục ${dong.name}`}
          className="flex min-w-0 flex-col gap-4 py-2"
          onSubmit={(e) => {
            e.preventDefault();
            const fd = new FormData(e.currentTarget);
            const than: SuaDongVao = {
              no: String(fd.get("no") ?? ""),
              name: String(fd.get("name") ?? ""),
              order: Number(fd.get("order") ?? dong.order),
            };

            if (moO) {
              const tho: Record<string, string> = {};
              for (const c of cot) tho[c.id] = String(fd.get(`gia:${c.id}`) ?? "");
              const gia = dungGiaSuaDong(tho, cot, dong.unavailable_reasons, donVi.ma);
              if (!gia.ok) {
                datLoiO(gia.thongBao);
                return;
              }
              than.values = gia.than;
            }

            datLoiO(null);
            luu(dong.id, than);
          }}
        >
          <div className="grid min-w-0 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Field label="Số thứ tự (TT)" htmlFor={`sua-no-${dong.id}`} grow="auto">
              <input
                id={`sua-no-${dong.id}`}
                name="no"
                className="o-nhap"
                type="text"
                defaultValue={dong.no}
                maxLength={32}
              />
            </Field>
            <Field label="Nội dung khoản mục" htmlFor={`sua-name-${dong.id}`} grow="auto">
              <input
                id={`sua-name-${dong.id}`}
                name="name"
                className="o-nhap"
                type="text"
                defaultValue={dong.name}
                required
              />
            </Field>
            <Field label="Thứ tự hiển thị" htmlFor={`sua-order-${dong.id}`} grow="auto">
              <input
                id={`sua-order-${dong.id}`}
                name="order"
                className="o-nhap"
                type="number"
                step={1}
                defaultValue={dong.order}
              />
            </Field>

            {moO ? (
              cot.map((c) => {
                const lyDo = lyDoKhongTinh(dong.unavailable_reasons?.[c.id]);
                const idGoiY = `sua-gia-goi-y-${dong.id}-${c.id}`;
                return (
                  <Field
                    key={c.id}
                    label={`${c.name} (${donVi.nhan.toLowerCase()})`}
                    htmlFor={`sua-gia-${dong.id}-${c.id}`}
                    grow="auto"
                    hint={
                      lyDo !== null ? (
                        <span id={idGoiY} className="inline-flex items-start gap-1 text-warning-600">
                          <Glyph icon={TriangleAlert} className="mt-0.5 size-3.5 shrink-0" />
                          <span>
                            Ô này không tính được: {lyDo}. Để nguyên chữ “{O_KHONG_TINH_DUOC}” thì
                            con số đang lưu được giữ nguyên; gõ số mới để thay; xoá trắng ô để xoá
                            con số.
                          </span>
                        </span>
                      ) : undefined
                    }
                  >
                    {/* Ô CHỮ, không `type="number"`: ô số của trình duyệt không đọc được `3.463.459,2`.
                        Điền sẵn ĐÚNG chuỗi màn hình in, và chuỗi ấy đọc ngược lại ra đúng số đồng cũ —
                        nên "Lưu" mà không sửa gì không đổi con số nào. Ô KHÔNG TÍNH ĐƯỢC điền chữ
                        `O_KHONG_TINH_DUOC`, không điền rỗng — xem `dungGiaSuaDong`. */}
                    <input
                      id={`sua-gia-${dong.id}-${c.id}`}
                      name={`gia:${c.id}`}
                      className="o-nhap"
                      type="text"
                      inputMode="decimal"
                      autoComplete="off"
                      aria-describedby={lyDo === null ? undefined : idGoiY}
                      defaultValue={
                        lyDo === null ? giaDienSan(dong.values[c.id] ?? null, donVi) : O_KHONG_TINH_DUOC
                      }
                    />
                  </Field>
                );
              })
            ) : dong.method === "entries" ? (
              <p className="m-0 text-xs text-ink-500 sm:col-span-2 lg:col-span-3">
                Khoản mục này đang Cộng theo đợt: con số của nó là tổng các đợt ghi ở hộp ⇄, không
                gõ thẳng vào đây. Muốn gõ tay, đổi Cách tính về Nhập trực tiếp.
              </p>
            ) : (
              <p className="m-0 text-xs text-ink-500 sm:col-span-2 lg:col-span-3">
                Khoản mục này có khoản mục con nên con số của nó là tổng các con — máy chủ từ chối
                số gõ thẳng vào đây. Sửa ở từng dòng con.
              </p>
            )}
          </div>

          {loiO !== null && (
            <p className="thong-bao-loi m-0" role="alert">
              {loiO}
            </p>
          )}

          <p className="m-0 flex flex-wrap justify-end gap-2">
            <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
              Huỷ
            </Button>
            <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui || undefined}>
              <BusyLabel busy={dangGui} label="Lưu khoản mục" busyText={BUSY_SAVING} />
            </Button>
          </p>
        </form>
      </td>
    </tr>
  );
}

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
 * Chuỗi điền sẵn vào ô tiền của biểu mẫu sửa dòng.
 *
 * Giá trị hỏng (không phải số nguyên an toàn) điền NGUYÊN chữ số thô, không điền rỗng: ô rỗng lúc
 * lưu là XOÁ TRẮNG ô ấy, nên điền rỗng cho một giá trị đọc hỏng là lặng lẽ xoá một con số ngân
 * sách. Chữ thô sẽ bị `docSoNhap` từ chối kèm một câu, và cán bộ thấy có chuyện.
 */
function giaDienSan(gia: number | null, donVi: DonViHien): string {
  if (gia === null) return "";
  if (!Number.isSafeInteger(gia)) return String(gia);
  return dongSangChuoi(gia, donVi.ma);
}

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
