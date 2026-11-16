package projecttest

import "fmt"

// This is an explicitly selected, versioned acceptance template, not an Agent fallback.
// Paths and expectations follow the Cloud Disk contract; AI plans use the same runner.
func CloudDiskPlan(task Task) []TaskStep {
	steps := []TaskStep{}
	add := func(id, name, method, path string, deps []string, query map[string]string, body string) *TaskStep {
		steps = append(steps, TaskStep{ID: id, Name: name, Method: method, URL: path, DependsOn: deps, Query: query, Body: body,
			Headers: map[string]string{"cloud_disk_token": "{{token}}"}, Assertions: []TaskAssertion{{Type: "status", Expected: "200"}, {Type: "json_equals", Path: "code", Expected: "200"}}})
		return &steps[len(steps)-1]
	}
	if task.AccountMode != "existing" {
		x := add("register", "注册本次测试账号", "POST", "/api/auth/register", nil, nil, `{"username":"{{username}}","password":"{{password}}","nickname":"{{username}}","email":"{{username}}@example.com"}`)
		x.Headers = map[string]string{}
	}
	dep := []string{}
	if task.AccountMode != "existing" {
		dep = []string{"register"}
	}
	x := add("login", "登录并提取 Token", "POST", "/api/auth/login", dep, nil, `{"username":"{{username}}","password":"{{password}}"}`)
	x.Headers = map[string]string{}
	x.Extract = []TaskExtract{{Name: "token", Path: "data.token"}}
	has := func(feature string) bool {
		for _, f := range task.Features {
			if f == feature {
				return true
			}
		}
		return false
	}
	folder := has("folder") || has("file") || has("lifecycle")
	if folder {
		add("folder", "创建本次测试文件夹", "POST", "/api/file/createFolder", []string{"login"}, map[string]string{"folderName": "{{folderName}}"}, "")
		x = add("folder_lookup", "查询并取得测试目录 ID", "GET", "/api/file/list", []string{"folder"}, map[string]string{"pageSize": "100"}, "")
		x.Extract = []TaskExtract{{Name: "folderId", Path: "data.list", MatchField: "fileName", MatchValue: "{{folderName}}", ValueField: "id", Owned: true}}
		if has("folder") {
			x = add("duplicate", "同目录同名文件夹应被拒绝", "POST", "/api/file/createFolder", []string{"folder"}, map[string]string{"folderName": "{{folderName}}"}, "")
			x.Assertions[1].Expected = "409"
		}
	}
	if has("file") || has("lifecycle") {
		x = add("upload", "上传测试文件", "POST", "/api/file/upload", []string{"folder_lookup"}, map[string]string{"parentId": "{{folderId}}"}, "")
		x.Upload = true
		x.FileField = "file"
		x.Guard = "folderId"
		x.Extract = []TaskExtract{{Name: "fileId", Path: "data.id", Owned: true}}
		x = add("file_lookup", "查询上传文件及大小", "GET", "/api/file/list", []string{"upload"}, map[string]string{"parentId": "{{folderId}}", "pageSize": "100"}, "")
		x.Assertions = append(x.Assertions, TaskAssertion{Type: "array_contains", Path: "data.list", Field: "id", Expected: "{{fileId}}"}, TaskAssertion{Type: "array_contains", Path: "data.list", Field: "fileSize", Expected: "{{fileSize}}", MatchField: "id", MatchValue: "{{fileId}}"})
		x = add("download", "下载并校验原始内容 SHA256", "GET", "/api/file/stream/{{fileId}}", []string{"file_lookup"}, nil, "")
		x.Assertions = []TaskAssertion{{Type: "status", Expected: "200"}, {Type: "binary_sha256", Expected: "{{fileHash}}"}}
	}
	if has("lifecycle") {
		x = add("rename", "重命名本次文件", "POST", "/api/file/rename", []string{"download"}, map[string]string{"fileId": "{{fileId}}", "newName": "{{renamedFile}}"}, "")
		x.Guard = "fileId"
		x = add("rename_check", "核对重命名结果", "GET", "/api/file/list", []string{"rename"}, map[string]string{"parentId": "{{folderId}}"}, "")
		x.Assertions = append(x.Assertions, TaskAssertion{Type: "array_contains", Path: "data.list", Field: "fileName", Expected: "{{renamedFile}}"})
		x = add("delete", "将本次测试文件移入回收站", "POST", "/api/file/delete", []string{"rename_check"}, map[string]string{"fileId": "{{fileId}}"}, "")
		x.Guard = "fileId"
		x = add("delete_check", "核对文件已从目录移除", "GET", "/api/file/list", []string{"delete"}, map[string]string{"parentId": "{{folderId}}"}, "")
		x.Assertions = append(x.Assertions, TaskAssertion{Type: "array_absent", Path: "data.list", Field: "id", Expected: "{{fileId}}"})
		x = add("recycle", "定位本次测试文件的回收记录", "GET", "/api/recycle/list", []string{"delete_check"}, nil, "")
		x.Extract = []TaskExtract{{Name: "recycleId", Path: "data.list", MatchField: "fileId", MatchValue: "{{fileId}}", ValueField: "id", Owned: true}}
		x = add("restore", "还原本次测试文件", "POST", "/api/recycle/restore", []string{"recycle"}, map[string]string{"recycleId": "{{recycleId}}"}, "")
		x.Guard = "recycleId"
		x = add("restore_check", "核对还原后内容一致", "GET", "/api/file/stream/{{fileId}}", []string{"restore"}, nil, "")
		x.Assertions = []TaskAssertion{{Type: "status", Expected: "200"}, {Type: "binary_sha256", Expected: "{{fileHash}}"}}
	}
	if has("auth") {
		x = add("unauthorized", "未登录访问必须被拒绝", "GET", "/api/file/list", nil, nil, "")
		x.Headers = map[string]string{}
		x.Assertions = []TaskAssertion{{Type: "auth_rejected", Expected: "401/403 或明确未登录业务响应"}}
	}
	return steps
}

func ValidateTask(task Task) error {
	if task.ProjectID <= 0 || task.Name == "" {
		return fmt.Errorf("请选择项目并填写任务名称")
	}
	if _, err := normalizeCheckBaseURL(task.BaseURL); err != nil {
		return err
	}
	if task.AccountMode != "auto" && task.AccountMode != "register" && task.AccountMode != "existing" {
		return fmt.Errorf("请选择测试账号来源")
	}
	if task.FileMode != "auto" && task.FileMode != "manual" {
		return fmt.Errorf("请选择测试文件来源")
	}
	if task.Permission != "review" && task.Permission != "auto" {
		return fmt.Errorf("请选择执行方式")
	}
	if len(task.Steps) > 80 {
		return fmt.Errorf("单任务最多 80 个步骤")
	}
	return nil
}
