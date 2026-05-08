# RAG 全栈应用 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付单机单用户 RAG 应用：Go 单体提供上传、索引、向量检索、DeepSeek 流式对话（SSE）与取消；React 前端完成文档管理与对话界面；会话与引用元数据 30 天保留；上传文件与索引同步删除；密钥仅在后端。

**Architecture:** 单体 `cmd/server` 启动 HTTP；`internal/` 按域拆分（配置、文档存储、解析、分块、嵌入、向量库、会话、RAG 编排、HTTP 处理器）。持久化用 **SQLite**（`modernc.org/sqlite`）存文档元数据、chunk、向量（float32 blob）、会话与消息；对象内容存本地目录 `data/uploads/`。向量检索在 SQLite 中存向量，Top-K 用 **纯 Go 余弦相似度** 扫描（MVP 文档量可控；后续可换 pgvector/Qdrant）。LLM 与嵌入均走 **OpenAI 兼容 HTTP**（DeepSeek 默认 Base URL，嵌入与对话共用密钥与 client 工厂）。定时任务清理 30 天外会话与引用元数据。

**Tech Stack:** Go 1.22+、`modernc.org/sqlite`、`github.com/ledongthuc/pdf`（无 OCR，仅文本层）、Vite + React + TypeScript、Vitest + RTL（前端）、`go test`（后端）。

---

## 文件结构（创建与职责）

| 路径 | 职责 |
|------|------|
| `go.mod` / `go.sum` | Go 模块根 |
| `cmd/server/main.go` | 读配置、打开 DB、迁移、启动 HTTP、后台 retention ticker |
| `internal/config/config.go` | 环境变量：端口、`DATABASE_PATH`、`UPLOAD_DIR`、`DEEPSEEK_BASE_URL`、`DEEPSEEK_API_KEY`、`DEEPSEEK_CHAT_MODEL`、`DEEPSEEK_EMBED_MODEL`、超时秒数 |
| `internal/store/db.go` | 打开 SQLite、连接池、外键 |
| `internal/store/migrate.go` | 嵌入 SQL 迁移：documents, chunks, sessions, messages, message_citations |
| `internal/store/queries.sql` | 可选：文档化 SQL；或迁移内联 |
| `internal/documents/model.go` | `Document` 状态：`pending/processing/ready/failed` |
| `internal/documents/repository.go` | CRUD、按 id 删除 |
| `internal/documents/service.go` | 上传保存文件、触发索引、删除同步文件+chunk |
| `internal/parse/extract.go` | 按扩展名路由：`.md`/`.txt` 直读；`.pdf` 用 ledongthuc/pdf，无文本则返回明确错误 |
| `internal/chunk/const.go` | `ChunkSizeRunes = 512`, `ChunkOverlapRunes = 64`（与规格「固定默认值」一致，可改为 byte 边界若实现时统一） |
| `internal/chunk/chunker.go` | 纯函数 `Chunk(text string) []string` |
| `internal/embed/interface.go` | `Embedder`：`Embed(ctx, texts []string) ([][]float32, error)` |
| `internal/embed/openai_compat.go` | OpenAI 兼容 `/v1/embeddings` 实现 |
| `internal/llm/interface.go` | `Streamer`：`StreamChat(ctx, messages, onDelta) error` |
| `internal/llm/deepseek.go` | 默认：BaseURL+`/v1/chat/completions`，`stream=true`，读 SSE 行 |
| `internal/vector/store.go` | 写入 chunk 向量、按 document_id 删除、Search(queryVec, k)` |
| `internal/rag/index.go` | 解析→分块→嵌入→写 chunks+vectors |
| `internal/rag/query.go` | 用户 query 嵌入→TopK→拼 citations（不记录完整 prompt 到 DB） |
| `internal/session/repository.go` | 会话、消息、引用元数据 CRUD；`PurgeOlderThan(d time.Duration)` |
| `internal/httpapi/router.go` | chi 或 std `net/http` 路由注册 |
| `internal/httpapi/middleware.go` | `request_id`、recover、超时包装 |
| `internal/httpapi/documents.go` | POST 上传、GET 列表、GET 状态、DELETE |
| `internal/httpapi/chat.go` | POST 创建流、SSE、`Request.Context` 取消 |
| `internal/httpapi/chat_cancel.go` | POST 取消（可选，与断开二选一；规格两者皆可，本计划实现**断开 + 显式 cancel**） |
| `internal/logx/logx.go` | 结构化 Info/Warn/Error，禁止打印完整用户消息体（仅长度或 hash） |
| `internal/chunk/chunker_test.go` | 分块单测 |
| `internal/parse/extract_test.go` | txt/md 提取单测 |
| `internal/llm/deepseek_test.go` | `httptest.Server` 模拟流式响应 |
| `internal/vector/store_test.go` | 内存或临时文件 DB 测 TopK |
| `web/package.json` | 前端依赖 |
| `web/vite.config.ts` | 代理 `/api` → Go |
| `web/src/main.tsx` | 入口 |
| `web/src/App.tsx` | 布局：文档区 + 聊天区 |
| `web/src/api/types.ts` | DTO |
| `web/src/api/documents.ts` | fetch 封装 |
| `web/src/api/chat.ts` | `fetch` + `ReadableStream` 解析 SSE 或 `EventSource`（POST SSE 需 fetch 读 body） |
| `web/src/hooks/useChatStream.ts` | AbortController、停止、累积文本与 citations |
| `web/src/components/DocumentPanel.tsx` | 列表、上传、状态 |
| `web/src/components/ChatPanel.tsx` | 消息列表、输入、停止 |
| `README.md` | 如何配置 `.env`、运行前后端 |

---

## Task 1: Go 模块与健康检查端点

**Files:**

- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `internal/httpapi/router.go`
- Create: `internal/httpapi/health_test.go`

- [ ] **Step 1: 编写失败测试（无路由时 404）**

Create `internal/httpapi/health_test.go`:

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth_OK(t *testing.T) {
	mux := NewRouter()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/httpapi/... -v`

