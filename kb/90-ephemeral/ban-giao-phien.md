---
id: ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: ec80994
expires: 2026-12-21
owns_facts:
  - "quyết định đã chốt với khách, cạm bẫy đã gặp, và phạm vi các phiên song song"
---

# Bàn giao phiên — cập nhật 2026-09-22

**Đọc tệp này SAU `kb/INDEX.yaml` và tầng `always_load`, không thay thế chúng.**
Nó chỉ trả lời: *đã quyết gì, đang kẹt ở ai, và cạm bẫy nào đã tốn thời gian của người trước.*

Viết bằng `/handover`. **MỘT tệp, ghi đè trọn vẹn mỗi lần** — tên tệp không mang ngày, vì một
tên mang ngày mời gọi đúng một thứ: tệp bàn giao thứ hai, và hai tệp bàn giao là hai tệp mâu
thuẫn mà người đọc không biết tin cái nào.

Hết hạn **2026-12-21**. Sau ngày đó tin `git log`, đừng tin tệp này.

---

## 1. Đã làm

**Không có danh sách commit ở đây.** `git log --oneline b22684a..HEAD` trả lời câu đó chính xác
hơn và không bao giờ lệch. Dưới đây chỉ những thứ `git log` không trả lời được.

### Quyết định đã chốt với người dùng — và nơi ghi

ADR `0001`–`0032` nằm trong `kb/10-decisions/`, mỗi tệp tự nói nó quyết gì. **Đừng chép lại vào
đây** — một bảng tóm tắt ADR là một bảng sẽ lệch với ADR. Chỉ những quyết định **chưa thành ADR**
mới cần chỗ này:

| Quyết định | Ngày | Ghi ở |
|---|---|---|
| Tiền tố **`vigov`** cho tên ảnh và module; người dùng đổi tên dự án sau khi v2 thay được bản gốc | — | — |
| Jenkins dùng **docker CLI trên agent**, không Kaniko | — | — |
| Phạm vi hạ tầng dừng ở **Dockerfile + Jenkinsfile**; cụm k8s do đội devops | — | — |
| Trần `always_load` nâng lên **27000 token** | 21/09 | `kb/INDEX.yaml:15` |
| **Miễn xã** cho `ResolveCitizenSession` — điều kiện dừng ADR 0012 quyết định 1, hỏi và được trả lời | 21/09 | `core/grpcx/grpcx.go`, khối trên `methodsWithoutTenant` |
| **Ingress SINH từ `openapi.json`** (lối b trong ba lối ra viết sẵn), không liệt kê tay, không gateway | 21/09 | `deploy/README.md` · bộ sinh ở `tools/ingress/` |
| Nối hai khoá `feedback.classify` / `feedback.unmask` vào tuyến | 21/09 | ADR 0030 là căn cứ; `unmask` đã làm, `classify` còn chặn |
| Dựng rào đối chiếu khoá quyền trong mã với bảng `quyen` | 21/09 | luật 5 bất biến **3c** |

**`ResolveCitizenSession` đáng đọc kỹ hơn một dòng bảng.** Nó là lời gọi THIẾT LẬP xã cho kênh
công dân — Mini App không có tên miền, nên tới khi nó trả lời thì không có gì để suy ra xã. Lý do
nó **không nới lỗ** nằm ở hợp đồng chứ không ở lời hứa: yêu cầu chỉ mang `session_token`, không có
trường xã, nên bên gọi không hỏi được *"phiên này có hợp lệ ở xã X không"*; xã đi về trong **phản
hồi**. `ListTenants` và `ResolveTenantSuccession` **vẫn nằm ngoài** danh sách — câu của chúng khác
và chưa ai đặt ra với người dùng.

### Thứ cố ý KHÔNG dựng, và vì sao

Thứ chưa dựng và lý do chưa dựng nằm cùng chỗ với tiến độ: `kb/90-ephemeral/tien-do.md`, khoá
`tiep_theo` của từng mục. Giữ hai bản là giữ hai bản sẽ lệch.

Riêng một quyết định KHÔNG thuộc module nào, nên nó ở lại đây:

