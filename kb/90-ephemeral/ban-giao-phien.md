---
id: ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 9307362
expires: 2026-12-16
owns_facts:
  - "trạng thái thi công tại 2026-09-17 và việc kế tiếp phải làm"
---

# Bàn giao phiên — cập nhật 2026-09-17

**Đọc tệp này SAU `kb/INDEX.yaml` và tầng `always_load`, không thay thế chúng.**
Nó chỉ trả lời: *đã quyết gì, đang kẹt ở ai, và cạm bẫy nào đã tốn thời gian của người trước.*

Viết bằng `/handover`. **MỘT tệp, ghi đè trọn vẹn mỗi lần** — tên tệp không mang ngày, vì một
tên mang ngày mời gọi đúng một thứ: tệp bàn giao thứ hai, và hai tệp bàn giao là hai tệp mâu
thuẫn mà người đọc không biết tin cái nào.

Hết hạn **2026-12-16**. Sau ngày đó tin `git log`, đừng tin tệp này.

---

## 1. Đã làm

**Không có danh sách commit ở đây.** `git log --oneline 96574b4..HEAD` trả lời câu đó chính
xác hơn và không bao giờ lệch. Dưới đây chỉ những thứ `git log` không trả lời được.

### Quyết định đã chốt với người dùng — và nơi ghi

| Quyết định | Ghi ở |
|---|---|
| Tên service tiếng Anh, tên bảng tiếng Việt · SLA đếm bằng **giờ làm việc** · vòng đời phiếu là cấu hình theo xã | ADR 0001 · 0007 · 0008 |
| Bí mật theo xã: envelope encryption, KEK ngoài CSDL · chỉ PostgreSQL · `MODULUS 32` | ADR 0009 · 0010 |
| URL path tiếng Anh, enum giữ tiếng Việt không dấu · tên sự kiện `<miền>.<việc>.v1` | ADR 0011 |
| Xã đi trong metadata gRPC `x-tenant-id` · `platform` sập = **404** | ADR 0012 |
| Append-only cưỡng chế bằng **trigger** · migration áp lúc khởi động, hỏng thì service không chạy | ADR 0013 |
| Hợp đồng REST **sinh từ khai báo route trong Go**, không viết tay | ADR 0014 |
| **Bố cục phẳng**: mỗi đơn vị triển khai là một thư mục cấp một, ngang hàng `.claude/`, `kb/`, `core/` | ADR 0015 |
| **Một Go module cho mỗi đơn vị triển khai**, `core` được `replace` theo đường dẫn; `go.work` **không** vào ảnh Docker | ADR 0016 |
| Tên trường hợp đồng gọi theo **thứ dữ liệu LÀ**, không theo nhãn màn hình | ADR 0017 |
| Kênh công dân: OA xác thực tách khỏi OA thông báo · QR ghép phiên · Zalo chỉ xác nhận **số thuộc tài khoản nào** | ADR 0018 · 0019 · 0020 |
| Rìa kênh công dân: xã đến **từ phiên**, không từ `Host`; thiếu khai lớp người dùng = **từ chối** | ADR 0021 · 0022 |
| Thuật ngữ kênh công dân; `/api/v1/communes/current` nằm cạnh `/api/v1/communes` và khác nhau chỗ nào | ADR 0023 |

### Quyết định chưa thành ADR, nhưng đã chốt miệng với người dùng

| | |
|---|---|
| Tiền tố **`vigov`** cho tên ảnh và module. Người dùng sẽ đổi tên dự án sau khi v2 thay được bản gốc | — |
| Jenkins dùng **docker CLI trên agent**, không Kaniko | — |
| Phạm vi hạ tầng dừng ở **Dockerfile + Jenkinsfile**. Cụm k8s do đội devops phụ trách | — |
| Trần `always_load` nâng lên **25000 token** (chốt 16/09/2026) | `kb/INDEX.yaml` |
| **Lời khai cư trú đã được cán bộ xác thực vẫn KHÔNG đủ làm căn cứ cho hành vi có hệ quả pháp lý.** Cán bộ xác thực được *"người này khai thế"*, không xác thực được *"người đang cầm điện thoại chính là người đó"* | ADR 0023 |

### Thứ cố ý KHÔNG dựng, và vì sao

