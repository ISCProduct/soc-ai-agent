#!/usr/bin/env python3
"""指定チャンネルへ進捗入力のお願いを送る。投稿の収集・保存は行わない。"""

import argparse
import json
import os
from pathlib import Path
import re
import sys
import urllib.error
import urllib.request


DEFAULT_ENV_FILE = Path(__file__).with_name(".env.progress.local")


class NotificationError(Exception):
    """秘密情報を含まない、表示可能なエラー。"""


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


OPENER = urllib.request.build_opener(NoRedirect())


def validate_webhook(url):
    if not re.fullmatch(r"https://discord\.com/api/(?:v10/)?webhooks/[0-9]+/[A-Za-z0-9_-]+", url):
        raise NotificationError("DISCORD_PROGRESS_WEBHOOK_URL に有効なDiscord Webhook URLを設定してください。")
    return url


def build_payload():
    return {
        "username": "進捗リマインダー",
        "content": (
            "📝 **進捗共有のお願い**\n"
            "このチャンネルに、現在の進捗をメッセージで投稿してください。\n"
            "短くて大丈夫です。以下をコピーして記入できます。\n\n"
            "**取り組んだこと・進捗：**\n"
            "**次にやること：**\n"
            "**困っていること・相談したいこと：**（なければ「なし」）\n\n"
            "作業中の場合も、現時点の状況を共有してください。"
        ),
        "allowed_mentions": {"parse": []},
    }


def send_notification(url):
    request = urllib.request.Request(
        validate_webhook(url) + "?wait=true",
        data=json.dumps(build_payload(), ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json", "User-Agent": "ProgressReminder/1.0"},
        method="POST",
    )
    try:
        with OPENER.open(request, timeout=15) as response:
            result = json.load(response)
    except urllib.error.HTTPError as error:
        # HTTPError の文字列表現には Webhook URL が含まれ得る。
        status = error.code
        error.close()
        raise NotificationError(
            f"Discordへの通知に失敗しました（HTTP {status}）。"
            "設定・権限・レート制限を確認してください。自動再送はしていません。"
        ) from None
    except (urllib.error.URLError, OSError, ValueError):
        # タイムアウト時はサーバーで送信済みの場合もあるため、無条件リトライしない。
        raise NotificationError(
            "送信結果を確認できませんでした。チャンネルの履歴を確認してから再実行してください。"
        ) from None
    if not isinstance(result, dict) or not result.get("id"):
        raise NotificationError("投稿の確認情報がありません。チャンネルの履歴を確認してください。")


def load_webhook(env_file):
    value = os.environ.get("DISCORD_PROGRESS_WEBHOOK_URL")
    if value is not None:
        return validate_webhook(value.strip())
    if env_file.is_file():
        for line in env_file.read_text().splitlines():
            key, separator, value = line.partition("=")
            if separator and key.strip() == "DISCORD_PROGRESS_WEBHOOK_URL":
                return validate_webhook(value.strip().strip("\"'"))
    raise NotificationError("DISCORD_PROGRESS_WEBHOOK_URL が未設定です。環境変数またはローカル設定に追加してください。")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dry-run", action="store_true", help="通知文を表示するだけで送信しない")
    parser.add_argument("--env-file", type=Path, default=DEFAULT_ENV_FILE,
                        help="Webhook設定ファイル（既定: スクリプトと同じ場所の .env.progress.local）")
    args = parser.parse_args()
    if args.dry_run:
        print(json.dumps(build_payload(), ensure_ascii=False, indent=2))
        return 0
    try:
        send_notification(load_webhook(args.env_file))
    except NotificationError as error:
        print(str(error), file=sys.stderr)
        return 1
    except OSError:
        print("ローカル設定ファイルを読み込めませんでした。", file=sys.stderr)
        return 1
    print("進捗入力の通知を送信しました。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
