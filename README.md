# DNA序列模式检索服务

基于 Go 1.27.1 与 Chi 5.2.1 的 HTTP 服务，在线性或环状 DNA 序列中检索带通配碱基（N）的模式，支持正链/反链/双链、重叠命中、跨原点片段和稳定分页。

## 启动

```bash
go build -o bin/server ./cmd/server
ADDR=127.0.0.1:8080 ./bin/server
# 或直接运行
ADDR=127.0.0.1:8080 go run ./cmd/server
```

健康检查：`GET /healthz` 返回 `{"status":"ok"}`。

## 接口

`POST /search`，`Content-Type: application/json`，UTF-8 JSON。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `fasta` | string | FASTA 文本。标题行首个空白分隔词为记录 ID（须唯一）；序列支持多行、空行与大小写，仅允许 ACGTN |
| `patterns` | array | `{"name": 唯一名称, "pattern": ACGTN模式}`，最多 64 个，单个模式最长 10000 |
| `circular_record_ids` | string[] | 可选。列在此处的记录按环状处理，未列出的一律线性 |
| `strand` | string | `forward`（正链）、`reverse`（反链）、`both`（双链，默认） |
| `page` / `page_size` | int | 可选，默认 `1` / `50`，`page_size` 最大 1000 |

请求体上限 1 MiB（超出返回 413）。任何校验错误都以 HTTP 400 拒绝整次请求，不返回部分结果：

```json
{"error":"请求校验失败，整次请求已拒绝","errors":[{"record_id":"r1","line":2,"column":3,"message":"..."}]}
```

错误项带定位：FASTA 用 `record_index`（从 0 开始）、`record_id`、`line`、`column`（从 1 开始）；模式用 `patterns[i].name` / `patterns[i].pattern` 字段名与 `column`。空记录、重复记录 ID、空/重复模式名、非法字符、未知环状 ID、非法 strand 都会被指出。

## 匹配与坐标语义

- N 为通配碱基，匹配任意单个碱基，模式两端含 N 同样匹配。
- 正链直接检索；反链按模式的反向互补（A↔T、C↔G、N 不变）检索。
- 所有坐标统一映射到输入正链，从 0 开始、左闭右开 `[start, end)`。
- 重叠命中全部保留。双链模式下同一模式、同一区间在两条链同时命中时合并为一条，`strand` 标为 `both`；不同名称的模式即使区间相同也不合并。
- 环状记录允许命中跨越原点：每个起点只检查一圈，比记录长的模式不产生命中。跨原点命中 `cross_origin=true`，`end=-1`，`segments` 给出沿输入正链向右排列的两段（如 `[{2,4},{0,2}]`），不会用 start>end 的普通区间代替。线性记录绝不首尾拼接。
- 结果按记录输入顺序 → 起点 → 模式输入顺序稳定排列；分页基于该稳定顺序，重复查询和翻页不漏不重，`total` 为未分页的总命中数。

命中对象：

```json
{
  "record_id": "circ",
  "pattern_name": "wrap-ataa",
  "strand": "forward",
  "start": 2,
  "end": -1,
  "length": 4,
  "cross_origin": true,
  "segments": [{"start": 2, "end": 4}, {"start": 0, "end": 2}]
}
```

## 示例

同时展示双链合并、重叠保留与环状跨原点：

```bash
curl -s -X POST http://127.0.0.1:8080/search \\
  -H 'Content-Type: application/json' \\
  -d '{
    "fasta": ">linear\\nATATCG\\n>circ\\nAAAT\\n",
    "patterns": [
      {"name": "ata", "pattern": "ata"},
      {"name": "palin-at", "pattern": "AT"},
      {"name": "wrap-ataa", "pattern": "ATAA"}
    ],
    "circular_record_ids": ["circ"],
    "strand": "both"
  }'
```

- `palin-at`（AT 的反向互补仍是 AT）在线性序列起点 0、2 两链同时命中，各合并为一条 `both`。
- `ata` 正链命中起点 0，其反向互补 TAT 命中起点 1，重叠保留且不合并（非同区间）。
- 环状记录 `circ`（AAAT）上，`wrap-ataa` 起点 2 跨原点：`segments=[2,4)+[0,2)`。

分页：

```bash
curl -s -X POST http://127.0.0.1:8080/search \\
  -H 'Content-Type: application/json' \\
  -d '{"fasta":">r\\nAAA\\n","patterns":[{"name":"a","pattern":"A"}],"strand":"forward","page":2,"page_size":2}'
```

## 测试与构建

```bash
go test ./...
go vet ./...
go build -o bin/server ./cmd/server
```

依赖源码在 `vendor/`，版本由 `go.mod`/`go.sum` 锁定；工具链见 `toolchain.json`。
