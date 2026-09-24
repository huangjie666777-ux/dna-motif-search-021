# DNA序列模式检索服务

Go与Chi的HTTP项目骨架。当前只有健康检查，检索业务尚未实现。

## 本地开发

当前目录内Go1.27.1位于.tools/go，Chi5.2.1依赖位于vendor，版本与校验信息见toolchain.json及go.sum。

```bash
source .tools/env.sh
go version
go test ./...
go build -o bin/server ./cmd/server
ADDR=127.0.0.1:8080 go run ./cmd/server
```

环境文件按自身位置定位，不含base或其他题的绝对路径。各副本独立使用本地工具、模块与编译缓存。ADDR默认127.0.0.1:8080，并行启动时可选择空闲端口。

## 从Git恢复本地工具

工具二进制和缓存不提交Git。按toolchain.json的source下载官方发行包，核对sha256后解压到.tools，使可执行文件位于.tools/go/bin/go。vendor中保留已锁定的依赖源码，无需另行安装。然后执行上面的source命令。
