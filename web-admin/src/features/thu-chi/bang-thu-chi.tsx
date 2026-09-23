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
  goBang,
  goKhoanMuc,
  layBang,
  layChiSoNganSach,
  suaKhoanMuc,
  themKhoanMuc,
  type LoaiBang,
  type SuaDongVao,
} from "@/lib/api/thu-chi";
import { namTheoDongHoMay } from "@/lib/nam";
import { coQuyen, QUYEN_GHI_NGAN_SACH, QUYEN_XAC_NHAN_NGAN_SACH } from "@/lib/quyen";

import { LapBang } from "./lap-bang";
import {
  CANH_BAO_GO_BANG,
  CANH_BAO_GO_KHOAN_MUC,
  CAU_KHONG_QUY_DOI,
  cotSo,
  docSoNhap,
  dongPhuTieuDe,
  dungCay,
  moiDongCoCon,
  nhanBoDem,
  nhanCachTinh,
  nhanChiSo,
  nhanChuaCoBang,
  nhanDatDongTong,
  nhanGoKhoanMuc,
  nhanSoTien,
  nhanSoTienChiSo,
  nhanTab,
  nhanThemCon,
  NHAN_SUA_TEN,
  O_TRONG,
  phangCay,
  PHAN_CHUA_DUNG,
  suaDuocOSo,
  type DongHien,
} from "./nhan-thu-chi";

/**
 * Màn "Thu - Chi ngân sách" (`docs/ui-ux/07-thu-chi-ngan-sach.md`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐẶC TẢ CỦA CHƯƠNG NÀY ĐÃ LỖI THỜI Ở NĂM CHỖ, VÀ MÀN HÌNH NÓI RA TỪNG CHỖ THAY VÌ DỰNG THEO.
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
  | { kieu: "goBang" };

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

      {/* THẺ BA CHỈ SỐ NẰM NGOÀI TAB, có chủ ý: `Cân đối thu - chi` cần CẢ HAI bảng, nên đặt nó
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
      <summary>Năm phần của bản thiết kế chưa dựng được — bấm để xem lý do</summary>
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
          <dt>Cân đối thu - chi</dt>
          <dd>{nhanSoTienChiSo(chiSo.balance)}</dd>
        </div>
        {chiSo.revenue_totals.map((o) => (
          <div key={o.column_id}>
            <dt>{o.name}</dt>
            <dd>{nhanSoTien(o.value)}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
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
}) {
  const cay = dungCay(duLieu.lines);
  const dongHien = phangCay(cay, thuGon);
  const cot = cotSo(duLieu.columns);

  return (
    <>
      <TheTomTat bang={duLieu.sheet} tomTat={duLieu.summary} soKhoanMuc={duLieu.lines.length} />

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
                />
              ),
            )}
          </tbody>
        </table>
      </div>
    </>
  );
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
}: {
  bang: finance_bangRa;
  tomTat: finance_tomTatRa;
  soKhoanMuc: number;
}) {
  return (
    <div className="khoi-chi-tiet">
      <div className="dau-khoi-chi-tiet">
        <h3>{bang.title}</h3>
      </div>
      <p className="ghi-chu">{dongPhuTieuDe(bang, soKhoanMuc)}</p>
      <p className="ghi-chu">{CAU_KHONG_QUY_DOI}</p>
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
              <dd>{nhanSoTien(o.value)}</dd>
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
  dongTongId,
  coGhi,
  coXacNhan,
  dangGui,
  moRongDoi,
  moSua,
  them,
  go,
  datTong,
}: {
  hien: DongHien;
  cot: readonly finance_cotRa[];
  dongTongId: string;
  coGhi: boolean;
  coXacNhan: boolean;
  dangGui: boolean;
  moRongDoi: () => void;
  moSua: () => void;
  them: () => void;
  go: () => void;
  datTong: () => void;
}) {
  const d = hien.dong;
  const laDongTong = d.id === dongTongId;

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
          <td key={c.id}>{nhanSoTien(d.values[c.id] ?? null)}</td>
        ) : (
          // Cột phần trăm: xem `PHAN_CHUA_DUNG`. Công thức là chuỗi máy chủ không diễn giải, và
          // đoán ánh xạ `col_N` sang mã cột là in một tỷ lệ sai trông y hệt một tỷ lệ đúng.
          <td key={c.id} className="nhan-trong" title={`Công thức của bảng: ${c.formula ?? ""}`}>
            {O_TRONG}
          </td>
        ),
      )}

      <td className="nhan-trong">{nhanCachTinh(d.method)}</td>

      <td className="o-thao-tac">
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
  soCotBang,
  dangGui,
  huy,
  luu,
}: {
  dong: finance_dongRa;
  /** CHỈ cột `so` — cột phần trăm không lưu giá trị nào (§9 quy tắc 3). */
  cot: readonly finance_cotRa[];
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
              const gia: Record<string, number | null> = {};
              for (const c of cot) {
                const doc = docSoNhap(String(fd.get(`gia:${c.id}`) ?? ""));
                if (doc.loai === "loi") {
                  datLoiO(
                    `Ô "${c.name}" phải là số nguyên. Để trống nếu muốn xoá con số trong ô ấy.`,
                  );
                  return;
                }
                // `null` XOÁ TRẮNG ô, không phải số 0 (§9 quy tắc 4).
                gia[c.id] = doc.loai === "trong" ? null : doc.gia;
              }
              than.values = gia;
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
            cot.map((c) => (
              <p key={c.id}>
                <label htmlFor={`sua-gia-${dong.id}-${c.id}`}>{c.name}</label>{" "}
                <input
                  id={`sua-gia-${dong.id}-${c.id}`}
                  name={`gia:${c.id}`}
                  className="o-nhap"
                  type="number"
                  step={1}
                  defaultValue={dong.values[c.id] ?? ""}
                />
              </p>
            ))
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
        Cách tính của khoản mục mới do máy chủ suy ra từ cây, không chọn ở đây: dòng chưa có con thì
        nhập số trực tiếp, và thành dòng cộng con ngay khi có khoản mục con đầu tiên.
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
 * Hộp xác nhận GỠ — **có ô lý do**, và ô ấy không phải để cho đẹp.
 *
 * §6 chỉ vẽ "hộp xác nhận"; máy chủ đòi `reason` trong thân và từ chối 400 khi thiếu (luật 7 bất
 * biến 1 kể tên `delete_reason`). Một hộp chỉ có nút Đồng ý sẽ nhận 400 ở mọi lần bấm.
 */
export function FormGoKemLyDo({
  tieuDe,
  canhBao,
  dangGui,
  huy,
  luu,
}: {
  tieuDe: string;
  canhBao: string;
  dangGui: boolean;
  huy: () => void;
  luu: (lyDo: string) => void;
}) {
  return (
    <form
      className="khoi-chua-khai"
      onSubmit={(e) => {
        e.preventDefault();
        const fd = new FormData(e.currentTarget);
        luu(String(fd.get("reason") ?? ""));
      }}
    >
      <h3>{tieuDe}</h3>
      <p className="hau-qua">{canhBao}</p>
      <p>
        <label htmlFor="go-reason">Lý do gỡ (bắt buộc, được lưu cùng bản ghi)</label>{" "}
        <input id="go-reason" name="reason" className="o-nhap" type="text" required maxLength={500} />
      </p>
      <button type="submit" className="nut-chinh" disabled={dangGui}>
        Gỡ
      </button>{" "}
      <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
        Huỷ
      </button>
    </form>
  );
}
