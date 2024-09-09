# 构建阶段：使用 Node.js 官方镜像（node:23-alpine3.20）作为构建环境
FROM node:23-alpine3.20 AS builder

# 设置工作目录
WORKDIR /app

# 拷贝 package.json 和 pnpm-lock.yaml 文件
COPY package.json pnpm-lock.yaml ./

# 安装项目依赖
RUN pnpm install --frozen-lockfile

# 拷贝剩余的项目文件
COPY . .

# 构建 Next.js 应用
RUN pnpm build

# 清理缓存以优化镜像
RUN pnpm store prune

# 运行阶段：使用更小的镜像运行应用
FROM node:23-alpine3.20 AS runner

# 设置工作目录
WORKDIR /app

# 从构建阶段复制必要的文件和文件夹
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json ./package.json

# 运行时的默认命令（启动应用）
CMD ["pnpm", "start"]

# 暴露端口
EXPOSE 3000
