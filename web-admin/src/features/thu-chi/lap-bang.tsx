"use client";

import { useState } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { KetQua } from "@/lib/api/goi";
import type { finance_cotVao } from "@/lib/api/schema.gen";
import { taoBang, type LoaiBang } from "@/lib/api/thu-chi";

import { boCotKhoiDiem, nhanLoaiBang, vaiTroChoLoai } from "./nhan-thu-chi";

/**
 * Biểu mẫu **Lập bảng ngân sách** — thứ thay cho `⬆ Nạp từ Excel` của §6.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO KHÔNG PHẢI MỘT Ô CHỌN TỆP: hợp đồng REST không có tuyến nào nhận tệp. `POST
 * /api/v1/budget-sheets` nhận JSON gồm năm, loại, tiêu đề, đơn vị tính, mốc luỹ kế và **bộ cột**.
 * Vẽ một vùng kéo thả `.xlsx` ở đây là hứa với cán bộ một chức năng không tồn tại — đúng điều
 * `dau-trang.tsx` đã từ chối làm với ô tìm kiếm.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CỘT LÀ DỮ LIỆU, KHÔNG PHẢI LƯỢC ĐỒ (§3, kết luận thiết kế) — nên bộ cột dưới đây **sửa được,
 * thêm được, bớt được**. Giá trị điền sẵn chỉ là biểu mẫu thường gặp của §3.1 và §3.2.
 *
 * TIÊU ĐỀ VÀ ĐƠN VỊ TÍNH DO NGƯỜI LẬP GÕ, KHÔNG ĐIỀN SẴN. `BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ
 * THĂNG BÌNH NĂM 2026` là chữ của MỘT xã, và một chuỗi như thế điền sẵn trong bundle là đúng thứ
 * luật 1 bất biến 10 cấm: một bundle phục vụ mọi xã.
 *
 * MỐC LUỸ KẾ CHỈ ĐẶT ĐƯỢC Ở ĐÂY. Hợp đồng không có tuyến sửa bảng, nên đổi `Luỹ kế đến` về sau
 * nghĩa là gỡ cả bảng kèm lý do rồi lập lại — nói ra trên màn hình, không giấu trong chú thích.
 */
