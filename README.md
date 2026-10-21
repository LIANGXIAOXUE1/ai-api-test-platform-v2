# Pulse API V2：AI API 测试工作台

## 一句话说明

这是一个把 **Postman 的请求调试** 和 **AI 自动填参、错误诊断、历史重放** 放在同一个页面里的 API 测试工作台。

用户只需要：

```text
选择接口 → 描述想验证什么 → AI 补齐参数 → 确认并发送 → 查看结果
```

## 它实际上能完成什么？

Pulse API V2 不负责替用户开发业务系统，而是帮助开发者把已有系统的接口测试，从“手动在 Postman 中反复填写并判断结果”，变成可配置、可批量执行、可保存、可比较、可分析的测试流程。

### 1. 测接口是否正常

像 Postman 一样发 GET、POST、上传文件、带 Token、填参数、看响应和状态码。

### 2. 帮用户少填配置

用户给接口地址、OpenAPI 文档，或直接说“我要测试登录接口”，AI 助手生成 URL、方法、请求体、Header 和断言草稿，用户确认后执行。

### 3. 根据业务规则自动补测试

用户上传 PRD、业务规则或错误码文档，例如：

```text
同目录文件夹不能重名
未登录不能查看文件
密码长度必须 6～32 位
```

平台自动生成“正常、边界、异常、鉴权”测试用例，而不是用户自己一个个想。

### 4. 自动执行复杂流程

例如 Cloud Disk 的分片上传：

```text
登录
→ 获取 Token
→ 初始化上传，得到 uploadId
→ 上传分片
→ 查询进度
→ 合并文件
```

平台能自动把 Token、uploadId 从上一步传到下一步，验证整条业务链路。

### 5. 发现改动后是否把旧功能改坏

保存一次“登录成功”“同名文件夹返回 409”等用例。以后后端改代码，重新执行，平台告诉你：

```text
原来：200，120ms，有 token
现在：401，或者 900ms，或者少了 token 字段
```

这就是回归分析。

### 用 Cloud Disk 演示时，完整故事就是

```text
启动 Cloud Disk
→ 导入它的 OpenAPI 接口文档
→ 上传它的业务规则文档
→ 平台生成登录、鉴权、文件夹重名等测试用例
→ 执行并保存结果
→ 后端修改后再次执行
→ 平台指出哪些接口、字段、状态码或性能发生变化
```

### AI 助手在这里的作用

AI 助手不是“聊天而已”，而是帮助用户操作这个测试平台：

```text
用户：帮我测试创建文件夹接口，同名时应该失败
        ↓
助手：读取当前接口和业务规则
        ↓
生成正常创建 + 重名冲突 + 未登录三条测试草稿
        ↓
用户确认
        ↓
平台执行、保存、后续回归比较
```

### 项目最终定位

> 一个面向开发者的 AI API 智能测试与回归分析平台。它支持普通 API、大模型 API 和业务链路测试，通过 AI 助手降低接口配置成本，并结合业务规则自动生成测试用例、执行测试和发现版本回归问题。

因此，本项目的价值不只是“再做一个接口发送工具”，而是将**接口配置、自动化测试、业务规则验证和回归分析**连接成可重复使用的开发流程。

## V2 解决了什么问题

传统接口测试经常需要反复填写 URL、请求方式、Headers、Body，再手动判断 401、404、500 等错误。V2 将这些内容集中在一屏：

- 左侧选择接口；
- 中间查看和修改真实请求；
- AI 根据自然语言补齐请求体和断言；
- 右侧立即显示状态码、耗时、响应和错误原因；
- 请求可以保存、重放，并进入回归对比。

## 主要功能

### 1. 一页式测试工作台

- 接口列表和搜索入口；
- GET、POST、PUT、DELETE、PATCH；
- URL、Headers、Body、查询参数可见且可修改；
- 一键发送或发送并保存为测试用例；
- 响应 Body 支持复制。

### 2. AI 补齐请求

输入：

```text
检查接口是否需要 Token，并确认返回 userId
```

AI 自动生成：

- 请求方式和 URL；
- 请求 Body；
- 基础断言；
- 生成依据。

AI 只负责生成草稿，用户确认后才真正调用接口，因此不是黑盒测试。

### 3. 独立的 AI 测试助手窗口

