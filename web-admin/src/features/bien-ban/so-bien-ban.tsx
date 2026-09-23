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
import {
  laySoBienBan,
  taoBienBan,
  themKetLuan,
  type TaoBienBanVao,
} from "@/lib/api/bien-ban";
import type { KetQua } from "@/lib/api/goi";
import type {
  page_Result_petitions_bienBanRa,
  petitions_bienBanRa,
  petitions_ketLuanRa,
} from "@/lib/api/schema.gen";

import {
  CHO_NUT_TACH,
  DANG_TAI_SO,
  DIA_DIEM_TOI_DA,
  dongMeta,
  KET_LUAN_MOI_LAN_TOI_DA,
  NHAN_NUT_HUY,
  NHAN_NUT_LUU,
  NHAN_NUT_NHAP_BIEN_BAN,
  NHAN_NUT_THEM_KET_LUAN,
  nhanBadge,
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
}: {
  bienBan: readonly petitions_bienBanRa[];
  lanGhiXong: number;
  dangGui: boolean;
  loiKetLuan: { bienBanID: string; thongBao: string } | null;
  guiKetLuan: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
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
}: {
  bienBan: petitions_bienBanRa;
  lanGhiXong: number;
  dangGui: boolean;
  loiKetLuan: string | null;
  guiKetLuan: (bienBanID: string, noiDung: string, khoaChongTrung: string) => void;
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
          {bienBan.conclusions.map((kl) => (
            <li key={kl.id}>
              <DongKetLuan ketLuan={kl} />
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
 * Một dòng kết luận §2: số thứ tự trong ô tròn · nội dung · dòng phụ đếm nhiệm vụ · chỗ của nút
 * `✂ Tách thành nhiệm vụ`.
 *
 * SỐ THỨ TỰ LẤY TỪ `ordinal`, KHÔNG TỪ VỊ TRÍ TRONG MẢNG — xem `soThuTuKetLuan`.
 */
export function DongKetLuan({ ketLuan }: { ketLuan: petitions_ketLuanRa }) {
  return (
    <div>
      {/* Ô tròn xanh nhạt của §2 cần một lớp CSS chưa có; `chip` là lớp sẵn có gần nhất và nó
          không mượn tên của thứ khác. */}
      <span className="chip">{soThuTuKetLuan(ketLuan)}</span>{" "}
      <span>{ketLuan.content}</span>
      <p className="dong-phu">{nhanTienDoKetLuan(ketLuan)}</p>
      {/* CHỖ CỦA NÚT TÁCH — một ô trống có nhãn nói rõ vì sao chưa có, không phải một nút mờ. */}
      <p className="ghi-chu">{CHO_NUT_TACH}</p>
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
