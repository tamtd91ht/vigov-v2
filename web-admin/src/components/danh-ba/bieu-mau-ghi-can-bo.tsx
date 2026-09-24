"use client";

import type { identity_canBoTomTat } from "@/lib/api/schema.gen";

import {
  CHON_KHONG_BO_PHAN,
  CHON_KHONG_VAI_TRO,
  GIAI_THICH_THEM,
  MO_TA_CO_ZALO,
  NUT_HUY,
  NUT_LUU,
  NUT_XAC_NHAN_KHOA,
  NUT_XAC_NHAN_MO_KHOA,
  O_BO_PHAN,
  O_CHUC_DANH,
  O_CO_ZALO,
  O_DI_DONG,
  O_EMAIL,
  O_HO_TEN,
  O_MAY_BAN,
  O_VAI_TRO,
  canhBaoKhoa,
  tieuDeKhoa,
  tieuDeSua,
  tieuDeThem,
  tieuDeVaiTro,
  type BanNhapCanBo,
} from "./nhan-ghi-danh-ba";

/**
 * Bốn biểu mẫu ghi của danh bạ cán bộ, trong MỘT component thuần trình bày.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BỐN BIỂU MẪU VÌ MÁY CHỦ CÓ NĂM TUYẾN, KHÔNG PHẢI VÌ GIAO DIỆN THÍCH THẾ. Đặc tả vẽ MỘT hộp
 * thoại mang cả họ tên, bộ phận, vai trò và trạng thái (`docs/ui-ux/14-cau-hinh.md §3`); màn hình
 * này cố ý KHÔNG dựng lại hộp thoại ấy, vì gộp bốn việc vào một lần Lưu là gộp bốn quy tắc khác
 * nhau vào một lần từ chối:
 *
 *   thêm       không đụng thẩm quyền của ai
 *   sửa hồ sơ  không đụng thẩm quyền của ai
 *   đổi vai trò mang HAI ràng buộc của #14 và đường thứ ba của #13
 *   khoá/mở    mang phép chặn "người quản trị cuối cùng" của #13
 *
 * Một hộp thoại gộp thì cán bộ sửa số điện thoại cũng nhận câu "Xã phải luôn còn ít nhất một
 * người quản trị" — và không hiểu vì sao, bởi họ có đụng vai trò đâu.
 *
 * THUẦN TRÌNH BÀY: mọi giá trị vào qua `ban`/`vaiTroID`, mọi thay đổi ra qua `datBan`/
 * `datVaiTroID`, mọi lời gọi mạng nằm ở chỗ gọi. Tách như vậy để hai nhánh KHÔNG ai nhìn thấy
 * trong lúc phát triển — "máy chủ vừa từ chối" và "biểu mẫu đang gửi" — kết xuất được bằng
 * `react-dom/server` mà không cần trình duyệt giả lập.
 *
 * KHÔNG CÓ NHÁNH NÀO CHO XOÁ: xoá dòng trùng mang quyền riêng và có hộp riêng ở màn `/danh-ba`
 * (`features/danh-ba/hop-xoa.tsx`). Xem `VI_SAO_KHONG_CO_NUT_XOA` trong `nhan-ghi-danh-ba.ts`.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

/** Một mục của ô chọn — hai danh mục của xã đều chỉ cần đúng hai trường này. */
export type MucChon = { readonly id: string; readonly name: string };

export type DangMoGhi =
  | { kieu: "them"; khoaChongTrung: string }
  | { kieu: "sua"; canBo: identity_canBoTomTat }
  | { kieu: "vaiTro"; canBo: identity_canBoTomTat }
  | { kieu: "khoa"; canBo: identity_canBoTomTat; khoa: boolean };

