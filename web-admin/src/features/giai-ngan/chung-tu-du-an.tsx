"use client";

import { useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { KetQua } from "@/lib/api/goi";
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
  lopTrangThaiChungTu,
  NOI_DUNG_CHUNG_TU_TOI_DA,
  nhanMocKhoa,
  nhanTrangThaiChungTu,
  SO_CHUNG_TU_TOI_DA,
  thaoTacChungTu,
  thanSuaChungTu,
  thanThemChungTu,
  type GiaTriFormChungTu,
} from "./nhan-ghi-giai-ngan";

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
    <form className="form-danh-muc" onSubmit={guiNgay} aria-label={tieuDeForm}>
      <h4>{tieuDeForm}</h4>

      {trangThaiHienTai === CHUNG_TU_DA_XAC_NHAN && (
        <p className="canh-bao-pham-vi">{CANH_BAO_SUA_VE_NHAP}</p>
      )}

      <div className="o-nhap">
        <label htmlFor="ngay-chi-chung-tu">Ngày chi *</label>
        <input
          id="ngay-chi-chung-tu"
          name="ngay-chi-chung-tu"
          type="date"
          value={gt.ngayChi}
          onChange={(e) => datGT({ ...gt, ngayChi: e.target.value })}
        />
        {/* HAI NGÀY KHÁC NHAU, và xã nhập chứng từ tuần trước vào sáng thứ Hai là chuyện thường —
            mọi biểu đồ luỹ kế của §4 xếp theo ngày CHI, không theo ngày gõ. */}
        <p className="ghi-chu">Ngày tiền thực sự chi ra, không phải ngày gõ vào sổ.</p>
      </div>

      <div className="o-nhap">
        <label htmlFor="so-tien-chung-tu">Số tiền (đồng) *</label>
        <input
          id="so-tien-chung-tu"
          name="so-tien-chung-tu"
          value={gt.soTien}
          inputMode="numeric"
          autoComplete="off"
          placeholder="30.000.000"
          onChange={(e) => datGT({ ...gt, soTien: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="noi-dung-chung-tu">Nội dung *</label>
        <textarea
          id="noi-dung-chung-tu"
          name="noi-dung-chung-tu"
          rows={2}
          value={gt.noiDung}
          maxLength={NOI_DUNG_CHUNG_TU_TOI_DA}
          placeholder="Thanh toán đợt 3"
          onChange={(e) => datGT({ ...gt, noiDung: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="doi-tac-chung-tu">Đối tác</label>
        <input
          id="doi-tac-chung-tu"
          name="doi-tac-chung-tu"
          value={gt.doiTac}
          maxLength={DOI_TAC_TOI_DA}
          autoComplete="off"
          onChange={(e) => datGT({ ...gt, doiTac: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="so-chung-tu">Số chứng từ</label>
        <input
          id="so-chung-tu"
          name="so-chung-tu"
          value={gt.soChungTu}
          maxLength={SO_CHUNG_TU_TOI_DA}
          autoComplete="off"
          onChange={(e) => datGT({ ...gt, soChungTu: e.target.value })}
        />
      </div>

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          Huỷ
        </button>
        <button type="submit" className="nut-chinh" disabled={dangGui}>
          Lưu chứng từ
        </button>
      </div>
    </form>
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
    return (
      <p className="trang-thai-rong">
        Chưa có chứng từ nào trong phiên làm việc này. Bảng này không đọc được chứng từ đã lưu từ
        trước — hợp đồng chưa có tuyến đọc danh sách chứng từ.
      </p>
    );
  }

  return (
    <div className="bang-cuon" role="region" aria-label="Chứng từ giải ngân vừa ghi" tabIndex={0}>
      <table className="bang-danh-muc">
        <caption className="an-thi-giac">
          Chứng từ giải ngân đã ghi hoặc đã đổi trạng thái trong phiên làm việc này
        </caption>
        <thead>
          <tr>
            <th scope="col">Ngày chi</th>
            <th scope="col">Số tiền</th>
            <th scope="col">Nội dung</th>
            <th scope="col">Chứng từ</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Thao tác</th>
          </tr>
        </thead>
        <tbody>
          {ds.map((ct) => {
            const cho = thaoTacChungTu(ct.status);
            return (
              <tr key={ct.id}>
                <td>{nhanNgay(ct.payment_date)}</td>
                {/* IN ĐÚNG CON SỐ MÁY CHỦ TRẢ. Không đổi đơn vị, không làm tròn: một con số sai ở
                    đây đi thẳng vào báo cáo ngân sách. */}
                <td>{nhanTien(ct.amount)}</td>
                <td>
                  {ct.description}
                  {ct.counterparty !== undefined && ct.counterparty !== "" && (
                    <span className="dong-phu">{ct.counterparty}</span>
                  )}
                </td>
                <td>{ct.voucher_no === undefined || ct.voucher_no === "" ? DAU_GACH : ct.voucher_no}</td>
                <td>
                  <span className={lopTrangThaiChungTu(ct.status)}>
                    {nhanTrangThaiChungTu(ct.status)}
                  </span>
                  {/* MỐC KHOÁ VÀ SỐ LẦN MỞ KHOÁ ĐỀU HIỆN, và `unlock_count` là con số nói rằng đã
                      có những lần mở khoá KHÁC — bốn trường mở khoá chỉ mô tả lần gần nhất. */}
                  {ct.locked_at !== undefined && ct.locked_at !== "" && (
                    <span className="dong-phu">Khoá lúc {nhanMocKhoa(ct.locked_at)}</span>
                  )}
                  {ct.unlock_count > 0 && (
                    <span className="dong-phu">Đã mở khoá {ct.unlock_count} lần</span>
                  )}
                </td>
                <td className="o-thao-tac">
                  {coGhi && cho.sua && (
                    <button
                      type="button"
                      className="nut-phu"
                      disabled={dangGui}
                      onClick={() => moSua(ct.id)}
                      aria-label={`✎ Sửa chứng từ ngày ${nhanNgay(ct.payment_date)}`}
                    >
                      ✎ Sửa
                    </button>
                  )}
                  {coXacNhan && cho.xacNhan && (
                    <button
                      type="button"
                      className="nut-chinh"
                      disabled={dangGui}
                      onClick={() => xacNhan(ct.id)}
                      aria-label={`Xác nhận chứng từ ngày ${nhanNgay(ct.payment_date)}`}
                    >
                      Xác nhận
                    </button>
                  )}
                  {coXacNhan && cho.khoa && (
                    <button
                      type="button"
                      className="nut-phu"
                      disabled={dangGui}
                      onClick={() => khoa(ct.id)}
                      aria-label={`Khoá chứng từ ngày ${nhanNgay(ct.payment_date)}`}
                      title={CANH_BAO_KHOA}
                    >
                      Khoá
                    </button>
                  )}
                  {coXacNhan && cho.moKhoa && (
                    <button
                      type="button"
                      className="nut-phu"
                      disabled={dangGui}
                      onClick={() => moMoKhoa(ct.id)}
                      aria-label={`Mở khoá chứng từ ngày ${nhanNgay(ct.payment_date)}`}
                    >
                      Mở khoá
                    </button>
                  )}
                  {coXacNhan && cho.go && (
                    <button
                      type="button"
                      className="nut-xoa"
                      disabled={dangGui}
                      onClick={() => moGo(ct.id)}
                      aria-label={`🗑 Gỡ chứng từ ngày ${nhanNgay(ct.payment_date)}`}
                    >
                      🗑 Gỡ
                    </button>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
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
    <section className="khoi-chi-tiet" aria-labelledby="tieu-de-chung-tu">
      <div className="dau-khoi-chi-tiet">
        <h3 id="tieu-de-chung-tu">Chứng từ giải ngân</h3>
      </div>

      {/* VÒNG ĐỜI NÓI RA NGAY TRÊN BẢNG, không để cán bộ suy từ việc nút nào hiện nút nào không. */}
      <p className="ghi-chu">
        Vòng đời: Kế toán nhập → Đã xác nhận → Đã khoá. Nhập và sửa cần quyền budget.update; xác
        nhận, khoá, mở khoá và gỡ cần quyền budget.confirm.
      </p>

      {coGhi ? (
        <div className="cum-nut">
          <button
            type="button"
            className="nut-chinh"
            aria-expanded={dangMo?.kieu === "them"}
            onClick={() => {
              datLoi(null);
              datDangMo(dangMo?.kieu === "them" ? null : { kieu: "them" });
            }}
          >
            + Ghi nhận khoản chi
          </button>
        </div>
      ) : (
        <p className="trang-thai-rong">{CAU_THIEU_QUYEN_GHI}</p>
      )}

      {!coXacNhan && <p className="trang-thai-rong">{CAU_THIEU_QUYEN_XAC_NHAN}</p>}

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
          tieuDe="Gỡ chứng từ"
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
          tieuDe="Mở khoá chứng từ"
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
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

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
    </section>
  );
}
