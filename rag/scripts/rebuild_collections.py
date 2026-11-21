"""企業データのコレクションを新しい世代へ作り直す（SOCAIAGENT-658）。

全件の再構築は、今のコレクションを消して作り直すと、その間は検索できず、途中で失敗すると空になる。
新しい世代（接尾辞つきの名前）へ文書・埋め込み・メタデータを写してから、RAG_COLLECTION_SUFFIX を
切り替えて使う。写している間も、今の世代で検索できる。埋め込みは写すだけなので OpenAI は呼ばない。

手順:
  1. python scripts/rebuild_collections.py --to-suffix g2 [--space cosine]
     （--from-suffix を省くと、今の RAG_COLLECTION_SUFFIX の世代から写す）
  2. 件数を確かめ、rag-review の RAG_COLLECTION_SUFFIX=g2 にして入れ替える
  3. 旧世代は 7日 残し（切り戻し用）、そのあと --drop-suffix <旧> で消す。接尾辞なしの世代は --drop-suffix ""

--space は新しい世代の距離関数（hnsw:space）。コレクションは作ったあとで距離関数を変えられないので、
cosine にする（SOCAIAGENT-554 / 660）ときはこの再構築で作り直す。
"""
from __future__ import annotations

import argparse
import os
import sys
from typing import Any, Dict, List, Optional

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import vector_store as vs  # noqa: E402

BATCH = 500


def copy_collections(client: Any, from_suffix: str, to_suffix: str, space: Optional[str] = None) -> Dict[str, int]:
    """3つの企業データのコレクションを from の世代から to の世代へ写し、論理名ごとの件数を返す。"""
    if from_suffix == to_suffix:
        raise ValueError("写し元と写し先の世代が同じ")
    existing = vs._collection_names(client)
    copied: Dict[str, int] = {}
    for logical in vs.ALL_COLLECTIONS:
        src_name = vs.physical_collection_name(logical, from_suffix)
        dst_name = vs.physical_collection_name(logical, to_suffix)
        metadata = {"hnsw:space": space} if space else None
        dst = client.get_or_create_collection(dst_name, metadata=metadata) if metadata else client.get_or_create_collection(dst_name)
        if src_name not in existing:
            copied[logical] = 0
            continue
        src = client.get_collection(src_name)
        total = 0
        offset = 0
        while True:
            got = src.get(include=["documents", "embeddings", "metadatas"], limit=BATCH, offset=offset)
            ids: List[str] = list(got.get("ids") or [])
            if not ids:
                break
            dst.upsert(ids=ids, documents=got["documents"], embeddings=got["embeddings"], metadatas=got["metadatas"])
            total += len(ids)
            offset += len(ids)
        if dst.count() < total:
            raise RuntimeError(f"{dst_name}: 写した件数が足りない（{dst.count()} < {total}）")
        copied[logical] = total
    return copied


def drop_generation(client: Any, suffix: str) -> List[str]:
    """ある世代の企業データのコレクションを消す。今使っている世代は消さない。"""
    if suffix == vs.collection_suffix():
        raise ValueError("今使っている世代（RAG_COLLECTION_SUFFIX）は消せない")
    existing = vs._collection_names(client)
    dropped = []
    for logical in vs.ALL_COLLECTIONS:
        name = vs.physical_collection_name(logical, suffix)
        if name in existing:
            client.delete_collection(name)
            dropped.append(name)
    return dropped


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--to-suffix", help="写し先の世代の接尾辞（英数字・_・-）")
    parser.add_argument("--from-suffix", default=None, help="写し元の世代。省略時は今の RAG_COLLECTION_SUFFIX")
    parser.add_argument("--space", choices=["l2", "cosine", "ip"], default=None, help="新しい世代の距離関数")
    parser.add_argument("--drop-suffix", default=None, help="この世代のコレクションを消す（写しはしない）")
    args = parser.parse_args(argv)

    client = vs.get_chroma_client()
    if args.drop_suffix is not None:
        print("dropped:", drop_generation(client, args.drop_suffix))
        return 0
    if not args.to_suffix:
        parser.error("--to-suffix か --drop-suffix を指定する")
    from_suffix = vs.collection_suffix() if args.from_suffix is None else args.from_suffix
    copied = copy_collections(client, from_suffix, args.to_suffix, args.space)
    print(f"copied from={from_suffix or '(なし)'} to={args.to_suffix}: {copied}")
    print(f"件数を確かめてから RAG_COLLECTION_SUFFIX={args.to_suffix} に切り替える。旧世代は7日残す。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
