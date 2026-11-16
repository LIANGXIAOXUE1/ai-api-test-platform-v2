"""Zero-cost HTTP acceptance against an explicitly started local test stack.

Creates a new platform project and isolated Cloud Disk test resources. No real
model request, permanent deletion, database reset, or credential export.
"""
import argparse
import base64
import datetime as dt
import json
import os
from pathlib import Path
import sys
import time
import urllib.error
import urllib.request
import uuid


class Blocked(Exception):
    pass


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-url', default='http://127.0.0.1:5177')
    parser.add_argument('--cloud-base', default='http://cloud-disk:8082', help='URL reached by platform backend')
    parser.add_argument('--cloud-doc-url', default='http://127.0.0.1:8082/v3/api-docs', help='URL reached by this runner')
    parser.add_argument('--output', default='.runtime/peer-acceptance')
    parser.add_argument('--env-file', help='Optional local acceptance env file; values are never printed')
    args = parser.parse_args()
    config = dict(os.environ)
    if args.env_file:
        for line in Path(args.env_file).read_text(encoding='utf-8-sig').splitlines():
            if '=' in line and not line.lstrip().startswith('#'):
                key, value = line.split('=', 1)
                config[key.strip()] = value.strip().strip('\"\'')
    user = config.get('PLATFORM_AUTH_USER', 'local')
    password = config.get('PLATFORM_AUTH_PASSWORD', 'local-demo-password-change-me')
    authorization = 'Basic ' + base64.b64encode((user + ':' + password).encode()).decode()
    output = Path(args.output) / (dt.datetime.now().strftime('%Y%m%d-%H%M%S') + '-' + uuid.uuid4().hex[:6])
    output.mkdir(parents=True, exist_ok=False)
    results, state = [], {}

    def request(path, method='GET', payload=None, auth=True, expected=(200, 201, 202, 204), raw=False, absolute=False):
        url = path if absolute else args.base_url.rstrip('/') + path
        body = json.dumps(payload, ensure_ascii=False).encode() if payload is not None else None
        headers = {'Content-Type': 'application/json'} if body is not None else {}
        if auth:
            headers['Authorization'] = authorization
        req = urllib.request.Request(url, body, headers, method=method)
        try:
            response = urllib.request.urlopen(req, timeout=60)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            data = response.read()
            if response.code not in expected:
                # Only platform errors are included; never print request headers.
                message = data.decode('utf-8', errors='replace')[:700]
                message = message.replace(password, '[redacted]')
                raise AssertionError(f'HTTP {response.code}: {message}')
            if raw:
                return data
            return json.loads(data) if data else None

    def require(condition, message):
        if not condition:
            raise AssertionError(message)

    def check(identifier, name, action):
        started = time.monotonic()
        try:
            detail = action()
            status = 'PASS'
        except Blocked as error:
            detail, status = str(error), 'BLOCKED'
        except Exception as error:
            detail, status = str(error).replace(password, '[redacted]'), 'FAIL'
        results.append({'id': identifier, 'name': name, 'status': status, 'actual': detail or '符合预期', 'durationMs': int((time.monotonic()-started)*1000)})
        print(f'{identifier} {status}: {name}', flush=True)

    def project():
        if 'project' not in state:
            raise Blocked('平台项目创建失败，后续写入未执行')
        return state['project']

    def create_project():
        p = request('/api/projects', 'POST', {'name': '同伴验收-' + uuid.uuid4().hex[:10], 'summary': '独立测试数据，不调用付费模型'})
        state['project'] = p['id']
        return f'projectId={p["id"]}'

    def create_endpoint():
        p = request('/api/apis', 'POST', {'projectId': project(), 'name': 'Peer Echo', 'method': 'POST', 'url': '/api/demo/echo', 'kind': 'REST', 'headers': {'Content-Type': 'application/json'}, 'query': {}, 'body': '{"message":"peer acceptance"}', 'enabled': True})
        state['api'] = p['id']
        return f'apiId={p["id"]}'

    def run_request(url, assertions, body='', headers=None, method='GET', save=False):
        return request('/api/runs', 'POST', {'projectId': project(), 'apiId': state.get('api', 0), 'name': 'Peer HTTP acceptance', 'saveCase': save, 'request': {'method': method, 'url': url, 'headers': headers or {}, 'query': {}, 'body': body}, 'assertions': assertions})

    def echo():
        run = run_request('/api/demo/echo', [{'type': 'status', 'expected': '200'}, {'type': 'json_value', 'path': 'received.message', 'expected': 'peer acceptance'}], '{"message":"peer acceptance"}', method='POST', save=True)
        require(run['success'], 'Echo或业务断言失败')
        state['run'], state['case'] = run['id'], run['caseId']
        return f'runId={run["id"]}; caseId={run["caseId"]}'

    def business_failure():
        run = run_request('/api/demo/echo', [{'type': 'json_value', 'path': 'received.message', 'expected': 'different'}], '{"message":"peer acceptance"}', method='POST')
        require(run['statusCode'] == 200 and not run['success'] and run['errorCategory'] == '断言失败', '未正确区分连通与业务通过')

    def auth_negative():
        run = run_request('/api/demo/auth', [{'type': 'status', 'expected': '401'}])
        require(run['success'] and run['statusCode'] == 401, '预期401未判通过')

    def auth_positive():
        run = run_request('/api/demo/auth', [{'type': 'status', 'expected': '200'}, {'type': 'json_field', 'path': 'userId'}], headers={'Authorization': 'Bearer demo-token'})
        require(run['success'], '正确Token未通过')

    def history():
        if 'run' not in state:
            raise Blocked('无成功Echo记录')
        runs = request('/api/runs?projectId=' + str(project()))
        cases = request('/api/cases?projectId=' + str(project()))
        require(any(r['id'] == state['run'] for r in runs) and any(c['id'] == state['case'] for c in cases), '用例或历史读回失败')

    def deny_metadata():
        run = run_request('http://169.254.169.254/latest/meta-data/', [])
        require(not run['success'] and run['errorCategory'] == '目标地址未授权', '元数据目标未被访问策略拒绝')

    def model_crud():
        model = request('/api/models?projectId=' + str(project()), 'POST', {'projectId': project(), 'name': 'Peer dummy model', 'provider': '测试桩', 'model': 'peer-exact-deployment', 'baseUrl': 'https://models.example.invalid/v1/chat/completions', 'apiKey': 'peer-dummy-not-real', 'authHeader': 'Authorization', 'enabled': True})
        model_id = model['id']
        require(not model.get('apiKey'), '公共模型响应含明文Key')
        updated = request(f'/api/models?projectId={project()}&id={model_id}', 'PUT', {'model': 'peer-exact-deployment', 'name': 'Peer renamed model', 'apiKey': ''})
        require(updated['model'] == 'peer-exact-deployment' and updated.get('apiKeyMasked'), '部署名或留空Key保存异常')
        other = request('/api/projects', 'POST', {'name': '同伴共享模型检查-' + uuid.uuid4().hex[:6]})
        shared = request('/api/models?projectId=' + str(other['id']))
        require(any(m['id'] == model_id for m in shared), '模型池未跨项目共享')
        request(f'/api/models?projectId={project()}&id={model_id}', 'DELETE')
        return '仅使用未验证的虚拟配置，未请求模型供应商'

    def no_fallback():
        settings = request('/api/settings/llm?projectId=' + str(project()))
        if settings.get('configured'):
            raise Blocked('已有真实模型配置，为避免计费未调用；请按人工用例验证模型失败')
        error = request('/api/agent/draft', 'POST', {'projectId': project(), 'prompt': '测试注册接口'}, expected=(409,))
        require(error.get('code') == 'assistant_needs_configuration' and 'request' not in error, '未配置模型仍生成草稿')

    def openapi():
        # The platform can be healthy while a restarted JVM is still booting.
        # Wait for the tested service independently instead of reporting a
        # premature connection refusal as a functional failure.
        deadline = time.monotonic() + 120
        while True:
            try:
                spec = request(args.cloud_doc_url, auth=False, absolute=True)
                break
            except Exception:
                if time.monotonic() >= deadline:
                    raise AssertionError('云盘OpenAPI在120秒等待后仍未就绪，请检查云盘依赖和启动日志')
                time.sleep(2)
        require(isinstance(spec.get('paths'), dict) and spec['paths'], '云盘OpenAPI无接口')
        spec['servers'] = [{'url': args.cloud_base.rstrip('/')}]
        imported = request('/api/apis/import', 'POST', {'projectId': project(), 'spec': spec})
        require(imported['imported'] > 0, '接口导入失败')
        state['cloud_ready'] = True
        return f'导入{imported["imported"]}个接口；导入数量不代表全部已测试'

    def cloud_plan():
        if not state.get('cloud_ready'):
            raise Blocked('云盘OpenAPI前置检查失败，未生成业务任务')
        task = {'projectId': project(), 'name': '同伴云盘核心业务冒烟', 'baseUrl': args.cloud_base, 'openApiUrl': args.cloud_base.rstrip('/') + '/v3/api-docs', 'goal': '验证注册登录、目录重名、文件上传下载和删除还原', 'template': 'cloud_disk', 'accountMode': 'auto', 'fileMode': 'auto', 'permission': 'review', 'features': ['folder', 'file', 'lifecycle', 'auth']}
        plan = request('/api/project-tests/plan', 'POST', task)
        require(len(plan.get('steps', [])) == 16 and plan.get('source') == 'cloud_disk', '核心模板步骤或来源不符合预期')
        task['steps'] = plan['steps']
        saved = request('/api/project-tests/tasks', 'POST', task)
        state['task'] = saved['id']
        return f'taskId={saved["id"]}，16步预置验收模板，未调用AI'

    def cloud_run():
        if 'task' not in state:
            raise Blocked('未生成有效云盘流程')
        run = request('/api/project-tests/runs', 'POST', {'projectId': project(), 'taskId': state['task'], 'confirmed': True})
        state['cloud_run'] = run['id']
        deadline = time.monotonic() + 600
        while run['status'] in ('queued', 'running') and time.monotonic() < deadline:
            time.sleep(1)
            run = request(f'/api/project-tests/runs?projectId={project()}&id={run["id"]}')
        state['cloud_report'] = run
        (output/'cloud-run.json').write_text(json.dumps(run, ensure_ascii=False, indent=2), encoding='utf-8')
        for step in run.get('results', []):
            results.append({'id': 'CD-' + step['id'], 'name': step['name'], 'status': {'passed': 'PASS', 'failed': 'FAIL', 'blocked': 'BLOCKED'}.get(step['status'], 'NOT_RUN'), 'actual': step.get('error') or f'HTTP {step["statusCode"]}; runId={run["id"]}', 'durationMs': step.get('durationMs', 0)})
        summary = run['summary']
        require(summary['passed'] == 16 and summary['failed'] == 0 and summary['blocked'] == 0, '云盘主链路未全部通过：' + json.dumps(summary, ensure_ascii=False))
        return f'runId={run["id"]}；16/16步骤通过'

    def cloud_report():
        if 'cloud_run' not in state:
            raise Blocked('无云盘运行记录')
        original = state['cloud_run']
        readback = request(f'/api/project-tests/runs?projectId={project()}&id={original}')
        require(readback['id'] == original, '历史未读回原记录')
        for fmt in ('markdown', 'json'):
            data = request(f'/api/project-tests/export?projectId={project()}&id={original}&format={fmt}', raw=True)
            require(data, '导出为空')
            (output/('cloud-report.md' if fmt == 'markdown' else 'cloud-report.json')).write_bytes(data)

    deadline = time.monotonic() + 120
    while True:
        try:
            request('/api/health')
            break
        except Exception:
            if time.monotonic() >= deadline:
                break
            time.sleep(2)
    check('SM-01', '匿名平台API被拦截', lambda: request('/api/projects', auth=False, expected=(401,), raw=True) and 'HTTP 401')
    check('SM-02', '平台登录及健康', lambda: request('/api/health')['status'] == 'ok' or (_ for _ in ()).throw(AssertionError('健康失败')))
    check('SM-03', '创建独立验收项目', create_project)
    check('SM-04', '新增Echo接口', create_endpoint)
    check('SM-05', 'Echo业务断言与保存', echo)
    check('SM-06', 'HTTP200但业务断言错应失败', business_failure)
    check('SM-07', '预期401的负向用例通过', auth_negative)
    check('SM-08', '正确Bearer Token通过', auth_positive)
    check('SM-09', '用例和历史读回', history)
    check('SM-10', '元数据地址被拒绝', deny_metadata)
    check('SM-11', '模型配置共享、部署名、脱敏和删除', model_crud)
    check('SM-12', '未配置助手时明确提示', no_fallback)
    check('SM-13', '读取及导入云盘OpenAPI', openapi)
    check('SM-14', '云盘16步流程及保存', cloud_plan)
    check('SM-15', '云盘真实核心业务执行', cloud_run)
    check('SM-16', '云盘历史及Markdown/JSON导出', cloud_report)
    summary = {s: sum(r['status'] == s for r in results) for s in ('PASS', 'FAIL', 'BLOCKED', 'NOT_RUN')}
    report = {'createdAt': dt.datetime.now().astimezone().isoformat(), 'platform': args.base_url, 'cloudBaseReachedByBackend': args.cloud_base, 'projectId': state.get('project'), 'summary': summary, 'scope': '平台HTTP基础功能与云盘核心16步；不包含真实模型质量、全部云盘接口或浏览器视觉验收', 'results': results}
    (output/'acceptance.json').write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding='utf-8')
    lines = ['# 自动冒烟验收实测记录', '', json.dumps(summary, ensure_ascii=False), '', report['scope'], '', '| 编号 | 用例 | 结果 | 实际 |', '| --- | --- | --- | --- |']
    lines += [f'| {r["id"]} | {r["name"]} | {r["status"]} | {str(r["actual"]).replace("|", "/").replace(chr(10), " ")} |' for r in results]
    (output/'acceptance.md').write_text('\n'.join(lines)+'\n', encoding='utf-8')
    print('结果目录：' + str(output))
    print(json.dumps(summary, ensure_ascii=False))
    return 0 if summary['FAIL'] == 0 and summary['BLOCKED'] == 0 and summary['NOT_RUN'] == 0 else 1


if __name__ == '__main__':
    sys.exit(main())
