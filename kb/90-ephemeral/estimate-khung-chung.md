---
id: estimate-khung-chung
tier: T5
source: CURATED
owner: architecture
derived_from_commit: ee6de83
expires: 2027-03-20
owns_facts:
  - "phương pháp estimate cho mô hình AI code 100% + 1 người định hướng: đơn vị, hệ số, quy ước đóng gói, khung hạng mục"
---

# Khung estimate — mô hình AI code 100% + 1 người định hướng

**Đây là KHUNG, không phải một bản estimate.** Điền vào rồi ra một bản cho dự án cụ thể.

**Bản mẫu đã điền đầy đủ:** `kb/90-ephemeral/estimate-vigov.md` (nội bộ) và
`estimate-vigov-khach.md` (bản gửi ra ngoài, sinh từ bản nội bộ). Đọc kèm khi không rõ một ô
nên viết gì.

> **Chỗ này sẽ trôi nếu không ai canh.** Khung này sở hữu **phương pháp**; bản của từng dự án
> sở hữu **con số của dự án ấy**. Bản dự án buộc phải nhắc lại phương pháp vì nó phải đứng một
> mình khi gửi ra ngoài — nên khi sửa phương pháp, sửa ở đây trước, rồi mới lan sang các bản
> dự án. Sửa ngược chiều là cách hai tệp bắt đầu mâu thuẫn.

**Hết hạn 20/03/2027, có lý do:** hệ số ở §2 đo trên năng lực mô hình tại thời điểm đo. Năng
lực đổi thì hệ số đổi. Quá hạn mà chưa đo lại thì **đừng dùng con số, chỉ dùng khung**.

---

## 0. DÙNG KHUNG NÀY THẾ NÀO — năm bước

| # | Bước | Ra cái gì |
|---|---|---|
| 1 | Đếm **dòng đặc tả** từng phân hệ của dự án | Đầu vào cho §5-H2 |
| 2 | Liệt kê các **câu phải chốt với khách**, gom theo **người ra quyết định** | Số buổi cho §5-H1 |
| 3 | Xác định **ràng buộc hệ thống** (§7) | Dòng dự kiến mỗi phân hệ cao hay thấp |
| 4 | Điền bảng §5, đóng gói mọi số theo §3 | Bản estimate |
| 5 | Chạy checklist §9 trước khi chốt giá | Thứ còn thiếu, thứ phải hỏi |

**Không bỏ bước 2.** Trên dự án tham chiếu, đường găng không phải coding — là tốc độ khách
chốt. Một bản estimate chỉ có phần coding là bản sẽ trễ mà không ai giải thích được vì sao.

---

## 1. ĐƠN VỊ ĐO

| | |
|---|---|
| **NN** — ngày-người | Công của **người định hướng**. 1 ngày = 8 giờ làm việc thật. **Thứ tính tiền** |
| **NC** — ngày chờ | Chờ khách chốt, chờ hạ tầng, chờ bên thứ ba duyệt. **Không tiêu công, nhưng tiêu lịch** |
| **NL** — ngày lịch | NN + NC, chồng lấn được một phần |

**Không dùng man-day lập trình viên.** Mô hình này không có lập trình viên, và quy đổi sang
đơn vị ấy làm người đọc tưởng thêm người thì nhanh hơn. Sản lượng tỉ lệ với **số quyết định
người định hướng ra được trong một ngày**, không tỉ lệ với số người.

**Luôn tách NN và NC trong báo giá.** Gộp lại thì chậm vì chờ cấp máy sẽ bị đọc thành chậm vì
thi công, và đó là tranh cãi không có bằng chứng để gỡ.

---

## 2. HỆ SỐ HIỆU CHUẨN

| | |
|---|---|
| **Hệ số tham chiếu** | **22.300 dòng mỗi NN** |
| Đã bao gồm | Test (~34% khối lượng), tài liệu, ADR |
| Đo trên | Hệ nhiều tenant, dữ liệu cá nhân có luật điều chỉnh, bản ghi có giá trị pháp lý, có kênh bên thứ ba |

### 2.1 Điều kiện để hệ số còn đúng

