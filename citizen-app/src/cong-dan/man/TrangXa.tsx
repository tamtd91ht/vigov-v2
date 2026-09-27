/**
 * APP RIÊNG CỦA MỘT XÃ — toàn bộ giao diện sau khi mở (`deploy.mjs --domain=<x> --vao-thang`, ADR 0047 §6).
 *
 * Mong muốn của chủ dự án (28/09/2026): chọn app của xã trên Zalo là thấy NGAY giao diện của xã ấy; xã
 * sau y hệt, không sửa mã. GIAO DIỆN theo bản mẫu `vi-gov/zalo-miniapp` do chủ dự án chỉ:
 * header xanh bo đáy, lưới truy cập nhanh, "Phản ánh của tôi", "Tin tức mới", thanh tab dưới có nút
 * "Gửi phản ánh" nổi ở giữa.
 *
 * Lấy từ bản mẫu: bố cục, màu, hình. KHÔNG lấy: router, `localStorage`, tên xã từ biến môi trường, lớp
 * gọi máy chủ của nó, màn định danh bắt buộc, quét căn cước, bản đồ, truyền thanh, video — những thứ
 * hoặc trái luật của kho này (luật 1 bất biến 10, luật 3, luật 4), hoặc chưa có máy chủ nào trả dữ liệu
 * thật. Ô nào không có dữ liệu thật thì KHÔNG hiện: một nút dẫn tới màn trống là thứ người lớn tuổi
 * bấm một lần rồi kết luận app hỏng.
 *
 * 1. Tra tên xã theo tên miền của bản dựng (`GET identity /api/v1/communes?host=`, công khai).
 * 2. Hiện app của xã. Tin tức, danh bạ đọc ngay — KHÔNG đăng nhập, KHÔNG qua `vihat-miniapp`.
 *
 * KHÔNG MỞ PHIÊN LÚC MỞ APP (ADR 0047 §6). Gửi, xem, tra cứu phản ánh cần phiên; đường đăng nhập theo
 * App ID của app xã CHƯA DỰNG, nên ba chỗ ấy nói thật "chưa đăng nhập được".
 *
 * Tên miền chỉ là KHOÁ TRA, không vẽ ra, không ghi log. Tên xã là thứ máy chủ trả.
 */
import { useEffect, useState } from "react";

import { type KetQuaCongKhai, traXaTheoTenMien } from "../api/goi-vigov";
import type { XaTraDuoc } from "../api/hop-dong-cong-khai";
import { layPhienViGov } from "../api/phien-vigov";

import { GuiPhanAnhScreen } from "./GuiPhanAnhScreen";
import { APP_RIENG, CHUA_DANG_NHAP_XA, CUA_TOI, DANH_BA, GUI, TIN_XA, TRA_CUU, XA_GIAO_DIEN } from "./noi-dung";
import { PhanAnhCuaToiScreen } from "./PhanAnhCuaToiScreen";
import { TraCuuPhieuScreen } from "./TraCuuPhieuScreen";
import { BieuTuong, type TenBieuTuong } from "./BieuTuong";
import { ThanDanhBaXa, useDanhBaXa } from "./DanhBaXa";
import { ChuaDangNhap, DauKhoi, DauManCon, KhoiTrangThai, OBieuTuong, TrangCon } from "./khung-xa";
import { BaiTinXa, DanhSachTinXa, HangTin, useTinXa } from "./TinTucAppXa";

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

export type TabXa = "trang-chu" | "phan-anh" | "tin-tuc" | "danh-ba";

type ManXa =
  | { readonly kieu: "tab"; readonly tab: TabXa }
  | { readonly kieu: "gui" }
  | { readonly kieu: "tra-cuu"; readonly ma?: string }
  | { readonly kieu: "bai"; readonly id: string; readonly tu: TabXa };

