# Go 配置管理最佳实践重构指南

本文档总结了 `asr_server` 项目配置模块的重构，展示 Go 语言中配置管理的最佳实践。

## 核心原则

### 1. 消除全局变量

**❌ 反模式（重构前）**
```go
// config/config.go
var GlobalConfig Config  // 全局变量

func InitConfig(path string) error {
    viper.Unmarshal(&GlobalConfig)
    return nil
}

// 其他包中直接访问
func ProcessAudio() {
    rate := config.GlobalConfig.Audio.SampleRate  // 隐式依赖
}
```

**✅ 最佳实践（重构后）**
```go
// config/config.go
func Load(path string) (*Config, error) {
    var cfg Config
    // ... 加载逻辑
    return &cfg, nil  // 返回实例，不修改全局状态
}

// main.go
func main() {
    cfg, err := config.Load("config.json")
    sessionManager := session.NewManager(cfg, recognizer, vadPool)  // 显式传递
}
```

### 2. 依赖注入

**原则**：每个组件通过构造函数声明其依赖

```go
// ✅ 良好的构造函数设计
type Manager struct {
    cfg        *config.Config
    recognizer *sherpa.OfflineRecognizer
    vadPool    pool.VADPoolInterface
}

func NewManager(cfg *config.Config, recognizer *sherpa.OfflineRecognizer, vadPool pool.VADPoolInterface) *Manager {
    return &Manager{
        cfg:        cfg,
        recognizer: recognizer,
        vadPool:    vadPool,
    }
}
```

### 3. 不可变配置

配置加载后应视为不可变：

```go
// Load 返回不可变的配置实例
func Load(configPath string) (*Config, error) {
    // 加载、验证、返回
    return &cfg, nil  // 返回后不应修改
}

// 如需"修改"配置，创建新实例
func (c *Config) WithPort(port int) *Config {
    newCfg := *c  // 复制
    newCfg.Server.Port = port
    return &newCfg
}
```

## 重构对比

### 配置包 (`config/config.go`)

| 方面 | 重构前 | 重构后 |
|------|--------|--------|
| 全局变量 | `var GlobalConfig Config` | 无全局变量 |
| 加载函数 | `InitConfig()` 修改全局变量 | `Load()` 返回新实例 |
| 默认值 | 无 | `setDefaults()` 注册所有默认值 |
| 验证 | 无 | `Validate()` 完整校验 |
| 错误类型 | 字符串 | 预定义错误变量 |

### 会话管理器 (`session/manager.go`)

**重构前：**
```go
func (m *Manager) ProcessAudioData(...) error {
    normalizeFactor := config.GlobalConfig.Audio.NormalizeFactor  // 隐式依赖
}
```

**重构后：**
```go
func NewManager(cfg *config.Config, ...) *Manager {
    return &Manager{cfg: cfg, ...}  // 显式注入
}

func (m *Manager) ProcessAudioData(...) error {
    normalizeFactor := m.cfg.Audio.NormalizeFactor  // 通过字段访问
}
```

### WebSocket 处理器 (`ws/websocket.go`)

**重构前：**
```go
// 包级别的全局 Upgrader
var Upgrader = websocket.Upgrader{
    ReadBufferSize: config.GlobalConfig.Server.WebSocket.ReadBufferSize,  // 问题：初始化时配置可能未加载
}
```

**重构后：**
```go
type Handler struct {
    cfg      *config.Config
    upgrader websocket.Upgrader
}

func NewHandler(cfg *config.Config, ...) *Handler {
    return &Handler{
        cfg: cfg,
        upgrader: websocket.Upgrader{
            ReadBufferSize: cfg.Server.WebSocket.ReadBufferSize,  // 构造时配置已加载
        },
    }
}
```

## 依赖注入图

```
main()
  │
  ├── config.Load() ──────────────────────────────┐
  │       │                                       │
  │       ▼                                       │
  ├── bootstrap.InitApp(cfg)                      │
  │       │                                       │
  │       ├── pool.NewVADFactory(cfg)             │
  │       │       └── 使用 cfg.VAD.*              │
  │       │                                       │
  │       ├── session.NewManager(cfg, ...)        │
  │       │       └── 使用 cfg.Session.*          │
  │       │       └── 使用 cfg.Audio.*            │
  │       │                                       │
  │       └── speaker.NewHandler(manager, cfg)    │
  │               └── 使用 cfg.Audio.*            │
  │                                               │
  ├── router.NewRouter(deps)                      │
  │       │                                       │
  │       └── ws.NewHandler(cfg, ...)             │
  │               └── 使用 cfg.Server.WebSocket.* │
  │                                               │
  └── http.Server{Addr: cfg.Addr(), ...}         │
                                                  │
                 所有组件共享同一个 cfg 实例 ◄──────┘
```

