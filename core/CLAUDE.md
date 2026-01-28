[根目录](../CLAUDE.md) > **core**

---

# Core 模块文档

> 最后更新: 2026-01-27T23:51:59+08:00

## 变更记录 (Changelog)

- **2026-01-27**: 初始化核心模块文档

---

## 模块职责

`core` 模块是 QuickDoctor 的核心业务逻辑层，负责与健康160平台的所有交互，包括：

1. **HTTP 客户端** (`client.go`): 封装 TLS 指纹客户端，处理 Cookie 管理与请求重试
2. **抢号引擎** (`grabber.go`): 实现多日期、多医生、多时段的自动抢号逻辑
3. **扫码登录** (`qr_login.go`): 处理微信二维码登录流程与轮询
4. **数据模型** (`types.go`): 定义所有核心数据结构
5. **错误处理** (`errors.go`): 统一错误定义
6. **Cookie 管理** (`cookies.go`): Cookie 持久化与加载
7. **路径工具** (`paths.go`): 配置文件路径解析
8. **代理管理** (`proxy.go`): 代理池轮换（用于频繁提交时的IP切换）
9. **状态管理** (`state.go`): 用户配置状态的加载与保存

---

## 入口与启动

### 核心类型初始化

```go
// 创建 HTTP 客户端实例
client, err := core.NewHealthClient()
if err != nil {
    log.Fatal(err)
}

// 加载已保存的 Cookie（如果存在）
client.EnsureCookiesLoaded()
```

### 抢号流程启动

```go
// 创建抢号器
grabber := core.NewGrabber(client)

// 配置抢号参数
config := map[string]any{
    "unit_id": "123",
    "dep_id": "456",
    "member_id": "789",
    "target_dates": []string{"2026-01-28"},
    "time_types": []string{"am"},
    // ... 更多配置见 GrabConfig
}

// 启动抢号（阻塞直到成功或取消）
result := grabber.Run(ctx, config, logCallback)
```

---

## 对外接口

### HealthClient 核心方法

| 方法 | 功能 | 返回值 |
|------|------|--------|
| `GetHospitalsByCity(cityID)` | 获取城市医院列表 | `[]map[string]any` |
| `GetDepsByUnit(unitID)` | 获取医院科室列表 | `[]map[string]any` |
| `GetMembers()` | 获取就诊人列表 | `[]Member` |
| `GetSchedule(unitID, depID, date)` | 查询医生排班 | `[]map[string]any` |
| `GetTicketDetail(unitID, depID, scheduleID, memberID)` | 获取号源详情 | `*TicketDetail` |
| `SubmitOrder(params)` | 提交挂号订单 | `*SubmitOrderResult` |
| `CheckLogin()` | 校验登录状态 | `bool` |
| `LoadCookies()` | 加载本地 Cookie | `bool` |
| `HasAccessHash()` | 检查是否有有效登录凭证 | `bool` |

### Grabber 抢号引擎

| 方法 | 功能 | 返回值 |
|------|------|--------|
| `Run(ctx, config, onLog)` | 启动抢号循环 | `GrabResult` |

### FastQRLogin 扫码登录

| 方法 | 功能 | 返回值 |
|------|------|--------|
| `GetQRImage()` | 获取二维码图片 | `([]byte, string, error)` |
| `PollStatus(ctx, timeout, onStatusChange)` | 轮询扫码状态 | `QRLoginResult` |

---

## 关键依赖与配置

### 外部依赖

```go
// go.mod 关键依赖
github.com/bogdanfinn/tls-client v1.7.11  // TLS 指纹客户端
github.com/PuerkitoBio/goquery v1.9.3     // HTML 解析
github.com/bogdanfinn/fhttp v0.5.32       // 兼容 net/http 的 TLS 包装
```

### 配置文件

- `config/cookies.json`: Cookie 持久化存储
- `config/user_state.json`: 用户抢号配置（由上层 `app.go` 调用 `state.go` 管理）
- `logs/submit_resp_*.bin`: 提交失败时的响应调试文件

### 环境变量

无环境变量依赖，所有配置通过函数参数或配置文件传递。

---

## 数据模型

### 核心类型定义

