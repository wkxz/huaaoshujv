# HTTP 接口质量监控平台

基于 Go 语言的轻量级 HTTP 接口质量工具，集成健康监控与压力测试功能。纯标准库实现，零第三方依赖。

## 功能

- **健康监控**：定时探测目标接口，记录状态码/响应延迟，支持动态增删监控目标
- **异常告警**：连续失败触发 webhook 告警，恢复后自动发送恢复通知
- **压力测试**：并发压测目标接口，统计 QPS、P50/P95/P99 延迟、成功率
- **数据持久化**：探测结果自动保存到 JSON 文件，重启后恢复
- **优雅关闭**：捕获信号，安全保存数据后退出

## 快速启动

### 本地运行

```bash
go build -o http-monitor .
./http-monitor
```

服务启动后监听 `http://localhost:8080`。

### Docker 运行

```bash
docker build -t http-monitor .
docker run -p 8080:8080 http-monitor
```

## 配置文件

编辑 `config.json` 配置监控目标和告警：

```json
{
  "targets": [
    {
      "id": "baidu",
      "name": "百度",
      "url": "https://www.baidu.com",
      "method": "GET",
      "interval_seconds": 30,
      "timeout_seconds": 10
    }
  ],
  "alert": {
    "webhook_url": "https://your-webhook-url.com/alert",
    "threshold": 3,
    "cooldown_minutes": 5
  }
}
```

## API 文档

### 健康检查

```
GET /health
```

响应：`{"status":"ok"}`

### 监控状态汇总

```
GET /api/status
```

响应：
```json
{"total": 3, "healthy": 2, "unhealthy": 1}
```

### 监控目标列表

```
GET /api/targets
```

### 查看目标详情

```
GET /api/targets/{id}
```

### 查看探测历史

```
GET /api/targets/{id}/history
```

### 添加监控目标

```
POST /api/targets
Content-Type: application/json

{
  "id": "my-api",
  "name": "My API",
  "url": "https://api.example.com/health",
  "method": "GET",
  "interval_seconds": 30,
  "timeout_seconds": 10
}
```

### 修改监控目标

```
PUT /api/targets/{id}
Content-Type: application/json

{"interval_seconds": 60}
```

### 删除监控目标

```
DELETE /api/targets/{id}
```

### 提交压测任务

```
POST /api/bench
Content-Type: application/json

{
  "url": "https://httpbin.org/get",
  "method": "GET",
  "concurrency": 10,
  "duration_seconds": 10
}
```

响应：
```json
{"id": "bench-1719900000000", "status": "running", ...}
```

### 查询压测结果

```
GET /api/bench/{task_id}
```

响应（完成后）：
```json
{
  "id": "bench-1719900000000",
  "status": "completed",
  "report": {
    "total_requests": 1520,
    "success_count": 1520,
    "fail_count": 0,
    "qps": 152.0,
    "avg_latency_ms": 65.3,
    "p50_latency_ms": 58.2,
    "p95_latency_ms": 120.5,
    "p99_latency_ms": 185.3,
    "status_code_dist": {"200": 1520}
  }
}
```

### 查询所有压测任务

```
GET /api/bench
```

## 技术栈

- Go 1.26
- 纯标准库：net/http、encoding/json、sync、time、sort
- Docker 多阶段构建