| Không dựng | Vì sao |
|---|---|
| **CHECK `co_tai_khoan => mat_khau_hash <> ''`** | Lập luận đầy đủ ở `service-identity/migrations/0003_…sql:189–217`. Viết ràng buộc ấy bây giờ làm **một trong ba phương án của câu mở #9 không cài đặt được nữa** — tức quyết hộ khách. Giá đảo ngược bất đối xứng: thêm sau là một dòng migration, gỡ sau là `ALTER TABLE` trên bảng đang chạy. *Đọc kèm §5: đúng khối chú thích này là thứ `drift_guard` từng buộc tội nhầm.* |

---

## 2. Việc kế tiếp

**Không nằm ở đây.** Việc kế tiếp theo từng module ở `kb/90-ephemeral/tien-do.md` — sinh ra từ
`kb/90-ephemeral/tien-do/<module>.json`, mỗi agent ghi đúng tệp module của mình.

Tách ra vì lý do rất cụ thể: hai tệp cùng liệt kê việc kế tiếp là hai tệp sẽ lệch, và bản lệch là
bản phiên sau đọc (luật 9, bất biến 2). Tệp bàn giao giữ đúng thứ `tien-do` không giữ.

Ghi bằng `/progress`. Soát bằng `progress-reviewer`.

---

## 3. Đang bị chặn — và chặn bởi ai

Nguồn chuẩn là `kb/00-foundation/open-questions.json`. **Câu nào đang chặn việc nào** thì đọc bảng
*"Nợ khách chốt"* ở đầu `kb/90-ephemeral/tien-do.md`: bảng ấy **sinh ra** từ khoá `no_confirm` của
từng mục nên không lệch được với tiến độ — khác hẳn một bảng chép tay, thứ đúng vào ngày viết rồi
im lặng sai dần.

**Bốn câu đang chặn mà KHÔNG nằm trong `open-questions.json`**, vì chúng là câu hỏi cho người dùng
chứ không phải cho khách hàng — hỏi ngày 21/09, chưa có trả lời lúc viết tệp này:

| Câu | Chặn cái gì |
|---|---|
| **Xã sửa được DANH SÁCH MÃ hay chỉ NHÃN + THỨ TỰ** của các danh mục? | ba tuyến ghi ở ba service (`documents` loại văn bản · `finance` hạng mục vốn · `petitions` hai danh mục nhiệm vụ). Một câu mở khoá cả ba |
| **web-admin ra ngoài bằng đường nào** — A `node:http` + gốc nội bộ · B mở 443 · C biên đọc `X-Forwarded-Host` | **BLOCKER PHÁT HÀNH**: mọi trang web-admin 500. Xem `tien-do/web-admin.json`, mục `goc-api-noi-bo` (`treo`) và §5 dưới đây |
| `/api/v1/<chưa định tuyến>` nên trả **404 HTML của web** hay **404 JSON**? | hình dạng hiện tại là HTML — sai an toàn nhưng khó đọc. `tien-do/deploy.json` |
| Quyết định miễn xã cho `ResolveCitizenSession` có cần **ADR riêng** không? | hiện ghi ở chú thích `core/grpcx/grpcx.go` + sổ. Không chặn mã |

Một câu còn OPEN là một việc **không ai được tự quyết thay khách** (ROUTING §8). `drift_guard`
cảnh báo khi mã đang lặng lẽ quyết một câu như thế.

---

## 4. Phiên song song

Ngày 21/09 có **ít nhất một phiên khác** chạy cùng kho — phiên tự xưng `vigov-v2-3c`, đã đẩy và
quét manifest `deploy/` của phiên này vào commit của họ (không mất gì, không force-push). **Không
biết nó còn mở hay không lúc viết tệp này** — phiên sau phải tự kiểm, đừng tin dòng này.

Trong phiên này còn có **bốn agent chạy song song trong cùng một phiên**, và đó là hình dạng sẽ
lặp lại. Hai bài học trả bằng thời gian thật:

- **Ảnh chụp `git status` đầu phiên sai giữa chừng.** Agent nào đọc nó rồi kết luận "cây sạch" là
  kết luận sai: cây có thay đổi chưa commit của agent khác. Đọc lại `git status` ngay trước khi ghi.
