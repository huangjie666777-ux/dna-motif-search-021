# DNA序列模式检索服务

Go与Chi的HTTP项目骨架。当前只有健康检查，检索业务尚未实现。

## 开发

使用Go1.27.1和Chi5.2.1。依赖由go.mod与go.sum锁定。

```bash
export PATH="/home/hj/.local/share/cc-codex/toolchains/go1.27.1-case004/go/bin:$PATH"
go test ./...
go build -o bin/server ./cmd/server
ADDR=127.0.0.1:8080 go run ./cmd/server
```

ADDR可设置监听地址，默认127.0.0.1:8080。并行运行时可在当前终端选择空闲端口。
