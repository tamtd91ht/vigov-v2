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
import type { KetQua } from "@/lib/api/goi";
import type { comms_thongBaoRa, page_Result_comms_thongBaoRa } from "@/lib/api/schema.gen";
import { laySoThongBao, phatHanhThongBao, type PhatHanhThongBaoVao } from "@/lib/api/thong-bao";

import {
  CANH_BAO_CHUA_GUI_THU,
  CANH_BAO_GHIM_TRONG_TRANG,
  CHIP_BAT_BUOC_XAC_NHAN,
  CHO_DANH_SACH_NGUOI_NHAN,
  CHO_NUT_GO,
  CHUA_CHON_THONG_BAO,
  coChipTrangThai,
  DANG_TAI_SO,
  DAU_GACH,
  DAU_GHIM,
  GHI_CHU_GHIM_TRONG_TRANG,
  GHI_CHU_NGUOI_NHAN,
  MA_CAN_BO_TOI_DA,
  MO_TA_SOAN,
  mocThe,
  nangGhimLenDau,
  NGUOI_NHAN_TOI_DA,
  NHAN_NUT_HUY,
  NHAN_NUT_PHAT_HANH,
  NHAN_NUT_SOAN,
  nhanBoDemXacNhan,
  nhanMoc,
  nhanTrangThai,
  nhanTrangThaiThu,
  NOI_DUNG_TOI_DA,
  PHAM_VI_DANG_HIEN,
  PHAN_CHUA_DUNG,
  PLACEHOLDER_TIEU_DE,
  SO_RONG,
  tachMaNguoiNhan,
  TIEU_DE_TOI_DA,
  trichNoiDung,
} from "./nhan-thong-bao";

/**
 * Sổ Thông báo nội bộ — `docs/ui-ux/08-thong-bao.md` §2 (bố cục), §3 (thẻ), §4 (chi tiết),
 * §5 (biểu mẫu soạn).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ĐIỀU QUAN TRỌNG NHẤT CỦA MÀN NÀY: **KHÔNG CÓ Ô CHỌN BỘ PHẬN**, và sự vắng mặt ấy là một câu
 * trả lời chứ không phải một phần còn thiếu.
 *
 * `POST /api/v1/announcements` trả **501 `not_implemented`** cho mọi thân mang `org_unit_ids`.
 * Nở một bộ phận thành danh sách cán bộ là dữ liệu của `identity` và chưa có RPC nào làm việc ấy.
 * Vẽ năm con chip bộ phận như §5 mô tả sẽ là vẽ đúng năm nút mà mọi lần bấm đều hỏng — và hỏng
 * sau khi cán bộ đã gõ xong cả nội dung. Lý do đầy đủ nằm ở `PHAN_CHUA_DUNG`, hiện ngay đầu màn.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * KHÔNG CÓ CỔNG QUYỀN Ở CLIENT, và đó là một quyết định đã ghi — xem `PHAN_CHUA_DUNG`.
 * `service-comms` kiểm `announcement.create` trên TỪNG lời gọi; tài khoản thiếu khoá nhận nguyên
 * câu 403 của máy chủ ra màn hình. Ẩn một nút chưa bao giờ là biện pháp (luật 5, cấm #1).
 *
 * NỘI DUNG THÔNG BÁO LÀ CHỮ MỘT CÁN BỘ VỪA GÕ VÀ CÓ THỂ NHẮC TỚI HỒ SƠ CÔNG DÂN: không dòng nào ở
 * đây ghi nó vào log, vào tên tệp hay vào một URL (luật 3, cấm #1 và #4). Mã cán bộ người nhận
 * cũng không — nó là định danh của một con người.
 */

/** Bao nhiêu thẻ một trang. Mỗi thẻ mang cả toàn văn nội dung, nên trang mỏng. */
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

