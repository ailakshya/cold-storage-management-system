package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cold-backend/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// StorageObject represents a file or directory in any storage backend.
type StorageObject struct {
	Name    string    `json:"name"`
	Key     string    `json:"key"`      // Full path/key relative to root
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// StorageBackend abstracts local filesystem vs S3-compatible storage operations.
// Both Cloudflare R2 and MinIO (NAS) implement this via S3Backend.
type StorageBackend interface {
	// List returns objects under the given prefix.
	// For local: lists directory contents. For S3: ListObjectsV2 with delimiter.
	List(ctx context.Context, prefix string) ([]StorageObject, error)

	// Download returns a ReadCloser for the object content and its size.
	// Caller must close the reader.
	Download(ctx context.Context, key string) (io.ReadCloser, int64, error)

	// Upload stores content at the given key.
	Upload(ctx context.Context, key string, reader io.Reader, size int64) error

	// Delete removes the object at the given key.
	Delete(ctx context.Context, key string) error

	// Stat returns metadata for a single object without downloading it.
	Stat(ctx context.Context, key string) (*StorageObject, error)

	// Exists checks whether an object exists at the given key.
	Exists(ctx context.Context, key string) (bool, error)

	// Move renames/moves an object within the same backend.
	Move(ctx context.Context, srcKey, dstKey string) error

	// Name returns a human-readable backend identifier ("local", "r2", "nas").
	Name() string
}

// ---------------------------------------------------------------------------
// LocalBackend — wraps os.* calls for local filesystem storage
// ---------------------------------------------------------------------------

type LocalBackend struct {
	baseDir string
	name    string
}

func NewLocalBackend(baseDir, name string) *LocalBackend {
	return &LocalBackend{baseDir: baseDir, name: name}
}

func (b *LocalBackend) Name() string { return b.name }

// resolve validates and resolves a key to an absolute filesystem path,
// preventing directory traversal outside baseDir.
func (b *LocalBackend) resolve(key string) (string, error) {
	if strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid path: contains '..'")
	}
	full := filepath.Join(b.baseDir, filepath.FromSlash(key))
	if !strings.HasPrefix(full, b.baseDir) {
		return "", fmt.Errorf("path escapes base directory")
	}
	return full, nil
}

