"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import type { identity_boPhanRa } from "@/lib/api/schema.gen";
import { QUYEN_QUAN_LY_SO_DO, quyetDinhTheoKhoa } from "@/lib/quyen";
import { usePhien } from "@/features/phien/phien-hien-tai";

import {
  API_SO_DO,
  banSua,
  banThem,
  dungCay,
  guiBieuMau,
  idNutMo,
  luaChonCha,
  moThem,
  theNeo,
  type BanNhap,
  type DangMo,
  type DongPhang,
  type NutCay,
} from "./cay-bo-phan";
import {
  CAU_THIEU_QUYEN_GHI,
  CHON_KHONG_CO_CHA,
  DANG_TAI,
  GHI_CHU_CHUA_XOA,
  GIAI_THICH_O_CHA_SUA,
  GIAI_THICH_O_MA_THEM,
  GIAI_THICH_O_TEN,
  GIAI_THICH_O_THU_TU,
  LOI_THU_TU,
  NUT_HUY,
  NUT_LUU,
  NUT_SUA_BO_PHAN,
  NUT_THEM_BO_PHAN,
  NUT_THEM_CON,
  O_CHA,
  O_MA,
  O_TEN,
  O_THU_TU,
  TIEU_DE_SO_DO,
  giaiThichMaKhongSua,
  nhanCayRong,
  nhanNutSua,
  nhanNutThemCon,
  nhanSoCanBo,
  tieuDeSua,
  tieuDeThemCon,
  tieuDeThemGoc,
} from "./nhan-so-do";

