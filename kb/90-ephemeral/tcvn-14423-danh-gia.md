---
id: tcvn-14423-danh-gia
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 812804d
expires: 2026-12-28
owns_facts:
  - "tổng hợp yêu cầu TCVN 14423:2026 áp vào phần mềm ViGov, giả định hệ thống cấp độ 3"
  - "mức đáp ứng TCVN 14423:2026 của kho vigov-v2, đo ngày 2026-09-28"
  - "khoảng trống của bộ não .claude so với TCVN 14423:2026 và đề xuất cập nhật"
---

# TCVN 14423:2026 — tổng hợp và đánh giá hiện trạng ViGov

**Đo ngày 28/09/2026** trên `vigov-v2@812804d`. Hết hạn **28/12/2026**: sau ngày đó mã đã đi xa,
cần đo lại. Nguồn là tệp `rule/TCVN 14423 2026 ANM HTTT.pdf`, do chủ dự án đưa vào ngày 28/09/2026
và chưa đưa vào git.

Tệp này **không chép lại tiêu chuẩn**. Nó chỉ rút ra những gì ràng buộc phần mềm này, rồi đối
chiếu từng điểm với mã.

---

## 0. Giả định — cần chủ dự án xác nhận

| # | Giả định | Vì sao phải hỏi |
|---|---|---|
| G1 | ViGov được xếp **cấp độ 3** | Cấp độ do **chủ quản hệ thống** đề xuất và được thẩm định, bên phát triển không tự chọn. Hệ thống xử lý dữ liệu cá nhân của nhiều xã và cung cấp dịch vụ công trực tuyến, nên cấp 3 là mức thấp nhất hợp lý. Nếu là **cấp 4**, xem cột "Cấp 4 thêm" ở §2 |
| G2 | Phần mềm ViGov là **"phần mềm thuê khoán"** (2.1.22) đối với chủ quản | Mục 5.17.2.1c khi đó ràng buộc chính ViHAT: phải có hợp đồng và cam kết bảo mật, và phải giao mã nguồn (hoặc chứng chỉ đánh giá độc lập kèm kiểm thử xâm nhập) |
| G3 | Hạ tầng (cụm k8s, mạng, sao lưu, SIEM) do **ViHAT vận hành** trên đám mây dùng chung | Nếu chủ quản tự vận hành, cột "Hạ tầng" ở §2 chuyển sang phía họ |

---

## 1. Tiêu chuẩn nói gì — cấu trúc

- Có **5 cấp độ**, cấp sau bao trùm cấp trước. Hệ thống quan trọng về an ninh quốc gia áp dụng mức của cấp 5.
- Cấp 1 và 2 có **15 nhóm**. Cấp 3 trở lên có **18 nhóm**: thêm *Giám sát và phòng thủ* (5.13),
  *Phát triển ứng dụng an toàn* (5.17) và *Quản lý kiểm tra an ninh mạng* (5.18). Phụ lục A là an toàn vật lý.
- Mỗi nhóm có hai phần: *Khái quát* (mục tiêu) và *Yêu cầu cụ thể*. Phần lớn yêu cầu là **quy trình
  có chu kỳ rà soát** (hằng năm, 6 tháng hoặc hằng quý), không phải tính năng.

**Chỉ khoảng 1/3 yêu cầu chạm vào mã nguồn.** Phần còn lại thuộc về tổ chức (quy trình, nhân sự,
đào tạo) và hạ tầng (mạng, máy trạm, mã độc, sao lưu). Cột "Ai" ở §2 tách ba loại này, để không ai
đòi mã nguồn giải quyết một việc của tổ chức.

---

## 2. Tổng hợp yêu cầu cấp độ 3

Chú thích cột "Ai": **M** = mã nguồn (kho này) · **H** = hạ tầng / vận hành · **T** = tổ chức, quy trình.

