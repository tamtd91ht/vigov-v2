/**
 * APP RIÊNG CỦA MỘT XÃ — toàn bộ giao diện sau khi mở (`deploy.mjs --domain=<x> --vao-thang`, ADR 0047 §6).
 *
 * Mong muốn của chủ dự án (28/09/2026): chọn app của xã trên Zalo là thấy NGAY giao diện của xã ấy; xã
 * sau y hệt, không sửa mã. GIAO DIỆN theo bản mẫu `vi-gov/zalo-miniapp`, ĐỦ CÁC MÀN (chủ dự án: "làm đủ
 * các màn như bản mẫu đi"): định danh, trang chủ, phản ánh, tin tức, cá nhân, gửi phản ánh, tra cứu hồ
 * sơ, danh bạ, truyền thanh, video, bản đồ, thông báo.
 *
 * DỮ LIỆU:
 *   · thật — tên xã (`/communes`), tin tức (`/commune-news`), danh bạ (`/commune-staff`), đều công khai
 *     theo tên miền, không đăng nhập, không qua `vihat-miniapp`;
 *   · trải nghiệm — người dùng giả lập và phiếu phản ánh CHỈ TRONG BỘ NHỚ (`trai-nghiem.ts`), gắn nhãn,
 *     KHÔNG gửi vào hệ thống của xã;
 *   · chưa có — truyền thanh, video, bản đồ, thông báo, tra cứu hồ sơ: trạng thái trống bằng lời.
 *
 * KHÔNG LẤY TỪ BẢN MẪU: router, `localStorage`, tên xã từ biến môi trường, lớp gọi máy chủ của nó, OTP,
 * quét căn cước, bước chọn lĩnh vực (ADR 0028), số ngày cam kết viết cứng (luật 10).
 *
 * KHÔNG MỞ PHIÊN LÚC MỞ APP (ADR 0047 §6). Tên miền chỉ là KHOÁ TRA, không vẽ ra, không ghi log.
 */
import { useEffect, useState } from "react";

import { type KetQuaCongKhai, traXaTheoTenMien } from "../api/goi-vigov";
import type { XaTraDuoc } from "../api/hop-dong-cong-khai";

import { BieuTuong, type TenBieuTuong } from "./BieuTuong";
import { ThanDanhBaXa, useDanhBaXa } from "./DanhBaXa";
import { DauKhoi, DauManCon, KhoiTrangThai, OBieuTuong, TrangCon } from "./khung-xa";
import { APP_RIENG, CUA_TOI, DANH_BA, TIN_XA, XA_GIAO_DIEN, XA_TN } from "./noi-dung";
import { ChiTietPhieuTN, DanhSachPhieuTN, GuiPhanAnhTN, NhanTraiNghiem, ThePhieuTN, TraCuuPhieuTN } from "./PhanAnhAppXa";
import { CaNhanXa, type CoChu, DinhDanhXa, ManChuaCoDuLieu, TraCuuHoSoXa } from "./TienIchAppXa";
import { BaiTinXa, DanhSachTinXa, HangTin, useTinXa } from "./TinTucAppXa";
import type { PhieuCuaToi } from "../api/hop-dong-phan-anh";
import { type LayMaViTri, type LayNguoiDung, layNguoiDungGiaLap, loiChao, type NguoiDungApp } from "./trai-nghiem";

export type XaCuaApp = { readonly ten: string; readonly tinh: string };

/** Kết quả tra xã → xã của app, hoặc câu báo lỗi. THUẦN. Đúng MỘT xã, tên không rỗng. */
export function xaTuKetQuaTra(
  kq: KetQuaCongKhai<readonly XaTraDuoc[]>,
): { readonly xa: XaCuaApp } | { readonly loi: string } {
  if (kq.kieu === "xong" && kq.gia_tri.length === 1 && kq.gia_tri[0]!.ten.trim() !== "") {
    return { xa: kq.gia_tri[0]! };
  }
  if (kq.kieu === "xong" || kq.kieu === "khong-hop-le" || kq.kieu === "khong-thay") {
    return { loi: APP_RIENG.khong_thay };
  }
  return { loi: APP_RIENG.chua_ket_noi };
}

/* ═════════════════════════════════ ĐIỀU HƯỚNG — không router ═════════════════════════════════ */

export type TabXa = "trang-chu" | "phan-anh" | "tin-tuc" | "ca-nhan";

