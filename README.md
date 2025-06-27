# IO - 轻量级存储基座

一个简单、高效的文件存储服务，具有SHA1去重功能，可作为其他项目的存储基础组件。

## 特性

- 🚀 **SHA1去重存储** - 相同内容只存储一次，节省空间
- 🔑 **简单认证** - 使用API密钥进行访问控制
- 📦 **轻量级** - 仅依赖SQLite，无需外部数据库
- 🛡️ **引用计数** - 安全的文件删除机制
- 🎯 **极简API** - 仅4个端点，易于集成

## 快速开始

### 二进制运行

```bash
go build -o io
./io
```

### Docker运行

```bash
docker build -t io .
docker run -p 8080:8080 -v ./storage:/root/storage io
```

## API使用

### 存储文件
```bash
curl -X POST http://localhost:8080/api/store \
  -H "X-API-Key: your-secure-api-key" \
  -F "file=@example.pdf"

# 响应: {"sha1":"2fd4e1c67a2d28fced849ee1bb76e7391b93eb12"}
```

### 获取文件
```bash
curl http://localhost:8080/api/file/2fd4e1c67a2d28fced849ee1bb76e7391b93eb12 \
  -H "X-API-Key: your-secure-api-key" \
  -o downloaded.pdf
```

### 删除文件
```bash
curl -X DELETE http://localhost:8080/api/file/2fd4e1c67a2d28fced849ee1bb76e7391b93eb12 \
  -H "X-API-Key: your-secure-api-key"
```

### 检查文件存在
```bash
curl http://localhost:8080/api/exists/2fd4e1c67a2d28fced849ee1bb76e7391b93eb12 \
  -H "X-API-Key: your-secure-api-key"

# 响应: {"exists":true}
```

## 配置

创建 `config.yaml` 文件：

```yaml
storage:
  path: ./storage      # 文件存储路径
  database: ./storage.db  # SQLite数据库路径

api:
  port: "8080"         # API服务端口
  key: "your-secure-api-key"  # API密钥
```

## 集成示例

### Go集成
```go
package main

import (
    "bytes"
    "encoding/json"
    "io"
    "mime/multipart"
    "net/http"
)

func StoreFile(data []byte, apiKey string) (string, error) {
    var buf bytes.Buffer
    w := multipart.NewWriter(&buf)
    fw, _ := w.CreateFormFile("file", "data")
    fw.Write(data)
    w.Close()
    
    req, _ := http.NewRequest("POST", "http://localhost:8080/api/store", &buf)
    req.Header.Set("X-API-Key", apiKey)
    req.Header.Set("Content-Type", w.FormDataContentType())
    
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    
    var result map[string]string
    json.NewDecoder(resp.Body).Decode(&result)
    return result["sha1"], nil
}
```

### Python集成
```python
import requests

def store_file(file_path, api_key):
    with open(file_path, 'rb') as f:
        files = {'file': f}
        headers = {'X-API-Key': api_key}
        resp = requests.post('http://localhost:8080/api/store', 
                           files=files, headers=headers)
        return resp.json()['sha1']

def get_file(sha1, api_key):
    headers = {'X-API-Key': api_key}
    resp = requests.get(f'http://localhost:8080/api/file/{sha1}', 
                       headers=headers)
    return resp.content
```

## 项目结构

```
io/
├── main.go       # 主程序入口
├── storage.go    # 存储逻辑
├── api.go        # HTTP API
├── config.go     # 配置管理
├── db.go         # 数据库操作
├── config.yaml   # 配置文件
└── Dockerfile    # Docker构建
```

## 存储原理

文件按SHA1哈希值组织存储：
```
storage/
├── 2f/
│   └── d4/
│       └── 2fd4e1c67a2d28fced849ee1bb76e7391b93eb12
└── a9/
    └── 4a/
        └── a94a8fe5ccb19ba61c4c0873d391e987982fbbd3
```

## 许可证

MIT License