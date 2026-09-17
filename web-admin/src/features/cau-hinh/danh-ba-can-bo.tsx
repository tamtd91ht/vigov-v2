"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import {
  KHOA_SAP_XEP,
  layChiTietCanBo,
  layDanhSachCanBo,
  type ChieuSapXep,
  type KhoaSapXep,
} from "@/lib/api/can-bo";
import type { identity_canBoTomTat, page_Result_identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  coTrangTruoc,
  sangTrangSau,
  veTrangTruoc,
  TRANG_DAU,
  type NganXepConTro,
} from "./ngan-xep-con-tro";
import {
  nhanDangNhapGanNhat,
  nhanNgayTao,
  nhanTaiKhoan,
  nhanTrangThai,
} from "./nhan-can-bo";

/**
 * Bảng danh bạ cán bộ — `docs/ui-ux/14-cau-hinh.md §3`, tab "Người dùng".
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * MÀN HÌNH NÀY ÍT HƠN ĐẶC TẢ, VÀ ĐÓ LÀ CHỦ Ý. Hợp đồng REST hiện có đúng HAI tuyến đọc:
 * `GET /api/v1/staff` và `GET /api/v1/staff/{id}`. Mỗi thứ đặc tả vẽ mà ở đây không có đều
 * mang một chú thích ngay tại chỗ nói vì sao nó vắng và cái gì mở khoá nó.
 *
 * Vẽ ra một điều khiển không chạy được tệ hơn hẳn không vẽ: một ô tìm kiếm gõ vào không có gì
 * xảy ra khiến cán bộ gõ tên một người, thấy danh sách không đổi, và kết luận người đó không
 * có trong hệ thống.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/** Trạng thái của một lần đọc. Ba nhánh rời nhau — không nhánh nào suy ra được từ nhánh khác. */
type TrangThaiTrang =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; trang: page_Result_identity_canBoTomTat };

type TrangThaiChiTiet =
  | { pha: "dangTai"; id: string }
  | { pha: "loi"; id: string; thongBao: string }
  | { pha: "xong"; id: string; canBo: identity_canBoTomTat };

