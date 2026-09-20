-- 出題時に選んだ評価軸を質問メッセージに残す。
--
-- これまで採点時に chat_score_updater.go の inferCategoryFromQuestion が
-- 質問文のキーワードから軸を推測し直していた。出題時には
-- chat_process_question.go が targetCategory を決めているのに、その情報を
-- 保存していなかったため、キーワードに当たらない質問文は既定の「技術志向」に
-- 落ちていた。結果として狙った軸は未評価のまま残り、次のターンでも同じ軸が
-- 選ばれて質問を浪費する。15問かけて6軸しか埋まらない原因(#1333)。
--
-- 「安定志向」の質問は特に落ちやすい。AI生成の文面が
-- 「長期的に働ける環境」等になるとキーワードに当たらず、
-- 全セッションで一度も測れていなかった。
--
-- assistant の質問行にだけ入る。user の回答行と既存行は NULL のままで、
-- 読む側は NULL なら従来どおり推測にフォールバックする。
ALTER TABLE chat_messages
  ADD COLUMN weight_category VARCHAR(100) NULL COMMENT '出題時に狙った評価軸(domain/valueobject/match.go の正典10種)' AFTER question_weight_id;