const TAB: ReadonlyArray<{ tab: TabXa; nhan: string; bieu_tuong: TenBieuTuong }> = [
  { tab: "trang-chu", nhan: XA_GIAO_DIEN.tab_trang_chu, bieu_tuong: "home" },
  { tab: "phan-anh", nhan: XA_GIAO_DIEN.tab_phan_anh, bieu_tuong: "chat" },
  { tab: "tin-tuc", nhan: XA_GIAO_DIEN.tab_tin_tuc, bieu_tuong: "news" },
  { tab: "danh-ba", nhan: XA_GIAO_DIEN.tab_danh_ba, bieu_tuong: "users" },
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

function TrangChuXa(props: {
  xa: XaCuaApp;
  tin: ReturnType<typeof useTinXa>;
  co_phien: boolean;
  di: (m: ManXa) => void;
}) {
  const { xa, tin, di } = props;
  const o_nhanh: ReadonlyArray<{ nhan: string; bieu_tuong: TenBieuTuong; mau: "hong" | "xanh" | "luc"; man: ManXa }> = [
    { nhan: XA_GIAO_DIEN.o_gui, bieu_tuong: "megaphone", mau: "hong", man: { kieu: "gui" } },
    { nhan: XA_GIAO_DIEN.o_tra_cuu, bieu_tuong: "search", mau: "xanh", man: { kieu: "tra-cuu" } },
    { nhan: XA_GIAO_DIEN.o_danh_ba, bieu_tuong: "phone", mau: "luc", man: { kieu: "tab", tab: "danh-ba" } },
  ];
  const tin_moi = tin.ds.muc.slice(0, 3);

  return (
    <div className="xa-trang">
      <header className="xa-hero">
        <div className="xa-hero__hang">
          <span className="xa-hero__dai-dien" aria-hidden="true">
            <BieuTuong ten="build" co={24} />
          </span>
          <div className="xa-hero__chu">
            <p className="xa-hero__chao">{XA_GIAO_DIEN.chao}</p>
            <h1 className="xa-hero__ten">{xa.ten}</h1>
            {xa.tinh !== "" && <p className="xa-hero__tinh">{xa.tinh}</p>}
          </div>
        </div>
      </header>

      <div className="xa-trang__than">
        <div className="xa-luoi">
          {o_nhanh.map((o) => (
            <button key={o.nhan} type="button" className="xa-the xa-o-nhanh" onClick={() => di(o.man)}>
              <OBieuTuong ten={o.bieu_tuong} mau={o.mau} />
              <span className="xa-o-nhanh__nhan">{o.nhan}</span>
            </button>
          ))}
        </div>

        <DauKhoi tieu_de={XA_GIAO_DIEN.muc_phan_anh} onXemTatCa={() => di({ kieu: "tab", tab: "phan-anh" })} />
        {props.co_phien ? (
          <button type="button" className="xa-the xa-hang-tin" onClick={() => di({ kieu: "tab", tab: "phan-anh" })}>
            <span className="xa-hang-tin__chu">
              <strong className="xa-hang-tin__tieu-de">{CUA_TOI.tieu_de}</strong>
            </span>
            <BieuTuong ten="right" co={20} />
          </button>
        ) : (
          <ChuaDangNhap cau={XA_GIAO_DIEN.chua_dang_nhap_ngan} />
        )}

        <DauKhoi tieu_de={XA_GIAO_DIEN.muc_tin_moi} onXemTatCa={() => di({ kieu: "tab", tab: "tin-tuc" })} />
        {!tin.ds.da_co_trang_dau && tin.ds.dang_tai ? (
          <KhoiTrangThai bieu_tuong="news" cau={TIN_XA.dang_tai} dang_tai />
        ) : !tin.ds.da_co_trang_dau && tin.ds.loi !== null ? (
          <KhoiTrangThai bieu_tuong="alert" loi cau={TIN_XA.loi_may_chu} nut={{ nhan: TIN_XA.nut_thu_lai, onBam: tin.taiTiep }} />
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

function DauTab({ tieu_de }: { tieu_de: string }) {
  return (
    <div className="xa-dau-tab">
      <h1 className="xa-dau-con__tieu-de">{tieu_de}</h1>
    </div>
  );
}

function TabDanhBa({ ten_mien }: { ten_mien: string }) {
  const { trang, taiLai } = useDanhBaXa(ten_mien);
  return (
    <>
      <DauTab tieu_de={DANH_BA.tieu_de} />
      <div className="xa-trang xa-trang--tab">
        <ThanDanhBaXa trang={trang} onTai={taiLai} />
      </div>
    </>
  );
}

/* ═════════════════════════════════ APP ═════════════════════════════════ */

function AppCuaXa({ ten_mien, xa }: { ten_mien: string; xa: XaCuaApp }) {
  const [man, datMan] = useState<ManXa>({ kieu: "tab", tab: "trang-chu" });
  // Đọc MỘT LẦN, cùng cách ba màn phản ánh đọc. App riêng hôm nay không mở phiên nào (ADR 0047 §6).
  const [co_phien] = useState(() => layPhienViGov() !== null);
  const tin = useTinXa(ten_mien);
  const veTab = (tab: TabXa) => datMan({ kieu: "tab", tab });

  if (man.kieu === "bai") {
    return (
      <div className="xa-app">
        <BaiTinXa ten_mien={ten_mien} id={man.id} onQuayLai={() => veTab(man.tu)} />
      </div>
    );
  }

  if (man.kieu === "gui" || man.kieu === "tra-cuu") {
    const tieu_de = man.kieu === "gui" ? GUI.tieu_de : TRA_CUU.tieu_de;
    return (
      <div className="xa-app">
        {co_phien ? (
          man.kieu === "gui" ? (
            <GuiPhanAnhScreen onQuayLai={() => veTab("trang-chu")} />
          ) : (
            <TraCuuPhieuScreen onQuayLai={() => veTab("trang-chu")} ma_ban_dau={man.ma ?? ""} />
          )
        ) : (
          <>
            <DauManCon tieu_de={tieu_de} onQuayLai={() => veTab("trang-chu")} />
            <TrangCon>
              <ChuaDangNhap cau={CHUA_DANG_NHAP_XA.cau} />
            </TrangCon>
          </>
        )}
      </div>
    );
  }

  let than;
  if (man.tab === "trang-chu") {
    than = <TrangChuXa xa={xa} tin={tin} co_phien={co_phien} di={datMan} />;
  } else if (man.tab === "tin-tuc") {
    than = (
      <>
        <DauTab tieu_de={TIN_XA.tieu_de} />
        <div className="xa-trang xa-trang--tab">
          <DanhSachTinXa ds={tin.ds} onMo={(id) => datMan({ kieu: "bai", id, tu: "tin-tuc" })} onTai={tin.taiTiep} />
        </div>
      </>
    );
  } else if (man.tab === "danh-ba") {
    than = <TabDanhBa ten_mien={ten_mien} />;
  } else {
    than = co_phien ? (
      <PhanAnhCuaToiScreen onQuayLai={() => veTab("trang-chu")} onMoPhieu={(ma) => datMan({ kieu: "tra-cuu", ma })}
        onGuiPhanAnh={() => datMan({ kieu: "gui" })}
      />
    ) : (
      <>
        <DauTab tieu_de={CUA_TOI.tieu_de} />
        <div className="xa-trang xa-trang--tab">
          <ChuaDangNhap cau={CHUA_DANG_NHAP_XA.cau} />
        </div>
      </>
    );
  }

  return (
    <div className="xa-app">
      <main id="main">{than}</main>
      <ThanhTabXa tab={man.tab} onChon={veTab} onGui={() => datMan({ kieu: "gui" })} />
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
}) {
  const { ten_mien } = props;
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

  if (trang.kieu === "xong") return <AppCuaXa ten_mien={ten_mien} xa={trang.xa} />;

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
