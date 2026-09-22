import io
import json
import unittest
import urllib.error
from unittest.mock import patch

from notify_progress import NotificationError, build_payload, send_notification, validate_webhook


class NotificationTest(unittest.TestCase):
    def test_reminder_explains_normal_channel_input_and_disables_mentions(self):
        payload = build_payload()
        self.assertIn("このチャンネル", payload["content"])
        self.assertIn("進捗", payload["content"])
        self.assertEqual(payload["allowed_mentions"], {"parse": []})
        self.assertLessEqual(len(payload["content"]), 2000)

    def test_rejects_invalid_urls_without_leaking_secret(self):
        for url in ["http://discord.com/api/webhooks/1/secret", "https://evil.test/secret",
                    "https://discord.com@evil.test/api/webhooks/1/secret",
                    "https://discord.com/api/webhooks/1/secret?redirect=secret"]:
            with self.assertRaises(NotificationError) as caught:
                validate_webhook(url)
            self.assertNotIn("secret", str(caught.exception))

    def test_send_waits_for_confirmation_without_bot_token(self):
        response = io.BytesIO(b'{"id":"123"}')
        with patch("notify_progress.OPENER.open", return_value=response) as request:
            send_notification("https://discord.com/api/webhooks/1/secret")
        req = request.call_args.args[0]
        self.assertEqual(req.full_url, "https://discord.com/api/webhooks/1/secret?wait=true")
        self.assertEqual(req.method, "POST")
        self.assertNotIn("Authorization", req.headers)
        self.assertEqual(json.loads(req.data), build_payload())

    def test_429_and_server_failure_are_not_reported_as_success_or_retried(self):
        for status in [401, 404, 429, 500]:
            error = urllib.error.HTTPError("https://discord.com/secret", status, "secret", {}, None)
            with patch("notify_progress.OPENER.open", side_effect=error) as request:
                with self.assertRaises(NotificationError) as caught:
                    send_notification("https://discord.com/api/webhooks/1/secret")
            self.assertIn(str(status), str(caught.exception))
            self.assertNotIn("secret", str(caught.exception))
            self.assertEqual(request.call_count, 1)

    def test_timeout_does_not_leak_url_or_automatically_resend(self):
        with patch("notify_progress.OPENER.open", side_effect=TimeoutError("secret")) as request:
            with self.assertRaisesRegex(NotificationError, "履歴") as caught:
                send_notification("https://discord.com/api/webhooks/1/secret")
        self.assertNotIn("secret", str(caught.exception))
        self.assertEqual(request.call_count, 1)

    def test_missing_message_confirmation_is_failure(self):
        with patch("notify_progress.OPENER.open", return_value=io.BytesIO(b'{}')):
            with self.assertRaises(NotificationError):
                send_notification("https://discord.com/api/webhooks/1/secret")


if __name__ == "__main__":
    unittest.main()
