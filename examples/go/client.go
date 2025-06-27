package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

type IOClient struct {
	baseURL string
	apiKey  string
}

func NewIOClient(baseURL, apiKey string) *IOClient {
	return &IOClient{
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

func (c *IOClient) StoreFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filePath)
	if err != nil {
		return "", err
	}
	
	if _, err = io.Copy(fw, file); err != nil {
		return "", err
	}
	w.Close()

	req, err := http.NewRequest("POST", c.baseURL+"/api/store", &buf)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	
	return result["sha1"], nil
}

func (c *IOClient) GetFile(sha1, outputPath string) error {
	req, err := http.NewRequest("GET", c.baseURL+"/api/file/"+sha1, nil)
	if err != nil {
		return err
	}
	
	req.Header.Set("X-API-Key", c.apiKey)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (c *IOClient) DeleteFile(sha1 string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/api/file/"+sha1, nil)
	if err != nil {
		return err
	}
	
	req.Header.Set("X-API-Key", c.apiKey)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	return nil
}

func (c *IOClient) FileExists(sha1 string) (bool, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/exists/"+sha1, nil)
	if err != nil {
		return false, err
	}
	
	req.Header.Set("X-API-Key", c.apiKey)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var result map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}
	
	return result["exists"], nil
}

func main() {
	client := NewIOClient("http://localhost:8080", "your-secure-api-key")
	
	// Store a file
	sha1, err := client.StoreFile("example.txt")
	if err != nil {
		panic(err)
	}
	fmt.Printf("File stored with SHA1: %s\n", sha1)
	
	// Check if file exists
	exists, err := client.FileExists(sha1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("File exists: %v\n", exists)
	
	// Download the file
	if err := client.GetFile(sha1, "downloaded.txt"); err != nil {
		panic(err)
	}
	fmt.Println("File downloaded successfully")
	
	// Delete the file
	if err := client.DeleteFile(sha1); err != nil {
		panic(err)
	}
	fmt.Println("File deleted successfully")
}