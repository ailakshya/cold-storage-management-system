package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	endpoint   = "https://8ac6054e727fbfd99ced86c9705a5893.r2.cloudflarestorage.com"
	accessKey  = "290bc63d7d6900dd2ca59751b7456899"
	secretKey  = "038697927a70289e79774479aa0156c3193e3d9253cf970fdb42b5c1a09a55f7"
	bucketName = "cold-db-backups"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	// Use current date to find latest backup folder (IST = UTC+5:30)
	ist := time.FixedZone("IST", 5*3600+30*60)
	now := time.Now().In(ist)
	prefix := fmt.Sprintf("base/%s/%s/%s/%s/", now.Format("2006"), now.Format("01"), now.Format("02"), now.Format("15"))
	fmt.Printf("Searching in: %s\n", prefix)

	// List objects from today's current hour folder
	result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		fmt.Printf("Error listing objects: %v\n", err)
		return
	}

	// Sort by last modified (newest first)
	objects := result.Contents
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(*objects[j].LastModified)
	})

	if len(objects) == 0 {
		fmt.Println("No backups found")
		return
	}

	// Show latest 5
	fmt.Println("Latest backups:")
	for i, obj := range objects {
		if i >= 5 {
			break
		}
		fmt.Printf("  %d. %s (%d bytes) - %s\n", i+1, *obj.Key, obj.Size, obj.LastModified.Format("2006-01-02 15:04:05"))
	}

	// Download latest
	latest := objects[0]
	fmt.Printf("\nDownloading: %s\n", *latest.Key)

	getResult, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    latest.Key,
	})
	if err != nil {
		fmt.Printf("Error downloading: %v\n", err)
		return
	}
	defer getResult.Body.Close()

	parts := strings.Split(*latest.Key, "/")
	filename := parts[len(parts)-1]

	outFile, err := os.Create("/tmp/" + filename)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, getResult.Body)
	if err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		return
	}

	fmt.Printf("Downloaded to /tmp/%s (%d bytes)\n", filename, written)
}
