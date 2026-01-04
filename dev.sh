#!/bin/bash
set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get project root directory
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

echo -e "${GREEN}ASR Server - Local Development Environment${NC}"
echo "=============================================="

# Set library paths for local development
export LD_LIBRARY_PATH="${PROJECT_ROOT}/lib:${PROJECT_ROOT}/lib/ten-vad/lib/Linux/x64:$LD_LIBRARY_PATH"
echo -e "${GREEN}✓${NC} Library paths configured"

# Check required model files
REQUIRED_FILES=(
    "models/vad/silero_vad/silero_vad.onnx"
    "models/asr/Fun-ASR-Nano-2512-8bit/model.onnx"
    "models/asr/Fun-ASR-Nano-2512-8bit/tokens.txt"
    "models/speaker/3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx"
)

MISSING_FILES=()

for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        MISSING_FILES+=("$file")
    fi
done

if [ ${#MISSING_FILES[@]} -ne 0 ]; then
    echo -e "${YELLOW}⚠${NC}  Missing required model files:"
    for file in "${MISSING_FILES[@]}"; do
        echo "  - $file"
    done
    echo ""
    echo "Run './scripts/download_models.sh' to download missing models."
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo -e "${GREEN}✓${NC} All required model files found"
fi

# Create logs directory if it doesn't exist
mkdir -p logs
echo -e "${GREEN}✓${NC} Logs directory ready"

# Determine which config file to use (default to config.json)
CONFIG_FILE="${CONFIG_FILE:-config.json}"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}✗${NC} Configuration file not found: $CONFIG_FILE"
    exit 1
fi
echo -e "${GREEN}✓${NC} Using configuration: $CONFIG_FILE"

# Export config file path for Go program
export CONFIG_FILE

echo ""
echo -e "${GREEN}Starting ASR Server in development mode...${NC}"
echo "Environment: Development"
echo "Config: $CONFIG_FILE"
echo "Press Ctrl+C to stop the server"
echo ""

# Run the application
go run main.go


