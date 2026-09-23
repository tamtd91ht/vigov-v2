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
import { FormGiaoViec, type DanhMucNhiemVu } from "@/features/nhiem-vu/so-nhiem-vu";
import {
  laySoBienBan,
  tachKetLuanThanhNhiemVu,
  taoBienBan,
  themKetLuan,
  type TachKetLuanVao,
  type TaoBienBanVao,
} from "@/lib/api/bien-ban";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import {
  layKhoiNhiemVu,
  layLoaiNhiemVu,
  layMucUuTienNhiemVu,
} from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import type {
  page_Result_petitions_bienBanRa,
  petitions_bienBanRa,
  petitions_ketLuanRa,
} from "@/lib/api/schema.gen";

import {
  cauDaTach,
  CHUA_DIEN_SAN,
  DANG_TAI_SO,
  DIA_DIEM_TOI_DA,
  dongMeta,
  KET_LUAN_MOI_LAN_TOI_DA,
  NGUON_GIAO_KHOA,
  NHAN_NUT_HUY,
  NHAN_NUT_LUU,
  NHAN_NUT_NHAP_BIEN_BAN,
  NHAN_NUT_TACH,
  NHAN_NUT_THEM_KET_LUAN,
  nhanBadge,
  nhanNutTach,
  nhanTienDoKetLuan,
  NOI_DUNG_BIEN_BAN_TOI_DA,
  NOI_DUNG_KET_LUAN_TOI_DA,
  PHAN_CHUA_DUNG,
  PLACEHOLDER_KET_LUAN,
  SO_HIEU_TOI_DA,
  SO_RONG,
  soThuTuKetLuan,
  tachKetLuan,
  tachThanhPhan,
  TEN_CUOC_HOP_TOI_DA,
} from "./nhan-bien-ban";

/**
 * Sổ Biên bản và kết luận họp — `docs/ui-ux/04-bien-ban-hop.md` §2 (thẻ), §4 (biểu mẫu nhập),
 * §7 (quy tắc nghiệp vụ).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * HAI ĐIỀU MÀ MÁY CHỦ ĐÃ QUYẾT VÀ MÀN HÌNH NÀY PHẢI THEO — cả hai đều dễ bị "sửa cho gọn":
 *
 *   SỐ THỨ TỰ KẾT LUẬN     nối tiếp số ĐÃ CẤP, không đếm lại số dòng còn sống. Gỡ ② thì kế tiếp
 *                          là ④, và khoảng trống ấy ĐÚNG (luật 7, bất biến 3). Màn hình vẽ
 *                          `ordinal` máy chủ trả, không bao giờ vẽ `index + 1` — xem
 *                          `soThuTuKetLuan` và bài kiểm của nó.
 *   BIÊN BẢN KHÔNG CÓ MÃ   `so_hieu` gõ tay, tuỳ chọn, KHÔNG duy nhất (`31/BB-UBND` lặp lại ở
 *                          năm sau). Nó nằm giữa dòng meta như một thông tin, không được vẽ như
 *                          một mã định danh và không được đòi bắt buộc.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ CỔNG QUYỀN Ở CLIENT, và đó là một quyết định đã ghi — xem `PHAN_CHUA_DUNG`. Dịch vụ
 * `petitions` kiểm `task.read` / `task.create` trên TỪNG lời gọi; tài khoản thiếu khoá nhận
 * nguyên câu 403 của máy chủ ra màn hình. Ẩn một nút chưa bao giờ là biện pháp (luật 5, cấm #1).
 *
 * NỘI DUNG BIÊN BẢN LÀ CHỮ CỦA MỘT CUỘC HỌP CÓ THỂ NHẮC TỚI HỒ SƠ CÔNG DÂN: không dòng nào ở đây
 * ghi nó vào log, vào tên tệp hay vào một URL (luật 3, cấm #1 và #4).
 */

/** Bao nhiêu thẻ một trang. Mỗi thẻ mang cả danh sách kết luận, nên trang mỏng hơn quyển sổ phản ánh. */
const SO_THE_MOI_TRANG = 10;

/** Chưa đọc được danh mục nào thì các ô chọn của biểu mẫu Giao việc rỗng — xem `PhepTach`. */
const KHONG_DANH_MUC: DanhMucNhiemVu = { loai: [], mucUuTien: [], khoi: [], boPhan: [] };