| Nhóm | Yêu cầu chính ở cấp 3 | Ai | Cấp 4 thêm |
|---|---|---|---|
| 5.1 Rủi ro | Quy trình xác định, đánh giá, xử lý, giám sát và truyền thông rủi ro; rà soát hằng năm; tính cả rủi ro từ nhà cung cấp | T | — |
| 5.2 Tài sản phần cứng | Danh mục tài sản (kể cả máy chủ đám mây); rà thiết bị lạ mỗi 6 tháng; DHCP log; xoá sạch dữ liệu khi chuyển giao thiết bị | H·T | — |
| 5.3 Tài sản phần mềm | Danh mục phần mềm được phép dùng, **gồm cả môi trường thực thi của ứng dụng**; chặn cài đặt thư viện và mã trái phép | H·T | Mọi phần mềm còn trong hạn hỗ trợ của hãng; rà phần mềm trái phép mỗi 6 tháng |
| 5.4 Tài sản thông tin | Phân loại **4 mức nhạy cảm** (công khai / nội bộ / hạn chế / bí mật nhà nước); danh sách kiểm soát truy cập theo từng loại tài sản; kiểm tra phân quyền dữ liệu **mỗi 6 tháng**; **mã hoá** thông tin xác thực và dữ liệu nhạy cảm cao khi lưu và khi truyền; quản lý vòng đời khoá; **mã kiểm tra toàn vẹn**; tài liệu luồng dữ liệu; tách môi trường theo độ nhạy cảm; **chữ ký số** khi trao đổi dữ liệu quan trọng | M·T | Mã hoá 2 lớp khi truyền; giám sát thay đổi dữ liệu; chống thất thoát dữ liệu (DLP); **nhật ký truy cập dữ liệu nhạy cảm** rà hằng quý; kiểm phân quyền hằng quý |
| 5.5 Cấu hình an toàn | Tài liệu cấu hình tăng cường bảo mật; tắt giao thức không an toàn; **tự khoá phiên**: phần mềm nghiệp vụ xử lý dữ liệu quan trọng ≤ **15 phút**, phiên quản trị ≤ **5 phút**; **khoá sau ≤ 5 lần đăng nhập sai** (phần mềm nghiệp vụ); mở khoá sau 12 giờ đến 30 ngày; có cơ chế mở khoá khẩn cấp cho quản trị | M·H | Gỡ tính năng không cần thiết; DNS tin cậy |
| 5.6 Tài khoản và truy cập | Danh sách tài khoản (loại, trạng thái, người quản lý, ngày kích hoạt, ngày vô hiệu) rà **mỗi 6 tháng**. **Mật khẩu**: có MFA thì ≥ 8 ký tự; không MFA thì ≥ 14 ký tự, đủ 4 loại ký tự; đổi mật khẩu mặc định; đổi ở lần đăng nhập đầu; có thời hạn mật khẩu. **Tài khoản quản trị**: MFA, đổi mật khẩu 2 tháng một lần, không trùng 10 mật khẩu gần nhất. Tách loại tài khoản quản trị / tác nghiệp / kỹ thuật / dịch vụ. Đổi hoặc tắt tài khoản mặc định. Mỗi tài khoản gắn đúng một người. **Vô hiệu tài khoản không hoạt động sau 45 ngày**. Quyền tối thiểu, phân tách nhiệm vụ. **MFA bắt buộc cho truy cập từ Internet**, từ bên thứ ba, và cho tài khoản quản trị | M·T | Giải pháp quản lý tài khoản đặc quyền (PAM); rà tài khoản hằng quý; quy trình cấp và thu hồi quyền |
| 5.7 Lỗ hổng | Quy trình quản lý lỗ hổng: phát hiện, chấm mức độ, chia sẻ, khắc phục; **rà quét mỗi 6 tháng**; quản lý bản vá tập trung; kiểm thử trước khi vá hệ thống chứa dữ liệu quan trọng | M·H | Rà tổng thể mỗi 6 tháng và rà tài sản quan trọng hằng quý |
| 5.8 Nhật ký | Thu thập nhật ký truy cập hệ thống (IP nguồn và đích, tài khoản, thời điểm, hành vi), nhật ký tiến trình, **nhật ký ứng dụng**, nhật ký cảnh báo; **đồng bộ thời gian (NTP)**; **SIEM** hoặc tương đương; lưu tập trung **≥ 3 tháng**; rà mỗi 6 tháng | M·H | Lưu ≥ **6 tháng**; nhật ký truy cập dữ liệu (đọc / thêm / sửa / xoá / sao chép); nhật ký ứng dụng gồm đăng nhập quản trị, lỗi và thay đổi cấu hình; **nhật ký không sửa, không xoá được**; rà hằng tháng |
| 5.9 Trình duyệt và thư | Danh sách trình duyệt được phép; lọc tên miền độc hại | T·H | — |
| 5.10 Mã độc | Phòng chống mã độc trên máy chủ; EDR nối về SIEM | H | — |
| 5.11 Sao lưu | Quy định sao lưu theo loại dữ liệu; sao lưu **tự động**; quy tắc **3-2-1**; **mã hoá** bản sao dữ liệu quan trọng; hạ tầng lưu bản sao tách khỏi môi trường vận hành; **khôi phục thử định kỳ** | H | — |
| 5.12 Hạ tầng mạng | Sơ đồ mạng; phân vùng (DMZ, vùng máy chủ nội bộ, vùng CSDL, vùng quản trị); **WAF**; chống DDoS; tường lửa CSDL; **giới hạn số kết nối đồng thời từ một nguồn**; timeout phiên; VPN cho quản trị từ xa. **Kiểm thử và nghiệm thu** trước vận hành, có bên độc lập giám sát và báo cáo nghiệm thu được phê duyệt | H·M·T | — |
| 5.13 Giám sát | Giám sát an ninh cho thiết bị, máy chủ và **ứng dụng**; SIEM; **tách cổng quản trị ứng dụng khỏi cổng dịch vụ**; chỉnh ngưỡng cảnh báo mỗi 6 tháng | H·M | — |
| 5.14 Nhân sự | Tách bộ phận vận hành / quản trị / an ninh; cam kết bảo mật; đào tạo hằng năm; thu hồi quyền khi nghỉ việc | T | — |
| 5.15 Nhà cung cấp | Danh sách nhà cung cấp, phân loại, văn bản phân định trách nhiệm (Zalo, Harbor, đám mây, MinIO…) | T | Thu thập nhật ký của nhà cung cấp dịch vụ |
| 5.16 Ứng phó sự cố | Người chủ chốt và người dự phòng; đầu mối báo cáo; phân nhóm sự cố; quy trình ứng cứu; diễn tập | T | Kênh liên lạc chính và phụ; đánh giá sau sự cố; diễn tập hằng năm |
| **5.17 Phát triển an toàn** | Quy trình SDLC an toàn, rà hằng năm; với phần mềm thuê khoán: **hợp đồng và cam kết bảo mật, giao mã nguồn**; **kênh tiếp nhận báo cáo lỗ hổng từ bên ngoài**; kiến trúc phải có: **kiểm tra đầu vào**, **kiểm tra đầu ra**, **chống các tấn công phổ biến**, **kiểm soát lỗi và thông báo lỗi**, **không để bí mật trong mã nguồn**; **kiểm tra lỗ hổng mã nguồn và thư viện bên thứ ba trước khi vận hành** | **M** | Kiểm thử trên môi trường thử nghiệm; đánh giá lại khi đổi mã hoặc kiến trúc; phân tích nguyên nhân gốc; **danh sách thành phần bên thứ ba kèm rủi ro, rà hằng quý**; **tách môi trường dev / test / prod**; đào tạo lập trình an toàn; **quản lý phiên an toàn**; dùng mô-đun đã kiểm chứng và thuật toán mạnh; **nhật ký kiểm toán hành vi người dùng**; kiểm thử xâm nhập trước vận hành |
| 5.18 Kiểm tra | Chương trình kiểm thử xâm nhập có phê duyệt phạm vi; khắc phục phát hiện; đánh giá lại biện pháp | T·M | Kiểm thử xâm nhập từ ngoài và từ trong, mỗi loại ít nhất 1 lần/năm |

