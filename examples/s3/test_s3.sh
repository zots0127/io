#!/bin/bash

# S3 API Test Script
# This script tests the S3-compatible API using AWS CLI

# Configuration
S3_ENDPOINT="http://localhost:9000"
ACCESS_KEY="minioadmin"
SECRET_KEY="minioadmin"
BUCKET="test-bucket"
REGION="us-east-1"

# Configure AWS CLI for local S3
aws configure set aws_access_key_id $ACCESS_KEY
aws configure set aws_secret_access_key $SECRET_KEY
aws configure set region $REGION

echo "Testing S3-compatible API..."
echo "=============================="

# Create bucket
echo "1. Creating bucket '$BUCKET'..."
aws s3api create-bucket --bucket $BUCKET --endpoint-url $S3_ENDPOINT

# Upload a file
echo "2. Uploading test file..."
echo "Hello, S3!" > test.txt
aws s3 cp test.txt s3://$BUCKET/test.txt --endpoint-url $S3_ENDPOINT

# List objects
echo "3. Listing objects in bucket..."
aws s3 ls s3://$BUCKET --endpoint-url $S3_ENDPOINT

# Download file
echo "4. Downloading file..."
aws s3 cp s3://$BUCKET/test.txt downloaded.txt --endpoint-url $S3_ENDPOINT
echo "Downloaded content:"
cat downloaded.txt

# Get object metadata
echo "5. Getting object metadata..."
aws s3api head-object --bucket $BUCKET --key test.txt --endpoint-url $S3_ENDPOINT

# Delete object
echo "6. Deleting object..."
aws s3 rm s3://$BUCKET/test.txt --endpoint-url $S3_ENDPOINT

# Delete bucket
echo "7. Deleting bucket..."
aws s3api delete-bucket --bucket $BUCKET --endpoint-url $S3_ENDPOINT

# Cleanup
rm test.txt downloaded.txt 2>/dev/null

echo "=============================="
echo "S3 API test completed!"