export function BieuMauGhiCanBo({
  dangMo,
  ban,
  datBan,
  vaiTroID,
  datVaiTroID,
  boPhan,
  vaiTro,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  dangMo: DangMoGhi;
  ban: BanNhapCanBo;
  datBan: (b: BanNhapCanBo) => void;
  vaiTroID: string;
  datVaiTroID: (id: string) => void;
  boPhan: readonly MucChon[];
  vaiTro: readonly MucChon[];
  loiMayChu: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const tieuDe = tieuDeCuaBieuMau(dangMo);
  const coOHoSo = dangMo.kieu === "them" || dangMo.kieu === "sua";

  return (
    <form
      className="form-danh-muc"
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <h4>{tieuDe}</h4>

      {dangMo.kieu === "them" && <p className="ghi-chu">{GIAI_THICH_THEM}</p>}

      {/* MÃ HIỆN RA Ở BIỂU MẪU SỬA, NHƯNG KHÔNG PHẢI MỘT Ô NHẬP — #15. Là chữ, không có `input`,
          không có `name`, nên không có đường nào cho nó đi vào thân yêu cầu. Người sửa hồ sơ cần
          nhìn thấy mã để đối chiếu với hồ sơ giấy; người NHẬP thì không được có ô ấy. */}
      {dangMo.kieu === "sua" && (
        <p className="ghi-chu">Mã cán bộ: {dangMo.canBo.code} — do hệ thống cấp, không sửa được.</p>
      )}

      {coOHoSo && (
        <>
          <ONhap
            id="o-ho-ten-can-bo"
            nhan={O_HO_TEN}
            giaTri={ban.hoTen}
            doi={(v) => datBan({ ...ban, hoTen: v })}
            batBuoc
          />
          <ONhap
            id="o-chuc-danh-can-bo"
            nhan={O_CHUC_DANH}
            giaTri={ban.chucDanh}
            doi={(v) => datBan({ ...ban, chucDanh: v })}
          />
          {/* `type="email"` KHÔNG dùng: bộ kiểm của trình duyệt chặt hơn bộ kiểm của máy chủ ở vài
              ca và lỏng hơn ở vài ca khác, nên nó sinh ra một lớp từ chối thứ hai mà không ai đọc
              được lý do. Máy chủ hạ chữ thường và kiểm hình dạng, kèm câu nói rõ phải sửa gì. */}
          <ONhap
            id="o-email-can-bo"
            nhan={O_EMAIL}
            giaTri={ban.email}
            doi={(v) => datBan({ ...ban, email: v })}
            batBuoc
          />

          <OChon
            id="o-bo-phan-can-bo"
            nhan={O_BO_PHAN}
            nhanRong={CHON_KHONG_BO_PHAN}
            giaTri={ban.boPhanID}
            muc={boPhan}
            doi={(v) => datBan({ ...ban, boPhanID: v })}
          />

          {/* HAI Ô RIÊNG, HAI NHÃN NÓI RÕ LOẠI SỐ — #16. Gộp lại thành một ô "Điện thoại" là chỗ
              người đang gõ không biết mình vừa đặt một số di động cá nhân vào cột công vụ, và mọi
              luật che / xuất Excel / công khai về sau áp sai mức cho cả hai. */}
          <ONhap
            id="o-may-ban-can-bo"
            nhan={O_MAY_BAN}
            giaTri={ban.mayBanCoQuan}
            doi={(v) => datBan({ ...ban, mayBanCoQuan: v })}
            moTa="Số máy bàn của cơ quan. Đây là thông tin công vụ."
          />
          <ONhap
            id="o-di-dong-can-bo"
            nhan={O_DI_DONG}
            giaTri={ban.diDongCaNhan}
            doi={(v) => datBan({ ...ban, diDongCaNhan: v })}
            moTa="Số di động cá nhân. Đây là dữ liệu cá nhân theo Nghị định 13/2023/NĐ-CP."
          />
        </>
      )}

      {/* "CÓ ZALO" CHỈ Ở BIỂU MẪU SỬA: thân thêm (`identity_themCanBoVao`) không có trường ấy, nên
          một ô tick ở biểu mẫu thêm là một giá trị người dùng tick rồi mất lặng lẽ. Nó cũng không
          công khai gì — công khai là tuyến riêng, quyền riêng (`MO_TA_CO_ZALO`). */}
      {dangMo.kieu === "sua" && (
        <div className="o-nhap">
          <label htmlFor="o-co-zalo-can-bo">
            <input
              id="o-co-zalo-can-bo"
              name="o-co-zalo-can-bo"
              type="checkbox"
              checked={ban.coZalo}
              aria-describedby="o-co-zalo-can-bo-mo-ta"
              onChange={(e) => datBan({ ...ban, coZalo: e.target.checked })}
            />{" "}
            {O_CO_ZALO}
          </label>
          <p className="ghi-chu" id="o-co-zalo-can-bo-mo-ta">
            {MO_TA_CO_ZALO}
          </p>
        </div>
      )}

      {dangMo.kieu === "vaiTro" && (
        <OChon
          id="o-vai-tro-can-bo"
          nhan={O_VAI_TRO}
          nhanRong={CHON_KHONG_VAI_TRO}
          giaTri={vaiTroID}
          muc={vaiTro}
          doi={datVaiTroID}
          moTa="Vai trò quyết định cán bộ này làm được những việc gì trong hệ thống."
        />
      )}

      {dangMo.kieu === "khoa" && <p className="canh-bao-pham-vi">{canhBaoKhoa(dangMo.khoa)}</p>}

      {/*
        LỖI CỦA MÁY CHỦ HIỆN NGUYÊN VĂN, và đây là chỗ ba quy tắc khách chốt 22/09/2026 thật sự
        tới được người đọc:

          #13 → "Xã phải luôn còn ít nhất một người quản trị…"
          #14 → "Không thao tác được lên chính tài khoản của mình…"
          #14 → "Vai trò này mang quyền mà tài khoản của bạn không có…"

        KHÔNG rẽ nhánh theo `code`, KHÔNG hiện số hiệu HTTP, KHÔNG viết lại câu. Viết lại là dựng
        bản sao thứ hai của một quy tắc nghiệp vụ, và bản sao ấy trôi mà không bài test nào đỏ.
        Việc của chỗ này chỉ là ĐƯA CÂU ẤY RA TRANG, cạnh đúng cái nút vừa bị từ chối.
      */}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiMayChu}
        </p>
      )}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={dangGui}>
          {nhanNutLuu(dangMo)}
        </button>
        <button type="button" className="nut-phu" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </button>
      </div>
    </form>
  );
}

