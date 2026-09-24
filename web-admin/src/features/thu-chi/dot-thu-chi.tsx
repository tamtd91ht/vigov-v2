"use client";

import { useEffect, useState, type ReactNode } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { KetQua } from "@/lib/api/goi";
import type { finance_cotRa, finance_danhSachDotRa, finance_dotRa } from "@/lib/api/schema.gen";
import { ghiDot, goDot, layDot } from "@/lib/api/thu-chi";

import { FormGoKemLyDo } from "./form-go-ly-do";
import {
  cauDieuKienDot,
  DO_DAI_TOI_DA_DOT,
  DOT_TRONG,
  dungThanDot,
  khoaSauLanGhi,
  MO_TA_HOP_DOT,
  nhanNgayLuyKe,
  nhanSoTien,
  O_TRONG,
  tieuDeHopDot,
  type DonViHien,
  type NhapDot,
} from "./nhan-thu-chi";

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

  return (
    <section className="khoi-chi-tiet" role="dialog" aria-labelledby="tieu-de-hop-dot">
      <div className="dau-khoi-chi-tiet">
        <h3 id="tieu-de-hop-dot">{tieuDeHopDot(tenKhoanMuc)}</h3>
        <button type="button" className="nut-phu" onClick={dong}>
          Đóng
        </button>
      </div>

      <NoiDungHopDot
        method={method}
        cot={cot}
        donVi={donVi}
        danhSach={danhSach}
        coXacNhan={coXacNhan}
        dangGui={dangGui}
        moGo={(d) => {
          datDangGo(d);
          datLoi(null);
          datThongBao(null);
        }}
      >
        {loi !== null && (
          <p className="thong-bao-loi" role="alert">
            {loi}
          </p>
        )}
        {thongBao !== null && <p role="status">{thongBao}</p>}

        {dangGo !== null && (
          <FormGoKemLyDo
            idTruong="go-dot-reason"
            tieuDe={`Gỡ đợt ngày ${nhanNgayLuyKe(dangGo.date)}`}
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

        {coGhi && (
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
    </section>
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
  dangGui,
  moGo,
  children,
}: {
  method: string;
  cot: readonly finance_cotRa[];
  donVi: DonViHien;
  danhSach: TrangThaiDot;
  coXacNhan: boolean;
  dangGui: boolean;
  moGo: (d: finance_dotRa) => void;
  children?: ReactNode;
}) {
  // Cách tính theo DANH SÁCH vừa đọc nếu có — mới hơn bản của bảng lúc mở hộp.
  const methodHien = danhSach.pha === "xong" ? danhSach.duLieu.method : method;

  return (
    <>
      <p className="mo-ta-trang">{MO_TA_HOP_DOT}</p>
      <p className="ghi-chu">{cauDieuKienDot(methodHien)}</p>
      <p className="ghi-chu">Số tiền theo đơn vị của bảng: {donVi.nhan}.</p>

      {children}

      <h4>Các đợt đã ghi</h4>
      {danhSach.pha === "dangTai" && <p role="status">Đang tải các đợt…</p>}
      {danhSach.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {danhSach.thongBao}
        </p>
      )}
      {danhSach.pha === "xong" && danhSach.duLieu.entries.length === 0 && (
        <p className="trang-thai-rong">{DOT_TRONG}</p>
      )}
      {danhSach.pha === "xong" && danhSach.duLieu.entries.length > 0 && (
        <div className="bang-cuon" role="region" aria-label="Các đợt đã ghi" tabIndex={0}>
          <table className="bang-danh-muc">
            <thead>
              <tr>
                <th scope="col">Ngày</th>
                <th scope="col">Nội dung</th>
                <th scope="col">Đơn vị, cá nhân</th>
                <th scope="col">Số chứng từ</th>
                {cot.map((c) => (
                  <th key={c.id} scope="col">
                    {c.name}
                  </th>
                ))}
                {coXacNhan && <th scope="col">Thao tác</th>}
              </tr>
            </thead>
            <tbody>
              {danhSach.duLieu.entries.map((d) => (
                <tr key={d.id}>
                  <td>{nhanNgayLuyKe(d.date)}</td>
                  <td>{d.content}</td>
                  {/* ĐÃ CHE Ở MÁY CHỦ — in nguyên, không khôi phục (luật 3). */}
                  <td>{d.counterparty !== undefined && d.counterparty !== "" ? d.counterparty : O_TRONG}</td>
                  <td>{d.document_no !== undefined && d.document_no !== "" ? d.document_no : O_TRONG}</td>
                  {cot.map((c) => (
                    <td key={c.id}>{nhanSoTien(d.values[c.id] ?? null, donVi.ma)}</td>
                  ))}
                  {coXacNhan && (
                    <td className="o-thao-tac">
                      <button
                        type="button"
                        className="nut-phu"
                        disabled={dangGui}
                        aria-label={`Gỡ đợt ngày ${nhanNgayLuyKe(d.date)}`}
                        onClick={() => moGo(d)}
                      >
                        🗑 Gỡ đợt
                      </button>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
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
    <form
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
            gia,
          },
          bieuMau,
        );
      }}
    >
      <h4>Ghi một đợt</h4>
      <p>
        <label htmlFor="dot-date">Ngày</label>{" "}
        <input
          id="dot-date"
          name="date"
          className="o-nhap"
          type="date"
          required
          defaultValue={ngayMacDinh}
        />
      </p>
      <p>
        <label htmlFor="dot-content">Nội dung</label>{" "}
        <input
          id="dot-content"
          name="content"
          className="o-nhap"
          type="text"
          required
          maxLength={DO_DAI_TOI_DA_DOT.content}
          placeholder="Thu tiền sử dụng đất đợt 2"
        />
      </p>
      <p>
        <label htmlFor="dot-counterparty">Đơn vị, cá nhân</label>{" "}
        <input
          id="dot-counterparty"
          name="counterparty"
          className="o-nhap"
          type="text"
          maxLength={DO_DAI_TOI_DA_DOT.counterparty}
          autoComplete="off"
        />
      </p>
      <p>
        <label htmlFor="dot-document-no">Số chứng từ</label>{" "}
        <input
          id="dot-document-no"
          name="document_no"
          className="o-nhap"
          type="text"
          maxLength={DO_DAI_TOI_DA_DOT.document_no}
        />
      </p>
      {cot.map((c) => (
        <p key={c.id}>
          <label htmlFor={`dot-gia-${c.id}`}>
            {c.name} ({donVi.nhan.toLowerCase()})
          </label>{" "}
          <input
            id={`dot-gia-${c.id}`}
            name={`dot-gia:${c.id}`}
            className="o-nhap"
            type="text"
            inputMode="decimal"
            autoComplete="off"
          />
        </p>
      ))}
      <button type="submit" className="nut-chinh" disabled={dangGui}>
        + Ghi đợt
      </button>
    </form>
  );
}
