"use client";

import { Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { controlClass } from "@/components/ui/field";
import { selectCls } from "@/features/cau-hinh/config-ui";
import { DIALOG_FOOTER_CLASS, DIALOG_LABEL_CLASS } from "@/features/cau-hinh/org-unit-dialog-classes";
import type { identity_canBoTomTat } from "@/lib/api/schema.gen";
import { cn } from "@/lib/cn";

import {
  ACCOUNT_EDIT_TITLE,
  ACCOUNT_EMAIL_LABEL,
  ACCOUNT_NO_UNIT_OPTION,
  EMAIL_PLACEHOLDER,
  NAME_PLACEHOLDER,
  NUT_THEM_CAN_BO,
  PHONE_PLACEHOLDER,
  POSITION_PLACEHOLDER,
  CANH_BAO_DOI_DI_DONG_CONG_KHAI,
  CANH_BAO_KHOA_CONG_KHAI,
  CHON_KHONG_BO_PHAN,
  CHON_KHONG_VAI_TRO,
  EMAIL_HINT,
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
  coCanhBaoDoiDiDong,
  coCanhBaoKhoaCongKhai,
  staffCodeNote,
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
 * (`features/danh-ba/hop-xoa.tsx`), dùng ở cả `/danh-ba` lẫn `/nguoi-dung` (từ 08/10/2026), sau
 * `admin.user.delete`; dòng có tài khoản thì khoá chứ không xoá (#10).
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
  layout = "contact",
}: {
  /**
   * `contact` (default): the former in-flow form of /mini-app's Danh bạ tab — UNCHANGED, and pinned by
   * `user-list-prototype.test.tsx`. Since 09/10/2026 that tab edits in its own prototype dialog
   * (`features/danh-ba/staff-contact-form.tsx`) and no screen renders this layout. `account`: the prototype's `UserFormDialog` shape, used ONLY by
   * `/nguoi-dung` (`danh-ba-can-bo.tsx`), which opens it inside a `ModalDialog` (owner, 08/10/2026).
   * Same fields, same handlers, same bodies sent — only the presentation differs.
   */
  layout?: "contact" | "account";
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
  if (layout === "account") {
    return (
      <AccountForm
        dangMo={dangMo}
        ban={ban}
        datBan={datBan}
        vaiTroID={vaiTroID}
        datVaiTroID={datVaiTroID}
        boPhan={boPhan}
        vaiTro={vaiTro}
        loiMayChu={loiMayChu}
        dangGui={dangGui}
        onGui={onGui}
        onHuy={onHuy}
      />
    );
  }

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
        <p className="ghi-chu">{staffCodeNote(dangMo.canBo.code)}</p>
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
              được lý do. Máy chủ hạ chữ thường và kiểm hình dạng, kèm câu nói rõ phải sửa gì.
              NOT `required` since 4cf87b6: the server accepts a staff row without an email. */}
          <ONhap
            id="o-email-can-bo"
            nhan={O_EMAIL}
            giaTri={ban.email}
            doi={(v) => datBan({ ...ban, email: v })}
            moTa={EMAIL_HINT}
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
          {/* Lời báo ngay dưới ô di động, CHỈ khi người này đang công khai và số vừa đổi — quyết
              định 28/09/2026. Máy chủ mới là nơi gỡ khỏi Mini App; sau khi Lưu, chỗ gọi đọc lại
              danh sách nên chip "Trên Mini App" phản ánh đúng điều máy chủ đã làm. */}
          {coCanhBaoDoiDiDong(dangMo, ban) && (
            <p className="canh-bao-pham-vi" role="status">
              {CANH_BAO_DOI_DI_DONG_CONG_KHAI}
            </p>
          )}
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
      {coCanhBaoKhoaCongKhai(dangMo) && <p className="canh-bao-pham-vi">{CANH_BAO_KHOA_CONG_KHAI}</p>}

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

type AccountFormProps = {
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
};

/**
 * The `/nguoi-dung` shape (`vigov-require/apps/admin/src/components/admin/UserFormDialog.tsx`): the
 * profile fields in a two-column grid, shadcn Label + Input `mt-1.5`, the footer band with `Huỷ` FIRST
 * and a spinner in the primary while sending. The prototype's password and role boxes are NOT here:
 * the password is a one-time value the server generates (#9, `Cấp tài khoản` / `Đặt lại mật khẩu`),
 * and the role goes through its own route with #13/#14's checks (`Đổi vai trò`).
 *
 * NO `form-danh-muc` / `cum-nut` / `o-nhap` ON THE PROFILE FIELDS: the legacy sheet gives buttons and
 * labels under those classes a 44px floor and a 12px/600 label, which the prototype's h-8 buttons and
 * 14px labels would lose to. `Đổi vai trò` and `Khoá` keep their bodies as they were.
 *
 * The heading is the dialog's (`ModalDialogHeader`), so there is no `<h4>`; the form keeps its name.
 */
function AccountForm({
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
}: AccountFormProps) {
  const profile = dangMo.kieu === "them" || dangMo.kieu === "sua";
  const formName = dangMo.kieu === "sua" ? ACCOUNT_EDIT_TITLE : tieuDeCuaBieuMau(dangMo);
  const primary = dangMo.kieu === "them" ? NUT_THEM_CAN_BO : nhanNutLuu(dangMo);
  const unitMissing = ban.boPhanID !== "" && !boPhan.some((m) => m.id === ban.boPhanID);

  return (
    <form
      className="m-0 flex min-h-0 min-w-0 flex-col gap-4"
      aria-label={formName}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <div className="flex min-h-0 flex-col gap-3 overflow-y-auto">
        {profile && (
          <div className="grid grid-cols-2 gap-4">
            <div className="col-span-2 block">
              <label htmlFor="o-ho-ten-can-bo" className={DIALOG_LABEL_CLASS}>
                {O_HO_TEN}
              </label>
              <input
                id="o-ho-ten-can-bo"
                name="o-ho-ten-can-bo"
                value={ban.hoTen}
                required
                placeholder={NAME_PLACEHOLDER}
                onChange={(e) => datBan({ ...ban, hoTen: e.target.value })}
                className={cn(controlClass, "mt-1.5")}
              />
            </div>
            <div className="block min-w-0">
              <label htmlFor="o-email-can-bo" className={DIALOG_LABEL_CLASS}>
                {ACCOUNT_EMAIL_LABEL}
              </label>
              <input
                id="o-email-can-bo"
                name="o-email-can-bo"
                type="email"
                value={ban.email}
                placeholder={EMAIL_PLACEHOLDER}
                onChange={(e) => datBan({ ...ban, email: e.target.value })}
                className={cn(controlClass, "mt-1.5")}
              />
            </div>
            <div className="block min-w-0">
              <label htmlFor="o-chuc-danh-can-bo" className={DIALOG_LABEL_CLASS}>
                {O_CHUC_DANH}
              </label>
              <input
                id="o-chuc-danh-can-bo"
                name="o-chuc-danh-can-bo"
                value={ban.chucDanh}
                placeholder={POSITION_PLACEHOLDER}
                onChange={(e) => datBan({ ...ban, chucDanh: e.target.value })}
                className={cn(controlClass, "mt-1.5")}
              />
            </div>
            {/* TWO phone fields, two labels saying which kind (#16): the prototype's single "Điện thoại"
                would put duty information and Decree 13 personal data under one name. */}
            <div className="block min-w-0">
              <label htmlFor="o-may-ban-can-bo" className={DIALOG_LABEL_CLASS}>
                {O_MAY_BAN}
              </label>
              <input
                id="o-may-ban-can-bo"
                name="o-may-ban-can-bo"
                value={ban.mayBanCoQuan}
                placeholder={PHONE_PLACEHOLDER}
                onChange={(e) => datBan({ ...ban, mayBanCoQuan: e.target.value })}
                className={cn(controlClass, "mt-1.5")}
              />
            </div>
            <div className="block min-w-0">
              <label htmlFor="o-di-dong-can-bo" className={DIALOG_LABEL_CLASS}>
                {O_DI_DONG}
              </label>
              <input
                id="o-di-dong-can-bo"
                name="o-di-dong-can-bo"
                value={ban.diDongCaNhan}
                placeholder={PHONE_PLACEHOLDER}
                onChange={(e) => datBan({ ...ban, diDongCaNhan: e.target.value })}
                className={cn(controlClass, "mt-1.5")}
              />
            </div>
            <div className="col-span-2 block">
              <label htmlFor="o-bo-phan-can-bo" className={DIALOG_LABEL_CLASS}>
                {O_BO_PHAN}
              </label>
              <select
                id="o-bo-phan-can-bo"
                name="o-bo-phan-can-bo"
                value={ban.boPhanID}
                onChange={(e) => datBan({ ...ban, boPhanID: e.target.value })}
                className={cn(selectCls, "mt-1.5 h-9 w-full min-w-0 pr-8 text-[13px]")}
              >
                <option value="">{ACCOUNT_NO_UNIT_OPTION}</option>
                {boPhan.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.name}
                  </option>
                ))}
                {/* Same reason as `OChon`: a stored unit the catalogue no longer lists stays selected,
                    so saving a phone number never silently unlinks it. */}
                {unitMissing && <option value={ban.boPhanID}>Giá trị đang lưu — không còn trong danh mục</option>}
              </select>
            </div>
            {dangMo.kieu === "sua" && (
              <div className="col-span-2 block">
                <label htmlFor="o-co-zalo-can-bo" className="m-0 flex cursor-pointer items-center gap-2 text-[12.5px]">
                  <input
                    id="o-co-zalo-can-bo"
                    name="o-co-zalo-can-bo"
                    type="checkbox"
                    checked={ban.coZalo}
                    aria-describedby="o-co-zalo-can-bo-mo-ta"
                    onChange={(e) => datBan({ ...ban, coZalo: e.target.checked })}
                    className="accent-brand m-0 size-3.5"
                  />
                  <span className="text-navy">{O_CO_ZALO}</span>
                </label>
                <p id="o-co-zalo-can-bo-mo-ta" className="text-ink-muted m-0 mt-1 text-[11.5px]">
                  {MO_TA_CO_ZALO}
                </p>
              </div>
            )}
          </div>
        )}

        {coCanhBaoDoiDiDong(dangMo, ban) && (
          <p className="canh-bao-pham-vi m-0" role="status">
            {CANH_BAO_DOI_DI_DONG_CONG_KHAI}
          </p>
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

        {dangMo.kieu === "khoa" && <p className="canh-bao-pham-vi m-0">{canhBaoKhoa(dangMo.khoa)}</p>}
        {coCanhBaoKhoaCongKhai(dangMo) && <p className="canh-bao-pham-vi m-0">{CANH_BAO_KHOA_CONG_KHAI}</p>}

        {/* The server's sentence, verbatim, beside the button it refused (#13, #14) — see above. */}
        {loiMayChu !== "" && (
          <p role="alert" className="text-danger m-0 text-[12px] font-medium">
            {loiMayChu}
          </p>
        )}
      </div>

      <div className={DIALOG_FOOTER_CLASS}>
        <Button type="button" variant="outline" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </Button>
        <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui}>
          {dangGui && <Loader2 aria-hidden="true" focusable="false" className="size-4 animate-spin" />}
          {primary}
        </Button>
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
