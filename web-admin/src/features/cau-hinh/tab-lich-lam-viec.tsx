"use client";

import { useEffect, useState } from "react";

import { ChonNam } from "@/components/chon-nam";
import type { KetQua } from "@/lib/api/goi";
import { layLichLamViec, layNgayLamBu, layNgayNghiLe } from "@/lib/api/lich-lam-viec";
import type {
  identity_danhSachCaLamBuRa,
  identity_danhSachCaLamViecRa,
  identity_danhSachNgayNghiLeRa,
} from "@/lib/api/schema.gen";
import { namTheoDongHoMay } from "@/lib/nam";

import {
  DAN_LICH_LAM_VIEC,
  GHI_CHU_CHI_XEM_LICH,
  LICH_TUAN_RONG,
  nhanCa,
  nhanNgay,
  nhanNgayLamBuRong,
  nhanNgayNghiRong,
  tenThu,
} from "./nhan-lich-lam-viec";

/**
 * Phần "Lịch làm việc của xã" trên màn Cấu hình — ba bảng, ba tuyến
 * (`GET /api/v1/working-hours`, `/public-holidays?year=`, `/swap-working-days?year=`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐÂY LÀ CẤU HÌNH THEO TỪNG XÃ, KHÔNG PHẢI HẰNG SỐ HỆ THỐNG — và cách trình bày phải nói ra
 * điều đó, vì nó quyết định cán bộ có nghĩ tới việc sửa nó hay không. Ba bảng này là nền của
 * cách đếm hạn xử lý theo GIỜ LÀM VIỆC (ADR 0007, luật 10 bất biến 4).
 *
 * KHÔNG MỘT PHÉP TÍNH HẠN NÀO Ở PHÍA WEB: `identity` sở hữu cả ba bảng và phép cộng giờ làm
 * việc. Ở đây chỉ hiện những dòng máy chủ trả về và những câu máy chủ viết.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ CỔNG QUYỀN, và đó là điều máy chủ quyết chứ không phải đây: cả ba tuyến khai
 * `any-authenticated`, có lý do ghi thẳng trong hợp đồng — giờ làm việc nằm dưới MỌI hạn hiện
 * trên màn hình, nên đòi một quyền cấu hình sẽ làm hỏng những màn hình đó cho mọi tài khoản
 * không phải quản trị. Dựng một cổng ở giao diện sẽ là để GIAO DIỆN từ chối điều máy chủ không
 * từ chối, và hiện một câu sai cho người mà máy chủ vẫn đang phục vụ (luật 5, cấm #1).
 *
 * KHÔNG CÓ BỀ MẶT GHI NÀO: cả ba tuyến chỉ đọc, và máy chủ nói rõ vì sao chưa có tuyến ghi.
 */

type TrangThai<T> = { pha: "dangTai" } | { pha: "loi"; thongBao: string } | { pha: "xong"; duLieu: T };

/**
 * Kết quả đã tải + năm sinh ra nó → trạng thái của bảng. Kết quả của năm khác đọc thành "đang
 * tải", KHÔNG đọc thành dữ liệu của năm đang chọn.
 */
function theoNam<T>(daTai: { nam: number; kq: KetQua<T> } | null, nam: number): TrangThai<T> {
  if (daTai === null || daTai.nam !== nam) return { pha: "dangTai" };
  return daTai.kq.ok
    ? { pha: "xong", duLieu: daTai.kq.duLieu }
    : { pha: "loi", thongBao: daTai.kq.thongBao };
}