/**
 * Tab "Sơ đồ tổ chức" — `docs/ui-ux/14-cau-hinh.md §1`: cây bộ phận của đơn vị; thêm, đổi tên,
 * dời sang bộ phận cha khác, đổi thứ tự. KHÔNG CÓ XOÁ — hợp đồng chưa có tuyến ấy
 * (`PHAN_CHUA_DUNG`, `nhan-cau-hinh.ts`).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * CÂY HIỆN CHO MỌI TÀI KHOẢN, CHỈ NÚT GHI MỚI ẨN — cùng khuôn tab Danh mục, và vì cùng một lý do ở
 * máy chủ: `GET /api/v1/org-units` khai `any-authenticated` (tên bộ phận có ở ô phân công và bộ lọc
 * của mọi màn), còn hai tuyến ghi khai `RequirePermission("admin.org")`. Ẩn nút là TIỆN DỤNG, không
 * phải biện pháp: máy chủ kiểm khoá trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * ĐỌC LẠI SAU MỖI LẦN GHI, KHÔNG VÁ TẠI CHỖ. Phản hồi của hai tuyến ghi KHÔNG mang `staff_count`
 * (`bo_phan.go:53`), nên vẽ lại thẻ từ phản hồi ấy là in "0 cán bộ" cho một bộ phận mười hai người.
 *
 * NÚT `⬆ Nhập từ Excel` CỦA ĐẶC TẢ KHÔNG CÓ Ở ĐÂY: không tuyến nào đứng sau nó. Một nút bấm vào
 * không có gì xảy ra khiến cán bộ tin mình thao tác sai.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function TabSoDoToChuc() {
  const [tai, datTai] = useState<TrangThaiTai>({ pha: "dangDoc" });
  const [lanDoc, datLanDoc] = useState(0);

  const [dangMo, datDangMo] = useState<DangMo | null>(null);
  const [ban, datBan] = useState<BanNhap>(banThem(""));
  const [loiTaiCho, datLoiTaiCho] = useState("");
  const [loiMayChu, datLoiMayChu] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [cauDaXong, datCauDaXong] = useState("");

  /**
   * Nút đã mở biểu mẫu — tiêu điểm trả về nó khi biểu mẫu đóng (Huỷ hoặc lưu xong). Giữ `id` chứ
   * không giữ phần tử: sau một lần lưu, cây được đọc lại và nút có thể là một phần tử MỚI.
   */
  const nutDaMo = useRef<string | null>(null);

  const phien = usePhien();
  /** Ba trạng thái: chưa đọc xong phiên thì chưa vẽ nút ghi nào, cũng chưa nói "thiếu quyền". */
  const quyetDinhGhi = phien === null ? null : quyetDinhTheoKhoa(phien, QUYEN_QUAN_LY_SO_DO);
  const coQuyenGhi = quyetDinhGhi !== null && quyetDinhGhi.hien;

  useEffect(() => {
    let bo = false;
    // KHÔNG đặt lại về "đang đọc" trước lượt đọc lại: cây cũ còn trên màn hình trong lúc chờ, nên
    // nút vừa mở biểu mẫu vẫn còn đó để nhận lại tiêu điểm.
    layDanhMucBoPhan().then((kq) => {
      if (bo) return;
      datTai(kq.ok ? { pha: "xong", items: kq.duLieu.items } : { pha: "loi", thongBao: kq.thongBao });
    });
    return () => {
      bo = true;
    };
  }, [lanDoc]);

  // Mở biểu mẫu → tiêu điểm vào ô đầu tiên. Đóng → tiêu điểm về nút đã mở nó.
  useEffect(() => {
    if (dangMo !== null) {
      document.getElementById(O_TEN_ID)?.focus();
      return;
    }
    if (nutDaMo.current !== null) {
      document.getElementById(nutDaMo.current)?.focus();
      nutDaMo.current = null;
    }
  }, [dangMo]);

  const items = useMemo(() => (tai.pha === "xong" ? tai.items : []), [tai]);
  const cay = useMemo(() => dungCay(items), [items]);

  const mo = useCallback((m: DangMo, banDau: BanNhap, idNut: string) => {
    nutDaMo.current = idNut;
    datDangMo(m);
    datBan(banDau);
    datLoiTaiCho("");
    datLoiMayChu("");
    datCauDaXong("");
  }, []);

  const thaoTac = useMemo<ThaoTacCay>(
    () => ({
      themGoc: () =>
        mo(moThem("", null, () => crypto.randomUUID()), banThem(""), idNutMo("themGoc")),
      themCon: (cha) =>
        mo(
          moThem(cha.id, cha.name, () => crypto.randomUUID()),
          banThem(cha.id),
          idNutMo("themCon", cha.id),
        ),
      sua: (bp) => mo({ kieu: "sua", bp }, banSua(bp), idNutMo("sua", bp.id)),
    }),
    [mo],
  );

  const dong = useCallback(() => {
    datDangMo(null);
    datLoiTaiCho("");
    datLoiMayChu("");
  }, []);

  const gui = useCallback(() => {
    if (dangMo === null || dangGui) return;
    datLoiTaiCho("");
    datLoiMayChu("");
    datDangGui(true);
    void guiBieuMau(dangMo, ban, API_SO_DO).then((kq) => {
      datDangGui(false);
      if (kq.kieu === "loiTaiCho") {
        datLoiTaiCho(kq.loi);
        return;
      }
      if (kq.kieu === "loiMayChu") {
        // Biểu mẫu GIỮ NGUYÊN chữ đã gõ, và GIỮ NGUYÊN khoá chống trùng: lần bấm lại là lần thử lại
        // của CÙNG một lần thêm.
        datLoiMayChu(kq.thongBao);
        return;
      }
      datDangMo(null);
      datCauDaXong(kq.cau);
      datLanDoc((n) => n + 1);
    });
  }, [ban, dangGui, dangMo]);

  const bieuMau =
    dangMo === null ? null : (
      <BieuMauBoPhan
        dangMo={dangMo}
        ban={ban}
        datBan={datBan}
        luaChon={luaChonCha(cay, items, dangMo.kieu === "sua" ? dangMo.bp.id : null)}
        loiTaiCho={loiTaiCho}
        loiMayChu={loiMayChu}
        dangGui={dangGui}
        onGui={gui}
        onHuy={dong}
      />
    );
  const neo = dangMo === null ? undefined : theNeo(dangMo);

  return (
    <section className="tab-so-do-to-chuc" aria-labelledby="tieu-de-so-do">
      <h2 id="tieu-de-so-do">{TIEU_DE_SO_DO}</h2>

      {tai.pha === "dangDoc" && <p role="status">{DANG_TAI}</p>}
      {cauDaXong !== "" && <p role="status">{cauDaXong}</p>}

      {quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-doc-duoc" && (
        <p className="thong-bao-loi" role="alert">
          {quyetDinhGhi.thongBao}
        </p>
      )}

      <KhungSoDo
        tai={tai}
        cay={cay}
        coQuyenGhi={coQuyenGhi}
        thieuQuyen={quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-du-quyen"}
        thaoTac={thaoTac}
        bieuMauDauTab={neo === null ? bieuMau : null}
        bieuMauTaiThe={neo !== undefined && neo !== null ? { id: neo, node: bieuMau } : null}
      />
    </section>
  );
}

