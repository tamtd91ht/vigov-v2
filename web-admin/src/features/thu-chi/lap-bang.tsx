"use client";

import { useState } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { KetQua } from "@/lib/api/goi";
import type { finance_cotVao } from "@/lib/api/schema.gen";
import { taoBang, type LoaiBang, type TaoBangVao } from "@/lib/api/thu-chi";

import {
  boCotKhoiDiem,
  DON_VI_KHOI_DIEM,
  DON_VI_TINH,
  laMaDonVi,
  nhanLoaiBang,
  vaiTroChoLoai,
  type KetQuaDung,
} from "./nhan-thu-chi";

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
 * TIÊU ĐỀ DO NGƯỜI LẬP GÕ, KHÔNG ĐIỀN SẴN. `BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM
 * 2026` là chữ của MỘT xã, và một chuỗi như thế điền sẵn trong bundle là đúng thứ luật 1 bất biến
 * 10 cấm: một bundle phục vụ mọi xã.
 *
 * ĐƠN VỊ TÍNH LÀ MỘT Ô CHỌN BA MÃ, không phải chữ tự do: máy chủ chỉ nhận `dong` | `nghin-dong` |
 * `trieu-dong` và trả 400 cho mọi chữ khác, kể cả "Triệu đồng" (`domain.KiemTraDonViTinh`). Đơn vị
 * là hằng của nền tảng, không phải giá trị riêng của xã. Mốc luỹ kế và đơn vị sửa được về sau ở
 * "Sửa thông tin bảng".
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
  const [loi, datLoi] = useState<string | null>(null);

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
        const dung = dungThanTaoBang({
          nam,
          loai,
          tieuDe: String(fd.get("title") ?? ""),
          donVi: String(fd.get("unit") ?? ""),
          luyKe: String(fd.get("cumulative_to") ?? ""),
          cot,
        });
        if (!dung.ok) {
          datLoi(dung.thongBao);
          return;
        }
        datLoi(null);
        datDangGui(true);
        taoBang(dung.than, khoaChongTrung).then(xong);
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
        <select id="lap-unit" name="unit" required defaultValue={DON_VI_KHOI_DIEM}>
          {DON_VI_TINH.map((d) => (
            <option key={d.ma} value={d.ma}>
              {d.nhan}
            </option>
          ))}
        </select>
      </p>
      <p className="ghi-chu">
        Số liệu luôn lưu bằng đồng. Đơn vị tính chỉ quyết định cách hiện và cách gõ số trên bảng, và
        đổi được về sau mà không con số nào bị quy đổi.
      </p>

      <p>
        <label htmlFor="lap-cumulative">Luỹ kế đến (sửa được về sau)</label>{" "}
        <input id="lap-cumulative" name="cumulative_to" className="o-nhap" type="date" />
      </p>

      <h4>Cột của bảng</h4>
      <p className="ghi-chu">
        Cột đánh dấu vai trò là cột hai chỉ số của năm đọc số từ đó. Bảng lập thiếu vai trò thì
        `Thu đạt dự toán` và `Chi đạt dự toán` trống suốt năm.
      </p>

      <ColumnFieldsets kind={loai} columns={cot} onChange={datCot} />

      <p>
        <button
          type="button"
          className="nut-phu"
          onClick={() => datCot([...cot, { name: "", order: cot.length + 1, type: "so" }])}
        >
          Thêm cột
        </button>
      </p>

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

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

/**
 * Các ô khai báo từng cột của biểu mẫu lập bảng. Tách khỏi `LapBang` để kiểm được bằng một lần dựng
 * tĩnh: `LapBang` chỉ hiện biểu mẫu sau một lần bấm, và bộ kiểm không có DOM để bấm.
 */