export function DanhBaCanBo() {
  const [khoaSapXep, datKhoaSapXep] = useState<KhoaSapXep>("code");
  const [chieu, datChieu] = useState<ChieuSapXep>("asc");
  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [trangThai, datTrangThai] = useState<TrangThaiTrang>({ pha: "dangTai" });
  const [chiTiet, datChiTiet] = useState<TrangThaiChiTiet | null>(null);
  /** Id của lần bấm "Chi tiết" mới nhất — xem `moChiTiet`. */
  const idDangDoi = useRef<string | null>(null);

  useEffect(() => {
    // `bo` chặn một phản hồi đến muộn của lần đọc trước ghi đè lên lần đọc sau. Không có nó thì
    // bấm "Trang sau" hai lần nhanh có thể để lại trên màn hình đúng trang vừa rời khỏi.
    let bo = false;

    // Không truyền `limit`: để máy chủ áp mặc định của chính nó (20). Giữ một bản sao của con
    // số ấy ở client là giữ một bản sẽ trôi.
    layDanhSachCanBo({ sort: khoaSapXep, order: chieu, cursor: nganXep.hienTai }).then((ketQua) => {
      if (bo) return;
      datTrangThai(
        ketQua.ok ? { pha: "xong", trang: ketQua.duLieu } : { pha: "loi", thongBao: ketQua.thongBao },
      );
    });

    return () => {
      bo = true;
    };
  }, [khoaSapXep, chieu, nganXep]);

  /**
   * Chuyển trang. `dangTai` được đặt Ở ĐÂY, trong sự kiện bấm, chứ không trong thân effect:
   * gọi setState thẳng trong thân effect kéo theo một lượt render phụ mỗi lần chạy, và lint của
   * React chặn đúng mẫu ấy. Trạng thái khởi tạo đã là `dangTai` nên lần tải đầu không cần ai
   * đặt gì.
   */
  const dongChiTiet = useCallback(() => {
    idDangDoi.current = null;
    datChiTiet(null);
  }, []);

  const diToiTrang = useCallback(
    (toi: NganXepConTro) => {
      datTrangThai({ pha: "dangTai" });
      dongChiTiet();
      datNganXep(toi);
    },
    [dongChiTiet],
  );

  /**
   * Đổi sắp xếp là VỀ TRANG ĐẦU, luôn luôn.
   *
   * Một con trỏ thuộc về đúng một cách sắp xếp: nó mã hoá mốc `(khoá sắp xếp, id)` của dòng
   * cuối vừa phát ra. Mang con trỏ của `sort=code` sang `sort=created_at` thì máy chủ trả 400
   * "con trỏ không hợp lệ" (`core/page/page.go`, `ErrCursor`) — nên ngăn xếp cũ phải bỏ đi,
   * không phải giữ lại.
   */
  const doiSapXep = useCallback(
    (khoa: KhoaSapXep) => {
      if (khoa === khoaSapXep) {
        datChieu((truoc) => (truoc === "asc" ? "desc" : "asc"));
      } else {
        datKhoaSapXep(khoa);
        datChieu("asc");
      }
      diToiTrang(TRANG_DAU);
    },
    [khoaSapXep, diToiTrang],
  );

  /**
   * Mở khối chi tiết của một cán bộ.
   *
   * `idDangDoi` KHÔNG PHẢI TỐI ƯU HOÁ. Không có nó, một phản hồi đến muộn của lần bấm trước sẽ
   * ghi đè khối chi tiết: trên màn hình là hồ sơ của người A nằm dưới dòng người B vừa bấm —
   * ghép sai dữ liệu cá nhân với sai người, không phải một lỗi hiển thị.
   */
  const moChiTiet = useCallback(async (id: string) => {
    idDangDoi.current = id;
    datChiTiet({ pha: "dangTai", id });
    const ketQua = await layChiTietCanBo(id);
    if (idDangDoi.current !== id) return;
    datChiTiet(
      ketQua.ok
        ? { pha: "xong", id, canBo: ketQua.duLieu }
        : { pha: "loi", id, thongBao: ketQua.thongBao },
    );
  }, []);

  return (
    <section className="tab-nguoi-dung" aria-labelledby="tieu-de-nguoi-dung">
      <h2 id="tieu-de-nguoi-dung">Người dùng</h2>

      {/*
        ĐẶC TẢ CÓ, Ở ĐÂY KHÔNG — và mỗi dòng nói luôn cái gì mở khoá nó:

          · Ô tìm `Tìm theo tên, thư điện tử, bộ phận…`: `GET /api/v1/staff` KHÔNG nhận tham số
            tìm kiếm nào (`core/page/page.go` chỉ đọc limit/cursor/sort/order). Mở khoá bằng một
            tham số truy vấn mới trên tuyến ấy — một thay đổi hợp đồng, phải qua khai báo route
            trong `service-identity/internal/`, không phải một ô input ở đây.
          · `⬆ Nhập từ Excel` và `+ Thêm cán bộ`: không có tuyến ghi nào. Đang chờ khách chốt
            câu hỏi mở #9 (mật khẩu đầu tiên tới tay cán bộ mới bằng cách nào).
          · `✎` `🗑` trên từng dòng: cũng không có tuyến ghi nào — câu hỏi mở #10 (nghỉ hưu thì
            khoá hay xoá), #13 (có chặn việc xã mất người quản trị cuối cùng không), #14
            (`admin.user` có được tác động lên chính tài khoản mình không).
      */}
      <p className="ghi-chu">
        Màn hình hiện chỉ xem. Thêm, sửa, xoá cán bộ và nhập từ Excel chưa mở vì quy trình cấp
        mật khẩu đầu tiên và quy trình kết thúc công tác chưa được đơn vị chốt.
      </p>

      <ThanhSapXep khoa={khoaSapXep} chieu={chieu} doiSapXep={doiSapXep} />

      {trangThai.pha === "dangTai" && <p role="status">Đang tải danh sách…</p>}

      {/*
        LỖI: hiện đúng `message` của máy chủ, không diễn giải. Mọi mã lỗi — kể cả 401, 403, 404 —
        đều trả cùng hình dạng `httpx.Error`, nên không có chỗ nào ở đây rẽ nhánh theo `code` để
        đoán chuyện gì đã xảy ra, và `trace_id` không hiện ra: nó là mốc tra log, không phải mã
        lỗi nghiệp vụ (xem `lib/api/goi.ts`).
      */}
      {trangThai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {trangThai.thongBao}
        </p>
      )}

      {trangThai.pha === "xong" && trangThai.trang.items.length === 0 && (
        // TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI. Một xã vừa onboard có danh bạ rỗng thật;
        // máy chủ trả `items: []` chứ không bao giờ trả `null`. Câu chữ vì vậy phải nói rõ là
        // "chưa có ai", để không ai đi tìm lỗi mạng ở một hệ thống đang chạy đúng.
        <p className="trang-thai-rong">
          Đơn vị chưa có cán bộ nào trong danh bạ. Khi cán bộ được thêm vào, danh sách sẽ hiện ở
          đây.
        </p>
      )}

      {trangThai.pha === "xong" && trangThai.trang.items.length > 0 && (
        <>
          <BangCanBo
            danhSach={trangThai.trang.items}
            khoa={khoaSapXep}
            chieu={chieu}
            doiSapXep={doiSapXep}
            moChiTiet={moChiTiet}
            idDangMo={chiTiet?.id ?? null}
          />
          <p className="ghi-chu">
            Số điện thoại hiển thị dạng che theo quy định về bảo vệ dữ liệu cá nhân.
          </p>
          <DieuHuongTrang
            nganXep={nganXep}
            conTroTiep={trangThai.trang.next_cursor}
            conTrangSau={trangThai.trang.has_more}
            diToiTrang={diToiTrang}
          />
        </>
      )}

      {chiTiet !== null && <KhoiChiTiet chiTiet={chiTiet} dong={dongChiTiet} />}
    </section>
  );
}

