package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"Backend/internal/models"
)

func (s *ResumeService) ensureS3Available() error {
	if s.s3Err != nil {
		return s.s3Err
	}
	return nil
}

func isS3URI(path string) bool {
	return strings.HasPrefix(path, "s3://")
}

func (s *ResumeService) ensureWorkingDir(docID uint) (string, error) {
	return os.MkdirTemp("", fmt.Sprintf("resume_work_%d_", docID))
}

func (s *ResumeService) resolveLocalPath(doc *models.ResumeDocument, workDir string) (string, error) {
	if !isS3URI(doc.StoredPath) {
		if strings.TrimSpace(doc.StoredPath) == "" && strings.TrimSpace(doc.SourceURL) != "" {
			if err := validateURL(doc.SourceURL); err != nil {
				return "", err
			}
			downloaded, _, err := downloadSourceFile(doc.SourceURL, workDir)
			if err != nil {
				return "", err
			}
			return downloaded, nil
		}
		return doc.StoredPath, nil
	}
	if err := s.ensureS3Available(); err != nil {
		return "", err
	}
	if s.s3 == nil || !s.s3.isEnabled() {
		return "", errors.New("s3 is not configured")
	}
	bucket, key, ok := parseS3URI(doc.StoredPath)
	if !ok {
		return "", errors.New("invalid s3 path")
	}
	if bucket != s.s3.bucket {
		return "", errors.New("s3 bucket mismatch")
	}
	ext := strings.ToLower(filepath.Ext(doc.OriginalFilename))
	if ext == "" {
		ext = ".pdf"
	}
	dest := filepath.Join(workDir, "original"+ext)
	if err := s.s3.downloadToFile(context.Background(), key, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func (s *ResumeService) s3KeyForDocument(doc *models.ResumeDocument, filename string) string {
	return s.s3.objectKey("resumes", fmt.Sprintf("%d", doc.UserID), fmt.Sprintf("%d", doc.ID), filename)
}

func (s *ResumeService) uploadToS3(ctx context.Context, doc *models.ResumeDocument, localPath, filename string) (string, error) {
	if s.s3 == nil || !s.s3.isEnabled() {
		return localPath, nil
	}
	if err := s.ensureS3Available(); err != nil {
		return "", err
	}
	key := s.s3KeyForDocument(doc, filename)
	return s.s3.uploadFile(ctx, key, localPath, contentTypeForPath(localPath))
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	return copyToWriter(in, out)
}
