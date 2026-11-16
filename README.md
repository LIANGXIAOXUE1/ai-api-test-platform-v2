# Pulse API：基于 AI Agent 的智能 API 测试与评测平台

面向已启动项目的接口冒烟/业务验收，以及按客户任务比较模型输出。支持单接口测试、业务步骤串联、人工审核、保存历史和导出报告。

## 第一次运行：从两个仓库启动完整演示

安装 **Git、Docker Desktop/Engine 和 Compose v2**，Windows启用WSL2/Linux容器，启动Docker引擎。推荐16GB系统内存、Docker可用内存6GB以上和20GB可用磁盘。首次需要访问Docker Hub、npm和Maven；不需要显卡，不必另装Go、Node或Java。

在同一空父目录执行：

```sh
git clone --depth 1 https://github.com/LIANGXIAOXUE1/ai-api-test-platform-v2.git
git clone --depth 1 https://github.com/LIANGXIAOXUE1/cloud-disk.git
cd ai-api-test-platform-v2
```

复制 `.env.acceptance.example` 为 `.env.acceptance`（PowerShell用 `Copy-Item`；macOS/Linux用 `cp`），已有文件不要覆盖，然后：

```sh
docker compose --env-file .env.acceptance -f deploy/docker-compose.acceptance.yml up -d --build
docker compose --env-file .env.acceptance -f deploy/docker-compose.acceptance.yml ps
```

