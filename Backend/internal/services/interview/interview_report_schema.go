package interview

import (
	"encoding/json"

	"Backend/internal/openai"
)

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
  "required": ["summary", "scores", "evidence", "evidence_utterance_ids", "strengths", "improvements", "teacher"],
  "additionalProperties": false
}`),
}
