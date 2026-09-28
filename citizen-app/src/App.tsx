import { type ReactNode, useEffect, useState } from "react";

import { feedbackDraftStore } from "./commune-app/feedback-draft-store";
import { TabBar } from "./components/TabBar";
import {
  KenhCongDan,
  type KetQuaMoPhien,
  type KetThucXacNhan,
  type LayMaViTri,
  type LayTenZalo,
  type MoPhienViGov,
  NutVaoKenhCongDan,
  TrangXa,
  XacNhanXa,
} from "./cong-dan";
import { COMPANY } from "./content/company-profile";
import { NutChatOA } from "./features/company-intro/NutChatOA";
import { type KetQuaMoPhienQuaCau, moPhienCongDanQuaCau } from "./features/dang-nhap/cau-vigov";
import { layTenZalo, xinTokenViTri } from "./features/tinh-nang/zalo-api";
import { NhaCungCapPhien } from "./features/dang-nhap/kho-phien";
import { TIEU_DE_XAC_NHAN_XA } from "./features/kham-pha";
import {
  DEFAULT_SCREEN_ID,
  type DiemDen,
  findScreen,
  type ScreenId,
} from "./features/company-intro/screens";
import { cuonToiMoc } from "./lib/cuon-toi";
import { type KetQuaDo, thamSoMoApp, thamSoXa } from "./lib/launch-params";
import { XA_CO_DINH } from "./lib/xa-co-dinh";

/**
 * Phase 1 shell: four static screens, no navigation library, no state beyond the current tab.
 *
 * WHY THE HEADER ALWAYS NAMES THE OWNER:
 *
 *   In phase 2 this header is where the COMMUNE name lives, on every screen, because a citizen
 *   who cannot see which commune they are addressing can submit to the wrong one (README
 *   §Non-negotiables #2). The slot was reserved from the first screen so phase 2 would fill it
 *   rather than invent it — and `KhungApp` below is where it now gets filled: a commune has been
 *   chosen, the slot holds its name; no commune yet, it holds the entity that published the app.
 *
 * WHY NOT A ROUTER:
 *
 *   Four sibling screens with no deep-linkable state do not need history. The one deep link the app
 *   reads (`?d=<commune domain>&src=qr|zns`, ADR 0047) opens ONE confirmation step, not a route, so
 *   there is still nothing to route.
 */

/**
 * CẦU NỐI HAI NỬA — CHỖ DUY NHẤT KẾT QUẢ CỦA CLIENT ĐĂNG NHẬP THƯƠNG MẠI ĐỔI SANG KIỂU CỦA NỬA NHÀ NƯỚC.
 *
 * Nửa nhà nước cần mở phiên ViGov sau khi công dân xác nhận xã, nhưng không được nhập `zmp-sdk` hay
 * client của `vihat-miniapp` (`ranh-gioi-hai-nua.test.ts` §3a). Nó chỉ khai KIỂU hàm nó cần
 * (`MoPhienViGov`); lớp vỏ — được nhập cả hai nửa — dựng hàm ấy ở đây và tiêm xuống `XacNhanXa`.
 *
 * Bảng dịch theo VIỆC NGƯỜI DÂN LÀM TIẾP, không theo mã trạng thái:
 *   xong                                        → có phiên
 *   cau-tat · chua-san-sang · chua-khai-host    → `chua-mo`: bấm lại không đổi được gì
 *   ma-het-han · tam-ngung · khong-goi-duoc ·
 *   khong-lay-duoc-ma                           → `thu-lai`
 *   ngoai-zalo                                  → `ngoai-zalo`
 */
export function sangKieuCongDan(kq: KetQuaMoPhienQuaCau): KetQuaMoPhien {
  switch (kq.kieu) {
    case "xong":
      return {
        kieu: "xong",
        token: kq.phien.token,
        ten_xa: kq.phien.ten_xa,
        ten_mien: kq.phien.ten_mien_xa,
      };
    case "cau-tat":
    case "chua-san-sang":
    case "chua-khai-host":
      return { kieu: "chua-mo" };
    case "ngoai-zalo":
      return { kieu: "ngoai-zalo" };
    default:
      return { kieu: "thu-lai" };
  }
}

