# AGENTS.md - Loky程序开发指南

## 项目概述

loky是一个将命令输出通过HTTP API发送到Grafana Loki的Go程序，支持pty能力、内存缓存和批量发送。

## 核心技术架构

### 1. 组件结构
```
loky/
├── cmd/loky/main.go          # 主入口
├── internal/
│   ├── config/              # 配置管理
│   │   └── config.go
│   ├── input/               # 输入处理
│   │   ├── pty.go          # PTY实现
│   │   └── reader.go       # 标准输出读取
│   ├── buffer/              # 内存缓存
│   │   └── ringbuffer.go   # 环形缓冲区
│   ├── loki/                # Loki客户端
│   │   ├── client.go       # HTTP客户端
│   │   └── auth.go         # Basic Auth
│   ├── batcher/             # 批量处理器
│   │   └── batcher.go      # 批量发送逻辑
│   └── retry/               # 重试机制
│       └── retry.go        # 简单重试策略
├── pkg/
│   └── types/               # 公共类型
│       └── types.go
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── Dockerfile
```

### 2. 核心流程
```
命令输出 → PTY捕获 → 行解析 → 内存缓存 → 批量处理器 → HTTP客户端 → Loki
```

## 开发环境设置

### 必要工具
- Go 1.21+
- Git
- Make (可选)
- Docker (可选)

### 项目初始化
```bash
# 创建项目结构
mkdir -p cmd/loky internal/{config,input,buffer,loki,batcher,retry} pkg/types

# 初始化Go模块
go mod init github.com/yourname/loky

# 安装依赖
go get github.com/creack/pty
go get github.com/sirupsen/logrus
go get github.com/spf13/cobra
go get github.com/spf13/viper
```

## 技术实现要点

### 1. PTY实现
使用 `github.com/creack/pty` 库实现伪终端：
- 启动子进程时分配PTY
- 实时读取标准输出和标准错误
- 支持终端大小调整

### 2. 内存缓存策略
- 实现环形缓冲区 (Ring Buffer)
- 默认容量10000条消息，可配置
- 满载时丢弃最旧消息 (FIFO)
- 线程安全设计

### 3. 批量发送机制
- 定时器触发发送 (默认900ms间隔)
- 缓冲区满载时立即发送
- 支持动态批量大小调整

### 4. HTTP客户端设计
- 使用Loki Push API: `/loki/api/v1/push`
- Basic Auth认证支持
- JSON格式请求体
- 压缩支持 (gzip)

### 5. 错误处理策略
- 简单重试机制：固定重试次数
- 重试间隔：1秒、2秒、4秒递增
- 重试失败后丢弃消息
- 连接超时设置

## 配置管理

### 命令行参数
```bash
loky [flags] -- command [args...]

Flags:
  --loki-endpoint string      Loki服务地址 (env: LOKI_ENDPOINT)
  --loki-username string      Basic Auth用户名 (env: LOKI_USERNAME)
  --loki-password string      Basic Auth密码 (env: LOKI_PASSWORD)
  --loki-labels strings       Loki标签，格式key=value (env: LOKI_LABELS)
  --buffer-size int          缓冲区大小，默认10000 (env: BUFFER_SIZE)
  --batch-interval duration   批量发送间隔，默认900ms (env: BATCH_INTERVAL)
  --retry-count int          重试次数，默认3 (env: RETRY_COUNT)
  --input-mode string        输入模式: pty或pipe，默认pty (env: INPUT_MODE)
  --log-level string         日志级别，默认info (env: LOG_LEVEL)
  --help                     显示帮助信息
```

### 环境变量支持
所有配置项都支持环境变量，优先级：命令行参数 > 环境变量 > 默认值

## 测试策略

### 单元测试
```bash
# 运行所有单元测试
go test ./...

# 生成测试覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 集成测试
- 模拟Loki服务进行测试
- 测试PTY功能
- 测试批量发送逻辑

### 性能测试
```bash
# 基准测试
go test -bench=. ./...

# 内存使用测试
go test -memprofile=mem.prof ./...
```

## 构建和部署

### 本地构建
```bash
# 构建
make build

# 交叉编译
make build-all

# 运行测试
make test

# 代码检查
make lint
```

### Docker构建
```bash
# 构建Docker镜像
docker build -t loky:latest .

# 运行容器
docker run -e LOKI_ENDPOINT=http://loki:3100 \
           -e LOKI_USERNAME=admin \
           -e LOKI_PASSWORD=password \
           loky:latest -- your-command
```

### Release发布
```bash
# 创建Git tag
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0

# 使用GoReleaser自动发布 (需配置)
goreleaser release --rm-dist
```

## 监控和日志

### 应用指标
- 发送到Loki的消息数量
- 缓冲区使用率
- HTTP请求成功率
- 重试次数统计

### 日志记录
使用logrus记录：
- 应用启动/关闭
- 配置加载
- 错误信息
- 性能指标

## 安全考虑

### 敏感信息保护
- 密码不在日志中显示
- 支持配置文件加密
- 运行时内存保护

### 网络安全
- HTTPS/TLS支持
- 连接超时设置
- 证书验证

## 常见问题排查

### 连接问题
- 检查Loki服务可达性
- 验证认证信息
- 查看网络连接状态

### 性能问题
- 监控内存使用
- 调整批量大小
- 优化缓冲区配置

### PTY问题
- 检查命令执行权限
- 验证终端环境
- 查看进程状态

## 开发规范

### 代码风格
- 遵循Go官方代码规范
- 使用gofmt格式化代码
- 使用golint检查代码质量

### Git工作流
- 功能分支开发
- Pull Request代码审查
- 自动化测试通过后合并

### 版本管理
- 语义化版本控制
- Changelog维护
- 向后兼容性保证