- **`go run ./tools/kb` chạy giữa lúc agent khác đang sửa mã sẽ sinh tầng GENERATED trên nền việc
  dang dở của họ.** Hai agent làm đúng thế trong phiên này. Cách xử: sinh lại **một lần cuối** sau
  khi mọi agent xong, trước khi commit.

Ba quy ước chia việc, sinh ra từ ba lần suýt giẫm chân chứ không từ lý thuyết, vẫn đúng: phạm vi
tuyên bố **bằng đường dẫn** và nhắc lại mỗi lần đổi · phát hiện trong vùng người khác thì **báo kèm
bằng chứng, không tự sửa** · `git add` theo đường dẫn **tường minh**, không bao giờ `-A`.

> **Mục này phải xoá hẳn khi phiên sau chỉ có một mình**, chứ không để lại. Một mục §4 trỏ vào
> những phiên đã kết thúc còn tệ hơn không có mục §4.

---

## 5. Cạm bẫy đã gặp — đọc để khỏi mất thời gian lại

Một nửa bảng này có chung một hình dạng: **thứ trông như biện pháp mà không phải biện pháp.** Gặp
cái tiếp theo cùng dạng thì đừng vá riêng nó — hỏi cả lớp đó còn ở đâu nữa.

**Ngày 21/09 lớp ấy trúng đúng tầng rào chắn.** Rà 18 hook tìm ra **sáu** cái hỏi đúng câu nhưng
đọc sai thứ, và cái nặng nhất là rào **BLOCK duy nhất của luật 3**: `_common.PII_TOKEN` dùng mẫu
`hoTen`/`soDienThoai` **camelCase, phân biệt hoa thường**, trong khi trường Go xuất khẩu của kho
là `HoTen`/`DienThoai`/`MatKhauHash` (`service-identity/internal/domain/can_bo.go:13,18,19`). Nó
không hỏng — **nó chưa từng canh được đúng thứ nó sinh ra để canh**, và mọi phiên trước đều thấy
nó xanh. Cùng mẻ: `audit_guard` chỉ biết `.Exec(` nên lọt `tx.ExecContext(...)` · `citizen_scope_guard`
đòi `Header()` **có ngoặc** (hình dạng bên *phản hồi*) nên lọt `r.Header.Get("citizen_id")` ·
`service_boundary_guard` so DSN với **tên thư mục** thay vì tên nghiệp vụ — **ca thứ chín** của
lớp lỗi ấy.

**Bố cục phẳng (ADR 0015/0016) sinh ra cả một mẻ, và không cái nào kêu.** Mọi cơ chế nhận diện mã
theo **tên thư mục** câm đi cùng lúc: sáu hook khớp theo đoạn `/services/` · `drift_guard` quét
danh sách trắng bảy thư mục mà sau đó chỉ còn **một** tồn tại · `.dockerignore` loại trừ `apps` ·
`check_brain` đếm tên hook thay vì đường dẫn · `stop_verify_guard.CODE_DIR` thiếu `/tools/`.
**Năm ca.** Bài học chung, đắt hơn từng ca riêng lẻ: **danh sách trắng tên thư mục là hình dạng
sai cho kho này.** Dùng **danh sách loại trừ** — nó hỏng theo chiều ngược lại, tức quét thừa vài
mili giây thay vì quét thiếu.

### Máy này — bốn cạm bẫy đều TRÔNG như lỗi mã

| Triệu chứng | Sự thật |
|---|---|
| `mingw32-make check` **đỏ ở mục `build`** với một trang stack trace của `cmd/link` | **Hết bộ nhớ, không phải mã.** `go build $(MODULES)` dựng cả 9 module một lượt và làm sập trình liên kết ở khâu sinh ký hiệu DWARF. Từng module dựng riêng thì **cả 9 đều OK** — đã kiểm 22/09. Đừng đi sửa Go |
| `tools/test_hooks.py` báo **thất bại** với `expected exit=2, got exit=3221225773` | Cũng hết bộ nhớ. `3221225773` = `0xC000013D`, `3221225794` = `0xC0000142` — **mã sập tiến trình của Windows**, không phải hook trả lời sai. Chạy lại khi máy rảnh: 196/196. **Đây là một cổng kiểm biết nói dối theo chiều ĐỎ**, và một cổng đỏ sai còn nguy hơn xanh sai: người ta sẽ đi "vá" một rào không hỏng |
| `go test -race` đổ ở khâu link, hoặc `ThreadSanitizer failed to allocate … (error code: 1455)` | Cùng nguyên nhân. `1455` là `ERROR_COMMITMENT_LIMIT`. **Luôn dùng `-p 1`** trên máy này |
| `docker ps` trả `Internal Server Error for API route` | Một lượt OOM đã **hạ cả Docker Desktop**. Không phải cấu hình Docker sai |