页面顶部和测试台右下角都有“AI 助手”入口，右下角入口以悬浮小动物形象呈现，不占用测试台布局。点击后会打开一个类似对话应用的工作区，用户可以像聊天一样描述目标，例如：

```text
帮我检查这个接口是否需要 Token，并确认返回 userId
```

助手会结合当前选中的接口调用 `/api/agent/draft`，生成并展示：

- 请求方法和 URL；
- Headers、Query 和 Body；
- 状态码、JSON 字段或结构断言；
- 本次生成的依据和测试目标。

用户可以在窗口中继续追问，也可以点击“应用到测试台”把草稿带回编辑器，检查后再点击“发送并保存”。助手不会未经确认直接调用被测接口。

助手窗口还提供了卡通化入口和文档导入能力：用户可以上传 TXT、Markdown、JSON、YAML、DOCX 或带文本层的 PDF。后端会先在本地提取文档文本，再把文本作为当前对话的上下文，帮助 Agent 识别接口需求、生成请求参数或回答文档相关问题。文档只作为本次对话上下文，不会自动写入接口表，也不会未经确认创建接口。

助手现在还支持新增接口：

```text
添加一个用户登录接口，POST /api/login，请求体包含 email 和 password
```

Agent 会先生成“接口配置草稿”，展示接口名称、方法、URL、Headers 和 Body。用户点击“添加到接口列表”后，系统才通过 `/api/apis` 写入 PostgreSQL，并自动选中这个新接口。这个流程采用“生成计划 → 用户确认 → 工具执行”的人机协作方式，模型不能直接修改数据。

### Agent 的设计方式

这个项目的 Agent 不是单独训练一个模型，而是由三部分组成：

```text
自然语言
   ↓
意图识别：测试接口 / 新增接口 / 配置模型接口
   ↓
结构化 Tool Calling：生成 request 或 endpoint JSON
   ↓
人工确认
   ↓
Go 工具执行：填入测试台、创建接口、发送测试请求
```

- 模型层：可接 Azure OpenAI、OpenAI、DeepSeek 或其他 OpenAI 兼容 API；
- Agent 层：Go 后端使用 `net/http` 调用 Chat Completions，并使用 Tool Calling 约束模型只返回结构化配置；
- 工具层：当前工具包括生成测试草稿和生成接口配置草稿，真正的接口创建、请求发送仍由 Go 后端完成；
- 兜底层：没有配置大模型时，使用规则 Agent 解析常见的请求方法、URL、接口类型和 JSON 字段，项目仍然可以离线演示。

因此，底层大模型不是“替系统发送请求”，而是负责理解用户意图和生成结构化参数；系统负责校验、展示、确认和执行。

### 为什么这是人工智能相关项目

这个项目属于 **AI 应用工程 / Agent 应用开发**，不是训练一个新的基础模型。AI 参与的是“理解、规划、生成、调用工具和分析结果”这一整条应用链路：

- 大模型理解“帮我添加登录接口”或“检查这个模型接口是否返回 JSON”等自然语言意图；
- Tool Calling 把意图转换为结构化的 URL、方法、Headers、Body 和断言；
- Agent 先生成任务计划，再把计划展示给用户，只有确认后才创建接口或发送请求；
- OpenAPI 文档和已保存接口 Schema 为模型提供真实参数依据，降低 URL、字段和请求体生成错误；
- 返回结果交给规则诊断工具，必要时由模型补充 401、429、500 等错误的排查建议；
- MCP 将这些能力暴露给外部 Agent，形成“外部 Agent 理解问题，Pulse API 执行测试工具”的协作方式。

所以它不是“给 Postman 换一个聊天框”，而是把自然语言理解、Agent 计划、结构化工具调用、接口执行和结果分析连接成了一个可确认、可审计的 AI 测试流程。

### 4. 可读的错误诊断

平台会根据状态码和网络错误给出直接建议：

- `401`：检查 Token、Authorization 或 API Key；
- `403`：当前凭证没有访问权限；
- `404`：检查 URL 和接口版本；
- `429`：检查限流和模型配额；
- `5xx`：检查上游服务和服务端日志；
- 网络错误：检查地址、服务是否启动。

### 5. 历史和回归

保存的请求会变成测试用例，可以一键重放。接口、模型或 Prompt 修改后，重新执行同一用例，就能观察状态码、成功状态和耗时是否出现变化。

### 6. OpenAPI 导入和项目管理