export function TabLichLamViec() {
  const [namGoc] = useState(namTheoDongHoMay);
  const [nam, datNam] = useState(namGoc);

  const [tuan, datTuan] = useState<TrangThai<identity_danhSachCaLamViecRa>>({ pha: "dangTai" });

  /**
   * Hai bảng theo năm giữ kết quả KÈM NĂM đã sinh ra nó, và "đang tải" được SUY RA từ chỗ năm
   * lưu khác năm đang chọn.
   *
   * Không chỉ để hết lỗi lint: đặt `dangTai` bằng một `setState` trong thân effect để lại một
   * cửa sổ, dù hẹp, ở đó bảng ngày nghỉ của năm cũ vẫn đứng dưới một ô chọn đã hiện năm mới —
   * và lịch nghỉ lễ sai năm là một hạn xử lý đếm qua ngày trụ sở đóng cửa.
   */
  const [nghi, datNghi] = useState<{
    nam: number;
    kq: KetQua<identity_danhSachNgayNghiLeRa>;
  } | null>(null);
  const [lamBu, datLamBu] = useState<{
    nam: number;
    kq: KetQua<identity_danhSachCaLamBuRa>;
  } | null>(null);

  // Lịch tuần KHÔNG gắn với năm nào, nên nó đọc một lần khi mở và không đọc lại khi đổi năm.
  useEffect(() => {
    let bo = false;
    layLichLamViec().then((kq) => {
      if (bo) return;
      datTuan(kq.ok ? { pha: "xong", duLieu: kq.duLieu } : { pha: "loi", thongBao: kq.thongBao });
    });
    return () => {
      bo = true;
    };
  }, []);

  /**
   * Hai tuyến theo năm đọc RIÊNG chứ không gộp một lời gọi, và hỏng một bên không kéo bên kia
   * xuống. Chúng có một trạng thái lỗi CHUNG rất đáng thấy: khi một ngày vừa được khai là nghỉ
   * lễ vừa được khai là làm bù, CẢ HAI tuyến trả 409 với cùng một câu — máy chủ cố ý từ chối ở
   * cả hai phía, vì nếu chỉ một phía báo thì bảng kia trông vẫn lành và xã sẽ không sửa gì.
   */
  useEffect(() => {
    let bo = false;
    layNgayNghiLe(nam).then((kq) => {
      if (!bo) datNghi({ nam, kq });
    });
    layNgayLamBu(nam).then((kq) => {
      if (!bo) datLamBu({ nam, kq });
    });
    return () => {
      bo = true;
    };
  }, [nam]);

  const trangThaiNghi = theoNam(nghi, nam);
  const trangThaiLamBu = theoNam(lamBu, nam);

  return (
    <section className="tab-lich-lam-viec" aria-labelledby="tieu-de-lich-lam-viec">
      <h2 id="tieu-de-lich-lam-viec">Lịch làm việc của đơn vị</h2>
      <p className="ghi-chu">{DAN_LICH_LAM_VIEC}</p>
      <p className="ghi-chu">{GHI_CHU_CHI_XEM_LICH}</p>

      <ChonNam
        id="nam-lich-lam-viec"
        nhan="Năm của lịch nghỉ lễ và làm bù"
        nam={nam}
        namGoc={namGoc}
        datNam={datNam}
      />

      <BangLichTuan trangThai={tuan} />
      <BangNgayNghiLe trangThai={trangThaiNghi} nam={nam} />
      <BangNgayLamBu trangThai={trangThaiLamBu} nam={nam} />
    </section>
  );
}

/** Lỗi và "đang tải" trông giống nhau ở cả ba bảng, nên chúng dùng chung một khối. */
function KhungTai({ trangThai, dangTai }: { trangThai: { pha: string }; dangTai: string }) {
  if (trangThai.pha === "dangTai") return <p role="status">{dangTai}</p>;
  return null;
}

