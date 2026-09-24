"use client";

import { useEffect, useState } from "react";

import { ChonNam } from "@/components/chon-nam";
import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import type {
  finance_bangDayDuRa,
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
  phangCay,
  PHAN_CHUA_DUNG,
  suaDuocOSo,
  type DongHien,
  type DonViHien,
} from "./nhan-thu-chi";
import { DanhSachKhongTinh, OTien } from "./o-tien";
import { FormSuaBang } from "./sua-bang";

// Giữ đường nhập cũ cho phía gọi và bài kiểm: hộp gỡ nay nằm ở tệp riêng vì hộp `⇄` cũng dùng nó.
export { FormGoKemLyDo };

/**
 * Màn "Thu - Chi ngân sách" (`docs/ui-ux/07-thu-chi-ngan-sach.md`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐẶC TẢ CỦA CHƯƠNG NÀY CÒN VÀI CHỖ HỢP ĐỒNG CHƯA ĐỠ, VÀ MÀN HÌNH NÓI RA TỪNG CHỖ THAY VÌ DỰNG THEO.
 * Danh sách nằm ở `PHAN_CHUA_DUNG` và **hiện lên đầu màn**, không giấu trong chú thích mã. Cái
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

  const [thuGon, datThuGon] = useState<ReadonlySet<string>>(new Set());
  const [dangMo, datDangMo] = useState<DangMo | null>(null);
  const [dangSuaDong, datDangSuaDong] = useState<string | null>(null);
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  const [dangGui, datDangGui] = useState(false);

  const khoaBang = `${nam}|${loai}|${lanTai}`;
  const khoaChiSo = `${nam}|${lanTai}`;

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

  const phien = usePhien();
  // FAIL CLOSED: chưa đọc xong phiên, hoặc đọc hỏng, thì KHÔNG có quyền nào — "chưa rõ" không
  // được hành xử như "có" (luật 1, cấm #1).
  const dsQuyen: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const coGhi = coQuyen(dsQuyen, QUYEN_GHI_NGAN_SACH);
  const coXacNhan = coQuyen(dsQuyen, QUYEN_XAC_NHAN_NGAN_SACH);

  const bang = trangThaiTu(daTaiBang, khoaBang);
  const chiSo = trangThaiTu(daTaiChiSo, khoaChiSo);

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
    <section className="man-giai-ngan" aria-labelledby="tieu-de-thu-chi">
      <h2 id="tieu-de-thu-chi">Thu - Chi ngân sách xã</h2>

      <KhoiChuaDung />

      <div className="hang-loc">
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

      {/* Hai tab của §2. `role="tablist"` đúng chuẩn như `15-phu-luc §8` đòi. */}
      <div role="tablist" aria-label="Chọn bảng thu hoặc bảng chi">
        {(["chi", "thu"] as const).map((l) => (
          <button
            key={l}
            type="button"
            role="tab"
            id={`tab-ngan-sach-${l}`}
            aria-selected={loai === l}
            aria-controls={`bang-ngan-sach-${l}`}
            className={loai === l ? "nut-chinh" : "nut-phu"}
            onClick={() => {
              datLoai(l);
              datDangMo(null);
              datDangSuaDong(null);
              datLoiGhi(null);
            }}
          >
            {nhanTab(l, nam)}
          </button>
        ))}
      </div>

      {/* THẺ BA CHỈ SỐ NẰM NGOÀI TAB, có chủ ý: con số chênh lệch cần CẢ HAI bảng, nên đặt nó
          trong một tab sẽ nói rằng con số ấy thuộc về tab ấy. */}
      {chiSo.pha === "dangTai" && <p role="status">Đang tải chỉ số ngân sách…</p>}
      {chiSo.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {chiSo.thongBao}
        </p>
      )}
      {chiSo.pha === "xong" && <TheChiSoNam chiSo={chiSo.duLieu} />}

      <div
        role="tabpanel"
        id={`bang-ngan-sach-${loai}`}
        aria-labelledby={`tab-ngan-sach-${loai}`}
        tabIndex={0}
      >
        {loiGhi !== null && (
          <p className="thong-bao-loi" role="alert">
            {loiGhi}
          </p>
        )}

        {bang.pha === "dangTai" && <p role="status">Đang tải bảng ngân sách…</p>}

        {bang.pha === "loi" && (
          <>
            {/* KHÔNG ĐOÁN VÌ SAO KHÔNG ĐỌC ĐƯỢC: hiện NGUYÊN câu máy chủ. 404 ở tuyến này thường
                là "xã chưa lập bảng", nhưng phân biệt bằng cách đọc câu chữ là dựng lại đúng thứ
                `goi.ts` cố ý giấu đi — nên câu dưới nói cả hai khả năng và không khẳng định. */}
            <p className="thong-bao-loi" role="alert">
              {bang.thongBao}
            </p>
            <p className="trang-thai-rong">{nhanChuaCoBang(nam, loai)}</p>
          </>
        )}

        {bang.pha === "xong" && (
          <BangDayDu
            duLieu={bang.duLieu}
            thuGon={thuGon}
            datThuGon={datThuGon}
            coGhi={coGhi}
            coXacNhan={coXacNhan}
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
            tieuDe={`Gỡ khoản mục ${dangMo.dong.name}`}
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
            tieuDe={`Gỡ bảng ${bang.duLieu.sheet.title}`}
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
 * Những phần đặc tả vẽ mà máy chủ hôm nay không đỡ — HIỆN RA MÀN HÌNH.
 *
 * Không phải trang trí và không được rút gọn: cán bộ mở màn này là để làm đúng những việc §5 và §6
 * vẽ, và một màn hình im lặng về bốn việc ấy sẽ được đọc thành "hệ thống hỏng".
 */
export function KhoiChuaDung() {
  return (
    <details className="khoi-chua-khai">
      <summary>
        Những phần của bản thiết kế chưa dựng được ({PHAN_CHUA_DUNG.length}) — bấm để xem lý do
      </summary>
      <ul>
        {PHAN_CHUA_DUNG.map((p) => (
          <li key={p.ten}>
            <strong>{p.ten}</strong> — {p.viSao}
          </li>
        ))}
      </ul>
    </details>
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
    <div className="khoi-chi-tiet">
      <div className="dau-khoi-chi-tiet">
        <h3>Chỉ số ngân sách năm {chiSo.year}</h3>
      </div>
      <dl className="danh-sach-truong">
        <OChiSo chi={chiSo.revenue_achievement} />
        <OChiSo chi={chiSo.expenditure_achievement} />
        <div>
          <dt>{NHAN_CHENH_LECH}</dt>
          <dd>{soTienDong(nhanSoTienChiSo(chiSo.balance, "dong"), chiSo.balance.amount !== null)}</dd>
        </div>
        {/* TỔNG THU mang câu lý do cho MỌI trường hợp không đưa ra được con số (chưa có dòng tổng,
            cột trống, tổng quá lớn) — nên `null` kèm câu là "không tính được", còn `null` không câu
            vẫn là `—`. */}
        {chiSo.revenue_totals.map((o) => (
          <div key={o.column_id}>
            <dt>{o.name}</dt>
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
      <p className="ghi-chu">
        {NHAN_CHENH_LECH}: {GHI_CHU_CHENH_LECH}
      </p>
    </div>
  );
}

/** Gắn chữ "đồng" sau một con số — không gắn sau `—` hay sau một câu lý do của máy chủ. */
function soTienDong(chu: string, coSo: boolean): string {
  return coSo && chu !== "Không đọc được" ? `${chu} đồng` : chu;
}

function OChiSo({ chi }: { chi: finance_chiSoRa }) {
  return (
    <div>
      {/* TÊN CHỈ SỐ LẤY TỪ MÁY CHỦ, không gõ lại: `ChiSoDatDuToan` đặt tên theo loại bảng, và một
          bản sao ở client sẽ gọi bảng thu là "Chi đạt dự toán" vào ngày ai đó đổi thứ tự. */}
      <dt>{chi.name}</dt>
      <dd>{nhanChiSo(chi)}</dd>
    </div>
  );
}

/** Thẻ tiêu đề báo cáo (§2) + thanh công cụ (§4.3) + bảng cây (§4.1). */
export function BangDayDu({
  duLieu,
  thuGon,
  datThuGon,
  coGhi,
  coXacNhan,
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

  return (
    <>
      <TheTomTat
        bang={duLieu.sheet}
        tomTat={duLieu.summary}
        soKhoanMuc={duLieu.lines.length}
        donVi={donVi}
      />

      <div className="hang-loc">
        <button
          type="button"
          className="nut-phu"
          onClick={() => datThuGon(moiDongCoCon(duLieu.lines))}
        >
          › Chỉ xem mục lớn
        </button>
        <button type="button" className="nut-phu" onClick={() => datThuGon(new Set())}>
          ⌄ Mở hết chi tiết
        </button>
        <span className="dem-muc">{nhanBoDem(dongHien.length, duLieu.lines.length)}</span>
        {coGhi && (
          <button
            type="button"
            className="nut-phu"
            disabled={dangGui}
            onClick={() => moThem("", "cấp cao nhất", thuTuKeTiep(duLieu.lines, ""))}
          >
            ⊞ Thêm khoản mục cấp cao nhất
          </button>
        )}
        {coGhi && (
          <button type="button" className="nut-phu" disabled={dangGui} onClick={moSuaBang}>
            ✎ Sửa thông tin bảng
          </button>
        )}
        {coXacNhan && (
          <button type="button" className="nut-phu" disabled={dangGui} onClick={moGoBang}>
            🗑 Gỡ bảng
          </button>
        )}
      </div>

      <div className="bang-cuon" role="region" aria-label={`Khoản mục của ${duLieu.sheet.title}`} tabIndex={0}>
        <table className="bang-danh-muc">
          <caption className="an-thi-giac">{duLieu.sheet.title}</caption>
          <thead>
            <tr>
              <th scope="col">TT</th>
              <th scope="col">Nội dung</th>
              {duLieu.columns.map((c) => (
                <th key={c.id} scope="col">
                  {c.name}
                </th>
              ))}
              <th scope="col">Cách tính</th>
              <th scope="col">Thao tác</th>
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
      </div>
      <DanhSachKhongTinh tieuDe="Ô không tính được con số" o={oKhongTinhCuaCay(duLieu)} />
    </>
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
    <div className="khoi-chi-tiet">
      <div className="dau-khoi-chi-tiet">
        <h3>{bang.title}</h3>
      </div>
      <p className="ghi-chu">{dongPhuTieuDe(bang, soKhoanMuc)}</p>
      {/* ĐƠN VỊ CŨ CHƯA ÁNH XẠ ĐƯỢC: nói NỔI BẬT, vì số đang in bằng đồng trong khi tờ giấy của
          xã in theo đơn vị khác — người đọc so hai bản sẽ thấy lệch hàng nghìn lần. */}
      {donVi.canhBao !== null && (
        <p className="thong-bao-loi" role="alert">
          {donVi.canhBao}
          {donVi.nhanCu !== null && ` Đơn vị đang lưu: "${donVi.nhanCu}".`}
        </p>
      )}
      <p className="ghi-chu">{cauQuyDoi(donVi)}</p>
      <p className="ghi-chu">
        Con số tổng lấy từ dòng được đánh sao. Bấm ngôi sao ở đầu một dòng khác để đổi.
      </p>

      {tomTat.unavailable_reason !== undefined && tomTat.unavailable_reason !== "" ? (
        <p className="trang-thai-rong">{tomTat.unavailable_reason}</p>
      ) : (
        <dl className="danh-sach-truong">
          {tomTat.cells.map((o) => (
            <div key={o.column_id}>
              <dt>{o.name}</dt>
              <dd>
                <OTien
                  chu={nhanSoTien(o.value, donVi.ma)}
                  lyDo={lyDoKhongTinh(o.unavailable_reason)}
                  hienLyDo
                />
              </dd>
            </div>
          ))}
          <div>
            <dt>{tomTat.indicator.name}</dt>
            <dd>{nhanChiSo(tomTat.indicator)}</dd>
          </div>
        </dl>
      )}
    </div>
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

  return (
    <tr>
      <td className="ma-muc">{d.no === "" ? O_TRONG : d.no}</td>
      <td>
        {/* Thụt lề bằng khoảng cách chứ không bằng một cột riêng: §4.1 vẽ cây trong CHÍNH cột
            Nội dung. `paddingInlineStart` (không phải `paddingLeft`) để không hỏng nếu giao diện
            có ngày chạy ở chiều ngược lại. */}
        <span style={{ paddingInlineStart: `${hien.cap * 1.25}rem`, display: "inline-block" }}>
          {hien.coCon && (
            <button
              type="button"
              className="nut-phu"
              aria-expanded={hien.moRong}
              onClick={moRongDoi}
            >
              {hien.moRong ? "⌄" : "›"}
              <span className="an-thi-giac">
                {hien.moRong ? `Thu gọn ${d.name}` : `Mở ${d.name}`}
              </span>
            </button>
          )}{" "}
          {coXacNhan ? (
            <button
              type="button"
              className="nut-phu"
              aria-label={nhanDatDongTong(d.name)}
              aria-pressed={laDongTong}
              disabled={dangGui}
              onClick={datTong}
            >
              {laDongTong ? "★" : "☆"}
            </button>
          ) : (
            // Không có quyền đổi thì ngôi sao vẫn phải ĐỌC ĐƯỢC: dòng nào đang là con số tổng là
            // thông tin ai xem bảng cũng cần, kể cả người không đổi được nó.
            laDongTong && <span title="Dòng đang là con số tổng">★</span>
          )}{" "}
          {coGhi ? (
            <button
              type="button"
              className="nut-phu"
              aria-label={NHAN_SUA_TEN}
              disabled={dangGui}
              onClick={moSua}
            >
              {d.name}
            </button>
          ) : (
            d.name
          )}
        </span>
      </td>

      {cot.map((c) =>
        c.type === "so" ? (
          <td key={c.id}>
            <OTien
              chu={nhanSoTien(d.values[c.id] ?? null, donVi.ma)}
              lyDo={lyDoKhongTinh(d.unavailable_reasons?.[c.id])}
            />
          </td>
        ) : (
          // Cột phần trăm: xem `PHAN_CHUA_DUNG`. Công thức là chuỗi máy chủ không diễn giải, và
          // đoán ánh xạ `col_N` sang mã cột là in một tỷ lệ sai trông y hệt một tỷ lệ đúng.
          <td key={c.id} className="nhan-trong" title={`Công thức của bảng: ${c.formula ?? ""}`}>
            {O_TRONG}
          </td>
        ),
      )}

      <td className="nhan-trong">
        {coGhi && chonDuocCachTinh(d.method, hien.coCon) ? (
          // CHỌN KHÔNG GỬI NGAY: đổi cách tính đổi con số đang hiện, nên lựa chọn mở một hộp cảnh
          // báo (`FormDoiCachTinh`) và chỉ gửi khi cán bộ xác nhận. Ô chọn vẫn hiện chế độ ĐANG LƯU
          // cho tới khi bảng được đọc lại.
          <select
            aria-label={`Cách tính của ${d.name}`}
            value={d.method}
            disabled={dangGui}
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

      <td className="o-thao-tac">
        {laLa && (
          <button
            type="button"
            className="nut-phu"
            aria-label={nhanNutDot(d.name)}
            disabled={dangGui}
            onClick={moDot}
          >
            ⇄
          </button>
        )}
        {coGhi && (
          <button
            type="button"
            className="nut-phu"
            aria-label={nhanThemCon(d.name)}
            disabled={dangGui}
            onClick={them}
          >
            ＋
          </button>
        )}
        {coXacNhan && (
          <button
            type="button"
            className="nut-phu"
            aria-label={nhanGoKhoanMuc(d.name)}
            disabled={dangGui}
            onClick={go}
          >
            🗑
          </button>
        )}
      </td>
    </tr>
  );
}

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
      <td colSpan={soCotBang}>
        <form
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
          <p>
            <label htmlFor={`sua-no-${dong.id}`}>Số thứ tự (TT)</label>{" "}
            <input
              id={`sua-no-${dong.id}`}
              name="no"
              className="o-nhap"
              type="text"
              defaultValue={dong.no}
              maxLength={32}
            />
          </p>
          <p>
            <label htmlFor={`sua-name-${dong.id}`}>Nội dung khoản mục</label>{" "}
            <input
              id={`sua-name-${dong.id}`}
              name="name"
              className="o-nhap"
              type="text"
              defaultValue={dong.name}
              required
            />
          </p>
          <p>
            <label htmlFor={`sua-order-${dong.id}`}>Thứ tự hiển thị</label>{" "}
            <input
              id={`sua-order-${dong.id}`}
              name="order"
              className="o-nhap"
              type="number"
              step={1}
              defaultValue={dong.order}
            />
          </p>

          {moO ? (
            cot.map((c) => {
              const lyDo = lyDoKhongTinh(dong.unavailable_reasons?.[c.id]);
              const idGoiY = `sua-gia-goi-y-${dong.id}-${c.id}`;
              return (
                <p key={c.id}>
                  <label htmlFor={`sua-gia-${dong.id}-${c.id}`}>
                    {c.name} ({donVi.nhan.toLowerCase()})
                  </label>{" "}
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
                  {lyDo !== null && (
                    <span id={idGoiY} className="ghi-chu">
                      {" "}
                      ⚠ Ô này không tính được: {lyDo}. Để nguyên chữ “{O_KHONG_TINH_DUOC}” thì con
                      số đang lưu được giữ nguyên; gõ số mới để thay; xoá trắng ô để xoá con số.
                    </span>
                  )}
                </p>
              );
            })
          ) : dong.method === "entries" ? (
            <p className="ghi-chu">
              Khoản mục này đang Cộng theo đợt: con số của nó là tổng các đợt ghi ở hộp ⇄, không gõ
              thẳng vào đây. Muốn gõ tay, đổi Cách tính về Nhập trực tiếp.
            </p>
          ) : (
            <p className="ghi-chu">
              Khoản mục này có khoản mục con nên con số của nó là tổng các con — máy chủ từ chối số
              gõ thẳng vào đây. Sửa ở từng dòng con.
            </p>
          )}

          {loiO !== null && (
            <p className="thong-bao-loi" role="alert">
              {loiO}
            </p>
          )}

          <button type="submit" className="nut-chinh" disabled={dangGui}>
            Lưu khoản mục
          </button>{" "}
          <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
            Huỷ
          </button>
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
    <form
      className="khoi-chi-tiet"
      onSubmit={(e) => {
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
      <div className="dau-khoi-chi-tiet">
        <h3>Thêm khoản mục dưới {tenCha}</h3>
      </div>
      <p className="ghi-chu">
        Khoản mục mới bắt đầu ở Nhập trực tiếp. Khi chưa có khoản mục con, có thể đổi sang Cộng theo
        đợt ở cột Cách tính; khi có khoản mục con đầu tiên, nó thành dòng cộng con.
      </p>
      <p>
        <label htmlFor="them-no">Số thứ tự (TT)</label>{" "}
        <input id="them-no" name="no" className="o-nhap" type="text" maxLength={32} />
      </p>
      <p>
        <label htmlFor="them-name">Nội dung khoản mục</label>{" "}
        <input id="them-name" name="name" className="o-nhap" type="text" required />
      </p>
      <p>
        <label htmlFor="them-order">Thứ tự hiển thị</label>{" "}
        <input
          id="them-order"
          name="order"
          className="o-nhap"
          type="number"
          step={1}
          defaultValue={thuTuGoiY}
        />
      </p>
      <button type="submit" className="nut-chinh" disabled={dangGui}>
        Thêm khoản mục
      </button>{" "}
      <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
        Huỷ
      </button>
    </form>
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
  return (
    <form
      className="khoi-chua-khai"
      onSubmit={(e) => {
        e.preventDefault();
        luu();
      }}
    >
      <h3>
        Đổi cách tính của {ten} sang {nhanCachTinh(den)}
      </h3>
      <p className="hau-qua">{canhBaoDoiCachTinh(den)}</p>
      <button type="submit" className="nut-chinh" disabled={dangGui}>
        Đổi cách tính
      </button>{" "}
      <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
        Huỷ
      </button>
    </form>
  );
}
