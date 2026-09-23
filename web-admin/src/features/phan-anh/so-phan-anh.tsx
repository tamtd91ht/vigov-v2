"use client";

import { useEffect, useState, type FormEvent } from "react";

import {
  coTrangTruoc,
  sangTrangSau,
  TRANG_DAU,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import type { KetQua } from "@/lib/api/goi";
import {
  chuyenXuLyPhieu,
  dongPhieu,
  laySoPhanAnh,
  phanLoaiPhieu,
  tienTrangThaiPhieu,
  type LocPhanAnh,
} from "@/lib/api/phieu-phan-anh";
import type {
  identity_boPhanRa,
  identity_thonToDanPhoRa,
  page_Result_petitions_phieuPhanAnhRa,
  petitions_phieuPhanAnhRa,
} from "@/lib/api/schema.gen";
import { layDanhSachThonToDanPho } from "@/lib/api/thon-to-dan-pho";
import { coQuyen } from "@/lib/quyen";

import {
  buocLuongChinh,
  CAU_THIEU_QUYEN_DONG,
  CAU_THIEU_QUYEN_PHAN_CONG,
  CAU_THIEU_QUYEN_PHAN_LOAI,
  CHI_TRE_HAN_NHAN,
  cauGiaiThichTrangThai,
  conBuocKeTiep,
  congThaoTac,
  DANG_TAI_SO,
  GHI_CHU_O_KET_QUA,
  GHI_CHU_TIEN_TRANG_THAI,
  LINH_VUC_PHAN_ANH,
  linhVucPhanAnh,
  lopHan,
  MOI_BO_PHAN_NHAN,
  MOI_DIA_BAN_NHAN,
  MOI_KENH,
  MOI_KENH_NHAN,
  MOI_LINH_VUC_NHAN,
  MOI_TRANG_THAI,
  MOI_TRANG_THAI_NHAN,
  NHAN_O_KET_QUA,
  NHAN_TIEN_TRANG_THAI,
  nhanBoPhan,
  nhanCanBoXuLy,
  nhanHan,
  nhanHienCongKhai,
  nhanKenh,
  nhanLinhVuc,
  nhanNguoiGui,
  nhanThoiDiem,
  nhanTrangThai,
  PHAN_CHUA_DUNG,
  phanLoaiDuoc,
  SO_RONG,
  TIM_PLACEHOLDER,
  trangThaiHan,
  type CongThaoTac,
} from "./nhan-phieu";
import {
  QUYEN_DONG_PHAN_ANH,
  QUYEN_PHAN_CONG_PHAN_ANH,
  QUYEN_PHAN_LOAI_PHAN_ANH,
} from "@/lib/quyen";

/**
 * Sổ Phản ánh của người dân — `docs/ui-ux/09-phan-anh-nguoi-dan.md` §2 (quyển sổ), §4 (bộ lọc),
 * §8 (chi tiết) và bốn thao tác xử lý.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐIỂM CHÍNH CỦA MÀN HÌNH NÀY LÀ HAI CỔNG KHÔNG GIỐNG NHAU, và nó dễ bị gộp lại "cho gọn":
 *
 *   `Chuyển sang bước kế tiếp`  KHÔNG có cổng ở giao diện. Tuyến khai `feedback.read`, điều kiện
 *                               thật là `feedback.resolve` HOẶC chính là cán bộ được phân công —
 *                               LUẬT NẮM GIỮ. Vế thứ hai giao diện không tính được (xem
 *                               `congThaoTac`), nên nút hiện với mọi người xem được sổ và câu 403
 *                               của máy chủ ra thẳng màn hình.
 *   `Đóng phiếu`                CÓ cổng: `feedback.resolve`, và **không** được nới theo luật nắm
 *                               giữ. Đóng phiếu ghi một kết quả NGƯỜI DÂN ĐỌC (luật 10, bất biến
 *                               6) — câu hỏi mở #7 chốt 16/09/2026.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * DỮ LIỆU CÁ NHÂN VỀ ĐÃ CHE, VÀ MÀN HÌNH KHÔNG GHÉP LẠI. Ở tuyến danh sách thì che là tuyệt đối —
 * kể cả tài khoản có `feedback.unmask` — vì một lời gọi mở hai mươi người gửi không viết nổi một
 * dòng vết kiểm toán trung thực (luật 6, bất biến 7).
 *
 * PHIẾU LĨNH VỰC `can-bo` KHÔNG CÓ TRONG TRANG khi tài khoản thiếu `feedback.restricted`, và màn
 * hình **không** nói "có phiếu bị ẩn": nói ra là nói cho một đồng nghiệp của người bị phản ánh
 * biết rằng phiếu ấy tồn tại (luật 4, cấm #2).
 */

/** Bao nhiêu thẻ một trang. Đủ để lướt buổi sáng, không nhiều tới mức trang đầu tải chậm. */
const SO_THE_MOI_TRANG = 20;

type TrangThaiTai<T> =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; duLieu: T };