| Không dựng | Vì sao |
|---|---|
| `/tong-quan` và `/nhiem-vu/so-tay` | Cần API thống kê chưa tồn tại. Một bảng điều khiển với số bịa ra là thứ lãnh đạo đọc rồi báo cáo lên trên |
| Mọi tuyến **ghi** cho cán bộ | Cả cụm đang chờ khách chốt — xem §3 |
| Bất kỳ scaffolding nào cho tuyến ghi ấy | Một tuyến ghi viết dở trông y hệt một quyết định ai đó đã ra |
| Biến môi trường API nội bộ cho web-admin | Chưa biết cụm k8s có chặn không — xem §3, hàng hạ tầng |
| **CHECK `co_tai_khoan => mat_khau_hash <> ''`** | Lập luận đầy đủ ở `service-identity/migrations/0003_…sql:189–217`. Viết ràng buộc ấy bây giờ làm **một trong ba phương án của câu mở #9 không cài đặt được nữa** — tức quyết hộ khách. Giá đảo ngược bất đối xứng: thêm sau là một dòng migration, gỡ sau là `ALTER TABLE` trên bảng đang chạy. *Đọc kèm §5: đúng khối chú thích này là thứ `drift_guard` từng buộc tội nhầm.* |
| **Bảng `tinh_thanh` ship RỖNG** | Cơ cấu 6 thành phố + 28 tỉnh thì chắc; **dạng viết** từng tên thì không. Xem §3 |

---

## 2. Việc kế tiếp — theo đúng thứ tự

### 2.1 Chạy migration thật ← **BẮT ĐẦU TỪ ĐÂY**

**Toàn bộ tầng SQL của kho này chưa từng chạy một lần nào.** Đó là khoảng trống lớn nhất, và
nó không tự lộ ra: các suite tích hợp **tự bỏ qua khi thiếu `VIGOV_TEST_DSN` và cả gói vẫn báo
`ok`** (§5). Xanh ở cổng kiểm **không** có nghĩa là SQL đã chạy.

Máy này nay **có Docker chạy được**, nên không cần chờ máy chủ test của chủ dự án nữa: dựng một
PostgreSQL cục bộ, đặt `VIGOV_TEST_DSN`, chạy `go test -count=1 ./...` trong từng module.

Phải tự mắt nhìn thấy, không suy ra:

| # | Phải thấy |
|---|---|
| 1 | Mọi tệp migration áp được, theo đúng thứ tự, trên một CSDL trống |
| 2 | Trigger append-only **bắn thật** khi gõ thẳng tên partition, không chỉ khi gõ tên bảng cha |
| 3 | Mảnh `PARTITION BY HASH` tồn tại đủ — thiếu một mảnh thì INSERT lỗi và **bản ghi nghiệp vụ rollback toàn bộ** |
| 4 | Bài test canh `co_tai_khoan` chạy thật, chứ không phải skip rồi báo `ok` |
| 5 | Sáu bảng kênh công dân vừa thêm áp được cùng mười bảy tệp cũ |

### 2.2 Kênh công dân giai đoạn 2 — mới xong một phần ba

Hợp đồng, rìa (`core/httpx/citizen.go`) và sáu bảng đã có. **Chưa có kho đọc, chưa có route,
chưa có màn hình nào.**

Phần kho đọc nằm ở `service-identity/internal/store/` — tức **vùng khác với vùng đã viết
migration**. Đó là đường nối phải bắc, không phải một chi tiết bàn giao.

### 2.3 `service-identity/internal/store/crosstenant/` — **chưa tồn tại**

Ba truy vấn đọc chéo xã **đã được nêu tên trong migration** nhưng chưa có chỗ ở. Chúng phải nằm
gọn trong một gói riêng, vì đó là cách duy nhất khiến "đọc chéo xã" thành một danh sách **đếm
được** thay vì một thói quen rải khắp kho (luật 1, cấm #6: mỗi truy vấn chéo mang
`// @cross-tenant: <lý do>`).

Liên quan trực tiếp tới cảnh báo `drift_guard` về câu mở **#4** — xem §5.

### 2.4 Server gRPC của `identity` — **chưa tồn tại**

Chưa có `service-identity/internal/grpc`. Khi dựng thì dùng lại interceptor hai đầu trong
`core/grpcx`, và **đọc ADR 0012 trước khi thêm bất kỳ RPC nào**.

### 2.5 `platform`: `danh_muc` + `loi_he_thong`

Chưa có. Không bị chặn bởi câu hỏi nào.

### 2.6 Màn hình xác thực lời khai cư trú

ADR 0023 đã chốt nghiệp vụ, nhưng **tuyến chưa viết được** — chờ câu hỏi #20 (§3). Hai điều
phải đúng ngay từ bản đầu, vì sửa sau là sửa chữ trên màn hình của một cơ quan nhà nước:

1. **Nhãn phải nói "xác nhận LỜI KHAI", không phải xác nhận nhân thân.** Một nút "Xác nhận
   thường trú" đứng trơ sẽ được cán bộ hiểu là mình đang cấp một xác nhận hành chính.
2. **`bị từ chối` là trạng thái riêng**, giữ nguyên lời khai và giữ lý do. Lý do là **văn bản
   cán bộ viết cho công dân đọc**, không phải mã lỗi nội bộ — nên nó là **trường nghiệp vụ bắt
   buộc**, không phải một textarea tuỳ chọn. Một ô tuỳ chọn thì thực tế sẽ rỗng, và công dân
   nhận về một "bị từ chối" không lý do, tức đúng cái im lặng luật 10 cấm.

### 2.7 Nộp Mini App cho Zalo duyệt — **chặn bởi thứ không nằm trong kho mã**

`citizen-app/` giai đoạn 1 (giới thiệu ViHAT Software) đã xong và xanh. Nó là thứ duy nhất
trong kho **sẵn sàng giao ra ngoài**, nhưng chưa nộp được, và hai thứ còn thiếu đều không phải mã:

1. **Logo/icon, ảnh chụp màn hình, mô tả store.** Zalo bắt buộc. Kho chưa có tệp ảnh nào.
2. **`citizen-app/app-config.json` phải đối chiếu Developer Console.** Tên khoá viết từ **nguồn
   thứ cấp** — tài liệu Zalo render bằng JS nên không đọc trực tiếp được. Thư mục build đang là
   `dist/` (mặc định Vite) trong khi `zmp-cli` thường dùng `www/`. Sai khoá là hồ sơ bị trả về.

Ba câu nên hỏi Zalo **cùng lúc lúc nộp**, vì cả ba đang là giả định: ràng buộc **1 Mini App ↔ 1
OA** (cả ADR 0018 đứng trên nguồn thứ cấp) · app duyệt dạng hồ sơ doanh nghiệp sau này gắn dịch
vụ công có phải xác thực lại không · tham số deep link có tới app khi app đang chạy nền không.

### 2.8 `petitions`

**Chỉ bắt đầu sau khi có `lich_lam_viec` + `ngay_nghi_le` theo xã** — ADR 0007. Đếm hạn bằng
giờ hành chính mà thiếu lịch của xã thì mọi con số hạn đều sai, và sai theo hướng không ai
thấy cho tới lúc báo cáo lên trên.

---

## 3. Đang bị chặn — và chặn bởi ai

`kb/00-foundation/open-questions.json` là nguồn chuẩn. **14 câu đang OPEN.** Dưới đây là cụm
và thứ chúng chặn — không chép lại nội dung câu hỏi.

| Chặn bởi | Câu | Không làm được gì cho tới khi chốt |
|---|---|---|
| **Khách** | #9 #17 #18 | Mật khẩu đầu tiên của cán bộ mới · tự đặt lại mật khẩu · `Ghi nhớ đăng nhập`. **Toàn bộ luồng cấp tài khoản** |
| **Khách** | #10 #13 #14 | Khoá hay xoá cán bộ · chặn mất quản trị viên cuối cùng · tự thao tác lên chính mình. **Toàn bộ tuyến ghi của danh bạ cán bộ** |
| **Khách** | #11 #12 | Che hay không che số di động cán bộ, và ai quyết việc công khai lên Mini App. Chặn cả cột hiển thị lẫn quyền |
| **Khách** | #15 #16 | Mã cán bộ do ai đặt · `dien_thoai` và `di_dong` là một trường hay hai. **Chặn schema**, nên đắt hơn các câu khác |
| **Khách** | #19 #20 | Công dân sửa lời khai đã xác thực · tên khoá quyền cho việc xác thực. Chặn §2.6 |
| **Khách** | #1 #4 | Sáp nhập/chia tách xã · cấp huyện-tỉnh xem tổng hợp tới mức nào. #4 chặn §2.3 |
| **Khách** | — | **34 tên tỉnh/thành ở dạng viết chính thức.** Người dùng đã chốt nhà cung cấp seed sẵn, xã chỉ được chọn — nhưng bảng `tinh_thanh` ship **RỖNG có chủ đích**: cơ cấu 6 thành phố + 28 tỉnh thì chắc, *dạng viết* từng tên thì không (`Đà Nẵng` hay `Thành phố Đà Nẵng`). Cột này in thẳng ra màn hình công dân nên dạng viết **là** nội dung. Đã thử ba nguồn chính phủ, cả ba render phía client |
| **Chủ dự án** | — | Mật khẩu máy chủ test, **nếu** muốn chạy trên máy ấy thay vì Postgres cục bộ. Không nằm trong kho mã (luật 8) |
| **Chủ dự án** | — | Ba câu **mã hoá khi lưu** vẫn chưa có đáp: khoá nằm ở biến môi trường hay nguồn khác · khoá có phải một danh sách xoay vòng được không · có mã hoá `ho_ten` không (mã hoá thì **mất khả năng sắp xếp theo tên ở mọi màn hình**) |
| **Chủ dự án / kiến trúc** | — | **Chính sách mã hoá cho vùng xuyên xã.** ADR 0009 là envelope encryption **theo xã**; `dinh_danh_cong_dan` không thuộc xã nào nên **không có DEK nào bọc nó**. Đây là chính sách KHÔNG ÁP DỤNG, không phải chưa cài. Người dùng đã chốt: ghi thành khoảng hở có tên, chưa thiết kế gì. Thứ đang bảo vệ nó là luật 3 + phân quyền CSDL |
| **Hạ tầng** | — | Chạy 10 Jenkinsfile trên Jenkins thật |
| **Hạ tầng** | — | **Đã chứng minh, không còn là suy đoán:** tiến trình Next.js gọi `https://<Host>/api/v1/communes/current` bằng **tên miền công khai**. Trong cụm có split-horizon DNS hoặc chặn egress thì **mọi yêu cầu 500**. Cần đội devops xác nhận; nếu chặn thì phải có biến môi trường gốc API nội bộ, và `web-admin` hiện **không có tệp mẫu env** nào để thêm vào |

