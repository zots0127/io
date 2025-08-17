# IO 存储服务

[![Tests](https://github.com/zots0127/io/actions/workflows/test.yml/badge.svg)](https://github.com/zots0127/io/actions/workflows/test.yml)
[![CI/CD](https://github.com/zots0127/io/actions/workflows/ci.yml/badge.svg)](https://github.com/zots0127/io/actions/workflows/ci.yml)
[![Release](https://github.com/zots0127/io/actions/workflows/release.yml/badge.svg)](https://github.com/zots0127/io/releases)

[English Documentation](./README.md)

一个使用 Go 构建的轻量级文件存储服务，具备基于 SHA1 的内容去重和引用计数功能，可高效管理存储。

## 特性

- **内容寻址存储**：使用文件的 SHA1 哈希值作为唯一标识符
- **去重存储**：相同文件仅存储一次，通过引用计数管理
- **原子操作**：确保数据一致性的原子文件操作
- **RESTful API**：简单的 HTTP API，支持认证
- **高效结构**：两级目录结构，优化文件系统性能
- **引用计数**：安全删除机制，引用计数归零时自动清理

## 架构设计

### 存储结构
文件按两级目录层次结构存储：
```
storage/
├── 2f/
│   └── d4/
│       └── 2fd4e1c67a2d28fced849ee1bb76e7391b93eb12
```
其中 `2fd4e1c67a2d28fced849ee1bb76e7391b93eb12` 是文件内容的 SHA1 哈希值。

### 数据库模式
SQLite 数据库跟踪文件元数据：
- `sha1` (TEXT PRIMARY KEY)：文件哈希标识符
- `ref_count` (INTEGER)：文件引用次数
- `created_at` (DATETIME)：文件创建时间戳
- `last_accessed` (DATETIME)：最后访问时间戳

## 安装

### 前置要求
- Go 1.19 或更高版本
- SQLite 支持

### 从源码构建
```bash
# 克隆仓库
git clone <repository-url>
cd io

# 构建二进制文件
go build -o io .

# 或使用交互式构建脚本
./cicd.sh
```

### Docker
```bash
# 构建 Docker 镜像
docker build -t io .

# 运行容器
docker run -p 8080:8080 -v ./storage:/root/storage io
```

## 配置

创建 `config.yaml` 文件（从 `config.yaml.example` 复制）：

```yaml
storage:
  path: "./storage"      # 文件存储目录
  database: "./storage.db"  # SQLite 数据库路径

api:
  port: "8080"          # API 服务端口
  key: "your-secret-key"  # API 认证密钥
```

### 环境变量
- `CONFIG_PATH`：覆盖配置文件位置
- `IO_API_KEY`：覆盖配置中的 API 密钥

## API 文档

所有 API 端点都需要通过 `X-API-Key` 头进行认证。

### 认证
```http
X-API-Key: your-secret-key
```

### 接口列表

#### 1. 存储文件
上传文件到存储服务。

**请求：**
```http
POST /api/store
Content-Type: multipart/form-data
X-API-Key: your-secret-key

file: <binary-data>
```

**响应：**
```json
{
  "sha1": "2fd4e1c67a2d28fced849ee1bb76e7391b93eb12"
}
```

**状态码：**
- `200 OK`：文件存储成功
- `400 Bad Request`：未提供文件
- `401 Unauthorized`：无效的 API 密钥
- `500 Internal Server Error`：存储错误

#### 2. 获取文件
通过 SHA1 哈希下载文件。

**请求：**
```http
GET /api/file/{sha1}
X-API-Key: your-secret-key
```

**响应：**
- 二进制文件内容，`Content-Type: application/octet-stream`

**状态码：**
- `200 OK`：文件获取成功
- `400 Bad Request`：无效的 SHA1 格式
- `401 Unauthorized`：无效的 API 密钥
- `404 Not Found`：文件不存在
- `500 Internal Server Error`：读取错误

#### 3. 删除文件
删除文件或减少其引用计数。

**请求：**
```http
DELETE /api/file/{sha1}
X-API-Key: your-secret-key
```

**响应：**
```json
{
  "message": "File deleted"
}
```

**状态码：**
- `200 OK`：文件已删除或引用计数已减少
- `400 Bad Request`：无效的 SHA1 格式
- `401 Unauthorized`：无效的 API 密钥
- `500 Internal Server Error`：删除错误

**注意：** 只有当引用计数降至零时，文件才会被物理删除。

#### 4. 检查文件存在性
检查文件是否存在于存储中。

**请求：**
```http
GET /api/exists/{sha1}
X-API-Key: your-secret-key
```

**响应：**
```json
{
  "exists": true
}
```

**状态码：**
- `200 OK`：检查完成
- `400 Bad Request`：无效的 SHA1 格式
- `401 Unauthorized`：无效的 API 密钥

## 客户端示例

### cURL
```bash
# 存储文件
curl -X POST http://localhost:8080/api/store \
  -H "X-API-Key: your-secret-key" \
  -F "file=@/path/to/file.txt"

# 获取文件
curl -X GET http://localhost:8080/api/file/2fd4e1c67a2d28fced849ee1bb76e7391b93eb12 \
  -H "X-API-Key: your-secret-key" \
  -o downloaded-file.txt

# 删除文件
curl -X DELETE http://localhost:8080/api/file/2fd4e1c67a2d28fced849ee1bb76e7391b93eb12 \
  -H "X-API-Key: your-secret-key"

# 检查存在性
curl -X GET http://localhost:8080/api/exists/2fd4e1c67a2d28fced849ee1bb76e7391b93eb12 \
  -H "X-API-Key: your-secret-key"
```

### Python
```python
import requests

API_KEY = "your-secret-key"
BASE_URL = "http://localhost:8080"

# 存储文件
with open("file.txt", "rb") as f:
    response = requests.post(
        f"{BASE_URL}/api/store",
        headers={"X-API-Key": API_KEY},
        files={"file": f}
    )
    sha1 = response.json()["sha1"]

# 获取文件
response = requests.get(
    f"{BASE_URL}/api/file/{sha1}",
    headers={"X-API-Key": API_KEY}
)
with open("downloaded.txt", "wb") as f:
    f.write(response.content)
```

### Go
```go
package main

import (
    "bytes"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "os"
)

const (
    baseURL = "http://localhost:8080"
    apiKey  = "your-secret-key"
)

func storeFile(filePath string) (string, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return "", err
    }
    defer file.Close()

    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)
    part, err := writer.CreateFormFile("file", filePath)
    if err != nil {
        return "", err
    }
    io.Copy(part, file)
    writer.Close()

    req, err := http.NewRequest("POST", baseURL+"/api/store", &buf)
    if err != nil {
        return "", err
    }
    req.Header.Set("X-API-Key", apiKey)
    req.Header.Set("Content-Type", writer.FormDataContentType())

    // 执行请求并解析响应...
    return sha1Hash, nil
}
```

### JavaScript/Node.js
```javascript
const FormData = require('form-data');
const fs = require('fs');
const axios = require('axios');

const API_KEY = 'your-secret-key';
const BASE_URL = 'http://localhost:8080';

// 存储文件
async function storeFile(filePath) {
    const form = new FormData();
    form.append('file', fs.createReadStream(filePath));
    
    const response = await axios.post(`${BASE_URL}/api/store`, form, {
        headers: {
            ...form.getHeaders(),
            'X-API-Key': API_KEY
        }
    });
    
    return response.data.sha1;
}

// 获取文件
async function getFile(sha1, outputPath) {
    const response = await axios.get(`${BASE_URL}/api/file/${sha1}`, {
        headers: { 'X-API-Key': API_KEY },
        responseType: 'stream'
    });
    
    response.data.pipe(fs.createWriteStream(outputPath));
}
```

## 开发

### 运行测试
```bash
go test ./...
```

### 代码格式化
```bash
go fmt ./...
```

### 代码检查
```bash
go vet ./...
```

## 安全考虑

1. **API 密钥保护**：安全存储 API 密钥，切勿提交到版本控制
2. **文件验证**：服务验证 SHA1 格式以防止路径遍历攻击
3. **原子操作**：数据库事务确保元数据与文件系统之间的一致性
4. **资源清理**：引用计数归零时自动清理孤儿文件

## 性能优化

- **两级目录结构**：防止大量文件导致的文件系统性能下降
- **连接池**：数据库连接池配置以获得最佳性能
- **去重存储**：相同文件只存储一次，节省磁盘空间
- **高效哈希**：上传流中计算 SHA1，无需二次读取

## 项目结构

```
io/
├── main.go         # 主程序入口
├── api.go          # HTTP API 处理
├── storage.go      # 存储引擎实现
├── db.go           # 数据库操作
├── config.go       # 配置管理
├── config.yaml     # 配置文件
├── config.yaml.example  # 配置示例
├── Dockerfile      # Docker 构建文件
├── cicd.sh        # 交互式构建脚本
├── go.mod         # Go 模块定义
├── go.sum         # Go 模块校验和
├── README.md      # 英文文档
├── README_CN.md   # 中文文档
└── examples/      # 客户端示例
    ├── go/
    ├── python/
    └── javascript/
```

## 使用场景

- **微服务存储基座**：作为其他服务的统一存储后端
- **文件去重系统**：企业级文件管理，避免重复存储
- **内容分发**：基于哈希的内容寻址，适合 CDN 场景
- **备份系统**：增量备份，仅存储变化的文件
- **Docker 镜像存储**：类似 Docker Registry 的层存储

## 监控指标

建议监控以下指标：
- 存储使用率
- API 请求速率
- 文件去重率
- 数据库连接池状态
- 错误率和响应时间

## 故障排查

### 常见问题

1. **无法启动服务**
   - 检查端口是否被占用
   - 确认配置文件路径正确
   - 验证 API 密钥已设置

2. **文件上传失败**
   - 检查存储目录权限
   - 确认磁盘空间充足
   - 查看服务日志

3. **数据库错误**
   - 确认 SQLite 文件权限
   - 检查数据库文件完整性
   - 查看数据库连接池状态

## 许可证

MIT License

## 贡献

欢迎贡献代码！请随时提交 Pull Request。

## 支持

如有问题和建议，请在 GitHub 仓库中提交 Issue。