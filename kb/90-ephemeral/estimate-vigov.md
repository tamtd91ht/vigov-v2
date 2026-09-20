---
id: estimate-vigov
tier: T5
source: CURATED
owner: architecture
derived_from_commit: b6ca85b
expires: 2026-12-19
owns_facts:
  - "khối lượng thi công toàn dự án ViGov v2 và cách quy nó thành ngày-người"
---

# Estimate thi công — ViGov v2

**Báo cáo đầu kỳ.** Ước cho **toàn bộ** dự án, lập trước khi thi công.

<!-- NOI-BO -->
**Phương pháp** (đơn vị, hệ số, quy ước đóng gói, khung hạng mục, ba phần đệm) thuộc về
`kb/90-ephemeral/estimate-khung-chung.md`. Bản này **nhắc lại** phương pháp ấy vì nó phải đứng
một mình khi gửi ra ngoài — nên **sửa phương pháp ở khung chung trước**, rồi mới lan về đây.
<!-- /NOI-BO -->


**Ô ghi `___` là ô NHẬP TAY** — người lập estimate điền, vì nó phụ thuộc cách làm việc với
khách hoặc phụ thuộc bên khác. Phần phân tích cho những ô ấy vẫn có ở đây; chỉ con số là của
người lập.

---

## 0. MÔ HÌNH NHÂN SỰ VÀ ĐƠN VỊ ĐO

| | |
|---|---|
| Nhân sự | **AI code 100% · 1 người định hướng và confirm** |
| Đơn vị | **ngày-người (NN)** của người định hướng — 1 ngày = 8 giờ làm việc thật |
| Không dùng | man-day lập trình viên — mô hình này không có lập trình viên |

**Vì sao đơn vị phải khác.** Sản lượng không tỉ lệ với số người, nó tỉ lệ với **số quyết định
người ấy ra được trong một ngày**. Thêm người thứ hai không làm nhanh hơn nếu cả hai chờ cùng
một câu trả lời từ khách.

**Ba loại thời gian, đừng cộng gộp:**

| | Ý nghĩa |
|---|---|
| **NN** | công của người định hướng — thứ tính tiền |
| **NC** | ngày chờ: khách chốt, devops cấp máy, Zalo duyệt. **Không tiêu công, nhưng tiêu lịch** |
| **NL** | ngày lịch = NN + NC, chồng lấn được một phần |

---

## 1. SỐ ĐO HIỆU CHUẨN

Mọi con số trong bản này quy về một hệ số duy nhất, hiệu chuẩn trên một chu kỳ thi công thật
theo đúng mô hình này (Go microservices · Next.js · Zalo Mini App · nhiều xã trên hạ tầng chung):

| | |
|---|---|
| **Hệ số** | **22.300 dòng mỗi NN** |
| Tỉ lệ test trong đó | **34%** — hệ số đã bao gồm chi phí viết test |
<!-- NOI-BO -->
| Cách đo | 122.625 dòng (kể cả test và tài liệu) trên 5,5 NN |

### 1.1 Kiểm chứng trên một lát cắt dọc thật

Một cụm danh mục hoàn chỉnh — 8 bảng + 9 tuyến đọc + mắc xác thực + sinh hợp đồng + việc cho web:

| Phần | Dòng |
|---|---|
| 8 bảng danh mục (migration) | 1.873 |
| 3 tuyến đọc + kho + test | 2.310 |
| 6 tuyến đọc ở 4 service | 7.666 |
| Mắc xác thực cán bộ vào 4 service | 1.448 |
| Sinh lại hợp đồng REST + phát sinh việc web | 1.566 |
| **Tổng** | **14.863** ⇒ **0,67 NN** |
<!-- /NOI-BO -->

### Giới hạn của hệ số này

22.300 dòng/NN đo trong giai đoạn dựng nền. Phân hệ nghiệp vụ trên nền đã có thì **nhanh hơn
trên mỗi dòng** (khuôn đã có) nhưng **nhiều quyết định hơn trên mỗi dòng**. Phần đệm cho chuyện
đó không đến từ một hệ số nhân, mà từ **quy ước đóng gói lên mốc 0,25 NN** (§4.2) cộng **một
dòng buffer riêng** (§4.7).

**Hệ số chỉ còn đúng khi:** đặc tả ở dạng **văn bản đọc được** (không phải chỉ có prototype để
xem), và có **một** người quyết được nghiệp vụ mà không phải xin ý kiến vòng hai.

---

## 2. HẠNG MỤC 0 — DỰNG BỘ NÃO AI

*(estimate.txt mục 2: "Con người xây dựng bộ não AI để làm việc và kiểm soát xuyên suốt")*

Hạng mục **không nhìn thấy trong sản phẩm** và quyết định mọi hạng mục sau. Phải tính tiền riêng.

| Thành phần | Quy mô dự kiến | Vai trò |
|---|---|---|
| Luật cưỡng chế | ~11 luật, mỗi luật nêu tên hook thực thi | Điều AI **không được phép** làm |
| Hook | ~17 | Chặn **trước khi** ghi, không phải review sau |
| Agent | ~11 (dựng · soát), mỗi agent một **ranh giới ghi** | Nền của việc chạy nhiều agent song song |
| Skill · Command | ~21 · ~9 | Tri thức nạp theo từ khoá · thủ tục lặp lại |
| Tầng tri thức | 5 tầng theo **tuổi thọ**, có chỉ mục nạp mọi phiên | Chống việc mỗi phiên khám phá lại hệ thống |
| Tự kiểm bộ não | Bất biến cấu trúc + ca chặn/ca cho qua cho **từng** hook | Rào không có test là rào sẽ chết trong im lặng |