```go
// types.go

type Member struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    Certified bool   `json:"certified"`
}

type TicketDetail struct {
    Times           []TimeSlot      `json:"times"`
    SchData         string          `json:"sch_data"`
    DetlidRealtime  string          `json:"detlid_realtime"`
    LevelCode       string          `json:"level_code"`
    AddressID       string          `json:"addressId"`
    Address         string          `json:"address"`
    Addresses       []AddressOption `json:"addresses"`
    // ... 更多字段见 types.go
}

type GrabConfig struct {
    UnitID         string   `json:"unit_id"`
    DepID          string   `json:"dep_id"`
    MemberID       string   `json:"member_id"`
    TargetDates    []string `json:"target_dates"`
    DoctorIDs      []string `json:"doctor_ids,omitempty"`
    TimeTypes      []string `json:"time_types,omitempty"`
    PreferredHours []string `json:"preferred_hours,omitempty"`
    StartTime      string   `json:"start_time,omitempty"`
    RetryInterval  float64  `json:"retry_interval,omitempty"`
    MaxRetries     int      `json:"max_retries,omitempty"`
    UseProxySubmit bool     `json:"use_proxy_submit,omitempty"`
}

type SubmitOrderResult struct {
    Success bool   `json:"success"`
    Status  bool   `json:"status"`
    Message string `json:"msg"`
    URL     string `json:"url,omitempty"`
}
```

---

## 测试与质量

### 测试覆盖

**当前状态**: 无自动化测试

**建议补充**:
1. `client_test.go`: Mock HTTP 响应测试
   - 测试 Cookie 加载与保存
   - 测试登录失效场景
   - 测试 API 错误处理

2. `grabber_test.go`: 抢号逻辑测试
   - 测试多日期/多医生匹配逻辑
   - 测试提交频率限制
   - 测试代理轮换机制

3. `qr_login_test.go`: 扫码登录测试
   - 测试二维码过期处理
   - 测试轮询超时

### 代码质量工具

建议配置:
- `golangci-lint`: 静态分析
- `go vet`: 标准检查
- `gofmt`: 格式化检查

---

## 常见问题 (FAQ)

### Q1: 登录失效如何处理？

**A**: `HealthClient.CheckLogin()` 返回 `false` 时，需要重新调用 `StartQRLogin()`。抢号过程中若遇到 `ErrLoginRequired` 错误，会自动停止并提示前端。

### Q2: 如何调试提交失败？

**A**: 提交失败时，响应内容会自动保存到 `logs/submit_resp_*.bin`，可查看该文件分析失败原因。同时 `HealthClient.LastError()` 会保存最后一次错误信息。

### Q3: 代理提交如何工作？

**A**: 当 `use_proxy_submit` 为 `true` 且提交遇到"太快"/"频繁"提示时，`grabber.go` 会自动调用 `RotateProxy()` 切换代理（需提前配置代理池）。

### Q4: TLS 指纹客户端为何重要？

**A**: 健康160等平台可能检测 HTTP 客户端的 TLS 指纹，使用 `bogdanfinn/tls-client` 可模拟真实浏览器（Chrome 120）的 TLS 握手特征，避免被识别为爬虫。

### Q5: 如何配置抢号时间同步？

**A**: 设置 `use_server_time: true` 并提供 `start_time`（如 `"08:00:00"`），抢号器会自动校准本地时间与服务器时间的偏移，在精确时刻启动。

---

## 相关文件清单

### 核心代码
- `client.go` (1241 行) - HTTP 客户端主逻辑
- `grabber.go` (878 行) - 抢号引擎
- `qr_login.go` - 扫码登录
- `types.go` (78 行) - 数据模型定义
- `cookies.go` - Cookie 管理
- `errors.go` - 错误定义
- `state.go` - 配置状态管理
- `paths.go` - 路径工具
- `proxy.go` - 代理管理

### 依赖管理
- `../go.mod` - Go 模块依赖
- `../go.sum` - 依赖校验和

---

## 架构建议

### 优化方向
1. **拆分 client.go**: 当前文件过长（1241行），建议拆分为：
   - `client.go`: 核心客户端结构
   - `api_hospital.go`: 医院/科室查询
   - `api_schedule.go`: 排班查询
   - `api_order.go`: 订单提交

2. **增加重试机制**: 网络请求建议增加指数退避重试

3. **日志结构化**: 当前使用回调函数传递日志，建议引入 `slog` 或 `zap`

4. **错误类型细化**: 增加更多错误类型（如 `ErrNetworkTimeout`, `ErrScheduleNotFound`）

### 扩展性
- **多平台支持**: 当前仅支持健康160，接口设计已考虑扩展性
- **代理池管理**: 可独立为服务，支持动态代理源