export function ColumnFieldsets({
  kind,
  columns,
  onChange,
}: {
  kind: LoaiBang;
  columns: readonly finance_cotVao[];
  onChange: (next: readonly finance_cotVao[]) => void;
}) {
  return (
    <>
      {columns.map((c, i) => (
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
              onChange={(e) => onChange(doiCot(columns, i, { name: e.target.value }))}
            />
          </p>
          <p>
            <label htmlFor={`cot-kieu-${i}`}>Kiểu cột</label>{" "}
            <select
              id={`cot-kieu-${i}`}
              value={c.type}
              onChange={(e) => onChange(changeColumnType(columns, i, e.target.value))}
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
                onChange={(e) => onChange(doiCot(columns, i, { role: e.target.value }))}
              >
                <option value="">Cột thường</option>
                {vaiTroChoLoai(kind).map((v) => (
                  <option key={v.ma} value={v.ma}>
                    {v.nhan}
                  </option>
                ))}
              </select>
            </p>
          ) : (
            <>
              <OperandSelect
                id={`cot-tuso-${i}`}
                label="Tử số"
                columns={columns}
                value={c.numerator_index}
                onChange={(v) => onChange(doiCot(columns, i, { numerator_index: v }))}
              />
              <OperandSelect
                id={`cot-mauso-${i}`}
                label="Mẫu số"
                columns={columns}
                value={c.denominator_index}
                onChange={(v) => onChange(doiCot(columns, i, { denominator_index: v }))}
              />
              <p className="ghi-chu">
                Tỷ lệ của từng dòng = Tử số / Mẫu số × 100, do hệ thống tính từ số liệu của chính
                dòng đó. Mẫu số bằng 0 hoặc để trống thì ô hiện “Không tính được”.
              </p>
            </>
          )}

          <button
            type="button"
            className="nut-phu"
            onClick={() => onChange(removeColumnAt(columns, i))}
          >
            Bỏ cột {i + 1}
          </button>
        </fieldset>
      ))}
    </>
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

/**
 * Ô chọn MỘT toán hạng của cột `%` — chỉ liệt kê cột SỐ của biểu mẫu, theo tên.
 *
 * GIÁ TRỊ LÀ VỊ TRÍ của cột trong mảng biểu mẫu đang giữ, và mảng ấy được gửi đi NGUYÊN THỨ TỰ
 * (`dungThanTaoBang` không lọc, không sắp lại), nên vị trí ở đây là đúng `numerator_index` /
 * `denominator_index` máy chủ đọc.
 */
function OperandSelect({
  id,
  label,
  columns,
  value,
  onChange,
}: {
  id: string;
  label: string;
  columns: readonly finance_cotVao[];
  value: number | null | undefined;
  onChange: (v: number | null) => void;
}) {
  return (
    <p>
      <label htmlFor={id}>{label}</label>{" "}
      <select
        id={id}
        required
        value={value === null || value === undefined ? "" : String(value)}
        onChange={(e) => onChange(e.target.value === "" ? null : Number(e.target.value))}
      >
        <option value="">— Chọn cột số —</option>
        {columns.map((c, j) =>
          c.type === "so" ? (
            <option key={j} value={String(j)}>
              {c.name.trim() === "" ? `Cột ${j + 1}` : c.name}
            </option>
          ) : null,
        )}
      </select>
    </p>
  );
}

/**
 * Bỏ cột thứ `index`, và DỜI mọi toán hạng trỏ ra sau nó.
 *
 * Toán hạng là VỊ TRÍ, nên bỏ một cột đứng trước làm mọi vị trí phía sau lùi một: không dời thì cột
 * `%` lặng lẽ chia hai cột KHÁC hai cột cán bộ đã chọn — một tỷ lệ sai trông y hệt tỷ lệ đúng. Toán
 * hạng trỏ ĐÚNG vào cột bị bỏ thì bị xoá trắng, để cán bộ phải chọn lại chứ không bị chọn thay.
 */
export function removeColumnAt(
  columns: readonly finance_cotVao[],
  index: number,
): readonly finance_cotVao[] {
  const shift = (v: number | null | undefined): number | null | undefined => {
    if (v === null || v === undefined) return v;
    if (v === index) return null;
    return v > index ? v - 1 : v;
  };
  return columns
    .filter((_, j) => j !== index)
    .map((c) =>
      c.type === "phan_tram"
        ? { ...c, numerator_index: shift(c.numerator_index), denominator_index: shift(c.denominator_index) }
        : c,
    );
}

/**
 * Đổi kiểu cột thứ `index`, bỏ trường của kiểu cũ: một cột `so` mang toán hạng, hay một cột `%` mang
 * vai trò, là 400. Cột vừa thôi là cột số thì mọi cột `%` đang lấy nó làm toán hạng được xoá trắng
 * toán hạng ấy — máy chủ từ chối lấy cột phần trăm làm tử số hay mẫu số.
 */
export function changeColumnType(
  columns: readonly finance_cotVao[],
  index: number,
  type: string,
): readonly finance_cotVao[] {
  return columns.map((c, j) => {
    if (j === index) {
      return type === "phan_tram"
        ? { name: c.name, order: c.order, type, numerator_index: null, denominator_index: null }
        : { name: c.name, order: c.order, type, role: c.role };
    }
    if (type !== "so" && c.type === "phan_tram") {
      return {
        ...c,
        numerator_index: c.numerator_index === index ? null : c.numerator_index,
        denominator_index: c.denominator_index === index ? null : c.denominator_index,
      };
    }
    return c;
  });
}