Expected: FAIL（`NewRouter` 未定义或包不存在）

- [ ] **Step 3: 最小实现**

Create `go.mod`:

```go
module my-rag

go 1.22.0
```

Create `internal/httpapi/router.go`:

```go
package httpapi

import "net/http"

func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	return mux
}
```

Create `cmd/server/main.go`:

```go
package main

import (
	"log"
	"net/http"
	"os"

	"my-rag/internal/httpapi"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		addr = v
	}
	log.Printf("listening %s", addr)
	if err := http.ListenAndServe(addr, httpapi.NewRouter()); err != nil {
		log.Fatal(err)
	}
}
```

若需发布到 Git，可将 `module my-rag` 改为带域名的 module path，并全局替换 import 前缀。

- [ ] **Step 4: 运行测试通过**

Run: `go test ./internal/httpapi/... -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd/server/main.go internal/httpapi/router.go internal/httpapi/health_test.go
git commit -m "feat: add Go module and /api/health"
```

---

## Task 2: 配置加载与环境变量

**Files:**

- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: 失败测试**

`internal/config/config_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-test")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr=%q", c.HTTPAddr)
	}
	if c.RetrieveTimeout != 10*time.Second {
		t.Fatalf("RetrieveTimeout=%v", c.RetrieveTimeout)
	}
	if c.LLMStreamTimeout != 180*time.Second {
		t.Fatalf("LLMStreamTimeout=%v", c.LLMStreamTimeout)
	}
}
```

- [ ] **Step 2: `go test ./internal/config/...` → FAIL**

- [ ] **Step 3: 实现 `Load()`**

`internal/config/config.go` 定义结构体字段：

- `HTTPAddr`, `DatabasePath`（默认 `./data/app.db`）, `UploadDir`（默认 `./data/uploads`）
- `DeepSeekBaseURL`, `DeepSeekAPIKey`, `DeepSeekChatModel`, `DeepSeekEmbedModel`
- `RetrieveTimeout` 默认 `10s`，`LLMStreamTimeout` 默认 `180s`，`IndexTimeout` 默认 `5m`
- `Load()`：若 `DEEPSEEK_API_KEY` 为空返回 `error`

- [ ] **Step 4: `go test ./internal/config/...` → PASS**

- [ ] **Step 5: Commit** `feat: add env-based config`

---

## Task 3: SQLite 迁移与文档表

**Files:**

- Create: `internal/store/db.go`
- Create: `internal/store/migrate.go`
- Create: `internal/store/migrate_test.go`

- [ ] **Step 1: 失败测试** — 打开临时路径 DB，跑迁移，查询 `documents` 表存在。

`migrate_test.go` 使用 `t.TempDir()` + `filepath.Join` 构造 `app.db`，调用 `Open` + `Migrate`，`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='documents'` 期望 1。

- [ ] **Step 2: `go test ./internal/store/...` → FAIL**

- [ ] **Step 3: 迁移 SQL（内嵌字符串）**