func (b *LocalBackend) List(_ context.Context, prefix string) ([]StorageObject, error) {
	dir, err := b.resolve(prefix)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var objects []StorageObject
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		relPath := filepath.ToSlash(filepath.Join(prefix, entry.Name()))
		objects = append(objects, StorageObject{
			Name:    entry.Name(),
			Key:     relPath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	return objects, nil
}

func (b *LocalBackend) Download(_ context.Context, key string) (io.ReadCloser, int64, error) {
	full, err := b.resolve(key)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(full)
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

func (b *LocalBackend) Upload(_ context.Context, key string, reader io.Reader, _ int64) error {
	full, err := b.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	f, err := os.Create(full)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, reader)
	return err
}

func (b *LocalBackend) Delete(_ context.Context, key string) error {
	full, err := b.resolve(key)
	if err != nil {
		return err
	}
	return os.RemoveAll(full)
}

func (b *LocalBackend) Stat(_ context.Context, key string) (*StorageObject, error) {
	full, err := b.resolve(key)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	return &StorageObject{
		Name:    info.Name(),
		Key:     key,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

func (b *LocalBackend) Exists(_ context.Context, key string) (bool, error) {
	full, err := b.resolve(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(full)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (b *LocalBackend) Move(_ context.Context, srcKey, dstKey string) error {
	src, err := b.resolve(srcKey)
	if err != nil {
		return err
	}
	dst, err := b.resolve(dstKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	return os.Rename(src, dst)
}

// ---------------------------------------------------------------------------
// S3Backend — wraps aws-sdk-go-v2 S3 client (Cloudflare R2, MinIO, AWS S3)
// ---------------------------------------------------------------------------

type S3Backend struct {
	client *s3.Client
	bucket string
	label  string
}

// NewS3Backend creates a new S3-compatible storage backend.
// Works with Cloudflare R2, MinIO, AWS S3, or any S3-compatible service.
func NewS3Backend(ctx context.Context, endpoint, accessKey, secretKey, bucket, region, label string) (*S3Backend, error) {
	if region == "" {
		region = "auto"
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		)),
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("configure S3 client for %s: %w", label, err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // Required for MinIO and R2
	})

	return &S3Backend{
		client: client,
		bucket: bucket,
		label:  label,
	}, nil
}

func (b *S3Backend) Name() string  { return b.label }
func (b *S3Backend) Client() *s3.Client { return b.client }
func (b *S3Backend) Bucket() string     { return b.bucket }

func (b *S3Backend) List(ctx context.Context, prefix string) ([]StorageObject, error) {
	// Normalize prefix for directory listing
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var objects []StorageObject
	var continuationToken *string

	for {
		result, err := b.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(b.bucket),
			Prefix:            aws.String(prefix),
			Delimiter:         aws.String("/"),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list objects in %s: %w", b.label, err)
		}

		// Common prefixes represent "subdirectories"
		for _, cp := range result.CommonPrefixes {
			dirKey := aws.ToString(cp.Prefix)
			name := strings.TrimSuffix(strings.TrimPrefix(dirKey, prefix), "/")
			if name != "" {
				objects = append(objects, StorageObject{
					Name:  name,
					Key:   dirKey,
					IsDir: true,
				})
			}
		}

		// Contents represent files at this level
		for _, obj := range result.Contents {
			key := aws.ToString(obj.Key)
			name := strings.TrimPrefix(key, prefix)
			if name == "" || name == "/" {
				continue // skip the prefix itself
			}
			var modTime time.Time
			if obj.LastModified != nil {
				modTime = *obj.LastModified
			}
			objects = append(objects, StorageObject{
				Name:    name,
				Key:     key,
				IsDir:   false,
				Size:    aws.ToInt64(obj.Size),
				ModTime: modTime,
			})
		}

		if result.IsTruncated == nil || !*result.IsTruncated {
			break
		}
		continuationToken = result.NextContinuationToken
	}

	// Sort: directories first, then alphabetically
	sort.Slice(objects, func(i, j int) bool {
		if objects[i].IsDir != objects[j].IsDir {
			return objects[i].IsDir
		}
		return objects[i].Name < objects[j].Name
	})

	return objects, nil
}

func (b *S3Backend) Download(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	result, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("download from %s: %w", b.label, err)
	}
	return result.Body, aws.ToInt64(result.ContentLength), nil
}

func (b *S3Backend) Upload(ctx context.Context, key string, reader io.Reader, size int64) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
		Body:   reader,
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}

	_, err := b.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("upload to %s: %w", b.label, err)
	}
	return nil
}

func (b *S3Backend) Delete(ctx context.Context, key string) error {
	// If key looks like a directory, delete all objects under it
	if strings.HasSuffix(key, "/") {
		return b.deletePrefix(ctx, key)
	}

	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete from %s: %w", b.label, err)
	}
	return nil
}

// deletePrefix removes all objects under a prefix (S3 "directory" delete).
func (b *S3Backend) deletePrefix(ctx context.Context, prefix string) error {
	var continuationToken *string

	for {
		result, err := b.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(b.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return fmt.Errorf("list for delete in %s: %w", b.label, err)
		}
		if len(result.Contents) == 0 {
			break
		}

		var delObjects []s3types.ObjectIdentifier
		for _, obj := range result.Contents {
			delObjects = append(delObjects, s3types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		_, err = b.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(b.bucket),
			Delete: &s3types.Delete{
				Objects: delObjects,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("batch delete in %s: %w", b.label, err)
		}

		if result.IsTruncated == nil || !*result.IsTruncated {
			break
		}
		continuationToken = result.NextContinuationToken
	}
	return nil
}

func (b *S3Backend) Stat(ctx context.Context, key string) (*StorageObject, error) {
	result, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("stat in %s: %w", b.label, err)
	}

	name := key
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		name = key[idx+1:]
	}

	var modTime time.Time
	if result.LastModified != nil {
		modTime = *result.LastModified
	}

	return &StorageObject{
		Name:    name,
		Key:     key,
		IsDir:   false,
		Size:    aws.ToInt64(result.ContentLength),
		ModTime: modTime,
	}, nil
}

