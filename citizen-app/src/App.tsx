import { type ReactNode, useEffect, useState } from "react";

import { feedbackDraftStore } from "./commune-app/feedback-draft-store";
import { TabBar } from "./components/TabBar";
import {
  type CommuneAppSessionResult,
  type GetSceneLocation,
  CitizenChannel,
  type SessionOpenResult,
  type ConfirmationOutcome,
  type GetZaloName,
  type OpenVigovSession,
  CitizenChannelButton,
  type OpenCommuneAppSession,
  type ReopenWithPhone,
  type ReopenWithPhoneResult,
  type SceneLocationResult,
  CommuneHome,
  CommuneConfirmation,
} from "./citizen";
import { COMPANY } from "./content/company-profile";
import { NutChatOA } from "./features/company-intro/NutChatOA";
import {
  type CommuneAppLoginResult,
  type BridgeSessionOpenResult,
  openCitizenSessionViaBridge,
  openCommuneAppSessionWithPhone,
  reopenCitizenSessionWithPhone,
  type ReopenWithPhoneBridgeResult,
} from "./features/log-in/vigov-bridge";
import {
  type CurrentLocationResult,
  getCommuneAppLocation,
  getCurrentLocation,
} from "./features/log-in/current-location";
import { layTenZalo } from "./features/tinh-nang/zalo-api";
import { SessionProvider } from "./features/log-in/session-store";
import { TIEU_DE_XAC_NHAN_XA } from "./features/kham-pha";
import {
  DEFAULT_SCREEN_ID,
  type DiemDen,
  findScreen,
  type ScreenId,
} from "./features/company-intro/screens";
import { scrollToAnchor } from "./lib/scroll-to";
import { type ReadResult, readLaunchParams, communeParams } from "./lib/launch-params";
import { FIXED_COMMUNE_DOMAIN } from "./lib/fixed-commune";

/**
 * Phase 1 shell: four static screens, no navigation library, no state beyond the current tab.
 *
 * WHY THE HEADER ALWAYS NAMES THE OWNER:
 *
 *   In phase 2 this header is where the COMMUNE name lives, on every screen, because a citizen
 *   who cannot see which commune they are addressing can submit to the wrong one (README
 *   §Non-negotiables #2). The slot was reserved from the first screen so phase 2 would fill it
 *   rather than invent it — and `AppFrame` below is where it now gets filled: a commune has been
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
 * client của `vihat-miniapp` (`two-halves-boundary.test.ts` §3a). Nó chỉ khai KIỂU hàm nó cần
 * (`OpenVigovSession`); lớp vỏ — được nhập cả hai nửa — dựng hàm ấy ở đây và tiêm xuống `CommuneConfirmation`.
 *
 * Bảng dịch theo VIỆC NGƯỜI DÂN LÀM TIẾP, không theo mã trạng thái:
 *   xong                                        → có phiên
 *   cau-tat · chua-san-sang · chua-khai-host    → `chua-mo`: bấm lại không đổi được gì
 *   ma-het-han · tam-ngung · khong-goi-duoc ·
 *   khong-lay-duoc-ma                           → `thu-lai`
 *   ngoai-zalo                                  → `ngoai-zalo`
 */
export function toCitizenShape(result: BridgeSessionOpenResult): SessionOpenResult {
  switch (result.kind) {
    case "xong":
      return {
        kind: "xong",
        token: result.session.token,
        commune_name: result.session.commune_name,
        domain: result.session.commune_domain,
      };
    case "cau-tat":
    case "chua-san-sang":
    case "chua-khai-host":
      return { kind: "chua-mo" };
    case "ngoai-zalo":
      return { kind: "ngoai-zalo" };
    default:
      return { kind: "thu-lai" };
  }
}

/** `communeConfirmed` không đọc ở đây: kiểu của nó là hằng `true`, và thân gửi đi cũng ghi hằng ấy. */
const openVigovSession: OpenVigovSession = async (req) =>
  toCitizenShape(await openCitizenSessionViaBridge(req.communeHostHint));

/**
 * Bảng dịch của lần MỞ LẠI KÈM SỐ — cùng bảng với `toCitizenShape`, thêm `tu-choi`, và nhánh `xong`
 * mang `phoneVerified` của phiên mới thay cho tên miền (lần mở lại không đổi khoá tra của hai màn công
 * khai). Không nhánh nào mang mã số điện thoại: mã ấy dừng ở `vigov-bridge.ts`.
 */
