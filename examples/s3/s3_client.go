package main

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	endpoint  = "http://localhost:9000"
	accessKey = "minioadmin"
	secretKey = "minioadmin"
	region    = "us-east-1"
	bucket    = "test-bucket"
)

func main() {
	// Create S3 session
	sess, err := session.NewSession(&aws.Config{
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		log.Fatal("Failed to create session:", err)
	}

	svc := s3.New(sess)

	fmt.Println("S3-Compatible Storage Go Client Example")
	fmt.Println("========================================")

	// 1. Create bucket
	fmt.Printf("1. Creating bucket '%s'...\n", bucket)
	_, err = svc.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		log.Printf("Failed to create bucket: %v", err)
	} else {
		fmt.Println("   ✓ Bucket created")
	}

	// 2. List buckets
	fmt.Println("\n2. Listing all buckets...")
	result, err := svc.ListBuckets(nil)
	if err != nil {
		log.Printf("Failed to list buckets: %v", err)
	} else {
		for _, b := range result.Buckets {
			fmt.Printf("   - %s\n", aws.StringValue(b.Name))
		}
	}

	// 3. Upload file
	fmt.Println("\n3. Uploading file...")
	content := []byte("Hello from Go S3 client!")
	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String("test-go.txt"),
		Body:        bytes.NewReader(content),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		log.Printf("Failed to upload file: %v", err)
	} else {
		fmt.Println("   ✓ File uploaded")
	}

	// 4. List objects
	fmt.Printf("\n4. Listing objects in '%s'...\n", bucket)
	listResult, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		log.Printf("Failed to list objects: %v", err)
	} else {
		for _, obj := range listResult.Contents {
			fmt.Printf("   - %s (Size: %d bytes)\n",
				aws.StringValue(obj.Key),
				aws.Int64Value(obj.Size))
		}
	}

	// 5. Download file
	fmt.Println("\n5. Downloading file...")
	getResult, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("test-go.txt"),
	})
	if err != nil {
		log.Printf("Failed to download file: %v", err)
	} else {
		defer getResult.Body.Close()
		downloadedContent, _ := io.ReadAll(getResult.Body)
		fmt.Printf("   Downloaded content: %s\n", string(downloadedContent))
	}

	// 6. Get object metadata
	fmt.Println("\n6. Getting object metadata...")
	headResult, err := svc.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("test-go.txt"),
	})
	if err != nil {
		log.Printf("Failed to get metadata: %v", err)
	} else {
		fmt.Printf("   - ETag: %s\n", aws.StringValue(headResult.ETag))
		fmt.Printf("   - Content-Type: %s\n", aws.StringValue(headResult.ContentType))
		fmt.Printf("   - Content-Length: %d\n", aws.Int64Value(headResult.ContentLength))
	}

	// 7. Delete object
	fmt.Println("\n7. Deleting object...")
	_, err = svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("test-go.txt"),
	})
	if err != nil {
		log.Printf("Failed to delete object: %v", err)
	} else {
		fmt.Println("   ✓ Object deleted")
	}

	// 8. Delete bucket
	fmt.Printf("\n8. Deleting bucket '%s'...\n", bucket)
	_, err = svc.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		log.Printf("Failed to delete bucket: %v", err)
	} else {
		fmt.Println("   ✓ Bucket deleted")
	}

	fmt.Println("\n========================================")
	fmt.Println("✓ All tests completed!")
}