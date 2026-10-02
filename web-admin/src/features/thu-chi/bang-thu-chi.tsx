"use client";

import {
  ArrowDownToLine,
  ArrowLeftRight,
  ArrowUpFromLine,
  Calculator,
  ChevronDown,
  ChevronRight,
  ChevronsDownUp,
  ChevronsUpDown,
  Database,
  FileSpreadsheet,
  Gauge,
  Info,
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
import { useEffect, useState, type FormEvent } from "react";

import { ChonNam } from "@/components/chon-nam";
import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Field } from "@/components/ui/field";
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
import { namTheoDongHoMay } from "@/lib/nam";
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_XAC_NHAN_NGAN_SACH } from "@/lib/quyen";

import { HopDotThuChi } from "./dot-thu-chi";
import { FormGoKemLyDo } from "./form-go-ly-do";
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
  dongPhuTieuDe,
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
  nhanThemCon,
  NHAN_SUA_TEN,
  O_KHONG_TINH_DUOC,
  O_TRONG,
  percentCell,
  phangCay,
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
 * ĐẶC TẢ CỦA CHƯƠNG NÀY CÒN VÀI CHỖ HỢP ĐỒNG CHƯA ĐỠ, VÀ MÀN HÌNH NÓI RA TỪNG CHỖ THAY VÌ DỰNG THEO.
 * Danh sách nằm ở `PHAN_CHUA_DUNG`; each entry is the description behind a disabled "?" placeholder
 * at its spec position (ADR 0068 §14, `header-actions.tsx`), không giấu trong chú thích mã. Cái
 * quyết định ở mọi chỗ hai bên lệch là HỢP ĐỒNG và handler thật, không phải bản vẽ (luật 2 bất
 * biến 7).
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

  /** Một lần ghi xong: đóng hộp thoại đang mở, xoá thông báo cũ, và đọc lại từ máy chủ. */
  function xong(kq: KetQua<unknown>): void {
    datDangGui(false);
    if (!kq.ok) {
      // NGUYÊN VĂN câu máy chủ: 409 của tuyến này mang đúng quy tắc nghiệp vụ đã từ chối ("khoản
      // mục có dòng con thì không gõ số vào cha"), và viết lại nó ở client là dựng bản sao thứ hai
      // của một quy tắc rồi để nó trôi.
      datLoiGhi(kq.thongBao);
      return;
    }
    datLoiGhi(null);
    datDangMo(null);
    datDangSuaDong(null);
    datLanTai((n) => n + 1);
  }

  return (
    <section className="man-giai-ngan flex min-w-0 flex-col gap-4" aria-labelledby="tieu-de-thu-chi">
      {/* The page's `<h1>` already says this; the heading stays as the region's accessible name. */}
      <h2 id="tieu-de-thu-chi" className="an-thi-giac">
        Thu - Chi ngân sách xã
      </h2>

      {/* ONE filter (the year), no search box on this screen: an aligned row, no "Bộ lọc" button
          (ADR 0068 §12, "Hàng ≤ 3 ô"). The year decides which budget every figure below belongs to. */}
      <div className="hang-loc m-0">
        <ChonNam
          id="nam-ngan-sach-thu-chi"
          nhan="Năm ngân sách"
          nam={nam}
          namGoc={namGoc}
          datNam={(n) => {
            datNam(n);
            datDangMo(null);
            datDangSuaDong(null);
            datLoiGhi(null);
          }}
        />
      </div>

      {/* THẺ BA CHỈ SỐ NẰM NGOÀI TAB, có chủ ý: con số chênh lệch cần CẢ HAI bảng, nên đặt nó
          trong một tab sẽ nói rằng con số ấy thuộc về tab ấy. */}
      {chiSo.pha === "dangTai" && (
        <Card>
          <p role="status" className="an-thi-giac">
            Đang tải chỉ số ngân sách…
          </p>
          <div aria-hidden="true" className="grid gap-4 p-4 sm:grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }, (_, i) => (
              <div key={i} className="flex flex-col gap-3 rounded-xl border border-line p-4">
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

      {/* THE CLOSE BLOCK SITS OUTSIDE THE TABS: a close covers a period of the commune's budget,
          both the revenue and the expenditure sheet of that year. */}
      <BudgetPeriodClosePanel
        // A new year is a new set of forms: a key kept from 2025 must not close 2026.
        key={nam}
        year={nam}
        view={closesView}
        canConfirm={coXacNhan}
        onChanged={() => datLanTai((n) => n + 1)}
      />

      {/* Hai tab của §2. `role="tablist"` đúng chuẩn như `15-phu-luc §8` đòi. */}
      <TabList aria-label="Chọn bảng thu hoặc bảng chi">
        {(["chi", "thu"] as const).map((l) => (
          <Tab
            key={l}
            id={`tab-ngan-sach-${l}`}
            selected={loai === l}
            icon={l === "chi" ? ArrowUpFromLine : ArrowDownToLine}
            aria-controls={`bang-ngan-sach-${l}`}
            onClick={() => {
              datLoai(l);
              datDangMo(null);
              datDangSuaDong(null);
              datLoiGhi(null);
            }}
          >
            {nhanTab(l, nam)}
          </Tab>
        ))}
      </TabList>

      <div
        role="tabpanel"
        id={`bang-ngan-sach-${loai}`}
        aria-labelledby={`tab-ngan-sach-${loai}`}
        tabIndex={0}
        className="flex min-w-0 flex-col gap-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500"
      >
        {loiGhi !== null && (
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
            sheetLock={sheetLockReason(closes, bang.duLieu.sheet.year)}
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
              datDangSuaDong(null);
              datDangMo({ kieu: "them", chaId, tenCha, thuTuGoiY });
            }}
            moGoDong={(dong) => {
              datDangSuaDong(null);
              datDangMo({ kieu: "goDong", dong });
            }}
            datTong={(dong) => {
              datDangGui(true);
              datDongTong(dong.id).then(xong);
            }}
            moGoBang={() => datDangMo({ kieu: "goBang" })}
            moSuaBang={() => {
              datDangSuaDong(null);
              datDangMo({ kieu: "suaBang" });
            }}
            moCachTinh={(dong, den) => {
              datDangSuaDong(null);
              datDangMo({ kieu: "cachTinh", dong, den });
            }}
            moDot={(dong) => {
              datDangSuaDong(null);
              datLoiGhi(null);
              datDangMo({
                kieu: "dot",
                dong,
                cot: cotSo(bang.duLieu.columns),
                donVi: donViCuaBang(bang.duLieu.sheet),
              });
            }}
          />
        )}

        {dangMo !== null && dangMo.kieu === "suaBang" && bang.pha === "xong" && (
          <FormSuaBang
            bang={bang.duLieu.sheet}
            dangGui={dangGui}
            huy={() => datDangMo(null)}
            luu={(than) => {
              datDangGui(true);
              suaBang(bang.duLieu.sheet.id, than).then(xong);
            }}
          />
        )}

        {dangMo !== null && dangMo.kieu === "cachTinh" && (
          <FormDoiCachTinh
            ten={dangMo.dong.name}
            den={dangMo.den}
            dangGui={dangGui}
            huy={() => datDangMo(null)}
            luu={() => {
              datDangGui(true);
              doiCachTinh(dangMo.dong.id, dangMo.den).then(xong);
            }}
          />
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

        {/* ── các hộp thoại ghi ───────────────────────────────────────────────────────────── */}

        {dangMo !== null && dangMo.kieu === "them" && bang.pha === "xong" && (
          <FormThemKhoanMuc
            tenCha={dangMo.tenCha}
            thuTuGoiY={dangMo.thuTuGoiY}
            dangGui={dangGui}
            huy={() => datDangMo(null)}
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

        {dangMo !== null && dangMo.kieu === "goDong" && (
          <FormGoKemLyDo
            tieuDe={`Gỡ khoản mục ${dangMo.dong.name}?`}
            submitLabel="Gỡ khoản mục"
            canhBao={CANH_BAO_GO_KHOAN_MUC}
            dangGui={dangGui}
            huy={() => datDangMo(null)}
            luu={(lyDo) => {
              datDangGui(true);
              goKhoanMuc(dangMo.dong.id, lyDo).then(xong);
            }}
          />
        )}

        {dangMo !== null && dangMo.kieu === "goBang" && bang.pha === "xong" && (
          <FormGoKemLyDo
            tieuDe={`Gỡ bảng ${bang.duLieu.sheet.title}?`}
            submitLabel="Gỡ bảng"
            canhBao={CANH_BAO_GO_BANG}
            dangGui={dangGui}
            huy={() => datDangMo(null)}
            luu={(lyDo) => {
              datDangGui(true);
              goBang(bang.duLieu.sheet.id, lyDo).then(xong);
            }}
          />
        )}

        {/* BIỂU MẪU LẬP BẢNG HIỆN KHI CHƯA ĐỌC ĐƯỢC BẢNG, và chỉ với người có `budget.update`.
            Nó không khẳng định vì sao không đọc được — nếu bảng đã có thật thì máy chủ trả 409
            "bảng đã tồn tại" và câu ấy ra thẳng màn hình. */}
        {bang.pha === "loi" && coGhi && (
          <LapBang
            // ĐỔI NĂM HOẶC ĐỔI TAB THÌ DỰNG LẠI BIỂU MẪU, bằng `key` chứ không bằng một `setState`
            // trong `useEffect` (React Compiler chặn, và đúng chỗ này thì nó chặn đúng): bộ cột
            // điền sẵn phụ thuộc cả năm lẫn loại bảng, nên giữ lại bộ cột cũ là lập bảng thu bằng
            // bộ cột của bảng chi.
            key={`${nam}|${loai}`}
            nam={nam}
            loai={loai}
            dangGui={dangGui}
            datDangGui={datDangGui}
            xong={xong}
          />
        )}
      </div>
    </section>
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
 * SỐ TIỀN Ở THẺ NÀY IN BẰNG ĐỒNG, kèm chữ "đồng": thẻ nằm NGOÀI hai tab và con số chênh lệch đọc từ
 * CẢ HAI bảng, hai bảng có thể mang hai đơn vị khác nhau. Chọn đơn vị của một bảng để in một con
 * số của cả hai là ngầm nói con số ấy thuộc bảng đó.
 */
export function TheChiSoNam({ chiSo }: { chiSo: finance_chiSoNamRa }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle as="h3" className="inline-flex items-center gap-2">
          <Gauge aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
          Chỉ số ngân sách năm {chiSo.year}
        </CardTitle>
      </CardHeader>
      <dl className={FIGURE_TILES}>
        <OChiSo chi={chiSo.revenue_achievement} />
        <OChiSo chi={chiSo.expenditure_achievement} />
        <div>
          <dt title={NHAN_CHENH_LECH}>{NHAN_CHENH_LECH}</dt>
          <dd>
            {chiSo.balance.amount !== null || nhanSoTienChiSo(chiSo.balance, "dong") === O_TRONG ? (
              soTienDong(nhanSoTienChiSo(chiSo.balance, "dong"), chiSo.balance.amount !== null)
            ) : (
              // The server's reason, at body size — see `FIGURE_TILES`.
              <span className="text-[13px] leading-snug font-medium text-ink-700">
                {nhanSoTienChiSo(chiSo.balance, "dong")}
              </span>
            )}
          </dd>
        </div>
        {/* TỔNG THU mang câu lý do cho MỌI trường hợp không đưa ra được con số (chưa có dòng tổng,
            cột trống, tổng quá lớn) — nên `null` kèm câu là "không tính được", còn `null` không câu
            vẫn là `—`. */}
        {chiSo.revenue_totals.map((o) => (
          <div key={o.column_id}>
            <dt title={o.name}>{o.name}</dt>
            <dd>
              <OTien
                chu={soTienDong(nhanSoTien(o.value, "dong"), o.value !== null)}
                lyDo={lyDoKhongTinh(o.unavailable_reason)}
                hienLyDo
              />
            </dd>
          </div>
        ))}
      </dl>
      <CardFooter>
        <span className="inline-flex items-start gap-1.5">
          <Info aria-hidden="true" focusable="false" strokeWidth={1.8} className="mt-0.5 size-3.5 shrink-0" />
          <span>
            {NHAN_CHENH_LECH}: {GHI_CHU_CHENH_LECH}
          </span>
        </span>
      </CardFooter>
    </Card>
  );
}

/**
 * Figure tiles of the two summary cards — the look of `StatCard` (spec §7 "Thẻ số liệu": one-line
 * 12px label, a large tabular figure, equal heights) drawn on the cards' EXISTING `<dl>`.
 *
 * NOT `StatCard` ITSELF, on purpose: each figure here is a `<dt>`/`<dd>` pair, and the `<dd>` may
 * hold a server sentence instead of a number ("Không tính được — <câu>"), which must stay readable
 * at text size, not at 26px. So the `<dd>` keeps its bare markup and is styled from the list: a
 * plain figure is large and bold; a `title`-bearing "không tính được" mark (`OTien`) or a long
 * sentence (`nhanChiSo`'s reason) drops back to body size.
 */
const FIGURE_TILES = cn(
  "m-0 grid min-w-0 gap-4 p-4 sm:grid-cols-2 lg:grid-cols-4",
  "[&>div]:flex [&>div]:min-w-0 [&>div]:flex-col [&>div]:gap-2 [&>div]:rounded-xl [&>div]:border [&>div]:border-line [&>div]:bg-surface [&>div]:p-4",
  "[&_dt]:truncate [&_dt]:text-xs [&_dt]:font-semibold [&_dt]:text-ink-500",
  "[&_dd]:m-0 [&_dd]:text-[22px] [&_dd]:leading-tight [&_dd]:font-bold [&_dd]:break-words [&_dd]:text-ink-900 [&_dd]:tabular-nums",
  "[&_dd>span[title]]:text-[13px] [&_dd>span[title]]:leading-snug [&_dd>span[title]]:font-medium",
);

/** Gắn chữ "đồng" sau một con số — không gắn sau `—` hay sau một câu lý do của máy chủ. */
function soTienDong(chu: string, coSo: boolean): string {
  return coSo && chu !== "Không đọc được" ? `${chu} đồng` : chu;
}

function OChiSo({ chi }: { chi: finance_chiSoRa }) {
  return (
    <div>
      {/* TÊN CHỈ SỐ LẤY TỪ MÁY CHỦ, không gõ lại: `ChiSoDatDuToan` đặt tên theo loại bảng, và một
          bản sao ở client sẽ gọi bảng thu là "Chi đạt dự toán" vào ngày ai đó đổi thứ tự. */}
      <dt title={chi.name}>{chi.name}</dt>
      {/* `nhanChiSo` prints the server's reason when there is no figure; that sentence is drawn at
          body size, a figure (or `—`) at tile size. Same words either way. */}
      <dd>
        {isFigureOrDash(chi) ? (
          nhanChiSo(chi)
        ) : (
          <span className="text-[13px] leading-snug font-medium text-ink-700">{nhanChiSo(chi)}</span>
        )}
      </dd>
    </div>
  );
}

/** True when `nhanChiSo` prints a figure or `—`, false when it prints the server's sentence. */
function isFigureOrDash(indicator: finance_chiSoRa): boolean {
  return indicator.basis_points !== null || nhanChiSo(indicator) === O_TRONG;
}

/** Thẻ tiêu đề báo cáo (§2) + thanh công cụ (§4.3) + bảng cây (§4.1). */
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
  moGoBang,
  moSuaBang,
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
  moGoBang: () => void;
  moSuaBang: () => void;
  moCachTinh: (dong: finance_dongRa, den: CachTinhChon) => void;
  moDot: (dong: finance_dongRa) => void;
}) {
  const cay = dungCay(duLieu.lines);
  const dongHien = phangCay(cay, thuGon);
  const cot = cotSo(duLieu.columns);
  const donVi = donViCuaBang(duLieu.sheet);
  const locked = sheetLock !== null;
  const lockTitle = sheetLock ?? undefined;

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
      />

      <Card>
        {/* Toolbar of the tree (§4.3): view controls and the count on the left, the sheet's write
            actions on the right. No solid button here — none of these is the page's main action,
            and the removal is the red outline (spec §5, §7). Every word kept; glyphs → lucide. */}
        <div className="flex min-w-0 flex-wrap items-center gap-2 border-b border-line px-4 py-3">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            icon={<Glyph icon={ChevronsDownUp} />}
            onClick={() => datThuGon(moiDongCoCon(duLieu.lines))}
          >
            Chỉ xem mục lớn
          </Button>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            icon={<Glyph icon={ChevronsUpDown} />}
            onClick={() => datThuGon(new Set())}
          >
            Mở hết chi tiết
          </Button>
          <span className="dem-muc tabular-nums">{nhanBoDem(dongHien.length, duLieu.lines.length)}</span>
          <span className="ml-auto flex flex-wrap items-center gap-2">
            {coGhi && (
              <Button
                type="button"
                variant="secondary"
                size="sm"
                icon={<Glyph icon={ListPlus} />}
                disabled={dangGui || locked}
                title={lockTitle}
                onClick={() => moThem("", "cấp cao nhất", thuTuKeTiep(duLieu.lines, ""))}
              >
                Thêm khoản mục cấp cao nhất
              </Button>
            )}
            {coGhi && (
              <Button
                type="button"
                variant="secondary"
                size="sm"
                icon={<Glyph icon={Pencil} />}
                disabled={dangGui || locked}
                title={lockTitle}
                onClick={moSuaBang}
              >
                Sửa thông tin bảng
              </Button>
            )}
            {coXacNhan && (
              <Button
                type="button"
                variant="danger"
                size="sm"
                icon={<Glyph icon={Trash2} />}
                disabled={dangGui || locked}
                title={lockTitle}
                onClick={moGoBang}
              >
                Gỡ bảng
              </Button>
            )}
          </span>
        </div>

      <TableScroll
        sticky
        aria-label={`Khoản mục của ${duLieu.sheet.title}`}
        className="rounded-none border-0 shadow-none"
      >
        <table className={cn("bang-danh-muc", DATA_TABLE_CLASS)}>
          <caption className="an-thi-giac">{duLieu.sheet.title}</caption>
          <thead>
            <tr>
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
              <th scope="col" className="text-right">
                Thao tác
              </th>
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
                  dongTongId={duLieu.summary.headline_line_id ?? ""}
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
      </Card>
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
 * Thẻ tiêu đề báo cáo của §2.
 *
 * `unavailable_reason` CỦA THẺ TÓM TẮT LÀ MỘT CÂU, KHÔNG PHẢI MỘT Ô TRỐNG. Chưa ai đánh dấu dòng
 * tổng, hoặc hai dòng cùng nhận là dòng tổng, thì không có gì để đọc — và câu của máy chủ nói ra
 * việc phải làm. Điền đại bằng "dòng đầu tiên" là đúng cái đoán mà `is_headline` sinh ra để từ
 * chối: bảng thu có hai dòng cấp cao lồng nhau, bảng chi có `Tổng số` đứng ngang hàng A…E.
 */
export function TheTomTat({
  bang,
  tomTat,
  soKhoanMuc,
  donVi,
}: {
  bang: finance_bangRa;
  tomTat: finance_tomTatRa;
  soKhoanMuc: number;
  donVi: DonViHien;
}) {
  return (
    <Card>
      <CardHeader>
        <div className="min-w-0">
          <CardTitle as="h3" className="inline-flex items-center gap-2">
            <Glyph icon={FileSpreadsheet} className="size-[18px] shrink-0 text-brand-600" />
            {bang.title}
          </CardTitle>
          <p className="m-0 mt-1 text-[13px] text-ink-500 tabular-nums">{dongPhuTieuDe(bang, soKhoanMuc)}</p>
        </div>
      </CardHeader>
      <div className="flex min-w-0 flex-col gap-1 px-4 pt-4 text-xs text-ink-500">
        {/* ĐƠN VỊ CŨ CHƯA ÁNH XẠ ĐƯỢC: nói NỔI BẬT, vì số đang in bằng đồng trong khi tờ giấy của
            xã in theo đơn vị khác — người đọc so hai bản sẽ thấy lệch hàng nghìn lần. */}
        {donVi.canhBao !== null && (
          <p className="thong-bao-loi m-0 mb-2 rounded-xl border border-danger-200 bg-danger-50 px-3.5 py-3" role="alert">
            {donVi.canhBao}
            {donVi.nhanCu !== null && ` Đơn vị đang lưu: "${donVi.nhanCu}".`}
          </p>
        )}
        <p className="m-0">{cauQuyDoi(donVi)}</p>
        <p className="m-0 inline-flex items-center gap-1">
          <Glyph icon={Star} className="size-3.5 shrink-0 fill-current text-warning-600" />
          Con số tổng lấy từ dòng được đánh sao. Bấm ngôi sao ở đầu một dòng khác để đổi.
        </p>
      </div>

      {tomTat.unavailable_reason !== undefined && tomTat.unavailable_reason !== "" ? (
        <EmptyState icon={Star} tone="neutral" title={tomTat.unavailable_reason} className="py-6" />
      ) : (
        <dl className={FIGURE_TILES}>
          {tomTat.cells.map((o) => (
            <div key={o.column_id}>
              <dt title={o.name}>{o.name}</dt>
              <dd>
                <OTien
                  chu={nhanSoTien(o.value, donVi.ma)}
                  lyDo={lyDoKhongTinh(o.unavailable_reason)}
                  hienLyDo
                />
              </dd>
            </div>
          ))}
          <OChiSo chi={tomTat.indicator} />
        </dl>
      )}
    </Card>
  );
}

/** Một dòng của cây khoản mục (§4.1). */
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

  return (
    <tr>
      <td className="ma-muc text-[13px] text-ink-500">{d.no === "" ? O_TRONG : d.no}</td>
      <td className={hien.cap === 0 ? "font-semibold text-ink-900" : undefined}>
        {/* Thụt lề bằng khoảng cách chứ không bằng một cột riêng: §4.1 vẽ cây trong CHÍNH cột
            Nội dung. `paddingInlineStart` (không phải `paddingLeft`) để không hỏng nếu giao diện
            có ngày chạy ở chiều ngược lại. */}
        <span
          style={{ paddingInlineStart: `${hien.cap * 1.25}rem` }}
          className="inline-flex min-w-0 items-center gap-1"
        >
          {hien.coCon ? (
            <button
              type="button"
              className={TREE_ICON_BUTTON}
              aria-expanded={hien.moRong}
              onClick={moRongDoi}
            >
              <Glyph icon={hien.moRong ? ChevronDown : ChevronRight} className="size-4" />
              <span className="an-thi-giac">
                {hien.moRong ? `Thu gọn ${d.name}` : `Mở ${d.name}`}
              </span>
            </button>
          ) : (
            // Same width as the toggle, so a leaf's name lines up with its siblings' names.
            <span aria-hidden="true" className="inline-block w-7 shrink-0" />
          )}
          {coXacNhan ? (
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
            // thông tin ai xem bảng cũng cần, kể cả người không đổi được nó. The sentence that was
            // only a tooltip is now also the accessible name (the `★` glyph read as "ngôi sao").
            laDongTong && (
              <span title={HEADLINE_LINE_LABEL} className="inline-flex w-7 shrink-0 justify-center">
                <Glyph icon={Star} className="size-4 fill-current text-warning-600" />
                <span className="an-thi-giac">{HEADLINE_LINE_LABEL}</span>
              </span>
            )
          )}
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
        </span>
      </td>

      {cot.map((c) => {
        if (c.type === "so") {
          return (
            <td key={c.id} className="text-right tabular-nums">
              <OTien
                chu={nhanSoTien(d.values[c.id] ?? null, donVi.ma)}
                lyDo={lyDoKhongTinh(d.unavailable_reasons?.[c.id])}
              />
            </td>
          );
        }
        // Cột phần trăm: tỷ lệ MÁY CHỦ tính cho dòng này (`percent_basis_points`), không chia lại ở
        // đây. Không có tỷ lệ thì máy chủ gửi câu lý do, và ô vẽ đúng dấu "Không tính được".
        const cell = percentCell(d, c.id);
        return (
          <td key={c.id} className="text-right tabular-nums">
            <OTien chu={cell.text} lyDo={cell.reason} />
          </td>
        );
      })}

      <td className="nhan-trong">
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
          nhanCachTinh(d.method)
        )}
      </td>

      <td>
        {/* Icon-only, 34px: each carries its full name as `aria-label` and as the tooltip — or,
            under a year close, the lock reason as the tooltip (same as before). */}
        <span className="flex items-center justify-end gap-1">
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
              <Glyph icon={ArrowLeftRight} />
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

/** Biểu mẫu thêm một khoản mục — dùng cho cả `⊞` cấp cao nhất lẫn `＋` dòng con. */
export function FormThemKhoanMuc({
  tenCha,
  thuTuGoiY,
  dangGui,
  huy,
  luu,
}: {
  tenCha: string;
  thuTuGoiY: number;
  dangGui: boolean;
  luu: (no: string, ten: string, thuTu: number, khoaChongTrung: string) => void;
  huy: () => void;
}) {
  /**
   * Khoá chống trùng sinh MỘT LẦN lúc mở biểu mẫu, không lúc gửi.
   *
   * Sinh lúc gửi thì mỗi lần bấm là một khoá mới, tức là không chống được gì — mà tuyến này đòi
   * khoá đúng vì hai dòng cùng tên dưới một cha là chuyện có thật trong biểu mẫu ngân sách.
   */
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  return (
    <Card
      as="form"
      aria-labelledby="tieu-de-them-khoan-muc"
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
      <CardHeader>
        <div className="min-w-0">
          <CardTitle as="h3" id="tieu-de-them-khoan-muc">
            Thêm khoản mục dưới {tenCha}
          </CardTitle>
          <p className="m-0 mt-1 text-[13px] text-ink-500">
            Khoản mục mới bắt đầu ở Nhập trực tiếp. Khi chưa có khoản mục con, có thể đổi sang Cộng
            theo đợt ở cột Cách tính; khi có khoản mục con đầu tiên, nó thành dòng cộng con.
          </p>
        </div>
      </CardHeader>
      {/* Labels above, 40px controls; the three fields share one row from 1024px. */}
      <div className="grid min-w-0 gap-4 p-4 sm:grid-cols-2 lg:grid-cols-3">
        <Field label="Số thứ tự (TT)" htmlFor="them-no" grow="auto">
          <input id="them-no" name="no" className="o-nhap" type="text" maxLength={32} />
        </Field>
        <Field label="Nội dung khoản mục" htmlFor="them-name" grow="auto">
          <input id="them-name" name="name" className="o-nhap" type="text" required />
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
      </div>
      <CardFooter className="justify-end">
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
      </CardFooter>
    </Card>
  );
}

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
}: {
  ten: string;
  den: CachTinhChon;
  dangGui: boolean;
  huy: () => void;
  luu: () => void;
}) {
  // A confirm box (spec v2 §7): the specific question, the consequence, the button naming the act.
  // Not red — changing how a line is computed destroys nothing; the consequence sentence says what.
  return (
    <ConfirmDialog
      as="form"
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