/** `id` của ô `Tên` — nơi tiêu điểm tới khi biểu mẫu mở. */
const O_TEN_ID = "o-ten-bo-phan";

type TrangThaiTai =
  | { pha: "dangDoc" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; items: readonly identity_boPhanRa[] };

/** Ba thao tác mà nút trên tab gọi. */
export type ThaoTacCay = {
  readonly themGoc: () => void;
  readonly themCon: (cha: identity_boPhanRa) => void;
  readonly sua: (bp: identity_boPhanRa) => void;
};

/**
 * Thân tab: nút thêm, câu thiếu quyền, cây hoặc trạng thái rỗng/lỗi.
 *
 * THUẦN TRÌNH BÀY và XUẤT RA để `tab-so-do-to-chuc.test.tsx` kết xuất bằng `react-dom/server`:
 * câu "nút ghi ẩn khi thiếu `admin.org`" là một điều của JSX, và một phép quyết định đúng trong
 * module thuần không nói gì về việc JSX có vẽ theo nó hay không.
 */
export function KhungSoDo({
  tai,
  cay,
  coQuyenGhi,
  thieuQuyen,
  thaoTac,
  bieuMauDauTab,
  bieuMauTaiThe,
}: {
  tai: TrangThaiTai;
  cay: readonly NutCay[];
  coQuyenGhi: boolean;
  thieuQuyen: boolean;
  thaoTac: ThaoTacCay;
  bieuMauDauTab: ReactNode;
  bieuMauTaiThe: { id: string; node: ReactNode } | null;
}) {
  return (
    <>
      {coQuyenGhi && (
        <p>
          <button
            type="button"
            className="nut-phu"
            id={idNutMo("themGoc")}
            onClick={thaoTac.themGoc}
          >
            {NUT_THEM_BO_PHAN}
          </button>
        </p>
      )}
      {thieuQuyen && <p className="trang-thai-rong">{CAU_THIEU_QUYEN_GHI}</p>}
      <p className="ghi-chu">{GHI_CHU_CHUA_XOA}</p>

      {bieuMauDauTab}

      {/* LỖI ĐỌC: nguyên câu của máy chủ, không diễn giải, không rẽ nhánh theo `code`. */}
      {tai.pha === "loi" && (
        <p className="thong-bao-loi" role="alert">
          {tai.thongBao}
        </p>
      )}

      {tai.pha === "xong" && cay.length === 0 && (
        <p className="trang-thai-rong">{nhanCayRong(coQuyenGhi)}</p>
      )}

      {tai.pha === "xong" && cay.length > 0 && (
        <CapBoPhan
          nut={cay}
          coQuyenGhi={coQuyenGhi}
          thaoTac={thaoTac}
          bieuMauTaiThe={bieuMauTaiThe}
          nhan={TIEU_DE_SO_DO}
        />
      )}
    </>
  );
}

/**
 * Một cấp của cây — danh sách lồng nhau, `<ul>` trong `<li>` của cha.
 *
 * DANH SÁCH LỒNG CHỨ KHÔNG PHẢI THỤT LỀ BẰNG CSS TRÊN MỘT DANH SÁCH PHẲNG: trình đọc màn hình đọc
 * được cấp của một `<ul>` lồng ("danh sách, cấp 2"), còn một lề trái chỉ người nhìn thấy mới đọc
 * được. Quan hệ cha–con là dữ liệu, không phải trang trí.
 */