/**
 * Mọi thứ luồng "Tách thành nhiệm vụ" (§3) cần, gom thành MỘT đối tượng đi qua ba tầng thành phần.
 *
 * GOM LẠI VÌ ĐƯỜNG ĐI DÀI: `SoBienBan` → `DanhSachBienBan` → `TheBienBan` → `DongKetLuan`. Bảy
 * prop rời nhau đi qua ba chặng là bảy chỗ để một chặng quên truyền một cái, và cái bị quên sẽ là
 * cái ít dùng nhất.
 *
 * MỘT HỘP GIAO VIỆC MỘT LÚC TRÊN CẢ MÀN (`moOKetLuan` là một id, không phải một tập). §3 gọi nó là
 * modal, và hai biểu mẫu mở cùng lúc là hai khoá chống trùng sống song song trên cùng một màn —
 * chưa kể chữ đã gõ ở hộp kia không ai nhìn thấy để mà lưu.
 */
export type PhepTach = {
  /** Bốn danh mục đổ vào ô chọn của biểu mẫu Giao việc. */
  readonly danhMuc: DanhMucNhiemVu;
  readonly dangGui: boolean;
  /** Id của kết luận đang mở hộp, hoặc `null`. */
  readonly moOKetLuan: string | null;
  readonly loi: { readonly ketLuanID: string; readonly thongBao: string } | null;
  readonly daXong: { readonly ketLuanID: string; readonly maNhiemVu: string } | null;
  readonly mo: (ketLuanID: string) => void;
  readonly dong: () => void;
  /**
   * ⚠ NHẬN CẢ DÒNG KẾT LUẬN, KHÔNG NHẬN MỘT CON SỐ. `{stt}` trên đường dẫn phải là `ordinal` máy
   * chủ trả; một tham số `thuTu: number` ở đây là chỗ `viTri + 1` đi lọt vào mà không có gì đỏ.
   * Xem `tachKetLuanThanhNhiemVu` trong `lib/api/bien-ban.ts`.
   */
  readonly gui: (
    bienBanID: string,
    ketLuan: petitions_ketLuanRa,
    than: TachKetLuanVao,
    khoaChongTrung: string,
  ) => void;
};

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

