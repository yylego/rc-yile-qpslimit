[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/rc-yile-qpslimit/release.yml?branch=main&label=BUILD)](https://github.com/yylego/rc-yile-qpslimit/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/rc-yile-qpslimit)](https://pkg.go.dev/github.com/yylego/rc-yile-qpslimit)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/rc-yile-qpslimit/main.svg)](https://coveralls.io/github/yylego/rc-yile-qpslimit?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.26+-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/badge/release-active-blue.svg)](https://github.com/yylego/rc-yile-qpslimit)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/rc-yile-qpslimit)](https://goreportcard.com/report/github.com/yylego/rc-yile-qpslimit)

# rc-yile-qpslimit

热点 Key QPS 限流示例 — 基于 `X-Rate-Limit-Token` 请求头的 Gin 限流中间件，使用滑动窗口。

---

<!-- TEMPLATE (ZH) BEGIN: LANGUAGE NAVIGATION -->

## 英文文档

[ENGLISH README](README.md)

<!-- TEMPLATE (ZH) END: LANGUAGE NAVIGATION -->

## 问题理解

在高流量 API 服务中，某些"热点 Key"（如特定用户、设备或 API Token）可能产生不成比例的请求量，压垮后端资源。如果没有按 Key 限流，一个滥用或配置错误的客户端就能拖累所有其他用户的体验。

核心挑战**不是拦截所有流量**，而是**隔离每个 Key 的配额**，使得一个 Key 触顶不影响其他 Key。这需要按 Key 的滑动窗口计数，并配合高效的内存管理 — 空闲 Key 必须被淘汰以防止内存无限增长。

## 架构

```
带 X-Rate-Limit-Token: "user_123" 的请求
    → Gin 中间件拦截
    → 从请求头提取 token
    → ratelimit.Group.Allow("user_123")
        → 滑动窗口检查（1秒窗口内）
        → 配额内  → ctx.Next() → 处理函数 → 200 OK
        → 超出配额 → ctx.Abort() → 429 Too Many Requests

不带 X-Rate-Limit-Token 的请求
    → Gin 中间件拦截
    → 未找到 token
    → ctx.Next() → 处理函数 → 200 OK（不限流）
```

要点：

- 每个 token 有独立的滑动窗口计数器
- 空闲 token 通过最小堆自动淘汰（无后台协程）
- 中间件对每个请求是无状态的 — 所有状态都在共享的 `ratelimit.Group` 中

## 设计决策

### 1. 系统边界

**本服务负责的：**

- 展示按 Key 的 QPS 限流作为 Gin 中间件的用法
- 演示如何在真实 HTTP 服务中使用 [ratelimit](https://github.com/yylego/ratelimit) 包
- 提供集成测试证明限流机制有效

**本服务不负责的：**

- 跨多实例的分布式限流 — 这是单进程示例
- 持久化限流状态 — 重启后计数器归零
- 按 Key 动态配置 QPS — 所有 Key 共享相同的 maxQps 设置
- 响应体检查或转换 — 中间件对处理函数透明

### 2. Key 提取方式

限流 Key 来自 `X-Rate-Limit-Token` 请求头。这意味着：

- **可选限流**：没有该请求头的请求直接放行，不做限流。这是有意为之 — 不是所有端点都需要按 Key 限流
- **客户端控制身份**：由客户端决定出示哪个 token。在生产环境中，这通常由 API 网关或认证中间件设置
- **不耦合认证**：限流 token 独立于认证机制，保持关注点分离

### 3. 滑动窗口 vs 固定窗口

[ratelimit](https://github.com/yylego/ratelimit) 包使用**滑动窗口**算法而非固定时间桶：

- 固定窗口有"边沿突发"问题：100 QPS 在秒边沿处允许1秒内通过200个请求（窗口 N 末尾100个 + 窗口 N+1 开头100个）
- 滑动窗口追踪精确时间戳，防止边界突发
- 代价：每个 Key 稍多内存（存储时间戳而非单个计数器），但精度高得多

### 4. 内存管理

按 Key 的计数器必须在空闲时被淘汰，否则内存会无限增长。[ratelimit](https://github.com/yylego/ratelimit) 包使用**最小堆**（[heapx](https://github.com/yylego/heapx)）追踪每个 Key 的最早访问时间：

- 无后台协程 — 淘汰在 `Allow()` 调用时惰性发生
- 每个 Key 的淘汰是 O(log N) 的堆操作
- 优于基于 TTL 的后台清扫机制，后者增加并发开销

### 5. 技术选型

| 选择             | 理由                          | 未采用的替代方案                                                            |
| ---------------- | ----------------------------- | --------------------------------------------------------------------------- |
| Gin              | 成熟的中间件生态，易于组合    | net/http（无中间件链）、Echo（类似但不太流行）                              |
| ratelimit        | 滑动窗口 + 堆淘汰，无后台协程 | golang.org/x/time/rate（不支持按 Key）、uber-go/ratelimit（令牌桶，单 Key） |
| 基于请求头的 Key | 与认证解耦，对处理函数透明    | URL 路径参数（耦合路由）、查询参数（不适合元数据）                          |

## API 接口

### GET /api/ping — 业务端点（受限流保护）

返回一个随机数。当请求头中有 `X-Rate-Limit-Token` 时，该请求计入对应 token 的 QPS 配额。

```bash
# 不带 token（不限流）
curl http://localhost:8080/api/ping
# {"value": 4892710192837465}

# 带 token（限流）
curl -H "X-Rate-Limit-Token: my_token" http://localhost:8080/api/ping
# 200: {"value": 7293610284756183}
# 或 429: {"message": "rate limited"}
```

### GET /health — 健康检查（不受限流）

```bash
curl http://localhost:8080/health
# {"status": "ok"}
```

## 快速启动

```bash
cd cmd
go run main.go
# 监听 :8080 (maxQps=1000)

# 普通请求（不限流）
curl http://localhost:8080/api/ping

# 带限流 token 的请求
curl -H "X-Rate-Limit-Token: test_token" http://localhost:8080/api/ping
```

## 项目结构

```
rc-yile-qpslimit/
├── cmd/main.go                      # 服务入口
├── internal/
│   └── service/
│       ├── service.go               # Gin + 中间件 + 路由
│       └── service_test.go          # 集成测试
└── README.md
```

## 技术栈

| 组件 | 选择                                                                 |
| ---- | -------------------------------------------------------------------- |
| HTTP | [Gin](https://github.com/gin-gonic/gin)                              |
| 限流 | [ratelimit](https://github.com/yylego/ratelimit)（滑动窗口 + heapx） |

---

<!-- TEMPLATE (ZH) BEGIN: STANDARD PROJECT FOOTER -->

## 📄 许可证类型

MIT 许可证 - 详见 [LICENSE](LICENSE)。

---

## 💬 联系与反馈

非常欢迎贡献代码！报告 BUG、建议功能、贡献代码：

- 🐛 **问题报告？** 在 GitHub 上提交问题并附上重现步骤
- 💡 **新颖思路？** 创建 issue 讨论
- 📖 **文档疑惑？** 报告问题，帮助我们完善文档
- 🚀 **需要功能？** 分享使用场景，帮助理解需求
- ⚡ **性能瓶颈？** 报告慢操作，协助解决性能问题
- 🔧 **配置困扰？** 询问复杂设置的相关问题
- 📢 **关注进展？** 关注仓库以获取新版本和功能
- 🌟 **成功案例？** 分享这个包如何改善工作流程
- 💬 **反馈意见？** 欢迎提出建议和意见

---

## 🔧 代码贡献

新代码贡献，请遵循此流程：

1. **Fork**：在 GitHub 上 Fork 仓库（使用网页界面）
2. **克隆**：克隆 Fork 的项目（`git clone https://github.com/yourname/repo-name.git`）
3. **导航**：进入克隆的项目（`cd repo-name`）
4. **分支**：创建功能分支（`git checkout -b feature/xxx`）
5. **编码**：实现您的更改并编写全面的测试
6. **测试**：（Golang 项目）确保测试通过（`go test ./...`）并遵循 Go 代码风格约定
7. **文档**：面向用户的更改需要更新文档
8. **暂存**：暂存更改（`git add .`）
9. **提交**：提交更改（`git commit -m "Add feature xxx"`）确保向后兼容的代码
10. **推送**：推送到分支（`git push origin feature/xxx`）
11. **PR**：在 GitHub 上打开 Merge Request（在 GitHub 网页上）并提供详细描述

请确保测试通过并包含相关的文档更新。

---

## 🌟 项目支持

非常欢迎通过提交 Merge Request 和报告问题来贡献此项目。

**项目支持：**

- ⭐ **给予星标**如果项目对您有帮助
- 🤝 **分享项目**给团队成员和（golang）编程朋友
- 📝 **撰写博客**关于开发工具和工作流程 - 我们提供写作支持
- 🌟 **加入生态** - 致力于支持开源和（golang）开发场景

**祝你用这个包编程愉快！** 🎉🎉🎉

<!-- TEMPLATE (ZH) END: STANDARD PROJECT FOOTER -->
