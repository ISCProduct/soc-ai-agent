#!/usr/bin/env bash
# LoRA トレーニングの実行スケルトン（要: Hugging Face Transformers + PEFT 環境）
# 使い方: chmod +x rag/training/train_lora.sh && ./rag/training/train_lora.sh data/train_data.jsonl

set -euo pipefail
DATASET_PATH=${1:-data/train_data.jsonl}
OUTPUT_DIR=${2:-models/lora_model}

echo "データセット: ${DATASET_PATH}"

echo "依存パッケージのインストール例 (venv 推奨):"
echo "  pip install transformers datasets accelerate bitsandbytes peft" 

echo "トレーニングは実環境のGPU/コストに注意して実行してください。"

echo "（ここに実行コマンドを追加）"
# 例（警告: 環境依存）
# python3 train.py --dataset ${DATASET_PATH} --output_dir ${OUTPUT_DIR} --model_name 'meta-llama/Llama-2-7b' --lora_rank 8

