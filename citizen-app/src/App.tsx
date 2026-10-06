import { type ReactNode, useEffect, useMemo, useState } from "react";

import { feedbackDraftStore, sharedAppDraftStore } from "./commune-app/feedback-draft-store";
import { TabBar } from "./components/TabBar";
import {
  type CommuneAppSessionResult,
  type GetSceneLocation,
  type KetQuaMoPhien,
  type LayTenZalo,
  type OpenCommuneAppSession,
  type OpenExternal,
  type OpenVideo,
  type PickScenePhotos,
  type ReopenWithPhoneResult,
  type SceneLocationResult,
  type ScenePhotoPickResult,
  TrangXa,
  type ZaloFailure,
} from "./cong-dan";
import { COMPANY } from "./content/company-profile";
import { NutChatOA } from "./features/company-intro/NutChatOA";
import {
  type CommuneAppLoginResult,
  type KetQuaMoPhienQuaCau,
  openCommuneAppSessionWithPhone,
  reopenCitizenSessionWithPhone,
  type ReopenWithPhoneBridgeResult,
} from "./features/dang-nhap/cau-vigov";
import {
  type CurrentLocationResult,
  getCommuneAppLocation,
  getCurrentLocation,
} from "./features/dang-nhap/current-location";
import { moRaNgoai } from "./features/tinh-nang/mo-ra-ngoai";
import {
  layTenZalo,
  chooseScenePhotos,
  type ScenePhotoPick,
  type SdkFailure,
  takeScenePhoto,
} from "./features/tinh-nang/zalo-api";
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
 *   reads (`?d=<commune domain>&src=qr|zns`, ADR 0047) swaps the WHOLE app for the commune's interface
 *   (`QrCommuneApp`, owner 06/10/2026), not a route, so there is still nothing to route.
 */

/**
 * ZALO'S REFUSAL, CROSSING THE TWO HALVES — the commercial half's `SdkFailure` (the SDK's own code) becomes
 * the state half's `ZaloFailure`, unchanged. `other` is a capability the state half never asks for, so
 * nothing crosses for it. Absent in → absent out: this never invents a code (user rule 30/09/2026 — the
 * displayed code is the measurement, and the one a tester reads to know which permission is missing).
 */
export function toZaloFailure(failure: SdkFailure | undefined): ZaloFailure | undefined {
  if (failure === undefined || failure.capability === "other") return undefined;
  return { capability: failure.capability, code: failure.code, transient: failure.transient };
}

/** `thu-lai`, with Zalo's refusal attached only when there is one (no `zalo: undefined` field). */
function retryWith(failure: SdkFailure | undefined): { kieu: "thu-lai"; zalo?: ZaloFailure } {
  const zalo = toZaloFailure(failure);
  return zalo === undefined ? { kieu: "thu-lai" } : { kieu: "thu-lai", zalo };
}

/**
 * CẦU NỐI HAI NỬA — CHỖ DUY NHẤT KẾT QUẢ CỦA CLIENT ĐĂNG NHẬP THƯƠNG MẠI ĐỔI SANG KIỂU CỦA NỬA NHÀ NƯỚC.
 *
 * Nửa nhà nước cần mở phiên ViGov, nhưng không được nhập `zmp-sdk` hay client của `vihat-miniapp`
 * (`ranh-gioi-hai-nua.test.ts` §3a). Since 06/10/2026 the only caller is `toReopenWithPhoneResult` below
 * (the shared app's QR path, `QrCommuneApp`); the confirmation step it was first written for is no longer
 * rendered.
 *
 * Bảng dịch theo VIỆC NGƯỜI DÂN LÀM TIẾP, không theo mã trạng thái:
 *   xong                                        → có phiên
 *   cau-tat · chua-san-sang · chua-khai-host    → `chua-mo`: bấm lại không đổi được gì
 *   ma-het-han · tam-ngung · khong-goi-duoc ·
 *   khong-lay-duoc-ma                           → `thu-lai` (with Zalo's code when Zalo refused)
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
    case "khong-lay-duoc-ma":
      return retryWith(kq.failure);
    default:
      return { kieu: "thu-lai" };
  }
}

/**
 * Bảng dịch của lần MỞ LẠI KÈM SỐ — cùng bảng với `sangKieuCongDan`, thêm `tu-choi`, và nhánh `xong`
 * mang `phoneVerified` của phiên mới thay cho tên miền (lần mở lại không đổi khoá tra của hai màn công
 * khai). Không nhánh nào mang mã số điện thoại: mã ấy dừng ở `cau-vigov.ts`.
 */