function CapBoPhan({
  nut,
  coQuyenGhi,
  thaoTac,
  bieuMauTaiThe,
  nhan,
}: {
  nut: readonly NutCay[];
  coQuyenGhi: boolean;
  thaoTac: ThaoTacCay;
  bieuMauTaiThe: { id: string; node: ReactNode } | null;
  nhan?: string;
}) {
  return (
    <ul className="cay-bo-phan" aria-label={nhan}>
      {nut.map((n) => (
        <li key={n.bp.id}>
          <TheBoPhan bp={n.bp} coQuyenGhi={coQuyenGhi} thaoTac={thaoTac} />
          {bieuMauTaiThe !== null && bieuMauTaiThe.id === n.bp.id && bieuMauTaiThe.node}
          {n.con.length > 0 && (
            <CapBoPhan
              nut={n.con}
              coQuyenGhi={coQuyenGhi}
              thaoTac={thaoTac}
              bieuMauTaiThe={bieuMauTaiThe}
            />
          )}
        </li>
      ))}
    </ul>
  );
}

/**
 * Một thẻ bộ phận: tên, mã, số cán bộ, và — khi có `admin.org` — hai nút `＋` và `✎`.
 *
 * KHÔNG CÓ NÚT `🗑`, KỂ CẢ NÚT MỜ. Một nút xoá bấm vào không có gì xảy ra còn tệ hơn không có nút.
 */
function TheBoPhan({
  bp,
  coQuyenGhi,
  thaoTac,
}: {
  bp: identity_boPhanRa;
  coQuyenGhi: boolean;
  thaoTac: ThaoTacCay;
}) {
  return (
    <div className="the-bo-phan">
      <div className="the-bo-phan-than">
        <span className="the-bo-phan-ten">
          <span aria-hidden="true">🏛 </span>
          {bp.name}
        </span>
        {/* Mã font đẳng chiều: nó là slug đọc qua điện thoại, `l`/`1` phải phân biệt được. */}
        <span className="ma-muc the-bo-phan-ma">{bp.code}</span>
      </div>
      <span className="the-bo-phan-so">
        <span aria-hidden="true">👥 </span>
        {nhanSoCanBo(bp.staff_count)}
      </span>
      {coQuyenGhi && (
        <span className="cum-nut">
          <button
            type="button"
            className="nut-phu"
            id={idNutMo("themCon", bp.id)}
            aria-label={nhanNutThemCon(bp.name)}
            onClick={() => thaoTac.themCon(bp)}
          >
            {NUT_THEM_CON}
          </button>
          <button
            type="button"
            className="nut-phu"
            id={idNutMo("sua", bp.id)}
            aria-label={nhanNutSua(bp.name)}
            onClick={() => thaoTac.sua(bp)}
          >
            {NUT_SUA_BO_PHAN}
          </button>
        </span>
      )}
    </div>
  );
}

/**
 * Biểu mẫu thêm hoặc sửa một bộ phận.
 *
 * THUẦN TRÌNH BÀY: mọi giá trị vào qua `ban`, mọi thay đổi ra qua `datBan`, phép dựng thân yêu cầu
 * nằm ở `cay-bo-phan.ts`. XUẤT RA để kết xuất được nhánh "máy chủ vừa từ chối" — câu 409 về vòng
 * lặp phải ra tới trang NGUYÊN VĂN.
 *
 * Ô `Mã` CHỈ CÓ Ở BIỂU MẪU THÊM. Ở biểu mẫu sửa, mã hiện thành chữ: một ô nhập mã sửa được là một ô
 * hứa điều máy chủ sẽ từ chối 400 (luật 7, bất biến 3).
 */
