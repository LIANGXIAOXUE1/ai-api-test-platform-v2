# Cloud Disk 接口测试操作手册

> 用途：使用 AI API 智能测试与回归分析平台，对 Cloud Disk 的真实后端进行接口测试与回归演示。

## 1. 是否必须先启动 Cloud Disk？

**要发送真实请求并获得真实结果，必须先启动 Cloud Disk 后端。**

在 AI API 测试平台中，你可以提前导入接口文档、编辑请求、保存测试用例；但后端未运行时，点击发送只会得到“连接失败”。

Cloud Disk 仓库自带 Docker Compose，后端依赖 MySQL、Redis 和 RabbitMQ。最适合演示的启动方式是在 Cloud Disk 仓库根目录执行：

```bash
docker compose up -d --build
```

启动完成后，后端地址为：

```text
http://localhost:8080
```

检查服务是否已启动：

```text
http://localhost:8080/v3/api-docs
```

能打开或返回 JSON，说明后端已启动。Cloud Disk 自己的前端不是接口测试必需条件；只测 API 时启动后端及其依赖即可。

> 注意：首次启动会拉取数据库、缓存和消息队列镜像，耗时会更长。开始前先确认 Docker Desktop 正在运行，且本机 8080、3306、6379、5672 端口未被占用。

## 2. 导入接口到测试平台

1. 打开 AI API 智能测试与回归分析平台；
2. 新建项目，名称填“Cloud Disk 接口测试”；
3. 在左侧“接口”区域点击“导入 OpenAPI”；
4. 导入地址下载得到的 OpenAPI JSON，或在浏览器打开 `http://localhost:8080/v3/api-docs` 后保存为 JSON 文件；
5. 设置环境变量：

```text
baseUrl: http://localhost:8080
token: 登录成功后复制的 Token
```

6. 接口 URL 写为 `{{baseUrl}}/api/...`；需要鉴权的接口增加 Header：

```text
cloud_disk_token: {{token}}
```

## 3. 推荐演示顺序

### 第一步：注册测试用户

```http
POST {{baseUrl}}/api/auth/register
Content-Type: application/json

{
  "username": "apitest_demo",
  "password": "Test123456",
  "nickname": "接口测试用户",
  "email": "apitest_demo@example.com"
}
```

断言：

- 状态码等于 `200`；
- JSON 存在字段：`code,message`；
- 响应包含文本：`success`。

### 第二步：登录并保存 Token

```http
POST {{baseUrl}}/api/auth/login
Content-Type: application/json

{
  "username": "apitest_demo",
  "password": "Test123456"
}
```

断言：

- 状态码等于 `200`；
- JSON 存在字段：`data.token,data.user`。

从响应的 `data.token` 复制 Token，填写到平台环境变量 `token`。后续受保护接口统一使用 `cloud_disk_token: {{token}}`。

### 第三步：验证鉴权与文件列表

正常用例：

```http
GET {{baseUrl}}/api/file/list?page=1&pageSize=20
cloud_disk_token: {{token}}
```

异常用例：删除 `cloud_disk_token` 后再发送一次。预期为鉴权失败，平台应识别为“Token 缺失、无效或已过期”。

### 第四步：文件夹重名规则

```http
POST {{baseUrl}}/api/file/createFolder?folderName=API-Test
cloud_disk_token: {{token}}
```

第一次期望成功；不修改参数再次发送，期望 `409` 且包含“同名文件夹”。这是一条非常直观的回归用例。

### 第五步：分片上传链路

此场景可在“链路测试”中配置：

1. `POST /api/file/chunk/init`：创建上传任务；
2. 从响应提取 `data.uploadId` 为变量 `uploadId`；
3. `POST /api/file/chunk/upload`：携带 `uploadId`、`chunkIndex` 和一个文件分片；
4. `GET /api/file/chunk/progress?uploadId={{uploadId}}`：查询上传进度；
5. `POST /api/file/chunk/merge`：Body 使用 `{"uploadId":"{{uploadId}}"}`。

该链路适合展示平台的“自动提取并传递上下文变量”能力。

### 第六步：删除与恢复链路

1. 上传或准备一个测试文件；
2. `POST /api/file/delete?fileId=...`；
3. `GET /api/recycle/list`，确认记录存在；
4. `POST /api/recycle/restore?recycleId=...`；
5. 再调用文件列表，确认文件恢复。

## 4. 推荐保存为回归用例的接口

| 用例名称 | 接口 | 重点断言 | 演示价值 |
| --- | --- | --- | --- |
| 登录成功 | `POST /api/auth/login` | `200`、存在 `data.token` | 自动提取 Token |
| 未登录访问文件 | `GET /api/file/list` | 鉴权失败 | 自动分析 401 |
| 同名文件夹 | `POST /api/file/createFolder` | `409`、包含同名提示 | 业务规则回归 |
| 分片上传初始化 | `POST /api/file/chunk/init` | 返回 `data.uploadId` | 上下文变量 |
| 文件流预览不存在 | `GET /api/file/stream/{fileId}` | `404` | 错误分类 |
| 默认存储不可删 | `DELETE /api/storage/binding/{id}` | 拒绝删除 | 业务约束 |

## 5. 知识库上传建议

上传顺序：

1. `01-CloudDisk-业务规则.md`：文档类型选择“业务规则”；
2. 本文档：文档类型选择“接口契约”；
3. 需要时再上传 Cloud Disk 的 `README.md` 或 `PROJECT_ARCH.md`：文档类型选择“PRD”或“接口契约”。

之后点击“根据规则生成测试用例”，系统应生成：注册校验、鉴权、重名冲突、分片上传、回收站恢复、分享提取码等测试场景。

## 6. 不能启动时怎么办？

即使 Cloud Disk 尚未启动，你仍然可以：

- 上传业务规则文档；
- 生成并查看测试用例；
- 导入或手动配置接口；
- 演示请求草稿、断言与回归用例。

但不能展示真实的 200、401、409、响应耗时或回归差异。要做完整演示，建议启动 Cloud Disk Docker 环境后再连接本平台。
