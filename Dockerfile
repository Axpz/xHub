# 使用轻量级基础镜像
FROM node:23-alpine3.20 AS builder

# 设置工作目录
WORKDIR /app

RUN npm install -g pnpm@latest

# 将项目文件复制到容器中（排除 .dockerignore 文件中配置的内容）
COPY package.json pnpm-lock.yaml ./

# 安装生产依赖（只安装 package.json 和锁文件中的运行时依赖）
RUN pnpm install --frozen-lockfile

# 复制其余文件并构建项目
COPY . .
RUN pnpm build

# 生产阶段，使用更小的镜像
FROM node:23-alpine3.20 AS runner

# 设置非 root 用户以提高安全性
USER node

# 设置工作目录
WORKDIR /app

# 仅复制必要的文件，减小体积
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

# 定义环境变量
ENV NODE_ENV=production
ENV PORT=3000

# 暴露端口
EXPOSE 3000

# 启动应用
CMD ["node", "server.js"]