---

## 3. Hiện trạng kho mã — chỉ các yêu cầu cột M

Chú thích: ✅ đạt · 🟡 một phần · ❌ chưa có · 📄 đã quyết định nhưng chưa dựng.

### 3.1 Điểm mạnh: kiến trúc đã đi đúng hướng tiêu chuẩn

| Yêu cầu | Mức | Bằng chứng |
|---|---|---|
| 5.6 Quyền tối thiểu, theo vai trò, trong phạm vi xã | ✅ | Quyền đọc từ CSDL ở mỗi request, lọc theo `tenant_id`: `service-identity/internal/store/checker.go:41-53`. Có chặn tự nâng quyền và cấp quyền mình không có: `app/danh_ba_can_bo.go:592-666` |
| 5.6 Đổi mật khẩu ở lần đăng nhập đầu | ✅ | Cột `phai_doi_mat_khau` mặc định true; middleware trả 403 `password_change_required`: `service-identity/internal/http/middleware.go:226-230` |
| 5.4.2.4 Băm mật khẩu | ✅ | argon2id, có muối, so sánh thời gian hằng, băm lại khi đăng nhập: `core/password/password.go:26-31,58,79,90` |
| Thu hồi phiên | ✅ | Kiểm `sid` trong sổ `phien` ở mỗi request: `middleware.go:142`. Đổi hoặc đặt lại mật khẩu thì thu hồi mọi phiên: `app/tai_khoan_can_bo.go:297,409` |
| Cookie phiên | ✅ | HttpOnly, Secure, SameSite=Lax, gắn host (không đặt Domain): `service-identity/internal/http/cookie.go:47-50` |
| 5.8 / 5.17 cấp 4: nhật ký kiểm toán hành vi người dùng | ✅ | `core/audit/audit.go:29-94` ghi xã, người, IP, hành động, đối tượng, thời điểm và phần chênh lệch đã che. Ghi cùng giao dịch với thay đổi (chỉ nhận `*store.ScopedTx`) |
| 5.8 cấp 4: nhật ký không sửa, không xoá được | ✅ | Trigger chặn UPDATE, DELETE và TRUNCATE: `service-identity/migrations/0002_audit_log_append_only.sql:83-87`, có ở cả 8 dịch vụ |
| 5.4.2.9 cấp 4: nhật ký truy cập dữ liệu nhạy cảm | 🟡 | Có cho việc xem người gửi phản ánh ở dạng không che (`service-petitions/internal/app/xem_nguoi_gui.go:64-89`). Các lần đọc PII đầy đủ khác chưa ghi |
| 5.17.2.3 Kiểm soát lỗi, không lộ nội bộ | ✅ | Một khuôn lỗi duy nhất; panic chỉ trả 500 chung chung: `core/httpx/edge.go:88-117`. Khoảng 10 chỗ trả `err.Error()` của domain cần rà lại (ví dụ `service-finance/internal/http/thu_chi_ngan_sach.go:871`) |
| 5.17.2.3 Không để bí mật trong mã | ✅ | Hook `secret_scan` chặn; kiểu `secret.Secret` giữ bí mật khỏi log: `core/secret/secret.go:61-111` |
| Chống SQL injection | ✅ | Tham số `$n`; kho lưu trữ có phạm vi chỉ ghép tên cột và bảng lấy từ mã: `core/store/scoped.go:49` |
| 5.13.2.3b Tách cổng quản trị khỏi cổng dịch vụ | 📄 | ADR 0048 tách host vận hành và miền tài khoản vận hành. Chưa dựng |
| Kiểm thử bảo mật tự động | ✅ | Khoảng 400 tệp `_test.go`; ca 401/403/khác xã có ở khoảng 130 tệp, ví dụ `service-identity/internal/http/an_toan_test.go` |

