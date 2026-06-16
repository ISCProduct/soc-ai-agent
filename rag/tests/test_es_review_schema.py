import unittest
import re
from pathlib import Path


class TestESReviewSchema(unittest.TestCase):
    def test_company_strategy_field_exists(self):
        repo_root = Path(__file__).resolve().parents[1]
        main_py = repo_root / "main.py"
        self.assertTrue(main_py.exists(), f"{main_py} does not exist")
        text = main_py.read_text(encoding="utf-8")
        # Find the ESReviewResponse class block
        m = re.search(r"class ESReviewResponse\(BaseModel\):([\s\S]*?)(?=\n\nclass |\n\n@|\n\ndef |\Z)", text)
        self.assertIsNotNone(m, "ESReviewResponse class not found in main.py")
        class_block = m.group(1)
        self.assertIn("company_strategy", class_block, "company_strategy field missing in ESReviewResponse class")


if __name__ == "__main__":
    unittest.main()
