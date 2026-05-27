"""
LoRA学習スクリプト（雛形）

概要:
- JSONL (prompt/completion 形式または openai_chat 形式) を読み取り、LoRA でファインチューニングするための雛形を提供します。
- 実際の学習は transformers/peft/accelerate 等の依存関係が必要です。依存がない場合は案内メッセージを出して終了します。

使い方例:
python rag/train/lora_train.py --input data/train.jsonl --output-dir /models/lora --dry-run

注意: デフォルトは dry-run で安全に動作します。実行環境に合わせて --dry-run を外して下さい。
"""
from __future__ import annotations

import argparse
import json
import logging
import os
import sys
from typing import Any, Dict, Iterable, List

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("lora_train")


def load_jsonl(path: str) -> List[Dict[str, Any]]:
    examples: List[Dict[str, Any]] = []
    with open(path, "r", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            examples.append(json.loads(line))
    return examples


def prepare_prompt_completion(examples: List[Dict[str, Any]]) -> List[Dict[str, str]]:
    """Normalize input into prompt/completion pairs.
    Supports objects with keys (prompt, completion) or openai_chat style {"messages": [...]}
    """
    out: List[Dict[str, str]] = []
    for ex in examples:
        if "prompt" in ex and "completion" in ex:
            out.append({"prompt": ex["prompt"], "completion": ex["completion"]})
        elif "messages" in ex and isinstance(ex["messages"], list):
            # find last user->assistant pair
            msgs = ex["messages"]
            for i in range(len(msgs) - 1):
                if msgs[i].get("role") == "user" and msgs[i + 1].get("role") in ("assistant", "ai"):
                    out.append({"prompt": msgs[i].get("content", ""), "completion": msgs[i + 1].get("content", "")})
    return out


def check_dependencies() -> bool:
    try:
        import transformers  # type: ignore
        import peft  # type: ignore
        import torch  # type: ignore
        return True
    except Exception:
        return False


def train_lora(
    train_data: List[Dict[str, str]],
    output_dir: str,
    epochs: int = 1,
    batch_size: int = 8,
    lr: float = 1e-4,
    dry_run: bool = True,
) -> None:
    """簡易な学習ワークフローの雛形。

    実行環境に transformers/peft がある場合のみ本格的に動きます。
    依存がない場合はインストール方法を表示して終了します。
    """
    if not check_dependencies():
        logger.error("依存関係が見つかりません: transformers, peft, torch が必要です")
        logger.info("インストール例: pip install transformers peft accelerate bitsandbytes --upgrade")
        sys.exit(2)

    # 遅延インポート（依存確定後）
    import torch  # type: ignore
    from transformers import AutoTokenizer, AutoModelForCausalLM, TrainingArguments, Trainer  # type: ignore
    from peft import LoraConfig, get_peft_model, prepare_model_for_kbit_training  # type: ignore

    # 設定（実運用では引数/設定ファイルへ移す）
    base_model = os.getenv("BASE_MODEL", "gpt2")
    logger.info("base_model=%s output_dir=%s dry_run=%s", base_model, output_dir, dry_run)

    # トークナイズとデータセット準備（非常に簡易）
    tokenizer = AutoTokenizer.from_pretrained(base_model)
    tokenizer.pad_token = tokenizer.eos_token

    prompts = [d["prompt"] for d in train_data]
    completions = [d["completion"] for d in train_data]
    texts = [p + tokenizer.eos_token + c for p, c in zip(prompts, completions)]

    # トークン化
    encodings = tokenizer(texts, return_tensors="pt", padding=True, truncation=True)

    # モデル読み込み
    model = AutoModelForCausalLM.from_pretrained(base_model, torch_dtype=torch.float16 if torch.cuda.is_available() else torch.float32)
    model = prepare_model_for_kbit_training(model)

    lora_config = LoraConfig(
        r=8,
        lora_alpha=32,
        target_modules=["q_proj", "v_proj"],
        lora_dropout=0.05,
        bias="none",
        task_type="CAUSAL_LM",
    )
    model = get_peft_model(model, lora_config)

    # Dataset wrapper
    class SimpleDataset(torch.utils.data.Dataset):
        def __init__(self, enc):
            self.input_ids = enc["input_ids"]
            self.attn = enc["attention_mask"]

        def __len__(self) -> int:
            return self.input_ids.size(0)

        def __getitem__(self, idx: int):
            return {"input_ids": self.input_ids[idx], "attention_mask": self.attn[idx], "labels": self.input_ids[idx]}

    dataset = SimpleDataset(encodings)

    if dry_run:
        logger.info("dry-run: prepared dataset samples=%d, model params=%d", len(dataset), sum(p.numel() for p in model.parameters()))
        return

    # Training arguments
    training_args = TrainingArguments(
        output_dir=output_dir,
        per_device_train_batch_size=batch_size,
        num_train_epochs=epochs,
        learning_rate=lr,
        fp16=torch.cuda.is_available(),
        logging_steps=10,
        save_total_limit=2,
    )

    trainer = Trainer(
        model=model,
        args=training_args,
        train_dataset=dataset,
    )

    trainer.train()
    trainer.save_model(output_dir)
    logger.info("training finished, model saved to %s", output_dir)


def main(argv: List[str] | None = None) -> int:
    p = argparse.ArgumentParser(description="LoRA 学習雛形スクリプト")
    p.add_argument("--input", "-i", required=True, help="入力 JSONL (prompt/completion または openai_chat 形式)")
    p.add_argument("--output-dir", "-o", required=True, help="モデル出力ディレクトリ")
    p.add_argument("--epochs", type=int, default=1)
    p.add_argument("--batch-size", type=int, default=8)
    p.add_argument("--lr", type=float, default=1e-4)
    p.add_argument("--dry-run", action="store_true", default=True, help="学習を実行せず準備のみ行う")
    args = p.parse_args(argv)

    examples = load_jsonl(args.input)
    train_pairs = prepare_prompt_completion(examples)
    if not train_pairs:
        logger.error("入力データに有効なトレーニングペアが見つかりません")
        return 2

    os.makedirs(args.output_dir, exist_ok=True)

    train_lora(train_pairs, args.output_dir, epochs=args.epochs, batch_size=args.batch_size, lr=args.lr, dry_run=args.dry_run)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
