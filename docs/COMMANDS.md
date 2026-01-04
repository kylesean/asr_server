# ASR Server - 快速命令参考

## 本地开发常用命令

### 首次设置
```bash
# 1. 给脚本添加执行权限
chmod +x dev.sh scripts/download_models.sh

# 2. 下载所有模型文件
./scripts/download_models.sh
```

### 启动服务
```bash
# 启动服务器 (使用默认 config.json)
./dev.sh

# 使用自定义配置
CONFIG_FILE=config.custom.json ./dev.sh
```

### 调试技巧
```bash
# 设置环境变量后直接运行
export LD_LIBRARY_PATH=$PWD/lib:$PWD/lib/ten-vad/lib/Linux/x64:$LD_LIBRARY_PATH
go run main.go

# 使用 Delve 调试器
export LD_LIBRARY_PATH=$PWD/lib:$PWD/lib/ten-vad/lib/Linux/x64:$LD_LIBRARY_PATH
dlv debug main.go

# 查看实时日志
tail -f logs/app.log
```

### 代码热重载 (可选)
```bash
# 安装 air
go install github.com/cosmtrek/air@latest

# 初始化配置
air init

# dev.sh 会自动检测并使用 air
./dev.sh
```

### 测试
```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/pool/...

# 运行压力测试
python test/asr/stress_test.py --connections 100 --audio-per-connection 2

# 运行单文件识别测试
python test/asr/audiofile_test.py
```

### 编译
```bash
# 开发编译 (包含调试信息)
go build -o asr_server main.go

# 生产编译 (优化大小和性能)
go build -ldflags="-s -w" -o asr_server main.go
```

### 服务验证
```bash
# 健康检查
curl http://127.0.0.1:8080/health

# 查看服务状态
ps aux | grep asr_server

# 检查端口占用
lsof -i :8080
```

### 清理
```bash
# 清理编译缓存
go clean -cache

# 清理模型缓存 (下载时产生)
rm -rf models_cache

# 清理日志
rm -rf logs/*.log

# 清理编译产物
rm -f asr_server config.json.dev
```

## Docker 部署命令

### 基本操作
```bash
# 构建镜像
docker build -t asr_server .

# 运行容器
docker run -d -p 6000:6000 --name asr_server asr_server

# 查看日志
docker logs -f asr_server

# 停止容器
docker stop asr_server

# 删除容器
docker rm asr_server
```

### 高级操作
```bash
# 交互式运行 (调试)
docker run -it --rm -p 6000:6000 asr_server /bin/bash

# 挂载本地配置
docker run -d -p 6000:6000 \
  -v $(pwd)/config.json:/app/config.json \
  --name asr_server asr_server

# 挂载本地模型 (避免重复下载)
docker run -d -p 6000:6000 \
  -v $(pwd)/models:/app/models \
  --name asr_server asr_server
```

## 故障排查

### 库加载问题
```bash
# 检查库文件是否存在
ls -la lib/libten_vad.so
ls -la lib/ten-vad/lib/Linux/x64/libten_vad.so

# 检查 LD_LIBRARY_PATH
echo $LD_LIBRARY_PATH

# 临时设置库路径
export LD_LIBRARY_PATH=$PWD/lib:$PWD/lib/ten-vad/lib/Linux/x64:$LD_LIBRARY_PATH
```

### 模型文件问题
```bash
# 检查模型文件
ls -lh models/asr/Fun-ASR-Nano-2512-8bit/
ls -lh models/vad/silero_vad/
ls -lh models/speaker/

# 重新下载模型
rm -rf models/asr models/speaker models/vad
./scripts/download_models.sh
```

### 端口占用
```bash
# 查找占用端口的进程
lsof -i :8080
sudo netstat -tlnp | grep :8080

# 杀死占用端口的进程
kill -9 <PID>
```

### Go 依赖问题
```bash
# 设置国内代理
go env -w GOPROXY=https://goproxy.cn,direct

# 清理并重新下载
go clean -modcache
go mod download
go mod tidy
```

## 性能分析

### CPU 分析
```bash
# 启用 pprof
# 在 main.go 中添加:
# import _ "net/http/pprof"
# go func() { log.Println(http.ListenAndServe("localhost:6060", nil)) }()

# 生成 CPU profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
```

### 内存分析
```bash
# 生成内存 profile
go tool pprof http://localhost:6060/debug/pprof/heap
```

## 配置管理

### 查看当前配置
```bash
# 格式化查看 JSON 配置
jq . config.json

# 比较两个配置文件差异
diff -u config.json config.json
```

### 验证 JSON 格式
```bash
# 验证 JSON 格式
jq empty config.json && echo "Valid JSON" || echo "Invalid JSON"
```

## 有用的别名 (可添加到 ~/.bashrc)

```bash
# ASR Server 开发别名
alias asr-dev='cd /home/kylesean/projects/go/asr_server && ./dev.sh'
alias asr-logs='tail -f /home/kylesean/projects/go/asr_server/logs/app.log'
alias asr-test='cd /home/kylesean/projects/go/asr_server && go test ./...'
alias asr-build='cd /home/kylesean/projects/go/asr_server && go build -o asr_server main.go'
```