export function toReopenWithPhoneResult(result: ReopenWithPhoneBridgeResult): ReopenWithPhoneResult {
  if (result.kieu === "tu-choi") return { kieu: "tu-choi" };
  if (result.kieu === "xong") {
    const { token, ten_xa, da_xac_thuc_so } = result.phien;
    return { kieu: "xong", token, ten_xa, da_xac_thuc_so };
  }
  const other = sangKieuCongDan(result);
  // `sangKieuCongDan` chỉ trả `xong` cho `result.kieu === "xong"`, đã xử lý ở trên.
  return other.kieu === "xong" ? { kieu: "thu-lai" } : other;
}

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
 *
 * NO PRODUCTION CALLER since 06/10/2026: the QR path now renders `QrCommuneApp`, whose public screens read
 * the QR host directly (`TrangXa`). Kept, with its test, until someone decides to remove the old path.
 */
export function khoaTraCongKhai(
  ten_mien_phien: string | null,
  ten_mien_da_xac_nhan: string | null,
): string | null {
  if (ten_mien_phien !== null) return ten_mien_phien;
  return ten_mien_da_xac_nhan;
}

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
  // `feedbackDraftStore` — the single-key store (ADR 0050 #7): a separate App ID, a separate origin, one commune.
  // The shared app's QR path (`QrCommuneApp`) gets a store keyed per commune host instead; its intro gets none
  // (`ranh-gioi-hai-nua.test.ts` §3b).
  return (
    <TrangXa
      ten_mien={ten_mien}
      lay_ten={layTenChoXa}
      getSceneLocation={getCommuneSceneLocation}
      pickScenePhotos={pickCommuneScenePhotos}
      draftStore={feedbackDraftStore}
      openSession={openCommuneAppSession}
      openVideo={openCommuneVideo}
      openLink={openCommuneLink}
    />
  );
}

/**
 * THE LINK OPENER of the commune's own app (ADR 0067 §1, §5; owner 01/10/2026): a link inside an article's body,
 * and a home-strip banner pointing at an https page. Built here for the same reason as `openCommuneVideo`; the
 * state half asks the citizen first (`cong-dan/man/leave-app.tsx`) and calls this only on "Mở trang". Destination
 * `"lien-ket-xa"`, so the privacy policy's counted sentence names it. Also injected by the shared app's QR path
 * (`QrCommuneApp`, owner 06/10/2026); never by its intro.
 */
const openCommuneLink: OpenExternal = (url) => moRaNgoai("lien-ket-xa", url);

/**
 * THE VIDEO OPENER of the commune's own app (ADR 0047 §6, 01/10/2026) — the shell builds it, the state half only
 * declares its type (`OpenVideo`), as for `lay_ten`. It goes through the one declared door, `moRaNgoai`, under
 * the destination `"video"`, which is what makes the privacy policy's counted sentence name it
 * (`content/dich-ra-ngoai.ts`). Only the link the commune posted goes out — nothing of the citizen's.
 * Also injected by the shared app's QR path (`QrCommuneApp`, owner 06/10/2026); the shared app's OLD news screen
 * (`TinTucXaScreen`) has no video button.
 */
const openCommuneVideo: OpenVideo = (url) => moRaNgoai("video", url);

/**
 * THE COMMUNE APP'S SESSION OPENER — the shell builds it, the state half only declares its type
 * (`OpenCommuneAppSession`), exactly like the session opener of the shared app's QR path. No host hint: the commune comes
 * from the App ID's `mini_app` row on the server (ADR 0047 §6), and the app never invents a domain to send
 * (the 27/09 incident, ADR 0047:271-277). The App ID, the two Zalo codes and the phone stop in
 * `features/dang-nhap/cau-vigov.ts`; only the ViGov session (or a branch) comes down.
 *
 * Table by WHAT THE CITIZEN DOES NEXT, not by status code:
 *   xong                                            → the session
 *   tu-choi · ngoai-zalo                            → the same branch
 *   khong-ro-app · app-chua-san-sang (422)          → `chua-ket-noi`: this app is not connected; pressing
 *                                                     again changes nothing
 *   cau-tat (503 / no usable session) · yeu-cau-hong
 *   (400) · chua-khai-host                          → `tam-ngung`: the channel is not serving now
 *   zalo-khong-tra-loi (502) · qua-nhieu-lan (429)  → `cho-lat`: wait a moment, then press again
 *   ma-het-han (401) · khong-goi-duoc ·
 *   khong-lay-duoc-ma                               → `thu-lai`: a fresh tap gets fresh codes — carrying
 *                                                     Zalo's code when Zalo refused (`toZaloFailure`)
 */
