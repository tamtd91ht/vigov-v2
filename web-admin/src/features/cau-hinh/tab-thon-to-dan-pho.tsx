"use client";

import { useEffect, useState } from "react";

import type { identity_thonToDanPhoRa } from "@/lib/api/schema.gen";
import { layDanhSachThonToDanPho } from "@/lib/api/thon-to-dan-pho";

import {
  GHI_CHU_CHI_XEM_THON,
  loaiDonVi,
  lopLoaiDonVi,
  lopTrangThaiDiaBan,
  nhanLoaiDonVi,
  nhanSoDem,
  nhanTrangThaiDiaBan,
  THON_RONG,
} from "./nhan-thon";

/**
 * Tab "Thôn / Tổ dân phố" — `docs/ui-ux/14-cau-hinh.md §2`, danh sách địa bàn của đơn vị.
 *
 * TAB RIÊNG, KHÔNG NHẬP VÀO TAB DANH MỤC, dù cả hai cùng đọc một tuyến kiểu danh sách: một thôn
 * không phải một mục danh mục. Hợp đồng nói ra điều đó bằng chính tên trường (`name` chứ không
 * phải `label`), máy chủ nói ra bằng hai con số chỉ một địa bàn có thật mới có (số hộ, nhân
 * khẩu), và đặc tả tách đôi thành hai tab. Gộp lại là để một bảng địa bàn nằm dưới một tiêu đề
 * "Danh mục" — và ngày ai đó thêm nút sửa cho danh mục thì nút ấy nằm luôn trên địa bàn.
 *
 * KHÔNG CÓ CỔNG QUYỀN, cùng một lý lẽ đã ghi đầy đủ ở `tab-danh-muc.tsx`: tuyến phía sau khai
 * `any-authenticated` ở máy chủ, nên một cổng ở giao diện sẽ là thứ DUY NHẤT quyết định — đúng
 * hình dạng luật 5 cấm #1 — và sẽ hiện một câu sai cho người mà máy chủ vẫn đang phục vụ.
 *
 * KHÔNG CÓ BỀ MẶT GHI NÀO. Đặc tả vẽ `+ Thêm thôn / tổ dân phố`, `⬆ Nhập từ Excel`, `✎`, `🗑`;
 * không thao tác nào có tuyến phía sau, và máy chủ ghi rõ vì sao ngay trên tuyến: lập một thôn,
 * nhập hai thôn làm một, hay cho một thôn ngừng hoạt động là hành vi hành chính tác động lên một
 * bản ghi mà phản ánh và hồ sơ hộ đang trỏ tới — ai được làm, và hồ sơ trỏ vào một địa bàn biến
 * mất thì ra sao, chưa ai trả lời (`service-identity/internal/http/thon_to_dan_pho.go`).
 */

/** Ba nhánh rời nhau — cùng khuôn với `danh-ba-can-bo.tsx`. Không nhánh nào suy ra được từ nhánh khác. */
type TrangThaiThon =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; danhSach: readonly identity_thonToDanPhoRa[] };

