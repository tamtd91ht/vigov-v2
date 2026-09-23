"use client";

import { useEffect, useState, type FormEvent } from "react";

import { khoaChongTrungMoi } from "@/components/danh-ba/nhan-ghi-danh-ba";
import {
  coTrangTruoc,
  sangTrangSau,
  TRANG_DAU,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import {
  layKhoiNhiemVu,
  layLoaiNhiemVu,
  layMucUuTienNhiemVu,
} from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import {
  deNghiLuiHan,
  doiTrangThaiNhiemVu,
  laySoNhiemVu,
  quyetDinhLuiHan,
  taoNhiemVu,
  xoaNhiemVu,
  type LocNhiemVu,
} from "@/lib/api/nhiem-vu";
import type {
  identity_boPhanRa,
  identity_khoiNhiemVuRa,
  page_Result_petitions_nhiemVuRa,
  petitions_deNghiLuiHanRa,
  petitions_loaiNhiemVuRa,
  petitions_mucUuTienRa,
  petitions_nhiemVuRa,
  petitions_taoNhiemVuVao,
} from "@/lib/api/schema.gen";

import {
  CANH_BAO_HAN_MOT_LAN,
  CHI_QUA_HAN_NHAN,
  CHUA_GIAO_BO_PHAN,
  CHUA_PHAN_CONG,
  CHUA_XAC_DINH,
  CHU_THICH_HAI_O_TICK,
  DANG_TAI_SO,
  GHI_CHU_HAN_VIEC_CON,
  GHI_CHU_LANH_DAO_GIAO_VIEC,
  GHI_CHU_LUI_HAN,
  GHI_CHU_TU_SINH_MA,
  MOI_BO_PHAN_NHAN,
  MOI_KHOI_NHAN,
  MOI_LOAI_NHAN,
  MOI_MUC_UU_TIEN_NHAN,
  MOI_NGUON_GIAO,
  MOI_NGUON_GIAO_NHAN,
  MOI_TRANG_THAI,
  MOI_TRANG_THAI_NHAN,
  MO_TA_FORM_GIAO_VIEC,
  O_TRONG,
  PHAM_VI_CUA_TOI,
  PHAM_VI_TOAN_XA,
  PHAN_CHUA_DUNG,
  SO_RONG,
  TIM_PLACEHOLDER,
  TRANG_THAI_CHINH,
  TRANG_THAI_RE_NHANH,
  cauGiaiThichTrangThai,
  chuyenSangDuoc,
  hoanThanhTreHan,
  mocCuoiNgay,
  nhanBoDem,
  nhanHanThe,
  nhanNgay,
  nhanNguonGiao,
  nhanTrangThai,
  oHan,
  quyetDinhDuyetLuiHan,
} from "./nhan-nhiem-vu";

/**
 * Sổ Quản lý nhiệm vụ — `docs/ui-ux/02-nhiem-vu.md` §2 (bố cục), §3 (bộ lọc), §4.2 (bảng danh
 * sách), §5 (drawer chi tiết), §6 (vòng đời) và §7 (form Giao việc mới).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BỐN LẦN TỪ CHỐI CỦA MÁY CHỦ MÀ MÀN HÌNH PHẢI NÓI ĐÚNG, và chúng là lý do màn này không có một
 * hằng chuỗi lỗi nào:
 *
 *   cha `hoan-thanh` còn con     câu từ chối LIỆT KÊ MÃ việc con còn lại
 *   xoá cha còn con              câu từ chối mang SỐ việc con
 *   duyệt lùi hạn                `task.extend` ở cổng + đúng người ghi ở `lanh_dao_giao_viec_ma`
 *   bước không có trong §6       409 kèm tên hai trạng thái
 *
 * Ở cả bốn, DANH SÁCH MÃ VÀ CON SỐ LÀ TOÀN BỘ PHẦN CÓ ÍCH. Nuốt chúng thành "có lỗi xảy ra" để
 * lại cho cán bộ đúng thông tin bằng không: họ biết mình không được phép, và không biết còn vướng
 * ở đâu. Nên mọi nhánh hỏng dưới đây vẽ THẲNG `KetQua.thongBao` — nguyên văn `message` máy chủ
 * viết (luật 9, cấm #2).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ CỔNG QUYỀN Ở GIAO DIỆN, và sự vắng mặt ấy được nói ra ở `PHAN_CHUA_DUNG` chứ không
 * giấu: `lib/quyen.ts` chưa có hằng cho bảy khoá `task.*` và lượt này không sửa tệp ấy. Lớp chặn
 * thật không đổi — mỗi tuyến khai `RequirePermission` và kiểm trên TỪNG yêu cầu (luật 5, cấm #1);
 * thứ thiếu là sự tiện dụng, cùng khuôn `document.read` đã chọn. NGOẠI LỆ DUY NHẤT là lớp hai của
 * ADR 0038, vì nó KHÔNG phải một khoá quyền: nó là phép so mã cán bộ với cột trên bản ghi, và nó
 * vẫn chạy đầy đủ ở đây.
 *
 * KHÔNG GẮN LỚP CSS MỚI: `globals.css` chưa có lớp cho màn này (`.man-nhiem-vu` không tồn tại) và
 * lượt này không được thêm CSS. Mượn lớp của màn khác sẽ trông gần đúng hôm nay rồi lệch hẳn vào
 * ngày lớp ấy đổi vì cái nó thật sự phục vụ. Tên lớp cần thêm đã báo về.
 */

/** Bao nhiêu dòng một trang. */
const SO_DONG_MOI_TRANG = 20;

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

/** Bộ lọc đang chọn trên màn hình. Cùng hình dạng với `LocNhiemVu`, trừ phân trang. */
type BoLoc = Omit<LocNhiemVu, "limit" | "cursor">;

const KHONG_LOC: BoLoc = {};

/** Ba danh mục đổ vào ô chọn, đọc một lần cho cả màn. */
export type DanhMucNhiemVu = {
  readonly loai: readonly petitions_loaiNhiemVuRa[];
  readonly mucUuTien: readonly petitions_mucUuTienRa[];
  readonly khoi: readonly identity_khoiNhiemVuRa[];
  readonly boPhan: readonly identity_boPhanRa[];
};

const KHONG_DANH_MUC: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };

/** Nhãn của một mã danh mục. Mã lạ hiện NGUYÊN VĂN — không dấu gạch, không im lặng bỏ qua. */
function nhanDanhMuc(
  ds: readonly { code: string; label: string }[],
  ma: string,
): string {
  if (ma === "") return O_TRONG;
  return ds.find((m) => m.code === ma)?.label ?? ma;
}