export function BangLichTuan({
  trangThai,
}: {
  trangThai: TrangThai<identity_danhSachCaLamViecRa>;
}) {
  return (
    <div className="nhom-lich">
      <h3>Giờ làm việc trong tuần</h3>
      <KhungTai trangThai={trangThai} dangTai="Đang tải giờ làm việc…" />

      {trangThai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {trangThai.thongBao}
        </p>
      )}

      {trangThai.pha === "xong" && (
        <>
          {/* NHỮNG SAI SÓT CỦA CHÍNH DỮ LIỆU XÃ, do máy chủ suy ra từ đúng những dòng nó vừa trả
              về. Tuyến vẫn trả 200 có chủ ý: đây là màn hình để SỬA lịch, nên nó phải mở được kể
              cả khi lịch đang sai. Hiện NGUYÊN câu máy chủ viết — câu ấy nêu rõ thứ nào hỏng. */}
          {trangThai.duLieu.problems.map((v, i) => (
            <p className="thong-bao-loi" role="alert" key={`${v.kind}-${v.weekday ?? "chung"}-${i}`}>
              {v.message}
            </p>
          ))}

          {trangThai.duLieu.items.length === 0 ? (
            // Lịch tuần trống ĐÃ có một `problems` nói đúng điều này, nên câu dưới đây chỉ là
            // trạng thái rỗng của bảng, không phải lời báo động thứ hai.
            <p className="trang-thai-rong">{LICH_TUAN_RONG}</p>
          ) : (
            <div className="bang-cuon" role="region" aria-label="Giờ làm việc trong tuần" tabIndex={0}>
              <table className="bang-danh-muc">
                <caption className="an-thi-giac">
                  Các ca làm việc thông thường của đơn vị theo từng thứ trong tuần
                </caption>
                <thead>
                  <tr>
                    <th scope="col">Thứ</th>
                    <th scope="col">Ca làm việc</th>
                    <th scope="col">Ghi chú</th>
                  </tr>
                </thead>
                <tbody>
                  {/* Giữ NGUYÊN thứ tự máy chủ trả về. Nghỉ trưa là KHOẢNG HỞ giữa hai ca cùng
                      một thứ, không phải một dòng riêng — nên gộp hai ca lại thành một khoảng là
                      xoá mất giờ nghỉ trưa khỏi cách đếm hạn. */}
                  {trangThai.duLieu.items.map((c) => (
                    <tr key={c.id}>
                      <td>{tenThu(c.weekday)}</td>
                      <td>{nhanCa(c.start, c.end)}</td>
                      <td>{c.note === "" ? <span className="nhan-trong">—</span> : c.note}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  );
}

export function BangNgayNghiLe({
  trangThai,
  nam,
}: {
  trangThai: TrangThai<identity_danhSachNgayNghiLeRa>;
  nam: number;
}) {
  return (
    <div className="nhom-lich">
      <h3>Ngày nghỉ lễ — đơn vị KHÔNG làm việc</h3>
      <KhungTai trangThai={trangThai} dangTai="Đang tải ngày nghỉ lễ…" />

      {/* 409 "lịch mâu thuẫn" đi vào đúng nhánh này, và câu của máy chủ nêu đích danh những ngày
          vừa được khai là nghỉ lễ vừa được khai là làm bù. Không rút gọn câu ấy: một lời từ chối
          không nói ngày nào là một lời từ chối không ai sửa được theo. */}
      {trangThai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {trangThai.thongBao}
        </p>
      )}

      {trangThai.pha === "xong" &&
        (trangThai.duLieu.items.length === 0 ? (
          <p className="trang-thai-rong">{nhanNgayNghiRong(nam)}</p>
        ) : (
          <div className="bang-cuon" role="region" aria-label={`Ngày nghỉ lễ năm ${nam}`} tabIndex={0}>
            <table className="bang-danh-muc">
              <caption className="an-thi-giac">
                Những ngày đơn vị đóng cửa trong năm {nam}, gồm cả lễ quốc gia và lễ địa phương
              </caption>
              <thead>
                <tr>
                  <th scope="col">Ngày</th>
                  <th scope="col">Tên</th>
                </tr>
              </thead>
              <tbody>
                {trangThai.duLieu.items.map((n) => (
                  <tr key={n.id}>
                    <td>{nhanNgay(n.date)}</td>
                    <td>{n.name}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))}
    </div>
  );
}

export function BangNgayLamBu({
  trangThai,
  nam,
}: {
  trangThai: TrangThai<identity_danhSachCaLamBuRa>;
  nam: number;
}) {
  return (
    <div className="nhom-lich">
      <h3>Ngày làm bù — đơn vị CÓ làm việc</h3>
      <KhungTai trangThai={trangThai} dangTai="Đang tải ngày làm bù…" />

      {trangThai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {trangThai.thongBao}
        </p>
      )}

      {trangThai.pha === "xong" && (
        <>
          {trangThai.duLieu.problems.map((v, i) => (
            <p className="thong-bao-loi" role="alert" key={`${v.kind}-${v.date}-${i}`}>
              {v.message}
            </p>
          ))}

          {trangThai.duLieu.items.length === 0 ? (
            <p className="trang-thai-rong">{nhanNgayLamBuRong(nam)}</p>
          ) : (
            <div className="bang-cuon" role="region" aria-label={`Ngày làm bù năm ${nam}`} tabIndex={0}>
              <table className="bang-danh-muc">
                <caption className="an-thi-giac">
                  Những ngày đơn vị vẫn làm việc trong năm {nam} dù lịch tuần nói không, kèm giờ
                  làm của chính ngày đó
                </caption>
                <thead>
                  <tr>
                    <th scope="col">Ngày</th>
                    <th scope="col">Ca làm việc</th>
                    <th scope="col">Theo thông báo</th>
                  </tr>
                </thead>
                <tbody>
                  {/* MỘT DÒNG LÀ MỘT CA, KHÔNG PHẢI MỘT NGÀY: ngày làm bù có nghỉ trưa là hai
                      dòng cùng ngày. Gộp lại theo ngày sẽ nuốt mất giờ nghỉ trưa của ngày ấy. */}
                  {trangThai.duLieu.items.map((c) => (
                    <tr key={c.id}>
                      <td>{nhanNgay(c.date)}</td>
                      <td>{nhanCa(c.start, c.end)}</td>
                      {/* Tên là thông báo mà dòng này thực hiện — "Làm bù nghỉ Tết theo Thông báo
                          số …". Chính chuỗi ấy là câu trả lời khi đoàn kiểm tra hỏi vì sao một
                          hạn chạy qua ngày thứ Bảy. */}
                      <td>{c.name}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  );
}
