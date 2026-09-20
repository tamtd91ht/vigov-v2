---
id: ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: b22684a
expires: 2026-12-16
owns_facts:
  - "quyết định đã chốt với khách, cạm bẫy đã gặp, và phạm vi các phiên song song"
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

Thứ chưa dựng và lý do chưa dựng nay nằm cùng một chỗ với tiến độ:
`kb/90-ephemeral/tien-do.md`, khoá `tiep_theo` của từng mục. Giữ hai bản là giữ hai bản sẽ lệch.

Riêng một quyết định KHÔNG thuộc module nào, nên nó ở lại đây:

| Không dựng | Vì sao |
|---|---|
| **CHECK `co_tai_khoan => mat_khau_hash <> ''`** | Lập luận đầy đủ ở `service-identity/migrations/0003_…sql:189–217`. Viết ràng buộc ấy bây giờ làm **một trong ba phương án của câu mở #9 không cài đặt được nữa** — tức quyết hộ khách. Giá đảo ngược bất đối xứng: thêm sau là một dòng migration, gỡ sau là `ALTER TABLE` trên bảng đang chạy. *Đọc kèm §5: đúng khối chú thích này là thứ `drift_guard` từng buộc tội nhầm.* |

---

## 2. Việc kế tiếp

**Không nằm ở đây nữa.** Việc kế tiếp theo từng module ở `kb/90-ephemeral/tien-do.md` — tệp ấy
sinh ra từ `kb/90-ephemeral/tien-do/<module>.json`, mỗi agent ghi đúng tệp module của mình.

Tách ra vì lý do rất cụ thể: hai tệp cùng liệt kê việc kế tiếp là hai tệp sẽ lệch, và bản lệch
là bản phiên sau đọc (luật 9, bất biến 2). Tệp bàn giao giữ đúng thứ `tien-do` không giữ —
**quyết định đã chốt, cạm bẫy đã gặp, phạm vi phiên song song** — thứ không thuộc module nào và
không suy ra được từ mã.

Ghi bằng `/progress`. Soát bằng `progress-reviewer`.

---

## 3. Đang bị chặn — và chặn bởi ai

Nguồn chuẩn vẫn là `kb/00-foundation/open-questions.json`. **Câu nào đang chặn việc nào** thì
đọc bảng *"Nợ khách chốt"* ở đầu `kb/90-ephemeral/tien-do.md`: bảng ấy **sinh ra** từ khoá
`no_confirm` của từng mục, nên nó không lệch được với tiến độ — khác hẳn một bảng chép tay, thứ
đúng vào ngày viết rồi im lặng sai dần.

Một câu còn OPEN là một việc **không ai được tự quyết thay khách** (ROUTING §8). `drift_guard`
cảnh báo khi mã đang lặng lẽ quyết một câu như thế.

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
`service-identity/internal/store/`, vùng bên kia. Xem `tien-do/service-identity.json`, mục `kho-doc-kenh-cong-dan` và `store-crosstenant`.

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
| **Hai cảnh báo còn lại là việc thật — xử bằng cách trả lời câu hỏi, không phải nới ngưỡng** | Câu mở **#4**: đã có đường đọc chéo xã trong mã trong khi khách chưa chốt cấp tỉnh xem tổng hợp tới mức nào (→ `tien-do/service-identity.json`, mục `store-crosstenant`). Câu mở **#19**: bảng quan hệ công dân↔xã vừa ra đời trong khi câu "công dân sửa lời khai đã xác thực thì sao" còn mở |
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
| **`doc_guard` chặn mọi `Edit` vào `kb/`** — **ĐÃ VÁ (`b22684a`)** | Hook chỉ đọc `new_string` nên không thấy frontmatter đang nằm trên đĩa, và buộc mọi sửa ba dòng thành **ghi đè toàn tệp**. Cái giá không phải bất tiện: ngày 17/09 việc ấy **suýt xoá sạch sáu chỗ sửa của phiên song song**, bắt được chỉ vì công cụ báo *"file has been modified since read"*. **Một rào bảo vệ tri thức mà đẩy người ta đi ghi đè tri thức là rào đang làm ngược việc của nó.** Nay hook phân biệt hai đầu vào: văn bản lần sửa **đưa vào** (cho `secret_scan`/`pii_guard` — phải tố cáo thứ lần sửa THÊM) và **tệp sau khi sửa**, đọc từ đĩa (cho luật frontmatter và hạn T5 — chúng là tính chất của TỆP) |
| Hook báo nhầm | Đã vá năm lần. Nếu gặp lần nữa: **sửa hook + thêm ca test, đừng đi vòng** — hook nhiễu là hook bị tắt |
| `PARTITION BY HASH` mà quên tạo mảnh | INSERT lỗi, và vì vết đi cùng giao dịch nên **bản ghi nghiệp vụ rollback toàn bộ**. Nay `tenant_scope_guard` chặn ngay lúc gõ `.sql` |
| `buf lint STANDARD` ép tên message theo tên RPC | Kiểu trả về phải **bọc**, không trả thẳng message nghiệp vụ |

### Thứ đã bắt được ba lỗi trên — và phiên sau sẽ không có nó

Ba dòng đầu bảng này là một chuỗi: vá `drift_guard` mù → bản vá đếm cả chú thích → vá chuyện
đếm chú thích thì suýt giết tín hiệu duy nhất của câu #4. **Bản vá cho lỗi X rất dễ chính là
một thể hiện mới của X**, vì nó được viết bởi người vừa nhìn X từ một góc duy nhất.

Điều đáng nói: **không lần nào cổng kiểm bắt được.** Cả ba lần đều xanh. Thứ bắt được là một
người khác đọc **kết quả đầu tiên** của bản vá và hỏi *"cái này có đúng không"* — hai phiên
chạy song song, mỗi phiên soi kết quả của phiên kia.

Phiên sau nhiều khả năng chỉ có một mình. Nên thay cơ chế ấy bằng một thói quen, và đây là
thói quen: **sau khi vá một hook, chạy nó trên kho thật và đọc từng cảnh báo nó bắn** — không
phải đếm số cảnh báo, mà mở đúng tệp nó tố cáo và xem lời tố cáo có đứng vững không. Cổng kiểm
nói hook chạy được; chỉ việc ấy mới nói hook nói đúng.

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
| **SQL** | Thiếu `VIGOV_TEST_DSN` ⇒ suite tích hợp tự bỏ qua **và vẫn báo `ok`**. Xem `tien-do/_chung.json`, mục `chay-migration-that` |
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

**Không nằm ở đây nữa.** Mỗi việc treo nằm ở module của nó với `trang_thai: "treo"`, kèm lý do
đã chọn chưa làm — `kb/90-ephemeral/tien-do.md`.

Phân biệt hai thứ dễ lẫn: **`treo`** là *không ai chặn, ta chọn chưa làm*; **bị chặn** là
`no_confirm` đang trỏ một câu còn OPEN. Cái thứ hai không tự gỡ được, cái thứ nhất thì được.

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
