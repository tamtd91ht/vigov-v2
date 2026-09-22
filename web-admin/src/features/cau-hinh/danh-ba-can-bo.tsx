"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  KHOA_SAP_XEP,
  layChiTietCanBo,
  layDanhSachCanBo,
  type ChieuSapXep,
  type KhoaSapXep,
} from "@/lib/api/can-bo";
import { docDanhMucDanhBa, type DanhMucDanhBa } from "@/lib/api/danh-muc";
import type { identity_canBoTomTat, page_Result_identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  coTrangTruoc,
  sangTrangSau,
  veTrangTruoc,
  TRANG_DAU,
  type NganXepConTro,
} from "./ngan-xep-con-tro";
import {
  nhanBoPhan,
  nhanDangNhapGanNhat,
  nhanNgayTao,
  nhanTaiKhoan,
  nhanTrangThai,
  nhanVaiTro,
} from "./nhan-can-bo";
import { bangTraTuKetQua, traTen, type BangTraDanhMuc, type KetTra } from "./tra-danh-muc";

/**
 * Bảng danh bạ cán bộ — `docs/ui-ux/14-cau-hinh.md §3`, tab "Người dùng".
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * MÀN HÌNH NÀY ÍT HƠN ĐẶC TẢ, VÀ ĐÓ LÀ CHỦ Ý. Hợp đồng REST hiện có BỐN tuyến đọc phục vụ màn
 * hình này: `GET /api/v1/staff`, `GET /api/v1/staff/{id}`, và hai danh mục của xã —
 * `GET /api/v1/org-units`, `GET /api/v1/roles` — dùng để tra `department_id` và `role_id` ra
 * tên. KHÔNG có tuyến ghi nào. Mỗi thứ đặc tả vẽ mà ở đây không có đều mang một chú thích ngay
 * tại chỗ nói vì sao nó vắng và cái gì mở khoá nó.
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
  /** `null` là chưa đọc xong. Hai danh mục của xã, đọc MỘT lần cho cả màn hình — xem dưới. */
  const [danhMuc, datDanhMuc] = useState<DanhMucDanhBa | null>(null);
  /** Id của lần bấm "Chi tiết" mới nhất — xem `moChiTiet`. */
  const idDangDoi = useRef<string | null>(null);

  /**
   * HAI DANH MỤC, ĐỌC ĐÚNG MỘT LƯỢT KHI MỞ MÀN HÌNH — `[]` ở cuối effect là phần quan trọng
   * nhất của khối này.
   *
   * Không đọc lại khi đổi trang, đổi sắp xếp hay mở khối chi tiết: danh mục bộ phận và vai trò
   * của một xã không đổi giữa hai lần bấm "Trang sau". Và tuyệt đối không đọc theo từng dòng —
   * ở đây mỗi trang hai mươi dòng, nên một lời gọi mỗi dòng là bốn mươi lời gọi thay vì hai,
   * và con số ấy đi lên theo dữ liệu chứ không đứng yên (`skills/load-data-once`, dạng 1).
   *
   * VÌ SAO ĐỌC Ở TRÌNH DUYỆT CHỨ KHÔNG Ở MÁY CHỦ: hai tuyến này đòi đã đăng nhập, nên gọi phía
   * máy chủ thì phải tự chuyển tiếp cookie phiên — thêm một chỗ cầm cookie, và là đúng chỗ dễ
   * chuyển tiếp sang sai host. Ở đây đường dẫn tương đối trên chính host của xã, trình duyệt
   * tự gửi cookie host-only (`lib/api/goi.ts`). Khác hẳn `GET /api/v1/communes/current`: tuyến
   * ấy công khai nên đọc được ở máy chủ (`lib/tenant-config.ts`).
   *
   * KHÔNG CÓ BỘ ĐỆM NÀO SỐNG QUA LẦN MỞ MÀN HÌNH: bảng tra nằm trong state của component, chết
   * cùng component. Một biến ở mức module giữ danh mục lại là đúng hình dạng của một lần danh
   * mục xã này hiện trên màn hình xã khác (`lib/api/danh-muc.ts`).
   */
  useEffect(() => {
    let bo = false;
    docDanhMucDanhBa().then((dm) => {
      if (!bo) datDanhMuc(dm);
    });
    return () => {
      bo = true;
    };
  }, []);

  // Dựng bảng tra một lần cho mỗi lần danh mục đổi, không dựng lại ở mỗi dòng.
  const traBoPhan = useMemo<BangTraDanhMuc>(
    () => bangTraTuKetQua(danhMuc === null ? null : danhMuc.boPhan),
    [danhMuc],
  );
  const traVaiTro = useMemo<BangTraDanhMuc>(
    () => bangTraTuKetQua(danhMuc === null ? null : danhMuc.vaiTro),
    [danhMuc],
  );

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
          {/*
            DANH MỤC HỎNG THÌ NÓI RA MỘT LẦN Ở ĐÂY, chứ không để hai mươi ô cùng báo lỗi. Hiện
            đúng `message` của máy chủ, không diễn giải và không rẽ nhánh theo `code`
            (`lib/api/goi.ts`). Mỗi danh mục một dòng: hỏng một tuyến không được nói thành hỏng
            cả hai.

            ĐẶT CẠNH BẢNG, KHÔNG ĐẶT TRÊN ĐẦU MÀN HÌNH: khi chính danh sách cán bộ cũng hỏng
            (phiên hết hạn chẳng hạn) thì cả ba tuyến cùng trả một câu, và ba dòng giống hệt
            nhau chồng lên nhau không nói thêm được gì. Ở đây hai dòng này chỉ hiện khi có bảng
            để mà thiếu cột — đúng lúc câu ấy giải thích được một thứ người dùng đang nhìn.
          */}
          <BaoLoiDanhMuc nhan="Danh mục bộ phận" bang={traBoPhan} />
          <BaoLoiDanhMuc nhan="Danh mục vai trò" bang={traVaiTro} />
          <BangCanBo
            danhSach={trangThai.trang.items}
            khoa={khoaSapXep}
            chieu={chieu}
            doiSapXep={doiSapXep}
            moChiTiet={moChiTiet}
            idDangMo={chiTiet?.id ?? null}
            traBoPhan={traBoPhan}
            traVaiTro={traVaiTro}
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

      {chiTiet !== null && (
        <KhoiChiTiet
          chiTiet={chiTiet}
          dong={dongChiTiet}
          traBoPhan={traBoPhan}
          traVaiTro={traVaiTro}
        />
      )}
    </section>
  );
}

