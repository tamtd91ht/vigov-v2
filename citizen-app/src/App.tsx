import { type ReactNode, useEffect, useState } from "react";

import { TabBar } from "./components/TabBar";
import { KenhCongDan, NutVaoKenhCongDan } from "./cong-dan";
import { COMPANY } from "./content/company-profile";
import { NutChatOA } from "./features/company-intro/NutChatOA";
import { NhaCungCapPhien } from "./features/dang-nhap/kho-phien";
import { GoiYXaScreen, phanGiaiGoiY, TIEU_DE_XAC_NHAN_XA, type XaGoiY } from "./features/kham-pha";
import {
  DEFAULT_SCREEN_ID,
  type DiemDen,
  findScreen,
  type ScreenId,
} from "./features/company-intro/screens";
import { cuonToiMoc } from "./lib/cuon-toi";
import { type KetQuaDo, thamSo as thamSoLaunch, thamSoMoApp } from "./lib/launch-params";

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
 *   Four sibling screens with no deep-linkable state do not need history. When phase 2 brings
 *   deep links (`https://zalo.me/s/<APP_ID>/?t=...`), routing becomes a real requirement and
 *   gets decided then, with that requirement in hand.
 */

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
  /** Xã công dân đã xác nhận, hoặc `null`. Xem ghi chú về trạng thái phía client trong `App`. */
  xaDaChon: XaGoiY | null;
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
            <p className="app-header__owner">{props.xaDaChon ? props.xaDaChon.ten : COMPANY.name}</p>
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

export function App() {
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
   * ⚠ XÃ ĐÃ CHỌN LÀ **TRẠNG THÁI GIAO DIỆN PHÍA CLIENT**, KHÔNG PHẢI MỘT PHIÊN.
   *
   * Nó sống trong `useState`: mất khi app đóng, không được lưu xuống máy, không được gửi đi đâu,
   * và **không cấp quyền gì cả**. ADR 0005: xã của phiên do MÁY CHỦ ghi sau khi công dân xác
   * nhận, và cầu phiên công dân ViGov chưa tồn tại (`cong-dan/api/phien-vigov.ts`).
   *
   * Khi tuyến ấy sống, dòng này được thay bằng xã đọc ra từ phiên do máy chủ trả về — không phải
   * được "đồng bộ thêm" với nó. Hai nguồn cho một sự thật thì một trong hai sẽ cũ, và cái cũ là
   * cái đi vào hồ sơ gửi nhầm cơ quan.
   */
  const [xaDaChon, setXaDaChon] = useState<XaGoiY | null>(null);
  const [boQuaKhamPha, setBoQuaKhamPha] = useState(false);
  /**
   * Đang mở kênh công dân (gửi / tra cứu phản ánh). App.tsx KHÔNG nhập client ViGov — chỉ mở màn
   * của nửa nhà nước qua cửa `./cong-dan` (`ranh-gioi-hai-nua.test.ts`).
   */
  const [moKenhCongDan, setMoKenhCongDan] = useState(false);

  const p = thamSoLaunch(thamSo);

  /**
   * LỚP KHÁM PHÁ (ADR 0005 · 0047). Tham số trên QR chỉ DẪN GIAO DIỆN — nó không chọn xã, không mở
   * một app khác, và không được phép làm hai việc đó.
   *
   * ⚠ `null` Ở ĐÂY LÀ FAIL CLOSED, KHÔNG PHẢI CHỖ CÒN DỞ ĐỂ "ĐIỀN TẠM". Tên xã chỉ có một nguồn: máy
   * chủ tra tên miền trên QR (`GET /api/v1/communes?host=`). Lời gọi ấy chưa được nối vào, nên không
   * có xã nào để gợi ý, màn xác nhận không bao giờ hiện, và app mở phần giới thiệu. Không bao giờ
   * dựng một tên xã từ tham số — công dân xác nhận theo TÊN, và một tên đoán ra là hồ sơ gửi nhầm
   * cơ quan.
   */
  const goiY = phanGiaiGoiY(p["src"] ?? "", null);
  const dangKhamPha = !xaDaChon && !boQuaKhamPha && goiY.kieu === "chon-san";

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
    noiDung = <KenhCongDan onDong={() => setMoKenhCongDan(false)} />;
  } else if (dangKhamPha && goiY.kieu === "chon-san") {
    noiDung = (
      <GoiYXaScreen
        xa={goiY.xa}
        nguon={goiY.nguon}
        onXacNhan={() => setXaDaChon(goiY.xa)}
        onKhongPhai={() => setBoQuaKhamPha(true)}
      />
    );
  } else if (xaDaChon && currentId === DEFAULT_SCREEN_ID) {
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
        xaDaChon={xaDaChon}
        khamPha={dangKhamPha}
      >
        {noiDung}
      </KhungApp>
    </NhaCungCapPhien>
  );
}
