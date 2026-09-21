// ViGov — cổng kiểm TOÀN KHO. Tệp này KHÔNG đóng ảnh nào.
//
// ─────────────────────────────────────────────────────────────────────────────────────────
// VÌ SAO TỆP NÀY VẪN TỒN TẠI KHI MỖI DỊCH VỤ ĐÃ CÓ PIPELINE RIÊNG
//
// Mỗi dịch vụ tự quyết build cái gì và khi nào: `service-<tên>/Jenkinsfile`. Web cũng vậy:
// `web-admin/Jenkinsfile`. Một thay đổi ở `comms` không còn bắt `finance` sinh ảnh
// mới rồi triển khai lại.
//
// Nhưng có một nhóm bất biến KHÔNG THUỘC VỀ DỊCH VỤ NÀO CẢ:
//
//   · 7 bất biến cấu trúc của bộ não (.claude/) — luật nào có hook nào, agent nào có thật
//   · 90 ca tự kiểm hook — bằng chứng lớp thực thi còn chạy
//   · bất biến an toàn của 9 Dockerfile + 9 Jenkinsfile (tools/check_build.py)
//   · `buf lint` trên proto/ — hợp đồng GIỮA các dịch vụ, không của bên nào
//   · `gofmt`, `go vet`, `go build`, `go test -race` trên TOÀN kho, gồm core/ và tools/
//
// Nhét chúng vào tám pipeline dịch vụ nghĩa là chạy tám lần một việc. Và tệ hơn: nếu chúng
// CHỈ chạy ở pipeline dịch vụ, thì một thay đổi chỉ đụng `.claude/` hoặc `tools/` sẽ không
// kích hoạt pipeline nào — bộ não trôi mà không có gì đỏ.
//
// Nên phân chia theo QUYỀN SỞ HỮU chứ không theo tiện tay:
//   dịch vụ sở hữu ảnh của nó  ·  kho sở hữu bất biến của kho.
// ─────────────────────────────────────────────────────────────────────────────────────────
//
// MÁY CHỦ BUILD CẦN: go (1.26+), buf, node (22+), python3, và một trình biên dịch C —
// `go test -race` cần cgo. KHÔNG cần docker: tệp này không đóng ảnh.