/**
 * Một dòng báo khi không đọc được một danh mục.
 *
 * KHÔNG dựng gì khi danh mục đang đọc hoặc đã đọc xong: một chỗ trống dành sẵn cho thông báo
 * lỗi là một chỗ trống nhảy chữ vào giữa lúc người dùng đang đọc bảng.
 */
function BaoLoiDanhMuc({ nhan, bang }: { nhan: string; bang: BangTraDanhMuc }) {
  if (bang.pha !== "loi") return null;
  return (
    <p className="thong-bao-loi" role="alert">
      {nhan}: {bang.thongBao}
    </p>
  );
}

/**
 * Một ô tra danh mục — cột Bộ phận và cột Vai trò dùng chung.
 *
 * KHÔNG BAO GIỜ DỰNG RA MỘT Ô TRỐNG: `nhan` luôn trả một câu, kể cả khi không tra được. Lớp CSS
 * đi theo LOẠI kết quả chứ không theo câu chữ, để "chưa gán" (một trạng thái bình thường) và
 * "không tra được" (một dòng dữ liệu lệch) không trông giống nhau.
 */
function ODanhMuc({ ket, nhan }: { ket: KetTra; nhan: (ket: KetTra) => string }) {
  return <span className={lopNhanDanhMuc(ket)}>{nhan(ket)}</span>;
}