export function toCommuneAppSessionResult(result: CommuneAppLoginResult): CommuneAppSessionResult {
  switch (result.kieu) {
    case "xong": {
      const { token, ten_xa, da_xac_thuc_so } = result.phien;
      return { kieu: "xong", token, ten_xa, da_xac_thuc_so };
    }
    case "tu-choi":
    case "ngoai-zalo":
      return { kieu: result.kieu };
    case "khong-ro-app":
    case "app-chua-san-sang":
      return { kieu: "chua-ket-noi" };
    case "cau-tat":
    case "yeu-cau-hong":
    case "chua-khai-host":
      return { kieu: "tam-ngung" };
    case "zalo-khong-tra-loi":
    case "qua-nhieu-lan":
      return { kieu: "cho-lat" };
    case "khong-lay-duoc-ma":
      return retryWith(result.failure);
    default:
      return { kieu: "thu-lai" };
  }
}

/**
 * Always the real identity path — Zalo's `getAccessToken` + `getPhoneNumber`, exchanged by ViGov identity
 * (owner 05/10/2026: there is no other build). It GOES TO ViGov IDENTITY (ADR 0066), not `vihat-miniapp`:
 * `identityHost` is handed in by the state half (`cong-dan/api/mo-phien-vigov.ts`, from its address map),
 * because this shell may not import that map. The shared app's QR-path openers near the top of this file are
 * untouched and still go to `vihat-miniapp` (`openSharedAppSession` below).
 */
const openCommuneAppSession: OpenCommuneAppSession = async (identityHost) =>
  toCommuneAppSessionResult(await openCommuneAppSessionWithPhone(identityHost));

/**
 * THE LOCATION BRIDGE, for BOTH apps — the shell builds it, the state half only declares its type
 * (`GetSceneLocation`), exactly like the session opener of the QR path. `getCurrentLocation` takes the
 * Zalo codes and exchanges them at `vihat-miniapp` `POST /api/v1/location` (public: no ViGov session and
 * no commercial ticket is involved, so the commune's own app — which has no session — can use it too).
 * The CODES stop in `features/dang-nhap/current-location.ts`; only coordinates or a branch come down.
 *
 * Table by WHAT THE CITIZEN DOES NEXT, not by status code:
 *   xong                                                        → the location
 *   tu-choi · ngoai-zalo · qua-nhieu-lan                        → the same branch
 *   zalo-khong-tra-loi · khong-goi-duoc · yeu-cau-hong ·
 *   khong-lay-duoc-ma                                           → `thu-lai` (a fresh tap gets fresh codes),
 *                                                                 with Zalo's code when Zalo refused
 *   tam-ngung · chua-khai-host                                  → `tam-ngung` (pressing again changes nothing)
 */
export function toSceneLocationResult(result: CurrentLocationResult): SceneLocationResult {
  switch (result.kieu) {
    case "xong":
      return { kind: "xong", location: { lat: result.location.latitude, lng: result.location.longitude } };
    case "tu-choi":
    case "ngoai-zalo":
    case "qua-nhieu-lan":
      return { kind: result.kieu };
    case "tam-ngung":
    case "chua-khai-host":
      return { kind: "tam-ngung" };
    case "khong-lay-duoc-ma": {
      const zalo = toZaloFailure(result.failure);
      return zalo === undefined ? { kind: "thu-lai" } : { kind: "thu-lai", zalo };
    }
    default:
      return { kind: "thu-lai" };
  }
}

const getSceneLocation: GetSceneLocation = async () => toSceneLocationResult(await getCurrentLocation());

/**
 * The commune app's location: same table, but the exchange carries this app's App ID so `vihat-miniapp`
 * uses its own secret (`getCommuneAppLocation`). The shared app above keeps the two-key body.
 */
const getCommuneSceneLocation: GetSceneLocation = async () => toSceneLocationResult(await getCommuneAppLocation());

