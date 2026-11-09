#!/usr/bin/env python3
"""gBizINFO（公的データ）から L1 カタログのシード CSV を作る。

実在する法人だけを取り込むため、社名の生成は LLM に任せず、gBizINFO の
法人検索（社名キーワード × 従業員数）から集める。業種は社名キーワードによる
推定のため、取り込み前に CSV を目視で確認すること。

使い方:
  GBIZINFO_API_TOKEN=... python3 automation/l1_catalog/generate.py [件数] > Backend/config/l1_catalog_seed.csv
"""
import json
import unicodedata
import os
import sys
import time
import urllib.parse
import urllib.request

BASE = os.environ.get("GBIZINFO_BASE_URL", "https://api.info.gbiz.go.jp/hojin/v2/hojin")
TOKEN = os.environ["GBIZINFO_API_TOKEN"]
TARGET = int(sys.argv[1]) if len(sys.argv) > 1 else 800

# 社名キーワード（IT 系）。先に書いたものを優先して採用する。
KEYWORDS = ["ソフト", "インフォメーション", "情報システム", "システムズ", "ソリューション", "コンピュータ", "システム"]
# 従業員数の区分: (下限, 上限, segment)。上から順に採用する。
RANGES = [(1000, None, "core"), (100, 999, "sme_si"), (30, 99, "sme_si")]
# IT 以外が混ざりやすい社名を除く。
EXCLUDE = ["建設", "工事", "不動産", "銀行", "信用金庫", "保険", "証券", "病院", "医療", "学校", "大学", "組合", "食品", "製薬", "運輸", "ホテル", "住宅", "レストラン", "フード", "外食", "生命", "損害", "鉄道", "電鉄", "百貨店", "クリニック", "介護", "福祉", "ガス", "海運"]


def search(**p):
    req = urllib.request.Request(
        BASE + "?" + urllib.parse.urlencode(p),
        headers={"X-hojinInfo-api-token": TOKEN, "Accept": "application/json"},
    )
    with urllib.request.urlopen(req, timeout=30) as r:
        data = json.load(r)
    time.sleep(1)  # API への負荷を抑える
    return data.get("hojin-infos") or []


def main():
    seen, names, rows = set(), set(), []
    for lo, hi, segment in RANGES:
        for kw in KEYWORDS:
            params = dict(name=kw, employee_number_from=lo, limit=1000, page=1)
            if hi:
                params["employee_number_to"] = hi
            for h in sorted(search(**params), key=lambda x: x["corporate_number"]):
                # 全角英数字を半角にそろえる（既存データとの重複を防ぐ）
                name, num = unicodedata.normalize("NFKC", h["name"]), h["corporate_number"]
                # 同名の別法人は取り込み時に衝突するため、社名でも重複を除く
                if num in seen or name in names or not name.startswith("株式会社") and not name.endswith("株式会社"):
                    continue
                if any(w in name for w in EXCLUDE):
                    continue
                seen.add(num)
                names.add(name)
                rows.append((name, segment))
                if len(rows) >= TARGET:
                    return emit(rows)
    emit(rows)


def emit(rows):
    print("name,industry,segment,website_url,publish")
    for name, segment in rows:
        print(f"{name},情報通信業,{segment},,true")
    print(f"{len(rows)}件を出力", file=sys.stderr)


main()