export function SoBienBan() {
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);
  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_petitions_bienBanRa>;
  } | null>(null);

  const [dangMoBieuMau, datDangMoBieuMau] = useState(false);
  const [dangGui, datDangGui] = useState(false);
  const [loiBieuMau, datLoiBieuMau] = useState<string | null>(null);
  const [loiKetLuan, datLoiKetLuan] = useState<{ bienBanID: string; thongBao: string } | null>(
    null,
  );

  // Đếm số lần GHI THÀNH CÔNG. Nó đi vào `key` của hàng thêm kết luận và của biểu mẫu nhập, nên
  // một lần ghi xong là một lần hai thành phần ấy dựng lại từ đầu: ô nhập trống trở lại VÀ một
  // khoá chống trùng mới được sinh. Lần ghi HỎNG thì không tăng — hàng giữ nguyên chữ đã gõ và
  // giữ nguyên khoá cũ, đúng điều `Idempotency-Key` sinh ra để làm.
  const [lanGhiXong, datLanGhiXong] = useState(0);

  // §3 — luồng Tách. Bốn mẩu trạng thái, tất cả mang ID KẾT LUẬN: một câu từ chối của kết luận này
  // hiện trên dòng kết luận khác là nói với cán bộ rằng họ vừa làm hỏng một việc họ không đụng tới.
  const [danhMuc, datDanhMuc] = useState<DanhMucNhiemVu>(KHONG_DANH_MUC);
  const [moTachO, datMoTachO] = useState<string | null>(null);
  const [loiTach, datLoiTach] = useState<{ ketLuanID: string; thongBao: string } | null>(null);
  const [daTach, datDaTach] = useState<{ ketLuanID: string; maNhiemVu: string } | null>(null);

  const khoa = `${nganXep.hienTai ?? ""}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    laySoBienBan({ limit: SO_THE_MOI_TRANG, cursor: nganXep.hienTai }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [nganXep.hienTai, khoa]);

  /**
   * BỐN DANH MỤC CỦA BIỂU MẪU GIAO VIỆC, đọc một lần cho cả màn. Một danh mục hỏng thì ô chọn
   * tương ứng rỗng — KHÔNG làm hỏng quyển sổ: bốn câu trả lời rời nhau.
   *
   * ĐỌC NGAY LÚC MỞ MÀN CHỨ KHÔNG ĐỢI TỚI LÚC BẤM TÁCH, dù đó là bốn lời gọi cho một hộp nhiều lần
   * xem không ai mở. Đợi tới lúc bấm thì `Loại nhiệm vụ` còn rỗng trong khoảnh khắc đầu, mà loại là
   * trường BẮT BUỘC — cán bộ mở hộp ra và thấy nút Giao việc mờ đi, không kèm lý do nào. Tách nhiệm
   * vụ là việc §1 nói màn này sinh ra để làm, không phải một tính năng bên lề.
   */
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

  const so = taiTu(daTai, khoa);

  function guiKetLuan(bienBanID: string, noiDung: string, khoaChongTrung: string): void {
    datDangGui(true);
    themKetLuan(bienBanID, { content: noiDung }, khoaChongTrung).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ. Nó mang đúng quy tắc đã từ chối ("thiếu nội dung kết luận — một
        // kết luận rỗng là một dòng không ai thi hành được"), và viết lại nó ở client là dựng bản
        // sao thứ hai của một quy tắc nghiệp vụ.
        datLoiKetLuan({ bienBanID, thongBao: kq.thongBao });
        return;
      }
      datLoiKetLuan(null);
      datLanGhiXong((n) => n + 1);
      // Đọc lại cả trang chứ không chèn kết luận vừa trả vào mảng đang giữ: bộ đếm nhiệm vụ ở
      // badge của thẻ do máy chủ cộng, và ghép tay một dòng vào là dựng con số thứ hai của cùng
      // một sự thật.
      datLanTai((n) => n + 1);
    });
  }

  /**
   * §3 — tách một kết luận thành một nhiệm vụ.
   *
   * KHÔNG TỰ SUY MỘT CON SỐ NÀO: `ketLuan` đi nguyên xuống lớp gọi, và `{stt}` trên đường dẫn đọc
   * từ `ordinal` của chính bản ghi ấy.
   *
   * GHI XONG THÌ ĐÓNG HỘP VÀ ĐỌC LẠI CẢ TRANG. Đóng hộp vì lần mở sau phải sinh một khoá chống
   * trùng mới — biểu mẫu giữ khoá theo đời của nó, nên đóng là huỷ khoá cũ. Đọc lại vì bộ đếm
   * `x/y` ở dòng kết luận và ở badge của thẻ do MÁY CHỦ cộng; cộng thêm một ở client là dựng con số
   * thứ hai của cùng một sự thật (luật 9, cấm #2).
   */
  function guiTach(
    bienBanID: string,
    ketLuan: petitions_ketLuanRa,
    than: TachKetLuanVao,
    khoaChongTrung: string,
  ): void {
    datDangGui(true);
    tachKetLuanThanhNhiemVu(bienBanID, ketLuan, than, khoaChongTrung).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ. Ở tuyến này nó mang những thứ không dựng lại được ở client: mã
        // việc cha còn dở, tên hai trạng thái của một bước không có trong §6, hay câu 404 nói kết
        // luận này không có trong biên bản.
        datLoiTach({ ketLuanID: ketLuan.id, thongBao: kq.thongBao });
        return;
      }
      datLoiTach(null);
      datDaTach({ ketLuanID: ketLuan.id, maNhiemVu: kq.duLieu.code });
      datMoTachO(null);
      datLanGhiXong((n) => n + 1);
      datLanTai((n) => n + 1);
    });
  }

  const phepTach: PhepTach = {
    danhMuc,
    dangGui,
    moOKetLuan: moTachO,
    loi: loiTach,
    daXong: daTach,
    mo: (ketLuanID) => {
      // Mở hộp khác là bỏ câu lỗi và câu báo xong của hộp trước: chúng nói về một kết luận khác.
      datMoTachO((dang) => (dang === ketLuanID ? null : ketLuanID));
      datLoiTach(null);
      datDaTach(null);
    },
    dong: () => {
      datMoTachO(null);
      datLoiTach(null);
    },
    gui: guiTach,
  };

  function luuBienBan(than: TaoBienBanVao, khoaChongTrung: string): void {
    datDangGui(true);
    taoBienBan(than, khoaChongTrung).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        datLoiBieuMau(kq.thongBao);
        return;
      }
      datLoiBieuMau(null);
      datDangMoBieuMau(false);
      datLanGhiXong((n) => n + 1);
      // Về TRANG ĐẦU: thẻ mới nhất nằm trên cùng, và con trỏ của trang đang xem không còn nghĩa
      // sau khi một bản ghi chen vào đầu danh sách.
      datNganXep(TRANG_DAU);
      datLanTai((n) => n + 1);
    });
  }

  return (
    <section className="man-bien-ban" aria-labelledby="tieu-de-so-bien-ban">
      <h2 id="tieu-de-so-bien-ban">Danh sách biên bản</h2>

      <KhoiChuaDung />

      <div className="cum-nut">
        <button
          type="button"
          className="nut-chinh"
          aria-expanded={dangMoBieuMau}
          onClick={() => {
            datDangMoBieuMau(!dangMoBieuMau);
            datLoiBieuMau(null);
          }}
        >
          {NHAN_NUT_NHAP_BIEN_BAN}
        </button>
      </div>

      {dangMoBieuMau && (
        <FormNhapBienBan
          // Khoá dựng lại: mỗi lần GHI XONG là một biểu mẫu mới, một khoá chống trùng mới.
          key={`bieu-mau|${lanGhiXong}`}
          dangGui={dangGui}
          loi={loiBieuMau}
          huy={() => {
            datDangMoBieuMau(false);
            datLoiBieuMau(null);
          }}
          luu={luuBienBan}
        />
      )}

      {so.pha === "dangTai" && <p role="status">{DANG_TAI_SO}</p>}
      {so.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {so.thongBao}
        </p>
      )}

      {so.pha === "xong" && (
        <>
          <DanhSachBienBan
            bienBan={so.duLieu.items}
            lanGhiXong={lanGhiXong}
            dangGui={dangGui}
            loiKetLuan={loiKetLuan}
            guiKetLuan={guiKetLuan}
            tach={phepTach}
          />
          <nav className="dieu-huong-trang" aria-label="Phân trang danh sách biên bản">
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
    </section>
  );
}

/**
 * Những phần đặc tả đòi mà hợp đồng hoặc lượt làm này không có — HIỆN LÊN ĐẦU MÀN, không giấu
 * trong chú thích mã.
 *
 * `<details>` chứ không phải một khối luôn mở: một bức tường chữ trên đầu màn hình là bức tường
 * người ta học cách không đọc.
 */
export function KhoiChuaDung() {
  return (
    <details className="khoi-chua-khai">
      <summary>
        {PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và lý
        do
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

/** Danh sách THẺ §2 — xếp dọc, mới nhất ở trên (thứ tự do máy chủ quyết, xem `PHAN_CHUA_DUNG`). */
export function DanhSachBienBan({
  bienBan,
  lanGhiXong,
  dangGui,
  loiKetLuan,
  guiKetLuan,
  tach,
}: {
  bienBan: readonly petitions_bienBanRa[];
  lanGhiXong: number;
  dangGui: boolean;
  loiKetLuan: { bienBanID: string; thongBao: string } | null;
  guiKetLuan: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
  tach: PhepTach;
}) {
  if (bienBan.length === 0) return <p className="trang-thai-rong">{SO_RONG}</p>;

  return (
    // KHÔNG GẮN LỚP CSS MỚI cho danh sách thẻ: `globals.css` chưa có lớp cho nó và lượt này không
    // được thêm CSS. Tên lớp cần thêm đã báo về — xem `PHAN_CHUA_DUNG`.
    <ul aria-label="Danh sách biên bản họp">
      {bienBan.map((bb) => (
        <li key={bb.id}>
          <TheBienBan
            bienBan={bb}
            lanGhiXong={lanGhiXong}
            dangGui={dangGui}
            loiKetLuan={loiKetLuan?.bienBanID === bb.id ? loiKetLuan.thongBao : null}
            guiKetLuan={guiKetLuan}
            tach={tach}
          />
        </li>
      ))}
    </ul>
  );
}

/** Một thẻ biên bản §2: header · các dòng kết luận · hàng thêm kết luận. */
export function TheBienBan({
  bienBan,
  lanGhiXong,
  dangGui,
  loiKetLuan,
  guiKetLuan,
  tach,
}: {
  bienBan: petitions_bienBanRa;
  lanGhiXong: number;
  dangGui: boolean;
  loiKetLuan: string | null;
  guiKetLuan: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
  tach: PhepTach;
}) {
  return (
    <div className="khoi-chi-tiet">
      <div className="dau-khoi-chi-tiet">
        {/* Icon tài liệu §2. `aria-hidden` vì nó không mang thông tin nào mà dòng chữ bên cạnh
            không nói rõ hơn. */}
        <span aria-hidden="true">📋</span>
        <h3>{bienBan.title}</h3>
        <span className="chip chip-ngung">{nhanBadge(bienBan)}</span>
      </div>

      <p className="dong-phu">{dongMeta(bienBan)}</p>

      {bienBan.conclusions.length === 0 ? (
        // §7.3: biên bản không có kết luận nào VẪN LƯU ĐƯỢC (nhập nháp trước, bổ sung sau). Nói
        // ra trạng thái ấy thay vì để một khoảng trống trông như đang tải.
        <p className="trang-thai-rong">Biên bản này chưa ghi kết luận nào.</p>
      ) : (
        <ol aria-label={`Các kết luận của biên bản ${bienBan.title}`}>
          {/* ⚠ KHÔNG LẤY CHỈ SỐ CỦA `map` RA DÙNG, ở đây hay ở bất kỳ đâu dưới nó. Số hiện trong ô
              tròn và số đi trên đường dẫn `{stt}` của tuyến Tách đều là `ordinal` MÁY CHỦ TRẢ —
              xem `DongKetLuan` và `soThuTuKetLuan`. */}
          {bienBan.conclusions.map((kl) => (
            <li key={kl.id}>
              <DongKetLuan bienBanID={bienBan.id} ketLuan={kl} tach={tach} />
            </li>
          ))}
        </ol>
      )}

      {loiKetLuan !== null && (
        <p className="thong-bao-loi" role="alert">
          {loiKetLuan}
        </p>
      )}

      <HangThemKetLuan
        // Khoá dựng lại: ghi xong một kết luận là ô nhập trống trở lại VÀ một khoá chống trùng
        // mới. Ghi hỏng thì `lanGhiXong` không đổi, hàng giữ nguyên chữ và giữ nguyên khoá.
        key={`${bienBan.id}|${lanGhiXong}`}
        bienBanID={bienBan.id}
        dangGui={dangGui}
        gui={guiKetLuan}
      />
    </div>
  );
}

/**
 * Một dòng kết luận §2: số thứ tự trong ô tròn · nội dung · dòng phụ đếm nhiệm vụ · nút
 * `✂ Tách thành nhiệm vụ` (§3).
 *
 * SỐ THỨ TỰ LẤY TỪ `ordinal`, KHÔNG TỪ VỊ TRÍ TRONG MẢNG — xem `soThuTuKetLuan`. Ở dòng này con số
 * ấy đi HAI đường: vào ô tròn, và vào `{stt}` của tuyến Tách. Thành phần này cố ý KHÔNG nhận chỉ
 * số của mảng, nên không có sẵn một con số sai nào để lỡ tay dùng.
 *
 * BIỂU MẪU MỞ NGAY DƯỚI DÒNG, KHÔNG PHẢI MỘT LỚP PHỦ. §3 gọi nó là modal; lớp CSS cho một lớp phủ
 * chưa có trong `globals.css` và lượt này không thêm CSS — cùng lý do biểu mẫu §4 nằm nối tiếp
 * trong trang. Xem `PHAN_CHUA_DUNG`.
 */
export function DongKetLuan({
  bienBanID,
  ketLuan,
  tach,
}: {
  bienBanID: string;
  ketLuan: petitions_ketLuanRa;
  tach: PhepTach;
}) {
  const dangMo = tach.moOKetLuan === ketLuan.id;
  const loi = tach.loi?.ketLuanID === ketLuan.id ? tach.loi.thongBao : null;
  const daXong = tach.daXong?.ketLuanID === ketLuan.id ? tach.daXong.maNhiemVu : null;

  return (
    <div>
      {/* Ô tròn xanh nhạt của §2 cần một lớp CSS chưa có; `chip` là lớp sẵn có gần nhất và nó
          không mượn tên của thứ khác. */}
      <span className="chip">{soThuTuKetLuan(ketLuan)}</span>{" "}
      <span>{ketLuan.content}</span>
      <p className="dong-phu">{nhanTienDoKetLuan(ketLuan)}</p>

      {/* KHÔNG CÓ CỔNG QUYỀN Ở ĐÂY: nút hiện với mọi tài khoản, và tài khoản thiếu `task.create`
          nhận nguyên câu 403 của máy chủ ngay dưới biểu mẫu. Ẩn một nút chưa bao giờ là biện pháp
          (luật 5, cấm #1) — xem `PHAN_CHUA_DUNG`. */}
      <button
        type="button"
        className="nut-phu"
        // Tên đọc được mang số thứ tự: một thẻ có nhiều dòng kết luận, và không có nó thì các nút
        // liền nhau mang cùng một tên.
        aria-label={nhanNutTach(ketLuan)}
        aria-expanded={dangMo}
        // Khoá lúc đang gửi, kể cả lần gửi của một hộp khác: bấm đóng giữa chừng là huỷ khoá chống
        // trùng của lời gọi đang bay.
        disabled={tach.dangGui}
        onClick={() => tach.mo(ketLuan.id)}
      >
        {NHAN_NUT_TACH}
      </button>

      {daXong !== null && (
        // SỐ SỔ MÁY CHỦ VỪA CẤP. Đó là thứ cán bộ không thể biết trước và là thứ họ cần để đi tìm
        // nhiệm vụ vừa lập ở màn `/nhiem-vu`.
        <p className="ghi-chu" role="status">
          {cauDaTach(daXong)}
        </p>
      )}

      {dangMo && (
        // KHÔNG BỌC THÊM MỘT `khoi-chi-tiet` NỮA. `form-danh-muc` đã tự có viền và vạch xanh bên
        // trái, còn ở 320px thì mỗi lớp hộp lồng nhau ăn thêm 2rem bề ngang: thẻ biên bản 1rem +
        // hộp này 1rem + biểu mẫu 1rem chỉ còn lại 224px cho chữ.
        <div>
          {/* KẾT LUẬN GỐC ĐỨNG NGAY TRÊN BIỂU MẪU, và đó không phải trang trí: ô "Nội dung nhiệm
              vụ" chưa điền sẵn được, nên đây là chỗ cán bộ chép từ. */}
          <p>
            <span className="chip">{soThuTuKetLuan(ketLuan)}</span> {ketLuan.content}
          </p>
          <p className="ghi-chu">{NGUON_GIAO_KHOA}</p>
          <p className="ghi-chu">{CHUA_DIEN_SAN}</p>

          {/* DÙNG LẠI NGUYÊN BIỂU MẪU CỦA `02-nhiem-vu.md` §7 — §3 nói rõ là dùng lại, và một bản
              thứ hai ở đây là hai biểu mẫu cùng gửi một tuyến rồi trôi khỏi nhau (luật 9, cấm #2).
              Khoá chống trùng do chính biểu mẫu giữ, sinh lúc MỞ: bấm lại sau một lỗi mạng dùng
              lại đúng khoá ấy, còn đóng hộp rồi mở lại là một khoá mới. */}
          <FormGiaoViec
            danhMuc={tach.danhMuc}
            dangGui={tach.dangGui}
            loi={loi}
            huy={tach.dong}
            giaoViec={(than, khoaChongTrung) =>
              tach.gui(bienBanID, ketLuan, than, khoaChongTrung)
            }
          />
        </div>
      )}
    </div>
  );
}

/** Hàng thêm kết luận §2, luôn ở cuối mỗi thẻ. `POST /api/v1/meetings/{id}/conclusions`. */
export function HangThemKetLuan({
  bienBanID,
  dangGui,
  gui,
}: {
  bienBanID: string;
  dangGui: boolean;
  gui: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
}) {
  const [noiDung, datNoiDung] = useState("");
  // Sinh ở chỗ MỞ hàng, không ở chỗ gửi: bấm lại sau một lỗi mạng phải dùng LẠI đúng khoá ấy, vì
  // lần gửi đầu có thể đã tới máy chủ và đã ghi một kết luận vào sổ lưu trữ.
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  const canGon = noiDung.trim();

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (canGon === "") return;
    gui(bienBanID, canGon, khoaChongTrung);
  }

  return (
    <form className="form-danh-muc" onSubmit={guiNgay}>
      <div className="o-nhap">
        <label htmlFor={`them-ket-luan-${bienBanID}`}>Thêm một kết luận</label>
        <textarea
          id={`them-ket-luan-${bienBanID}`}
          name="noi-dung-ket-luan"
          rows={1}
          value={noiDung}
          placeholder={PLACEHOLDER_KET_LUAN}
          maxLength={NOI_DUNG_KET_LUAN_TOI_DA}
          onChange={(e) => datNoiDung(e.target.value)}
        />
      </div>
      <button type="submit" className="nut-phu" disabled={dangGui || canGon === ""}>
        {NHAN_NUT_THEM_KET_LUAN}
      </button>
    </form>
  );
}

/**
 * Biểu mẫu "Nhập biên bản" §4.
 *
 * ĐẶC TẢ GỌI NÓ LÀ MODAL. Ở đây nó là một khối nằm trong trang — KHÔNG phải một lớp phủ — vì một
 * lớp phủ cần lớp CSS chưa có trong `globals.css`, và lượt này không được thêm CSS. Đã báo về tên
 * lớp cần thêm.
 *
 * BA TRƯỜNG CỦA §4 KHÔNG CÓ Ở ĐÂY — Chủ trì, Tệp đính kèm, và danh sách kết luận dạng từng ô
 * riêng. Lý do từng cái một nằm ở `PHAN_CHUA_DUNG`, hiện ngay đầu màn.
 */
export function FormNhapBienBan({
  dangGui,
  loi,
  huy,
  luu,
}: {
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  luu: (than: TaoBienBanVao, khoaChongTrung: string) => void;
}) {
  const [ten, datTen] = useState("");
  const [ngay, datNgay] = useState("");
  const [soHieu, datSoHieu] = useState("");
  const [diaDiem, datDiaDiem] = useState("");
  const [noiDung, datNoiDung] = useState("");
  const [thanhPhan, datThanhPhan] = useState("");
  const [ketLuan, datKetLuan] = useState("");
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  const tenGon = ten.trim();
  // HAI TRƯỜNG BẮT BUỘC, ĐÚNG HAI. §4 đánh dấu "Tên cuộc họp" và "Ngày họp"; mọi trường còn lại
  // tuỳ chọn, kể cả số hiệu — biên bản không có mã nghiệp vụ nào cả.
  const duDieuKien = tenGon !== "" && ngay !== "";

  function luuNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien) return;

    const dsThanhPhan = tachThanhPhan(thanhPhan);
    const dsKetLuan = tachKetLuan(ketLuan);

    // Trường rỗng thì VẮNG MẶT khỏi thân, không gửi chuỗi rỗng: `omitempty` ở máy chủ nói rằng
    // "không gửi" là trạng thái bình thường của các trường này.
    luu(
      {
        title: tenGon,
        held_on: ngay,
        reference_no: soHieu.trim() === "" ? undefined : soHieu.trim(),
        location: diaDiem.trim() === "" ? undefined : diaDiem.trim(),
        // Chủ trì luôn rỗng ở lượt này — xem `PHAN_CHUA_DUNG`. Khoá vẫn có mặt vì hợp đồng khai
        // nó là `string` thường, và rỗng là trạng thái máy chủ chấp nhận.
        chaired_by: "",
        content: noiDung.trim() === "" ? undefined : noiDung,
        attendees: dsThanhPhan.length === 0 ? undefined : dsThanhPhan,
        conclusions: dsKetLuan.length === 0 ? undefined : dsKetLuan,
      },
      khoaChongTrung,
    );
  }

  return (
    <form className="form-danh-muc" onSubmit={luuNgay} aria-labelledby="tieu-de-nhap-bien-ban">
      <h3 id="tieu-de-nhap-bien-ban">Nhập biên bản</h3>

      <div className="o-nhap">
        <label htmlFor="ten-cuoc-hop">Tên cuộc họp *</label>
        <input
          id="ten-cuoc-hop"
          name="ten-cuoc-hop"
          value={ten}
          maxLength={TEN_CUOC_HOP_TOI_DA}
          autoComplete="off"
          onChange={(e) => datTen(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="ngay-hop">Ngày họp *</label>
        {/* `type="date"` trả đúng `2026-08-05` — ngày lịch, không múi giờ, đúng thứ cột DATE của
            máy chủ nhận. */}
        <input
          id="ngay-hop"
          name="ngay-hop"
          type="date"
          value={ngay}
          onChange={(e) => datNgay(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="so-hieu-bien-ban">Số hiệu biên bản</label>
        <input
          id="so-hieu-bien-ban"
          name="so-hieu-bien-ban"
          value={soHieu}
          maxLength={SO_HIEU_TOI_DA}
          autoComplete="off"
          onChange={(e) => datSoHieu(e.target.value)}
        />
        {/* Nói thẳng rằng đây KHÔNG phải một mã: số hiệu biên bản Việt Nam lặp lại theo năm, và
            một cán bộ tưởng nó là mã tra cứu sẽ đi tìm một biên bản bằng nó. */}
        <p className="ghi-chu">
          Ví dụ 31/BB-UBND. Không bắt buộc, và không phải mã tra cứu — số hiệu lặp lại giữa các
          năm.
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="dia-diem-hop">Địa điểm</label>
        <input
          id="dia-diem-hop"
          name="dia-diem-hop"
          value={diaDiem}
          maxLength={DIA_DIEM_TOI_DA}
          autoComplete="off"
          onChange={(e) => datDiaDiem(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="thanh-phan-tham-du">Thành phần tham dự</label>
        <textarea
          id="thanh-phan-tham-du"
          name="thanh-phan-tham-du"
          rows={3}
          value={thanhPhan}
          // KHÔNG CÓ `maxLength` Ở ĐÂY. Trần của máy chủ là hai trần khác nhau — 200 dòng và 200
          // ký tự MỖI DÒNG — nên một con số duy nhất trên cả ô sẽ cắt sai ở cả hai chiều: chặn
          // một danh sách 30 người hợp lệ, hoặc cho qua một dòng 400 ký tự. Máy chủ từ chối bằng
          // câu của nó, và câu ấy nói đúng dòng nào sai.
          onChange={(e) => datThanhPhan(e.target.value)}
        />
        <p className="ghi-chu">
          Mỗi dòng một người. Ô chọn cán bộ chưa dựng — xem phần chưa dựng được ở đầu màn.
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="noi-dung-bien-ban">Nội dung biên bản</label>
        <textarea
          id="noi-dung-bien-ban"
          name="noi-dung-bien-ban"
          rows={6}
          value={noiDung}
          maxLength={NOI_DUNG_BIEN_BAN_TOI_DA}
          onChange={(e) => datNoiDung(e.target.value)}
        />
        <p className="ghi-chu">
          Toàn văn. Lưu xong thì màn này chưa đọc lại được — xem phần chưa dựng được ở đầu màn.
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="cac-ket-luan">Các kết luận</label>
        <textarea
          id="cac-ket-luan"
          name="cac-ket-luan"
          rows={4}
          value={ketLuan}
          onChange={(e) => datKetLuan(e.target.value)}
        />
        <p className="ghi-chu">
          Mỗi dòng một kết luận, đánh số ① ② ③ theo thứ tự nhập. Tối đa{" "}
          {KET_LUAN_MOI_LAN_TOI_DA} kết luận một lần nhập; thêm tiếp bằng nút “
          {NHAN_NUT_THEM_KET_LUAN}” ở cuối thẻ. Để trống cũng lưu được.
        </p>
      </div>

      {loi !== null && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <div className="cum-nut">
        <button type="button" className="nut-phu" disabled={dangGui} onClick={huy}>
          {NHAN_NUT_HUY}
        </button>
        <button type="submit" className="nut-chinh" disabled={dangGui || !duDieuKien}>
          {NHAN_NUT_LUU}
        </button>
      </div>
    </form>
  );
}