/** `communeConfirmed` không đọc ở đây: kiểu của nó là hằng `true`, và thân gửi đi cũng ghi hằng ấy. */
const moPhienViGov: MoPhienViGov = async (yc) =>
  sangKieuCongDan(await moPhienCongDanQuaCau(yc.communeHostHint));

/**
 * KHOÁ TRA `?host=` CHO HAI MÀN CÔNG KHAI (tin tức · danh bạ) — hoặc `null`, và hai màn ấy ẩn đi.
 *
 *   1. `communePrimaryHost` của PHIÊN, nếu phiên có và đúng khuôn. Phiên nói thật (ADR 0047 §Trả lời
 *      mục 4): header đọc tên xã của phiên, nên tin tức và danh bạ phải là của CÙNG xã ấy — kể cả trong
 *      ca hiếm phiên mở cho một xã khác `d`. Trên app riêng của một xã (không có `d`) đây là nguồn duy nhất.
 *   2. Không thì `d` công dân vừa xác nhận ở lần mở này — đúng thứ `/communes` đã tra ra tên trên header.
 *   3. Không có cả hai → `null`. Không đoán một xã.
 *
 * Đây KHÔNG phải đường cô lập xã: hai tuyến này công khai, chỉ trả thứ xã đã công bố, và không mang
 * phiên. Nhưng thứ tự vẫn là một quyết định, nên nó có tên và có ca kiểm (`kham-pha.test.tsx`).
 */
export function khoaTraCongKhai(
  ten_mien_phien: string | null,
  ten_mien_da_xac_nhan: string | null,
): string | null {
  if (ten_mien_phien !== null) return ten_mien_phien;
  return ten_mien_da_xac_nhan;
}

/**
 * Xã đã xác nhận ở lần mở này — chỉ trong `useState`, mất khi app đóng (`ranh-gioi-hai-nua.test.ts` §3b).
 *
 *   `ten`       có phiên → `tenantDisplayName` của phiên; không phiên → `name` của `/communes`
 *   `tinh`      chỉ khi KHÔNG phiên: `province` của `/communes`. Phiên không trả tỉnh, và ghép tỉnh của
 *               `/communes` với tên của phiên là ghép hai nguồn có thể nói hai xã khác nhau
 *   `ten_mien`  khoá tra của hai màn công khai (`khoaTraCongKhai`), hoặc `null`
 */
type XaCuaLanMo = { ten: string; tinh: string | null; ten_mien: string | null };

/**
 * Vỏ ứng dụng, THUẦN — nhận mọi thứ qua tham số, không giữ trạng thái nào.
 *
 * Tách ra vì bất di dịch #2 ("đã chọn xã thì tên xã hiện trên MỌI màn hình") chỉ kiểm được khi
 * dựng được cái vỏ ấy với một xã đã chọn và với TỪNG màn hình một. Gói chung trong một component
 * có `useState` thì phép kiểm phải bấm nút, mà bộ test ở đây dựng bằng `react-dom/server` và
 * không có DOM để bấm. Một bất biến không kiểm được là một bất biến sẽ trôi.
 */
