package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

func uploadAudioToGCS(ctx context.Context, bucketName, localAudioPath string) (string, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create storage client: %w", err)
	}
	defer client.Close()

	file, err := os.Open(localAudioPath)
	if err != nil {
		return "", fmt.Errorf("failed to open local audio file: %w", err)
	}
	defer file.Close()

	objectName := fmt.Sprintf("audio-staging/%d-%s", time.Now().UnixNano(), filepath.Base(localAudioPath))
	wc := client.Bucket(bucketName).Object(objectName).NewWriter(ctx)
	wc.ContentType = "audio/mpeg"
	if strings.ToLower(filepath.Ext(localAudioPath)) == ".wav" {
		wc.ContentType = "audio/wav"
	}

	if _, err := io.Copy(wc, file); err != nil {
		_ = wc.Close()
		return "", fmt.Errorf("failed to upload audio to GCS: %w", err)
	}
	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS object: %w", err)
	}

	return fmt.Sprintf("gs://%s/%s", bucketName, objectName), nil
}

func deleteGCSObject(ctx context.Context, bucketName, gcsURI string) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return
	}
	defer client.Close()

	prefix := fmt.Sprintf("gs://%s/", bucketName)
	if len(gcsURI) > len(prefix) {
		objName := gcsURI[len(prefix):]
		_ = client.Bucket(bucketName).Object(objName).Delete(ctx)
	}
}