func (b *S3Backend) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "NotFound") ||
			strings.Contains(errMsg, "404") ||
			strings.Contains(errMsg, "NoSuchKey") {
			return false, nil
		}
		return false, fmt.Errorf("check existence in %s: %w", b.label, err)
	}
	return true, nil
}

func (b *S3Backend) Move(ctx context.Context, srcKey, dstKey string) error {
	// S3 has no native move — copy then delete
	copySource := b.bucket + "/" + srcKey

	_, err := b.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(b.bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(dstKey),
	})
	if err != nil {
		return fmt.Errorf("copy in %s: %w", b.label, err)
	}

	_, err = b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(srcKey),
	})
	if err != nil {
		return fmt.Errorf("delete source after move in %s: %w", b.label, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Factory functions — create pre-configured backends for R2 and NAS
// ---------------------------------------------------------------------------

// NewR2MediaBackend creates an S3Backend for Cloudflare R2 media bucket.
func NewR2MediaBackend(ctx context.Context) (*S3Backend, error) {
	return NewS3Backend(ctx,
		config.R2Endpoint,
		config.R2AccessKey,
		config.R2SecretKey,
		config.R2MediaBucketName,
		config.R2Region,
		"r2",
	)
}

// NewNASBackend creates an S3Backend for MinIO on TrueNAS.
func NewNASBackend(ctx context.Context, nasCfg config.NASConfig) (*S3Backend, error) {
	if !nasCfg.Enabled {
		return nil, fmt.Errorf("NAS storage not configured (NAS_S3_ENDPOINT not set)")
	}
	return NewS3Backend(ctx,
		nasCfg.Endpoint,
		nasCfg.AccessKey,
		nasCfg.SecretKey,
		nasCfg.Bucket,
		"us-east-1", // MinIO default region
		"nas",
	)
}

// ---------------------------------------------------------------------------
// Cross-backend utilities
// ---------------------------------------------------------------------------

// CrossBackendTransfer copies a file from one backend to another.
// Used for cross-storage moves (e.g., local -> R2, NAS -> local).
func CrossBackendTransfer(ctx context.Context, src StorageBackend, srcKey string, dst StorageBackend, dstKey string) error {
	reader, size, err := src.Download(ctx, srcKey)
	if err != nil {
		return fmt.Errorf("download from %s: %w", src.Name(), err)
	}
	defer reader.Close()

	if err := dst.Upload(ctx, dstKey, reader, size); err != nil {
		return fmt.Errorf("upload to %s: %w", dst.Name(), err)
	}
	return nil
}

// CrossBackendMove copies a file between backends, then deletes the source.
func CrossBackendMove(ctx context.Context, src StorageBackend, srcKey string, dst StorageBackend, dstKey string) error {
	if err := CrossBackendTransfer(ctx, src, srcKey, dst, dstKey); err != nil {
		return err
	}
	return src.Delete(ctx, srcKey)
}

// DownloadWithFallback tries to download a file from multiple backends in order.
// Returns the reader, size, and backend name on success, or error if all fail.
func DownloadWithFallback(ctx context.Context, key string, backends ...StorageBackend) (io.ReadCloser, int64, string, error) {
	var lastErr error
	for _, b := range backends {
		if b == nil {
			continue
		}
		reader, size, err := b.Download(ctx, key)
		if err == nil {
			return reader, size, b.Name(), nil
		}
		lastErr = err
	}
	if lastErr == nil {
		return nil, 0, "", fmt.Errorf("no backends available")
	}
	return nil, 0, "", fmt.Errorf("all backends failed, last error: %w", lastErr)
}
