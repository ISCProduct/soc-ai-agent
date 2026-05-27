# ファインチューニング実行ガイド (コンテナ利用)

このフォルダには LoRA/QLoRA によるファインチューニングの雛形スクリプトと、コンテナでの実行手順が含まれます。

ビルド:

  docker build -f rag/Dockerfile.finetune -t soc-agent-finetune:latest .

簡易 dry-run 実行例:

  docker run --rm -v $(pwd)/rag/tests:/app/rag/tests soc-agent-finetune:latest -i tests/sample_train.jsonl -o /tmp/lora --dry-run

本格実行時の注意:

- GPU 環境で実行する場合は nvidia/cuda ベースのイメージを利用し、--gpus フラグを渡してください。
- データは候補者の個人情報を匿名化した上で利用してください。
- 依存パッケージは requirements-finetune.txt を参照してください。

