// ViGov — cổng kiểm TOÀN KHO. Tệp này KHÔNG đóng ảnh nào.
//
// ─────────────────────────────────────────────────────────────────────────────────────────
// VÌ SAO TỆP NÀY VẪN TỒN TẠI KHI MỖI DỊCH VỤ ĐÃ CÓ PIPELINE RIÊNG
//
// Mỗi dịch vụ tự quyết build cái gì và khi nào: `<tên>/Jenkinsfile`. Web cũng vậy:
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
        sh '''
          set -eu
          thieu=""
          for cc in go buf node npm python3; do
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
        sh 'test -d gen || { echo "make proto chạy xong mà không có gen/"; exit 1; }'
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
           '<tên>/Jenkinsfile và web-admin/Jenkinsfile.'
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
// trong mỗi pipeline dịch vụ PHẢI có `core/**`, `proto/**`, `go.mod`, `go.sum`.
//
// Cái giá là manifest phải sửa thẻ mỗi lần triển khai. Đó chính là điều mong muốn: một lần
// triển khai phải là một thay đổi ai đó nhìn thấy được.
// ─────────────────────────────────────────────────────────────────────────────────────────
