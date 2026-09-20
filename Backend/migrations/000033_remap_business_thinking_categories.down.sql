-- ロールバック: ビジネス思考系を成長志向へ戻す（移行前の弱い統合）。

UPDATE `question_weights`
SET `weight_category` = '成長志向'
WHERE `question` LIKE '%どのような成果を出すことを重視%'
  AND `weight_category` = 'チャレンジ志向';

UPDATE `question_weights`
SET `weight_category` = '成長志向'
WHERE `question` LIKE '%顧客や利用者の視点%'
  AND `weight_category` = 'コミュニケーション力';

UPDATE `question_weights`
SET `weight_category` = '成長志向'
WHERE `question` LIKE '%どのような価値を社会や組織に提供%'
  AND `weight_category` = 'リーダーシップ志向';

UPDATE `ai_question_templates`
SET `category` = '成長志向'
WHERE `prompt` LIKE '%どのような成果を出すことを重視%'
  AND `category` = 'チャレンジ志向';

UPDATE `ai_question_templates`
SET `category` = '成長志向'
WHERE `prompt` LIKE '%顧客や利用者の視点%'
  AND `category` = 'コミュニケーション力';

UPDATE `ai_question_templates`
SET `category` = '成長志向'
WHERE `prompt` LIKE '%どのような価値を社会や組織に提供%'
  AND `category` = 'リーダーシップ志向';