function lopNhanDanhMuc(ket: KetTra): string | undefined {
  switch (ket.loai) {
    case "coTen":
      return undefined;
    case "khongTraDuoc":
      return "nhan-lech";
    default:
      return "nhan-trong";
  }
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

// EXPORTED SO THE TWO PHONE COLUMNS CAN BE PINNED BY A RENDER TEST. The parent reads the API in
// `useEffect`, which `renderToStaticMarkup` never runs, so rendering it proves nothing about a row.
// The property being pinned is not cosmetic: merging these two back into one column is a one-line
// edit that no existing test sees, and it would put duty information and Decree 13 personal data
// under one label — see the header comment on the columns.
export function BangCanBo({
  danhSach,
  khoa,
  chieu,
  doiSapXep,
  moChiTiet,
  idDangMo,
  traBoPhan,
  traVaiTro,
}: {
  danhSach: readonly identity_canBoTomTat[];
  khoa: KhoaSapXep;
  chieu: ChieuSapXep;
  doiSapXep: (khoa: KhoaSapXep) => void;
  moChiTiet: (id: string) => void;
  idDangMo: string | null;
  /** Bảng tra đã dựng sẵn, đi XUỐNG như tham số. Không dòng nào tự đi hỏi máy chủ. */
  traBoPhan: BangTraDanhMuc;
  traVaiTro: BangTraDanhMuc;
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
              HAI CỘT NÀY TRA TỪ DANH MỤC, KHÔNG HIỆN ID. Hợp đồng trả `department_id` và
              `role_id` là ULID; một ULID trên màn hình là một chuỗi vô nghĩa với cán bộ. Tên
              lấy từ `GET /api/v1/org-units` và `GET /api/v1/roles`, đọc một lần cho cả màn
              hình, và mọi ca không tra được đều có câu chữ riêng (`tra-danh-muc.ts`).

              Cột `Vai trò` không nằm trong bảng cột của đặc tả §3 — §3 chỉ liệt `Bộ phận` — mà
              đến từ ô `Vai trò` của form người dùng ngay dưới đó và từ lý do phân quyền của
              chính tuyến `GET /api/v1/roles` ("cột Vai trò của danh bạ"). Nêu ra vì đây là chỗ
              màn hình NHIỀU hơn bảng cột của đặc tả, ngược với phần còn lại của tệp này.
            */}
            <th scope="col">Bộ phận</th>
            <th scope="col">Vai trò</th>
            {/* HAI CỘT, KHÔNG MỘT — và nhãn phải nói rõ loại nào, không phải "Điện thoại" trung
                tính. Câu mở #16, chốt 22/09/2026: máy bàn cơ quan là THÔNG TIN CÔNG VỤ, di động cá
                nhân là DỮ LIỆU CÁ NHÂN theo Nghị định 13. Hai địa vị pháp lý khác nhau nghĩa là hai
                luật che, hai luật xuất Excel, hai luật công khai ra Mini App.

                Một nhãn trung tính là chỗ người sắp bấm nút xuất, hay sắp tick ô công khai, không
                biết mình đang đụng loại nào — và đó là lúc một số di động cá nhân rời khỏi cơ quan
                mà không ai định làm thế. Cột này trước 22/09 hiện số CƠ QUAN dưới nhãn trung tính
                ấy, tức nó còn mập mờ theo cả chiều ngược lại. */}
            <th scope="col">Máy bàn cơ quan</th>
            <th scope="col">Di động cá nhân</th>
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
              <td>
                <ODanhMuc ket={traTen(traBoPhan, cb.department_id)} nhan={nhanBoPhan} />
              </td>
              <td>
                <ODanhMuc ket={traTen(traVaiTro, cb.role_id)} nhan={nhanVaiTro} />
              </td>
              {/* HAI Ô RIÊNG. Không gộp bằng `phone || mobile` và không nối bằng dấu phẩy: một ô
                  chứa hai loại số là ô mà mọi luật che, luật xuất và luật công khai về sau phải áp
                  CHUNG một mức cho hai thứ có địa vị pháp lý khác nhau — và mức an toàn buộc lấy
                  theo loại nhạy hơn, tức số máy bàn của cơ quan cũng bị che vô cớ.

                  HIỆN NGUYÊN VĂN thứ máy chủ trả, không định dạng lại thành `0900 000 001` như ví
                  dụ trong đặc tả. Câu chú thích cũ ở đây nói `phone` "LUÔN về đây đã che" — nay
                  SAI: #11 chốt 22/09/2026 là không che trong nội bộ xã, và máy chủ trả số nguyên
                  vẹn (`soRaManHinhNoiBo`). Việc che còn nguyên ở bản xuất Excel và ở mọi đường ra
                  ngoài cơ quan, hai bề mặt chưa tồn tại. */}
              <td>{cb.phone}</td>
              <td>{cb.mobile}</td>
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
 * BỘ PHẬN VÀ VAI TRÒ HIỆN Ở ĐÂY BẰNG CHÍNH BẢNG TRA CỦA BẢNG DANH SÁCH, không đọc lại danh mục
 * khi mở khối này: dữ liệu đã nằm trong tay màn hình rồi, và một lời gọi nữa ở đây chỉ thêm một
 * câu trả lời thứ hai có thể lệch với câu đang hiện trên dòng ngay phía trên.
 */
function KhoiChiTiet({
  chiTiet,
  dong,
  traBoPhan,
  traVaiTro,
}: {
  chiTiet: TrangThaiChiTiet;
  dong: () => void;
  traBoPhan: BangTraDanhMuc;
  traVaiTro: BangTraDanhMuc;
}) {
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
          <dt>Bộ phận</dt>
          <dd>
            <ODanhMuc ket={traTen(traBoPhan, chiTiet.canBo.department_id)} nhan={nhanBoPhan} />
          </dd>
          <dt>Vai trò</dt>
          <dd>
            <ODanhMuc ket={traTen(traVaiTro, chiTiet.canBo.role_id)} nhan={nhanVaiTro} />
          </dd>
          {/* Cùng lý do như hai cột của bảng: hai địa vị pháp lý khác nhau thì hai nhãn khác nhau,
              kể cả ở màn chi tiết nơi chỗ hiển thị không thiếu. */}
          <dt>Máy bàn cơ quan</dt>
          <dd>{chiTiet.canBo.phone}</dd>
          <dt>Di động cá nhân</dt>
          <dd>{chiTiet.canBo.mobile}</dd>
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
