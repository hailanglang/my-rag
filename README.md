# my-rag

单机 RAG：Go 后端（上传、索引、向量检索、SSE 对话）+ React（Vite）前端。密钥仅在后端使用。

## 前置条件

- Go **1.22+**（以根目录 `go.mod` 的 `go` 指令为准）
- **pnpm**（前端）
- 可用的 **DeepSeek API Key**（对话）
- 可用的 **阿里云百炼 API Key**（向量化，[官方说明](https://help.aliyun.com/zh/model-studio/embedding)）；若未单独配置，会回退使用 `DEEPSEEK_API_KEY`（仅在你把 `EMBED_BASE_URL` 指到同一兼容网关时才有意义）

## 环境变量（后端）

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `DEEPSEEK_API_KEY` | **是** | — | 对话用：DeepSeek（或兼容 Chat Completions 的服务）API Key |
| `DEEPSEEK_BASE_URL` | 否 | `https://api.deepseek.com` | 对话 Base URL，**不要**带末尾 `/` |
| `DEEPSEEK_CHAT_MODEL` | 否 | `deepseek-chat` | 对话模型，`POST .../v1/chat/completions` |
| `DASHSCOPE_API_KEY` | 否* | — | 向量化用：阿里云百炼（DashScope）API Key；未设置时依次尝试 `EMBED_API_KEY`、`DEEPSEEK_API_KEY` |
| `EMBED_BASE_URL` | 否 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | 嵌入 OpenAI 兼容 Base（北京地域）；新加坡见下方 |
| `EMBED_MODEL` | 否 | `text-embedding-v4` | 嵌入模型名，见[向量化文档](https://help.aliyun.com/zh/model-studio/embedding) |
| `EMBED_DIMENSIONS` | 否 | `1024` | 向量维度；设为 `0` 表示请求体不传 `dimensions`（使用服务商默认）。**更换维度或模型后需重新索引文档** |
| `EMBED_API_KEY` | 否 | — | 仅嵌入用的 Key（不配则用 `DASHSCOPE_API_KEY` / `DEEPSEEK_API_KEY`） |
| `HTTP_ADDR` | 否 | `:8080` | HTTP 监听地址 |
| `DATABASE_PATH` | 否 | `./data/app.db` | SQLite 数据库文件路径 |
| `UPLOAD_DIR` | 否 | `./data/uploads` | 上传文件目录 |
| `RETRIEVE_TIMEOUT_SEC` | 否 | `10` | 检索超时（秒） |
| `LLM_STREAM_TIMEOUT_SEC` | 否 | `180` | 单次流式对话总超时（秒） |
| `INDEX_TIMEOUT_SEC` | 否 | `300` | 单文档索引超时（秒） |

\* 使用默认百炼嵌入时，请配置 **`DASHSCOPE_API_KEY`**（与对话 Key 可不同）。

**新加坡地域**嵌入：将 `EMBED_BASE_URL` 设为 `https://dashscope-intl.aliyuncs.com/compatible-mode/v1`。

将 `DEEPSEEK_*` 指到其它 OpenAI 兼容网关即可替换对话供应商；将 `EMBED_*` 指到其它兼容 `/v1/embeddings` 的网关即可替换嵌入供应商。

## DeepSeek 与百炼（OpenAI 兼容）说明

后端通过 **OpenAI 兼容 HTTP 接口** 调用上游：

- **对话**：`{DEEPSEEK_BASE_URL}/v1/chat/completions`（流式 `stream: true`），`Authorization: Bearer <DEEPSEEK_API_KEY>`
- **向量化**：`{EMBED_BASE_URL}/v1/embeddings`，`Authorization: Bearer <Embed 侧 API Key>`（默认模型 **text-embedding-v4**，见阿里云 [向量化](https://help.aliyun.com/zh/model-studio/embedding)）

## 启动后端

在仓库根目录（本 worktree 根）执行：

```bash
set DEEPSEEK_API_KEY=你的DeepSeek密钥
set DASHSCOPE_API_KEY=你的百炼密钥
go run ./cmd/server
```

Linux / macOS：

```bash
export DEEPSEEK_API_KEY=你的DeepSeek密钥
export DASHSCOPE_API_KEY=你的百炼密钥
go run ./cmd/server
```

看到日志 `listening :8080`（或你配置的 `HTTP_ADDR`）即表示服务已就绪。健康检查：`GET http://127.0.0.1:8080/api/health`。

## 启动前端

另开终端：

```bash
cd web
pnpm install
pnpm dev
```

Vite 开发服务器默认将 **`/api` 代理到 `http://127.0.0.1:8080`**（见 `web/vite.config.ts`）。请保持后端监听地址与代理一致，或按需修改代理目标。

浏览器打开终端里打印的本地 URL（一般为 `http://localhost:5173`）。

## 浏览器里快速验证（上传 + 问答）

1. 在 **Documents** 区域点击 **Upload**，选择一段与知识库相关的 **`.txt`**（例如剪贴一小段 README 内容保存为文件后上传）。
2. 等待文档状态变为 **ready**（索引完成；失败时查看列表中的错误信息）。
3. 在 **Chat** 输入问题，点击 **Send**，应看到流式回答；若检索到片段，回答下方可展开 **Citations** 查看引用。
4. 点击 **Stop**：前端会中止 `fetch` 并调用 `POST /api/sessions/{id}/abort`；后端会取消进行中的上游请求。可在后端终端日志中关注是否出现取消相关记录（例如 `CANCELED` / `request canceled` 等，视上游返回而定）。

生产构建前端：

```bash
cd web
pnpm run build
```

## 本地冒烟清单（建议）

- [ ] 上传含「可检索」正文的 `.txt`，索引成功。
- [ ] 针对该内容提问，流式输出正常。
- [ ] 有命中时 **Citations** 非空。
- [ ] 生成过程中点 **Stop**，流式输出停止，且后端不再持续消耗上游（结合日志或上游控制台确认）。

## 测试（前端）

```bash
cd web
pnpm test
```

## 许可证

若根目录未单独声明，以仓库上层或组织约定为准。