export function toReopenWithPhoneResult(result: ReopenWithPhoneBridgeResult): ReopenWithPhoneResult {
  if (result.kind === "tu-choi") return { kind: "tu-choi" };
  if (result.kind === "xong") {
    const { token, commune_name, phone_verified } = result.session;
    return { kind: "xong", token, commune_name, phone_verified };
  }
  const other = toCitizenShape(result);
  // `toCitizenShape` chỉ trả `xong` cho `result.kind === "xong"`, đã xử lý ở trên.
  return other.kind === "xong" ? { kind: "thu-lai" } : other;
}

/**
 * Hàm mở lại phiên kèm số cho kênh công dân, gắn với TÊN MIỀN CÔNG DÂN ĐÃ XÁC NHẬN ở lần mở này (`d`).
 *
 * VÌ SAO `d`, KHÔNG PHẢI `communePrimaryHost` của phiên: `communeHostHint` là gợi ý mà công dân đã bấm
 * xác nhận; lần mở lại dùng lại ĐÚNG cú xác nhận ấy, không một tên miền nào khác. Nếu máy chủ phân giải
 * nó ra một xã khác xã của phiên đang dùng, `reopenSessionWithPhone` (nửa nhà nước) từ chối thay phiên.
 */
function reopenWithPhoneFor(confirmed_domain: string): ReopenWithPhone {
  return async () => toReopenWithPhoneResult(await reopenCitizenSessionWithPhone(confirmed_domain));
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
 */
export function publicLookupKey(
  session_domain: string | null,
  confirmed_domain: string | null,
): string | null {
  if (session_domain !== null) return session_domain;
  return confirmed_domain;
}

/**
 * Xã đã xác nhận ở lần mở này — chỉ trong `useState`, mất khi app đóng (`two-halves-boundary.test.ts` §3b).
 *
 *   `ten`       có phiên → `tenantDisplayName` của phiên; không phiên → `name` của `/communes`
 *   `tinh`      chỉ khi KHÔNG phiên: `province` của `/communes`. Phiên không trả tỉnh, và ghép tỉnh của
 *               `/communes` với tên của phiên là ghép hai nguồn có thể nói hai xã khác nhau
 *   `domain`    khoá tra của hai màn công khai (`publicLookupKey`), hoặc `null`
 *
 * `ten` · `tinh` giữ tên cũ: cùng hình dạng xã với lớp khám phá của nửa thương mại (`features/kham-pha`).
 */
type LaunchCommune = { ten: string; tinh: string | null; domain: string | null };

/**
 * Vỏ ứng dụng, THUẦN — nhận mọi thứ qua tham số, không giữ trạng thái nào.
 *
 * Tách ra vì bất di dịch #2 ("đã chọn xã thì tên xã hiện trên MỌI màn hình") chỉ kiểm được khi
 * dựng được cái vỏ ấy với một xã đã chọn và với TỪNG màn hình một. Gói chung trong một component
 * có `useState` thì phép kiểm phải bấm nút, mà bộ test ở đây dựng bằng `react-dom/server` và
 * không có DOM để bấm. Một bất biến không kiểm được là một bất biến sẽ trôi.
 */
export function AppFrame(props: {
  screen: ScreenId;
  onSelectScreen: (id: ScreenId) => void;
  /**
   * Xã đã xác nhận, hoặc `null`. `ten` là tên xã của PHIÊN khi có phiên, tên `/communes` trả khi chưa;
   * `tinh` chỉ có khi chưa có phiên. Xem `LaunchCommune`.
   */
  selectedCommune: { readonly ten: string; readonly tinh?: string | null } | null;
  /**
   * Đang ở lớp khám phá: xác nhận xã. Lúc ấy thanh tab BIẾN MẤT.
   *
   * `skills/accessibility-elderly`, yêu cầu #4: mỗi màn một việc. Để thanh tab lại thì công dân
   * bấm nó, không có gì xảy ra (vì chưa có xã), và một nút bấm không phản hồi là cách nhanh
   * nhất để người lớn tuổi kết luận rằng app hỏng và bỏ đi.
   */
  discovery?: boolean;
  children: ReactNode;
}) {
  const screen = findScreen(props.screen);

  return (
    <div className="app">
      <header className="app-header">
        <div className="app-header__hang">
          <div className="app-header__ten">
            {/* MỘT Ô, HAI CHỦ SỞ HỮU. Chưa chọn xã thì đây là đơn vị phát hành ứng dụng — thứ
                Zalo đã duyệt. Chọn xã rồi thì đây là xã, trên mọi màn hình, không có ngoại lệ. */}
            <p className="app-header__owner">
              {props.selectedCommune === null
                ? COMPANY.name
                : props.selectedCommune.tinh
                  ? `${props.selectedCommune.ten}, ${props.selectedCommune.tinh}`
                  : props.selectedCommune.ten}
            </p>
            <p className="app-header__screen">
              {props.discovery ? TIEU_DE_XAC_NHAN_XA : screen.headerTitle}
            </p>
          </div>

          {/* KHÔNG CÓ NÚT "ĐỔI XÃ" (ADR 0044 câu 4 · ADR 0047): một phiên, một xã. Người cần xã
              khác quét QR của xã ấy — không có danh mục nào trong app để đổi sang. */}
        </div>
      </header>

      <main className="app-main" id="main">
        {props.children}
      </main>

      {/* NÚT CHAT NỔI BIẾN MẤT CÙNG LÚC VỚI THANH TAB, CÙNG MỘT LÝ DO (xem `discovery` ở trên):
          màn xác nhận xã là màn mỗi-màn-một-việc, và việc ấy là xác nhận đúng xã. Một nút nổi mời
          trò chuyện với một doanh nghiệp giữa lúc ấy là một đường rẽ sai chỗ. */}
      {!props.discovery && <NutChatOA />}

      {!props.discovery && <TabBar current={props.screen} onSelect={props.onSelectScreen} />}
    </div>
  );
}

/**
 * VỊ TRÍ HIỆN TẠI TRONG APP — màn nào, chỗ nào trong màn ấy, và lần điều hướng thứ mấy.
 *
 * `visit` KHÔNG PHẢI MỘT BỘ ĐẾM CHO VUI. Không có nó thì bấm "Văn phòng" hai lần liên tiếp chỉ cuộn
 * một lần: lần thứ hai `screen` và `anchor` y hệt lần đầu, React không thấy gì đổi, và người bấm thấy
 * một cái nút không phản hồi. Với người lớn tuổi, một nút không phản hồi là kết luận "app hỏng".
 */
type Position = { screen: ScreenId; anchor?: string; visit: number };

/**
 * CHỌN APP LÚC DỰNG — `FIXED_COMMUNE_DOMAIN` là hằng của bản dựng, nên nhánh này không bao giờ đổi giữa hai lần
 * vẽ (và không vi phạm thứ tự hook: mỗi nhánh là một component riêng).
 */
export function App() {
  return FIXED_COMMUNE_DOMAIN !== null ? <CommuneApp domain={FIXED_COMMUNE_DOMAIN} /> : <SharedApp />;
}

/**
 * APP RIÊNG CỦA MỘT XÃ (`deploy.mjs --domain=<x> --vao-thang`, ADR 0047 §6).
 *
 * Chọn app của xã trên Zalo là thấy NGAY trang của xã ấy (chủ dự án, 28/09/2026). KHÔNG MỘT CHỮ NÀO
 * CỦA ViHAT GROUP: không màn giới thiệu, không thanh tab của phần thương mại, không nút chat OA, không
 * tên đơn vị phát hành trên header. Không bước xác nhận, không đăng nhập lúc mở — `CommuneHome` tra tên xã
 * qua tuyến công khai rồi hiện kênh công dân. Kênh là màn gốc: không nút "Quay lại".
 *
 * Toàn bộ giao diện nằm ở nửa nhà nước (`citizen/screens/CommuneHome.tsx`), theo bản mẫu `vi-gov/zalo-miniapp`
 * chủ dự án chọn 28/09/2026; lớp vỏ này chỉ chọn nó.
 */
export function CommuneApp({ domain }: { domain: string }) {
  // `feedbackDraftStore` goes to THIS app only (ADR 0050 #7): a separate App ID, a separate origin. The
  // shared app below never receives it — `two-halves-boundary.test.ts` §3b reads `SharedApp`'s body for it.
  return (
    <CommuneHome
      domain={domain}
      getName={getNameForCommune}
      getSceneLocation={getCommuneSceneLocation}
      draftStore={feedbackDraftStore}
      openSession={openCommuneAppSession}
    />
  );
}

/**
 * THE COMMUNE APP'S SESSION OPENER — the shell builds it, the state half only declares its type
 * (`OpenCommuneAppSession`), exactly like the session opener of the shared app's QR path. No host hint: the commune comes
 * from the App ID's `mini_app` row on the server (ADR 0047 §6), and the app never invents a domain to send
 * (the 27/09 incident, ADR 0047:271-277). The App ID, the two Zalo codes and the phone stop in
 * `features/log-in/vigov-bridge.ts`; only the ViGov session (or a branch) comes down.
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
 *   khong-lay-duoc-ma                               → `thu-lai`: a fresh tap gets fresh codes
 */
export function toCommuneAppSessionResult(result: CommuneAppLoginResult): CommuneAppSessionResult {
  switch (result.kind) {
    case "xong": {
      const { token, commune_name, phone_verified } = result.session;
      return { kind: "xong", token, commune_name, phone_verified };
    }
    case "tu-choi":
    case "ngoai-zalo":
      return { kind: result.kind };
    case "khong-ro-app":
    case "app-chua-san-sang":
      return { kind: "chua-ket-noi" };
    case "cau-tat":
    case "yeu-cau-hong":
    case "chua-khai-host":
      return { kind: "tam-ngung" };
    case "zalo-khong-tra-loi":
    case "qua-nhieu-lan":
      return { kind: "cho-lat" };
    default:
      return { kind: "thu-lai" };
  }
}

const openCommuneAppSession: OpenCommuneAppSession = async () =>
  toCommuneAppSessionResult(await openCommuneAppSessionWithPhone());

/**
 * THE LOCATION BRIDGE, for BOTH apps — the shell builds it, the state half only declares its type
 * (`GetSceneLocation`), exactly like the session opener of the QR path. `getCurrentLocation` takes the
 * Zalo codes and exchanges them at `vihat-miniapp` `POST /api/v1/location` (public: no ViGov session and
 * no commercial ticket is involved, so the commune's own app — which has no session — can use it too).
 * The CODES stop in `features/log-in/current-location.ts`; only coordinates or a branch come down.
 *
 * Table by WHAT THE CITIZEN DOES NEXT, not by status code:
 *   xong                                                        → the location
 *   tu-choi · ngoai-zalo · qua-nhieu-lan                        → the same branch
 *   zalo-khong-tra-loi · khong-goi-duoc · yeu-cau-hong ·
 *   khong-lay-duoc-ma                                           → `thu-lai` (a fresh tap gets fresh codes)
 *   tam-ngung · chua-khai-host                                  → `tam-ngung` (pressing again changes nothing)
 */
export function toSceneLocationResult(result: CurrentLocationResult): SceneLocationResult {
  switch (result.kind) {
    case "xong":
      return { kind: "xong", location: { lat: result.location.latitude, lng: result.location.longitude } };
    case "tu-choi":
    case "ngoai-zalo":
    case "qua-nhieu-lan":
      return { kind: result.kind };
    case "tam-ngung":
    case "chua-khai-host":
      return { kind: "tam-ngung" };
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
 * CẦU HỌ TÊN — `getUserInfo`, gọi CHỈ từ bước mở app của `CommuneHome` (29/09/2026): "check" không bật hộp
 * của Zalo, "ask" bật. Chỉ tên đi xuống nửa nhà nước; tên rỗng là "không lấy được", không bao giờ một
 * chuỗi rỗng giả làm tên.
 */
const getNameForCommune: GetZaloName = async (mode) => {
  const result = await layTenZalo(mode === "ask");
  if (result.kieu !== "xong") return { kind: result.kieu };
  const full_name = result.du_lieu.trim();
  return full_name === "" ? { kind: "khong-lay-duoc" } : { kind: "xong", full_name };
};

function SharedApp() {
  const [position, setPosition] = useState<Position>({ screen: DEFAULT_SCREEN_ID, visit: 0 });
  const currentId = position.screen;
  const screen = findScreen(currentId);
  const Screen = screen.component;

  /** Đi tới một chỗ khác trong app. Đây là chỗ DUY NHẤT trong cả kho cài đặt việc điều hướng. */
  function go(destination: DiemDen) {
    setPosition((previous) => ({ screen: destination.man, anchor: destination.moc, visit: previous.visit + 1 }));
  }

  /**
   * Cuộn tới mỏ neo SAU KHI màn mới đã vẽ xong — đó là lý do việc này nằm trong `useEffect` chứ
   * không nằm trong hàm `go` ở trên: lúc `go` chạy, phần tử mang `id` ấy chưa tồn tại.
   *
   * `moc` là tên một màn CON (`MOC_QUAN_LY_QUYEN`) thì không có gì để cuộn tới, và `scrollToAnchor`
   * không làm gì — màn cha mới là chỗ đọc nó. Xem `features/company-intro/dieu-huong.ts`.
   */
  useEffect(() => {
    scrollToAnchor(position.anchor);
  }, [position.visit, position.anchor]);

  // Đọc MỘT LẦN, NGAY LÚC DỰNG. `location.search` có sẵn đồng bộ, nên không còn `useEffect` và
  // không còn nhịp nhấp nháy: công dân quét QR thấy thẳng màn xác nhận xã, không thấy màn giới
  // thiệu công ty loé lên trước. Đó là món quà kèm theo của việc bỏ `zmp-sdk` — xem
  // lib/launch-params.ts. Hàm bọc try/catch, nên ngoài trình duyệt nó trả về "không có tham số".
  const [launchParams] = useState<ReadResult>(readLaunchParams);

  /**
   * XÃ ĐÃ XÁC NHẬN Ở LẦN MỞ NÀY — `LaunchCommune`. Có hai cách tới đây, cả hai đều qua cú bấm xác nhận:
   *
   *   có phiên   `ten` là `tenantDisplayName` cầu phiên trả (ADR 0047 §Trả lời mục 4: "phiên nói
   *              thật"), nên header và bước xác nhận cuối trước khi gửi nói CÙNG một xã với xã máy chủ
   *              sẽ ghi phiếu.
   *   không phiên (27/09/2026, quyết định của chủ sản phẩm) `ten` + `tinh` là thứ `/communes` trả cho
   *              `d` và công dân vừa xác nhận. Chỉ tin tức và danh bạ mở được; gửi phản ánh vẫn cần phiên.
   *
   * `domain` chỉ làm khoá tra `?host=`. Nó không phải tham chiếu xã: không được gửi làm "xã của
   * tôi", không lưu, không vẽ ra (ADR 0047 điều kiện dừng #1). Tất cả sống trong `useState`.
   */
  const [commune, setCommune] = useState<LaunchCommune | null>(null);
  /** Lớp khám phá đã kết thúc (xác nhận xong, "không phải xã này", hoặc fail closed). */
  const [discoveryDone, setDiscoveryDone] = useState(false);
  /** Câu nói vì sao app về phần giới thiệu thay vì mở kênh — hoặc `null`. */
  const [notice, setNotice] = useState<string | null>(null);
  /**
   * Đang mở kênh công dân (gửi / tra cứu phản ánh). App.tsx KHÔNG nhập client ViGov — chỉ mở màn
   * của nửa nhà nước qua cửa `./citizen` (`two-halves-boundary.test.ts`).
   */
  const [citizenChannelOpen, setCitizenChannelOpen] = useState(false);

  /**
   * LỚP KHÁM PHÁ (ADR 0005 · 0047). `d` + `src` trên QR chỉ DẪN GIAO DIỆN — không chọn xã, không mở
   * phiên nếu công dân chưa bấm xác nhận. `null` (không có `d`, `src` không tin được, `d` sai khuôn)
   * là mở như không tham số: KHÔNG một lời gọi nào tới `identity`/`comms`.
   */
  const suggestion = communeParams(launchParams);
  const discovering = suggestion !== null && commune === null && !discoveryDone;

  function endDiscovery(result: ConfirmationOutcome) {
    setDiscoveryDone(true);
    // `suggestion === null` không tới được đây (màn xác nhận chỉ dựng khi có nó); nếu có thì KHÔNG đặt xã nào.
    if (result.kind === "da-mo" && suggestion !== null) {
      setCommune({ ten: result.commune_name, tinh: null, domain: publicLookupKey(result.domain, suggestion.domain) });
      setNotice(null);
    } else if (result.kind === "xac-nhan-khong-phien" && suggestion !== null) {
      setCommune({ ten: result.commune.ten, tinh: result.commune.tinh, domain: publicLookupKey(null, suggestion.domain) });
      setNotice(null);
    } else if (result.kind === "ve-gioi-thieu") {
      setNotice(result.text);
    }
  }

  /**
   * `key` MANG CẢ `anchor` LẪN `visit`, VÀ ĐÓ LÀ THỨ LÀM MỤC "QUYỀN" TRÊN MÀN CHỦ CHẠY ĐƯỢC LẦN THỨ HAI.
   *
   * `ContactScreen` đọc `moc` làm GIÁ TRỊ BAN ĐẦU của trạng thái "đang xem màn quyền". Giá trị ban
   * đầu chỉ được đọc một lần cho mỗi lần dựng, nên không có `key` đổi thì: bấm Quyền -> mở, bấm
   * Quay lại -> đóng, bấm Quyền lần nữa -> KHÔNG mở lại, vì màn cha không hề được dựng lại. Một
   * nút chạy đúng một lần rồi im là kiểu hỏng không ai báo, người ta chỉ thôi bấm nó.
   */
  let body: ReactNode = (
    <Screen key={`${currentId}:${position.anchor ?? ""}:${position.visit}`} moc={position.anchor} onDi={go} />
  );
  if (citizenChannelOpen) {
    body = (
      <CitizenChannel
        onClose={() => setCitizenChannelOpen(false)}
        domain={commune === null ? null : commune.domain}
        // Chỉ có khi lần mở này đi qua bước xác nhận xã — nơi DUY NHẤT phiên ViGov được mở.
        reopenWithPhone={suggestion === null ? undefined : reopenWithPhoneFor(suggestion.domain)}
        getSceneLocation={getSceneLocation}
      />
    );
  } else if (discovering && suggestion !== null) {
    body = (
      <CommuneConfirmation
        domain={suggestion.domain}
        source={suggestion.source}
        openVigovSession={openVigovSession}
        onDone={endDiscovery}
      />
    );
  } else if (notice !== null && currentId === DEFAULT_SCREEN_ID) {
    // FAIL CLOSED CÓ LỜI: về phần giới thiệu, và nói bằng một câu vì sao — không mã lỗi, không tên
    // miền. Chỉ trên tab đầu: đó là chỗ công dân vừa được đưa về.
    body = (
      <>
        <p className="cd-loi" role="status">
          {notice}
        </p>
        {body}
      </>
    );
  } else if (commune !== null && currentId === DEFAULT_SCREEN_ID) {
    /**
     * ĐÃ XÁC NHẬN XÃ THÌ TAB ĐẦU MỞ LỐI VÀO KÊNH CÔNG DÂN — đặt TRÊN màn chủ, không thay nó.
     *
     *   Trang xã mẫu (dịch vụ, số trực, giờ làm việc) đã bị xoá cùng danh mục xã mẫu: mọi chữ trên
     *   đó là dữ liệu đặt ra, và một trang xã thật cần nguồn máy chủ chưa có. Nên ở đây chỉ còn
     *   đúng thứ có thật — lối vào kênh — và phần giới thiệu, thứ Zalo đã duyệt, vẫn còn đường tới.
     *   Chỗ đặt nút là quyết định của card nối nguồn xã (TASK-04b), không phải của card này.
     */
    body = (
      <>
        <CitizenChannelButton onPress={() => setCitizenChannelOpen(true)} />
        {body}
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
     *   là mất, mở lại đăng nhập một chạm. Xem `features/log-in/session-store.tsx`.
     */
    <SessionProvider>
      <AppFrame
        screen={currentId}
        // Bấm một tab là điều hướng KHÔNG CÓ MỐC: người bấm muốn về đầu màn ấy, không muốn bị thả
        // xuống giữa một khối mà lần trước họ đi tới từ màn chủ.
        onSelectScreen={(id) => go({ man: id })}
        selectedCommune={commune === null ? null : { ten: commune.ten, tinh: commune.tinh }}
        discovery={discovering}
      >
        {body}
      </AppFrame>
    </SessionProvider>
  );
}
