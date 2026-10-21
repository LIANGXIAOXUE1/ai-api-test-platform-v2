# Cloud Disk 业务规则与测试依据

> 用途：上传至 AI API 测试平台的“业务规则”知识库，用于自动生成正常、边界、异常与鉴权测试用例。  
> 来源：Cloud Disk 项目的 README、控制器、DTO 和 OpenAPI 注解；规则以仓库 `main` 分支为准。  
> 适用范围：用户认证、文件管理、分片上传、回收站、分享与存储绑定。

## 1. 统一约定

- 后端 API 默认地址为 `http://localhost:8080`。
- 除注册与登录外，`/api/**` 接口需要已登录用户身份；未登录请求应被拒绝。
- 登录成功后返回 Token；后续请求需要在 Header 中携带 `cloud_disk_token: <token>`。
- 常规接口返回统一结构：`code`、`message`、`data`。成功时 `code=200`、`message=success`。
- 同一测试项目中的数据应相互隔离：文件、回收站记录、存储绑定均只能由当前登录用户访问和修改。

## 2. 用户认证规则

### BR-AUTH-01 用户注册必填字段

注册接口为 `POST /api/auth/register`。请求体必须包含：

- `username`：不能为空；
- `password`：不能为空，长度为 6 至 32 位；
- `nickname`：不能为空；
- `email`：若提供，必须是合法邮箱格式。

### BR-AUTH-02 用户名唯一

已存在的用户名不能再次注册。重复注册应返回业务错误，且不得新增第二个同名用户。

### BR-AUTH-03 登录与 Token

`POST /api/auth/login` 仅接受非空的 `username` 和 `password`。登录成功应在 `data.token` 返回可用 Token，并在 `data.user` 返回用户基本信息；返回对象不得包含密码。

### BR-AUTH-04 受保护接口

未携带 Token、Token 无效或 Token 过期时，访问文件、回收站、存储管理等受保护接口应失败；平台可将该结果归类为“鉴权失败”。

### BR-AUTH-05 密码修改

`POST /api/auth/changePassword` 仅允许当前登录用户操作，需要 `oldPassword` 与 `newPassword`。原密码不正确或新密码不符合规则时，不得修改原密码。

## 3. 文件与文件夹规则

### BR-FILE-01 文件列表分页

`GET /api/file/list` 支持：

- `parentId`：可选；缺省时查询根目录；
- `page`：从 1 开始，默认 1；
- `pageSize`：默认 20。

响应中的列表与总数应只属于当前登录用户。

### BR-FILE-02 创建文件夹不能重名

`POST /api/file/createFolder` 使用 `folderName` 和可选 `parentId` 创建文件夹。同一父目录下存在同名文件夹时，应返回 `409`，消息为“目标位置已存在同名文件夹”。

### BR-FILE-03 所有者才能操作文件

重命名、移动、复制、删除、获取下载地址、获取预览地址等文件操作必须校验文件归属。用户不能通过猜测 `fileId` 操作其他用户的文件。

### BR-FILE-04 重命名与移动冲突

- 重命名为同一目录已存在的名称时，应失败并返回 `409`；
- 移动到目标目录后若存在同名文件，也应失败并返回 `409`；
- 操作成功后返回 `data.result = "OK"`。

### BR-FILE-05 删除进入回收站

`POST /api/file/delete` 不应直接物理删除文件，而是将文件移入回收站并创建回收站记录。删除后文件不应继续出现在正常文件列表中，但应可在回收站查询到。

### BR-FILE-06 搜索只返回自己的文件

`GET /api/file/search?fileName=关键词` 按文件名模糊查询，只返回当前用户拥有的匹配文件。

### BR-FILE-07 预览与流式读取

`GET /api/file/stream/{fileId}`：

- 文件不存在、文件是文件夹或磁盘上的文件不存在时，应返回 `404`；
- 普通文件应按文件扩展名返回合理的 `Content-Type`；
- 请求文件夹 ID 不得作为文件流返回。