- 导入基础 OpenAPI/Swagger JSON；
- 按项目管理接口；
- 新增 REST、LLM 和多模态接口类型；
- 保留现有 V1 的 PostgreSQL 数据存储、Agent、批量执行和演示接口。

## 系统架构

```text
┌────────────────────────────────────────────┐
│ Vue 3 + TypeScript + Vite                  │
│ 一页式测试台 / 请求编辑 / 响应诊断 / 历史   │
└─────────────────────┬──────────────────────┘
                      │ HTTP JSON
┌─────────────────────▼──────────────────────┐
│ Go API Server                              │
│ 接口管理 / Agent 草稿 / 请求执行 / 错误分类 │
│ 断言检查 / 用例保存 / 回归数据              │
└───────────────┬────────────────────────────┘
                │ HTTP
        ┌───────▼────────┐
        │ 被测 REST / LLM │
        │ SSE / 多模态 API│
        └────────────────┘
                │ SQL
        ┌───────▼────────┐
        │ PostgreSQL      │
        │ 项目 / 接口 / 用例│
        │ 执行历史         │
        └────────────────┘
```

### Agent 与 MCP 架构

```text
┌──────────────────────┐       MCP stdio        ┌─────────────────────┐
│ WorkBuddy / Cursor   │ ─────────────────────▶ │ api-mcp.exe         │
│ “帮我测登录接口”     │                         │ list / draft / test │
└──────────────────────┘                         └──────────┬──────────┘
                                                           │ HTTP JSON
┌──────────────────────┐       HTTP JSON           ┌───────▼───────────┐
│ 浏览器内置 AI 助手    │ ───────────────────────▶ │ Go API Server      │
│ 计划 / 草稿 / 确认    │                          │ Agent + 执行 + 审计│
└──────────────────────┘                          └───────┬───────────┘
                                                          │
                                      ┌───────────────────┼──────────────────┐
                                      ▼                   ▼                  ▼
                                 PostgreSQL          被测 REST/LLM      OpenAPI Schema
```

MCP 不是另一个大模型，也不是把平台复制一份。它只是把平台的能力包装成一组标准工具，使 WorkBuddy、Cursor 或 Claude Desktop 可以调用同一套接口管理和测试能力。

例如 WorkBuddy 连接本项目后，用户可以直接说：

```text
帮我检查本地 Echo 接口；如果接口不存在，就先告诉我准备创建什么。
```

外部 Agent 会先调用 `list_apis` 和 `get_api_schema`，再调用 `create_api_draft`。如果用户确认，才调用 `create_endpoint`；如果用户要求执行测试，才调用 `run_api_test`。所有写入和执行工具都要求显式 `confirm=true`，避免外部 Agent 直接修改数据或发送未知请求。

### MCP 配置示例（WorkBuddy / Cursor 类客户端）

先构建 `backend/api-mcp.exe`，然后在客户端的 MCP 配置中加入：

```json
{
  "mcpServers": {
    "pulse-api": {
      "command": "D:\\课设\\ai-api-test-platform-v2\\backend\\api-mcp.exe",
      "env": {
        "PULSE_API_URL": "http://localhost:8081"
      }
    }
  }
}
```

构建命令：

```powershell
cd D:\课设\ai-api-test-platform-v2\backend
go build -mod=vendor -o api-mcp.exe ./cmd/mcp
```

MCP Server 通过标准输入输出接收 JSON-RPC 消息，不需要额外开放端口；它只连接已经运行的 `http://localhost:8081` Go API。若 WorkBuddy 在 Docker 或其他环境中运行，把 `PULSE_API_URL` 改成该环境可以访问的后端地址即可。

当前提供的工具为：`list_apis`、`get_api_schema`、`create_api_draft`、`create_endpoint`、`run_api_test`、`analyze_response`、`create_regression_case`。其中新增接口、执行测试和保存回归用例是有副作用的工具，必须先由用户确认。

## 技术栈

- 前端：Vue 3、TypeScript、Vite、原生 CSS；
- 后端：Go、标准库 `net/http`；
- 数据库：PostgreSQL；
- Agent：规则型 Agent + 可选 OpenAI 兼容 Tool Calling；
- 部署：Docker、Docker Compose、Nginx。

## 为什么 V2 更方便

V1 把工作台、接口管理、Agent、历史和回归分成多个页面，功能完整但第一次使用需要理解较多概念。V2 将最常用的“选接口、填参数、发送、看结果”合并到同一屏：