export function LapBang({
  nam,
  loai,
  dangGui,
  datDangGui,
  xong,
}: {
  nam: number;
  loai: LoaiBang;
  dangGui: boolean;
  datDangGui: (b: boolean) => void;
  xong: (kq: KetQua<unknown>) => void;
}) {
  const [mo, datMo] = useState(false);
  const [cot, datCot] = useState<readonly finance_cotVao[]>(() => boCotKhoiDiem(loai, nam));

  /**
   * Khoá chống trùng sinh MỘT LẦN, lúc biểu mẫu mở ra.
   *
   * Tuyến này khai `idem.Required(idem.DongKhiHong)`: Redis hỏng thì nó trả **503** và xã không
   * lập được bảng. Cái giá ấy được chọn vì không có ràng buộc duy nhất nào phân biệt một lần bấm
   * hai lần với một lần lập bảng thứ hai có chủ ý — và hai bảng sống cùng một năm là trạng thái
   * `CoBangConSong` từ chối.
   */
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  if (!mo) {
    return (
      <p>
        <button type="button" className="nut-phu" onClick={() => datMo(true)}>
          Lập bảng {nhanLoaiBang(loai).toLowerCase()} năm {nam}
        </button>
      </p>
    );
  }

  return (
    <form
      className="khoi-chi-tiet"
      onSubmit={(e) => {
        e.preventDefault();
        const fd = new FormData(e.currentTarget);
        const luyKe = String(fd.get("cumulative_to") ?? "").trim();

        datDangGui(true);
        taoBang(
          {
            year: nam,
            kind: loai,
            title: String(fd.get("title") ?? ""),
            unit: String(fd.get("unit") ?? ""),
            // Chuỗi rỗng KHÔNG được gửi: máy chủ phân giải `cumulative_to` theo khuôn YYYY-MM-DD và
            // một chuỗi rỗng đi vào đó là 400, ngay ở lần lập bảng đầu tiên của xã.
            cumulative_to: luyKe === "" ? undefined : luyKe,
            columns: cot.map((c, i) => ({
              name: c.name,
              order: i + 1,
              type: c.type,
              // Máy chủ đòi `formula` trên cột `phan_tram` và TỪ CHỐI nó trên cột `so`; `role` thì
              // ngược lại (`KiemTraCot`, `ErrVaiTroTrenCotPhanTram`). Dựng đúng hình dạng ấy ở đây
              // để cán bộ không phải học hai quy tắc của máy chủ qua hai lần 400.
              formula: c.type === "phan_tram" ? c.formula : undefined,
              role: c.type === "so" && c.role !== "" ? c.role : undefined,
            })),
          },
          khoaChongTrung,
        ).then(xong);
      }}
    >
      <div className="dau-khoi-chi-tiet">
        <h3>
          Lập bảng {nhanLoaiBang(loai).toLowerCase()} năm {nam}
        </h3>
      </div>

      <p className="ghi-chu">
        Hệ thống chưa nhận tệp Excel của Phòng Tài chính: hợp đồng chưa có tuyến nạp tệp. Bộ cột
        dưới đây điền sẵn theo biểu mẫu thường gặp và sửa được — cột là dữ liệu của bảng, không phải
        cấu trúc cố định.
      </p>

      <p>
        <label htmlFor="lap-title">Tiêu đề bảng (in trên đầu báo cáo)</label>{" "}
        <input
          id="lap-title"
          name="title"
          className="o-nhap"
          type="text"
          required
          maxLength={300}
          placeholder="BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ … NĂM …"
        />
      </p>

      <p>
        <label htmlFor="lap-unit">Đơn vị tính</label>{" "}
        <input id="lap-unit" name="unit" className="o-nhap" type="text" required maxLength={50} />
      </p>
      <p className="ghi-chu">
        Đơn vị tính là NHÃN in cạnh các con số. Hệ thống không quy đổi theo nhãn này — số nhập vào
        và số hiện ra là cùng một con số.
      </p>

      <p>
        <label htmlFor="lap-cumulative">Luỹ kế đến (đặt một lần, không sửa lại được)</label>{" "}
        <input id="lap-cumulative" name="cumulative_to" className="o-nhap" type="date" />
      </p>

      <h4>Cột của bảng</h4>
      <p className="ghi-chu">
        Cột đánh dấu vai trò là cột hai chỉ số của năm đọc số từ đó. Bảng lập thiếu vai trò thì
        `Thu đạt dự toán` và `Chi đạt dự toán` trống suốt năm.
      </p>

      {cot.map((c, i) => (
        <fieldset key={i}>
          <legend>Cột {i + 1}</legend>
          <p>
            <label htmlFor={`cot-ten-${i}`}>Tên cột</label>{" "}
            <input
              id={`cot-ten-${i}`}
              className="o-nhap"
              type="text"
              required
              maxLength={200}
              value={c.name}
              onChange={(e) => datCot(doiCot(cot, i, { name: e.target.value }))}
            />
          </p>
          <p>
            <label htmlFor={`cot-kieu-${i}`}>Kiểu cột</label>{" "}
            <select
              id={`cot-kieu-${i}`}
              value={c.type}
              onChange={(e) =>
                datCot(
                  doiCot(cot, i, {
                    type: e.target.value,
                    // Đổi kiểu thì bỏ trường của kiểu cũ: một cột `so` còn `formula` sót lại là 400.
                    formula: e.target.value === "phan_tram" ? (c.formula ?? "") : undefined,
                    role: e.target.value === "so" ? c.role : undefined,
                  }),
                )
              }
            >
              <option value="so">Cột số</option>
              <option value="phan_tram">Cột phần trăm</option>
            </select>
          </p>

          {c.type === "so" ? (
            <p>
              <label htmlFor={`cot-vaitro-${i}`}>Vai trò trong chỉ số</label>{" "}
              <select
                id={`cot-vaitro-${i}`}
                value={c.role ?? ""}
                onChange={(e) => datCot(doiCot(cot, i, { role: e.target.value }))}
              >
                <option value="">Cột thường</option>
                {vaiTroChoLoai(loai).map((v) => (
                  <option key={v.ma} value={v.ma}>
                    {v.nhan}
                  </option>
                ))}
              </select>
            </p>
          ) : (
            <p>
              <label htmlFor={`cot-congthuc-${i}`}>Công thức (bắt buộc với cột phần trăm)</label>{" "}
              <input
                id={`cot-congthuc-${i}`}
                className="o-nhap"
                type="text"
                required
                maxLength={200}
                value={c.formula ?? ""}
                onChange={(e) => datCot(doiCot(cot, i, { formula: e.target.value }))}
              />
            </p>
          )}

          <button
            type="button"
            className="nut-phu"
            onClick={() => datCot(cot.filter((_, j) => j !== i))}
          >
            Bỏ cột {i + 1}
          </button>
        </fieldset>
      ))}

      <p>
        <button
          type="button"
          className="nut-phu"
          onClick={() => datCot([...cot, { name: "", order: cot.length + 1, type: "so" }])}
        >
          Thêm cột
        </button>
      </p>

      <button type="submit" className="nut-chinh" disabled={dangGui || cot.length === 0}>
        Lập bảng
      </button>{" "}
      <button
        type="button"
        className="nut-phu"
        disabled={dangGui}
        onClick={() => {
          datMo(false);
          datCot(boCotKhoiDiem(loai, nam));
        }}
      >
        Huỷ
      </button>
    </form>
  );
}

/** Sửa một cột, trả về MẢNG MỚI. Sửa tại chỗ thì React không thấy gì đổi và bảng đứng yên. */
function doiCot(
  cot: readonly finance_cotVao[],
  chiSo: number,
  sua: Partial<finance_cotVao>,
): readonly finance_cotVao[] {
  return cot.map((c, i) => (i === chiSo ? { ...c, ...sua } : c));
}