| | NN |
|---|---|
| **Giá trị** | **1,50** |

> **Đây là điều kiện để §4 rẻ như thế.** Ranh giới ghi của các agent chính là thứ cho phép
> nhiều việc chạy song song trong một ngày mà không ghi đè nhau. Cắt hạng mục này thì không chỉ
> mất phần kiểm soát — **mất luôn tốc độ**, và §4 phải nhân ×2..×3.

---

## 3. HẠNG MỤC 1 — NGHIỆP VỤ · PROTOTYPE · HỌP CHỐT

### 3.1 Giá trị

| | |
|---|---|
| Đơn giá một buổi | **0,50 NN** (chuẩn bị phương án + họp + ghi quyết định thành ADR) |
| Số buổi | **___ buổi** ← nhập tay |
| **Thành tiền** | **`___` × 0,50 NN** |

Số buổi do **người trực tiếp làm việc với khách** xác định: phụ thuộc cách khách họp, số người
phải có mặt để chốt, và việc khách đã có prototype chạy được hay chưa.

### 3.2 Phân tích — cơ sở để điền số buổi

Đặc tả nhận vào: **16 chương · ~3.900 dòng**, chép từ prototype đang chạy của khách.

Các câu phải chốt, gom theo **người ra quyết định** (không theo phân hệ — hai câu cùng một
người quyết thì vào một buổi):

| Nhóm | Chủ đề | Mở khoá được | NC chờ khách |
|---|---|---|---|
| 1 | Mật khẩu đầu tiên của cán bộ mới · tự đặt lại mật khẩu · "ghi nhớ đăng nhập" | Toàn bộ luồng cấp tài khoản cán bộ | 1–5 |
| 2 | Khoá hay xoá cán bộ · chặn mất quản trị viên cuối cùng · tự thao tác lên chính mình | Toàn bộ tuyến GHI danh bạ cán bộ | 1–5 |
| 3 | Che hay không che số di động cán bộ · ai quyết việc công khai lên Mini App | Cột hiển thị + khoá quyền của danh bạ | 1–3 |
| 4 | Mã cán bộ do ai đặt · `dien_thoai` và `di_dong` là một trường hay hai | **Chặn schema** — xem cảnh báo dưới | 1–5 |
| 5 | Công dân sửa lời khai đã được cán bộ xác thực thì sao · tên khoá quyền cho việc xác thực · xã được sửa danh mục tới mức nào | Màn xác thực lời khai · tuyến ghi danh mục | 2–7 |
| 6 | Sáp nhập/chia tách xã · cấp huyện-tỉnh xem tổng hợp nhiều xã tới mức chi tiết nào | Phân hệ báo cáo · mọi đường đọc chéo xã | 3–10 |

**Hai nhóm đắt bất đối xứng — đưa lên buổi đầu:**

| Nhóm | Trả lời muộn tốn gì |
|---|---|
| **4** | **Chặn schema.** Trả lời sau khi có dữ liệu thật ⇒ `ALTER TABLE` trên **bản ghi lưu trữ** — thủ tục hành chính, không phải một lệnh |
| **5** | **Một chiều.** Thêm lịch sử phiên bản về sau chỉ ghi được từ lúc thêm; những lần công dân đã sửa lời khai trước đó là **mất vĩnh viễn** |

> **Đường găng của dự án này là quyết định nghiệp vụ, không phải coding.** Sáu nhóm trên chặn
> phần lớn các phân hệ có tuyến ghi. Coding chạy liên tục được; chờ khách thì không mua được
> bằng tiền.

### 3.3 Phạm vi CHƯA XÁC ĐỊNH — phải hỏi trước khi ký

**Hồ sơ một cửa** không có chương đặc tả nào trong 16 chương nhận được. Không estimate được.
Nếu trong phạm vi hợp đồng: ước sơ bộ **+0,75 NN** theo hệ số §1, và phải có đặc tả trước.

---

## 3B. HẠNG MỤC 1B — BUFFER THAY ĐỔI NGHIỆP VỤ / PROTOTYPE

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay |

### 3B.1 Vì sao là hạng mục riêng, không phải phần trăm đệm

§4 estimate **dưới giả thiết prototype và nghiệp vụ không đổi**. Giả thiết ấy hầu như không
đúng trọn vẹn, và khi nó vỡ thì **chi phí không rơi vào phân hệ đang làm — nó rơi vào phân hệ
đã xong**. Đó là lý do nó không gộp được vào §4 dưới dạng %.

### 3B.2 Công thức đề nghị

> **Buffer ≈ Σ (dòng của lát cắt bị ảnh hưởng ÷ 22.300) × hệ số lan**

| Hệ số lan | Khi nào | Ví dụ |
|---|---|---|
| **×0,3** | Đổi nhãn, đổi thứ tự, đổi văn bản hiển thị | Đổi tiêu đề cột trên một bảng |
| **×0,5** | Đổi một trường **trên dây** — hợp đồng sinh lại, web phải theo | Đổi tên một trường trong phản hồi API |
| **×1,0** | Đổi **chủ sở hữu** một thực thể giữa các service | Danh mục chuyển từ service nền tảng sang service sở hữu: kéo theo 5 service, 5 migration, hợp đồng |
| **×1,5** | Đổi thứ đã có **dữ liệu lưu trữ** | Không còn là sửa mã — là **thủ tục hành chính** (dữ liệu lưu trữ không được sửa lặng lẽ) |