| Điều kiện | Thiếu thì sao |
|---|---|
| Đặc tả ở dạng **văn bản đọc được** — không phải chỉ có prototype để xem | Mỗi phân hệ cộng thêm một buổi ở §5-H1 |
| **Một** người quyết được nghiệp vụ, không phải xin ý kiến vòng hai | Nhân đôi số buổi, và NC tăng mạnh |
| Chạy được **nhiều agent song song** với ranh giới ghi rời nhau | Mất song song ⇒ **nhân ×2..×3** toàn bộ H2 |
| Có tầng cưỡng chế tự kiểm (§5-H0) | Không có thì tốc độ ấy vẫn đạt được, nhưng **không ai biết kết quả đúng hay sai** |

### 2.2 Tự đo lại cho tổ chức mình

Đừng tin hệ số của người khác quá một chu kỳ. Cách đo:

```
hệ số = tổng dòng THÊM (git log --shortstat, kể cả test và tài liệu)
        ──────────────────────────────────────────────────────────
        tổng NN của người định hướng trong cùng khoảng
```

Kiểm chứng lại bằng **một lát cắt dọc hoàn chỉnh** (một phân hệ từ migration tới màn hình):
lấy tổng dòng của đúng các commit ấy chia hệ số, xem có ra đúng công đã bỏ không.

> **Sai lầm hay gặp khi hiệu chuẩn — đã mắc một lần và tốn hơn 10 lần sai số:** lấy *thời gian
> trôi* của một ngày rồi quy cả ngày ấy cho **một** phân hệ, trong khi ngày đó chạy song song
> 6–8 việc. Phép chia đúng là **tổng dòng chia tổng NN**, không bao giờ là "hôm nay tôi làm
> phân hệ X nên phân hệ X tốn một ngày".

---

## 3. QUY ƯỚC ĐÓNG GÓI — mốc 0,25 NN, luôn làm tròn LÊN

Mọi hạng mục đóng gói về **bội số 0,25 NN**, **luôn làm tròn lên**, áp cả trên 1:

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
| Không có hạng mục "gần bằng 0" | Việc nhỏ nhất vẫn tốn đọc đặc tả, ra quyết định, một lượt qua cổng kiểm. **0,25 là sàn thật** |
| Phần đệm **đo được** | Trên dự án tham chiếu, đóng gói thêm **+32%** — gần đúng bằng một hệ số ×1,5 áp lên toàn bộ |

**Hệ quả phải nhớ: đã đóng gói thì ĐỪNG nhân thêm hệ số an toàn.** Hai thứ ấy làm cùng một
việc; ghi cả hai là đệm hai lần và không ai giải thích được con số cuối.

> **Giới hạn:** hạng mục nằm **sát dưới một mốc** hầu như không được đệm gì (0,247 → 0,25 là
> đệm 1%). Đánh dấu ⚠ những ô ấy và soát lại con số dòng dự kiến của chúng, đừng tin vào làm
> tròn.

---

## 4. MỘT LÁT CẮT DỌC GỒM ĐÚNG NHỮNG TẦNG NÀO

Dùng bảng này để **ước dòng** cho từng phân hệ. Một AI làm cả dọc — không tách backend/frontend.

| Tầng | Nội dung | Dòng điển hình |
|---|---|---|
| 1 | Migration + ràng buộc bất biến + phân mảnh theo tenant | 400–700 |
| 2 | `domain/` — trạng thái, bất biến, phép suy | 300–800 |
| 3 | `store/` — kho có phạm vi tenant + test kho | 800–1.500 |
| 4 | `app/` — luồng nghiệp vụ, vết kiểm **trong cùng giao dịch** | 400–1.000 |
| 5 | `http/` — tuyến, khai quyền tường minh, test 401/403/403-sai-tenant/200 | 600–1.200 |
| 6 | Màn hình quản trị — kiểu sinh từ hợp đồng, test kết xuất | 800–1.800 |
| | **Tổng một phân hệ** | **3.300–7.000** |

Dải trên ứng với dự án có **đủ bốn ràng buộc** ở §7. Thiếu ràng buộc nào thì trừ theo §7.

---

## 5. KHUNG SÁU HẠNG MỤC — BIỂU MẪU

Ô `___` là ô **nhập tay**.