`documents`：`id TEXT PK`, `filename TEXT`, `mime TEXT`, `size INTEGER`, `storage_path TEXT`, `status TEXT`, `error_message TEXT`, `created_at INTEGER`, `updated_at INTEGER`

`chunks`：`id TEXT PK`, `document_id TEXT`, `chunk_index INTEGER`, `text TEXT`, `embedding BLOB`, `created_at INTEGER`，FK `document_id` → `documents.id` ON DELETE CASCADE

（先不加 sessions，Task 6 再加，或本任务一次性加全表以减少迁移次数；**推荐本任务一次性创建** `sessions`, `messages`, `message_citations` 表结构见 Task 6 描述，避免多次迁移版本号逻辑。）

若拆两次迁移：本 Task 仅 documents+chunks，Task 6 追加 ALTER。计划采用 **单文件 migrate 含全部表** 在 Task 6 前合并 — 为简化，在 **Task 3** 直接创建完整 schema（复制 Task 6 的列定义到 `migrate.go`）。

`messages`：`id`, `session_id`, `role`, `content`, `created_at`（不存完整 prompt 字段；`content` 仅存用户可见助手/用户消息正文，检索上下文不入库或仅存摘要由你定——规格要求不存完整 prompt：**assistant 消息可存最终展示文本；user 消息存用户输入**；RAG 拼接的中间件不写入 `messages` 表。）

`message_citations`：`id`, `message_id`, `chunk_id`, `quote TEXT`, `document_id`, `created_at`

`sessions`：`id`, `title`, `created_at`, `updated_at`

- [ ] **Step 4: 测试 PASS**

- [ ] **Step 5: Commit** `feat: sqlite store and migrations`

---

## Task 4: 分块常量与纯函数

**Files:**

- Create: `internal/chunk/const.go`
- Create: `internal/chunk/chunker.go`
- Create: `internal/chunk/chunker_test.go`

- [ ] **Step 1: 测试** — 输入 1200 字符字符串，断言块数 ≥ 2，相邻块在 overlap 处有重复子串（或长度约束）。

`chunker_test.go` 示例：

```go
func TestChunker_Overlap(t *testing.T) {
	s := strings.Repeat("あ", 2000) // 或 ASCII
	parts := Chunk(s)
	if len(parts) < 2 {
		t.Fatalf("len=%d", len(parts))
	}
}
```

- [ ] **Step 2: FAIL**

- [ ] **Step 3: 实现** `const.go`：`ChunkSizeRunes=512`, `OverlapRunes=64`。`chunker.go`：按 rune 切片滑动窗口。

- [ ] **Step 4: PASS**

- [ ] **Step 5: Commit** `feat: fixed-size text chunker`

---

## Task 5: 文本/Markdown/PDF 解析

**Files:**

- Create: `internal/parse/extract.go`
- Create: `internal/parse/extract_test.go`

- [ ] **Step 1: 测试** — `.txt` 内容 `"hello"`；`.md` 同；`.pdf` 用生成的最小 pdf fixture 或跳过集成仅 mock **接口**（若 CI 无 fixture，用 `t.Skip` 保护，同时保留 `TestExtractTxt` 必过）。

- [ ] **Step 2: FAIL**

- [ ] **Step 3: 实现** `Extract(path string) (string, error)`：根据扩展名；pdf 用 `pdf.Open`，遍历取 `GetPlainText()` 或库 API；若提取空串返回 `fmt.Errorf("pdf: no extractable text (OCR not supported)")`

- [ ] **Step 4: PASS**

- [ ] **Step 5: Commit** `feat: document text extraction`

---

## Task 6: Embedder（OpenAI 兼容）单测 + 实现

**Files:**

- Create: `internal/embed/interface.go`
- Create: `internal/embed/openai_compat.go`
- Create: `internal/embed/openai_compat_test.go`

- [ ] **Step 1: 测试** — `httptest.Server` 返回固定 embedding JSON；调用 `NewClient(...).Embed(ctx, []string{"a"})` 得 `len(vec[0])>0`

- [ ] **Step 2-4: 实现** HTTP POST `{ "input": texts, "model": model }` 到 `/v1/embeddings`，解析 `data[].embedding`

- [ ] **Step 5: Commit** `feat: openai-compatible embedder`

---

## Task 7: DeepSeek 流式 Chat（可取消）

**Files:**

- Create: `internal/llm/interface.go`
- Create: `internal/llm/deepseek.go`
- Create: `internal/llm/deepseek_test.go`