**Mốc để ước:** một lần đổi chủ sở hữu thực thể ở mức ~3.000 dòng bị ảnh hưởng ⇒ ~0,13 NN.
Nhân với số lần đổi mà người lập dự kiến.

### 3B.3 Bốn cơ chế làm buffer này rẻ đi — phải dựng từ đầu, không vá sau

| Cơ chế | Nó cứu cái gì |
|---|---|
| Hợp đồng REST **sinh từ khai báo route**, không viết tay | Đổi một trường: sinh lại, không sửa hai nơi |
| `tenant_id` là **định danh vô nghĩa (ULID)**, không phải mã hành chính | Sáp nhập xã **không** buộc sửa khoá ngoại trên dữ liệu lịch sử |
| Vòng đời phiếu và SLA là **cấu hình theo xã** | Xã đổi quy trình: đổi cấu hình, không đổi mã |
| Sổ tiến độ theo module + ADR cho mọi quyết định | Biết **chính xác** lát cắt nào bị ảnh hưởng, thay vì đoán |

Không có bốn thứ trên thì buffer này là hạng mục lớn nhất dự án.

---

## 4. HẠNG MỤC 2 — CODING

Dưới giả thiết **prototype và nghiệp vụ không đổi** (phần đổi ở §3B).

### 4.1 Một lát cắt dọc gồm đúng những tầng nào

Mỗi con số ở §4.3–§4.5 phủ **đủ 6 tầng** — một AI làm cả dọc, không tách backend/frontend:

| Tầng | Nội dung | Dòng điển hình cho một phân hệ nghiệp vụ |
|---|---|---|
| 1 | Migration + trigger append-only + phân mảnh theo xã | 400–700 |
| 2 | `domain/` — trạng thái, bất biến, phép suy | 300–800 |
| 3 | `store/` — kho có phạm vi xã + test kho | 800–1.500 |
| 4 | `app/` — luồng nghiệp vụ, vết kiểm trong cùng giao dịch | 400–1.000 |
| 5 | `http/` — tuyến, khai quyền tường minh, test 401/403/403-sai-xã/200 | 600–1.200 |
| 6 | Màn hình quản trị — kiểu sinh từ hợp đồng, test kết xuất | 800–1.800 |
| | **Tổng một phân hệ** | **3.300–7.000** |

### 4.2 QUY ƯỚC ĐÓNG GÓI — mốc 0,25 NN, luôn làm tròn LÊN

Mọi hạng mục coding đóng gói về **bội số của 0,25 NN**, **luôn làm tròn lên**, áp cả trên 1:

| NN thô | → đóng gói |
|---|---|
| 0,18 | **0,25** |
| 0,26 | **0,50** |
| 0,57 | **0,75** |
| 1,12 | **1,25** |
| 2,60 | **2,75** |

| Vì sao | |
|---|---|
| Lập lịch được | Một hạng mục là một phần tư, nửa, ba phần tư hay cả ngày |
| Không có hạng mục "gần bằng 0" | Việc nhỏ nhất vẫn tốn đọc đặc tả, ra quyết định, và một lượt qua cổng kiểm. **0,25 NN là sàn thật** |
| Phần đệm trở nên đo được | Xem §4.7 |

> **Giới hạn của phép đóng gói:** hạng mục nằm **sát dưới một mốc** hầu như không được đệm gì.
> Ô nào có NN thô sát mốc thì phải soát lại con số dòng dự kiến, đừng tin vào làm tròn. Các ô
> ấy được đánh dấu ⚠ trong bảng.

### 4.3 Nền móng dùng chung

Phần này dựng **một lần**, mọi phân hệ ở §4.4 đứng trên nó. Đây cũng là phần khiến từng phân hệ
sau đó rẻ: mỗi phân hệ mới là **lặp một khuôn đã chạy được**.

| Hạng mục | Dòng dự kiến | NN thô | **Đóng gói** | Nội dung |
|---|---|---|---|---|
| Dịch vụ định danh: phiên, phân quyền, cán bộ, kênh công dân, cổng gRPC | 22.800 | 1,022 | **1,25** | Phiên thu hồi được (sổ đăng ký kiểm mỗi request) · ma trận quyền · định danh công dân đa xã |
| Thư viện dùng chung `core` | 14.000 | 0,628 | **0,75** | Rìa đa xã (xác định xã từ `Host`, **không phân giải được = 404**) · cổng gác quyền · chạy migration · mã hoá bí mật theo xã · xác thực giữa các service |
| Nền màn hình quản trị | 12.900 | 0,578 | **0,75** | Đăng nhập · cấu hình · kiểu sinh từ hợp đồng · bảo vệ tuyến phía máy chủ |
| Nền kênh công dân (Zalo Mini App) | 11.600 | 0,520 | **0,75** | Khung app · xác định xã **không qua tên miền** · kiểm accessibility cho người cao tuổi |
| Khung 6 service nghiệp vụ còn lại | 13.300 | 0,596 | **0,75** | Migration nền · vết kiểm append-only có phân mảnh · rìa · cấu hình · Dockerfile · pipeline |
| Hợp đồng `.proto` + bộ sinh hợp đồng REST từ mã | 7.200 | 0,323 | **0,50** | Nguồn chuẩn cho hợp đồng giữa service; hợp đồng REST **sinh** chứ không viết tay |
| Console quản trị nền tảng | 13.000 | 0,583 | **0,75** | Chỉ **siêu dữ liệu**: xã, tên miền, gói dịch vụ. **Không** có đường đọc dữ liệu nghiệp vụ của xã |
| Dịch vụ nền tảng: xã, danh mục tỉnh thành, webhook kênh ngoài | 3.000 | 0,135 | **0,25** | |
| Hạ tầng triển khai trong mã: manifest, pipeline đóng ảnh | 900 | 0,040 | **0,25** | Phần **trong kho mã**; phần chạm cụm ở §5 |
| | **98.700** | **4,425** | **6,00 NN** | |