### 3.2 Khoảng trống — sắp theo mức nghiêm trọng

| # | Yêu cầu | Mức | Hiện trạng và bằng chứng |
|---|---|---|---|
| K1 | 5.6.2.4 **MFA** cho truy cập từ Internet và cho quản trị | ❌ | Đăng nhập chỉ có email và mật khẩu (`service-identity/internal/app/dang_nhap.go:102-136`). Không có mã TOTP nào trong kho. MFA vận hành đã quyết ở ADR 0048 nhưng chưa dựng. **Cán bộ xã vào `<xã>.vigov.vn` qua Internet, nên yêu cầu này áp cho mọi cán bộ, không riêng quản trị** |
| K2 | 5.6.2.2 Chính sách mật khẩu | 🟡 | Tối thiểu 12 ký tự, không đòi loại ký tự — một lựa chọn có chủ ý (`core/password/password.go:42-44`). Theo tiêu chuẩn: **không MFA thì phải ≥ 14 ký tự và đủ 4 loại**; có MFA thì 8 là đủ. Chưa có thời hạn mật khẩu, chưa có lịch sử 10 mật khẩu, chưa có chu kỳ 2 tháng cho quản trị |
| K3 | 5.5.2.2 Khoá sau ≤ 5 lần đăng nhập sai | ❌ | Đăng nhập sai chỉ ghi một dòng log (`dang_nhap.go:206-209`). Khoá thủ công thì đã có (`lockout`) |
| K4 | 5.12.2.4e Giới hạn kết nối; chống dò mật khẩu | ❌ | Không có bộ giới hạn tần suất nào trong kho. Mã tuyến đăng nhập giao việc này cho "rate limit" (`service-identity/internal/http/routes.go:602`), nhưng thứ đó không tồn tại. Ingress cũng không có chú thích `limit-rps` |
| K5 | 5.5.2.2 Tự khoá phiên khi không dùng (≤ 15 phút nghiệp vụ, ≤ 5 phút quản trị) | ❌ | Chỉ có hạn tuyệt đối 12 giờ, quản trị và cán bộ như nhau (`service-identity/internal/store/phien.go:85`). Cột `dung_gan_nhat` được ghi nhưng không ai kiểm. Phiên công dân mặc định 30 ngày (`.env.example:172-174`) |
| K6 | 5.6.2.3 Vô hiệu tài khoản không hoạt động 45 ngày | ❌ | Đã có dữ liệu `dang_nhap_gan_nhat` nhưng chưa có tác vụ nào dùng nó |
| K7 | 5.6.2.3 Tài khoản mặc định | 🟡 | Có tài khoản gieo sẵn tên `admin` (`service-identity/internal/store/quan_tri_mac_dinh.go:30`). Đã giảm rủi ro: gieo từ biến môi trường, tắt khi biến rỗng, bắt đổi mật khẩu |
| K8 | 5.6.2.1 Danh sách tài khoản | 🟡 | `GET /api/v1/staff` không có loại tài khoản, ngày kích hoạt, ngày vô hiệu. Quản trị chỉ là một vai trò, không phải một loại tài khoản |
| K9 | 5.17.2.3 Chống tấn công phổ biến — header bảo mật | ❌ | Không có CSP, HSTS, X-Frame-Options, nosniff, Referrer-Policy (`web-admin/next.config.ts` chỉ tắt `poweredByHeader`). Chống CSRF chỉ dựa vào SameSite=Lax |
| K10 | 5.4.2.4 Mã hoá khi truyền giữa các dịch vụ | ❌ | gRPC nội bộ dùng `insecure.NewCredentials()` (`core/platformclient/directory.go:80`, `core/identityclient/identityclient.go:93`). Biên duy nhất là NetworkPolicy, mà NetworkPolicy chưa được áp (K13) |
| K11 | 5.4.2.4 Mã hoá bí mật lưu trữ theo xã | 📄 | ADR 0009 (mã hoá phong bì) chưa dựng: không có `core/crypto`, không có lời gọi `aes.NewCipher` nào |
| K12 | 5.17.2.4 **Rà lỗ hổng thư viện bên thứ ba** trước vận hành; cấp 4: SBOM | ❌ | Không có govulncheck, npm audit, osv-scanner, trivy hay Dependabot. `golangci-lint` có tiền tố `-` nên lỗi bị bỏ qua (`Makefile:101`), và máy Jenkins chưa cài nó (`deploy/README.md:566`) |
| K13 | 5.12 Phân vùng mạng, TLS, WAF | 🟡 (trên giấy) | `deploy/base` có NetworkPolicy, securityContext và TLS, nhưng README nói **không áp** lên cụm; cụm thật dựng tay trong Rancher (`deploy/README.md:6-13`). Chiều ra của NetworkPolicy tới CSDL để `0.0.0.0/0`. Không có WAF |
| K14 | 5.17 cấp 4: tách môi trường dev / test / prod | 🟡 | `ENV` bắt buộc và từ chối `DANGEROUS_AUTH_BYPASS` khi prod (`core/config/config.go:612-614`). Nhưng staging và prod chung cụm, chung kubeconfig; job dịch vụ triển khai thẳng `vigov-prod` |
| K15 | 5.8 Nhật ký ứng dụng tập trung ≥ 3 tháng, SIEM, NTP | ❌ | Log JSON ra stdout. Không có middleware ghi nhật ký truy cập, không có bộ chuyển log, không có cấu hình thời gian lưu. Nhật ký kiểm toán nghiệp vụ tốt (§3.1), nhưng nó không thay được nhật ký an ninh |
| K16 | 5.11 Sao lưu và khôi phục | ❌ | Không có tác vụ sao lưu PostgreSQL hay MinIO, không sao lưu KEK (ADR 0009 đòi có trước khi phát hành), không khôi phục thử |
| K17 | 5.4.2.5 Toàn vẹn; 5.10 quét mã độc tệp tải lên | 🟡 | `core/storage` tính được SHA-256 và `core/malwarescan` (ClamAV, fail closed) đã có, nhưng **chưa dịch vụ nào gọi**. Hiện chưa có tuyến tải tệp nào |
| K18 | 5.4.2.8 Chữ ký số khi trao đổi dữ liệu quan trọng | ❌ | Chưa có. Liên quan văn bản đến / đi — cần ADR |
| K19 | 5.17.2.2 Kênh tiếp nhận báo cáo lỗ hổng; 5.16 ứng phó sự cố | ❌ | Không có `SECURITY.md`, `security.txt`; `kb/40-runbooks/` chưa có |
| K20 | 5.4.2.1 Phân loại 4 mức nhạy cảm | 🟡 | Luật 3 đã định nghĩa "dữ liệu cá nhân" và cách che, nhưng chưa có bảng gán mỗi thực thể vào một trong 4 mức. `kb/30-indexes/data-ownership.json` là chỗ tự nhiên để sinh ra bảng ấy |

