"use client";

import { useState, type FormEvent } from "react";

import { layPhieuPhanAnh } from "@/lib/api/phieu-phan-anh";
import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import {
  CHUA_TRA_CUU,
  HUONG_DAN_TRA_CUU,
  linhVucPhanAnh,
  lopHan,
  nhanHan,
  nhanHienCongKhai,
  nhanKenh,
  nhanLinhVuc,
  nhanNguoiGui,
  nhanThoiDiem,
  nhanTrangThai,
  trangThaiHan,
} from "./nhan-phieu";

/**
 * Tra cứu một phiếu phản ánh theo **mã tra cứu** — `docs/ui-ux/09-phan-anh-nguoi-dan.md §8`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * VÌ SAO ĐÂY LÀ MÀN TRA MỘT PHIẾU CHỨ KHÔNG PHẢI DANH SÁCH THẺ MÀ ĐẶC TẢ §2 VẼ.
 *
 * Hợp đồng REST có đúng một tuyến đọc phản ánh: `GET /api/v1/citizen-reports/{maTraCuu}`. Không
 * có tuyến danh sách, không có tuyến thống kê, nên bốn thẻ KPI, ba bộ lọc, tab Bản đồ nhiệt và
 * tab Báo cáo đều không có gì đứng sau. Vẽ chúng ra là hứa với cán bộ những thứ không tồn tại,
 * và bốn con số KPI bịa ra là thứ lãnh đạo đọc rồi báo cáo lên trên.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * MÃ TRA CỨU KHÔNG ĐOÁN ĐƯỢC, VÀ MÀN HÌNH NÀY KHÔNG LÀM NÓ ĐOÁN ĐƯỢC. Không gợi ý, không tự hoàn
 * thành, không liệt kê mã gần đúng; sai mã thì nhận đúng một câu trả lời — cùng câu mà một phiếu
 * của xã khác và một phiếu thuộc lĩnh vực hạn chế nhận được (luật 4, cấm #2 và #3).
 *
 * KHÔNG CÓ GHI CHÚ NỘI BỘ, KHÔNG CÓ LỊCH SỬ LUÂN CHUYỂN, KHÔNG CÓ NHẬT KÝ XỬ LÝ trên màn này —
 * vì hợp đồng không trả về chúng. Đã đối chiếu từng trường của `petitions.phieuPhanAnhRa`.
 *
 * KHÔNG CÓ BỀ MẶT GHI NÀO: chuyển trạng thái, chuyển xử lý, ghi nhận đánh giá, cho hiện công
 * khai — đặc tả vẽ cả bốn, không thao tác nào có tuyến phía sau. Mỗi thao tác ấy đổi trạng thái
 * một hồ sơ hành chính và phải thông báo cho người dân cùng lúc (luật 10, bất biến 5).
 */

type TrangThaiTra =
  | { pha: "chuaTra" }
  | { pha: "dangTra" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; phieu: petitions_phieuPhanAnhRa };

export function TraCuuPhieu() {
  const [ma, datMa] = useState("");
  const [trangThai, datTrangThai] = useState<TrangThaiTra>({ pha: "chuaTra" });

  async function tra(e: FormEvent) {
    e.preventDefault();
    const canGon = ma.trim();
    // Mã rỗng không gọi tuyến nào: một đoạn đường dẫn rỗng không khớp mã nào và không có việc gì
    // chạm tới máy chủ (máy chủ cũng từ chối, `phieu_phan_anh.go:173`).
    if (canGon === "") return;

    datTrangThai({ pha: "dangTra" });
    const kq = await layPhieuPhanAnh(canGon);
    datTrangThai(kq.ok ? { pha: "xong", phieu: kq.duLieu } : { pha: "loi", thongBao: kq.thongBao });
  }

  return (
    <section className="man-phan-anh" aria-labelledby="tieu-de-tra-cuu">
      <h2 id="tieu-de-tra-cuu">Tra cứu phiếu phản ánh</h2>
      <p className="ghi-chu">{HUONG_DAN_TRA_CUU}</p>

      <form className="form-tra-cuu" onSubmit={tra}>
        <div className="o-nhap">
          <label htmlFor="ma-tra-cuu">Mã tra cứu</label>
          <input
            id="ma-tra-cuu"
            name="ma-tra-cuu"
            value={ma}
            onChange={(e) => datMa(e.target.value)}
            // Không `autoComplete`: mã tra cứu là chuỗi mở một phiếu của người dân, và trình
            // duyệt lưu lại nó trên một máy dùng chung ở trụ sở xã là một bản sao không ai quản.
            autoComplete="off"
            spellCheck={false}
          />
        </div>
        <button className="nut-chinh" type="submit" disabled={trangThai.pha === "dangTra"}>
          {trangThai.pha === "dangTra" ? "Đang tra…" : "Tra cứu"}
        </button>
      </form>

      {trangThai.pha === "chuaTra" && <p className="trang-thai-rong">{CHUA_TRA_CUU}</p>}
      {trangThai.pha === "dangTra" && <p role="status">Đang tra phiếu…</p>}

      {/* Hiện ĐÚNG `message` của máy chủ. Không thêm "có thể bạn gõ nhầm", không thêm "phiếu này
          thuộc xã khác" — cả hai đều là những câu nói ra điều máy chủ vừa cố ý không nói. */}
      {trangThai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {trangThai.thongBao}
        </p>
      )}

      {trangThai.pha === "xong" && <ThongTinPhieu phieu={trangThai.phieu} bayGio={new Date()} />}
    </section>
  );
}