## 4. 上传与分片规则

### BR-UPLOAD-01 普通上传

`POST /api/file/upload` 使用 `multipart/form-data`，字段名必须为 `file`，`parentId` 可选。上传成功后应返回新建文件信息。

### BR-UPLOAD-02 分片初始化

`POST /api/file/chunk/init` 的 JSON 请求体必须包含：

- `fileName`：非空，带扩展名；
- `fileSize`：非空，单位为字节；
- `totalChunks`：非空；
- `parentId`：可选。

初始化成功应返回唯一 `uploadId`，该 ID 是后续上传分片、查询进度、合并分片的上下文变量。

### BR-UPLOAD-03 分片序号与断点续传

`POST /api/file/chunk/upload` 需要 `uploadId`、从 0 开始的 `chunkIndex` 以及分片文件 `file`。已上传的分片可以通过 `GET /api/file/chunk/progress?uploadId=...` 查询；重复上传同一分片应安全处理，不应损坏上传任务。

### BR-UPLOAD-04 合并必须依赖有效任务

`POST /api/file/chunk/merge` 的请求体只接受非空 `uploadId`。不存在、过期或未完成的上传任务不能生成完整文件；成功合并后应生成文件记录，并清理临时分片。

## 5. 回收站规则

### BR-RECYCLE-01 回收站隔离

`GET /api/recycle/list` 仅显示当前登录用户的回收站记录，并返回 `list` 与 `total`。

### BR-RECYCLE-02 恢复文件

`POST /api/recycle/restore?recycleId=...` 成功后，文件应重新回到正常文件列表，并从回收站移除或更新状态。

### BR-RECYCLE-03 彻底删除不可恢复

`POST /api/recycle/permanentDelete?recycleId=...` 成功后，文件及其回收站记录不应再被恢复。

### BR-RECYCLE-04 清空回收站

`POST /api/recycle/clear` 只清空当前登录用户的回收站，不得影响其他用户记录。

## 6. 分享与存储规则

### BR-SHARE-01 分享校验

创建分享时可设置提取码和过期策略。提取码错误时，`POST /api/share/verify` 应返回 `INVALID`；正确时返回 `OK`。

### BR-SHARE-02 过期分享不可访问

过期分享通过 `POST /api/share/access` 访问时应返回 `EXPIRED`；有效分享返回 `OK`。

### BR-STORAGE-01 默认本地存储不能删除

`GET /api/storage/bindings` 在没有额外绑定时应提供默认本地磁盘。`DELETE /api/storage/binding/{id}` 不允许删除默认 `LOCAL` 存储。

### BR-STORAGE-02 新存储先测连接

新增 MinIO 或阿里云 OSS 等存储前，应先使用 `POST /api/storage/test` 校验配置；连接失败时返回 `400`，成功时返回 `data.result = "OK"`。

## 7. AI 文件能力规则

- `POST /api/file/summary/{fileId}`：文件不存在或传入文件夹时返回 `404`；分析失败时返回 `500`。
- `POST /api/file/describe/{fileId}`：图片描述或 OCR 成功后应保存 AI 笔记。
- `POST /api/file/video-note/{fileId}`：视频/音频分析成功后应保存 AI 笔记。
- `GET /api/file/note/{fileId}`：只返回当前文件对应的 AI 笔记。

## 8. 推荐由平台自动生成的测试场景

1. 注册正常用例、短密码、空用户名、重复用户名；
2. 未携带 Token 访问文件列表，期望鉴权失败；
3. 创建文件夹后再次创建同名文件夹，期望 `409`；
4. 上传分片 → 查询进度 → 合并分片的链路测试；
5. 删除文件 → 回收站存在 → 恢复文件的链路测试；
6. 用非所有者 Token 操作文件，期望拒绝；
7. 传入不存在 `fileId` 调用预览、AI 摘要，期望 `404`；
8. 错误提取码访问分享，期望 `INVALID`。