type ManXa =
  | { readonly kieu: "tab"; readonly tab: TabXa }
  | { readonly kieu: "gui" }
  | { readonly kieu: "phieu"; readonly ma: string; readonly tu: TabXa }
  | { readonly kieu: "bai"; readonly id: string; readonly tu: TabXa }
  | { readonly kieu: "danh-ba" }
  | { readonly kieu: "tra-cuu" }
  | { readonly kieu: "tra-cuu-phieu" }
  | { readonly kieu: "truyen-thanh" }
  | { readonly kieu: "video" }
  | { readonly kieu: "ban-do" }
  | { readonly kieu: "thong-bao" };

const TAB: ReadonlyArray<{ tab: TabXa; nhan: string; bieu_tuong: TenBieuTuong }> = [
  { tab: "trang-chu", nhan: XA_GIAO_DIEN.tab_trang_chu, bieu_tuong: "home" },
  { tab: "phan-anh", nhan: XA_GIAO_DIEN.tab_phan_anh, bieu_tuong: "chat" },
  { tab: "tin-tuc", nhan: XA_GIAO_DIEN.tab_tin_tuc, bieu_tuong: "news" },
  { tab: "ca-nhan", nhan: XA_TN.tab_ca_nhan, bieu_tuong: "user" },
];

/** Thanh tab dưới: hai tab — chỗ trống cho nút nổi — hai tab. */
function ThanhTabXa({ tab, onChon, onGui }: { tab: TabXa; onChon: (t: TabXa) => void; onGui: () => void }) {
  const nut = (t: (typeof TAB)[number]) => (
    <button
      key={t.tab}
      type="button"
      className={`xa-tab__muc${t.tab === tab ? " xa-tab__muc--on" : ""}`}
      aria-current={t.tab === tab ? "page" : undefined}
      onClick={() => onChon(t.tab)}
    >
      <BieuTuong ten={t.bieu_tuong} co={24} />
      <span>{t.nhan}</span>
    </button>
  );
  return (
    <>
      <button type="button" className="xa-noi" onClick={onGui} aria-label={XA_GIAO_DIEN.nut_gui_noi}>
        <BieuTuong ten="megaphone" co={28} />
      </button>
      <nav className="xa-tab" aria-label={XA_GIAO_DIEN.thanh_tab}>
        {TAB.slice(0, 2).map(nut)}
        <span className="xa-tab__cho-noi" aria-hidden="true" />
        {TAB.slice(2).map(nut)}
      </nav>
    </>
  );
}

/* ═════════════════════════════════ TRANG CHỦ ═════════════════════════════════ */

/**
 * LOGO XÃ trên header. TẠM THỜI: `deploy.mjs --vao-thang` chép `scripts/logo-xa/<tên-miền>.png` vào bản
 * dựng thành `./logo-xa.png` (chủ dự án, 28/09/2026); nguồn thật sau này là hồ sơ hiển thị của xã ở
 * service-platform, cấu hình qua platform-admin. Không có tệp thì ô trở về biểu tượng tòa nhà.
 * `alt=""`: tên xã đứng ngay cạnh bằng chữ.
 */
function LogoXa() {
  const [loi, datLoi] = useState(false);
  if (loi) {
    return (
      <span className="xa-hero__dai-dien" aria-hidden="true">
        <BieuTuong ten="build" co={24} />
      </span>
    );
  }
  return <img className="xa-hero__logo" src="./logo-xa.png" alt="" onError={() => datLoi(true)} />;
}

/** Giờ Việt Nam (+07) — không theo múi giờ của máy (cùng lý do `lib/thoi-diem.ts`). */
function gioVN(): number {
  return (new Date().getUTCHours() + 7) % 24;
}

type OMenu = {
  nhan: string;
  bieu_tuong: TenBieuTuong;
  mau: "hong" | "xanh" | "luc" | "cam" | "navy" | "tim";
  man: ManXa;
};

const O_NHANH: readonly OMenu[] = [
  { nhan: XA_GIAO_DIEN.o_gui, bieu_tuong: "megaphone", mau: "hong", man: { kieu: "gui" } },
  { nhan: XA_TN.o_tra_cuu_ho_so, bieu_tuong: "search", mau: "xanh", man: { kieu: "tra-cuu" } },
  { nhan: XA_TN.o_truyen_thanh, bieu_tuong: "radio", mau: "cam", man: { kieu: "truyen-thanh" } },
  { nhan: XA_TN.o_video, bieu_tuong: "play", mau: "tim", man: { kieu: "video" } },
  { nhan: XA_GIAO_DIEN.o_danh_ba, bieu_tuong: "phone", mau: "luc", man: { kieu: "danh-ba" } },
  { nhan: XA_TN.o_ban_do, bieu_tuong: "map", mau: "navy", man: { kieu: "ban-do" } },
];