function taiTu<T>(daTai: { khoa: string; kq: KetQua<T> } | null, khoa: string): TrangThaiTai<T> {
  if (daTai === null || daTai.khoa !== khoa) return { pha: "dangTai" };
  return daTai.kq.ok
    ? { pha: "xong", duLieu: daTai.kq.duLieu }
    : { pha: "loi", thongBao: daTai.kq.thongBao };
}

/** Bộ lọc đang chọn trên màn hình. Cùng hình dạng với `LocPhanAnh`, trừ phân trang. */
type BoLoc = Omit<LocPhanAnh, "limit" | "cursor">;

const KHONG_LOC: BoLoc = {};

export function SoPhanAnh() {
  const [loc, datLoc] = useState<BoLoc>(KHONG_LOC);
  const [tim, datTim] = useState("");
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);

  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_petitions_phieuPhanAnhRa>;
  } | null>(null);
  const [daTaiBoPhan, datDaTaiBoPhan] = useState<readonly identity_boPhanRa[]>([]);
  const [daTaiThon, datDaTaiThon] = useState<readonly identity_thonToDanPhoRa[]>([]);

  const [dangMo, datDangMo] = useState<petitions_phieuPhanAnhRa | null>(null);
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  const [dangGui, datDangGui] = useState(false);

  const khoa = `${JSON.stringify(loc)}|${nganXep.hienTai ?? ""}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    laySoPhanAnh({ ...loc, limit: SO_THE_MOI_TRANG, cursor: nganXep.hienTai }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [loc, nganXep.hienTai, khoa]);

  // HAI DANH MỤC, ĐỌC MỘT LẦN CHO CẢ MÀN. Cả hai tuyến là `any-authenticated`, nên mọi tài khoản
  // xem được sổ đều đọc được. Danh mục hỏng thì ô lọc tương ứng rỗng — KHÔNG làm hỏng quyển sổ:
  // hai câu trả lời rời nhau, mỗi cái nói chuyện của nó.
  useEffect(() => {
    let bo = false;
    layDanhMucBoPhan().then((kq) => {
      if (!bo && kq.ok) datDaTaiBoPhan(kq.duLieu.items);
    });
    layDanhSachThonToDanPho().then((kq) => {
      if (!bo && kq.ok) datDaTaiThon(kq.duLieu.items);
    });
    return () => {
      bo = true;
    };
  }, []);

  const phien = usePhien();
  // FAIL CLOSED: chưa đọc xong phiên, hoặc đọc hỏng, thì KHÔNG có quyền nào — "chưa rõ" không được
  // hành xử như "có" (luật 1, cấm #1).
  const dsQuyen: readonly string[] = phien !== null && phien.ok ? phien.duLieu.permissions : [];
  const cong = congThaoTac(
    coQuyen(dsQuyen, QUYEN_PHAN_LOAI_PHAN_ANH),
    coQuyen(dsQuyen, QUYEN_PHAN_CONG_PHAN_ANH),
    coQuyen(dsQuyen, QUYEN_DONG_PHAN_ANH),
  );

  const so = taiTu(daTai, khoa);
  const tenBoPhan = new Map(daTaiBoPhan.map((b) => [b.id, b.name]));

  /** Đổi bộ lọc là về trang đầu: con trỏ của bộ lọc cũ không có nghĩa với bộ lọc mới. */
  function datLocMoi(moi: BoLoc): void {
    datLoc(moi);
    datNganXep(TRANG_DAU);
  }

  /** Một lần ghi xong: giữ phiếu máy chủ vừa trả, xoá lỗi cũ, và đọc lại quyển sổ. */
  function xongGhi(kq: KetQua<petitions_phieuPhanAnhRa>): void {
    datDangGui(false);
    if (!kq.ok) {
      // NGUYÊN VĂN câu máy chủ. 409 của các tuyến này mang đúng quy tắc đã từ chối ("phiếu đã
      // chuyển trạng thái trong lúc bạn đang mở màn hình", "xã chưa cấu hình thời hạn xử lý cho
      // lĩnh vực này"), và viết lại nó ở client là dựng bản sao thứ hai của một quy tắc nghiệp vụ.
      datLoiGhi(kq.thongBao);
      return;
    }
    datLoiGhi(null);
    datDangMo(kq.duLieu);
    datLanTai((n) => n + 1);
  }

  function chay(goi: Promise<KetQua<petitions_phieuPhanAnhRa>>): void {
    datDangGui(true);
    goi.then(xongGhi);
  }

  return (
    <section className="man-phan-anh" aria-labelledby="tieu-de-so-phan-anh">
      <h2 id="tieu-de-so-phan-anh">Sổ phản ánh của xã</h2>

      <KhoiChuaDung />

      <HangLoc
        loc={loc}
        tim={tim}
        datTim={datTim}
        datLoc={datLocMoi}
        boPhan={daTaiBoPhan}
        thon={daTaiThon}
      />

      {so.pha === "dangTai" && <p role="status">{DANG_TAI_SO}</p>}
      {so.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {so.thongBao}
        </p>
      )}

      {so.pha === "xong" && (
        <>
          <DanhSachThe
            phieu={so.duLieu.items}
            tenBoPhan={tenBoPhan}
            bayGio={new Date()}
            maDangMo={dangMo?.code ?? null}
            moPhieu={(p) => {
              datDangMo(p);
              datLoiGhi(null);
            }}
          />
          <nav className="dieu-huong-trang" aria-label="Phân trang sổ phản ánh">
            <button
              type="button"
              className="nut-phu"
              disabled={!coTrangTruoc(nganXep)}
              onClick={() => datNganXep(veTrangTruoc(nganXep))}
            >
              Trang trước
            </button>
            <button
              type="button"
              className="nut-phu"
              // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi
              // tới đó.
              disabled={!so.duLieu.has_more || so.duLieu.next_cursor === ""}
              onClick={() => datNganXep(sangTrangSau(nganXep, so.duLieu.next_cursor))}
            >
              Trang sau
            </button>
          </nav>
        </>
      )}

      {dangMo !== null && (
        <ChiTietPhieu
          phieu={dangMo}
          bayGio={new Date()}
          cong={cong}
          tenBoPhan={tenBoPhan}
          boPhan={daTaiBoPhan}
          dangGui={dangGui}
          loiGhi={loiGhi}
          dong={() => {
            datDangMo(null);
            datLoiGhi(null);
          }}
          phanLoai={(linhVuc) => chay(phanLoaiPhieu(dangMo.code, linhVuc))}
          chuyenXuLy={(boPhanID) => chay(chuyenXuLyPhieu(dangMo.code, boPhanID))}
          tienTrangThai={() => chay(tienTrangThaiPhieu(dangMo.code))}
          dongPhieuLai={(ketQua) => chay(dongPhieu(dangMo.code, ketQua))}
        />
      )}
    </section>
  );
}

/**
 * Những phần đặc tả đòi mà hợp đồng không có — HIỆN LÊN ĐẦU MÀN, không giấu trong chú thích mã.
 *
 * `<details>` chứ không phải một khối luôn mở: danh sách dài hơn quyển sổ ở những ngày đầu, và một
 * bức tường chữ trên đầu màn hình là bức tường người ta học cách không đọc.
 */
export function KhoiChuaDung() {
  return (
    <details className="khoi-chua-khai">
      <summary>
        {PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và lý do
      </summary>
      <dl className="danh-sach-truong">
        {PHAN_CHUA_DUNG.map((p) => (
          <div key={p.ten}>
            <dt>{p.ten}</dt>
            <dd>{p.viSao}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}

/** Bộ lọc §4. Bảy ô, đúng bảy tham số máy chủ nhận — không vẽ ô nào không có tuyến đứng sau. */
export function HangLoc({
  loc,
  tim,
  datTim,
  datLoc,
  boPhan,
  thon,
}: {
  loc: BoLoc;
  tim: string;
  datTim: (s: string) => void;
  datLoc: (moi: BoLoc) => void;
  boPhan: readonly identity_boPhanRa[];
  thon: readonly identity_thonToDanPhoRa[];
}) {
  function timNgay(e: FormEvent) {
    e.preventDefault();
    const canGon = tim.trim();
    datLoc({ ...loc, tim: canGon === "" ? undefined : canGon });
  }

  return (
    <div className="hang-loc">
      {/* Ô TÌM GỬI BẰNG SUBMIT, KHÔNG GỬI THEO TỪNG PHÍM: mỗi phím là một lời gọi mang chữ cán bộ
          đang gõ vào một URL — và chuỗi ấy có thể là tên hay địa chỉ một công dân (luật 3, cấm #4).
          Gõ xong rồi bấm là một lần. */}
      <form className="form-tra-cuu" onSubmit={timNgay} role="search">
        <div className="o-nhap">
          <label htmlFor="tim-phan-anh">Tìm trong sổ</label>
          <input
            id="tim-phan-anh"
            name="tim-phan-anh"
            value={tim}
            placeholder={TIM_PLACEHOLDER}
            onChange={(e) => datTim(e.target.value)}
            autoComplete="off"
            // Máy chủ trả 400 khi quá 200 ký tự (`store.TimPhieuToiDa`). Chặn ở ô nhập để cán bộ
            // thấy giới hạn thay vì thấy "không tải được".
            maxLength={200}
          />
        </div>
        <button className="nut-phu" type="submit">
          Tìm
        </button>
      </form>

      <div className="o-chon">
        <label htmlFor="loc-trang-thai">Trạng thái</label>
        <select
          id="loc-trang-thai"
          value={loc.trangThai ?? ""}
          onChange={(e) => datLoc({ ...loc, trangThai: e.target.value || undefined })}
        >
          <option value="">{MOI_TRANG_THAI_NHAN}</option>
          {MOI_TRANG_THAI.map((ma) => (
            <option key={ma} value={ma}>
              {nhanTrangThai(ma)}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="loc-linh-vuc">Lĩnh vực</label>
        <select
          id="loc-linh-vuc"
          value={loc.linhVuc ?? ""}
          onChange={(e) => datLoc({ ...loc, linhVuc: e.target.value || undefined })}
        >
          <option value="">{MOI_LINH_VUC_NHAN}</option>
          {LINH_VUC_PHAN_ANH.map((l) => (
            <option key={l.ma} value={l.ma}>
              {l.nhan}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="loc-dia-ban">Địa bàn</label>
        <select
          id="loc-dia-ban"
          value={loc.thonID ?? ""}
          onChange={(e) => datLoc({ ...loc, thonID: e.target.value || undefined })}
        >
          <option value="">{MOI_DIA_BAN_NHAN}</option>
          {thon.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="loc-bo-phan">Bộ phận đang giữ</label>
        <select
          id="loc-bo-phan"
          value={loc.boPhanID ?? ""}
          onChange={(e) => datLoc({ ...loc, boPhanID: e.target.value || undefined })}
        >
          <option value="">{MOI_BO_PHAN_NHAN}</option>
          {boPhan.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="loc-kenh">Kênh tiếp nhận</label>
        <select
          id="loc-kenh"
          value={loc.kenh ?? ""}
          onChange={(e) => datLoc({ ...loc, kenh: e.target.value || undefined })}
        >
          <option value="">{MOI_KENH_NHAN}</option>
          {MOI_KENH.map((ma) => (
            <option key={ma} value={ma}>
              {nhanKenh(ma)}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="loc-tre-han">
          <input
            id="loc-tre-han"
            type="checkbox"
            checked={loc.chiTreHan === true}
            // Ô bỏ tích thì tham số VẮNG MẶT HẲN, không gửi `late=false` — máy chủ chỉ nhận đúng
            // chuỗi `true` và trả 400 cho mọi giá trị khác.
            onChange={(e) => datLoc({ ...loc, chiTreHan: e.target.checked ? true : undefined })}
          />{" "}
          {CHI_TRE_HAN_NHAN}
        </label>
      </div>
    </div>
  );
}

/** Danh sách THẺ (§2 vẽ card list, không phải bảng). */
export function DanhSachThe({
  phieu,
  tenBoPhan,
  bayGio,
  maDangMo,
  moPhieu,
}: {
  phieu: readonly petitions_phieuPhanAnhRa[];
  tenBoPhan: ReadonlyMap<string, string>;
  bayGio: Date;
  maDangMo: string | null;
  moPhieu: (p: petitions_phieuPhanAnhRa) => void;
}) {
  if (phieu.length === 0) return <p className="trang-thai-rong">{SO_RONG}</p>;

  return (
    // KHÔNG GẮN LỚP CSS NÀO cho hai danh sách này và cho stepper bên dưới: `globals.css` chưa có
    // lớp cho danh sách thẻ, và lượt này không được thêm CSS. Mượn một lớp có sẵn của thứ khác
    // (`danh-sach-truong` vốn cho `<dl>`) sẽ trông gần đúng hôm nay rồi lệch hẳn vào ngày lớp ấy
    // đổi vì cái nó thật sự phục vụ. Tên lớp cần thêm đã báo về.
    <ul aria-label="Danh sách phiếu phản ánh">
      {phieu.map((p) => (
        <li key={p.code}>
          <ThePhieu
            phieu={p}
            tenBoPhan={tenBoPhan}
            bayGio={bayGio}
            dangMo={p.code === maDangMo}
            mo={() => moPhieu(p)}
          />
        </li>
      ))}
    </ul>
  );
}

/**
 * Một thẻ phiếu (§7).
 *
 * KHÔNG CÓ THUMBNAIL: bảng ảnh chưa tồn tại — xem `PHAN_CHUA_DUNG`. Không vẽ một ô ảnh giữ chỗ
 * trông như đang chờ tải.
 */
export function ThePhieu({
  phieu,
  tenBoPhan,
  bayGio,
  dangMo,
  mo,
}: {
  phieu: petitions_phieuPhanAnhRa;
  tenBoPhan: ReadonlyMap<string, string>;
  bayGio: Date;
  dangMo: boolean;
  mo: () => void;
}) {
  const linhVuc = linhVucPhanAnh(phieu.field, phieu.field_label);
  const hanXuLy = trangThaiHan(phieu.resolve_due, "chuaCo", bayGio);

  return (
    <div className="khoi-chi-tiet">
      <div className="dau-khoi-chi-tiet">
        <span className="ma-muc">{phieu.code}</span>
        <span className="chip chip-ngung">{nhanTrangThai(phieu.status)}</span>
        <span className="chip">{nhanLinhVuc(linhVuc)}</span>
        {hanXuLy.loai === "quaHan" && <span className="chip nhan-lech">Quá hạn</span>}
      </div>

      {/* Nội dung phản ánh KHÔNG che (cán bộ không đọc được thì không xử lý được), nhưng nó là
          chữ của một công dân: không bao giờ ghi nó vào log, tên tệp hay URL. */}
      <p className="noi-dung-phan-anh">{phieu.content}</p>

      <dl className="danh-sach-truong">
        <dt>Địa chỉ</dt>
        <dd>{phieu.address === "" ? "Chưa rõ vị trí" : phieu.address}</dd>

        <dt>Người gửi</dt>
        <dd>{nhanNguoiGui(phieu)}</dd>

        <dt>Bộ phận đang giữ</dt>
        <dd>{nhanBoPhan(phieu.unit, tenBoPhan)}</dd>

        <dt>Kênh tiếp nhận</dt>
        <dd>{nhanKenh(phieu.channel)}</dd>

        <dt>Hạn xử lý xong</dt>
        <dd>
          <span className={lopHan(hanXuLy)}>{nhanHan(hanXuLy)}</span>
        </dd>
      </dl>

      <button type="button" className="nut-phu" onClick={mo} aria-expanded={dangMo}>
        {dangMo ? "Đang mở phiếu này" : `Mở phiếu ${phieu.code}`}
      </button>
    </div>
  );
}

/**
 * Khối chi tiết §8, kèm bốn thao tác.
 *
 * ĐẶC TẢ GỌI NÓ LÀ DetailDrawer "mở gần toàn màn hình". Ở đây nó là một khối nằm dưới danh sách —
 * KHÔNG phải một lớp phủ — vì một lớp phủ cần lớp CSS chưa có trong `globals.css`, và lượt này
 * không được thêm CSS. Đã báo về tên lớp cần thêm.
 */
export function ChiTietPhieu({
  phieu,
  bayGio,
  cong,
  tenBoPhan,
  boPhan,
  dangGui,
  loiGhi,
  dong,
  phanLoai,
  chuyenXuLy,
  tienTrangThai,
  dongPhieuLai,
}: {
  phieu: petitions_phieuPhanAnhRa;
  bayGio: Date;
  cong: CongThaoTac;
  tenBoPhan: ReadonlyMap<string, string>;
  boPhan: readonly identity_boPhanRa[];
  dangGui: boolean;
  loiGhi: string | null;
  dong: () => void;
  phanLoai: (linhVuc: string) => void;
  chuyenXuLy: (boPhanID: string) => void;
  tienTrangThai: () => void;
  dongPhieuLai: (ketQua: string) => void;
}) {
  const [linhVucChon, datLinhVucChon] = useState("");
  const [boPhanChon, datBoPhanChon] = useState("");
  const [ketQua, datKetQua] = useState("");

  const linhVuc = linhVucPhanAnh(phieu.field, phieu.field_label);
  const hanTiepNhan = trangThaiHan(phieu.acknowledge_due, "khongApDung", bayGio);
  const hanXuLy = trangThaiHan(phieu.resolve_due, "chuaCo", bayGio);
  const hanPhanLoai = trangThaiHan(phieu.classify_due, "khongApDung", bayGio);
  const giaiThich = cauGiaiThichTrangThai(phieu.status);

  return (
    <div className="khoi-chi-tiet" aria-labelledby="tieu-de-chi-tiet-phieu">
      <div className="dau-khoi-chi-tiet">
        <h3 id="tieu-de-chi-tiet-phieu" className="ma-muc">
          {phieu.code} · {nhanKenh(phieu.channel)} · {nhanThoiDiem(phieu.booked_at)}
        </h3>
        <button type="button" className="nut-phu" onClick={dong} aria-label="Đóng chi tiết phiếu">
          ✕
        </button>
      </div>

      {/* StatusStepper §8.2 — BẢY Ô LUỒNG CHÍNH. Hai ô rẽ nhánh không vẽ: tuyến `…/status` không
          nhận trạng thái đích, nên chúng sẽ là hai nút không có gì đứng sau (`PHAN_CHUA_DUNG`). */}
      <ol aria-label="Các bước xử lý phiếu">
        {buocLuongChinh(phieu.status).map((o) => (
          <li key={o.ma}>
            <span className={o.vaiTro === "dangODay" ? "chip chip-hoat-dong" : "chip chip-ngung"}>
              {o.nhan}
            </span>{" "}
            {o.vaiTro === "dangODay" ? "đang ở đây" : o.vaiTro === "daQua" ? "đã qua" : "—"}
          </li>
        ))}
      </ol>
      {giaiThich !== null && <p className="ghi-chu">{giaiThich}</p>}

      <dl className="danh-sach-truong">
        <dt>Lĩnh vực</dt>
        <dd>{nhanLinhVuc(linhVuc)}</dd>

        <dt>Người gửi</dt>
        <dd>{nhanNguoiGui(phieu)}</dd>

        <dt>Nội dung</dt>
        <dd className="noi-dung-phan-anh">{phieu.content}</dd>

        <dt>Địa chỉ</dt>
        <dd>{phieu.address === "" ? "Chưa rõ vị trí" : phieu.address}</dd>

        <dt>Người dân gửi lúc</dt>
        <dd>{nhanThoiDiem(phieu.clock_from)}</dd>

        <dt>Hạn tiếp nhận</dt>
        <dd>
          <span className={lopHan(hanTiepNhan)}>{nhanHan(hanTiepNhan)}</span>
        </dd>

        <dt>Trần phân loại</dt>
        <dd>
          <span className={lopHan(hanPhanLoai)}>{nhanHan(hanPhanLoai)}</span>
        </dd>

        <dt>Hạn xử lý xong</dt>
        <dd>
          <span className={lopHan(hanXuLy)}>{nhanHan(hanXuLy)}</span>
        </dd>

        <dt>Đang giao cho</dt>
        <dd>
          {nhanBoPhan(phieu.unit, tenBoPhan)} · {nhanCanBoXuLy(phieu.assignee)}
        </dd>

        <dt>Hiển thị với người dân</dt>
        <dd>{nhanHienCongKhai(phieu.public)}</dd>

        {/* Kết quả CHỈ hiện khi đã có: một ô trống ở đây trông như một trường chưa điền, trong khi
            phiếu chưa đóng thì nó chưa tồn tại. */}
        {phieu.result !== "" && (
          <>
            <dt>Kết quả xử lý</dt>
            <dd>{phieu.result}</dd>
          </>
        )}
      </dl>

      {loiGhi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loiGhi}
        </p>
      )}

      {/* ── 1. PHÂN LOẠI ─────────────────────────────────────────────────────────────────── */}
      {cong.phanLoai ? (
        phanLoaiDuoc(phieu.status) && (
          <form
            className="form-danh-muc"
            onSubmit={(e) => {
              e.preventDefault();
              if (linhVucChon !== "") phanLoai(linhVucChon);
            }}
          >
            <h4>Phân loại phiếu</h4>
            <p className="ghi-chu">
              Chốt lĩnh vực là hành vi ấn định hạn xử lý xong theo cấu hình thời hạn của xã. Xã chưa
              cấu hình lĩnh vực này thì máy chủ từ chối và nói ra màn cần vào.
            </p>
            <div className="o-chon">
              <label htmlFor="chon-linh-vuc">Lĩnh vực</label>
              <select
                id="chon-linh-vuc"
                value={linhVucChon}
                onChange={(e) => datLinhVucChon(e.target.value)}
              >
                <option value="">— Chọn lĩnh vực —</option>
                {LINH_VUC_PHAN_ANH.map((l) => (
                  <option key={l.ma} value={l.ma}>
                    {l.nhan}
                  </option>
                ))}
              </select>
            </div>
            <button
              type="submit"
              className="nut-chinh"
              disabled={dangGui || linhVucChon === ""}
            >
              Chốt lĩnh vực
            </button>
          </form>
        )
      ) : (
        <p className="trang-thai-rong">{CAU_THIEU_QUYEN_PHAN_LOAI}</p>
      )}

      {/* ── 2. CHUYỂN XỬ LÝ (§8.5) ───────────────────────────────────────────────────────── */}
      {cong.phanCong ? (
        <form
          className="form-danh-muc"
          onSubmit={(e) => {
            e.preventDefault();
            if (boPhanChon !== "") chuyenXuLy(boPhanChon);
          }}
        >
          <h4>Chuyển xử lý, không đổi trạng thái</h4>
          <div className="o-chon">
            <label htmlFor="chon-bo-phan">Bộ phận</label>
            <select
              id="chon-bo-phan"
              value={boPhanChon}
              onChange={(e) => datBoPhanChon(e.target.value)}
            >
              <option value="">— Chọn bộ phận —</option>
              {boPhan.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </select>
          </div>
          {/* Ô CHỌN `Cán bộ xử lý` KHÔNG DỰNG — xem `PHAN_CHUA_DUNG`. Phiếu đi tới bộ phận, đúng
              lựa chọn mặc định `— Để bộ phận phân công —` mà đặc tả đã ghi. */}
          <p className="ghi-chu">
            Phiếu chuyển tới bộ phận; bộ phận tự phân công cán bộ. Ô chọn cán bộ chưa dựng được —
            xem phần chưa dựng được ở đầu màn.
          </p>
          <button type="submit" className="nut-chinh" disabled={dangGui || boPhanChon === ""}>
            Chuyển xử lý
          </button>
        </form>
      ) : (
        <p className="trang-thai-rong">{CAU_THIEU_QUYEN_PHAN_CONG}</p>
      )}

      {/* ── 3. TIẾN TRẠNG THÁI — KHÔNG CÓ CỔNG Ở GIAO DIỆN ───────────────────────────────── */}
      {conBuocKeTiep(phieu.status) && (
        <div className="cum-nut">
          <button type="button" className="nut-chinh" disabled={dangGui} onClick={tienTrangThai}>
            {NHAN_TIEN_TRANG_THAI}
          </button>
          <p className="ghi-chu">{GHI_CHU_TIEN_TRANG_THAI}</p>
        </div>
      )}

      {/* ── 4. ĐÓNG PHIẾU — CỔNG `feedback.resolve`, KHÔNG NỚI THEO LUẬT NẮM GIỮ ─────────── */}
      {cong.dongPhieu ? (
        <form
          className="form-danh-muc"
          onSubmit={(e) => {
            e.preventDefault();
            if (ketQua.trim() !== "") dongPhieuLai(ketQua.trim());
          }}
        >
          <h4>Đóng phiếu</h4>
          <div className="o-nhap">
            <label htmlFor="ket-qua-xu-ly">{NHAN_O_KET_QUA}</label>
            <textarea
              id="ket-qua-xu-ly"
              name="ket-qua-xu-ly"
              rows={3}
              value={ketQua}
              onChange={(e) => datKetQua(e.target.value)}
            />
          </div>
          <p className="ghi-chu">{GHI_CHU_O_KET_QUA}</p>
          <button type="submit" className="nut-chinh" disabled={dangGui || ketQua.trim() === ""}>
            Đóng phiếu
          </button>
        </form>
      ) : (
        <p className="trang-thai-rong">{CAU_THIEU_QUYEN_DONG}</p>
      )}
    </div>
  );
}
