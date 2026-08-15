package shared

import "strings"

// ParseS3URI は "s3://bucket/key" 形式のURIを bucket と key に分解する。
func ParseS3URI(uri string) (bucket string, key string, ok bool) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", false
	}
	trimmed := strings.TrimPrefix(uri, "s3://")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