```text
左侧：我测哪个接口？
中间：我要发送什么？
右侧：发送结果是什么？
```

高级能力仍然保留在历史、回归和导入入口中，不会干扰第一次测试。

## Docker 启动

确保 Docker Desktop 已启动，然后执行：

```powershell
cd D:\课设\ai-api-test-platform-v2\deploy
docker compose up -d --build
```

浏览器打开：

```text
http://localhost:5174
```

V2 为了和 V1 同时运行，使用独立端口：

- 前端：`5174`；
- 后端：`8081`；
- PostgreSQL：`5433`。

查看状态：

```powershell
docker compose ps
```

停止 V2：

```powershell
docker compose down
```

## 推荐演示流程

1. 打开 `http://localhost:5174`；
2. 左侧选择“回声接口 / Echo”；
3. 点击顶部“AI 助手”，或点击测试台中的“打开助手”；
4. 输入“测试接口是否可用，并检查返回 JSON”，点击“发送给 AI”；
5. 查看助手窗口里的请求草稿、Body 和断言，点击“应用到测试台”；
6. 回到测试台检查 URL、Headers、Body 和断言，再点击“发送并保存”；
7. 在右侧查看 `200`、耗时、断言和 Response Body；
8. 选择“需要鉴权的接口 / Auth”，不填 Token，演示平台识别 `401` 并给出原因；
9. 点击“历史”或“回归”，演示重放测试用例。

## 目录结构

```text
ai-api-test-platform-v2/
├─ README.md
├─ backend/                 # 复用 V1 的 Go API 和 PostgreSQL 逻辑
├─ frontend/
│  ├─ src/main.ts           # V2 一页式工作台和交互
│  ├─ src/style.css         # V2 简洁三栏布局
│  └─ vite.config.ts        # 带运行时模板编译器的 Vue 配置
└─ deploy/
   ├─ docker-compose.yml    # V2 独立端口 5174 / 8081 / 5433
   ├─ backend.Dockerfile
   └─ frontend.Dockerfile
```

## 当前 V2 已完成范围

- 接口搜索、项目管理和 OpenAPI/Swagger JSON 导入；
- URL、请求方法、查询参数、Headers、Body 可见且可编辑；
- `{{baseUrl}}`、`{{token}}` 等环境变量替换；
- Token/API Key 默认脱敏，确认后才显示敏感值；
- 规则型 Agent 自动生成请求草稿、参数和基础断言；
- 独立 AI 助手窗口：自然语言提问、查看草稿、应用到测试台；
- 卡通化 AI 助手入口和文档导入：TXT、Markdown、JSON、YAML、DOCX、带文本层 PDF；
- 单次、批量和并发测试；
- 普通 REST、SSE 流式模型和多模态请求；
- 状态码、响应字段、JSON 契约断言；
- Response Headers、格式化 JSON、错误分类和失败原因建议；
- TTFT、输入/输出 Token、Tokens/s 等模型指标；
- 测试用例编辑、历史重放、多模型配置和回归差异查看；
- 接口链路测试：按顺序执行多个接口，并自动提取、传递 Token、订单号和用户 ID；
- JSON Schema 校验：检查响应类型、必填字段、嵌套对象、数组和数值约束；
- 回归响应差异：除了状态码和耗时，还会标记新增、移除和变化的 JSON 字段；
- PostgreSQL 持久化和 Docker Compose 一键启动。
- Agent 计划层：先列出“读取契约 → 生成草稿 → 确认 → 执行/写入 → 分析”的步骤；
- OpenAPI 操作级 Schema 持久化，规则 Agent 可根据 requestBody 和响应 required 字段补齐示例 Body 与断言；
- Agent 审计记录：保存草稿生成、接口确认、草稿应用和测试执行动作；
- MCP stdio Server：将平台能力暴露为标准工具，并用测试覆盖工具发现、HTTP 转发和确认安全机制。

## 后续增强方向

- 登录后自动提取 Token 并传递给下一个接口（当前已支持链路页内传递）；
- 将链路步骤保存为可复用的工作流模板；
- 引入 OpenAPI Schema Grounding：把当前项目的接口文档、参数说明和响应 Schema 作为 Agent 上下文，降低 URL 和字段生成错误；
- 增加 Agent 操作确认记录、工具调用日志和危险 URL 校验，形成可审计的人机协作流程；
- 使用 Model Context Protocol（MCP）把“读取接口、生成草稿、创建接口、运行测试、分析响应”封装成可复用工具；
- Redis 异步任务、Prometheus、OpenTelemetry；
- Kubernetes、Helm、GitHub Actions 和 Azure 云部署；
- 将当前单轮 Tool Calling 扩展为“计划—确认—执行—结果分析”的多步骤 Agent 工作流。