---

## 4. Bộ não `.claude` so với tiêu chuẩn

### 4.1 Đã thiết kế theo hướng này — một phần, và phần ấy mạnh

Bộ não được dựng quanh **cô lập giữa các xã, dữ liệu cá nhân và hồ sơ lưu trữ**. Các mảng ấy
trùng khá sát với 5.4, 5.6 và 5.17 của tiêu chuẩn:

| Tiêu chuẩn | Bộ não đang giữ bằng |
|---|---|
| 5.4 Kiểm soát truy cập tài sản thông tin | Luật 1, 4 + `tenant_scope_guard`, `citizen_scope_guard` |
| 5.6 Quyền tối thiểu, khai quyền rõ ràng | Luật 5 + `rbac_guard`, `quyen_key_guard`, `tools/check_quyen.py` |
| 5.8 / 5.17 cấp 4: nhật ký kiểm toán hành vi | Luật 6 + `audit_guard`, `audit_actor_guard` |
| 5.17.2.3 Không để bí mật trong mã | Luật 8 + `secret_scan`; `session_start` báo cờ nguy hiểm |
| 5.4 Không lộ dữ liệu nhạy cảm qua log | Luật 3 + `pii_guard` |
| 5.11 / toàn vẹn hồ sơ | Luật 7 + `data_safety_guard` |
| 5.12 Cấu hình hạ tầng kiểm toán được | Luật 11 + `env_contract_guard` |
| 5.17 cấp 4: xác minh trước khi coi là xong | `stop_verify_guard`, `make check` |