---

## 4. Phiên song song

Hai phiên chạy cùng kho trong ngày 17/09. **Cả hai đã đẩy hết** — cây làm việc sạch tại
`9307362` — nhưng **`vigov-v2-92` vẫn đang mở** lúc tệp này được viết, nên phiên sau đọc tới
đây phải tự kiểm chứ đừng tin dòng này. Ghi lại phạm vi từng phiên vì nó giải thích vì sao một
số vùng có chú thích dày hơn hẳn phần còn lại:

| Phiên | Giữ |
|---|---|
| `vigov-v2-92` | `proto/` · `core/` · `tools/apidoc/` · `service-platform/` · `service-identity/migrations/` · `citizen-app/` · `.claude/hooks/drift_guard.py` · `kb/10-decisions/0018`–`0023` · `kb/00-foundation/{ubiquitous-language,open-questions}` |
| `vigov-v2-65` | `service-identity/internal/**` · `web-admin/**` · `tools/{check_build,check_brain,test_hooks}.py` · `.claude/hooks/stop_verify_guard.py` · `.claude/commands/` · `makefile` · `.dockerignore` · `*/Dockerfile` · `*/Jenkinsfile` |

**Chỗ hai phạm vi chạm nhau, và là việc kế tiếp thật:** sáu bảng kênh công dân đã xong ở
`service-identity/migrations/`, nhưng kho đọc cho chúng — và `store/crosstenant/` — nằm ở
`service-identity/internal/store/`, vùng bên kia. Xem §2.2 và §2.3.

**Cách hai phiên chia việc, đáng giữ lại vì nó chạy được:** phạm vi tuyên bố bằng đường dẫn và
nhắc lại mỗi lần đổi · phát hiện trong vùng người khác thì **báo kèm bằng chứng, không tự sửa**
· `git add` theo đường dẫn tường minh, **không bao giờ `-A`**. Ba quy ước ấy sinh ra từ ba lần
suýt giẫm chân, không phải từ lý thuyết.

> **Mục này phải xoá hẳn khi phiên sau chỉ có một mình**, chứ không để lại. Một mục §4 trỏ vào
> những phiên đã kết thúc còn tệ hơn không có mục §4.

---

## 5. Cạm bẫy đã gặp — đọc để khỏi mất thời gian lại

Một nửa bảng này có chung một hình dạng: **thứ trông như biện pháp mà không phải biện pháp.**
Gặp cái tiếp theo cùng dạng thì đừng vá riêng nó — hỏi cả lớp đó còn ở đâu nữa.