function tieuDeCuaBieuMau(dangMo: DangMoGhi): string {
  switch (dangMo.kieu) {
    case "them":
      return tieuDeThem();
    case "sua":
      return tieuDeSua(dangMo.canBo.full_name);
    case "vaiTro":
      return tieuDeVaiTro(dangMo.canBo.full_name);
    case "khoa":
      return tieuDeKhoa(dangMo.canBo.full_name, dangMo.khoa);
  }
}

/**
 * Nhãn nút Lưu.
 *
 * KHOÁ VÀ MỞ KHOÁ CÓ NHÃN RIÊNG, KHÔNG DÙNG CHUNG CHỮ "Lưu". Đây là hai thao tác có hậu quả —
 * một người mất đường đăng nhập — nên nút phải nói ra việc nó sắp làm, ngay cạnh câu cảnh báo
 * (accessibility-elderly, REQUIRED #7).
 */
function nhanNutLuu(dangMo: DangMoGhi): string {
  if (dangMo.kieu !== "khoa") return NUT_LUU;
  return dangMo.khoa ? NUT_XAC_NHAN_KHOA : NUT_XAC_NHAN_MO_KHOA;
}

/**
 * Một ô nhập chữ. Nhãn NẰM TRÊN ô, không phải chữ mờ bên trong
 * (accessibility-elderly, REQUIRED #4): chữ mờ biến mất ngay khi người ta bắt đầu gõ, đúng lúc
 * người ta cần nó nhất để kiểm lại mình đang điền ô nào.
 */
function ONhap({
  id,
  nhan,
  giaTri,
  doi,
  batBuoc = false,
  moTa,
}: {
  id: string;
  nhan: string;
  giaTri: string;
  doi: (v: string) => void;
  batBuoc?: boolean;
  moTa?: string;
}) {
  const idMoTa = moTa === undefined ? undefined : `${id}-mo-ta`;
  return (
    <div className="o-nhap">
      <label htmlFor={id}>{nhan}</label>
      {/* `required` là lớp NHẮC của trình duyệt, không phải phép kiểm: nó không bắt được một ô
          toàn dấu cách, và tắt được. Phép kiểm thật nằm ở máy chủ, kèm câu nói rõ phải sửa gì
          (`domain.ChuanHoaHoTen`, `domain.ChuanHoaEmail`). */}
      <input
        id={id}
        name={id}
        value={giaTri}
        required={batBuoc}
        aria-describedby={idMoTa}
        onChange={(e) => doi(e.target.value)}
      />
      {moTa !== undefined && (
        <p className="ghi-chu" id={idMoTa}>
          {moTa}
        </p>
      )}
    </div>
  );
}

/**
 * Một ô chọn từ danh mục của xã.
 *
 * MỤC RỖNG LUÔN CÓ MẶT VÀ ĐỨNG ĐẦU: `""` là một giá trị THẬT của hợp đồng — "chưa phân bộ phận",
 * "không giữ vai trò nào" — chứ không phải trạng thái "chưa chọn". Không có mục ấy thì việc GỠ
 * vai trò của một người không làm được từ màn hình, mà đó đúng là việc cần làm khi một cán bộ
 * chuyển sang bộ phận khác.
 *
 * GIÁ TRỊ ĐANG CÓ MÀ DANH MỤC KHÔNG TRA ĐƯỢC THÌ VẪN GIỮ, KÈM MỘT MỤC NÓI RÕ. Ca này có thật:
 * một bộ phận đã bị xoá mềm thì danh mục không trả về nữa, trong khi dòng danh bạ vẫn trỏ tới nó
 * (`features/cau-hinh/tra-danh-muc.ts`). Không có mục bù này thì `value` không khớp option nào,
 * ô chọn hiện TRỐNG, và người dùng đọc ra "người này chưa phân bộ phận" — rồi bấm Lưu và ghi đè
 * một liên kết mà họ không định đụng tới.
 */
function OChon({
  id,
  nhan,
  nhanRong,
  giaTri,
  muc,
  doi,
  moTa,
}: {
  id: string;
  nhan: string;
  nhanRong: string;
  giaTri: string;
  muc: readonly MucChon[];
  doi: (v: string) => void;
  moTa?: string;
}) {
  const idMoTa = moTa === undefined ? undefined : `${id}-mo-ta`;
  const thieu = giaTri !== "" && !muc.some((m) => m.id === giaTri);

  return (
    <div className="o-nhap">
      <label htmlFor={id}>{nhan}</label>
      <select
        id={id}
        name={id}
        value={giaTri}
        aria-describedby={idMoTa}
        onChange={(e) => doi(e.target.value)}
      >
        <option value="">{nhanRong}</option>
        {muc.map((m) => (
          <option key={m.id} value={m.id}>
            {m.name}
          </option>
        ))}
        {thieu && <option value={giaTri}>Giá trị đang lưu — không còn trong danh mục</option>}
      </select>
      {moTa !== undefined && (
        <p className="ghi-chu" id={idMoTa}>
          {moTa}
        </p>
      )}
    </div>
  );
}
