package interview

import (
	"encoding/json"

	"Backend/internal/openai"
)

// ReportResponseSchema は面接レポート採点の strict JSON Schema を返す。
// 評価ハーネス（aibench）が本番と同じスキーマで呼ぶために公開している。
func ReportResponseSchema() *openai.ResponseSchema { return interviewReportResponseSchema }

var interviewReportResponseSchema = &openai.ResponseSchema{
	Name: "interview_report",
	Schema: json.RawMessage(`{
  "type": "object",
  "properties": {
    "summary": {"type": "string"},
    "scores": {
      "type": "object",
      "properties": {
        "logic": {"type": "integer"},
        "specificity": {"type": "integer"},
        "ownership": {"type": "integer"},
        "communication": {"type": "integer"},
        "enthusiasm": {"type": "integer"}
      },
      "required": ["logic", "specificity", "ownership", "communication", "enthusiasm"],
      "additionalProperties": false
    },
    "evidence": {
      "type": "object",
      "properties": {
        "logic": {"type": "string"},
        "specificity": {"type": "string"},
        "ownership": {"type": "string"},
        "communication": {"type": "string"},
        "enthusiasm": {"type": "string"}
      },
      "required": ["logic", "specificity", "ownership", "communication", "enthusiasm"],
      "additionalProperties": false
    },
    "evidence_utterance_ids": {
      "type": "object",
      "properties": {
        "logic": {"anyOf": [{"type": "integer", "minimum": 1}, {"type": "null"}]},
        "specificity": {"anyOf": [{"type": "integer", "minimum": 1}, {"type": "null"}]},
        "ownership": {"anyOf": [{"type": "integer", "minimum": 1}, {"type": "null"}]},
        "communication": {"anyOf": [{"type": "integer", "minimum": 1}, {"type": "null"}]},
        "enthusiasm": {"anyOf": [{"type": "integer", "minimum": 1}, {"type": "null"}]}
      },
      "required": ["logic", "specificity", "ownership", "communication", "enthusiasm"],
      "additionalProperties": false
    },
    "category_breakdown": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "category": {"type": "string", "enum": ["自己紹介・志望動機", "職務経験・実績", "強み・弱み", "キャリアビジョン", "逆質問", "企業別の質問"]},
          "score": {"type": "integer"},
          "weakness": {"type": "string"},
          "practice": {"type": "string"}
        },
        "required": ["category", "score", "weakness", "practice"],
        "additionalProperties": false
      }
    },
    "strengths": {"type": "array", "items": {"type": "string"}},
    "improvements": {"type": "array", "items": {"type": "string"}},
    "teacher": {
      "type": "object",
      "properties": {
        "overall_comment": {"type": "string"},
        "detailed_evidence": {
          "type": "object",
          "properties": {
            "logic": {"type": "string"},
            "specificity": {"type": "string"},
            "ownership": {"type": "string"},
            "communication": {"type": "string"},
            "enthusiasm": {"type": "string"}
          },
          "required": ["logic", "specificity", "ownership", "communication", "enthusiasm"],
          "additionalProperties": false
        },
        "coaching_points": {"type": "array", "items": {"type": "string"}},
        "strengths_for_teacher": {"type": "array", "items": {"type": "string"}},
        "next_steps": {"type": "array", "items": {"type": "string"}}
      },
      "required": ["overall_comment", "detailed_evidence", "coaching_points", "strengths_for_teacher", "next_steps"],
      "additionalProperties": false
    }
  },
  "required": ["summary", "scores", "evidence", "evidence_utterance_ids", "category_breakdown", "strengths", "improvements", "teacher"],
  "additionalProperties": false
}`),
}