export function TabThonToDanPho() {
  const [trangThai, datTrangThai] = useState<TrangThaiThon>({ pha: "dangTai" });

  /**
   * MỘT lời gọi cho cả màn hình, đọc một lượt khi mở — `[]` ở cuối effect là phần quan trọng nhất.
   *
   * Tuyến trả NGUYÊN danh sách, không phân trang: khi danh sách vượt trần máy chủ TỪ CHỐI thay vì
   * cắt bớt, vì một danh sách địa bàn ngắn đi lặng lẽ khiến phản ánh bị lập cho sai địa bàn trong
   * khi màn hình trông hoàn toàn bình thường (`lib/api/thon-to-dan-pho.ts`).
   *
   * KHÔNG ĐỆM QUA LẦN MỞ MÀN HÌNH: danh sách địa bàn là dữ liệu của MỘT đơn vị, và một bản đệm
   * sống lâu hơn yêu cầu, trong một tiến trình phục vụ nhiều tên miền, là đúng hình dạng của một
   * lần dữ liệu đơn vị này hiện trên màn hình đơn vị khác (luật 1, cấm #1).
   */
  useEffect(() => {
    let bo = false;
    layDanhSachThonToDanPho().then((kq) => {
      if (bo) return;
      datTrangThai(
        kq.ok ? { pha: "xong", danhSach: kq.duLieu.items } : { pha: "loi", thongBao: kq.thongBao },
      );
    });
    return () => {
      bo = true;
    };
  }, []);

  return (
    <section className="tab-thon-to-dan-pho" aria-labelledby="tieu-de-thon">
      <h2 id="tieu-de-thon">Thôn / Tổ dân phố</h2>
      <p className="ghi-chu">{GHI_CHU_CHI_XEM_THON}</p>

      {trangThai.pha === "dangTai" && <p role="status">Đang tải danh sách địa bàn…</p>}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải và không rẽ nhánh theo `code`
          (`lib/api/goi.ts`). */}
      {trangThai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {trangThai.thongBao}
        </p>
      )}

      {/* TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI, và hôm nay là đường THÔNG THƯỜNG của mọi đơn
          vị: migration 0005 tạo bảng và cố ý không gieo bản ghi nào, bước khởi tạo đơn vị chưa
          tồn tại. Máy chủ trả `items: []` chứ không bao giờ trả `null`. */}
      {trangThai.pha === "xong" && trangThai.danhSach.length === 0 && (
        <p className="trang-thai-rong">{THON_RONG}</p>
      )}

      {trangThai.pha === "xong" && trangThai.danhSach.length > 0 && (
        <BangDiaBan danhSach={trangThai.danhSach} />
      )}
    </section>
  );
}

/**
 * Bảng địa bàn. Giữ nguyên thứ tự máy chủ trả về — theo tên, với mã phá hoà để thứ tự là toàn
 * phần (`service-identity/internal/store`). Không sắp lại, không lọc bỏ địa bàn đã tắt: một hồ sơ
 * đã lập ở địa bàn đã ngừng dùng vẫn phải tra ra được tên của nó.
 */
function BangDiaBan({ danhSach }: { danhSach: readonly identity_thonToDanPhoRa[] }) {
  return (
    <div className="bang-cuon" role="region" aria-label="Danh sách thôn / tổ dân phố" tabIndex={0}>
      <table className="bang-danh-muc">
        <caption className="an-thi-giac">
          Danh sách thôn và tổ dân phố của đơn vị, sắp xếp theo tên
        </caption>
        <thead>
          <tr>
            <th scope="col">Tên</th>
            <th scope="col">Mã</th>
            <th scope="col">Loại</th>
            <th scope="col">Số hộ</th>
            <th scope="col">Nhân khẩu</th>
            <th scope="col">Trạng thái</th>
          </tr>
        </thead>
        <tbody>
          {danhSach.map((t) => {
            // Ba ca của cột Loại, tính MỘT lần cho mỗi dòng. Xem `nhan-thon.ts`: không ca nào để
            // lại một ô trống, và "chưa phân loại" không được nhìn giống "nhãn đã biến mất".
            const loai = loaiDonVi(t.type_code, t.type_label);
            return (
              <tr key={t.id}>
                <td>{t.name}</td>
                {/* Mã font mono: một slug được gõ lại và đọc qua điện thoại. */}
                <td className="ma-muc">{t.code}</td>
                <td>
                  <span className={lopLoaiDonVi(loai)}>{nhanLoaiDonVi(loai)}</span>
                </td>
                {/* `null` KHÔNG hiện thành `0` — một con số không chưa ai khẳng định sẽ đi tiếp
                    vào báo cáo gửi lên trên như một con số thật (`nhan-thon.ts`). */}
                <td>{nhanSoDem(t.household_count)}</td>
                <td>{nhanSoDem(t.population_count)}</td>
                <td>
                  <span className={lopTrangThaiDiaBan(t.active)}>
                    {nhanTrangThaiDiaBan(t.active)}
                  </span>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
