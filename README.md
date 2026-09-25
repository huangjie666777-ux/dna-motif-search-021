# DNA序列模式检索服务

基于 Go 1.27.1 与 Chi 5.2.1 的 HTTP 服务，在线性或环状 DNA 记录中检索带名称的模式，支持正链、反链与双链检索、通配符 N、重叠命中、跨原点命中与分页。

## 启动

```bash
go test ./...
go build -o bin/server ./cmd/server
ADDR=127.0.0.1:8080 ./bin/server
# 或直接运行
ADDR=127.0.0.1:8080 go run ./cmd/server
```

健康检查：`GET /healthz` → `{"status":"ok"}`。

## 请求

`POST /api/v1/search`，`Content-Type: application/json`，请求体上限 **2 MiB**（超出返回 413），模式数量上限 **100**，单个模式长度上限 **256**，`page_size` 上限 **500**。

```json
{
  "fasta": ">plasmid demo record\nACGTAC\n\n>chr2 another\nAAAA\n",
  "motifs": [
    {"name": "site-1", "sequence": "ACACG"},
    {"name": "site-2", "sequence": "NNTG"}
  ],
  "circular_ids": ["plasmid"],
  "strand": "both",
  "page": 1,
  "page_size": 50
}
```

字段说明：

- `fasta`：FASTA 文本。支持多行序列、空行与大小写混用；标题 `>` 后第一个空白分隔词作为唯一记录 ID。序列只允许 `ACGTN`（大小写不敏感）。
- `motifs`：模式数组，`name` 必须非空且在请求内唯一，`sequence` 只允许 `ACGTN`。`N` 表示任意单个碱基；模式或序列任意一侧为 N 即匹配，因此两端含 N 也能命中。
- `circular_ids`：按环状处理的记录 ID 列表；ID 必须存在于 FASTA 中，否则整次请求拒绝。不在列表中的记录一律按线性处理，绝不首尾拼接。
- `strand`：`forward`（仅正链）、`reverse`（仅反链）或 `both`（双链）。
- `page` / `page_size`：1 起始页码；省略时 `page=1`、`page_size=50`。结果排序固定，重复查询和翻页不漏不重。

## 坐标与命中语义

- 所有坐标统一映射到**输入正链**，0 起始、左闭右开 `[start, end)`。
- 反链检索使用模式的反向互补（A↔T、C↔G、N 不变），返回的仍是正链坐标，并标记 `"strand":"reverse"`。
- 双链模式下，同一模式在同一区间同时命中正链和反链时合并为一条，标记 `"strand":"both"`；不同名称的模式即使区间相同也不合并。
- 重叠命中全部保留（例如线性序列 `AAAA` 检索 `AA` 有起点 0、1、2 三个命中）。
- 环状记录每个起点只检查一圈；模式长于记录时不产生命中。
- 跨原点命中 `cross_origin=true`，返回沿正链向右排列的两段 `segments`：第一段 `[start, 记录长度)`，第二段 `[0, end)`；不会返回起点大于终点的普通区间。
- 排序：记录输入顺序 → 起点升序 → 模式输入顺序（稳定排序）。

响应示例（跨原点）：

```json
{
  "total": 1,
  "page": 1,
  "page_size": 50,
  "hits": [
    {
      "record_id": "plasmid",
      "motif_name": "ACACG",
      "strand": "forward",
      "start": 4,
      "end": 3,
      "cross_origin": true,
      "segments": [{"start": 4, "end": 6}, {"start": 0, "end": 3}],
      "circular": true
    }
  ]
}
```

## 错误处理

校验失败时整次请求拒绝（HTTP 400），不返回任何部分结果；错误为 JSON，`issues` 中每项给出 FASTA 的 `line`/`column` 或请求字段 `field`：

```json
{
  "error": "request rejected: validation failed; no partial results returned",
  "issues": [
    {"line": 2, "column": 3, "message": "illegal character 'X' in sequence; only ACGTN (case insensitive) are allowed"},
    {"field": "circular_ids[0]", "message": "unknown circular record ID \"ghost\""}
  ]
}
```

拒绝条件包括：空输入或空记录、重复/缺失记录 ID、序列或模式中的非法字符（带行/列或字段位置）、空或重复模式名、未知或重复的环状 ID、非法 `strand`、模式数量或长度超限。

## curl 示例

```bash
# 双链：ACGT 与其反向互补相同，同区间合并为 both
curl -s -X POST http://127.0.0.1:8080/api/v1/search \
  -H 'Content-Type: application/json' \
  -d '{
    "fasta": ">chrX\nACGT\n",
    "motifs": [{"name": "pal", "sequence": "acgt"}],
    "strand": "both"
  }'

# 环状跨原点：长度 6 的环上，ACACG 起点 4 -> 段 [4,6) 与 [0,3)
curl -s -X POST http://127.0.0.1:8080/api/v1/search \
  -H 'Content-Type: application/json' \
  -d '{
    "fasta": ">plasmid\nACGTAC\n",
    "motifs": [{"name": "cross", "sequence": "ACACG"}],
    "circular_ids": ["plasmid"],
    "strand": "forward"
  }'

# 重叠命中 + 分页
curl -s -X POST http://127.0.0.1:8080/api/v1/search \
  -H 'Content-Type: application/json' \
  -d '{
    "fasta": ">r\nAAAA\n",
    "motifs": [{"name": "AA", "sequence": "AA"}],
    "strand": "forward",
    "page": 1, "page_size": 2
  }'
```

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