**Riêng bố cục phẳng (ADR 0015/0016) sinh ra cả một mẻ, và không cái nào kêu.** Mọi cơ chế
nhận diện mã theo **tên thư mục** câm đi cùng lúc: sáu hook khớp theo đoạn `/services/` ·
`drift_guard` quét danh sách trắng bảy thư mục mà sau đó chỉ còn **một** tồn tại ·
`.dockerignore` loại trừ `apps` · `check_brain` đếm tên hook thay vì đường dẫn ·
`stop_verify_guard.CODE_DIR` thiếu `/tools/`. **Năm ca.** Bài học chung, đắt hơn từng ca riêng
lẻ: **danh sách trắng tên thư mục là hình dạng sai cho kho này.** Đơn vị triển khai tiếp theo
thêm vào sẽ lại không được quét, và không có gì đỏ vào ngày ấy. Dùng **danh sách loại trừ** —
nó hỏng theo chiều ngược lại, tức quét thừa vài mili giây thay vì quét thiếu.

| Vấn đề | Cách xử |
|---|---|
| **`drift_guard` mù hẳn mà cổng kiểm vẫn 7/7** — **ĐÃ VÁ (`bbaad10`, `f9a2f45`)** | Hook DUY NHẤT canh chuyện "mã đang lặng lẽ quyết hộ khách một câu hỏi mở" — đúng lớp lỗi CLAUDE.md nói đã làm dự án trước mất 20–28 ngày. Từ ADR 0015 nó không đọc một dòng service, `core/`, web hay migration nào. **`check_brain` vẫn xanh vì nó kiểm "mỗi luật có NÊU TÊN một hook", không kiểm "hook ấy có NHÌN THẤY gì không"** — xem §6. Nay đổi sang **danh sách loại trừ**, và phần thuần (`duoc_quet`, `nen_canh_bao`) có ca test. **Giá trị còn lại của dòng này không nằm ở bản vá mà ở chỗ: nó câm hàng tháng trời và không ai nghe thấy gì.** |
| **Guard vừa sống dậy bắn ngay một ÂM TÍNH GIẢ — và trúng tệp lập luận cẩn thận nhất kho** — **ĐÃ VÁ (`9307362`)** | Bản vá đầu đếm cả tín hiệu nằm trong **chú thích**, nên nó khớp dòng `--   CHECK (NOT co_tai_khoan OR ...)` trong `0003_nguoi_dung_co_tai_khoan.sql:215` — một **mẫu đã bị chú thích**, nằm trong khối *"NO CHECK CONSTRAINT. Decided, not overlooked"* mà chính nó giải thích rằng viết ràng buộc ấy bây giờ là quyết hộ khách. Migration làm **đúng** điều luật 9 đòi và bị guard phạt **vì đã giải thích lý do**. Bảng này đã ghi hệ quả ở dòng khác: **hook nhiễu là hook bị tắt** — và không gì làm người ta tắt nhanh bằng một guard câm hàng tháng rồi mở miệng ra là buộc tội nhầm |
| **Bản vá cho âm tính giả suýt lặp lại đúng lỗi nó đang vá** | Cách hiển nhiên — bỏ sạch chú thích rồi đếm — làm **#9 biến mất và #4 cũng biến mất**, vì mẫu tín hiệu duy nhất của #4 là `@cross-tenant`, mà luật 1 cấm #6 **bắt buộc** dấu ấy nằm trong chú thích: Go và SQL không có chỗ nào khác đặt nó. Hook sẽ vĩnh viễn không báo được #4 trong khi vẫn trông như đang canh. Bản đã đẩy phân biệt hai thứ: **chú thích là văn xuôi, dấu khai báo thì không** — `@cross-tenant`, `@entity`, `@scope` sống sót, văn xuôi và mẫu đã chú thích thì không |
| **Hai cảnh báo còn lại là việc thật — xử bằng cách trả lời câu hỏi, không phải nới ngưỡng** | Câu mở **#4**: đã có đường đọc chéo xã trong mã trong khi khách chưa chốt cấp tỉnh xem tổng hợp tới mức nào (→ §2.3). Câu mở **#19**: bảng quan hệ công dân↔xã vừa ra đời trong khi câu "công dân sửa lời khai đã xác thực thì sao" còn mở |
| **Test tích hợp SKIP nhưng cả gói vẫn báo `ok`** | Dạng nặng nhất. Đã kiểm chứng: đột biến một dòng vào **mã sản phẩm** (`AND nd.co_tai_khoan`) mà không có gì đỏ. Trước khi tin "có test canh chỗ này", **gỡ thử dòng đó ra và xem có đỏ không** |
| **Kết quả grep âm tính KHÔNG phải bằng chứng vắng mặt** | Một agent báo "grep không có kết quả nào" ⇒ kết luận web không gọi tuyến ấy ⇒ **bảng thuật ngữ bị sửa yếu đi theo**. Thực tế có gọi: một chỗ là template literal có nội suy, một chỗ gán qua biến có kiểu sinh chứ không nằm trong lời gọi `fetch`. Loại sai này không ai soi ra **vì nó trông như thận trọng** |
| **`--build-arg` cho một `ARG` không khai bị Docker bỏ qua lặng lẽ** | Một lượt "đột biến" để thử rào chắn sẽ **xanh** và trông như rào đã bắn. Muốn thử thật thì sửa `ENV` trong chính Dockerfile. Áp cho mọi phép thử rào chắn trong ảnh |
| **Rào chắn đứng thành `RUN` riêng chỉ đo môi trường tại thời điểm ấy** | Rào `NEXT_PUBLIC_*` từng đứng trên `npm run build`; một dòng `ENV` chen vào giữa thì nó không thấy — mà đó đúng là chỗ người ta sẽ thêm. Gộp vào **cùng một `RUN`** với lệnh nó bảo vệ |
| **Heredoc `<<'PY'` trong công cụ Bash vẫn nuốt dấu thoát** | `\\n` ra thành xuống dòng thật, và một lần `\\b` ra thành **ký tự backspace 0x08 nằm trong regex** khiến phép kiểm không bao giờ khớp. Dựng dấu thoát bằng `chr(92)`, hoặc dùng công cụ Edit/Write |
| **`/tmp/...` bị MSYS đổi đường dẫn khi truyền cho `docker -f`** | Báo *"open Dockerfile.mut: no such file"*, tức `rc=1` **trông y hệt rào chắn vừa bắn**. Đặt tệp tạm trong thư mục ngữ cảnh và dùng đường dẫn tương đối |
| **`ThreadSanitizer failed to allocate … (error code: 1455)`** | 1455 là `ERROR_COMMITMENT_LIMIT` của Windows — **hết commit charge, không phải mã sai**. Xảy ra khi `go test -race` chạy lúc Docker Desktop đang bật. Chạy riêng gói đó thì xanh ngay. Cùng nguyên nhân làm `docker build` đổ nhất thời rồi xanh khi dựng lại. **Đừng đi sửa `core/password`** |
| **`go list -m` trả đường dẫn Windows có `\`** | `sh` nuốt mọi dấu `\`, `cd "D:\works\..."` thành `cd "D:worksvihat..."`. Đã vá trong `makefile` bằng `tr '\134' '/'` |
| **`.dockerignore` loại trừ một thư mục đã bị xoá** | Dòng `apps` chết từ ADR 0015; nó không loại gì nữa mà vẫn nằm đó trông như một biện pháp. Nay `tools/check_build.py` kiểm **chiều ngược lại**: mọi thư mục cấp một phải được kể tới |
| `gofmt -l .` **in tên tệp chưa định dạng rồi thoát mã 0** | Một cổng báo rồi cho qua không phải cổng. Đã vá (`buf lint` cũng từng bị nuốt lỗi vì tiền tố `-`). **Dạng lỗi này còn ở đâu nữa — hỏi trước khi tin một cổng** |
| `SET search_path` là trạng thái **SESSION** | Trên pool chỉ áp cho kết nối đã phục vụ câu lệnh đó. `core/migrate` ghim kết nối riêng nên là kết nối **thứ hai**, nằm ở `public`. Hai suite tích hợp phải `SetMaxOpenConns(1)` |
| Trigger gắn trên bảng cha mà **không bắn** khi gõ thẳng tên partition | Mức **câu lệnh** không nhân bản xuống partition, mức **dòng** thì có. Hỏng **im lặng** — trigger vẫn hiện trong `\d`. → ADR 0013 |
| **Index con của bảng phân mảnh không drop riêng lẻ được** | Drop index **cha** kéo theo cả 32 con. Không bao giờ viết thao tác index theo từng mảnh |
| **Hai cột `bool` cạnh nhau, đọc theo vị trí trong `Scan`** | Hoán đổi hai con trỏ là **lỗi im lặng đối xứng**: biên dịch được, test thường vẫn xanh, chỉ sai nghĩa. Ca duy nhất bắt được là `(false, true)` |
| **Lọc ở Go thay vì lọc trong SQL** | Hai nhánh tốn thời gian khác nhau ⇒ **kênh biên thời gian** cho biết một email có tồn tại hay không. Điều kiện phân biệt người dùng phải nằm trong `WHERE` |
| `fmt` **không gọi `String()`** cho `%d %c %U %b %o` | Phải cài `fmt.Formatter`, không phải `Stringer`. → `core/secret` |
| `doc_guard` chặn mọi `Edit` vào `kb/` | Hook chỉ đọc `new_string`, không thấy frontmatter. **Dùng `Write` toàn tệp** — và xem §7 mục 10: buộc ghi đè toàn tệp có cái giá của nó |
| Hook báo nhầm | Đã vá năm lần. Nếu gặp lần nữa: **sửa hook + thêm ca test, đừng đi vòng** — hook nhiễu là hook bị tắt |
| `PARTITION BY HASH` mà quên tạo mảnh | INSERT lỗi, và vì vết đi cùng giao dịch nên **bản ghi nghiệp vụ rollback toàn bộ**. Nay `tenant_scope_guard` chặn ngay lúc gõ `.sql` |
| `buf lint STANDARD` ép tên message theo tên RPC | Kiểu trả về phải **bọc**, không trả thẳng message nghiệp vụ |

---

## 6. Cổng kiểm

```
make check
```

Trên Windows dùng `mingw32-make` (Git Bash không có `make`).

**Không có bảng số liệu ở đây, cố ý.** Mọi con số chép vào tệp này đều sai trong vòng vài
commit, và một con số sai trông y hệt một con số đúng.

**Thứ cổng kiểm KHÔNG phủ trên máy này** — đọc kỹ, đây là phần quyết định "xanh" nghĩa là gì:

| Không phủ | Hệ quả |
|---|---|
| **SQL** | Thiếu `VIGOV_TEST_DSN` ⇒ suite tích hợp tự bỏ qua **và vẫn báo `ok`**. Xem §2.1 |
| **`golangci-lint`** | Không có trên máy này; mục `lint` bỏ qua nó bằng tiền tố `-`. Chưa từng chạy ở đây |
| **`platform-admin/`** | In dòng BỎ QUA vì thiếu `node_modules`. Mã TypeScript của nó **không được kiểm** |
| **10 Jenkinsfile** | Chưa từng chạy trên Jenkins thật |
| **Việc một hook có NHÌN THẤY gì không** | `check_brain` bất biến 1 kiểm mỗi luật có **nêu tên** một hook và mọi đường dẫn hook có neo — **không** kiểm hook ấy đọc được tệp nào. `drift_guard` mù suốt từ ADR 0015 mà cổng vẫn 7/7. Phép kiểm duy nhất đáng tin cho một hook là **đột biến**: sửa một dòng mà nó đáng lẽ phải chặn, rồi xem nó có chặn không. Đã bịt một phần: `tools/test_hooks.py` nay có ca thuần cho **hai** hook — `stop_verify_guard.is_code` (thứ gì được tính là mã) và `drift_guard.duoc_quet` + `nen_canh_bao` + `bo_chu_thich` (đọc tệp nào, đọc phần nào của tệp, khi nào lên tiếng). Mỗi ca `duoc_quet` là một **hình dạng đơn vị triển khai**, nên quay về danh sách trắng là đỏ tại chỗ. Bài học ghi thẳng vào chỗ khai miễn trừ: **miễn ca payload không phải miễn test; phần thuần của một hook luôn kiểm được** |
| **Việc một cảnh báo có ĐÚNG không** | Không gì kiểm điều đó, và ngày 17/09 đã có một ca thật: `drift_guard` bắn ba cảnh báo, **một là âm tính giả** (§5). Cổng chỉ nói hook chạy được và phần thuần của nó cư xử đúng — không nói **kết luận** nó rút ra là đúng. Cảnh báo là **chỗ đáng nhìn**, không phải phán quyết: đi kiểm tận nơi trước khi sửa mã theo nó |

**Đã kiểm trong phiên này, không còn là văn bản Dockerfile:** cả 9 ảnh dựng được và dựng
**không có `go.work`** (nên `go.mod` từng dịch vụ thật sự đủ) · nhị phân liên kết tĩnh, đã
strip, 0 dấu vết đường dẫn máy build · `zoneinfo/Asia/Ho_Chi_Minh` có mặt · UID 65532 · cả 8
dịch vụ chạy **không cấu hình** đều **thoát 1** và gọi tên biến còn thiếu · web chạy được, và
với `Host` không phân giải được thì trả 500 **fail closed** mà thân phản hồi không lộ tên miền
hay đường dẫn nội bộ.

---

## 7. Việc treo — không ai chặn, ta chọn chưa làm

| # | Việc | Ghi chú |
|---|---|---|
| 1 | **Backfill dữ liệu theo từng xã chưa tồn tại** | `core/migrate` chỉ lo DDL ⇒ **luật 7 bất biến 5 mới đạt một nửa**. Ngưỡng cần cơ chế thật là khi thời gian giữ khoá thành đáng kể. → ADR 0013, mục Giới hạn |
| 2 | **`REVOKE` trên `audit_log` thuộc khâu cấp phát CSDL** | Câu đúng giữ trong comment tệp `0002`. **Không nằm trong tay mã nguồn**: khâu cấp phát không làm thì lớp quyền vẫn hở dù trigger vẫn đúng |
| 3 | **Đặc tả ghi 43 quyền nhưng chỉ liệt kê 33** | Đã nạp 33 vào `quyen`. Mười khoá còn lại là câu hỏi cho khách, **đừng bịa** |
| 4 | **Dữ liệu cá nhân thật vẫn còn trong LỊCH SỬ GIT** | Cây làm việc đã dọn. Gỡ khỏi lịch sử là **viết lại lịch sử** trên `main` — cần quyết định của chủ dự án, không phải việc agent tự làm |
| 5 | **Xác thực service↔service chưa có** | Rủi ro đã chấp nhận có chủ ý, kèm điều kiện gỡ: ADR 0012, quyết định 3. **Không** dựng cơ chế bí mật chia sẻ tạm |
| 6 | **`Staff` không có trường họ tên** | `BatchGetStaff` chưa phục vụ được mục đích nó tự khai. Thêm trường là sửa hợp đồng — cùng lúc phải trả lời câu che/không che (#11) |
| 7 | **Bàn giao việc đang xử lý khi khoá tài khoản** | Cố ý **chưa** ghi thành câu hỏi mở: chưa có bảng giao việc nào tồn tại để nói "việc đang giữ" nghĩa là gì, nên hỏi bây giờ là hỏi một câu trừu tượng. Hỏi khi dựng bảng nghiệp vụ đầu tiên có người phụ trách — và nhớ câu trả lời nhiều khả năng là "tuỳ xã" |
| 8 | **Cán bộ không có vai trò nào thì vào `/` thấy gì** | Đặc tả §1 chỉ chia "Lãnh đạo" / "vai trò khác". Chưa ghi thành câu hỏi mở vì chưa biết trạng thái ấy có tồn tại thật trên dữ liệu xã hay không |
| 9 | **Rà cả lớp "cơ chế nhận diện mã theo tên thư mục"** | Đã soát `.claude/hooks/` và `tools/` và tìm được **ca thứ năm**: `stop_verify_guard.CODE_DIR` thiếu `/tools/`, nên sửa trình sinh hợp đồng rồi nói "xong" thì cổng không thấy gì. Đã vá + thêm ca test + đột biến để chắc nó bắn. **Chưa soát:** không có `.github/` nên chưa có cấu hình CI nào để soát, nhưng ngày dựng CI thì đây là thứ phải soát lại đầu tiên |
| 10 | **`doc_guard` chặn mọi `Edit` vào `kb/`, nên mọi sửa đổi nhỏ đều thành ghi đè toàn tệp** | Hook chỉ đọc `new_string` nên không thấy frontmatter đang nằm trên đĩa. Hệ quả **không** chỉ là bất tiện: ngày 17/09 hai phiên cùng sửa tệp bàn giao, và việc buộc ghi đè toàn tệp suýt xoá sạch sáu chỗ sửa của phiên kia — bắt được chỉ vì công cụ báo "file has been modified since read". Sửa đúng là cho hook **đọc tệp trên đĩa** khi thao tác là `Edit`, giữ nguyên đường chặn khi là `Write`. Kèm ca test: một `Edit` vào tệp `kb/` có frontmatter hợp lệ phải **PASS** |

---

## 8. Nếu chỉ đọc được một mục

**§3.** Việc kế tiếp thì đọc mã ra được; cái chặn thì không — nó nằm ở một cuộc trao đổi với
khách mà kho mã không ghi lại. Đừng quyết hộ khách để đi tiếp: `drift_guard` cảnh báo khi mã
đang lặng lẽ quyết một câu còn mở, và một quyết định lặng lẽ trong hệ thống hành chính là một
quyết định có người phải trả lời.

Nhưng đọc cảnh báo ấy cho đúng: nó chỉ ra **chỗ đáng nhìn**, không phải **kết luận**. Ngày
17/09 một trong ba cảnh báo là âm tính giả, và nó bắn vào đúng tệp đã lập luận cẩn thận nhất
kho về việc *không* quyết. Đi kiểm tận nơi trước khi tin — cả khi thứ đang nói là một biện
pháp của chính ta.
