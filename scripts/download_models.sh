#!/bin/bash
set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get project root directory
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo -e "${BLUE}ASR Server - Model Downloader${NC}"
echo "=============================="
echo ""

# Check if Python3 is installed
if ! command -v python3 &> /dev/null; then
    echo -e "${RED}✗${NC} Python3 is required but not installed."
    echo "Please install Python3 first"
    exit 1
fi

# Setup Python environment using uv
VENV_DIR=".venv-models"

if ! command -v uv &> /dev/null; then
    echo -e "${YELLOW}⚠${NC}  uv not found, falling back to system Python..."
    PYTHON_CMD="python3"
    # Try system Python with pip
    if ! python3 -c "import modelscope" 2>/dev/null; then
        echo -e "${YELLOW}⚠${NC}  modelscope not found. Installing with pip3..."
        python3 -m pip install --user modelscope -i https://mirrors.aliyun.com/pypi/simple/ || {
            echo -e "${RED}✗${NC} Failed to install modelscope"
            echo "Please install uv or manually install modelscope"
            exit 1
        }
        echo -e "${GREEN}✓${NC} modelscope installed"
    fi
else
    # Use uv to manage Python environment
    echo -e "${BLUE}Using uv to setup Python environment...${NC}"
    
    # Create venv if not exists
    if [ ! -d "$VENV_DIR" ]; then
        echo -e "${YELLOW}Creating virtual environment...${NC}"
        uv venv "$VENV_DIR" || {
            echo -e "${RED}✗${NC} Failed to create virtual environment"
            exit 1
        }
    fi
    
    # Activate venv and check/install modelscope
    source "$VENV_DIR/bin/activate"
    PYTHON_CMD="python"  # In venv, use python not python3
    
    if ! python -c "import modelscope" 2>/dev/null; then
        echo -e "${YELLOW}⚠${NC}  modelscope not found. Installing with uv..."
        uv pip install modelscope -i https://mirrors.aliyun.com/pypi/simple/ || {
            echo -e "${RED}✗${NC} Failed to install modelscope"
            deactivate
            exit 1
        }
        echo -e "${GREEN}✓${NC} modelscope installed"
    else
        echo -e "${GREEN}✓${NC} modelscope already available"
    fi
fi

# Create model directories
mkdir -p models/vad/silero_vad
mkdir -p models/asr/Fun-ASR-Nano-2512-8bit
mkdir -p models/speaker

# Download ASR model
if [[ ! -f "models/asr/Fun-ASR-Nano-2512-8bit/model.onnx" ]] || [[ ! -f "models/asr/Fun-ASR-Nano-2512-8bit/tokens.txt" ]]; then
    echo -e "${BLUE}Downloading Fun-ASR-Nano-2512-8bit model from ModelScope...${NC}"
    $PYTHON_CMD -c "
from modelscope import snapshot_download
model_dir = snapshot_download('fengge2024/Fun-ASR-Nano-2512-8bit', cache_dir='models_cache')
print(f'Model downloaded to: {model_dir}')
" || {
        echo -e "${RED}✗${NC} Failed to download ASR model"
        exit 1
    }
    
    # Copy model files
    find models_cache -name "model*.onnx" -exec cp {} models/asr/Fun-ASR-Nano-2512-8bit/model.onnx \;
    find models_cache -name "tokens.txt" -exec cp {} models/asr/Fun-ASR-Nano-2512-8bit/ \;
    echo -e "${GREEN}✓${NC} ASR model files installed successfully"
else
    echo -e "${GREEN}✓${NC} ASR model already exists"
fi

# Download VAD model
if [[ ! -f "models/vad/silero_vad/silero_vad.onnx" ]]; then
    echo -e "${BLUE}Downloading Silero VAD model...${NC}"
    wget -q --show-progress -O models/vad/silero_vad/silero_vad.onnx \
        https://github.com/snakers4/silero-vad/raw/master/files/silero_vad.onnx || {
        echo -e "${RED}✗${NC} Failed to download VAD model"
        exit 1
    }
    echo -e "${GREEN}✓${NC} VAD model downloaded successfully"
else
    echo -e "${GREEN}✓${NC} VAD model already exists"
fi

# Download Speaker model
if [[ ! -f "models/speaker/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx" ]]; then
    echo -e "${BLUE}Downloading 3DSpeaker model...${NC}"
    $PYTHON_CMD -c "
from modelscope import snapshot_download
model_dir = snapshot_download('fengge2024/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx', cache_dir='models_cache')
print(f'Speaker model downloaded to: {model_dir}')
" || {
        echo -e "${RED}✗${NC} Failed to download Speaker model"
        exit 1
    }
    
    # Copy speaker model
    find models_cache -name "3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx" -exec cp {} models/speaker/ \; 2>/dev/null || \
    find models_cache -name "*.onnx" -path "*/3dspeaker*" -exec cp {} models/speaker/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx \;
    echo -e "${GREEN}✓${NC} Speaker model downloaded successfully"
else
    echo -e "${GREEN}✓${NC} Speaker model already exists"
fi

# Clean up cache
if [ -d "models_cache" ]; then
    echo -e "${YELLOW}Cleaning up cache...${NC}"
    rm -rf models_cache
fi

# Deactivate venv if it was activated
if [ -n "$VIRTUAL_ENV" ]; then
    deactivate 2>/dev/null || true
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}All models downloaded successfully!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "You can now run the server with: ./dev.sh"

