---
id: estimate-vigov
tier: T5
source: CURATED
owner: architecture
derived_from_commit: b6ca85b
expires: 2026-12-19
owns_facts:
  - "khối lượng thi công còn lại của ViGov v2 và cách quy nó thành ngày-người, đo tại 20/09/2026"
---

# Estimate thi công — ViGov v2

**Bản cho DỰ ÁN NÀY.** §1–§2 là **số đo trên kho mã**. Từ §3 trở đi là suy ra từ §1 kèm hệ số
ghi rõ. Bản tổng quan cho dự án tương tự tách riêng, viết sau khi bản này được chốt.

**Ô ghi `___` là ô NHẬP TAY** — người lập estimate điền, vì nó phụ thuộc dự án hoặc phụ thuộc
bên khác. Phần phân tích cho những ô ấy vẫn có ở đây; chỉ con số là của người lập.

Hết hạn **19/12/2026**: sau đó phạm vi đã đổi và các hệ số phải đo lại.

---

## 0. MÔ HÌNH NHÂN SỰ VÀ ĐƠN VỊ ĐO

| | |
|---|---|
| Nhân sự | **AI code 100% · 1 người định hướng và confirm** |
| Đơn vị | **ngày-người (NN)** của người định hướng — 1 ngày = 8 giờ làm việc thật |
| Không dùng | man-day lập trình viên — dự án này không có lập trình viên |

**Ba loại thời gian, đừng cộng gộp:**

| | Ý nghĩa |
|---|---|
| **NN** | công của người định hướng — thứ tính tiền |
| **NC** | ngày chờ: khách chốt, devops cấp máy, Zalo duyệt. **Không tiêu công, nhưng tiêu lịch** |
| **NL** | ngày lịch = NN + NC, chồng lấn được một phần |

---

## 1. SỐ ĐO THẬT CỦA DỰ ÁN NÀY

Đo tại commit `b6ca85b`, ngày 20/09/2026.

| Ngày | Commit | Dòng thêm | Dòng xoá |
|---|---|---|---|
| 15/09 | 2 | 9.531 | 39 |
| 16/09 | 33 | 22.414 | 940 |
| 17/09 | 58 | 42.917 | 3.394 |
| 18/09 | 34 | 19.731 | 3.450 |
| 20/09 | 31 | 28.032 | 974 |
| **5 ngày làm việc** | **158** | **122.625** | **8.797** |

Kho hiện còn **107.673 dòng**, trong đó **36.435 dòng (34%) là test**.

### 1.1 Đã có gì sau 5 ngày

| Hạng mục | Số đo |
|---|---|
| Service backend | 8 (Go, mỗi service một module riêng) |
| App frontend | 3 — `web-admin`, `citizen-app` (Zalo Mini App), `platform-admin` |
| Migration | 27 tệp |
| Tuyến REST trong hợp đồng | 17 |
| ADR đã ghi | 25 |
| Bộ não AI | 7.704 dòng — 11 luật · 17 hook · 11 agent · 21 skill · 9 command |
| Đặc tả nhận từ khách | 16 chương · 3.900 dòng |
| **Phân hệ đã có màn hình thật** | **2 / 16** |

**Nền móng đã xong** — và đó là lý do phần còn lại rẻ hơn hẳn phần đã làm: hợp đồng REST sinh
từ mã, xác thực cán bộ dùng chung ở `core`, rìa đa xã, vết kiểm append-only, bộ não cưỡng chế.
Mỗi phân hệ mới nay là **lặp một khuôn đã có**, không phải dựng khuôn.

### 1.2 HỆ SỐ HIỆU CHUẨN — và một lỗi đã sửa

**122.625 dòng / 5,5 NN ≈ 22.300 dòng mỗi NN.**

Kiểm chứng trên một lát cắt dọc thật — cụm danh mục, 8 bảng + 9 tuyến đọc + mắc xác thực +
sinh hợp đồng + việc web:

| Commit | Nội dung | Dòng |
|---|---|---|
| `fa10cf1` | 8 bảng danh mục | 1.873 |
| `cdc5b5f` | 3 tuyến đọc + kho + test (identity) | 2.310 |
| `9e3345f` | 6 tuyến đọc ở 4 service | 7.666 |
| `19c6008` | mắc xác thực cán bộ vào 4 service | 1.448 |
| `b899f4c` | sinh lại hợp đồng + 8 việc web | 1.566 |
| | **Tổng** | **14.863** |

⇒ **~0,67 NN cho cả cụm 8 phân hệ danh mục**, tức **~0,08 NN mỗi phân hệ danh mục**.

> **Bản trước của tệp này ghi 48 NN cho coding. Con số đó SAI hơn 10 lần, và nguyên nhân đáng
> ghi lại:** tôi lấy *thời gian trôi* của một ngày rồi quy cả ngày ấy cho một phân hệ, trong
> khi ngày đó chạy **song song 6–8 việc** bởi nhiều agent. Phép chia đúng là tổng dòng chia
> tổng NN. Đây đúng là dạng lỗi mà một bản estimate dựa vào cảm giác sẽ mắc, và nó chỉ lộ ra
> khi đem chia cho số đo.

**Giới hạn của hệ số này, nói trước:** 22.300 dòng/NN đo trong giai đoạn dựng nền. Phân hệ
nghiệp vụ trên nền đã có thì **nhanh hơn trên mỗi dòng** (khuôn đã có) nhưng **nhiều quyết
định hơn trên mỗi dòng**. Phần đệm cho chuyện đó **không** đến từ một hệ số nhân, mà từ **quy
ước đóng gói lên mốc 0,25 NN** ở §4.2 — đo được là +30% trên toàn hạng mục coding.

---

## 2. HẠNG MỤC 0 — DỰNG BỘ NÃO AI *(đã tiêu, không lặp lại)*