### 4.2 Chưa thiết kế theo hướng này

> **Cập nhật 28/09/2026 (sau lần đo):** đợt 1 đã dựng luật 13, `security_guard`,
> `tools/check_security.py`, `make vuln`, hai skill, `/review-security`, agent
> `security-reviewer` và câu hỏi mở #35–#39 — theo §5.1 (B1–B6). Bảng dưới là hiện trạng
> **trước** đợt 1; việc còn lại: sổ `_chung`, mục `luat-13-dot-2`.

**Lúc đo, không tệp nào trong `.claude/` hay `kb/` nhắc tới TCVN 14423 hay cấp độ an toàn.** Các mảng
dưới đây không có luật, không có skill, không có hook:

| Mảng | Nhóm TCVN | Hiện trạng trong bộ não |
|---|---|---|
| Xác thực mạnh: MFA, khoá sau đăng nhập sai, khoá phiên khi không dùng, thời hạn và lịch sử mật khẩu, 45 ngày | 5.5, 5.6 | `skills/session-and-token` chỉ nói token, `sid`, argon2id. Không có con số nào của tiêu chuẩn |
| Giới hạn tần suất / chống dò | 5.12.2.4e | Không có. `rest_api_guard` đòi khai **chống trùng request** nhưng không đòi khai **giới hạn tần suất** |
| Header bảo mật, CSRF | 5.17.2.3 | `skills/nextjs-multi-tenant` không nhắc |
| Rà lỗ hổng thư viện, SBOM, SAST phải chặn | 5.17.2.4, 6.16.2.4 | `make check` không có bước nào như vậy |
| Mã hoá khi truyền (mTLS nội bộ) | 5.4.2.4 | Không có. `insecure.NewCredentials()` không bị hook nào chú ý |
| Nhật ký an ninh (khác nhật ký kiểm toán): truy cập, thời gian lưu, NTP | 5.8 | Luật 6 chỉ phủ nhật ký nghiệp vụ |
| Sao lưu, khôi phục thử | 5.11 | Luật 7 bất biến 4 nói "chạy sau bản sao lưu đã kiểm", nhưng không có gì kiểm việc sao lưu tồn tại |
| Kênh báo lỗ hổng, runbook sự cố, kiểm thử xâm nhập | 5.16, 5.17.2.2, 5.18 | Không có |
| `/review-compliance` | cả tiêu chuẩn | Có 4 nhóm (thuật ngữ, tiếng Việt, tiếp cận, pháp lý). Không có nhóm an ninh mạng |

