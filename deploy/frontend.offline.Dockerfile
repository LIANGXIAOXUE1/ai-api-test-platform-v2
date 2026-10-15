# 无法访问 Docker Hub 时使用：复用本机已有 Nginx 运行时镜像，只替换当前前端构建产物。
FROM pulse-api-v2-frontend:latest
COPY frontend/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