- 浏览器平台：[http://127.0.0.1:5177/](http://127.0.0.1:5177/)。本地演示用户 `local`，密码 `local-demo-password-change-me`，可在env修改。
- 云盘文档：[http://127.0.0.1:8082/v3/api-docs](http://127.0.0.1:8082/v3/api-docs)。云盘根路径没有打包前端，不以其404判后端未启动。
- **项目任务的后台目标用 `http://cloud-disk:8082`**，因为平台后端在容器里。导入组件选择OpenAPI JSON文件，导入前将其servers指向该内部地址。
- 端口冲突改env里的 `PLATFORM_WEB_PORT` / `CLOUD_DISK_PORT`。不要同时运行云盘仓库的独立Compose。
- PowerShell快捷启动：`./scripts/start-local.ps1 -RunSmoke`，可用 `-CloudDiskRoot` 指定另一源码路径。
- 停止保留数据：同一Compose命令改为 `stop`；恢复用 `start`。不要用 `down -v` 清掉历史/密钥。

## 自动冒烟与测试团队验收

```sh
docker compose --env-file .env.acceptance -f deploy/docker-compose.acceptance.yml --profile checks run --rm smoke
```

不调用付费模型，实际检查平台基础HTTP功能及云盘核心16步骤：自动注册→登录提取Token→建目录/重名拒绝→上传/查文件/下载SHA256→改名→软删除→回收站还原/字节校验→未登录拒绝。

结果写 `.runtime/peer-acceptance/<时间-编号>/`，含Markdown和JSON。脚本只用本次创建的数据，不永久删除、不重置数据库。真实模型、页面交互和未覆盖云盘接口另测。

- [测试团队交付清单](docs/测试团队交付清单.md)：统一入口，说明4份配套资料、可选附件、执行顺序和已知问题。
- [完整同伴验收指南](docs/同伴测试验收文档-2026-11-16.md)：依赖、数据库、67项操作/预期、边界、填写标准。
- [同伴实际结果填写表](docs/同伴验收结果填写模板.csv)：空白实际结果，UTF-8 BOM，可用Excel。
- [直接发给AI的启动与测试提示词](docs/AI运行与测试提示词.md)：双仓库、云盘单独运行、内置助手三段。
- [本次发布前验证记录](docs/发布前验证记录-2026-11-16.md)：实际通过与未验证项分别记录。

当前为待验收版本：32项既有基础冒烟通过不等于全平台已上线验收。旧测评生成入口、空/部分usage、跨批排队限制仍需收尾；证据原文定位不等于文档引用认证。详见验收指南DOC-Q01～DOC-Q04。本轮只同步文档，没有修复这些代码问题。

## 两条业务主线

**项目测试**：用户选择已启动系统、测试账号策略和样本文件，预览完整步骤/断言后执行。Token、文件ID和目录ID按依赖传递；错误会定位到步骤，后续依赖标阻塞。保存任务可再次打开，保存历史不用重新执行，报告能导出。HTTP连通与业务断言分别判断，同名目录的预期 `code=409` 可以是通过的负向用例。

**业务场景模型测评**：按客户目标、已知资料、必须满足的要求比较多个真实模型。内置短视频脚本与分镜、企业知识问答、客户信息提取三个文本任务。查看原始回答、检查项、耗时、Token来源、人工意见和历史报告。短视频场景比较的是脚本，不需要视频生成订阅，也不声称验证生成视频质量。

另有测试台单接口请求、环境变量、OpenAPI导入、知识库、模型池、助手和MCP服务。

## Agent 的职责和边界

使用已配置的真实助手模型做语义理解、接口选择、结构化参数/任务草稿。上下文来自当前项目接口、契约、环境变量和用户提供的业务资料；Go执行器负责验证、权限、请求发送、断言与持久化。

- 模型池及助手配置是同一平台实例的公共资源；项目接口、用例、任务和历史按项目隔离。
- 没有有效模型配置或模型调用失败，明确提示配置/错误，**不回退规则Agent**。
- 项目测试里的Cloud Disk预置验收模板是用户明确选择的零模型调用流程，和真实模型规划分开标识。
- 人工审核和任务范围自动执行共用后端权限/目标访问策略；“自动”不等于系统或电脑任意权限。
- 执行证据才算测试结果；模型建议不能当通过证明。接口层验收不替代页面、支付、兼容性、性能和完整安全审计。

真实助手/测评需要测试者自己配置有效模型URL、原始部署名、Key和额度；代码仓库不提供作者的密钥/数据库/测试账号。容器访问真实模型时按验收指南配置精确的 `PLATFORM_ALLOWED_TARGETS`。

## 代码结构与技术

```text
frontend/src/main.ts                    现有测试台、模型池和助手入口
frontend/src/features/project-test/     项目任务配置、执行结果、历史和状态管理
frontend/src/features/evaluation/       业务场景测评、结果比较
frontend/src/features/shared/           测试导航与共享样式
backend/cmd/server/                     net/http入口、适配器、运行安全、兼容接口
backend/internal/projecttest/           任务计划、依赖执行、断言、权限、持久化、报告
backend/internal/evaluation/            场景评测、检查与存储
backend/internal/security/              访问控制、目标URL/DNS/IP/重定向校验
backend/internal/secrets/               模型/任务敏感数据加密
backend/cmd/mcp/                        stdio MCP适配器
deploy/                                开发/验收/生产Compose和镜像构建
scripts/                               启动、验收、备份恢复
docs/                                  验收、架构和发布说明
```

前端Vue3、TypeScript、Vite及项目自有CSS；后端Go `net/http`、pgx；平台PostgreSQL16（`api_test`），被测云盘MySQL8（`private_aggregator`）+Redis7+RabbitMQ3。使用OpenAPI JSON、OpenAI兼容模型调用和stdio MCP。当前后端入口仍保留兼容功能，新增业务模块已拆入internal，前端新增业务拆入features。

## 本机源码检查（可选）

本机开发需要Go1.23+、Node22.12+；云盘源码需JDK17/Maven3.9。Docker验收流程不要求主机安装这些工具。

```sh
cd backend
go test ./...
cd ../frontend
npm ci
npm run test:project-test
npm run build
```

原生Vite开发入口可使用 `npm run dev -- --port 5177`；后端、数据库、模型和目标地址仍须正确配置。不要把只启动前端当完整项目已运行。

## MCP 与生产部署

- [MCP接入指南](docs/MCP接入指南.md)：stdio工具、外部客户端配置及权限范围。当前MCP联通巡检不等于已暴露完整16步业务任务控制。
- [五项修复与生产上线说明](docs/五项修复与生产上线说明-2026-11-12.md)：生产认证、目标授权、密钥、备份恢复、Caddy/Nginx。
- [技术点与答辩优势](docs/技术点与答辩优势-2026-11-11.md)。

默认验收Compose仅发布127.0.0.1端口，演示密码用于本地测试。公网使用独立 `deploy/docker-compose.production.yml`、自有域名/TLS、独立密钥及访问控制；验收通过后再按生产文档部署。