export function KhungApp(props: {
  man: ScreenId;
  onChonMan: (id: ScreenId) => void;
  /**
   * Xã đã xác nhận, hoặc `null`. `ten` là tên xã của PHIÊN khi có phiên, tên `/communes` trả khi chưa;
   * `tinh` chỉ có khi chưa có phiên. Xem `XaCuaLanMo`.
   */
  xaDaChon: { readonly ten: string; readonly tinh?: string | null } | null;
  /**
   * Đang ở lớp khám phá: xác nhận xã. Lúc ấy thanh tab BIẾN MẤT.
   *
   * `skills/accessibility-elderly`, yêu cầu #4: mỗi màn một việc. Để thanh tab lại thì công dân
   * bấm nó, không có gì xảy ra (vì chưa có xã), và một nút bấm không phản hồi là cách nhanh
   * nhất để người lớn tuổi kết luận rằng app hỏng và bỏ đi.
   */
  khamPha?: boolean;
  children: ReactNode;
}) {
  const man = findScreen(props.man);

  return (
    <div className="app">
      <header className="app-header">
        <div className="app-header__hang">
          <div className="app-header__ten">
            {/* MỘT Ô, HAI CHỦ SỞ HỮU. Chưa chọn xã thì đây là đơn vị phát hành ứng dụng — thứ
                Zalo đã duyệt. Chọn xã rồi thì đây là xã, trên mọi màn hình, không có ngoại lệ. */}
            <p className="app-header__owner">
              {props.xaDaChon === null
                ? COMPANY.name
                : props.xaDaChon.tinh
                  ? `${props.xaDaChon.ten}, ${props.xaDaChon.tinh}`
                  : props.xaDaChon.ten}
            </p>
            <p className="app-header__screen">
              {props.khamPha ? TIEU_DE_XAC_NHAN_XA : man.headerTitle}
            </p>
          </div>

          {/* KHÔNG CÓ NÚT "ĐỔI XÃ" (ADR 0044 câu 4 · ADR 0047): một phiên, một xã. Người cần xã
              khác quét QR của xã ấy — không có danh mục nào trong app để đổi sang. */}
        </div>
      </header>

      <main className="app-main" id="main">
        {props.children}
      </main>

      {/* NÚT CHAT NỔI BIẾN MẤT CÙNG LÚC VỚI THANH TAB, CÙNG MỘT LÝ DO (xem `khamPha` ở trên):
          màn xác nhận xã là màn mỗi-màn-một-việc, và việc ấy là xác nhận đúng xã. Một nút nổi mời
          trò chuyện với một doanh nghiệp giữa lúc ấy là một đường rẽ sai chỗ. */}
      {!props.khamPha && <NutChatOA />}

      {!props.khamPha && <TabBar current={props.man} onSelect={props.onChonMan} />}
    </div>
  );
}

/**
 * VỊ TRÍ HIỆN TẠI TRONG APP — màn nào, chỗ nào trong màn ấy, và lần điều hướng thứ mấy.
 *
 * `lan` KHÔNG PHẢI MỘT BỘ ĐẾM CHO VUI. Không có nó thì bấm "Văn phòng" hai lần liên tiếp chỉ cuộn
 * một lần: lần thứ hai `man` và `moc` y hệt lần đầu, React không thấy gì đổi, và người bấm thấy
 * một cái nút không phản hồi. Với người lớn tuổi, một nút không phản hồi là kết luận "app hỏng".
 */
type ViTri = { man: ScreenId; moc?: string; lan: number };

/**
 * CHỌN APP LÚC DỰNG — `XA_CO_DINH` là hằng của bản dựng, nên nhánh này không bao giờ đổi giữa hai lần
 * vẽ (và không vi phạm thứ tự hook: mỗi nhánh là một component riêng).
 */
export function App() {
  return XA_CO_DINH !== null ? <AppRieng ten_mien={XA_CO_DINH} /> : <AppChung />;
}

/**
 * APP RIÊNG CỦA MỘT XÃ (`deploy.mjs --domain=<x> --vao-thang`, ADR 0047 §6).
 *
 * Chọn app của xã trên Zalo là thấy NGAY trang của xã ấy (chủ dự án, 28/09/2026). KHÔNG MỘT CHỮ NÀO
 * CỦA ViHAT GROUP: không màn giới thiệu, không thanh tab của phần thương mại, không nút chat OA, không
 * tên đơn vị phát hành trên header. Không bước xác nhận, không đăng nhập lúc mở — `TrangXa` tra tên xã
 * qua tuyến công khai rồi hiện kênh công dân. Kênh là màn gốc: không nút "Quay lại".
 *
 * Toàn bộ giao diện nằm ở nửa nhà nước (`cong-dan/man/TrangXa.tsx`), theo bản mẫu `vi-gov/zalo-miniapp`
 * chủ dự án chọn 28/09/2026; lớp vỏ này chỉ chọn nó.
 */