/**
 * THE SCENE-PHOTO BRIDGE — the commune's own app, and the shared app ONLY on its QR commune path (`QrCommuneApp`,
 * owner 06/10/2026, revoking "own app only" of 02/10/2026). The SDK picker is app-independent. The shell builds
 * it, the state half only declares its
 * type (`PickScenePhotos`), exactly like the location bridge above. What comes down is LOCAL temp paths: the
 * picker uploads nothing, and the bytes leave the phone only through the state half's presigned POST, after
 * the petition is recorded.
 *
 * Table by WHAT THE CITIZEN DOES NEXT:
 *   xong                → the paths
 *   huy                 → nothing (they closed the picker)
 *   tu-choi · ngoai-zalo→ the same branch
 *   khong-lay-duoc      → `thu-lai`, with Zalo's code when Zalo refused (`toZaloFailure`)
 */
export function toScenePhotoPickResult(result: ScenePhotoPick): ScenePhotoPickResult {
  switch (result.kieu) {
    case "xong":
      return { kind: "xong", paths: result.du_lieu };
    case "huy":
    case "tu-choi":
    case "ngoai-zalo":
      return { kind: result.kieu };
    default: {
      const zalo = toZaloFailure(result.failure);
      return zalo === undefined ? { kind: "thu-lai" } : { kind: "thu-lai", zalo };
    }
  }
}

const pickCommuneScenePhotos: PickScenePhotos = async (source, max) =>
  toScenePhotoPickResult(source === "camera" ? await takeScenePhoto() : await chooseScenePhotos(max));

/**
 * CẦU HỌ TÊN — `getUserInfo`, gọi CHỈ từ bước mở app của `TrangXa` (29/09/2026): "check" không bật hộp
 * của Zalo, "ask" bật. Chỉ tên đi xuống nửa nhà nước; tên rỗng là "không lấy được", không bao giờ một
 * chuỗi rỗng giả làm tên. Zalo refused with a code → that code goes down too (`toZaloFailure`).
 */
const layTenChoXa: LayTenZalo = async (mode) => {
  const kq = await layTenZalo(mode === "ask");
  if (kq.kieu === "khong-lay-duoc") {
    const zalo = toZaloFailure(kq.failure);
    return zalo === undefined ? { kieu: "khong-lay-duoc" } : { kieu: "khong-lay-duoc", zalo };
  }
  if (kq.kieu !== "xong") return { kieu: kq.kieu };
  const ho_ten = kq.du_lieu.trim();
  return ho_ten === "" ? { kieu: "khong-lay-duoc" } : { kieu: "xong", ho_ten };
};

/**
 * THE SHARED VIHAT APP — two shapes, chosen once from the launch link (read at mount; `location.search` is
 * synchronous, so the branch never changes between renders and each branch keeps its own hooks):
 *   a trusted commune link (`?d=<host>&src=qr|zns`, `thamSoXa`) → `QrCommuneApp`, the commune's full interface
 *   anything else                                                → `SharedAppIntro`, the ViHAT intro and tabs
 */
function AppChung() {
  const [thamSo] = useState<KetQuaDo>(thamSoMoApp);
  const goiY = thamSoXa(thamSo);
  return goiY !== null ? <QrCommuneApp ten_mien={goiY.ten_mien} /> : <SharedAppIntro />;
}

/** The ViHAT intro and tabs — the shared app opened WITHOUT a commune QR. Unchanged by 06/10/2026. */
function SharedAppIntro() {
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

  /**
   * `key` MANG CẢ `moc` LẪN `lan`, VÀ ĐÓ LÀ THỨ LÀM MỤC "QUYỀN" TRÊN MÀN CHỦ CHẠY ĐƯỢC LẦN THỨ HAI.
   *
   * `ContactScreen` đọc `moc` làm GIÁ TRỊ BAN ĐẦU của trạng thái "đang xem màn quyền". Giá trị ban
   * đầu chỉ được đọc một lần cho mỗi lần dựng, nên không có `key` đổi thì: bấm Quyền -> mở, bấm
   * Quay lại -> đóng, bấm Quyền lần nữa -> KHÔNG mở lại, vì màn cha không hề được dựng lại. Một
   * nút chạy đúng một lần rồi im là kiểu hỏng không ai báo, người ta chỉ thôi bấm nó.
   */
  const noiDung = <Screen key={`${currentId}:${vi_tri.moc ?? ""}:${vi_tri.lan}`} moc={vi_tri.moc} onDi={di} />;

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
        // No commune on this path: a commune QR renders `QrCommuneApp`, which carries the commune's own header.
        xaDaChon={null}
      >
        {noiDung}
      </KhungApp>
    </NhaCungCapPhien>
  );
}