export function SoNhiemVu() {
  const [loc, datLoc] = useState<BoLoc>(KHONG_LOC);
  const [tim, datTim] = useState("");
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);

  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_petitions_nhiemVuRa>;
  } | null>(null);
  const [danhMuc, datDanhMuc] = useState<DanhMucNhiemVu>(KHONG_DANH_MUC);

  const [dangMo, datDangMo] = useState<petitions_nhiemVuRa | null>(null);
  const [loiGhi, datLoiGhi] = useState<string | null>(null);
  const [dangGui, datDangGui] = useState(false);
  const [moFormTao, datMoFormTao] = useState(false);

  const khoa = `${JSON.stringify(loc)}|${nganXep.hienTai ?? ""}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    laySoNhiemVu({ ...loc, limit: SO_DONG_MOI_TRANG, cursor: nganXep.hienTai }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [loc, nganXep.hienTai, khoa]);

  // BỐN DANH MỤC, ĐỌC MỘT LẦN CHO CẢ MÀN. Một danh mục hỏng thì ô lọc tương ứng rỗng — KHÔNG làm
  // hỏng quyển sổ: bốn câu trả lời rời nhau, mỗi cái nói chuyện của nó.
  useEffect(() => {
    let bo = false;
    Promise.all([
      layLoaiNhiemVu(),
      layMucUuTienNhiemVu(),
      layKhoiNhiemVu(),
      layDanhMucBoPhan(),
    ]).then(([loai, uuTien, khoiNV, boPhan]) => {
      if (bo) return;
      datDanhMuc({
        loai: loai.ok ? loai.duLieu.items : [],
        mucUuTien: uuTien.ok ? uuTien.duLieu.items : [],
        khoi: khoiNV.ok ? khoiNV.duLieu.items : [],
        boPhan: boPhan.ok ? boPhan.duLieu.items : [],
      });
    });
    return () => {
      bo = true;
    };
  }, []);

  const phien = usePhien();
  // FAIL CLOSED: chưa đọc xong phiên, hoặc đọc hỏng, thì KHÔNG có mã cán bộ — và không có mã thì
  // không so được với `lanh_dao_giao_viec_ma`, nên nút duyệt lùi hạn ẩn (luật 1, cấm #1).
  const maNguoiDangNhap = phien !== null && phien.ok ? phien.duLieu.staff.code : "";

  const so = taiTu(daTai, khoa);
  const tenBoPhan = new Map(danhMuc.boPhan.map((b) => [b.id, b.name]));

  /** Đổi bộ lọc là về trang đầu: con trỏ của bộ lọc cũ không có nghĩa với bộ lọc mới. */
  function datLocMoi(moi: BoLoc): void {
    datLoc(moi);
    datNganXep(TRANG_DAU);
  }

  /** Một lần ghi xong: giữ nhiệm vụ máy chủ vừa trả, xoá lỗi cũ, và đọc lại quyển sổ. */
  function xongGhi(kq: KetQua<petitions_nhiemVuRa>): void {
    datDangGui(false);
    if (!kq.ok) {
      // NGUYÊN VĂN câu máy chủ — xem khối đầu tệp. Đây là chỗ câu "còn 3 việc con (NV20, NV21,
      // NV22)…" ra tới màn hình.
      datLoiGhi(kq.thongBao);
      return;
    }
    datLoiGhi(null);
    datDangMo(kq.duLieu);
    datLanTai((n) => n + 1);
  }

  function chay(goi: Promise<KetQua<petitions_nhiemVuRa>>): void {
    datDangGui(true);
    goi.then(xongGhi);
  }

  return (
    <section className="man-nhiem-vu" aria-labelledby="tieu-de-so-nhiem-vu">
      <h2 id="tieu-de-so-nhiem-vu">Sổ nhiệm vụ của xã</h2>

      <KhoiChuaDung />

      <div className="cum-nut">
        <button
          type="button"
          className="nut-chinh"
          onClick={() => {
            datMoFormTao((m) => !m);
            datLoiGhi(null);
          }}
          aria-expanded={moFormTao}
        >
          {moFormTao ? "Đóng biểu mẫu giao việc" : "+ Giao việc mới"}
        </button>
      </div>

      {moFormTao && (
        <FormGiaoViec
          danhMuc={danhMuc}
          dangGui={dangGui}
          loi={loiGhi}
          huy={() => {
            datMoFormTao(false);
            datLoiGhi(null);
          }}
          giaoViec={(than, khoaChongTrung) => {
            datDangGui(true);
            taoNhiemVu(than, khoaChongTrung).then((kq) => {
              datDangGui(false);
              if (!kq.ok) {
                datLoiGhi(kq.thongBao);
                return;
              }
              datLoiGhi(null);
              datMoFormTao(false);
              datDangMo(kq.duLieu);
              datLanTai((n) => n + 1);
            });
          }}
        />
      )}

      <HangLoc loc={loc} tim={tim} datTim={datTim} datLoc={datLocMoi} danhMuc={danhMuc} />

      {so.pha === "dangTai" && <p role="status">{DANG_TAI_SO}</p>}
      {so.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {so.thongBao}
        </p>
      )}

      {so.pha === "xong" && (
        <>
          <BangNhiemVu
            nhiemVu={so.duLieu.items}
            danhMuc={danhMuc}
            tenBoPhan={tenBoPhan}
            bayGio={new Date()}
            maDangMo={dangMo?.code ?? null}
            moNhiemVu={(n) => {
              datDangMo(n);
              datLoiGhi(null);
            }}
          />
          <p className="ghi-chu">{nhanBoDem(so.duLieu.items.length)}</p>
          <nav className="dieu-huong-trang" aria-label="Phân trang sổ nhiệm vụ">
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
        <ChiTietNhiemVu
          nhiemVu={dangMo}
          danhMuc={danhMuc}
          tenBoPhan={tenBoPhan}
          bayGio={new Date()}
          maNguoiDangNhap={maNguoiDangNhap}
          dangGui={dangGui}
          loiGhi={loiGhi}
          dong={() => {
            datDangMo(null);
            datLoiGhi(null);
          }}
          doiTrangThai={(trangThai, ghiChu) =>
            chay(doiTrangThaiNhiemVu(dangMo.code, trangThai, ghiChu))
          }
          xoa={(lyDo) => {
            datDangGui(true);
            xoaNhiemVu(dangMo.code, lyDo).then((kq) => {
              datDangGui(false);
              if (!kq.ok) {
                // ĐÂY LÀ CHỖ CÂU "còn 3 việc con chưa xoá — xử lý hoặc xoá các việc con trước" RA
                // TỚI MÀN HÌNH, kèm đúng con số (ADR 0037 quyết định 3).
                datLoiGhi(kq.thongBao);
                return;
              }
              datLoiGhi(null);
              datDangMo(null);
              datLanTai((n) => n + 1);
            });
          }}
          guiDeNghiLuiHan={(hanMoi, lyDo) => deNghiLuiHan(dangMo.code, hanMoi, lyDo)}
          quyetDinh={(deNghiID, duyet, ghiChu) =>
            quyetDinhLuiHan(dangMo.code, deNghiID, duyet, ghiChu)
          }
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

/**
 * Bộ lọc §3.
 *
 * TÁM Ô, ĐÚNG TÁM THAM SỐ MÁY CHỦ NHẬN — không vẽ ô nào không có tuyến đứng sau. Hai ô của đặc
 * tả vắng mặt CÓ CHỦ Ý và lý do ra tới `PHAN_CHUA_DUNG`: tab `Liên quan đến tôi` và ô tick
 * `Sắp đến hạn` đều bị máy chủ TỪ CHỐI bằng 400 kèm lý do, nên vẽ chúng ra là vẽ hai ô mà mỗi lần
 * bấm đổi quyển sổ thành một trang lỗi.
 *
 * Ô `Người thực hiện` là Ô GÕ MÃ CÁN BỘ chứ không phải ô chọn: danh bạ đứng sau `admin.user` —
 * xem `PHAN_CHUA_DUNG`.
 */
export function HangLoc({
  loc,
  tim,
  datTim,
  datLoc,
  danhMuc,
}: {
  loc: BoLoc;
  tim: string;
  datTim: (s: string) => void;
  datLoc: (moi: BoLoc) => void;
  danhMuc: DanhMucNhiemVu;
}) {
  function timNgay(e: FormEvent) {
    e.preventDefault();
    const canGon = tim.trim();
    datLoc({ ...loc, tim: canGon === "" ? undefined : canGon });
  }

  return (
    <div className="hang-loc">
      {/* HAI TAB PHẠM VI. `mine` KHÔNG mang theo danh tính nào — máy chủ điền mã cán bộ từ PHIÊN
          (`nhiem_vu.go:273-288`). Một tab gửi lên `?assignee=CB-…` của chính mình sẽ là client tự
          khai mình là ai, điều luật 1 cấm #2 không cho phép. */}
      <div className="o-chon" role="group" aria-label="Phạm vi">
        <button
          type="button"
          className="nut-phu"
          aria-pressed={loc.phamVi !== "mine"}
          onClick={() => datLoc({ ...loc, phamVi: undefined })}
        >
          {PHAM_VI_TOAN_XA}
        </button>
        <button
          type="button"
          className="nut-phu"
          aria-pressed={loc.phamVi === "mine"}
          onClick={() => datLoc({ ...loc, phamVi: "mine" })}
        >
          {PHAM_VI_CUA_TOI}
        </button>
      </div>

      {/* Ô TÌM GỬI BẰNG SUBMIT, KHÔNG GỬI THEO TỪNG PHÍM: mỗi phím là một lời gọi mang chữ cán bộ
          đang gõ vào một URL, và một URL đi vào mọi log truy cập (luật 3, cấm #4). */}
      <form className="form-tra-cuu" onSubmit={timNgay} role="search">
        <div className="o-nhap">
          <label htmlFor="tim-nhiem-vu">Tìm trong sổ</label>
          <input
            id="tim-nhiem-vu"
            name="tim-nhiem-vu"
            value={tim}
            placeholder={TIM_PLACEHOLDER}
            onChange={(e) => datTim(e.target.value)}
            autoComplete="off"
            // Máy chủ trả 400 khi quá 200 ký tự (`store.TimNhiemVuToiDa`). Chặn ở ô nhập để cán bộ
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
        <label htmlFor="loc-loai">Loại nhiệm vụ</label>
        <select
          id="loc-loai"
          value={loc.loai ?? ""}
          onChange={(e) => datLoc({ ...loc, loai: e.target.value || undefined })}
        >
          <option value="">{MOI_LOAI_NHAN}</option>
          {danhMuc.loai.map((l) => (
            <option key={l.code} value={l.code}>
              {l.label}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="loc-khoi">Khối</label>
        <select
          id="loc-khoi"
          value={loc.khoi ?? ""}
          onChange={(e) => datLoc({ ...loc, khoi: e.target.value || undefined })}
        >
          <option value="">{MOI_KHOI_NHAN}</option>
          {danhMuc.khoi.map((k) => (
            <option key={k.code} value={k.code}>
              {k.label}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="loc-uu-tien">Mức ưu tiên</label>
        <select
          id="loc-uu-tien"
          value={loc.mucUuTien ?? ""}
          onChange={(e) => datLoc({ ...loc, mucUuTien: e.target.value || undefined })}
        >
          <option value="">{MOI_MUC_UU_TIEN_NHAN}</option>
          {/* KHÔNG SẮP XẾP LẠI MẢNG NÀY: thứ tự `items` LÀ thang bậc của xã, không phải sở thích
              trình bày (`muc_uu_tien_nhiem_vu.go`). */}
          {danhMuc.mucUuTien.map((m) => (
            <option key={m.code} value={m.code}>
              {m.label}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="loc-nguon-giao">Nguồn giao</label>
        <select
          id="loc-nguon-giao"
          value={loc.nguonGiao ?? ""}
          onChange={(e) => datLoc({ ...loc, nguonGiao: e.target.value || undefined })}
        >
          <option value="">{MOI_NGUON_GIAO_NHAN}</option>
          {MOI_NGUON_GIAO.map((ma) => (
            <option key={ma} value={ma}>
              {nhanNguonGiao(ma)}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="loc-bo-phan">Bộ phận</label>
        <select
          id="loc-bo-phan"
          value={loc.boPhanID ?? ""}
          onChange={(e) => datLoc({ ...loc, boPhanID: e.target.value || undefined })}
        >
          <option value="">{MOI_BO_PHAN_NHAN}</option>
          {danhMuc.boPhan.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>
      </div>

      <div className="o-nhap">
        <label htmlFor="loc-nguoi-thuc-hien">Người thực hiện (mã cán bộ)</label>
        <input
          id="loc-nguoi-thuc-hien"
          name="loc-nguoi-thuc-hien"
          value={loc.nguoiThucHienMa ?? ""}
          placeholder="CB-…"
          autoComplete="off"
          onChange={(e) => datLoc({ ...loc, nguoiThucHienMa: e.target.value || undefined })}
        />
      </div>

      <div className="o-chon">
        <label htmlFor="loc-qua-han">
          <input
            id="loc-qua-han"
            type="checkbox"
            checked={loc.chiTreHan === true}
            // Ô bỏ tích thì tham số VẮNG MẶT HẲN, không gửi `late=false` — máy chủ chỉ nhận đúng
            // chuỗi `true` và trả 400 cho mọi giá trị khác.
            onChange={(e) => datLoc({ ...loc, chiTreHan: e.target.checked ? true : undefined })}
          />{" "}
          {CHI_QUA_HAN_NHAN}
        </label>
      </div>
    </div>
  );
}

/**
 * Bảng Danh sách §4.2.
 *
 * KHÔNG CÓ CỘT Ô TICK: `Xoá đã chọn` không có tuyến nào (xem `PHAN_CHUA_DUNG`), và một ô tick
 * không dẫn tới thao tác nào là một ô tick mời cán bộ chọn hai mươi dòng rồi không tìm thấy nút.
 *
 * KHÔNG CÓ CHIP `{n} việc con`: phản hồi không mang số việc con, và đếm trong trang đang mở cho ra
 * một con số PHỤ THUỘC VÀO TRANG.
 */
export function BangNhiemVu({
  nhiemVu,
  danhMuc,
  tenBoPhan,
  bayGio,
  maDangMo,
  moNhiemVu,
}: {
  nhiemVu: readonly petitions_nhiemVuRa[];
  danhMuc: DanhMucNhiemVu;
  tenBoPhan: ReadonlyMap<string, string>;
  bayGio: Date;
  maDangMo: string | null;
  moNhiemVu: (n: petitions_nhiemVuRa) => void;
}) {
  if (nhiemVu.length === 0) return <p className="trang-thai-rong">{SO_RONG}</p>;

  return (
    <div className="bang-cuon" role="region" aria-label="Sổ nhiệm vụ" tabIndex={0}>
      <table className="bang-danh-muc">
        <thead>
          <tr>
            <th scope="col">Mã</th>
            <th scope="col">Tên việc</th>
            <th scope="col">Người thực hiện</th>
            <th scope="col">Bộ phận</th>
            <th scope="col">Ưu tiên</th>
            <th scope="col">Hạn</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">
              <span className="an-thi-giac">Thao tác</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {nhiemVu.map((n) => {
            const o = oHan(n.due_at, bayGio);
            return (
              <tr key={n.code}>
                <td className="ma-muc">{n.code}</td>
                <td>
                  {n.title}
                  <span className="dong-phu">{nhanNguonGiao(n.source)}</span>
                </td>
                <td>{n.assignee === "" ? CHUA_PHAN_CONG : n.assignee}</td>
                <td>{n.unit === "" ? O_TRONG : (tenBoPhan.get(n.unit) ?? n.unit)}</td>
                <td>{nhanDanhMuc(danhMuc.mucUuTien, n.priority)}</td>
                <td>
                  {o.ngay}
                  {/* PHẦN TRỄ TÁCH RIÊNG để tô đỏ, đúng §4.2. `oHan` trả hai mảnh sẵn, nên ở đây
                      không có phép cắt chuỗi nào. */}
                  {o.phanTre !== "" && <span className="nhan-lech"> {o.phanTre}</span>}
                </td>
                <td>
                  <span className="chip chip-ngung">{nhanTrangThai(n.status)}</span>
                  {hoanThanhTreHan(n.completed_at, n.original_due_at) && (
                    <span className="chip chip-hoat-dong">Hoàn thành trễ hạn</span>
                  )}
                </td>
                <td className="o-thao-tac">
                  <button
                    type="button"
                    className="nut-phu"
                    onClick={() => moNhiemVu(n)}
                    aria-expanded={n.code === maDangMo}
                  >
                    {n.code === maDangMo ? "Đang mở" : `Mở ${n.code}`}
                  </button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

/**
 * Chi tiết §5 — dải bước §5.2, ba ô tóm tắt §5.3, hai hạn §5.6, đổi trạng thái §6, đề nghị lùi
 * hạn §5.8 và xoá §11.5.
 *
 * ĐẶC TẢ GỌI NÓ LÀ DetailDrawer "mở gần toàn màn hình". Ở đây nó là một khối nằm dưới bảng —
 * KHÔNG phải một lớp phủ — vì một lớp phủ cần lớp CSS chưa có trong `globals.css`, và lượt này
 * không được thêm CSS. Đã báo về tên lớp cần thêm.
 */
export function ChiTietNhiemVu({
  nhiemVu,
  danhMuc,
  tenBoPhan,
  bayGio,
  maNguoiDangNhap,
  dangGui,
  loiGhi,
  dong,
  doiTrangThai,
  xoa,
  guiDeNghiLuiHan,
  quyetDinh,
}: {
  nhiemVu: petitions_nhiemVuRa;
  danhMuc: DanhMucNhiemVu;
  tenBoPhan: ReadonlyMap<string, string>;
  bayGio: Date;
  /** `phien.staff.code` — mã nghiệp vụ `CB-…`, rỗng khi chưa đọc được phiên. */
  maNguoiDangNhap: string;
  dangGui: boolean;
  loiGhi: string | null;
  dong: () => void;
  doiTrangThai: (trangThai: string, ghiChu?: string) => void;
  xoa: (lyDo: string) => void;
  guiDeNghiLuiHan: (hanMoiISO: string, lyDo: string) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  quyetDinh: (
    deNghiID: string,
    duyet: boolean,
    ghiChu?: string,
  ) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
}) {
  const [ghiChuChuyen, datGhiChuChuyen] = useState("");
  const [lyDoXoa, datLyDoXoa] = useState("");

  const o = oHan(nhiemVu.due_at, bayGio);
  const giaiThich = cauGiaiThichTrangThai(nhiemVu.status);
  const congDuyet = quyetDinhDuyetLuiHan(
    maNguoiDangNhap,
    nhiemVu.assigner,
    // ⚠ LỚP MỘT (`task.extend`) TRUYỀN VÀO `true` VÌ MÀN HÌNH CHƯA ĐỌC ĐƯỢC KHOÁ NÀO — xem
    // `PHAN_CHUA_DUNG`. Hệ quả có giới hạn và được nói thẳng: lãnh đạo được ghi trên bản ghi mà
    // THIẾU `task.extend` sẽ thấy nút rồi nhận nguyên văn câu 403 của máy chủ. Lớp hai — phép so
    // mã cán bộ của ADR 0038 — vẫn chạy đủ ngay dưới đây, và nó là lớp mà một khoá quyền không
    // diễn đạt được.
    true,
  );

  return (
    <div className="khoi-chi-tiet" aria-labelledby="tieu-de-chi-tiet-nhiem-vu">
      <div className="dau-khoi-chi-tiet">
        <h3 id="tieu-de-chi-tiet-nhiem-vu" className="ma-muc">
          [{nhiemVu.code}] {nhiemVu.title}
        </h3>
        <button type="button" className="nut-phu" onClick={dong} aria-label="Đóng chi tiết nhiệm vụ">
          ✕
        </button>
      </div>

      <p className="ghi-chu">
        {nhiemVu.unit === ""
          ? CHUA_GIAO_BO_PHAN
          : (tenBoPhan.get(nhiemVu.unit) ?? nhiemVu.unit)}{" "}
        · {nhiemVu.assignee === "" ? CHUA_PHAN_CONG : nhiemVu.assignee}
      </p>

      {/* ── DẢI BƯỚC §5.2 ─────────────────────────────────────────────────────────────────
          THỜI GIAN ĐÃ Ở TRẠNG THÁI (`19 ngày 23 giờ`) HIỆN DẤU GẠCH, không hiện số 0: mốc đổi
          trạng thái gần nhất nằm trong nhật ký, và hợp đồng không có tuyến nhật ký nào. Một con
          số 0 ở đây đọc ra là "vừa chuyển xong", đúng điều ngược lại với "không biết". */}
      <ol aria-label="Các bước của vòng đời nhiệm vụ">
        {TRANG_THAI_CHINH.map((ma) => (
          <li key={ma}>
            <span
              className={ma === nhiemVu.status ? "chip chip-hoat-dong" : "chip chip-ngung"}
            >
              {nhanTrangThai(ma)}
            </span>{" "}
            {O_TRONG}
          </li>
        ))}
      </ol>
      <p className="ghi-chu">
        Rẽ nhánh:{" "}
        {TRANG_THAI_RE_NHANH.map((ma) => (
          <span key={ma}>
            <span
              className={ma === nhiemVu.status ? "chip chip-hoat-dong" : "chip chip-ngung"}
            >
              {nhanTrangThai(ma)}
            </span>{" "}
            {O_TRONG}{" "}
          </span>
        ))}
      </p>
      {giaiThich !== "" && <p className="ghi-chu">{giaiThich}</p>}

      <dl className="danh-sach-truong">
        {/* §5.3 — HAI MẢNH, KHÔNG MỘT: `Trễ 87 ngày` (đỏ) **và** `Hạn 20/6/2026`. Chỉ hiện số
            ngày trễ thì cán bộ không biết hạn là ngày nào để đối chiếu với văn bản giấy; chỉ hiện
            ngày thì con số phải tự nhẩm. Đặc tả đòi cả hai và cả hai đều có việc riêng. */}
        <dt>Hạn xử lý</dt>
        <dd>
          {o.phanTre !== "" ? (
            <>
              <span className="nhan-lech">{nhanHanThe(nhiemVu.due_at, bayGio)}</span> · Hạn{" "}
              {nhanNgay(nhiemVu.due_at)}
            </>
          ) : (
            nhanHanThe(nhiemVu.due_at, bayGio)
          )}
        </dd>

        {/* §5.6 — HAI HẠN CẠNH NHAU, và đó là toàn bộ điểm của hai cột. `HẠN BAN ĐẦU` không đổi
            khi gia hạn; tỷ lệ đúng hạn §11.3 đếm theo nó. */}
        <dt>Hạn ban đầu</dt>
        <dd>{nhanNgay(nhiemVu.original_due_at)}</dd>

        <dt>Đang giao cho</dt>
        <dd>
          {nhiemVu.unit === ""
            ? CHUA_GIAO_BO_PHAN
            : (tenBoPhan.get(nhiemVu.unit) ?? nhiemVu.unit)}{" "}
          · {nhiemVu.assignee === "" ? CHUA_PHAN_CONG : nhiemVu.assignee}
        </dd>

        <dt>Mức ưu tiên</dt>
        <dd>
          {nhanDanhMuc(danhMuc.mucUuTien, nhiemVu.priority)} · {nhiemVu.progress}% tiến độ ghi nhận
        </dd>

        <dt>Loại nhiệm vụ</dt>
        <dd>{nhanDanhMuc(danhMuc.loai, nhiemVu.type)}</dd>

        <dt>Khối</dt>
        <dd>{nhanDanhMuc(danhMuc.khoi, nhiemVu.bloc)}</dd>

        <dt>Nguồn giao</dt>
        <dd>{nhanNguonGiao(nhiemVu.source)}</dd>

        <dt>Lãnh đạo giao việc</dt>
        <dd>{nhiemVu.assigner === "" ? O_TRONG : nhiemVu.assigner}</dd>

        <dt>Cơ quan chủ trì tham mưu</dt>
        <dd>
          {nhiemVu.lead_unit === ""
            ? O_TRONG
            : (tenBoPhan.get(nhiemVu.lead_unit) ?? nhiemVu.lead_unit)}
        </dd>

        <dt>Chuyên viên theo dõi</dt>
        <dd>{nhiemVu.monitor === "" ? O_TRONG : nhiemVu.monitor}</dd>

        <dt>Mô tả nhiệm vụ</dt>
        <dd>{nhiemVu.description === "" ? O_TRONG : nhiemVu.description}</dd>

        <dt>Tóm tắt kết quả thực hiện</dt>
        <dd>{nhiemVu.result_summary === "" ? O_TRONG : nhiemVu.result_summary}</dd>

        <dt>Ghi chú</dt>
        <dd>{nhiemVu.note === "" ? O_TRONG : nhiemVu.note}</dd>

        <dt>Lãnh đạo xã đã phê duyệt hoàn thành</dt>
        <dd>{nhiemVu.leader_approved ? "Đã đánh dấu" : "Chưa đánh dấu"}</dd>

        <dt>Cấp trên đã công nhận hoàn thành</dt>
        <dd>{nhiemVu.superior_acknowledged ? "Đã đánh dấu" : "Chưa đánh dấu"}</dd>
      </dl>
      <p className="ghi-chu">{CHU_THICH_HAI_O_TICK}</p>

      {loiGhi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loiGhi}
        </p>
      )}

      {/* ── ĐỔI TRẠNG THÁI §6 ─────────────────────────────────────────────────────────────
          CHỈ VẼ NHỮNG BƯỚC §6 CÓ. Một nút thừa ở đây không mở được gì — máy chủ kiểm lại bằng
          `ChuyenTrangThaiDuoc` — nhưng một nút thiếu thì cán bộ báo ngay, vì họ đang cần bấm nó.

          BƯỚC `hoan-thanh` VẪN HIỆN KỂ CẢ KHI CÒN VIỆC CON: màn hình không biết có việc con hay
          không (phản hồi không mang số ấy), và câu từ chối của máy chủ LIỆT KÊ MÃ việc con còn
          lại — thông tin cán bộ cần, và là thông tin màn hình không tự dựng được. */}
      <div className="form-danh-muc">
        <h4>Chuyển trạng thái</h4>
        <div className="o-nhap">
          <label htmlFor="ghi-chu-chuyen-trang-thai">Ghi chú (không bắt buộc)</label>
          <input
            id="ghi-chu-chuyen-trang-thai"
            name="ghi-chu-chuyen-trang-thai"
            value={ghiChuChuyen}
            autoComplete="off"
            onChange={(e) => datGhiChuChuyen(e.target.value)}
          />
        </div>
        <div className="cum-nut">
          {MOI_TRANG_THAI.filter((t) => chuyenSangDuoc(nhiemVu.status, t)).map((t) => (
            <button
              key={t}
              type="button"
              className="nut-phu"
              disabled={dangGui}
              onClick={() => doiTrangThai(t, ghiChuChuyen.trim())}
            >
              Chuyển sang {nhanTrangThai(t)}
            </button>
          ))}
        </div>
        {MOI_TRANG_THAI.filter((t) => chuyenSangDuoc(nhiemVu.status, t)).length === 0 && (
          <p className="trang-thai-rong">
            Vòng đời §6 không có lối ra khỏi trạng thái này — nhiệm vụ khép lại tại đây.
          </p>
        )}
      </div>

      <KhoiLuiHan
        congDuyet={congDuyet}
        hanHienTai={nhiemVu.due_at}
        dangGui={dangGui}
        guiDeNghi={guiDeNghiLuiHan}
        quyetDinh={quyetDinh}
      />

      {/* ── XOÁ MỀM §11.5 ────────────────────────────────────────────────────────────────── */}
      <form
        className="form-danh-muc"
        onSubmit={(e) => {
          e.preventDefault();
          if (lyDoXoa.trim() !== "") xoa(lyDoXoa.trim());
        }}
      >
        <h4>Xoá nhiệm vụ khỏi sổ</h4>
        <p className="ghi-chu">
          Xoá mềm: dòng ở lại cùng người xoá và lý do, mã sổ đã cấp thì không bao giờ cấp lại. Nhiệm
          vụ còn việc con chưa xoá thì máy chủ từ chối và nói rõ còn mấy việc.
        </p>
        <div className="o-nhap">
          <label htmlFor="ly-do-xoa-nhiem-vu">Lý do xoá (bắt buộc)</label>
          <input
            id="ly-do-xoa-nhiem-vu"
            name="ly-do-xoa-nhiem-vu"
            value={lyDoXoa}
            autoComplete="off"
            onChange={(e) => datLyDoXoa(e.target.value)}
          />
        </div>
        <button type="submit" className="nut-xoa" disabled={dangGui || lyDoXoa.trim() === ""}>
          Xoá nhiệm vụ
        </button>
      </form>
    </div>
  );
}

/**
 * §5.8 — đề nghị lùi hạn, và quyết định của lãnh đạo giao việc.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * HAI NỬA CỦA KHỐI NÀY ĐỨNG SAU HAI KHOÁ KHÁC NHAU, và gộp chúng là làm hỏng đúng điều ADR 0038
 * dựng ra:
 *
 *   gửi đề nghị     `task.update` — việc của NGƯỜI ĐANG LÀM. Ô này luôn hiện.
 *   duyệt / từ chối `task.extend` ở cổng **và** đúng người ghi ở `lanh_dao_giao_viec_ma`.
 *
 * Bỏ lớp thứ hai thì **mọi lãnh đạo cầm khoá duyệt được mọi nhiệm vụ của cả xã**, kể cả của bộ
 * phận họ không liên quan — vì luật 5 kiểm `(tenant_id, role, permission)` và không có chiều "bản
 * ghi nào".
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function KhoiLuiHan({
  congDuyet,
  hanHienTai,
  dangGui,
  guiDeNghi,
  quyetDinh,
}: {
  congDuyet: ReturnType<typeof quyetDinhDuyetLuiHan>;
  hanHienTai: string | null;
  dangGui: boolean;
  guiDeNghi: (hanMoiISO: string, lyDo: string) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  quyetDinh: (
    deNghiID: string,
    duyet: boolean,
    ghiChu?: string,
  ) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
}) {
  const [hanMoi, datHanMoi] = useState("");
  const [lyDo, datLyDo] = useState("");
  const [dangChay, datDangChay] = useState(false);
  const [loi, datLoi] = useState<string | null>(null);
  const [deNghi, datDeNghi] = useState<petitions_deNghiLuiHanRa | null>(null);

  function xong(kq: KetQua<petitions_deNghiLuiHanRa>): void {
    datDangChay(false);
    if (!kq.ok) {
      datLoi(kq.thongBao);
      return;
    }
    datLoi(null);
    datDeNghi(kq.duLieu);
  }

  return (
    <div className="form-danh-muc">
      <h4>Đề nghị lùi hạn</h4>
      <p className="ghi-chu">{GHI_CHU_LUI_HAN}</p>

      {hanHienTai === null && (
        <p className="trang-thai-rong">
          Nhiệm vụ này không có hạn, nên không có gì để lùi. Hạn chỉ đặt được một lần, lúc tạo việc.
        </p>
      )}

      {hanHienTai !== null && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (hanMoi === "" || lyDo.trim() === "") return;
            datDangChay(true);
            guiDeNghi(mocCuoiNgay(hanMoi), lyDo.trim()).then(xong);
          }}
        >
          <div className="o-nhap">
            <label htmlFor="han-moi-lui-han">Hạn mới</label>
            <input
              id="han-moi-lui-han"
              name="han-moi-lui-han"
              type="date"
              value={hanMoi}
              onChange={(e) => datHanMoi(e.target.value)}
            />
          </div>
          <div className="o-nhap">
            <label htmlFor="ly-do-lui-han">Lý do</label>
            <input
              id="ly-do-lui-han"
              name="ly-do-lui-han"
              value={lyDo}
              autoComplete="off"
              onChange={(e) => datLyDo(e.target.value)}
            />
          </div>
          <button
            type="submit"
            className="nut-chinh"
            disabled={dangGui || dangChay || hanMoi === "" || lyDo.trim() === ""}
          >
            Gửi đề nghị lùi hạn
          </button>
        </form>
      )}

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      {deNghi !== null && (
        <dl className="danh-sach-truong">
          <dt>Đề nghị đang chờ</dt>
          <dd>
            Hạn mới {nhanNgay(deNghi.new_due_at)} · người đề nghị {deNghi.requested_by} · trạng thái{" "}
            {deNghi.status}
          </dd>
        </dl>
      )}

      {/* ── QUYẾT ĐỊNH — LỚP HAI CỦA ADR 0038 ────────────────────────────────────────────
          ẨN MỘT NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP (luật 5, cấm #1): máy chủ kiểm lại cùng
          hai câu hỏi ấy trong giao dịch, trên dòng đọc dưới khoá. Việc của khối này là để cán bộ
          không bấm vào một thứ chắc chắn bị từ chối, và để người KHÔNG phải lãnh đạo giao việc đọc
          được vì sao. */}
      {!congDuyet.hien && congDuyet.vi !== "thieu-quyen" && (
        <p className="trang-thai-rong">{congDuyet.thongBao}</p>
      )}

      {congDuyet.hien &&
        (deNghi === null ? (
          <p className="trang-thai-rong">
            Bạn là lãnh đạo giao việc của nhiệm vụ này, nhưng hợp đồng chưa có tuyến liệt kê đề nghị
            đang chờ — xem phần chưa dựng được ở đầu màn.
          </p>
        ) : (
          <div className="cum-nut">
            <button
              type="button"
              className="nut-chinh"
              disabled={dangGui || dangChay}
              onClick={() => {
                datDangChay(true);
                quyetDinh(deNghi.id, true).then(xong);
              }}
            >
              Duyệt lùi hạn
            </button>
            <button
              type="button"
              className="nut-phu"
              disabled={dangGui || dangChay}
              onClick={() => {
                datDangChay(true);
                quyetDinh(deNghi.id, false).then(xong);
              }}
            >
              Từ chối
            </button>
          </div>
        ))}
    </div>
  );
}

/**
 * Form `Giao việc mới` §7.
 *
 * HAI LOẠI, HAI BỘ TRƯỜNG (§7.2 / §7.3) — nhưng BA NHÓM VĂN BẢN CỦA §7.2 KHÔNG DỰNG ĐƯỢC: bảng
 * `nhiem_vu_van_ban` chưa tồn tại và `petitions.taoNhiemVuVao` không nhận chúng. Vẽ ba danh sách
 * động rỗng là mời cán bộ gõ vào một chỗ không đi tới đâu. Xem `PHAN_CHUA_DUNG`.
 *
 * `Tự sinh mã` MẶC ĐỊNH BẬT, đúng §7.1: mã do máy chủ cấp theo dãy `NV01, NV02…`, và một mã đã
 * cấp thì không bao giờ cấp lại kể cả sau xoá mềm (luật 7, bất biến 3).
 *
 * KHOÁ CHỐNG TRÙNG SỐNG BẰNG ĐỜI MỘT LẦN MỞ FORM. Sinh mới ở mỗi lần bấm thì lần bấm lại sau một
 * lỗi mạng là một khoá mới — tức đúng cái khoá chống trùng sinh ra để chặn, vì lần gửi đầu CÓ THỂ
 * đã tới máy chủ và đã cấp một số sổ.
 */
export function FormGiaoViec({
  danhMuc,
  dangGui,
  loi,
  huy,
  giaoViec,
  maChaCoSan,
}: {
  danhMuc: DanhMucNhiemVu;
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  giaoViec: (than: petitions_taoNhiemVuVao, khoaChongTrung: string) => void;
  /** Id nội bộ của việc cha, khi có. Hôm nay KHÔNG BAO GIỜ có — xem `PHAN_CHUA_DUNG`. */
  maChaCoSan?: string;
}) {
  const [khoaChongTrung] = useState(khoaChongTrungMoi);
  const [tuSinhMa, datTuSinhMa] = useState(true);
  const [ma, datMa] = useState("");
  const [loai, datLoai] = useState("");
  const [khoi, datKhoi] = useState("");
  const [tieuDe, datTieuDe] = useState("");
  const [moTa, datMoTa] = useState("");
  const [mucUuTien, datMucUuTien] = useState("");
  const [boPhan, datBoPhan] = useState("");
  const [nguoiThucHien, datNguoiThucHien] = useState("");
  const [lanhDaoGiaoViec, datLanhDaoGiaoViec] = useState("");
  const [coQuanChuTri, datCoQuanChuTri] = useState("");
  const [chuyenVien, datChuyenVien] = useState("");
  const [han, datHan] = useState("");

  // Loại mặc định lấy từ DANH MỤC CỦA XÃ (`is_default`), không gõ cứng `theo-van-ban`: §7.1 nói
  // loại `Theo văn bản` là mặc định, nhưng đó là một dòng danh mục xã sửa được.
  const loaiMacDinh = danhMuc.loai.find((l) => l.is_default)?.code ?? "";
  const loaiChon = loai === "" ? loaiMacDinh : loai;

  function gui(e: FormEvent) {
    e.preventDefault();
    if (tieuDe.trim() === "" || loaiChon === "") return;

    const than: petitions_taoNhiemVuVao = {
      auto_code: tuSinhMa,
      type: loaiChon,
      title: tieuDe.trim(),
    };
    if (!tuSinhMa && ma.trim() !== "") than.code = ma.trim();
    if (khoi !== "") than.bloc = khoi;
    if (moTa.trim() !== "") than.description = moTa.trim();
    if (mucUuTien !== "") than.priority = mucUuTien;
    if (boPhan !== "") than.unit = boPhan;
    if (nguoiThucHien.trim() !== "") than.assignee = nguoiThucHien.trim();
    if (lanhDaoGiaoViec.trim() !== "") than.assigner = lanhDaoGiaoViec.trim();
    if (coQuanChuTri !== "") than.lead_unit = coQuanChuTri;
    if (chuyenVien.trim() !== "") than.monitor = chuyenVien.trim();
    // HẠN: ô ngày → mốc cuối ngày theo giờ Việt Nam. Bỏ trống thì trường VẮNG MẶT HẲN, không gửi
    // chuỗi rỗng — `due_at` là con trỏ ở máy chủ và "không có hạn" là một trạng thái thật (§4.1
    // vẽ nó thành `Hạn —`).
    if (han !== "") than.due_at = mocCuoiNgay(han);
    if (maChaCoSan !== undefined && maChaCoSan !== "") than.parent = maChaCoSan;

    giaoViec(than, khoaChongTrung);
  }

  return (
    <form className="form-danh-muc" onSubmit={gui}>
      <h4>Giao việc mới</h4>
      <p className="ghi-chu">{MO_TA_FORM_GIAO_VIEC}</p>

      <div className="o-chon">
        <label htmlFor="giao-loai">Loại nhiệm vụ</label>
        <select id="giao-loai" value={loaiChon} onChange={(e) => datLoai(e.target.value)}>
          {danhMuc.loai.length === 0 && <option value="">{CHUA_XAC_DINH}</option>}
          {danhMuc.loai.map((l) => (
            <option key={l.code} value={l.code}>
              {l.label}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="giao-khoi">Khối nhiệm vụ</label>
        <select id="giao-khoi" value={khoi} onChange={(e) => datKhoi(e.target.value)}>
          <option value="">{CHUA_XAC_DINH}</option>
          {danhMuc.khoi.map((k) => (
            <option key={k.code} value={k.code}>
              {k.label}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="giao-tu-sinh-ma">
          <input
            id="giao-tu-sinh-ma"
            type="checkbox"
            checked={tuSinhMa}
            onChange={(e) => datTuSinhMa(e.target.checked)}
          />{" "}
          Tự sinh mã
        </label>
      </div>
      <p className="ghi-chu">{GHI_CHU_TU_SINH_MA}</p>

      {!tuSinhMa && (
        <div className="o-nhap">
          <label htmlFor="giao-ma">Mã nhiệm vụ</label>
          <input
            id="giao-ma"
            name="giao-ma"
            value={ma}
            autoComplete="off"
            onChange={(e) => datMa(e.target.value)}
          />
        </div>
      )}

      <div className="o-nhap">
        <label htmlFor="giao-tieu-de">Nội dung nhiệm vụ / Trích yếu văn bản</label>
        <input
          id="giao-tieu-de"
          name="giao-tieu-de"
          value={tieuDe}
          autoComplete="off"
          onChange={(e) => datTieuDe(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="giao-mo-ta">Mô tả</label>
        <textarea
          id="giao-mo-ta"
          name="giao-mo-ta"
          rows={3}
          value={moTa}
          onChange={(e) => datMoTa(e.target.value)}
        />
      </div>

      <div className="o-chon">
        <label htmlFor="giao-uu-tien">Mức ưu tiên</label>
        <select
          id="giao-uu-tien"
          value={mucUuTien}
          onChange={(e) => datMucUuTien(e.target.value)}
        >
          <option value="">{CHUA_XAC_DINH}</option>
          {danhMuc.mucUuTien.map((m) => (
            <option key={m.code} value={m.code}>
              {m.label}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="giao-bo-phan">Đơn vị thực hiện</label>
        <select id="giao-bo-phan" value={boPhan} onChange={(e) => datBoPhan(e.target.value)}>
          <option value="">{CHUA_XAC_DINH}</option>
          {danhMuc.boPhan.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>
      </div>

      <div className="o-chon">
        <label htmlFor="giao-co-quan-chu-tri">Cơ quan chủ trì tham mưu</label>
        <select
          id="giao-co-quan-chu-tri"
          value={coQuanChuTri}
          onChange={(e) => datCoQuanChuTri(e.target.value)}
        >
          <option value="">{CHUA_XAC_DINH}</option>
          {danhMuc.boPhan.map((b) => (
            <option key={b.id} value={b.id}>
              {b.name}
            </option>
          ))}
        </select>
      </div>

      {/* BA Ô GÕ MÃ CÁN BỘ, KHÔNG PHẢI BA Ô CHỌN — danh bạ đứng sau `admin.user`, xem
          `PHAN_CHUA_DUNG`. Giá trị là MÃ NGHIỆP VỤ `CB-…`, đúng loại định danh ba cột kia giữ:
          một ULID ở đây được máy chủ nhận nhưng không khớp cán bộ nào, và không có gì đỏ ở đâu
          (luật 6, bất biến 8). */}
      <div className="o-nhap">
        <label htmlFor="giao-nguoi-thuc-hien">Người thực hiện (mã cán bộ, bỏ trống để bộ phận tự phân công)</label>
        <input
          id="giao-nguoi-thuc-hien"
          name="giao-nguoi-thuc-hien"
          value={nguoiThucHien}
          placeholder="CB-…"
          autoComplete="off"
          onChange={(e) => datNguoiThucHien(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="giao-lanh-dao">Lãnh đạo giao việc (mã cán bộ)</label>
        <input
          id="giao-lanh-dao"
          name="giao-lanh-dao"
          value={lanhDaoGiaoViec}
          placeholder="CB-…"
          autoComplete="off"
          onChange={(e) => datLanhDaoGiaoViec(e.target.value)}
        />
      </div>
      {/* KHÔNG CHỈ LÀ NƠI NHẬN THÔNG BÁO: ô này quyết định AI DUYỆT ĐƯỢC ĐỀ NGHỊ LÙI HẠN (ADR
          0038), và bỏ trống nghĩa là KHÔNG AI duyệt được — vĩnh viễn, vì `PATCH` cố ý không sửa
          được cột này. Câu ấy đứng cạnh ô chứ không nằm trong tài liệu. */}
      <p className="ghi-chu">
        {GHI_CHU_LANH_DAO_GIAO_VIEC} Bỏ trống thì không ai duyệt được đề nghị lùi hạn của nhiệm vụ
        này, và ô này không sửa lại được sau khi tạo.
      </p>

      <div className="o-nhap">
        <label htmlFor="giao-chuyen-vien">Chuyên viên Văn phòng tham mưu / theo dõi (mã cán bộ)</label>
        <input
          id="giao-chuyen-vien"
          name="giao-chuyen-vien"
          value={chuyenVien}
          placeholder="CB-…"
          autoComplete="off"
          onChange={(e) => datChuyenVien(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="giao-han">Hạn hoàn thành</label>
        <input
          id="giao-han"
          name="giao-han"
          type="date"
          value={han}
          onChange={(e) => datHan(e.target.value)}
        />
      </div>
      <p className="ghi-chu">{CANH_BAO_HAN_MOT_LAN}</p>
      {maChaCoSan !== undefined && maChaCoSan !== "" && (
        <p className="ghi-chu">{GHI_CHU_HAN_VIEC_CON}</p>
      )}

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" onClick={huy} disabled={dangGui}>
          Huỷ
        </button>
        <button
          type="submit"
          className="nut-chinh"
          disabled={dangGui || tieuDe.trim() === "" || loaiChon === ""}
        >
          Giao việc
        </button>
      </div>
    </form>
  );
}