export function BieuMauBoPhan({
  dangMo,
  ban,
  datBan,
  luaChon,
  loiTaiCho,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  dangMo: DangMo;
  ban: BanNhap;
  datBan: (b: BanNhap) => void;
  luaChon: readonly DongPhang[];
  loiTaiCho: string;
  loiMayChu: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const tieuDe =
    dangMo.kieu === "sua"
      ? tieuDeSua(dangMo.bp.name)
      : dangMo.tenCha === null
        ? tieuDeThemGoc()
        : tieuDeThemCon(dangMo.tenCha);

  return (
    <form
      className="form-danh-muc form-bo-phan"
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
      onKeyDown={(e) => {
        // Esc đóng biểu mẫu như `Huỷ` — trừ khi đang gửi, để một lần gửi dở không mất dấu.
        if (e.key === "Escape" && !dangGui) onHuy();
      }}
    >
      <h4>{tieuDe}</h4>

      <div className="o-nhap">
        <label htmlFor={O_TEN_ID}>{O_TEN}</label>
        <input
          id={O_TEN_ID}
          name="ten"
          value={ban.ten}
          onChange={(e) => datBan({ ...ban, ten: e.target.value })}
          aria-describedby="giai-thich-ten-bo-phan"
        />
        <p className="ghi-chu" id="giai-thich-ten-bo-phan">
          {GIAI_THICH_O_TEN}
        </p>
      </div>

      {dangMo.kieu === "them" ? (
        <div className="o-nhap">
          <label htmlFor="o-ma-bo-phan">{O_MA}</label>
          <input
            id="o-ma-bo-phan"
            name="ma"
            value={ban.ma}
            onChange={(e) => datBan({ ...ban, ma: e.target.value })}
            aria-describedby="giai-thich-ma-bo-phan"
          />
          <p className="ghi-chu" id="giai-thich-ma-bo-phan">
            {GIAI_THICH_O_MA_THEM}
          </p>
        </div>
      ) : (
        <p className="ghi-chu">{giaiThichMaKhongSua(dangMo.bp.code)}</p>
      )}

      <div className="o-nhap">
        <label htmlFor="o-cha-bo-phan">{O_CHA}</label>
        <select
          id="o-cha-bo-phan"
          name="chaId"
          value={ban.chaId}
          onChange={(e) => datBan({ ...ban, chaId: e.target.value })}
          aria-describedby={dangMo.kieu === "sua" ? "giai-thich-cha-bo-phan" : undefined}
        >
          <option value="">{CHON_KHONG_CO_CHA}</option>
          {luaChon.map((d) => (
            <option key={d.bp.id} value={d.bp.id}>
              {/* Thụt bằng khoảng trắng không ngắt: ô chọn không nhận lề, và cấp phải thấy được. */}
              {"   ".repeat(d.cap)}
              {d.bp.name}
            </option>
          ))}
        </select>
        {dangMo.kieu === "sua" && (
          <p className="ghi-chu" id="giai-thich-cha-bo-phan">
            {GIAI_THICH_O_CHA_SUA}
          </p>
        )}
      </div>

      <div className="o-nhap">
        <label htmlFor="o-thu-tu-bo-phan">{O_THU_TU}</label>
        {/* `inputMode="numeric"` chứ không `type="number"`: cuộn chuột trên ô số đổi giá trị mà
            người dùng không biết. */}
        <input
          id="o-thu-tu-bo-phan"
          name="thuTu"
          inputMode="numeric"
          value={ban.thuTu}
          onChange={(e) => datBan({ ...ban, thuTu: e.target.value })}
          aria-invalid={loiTaiCho === LOI_THU_TU}
          aria-describedby="giai-thich-thu-tu-bo-phan"
        />
        <p className="ghi-chu" id="giai-thich-thu-tu-bo-phan">
          {GIAI_THICH_O_THU_TU}
        </p>
      </div>

      {/* HAI VÙNG LỖI RIÊNG: lỗi tại chỗ (chưa gửi gì) và câu của máy chủ, nguyên văn. */}
      {loiTaiCho !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiTaiCho}
        </p>
      )}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiMayChu}
        </p>
      )}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={dangGui}>
          {NUT_LUU}
        </button>
        <button type="button" className="nut-phu" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </button>
      </div>
    </form>
  );
}