## 优势总结

| 优势 | 说明 |
|------|------|
| **可测试性** | 可以轻松注入模拟配置进行单元测试 |
| **明确依赖** | 查看构造函数即可知道组件需要什么 |
| **无初始化顺序问题** | 配置在使用前已完全加载 |
| **并发安全** | 不可变配置无需加锁 |
| **IDE 支持** | 可以追踪配置的使用位置 |
| **重构友好** | 修改配置结构时编译器会报告所有使用点 |

## 何时使用全局变量？

在极少数情况下，全局变量是合理的：

1. **日志器** - 因为几乎所有代码都需要日志
2. **指标收集器** - 类似日志，是横切关注点
3. **单例资源池** - 如数据库连接池（但仍建议通过 DI 传递）

即使这些情况，也应该：
- 通过 `sync.Once` 确保只初始化一次
- 提供显式的初始化函数
- 考虑是否可以用依赖注入替代

## 敏感信息脱敏

### 为什么重要？

配置中的敏感信息（密码、密钥、令牌）不应该出现在：
- 日志文件
- 错误消息
- 调试输出
- 监控指标

### 实现方案

#### 1. Mask 函数 - 部分隐藏

```go
// Mask masks a sensitive string, showing only first and last 2 characters.
func Mask(s string) string {
    if len(s) == 0 {
        return ""
    }
    if len(s) <= 4 {
        return "****"
    }
    return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

// 示例：
// "mysecretpassword" -> "my************rd"
// "short" -> "****"
```

#### 2. MaskWithLength - 保留长度信息

```go
func MaskWithLength(s string) string {
    if len(s) == 0 {
        return ""
    }
    return fmt.Sprintf("[MASKED:%d]", len(s))
}

// 示例：
// "mysecretpassword" -> "[MASKED:16]"
```

#### 3. IsSensitiveKey - 自动检测

```go
var SensitiveKeywords = []string{
    "password", "passwd", "pwd",
    "secret", "private",
    "key", "apikey", "api_key",
    "token", "auth",
    "credential", "cred",
}

func IsSensitiveKey(key string) bool {
    keyLower := strings.ToLower(key)
    for _, keyword := range SensitiveKeywords {
        if strings.Contains(keyLower, keyword) {
            return true
        }
    }
    return false
}
```

### 使用场景

#### 在 Print() 中使用

```go
func (c *Config) Print() {
    fmt.Printf("  Server: %s:%d\n", c.Server.Host, c.Server.Port)
    fmt.Printf("  API Key: %s\n", Mask(c.APIKey))           // ✅ 安全
    fmt.Printf("  DB Password: %s\n", MaskWithLength(c.DB.Password))  // ✅ 安全
}
```

#### 在结构化日志中使用

```go
func (c *Config) ToSafeMap() map[string]interface{} {
    return map[string]interface{}{
        "server": map[string]interface{}{
            "host": c.Server.Host,
            "port": c.Server.Port,
        },
        "api_key": Mask(c.APIKey),  // 脱敏
        // ...
    }
}

// 用于 JSON 日志
logger.WithFields(cfg.ToSafeMap()).Info("Configuration loaded")
```

#### 在错误消息中使用

```go
// ❌ 危险
return fmt.Errorf("auth failed for key: %s", apiKey)

// ✅ 安全
return fmt.Errorf("auth failed for key: %s", config.Mask(apiKey))
```

### 测试验证

```go
func TestMask(t *testing.T) {
    tests := []struct {
        input    string
        expected string
    }{
        {"", ""},
        {"ab", "****"},
        {"password123", "pa*******23"},
    }
    for _, tt := range tests {
        if got := Mask(tt.input); got != tt.expected {
            t.Errorf("Mask(%q) = %q, want %q", tt.input, got, tt.expected)
        }
    }
}
```