export function SoThongBao() {
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanTai, datLanTai] = useState(0);
  const [daTai, datDaTai] = useState<{
    khoa: string;
    kq: KetQua<page_Result_comms_thongBaoRa>;
  } | null>(null);

  const [dangMoBieuMau, datDangMoBieuMau] = useState(false);
  const [dangGui, datDangGui] = useState(false);
  const [loiBieuMau, datLoiBieuMau] = useState<string | null>(null);

  // Id của thẻ đang chọn, KHÔNG phải cả bản ghi. Giữ bản ghi ở đây là giữ một bản sao thứ hai của
  // một hàng vừa tải: sau một lần phát hành, bản sao ấy là bản cũ, và cột phải sẽ hiện một con số
  // xác nhận không còn đúng trong khi danh sách bên trái đã mới.
  const [dangChon, datDangChon] = useState<string | null>(null);

  // Đếm số lần GHI THÀNH CÔNG. Nó đi vào `key` của biểu mẫu, nên một lần ghi xong là một lần biểu
  // mẫu dựng lại từ đầu: các ô trống trở lại VÀ một khoá chống trùng mới được sinh. Lần ghi HỎNG
  // thì không tăng — biểu mẫu giữ nguyên chữ đã gõ và giữ nguyên khoá cũ, đúng điều
  // `Idempotency-Key` sinh ra để làm.
  const [lanGhiXong, datLanGhiXong] = useState(0);

  const khoa = `${nganXep.hienTai ?? ""}|${lanTai}`;

  useEffect(() => {
    let bo = false;
    laySoThongBao({ limit: SO_THE_MOI_TRANG, cursor: nganXep.hienTai }).then((kq) => {
      if (!bo) datDaTai({ khoa, kq });
    });
    return () => {
      bo = true;
    };
  }, [nganXep.hienTai, khoa]);

  const so = taiTu(daTai, khoa);

  function phatHanh(than: PhatHanhThongBaoVao, khoaChongTrung: string): void {
    datDangGui(true);
    phatHanhThongBao(than, khoaChongTrung).then((kq) => {
      datDangGui(false);
      if (!kq.ok) {
        // NGUYÊN VĂN câu máy chủ, kể cả câu 501. Câu ấy CHỈ ĐƯỜNG — nó nói đúng mục biểu mẫu còn
        // dùng được — và viết lại nó ở client là dựng bản sao thứ hai của một quy tắc nghiệp vụ.
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

  // Thẻ đang chọn được TÌM LẠI trong dữ liệu vừa tải, không giữ riêng. Sang trang khác hoặc sau
  // một lần phát hành, thẻ ấy có thể không còn trong trang — khi đó cột phải quay về câu "Chọn một
  // thông báo để xem." thay vì hiện một bản ghi không còn nằm trong danh sách bên trái.
  const dsTrongTrang = so.pha === "xong" ? nangGhimLenDau(so.duLieu.items) : [];
  const theDangChon = dsTrongTrang.find((t) => t.id === dangChon) ?? null;

  return (
    <section className="man-thong-bao" aria-labelledby="tieu-de-so-thong-bao">
      <h2 id="tieu-de-so-thong-bao">Danh sách thông báo</h2>

      <KhoiChuaDung />

      <p className="ghi-chu">{PHAM_VI_DANG_HIEN}</p>

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
          {NHAN_NUT_SOAN}
        </button>
      </div>

      {dangMoBieuMau && (
        <FormSoanThongBao
          // Khoá dựng lại: mỗi lần GHI XONG là một biểu mẫu mới, một khoá chống trùng mới.
          key={`bieu-mau|${lanGhiXong}`}
          dangGui={dangGui}
          loi={loiBieuMau}
          huy={() => {
            datDangMoBieuMau(false);
            datLoiBieuMau(null);
          }}
          phatHanh={phatHanh}
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
          <DanhSachThongBao
            thongBao={dsTrongTrang}
            dangChon={dangChon}
            chon={datDangChon}
          />
          <ChiTietThongBao thongBao={theDangChon} />
          <nav className="dieu-huong-trang" aria-label="Phân trang danh sách thông báo">
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

/**
 * Cột trái §2 — danh sách thẻ, mới nhất ở trên, thẻ ghim được nâng lên đầu TRANG.
 *
 * KHÔNG GẮN LỚP CSS MỚI: `globals.css` chưa có lớp cho lưới hai cột hay cho viền thẻ đang chọn, và
 * lượt này không được thêm CSS. Tên lớp cần thêm đã báo về — xem `PHAN_CHUA_DUNG`. Thẻ đang chọn
 * vì thế được đánh dấu bằng `aria-current`, thứ đọc được bằng trình đọc màn hình và không cần một
 * lớp nào.
 */
export function DanhSachThongBao({
  thongBao,
  dangChon,
  chon,
}: {
  thongBao: readonly comms_thongBaoRa[];
  dangChon: string | null;
  chon: (id: string) => void;
}) {
  if (thongBao.length === 0) return <p className="trang-thai-rong">{SO_RONG}</p>;

  return (
    <>
      <p className="ghi-chu">{GHI_CHU_GHIM_TRONG_TRANG}</p>
      <ul aria-label="Danh sách thông báo nội bộ">
        {thongBao.map((tb) => (
          <li key={tb.id}>
            <TheThongBao thongBao={tb} dangChon={tb.id === dangChon} chon={chon} />
          </li>
        ))}
      </ul>
    </>
  );
}

/** Một thẻ §3: tiêu đề · chip · trích nội dung · mốc · bộ đếm · chip thư. */
export function TheThongBao({
  thongBao,
  dangChon,
  chon,
}: {
  thongBao: comms_thongBaoRa;
  dangChon: boolean;
  chon: (id: string) => void;
}) {
  const boDem = nhanBoDemXacNhan(thongBao);
  const chipThu = nhanTrangThaiThu(thongBao.email_status);

  return (
    <div className="khoi-chi-tiet" aria-current={dangChon ? "true" : undefined}>
      <div className="dau-khoi-chi-tiet">
        {/* Ký hiệu ghim §3. `aria-hidden` vì chữ "Ghim" đứng ngay cạnh trong một chip nói rõ hơn. */}
        {thongBao.pinned && <span aria-hidden="true">{DAU_GHIM}</span>}
        <h3>{thongBao.title}</h3>
        {thongBao.ack_required && <span className="chip">{CHIP_BAT_BUOC_XAC_NHAN}</span>}
        {coChipTrangThai(thongBao.status) && (
          <span className="chip chip-ngung">{nhanTrangThai(thongBao.status)}</span>
        )}
      </div>

      <p>{trichNoiDung(thongBao.body)}</p>

      <p className="dong-phu">
        {mocThe(thongBao)}
        {boDem !== null && <> · {boDem}</>}
        {chipThu !== null && <> · {chipThu}</>}
      </p>

      <button type="button" className="nut-phu" onClick={() => chon(thongBao.id)}>
        Xem chi tiết
      </button>
    </div>
  );
}

/**
 * Cột phải §4 — toàn văn nội dung của thẻ đang chọn.
 *
 * TOÀN VĂN LẤY TỪ CHÍNH PHẢN HỒI DANH SÁCH, không từ một tuyến chi tiết: `body` đi kèm mỗi thẻ vì
 * không có tuyến chi tiết nào, và máy chủ cố ý KHÔNG cắt bớt nó ở đó (nếu cắt, cột này sẽ hiện một
 * thông báo cụt mà không dòng nào nói ra).
 *
 * HAI KHỐI CỦA §4 KHÔNG CÓ Ở ĐÂY — `BỘ PHẬN NHẬN` và `NGƯỜI NHẬN (12)`. Chỗ của chúng là một dòng
 * chữ nói vì sao, không phải một danh sách rỗng: một danh sách rỗng nói với cán bộ rằng thông báo
 * này không có người nhận nào, trong khi sự thật là màn hình không đọc được danh sách ấy.
 */
export function ChiTietThongBao({ thongBao }: { thongBao: comms_thongBaoRa | null }) {
  if (thongBao === null) {
    return <p className="trang-thai-rong">{CHUA_CHON_THONG_BAO}</p>;
  }

  const boDem = nhanBoDemXacNhan(thongBao);

  return (
    <div className="khoi-chi-tiet" aria-labelledby="tieu-de-chi-tiet-thong-bao">
      <h3 id="tieu-de-chi-tiet-thong-bao">{thongBao.title}</h3>

      {/* `white-space: pre-line` không có lớp nào trong `globals.css`, nên đoạn văn nhiều dòng
          hiện liền mạch. Toàn văn vẫn ra đủ chữ — không mất câu nào. */}
      <p>{thongBao.body}</p>

      <dl className="danh-sach-truong">
        <div>
          <dt>Phát hành lúc</dt>
          <dd>{nhanMoc(thongBao.issued_at)}</dd>
        </div>
        <div>
          <dt>Người soạn</dt>
          {/* MÃ NGHIỆP VỤ (`CB-2026-7K3M9Q`), không phải họ tên và không phải id nội bộ (luật 6,
              bất biến 8). `service-comms` không sở hữu danh bạ cán bộ nên không có tên để nối. */}
          <dd>{thongBao.author_code === "" ? DAU_GACH : thongBao.author_code}</dd>
        </div>
        <div>
          <dt>Xác nhận đã đọc</dt>
          <dd>{boDem ?? "Không bắt buộc xác nhận"}</dd>
        </div>
        <div>
          <dt>Thư điện tử</dt>
          {/* HAI SỰ THẬT, KHÔNG MỘT: `email_requested` là điều đã được yêu cầu, `email_status` là
              điều đã xảy ra. Gộp chúng lại sẽ làm "chưa gửi được" không phân biệt được với "không
              ai yêu cầu gửi thư". */}
          <dd>
            {thongBao.email_requested ? "Có yêu cầu gửi" : "Không yêu cầu gửi"} ·{" "}
            {nhanTrangThaiThu(thongBao.email_status) ?? "Chưa gửi"}
          </dd>
        </div>
      </dl>

      {/* CHỖ CỦA KHỐI NGƯỜI NHẬN VÀ NÚT GỠ — hai dòng chữ nói rõ vì sao, không phải hai nút mờ.
          Một nút mờ nói "bạn không có quyền"; sự thật là màn hình chưa dựng, và hai câu ấy không
          được lẫn vào nhau. */}
      <p className="ghi-chu">{CHO_DANH_SACH_NGUOI_NHAN}</p>
      <p className="ghi-chu">{CHO_NUT_GO}</p>
    </div>
  );
}

/**
 * Biểu mẫu "Soạn thông báo" §5.
 *
 * ĐẶC TẢ GỌI NÓ LÀ MODAL. Ở đây nó là một khối nằm trong trang — KHÔNG phải một lớp phủ — vì một
 * lớp phủ cần lớp CSS chưa có trong `globals.css`, và lượt này không được thêm CSS.
 *
 * BA THỨ CỦA §5 KHÔNG CÓ Ở ĐÂY: ô chọn bộ phận (máy chủ trả 501), ô chọn người nhận theo
 * `Họ tên — email` (`GET /api/v1/staff` đòi `admin.user`), và nút `Lưu nháp` (không có tuyến). Lý
 * do từng cái một nằm ở `PHAN_CHUA_DUNG`, hiện ngay đầu màn.
 */
export function FormSoanThongBao({
  dangGui,
  loi,
  huy,
  phatHanh,
}: {
  dangGui: boolean;
  loi: string | null;
  huy: () => void;
  phatHanh: (than: PhatHanhThongBaoVao, khoaChongTrung: string) => void;
}) {
  const [tieuDe, datTieuDe] = useState("");
  const [noiDung, datNoiDung] = useState("");
  const [nguoiNhan, datNguoiNhan] = useState("");
  const [ghim, datGhim] = useState(false);
  const [batBuocXacNhan, datBatBuocXacNhan] = useState(false);
  // §5: ô này **mặc định bật**. Cột `gui_thu_dien_tu` ghi lại ĐIỀU ĐÃ ĐƯỢC YÊU CẦU, nên giá trị
  // mặc định của đặc tả được giữ nguyên — kèm một câu ngay dưới nói rằng chưa có thư nào đi.
  const [guiThu, datGuiThu] = useState(true);
  // Sinh ở chỗ MỞ biểu mẫu, không ở chỗ gửi: bấm lại sau một lỗi mạng phải dùng LẠI đúng khoá ấy,
  // vì lần gửi đầu có thể đã tới máy chủ và đã phát một thông báo đi khắp xã.
  const [khoaChongTrung] = useState(khoaChongTrungMoi);

  const tieuDeGon = tieuDe.trim();
  const noiDungGon = noiDung.trim();
  const dsNguoiNhan = tachMaNguoiNhan(nguoiNhan);

  // BA ĐIỀU KIỆN, KHÔNG HAI. Hợp đồng đánh dấu `title` và `body` bắt buộc, nhưng máy chủ còn từ
  // chối một thông báo KHÔNG CÓ NGƯỜI NHẬN — và đó là lần từ chối tốn nhất nếu để lọt: người soạn
  // gõ xong cả trang rồi mới biết. Máy chủ vẫn là nơi từ chối thật; nút tắt chỉ để không phải gõ lại.
  const duDieuKien = tieuDeGon !== "" && noiDungGon !== "" && dsNguoiNhan.length > 0;

  function guiNgay(e: FormEvent) {
    e.preventDefault();
    if (!duDieuKien) return;

    phatHanh(
      {
        title: tieuDeGon,
        body: noiDungGon,
        recipient_codes: dsNguoiNhan,
        pinned: ghim,
        ack_required: batBuocXacNhan,
        email_requested: guiThu,
      },
      khoaChongTrung,
    );
  }

  return (
    <form className="form-danh-muc" onSubmit={guiNgay} aria-labelledby="tieu-de-soan-thong-bao">
      <h3 id="tieu-de-soan-thong-bao">Soạn thông báo</h3>
      <p className="ghi-chu">{MO_TA_SOAN}</p>

      <div className="o-nhap">
        <label htmlFor="tieu-de-thong-bao">Tiêu đề *</label>
        <input
          id="tieu-de-thong-bao"
          name="tieu-de-thong-bao"
          value={tieuDe}
          placeholder={PLACEHOLDER_TIEU_DE}
          maxLength={TIEU_DE_TOI_DA}
          autoComplete="off"
          onChange={(e) => datTieuDe(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="noi-dung-thong-bao">Nội dung *</label>
        <textarea
          id="noi-dung-thong-bao"
          name="noi-dung-thong-bao"
          rows={6}
          value={noiDung}
          maxLength={NOI_DUNG_TOI_DA}
          onChange={(e) => datNoiDung(e.target.value)}
        />
      </div>

      <div className="o-nhap">
        <label htmlFor="nguoi-nhan-thong-bao">Gửi thêm đích danh *</label>
        <textarea
          id="nguoi-nhan-thong-bao"
          name="nguoi-nhan-thong-bao"
          rows={4}
          value={nguoiNhan}
          // KHÔNG CÓ `maxLength` TRÊN CẢ Ô: trần của máy chủ là hai trần khác nhau — 500 người và
          // 64 ký tự MỖI MÃ — nên một con số duy nhất trên cả ô sẽ cắt sai ở cả hai chiều. Máy chủ
          // từ chối bằng câu của nó, và câu ấy nói đúng chỗ sai.
          onChange={(e) => datNguoiNhan(e.target.value)}
        />
        <p className="ghi-chu">
          {GHI_CHU_NGUOI_NHAN} Tối đa {NGUOI_NHAN_TOI_DA} người, mỗi mã tối đa{" "}
          {MA_CAN_BO_TOI_DA} ký tự.
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="ghim-thong-bao">
          <input
            id="ghim-thong-bao"
            name="ghim-thong-bao"
            type="checkbox"
            checked={ghim}
            onChange={(e) => datGhim(e.target.checked)}
          />{" "}
          Ghim lên đầu danh sách
        </label>
        <p className="ghi-chu">{CANH_BAO_GHIM_TRONG_TRANG}</p>
      </div>

      <div className="o-nhap">
        <label htmlFor="bat-buoc-xac-nhan">
          <input
            id="bat-buoc-xac-nhan"
            name="bat-buoc-xac-nhan"
            type="checkbox"
            checked={batBuocXacNhan}
            onChange={(e) => datBatBuocXacNhan(e.target.checked)}
          />{" "}
          Bắt buộc xác nhận đã đọc
        </label>
      </div>

      <div className="o-nhap">
        <label htmlFor="gui-thu-dien-tu">
          <input
            id="gui-thu-dien-tu"
            name="gui-thu-dien-tu"
            type="checkbox"
            checked={guiThu}
            onChange={(e) => datGuiThu(e.target.checked)}
          />{" "}
          Gửi thư điện tử cho người nhận
        </label>
        <p className="ghi-chu">{CANH_BAO_CHUA_GUI_THU}</p>
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
          {NHAN_NUT_PHAT_HANH}
        </button>
      </div>
    </form>
  );
}
