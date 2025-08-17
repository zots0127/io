#!/usr/bin/env python3
"""
S3-compatible storage client example
Requires: pip install boto3
"""

import boto3
from botocore.client import Config

# Configuration
S3_ENDPOINT = 'http://localhost:9000'
ACCESS_KEY = 'minioadmin'
SECRET_KEY = 'minioadmin'
BUCKET_NAME = 'test-bucket'

def main():
    # Create S3 client
    s3_client = boto3.client(
        's3',
        endpoint_url=S3_ENDPOINT,
        aws_access_key_id=ACCESS_KEY,
        aws_secret_access_key=SECRET_KEY,
        config=Config(signature_version='s3v4'),
        region_name='us-east-1'
    )
    
    print("S3-Compatible Storage Client Example")
    print("=" * 40)
    
    try:
        # 1. Create bucket
        print(f"1. Creating bucket '{BUCKET_NAME}'...")
        s3_client.create_bucket(Bucket=BUCKET_NAME)
        print("   ✓ Bucket created")
        
        # 2. List buckets
        print("\n2. Listing all buckets...")
        response = s3_client.list_buckets()
        for bucket in response['Buckets']:
            print(f"   - {bucket['Name']}")
        
        # 3. Upload file
        print("\n3. Uploading file...")
        test_content = b"Hello from Python S3 client!"
        s3_client.put_object(
            Bucket=BUCKET_NAME,
            Key='test-python.txt',
            Body=test_content,
            ContentType='text/plain'
        )
        print("   ✓ File uploaded")
        
        # 4. List objects in bucket
        print(f"\n4. Listing objects in '{BUCKET_NAME}'...")
        response = s3_client.list_objects_v2(Bucket=BUCKET_NAME)
        if 'Contents' in response:
            for obj in response['Contents']:
                print(f"   - {obj['Key']} (Size: {obj['Size']} bytes)")
        
        # 5. Download file
        print("\n5. Downloading file...")
        response = s3_client.get_object(Bucket=BUCKET_NAME, Key='test-python.txt')
        downloaded_content = response['Body'].read()
        print(f"   Downloaded content: {downloaded_content.decode()}")
        
        # 6. Get object metadata
        print("\n6. Getting object metadata...")
        response = s3_client.head_object(Bucket=BUCKET_NAME, Key='test-python.txt')
        print(f"   - ETag: {response['ETag']}")
        print(f"   - Content-Type: {response['ContentType']}")
        print(f"   - Content-Length: {response['ContentLength']}")
        
        # 7. Delete object
        print("\n7. Deleting object...")
        s3_client.delete_object(Bucket=BUCKET_NAME, Key='test-python.txt')
        print("   ✓ Object deleted")
        
        # 8. Delete bucket
        print(f"\n8. Deleting bucket '{BUCKET_NAME}'...")
        s3_client.delete_bucket(Bucket=BUCKET_NAME)
        print("   ✓ Bucket deleted")
        
    except Exception as e:
        print(f"\n❌ Error: {e}")
        return 1
    
    print("\n" + "=" * 40)
    print("✓ All tests completed successfully!")
    return 0

if __name__ == '__main__':
    exit(main())