### 4.4 Mười sáu phân hệ nghiệp vụ

Dòng dự kiến chia **22.300**, rồi đóng gói lên mốc 0,25.

| Chương | Phân hệ | Dòng dự kiến | NN thô | **Đóng gói** | Vì sao nặng/nhẹ |
|---|---|---|---|---|---|
| 09 | Phản ánh người dân | 8.500 | 0,381 | **0,50** | Nặng nhất: hạn đếm bằng **giờ làm việc** theo lịch từng xã · mọi trạng thái phải thông báo công dân và để lại vết · mã tra cứu không đoán được |
| 06 | Giải ngân | 7.000 | 0,314 | **0,50** | Có tiền, có quyết định hành chính, có số hiệu **không được cấp lại** |
| 05 | Văn bản đơn thư | 6.500 | 0,291 | **0,50** | Số hiệu văn bản đến/đi · luồng chuyển xử lý · lưu trữ |
| 02 | Nhiệm vụ | 6.000 | 0,269 | **0,50** | Phân công, gia hạn, nhiều trạng thái |
| 14 | Cấu hình + 8 danh mục tham chiếu | 14.900 | 0,668 | **0,75** | Cụm danh mục trải trên 5 service + màn cấu hình. Số đo ở §1.1 |
| 01+03 | Tổng quan điều hành + Sổ tay lãnh đạo | 5.500 | 0,247 | **0,25** ⚠ | Cần **API thống kê** riêng. Bảng điều khiển hiện số sai là thứ lãnh đạo đọc rồi báo cáo lên trên |
| 10 | Bản đồ kinh tế số | 5.000 | 0,224 | **0,25** | Toạ độ + ảnh hiện trường ⇒ chạm dữ liệu cá nhân |
| 07 | Thu chi ngân sách | 4.500 | 0,202 | **0,25** | Có tiền nhưng ít trạng thái hơn giải ngân |
| 13 | Báo cáo | 4.000 | 0,179 | **0,25** | Đọc nhiều xã ⇒ mỗi truy vấn phải khai rõ lý do đọc chéo và **tự để lại vết** |
| 08 | Thông báo | 3.800 | 0,170 | **0,25** | Gửi ra ngoài (ZNS) ⇒ adapter khai rõ trường nào đi ra |
| 04 | Biên bản họp | 3.500 | 0,157 | **0,25** | Chủ yếu CRUD + tệp kèm |
| 11 | Nội dung Mini App | 3.500 | 0,157 | **0,25** | Soạn nội dung cho kênh công dân |
| 12 | Danh bạ cán bộ | 3.300 | 0,148 | **0,25** | |
| 00+15 | Tổng quan hệ thống + phụ lục giao diện chung | 2.000 | 0,090 | **0,25** | Thành phần dùng lại, không phải phân hệ riêng |
| | **Tổng** | **78.000** | **3,497** | **5,00 NN** | |

### 4.5 Việc nền vận hành — không thuộc phân hệ nào

Không tính bằng dòng: đây là **chạy, đo, và sửa cái gì vỡ**.

| Việc | NN thô | **Đóng gói** | Vì sao không rẻ như dòng mã |
|---|---|---|---|
| Chạy toàn bộ migration trên PostgreSQL thật + sửa cái gì vỡ | 1,50 | **1,50** | **Không song song hoá được** — phải có người đọc từng lỗi. Và đây là chỗ lộ ra những thứ chỉ vỡ khi có cơ sở dữ liệu thật: định tuyến phân mảnh, partial index, trigger bắn ở mức dòng hay mức câu lệnh |
| Rà lớp lỗi "rào chắn trông như đang canh mà đã chết" | 0,60 | **0,75** | Xem §4.7 |
| Backfill dữ liệu theo từng xã, chạy lại được, ghi tiến độ | 0,45 | **0,50** | Migration chạy **per-commune** là bắt buộc với hệ nhiều xã; chỉ lo DDL là mới nửa đường |
| Đệm cho đường tra cứu phiên công dân + kênh vô hiệu hoá | 0,45 | **0,50** | Nhiều bản sao, một cơ sở dữ liệu ⇒ phải có kênh để một lần thu hồi ở bản sao A với tới bản sao B |
| Đối chiếu danh sách đơn vị hành chính cấp tỉnh với văn bản gốc | 0,30 | **0,50** | **Đọc văn bản pháp luật**, không phải viết mã. Cột này in thẳng ra màn hình công dân nên **dạng viết là nội dung** |
| Bộ sinh chỉ mục "ai sở hữu thực thể nào" từ chính mã nguồn | 0,20 | **0,25** | Chỉ mục viết tay sẽ mục nát; sinh thì không |
| Bảng lỗi hệ thống | 0,10 | **0,25** | Sàn 0,25 |
| | **3,60** | **4,25 NN** | |

### 4.6 Tổng hạng mục coding

| | NN thô | **Đóng gói** |
|---|---|---|
| Nền móng dùng chung (§4.3) | 4,425 | **6,00** |
| 16 phân hệ nghiệp vụ (§4.4) | 3,497 | **5,00** |
| Việc nền vận hành (§4.5) | 3,600 | **4,25** |
| **Tổng coding** | **11,52** | **15,25 NN** |

