package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestUploadStorageDefaultsAndEnvironmentOverrides(t *testing.T) {
	t.Chdir(t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)
	defaults := Load().Upload
	if defaults.Storage != "local" || defaults.S3.Region != "us-east-1" || defaults.S3.TimeoutSeconds != 30 {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}
	t.Setenv("UPLOAD_STORAGE", "s3")
	t.Setenv("UPLOAD_S3_ENDPOINT", "http://minio:9000")
	t.Setenv("UPLOAD_S3_REGION", "test-region")
	t.Setenv("UPLOAD_S3_BUCKET", "shop-media")
	t.Setenv("UPLOAD_S3_ACCESS_KEY_ID", "test-access")
	t.Setenv("UPLOAD_S3_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("UPLOAD_S3_PUBLIC_BASE_URL", "https://shop.example.com/media")
	t.Setenv("UPLOAD_S3_KEY_PREFIX", "images")
	t.Setenv("UPLOAD_S3_TIMEOUT_SECONDS", "45")
	override := Load().Upload
	if override.Storage != "s3" || override.S3.Endpoint != "http://minio:9000" || override.S3.Region != "test-region" || override.S3.Bucket != "shop-media" || override.S3.AccessKeyID != "test-access" || override.S3.SecretAccessKey != "test-secret" || override.S3.PublicBaseURL != "https://shop.example.com/media" || override.S3.KeyPrefix != "images" || override.S3.TimeoutSeconds != 45 {
		t.Fatalf("environment overrides were not loaded")
	}
}