| Thành phần | Số đo | Vai trò |
|---|---|---|
| Luật cưỡng chế | 11 luật, mỗi luật nêu tên hook thực thi | Điều AI **không được phép** làm |
| Hook | 17 | Chặn **trước khi** ghi, không phải review sau |
| Agent | 11 (5 dựng, 5 soát, 1 chung) | Mỗi agent một ranh giới ghi — nền của việc chạy song song |
| Skill · Command | 21 · 9 | Tri thức nạp theo từ khoá · thủ tục lặp lại |
| Tầng tri thức `kb/` | 9.522 dòng, 5 tầng theo tuổi thọ | Chống việc mỗi phiên khám phá lại hệ thống |
| Tự kiểm bộ não | `check_brain` 7 bất biến · `test_hooks` 139 ca | Rào không có test là rào sẽ chết trong im lặng |

**Đã tiêu ~1,5 NN.** Với dự án tương tự: **~0,5 NN** (≈70% dùng lại được).

> Hạng mục này **là điều kiện để §4 rẻ như thế**. Ranh giới ghi của 11 agent chính là thứ cho
> phép 6–8 việc chạy song song trong một ngày mà không ghi đè nhau. Cắt nó đi thì không chỉ
> mất phần kiểm soát — mất luôn tốc độ.

---

## 3. HẠNG MỤC 1 — NGHIỆP VỤ · PROTOTYPE · HỌP CHỐT

### 3.1 Giá trị

| | |
|---|---|
| Đơn giá một buổi | **0,5 NN** (chuẩn bị phương án + họp + ghi ADR) |
| Số buổi | **___ buổi** ← nhập tay |
| **Thành tiền** | **___ × 0,5 NN** |

Số buổi do **người trực tiếp làm việc với khách** xác định — nó phụ thuộc cách khách họp, số
người phải có mặt để chốt, và việc khách đã có prototype chạy được hay chưa.

### 3.2 Phân tích cho dự án này — cơ sở để điền số buổi

**15/21 câu hỏi của khách vẫn OPEN**, gom theo chủ đề thành 6 nhóm. Gom theo *người ra quyết
định*, không theo phân hệ: hai câu cùng một người quyết thì vào một buổi.

| Nhóm | Câu | Mở khoá được | NC chờ khách |
|---|---|---|---|
| 1 | #9 #17 #18 | Toàn bộ luồng cấp tài khoản cán bộ | 1–5 |
| 2 | #10 #13 #14 | Toàn bộ tuyến GHI danh bạ cán bộ | 1–5 |
| 3 | #11 #12 | Che số di động cán bộ · công khai lên Mini App | 1–3 |
| 4 | #15 #16 | Mã cán bộ · `dien_thoai`/`di_dong` | 1–5 |
| 5 | #19 #20 #21 | Công dân sửa lời khai đã xác thực · tên khoá quyền · xã sửa danh mục tới mức nào | 2–7 |
| 6 | #1 #4 | Sáp nhập/chia tách xã · cấp huyện-tỉnh xem tổng hợp tới mức nào | 3–10 |

**Hai câu đắt bất đối xứng — ưu tiên đưa lên buổi đầu:**

| Câu | Trả lời muộn tốn gì |
|---|---|
| **#15 #16** | **Chặn schema.** Trả lời sau khi có dữ liệu thật ⇒ `ALTER TABLE` trên **bản ghi lưu trữ** |
| **#19** | **Một chiều.** Thêm lịch sử phiên bản về sau chỉ ghi được từ lúc thêm; những lần công dân đã sửa lời khai là **mất vĩnh viễn** |

**13/47 mục còn lại đang bị 15 câu này chặn.** Đây là đường găng của dự án — không phải coding.

### 3.3 Phạm vi CHƯA XÁC ĐỊNH

`service-dossiers` (hồ sơ một cửa) là khung rỗng và **không có chương đặc tả nào** trong 16
chương nhận được. Không estimate được. Phải hỏi khách: có trong phạm vi hợp đồng không, đặc tả
ở đâu. Nếu có: ước sơ bộ **+1,0 NN** theo hệ số §4.

---

## 3B. HẠNG MỤC 1B — BUFFER THAY ĐỔI NGHIỆP VỤ / PROTOTYPE

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay |

### 3B.1 Vì sao phải là một hạng mục riêng, không phải phần trăm đệm

§4 estimate **dưới giả thiết prototype và nghiệp vụ không đổi**. Giả thiết ấy hầu như không
đúng trọn vẹn, và khi nó vỡ thì **chi phí không rơi vào phân hệ đang làm — nó rơi vào phân hệ
đã xong**. Đó là lý do nó không gộp được vào §4 dưới dạng %.

### 3B.2 Công thức đề nghị, và bằng chứng đo được ở chính dự án này

> **Buffer ≈ Σ (dòng của lát cắt bị ảnh hưởng ÷ 22.300) × hệ số lan**

| Hệ số lan | Khi nào | Ca thật đã xảy ra ở dự án này |
|---|---|---|
| **×0,3** | Đổi nhãn, đổi thứ tự, đổi văn bản hiển thị | — |
| **×0,5** | Đổi một trường trên dây (hợp đồng REST sinh lại, web phải theo) | `fcea7bd`: đổi `is_active` → `active`. Kéo theo hợp đồng + kiểu sinh cho web + ca test mới canh **tên trường** |
| **×1,0** | Đổi **chủ sở hữu** một thực thể giữa các service | **ADR 0024**: danh mục chuyển từ `platform` sang **dịch vụ sở hữu**. Kéo theo 5 service, 5 migration, hợp đồng, và làm một mục trong sổ tiến độ lỗi thời |
| **×1,5** | Đổi thứ đã có **dữ liệu lưu trữ** | Chưa xảy ra. Nếu xảy ra: không còn là sửa mã, là **thủ tục hành chính** (luật 7) |

