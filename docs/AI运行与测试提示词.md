# AI 运行与测试提示词

统一修订：2026-11-16。与同日验收指南、83项CSV及测试团队交付清单配套；已更新平台登录页、用户角色、项目授权、自动执行授权和配套验收项。先读取验收指南“当前版本状态与已知问题”，不要把旧方案或历史演示当作当前版已验收结果。

## 一、把下面整段发给有终端权限的 AI（推荐）

```text
请协助我从源码运行并验收这个项目：
平台仓库 https://github.com/LIANGXIAOXUE1/ai-api-test-platform-v2
云盘仓库 https://github.com/LIANGXIAOXUE1/cloud-disk

目标是验证完成项目后的接口主链路和平台功能。只在我电脑本地启动，不需要部署服务器。
先读取平台 docs/测试团队交付清单.md、两个仓库的 README、平台 docs/同伴测试验收文档-2026-11-16.md、
docs/同伴验收结果填写模板.csv 和 deploy/docker-compose.acceptance.yml。

1. 检查 Git、Docker Desktop/引擎、Compose v2；Windows 检查 WSL2/虚拟化。
   容器方案不要求主机安装 Go、Node、Java、Maven 或 Python。缺少软件时说明需安装什么；
   安装需要我操作的部分请明确提示。不要把静态阅读代码当成已经运行。
2. 在一个我可写的空父目录，将两个仓库克隆成 ai-api-test-platform-v2、cloud-disk 两个相邻文件夹。
   如果已经克隆，先检查未提交工作和分支，不覆盖我的改动。记录两个 git rev-parse HEAD。
3. 在平台根目录复制 .env.acceptance.example 为 .env.acceptance（已存在则保留）。
   检查 5177/8082 端口；冲突时使用 PLATFORM_WEB_PORT/CLOUD_DISK_PORT 改映射，不终止其他项目。
   执行 docker compose --env-file .env.acceptance -f deploy/docker-compose.acceptance.yml up -d --build。
   用 ps/日志/真实HTTP检查确认平台和云盘就绪。首次拉镜像需要网络，失败请检查Docker代理，不能虚报成功。
4. 平台浏览器默认 http://127.0.0.1:5177，在平台内登录表单填写本地env账号，不需要注册；空库先由env创建管理员，随后在用户与权限分配测试员/只读账号与项目，云盘账号另填；
   云盘文档 http://127.0.0.1:8082/v3/api-docs。
   平台后端在容器里，所以项目任务的 baseUrl 和 OpenAPI地址使用 http://cloud-disk:8082；
   不把容器 localhost 误当主机。网页导入是JSON文件：下载文档并改servers[0].url后导入。
5. 执行零模型费用冒烟：
   docker compose --env-file .env.acceptance -f deploy/docker-compose.acceptance.yml --profile checks run --rm smoke
   打开 .runtime/peer-acceptance 最新目录的 acceptance.md/acceptance.json 和云盘报告。
   核心链路自动注册唯一测试账号，登录提取token，目录重名验证，上传下载SHA256，改名，软删除/还原及鉴权。
6. 对照83项验收CSV补测页面、历史、下载、重启、权限和异常。
   DOC-Q01～DOC-Q04是已知收尾/边界项，按EV-07～EV-14实际核对；没有修好时如实记录问题。
   这次任务是运行与测试，不擅自修改评分标准、删除旧证据或修代码；修复应在另行授权后重测。原CSV是空白模板，另存一份我的执行结果。
   如我提供真实模型Key，再验收助手语义与3类业务模型测评；Key只存本地，不发到报告、聊天、Git。
   没有Key、余额、第三方服务或浏览器控制权限时，对应项目标BLOCKED/待执行，不能填通过。
7. 只创建本次专用测试项目/账号/目录/文件；写入仅限这次已授权测试范围。
   不碰已有业务数据，不永久删除，不清空数据库/卷，不调用真实支付或未授权付费模型。
   若要更改范围或调用付费模型，先说明具体测试和预计调用量。
8. 最后给我实际版本、地址、依赖状态、PASS/FAIL/BLOCKED/未执行数、失败原因、报告路径，
   并说明接口验收不等于云盘页面、全部接口、RabbitMQ消费链路、外部存储和安全/性能专项都通过。
   “原文片段可定位”不代表文档引用或事实已自动认证；未知Token不得当精确费用。
```

## 二、只运行云盘作为被测项目

```text
云盘仓库是 https://github.com/LIANGXIAOXUE1/cloud-disk。
请阅读README，在本机克隆并用 docker compose up -d --build 从源码运行。
检查Git、Docker/Compose，不依赖作者本机JAR或数据库。
该项目用Java17/Spring Boot3、MySQL8数据库private_aggregator、Redis7、RabbitMQ3。
首次空数据卷自动执行sql/init.sql；不用作者账号，创建唯一测试账号再登录。
登录 POST /api/auth/login 从data.token提取，后续带cloud_disk_token请求头。
真实检查 http://127.0.0.1:8082/v3/api-docs，不以根路径404判启动失败。
如果同时测试Pulse平台，请改用平台仓库的一体化验收Compose，不同时启动两套云盘抢占8082。
请验证注册→登录→建目录→重复同名拒绝→上传→查询→下载字节一致→改名→软删除→回收站→还原→未登录拒绝。
只用本次创建的资源，禁止永久删除/清库；逐步记录请求、业务码和断言，Token/密码脱敏。
没有实际执行的步骤标未执行，失败或依赖缺失标FAIL/BLOCKED。
生成验收Markdown与JSON，说明这覆盖核心链路而非全部云盘功能。
```

## 三、发给平台内置助手（需先配置真实助手模型）

```text
当前项目是Cloud Disk验收。系统已启动，平台后端可访问 http://cloud-disk:8082，
OpenAPI在 http://cloud-disk:8082/v3/api-docs。
我要验证交付前核心业务链路：注册、登录、目录创建与同名拒绝、文件上传下载校验、重命名、软删除、回收站还原、未登录拒绝。
账号选择自动生成本次新测试账号；文件选择自动生成样本，不使用我的业务文件。
先生成可审核的任务，把请求参数、步骤依赖、Token/资源ID提取、业务断言列出来；
缺失资料时问我，不编造接口或业务成功。审核确认后只执行本次资源并保存任务和历史，导出报告。
如果当前助手只能给草稿，请明确指出需要我到“测试与测评→项目测试”确认执行的位置。
```

平台内置助手不能替同伴电脑安装依赖或克隆仓库；启动工作发给有本地终端能力的 AI。
没有模型Key时用项目测试页的 **Cloud Disk预置验收模板**；它是明确选择的流程，不能冒称模型规划。


## 四、用户与权限专项

请在专用测试环境完成RBAC-01～RBAC-12。由管理员创建测试员、只读及无项目账号，使用隔离会话验证不同角色，不使用作者个人账号或模型Key。公共模型池仍互通，只有管理员改公共配置；模型质量专项需另有有效Key和额度。自动执行权限与项目成员分别授权。跨项目、错误资源编号、批量/链路混入其他项目必须在发送前拒绝。用户密码首次空库来自env，已有库请在平台改密码；不要清库或覆写env来恢复密码。检查改密码、禁用、权限变更后旧会话失效，以及重启后授权/报告保留。测试结束禁用本次测试账号，保留报告证据。