/**
 * Câu lỗi cho toán hạng của cột `%` thứ `index`, hoặc `null` khi đủ. CHỈ ĐỂ TIỆN cho cán bộ — máy
 * chủ kiểm lại đúng ba điều này và trả 400 (`domain.AssignOperandsByIndex`, `checkOperandSet`).
 */
export function percentOperandError(
  columns: readonly finance_cotVao[],
  index: number,
): string | null {
  const c = columns[index];
  if (c === undefined || c.type !== "phan_tram") return null;
  const label = `Cột ${index + 1}${c.name.trim() === "" ? "" : ` (${c.name.trim()})`}`;
  const n = c.numerator_index;
  const d = c.denominator_index;
  if (n === null || n === undefined || d === null || d === undefined) {
    return `${label}: chọn cả cột tử số và cột mẫu số.`;
  }
  if (n === d) return `${label}: cột tử số và cột mẫu số phải là hai cột khác nhau.`;
  if (columns[n]?.type !== "so" || columns[d]?.type !== "so") {
    return `${label}: tử số và mẫu số phải là cột số, không lấy cột phần trăm.`;
  }
  return null;
}

/**
 * Dựng thân `POST /budget-sheets` từ biểu mẫu.
 *
 * ĐƠN VỊ PHẢI LÀ MỘT TRONG BA MÃ, và một giá trị khác bị TỪ CHỐI chứ không bị thay bằng mã khởi
 * điểm: thay thầm là chọn hệ số chia cho mọi con số của bảng thay cán bộ.
 */
export function dungThanTaoBang(nhap: {
  nam: number;
  loai: LoaiBang;
  tieuDe: string;
  donVi: string;
  luyKe: string;
  cot: readonly finance_cotVao[];
}): KetQuaDung<TaoBangVao> {
  if (!laMaDonVi(nhap.donVi)) return { ok: false, thongBao: "Chọn đơn vị tính của bảng." };
  for (let i = 0; i < nhap.cot.length; i++) {
    const error = percentOperandError(nhap.cot, i);
    if (error !== null) return { ok: false, thongBao: error };
  }
  const luyKe = nhap.luyKe.trim();
  return {
    ok: true,
    than: {
      year: nhap.nam,
      kind: nhap.loai,
      title: nhap.tieuDe,
      unit: nhap.donVi,
      // Chuỗi rỗng KHÔNG được gửi: máy chủ phân giải `cumulative_to` theo khuôn YYYY-MM-DD và
      // một chuỗi rỗng đi vào đó là 400, ngay ở lần lập bảng đầu tiên của xã.
      cumulative_to: luyKe === "" ? undefined : luyKe,
      // MẢNG GỬI ĐI CÙNG THỨ TỰ, CÙNG ĐỘ DÀI với mảng biểu mẫu — không lọc, không sắp lại. Toán hạng
      // của cột `%` là VỊ TRÍ trong chính mảng này; một phép lọc chen vào đây làm mọi vị trí sau nó
      // trỏ lệch một cột, và máy chủ không có cách nào biết cán bộ đã định chọn cột nào.
      columns: nhap.cot.map((c, i) => ({
        name: c.name,
        order: i + 1,
        type: c.type,
        // Máy chủ đòi hai toán hạng trên cột `phan_tram` và TỪ CHỐI chúng trên cột `so`; `role` thì
        // ngược lại (`KiemTraCot`, `ErrVaiTroTrenCotPhanTram`). Dựng đúng hình dạng ấy ở đây để cán
        // bộ không phải học các quy tắc của máy chủ qua từng lần 400.
        //
        // `formula` KHÔNG GỬI: máy chủ tự viết chú thích "<tên tử số> / <tên mẫu số> × 100" từ tên
        // hai cột (`domain.PercentFormula`). Đặc tả không đòi xã gõ công thức (§4, §9), và một ô gõ
        // tự do cạnh hai ô chọn là hai lời nói về cùng một phép chia — lệch nhau ngay lần gõ nhầm.
        numerator_index: c.type === "phan_tram" ? c.numerator_index : undefined,
        denominator_index: c.type === "phan_tram" ? c.denominator_index : undefined,
        role: c.type === "so" && c.role !== "" ? c.role : undefined,
      })),
    },
  };
}
