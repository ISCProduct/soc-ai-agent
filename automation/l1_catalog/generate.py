#!/usr/bin/env python3
"""gBizINFO（公的データ）から L1 カタログのシード CSV を作る。

実在する法人だけを取り込むため、社名の生成は LLM に任せず、gBizINFO の
法人検索（社名キーワード × 従業員数）から集める。業種は社名キーワードによる
推定のため、取り込み前に CSV を目視で確認すること。

使い方:
  GBIZINFO_API_TOKEN=... python3 automation/l1_catalog/generate.py [件数] [最小従業員数] > Backend/config/l1_catalog_seed.csv

4万社規模の候補が欲しいときは、最小従業員数を 0 にする（従業員数で絞らず、社名キーワードだけで集める）。
"""
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

BASE = os.environ.get("GBIZINFO_BASE_URL", "https://api.info.gbiz.go.jp/hojin/v2/hojin")
TOKEN = os.environ["GBIZINFO_API_TOKEN"]
TARGET = int(sys.argv[1]) if len(sys.argv) > 1 else 800
MIN_EMP = int(sys.argv[2]) if len(sys.argv) > 2 else 30

# 社名キーワード（IT 系）。先に書いたものを優先して採用する。
KEYWORDS = ["ソフト", "インフォメーション", "情報システム", "システムズ", "ソリューション", "コンピュータ", "システム", "情報", "ネットワーク", "データ", "デジタル", "テクノロジ", "インフォ", "ＩＴ", "アイティー", "ネット", "クラウド", "ＡＩ", "ウェブ", "ウエブ", "ドットコム", "テック", "エンジニアリング"]
# 従業員数の区分: (下限, 上限, segment)。上から順に採用する。
# MIN_EMP が 0 のときは従業員数で絞らない。
RANGES = [(1000, None, "core"), (100, 999, "sme_si"), (MIN_EMP, 99, "sme_si")] if MIN_EMP else [(0, None, "sme_si")]
# IT 以外が混ざりやすい社名を除く。
EXCLUDE = ["建設", "工事", "不動産", "銀行", "信用金庫", "保険", "証券", "病院", "医療", "学校", "大学", "組合", "食品", "製薬", "運輸", "ホテル", "住宅", "レストラン", "フード", "外食", "生命", "損害", "鉄道", "電鉄", "百貨店", "クリニック", "介護", "福祉", "ガス", "海運"]


def search(**p):
    req = urllib.request.Request(
        BASE + "?" + urllib.parse.urlencode(p),
        headers={"X-hojinInfo-api-token": TOKEN, "Accept": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as r:
            data = json.load(r)
    except urllib.error.HTTPError as e:
        if e.code == 404:  # 該当なし、またはページが尽きた
            return []
        raise
    time.sleep(1)  # API への負荷を抑える
    return data.get("hojin-infos") or []


def pages(params):
    """gBizINFO は 1 条件あたり 7 ページ（7,000 件）までしか返さないため、取れるだけ取る。"""
    out = []
    for page in range(1, 11):
        got = search(**params, page=page)
        out += got
        if len(got) < params["limit"]:
            break
    return out


def main():
    seen, names, rows = set(), set(), []
    for lo, hi, segment in RANGES:
        for kw in KEYWORDS:
            params = dict(name=kw, limit=1000)
            if lo:
                params["employee_number_from"] = lo
            if hi:
                params["employee_number_to"] = hi
            for h in sorted(pages(params), key=lambda x: x["corporate_number"]):
                # 社名は gBizINFO のまま使う。既存データ（相関図シード等）も全角のまま持つため、そろえると別会社として重複する。
                name, num = h["name"], h["corporate_number"]
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