**Một ca đo được để hiệu chuẩn:** ADR 0024 đáp đất khi 8 bảng danh mục đã viết xong. Lát cắt
bị ảnh hưởng ~1.873 + phần liên quan ≈ 3.000 dòng ⇒ ~0,13 NN × ×1,0 = **~0,13 NN** cho một lần
đổi chủ sở hữu thực thể. Nhân với số lần đổi mà người lập dự kiến.

### 3B.3 Thứ làm buffer này rẻ đi, đã có sẵn

| Cơ chế | Nó cứu cái gì |
|---|---|
| Hợp đồng REST **sinh từ khai báo route**, không viết tay (ADR 0014) | Đổi một trường: sinh lại, không sửa hai nơi |
| `tenant_id` là **ULID vô nghĩa** (luật 1 bất biến 2) | Sáp nhập xã **không** buộc sửa khoá ngoại trên dữ liệu lịch sử |
| Vòng đời phiếu là **cấu hình theo xã** (ADR 0008) | Xã đổi quy trình: đổi cấu hình, không đổi mã |
| Sổ tiến độ + 25 ADR | Biết **chính xác** lát cắt nào bị ảnh hưởng, thay vì đoán |

Không có bốn thứ trên thì buffer này là hạng mục lớn nhất dự án.

---

## 4. HẠNG MỤC 2 — CODING THEO PHÂN HỆ

**Giá trị: 10,25 – 10,75 NN** *(9,25 sau đóng gói + buffer 10–15%)*. Bóc tách dưới đây, dưới
giả thiết **prototype và nghiệp vụ không đổi** (phần đổi nằm ở §3B). Mọi hạng mục đóng gói lên
**bội số 0,25 NN** — quy ước ở §4.2.

### 4.1 Một lát cắt dọc gồm đúng những tầng nào

Mỗi con số ở §4.3 phủ **đủ 6 tầng** — đó là cách dự án này thực sự chạy, một AI làm cả dọc:

| Tầng | Nội dung | Dòng điển hình cho một phân hệ nghiệp vụ |
|---|---|---|
| 1 | Migration + trigger append-only + phân mảnh theo xã | 400–700 |
| 2 | `domain/` — trạng thái, bất biến, phép suy | 300–800 |
| 3 | `store/` — kho có phạm vi xã + test kho | 800–1.500 |
| 4 | `app/` — luồng nghiệp vụ, vết kiểm trong cùng giao dịch | 400–1.000 |
| 5 | `http/` — tuyến, khai quyền tường minh, test 401/403/403-sai-xã/200 | 600–1.200 |
| 6 | `web-admin/` — màn hình, kiểu sinh từ hợp đồng, test kết xuất | 800–1.800 |
| | **Tổng một phân hệ** | **3.300–7.000** |

### 4.2 QUY ƯỚC ĐÓNG GÓI — mốc 0,25 NN, luôn làm tròn LÊN

Mọi hạng mục coding được đóng gói về **bội số của 0,25 NN** (0,25 · 0,50 · 0,75 · 1,00 · 1,25 …)
và **luôn làm tròn lên** — áp cho cả giá trị trên 1:

| NN thô | → đóng gói |
|---|---|
| 0,18 | **0,25** |
| 0,26 | **0,50** |
| 0,57 | **0,75** |
| 1,12 | **1,25** |
| 2,60 | **2,75** |

| Vì sao | |
|---|---|
| Lập lịch được | Một hạng mục là một phần tư, nửa, ba phần tư hay cả ngày — xếp vào lịch không cần chia lẻ |
| Không có hạng mục "gần bằng 0" | Việc nhỏ nhất vẫn tốn đọc đặc tả, ra quyết định, và một lượt qua cổng kiểm. **0,25 NN là sàn thật**, không phải sàn hành chính |
| Phần đệm trở nên **đo được** | Xem ngay dưới |

**Đo được: phép làm tròn lên THAY CHO hệ số ×1,5.**

| | NN |
|---|---|
| Tổng thô 12 phân hệ (61.100 ÷ 22.300) | **2,74** |
| Tổng sau đóng gói | **4,00** |
| Phần đệm do làm tròn | **+46%** |
| Cận trên cũ (×1,5) | 4,11 |

Hai con số 4,00 và 4,11 gần trùng nhau, nên **không cộng cả hai**. Từ đây bản này dùng **NN thô
× đóng gói 0,25**, bỏ hẳn dải ×0,7…×1,5 để tránh đệm hai lần.

> **Một giới hạn của phép đóng gói, nói trước:** hạng mục nằm **sát dưới một mốc** hầu như không
> được đệm gì. Chương 01+03 có NN thô **0,247** và đóng gói thành 0,25 — đệm 1%. Nếu nó vỡ thì
> vỡ toàn bộ. Hạng mục nào sát mốc thì phải xem lại con số dòng dự kiến, đừng tin vào làm tròn.

### 4.3 Mười hai phân hệ chưa dựng

Dòng dự kiến chia **22.300**, rồi đóng gói lên mốc 0,25.

