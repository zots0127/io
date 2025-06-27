import requests
import os

class IOClient:
    def __init__(self, base_url="http://localhost:8080", api_key="your-secure-api-key"):
        self.base_url = base_url
        self.api_key = api_key
        self.headers = {"X-API-Key": api_key}
    
    def store_file(self, file_path):
        """Store a file and return its SHA1 hash"""
        with open(file_path, 'rb') as f:
            files = {'file': (os.path.basename(file_path), f)}
            resp = requests.post(
                f"{self.base_url}/api/store",
                files=files,
                headers=self.headers
            )
            resp.raise_for_status()
            return resp.json()['sha1']
    
    def get_file(self, sha1, output_path):
        """Download a file by its SHA1 hash"""
        resp = requests.get(
            f"{self.base_url}/api/file/{sha1}",
            headers=self.headers,
            stream=True
        )
        resp.raise_for_status()
        
        with open(output_path, 'wb') as f:
            for chunk in resp.iter_content(chunk_size=8192):
                f.write(chunk)
    
    def delete_file(self, sha1):
        """Delete a file by its SHA1 hash"""
        resp = requests.delete(
            f"{self.base_url}/api/file/{sha1}",
            headers=self.headers
        )
        resp.raise_for_status()
        return resp.json()
    
    def file_exists(self, sha1):
        """Check if a file exists by its SHA1 hash"""
        resp = requests.get(
            f"{self.base_url}/api/exists/{sha1}",
            headers=self.headers
        )
        resp.raise_for_status()
        return resp.json()['exists']

# Example usage
if __name__ == "__main__":
    client = IOClient()
    
    # Create a test file
    with open("test.txt", "w") as f:
        f.write("Hello, IO Storage!")
    
    # Store the file
    sha1 = client.store_file("test.txt")
    print(f"File stored with SHA1: {sha1}")
    
    # Check if file exists
    exists = client.file_exists(sha1)
    print(f"File exists: {exists}")
    
    # Download the file
    client.get_file(sha1, "downloaded.txt")
    print("File downloaded successfully")
    
    # Read and verify content
    with open("downloaded.txt", "r") as f:
        content = f.read()
        print(f"Downloaded content: {content}")
    
    # Delete the file
    result = client.delete_file(sha1)
    print(f"Delete result: {result}")
    
    # Clean up
    os.remove("test.txt")
    os.remove("downloaded.txt")