---

## 5. Đề xuất cập nhật

### 5.1 Bộ não — theo đúng quy ước "luật = bất biến kiểm được bằng máy, còn lại là skill"

| # | Đề xuất | Tầng | Lý do chọn tầng |
|---|---|---|---|
| B1 | Skill mới `security-baseline`: bảng con số của §2 cột M (15/5 phút, 5 lần, 45 ngày, 14/8 ký tự, 3/6 tháng log…), header bảo mật, mTLS, danh mục thư viện; trỏ về tệp này | skill | Phần lớn là **giải pháp** và con số có thể đổi theo cấp độ, không phải bất biến |
| B2 | Mở rộng `skills/session-and-token`: MFA, khoá sau N lần sai, khoá phiên khi không dùng, thời hạn và lịch sử mật khẩu, vô hiệu 45 ngày | skill | Đúng chỗ, đang thiếu |
| B3 | Mở rộng `rest_api_guard`: tuyến **không cần xác thực** (`Public`, đăng nhập, cầu phiên công dân) phải khai giới hạn tần suất, cùng khuôn với khai chống trùng (`idem.KhongCan("<lý do>")`) | hook (advisory) | Quyết định được bằng máy: có hay không có lời khai |
| B4 | Hook hoặc bước `make check`: cấm `insecure.NewCredentials()` ngoài danh sách có lý do (`// @insecure-transport: <lý do>`) | hook | Một chuỗi xác định, có lối thoát ghi được — đúng mẫu của luật 1 `@cross-tenant` |
| B5 | Thêm vào `make check`: `govulncheck ./...`, `npm audit --omit=dev` (hoặc osv-scanner); bỏ tiền tố `-` của `golangci-lint` và thêm cấu hình có gosec | tools | Kiểm toàn kho, không phải một tệp — cùng lý do luật 5 cần `tools/check_quyen.py` |
| B6 | Thêm nhóm 5 **"An ninh mạng — TCVN 14423"** vào `/review-compliance` cho những gì máy không quyết được (header, phân loại mức nhạy cảm, kênh báo lỗ hổng, sao lưu) | command | Đúng vai của lệnh ấy |
| B7 | **Chưa đề xuất luật 13.** Chỉ nên thành luật khi chốt được một bất biến có hook (ví dụ "mọi đường truyền giữa dịch vụ đều mã hoá") | — | Luật không hook được sẽ trôi (`.claude/README.md` §*Brain invariant*) |