/**
 * Điều khiển sắp xếp, đặt trên bảng để dùng được cả ở bề rộng nhỏ nhất (320px) — ở đó bảng cuộn
 * ngang, nên một nút nằm trong ô tiêu đề cột có thể đang ở ngoài khung nhìn.
 *
 * CHỈ HAI KHOÁ, và đó là toàn bộ những gì máy chủ nhận (xem `KHOA_SAP_XEP`).
 */
function ThanhSapXep({
  khoa,
  chieu,
  doiSapXep,
}: {
  khoa: KhoaSapXep;
  chieu: ChieuSapXep;
  doiSapXep: (khoa: KhoaSapXep) => void;
}) {
  return (
    <div className="thanh-sap-xep">
      <span className="nhan-sap-xep">Sắp xếp theo</span>
      {KHOA_SAP_XEP.map((k) => (
        <button
          key={k}
          type="button"
          className="nut-phu"
          aria-pressed={k === khoa}
          onClick={() => doiSapXep(k)}
        >
          {NHAN_KHOA[k]}
          {k === khoa ? (chieu === "asc" ? " ↑" : " ↓") : " ⇅"}
        </button>
      ))}
    </div>
  );
}

/** Nhãn người đọc của hai khoá sắp xếp. Khoá là chuỗi của hợp đồng, nhãn là chữ của đặc tả. */
const NHAN_KHOA: Record<KhoaSapXep, string> = {
  code: "Mã cán bộ",
  created_at: "Ngày tạo",
};

