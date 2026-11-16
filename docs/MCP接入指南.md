# MCP接入指南

当前实现为 `backend/cmd/mcp` 的 **stdio MCP适配器**，通过HTTP调用已运行的Pulse平台。WorkBuddy等客户端启动这个本机进程，并用标准输入输出交换JSON-RPC；平台5177/8081并非一个可直接填入的MCP HTTP端点。

## 构建与配置

仅使用浏览器平台不需要Go；构建本机MCP二进制需要Go1.23+。

```sh
cd backend
go build -mod=vendor -o api-mcp.exe ./cmd/mcp
```

Windows示意配置（把路径改为自己的**绝对路径**，其他系统去掉.exe）：

```json
{
  "mcpServers": {
    "pulse-api": {
      "command": "C:/projects/ai-api-test-platform-v2/backend/api-mcp.exe",
      "args": [],
      "env": {
        "PULSE_API_URL": "http://127.0.0.1:5177",
        "PULSE_API_AUTH_USER": "local",
        "PULSE_API_AUTH_PASSWORD": "填写本地.env.acceptance里的平台密码"
      }
    }
  }
}
```

示意不是客户端自动识别的已配置文件；在支持stdio的客户端按其入口添加。密码也可通过 `PULSE_API_AUTH_PASSWORD_FILE` 指向本地私有文件。公网连接用HTTPS；不要提交含真实密码的客户端配置。查询用的projectId先从平台UI或已鉴权 `/api/projects` 获取。

## 已有工具

| 工具 | 能力 |
| --- | --- |
| list_apis / get_api_schema | 查询当前项目接口与参数 |
| import_openapi_file | 从用户明确指定的本地OpenAPI JSON导入，需confirm |
| create_api_draft | 根据需求生成草稿；真实模型失败不规则兜底 |
| create_endpoint | 写入已确认接口配置 |
| run_api_test / create_regression_case | 执行请求、保存用例，需confirm |
| analyze_response | 分析状态码、错误与响应 |
| run_project_connectivity_check | 审核计划或无凭证只读联通性巡检；auto也需confirm |

MCP使用同一平台认证和目标地址访问控制。**无凭证联通巡检不包含自动注册登录、带Token写入或完整16步云盘任务**；当前完整业务验收使用项目测试页或验收脚本，不能把它们当现有MCP工具宣称。

建议首次只测初始化、tools/list、list_apis。再在专用验收项目测试创建草稿和已确认执行；无平台凭据、未确认写入、目标未授权等异常也要测试。客户端连接成功和单元测试通过都不能替代真实工具操作验收。
