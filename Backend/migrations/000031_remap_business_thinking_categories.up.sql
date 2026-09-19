-- ビジネス思考系の質問を成長志向から切り離す（診断品質）。
-- seed.go の新規投入分と揃え、既存DBの質問カテゴリを前方修正する。

UPDATE `question_weights`
SET `weight_category` = 'チャレンジ志向'
WHERE `question` LIKE '%どのような成果を出すことを重視%'
  AND `weight_category` = '成長志向';

UPDATE `question_weights`
SET `weight_category` = 'コミュニケーション力'
WHERE `question` LIKE '%顧客や利用者の視点%'
  AND `weight_category` = '成長志向';

UPDATE `question_weights`
SET `weight_category` = 'リーダーシップ志向'
WHERE `question` LIKE '%どのような価値を社会や組織に提供%'
  AND `weight_category` = '成長志向';

UPDATE `ai_question_templates`
SET `category` = 'チャレンジ志向'
WHERE `prompt` LIKE '%どのような成果を出すことを重視%'
  AND `category` = '成長志向';

UPDATE `ai_question_templates`
SET `category` = 'コミュニケーション力'
WHERE `prompt` LIKE '%顧客や利用者の視点%'
  AND `category` = '成長志向';

UPDATE `ai_question_templates`
SET `category` = 'リーダーシップ志向'
WHERE `prompt` LIKE '%どのような価値を社会や組織に提供%'
  AND `category` = '成長志向';