| # | Hạng mục | Ai làm | Cách tính | NN |
|---|---|---|---|---|
| **H0** | **Dựng bộ não AI** | AI + người định hướng | Dựng mới **1,50** · dự án sau dùng lại ~70% ⇒ **0,50** | `___` |
| **H1** | **Nghiệp vụ · prototype · họp chốt** | người định hướng + khách | `___ buổi` × **0,50** | `___` |
| **H1B** | **Buffer thay đổi nghiệp vụ/prototype** | AI | Công thức §6.3 | `___` |
| **H2** | **Coding** | AI | H2a + H2b + H2c, đóng gói theo §3 | `___` |
| **H2B** | **Buffer coding 10–15%** | AI | **Dòng riêng**, không gộp vào H2 | `___` |
| **H3A** | **Sizing + cung cấp hạ tầng** | devops / chủ dự án | Không phải công của mình | `___` NN · `___` NC |
| **H3B** | **Deployment: validate + thực thi** | AI | **1,00 + 2,00 = 3,00** *(+1,00 có điều kiện)* | `___` |
| **H4A** | **Kênh bên thứ ba: hồ sơ xác thực** | chủ dự án + bên thứ ba | Không phải công của mình | `___` NN · `___` NC |
| **H4B** | **Kênh bên thứ ba: coding + đẩy lên** | AI | Theo §4 | `___` |
| **H4C** | **Kênh bên thứ ba: chờ duyệt + xử lý sau review** | AI + bên thứ ba | `___ vòng` × (0,50–1,00) | `___` · NC `___` |
| **H5A** | **Testing — người làm tự test** | AI + người định hướng | **1,00 – 3,00** | `___` |
| **H5B** | **Testing — BA / tester** | BA / tester | Không phải công của mình | `___` |
| **H6** | **Bàn giao — note chức năng** | người định hướng | Dự kiến **1,00 – 3,00** | `___` |

### H0 — Bộ não AI

Hạng mục **không nhìn thấy trong sản phẩm** và quyết định mọi hạng mục sau.

| Thành phần | Vai trò |
|---|---|
| Luật cưỡng chế, mỗi luật **nêu tên** cơ chế thực thi nó | Điều AI không được phép làm. Luật không có cơ chế thực thi là luật sẽ trôi |
| Hook chặn **trước khi ghi** | Review sau khi ghi thì thứ sai đã nằm trong kho |
| Agent có **ranh giới ghi rời nhau** | **Đây là thứ cho phép chạy song song** — và song song là thứ làm H2 rẻ |
| Tầng tri thức tiết lộ dần, có chỉ mục nạp mọi phiên | AI không có trí nhớ giữa các phiên |
| **Tự kiểm: mỗi hook có ca phải chặn + ca phải cho qua** | Xem cảnh báo §8 |

> Cắt H0 để giảm giá thì **mất luôn tốc độ của H2**, không chỉ mất phần kiểm soát.

### H1 — Nghiệp vụ, prototype, họp chốt

Đơn giá **0,50 NN/buổi** (chuẩn bị phương án + họp + ghi quyết định thành ADR).

**Gom câu hỏi theo NGƯỜI RA QUYẾT ĐỊNH, không theo phân hệ.** Hai câu cùng một người quyết thì
vào một buổi; hai câu cùng một phân hệ mà khác người quyết thì là hai buổi.

Với mỗi nhóm ghi: câu hỏi · cụm việc nó mở khoá · **NC chờ khách**.

**Đánh dấu riêng câu ĐẮT BẤT ĐỐI XỨNG** — câu mà trả lời muộn đắt hơn hẳn trả lời sớm:

| Dạng | Vì sao đắt bất đối xứng |
|---|---|
| Câu **chặn schema** | Trả lời sau khi có dữ liệu thật ⇒ đổi cấu trúc trên dữ liệu đang chạy |
| Câu **một chiều** | Ví dụ: thêm lịch sử phiên bản về sau chỉ ghi được từ lúc thêm; những thay đổi đã xảy ra là mất vĩnh viễn |
| Câu **định hình khoá chính / định danh** | Sai thì phải sửa khoá ngoại trên toàn bộ dữ liệu lịch sử |

### H2 — Coding, chia ba phần

| | Phần | Cách ước |
|---|---|---|
| **H2a** | **Nền móng dùng chung** | Dựng một lần: thư viện chung, rìa xác định tenant, cổng gác quyền, hợp đồng + bộ sinh, khung từng service, nền từng app. Ước bằng §4 cho từng khối |
| **H2b** | **Từng phân hệ nghiệp vụ** | `dòng đặc tả → dòng dự kiến (§4) → ÷ hệ số (§2) → đóng gói (§3)` |
| **H2c** | **Việc nền vận hành** | **Không tính bằng dòng** — đây là chạy, đo, sửa cái gì vỡ |