| Chương | Phân hệ | Dòng dự kiến | NN thô | **NN đóng gói** | Vì sao nặng/nhẹ |
|---|---|---|---|---|---|
| 09 | Phản ánh người dân | 8.500 | 0,381 | **0,50** | Nặng nhất: hạn đếm bằng **giờ làm việc** theo lịch từng xã · mọi trạng thái phải thông báo công dân và để lại vết · mã tra cứu không đoán được (luật 10) |
| 06 | Giải ngân | 7.000 | 0,314 | **0,50** | Có tiền, có quyết định hành chính, có số hiệu không được cấp lại |
| 05 | Văn bản đơn thư | 6.500 | 0,291 | **0,50** | Số hiệu văn bản đến/đi · luồng chuyển xử lý · lưu trữ |
| 02 | Nhiệm vụ | 6.000 | 0,269 | **0,50** | Phân công, gia hạn, nhiều trạng thái. Danh mục đã có |
| 01+03 | Tổng quan điều hành + Sổ tay | 5.500 | 0,247 | **0,25** ⚠ | Cần **API thống kê chưa tồn tại**. Sát mốc — xem cảnh báo §4.2 |
| 10 | Bản đồ kinh tế số | 5.000 | 0,224 | **0,25** | Toạ độ + ảnh hiện trường ⇒ chạm dữ liệu cá nhân (luật 3). Danh mục đã có |
| 07 | Thu chi ngân sách | 4.500 | 0,202 | **0,25** | Có tiền nhưng ít trạng thái hơn giải ngân. Danh mục đã có |
| 13 | Báo cáo *(chặn #4)* | 4.000 | 0,179 | **0,25** | Đọc nhiều xã ⇒ mỗi truy vấn phải khai `@cross-tenant` và tự để lại vết |
| 08 | Thông báo | 3.800 | 0,170 | **0,25** | Gửi ra ngoài (ZNS) ⇒ adapter khai rõ trường nào đi ra |
| 04 | Biên bản họp | 3.500 | 0,157 | **0,25** | Chủ yếu CRUD + tệp kèm |
| 11 | Nội dung Mini App | 3.500 | 0,157 | **0,25** | Soạn nội dung cho kênh công dân |
| 12 | Danh bạ cán bộ *(chặn 10 câu)* | 3.300 | 0,148 | **0,25** | Kho + tuyến đọc **đã có**; còn tuyến ghi |
| | **Tổng** | **61.100** | **2,74** | **4,00 NN** | |

### 4.4 Việc nền còn lại — không thuộc phân hệ nào

Phần này **không tính bằng dòng** vì nó không phải viết mã: nó là chạy, đo, và sửa cái gì vỡ.
Vẫn đóng gói lên mốc 0,25.

| Việc | NN thô | **NN đóng gói** | Vì sao không rẻ như dòng mã |
|---|---|---|---|
| Chạy thật 27 migration trên PostgreSQL + sửa cái gì vỡ | 1,50 | **1,50** | **Tầng SQL chưa chạy một lần nào.** Bộ `*_pg_test.go` tự bỏ qua khi thiếu `VIGOV_TEST_DSN` mà gói vẫn in `ok`. Khoảng trống lớn nhất kho, và **không song song hoá được** — phải có người đọc từng lỗi |
| Rà nốt hai lớp lỗi rào chắn đã mở | 0,60 | **0,75** | Xem cảnh báo §4.5 |
| `platform-admin` — console quản trị nền tảng, hiện rỗng | 0,60 | **0,75** | Phải đọc ADR 0003 trước: `platform` chỉ giữ **siêu dữ liệu**, console này không được có đường đọc dữ liệu nghiệp vụ của xã |
| Backfill dữ liệu theo từng xã | 0,45 | **0,50** | Luật 7 bất biến 5 mới đạt một nửa (`core/migrate` chỉ lo DDL) |
| Đệm TTL cho tra cứu phiên công dân + kênh vô hiệu hoá thật | 0,45 | **0,50** | identity chạy nhiều bản sao, chỉ có PostgreSQL (ADR 0010) ⇒ chưa có kênh để một lần thu hồi ở bản sao A với tới bản sao B |
| Đối chiếu 34 tên tỉnh/thành với Nghị quyết 202/2025/QH15 | 0,30 | **0,50** | **Đọc văn bản pháp luật**, không phải viết mã. Cột này in thẳng ra màn hình công dân nên dạng viết **là** nội dung |
| Bộ sinh đọc dấu `@entity` để điền `data-ownership.json` | 0,20 | **0,25** | `CLAUDE.md` dạy phiên sau mở tệp ấy để hỏi "ai sở hữu X"; hiện nó rỗng |
| Ba tuyến GHI danh mục *(chặn #21)* | 0,15 | **0,25** | Khuôn đã có |
| Bảng `loi_he_thong` | 0,10 | **0,25** | Việc nhỏ nhất vẫn tốn một lượt qua cổng kiểm — sàn 0,25 |
| | **4,35** | **5,25 NN** | |

### 4.5 Tổng hạng mục coding

| | NN thô | **NN đóng gói** |
|---|---|---|
| 12 phân hệ (§4.3) | 2,74 | **4,00** |
| Việc nền (§4.4) | 4,35 | **5,25** |
| Cộng | **7,09** | **9,25** |
| **Buffer coding 10–15%** *(§4.6)* | 0,93 – 1,39 | **1,00 – 1,50** |
| **CHỐT** | | **10,25 – 10,75 NN** |

Bỏ hẳn cách ghi dải 7–10 của bản trước: đóng gói lên mốc 0,25 đã làm đúng việc mà hệ số ×1,5
từng làm, nên không ghi cả hai.

### 4.6 HAI PHẦN ĐỆM, HAI VIỆC KHÁC NHAU — và tổng cộng lại là bao nhiêu

Phải nói rõ vì hai phần này **xếp lên nhau**, và người đọc hợp đồng có quyền biết tổng đệm thật.

| Phần đệm | Giá trị | Nó chịu rủi ro gì | Vì sao không trùng phần kia |
|---|---|---|---|
| **A — do đóng gói 0,25** | +2,16 NN (**+30%**) | **Nhiễu hạt** của từng hạng mục: con số dòng dự kiến lệch trong phạm vi một hạng mục đã biết | Nằm **bên trong** từng hạng mục, và mỗi hạng mục có tên |
| **B — buffer 10–15% trên tổng** | +1,00 … +1,50 NN | **Hạng mục chưa ai nghĩ tới**: một phân hệ phát sinh khi thi công, một tầng bị bỏ sót khỏi bảng §4.1, một ràng buộc pháp lý mới lộ ra | Nằm **ngoài** mọi hạng mục — nếu nó nằm trong thì đã có tên rồi |

**Tổng đệm thật so với con số thô:**

| | NN | So với thô |
|---|---|---|
| Thô | 7,09 | — |
| Sau đóng gói | 9,25 | ×1,30 |
| Sau buffer 10% | 10,25 | **×1,45** |
| Sau buffer 15% | 10,75 | **×1,52** |

Đệm A neo vào một thứ **đã đo được**, không phải ước:

> **Trong 5 ngày đã đo, thứ tốn thời gian nhất không phải viết mã — là 7 ca "rào chắn trông
> như đang canh mà đã chết", và cả 7 đều XANH ở cổng kiểm khi chúng chết.** Ví dụ: mẫu của
> `tenant_scope_guard` đòi `.Query(` không hậu tố trong khi cả 20 lời gọi CSDL của kho đều là
> bản `*Context` — rào chắn mà luật 1 nêu tên làm cơ chế chặn **không khớp một call site nào**.
> Mỗi ca cùng lớp tốn ~0,2–0,5 NN để truy. **+2,16 NN ≈ 4–10 ca.** Dự án này gặp 7 ca trong
> 5 ngày, nên đệm A không dư — và **nó đã có chủ**, không dùng được cho việc của đệm B.

**Chọn 10% hay 15%:**

| Chọn | Khi nào |
|---|---|
| **10%** | Đặc tả đã ở dạng văn bản đọc được, đủ 16 chương, và phạm vi `service-dossiers` đã xác định |
| **15%** | Còn phân hệ chưa có đặc tả — **đúng tình trạng hiện tại**: `service-dossiers` không có chương nào trong 16 chương (§3.3) |

⇒ Với dự án này, **đề nghị 15% ⇒ coding = 10,75 NN**.

### 4.7 Điều kiện để con số 10,25 – 10,75 còn đúng

| Điều kiện | Nếu vỡ |
|---|---|
| Prototype và nghiệp vụ **không đổi** | → §3B, không phải §4 |
| Chạy được **nhiều agent song song** với ranh giới ghi rời nhau | Mất song song ⇒ nhân **×2..×3**. Ranh giới ấy là §2 |
| Verification vẫn **tuần tự** ở luồng chính | `go.mod`, `gen/`, `make kb`, cổng kiểm **không** song song hoá được — đã tính |
| Đặc tả ở dạng **văn bản đọc được** | Chỉ có prototype để xem ⇒ mỗi phân hệ cộng thêm một buổi ở §3 |

---

## 5. HẠNG MỤC 3 — DEPLOYMENT *(tách làm hai)*

### 5.1 — 3A · Sizing và cung cấp hạ tầng *(việc của bên khác)*

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay · **___ NC** ← nhập tay |
| Ai làm | Đội devops / nhà cung cấp / chủ dự án |

**Phân tích — thứ phải có đủ và ĐÚNG trước khi 3B bắt đầu:**

| Cần | Vì sao nó là điều kiện, không phải chi tiết |
|---|---|
| Cụm Kubernetes + quyền cho Jenkins | `deploy/cluster/` đã có manifest, chưa ai `kubectl apply` |
| Registry ảnh (Harbor) | 10 Jenkinsfile đóng ảnh, dừng ở Harbor, chưa chạy thật |
| **Tên miền theo từng xã + chứng thư** | Hệ thống phân biệt xã **bằng domain** (luật 1 bất biến 3). Thiếu domain thì không kiểm được gì có ý nghĩa |
| **Trả lời: cụm có split-horizon DNS / chặn egress không** | Tiến trình Next.js gọi `https://<Host>/api/v1/communes/current` bằng **tên miền công khai**. Nếu cụm chặn thì **mọi yêu cầu trả 500**, và `web-admin` hiện không có tệp mẫu env nào để thêm biến gốc API nội bộ |
| PostgreSQL có phân mảnh + khoá KEK ngoài CSDL | ADR 0009 · 0010 |

### 5.2 — 3B · AI dựng deployment *(sau khi hạ tầng đã đủ và đúng)*

| | NN |
|---|---|
| **Bước 1 — Validate** | **1,0** |
| **Bước 2 — Thực thi** | **2,0** |
| **Tổng** | **3,0 NN** |

**Bước 1 — Validate (1,0 NN).** Không phải thủ tục hành chính; nó là bước **fail closed** cho
hạ tầng:

- Đối chiếu tài nguyên nhận được với `deploy/base/` và `overlays/`
- Kiểm `Host` phân giải đúng xã, và **`Host` không phân giải được thì trả 404** (luật 1 bất biến 3)
- Kiểm biến bí mật tới được từ secret của cụm, **không** nằm trong ảnh (luật 8)
- Kiểm cờ nguy hiểm (bypass auth, demo mode) **tắt** trước khi chạy thật (luật 8 bất biến 7)
- Kiểm cách ly hai xã trên hạ tầng thật: dựng 2 tenant, thử đọc chéo

**Bước 2 — Thực thi (2,0 NN).**

| Việc | Đã có gì |
|---|---|
| Manifest cho 8 đơn vị còn lại | `deploy/base/` đã có 4/12 (`identity`, `platform`, `web-admin`, `mang`) — 8 cái còn lại là **lặp khuôn kustomize**, rẻ |
| Chạy 10 Jenkinsfile trên Jenkins thật | Đã có đủ 10 tệp; chưa chạy |
| Ingress + domain từng xã + chứng thư | `overlays/{prod,staging}/ingress.yaml` đã có khung |
| Logs tập trung + cảnh báo | Chưa có |

**Phản biện của tôi về con số 3 NN — chấp nhận được, kèm một điều kiện:**

3 NN đúng **nếu chữ "đúng yêu cầu" ở 3A thành thật**. Rủi ro tập trung vào đúng một chỗ: câu
split-horizon DNS / chặn egress. Nếu cụm chặn, thì phát sinh **thêm ~1,0 NN** — phải thêm biến
gốc API nội bộ, thêm tệp mẫu env cho `web-admin`, và sửa đường gọi phía server. **Bước Validate
tồn tại chính để phát hiện việc này trước khi tiêu 2 NN của bước Thực thi**, chứ không phải
sau. Đề nghị ghi vào hợp đồng: 3 NN + 1 NN **có điều kiện**, điều kiện nêu tên rõ.

---

## 6. HẠNG MỤC 4 — ZALO MINI APP *(tách làm ba)*

**Mobile app native: KHÔNG có trong dự án này.** Kênh công dân chỉ đi qua Zalo Mini App.

### 6.1 — 4A · Tài liệu cho yêu cầu xác thực *(bên thứ ba)*

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay · **___ NC** ← nhập tay |
| Ai làm | Chủ dự án + Zalo (bên thứ ba) |

**Phân tích — danh mục phải có, và ai giữ:**

| Hạng mục | Ai giữ | Ghi chú |
|---|---|---|
| Hồ sơ doanh nghiệp, giấy phép | Chủ dự án | Zalo bắt buộc với app gắn dịch vụ công |
| Quyền OA, xác thực số điện thoại OA | Chủ dự án | **Hai OA riêng**: OA xác thực tách khỏi OA thông báo (ADR 0018) |
| Logo, icon, ảnh chụp màn hình, mô tả store | Chủ dự án cấp nội dung | **Kho hiện chưa có tệp ảnh nào** |
| Chính sách quyền riêng tư, điều khoản | Chủ dự án | Chạm dữ liệu cá nhân ⇒ Nghị định 13/2023 |
| **Ba câu chỉ Zalo trả lời được** | Zalo | Ràng buộc 1 Mini App ↔ 1 OA · app hồ sơ doanh nghiệp sau này gắn dịch vụ công có phải xác thực lại · tham số deep link có tới app khi app chạy nền. **Cả ba hiện là giả định đứng trên nguồn thứ cấp**, kể cả ADR 0018 |

### 6.2 — 4B · Coding các chức năng + đẩy lên Mini App

Đóng gói theo cùng quy ước §4.2.

| Việc | NN thô | **NN đóng gói** |
|---|---|---|
| Giai đoạn 2 — màn hình kênh công dân | 1,20 | **1,25** |
| Đăng nhập công dân (OTP qua OA xác thực, QR ghép phiên) | 0,50 | **0,50** |
| Đẩy lên Mini App: `app-config.json`, thư mục build, gói nộp | 0,40 | **0,50** |
| **Tổng** | **2,10** | **2,25 NN** |

Giai đoạn 1 **đã xong** (11.560 dòng, 369 ca test) — app giới thiệu, 9 quyền Zalo thành 9 tính
năng thật. Đó là thứ duy nhất trong kho sẵn sàng giao ra ngoài.

**Một rủi ro mã, đã biết, nằm ở bước đẩy lên:** tên khoá trong `app-config.json` hiện viết từ
**nguồn thứ cấp** (tài liệu Zalo render phía client nên không đọc trực tiếp được), và thư mục
build đang là `dist/` (mặc định Vite) trong khi `zmp-cli` thường dùng `www/`. **Sai khoá là hồ
sơ bị trả về** — mất một vòng duyệt, tức tiêu NC ở §6.3 chứ không tiêu NN ở đây. Phải đối chiếu
Developer Console **trước** khi nộp.

### 6.3 — 4C · Chờ phản hồi và xử lý yêu cầu sau in-review

| | |
|---|---|
| **NN xử lý mỗi vòng** | **0,5 – 1,0** |
| **Số vòng dự kiến** | **___ vòng** ← nhập tay |
| **NC mỗi vòng** | **5 – 20** |

**Phân tích — vì sao số vòng phải nhập tay, không đoán:** nó phụ thuộc Zalo, và ba giả định ở
§6.1 chưa ai xác nhận. Nếu một trong ba sai thì phát sinh **sửa kiến trúc**, không phải sửa
giao diện — nặng nhất là ràng buộc 1 Mini App ↔ 1 OA, vì ADR 0018 (OA xác thực tách khỏi OA
thông báo) đứng trên đúng giả định ấy.

Đề nghị ghi hợp đồng: **1 vòng nằm trong giá, từ vòng 2 tính thêm** — vì số vòng không nằm
trong tay đội thi công.

---

## 7. HẠNG MỤC 5 — TESTING *(tách làm hai người làm)*

### 7.1 — 5A · Người làm tự test

| | |
|---|---|
| **Giá trị** | **1 – 3 NN** |

**Vì sao ít:** chức năng **đã được review ngay lúc đẩy lên**. Mỗi lát cắt dọc ở §4 đã mang
theo test ở cả 6 tầng, và cổng kiểm chạy trước mỗi lần kết phiên. 34% kho là test
(**36.435 dòng**). Phần này chỉ còn là **chạy thật những thứ cổng kiểm không phủ**:

| Nội dung | Vì sao phải làm tay |
|---|---|
| Chạy toàn bộ bộ `*_pg_test.go` trên PostgreSQL thật | Thiếu `VIGOV_TEST_DSN` ⇒ **tự bỏ qua và gói vẫn in `ok`** |
| Cài `golangci-lint` và chạy lần đầu | Không có trên máy phát triển; mục `lint` **nuốt lỗi bằng tiền tố `-`** ⇒ chưa từng chạy |
| Đưa `platform-admin` vào cổng | `make check` in "BỎ QUA vì chưa có `node_modules`" rồi **vẫn trả rc=0** |
| Cách ly hai xã trên dữ liệu thật | Luật 1 — không hook nào kiểm được quan hệ giữa hai xã thật |
| **Đột biến** cho 17 hook | Gỡ thứ mỗi hook đáng lẽ phải chặn rồi xem nó có chặn không. **7 ca đã chết trong im lặng** trong 5 ngày; phép kiểm đáng tin duy nhất là đột biến |
| Kiểm tay theo đặc tả từng phân hệ | Cổng nói mã chạy được; nó **không** nói kết luận nghiệp vụ đúng |

*(Ba dòng đầu đã tính trong §4.4 — không cộng lại.)*

### 7.2 — 5B · BA hoặc tester tham gia

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay |
| Ai làm | BA / tester của dự án — **không phải người định hướng** |

**Nội dung cần liệt kê cho người test, không kèm số:**

| Nhóm | Nội dung |
|---|---|
| Theo phân hệ | Chạy đúng luồng trong `docs/ui-ux/` từng chương · đối chiếu nhãn và thuật ngữ hành chính · kiểm số hiệu văn bản và con số trên báo cáo |
| Cách ly | Đăng nhập xã A, thử thấy dữ liệu xã B · công dân thử xem phiếu của người khác · cán bộ sai vai trò gọi tuyến ngoài quyền |
| Cam kết với công dân | Hạn xử lý đếm đúng **giờ làm việc** của xã (không phải giờ đồng hồ) · mỗi lần đổi trạng thái công dân **có nhận được thông báo** · đóng phiếu có kết quả công dân đọc được |
| Dữ liệu cá nhân | Số điện thoại và số định danh **bị che** ở mọi màn hình và mọi bản xuất, trừ khi có quyền xem đầy đủ — và lần xem đầy đủ ấy **có nằm trong vết kiểm** |
| Kênh công dân | Trên máy thật, tài khoản Zalo thật · cỡ chữ và vùng bấm cho người cao tuổi · thông báo ZNS tới đúng người |
| Vết kiểm | Mỗi thao tác ghi có một dòng: ai · làm gì · lúc nào · từ IP nào · **ở xã nào** |

---

## 8. HẠNG MỤC 6 — BÀN GIAO

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay · dự kiến **1 – 3 NN** |

Chỉ **note chức năng** — phần kiểm chất lượng đã nằm ở §7.2 với tester.

**Nội dung cần note, không kèm số:**

| Nhóm | Nội dung |
|---|---|
| Theo phân hệ | Làm được gì · ai được làm (khoá quyền nào) · cấu hình theo xã ở đâu |
| Vận hành | Onboard một xã mới (đây là hệ thống **nhiều xã**) · xoay khoá ký và khoá phiên · nơi đọc vết kiểm |
| Giới hạn đã biết | Ba giới hạn của xác thực service↔service bằng khoá chung (ADR 0025) · `ListCitizenCommunes` chưa cài · chưa có interceptor bắt panic trên cổng gRPC |
| Nợ phải nói rõ | **Dữ liệu cá nhân còn trong lịch sử git** — gỡ là viết lại lịch sử `main`, cần quyết định của chủ dự án |
| Bàn giao bộ não AI | `kb/INDEX.yaml` → `always_load` → chỉ mục · `/progress` · `/handover` · vì sao không được sửa tay tầng sinh |

---

## 9. TỔNG HỢP

Mọi giá trị NN đã đóng gói lên **bội số 0,25** (§4.2).

| # | Hạng mục | Ai làm | NN | Nhập tay | NC |
|---|---|---|---|---|---|
| 0 | Bộ não AI | AI + người định hướng | *1,50 đã tiêu* | — | — |
| 1 | Nghiệp vụ · prototype · họp chốt | người định hướng + khách | `___ × 0,50` | **số buổi `___`** | 9–35 |
| 1B | **Buffer thay đổi nghiệp vụ/prototype** | AI | — | **`___` NN** | — |
| 2 | Coding theo phân hệ *(9,25 + buffer 10–15%)* | AI | **10,25 – 10,75** | — | — |
| 3A | Sizing + cung cấp hạ tầng | devops / chủ dự án | — | **`___` NN · `___` NC** | — |
| 3B | Deployment: validate + thực thi | AI | **3,00** *(+1,00 có điều kiện)* | — | — |
| 4A | Zalo: tài liệu xác thực | chủ dự án + Zalo | — | **`___` NN · `___` NC** | — |
| 4B | Zalo: coding + đẩy lên | AI | **2,25** | — | — |
| 4C | Zalo: chờ phản hồi + xử lý sau review | AI + Zalo | `___ ×` (0,50–1,00) | **số vòng `___`** | 5–20/vòng |
| 5A | Testing — người làm tự test | AI + người định hướng | **1,00 – 3,00** | — | — |
| 5B | Testing — BA / tester | BA / tester | — | **`___` NN** | — |
| 6 | Bàn giao — note chức năng | người định hướng | — | **`___` NN** *(dự kiến 1,00–3,00)* | — |
| | **Phần AI chịu trách nhiệm, đã chốt số** | | **16,50 – 19,00 NN** | | |

**Cộng thêm các ô nhập tay** để ra tổng hợp đồng. Với dự án này, nếu điền theo dự kiến ở từng
mục — 6 buổi = 3,00 · buffer nghiệp vụ 1,00 · 3A 0 (của bên khác) · 4A 0 (của bên khác) ·
1 vòng Zalo 1,00 · 5B 3,00 · bàn giao 2,00 — thì tổng **≈ 26,50 – 29,00 NN**, lịch
**≈ 45 – 70 NL** tuỳ tốc độ khách chốt 15 câu hỏi.

**Ba phần đệm trong bản này, đừng lẫn:** đóng gói 0,25 (§4.6-A, nằm trong từng hạng mục coding)
· buffer coding 10–15% (§4.6-B, cho hạng mục coding **chưa ai nghĩ tới**) · buffer thay đổi
nghiệp vụ/prototype (§3B, nhập tay, cho việc **khách đổi ý**). Ba rủi ro khác nhau, ba chủ khác
nhau — gộp lại là không biết đang đệm cho cái gì.

### 9.1 Đối chiếu để tin được dải này

| | |
|---|---|
| Đã tiêu | **5,50 NN** → nền móng đầy đủ + **2/16** phân hệ + bộ não + 25 ADR |
| Còn lại | **16,50–19,00 NN** → **14/16** phân hệ + deployment + Zalo + testing |

Tỉ lệ ấy hợp lý vì **phần đắt là nền móng và nó đã xong**: hợp đồng REST sinh từ mã, xác thực
dùng chung, rìa đa xã, vết kiểm, và 11 agent có ranh giới ghi rời nhau. Mỗi phân hệ mới nay là
**lặp một khuôn đã chạy được**.

### 9.2 Quy đổi sang mô hình cũ, nếu khách cần so sánh

5,5 NN đã sinh 107.673 dòng (34% test), 25 ADR, 8 service, 3 app, 17 tuyến REST. Mốc thường của
đội viết tay là 50–150 dòng sản phẩm/ngày-người kể cả test và tài liệu ⇒ **700–2.100 ngày-người**
cho cùng khối lượng.

Con số ấy **đúng về khối lượng và sai về bản chất**. Mô hình này đổi chi phí viết mã thành **chi
phí kiểm soát**: thứ tính tiền không phải mã, mà là bộ não cưỡng chế, 25 ADR, và **một người
chịu trách nhiệm về từng quyết định**. Cắt phần kiểm soát thì vẫn ra lượng mã ấy, nhưng không ai
biết nó đúng hay sai — với hệ thống hành chính nhà nước, đó không phải một lựa chọn.

---

## 10. RỦI RO ĐÃ BIẾT, CÓ GIÁ

| Rủi ro | Xác suất | Giá | Giảm bằng cách |
|---|---|---|---|
| Khách trả lời #15 #16 sau khi đã có dữ liệu thật | Trung bình | `ALTER TABLE` trên **bản ghi lưu trữ** — thủ tục hành chính, không phải lệnh | Đưa lên buổi họp đầu |
| Cụm chặn egress / split-horizon DNS | Chưa ai xác nhận | **+1,0 NN** và mọi yêu cầu `web-admin` trả 500 nếu phát hiện muộn | Hỏi devops **trước** khi nhận resources; bước Validate §5.2 |
| Zalo trả hồ sơ vì sai khoá `app-config.json` | Trung bình | 1 vòng = 0,5–1,0 NN + 5–20 NC | Đối chiếu Developer Console trước khi nộp |
| Một trong ba giả định Zalo sai | Chưa ai xác nhận | Sửa **kiến trúc**, không sửa giao diện. Nặng nhất: 1 Mini App ↔ 1 OA (ADR 0018 đứng trên nó) | Hỏi cùng lượt nộp |
| Phát sinh **lớp lỗi tiềm ẩn mới** kiểu "rào chắn đã chết" | **Cao — 7 ca trong 5 ngày** | 0,2–0,5 NN mỗi ca | Đã tính trong việc làm tròn §4.4 lên 7 |
| Mất khả năng chạy nhiều agent song song | Thấp | **×2..×3 toàn bộ §4** | Giữ ranh giới ghi của 11 agent (§2) |
| Phạm vi `service-dossiers` chưa xác định | Cao | +1,0 NN nếu trong phạm vi | Hỏi ngay lượt chỉnh bản này |
| Sáp nhập/chia tách xã giữa dự án (#1) | Thấp, tác động lớn | **Đã phòng**: `tenant_id` là ULID vô nghĩa | Không cần làm gì thêm |

---

## 11. BẢN NÀY KHÔNG BAO GỒM

| Không bao gồm | Vì sao |
|---|---|
| App mobile native | Kênh công dân đi qua Zalo Mini App. Nếu cần, là hạng mục mới |
| Chi phí hạ tầng (cụm, Harbor, chứng thư, ZNS) | Không phải công thi công — §5.1 |
| Chi phí token AI | Tính riêng theo lượt dùng thật; nhỏ hơn hẳn chi phí người định hướng |
| Vận hành sau bàn giao (SLA, trực sự cố) | Hợp đồng riêng |
| `service-dossiers` — hồ sơ một cửa | Chưa có đặc tả — §3.3 |
| Di trú dữ liệu từ hệ thống cũ | Khách chưa nêu |
| Đào tạo cán bộ từng xã | Khác với bàn giao ở §8 |

---

## 12. CÁCH DÙNG BẢN NÀY CHO DỰ ÁN TƯƠNG TỰ

| Phải đo lại | Dùng lại được |
|---|---|
| Dòng dự kiến từng phân hệ (§4.3) — theo đặc tả của dự án ấy | **22.300 dòng/NN** |
| Số buổi chốt nghiệp vụ và số câu hỏi còn mở (§3) | **Quy ước đóng gói 0,25 NN, làm tròn lên** (§4.2) — kèm số đo rằng nó thay được hệ số ×1,5 |
| Chọn 10% hay 15% cho buffer coding — theo việc đặc tả có đủ hay không (§4.6) | **Ba phần đệm và ranh giới giữa chúng** (§4.6 + §3B): nhiễu hạt · hạng mục chưa ai nghĩ tới · khách đổi ý |
| Phần bộ não phải viết lại vì nghiệp vụ khác (§2) | Bảng 6 tầng của một lát cắt dọc (§4.1) · bốn hệ số lan của buffer (§3B.2) |
| Các ô `___` — đều phụ thuộc dự án hoặc bên khác | Cấu trúc 6 hạng mục + ba loại thời gian NN/NC/NL + bảng rủi ro |

**Hai điều kiện để 22.300 dòng/NN còn đúng:** đặc tả đã ở dạng **văn bản đọc được** (không phải
chỉ có prototype để xem), và có **một** người quyết được nghiệp vụ mà không phải xin ý kiến vòng
hai. Thiếu một trong hai, phần thiếu **không nằm ở coding** — nó nằm ở §3 và §3B.
