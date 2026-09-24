# DNA序列模式检索服务

Go与Chi的HTTP项目骨架，目前只有健康检查，检索业务尚未实现。

## 开发

使用WSL共用的Go1.27.1。Chi5.2.1依赖源码保存在各自项目的vendor中，版本由go.mod和go.sum锁定。

```bash
go version
go test ./...
go build -o bin/server ./cmd/server
ADDR=127.0.0.1:8080 go run ./cmd/server
```

正常WSL交互终端和登录shell已能直接调用go。若通过不加载用户环境的非登录shell调用，两侧可使用相同的临时设置：

```bash
export PATH="/home/hj/.local/bin:$PATH"
```

ADDR默认127.0.0.1:8080，并行启动可选择空闲端口。工具版本和来源见toolchain.json，不需要source项目环境文件。