/**
 * Phần thuần trình bày. `bayGio` truyền vào để kiểm được cả hai phía của mốc hạn — một component
 * tự gọi `new Date()` bên trong chỉ kiểm được đúng khoảnh khắc chạy test.
 */
export function ThongTinPhieu({
  phieu,
  bayGio,
}: {
  phieu: petitions_phieuPhanAnhRa;
  bayGio: Date;
}) {
  const linhVuc = linhVucPhanAnh(phieu.field, phieu.field_label);
  const hanTiepNhan = trangThaiHan(phieu.acknowledge_due, "khongApDung", bayGio);
  const hanXuLy = trangThaiHan(phieu.resolve_due, "chuaCo", bayGio);

  return (
    <div className="khoi-chi-tiet">
      <div className="dau-khoi-chi-tiet">
        <h3 className="ma-muc">{phieu.code}</h3>
        <span className="chip chip-ngung">{nhanTrangThai(phieu.status)}</span>
      </div>

      <dl className="danh-sach-truong">
        <dt>Lĩnh vực</dt>
        <dd>{nhanLinhVuc(linhVuc)}</dd>

        <dt>Kênh tiếp nhận</dt>
        <dd>{nhanKenh(phieu.channel)}</dd>

        {/* NGƯỜI GỬI ĐÃ CHE SẴN Ở MÁY CHỦ — không ghép lại, không hiện thêm chữ số nào. */}
        <dt>Người gửi</dt>
        <dd>{nhanNguoiGui(phieu)}</dd>

        <dt>Nội dung</dt>
        {/* Nội dung KHÔNG che, và đó không phải mâu thuẫn: cán bộ không đọc được phản ánh thì
            không xử lý được nó, và một câu che một nửa thì không còn dùng được. Cái được che là
            những gì buộc phiếu vào một người có tên. */}
        <dd className="noi-dung-phan-anh">{phieu.content}</dd>

        <dt>Địa chỉ</dt>
        <dd>{phieu.address === "" ? "Không có" : phieu.address}</dd>

        {/* BA MỐC THỜI GIAN, KHÔNG PHẢI HAI, và không mốc nào thay được mốc kia:
            `clock_from` là lúc người dân bấm gửi — GỐC ĐẾM của cả hai hạn, và là thứ duy nhất
            giải thích được hai hạn ấy cho một đoàn kiểm tra;
            `booked_at` là lúc phiếu vào sổ. */}
        <dt>Người dân gửi lúc</dt>
        <dd>{nhanThoiDiem(phieu.clock_from)}</dd>

        <dt>Vào sổ lúc</dt>
        <dd>{nhanThoiDiem(phieu.booked_at)}</dd>

        <dt>Hạn tiếp nhận</dt>
        <dd>
          <span className={lopHan(hanTiepNhan)}>{nhanHan(hanTiepNhan)}</span>
        </dd>

        <dt>Hạn xử lý xong</dt>
        <dd>
          <span className={lopHan(hanXuLy)}>{nhanHan(hanXuLy)}</span>
        </dd>

        <dt>Hiển thị với người dân</dt>
        <dd>{nhanHienCongKhai(phieu.public)}</dd>
      </dl>
    </div>
  );
}