function BangCanBo({
  danhSach,
  khoa,
  chieu,
  doiSapXep,
  moChiTiet,
  idDangMo,
}: {
  danhSach: readonly identity_canBoTomTat[];
  khoa: KhoaSapXep;
  chieu: ChieuSapXep;
  doiSapXep: (khoa: KhoaSapXep) => void;
  moChiTiet: (id: string) => void;
  idDangMo: string | null;
}) {
  return (
    // `role="region"` + `tabIndex` để vùng cuộn ngang tới được bằng bàn phím. Ở dưới 768px bảng
    // cuộn ngang chứ không đổi thành thẻ: đổi `display` của các phần tử bảng làm mất ngữ nghĩa
    // bảng với trình đọc màn hình, mà đây đúng là dữ liệu dạng bảng.
    <div className="bang-cuon" role="region" aria-label="Danh sách cán bộ" tabIndex={0}>
      <table className="bang-can-bo">
        <caption className="an-thi-giac">
          Danh sách cán bộ của đơn vị, sắp xếp theo {NHAN_KHOA[khoa].toLowerCase()}{" "}
          {chieu === "asc" ? "tăng dần" : "giảm dần"}
        </caption>
        <thead>
          <tr>
            <OTieuDeSapXep khoa="code" khoaHienTai={khoa} chieu={chieu} doiSapXep={doiSapXep} />
            <th scope="col">Họ và tên</th>
            <th scope="col">Chức danh</th>
            {/*
              CỘT `Bộ phận` CỦA ĐẶC TẢ KHÔNG CÓ Ở ĐÂY. Hợp đồng chỉ trả `department_id` và
              `role_id` — chưa tuyến nào dịch id ra tên bộ phận hay tên vai trò. Hiện một ULID
              thô cho cán bộ là hiện một chuỗi vô nghĩa với họ; bịa tên thì tệ hơn nữa. Mở khoá
              bằng một tuyến trả danh mục bộ phận / vai trò của xã, hoặc bằng việc tuyến danh
              sách trả kèm tên — cả hai đều là thay đổi hợp đồng.
            */}
            <th scope="col">Điện thoại</th>
            <th scope="col">Đăng nhập gần nhất</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Tài khoản</th>
            <OTieuDeSapXep
              khoa="created_at"
              khoaHienTai={khoa}
              chieu={chieu}
              doiSapXep={doiSapXep}
            />
            <th scope="col">
              <span className="an-thi-giac">Hành động</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {danhSach.map((cb) => (
            <tr key={cb.id}>
              <td>{cb.code}</td>
              <td>
                <span className="ten-can-bo">{cb.full_name}</span>
                <span className="dong-phu">{cb.email}</span>
              </td>
              <td>{cb.position}</td>
              {/* `phone` LUÔN về đây đã che (`09****0000`) — hiện đúng thứ máy chủ trả, không
                  ghép lại, không định dạng lại thành `0900 000 001` như ví dụ trong đặc tả: một
                  số đã che mà định dạng như số thật là mời người đọc tin đó là số thật. */}
              <td>{cb.phone}</td>
              <td>{nhanDangNhapGanNhat(cb.last_login_at)}</td>
              <td>
                <span className={cb.active ? "chip chip-hoat-dong" : "chip chip-ngung"}>
                  {nhanTrangThai(cb.active)}
                </span>
              </td>
              <td>{nhanTaiKhoan(cb.has_account)}</td>
              <td>{nhanNgayTao(cb.created_at)}</td>
              <td>
                {/* Chỉ một hành động: XEM. `✎` và `🗑` của đặc tả không có tuyến nào phía sau. */}
                <button
                  type="button"
                  className="nut-phu"
                  aria-expanded={idDangMo === cb.id}
                  onClick={() => moChiTiet(cb.id)}
                >
                  Chi tiết
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** Ô tiêu đề của một cột sắp xếp được. `aria-sort` để trình đọc màn hình đọc đúng chiều. */
function OTieuDeSapXep({
  khoa,
  khoaHienTai,
  chieu,
  doiSapXep,
}: {
  khoa: KhoaSapXep;
  khoaHienTai: KhoaSapXep;
  chieu: ChieuSapXep;
  doiSapXep: (khoa: KhoaSapXep) => void;
}) {
  const dangSapXep = khoa === khoaHienTai;
  return (
    <th
      scope="col"
      aria-sort={dangSapXep ? (chieu === "asc" ? "ascending" : "descending") : "none"}
    >
      <button type="button" className="nut-sap-xep" onClick={() => doiSapXep(khoa)}>
        {NHAN_KHOA[khoa]}
        {dangSapXep ? (chieu === "asc" ? " ↑" : " ↓") : " ⇅"}
      </button>
    </th>
  );
}

/**
 * Phân trang theo con trỏ.
 *
 * KHÔNG CÓ SỐ TRANG VÀ KHÔNG CÓ TỔNG SỐ, và đó không phải thiếu sót: hợp đồng trả `next_cursor`
 * + `has_more` chứ không trả `total`, vì máy chủ đọc theo mốc và cố ý không chạy `COUNT(*)` trên
 * bảng đã phân mảnh. Hiện "Trang 3/12" ở đây là báo một con số không ai tính (`core/page`).
 */
function DieuHuongTrang({
  nganXep,
  conTroTiep,
  conTrangSau,
  diToiTrang,
}: {
  nganXep: NganXepConTro;
  conTroTiep: string;
  conTrangSau: boolean;
  diToiTrang: (toi: NganXepConTro) => void;
}) {
  // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi tới đó.
  // Bấm khi con trỏ rỗng thì `sangTrangSau` ném lỗi — nút phải mờ đi trước khi tới đó.
  const coSau = conTrangSau && conTroTiep !== "";
  return (
    <nav className="dieu-huong-trang" aria-label="Phân trang danh sách cán bộ">
      <button
        type="button"
        className="nut-phu"
        disabled={!coTrangTruoc(nganXep)}
        onClick={() => diToiTrang(veTrangTruoc(nganXep))}
      >
        Trang trước
      </button>
      <button
        type="button"
        className="nut-phu"
        disabled={!coSau}
        onClick={() => diToiTrang(sangTrangSau(nganXep, conTroTiep))}
      >
        Trang sau
      </button>
    </nav>
  );
}

/**
 * Chi tiết một cán bộ — `GET /api/v1/staff/{id}`.
 *
 * VÌ SAO KHÔNG PHẢI MỘT ĐƯỜNG DẪN RIÊNG `/…/{id}`: tên tài nguyên URL cho khái niệm "cán bộ"
 * CHƯA ĐƯỢC KHÁCH CHỐT (`kb/00-foundation/ubiquitous-language.md` — ô "Tài nguyên URL" của dòng
 * Cán bộ ghi rõ "CHƯA CHỐT — HỎI KHÁCH", và đường dẫn `/cau-hinh/nguoi-dung` của đặc tả cũ
 * không dùng nữa). Một đường dẫn đã chạy thật thì không sửa lại được, nên ở đây không đặt ra
 * đoạn đường dẫn nào cả; khối chi tiết mở ngay trong trang.
 *
 * VÌ SAO KHÔNG HIỆN `department_id` VÀ `role_id`: xem chú thích ở cột "Bộ phận".
 */
function KhoiChiTiet({ chiTiet, dong }: { chiTiet: TrangThaiChiTiet; dong: () => void }) {
  return (
    <aside className="khoi-chi-tiet" aria-label="Chi tiết cán bộ">
      <div className="dau-khoi-chi-tiet">
        <h3>Chi tiết cán bộ</h3>
        <button type="button" className="nut-phu" onClick={dong}>
          Đóng
        </button>
      </div>

      {chiTiet.pha === "dangTai" && <p role="status">Đang tải…</p>}

      {chiTiet.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {chiTiet.thongBao}
        </p>
      )}

      {chiTiet.pha === "xong" && (
        <dl className="danh-sach-truong">
          <dt>Mã cán bộ</dt>
          <dd>{chiTiet.canBo.code}</dd>
          <dt>Họ và tên</dt>
          <dd>{chiTiet.canBo.full_name}</dd>
          <dt>Thư điện tử</dt>
          <dd>{chiTiet.canBo.email}</dd>
          <dt>Chức danh</dt>
          <dd>{chiTiet.canBo.position}</dd>
          <dt>Điện thoại</dt>
          <dd>{chiTiet.canBo.phone}</dd>
          <dt>Đăng nhập gần nhất</dt>
          <dd>{nhanDangNhapGanNhat(chiTiet.canBo.last_login_at)}</dd>
          <dt>Trạng thái</dt>
          <dd>{nhanTrangThai(chiTiet.canBo.active)}</dd>
          <dt>Tài khoản</dt>
          <dd>{nhanTaiKhoan(chiTiet.canBo.has_account)}</dd>
          <dt>Ngày tạo</dt>
          <dd>{nhanNgayTao(chiTiet.canBo.created_at)}</dd>
        </dl>
      )}
    </aside>
  );
}