**Buffer 15% là một dòng RIÊNG ở bảng tổng hợp §9 — không gộp vào 15,25.** Xem §4.7.

### 4.7 HAI PHẦN ĐỆM, HAI VIỆC KHÁC NHAU

| Phần đệm | Giá trị | Chịu rủi ro gì | Vì sao không trùng phần kia |
|---|---|---|---|
| **A — do đóng gói 0,25** *(đã nằm trong 15,25)* | +3,73 NN (**+32%**) | **Nhiễu hạt** của từng hạng mục: con số dòng dự kiến lệch trong phạm vi một hạng mục **đã có tên** | Nằm **bên trong** từng hạng mục |
| **B — buffer coding 15%** *(dòng riêng ở §9)* | **+2,50 NN** | **Hạng mục chưa ai nghĩ tới**: một phân hệ phát sinh khi thi công, một tầng bị bỏ sót khỏi §4.1, một ràng buộc pháp lý mới lộ ra | Nằm **ngoài** mọi hạng mục — nếu nằm trong thì đã có tên rồi |

*(15,25 × 15% = 2,29 → đóng gói lên 2,50.)*

**Tổng đệm thật so với con số thô:**

| | NN | So với thô |
|---|---|---|
| Thô | 11,52 | — |
| Sau đóng gói | 15,25 | ×1,32 |
| Sau buffer 15% | **17,75** | **×1,54** |

**Vì sao chọn 15% chứ không 10%:** phạm vi hồ sơ một cửa chưa xác định (§3.3). Khi còn một phân
hệ chưa có đặc tả thì rủi ro "hạng mục chưa ai nghĩ tới" là rủi ro **đã biết là có**, không phải
rủi ro giả định. Có đủ đặc tả 16 chương và chốt được phạm vi ⇒ hạ về 10%.

**Đệm A neo vào một lớp rủi ro cụ thể, không phải đệm chung:**

> **Lớp rủi ro đắt nhất của mô hình này không phải viết sai mã — là "rào chắn trông như đang
> canh mà đã chết".** Một phép kiểm tự động vẫn chạy, vẫn xanh, và không còn khớp gì nữa.
> Không có gì đỏ vào ngày nó chết.
<!-- NOI-BO -->
> Ví dụ điển hình: mẫu dò truy vấn không phạm vi hoá đòi `.Query(` không hậu tố, trong khi cả
> kho dùng bản `*Context` — rào chắn được nêu tên làm cơ chế chặn chính **không khớp một call
> site nào**, mà cổng kiểm vẫn xanh.
<!-- /NOI-BO -->
> Mỗi ca cùng lớp tốn ~0,2–0,5 NN để truy. **+3,73 NN của đệm A ≈ 7–18 ca**, nên đệm A **đã có
> chủ** và không dùng thay được cho việc của đệm B.

### 4.8 Điều kiện để 15,25 + 2,50 còn đúng

| Điều kiện | Nếu vỡ |
|---|---|
| Prototype và nghiệp vụ **không đổi** | → §3B, không phải §4 |
| Chạy được **nhiều agent song song** với ranh giới ghi rời nhau | Mất song song ⇒ nhân **×2..×3**. Ranh giới ấy là §2 |
| Verification vẫn **tuần tự** ở luồng chính | Quản lý phụ thuộc, sinh mã, sinh chỉ mục, cổng kiểm **không** song song hoá được — đã tính |
| Đặc tả ở dạng **văn bản đọc được** | Chỉ có prototype để xem ⇒ mỗi phân hệ cộng thêm một buổi ở §3 |

---

## 5. HẠNG MỤC 3 — DEPLOYMENT *(tách làm hai)*

### 5.1 — 3A · Sizing và cung cấp hạ tầng *(việc của bên khác)*

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay · **___ NC** ← nhập tay |
| Ai làm | Đội devops / nhà cung cấp / chủ dự án |

**Phân tích — thứ phải có đủ và ĐÚNG trước khi 3B bắt đầu:**

| Cần | Vì sao là điều kiện, không phải chi tiết |
|---|---|
| Cụm Kubernetes + quyền cho hệ thống CI | Không có thì mọi manifest chỉ là văn bản |
| Registry ảnh | Pipeline đóng ảnh dừng ở đây |
| **Tên miền theo từng xã + chứng thư** | Hệ thống phân biệt xã **bằng tên miền**. Thiếu tên miền thì không kiểm được gì có ý nghĩa |
| **Trả lời: cụm có split-horizon DNS / chặn egress không** | Tiến trình web gọi API bằng **tên miền công khai**. Nếu cụm chặn thì **mọi yêu cầu trả 500** — xem rủi ro có giá ở §5.2 |
| PostgreSQL có phân mảnh + khoá gốc nằm **ngoài** cơ sở dữ liệu | Mã hoá bí mật theo xã đòi khoá gốc không nằm cùng chỗ với dữ liệu |

### 5.2 — 3B · AI dựng deployment *(sau khi hạ tầng đã đủ và đúng)*

| | NN |
|---|---|
| **Bước 1 — Validate** | **1,00** |
| **Bước 2 — Thực thi** | **2,00** |
| **Tổng** | **3,00 NN** |

**Bước 1 — Validate.** Không phải thủ tục hành chính; là bước **fail closed** cho hạ tầng:

- Đối chiếu tài nguyên nhận được với manifest và cấu hình từng môi trường
- Kiểm tên miền phân giải đúng xã, và **tên miền không phân giải được thì trả 404**
- Kiểm bí mật tới được từ kho bí mật của cụm, **không** nằm trong ảnh
- Kiểm cờ nguy hiểm (bỏ qua xác thực, chế độ demo) **tắt** trước khi chạy thật
- Kiểm cách ly hai xã trên hạ tầng thật: dựng 2 xã, thử đọc chéo

**Bước 2 — Thực thi:** manifest cho các đơn vị còn lại · chạy pipeline trên hệ thống CI thật ·
ingress + tên miền từng xã + chứng thư · logs tập trung và cảnh báo.

**Rủi ro có giá, nêu tên rõ:** nếu cụm chặn egress hoặc có split-horizon DNS thì phát sinh
**+1,00 NN** — phải thêm biến gốc API nội bộ, thêm tệp mẫu cấu hình, sửa đường gọi phía máy
chủ. **Bước Validate tồn tại chính để phát hiện việc này trước khi tiêu 2,00 NN của bước Thực
thi.** Đề nghị ghi hợp đồng: **3,00 NN + 1,00 NN có điều kiện**, điều kiện nêu tên.

---

## 6. HẠNG MỤC 4 — ZALO MINI APP *(tách làm ba)*

**Mobile app native: KHÔNG có trong phạm vi.** Kênh công dân chỉ đi qua Zalo Mini App. Nếu
khách muốn app native, đó là hạng mục mới chưa estimate.

### 6.1 — 4A · Tài liệu cho yêu cầu xác thực *(bên thứ ba)*

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay · **___ NC** ← nhập tay |
| Ai làm | Chủ dự án + Zalo |

| Hạng mục | Ai giữ | Ghi chú |
|---|---|---|
| Hồ sơ doanh nghiệp, giấy phép | Chủ dự án | Bắt buộc với app gắn dịch vụ công |
| Quyền OA, xác thực số điện thoại OA | Chủ dự án | Cần **hai OA riêng**: OA xác thực tách khỏi OA thông báo |
| Logo, icon, ảnh chụp màn hình, mô tả store | Chủ dự án cấp nội dung | |
| Chính sách quyền riêng tư, điều khoản | Chủ dự án | Chạm dữ liệu cá nhân ⇒ Nghị định 13/2023 |
| **Ba câu chỉ Zalo trả lời được** | Zalo | Ràng buộc 1 Mini App ↔ 1 OA · app hồ sơ doanh nghiệp sau này gắn dịch vụ công có phải xác thực lại · tham số deep link có tới app khi app chạy nền. **Tài liệu công khai không trả lời dứt khoát cả ba** |

### 6.2 — 4B · Coding các chức năng + đẩy lên Mini App

Phần **nền** kênh công dân đã tính ở §4.3; đây là phần nghiệp vụ và phần nộp.

| Việc | NN thô | **Đóng gói** |
|---|---|---|
| Màn hình nghiệp vụ cho công dân | 1,20 | **1,25** |
| Đăng nhập công dân (một chạm qua OA xác thực, QR ghép phiên) | 0,50 | **0,50** |
| Đẩy lên Mini App: cấu hình app, thư mục build, gói nộp | 0,40 | **0,50** |
| **Tổng** | **2,10** | **2,25 NN** |

**Một rủi ro mã nằm ở bước đẩy lên:** tên khoá trong tệp cấu hình app và thư mục build mặc định
của công cụ dựng **không trùng** với thứ công cụ đóng gói Mini App chờ đợi. **Sai khoá là hồ sơ
bị trả về** — mất một vòng duyệt, tức tiêu NC ở §6.3. Phải đối chiếu Developer Console **trước**
khi nộp.

### 6.3 — 4C · Chờ phản hồi và xử lý yêu cầu sau in-review

| | |
|---|---|
| **NN xử lý mỗi vòng** | **0,50 – 1,00** |
| **Số vòng dự kiến** | **___ vòng** ← nhập tay |
| **NC mỗi vòng** | **5 – 20** |

Số vòng phụ thuộc Zalo và phụ thuộc ba câu ở §6.1. Nếu một trong ba sai thì phát sinh **sửa
kiến trúc**, không phải sửa giao diện — nặng nhất là ràng buộc 1 Mini App ↔ 1 OA, vì mô hình
hai OA đứng trên đúng giả định ấy.

Đề nghị ghi hợp đồng: **1 vòng nằm trong giá, từ vòng 2 tính thêm** — số vòng không nằm trong
tay đội thi công.

---

## 7. HẠNG MỤC 5 — TESTING *(tách làm hai người làm)*

### 7.1 — 5A · Người làm tự test

| | |
|---|---|
| **Giá trị** | **1,00 – 3,00 NN** |

**Vì sao ít:** chức năng **đã được review ngay lúc đẩy lên**. Mỗi lát cắt dọc ở §4.1 mang theo
test ở cả 6 tầng, và cổng kiểm chạy trước mỗi lần kết phiên — test chiếm ~34% khối lượng mã.
Phần này chỉ còn là **chạy thật những thứ cổng kiểm không phủ**:

| Nội dung | Vì sao phải làm tay |
|---|---|
| Chạy toàn bộ bộ test cần cơ sở dữ liệu thật | Thiếu biến DSN thì bộ ấy **tự bỏ qua và gói vẫn báo `ok`** — dạng âm tính giả nguy hiểm nhất |
| Chạy linter đầy đủ lần đầu trên máy có cài đủ | Một mục lint "báo rồi cho qua" không phải một cổng |
| Đưa **mọi** app frontend vào cổng kiểm | Một app thiếu `node_modules` bị bỏ qua mà cổng vẫn trả rc=0 |
| Cách ly hai xã trên dữ liệu thật | Không hook nào kiểm được quan hệ giữa hai xã thật |
| **Đột biến** cho từng hook | Gỡ thứ mỗi hook đáng lẽ phải chặn rồi xem nó có chặn không. Đây là phép kiểm **duy nhất** đáng tin cho một rào chắn |
| Kiểm tay theo đặc tả từng phân hệ | Cổng nói mã chạy được; nó **không** nói kết luận nghiệp vụ đúng |

*(Dòng đầu đã tính trong §4.5 — không cộng lại.)*

### 7.2 — 5B · BA hoặc tester tham gia

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay |
| Ai làm | BA / tester của dự án — **không phải người định hướng** |

| Nhóm | Nội dung |
|---|---|
| Theo phân hệ | Chạy đúng luồng trong từng chương đặc tả · đối chiếu nhãn và thuật ngữ hành chính · kiểm số hiệu văn bản và con số trên báo cáo |
| Cách ly | Đăng nhập xã A, thử thấy dữ liệu xã B · công dân thử xem phiếu của người khác · cán bộ sai vai trò gọi tuyến ngoài quyền |
| Cam kết với công dân | Hạn xử lý đếm đúng **giờ làm việc** của xã (không phải giờ đồng hồ) · mỗi lần đổi trạng thái công dân **có nhận được thông báo** · đóng phiếu có kết quả công dân đọc được |
| Dữ liệu cá nhân | Số điện thoại và số định danh **bị che** ở mọi màn hình và mọi bản xuất, trừ khi có quyền xem đầy đủ — và lần xem đầy đủ ấy **có nằm trong vết kiểm** |
| Kênh công dân | Trên máy thật, tài khoản Zalo thật · cỡ chữ và vùng bấm cho người cao tuổi · thông báo tới đúng người |
| Vết kiểm | Mỗi thao tác ghi có một dòng: ai · làm gì · lúc nào · từ IP nào · **ở xã nào** |

---

## 8. HẠNG MỤC 6 — BÀN GIAO

| | |
|---|---|
| **Giá trị** | **___ NN** ← nhập tay · dự kiến **1,00 – 3,00 NN** |

Chỉ **note chức năng** — phần kiểm chất lượng đã nằm ở §7.2 với tester.

| Nhóm | Nội dung |
|---|---|
| Theo phân hệ | Làm được gì · ai được làm (khoá quyền nào) · cấu hình theo xã ở đâu |
| Vận hành | Onboard một xã mới (đây là hệ thống **nhiều xã**) · xoay khoá ký và khoá phiên · nơi đọc vết kiểm |
| Giới hạn đã biết | Mọi thứ cố ý chưa làm, kèm lý do và điều kiện gỡ |
| Bàn giao bộ não AI | Chỉ mục tri thức · thủ tục cập nhật tiến độ · thủ tục bàn giao phiên · vì sao không được sửa tay tầng sinh |

---

## 9. TỔNG HỢP

Mọi giá trị NN đã đóng gói lên **bội số 0,25** (§4.2).

| # | Hạng mục | Ai làm | NN | Nhập tay | NC |
|---|---|---|---|---|---|
| 0 | Bộ não AI | AI + người định hướng | **1,50** | — | — |
| 1 | Nghiệp vụ · prototype · họp chốt | người định hướng + khách | `___ × 0,50` | **số buổi `___`** | 9–35 |
| 1B | Buffer thay đổi nghiệp vụ/prototype | AI | — | **`___` NN** | — |
| 2 | **Coding** | AI | **15,25** | — | — |
| 2B | **Buffer coding 15%** | AI | **2,50** | — | — |
| 3A | Sizing + cung cấp hạ tầng | devops / chủ dự án | — | **`___` NN · `___` NC** | — |
| 3B | Deployment: validate + thực thi | AI | **3,00** | — | — |
| 3B' | *Phát sinh có điều kiện: cụm chặn egress / split-horizon DNS* | AI | *+1,00* | — | — |
| 4A | Zalo: tài liệu xác thực | chủ dự án + Zalo | — | **`___` NN · `___` NC** | — |
| 4B | Zalo: coding + đẩy lên | AI | **2,25** | — | — |
| 4C | Zalo: chờ phản hồi + xử lý sau review | AI + Zalo | `___ ×` (0,50–1,00) | **số vòng `___`** | 5–20/vòng |
| 5A | Testing — người làm tự test | AI + người định hướng | **1,00 – 3,00** | — | — |
| 5B | Testing — BA / tester | BA / tester | — | **`___` NN** | — |
| 6 | Bàn giao — note chức năng | người định hướng | — | **`___` NN** *(dự kiến 1,00–3,00)* | — |
| | **Phần AI chịu trách nhiệm, đã chốt số** | | **25,50 – 27,50 NN** | | |

**Cộng thêm các ô nhập tay** để ra tổng hợp đồng. Nếu điền theo dự kiến ở từng mục — 6 buổi
= 3,00 · buffer nghiệp vụ 1,00 · 3A 0 (bên khác) · 4A 0 (bên khác) · 1 vòng Zalo 1,00 ·
5B 3,00 · bàn giao 2,00 — thì tổng **≈ 35,50 – 37,50 NN**, lịch **≈ 60 – 95 NL** tuỳ tốc độ
khách chốt sáu nhóm câu hỏi ở §3.2.

