# ASR Server - 本地开发指南

本指南介绍如何在本地环境中进行 ASR Server 的开发和调试。

## 环境要求

- Go 1.24+
- Python 3.6+ (用于下载模型)
- gcc/g++ (用于 CGO 支持)
- 足够的磁盘空间 (模型文件约 500MB)

### 安装依赖

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y build-essential python3 python3-pip wget

# 安装 Python modelscope (可选,脚本会自动安装)
pip3 install modelscope -i https://mirrors.aliyun.com/pypi/simple/
```

## 快速开始

### 1. 下载模型文件

首次运行需要下载 ASR、VAD 和 Speaker 模型:

```bash
# 给脚本添加执行权限
chmod +x scripts/download_models.sh
chmod +x dev.sh

# 下载所有模型
./scripts/download_models.sh
```

**注意**: 如果模型文件已经存在(例如从 Docker 环境复制),可以跳过此步骤。

### 2. 启动开发服务器

```bash
# 启动开发服务器
./dev.sh
```

开发启动脚本会自动:
- ✅ 配置动态链接库路径
- ✅ 检查模型文件
- ✅ 创建日志目录
- ✅ 启动服务器

### 3. 验证服务

服务启动后,可以访问以下端点验证:

```bash
# 健康检查
curl http://127.0.0.1:8080/health

# WebSocket 连接 (使用测试客户端)
# ws://127.0.0.1:8080/ws
```

## 配置文件说明

### config.json (统一配置)

**开发和生产使用同一配置**,好处:
- ✅ **简单直观** - 只有一个配置文件
- ✅ **环境一致** - 避免"在我机器上能跑"的问题
- ✅ **易于维护** - 不用同步多个配置文件
- ✅ **减少 bug** - 配置差异不会导致环境问题

**关键配置**:
```json
{
  "server": {
    "port": 8080,           // 浏览器安全端口
    "host": "0.0.0.0"       // 允许外部访问 (开发时可改为 127.0.0.1)
  },
  "logging": {
    "level": "info",        // 开发时可改为 "debug"
    "output": "both"        // console + file
  }
}
```

**灵活使用**:
- 开发时需要 debug 日志? 直接修改 `config.json` 的 `level`
- 只想本地访问? 修改 `host` 为 `127.0.0.1`
- 修改后重启服务器即可生效

## 开发技巧

### 环境变量

```bash
# 使用自定义配置文件
CONFIG_FILE=config.custom.json ./dev.sh

# 手动设置库路径 (不推荐,dev.sh 会自动处理)
export LD_LIBRARY_PATH=$PWD/lib:$PWD/lib/ten-vad/lib/Linux/x64:$LD_LIBRARY_PATH
go run main.go
```

### 热重载 (可选)

安装 Air 实现代码热重载:

```bash
# 安装 air
go install github.com/cosmtrek/air@latest

# 创建 .air.toml 配置文件
air init

# dev.sh 会自动检测并使用 air
./dev.sh
```

### 调试技巧

1. **启用 Debug 日志**:
   编辑 `config.json`,设置 `"logging": {"level": "debug"}`

2. **查看实时日志**:
   ```bash
   tail -f logs/app.log
   ```

3. **使用 Delve 调试器**:
   ```bash
   # 安装 delve
   go install github.com/go-delve/delve/cmd/dlv@latest
   
   # 启动调试
   export LD_LIBRARY_PATH=$PWD/lib:$PWD/lib/ten-vad/lib/Linux/x64:$LD_LIBRARY_PATH
   dlv debug main.go
   ```

## 项目结构

```
asr_server/
├── main.go                 # 入口文件
├── config/                 # 配置模块
├── internal/              # 内部实现
│   ├── bootstrap/         # 依赖初始化
│   ├── logger/            # 日志模块
│   ├── router/            # 路由
│   ├── handlers/          # HTTP/WebSocket 处理器
│   ├── pool/              # 对象池
│   ├── session/           # 会话管理
│   └── ...
├── models/                # AI 模型文件
│   ├── asr/              # 语音识别模型
│   ├── vad/              # 语音活动检测模型
│   └── speaker/          # 说话人识别模型
├── lib/                   # 动态链接库
├── scripts/               # 工具脚本
│   └── download_models.sh # 模型下载脚本
├── dev.sh                 # 开发启动脚本
└── config.json            # 统一配置
```

## 常见问题

### 1. 动态库加载失败

**错误**: `error while loading shared libraries: libten_vad.so`

**解决方案**:
- 使用 `./dev.sh` 启动 (自动配置库路径)
- 或手动设置: `export LD_LIBRARY_PATH=$PWD/lib:$PWD/lib/ten-vad/lib/Linux/x64`

### 2. 模型文件缺失

**错误**: `tokens: 'models/asr/.../tokens.txt' does not exist`

**解决方案**:
```bash
# 运行模型下载脚本
./scripts/download_models.sh
```

### 3. 端口被占用

**错误**: `bind: address already in use`

**解决方案**:
```bash
# 查找占用端口的进程
lsof -i :8080

# 或修改配置文件中的端口号
# config.json -> "port": 9000
```

### 4. Go 依赖问题

**解决方案**:
```bash
# 设置国内代理
go env -w GOPROXY=https://goproxy.cn,direct

# 重新下载依赖
go mod download
```

## 与 Docker 环境的区别

| 特性 | 本地开发 | Docker |
|------|---------|--------|
| 配置文件 | config.json | config.json |
| 库路径 | dev.sh 自动设置 | Dockerfile ENV |
| 模型下载 | scripts/download_models.sh | docker-entrypoint.sh |
| 启动方式 | ./dev.sh | docker run |
| 环境一致性 | ✅ 完全一致 | ✅ 完全一致 |

## 贡献指南

开发新功能时:
1. 修改 `config.json` 按需调整配置
2. 需要 debug 日志时设置 `logging.level` 为 `debug`
3. 运行现有测试: `go test ./...`
4. 开发和生产环境配置完全一致,无需担心环境差异

## 相关文档

- [配置说明](docs/configuration.md) (如果存在)
- [API 文档](docs/api.md) (如果存在)
- [Docker 部署](README.md)
