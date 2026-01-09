# Loky

Loky是一个将命令输出通过HTTP API发送到Grafana Loki的Go程序，支持PTY能力、内存缓存和批量发送。

## 特性

- **双模式输入**：支持PTY模式（默认）和简单管道模式
- **内存缓存**：使用环形缓冲区，默认10000条消息，FIFO丢弃策略
- **批量发送**：支持时间触发（默认900ms）和缓冲区满载触发
- **Basic Auth**：完整的Loki HTTP API认证支持
- **配置灵活**：支持命令行参数和环境变量
- **重试机制**：指数退避算法，默认重试3次
- **优雅关闭**：支持SIGINT和SIGTERM信号处理

## 安装

### 从源码构建

```bash
git clone https://github.com/yourname/loky.git
cd loky
make build
```

### 使用Docker

```bash
docker build -t loky:latest .
docker run -e LOKI_ENDPOINT=http://loki:3100 \
           -e LOKI_USERNAME=admin \
           -e LOKI_PASSWORD=password \
           loky:latest -- your-command
```

## 配置

所有配置项都支持环境变量，优先级：命令行参数 > 环境变量 > 默认值

### 环境变量

| 变量名 | 描述 | 默认值 |
|--------|------|--------|
| `LOKI_ENDPOINT` | Loki服务地址 | 无（必需） |
| `LOKI_USERNAME` | Basic Auth用户名 | 无 |
| `LOKI_PASSWORD` | Basic Auth密码 | 无 |
| `LOKI_LABELS` | Loki标签，格式key=value | 无 |
| `BUFFER_SIZE` | 缓冲区大小 | 10000 |
| `BATCH_INTERVAL` | 批量发送间隔 | 900ms |
| `BATCH_SIZE` | 批量大小 | 1000 |
| `RETRY_COUNT` | 重试次数 | 3 |
| `INPUT_MODE` | 输入模式(pty/pipe) | pty |
| `LOG_LEVEL` | 日志级别 | info |

### 命令行参数

```bash
loky [flags] -- command [args...]

Flags:
  --config string       配置文件 (默认是 $HOME/.loky.yaml)
  --input-mode string   输入模式 (pty, pipe) (默认 "pty")
  --log-level string    日志级别 (debug, info, warn, error) (默认 "info")
  -h, --help           显示帮助信息
```

## 使用示例

### 基本用法

```bash
# 使用环境变量配置
export LOKI_ENDPOINT="http://localhost:3100/loki/api/v1/push"
export LOKI_USERNAME="admin"
export LOKI_PASSWORD="password"
export LOKI_LABELS="service=myapp,env=prod"

# 发送ping输出到Loki
loky -- ping google.com
```

### 使用管道模式

```bash
# 监控日志文件
loky --input-mode pipe -- tail -f /var/log/app.log
```

### 使用PTY模式（默认）

```bash
# 监控应用程序输出（支持彩色输出和交互式程序）
loky -- my-app --arg1 --arg2
```

## 架构

```
命令输出 → PTY/管道捕获 → 行解析 → 内存缓存 → 批量处理器 → HTTP客户端 → Loki
```

### 核心组件

- **输入处理**：支持PTY和管道两种模式，捕获命令的标准输出和标准错误
- **内存缓存**：环形缓冲区实现，线程安全，满载时FIFO丢弃
- **批量处理器**：定时器触发和缓冲区满载触发，支持动态批量大小
- **Loki客户端**：HTTP客户端，支持Basic Auth，JSON格式请求
- **重试机制**：指数退避算法，最大重试次数可配置

## 开发

### 运行测试

```bash
# 运行所有测试
make test

# 生成测试覆盖率报告
make test-coverage
```

### 代码检查

```bash
# 运行linter
make lint

# 格式化代码
make fmt
```

### 构建

```bash
# 构建二进制文件
make build

# 交叉编译
make build-all
```

## 项目结构

```
loky/
├── cmd/loky/main.go          # 主入口
├── internal/
│   ├── config/              # 配置管理
│   ├── input/               # 输入处理（pty + reader）
│   ├── buffer/              # 环形缓冲区
│   ├── loki/                # HTTP客户端 + Auth
│   ├── batcher/             # 批量处理器
│   └── retry/               # 重试机制
├── pkg/
│   └── types/               # 公共类型
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
└── README.md
```

## 性能指标

- **内存占用**：根据缓冲区大小配置，默认约10MB
- **吞吐量**：取决于批量间隔和网络延迟
- **CPU占用**：低，主要在批量发送和网络请求时

## 故障排查

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

## 许可证

MIT License

## 贡献

欢迎提交Pull Request或Issue。