- [ ] **Step 1: 测试** — mock server 返回 `data: {"choices":[{"delta":{"content":"Hi"}}]}\n\n` 与 `data: [DONE]`；`StreamChat` 累积到 `onDelta`；`ctx` cancel 后 `StreamChat` 返回 `context.Canceled`

- [ ] **Step 2: 实现** — `messages` 用 `[]map[string]string` 或强类型；设置 `stream:true`；读 body 按行；尊重 `ctx.Done()`

- [ ] **Step 3: PASS + Commit**

---

## Task 8: 向量存储与 TopK 搜索

**Files:**

- Create: `internal/vector/store.go`
- Create: `internal/vector/similarity.go`（余弦）
- Create: `internal/vector/store_test.go`

- [ ] **Step 1: 测试** — 插入三个已知向量（二维即可），query 与第一个相同，K=2，期望 id 顺序包含第一个。

- [ ] **Step 2: 实现** — `UpsertChunk(ctx, chunkID, docID, text, vec []float32)` 序列化 float32 little-endian；`DeleteByDocument(ctx, docID)`；`Search(ctx, query []float32, k int)` 全表扫描+堆或排序。

- [ ] **Step 3: PASS + Commit**

---

## Task 9: 索引流水线（文档 → chunks → embed → store）

**Files:**

- Create: `internal/rag/index.go`
- Create: `internal/rag/index_integration_test.go`（`//go:build integration` 可选；或用小 fake embedder）

- [ ] **Step 1: 测试** — fake `Embedder` 返回固定维向量；内存 sqlite；索引后 `Search` 能命中。

- [ ] **Step 2: 实现** `IndexDocument(ctx, docID)`：读文件→Extract→Chunk→Embed(batch)→写 DB+vector

- [ ] **Step 3: PASS + Commit**

---

## Task 10: HTTP 文档 API（上传、列表、删除）

**Files:**

- Modify: `internal/httpapi/router.go`
- Create: `internal/httpapi/documents.go`
- Create: `internal/documents/service.go`
- Create: `internal/documents/repository.go`
- Create: `internal/httpapi/documents_test.go`（`httptest` + 内存 DB）

- [ ] **Step 1: 测试** — `multipart` 上传小 txt，`GET /api/documents` JSON 数组含 status；`DELETE` 后文件不存在且 chunks 数为 0

- [ ] **Step 2: 实现** 保存到 `UploadDir/uuid+ext`，DB 记录 `pending`，异步 `go index`（错误写 `failed`+`error_message`）

- [ ] **Step 3: PASS + Commit**

---

## Task 11: RAG 查询组装（不含 HTTP）

**Files:**

- Create: `internal/rag/query.go`
- Create: `internal/rag/query_test.go`

- [ ] **Step 1: 测试** — 给定 DB 中两条 chunk，`BuildContext(query)` 返回 system 片段字符串与 `[]Citation`（chunk id, quote, doc name）

- [ ] **Step 2: 实现** — embed query → TopK → 拼 bullet 上下文；**不**调用真实 LLM

- [ ] **Step 3: PASS + Commit**

---

## Task 12: SSE 聊天与停止

**Files:**

- Create: `internal/httpapi/chat.go`
- Create: `internal/session/repository.go`（若未写）
- Create: `internal/httpapi/chat_test.go`

- [ ] **Step 1: 测试** — mock `Streamer`；`POST /api/sessions/{id}/messages` 或 `POST /api/chat` 返回 `Content-Type: text/event-stream`，body 含 `event: token\ndata: ...\n\n`；客户端 context cancel 后 handler 返回

- [ ] **Step 2: 实现** — 路由：`POST /api/sessions` 创建会话；`POST /api/sessions/:id/messages` body `{ "content":"..." }`；handler 使用 `flusher`；首条 SSE `event: citations\ndata: <json>\n\n`（若 citations 在流前已知）；随后 token；`log` 仅打 request_id 与 content 长度

- [ ] **Step 3: 可选** `POST /api/sessions/:id/abort` 设置 server-side cancel（与 `r.Context()` 组合）

- [ ] **Step 4: PASS + Commit**

---

## Task 13: 30 天保留清理任务

**Files:**

- Modify: `cmd/server/main.go`
- Create: `internal/session/purge.go`
- Create: `internal/session/purge_test.go`

- [ ] **Step 1: 测试** — 插入旧于 31 天的 `messages` 与 `message_citations`，跑 `PurgeOlderThan(30*24*time.Hour)`，断言删除

