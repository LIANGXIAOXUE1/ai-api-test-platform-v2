# 无法访问 Docker Hub 时使用：复用本机已有运行时镜像，只替换当前源码编译出的 Linux 二进制。
FROM pulse-api-v2-backend:latest
COPY backend/api-server-linux /app/api-server
RUN chmod +x /app/api-server
