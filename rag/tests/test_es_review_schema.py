import unittest
import re
from pathlib import Path


class TestESReviewSchema(unittest.TestCase):
    def test_company_strategy_field_exists(self):
        repo_root = Path(__file__).resolve().parents[1]
        models_py = repo_root / "models.py"
        self.assertTrue(models_py.exists(), f"{models_py} does not exist")
        text = models_py.read_text(encoding="utf-8")
        # Find the ESReviewResponse class block
        m = re.search(r"class ESReviewResponse\(BaseModel\):([\s\S]*?)(?=\n\nclass |\n\n@|\n\ndef |\Z)", text)
        self.assertIsNotNone(m, "ESReviewResponse class not found in models.py")
        class_block = m.group(1)
        self.assertIn("company_strategy", class_block, "company_strategy field missing in ESReviewResponse class")


if __name__ == "__main__":
    unittest.main()