/**
 * Bridge result of the shared app's session opening → the commune app's session result, for the QR path.
 *
 * The shared app opens the session through the `vihat-miniapp` bridge WITH the phone, bound to the QR host
 * (`reopenCitizenSessionWithPhone`, translated by `toReopenWithPhoneResult`), because the shared App ID's
 * secret lives there (ADR 0032). `TrangXa` expects `CommuneAppSessionResult`; this maps one onto the other by
 * WHAT THE CITIZEN DOES NEXT:
 *   xong                      → xong (token, commune name, phone-verified flag — unchanged)
 *   tu-choi · ngoai-zalo      → the same branch
 *   thu-lai                   → thu-lai, keeping Zalo's code when Zalo refused
 *   chua-mo                   → tam-ngung: the bridge is off or not serving; pressing again changes nothing now
 * No branch carries the phone code: it stops in `features/dang-nhap/cau-vigov.ts`.
 */
export function toSharedAppSessionResult(result: ReopenWithPhoneResult): CommuneAppSessionResult {
  switch (result.kieu) {
    case "xong": {
      const { token, ten_xa, da_xac_thuc_so } = result;
      return { kieu: "xong", token, ten_xa, da_xac_thuc_so };
    }
    case "tu-choi":
    case "ngoai-zalo":
      return { kieu: result.kieu };
    case "thu-lai":
      return result.zalo === undefined ? { kieu: "thu-lai" } : { kieu: "thu-lai", zalo: result.zalo };
    case "chua-mo":
      return { kieu: "tam-ngung" };
  }
}

/**
 * The shared app's session opener for ONE commune host. It IGNORES the identity host `TrangXa` hands out: that
 * host is where a commune's OWN App ID logs in (ADR 0066); the shared App ID logs in through `vihat-miniapp`.
 * The host it sends is the QR's `d` — the commune whose public pages the citizen is looking at — and the
 * session is still kept only if the server's commune name equals the name on screen (`openCommuneAppSession`).
 *
 * `reopen` is for tests only — production code never passes it.
 */
export function openSharedAppSession(
  communeHost: string,
  reopen: (communeHost: string) => Promise<ReopenWithPhoneBridgeResult> = reopenCitizenSessionWithPhone,
): OpenCommuneAppSession {
  return async () => toSharedAppSessionResult(toReopenWithPhoneResult(await reopen(communeHost)));
}

/**
 * THE SHARED APP ON A COMMUNE QR — the commune's full interface (owner 06/10/2026: the commune apps cannot be
 * published yet, so citizens borrow the shared app — "full nội dung trong citizen app"). `TrangXa` owns the
 * whole screen, as in `AppRieng`: no ViHAT header, tab bar or chat button; the commune name is on every screen
 * and the send button names the commune (README §Non-negotiables #2, #5).
 *
 * `d` STEERS THE INTERFACE AND GRANTS NOTHING (ADR 0005 · 0019 · 0022): it picks whose PUBLIC pages are looked
 * up. A session opens only by the citizen's explicit act through `TrangXa`'s gate (`commune-session.ts`), and
 * is kept only if its commune name equals the one on screen (`cong-dan/api/mo-phien-vigov.ts`).
 *
 * What differs from `AppRieng`, and why:
 *   openSession       through `vihat-miniapp`, bound to this host (`openSharedAppSession`)
 *   getSceneLocation  the shared App ID's exchange (`getSceneLocation`), not the own app's App ID
 *   draftStore        keyed by THIS host (`sharedAppDraftStore`): one origin serves every commune, so a single
 *                     key would offer one commune's draft — name and phone included — inside another (rule 1)
 * The rest (name, scene photos, video, links) is app-independent and the same as `AppRieng`.
 *
 * Both per-host values are memoised: `TrangXa` keys its session gate and effects on them.
 */
function QrCommuneApp({ ten_mien }: { ten_mien: string }) {
  const openSession = useMemo(() => openSharedAppSession(ten_mien), [ten_mien]);
  const draftStore = useMemo(() => sharedAppDraftStore(ten_mien), [ten_mien]);
  return (
    <TrangXa
      ten_mien={ten_mien}
      lay_ten={layTenChoXa}
      getSceneLocation={getSceneLocation}
      pickScenePhotos={pickCommuneScenePhotos}
      draftStore={draftStore}
      openSession={openSession}
      openVideo={openCommuneVideo}
      openLink={openCommuneLink}
    />
  );
}