function TrangChuXa(props: {
  xa: XaCuaApp;
  nguoi_dung: NguoiDungApp;
  tin: ReturnType<typeof useTinXa>;
  phieu: readonly PhieuCuaToi[];
  di: (m: ManXa) => void;
}) {
  const { xa, tin, di, nguoi_dung } = props;
  const tin_moi = tin.ds.muc.slice(0, 3);
  const moi_nhat = props.phieu[0];

  return (
    <div className="xa-trang">
      <header className="xa-hero">
        <div className="xa-hero__hang">
          <LogoXa />
          <div className="xa-hero__chu">
            <p className="xa-hero__chao">{loiChao(gioVN())}</p>
            <p className="xa-hero__nguoi">{nguoi_dung.ho_ten}</p>
          </div>
          <button
            type="button"
            className="xa-hero__chuong"
            onClick={() => di({ kieu: "thong-bao" })}
            aria-label={XA_TN.thong_bao}
          >
            <BieuTuong ten="bell" co={22} />
          </button>
        </div>
        <div className="xa-hero__don-vi">
          {nguoi_dung.nguon === "gia-lap" && <NhanTraiNghiem />}
          <h1 className="xa-hero__ten">{xa.ten}</h1>
          {xa.tinh !== "" && <p className="xa-hero__tinh">{xa.tinh}</p>}
        </div>
      </header>

      <div className="xa-trang__than">
        <div className="xa-luoi">
          {O_NHANH.map((o) => (
            <button key={o.nhan} type="button" className="xa-the xa-o-nhanh" onClick={() => di(o.man)}>
              <OBieuTuong ten={o.bieu_tuong} mau={o.mau} />
              <span className="xa-o-nhanh__nhan">{o.nhan}</span>
            </button>
          ))}
        </div>

        <DauKhoi tieu_de={XA_GIAO_DIEN.muc_phan_anh} onXemTatCa={() => di({ kieu: "tab", tab: "phan-anh" })} />
        {moi_nhat ? (
          <ThePhieuTN phieu={moi_nhat} onMo={() => di({ kieu: "phieu", ma: moi_nhat.ma_tra_cuu, tu: "trang-chu" })} />
        ) : (
          <div className="xa-the">
            <KhoiTrangThai bieu_tuong="chat" cau={XA_TN.chua_co_phieu} />
          </div>
        )}

        <DauKhoi tieu_de={XA_GIAO_DIEN.muc_tin_moi} onXemTatCa={() => di({ kieu: "tab", tab: "tin-tuc" })} />
        {!tin.ds.da_co_trang_dau && tin.ds.dang_tai ? (
          <KhoiTrangThai bieu_tuong="news" cau={TIN_XA.dang_tai} dang_tai />
        ) : !tin.ds.da_co_trang_dau && tin.ds.loi !== null ? (
          <KhoiTrangThai
            bieu_tuong="alert"
            loi
            cau={TIN_XA.loi_may_chu}
            nut={{ nhan: TIN_XA.nut_thu_lai, onBam: tin.taiTiep }}
          />
        ) : tin_moi.length === 0 ? (
          <KhoiTrangThai bieu_tuong="news" cau={XA_GIAO_DIEN.tin_moi_trong} />
        ) : (
          <ul className="xa-ds">
            {tin_moi.map((t) => (
              <li key={t.id}>
                <HangTin tin={t} onMo={(id) => di({ kieu: "bai", id, tu: "trang-chu" })} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

/* ═════════════════════════════════ CÁC TAB KHÁC ═════════════════════════════════ */

function DauTab({ tieu_de, nhan_tn }: { tieu_de: string; nhan_tn?: boolean }) {
  return (
    <div className="xa-dau-tab">
      <h1 className="xa-dau-con__tieu-de">{tieu_de}</h1>
      {nhan_tn && <NhanTraiNghiem />}
    </div>
  );
}

function ManDanhBa({ ten_mien, onQuayLai }: { ten_mien: string; onQuayLai: () => void }) {
  const { trang, taiLai } = useDanhBaXa(ten_mien);
  return (
    <>
      <DauManCon tieu_de={DANH_BA.tieu_de} onQuayLai={onQuayLai} />
      <TrangCon>
        <ThanDanhBaXa trang={trang} onTai={taiLai} />
      </TrangCon>
    </>
  );
}

/* ═════════════════════════════════ APP ═════════════════════════════════ */

function AppCuaXa(props: {
  ten_mien: string;
  xa: XaCuaApp;
  lay_nguoi_dung: LayNguoiDung;
  lay_ma_vi_tri?: LayMaViTri;
}) {
  const { ten_mien, xa } = props;
  // NGƯỜI DÙNG VÀ PHIẾU CHỈ TRONG BỘ NHỚ (`trai-nghiem.ts`): đóng app là mất, không ghi xuống máy.
  const [nguoi_dung, datNguoiDung] = useState<NguoiDungApp | null>(null);
  const [phieu, datPhieu] = useState<readonly PhieuCuaToi[]>([]);
  const [co_chu, datCoChu] = useState<CoChu>("vua");
  const [man, datMan] = useState<ManXa>({ kieu: "tab", tab: "trang-chu" });
  const tin = useTinXa(ten_mien);
  const veTab = (tab: TabXa) => datMan({ kieu: "tab", tab });
  const lop = `xa-app xa-co-chu--${co_chu}`;

  if (nguoi_dung === null) {
    return (
      <div className={lop}>
        <DinhDanhXa ten_xa={xa.ten} tinh={xa.tinh} lay={props.lay_nguoi_dung} onXong={datNguoiDung} />
      </div>
    );
  }

  const ve = () => veTab("trang-chu");
  let man_con = null;
  switch (man.kieu) {
    case "bai":
      man_con = (
        <BaiTinXa
          key={man.id}
          ten_mien={ten_mien}
          id={man.id}
          ds={tin.ds.muc}
          onMo={(id) => datMan({ kieu: "bai", id, tu: man.tu })}
          onQuayLai={() => veTab(man.tu)}
        />
      );
      break;
    case "gui":
      man_con = (
        <GuiPhanAnhTN
          ten_xa={xa.ten}
          nguoi_dung={nguoi_dung}
          lay_ma_vi_tri={props.lay_ma_vi_tri}
          onQuayLai={ve}
          onDaGui={(p) => datPhieu((ds) => [p, ...ds])}
          onXemPhieu={(ma) => datMan({ kieu: "phieu", ma, tu: "phan-anh" })}
        />
      );
      break;
    case "phieu":
      man_con = (
        <ChiTietPhieuTN phieu={phieu.find((p) => p.ma_tra_cuu === man.ma) ?? null} onQuayLai={() => veTab(man.tu)} />
      );
      break;
    case "danh-ba":
      man_con = <ManDanhBa ten_mien={ten_mien} onQuayLai={ve} />;
      break;
    case "tra-cuu":
      man_con = <TraCuuHoSoXa onQuayLai={ve} />;
      break;
    case "tra-cuu-phieu":
      man_con = <TraCuuPhieuTN phieu={phieu} onQuayLai={() => veTab("phan-anh")} />;
      break;
    case "truyen-thanh":
      man_con = (
        <ManChuaCoDuLieu tieu_de={XA_TN.truyen_thanh_tieu_de} bieu_tuong="radio" cau={XA_TN.truyen_thanh_trong} onQuayLai={ve} />
      );
      break;
    case "video":
      man_con = <ManChuaCoDuLieu tieu_de={XA_TN.video_tieu_de} bieu_tuong="play" cau={XA_TN.video_trong} onQuayLai={ve} />;
      break;
    case "ban-do":
      man_con = <ManChuaCoDuLieu tieu_de={XA_TN.ban_do_tieu_de} bieu_tuong="map" cau={XA_TN.ban_do_trong} onQuayLai={ve} />;
      break;
    case "thong-bao":
      man_con = <ManChuaCoDuLieu tieu_de={XA_TN.thong_bao} bieu_tuong="bell" cau={XA_TN.thong_bao_trong} onQuayLai={ve} />;
      break;
    default:
      break;
  }
  if (man_con !== null) return <div className={lop}>{man_con}</div>;

  const tab = man.kieu === "tab" ? man.tab : "trang-chu";
  let than;
  if (tab === "trang-chu") {
    than = <TrangChuXa xa={xa} nguoi_dung={nguoi_dung} tin={tin} phieu={phieu} di={datMan} />;
  } else if (tab === "tin-tuc") {
    than = (
      <>
        <DauTab tieu_de={TIN_XA.tieu_de} />
        <div className="xa-trang xa-trang--tab">
          <DanhSachTinXa ds={tin.ds} onMo={(id) => datMan({ kieu: "bai", id, tu: "tin-tuc" })} onTai={tin.taiTiep} />
        </div>
      </>
    );
  } else if (tab === "phan-anh") {
    than = (
      <>
        <DauTab tieu_de={CUA_TOI.tieu_de} nhan_tn={nguoi_dung.nguon === "gia-lap"} />
        <div className="xa-trang xa-trang--tab">
          <DanhSachPhieuTN
            phieu={phieu}
            onMo={(ma) => datMan({ kieu: "phieu", ma, tu: "phan-anh" })}
            onTraCuu={() => datMan({ kieu: "tra-cuu-phieu" })}
          />
        </div>
      </>
    );
  } else {
    than = (
      <>
        <DauTab tieu_de={XA_TN.ca_nhan_tieu_de} nhan_tn={nguoi_dung.nguon === "gia-lap"} />
        <CaNhanXa
          nguoi_dung={nguoi_dung}
          ten_xa={xa.ten}
          tinh={xa.tinh}
          so_phieu={phieu.length}
          co_chu={co_chu}
          onDoiCoChu={datCoChu}
          onMoPhanAnh={() => veTab("phan-anh")}
          onMoTraCuu={() => datMan({ kieu: "tra-cuu" })}
          onDangXuat={() => {
            datNguoiDung(null);
            datPhieu([]);
            veTab("trang-chu");
          }}
        />
      </>
    );
  }

  return (
    <div className={lop}>
      <main id="main">{than}</main>
      <ThanhTabXa tab={tab} onChon={veTab} onGui={() => datMan({ kieu: "gui" })} />
    </div>
  );
}

type TrangTra =
  | { readonly kieu: "dang-tra" }
  | { readonly kieu: "loi"; readonly cau: string }
  | { readonly kieu: "xong"; readonly xa: XaCuaApp };

export function TrangXa(props: {
  /** Tên miền xã nung vào bản dựng (`lib/xa-co-dinh.ts`). */
  ten_mien: string;
  /**
   * Hành động DUY NHẤT lấy người dùng (`trai-nghiem.ts`). Mặc định: giả lập cố định. Ngày có quyền Zalo,
   * lớp vỏ tiêm hàm thật vào đây — nửa này không nhập zmp-sdk (`ranh-gioi-hai-nua.test.ts` §3a).
   */
  lay_nguoi_dung?: LayNguoiDung;
  /**
   * Lấy MÃ vị trí (`getLocation`), do lớp vỏ tiêm — nửa này không nhập zmp-sdk. Không truyền thì màn gửi
   * phản ánh không có nút vị trí (chạy thử ngoài Zalo, test).
   */
  lay_ma_vi_tri?: LayMaViTri;
}) {
  const { ten_mien, lay_nguoi_dung = layNguoiDungGiaLap, lay_ma_vi_tri } = props;
  const [trang, datTrang] = useState<TrangTra>({ kieu: "dang-tra" });
  /** Mỗi lần bấm "Thử lại" tăng một — hiệu ứng tra chạy lại đúng một lần cho mỗi giá trị. */
  const [lan, datLan] = useState(0);

  useEffect(() => {
    let con_song = true;
    void traXaTheoTenMien(ten_mien).then((kq) => {
      if (!con_song) return;
      const kq_xa = xaTuKetQuaTra(kq);
      datTrang("xa" in kq_xa ? { kieu: "xong", xa: kq_xa.xa } : { kieu: "loi", cau: kq_xa.loi });
    });
    return () => {
      con_song = false;
    };
    // Tên miền là hằng của bản dựng; chỉ `lan` đổi.
  }, [lan]);

  if (trang.kieu === "xong") {
    return <AppCuaXa ten_mien={ten_mien} xa={trang.xa} lay_nguoi_dung={lay_nguoi_dung} lay_ma_vi_tri={lay_ma_vi_tri} />;
  }

  return (
    <div className="xa-app">
      <div className="xa-trang xa-trang--con">
        {trang.kieu === "dang-tra" ? (
          <KhoiTrangThai bieu_tuong="build" cau={APP_RIENG.dang_mo} dang_tai />
        ) : (
          <KhoiTrangThai
            bieu_tuong="alert"
            loi
            cau={trang.cau}
            nut={{
              nhan: APP_RIENG.thu_lai,
              onBam: () => {
                datTrang({ kieu: "dang-tra" });
                datLan((n) => n + 1);
              },
            }}
          />
        )}
      </div>
    </div>
  );
}