pipeline {
  agent any

  options {
    timestamps()
    timeout(time: 40, unit: 'MINUTES')
    buildDiscarder(logRotator(numToKeepStr: '30'))
    disableConcurrentBuilds()
  }

  stages {

    stage('Kiểm công cụ') {
      steps {
        // `make` và `gcc` vào danh sách sau lượt chạy thật đầu tiên (2026-09-21).
        //   · `make`: mọi stage dưới đây gọi `make proto` / `make check`. Thiếu nó thì lỗi là
        //     "make: not found" ở giữa một stage, trông như mục tiêu hỏng chứ không như máy
        //     chủ thiếu công cụ.
        //   · `gcc`: `make check` chạy `go test -race`, mà `-race` cần cgo. Thiếu nó thì lỗi
        //     là "race is only supported ... with cgo" — trông như mã hỏng.
        sh '''
          set -eu
          thieu=""
          for cc in go buf node npm python3 make gcc; do
            command -v "$cc" >/dev/null 2>&1 || thieu="$thieu $cc"
          done
          if [ -n "$thieu" ]; then
            echo ""
            echo "MÁY CHỦ BUILD THIẾU:$thieu"
            echo "Dừng ở đây thay vì đổ giữa chừng với một lỗi không đọc được."
            echo ""
            exit 1
          fi
          go version; buf --version; node --version
        '''
      }
    }

    stage('Sinh mã từ .proto') {
      steps {
        // gen/ nằm trong .gitignore và mã nguồn import nó; buf.gen.yaml dùng plugin TỪ XA
        // nên bước này cần mạng ra buf.build — lý do nó ở máy chủ build chứ không ở trong
        // Dockerfile, nơi mạng thường bị chặn và cũng nên bị chặn.
        sh 'make proto'
        sh 'test -d core/gen || { echo "make proto chạy xong mà không có core/gen/"; exit 1; }'
      }
    }

    stage('Phụ thuộc web') {
      steps {
        // Chạy TRƯỚC cổng kiểm, có chủ ý. Mục `web` của `make check` BỎ QUA phần kiểm
        // TypeScript khi không có node_modules — nó báo to, nhưng vẫn là bỏ qua, và một cổng
        // kiểm có thể bỏ qua chính là thứ dự án này liên tục gặp: thứ trông như biện pháp mà
        // không phải biện pháp. Cài trước thì nó không còn đường bỏ qua.
        dir('web-admin') {
          sh 'npm ci'
        }
      }
    }

    stage('Cổng kiểm') {
      steps {
        // MỘT ĐỊNH NGHĨA DUY NHẤT của "đã kiểm": `make check`. Pipeline cố ý KHÔNG liệt kê
        // lại từng bước — hai danh sách sẽ lệch, và bản lỏng hơn là bản chạy trên CI trong
        // khi mọi người tin vào bản chặt hơn (luật 9).
        sh 'make check'
      }
    }
  }

  post {
    success {
      echo 'Bất biến toàn kho: xanh. Ảnh do pipeline của TỪNG dịch vụ đóng — xem ' +
           'service-<tên>/Jenkinsfile và web-admin/Jenkinsfile.'
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────────────────
// QUY ƯỚC THẺ ẢNH — nêu MỘT LẦN ở đây, và `tools/check_build.py` giữ cho chín pipeline kia
// không ai phá.
//
// Mọi ảnh chỉ mang đúng một thẻ: commit đã sinh ra nó. Không `latest`, không thẻ di động nào.
//
// Một thẻ di động (`latest`, `main`, `prod`) nghĩa là hai pod cùng một manifest có thể đang
// chạy hai đoạn mã khác nhau, tuỳ lúc nào chúng kéo ảnh. Trong một hệ thống lưu hồ sơ hành
// chính có giá trị pháp lý, câu "bản nào đang chạy lúc đó" phải trả lời được bằng một mã
// commit, không phải bằng suy đoán từ thời điểm kéo ảnh.
//
// Vì mỗi dịch vụ nay dựng độc lập, HAI DỊCH VỤ Ở HAI COMMIT KHÁC NHAU LÀ BÌNH THƯỜNG — đó
// chính là điểm của việc tách. Nhưng vì tám dịch vụ dùng chung `core/`, một dịch vụ không
// được dựng lại là một dịch vụ đang chạy `core/` cũ. Đó là lý do danh sách đường kích hoạt
// trong mỗi pipeline dịch vụ PHẢI có `core/**`, `proto/**`, `go.work`.
//
// Cái giá là manifest phải sửa thẻ mỗi lần triển khai. Đó chính là điều mong muốn: một lần
// triển khai phải là một thay đổi ai đó nhìn thấy được.
// ─────────────────────────────────────────────────────────────────────────────────────────
//
// VÌ SAO KHÔNG GỌI PLUGIN DOCKER PIPELINE — nêu MỘT LẦN ở đây; tám pipeline đóng ảnh
// (`web-admin` + bảy dịch vụ) chỉ trỏ tới mục này, không chép nó xuống.
//
// Tám tệp ấy từng viết `docker.withRegistry { docker.build(...).push() }`. Hai lệnh đó thuộc
// plugin **Docker Pipeline**, và MÁY CHỦ JENKINS THẬT KHÔNG CÓ NÓ. Phát hiện ngày 2026-09-21
// trên kho `vihat-miniapp` (dùng chung máy chủ này), và triệu chứng đáng nhớ vì nó không
// giống một lỗi thiếu plugin:
//
//     Invalid agent type "docker" specified. Must be one of [any, label, none]
//
// Ba tên ấy là những loại agent Jenkins core tự biết. Danh sách chỉ có ba tên nghĩa là KHÔNG
// plugin nào đăng ký thêm loại nào — và lỗi xảy ra lúc BIÊN DỊCH Jenkinsfile, trước khi lượt
// build kịp chạy một dòng, nên trong log không có stage nào để lần theo.
//
// Máy chủ Jenkins dùng chung với dự án khác, nên "cài thêm plugin" không phải quyết định của
// kho này. Tám tệp ấy nay gọi thẳng `docker` CLI.
//
// KHÔNG `docker login`, KHÔNG credentials — VÀ ĐÓ LÀ MỘT PHỤ THUỘC, không phải một chỗ thiếu.
// Máy chủ này đã đăng nhập sẵn và lâu dài vào Harbor bằng tài khoản của user `jenkins`: token
// nằm trong `~jenkins/.docker/config.json`, do ai đó đăng nhập một lần và KHÔNG phiên bản hoá ở
// đâu cả. Pipeline `cloud-vihat-saas-omicrm-callbot-service` trên cùng máy chủ đẩy ảnh đúng
// theo cách này và đã chạy nhiều tháng; ngày 2026-09-21 người dùng chốt dùng chung cơ chế ấy
// thay vì tạo mục credentials riêng cho ViGov.
//
// CÁI GIÁ, ghi ra để người sau biết mình đang đổi cái gì lấy cái gì:
//
//   · NHẬT KÝ MẤT MỘT CÂU. Vết chỉ trả lời được "Jenkins đẩy ảnh này", không trả lời được
//     "TÀI KHOẢN NÀO đẩy". Với hệ thống hành chính, đó là một câu hỏi thanh tra có thể hỏi.
//     Nó chấp nhận được CHỪNG NÀO Jenkins này chỉ có đúng một danh tính đẩy ảnh — ngày có
//     danh tính thứ hai, phải quay lại đây.
//   · MỘT TRẠNG THÁI VÔ HÌNH THÀNH ĐIỀU KIỆN CHẠY. Ngày token trên máy chủ hết hạn hoặc bị thu
//     hồi, MỌI job đóng ảnh đỏ cùng lúc, và không kho mã nào chứa manh mối vì trạng thái ấy
//     không nằm trong kho nào. Vì thế mỗi tệp KIỂM NÓ RA MẶT ở stage 'Chuẩn bị': tìm tên
//     registry trong `config.json` và dừng ngay với câu giải thích, thay vì để `docker push`
//     đỏ sau hai phút dựng với "denied: requested access to the resource is denied" — một dòng
//     đọc như lỗi phân quyền của tài khoản chứ không như "máy này chưa đăng nhập bao giờ".
//
// Muốn lấy lại danh tính riêng: tạo một mục credentials kiểu Username with password ở phạm vi
// GLOBAL (phạm vi System thì job không đọc được), rồi bọc bước `sh` đóng ảnh bằng
// `withCredentials` + `docker login --password-stdin`. Khi ấy nhớ hai thứ đi kèm: `set +x`
// quanh lệnh login, vì bước `sh` chạy `sh -xe` và `-x` in ra ĐỐI SỐ ĐÃ KHAI TRIỂN (luật 8); và
// `DOCKER_CONFIG` riêng từng lượt, vì `docker logout` trên tệp dùng chung sẽ đá văng phiên
// đăng nhập của job dự án khác đang đẩy ảnh giữa chừng.
//
// `docker image rm` SAU KHI ĐẨY thì giữ lại trong cả hai đường: máy dùng chung, mỗi commit một
// thẻ mới, không dọn thì đĩa của người khác đầy vì kho này. Chỉ bỏ THẺ — các tầng nằm lại
// trong cache và lượt sau vẫn dựng nhanh.
// ─────────────────────────────────────────────────────────────────────────────────────────