### 5.2 Mã nguồn — ưu tiên trước khi đưa xã đầu tiên vào dùng thật

| Ưu tiên | Việc | Khoảng trống |
|---|---|---|
| P0 | Giới hạn tần suất ở đăng nhập, đổi mật khẩu, cầu phiên (Redis, theo IP và theo tài khoản) + khoá sau 5 lần sai + chú thích ingress | K3, K4 |
| P0 | Header bảo mật trong `next.config.ts` hoặc ingress | K9 |
| P0 | `govulncheck` / `npm audit` / trivy trong CI; SAST phải chặn | K12 |
| P0 | Sao lưu PostgreSQL + MinIO + KEK, khôi phục thử một lần | K16 |
| P1 | MFA TOTP cho cán bộ (bắt đầu từ vai trò có `admin.user`) — gỡ luôn được K2 nhờ ngưỡng 8 ký tự | K1, K2 |
| P1 | Khoá phiên khi không dùng, dựa trên cột `dung_gan_nhat` đã có | K5 |
| P1 | Tác vụ vô hiệu tài khoản 45 ngày; thêm ngày kích hoạt và ngày vô hiệu vào danh sách tài khoản | K6, K8 |
| P1 | Áp `deploy/base` lên cụm thật bằng `apply -k`; tách kubeconfig prod | K13, K14 |
| P1 | Chuyển log tập trung, lưu ≥ 3 tháng (≥ 6 nếu cấp 4); middleware ghi nhật ký truy cập | K15 |
| P2 | mTLS gRPC nội bộ; dựng ADR 0009; nối SHA-256 và ClamAV vào tuyến tải tệp đầu tiên | K10, K11, K17 |
| P2 | `SECURITY.md` + `security.txt`; runbook sự cố đầu tiên | K19 |

### 5.3 Câu hỏi cho chủ dự án — không tự quyết

| # | Câu hỏi | Vì sao phải hỏi |
|---|---|---|
| Q1 | ViGov được đề xuất **cấp độ mấy**? | Quyết định lưu log 3 hay 6 tháng, kiểm thử xâm nhập, DLP, SBOM hằng quý |
| Q2 | Mật khẩu: giữ "12 ký tự, không đòi loại ký tự" và **thêm MFA**, hay chuyển sang 14 ký tự đủ 4 loại? | Lựa chọn hiện tại là có chủ ý và **không đạt 5.6.2.2 nếu không có MFA** |
| Q3 | MFA cho **mọi cán bộ** (vì họ vào qua Internet), hay chỉ quản trị kèm VPN / giới hạn IP cho số còn lại? | 5.6.2.4 cho phép một trong hai; chi phí và trải nghiệm ở cấp xã rất khác nhau |
| Q4 | Thời gian khoá phiên khi không dùng: 15 phút cho cán bộ, 5 phút cho quản trị — xã chấp nhận không? Có cấu hình theo xã không? | Tiêu chuẩn cho phép "dựa trên đánh giá rủi ro" |
| Q5 | Phiên công dân 30 ngày trên Zalo có phải là "tài khoản dịch vụ" chịu 5.5.2.2 không? | Tiêu chuẩn không nói rõ; một quyết định sai ở đây chạm trải nghiệm của mọi công dân |
| Q6 | Ai chịu các nhóm T và H (5.1, 5.2, 5.9, 5.10, 5.14–5.16): ViHAT hay chủ quản? | Phân định trách nhiệm theo 5.15 |
| Q7 | Chữ ký số cho văn bản đi và đến: dùng nhà cung cấp nào? | 5.4.2.8; cần ADR |