export function AppRieng({ ten_mien }: { ten_mien: string }) {
  // `feedbackDraftStore` goes to THIS app only (ADR 0050 #7): a separate App ID, a separate origin. The
  // shared app below never receives it — `ranh-gioi-hai-nua.test.ts` §3b reads `AppChung`'s body for it.
  return (
    <TrangXa ten_mien={ten_mien} lay_ten={layTenChoXa} lay_ma_vi_tri={layMaViTri} draftStore={feedbackDraftStore} />
  );
}

/**
 * CẦU VỊ TRÍ CHO APP RIÊNG — lớp vỏ dựng hàm, nửa nhà nước chỉ khai kiểu (`LayMaViTri`), như
 * hàm mở phiên của đường QR. `getLocation` CHỈ trả một token; token BỊ BỎ ở đây, không đi xuống nửa kia và không
 * rời máy (bảng khai `getLocation`: `roi_khoi_may: ""`). Đổi token ra toạ độ cần máy chủ có app secret —
 * chưa có; ngày có, tuyến ấy nhận token tại đây.
 */
/**
 * CẦU HỌ TÊN — `getUserInfo` kèm hộp xin quyền của Zalo. Chỉ tên đi xuống nửa nhà nước; tên rỗng là
 * "không lấy được", không bao giờ một chuỗi rỗng giả làm tên.
 */
const layTenChoXa: LayTenZalo = async () => {
  const kq = await layTenZalo();
  if (kq.kieu !== "xong") return { kieu: kq.kieu };
  const ho_ten = kq.du_lieu.trim();
  return ho_ten === "" ? { kieu: "khong-lay-duoc" } : { kieu: "xong", ho_ten };
};

const layMaViTri: LayMaViTri = async () => {
  const kq = await xinTokenViTri();
  return kq.kieu === "xong" ? "da-nhan-ma" : kq.kieu;
};

