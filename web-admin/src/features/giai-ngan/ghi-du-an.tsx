"use client";

import { useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { suaDuAn, themDuAn, xoaDuAn } from "@/lib/api/giai-ngan";
import type { finance_duAnRa, finance_hangMucRa } from "@/lib/api/schema.gen";

import { FormLyDo } from "./form-ly-do";
import {
  CAU_THIEU_QUYEN_GHI,
  CAU_THIEU_QUYEN_XAC_NHAN,
  FORM_DU_AN_TRONG,
  MA_DU_AN_TOI_DA,
  MO_TA_DU_AN_TOI_DA,
  TEN_DU_AN_TOI_DA,
  thanSuaDuAn,
  thanThemDuAn,
  type GiaTriFormDuAn,
} from "./nhan-ghi-giai-ngan";

/**
 * Ba tuyến GHI của dự án đầu tư — §9 (`Thêm dự án`), §8 (`✎ Sửa dự án`), §7 (xoá mềm kèm lý do).
 *
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 * HAI KHOÁ, VÀ CHÚNG KHÔNG PHẢI MỘT: `budget.update` cho THÊM và SỬA, `budget.confirm` cho XOÁ.
 * Bảng phân quyền của hợp đồng chia đôi đúng như thế, và đó là cách xã tách người nhập liệu khỏi
 * người chịu trách nhiệm (§8.2, `06-giai-ngan.md:202`). Một tài khoản chỉ có `budget.update` thấy
 * nút Sửa và KHÔNG thấy nút Gỡ — nếu hai nút cùng hiện ra thì cổng đã gắn nhầm, và không có gì
 * khác trên màn hình nói ra điều đó vì máy chủ chỉ trả 403 vào đúng lúc bấm.
 * ═══════════════════════════════════════════════════════════════════════════════════════════
 *
 * ĐẶC TẢ GỌI NÓ LÀ MODAL. Ở đây nó là một khối nằm trong trang — KHÔNG phải một lớp phủ — vì một
 * lớp phủ cần lớp CSS chưa có trong `globals.css`, và lượt này không được thêm CSS.
 *
 * NÚT XOÁ DỰ ÁN ĐẶT Ở TRANG CHI TIẾT, KHÔNG Ở HÀNG CỦA BẢNG §7. Xoá một dự án là thao tác cần nhìn
 * thấy kế hoạch vốn và số đã giải ngân của nó trước khi bấm; một nút ở cuối hàng trong bảng 63 dòng
 * là một nút bấm nhầm hàng. Tuyến vẫn là tuyến §7 khai, chỉ chỗ đặt nút là quyết định của màn hình.
 */

/**
 * Biểu mẫu dự án §9, dùng cho cả THÊM và SỬA.
 *
 * `khoaChongTrung` SINH LÚC MỞ BIỂU MẪU, không lúc gửi: bấm lại sau một lỗi mạng phải dùng LẠI đúng
 * khoá ấy, vì lần gửi đầu có thể đã tới máy chủ và đã cấp một mã dự án. Biểu mẫu SỬA không cần khoá
 * (hợp đồng không đòi), nhưng vẫn nhận cùng chữ ký để hai lối gọi không rẽ nhánh ở đây.
 */
export function FormDuAn({
  tieuDeForm,
  giaTriDau,
  maChiDoc,
  danhMuc,
  dangGui,
  loi,
  huy,
  luu,
}: {
  tieuDeForm: string;
  giaTriDau: GiaTriFormDuAn;
  /** Mã dự án khi đang SỬA. Hiện để ĐỌC: mã đã cấp thì không đánh lại (luật 7, cấm #4). */
  maChiDoc?: string;
  danhMuc: readonly finance_hangMucRa[];
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (gt: GiaTriFormDuAn, khoaChongTrung: string) => void;
}) {
  const [gt, datGT] = useState<GiaTriFormDuAn>(giaTriDau);
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  // Danh mục hạng mục ship RỖNG cho tới khi có bước khởi tạo xã, và `category_id` là bắt buộc — một
  // biểu mẫu gửi đi lúc ấy chỉ nhận 404 "không tìm thấy hạng mục kế hoạch vốn này trong xã". Nói
  // trước, thay vì để cán bộ gõ xong cả biểu mẫu rồi mới biết.
  const thieuDanhMuc = danhMuc.length === 0;

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (thieuDanhMuc) return;
    luu(gt, khoaChongTrung);
  }

  return (
    <form className="form-danh-muc" onSubmit={guiNgay} aria-label={tieuDeForm}>
      <h3>{tieuDeForm}</h3>

      {thieuDanhMuc && (
        <p className="ghi-chu">
          Danh mục hạng mục kế hoạch vốn của xã đang rỗng, mà mỗi dự án phải thuộc đúng một hạng
          mục. Cần khai hạng mục ở màn Cấu hình (quyền admin.lookup) trước khi thêm dự án.
        </p>
      )}

      {maChiDoc === undefined ? (
        <div className="o-nhap">
          <label htmlFor="ma-du-an">Mã dự án *</label>
          <input
            id="ma-du-an"
            name="ma-du-an"
            value={gt.ma}
            maxLength={MA_DU_AN_TOI_DA}
            autoComplete="off"
            onChange={(e) => datGT({ ...gt, ma: e.target.value })}
          />
          {/* MÃ ĐÃ CẤP THÌ KHÔNG CẤP LẠI, KỂ CẢ KHI DỰ ÁN MANG MÃ ẤY ĐÃ RÚT KHỎI DANH SÁCH — nói
              trước, vì nếu không thì câu 409 của máy chủ đọc như một lỗi trước mặt người vừa xem
              hết danh sách và không thấy mã ấy ở đâu. */}
          <p className="ghi-chu">
            Chỉ gồm chữ cái, chữ số và dấu gạch nối. Hệ thống chưa tự sinh mã. Mã đã cấp thì không
            cấp lại, kể cả khi dự án mang mã đó đã rút khỏi danh sách.
          </p>
        </div>
      ) : (
        <p className="ma-muc">
          Mã dự án: {maChiDoc} — mã đã cấp thì không đánh lại, nên ô này chỉ để đọc.
        </p>
      )}

      <div className="o-chon">
        <label htmlFor="hang-muc-du-an">Hạng mục *</label>
        <select
          id="hang-muc-du-an"
          value={gt.hangMucID}
          disabled={thieuDanhMuc}
          onChange={(e) => datGT({ ...gt, hangMucID: e.target.value })}
        >
          <option value="">— Chọn hạng mục —</option>
          {danhMuc.map((h) => (
            <option key={h.id} value={h.id}>
              {h.label}
            </option>
          ))}
        </select>
        <p className="ghi-chu">
          Báo cáo tiến độ cộng dồn theo hạng mục, nên mỗi dự án thuộc đúng một hạng mục.
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="ten-du-an">Tên dự án *</label>
        <input
          id="ten-du-an"
          name="ten-du-an"
          value={gt.ten}
          maxLength={TEN_DU_AN_TOI_DA}
          autoComplete="off"
          placeholder="Bê tông hoá đường trục chính thôn Hà Lam"
          onChange={(e) => datGT({ ...gt, ten: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="ke-hoach-von-du-an">Số tiền bố trí năm (đồng) *</label>
        {/* Ô CHỮ, KHÔNG PHẢI `type=number`: một ô số của trình duyệt nhận cả `1e9` và cả dấu phẩy
            thập phân theo vùng miền, rồi đưa xuống một giá trị không phải thứ người ta gõ. Đơn vị
            ở đây là ĐỒNG và con số là số nguyên — `docSoTien` đọc đúng một khuôn và từ chối phần
            còn lại bằng một câu. */}
        <input
          id="ke-hoach-von-du-an"
          name="ke-hoach-von-du-an"
          value={gt.keHoachVon}
          inputMode="numeric"
          autoComplete="off"
          placeholder="7.500.000.000"
          onChange={(e) => datGT({ ...gt, keHoachVon: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="tong-muc-du-an">Tổng mức được duyệt cả dự án (đồng)</label>
        <input
          id="tong-muc-du-an"
          name="tong-muc-du-an"
          value={gt.tongMucDuyet}
          inputMode="numeric"
          autoComplete="off"
          onChange={(e) => datGT({ ...gt, tongMucDuyet: e.target.value })}
        />
        <p className="ghi-chu">Để trống thì lấy bằng số tiền bố trí năm nay.</p>
      </div>

      <div className="o-nhap">
        <label htmlFor="ngay-khoi-cong">Ngày khởi công</label>
        <input
          id="ngay-khoi-cong"
          name="ngay-khoi-cong"
          type="date"
          value={gt.ngayKhoiCong}
          onChange={(e) => datGT({ ...gt, ngayKhoiCong: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="ngay-hoan-thanh">Ngày hoàn thành</label>
        <input
          id="ngay-hoan-thanh"
          name="ngay-hoan-thanh"
          type="date"
          value={gt.ngayHoanThanh}
          onChange={(e) => datGT({ ...gt, ngayHoanThanh: e.target.value })}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="thoi-han-giai-ngan">Thời hạn giải ngân</label>
        <input
          id="thoi-han-giai-ngan"
          name="thoi-han-giai-ngan"
          type="date"
          value={gt.thoiHanGiaiNgan}
          onChange={(e) => datGT({ ...gt, thoiHanGiaiNgan: e.target.value })}
        />
        {/* HAI MỐC KHÁC NHAU, CỐ Ý ĐỂ CẠNH NHAU — §9 nói rõ và §8 lặp lại. */}
        <p className="ghi-chu">
          Mốc phải hoàn tất phần vốn của năm. Khác với ngày hoàn thành công trình: công trình xong
          tháng 3 vẫn có thể phải giải ngân trước 31/12. Để trống thì lấy mặc định 31/12.
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="mo-ta-du-an">Mô tả</label>
        <textarea
          id="mo-ta-du-an"
          name="mo-ta-du-an"
          rows={3}
          value={gt.moTa}
          maxLength={MO_TA_DU_AN_TOI_DA}
          onChange={(e) => datGT({ ...gt, moTa: e.target.value })}
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
        <button type="submit" className="nut-chinh" disabled={dangGui || thieuDanhMuc}>
          Lưu dự án
        </button>
      </div>
    </form>
  );
}

/**
 * Khối `+ Thêm dự án` của §2, đặt trên bảng dự án.
 *
 * CỔNG `budget.update`. Thiếu khoá thì KHÔNG có nút, và một câu nói rõ thiếu khoá nào — không phải
 * một nút mờ: một nút mờ và một chức năng không tồn tại là hai câu khác nhau, và ở đây sự thật là
 * câu thứ nhất.
 */
export function KhoiThemDuAn({
  nam,
  danhMuc,
  coGhi,
  daGhiXong,
}: {
  nam: number;
  danhMuc: readonly finance_hangMucRa[];
  coGhi: boolean;
  daGhiXong: () => void;
}) {
  const [dangMo, datDangMo] = useState(false);
  const [dangGui, datDangGui] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  // Đếm số lần GHI THÀNH CÔNG. Nó đi vào `key` của biểu mẫu, nên một lần ghi xong là một lần biểu
  // mẫu dựng lại từ đầu: các ô trống trở lại VÀ một khoá chống trùng mới được sinh. Lần ghi HỎNG
  // thì không tăng — biểu mẫu giữ nguyên chữ đã gõ và giữ nguyên khoá cũ, đúng điều
  // `Idempotency-Key` sinh ra để làm.
  const [lanGhiXong, datLanGhiXong] = useState(0);

  if (!coGhi) return <p className="trang-thai-rong">{CAU_THIEU_QUYEN_GHI}</p>;

  function them(gt: GiaTriFormDuAn, khoa: string): void {
    const than = thanThemDuAn(nam, gt);
    if (!than.ok) {
      datLoi(than.cau);
      return;
    }
    datDangGui(true);
    themDuAn(than.than, khoa).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ. 409 của tuyến này nói rõ vì sao một mã không thấy trên màn hình
        // vẫn bị coi là đã dùng, và 404 của nó gọi đúng tên trường sai.
        datLoi(kq.thongBao);
        return;
      }
      datLoi(null);
      datDangMo(false);
      datLanGhiXong((n) => n + 1);
      daGhiXong();
    });
  }

  return (
    <div className="cum-nut">
      <button
        type="button"
        className="nut-chinh"
        aria-expanded={dangMo}
        onClick={() => {
          datLoi(null);
          datDangMo(!dangMo);
        }}
      >
        + Thêm dự án
      </button>

      {dangMo && (
        <FormDuAn
          key={`them-du-an|${lanGhiXong}`}
          tieuDeForm={`Thêm dự án — năm ngân sách ${nam}`}
          giaTriDau={FORM_DU_AN_TRONG}
          danhMuc={danhMuc}
          dangGui={dangGui}
          loi={loi}
          huy={() => {
            datDangMo(false);
            datLoi(null);
          }}
          luu={them}
        />
      )}
    </div>
  );
}

/** Các ô của biểu mẫu SỬA, đổ từ dự án máy chủ vừa trả về. */
export function giaTriTuDuAn(duAn: finance_duAnRa): GiaTriFormDuAn {
  return {
    ma: duAn.code,
    hangMucID: duAn.category_id,
    ten: duAn.name,
    moTa: duAn.description ?? "",
    // SỐ THÔ, KHÔNG ĐỊNH DẠNG: ô nhập phải chứa đúng con số máy chủ giữ, để một lần Lưu không đổi
    // gì thật sự KHÔNG đổi gì. Đưa `100.000.000` vào ô rồi đọc lại vẫn ra đúng số ấy nhờ
    // `docSoTien` bỏ dấu chấm, nhưng phép so "có đổi không" sẽ thấy khác và gửi đi một `PATCH`
    // thừa — thứ có thể bóc chữ xác nhận của một chứng từ ở tuyến bên cạnh, và ở đây là một lần
    // ghi vào hồ sơ lưu trữ không ai yêu cầu.
    keHoachVon: String(duAn.planned_amount),
    tongMucDuyet: String(duAn.approved_amount),
    ngayKhoiCong: duAn.start_date ?? "",
    ngayHoanThanh: duAn.completion_date ?? "",
    thoiHanGiaiNgan: duAn.disbursement_deadline,
  };
}

/**
 * Hai nút của trang chi tiết: `✎ Sửa dự án` (§8, `budget.update`) và `🗑 Gỡ dự án`
 * (§7, `budget.confirm`).
 *
 * HAI CỔNG RIÊNG, KHÔNG MỘT. Đây là chỗ dễ gắn nhầm nhất của phân hệ: gộp hai khoá lại thì người
 * chỉ được nhập liệu bỗng xoá được cả một dự án khỏi mọi con số tổng của năm.
 */
export function KhoiSuaXoaDuAn({
  duAn,
  danhMuc,
  coGhi,
  coXacNhan,
  daSuaXong,
  daXoaXong,
}: {
  duAn: finance_duAnRa;
  danhMuc: readonly finance_hangMucRa[];
  coGhi: boolean;
  coXacNhan: boolean;
  daSuaXong: () => void;
  daXoaXong: () => void;
}) {
  const [dangMo, datDangMo] = useState<"sua" | "xoa" | null>(null);
  const [dangGui, datDangGui] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);

  function sua(gt: GiaTriFormDuAn): void {
    const than = thanSuaDuAn(giaTriTuDuAn(duAn), gt);
    if (!than.ok) {
      datLoi(than.cau);
      return;
    }
    datDangGui(true);
    suaDuAn(duAn.id, than.than).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoi(kq.thongBao);
        return;
      }
      datLoi(null);
      datDangMo(null);
      // ĐỌC LẠI DỰ ÁN chứ không vá bằng phản hồi: `duAnGhiRa` cố ý không mang `disbursed_amount`,
      // `disbursed_ratio`, `delay_score` hay `is_delayed` — tuyến ghi không đọc chúng. Vá bằng nó
      // sẽ đặt bốn con số 0 lên màn hình, và chúng trông y hệt số thật.
      daSuaXong();
    });
  }

  function xoa(lyDo: string): void {
    datDangGui(true);
    xoaDuAn(duAn.id, lyDo).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // 409 "Dự án này còn chứng từ giải ngân nên chưa xoá được…" ra NGUYÊN VĂN: không có câu ấy
        // thì màn hình trông như hỏng — dự án nằm ngay đó và nút Gỡ không làm gì.
        datLoi(kq.thongBao);
        return;
      }
      datLoi(null);
      datDangMo(null);
      daXoaXong();
    });
  }

  return (
    <div className="cum-nut">
      {coGhi ? (
        <button
          type="button"
          className="nut-phu"
          aria-expanded={dangMo === "sua"}
          onClick={() => {
            datLoi(null);
            datDangMo(dangMo === "sua" ? null : "sua");
          }}
        >
          ✎ Sửa dự án
        </button>
      ) : (
        <p className="trang-thai-rong">{CAU_THIEU_QUYEN_GHI}</p>
      )}

      {coXacNhan ? (
        <button
          type="button"
          className="nut-xoa"
          aria-expanded={dangMo === "xoa"}
          onClick={() => {
            datLoi(null);
            datDangMo(dangMo === "xoa" ? null : "xoa");
          }}
        >
          🗑 Gỡ dự án
        </button>
      ) : (
        <p className="trang-thai-rong">{CAU_THIEU_QUYEN_XAC_NHAN}</p>
      )}

      {dangMo === "sua" && coGhi && (
        <FormDuAn
          tieuDeForm={`Sửa dự án: ${duAn.name}`}
          giaTriDau={giaTriTuDuAn(duAn)}
          maChiDoc={duAn.code}
          danhMuc={danhMuc}
          dangGui={dangGui}
          loi={loi}
          huy={() => {
            datDangMo(null);
            datLoi(null);
          }}
          luu={(gt) => sua(gt)}
        />
      )}

      {dangMo === "xoa" && coXacNhan && (
        <FormLyDo
          tieuDe={`Gỡ dự án: ${duAn.name}`}
          moTa={
            "Dự án được xoá MỀM: hàng vẫn còn kèm người xoá và lý do, và mã dự án KHÔNG quay lại " +
            "dãy — nhập lại phải chọn mã khác. Dự án còn chứng từ giải ngân thì máy chủ từ chối, " +
            "phải gỡ từng chứng từ kèm lý do trước."
          }
          nhanNut="Gỡ dự án"
          dangGui={dangGui}
          loi={loi}
          huy={() => {
            datDangMo(null);
            datLoi(null);
          }}
          xacNhan={xoa}
        />
      )}
    </div>
  );
}