## 可参考的开源项目

本项目没有直接复制这些项目，而是借鉴它们已经验证过的产品和技术思路：

- [Hoppscotch](https://github.com/hoppscotch/hoppscotch)：学习 API 客户端的请求编辑器、环境变量和集合管理；
- [Insomnia](https://github.com/Kong/insomnia)：学习 REST、SSE、WebSocket、GraphQL 等多协议请求模型；
- [Bruno](https://github.com/usebruno/bruno)：学习 Git 友好的接口集合、脚本和测试用例组织方式；
- [LangGraph](https://github.com/langchain-ai/langgraph)：学习有状态 Agent、人工确认、暂停恢复和多步骤工作流；
- [MCP Servers](https://github.com/modelcontextprotocol/servers) 与 [Go MCP SDK](https://github.com/modelcontextprotocol/go-sdk)：学习把业务能力封装为标准工具，后续让 Agent 能复用接口管理和测试能力；
- [Microsoft Agent Framework](https://github.com/microsoft/agent-framework)：学习生产级 Agent 的编排、工具调用和部署方向。

当前 V2 选择直接在 Go 中实现 Tool Calling，没有强行引入 LangChain、Dify 或 LangGraph，因为单轮“生成草稿—确认—执行”用普通流程更容易理解和演示。后续需要多步骤、可恢复、可审计的工作流时，再引入 LangGraph 或 MCP 更合适。

## Agent 功能升级清单

后续按照下面的优先级逐项实现，不一次性把项目做成难以维护的“大而全”系统。

### P0：当前版本已完成

- 卡通化助手入口和聊天窗口；
- 自然语言生成测试请求草稿；
- 自然语言生成新增接口草稿；
- 用户确认后创建接口或应用测试配置；
- 文档导入和文本提取；
- OpenAI 兼容 Tool Calling；
- 无模型配置时的规则 Agent 兜底。

### P1：优先增强，直接提升实用性（本轮已完成核心项）

1. OpenAPI Schema Grounding：已完成操作级 Schema 保存和规则 Agent 的请求体/响应字段依据；
2. 多工具 Agent：已完成网页端计划工具和 MCP 工具集合；
3. Agent 任务计划：已完成可视化的“计划—确认—执行”步骤；
4. 对话上下文：当前助手保留本次会话消息、当前项目、接口和文档上下文；
5. 自动错误处理建议：已支持 401、403、404、429、500 和网络错误；
6. 测试用例自动生成：已支持从 Schema 生成基础断言，并支持根据项目业务规则生成正常、边界、异常、鉴权和库存用例；
7. 文档增强解析：当前支持 TXT、Markdown、JSON、YAML、DOCX 和带文本层 PDF；文档会按项目切片保存并检索；
8. 操作审计：已完成 Agent 草稿、确认、应用和执行记录。

### P2：前沿 Agent 能力（本轮已完成 MCP 核心，其他能力按阶段扩展）

1. MCP 工具适配器：已完成 stdio Server，可被支持 MCP 的外部 Agent 调用；
2. 状态化工作流：当前用 Go 状态计划实现暂停和确认，后续需要复杂恢复/重试时再替换为 LangGraph；
3. OpenAPI 合同测试：已有 JSON Schema 断言和响应差异，后续补齐更完整的参数类型校验；
4. RAG 文档知识库：已完成项目级文档、分块检索、来源片段和跨项目隔离；后续可接向量数据库；
5. 多模型评测集：已完成多个模型 × 多个问题的并发业务知识评测；后续增加成本统计、人工标注和更严格的 LLM-as-a-Judge；
6. Agent 评测集：后续保存标准任务，统计 URL、方法、参数和断言的生成准确率。

### P3：工程化和云原生扩展

1. Redis + Worker：把批量测试、并发测试和文档解析放入异步任务队列；
2. OpenTelemetry、Prometheus、Grafana：监控 Agent、接口和模型调用链路；
3. SSRF 防护、URL 白名单、密钥隔离和模型输出校验；
4. Kubernetes、Helm、GitHub Actions 和 Azure 部署；
5. 多租户、团队协作、角色权限和项目级凭证管理。

## 依赖说明

- 推荐使用 Docker Desktop 运行完整系统，不需要在本机单独安装 Go 或 PostgreSQL；
- Docker 构建后端时使用 `golang:1.23-alpine`，并使用仓库内的 `backend/vendor` 编译；
- Docker 构建前端时使用 `frontend/package-lock.json` 执行 `npm ci`，保证依赖版本可复现；
- 若只做前端页面开发，需要 Node.js 20 或以上；前端源码位于 `frontend/src`。

## 本轮新增：P0 / P1 实际使用闭环

### 项目级知识库

文档不是“上传给通用聊天机器人”，而是绑定到当前项目的测试上下文。一个项目可以保存：

- OpenAPI / Swagger 接口契约；
- PRD 和业务规则，例如最低金额、库存、退款期限；
- 错误码和认证说明。

每份文档会被切成小片段，Agent 按当前问题检索相关片段，并且查询始终带 `projectId`。因此商城项目的退款规则不会被客服项目的文档混用。当前实现是零额外向量数据库的轻量 RAG，适合课设演示；后续可替换为 pgvector、Qdrant 或 Azure AI Search。

### 根据业务规则生成用例

进入“知识库”，上传 `demo/订单业务规则.md`，选中文档后点击“根据规则生成测试用例”。平台会生成：

```text
正常下单、金额边界、金额非法、未登录下单、库存不足、非法订单状态
```

每条用例都展示请求体、预期状态码和来源文档。生成结果仍需用户确认，之后再应用到测试台执行。

### 多模型并发业务评测

进入“模型评测”，可配置 DeepSeek、Azure OpenAI、OpenAI 或其他 OpenAI 兼容接口。填写一次模型地址、模型名和 API Key 后，选择多个模型和一组问题，平台会按下面的矩阵并发执行：

```text
4 个问题 × 2 个模型 = 8 次评测
```

结果会比较回答、耗时、Token、规则覆盖率、是否引用文档和幻觉风险。这个功能验证的不是“接口能不能通”，而是“模型是否正确遵守当前项目的业务知识”。

### Agent 的真实工作方式

当前 Agent 是“规则 Agent + 可选大模型 Agent”双层设计：

1. 未配置模型时，Go 后端用关键词、接口 Schema 和项目文档做规则检索，生成可演示的草稿和业务测试用例；
2. 配置模型后，Go 后端调用 Chat Completions，并通过 Tool Calling 约束模型只返回结构化的 URL、方法、Headers、Body 和断言；
3. 无论哪种模式，都先展示计划和草稿，用户确认后才创建接口或发送被测请求。

使用 DeepSeek 作为 Agent 模型时，在“配置”页面填写：

```text
Chat Completions 地址：https://api.deepseek.com/chat/completions
模型：deepseek-chat
鉴权 Header：Authorization
API Key：你的 DeepSeek Key
```

如果只需要多模型业务评测，则在“模型评测”页面添加一个或多个模型配置；API Key 由后端保存并且列表只显示脱敏值。

### 新增接口

```text
GET  /api/documents?projectId=1
POST /api/documents                 上传并保存项目文档
DELETE /api/documents?projectId=1&id=1
GET  /api/documents/search?...      项目内检索文档片段
GET  /api/models?projectId=1
POST /api/models?projectId=1        保存模型配置
POST /api/agent/business-cases      根据业务规则生成测试用例
POST /api/evaluations/run           多模型并发业务评测
```

### 无法访问 Docker Hub 时

如果 Docker Desktop 可以运行，但网络暂时无法拉取基础镜像，可先确保本机已有旧版 V2 镜像，然后执行：

```powershell
cd D:\课设\ai-api-test-platform-v2\backend
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -mod=vendor -o api-server-linux ./cmd/server

cd ..\frontend
npm run build

cd ..\deploy
docker build -f backend.offline.Dockerfile -t pulse-api-v2-backend:latest ..
docker build -f frontend.offline.Dockerfile -t pulse-api-v2-frontend:latest ..
docker compose up -d --no-build
```

该方式只用于本机离线验证；联网环境仍使用标准 `backend.Dockerfile` 和 `frontend.Dockerfile`，可以从干净环境重新构建。