### web-admin gọi ra ngoài — ba số đo, mỗi số tự nó đủ để dừng

Blocker phát hành, và cách sửa "hiển nhiên" **không chạy được**:

1. **`fetch` của Node KHÔNG gửi được `Host` do người gọi đặt.** `host` là forbidden header;
   undici ghi đè bằng host của URL. Đã thử bốn đường (`{Host:}`, `{host:}`, `Headers`, `Request`)
   — cả bốn ra host của origin. `node:http` thì tôn trọng.
2. **`core/httpx/edge.go:24` chỉ đọc `r.Host`**, không đọc `X-Forwarded-Host`. Nên đổi gốc mà bỏ
   `Host` → biên nhận `Host: identity` → không xã nào → **404 trên mọi trang**. Tệ hơn 500, vì 404
   là câu trả lời *hợp lệ* nên không ai nghi cấu hình sai. Và nếu người vận hành "chữa" bằng cách
   đăng ký host nội bộ thành một xã, **mọi tên miền xã sẽ in ra tên của đúng xã ấy** — rò dữ liệu
   giữa hai cơ quan nhà nước (luật 1).
3. **REST của identity là 8080** (`deploy/base/identity/service.yaml`) nhưng `netpol.yaml` chỉ mở
   **9090** ra pod. Nên một gốc nội bộ bị chặn **y hệt** 443. **Không phương án nào tránh được một
   dòng egress** — điều mà đề xuất ban đầu tưởng là tránh được.

**Và cạm bẫy đi kèm, đúng hình dạng "phép kiểm xanh sai lý do":** bộ kiểm hiện tại soi lời gọi
bằng cách thay `globalThis.fetch` rồi đọc `mock.calls[0]`, mà đối tượng `Headers` **giữ** giá trị
`Host`. Nên một bài kiểm *"vẫn mang `Host` của xã"* sẽ **XANH** — kể cả đột biến bỏ `Host` cũng bắt
được — trong khi sản xuất gửi host sai. **Rào chết ngay từ lúc sinh ra.**

### Bảng cũ — vẫn đúng, đã kiểm lại 22/09

