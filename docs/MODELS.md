# ASR Server - 模型说明文档

本文档详细介绍 ASR Server 使用的三个 AI 模型及其作用。

## 📚 模型概览

| 模型类型 | 模型名称 | 文件大小 | 作用 | 必需性 |
|---------|---------|---------|------|--------|
| **VAD** | Silero VAD | ~1.8MB | 语音活动检测 | ✅ 必需 |
| **ASR** | Fun-ASR-Nano-2512-8bit | ~251MB | 语音转文字 | ✅ 必需 |
| **Speaker** | 3DSpeaker CampPlus | ~27MB | 说话人识别 | ⚪ 可选 |

**总大小**: 约 280MB

---

## 1. 🔇 VAD 模型 (Voice Activity Detection)

### 基本信息
- **模型名称**: Silero VAD
- **文件路径**: `models/vad/silero_vad/silero_vad.onnx`
- **文件大小**: ~1.8MB
- **来源**: [Silero VAD GitHub](https://github.com/snakers4/silero-vad)

### 作用
**检测音频中哪些部分是人声,哪些是静音或噪音**。

### 工作原理
```
原始音频: [静音 500ms][你好 800ms][静音 300ms][世界 600ms][静音 400ms]
         ↓ VAD 检测
有效语音: [你好 800ms] [世界 600ms]
```

### 为什么需要 VAD?
1. **提高识别准确率**: 
   - 过滤掉静音、背景噪音、咳嗽声等无效音频
   - 只将真正的语音送给 ASR 模型

2. **提升性能**:
   - 减少 ASR 模型的计算量
   - 降低延迟,提高实时性

3. **智能分段**:
   - 自动将长音频切分成语音片段
   - 每个片段单独识别,结果更准确

### 配置参数
```json
"vad": {
  "provider": "ten_vad",        // 使用 ten_vad 引擎 (或 silero_vad)
  "pool_size": 200,            // VAD 实例池大小
  "threshold": 0.5,            // 检测阈值 (0-1, 越高越严格)
  "silero_vad": {
    "min_silence_duration": 0.1,   // 最小静音时长(秒)
    "min_speech_duration": 0.25,   // 最小语音时长(秒)
    "max_speech_duration": 8.0     // 最大语音时长(秒)
  }
}
```

### 实际应用场景
- ✅ 会议录音转写 (过滤会议中的长时间静音)
- ✅ 客服对话分析 (区分客服和客户的说话时间)
- ✅ 实时语音助手 (只在用户说话时激活)

---

## 2. 🎙️ ASR 模型 (Automatic Speech Recognition)

### 基本信息
- **模型名称**: Fun-ASR-Nano-2512-8bit
- **文件路径**: 
  - `models/asr/Fun-ASR-Nano-2512-8bit/model.onnx` (模型文件)
  - `models/asr/Fun-ASR-Nano-2512-8bit/tokens.txt` (词表文件)
- **文件大小**: ~251MB
- **来源**: [FunASR - 阿里达摩院](https://github.com/alibaba-damo-academy/FunASR)

### 作用
**将语音音频转换为文字**,这是系统的**核心功能**。

### 工作原理
```
音频波形: [声波数据]
       ↓ 特征提取
声学特征: [MFCC/Mel频谱]
       ↓ ASR 模型推理
概率分布: [你:0.95, 好:0.92, 世:0.88, 界:0.91]
       ↓ 解码
识别文字: "你好世界"
```

### 模型特点

#### 1. **多语言支持** (31种语言)
- 🇨🇳 中文 (普通话、粤语等方言)
- 🇬🇧 英文
- 🇯🇵 日语
- 🇰🇷 韩语
- 🇫🇷 法语
- 🇩🇪 德语
- ... 等 31 种语言

#### 2. **8-bit 量化优势**
- ⚡ **速度快**: 比原始模型快 2-4 倍
- 💾 **体积小**: 原始模型 ~1GB,量化后 ~251MB
- 🎯 **准确率**: 损失 <2%,实际应用几乎无影响
- 💻 **资源占用低**: 适合 CPU 推理

#### 3. **端到端架构**
- 无需传统的声学模型 + 语言模型
- 直接从音频到文字,延迟更低

### 配置参数
```json
"recognition": {
  "model_path": "models/asr/Fun-ASR-Nano-2512-8bit/model.onnx",
  "tokens_path": "models/asr/Fun-ASR-Nano-2512-8bit/tokens.txt",
  "language": "auto",              // 自动检测语言
  "num_threads": 8,                // CPU 线程数 (根据 CPU 调整)
  "provider": "cpu",               // 使用 CPU 推理
  "debug": false
}
```

### 性能指标
- **实时率 (RTF)**: ~0.1-0.3 (比实时快 3-10 倍)
  - 例: 10秒音频,只需 1-3 秒识别完成
- **字错率 (CER)**: 
  - 普通话: ~3-5%
  - 英文: ~5-8%
  - 其他语言: ~8-15%

### 实际应用场景
- ✅ 会议实时转写
- ✅ 智能客服对话记录
- ✅ 视频自动字幕生成
- ✅ 语音输入法
- ✅ 多语言翻译前处理

---

## 3. 👤 Speaker 模型 (Speaker Recognition)

### 基本信息
- **模型名称**: 3DSpeaker CampPlus
- **文件路径**: `models/speaker/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx`
- **文件大小**: ~27MB
- **来源**: [3D-Speaker](https://github.com/modelscope/3D-Speaker)

### 作用
**识别"是谁在说话",提取和比对声纹特征**。

### 工作原理
```
音频输入
    ↓
特征提取 → [声纹向量: 192维特征]
    ↓
数据库比对:
  - 张三的声纹: [0.92] 相似度
  - 李四的声纹: [0.31] 相似度
  - 王五的声纹: [0.15] 相似度
    ↓
识别结果: 说话人是 "张三" (置信度 92%)
```

### 核心功能

#### 1. **说话人注册** (Enrollment)
- 录制用户的几段语音
- 提取声纹特征向量
- 存储到数据库

#### 2. **说话人识别** (Identification)
- "这段语音是数据库中的谁?"
- 1:N 比对 (与数据库中所有人比对)
- 返回最相似的说话人

#### 3. **说话人验证** (Verification)
- "这段语音是不是张三?"
- 1:1 比对 (与特定人比对)
- 返回是/否 + 置信度

### 配置参数
```json
"speaker": {
  "enabled": true,              // 是否启用说话人识别
  "model_path": "models/speaker/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx",
  "num_threads": 4,             // CPU 线程数
  "provider": "cpu",
  "threshold": 0.6,             // 识别阈值 (0-1, 越高越严格)
  "data_dir": "data/speaker"    // 声纹数据库目录
}
```

### 性能指标
- **特征维度**: 192维向量
- **相似度计算**: 余弦相似度 (Cosine Similarity)
- **识别准确率**: 
  - 清晰语音: >95%
  - 噪音环境: >85%
- **处理速度**: ~10ms/段语音

### 实际应用场景
- ✅ **会议纪要**: 区分不同发言人
- ✅ **智能家居**: 声纹解锁
- ✅ **客服质检**: 区分客服和客户
- ✅ **多人对话**: 自动标注谁在说话
- ⚪ **注意**: 可选功能,不启用也不影响基本转写

### 可选性说明
如果不需要区分说话人,可以在配置中禁用:
```json
"speaker": {
  "enabled": false  // 禁用说话人识别
}
```

---

## 🔄 三个模型的协作流程

```
原始音频流
    ↓
┌─────────────────┐
│  1. VAD 模型    │ ← 检测有效语音片段
│  语音活动检测   │
└─────────────────┘
    ↓
有效语音片段
    ↓
┌─────────────────┐
│  2. ASR 模型    │ ← 语音转文字 (核心)
│  语音识别       │
└─────────────────┘
    ↓
识别文字结果
    ↓
┌─────────────────┐
│ 3. Speaker 模型 │ ← 识别说话人 (可选)
│ 说话人识别      │
└─────────────────┘
    ↓
最终结果:
{
  "text": "你好世界",
  "speaker": "张三",
  "confidence": 0.92,
  "timestamp": "00:01:23"
}
```

---

## 📊 模型选型建议

### 场景 1: 基础语音转写
**只需要**: VAD + ASR
```json
"speaker": { "enabled": false }
```
**优点**: 
- 资源占用低
- 处理速度快
- 满足基本需求

**适用**: 
- 单人录音转写
- 语音输入法
- 简单的语音助手

### 场景 2: 多人对话分析
**需要**: VAD + ASR + Speaker
```json
"speaker": { "enabled": true }
```
**优点**: 
- 自动区分发言人
- 生成结构化数据
- 适合会议/采访场景

**适用**: 
- 会议纪要
- 客服对话分析
- 多人访谈录音

---

## 🔧 模型管理命令

### 下载所有模型
```bash
./scripts/download_models.sh
```

### 检查模型完整性
```bash
# 检查 VAD 模型
ls -lh models/vad/silero_vad/silero_vad.onnx

# 检查 ASR 模型
ls -lh models/asr/Fun-ASR-Nano-2512-8bit/model.onnx
ls -lh models/asr/Fun-ASR-Nano-2512-8bit/tokens.txt

# 检查 Speaker 模型
ls -lh models/speaker/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx
```

### 重新下载某个模型
```bash
# 删除旧模型
rm -rf models/asr/Fun-ASR-Nano-2512-8bit

# 重新运行下载脚本
./scripts/download_models.sh
```

---

## ❓ 常见问题

### Q: 可以只使用 ASR 模型,不用 VAD 吗?
**A**: 理论上可以,但**强烈不推荐**。没有 VAD:
- ❌ 识别准确率会显著下降
- ❌ 处理速度变慢
- ❌ 资源占用增加
- ❌ 无法自动分段

### Q: Speaker 模型不启用会影响基本转写吗?
**A**: **完全不影响**。Speaker 模型是可选的增强功能,只影响是否能识别说话人。

### Q: 为什么不用更大的 ASR 模型?
**A**: Fun-ASR-Nano 8bit 版本在**准确率、速度、资源占用**之间达到了最佳平衡:
- ✅ 准确率与大模型差距 <2%
- ✅ 速度快 2-4 倍
- ✅ 可以在 CPU 上实时运行
- ✅ 适合生产环境部署

### Q: 模型存放在哪里?为什么这么大?
**A**: 
- 位置: `models/` 目录
- 总大小: ~280MB
- 主要是 ASR 模型 (251MB),包含了所有语言的知识

### Q: 可以使用 GPU 加速吗?
**A**: 可以,修改配置:
```json
"recognition": {
  "provider": "cuda"  // 需要安装 CUDA 和对应的 ONNX Runtime
}
```

---

## 📚 相关资源

- [模型下载脚本](../scripts/download_models.sh)
- [配置文件说明](../config.dev.json)
- [本地开发指南](LOCAL_DEVELOPMENT.md)
- [快速命令参考](COMMANDS.md)

---

**更新时间**: 2026-01-04
**文档版本**: 1.0