- [ ] **Step 2: 实现** — `DELETE FROM message_citations WHERE message_id IN (...)`；`DELETE FROM messages WHERE ...`；`DELETE FROM sessions` 若无消息（按你定义：删空会话或保留）

- [ ] **Step 3: `main` 中每 1h ticker 调用**

- [ ] **Step 4: PASS + Commit**

---

## Task 14: 前端脚手架与代理

**Files:**

- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/vite-env.d.ts`

- [ ] **Step 1:** `pnpm create vite@latest web -- --template react-ts`（或手写上述文件等价）；若脚手架未自动装依赖则执行 `cd web && pnpm install`

- [ ] **Step 2:** `vite.config.ts` 配置 `server.proxy['/api'] = 'http://127.0.0.1:8080'`

- [ ] **Step 3:** `cd web && pnpm run build` 成功

- [ ] **Step 4: Commit** `feat: vite react-ts frontend scaffold`

---

## Task 15: 文档面板 + 聊天面板（集成 UI）

**Files:**

- Create: `web/src/api/types.ts`
- Create: `web/src/api/documents.ts`
- Create: `web/src/api/chat.ts`
- Create: `web/src/hooks/useChatStream.ts`
- Create: `web/src/components/DocumentPanel.tsx`
- Create: `web/src/components/ChatPanel.tsx`
- Create: `web/src/App.tsx`
- Create: `web/src/App.test.tsx`

- [ ] **Step 1: 测试** — `msw` 或 mock `fetch`：`DocumentPanel` 渲染上传按钮；`ChatPanel` 在 mock SSE 下累积文本；停止按钮调用 `abort()`

Vitest 示例 `App.test.tsx`：

```tsx
import { render, screen } from '@testing-library/react'
import App from './App'

it('renders chat and documents', () => {
  render(<App />)
  expect(screen.getByRole('button', { name: /send/i })).toBeInTheDocument()
  expect(screen.getByText(/documents/i)).toBeInTheDocument()
})
```

- [ ] **Step 2: 实现** — `useChatStream`：`fetch('/api/sessions/'+id+'/messages',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({content}),signal})` 读 `body.getReader()` 解析 SSE；`AbortController` 绑定停止按钮

- [ ] **Step 3:** `pnpm test` PASS

- [ ] **Step 4: Commit** `feat: rag chat and document UI`

---

## Task 16: README 与端到端冒烟

**Files:**

- Create: `README.md`

- [ ] **Step 1:** 文档说明：`DEEPSEEK_*` 环境变量、DeepSeek OpenAI 兼容说明、`go run ./cmd/server`、`cd web && pnpm dev`、浏览器验证上传+问答

- [ ] **Step 2:** 本地手动：上传 `readme` 片段 txt → 提问 → 看到引用 → 点停止 → 上游请求停止（通过 mock 或日志验证）

- [ ] **Step 3: Commit** `docs: readme for local run`

---

## Self-review（对照规格）

| 规格条款 | 覆盖任务 |
|----------|----------|
| F1 上传 | Task 10 |
| F2 PDF 无文本错误 | Task 5 |
| F3 固定分块 | Task 4 |
| F4/F5/F6 向量检索 | Task 8, 9, 11 |
| F7/F8/F9/F10 SSE+citations+停止 | Task 12, 15 |
| F11–F14 前端 | Task 14–15 |
| F15 无登录 | 无 auth 中间件 |
| T3 DeepSeek+接口 | Task 6–7, `internal/llm/interface.go`, `internal/embed/interface.go` |
| T4 单体 | 单 `cmd/server` |
| NF6 30 天、无完整 prompt | Task 12 日志策略 + Task 13 |
| NF1 超时默认值 | Task 2 |
| 5.2 同步删除 | Task 10 DELETE |
| 5.4 错误类型 | handlers 返回 JSON `code` 字段（实现时在 Task 10/12 增加） |

**补遗（请在实现 Task 10/12 时写入代码，本计划不留 TBD）：** HTTP 错误体统一形如 `{"error":{"code":"INDEX_FAILED","message":"..."}}`，前端展示 `message`。

---

## Plan complete

Plan complete and saved to `docs/superpowers/plans/2026-05-08-rag-fullstack.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — 每个 Task 派生子代理执行，任务间人工或自动复核；**必须**配合 superpowers:subagent-driven-development。

2. **Inline Execution** — 本会话内按 Task 顺序执行；**必须**配合 superpowers:executing-plans，在检查点停顿复核。

**Which approach?**