| Vấn đề | Cách xử |
|---|---|
| **Test tích hợp SKIP nhưng cả gói vẫn báo `ok`** | Dạng nặng nhất, và ngày 21/09 nó lộ ra quy mô thật: **159 ca** của 6 service chưa từng chạy một lần nào. Nay có `tools/schema-smoke` — **thiếu DSN thì thoát mã 2, không bỏ qua**. Với một suite bất kỳ khác: trước khi tin "có test canh chỗ này", **gỡ thử dòng đó ra và xem có đỏ không** |
| **Kết quả grep âm tính KHÔNG phải bằng chứng vắng mặt** | Một agent báo "grep không có kết quả" ⇒ kết luận web không gọi tuyến ấy ⇒ **bảng thuật ngữ bị sửa yếu đi theo**. Thực tế có gọi: một chỗ là template literal có nội suy, một chỗ gán qua biến có kiểu sinh. Loại sai này không ai soi ra **vì nó trông như thận trọng** |
| **Bản vá cho lỗi X rất dễ chính là một thể hiện mới của X** | Chuỗi ba lần đã xảy ra với `drift_guard`. Và 21/09 lặp lại: bản vá `pii_guard` cho lời gọi nhiều dòng lập tức **buộc tội một chú thích giải thích chính sách riêng tư**. Bắt được bằng **hiệu chuẩn trên 710 tệp thật**, không bằng suy đoán — đó là thói quen cần giữ: **sau khi vá một hook, chạy nó trên kho thật và mở từng tệp nó tố cáo** |
| **`git checkout --` / `git restore` để hoàn tác sau đột biến** | Một agent xoá **529 dòng chưa commit của người khác** bằng đúng lệnh đó. Phục hồi bằng **bản chụp tệp** ra thư mục tạm, không bao giờ bằng git. Mọi agent trong phiên này đều được dặn, và đều tuân |
| **`--build-arg` cho một `ARG` không khai bị Docker bỏ qua lặng lẽ** | Một lượt "đột biến" để thử rào chắn sẽ **xanh** và trông như rào đã bắn. Muốn thử thật thì sửa `ENV` trong chính Dockerfile |
| **Rào chắn đứng thành `RUN` riêng chỉ đo môi trường tại thời điểm ấy** | Rào `NEXT_PUBLIC_*` từng đứng trên `npm run build`; một dòng `ENV` chen vào giữa thì nó không thấy. Gộp vào **cùng một `RUN`** với lệnh nó bảo vệ |
| **Heredoc `<<'PY'` trong công cụ Bash vẫn nuốt dấu thoát** | `\n` ra thành xuống dòng thật, `\b` ra thành ký tự backspace nằm trong regex. Dựng dấu thoát bằng `chr(92)`, hoặc dùng Edit/Write |
| **`/tmp/...` bị MSYS đổi đường dẫn khi truyền cho `docker -f`** | Báo *"no such file"*, tức `rc=1` **trông y hệt rào chắn vừa bắn**. Đặt tệp tạm trong thư mục ngữ cảnh, dùng đường dẫn tương đối |
| `gofmt -l .` **in tên tệp chưa định dạng rồi thoát mã 0** | Một cổng báo rồi cho qua không phải cổng. Đã vá (`buf lint` cũng từng bị nuốt lỗi vì tiền tố `-`) |
| `SET search_path` là trạng thái **SESSION** | Trên pool chỉ áp cho kết nối đã phục vụ câu lệnh đó. `core/migrate` ghim kết nối riêng nên là kết nối **thứ hai**, nằm ở `public`. Hai suite tích hợp phải `SetMaxOpenConns(1)` |
| Trigger gắn trên bảng cha mà **không bắn** khi gõ thẳng tên partition | Mức **câu lệnh** không nhân bản xuống partition, mức **dòng** thì có. Hỏng **im lặng**. 21/09 đã truy `pg_catalog` và xác nhận trigger append-only **có mặt đúng hình dạng ADR 0013** — nhưng **chưa ai bắn thử nó từ chối một lệnh xoá**. Viết ca ấy vướng `data_safety_guard`: một bài kiểm chứng minh sổ không xoá được lại bị chặn vì trông giống lệnh xoá sổ |
| **Index con của bảng phân mảnh không drop riêng lẻ được** | Drop index **cha** kéo theo cả 32 con. Không bao giờ viết thao tác index theo từng mảnh |
| **Hai cột `bool` cạnh nhau, đọc theo vị trí trong `Scan`** | Hoán đổi hai con trỏ là **lỗi im lặng đối xứng**: biên dịch được, test thường vẫn xanh, chỉ sai nghĩa. Ca duy nhất bắt được là `(false, true)` |
| **Lọc ở Go thay vì lọc trong SQL** | Hai nhánh tốn thời gian khác nhau ⇒ **kênh biên thời gian** cho biết một email có tồn tại hay không. Điều kiện phân biệt người dùng phải nằm trong `WHERE` |
| `fmt` **không gọi `String()`** cho `%d %c %U %b %o` | Phải cài `fmt.Formatter`, không phải `Stringer`. → `core/secret` |
| `PARTITION BY HASH` mà quên tạo mảnh | INSERT lỗi, và vì vết đi cùng giao dịch nên **bản ghi nghiệp vụ rollback toàn bộ**. `tenant_scope_guard` chặn ngay lúc gõ `.sql` |
| `buf lint STANDARD` ép tên message theo tên RPC | Kiểu trả về phải **bọc**, không trả thẳng message nghiệp vụ |
| Hook báo nhầm | Đã vá nhiều lần. Nếu gặp lần nữa: **sửa hook + thêm ca test, đừng đi vòng** — hook nhiễu là hook bị tắt |