**H2c hay bị bỏ sót nhất.** Danh mục tối thiểu:

| Việc | Vì sao không rẻ như dòng mã |
|---|---|
| Chạy toàn bộ migration trên cơ sở dữ liệu **thật** | **Không song song hoá được.** Là chỗ lộ ra thứ chỉ vỡ khi có CSDL thật: định tuyến phân mảnh, partial index, trigger bắn ở mức dòng hay mức câu lệnh |
| Rà lớp "rào chắn trông như đang canh mà đã chết" | §8 |
| Backfill theo từng tenant, chạy lại được, ghi tiến độ | Migration per-tenant là bắt buộc với hệ nhiều tenant |
| Đối chiếu dữ liệu danh mục với **văn bản gốc** | Đọc văn bản, không phải viết mã |
| Bộ sinh chỉ mục từ chính mã nguồn | Chỉ mục viết tay sẽ mục nát |

### H3 — Deployment, tách làm hai

**H3A không phải công của mình.** Liệt kê thứ phải có **đủ và ĐÚNG** trước khi H3B bắt đầu, và
ghi rõ ai giữ: cụm · registry ảnh · tên miền + chứng thư · **câu hỏi về DNS nội bộ / chặn
egress** · cơ sở dữ liệu đúng cấu hình · khoá gốc nằm ngoài CSDL.

**H3B = Validate (1,00) + Thực thi (2,00).** Validate không phải thủ tục hành chính — nó là
bước **fail closed** cho hạ tầng, và nó tồn tại để phát hiện "hạ tầng không đúng yêu cầu"
**trước khi** tiêu 2,00 NN của bước Thực thi:

- Đối chiếu tài nguyên nhận được với manifest từng môi trường
- Tên miền phân giải đúng tenant, và **không phân giải được thì từ chối**, không đoán
- Bí mật tới từ kho bí mật của cụm, **không** nằm trong ảnh
- Cờ nguy hiểm (bỏ qua xác thực, chế độ demo) **tắt**
- Cách ly hai tenant **trên hạ tầng thật**

### H4 — Kênh bên thứ ba (app store, Zalo Mini App, cổng thanh toán…), tách làm ba

| | Đặc tính quyết định cách estimate |
|---|---|
| **H4A** hồ sơ xác thực | Của chủ dự án và của bên thứ ba. **Chỉ phân tích, giá nhập tay** |
| **H4B** coding + đẩy lên | Công của mình, ước theo §4 |
| **H4C** chờ duyệt + xử lý sau review | **Số vòng không đoán được** ⇒ `___ vòng × (0,50–1,00)`, NC 5–20 mỗi vòng |

**Đề nghị điều khoản: 1 vòng nằm trong giá, từ vòng 2 tính thêm.** Số vòng không nằm trong tay
đội thi công, nên cam kết một phía về nó là cam kết sẽ vỡ.

**Liệt kê riêng những giả định chỉ bên thứ ba trả lời được.** Nếu một giả định sai thì phát
sinh **sửa kiến trúc**, không phải sửa giao diện — đó là rủi ro phải nêu tên trước khi ký.

### H5 — Testing, tách theo NGƯỜI LÀM

| | |
|---|---|
| **H5A — người làm tự test: 1,00–3,00 NN** | Ít, vì chức năng **đã được review ngay lúc đẩy lên** và test đã đi kèm từng lát cắt. Chỉ còn chạy thật **những thứ cổng kiểm không phủ** |
| **H5B — BA/tester: nhập tay** | Chỉ **liệt kê nội dung**, không kèm số — công của họ, không phải của mình |

Danh mục H5A tối thiểu: bộ test cần CSDL thật · linter đầy đủ trên máy cài đủ · **mọi** app
frontend đều vào cổng · cách ly hai tenant trên dữ liệu thật · **đột biến cho từng rào chắn** ·
kiểm tay theo đặc tả.

### H6 — Bàn giao

Chỉ **note chức năng** — phần kiểm chất lượng đã ở H5B. Nội dung: từng phân hệ làm được gì và
ai được làm · thủ tục onboard một tenant mới · xoay khoá · **giới hạn đã biết kèm điều kiện
gỡ** · cách bàn giao chính bộ não AI.

