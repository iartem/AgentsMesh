package file

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestUpload_Success(t *testing.T) {
	svc, mockStorage, _ := setupTestService(t)

	content := []byte("test image content")
	reader := bytes.NewReader(content)

	resp, err := svc.Upload(context.Background(), &UploadRequest{
		OrganizationID: 1,
		UploaderID:     100,
		FileName:       "test.png",
		ContentType:    "image/png",
		Size:           int64(len(content)),
		Reader:         reader,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.File == nil {
		t.Fatal("expected file in response")
	}
	if resp.File.OriginalName != "test.png" {
		t.Errorf("expected OriginalName 'test.png', got %s", resp.File.OriginalName)
	}
	if resp.File.MimeType != "image/png" {
		t.Errorf("expected MimeType 'image/png', got %s", resp.File.MimeType)
	}
	if resp.File.OrganizationID != 1 {
		t.Errorf("expected OrganizationID 1, got %d", resp.File.OrganizationID)
	}
	if resp.File.UploaderID != 100 {
		t.Errorf("expected UploaderID 100, got %d", resp.File.UploaderID)
	}
	if resp.URL == "" {
		t.Error("expected URL in response")
	}

	// Verify file was stored
	if mockStorage.FileCount() != 1 {
		t.Errorf("expected 1 file in storage, got %d", mockStorage.FileCount())
	}
}

func TestUpload_FileTooLarge(t *testing.T) {
	svc, _, _ := setupTestService(t)

	// MaxFileSize is 10MB, try to upload 11MB
	content := make([]byte, 11*1024*1024)
	reader := bytes.NewReader(content)

	_, err := svc.Upload(context.Background(), &UploadRequest{
		OrganizationID: 1,
		UploaderID:     100,
		FileName:       "large.png",
		ContentType:    "image/png",
		Size:           int64(len(content)),
		Reader:         reader,
	})

	if err == nil {
		t.Fatal("expected error for large file")
	}
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("expected ErrFileTooLarge, got %v", err)
	}
}

func TestUpload_InvalidFileType(t *testing.T) {
	svc, _, _ := setupTestService(t)

	content := []byte("test content")
	reader := bytes.NewReader(content)

	_, err := svc.Upload(context.Background(), &UploadRequest{
		OrganizationID: 1,
		UploaderID:     100,
		FileName:       "test.exe",
		ContentType:    "application/x-executable",
		Size:           int64(len(content)),
		Reader:         reader,
	})

	if err == nil {
		t.Fatal("expected error for invalid file type")
	}
	if !errors.Is(err, ErrInvalidFileType) {
		t.Errorf("expected ErrInvalidFileType, got %v", err)
	}
}

func TestUpload_StorageError(t *testing.T) {
	svc, mockStorage, _ := setupTestService(t)
	mockStorage.UploadErr = errors.New("storage unavailable")

	content := []byte("test content")
	reader := bytes.NewReader(content)

	_, err := svc.Upload(context.Background(), &UploadRequest{
		OrganizationID: 1,
		UploaderID:     100,
		FileName:       "test.png",
		ContentType:    "image/png",
		Size:           int64(len(content)),
		Reader:         reader,
	})

	if err == nil {
		t.Fatal("expected error for storage failure")
	}
	if !errors.Is(err, ErrStorageError) {
		t.Errorf("expected ErrStorageError, got %v", err)
	}
}