---

## 6. Cổng kiểm

```
mingw32-make check
```

`make` không có trên Windows; `mingw32-make` **thì có** (đã kiểm 22/09). Không có nó thì chạy thẳng
các lệnh dưới từng mục của `Makefile`.

**Không có bảng số liệu ở đây, cố ý.** Mọi con số chép vào tệp này đều sai trong vòng vài commit,
và một con số sai trông y hệt một con số đúng.

**Thứ cổng kiểm KHÔNG phủ trên máy này** — đọc kỹ, đây là phần quyết định "xanh" nghĩa là gì:

| Không phủ | Hệ quả |
|---|---|
| **Mục `build` không chạy nổi** | Sập trình liên kết vì hết bộ nhớ (§5). Nên `mingw32-make check` **không đi hết được** trên máy này. Thay bằng: `go build ./...` trong **từng** module, và `go test -count=1 -p 1 ./...` trong từng module |
| **`golangci-lint`** | Không có trên máy này; mục `lint` bỏ qua nó bằng tiền tố `-`. Chưa từng chạy ở đây |
| **`platform-admin/`** | Thiếu `node_modules`. Mã TypeScript của nó **không được kiểm** |
| **Jenkins cho ViGov** | Lượt build thật đầu tiên của **dự án** đã xanh — nhưng ở **kho anh em `vihat-miniapp`**, không phải ViGov. Mười tệp của ViGov cùng hình dạng với tệp đã xanh, vẫn **chưa lượt nào chạy**. `buf` thiếu trên máy chủ Jenkins, mà `core/gen` bị gitignore ⇒ đó là thứ chặn lượt đầu |
| **k8s theo lược đồ máy chủ** | `kubectl kustomize` chứng minh YAML **dựng được và trộn đúng**, KHÔNG chứng minh API server chấp nhận. `--dry-run=client` cần API discovery, máy này không có cụm. Chưa lượt deploy nào chạy thật |
| **Vết kiểm toán đọc lại từ bảng thật** | `TestPgVetXemDayDuDocLaiDuocTuBang` là ca duy nhất đọc lại dòng từ `audit_log` thật, và nó SKIP vì Docker đã sập. Dựng lại Docker rồi chạy nó là việc rẻ nhất còn lại |
| **Việc một hook có NHÌN THẤY gì không** | `check_brain` kiểm mỗi luật có **nêu tên** một hook — **không** kiểm hook ấy đọc được tệp nào. Sáu hook mù suốt nhiều tháng mà cổng vẫn 7/7 (§5). Phép kiểm duy nhất đáng tin là **đột biến**: sửa một dòng nó đáng lẽ phải chặn, rồi xem nó có chặn không |
| **Việc một cảnh báo có ĐÚNG không** | Không gì kiểm điều đó. Cảnh báo là **chỗ đáng nhìn**, không phải phán quyết: đi kiểm tận nơi trước khi sửa mã theo nó |

**Nay ĐÃ phủ, khác với bản bàn giao trước:** lược đồ CSDL **đã chạy thật** — 30 tệp migration của
7 service áp xanh trên PostgreSQL 16.10, `PARTITION BY HASH` 32 mảnh có thật, và truy toàn bộ
`pg_index` xác nhận **không bảng nghiệp vụ nào có khoá duy nhất đơn cột**. Dựng lại bằng
`go run ./tools/schema-smoke` (cần Docker). `tools/check_quyen.py` nay đối chiếu mọi khoá quyền
trong mã Go với bảng `quyen`.

---

## 7. Việc treo — không ai chặn, ta chọn chưa làm

**Không nằm ở đây.** Mỗi việc treo nằm ở module của nó với `trang_thai: "treo"`, kèm lý do đã chọn
chưa làm — `kb/90-ephemeral/tien-do.md`.

Phân biệt hai thứ dễ lẫn: **`treo`** là *không ai chặn, ta chọn chưa làm*; **bị chặn** là
`no_confirm` đang trỏ một câu còn OPEN. Cái thứ hai không tự gỡ được, cái thứ nhất thì được.

---