---

## 6. BA PHẦN ĐỆM — ba rủi ro, ba chủ

Gộp lại là không biết đang đệm cho cái gì.

| Đệm | Ở đâu | Chịu rủi ro gì |
|---|---|---|
| **A — đóng gói 0,25** | Nằm **trong** từng hạng mục H2 | **Nhiễu hạt** của hạng mục **đã có tên**: con số dòng dự kiến lệch |
| **B — buffer coding 10–15%** | **Dòng riêng** H2B | Hạng mục **chưa ai nghĩ tới**: phân hệ phát sinh khi thi công, một tầng bị bỏ sót, ràng buộc pháp lý mới lộ ra |
| **C — buffer nghiệp vụ** | **Dòng riêng** H1B, nhập tay | **Khách đổi ý** |

### 6.1 Chọn 10% hay 15% cho đệm B

| Chọn | Khi nào |
|---|---|
| **10%** | Đặc tả đủ, mọi phân hệ đều có văn bản, phạm vi đã chốt |
| **15%** | **Còn phân hệ chưa có đặc tả.** Khi đó "hạng mục chưa ai nghĩ tới" là rủi ro **đã biết là có**, không phải giả định |

### 6.2 In tổng đệm ra, đừng giấu

Bảng ba dòng, đưa vào cả bản gửi khách:

```
Thô                    x,xx
Sau đóng gói           x,xx   (×1,3 điển hình)
Sau buffer             x,xx   (×1,5 điển hình)
```

Một bản estimate giấu phần đệm là bản **không giải thích được khi phát sinh** — và lúc ấy mất
nhiều hơn phần trăm đã giấu.

### 6.3 Công thức cho đệm C

> **Buffer ≈ Σ (dòng của lát cắt bị ảnh hưởng ÷ hệ số) × hệ số lan**

| Hệ số lan | Khi nào |
|---|---|
| **×0,3** | Đổi nhãn, thứ tự, văn bản hiển thị |
| **×0,5** | Đổi một trường **trên dây** — hợp đồng sinh lại, client phải theo |
| **×1,0** | Đổi **chủ sở hữu** một thực thể giữa các service |
| **×1,5** | Đổi thứ đã có **dữ liệu lưu trữ** — không còn là sửa mã |

**Bốn cơ chế làm đệm C rẻ đi, phải dựng từ đầu chứ không vá sau:** hợp đồng **sinh** từ khai
báo trong mã · định danh tenant **vô nghĩa** (ULID), không phải mã nghiệp vụ · vòng đời và
ngưỡng là **cấu hình theo tenant** · sổ tiến độ + ADR để biết **chính xác** lát cắt nào bị ảnh
hưởng thay vì đoán.

---

## 7. RÀNG BUỘC HỆ THỐNG — điều chỉnh dòng dự kiến

Dải 3.300–7.000 dòng/phân hệ ở §4 đo trên dự án có **đủ bốn** ràng buộc dưới. Thiếu ràng buộc
nào thì **trừ dòng**, đừng đổi hệ số — ràng buộc hiện ra thành **mã thật**, nên trừ ở đúng chỗ
nó sinh ra mã.

| Ràng buộc | Nó sinh thêm mã ở đâu | Không có thì |
|---|---|---|
| **Nhiều tenant trên hạ tầng chung** | Mọi truy vấn phải phạm vi hoá · khoá duy nhất phải ghép tenant · rìa xác định tenant · test 403-sai-tenant cho **mọi** tuyến | **−15%** |
| **Bản ghi có giá trị pháp lý** | Xoá mềm khắp nơi · vết kiểm trong cùng giao dịch · append-only cưỡng chế · số hiệu không cấp lại | **−15%** |
| **Dữ liệu cá nhân có luật điều chỉnh** | Che ở API và ở mọi bản xuất · vết kiểm khi xem đầy đủ · adapter khai rõ trường nào đi ra ngoài | **−10%** |
| **Cam kết thời hạn với người dùng cuối** | Hạn tính theo **lịch làm việc** cấu hình được · thông báo mọi lần đổi trạng thái · trạng thái quá hạn phải **suy ra**, không lưu | **−10%** |

Không ràng buộc nào ⇒ dải còn **~1.700–3.500 dòng/phân hệ**. Đó là khác biệt giữa một hệ quản
trị nội bộ và một hệ hành chính công.