function AppChung() {
  const [vi_tri, datViTri] = useState<ViTri>({ man: DEFAULT_SCREEN_ID, lan: 0 });
  const currentId = vi_tri.man;
  const screen = findScreen(currentId);
  const Screen = screen.component;

  /** Đi tới một chỗ khác trong app. Đây là chỗ DUY NHẤT trong cả kho cài đặt việc điều hướng. */
  function di(diem: DiemDen) {
    datViTri((truoc) => ({ man: diem.man, moc: diem.moc, lan: truoc.lan + 1 }));
  }

  /**
   * Cuộn tới mỏ neo SAU KHI màn mới đã vẽ xong — đó là lý do việc này nằm trong `useEffect` chứ
   * không nằm trong hàm `di` ở trên: lúc `di` chạy, phần tử mang `id` ấy chưa tồn tại.
   *
   * `moc` là tên một màn CON (`MOC_QUAN_LY_QUYEN`) thì không có gì để cuộn tới, và `cuonToiMoc`
   * không làm gì — màn cha mới là chỗ đọc nó. Xem `features/company-intro/dieu-huong.ts`.
   */
  useEffect(() => {
    cuonToiMoc(vi_tri.moc);
  }, [vi_tri.lan, vi_tri.moc]);

  // Đọc MỘT LẦN, NGAY LÚC DỰNG. `location.search` có sẵn đồng bộ, nên không còn `useEffect` và
  // không còn nhịp nhấp nháy: công dân quét QR thấy thẳng màn xác nhận xã, không thấy màn giới
  // thiệu công ty loé lên trước. Đó là món quà kèm theo của việc bỏ `zmp-sdk` — xem
  // lib/launch-params.ts. Hàm bọc try/catch, nên ngoài trình duyệt nó trả về "không có tham số".
  const [thamSo] = useState<KetQuaDo>(thamSoMoApp);

  /**
   * XÃ ĐÃ XÁC NHẬN Ở LẦN MỞ NÀY — `XaCuaLanMo`. Có hai cách tới đây, cả hai đều qua cú bấm xác nhận:
   *
   *   có phiên   `ten` là `tenantDisplayName` cầu phiên trả (ADR 0047 §Trả lời mục 4: "phiên nói
   *              thật"), nên header và bước xác nhận cuối trước khi gửi nói CÙNG một xã với xã máy chủ
   *              sẽ ghi phiếu.
   *   không phiên (27/09/2026, quyết định của chủ sản phẩm) `ten` + `tinh` là thứ `/communes` trả cho
   *              `d` và công dân vừa xác nhận. Chỉ tin tức và danh bạ mở được; gửi phản ánh vẫn cần phiên.
   *
   * `ten_mien` chỉ làm khoá tra `?host=`. Nó không phải tham chiếu xã: không được gửi làm "xã của
   * tôi", không lưu, không vẽ ra (ADR 0047 điều kiện dừng #1). Tất cả sống trong `useState`.
   */
  const [xa, datXa] = useState<XaCuaLanMo | null>(null);
  /** Lớp khám phá đã kết thúc (xác nhận xong, "không phải xã này", hoặc fail closed). */
  const [xongKhamPha, datXongKhamPha] = useState(false);
  /** Câu nói vì sao app về phần giới thiệu thay vì mở kênh — hoặc `null`. */
  const [thongBao, datThongBao] = useState<string | null>(null);
  /**
   * Đang mở kênh công dân (gửi / tra cứu phản ánh). App.tsx KHÔNG nhập client ViGov — chỉ mở màn
   * của nửa nhà nước qua cửa `./cong-dan` (`ranh-gioi-hai-nua.test.ts`).
   */
  const [moKenhCongDan, setMoKenhCongDan] = useState(false);

  /**
   * LỚP KHÁM PHÁ (ADR 0005 · 0047). `d` + `src` trên QR chỉ DẪN GIAO DIỆN — không chọn xã, không mở
   * phiên nếu công dân chưa bấm xác nhận. `null` (không có `d`, `src` không tin được, `d` sai khuôn)
   * là mở như không tham số: KHÔNG một lời gọi nào tới `identity`/`comms`.
   */
  const goiY = thamSoXa(thamSo);
  const dangKhamPha = goiY !== null && xa === null && !xongKhamPha;

  function ketThucKhamPha(kq: KetThucXacNhan) {
    datXongKhamPha(true);
    // `goiY === null` không tới được đây (màn xác nhận chỉ dựng khi có nó); nếu có thì KHÔNG đặt xã nào.
    if (kq.kieu === "da-mo" && goiY !== null) {
      datXa({ ten: kq.ten_xa, tinh: null, ten_mien: khoaTraCongKhai(kq.ten_mien, goiY.ten_mien) });
      datThongBao(null);
    } else if (kq.kieu === "xac-nhan-khong-phien" && goiY !== null) {
      datXa({ ten: kq.xa.ten, tinh: kq.xa.tinh, ten_mien: khoaTraCongKhai(null, goiY.ten_mien) });
      datThongBao(null);
    } else if (kq.kieu === "ve-gioi-thieu") {
      datThongBao(kq.cau);
    }
  }

  /**
   * `key` MANG CẢ `moc` LẪN `lan`, VÀ ĐÓ LÀ THỨ LÀM MỤC "QUYỀN" TRÊN MÀN CHỦ CHẠY ĐƯỢC LẦN THỨ HAI.
   *
   * `ContactScreen` đọc `moc` làm GIÁ TRỊ BAN ĐẦU của trạng thái "đang xem màn quyền". Giá trị ban
   * đầu chỉ được đọc một lần cho mỗi lần dựng, nên không có `key` đổi thì: bấm Quyền -> mở, bấm
   * Quay lại -> đóng, bấm Quyền lần nữa -> KHÔNG mở lại, vì màn cha không hề được dựng lại. Một
   * nút chạy đúng một lần rồi im là kiểu hỏng không ai báo, người ta chỉ thôi bấm nó.
   */
  let noiDung: ReactNode = (
    <Screen key={`${currentId}:${vi_tri.moc ?? ""}:${vi_tri.lan}`} moc={vi_tri.moc} onDi={di} />
  );
  if (moKenhCongDan) {
    noiDung = (
      <KenhCongDan onDong={() => setMoKenhCongDan(false)} ten_mien={xa === null ? null : xa.ten_mien} />
    );
  } else if (dangKhamPha && goiY !== null) {
    noiDung = (
      <XacNhanXa
        ten_mien={goiY.ten_mien}
        nguon={goiY.nguon}
        moPhienViGov={moPhienViGov}
        onKetThuc={ketThucKhamPha}
      />
    );
  } else if (thongBao !== null && currentId === DEFAULT_SCREEN_ID) {
    // FAIL CLOSED CÓ LỜI: về phần giới thiệu, và nói bằng một câu vì sao — không mã lỗi, không tên
    // miền. Chỉ trên tab đầu: đó là chỗ công dân vừa được đưa về.
    noiDung = (
      <>
        <p className="cd-loi" role="status">
          {thongBao}
        </p>
        {noiDung}
      </>
    );
  } else if (xa !== null && currentId === DEFAULT_SCREEN_ID) {
    /**
     * ĐÃ XÁC NHẬN XÃ THÌ TAB ĐẦU MỞ LỐI VÀO KÊNH CÔNG DÂN — đặt TRÊN màn chủ, không thay nó.
     *
     *   Trang xã mẫu (dịch vụ, số trực, giờ làm việc) đã bị xoá cùng danh mục xã mẫu: mọi chữ trên
     *   đó là dữ liệu đặt ra, và một trang xã thật cần nguồn máy chủ chưa có. Nên ở đây chỉ còn
     *   đúng thứ có thật — lối vào kênh — và phần giới thiệu, thứ Zalo đã duyệt, vẫn còn đường tới.
     *   Chỗ đặt nút là quyết định của card nối nguồn xã (TASK-04b), không phải của card này.
     */
    noiDung = (
      <>
        <NutVaoKenhCongDan onBam={() => setMoKenhCongDan(true)} />
        {noiDung}
      </>
    );
  }

  return (
    /**
     * ⚠ CHỖ GIỮ PHIÊN BỌC CẢ VỎ APP, VÀ NÓ CHỈ SỐNG TRONG BỘ NHỚ.
     *
     *   Phiếu phiên được phát hành ở khối đăng nhập trên màn Liên hệ, và được ĐỌC ở hai màn khác
     *   (Tư vấn & báo giá · Yêu cầu của tôi). Cách rẻ nhất để chuyền nó giữa ba chỗ ấy là ghi
     *   xuống máy — và đó chính là dòng `phase1-collects-nothing.test.ts` cấm ở MỌI tệp. Lệnh cấm
     *   ấy KHÔNG được thu hẹp trong lượt này, nên chỗ giữ là một context trên `useState`: đóng app
     *   là mất, mở lại đăng nhập một chạm. Xem `features/dang-nhap/kho-phien.tsx`.
     */
    <NhaCungCapPhien>
      <KhungApp
        man={currentId}
        // Bấm một tab là điều hướng KHÔNG CÓ MỐC: người bấm muốn về đầu màn ấy, không muốn bị thả
        // xuống giữa một khối mà lần trước họ đi tới từ màn chủ.
        onChonMan={(id) => di({ man: id })}
        xaDaChon={xa === null ? null : { ten: xa.ten, tinh: xa.tinh }}
        khamPha={dangKhamPha}
      >
        {noiDung}
      </KhungApp>
    </NhaCungCapPhien>
  );
}
