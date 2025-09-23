# IO Server 测试指南

本文档提供了 IO Server 的完整测试方法，包括单元测试、集成测试和端到端测试。

## 目录

1. [环境准备](#环境准备)
2. [单元测试](#单元测试)
3. [Docker 测试](#docker-测试)
4. [集成测试](#集成测试)
5. [API 测试](#api-测试)
6. [性能测试](#性能测试)
7. [故障排查](#故障排查)

## 环境准备

### 依赖要求

- Go 1.21+
- Docker 和 Docker Compose
- curl 或其他 HTTP 客户端
- jq (可选，用于解析 JSON 响应)

### 配置文件

确保 `config.yaml` 文件配置正确：

```yaml
storage:
  path: ./uploads
  database: io.db

api:
  port: "8080"
  key: "test-api-key-12345"
  mode: hybrid  # native, s3, or hybrid

s3:
  enabled: true
  port: "9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  region: "us-east-1"
```

## 单元测试

### 运行所有测试

```bash
go test ./...
```

### 运行特定测试

```bash
go test -run TestFunctionName
```

### 查看测试覆盖率

```bash
go test -cover ./...
```

### 生成覆盖率报告

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Docker 测试

### 构建镜像

```bash
docker-compose build
```

### 启动服务

```bash
docker-compose up -d
```

### 查看日志

```bash
docker-compose logs -f
```

### 停止服务

```bash
docker-compose down
```

### 端口映射

- Native API: `localhost:8090` (映射到容器内 8080)
- S3 API: `localhost:9001` (映射到容器内 9000)

## 集成测试

### 使用自动化测试脚本

我们提供了两个测试脚本：

1. **基础测试** (`test_integration.sh`)
   - 测试基本的文件操作
   - 验证 Native API 和 S3 API

2. **完整测试** (`test_complete.sh`)
   - 全面的功能测试
   - 包含测试结果统计
   - 彩色输出显示通过/失败状态

运行测试：

```bash
# 基础测试
./test_integration.sh

# 完整测试
./test_complete.sh
```

## API 测试

### Native API 测试

#### 1. 上传文件

```bash
curl -X POST \
  -H "X-API-Key: test-api-key-12345" \
  -F "file=@test.txt" \
  http://localhost:8090/api/store
```

响应示例：
```json
{"sha1":"60fde9c2310b0d4cad4dab8d126b04387efba289"}
```

#### 2. 检查文件是否存在

```bash
curl -H "X-API-Key: test-api-key-12345" \
  http://localhost:8090/api/exists/{sha1}
```

响应示例：
```json
{"exists":true}
```

#### 3. 获取文件

```bash
curl -H "X-API-Key: test-api-key-12345" \
  http://localhost:8090/api/file/{sha1}
```

#### 4. 删除文件

```bash
curl -X DELETE \
  -H "X-API-Key: test-api-key-12345" \
  http://localhost:8090/api/file/{sha1}
```

### S3-Compatible API 测试

#### 1. 列出所有桶

```bash
curl http://localhost:9001/
```

#### 2. 创建桶

```bash
curl -X PUT http://localhost:9001/test-bucket
```

#### 3. 上传对象

```bash
curl -X PUT \
  -H "Content-Type: text/plain" \
  --data "Hello S3" \
  http://localhost:9001/test-bucket/hello.txt
```

#### 4. 列出桶中的对象

```bash
curl http://localhost:9001/test-bucket
```

#### 5. 获取对象

```bash
curl http://localhost:9001/test-bucket/hello.txt
```

#### 6. 获取对象元数据

```bash
curl -I http://localhost:9001/test-bucket/hello.txt
```

#### 7. 删除对象

```bash
curl -X DELETE http://localhost:9001/test-bucket/hello.txt
```

#### 8. 删除桶

```bash
curl -X DELETE http://localhost:9001/test-bucket
```

### 分片上传测试

#### 1. 初始化分片上传

```bash
curl -X POST \
  "http://localhost:9001/test-bucket/large-file.bin?uploads"
```

响应示例：
```xml
<InitiateMultipartUploadResult>
  <Bucket>test-bucket</Bucket>
  <Key>large-file.bin</Key>
  <UploadId>32f5efd7-5b34-48cd-884e-66ba9023defe</UploadId>
</InitiateMultipartUploadResult>
```

#### 2. 上传分片

```bash
curl -X PUT \
  --data-binary @part1.bin \
  "http://localhost:9001/test-bucket/large-file.bin?partNumber=1&uploadId={uploadId}"
```

#### 3. 完成分片上传

```bash
curl -X POST \
  -H "Content-Type: application/xml" \
  --data '<CompleteMultipartUpload>
    <Part>
      <PartNumber>1</PartNumber>
      <ETag>etag1</ETag>
    </Part>
  </CompleteMultipartUpload>' \
  "http://localhost:9001/test-bucket/large-file.bin?uploadId={uploadId}"
```

## 性能测试

### 使用 Apache Bench (ab)

测试并发上传：
```bash
ab -n 1000 -c 10 \
  -H "X-API-Key: test-api-key-12345" \
  -p test.txt \
  http://localhost:8090/api/store
```

### 使用 wrk

测试读取性能：
```bash
wrk -t12 -c400 -d30s \
  -H "X-API-Key: test-api-key-12345" \
  http://localhost:8090/api/file/{sha1}
```

### 使用 S3 Bench

测试 S3 API 性能：
```bash
s3bench \
  -endpoint http://localhost:9001 \
  -accessKey minioadmin \
  -secretKey minioadmin \
  -bucket test-bucket \
  -numClients 10 \
  -numSamples 100 \
  -objectSize 1024
```

## 故障排查

### 常见问题

#### 1. 端口占用

如果端口被占用，修改 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "8091:8080"  # 改用其他端口
  - "9002:9000"
```

#### 2. API Key 错误

确保在请求中包含正确的 API Key：
```bash
-H "X-API-Key: test-api-key-12345"
```

#### 3. 文件权限问题

确保 uploads 目录有正确的权限：
```bash
chmod 755 uploads
```

#### 4. Docker 容器启动失败

查看详细日志：
```bash
docker-compose logs io-server
```

### 调试模式

设置环境变量启用调试日志：
```bash
export GIN_MODE=debug
docker-compose up
```

### 健康检查

添加健康检查端点（如果需要）：
```bash
curl http://localhost:8090/health
```

## 测试检查清单

- [ ] 单元测试全部通过
- [ ] Docker 镜像成功构建
- [ ] 容器正常启动
- [ ] Native API 文件上传/下载/删除正常
- [ ] S3 API 桶操作正常
- [ ] S3 API 对象操作正常
- [ ] 分片上传功能正常
- [ ] 并发请求处理正常
- [ ] 错误处理和日志记录正常
- [ ] 性能指标符合预期

## 持续集成

GitHub Actions 配置示例：

```yaml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    
    - name: Set up Go
      uses: actions/setup-go@v2
      with:
        go-version: 1.21
    
    - name: Run tests
      run: go test ./...
    
    - name: Build Docker image
      run: docker-compose build
    
    - name: Run integration tests
      run: |
        docker-compose up -d
        sleep 5
        ./test_complete.sh
        docker-compose down
```

## 总结

本测试指南涵盖了 IO Server 的所有主要功能测试。建议在每次代码变更后运行完整的测试套件，确保系统稳定性和功能完整性。

如有任何问题或需要额外的测试场景，请参考源代码或联系开发团队。