---

## 8. RỦI RO THEO LỚP — dùng lại cho mọi dự án

| Lớp rủi ro | Dấu hiệu | Giá | Đệm nào chịu |
|---|---|---|---|
| **"Rào chắn trông như đang canh mà đã chết"** | Một phép kiểm tự động vẫn chạy, vẫn xanh, và **không còn khớp gì nữa**. Không có gì đỏ vào ngày nó chết | 0,2–0,5 NN mỗi ca · **đếm bằng nhiều ca, không phải một** | A |
| Câu hỏi **chặn schema** trả lời muộn | Khách chưa chốt mà schema đã phải viết | Đổi cấu trúc trên dữ liệu đang chạy | C |
| Hạ tầng **khác mô tả** | Nhận được cụm rồi mới biết nó chặn egress / DNS nội bộ | +1,00 NN, và nhiều hơn nếu phát hiện muộn | H3B Validate |
| Bên thứ ba trả hồ sơ | Sai một khoá cấu hình, sai thư mục build | 1 vòng = 0,5–1,0 NN + 5–20 NC | H4C |
| **Mất song song** giữa các agent | Hai agent ghi đè nhau ⇒ phải chuyển tuần tự | **×2..×3 toàn bộ H2** | không đệm nào chịu nổi — giữ H0 |
| Phạm vi chưa xác định | Có phân hệ không có đặc tả | Ước sơ bộ + hỏi trước khi ký | B (chọn 15%) |

> **Lớp đầu bảng là lớp đắt nhất và ít ai tính vào estimate.** Phép kiểm duy nhất đáng tin cho
> một rào chắn là **đột biến**: sửa một dòng mà nó đáng lẽ phải chặn, rồi xem nó có chặn không.
> Cổng kiểm xanh chỉ nói rào **chạy được**; nó không nói rào **còn nhìn thấy gì**.

---

## 9. CHECKLIST TRƯỚC KHI CHỐT GIÁ

```
[ ] Mọi phân hệ trong đặc tả đều có một dòng trong H2b — kể cả phân hệ "chắc là nhỏ"
[ ] Phân hệ nào KHÔNG có đặc tả đã được nêu tên và hỏi khách (đừng ước thầm)
[ ] H2c có mặt, và trong đó có dòng "chạy migration trên CSDL thật"
[ ] Câu hỏi khách đã gom theo NGƯỜI quyết, không theo phân hệ
[ ] Câu đắt bất đối xứng đã được đánh dấu và xếp vào buổi đầu
[ ] NN và NC tách bạch ở mọi hạng mục phụ thuộc bên khác
[ ] Ba phần đệm đứng thành ba dòng riêng, không gộp
[ ] Đã đóng gói 0,25 VÀ không nhân thêm hệ số an toàn nào nữa
[ ] Ô nào sát dưới mốc 0,25 đã được đánh ⚠ và soát lại
[ ] Bảng tổng đệm (thô → đóng gói → sau buffer) có trong bản gửi khách
[ ] Điều khoản "1 vòng duyệt trong giá, từ vòng 2 tính thêm" nếu có bên thứ ba
[ ] Điều khoản hạ tầng: NN có điều kiện, điều kiện NÊU TÊN
[ ] Bảng "bản này KHÔNG bao gồm" đã viết
```

---

## 10. KHUNG NÀY KHÔNG DÙNG ĐƯỢC KHI NÀO

| Không dùng khi | Vì sao |
|---|---|
| Có **đội lập trình viên** làm cùng | Đơn vị đo sai từ gốc; hệ số đo trên mô hình một người định hướng |
| Không có tầng cưỡng chế tự kiểm (H0) | Vẫn ra được lượng mã ấy, nhưng estimate không còn nghĩa: không ai biết kết quả đúng hay sai |
| Dự án **sửa hệ thống đang chạy có người dùng thật** | Khung này ước **dựng mới**. Sửa hệ đang chạy có lớp chi phí riêng: di trú dữ liệu, tương thích ngược, cửa sổ ngừng dịch vụ |
| Đặc tả chỉ có prototype, không có văn bản | Hệ số không còn đúng — xem §2.1 |
| Quá hạn tệp này mà chưa đo lại hệ số | Dùng **khung**, đừng dùng **số** |