**Ba phần đệm trong bản này, đừng lẫn:**

| Đệm | Ở đâu | Chịu rủi ro gì |
|---|---|---|
| Đóng gói 0,25 | nằm **trong** từng hạng mục coding (§4.7-A) | Nhiễu hạt của hạng mục **đã có tên** |
| Buffer coding 15% | **dòng riêng** 2B (§4.7-B) | Hạng mục coding **chưa ai nghĩ tới** |
| Buffer nghiệp vụ | **dòng riêng** 1B (§3B), nhập tay | **Khách đổi ý** |

Ba rủi ro khác nhau, ba chủ khác nhau — gộp lại là không biết đang đệm cho cái gì.

### 9.1 Quy đổi sang mô hình cũ, nếu khách cần so sánh

Khối lượng dự kiến của hệ thống hoàn chỉnh: **~177.000 dòng** kể cả test (34%) và tài liệu.
Mốc thường của đội viết tay là 50–150 dòng sản phẩm/ngày-người kể cả test và tài liệu ⇒
**1.200–3.500 ngày-người** cho cùng khối lượng.

Con số ấy **đúng về khối lượng và sai về bản chất**. Mô hình này đổi chi phí viết mã thành **chi
phí kiểm soát**: thứ tính tiền không phải mã, mà là bộ não cưỡng chế, tập ADR, và **một người
chịu trách nhiệm về từng quyết định**. Cắt phần kiểm soát thì vẫn ra lượng mã ấy, nhưng không ai
biết nó đúng hay sai — với hệ thống hành chính nhà nước, đó không phải một lựa chọn.

---

## 10. RỦI RO ĐÃ BIẾT, CÓ GIÁ

| Rủi ro | Xác suất | Giá | Giảm bằng cách |
|---|---|---|---|
| Khách chốt muộn nhóm câu hỏi 4 (mã cán bộ, trường điện thoại) | Trung bình | `ALTER TABLE` trên **bản ghi lưu trữ** — thủ tục hành chính, không phải lệnh | Đưa lên buổi họp đầu (§3.2) |
| Cụm chặn egress / split-horizon DNS | Chưa xác nhận được trước khi có cụm | **+1,00 NN**, và nếu phát hiện muộn thì mọi yêu cầu web trả 500 | Hỏi devops **trước** khi nhận resources; bước Validate §5.2 |
| Zalo trả hồ sơ vì sai khoá cấu hình app | Trung bình | 1 vòng = 0,50–1,00 NN + 5–20 NC | Đối chiếu Developer Console trước khi nộp |
| Một trong ba giả định về Zalo sai | Chưa xác nhận | Sửa **kiến trúc**, không sửa giao diện | Hỏi cùng lượt nộp (§6.1) |
| Lớp "rào chắn trông như đang canh mà đã chết" | **Cao**<!-- NOI-BO --> — xuất hiện ~7 lần trên chu kỳ hiệu chuẩn<!-- /NOI-BO --> | 0,2–0,5 NN mỗi ca | Đã tính trong đệm A (§4.7) |
| Mất khả năng chạy nhiều agent song song | Thấp | **×2..×3 toàn bộ §4** | Giữ ranh giới ghi của các agent (§2) |
| Phạm vi hồ sơ một cửa chưa xác định | Cao | +0,75 NN nếu trong phạm vi | Hỏi trước khi ký (§3.3) |
| Sáp nhập/chia tách xã giữa dự án | Thấp, tác động lớn | Nếu định danh xã mang nghĩa thì phải sửa khoá ngoại trên toàn bộ dữ liệu lịch sử | **Phòng từ đầu**: định danh xã là ULID vô nghĩa (§3B.3) |

---

## 11. BẢN NÀY KHÔNG BAO GỒM

| Không bao gồm | Vì sao |
|---|---|
| App mobile native | Kênh công dân đi qua Zalo Mini App. Nếu cần, là hạng mục mới |
| Chi phí hạ tầng (cụm, registry, chứng thư, ZNS) | Không phải công thi công — §5.1 |
| Chi phí token AI | Tính riêng theo lượt dùng thật; nhỏ hơn hẳn chi phí người định hướng |
| Vận hành sau bàn giao (SLA, trực sự cố) | Hợp đồng riêng |
| Hồ sơ một cửa | Chưa có đặc tả — §3.3 |
| Di trú dữ liệu từ hệ thống cũ | Khách chưa nêu |
| Đào tạo cán bộ từng xã | Khác với bàn giao ở §8 |

---

## 12. CÁCH DÙNG BẢN NÀY CHO DỰ ÁN TƯƠNG TỰ

| Phải đo lại | Dùng lại được |
|---|---|
| Dòng dự kiến từng phân hệ (§4.4) — theo đặc tả của dự án ấy | **22.300 dòng/NN** (§1) |
| Quy mô nền móng (§4.3) — bao nhiêu service, bao nhiêu app, có kênh công dân không | **Quy ước đóng gói 0,25 NN, làm tròn lên** (§4.2) |
| Các nhóm câu hỏi phải chốt và số buổi (§3.2) | Bảng 6 tầng của một lát cắt dọc (§4.1) |
| Chọn 10% hay 15% cho buffer coding (§4.7) | **Ba phần đệm và ranh giới giữa chúng** (§4.7 + §3B) |
| Các ô `___` — đều phụ thuộc dự án hoặc bên khác | Cấu trúc 6 hạng mục + ba loại thời gian NN/NC/NL + bảng rủi ro |
