[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/rc-yile-qpslimit/release.yml?branch=main&label=BUILD)](https://github.com/yylego/rc-yile-qpslimit/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/rc-yile-qpslimit)](https://pkg.go.dev/github.com/yylego/rc-yile-qpslimit)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/rc-yile-qpslimit/main.svg)](https://coveralls.io/github/yylego/rc-yile-qpslimit?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.26+-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/badge/release-active-blue.svg)](https://github.com/yylego/rc-yile-qpslimit)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/rc-yile-qpslimit)](https://goreportcard.com/report/github.com/yylego/rc-yile-qpslimit)

# rc-yile-qpslimit

Hot key QPS rate limiting demo — Gin middleware that throttles requests based on `X-Rate-Limit-Token` using sliding window.

---

<!-- TEMPLATE (EN) BEGIN: LANGUAGE NAVIGATION -->

## CHINESE README

[中文说明](README.zh.md)

<!-- TEMPLATE (EN) END: LANGUAGE NAVIGATION -->

## Problem Understanding

In high-traffic API services, certain "hot keys" (e.g., specific users, devices, or API tokens) can generate disproportionate request volumes and overwhelm backend resources. Without per-key rate limiting, a single abusive or misconfigured client can degrade the experience of all others.

The core challenge is **not blocking all traffic**, but **isolating each key's quota** so that one key hitting its ceiling does not affect others. This requires per-key sliding window counting with efficient memory management — idle keys must be evicted to prevent unbounded growth.

## Architecture

```
Request with X-Rate-Limit-Token: "user_123"
    → Gin Middleware intercepts
    → Extract token from header
    → ratelimit.Group.Allow("user_123")
        → Sliding window check (within 1s window)
        → Within quota  → ctx.Next() → Handler → 200 OK
        → Over quota    → ctx.Abort() → 429 Too Many Requests

Request without X-Rate-Limit-Token
    → Gin Middleware intercepts
    → No token found
    → ctx.Next() → Handler → 200 OK (no throttling)
```

Key points:

- Each token has an independent sliding window count
- Idle tokens are auto-evicted via min-heap (no background goroutine)
- The middleware is stateless per request — all state is in the shared `ratelimit.Group`

## Design Decisions

### 1. System Boundaries

**What this service does:**

- Demonstrate per-key QPS rate limiting as Gin middleware
- Show how to use the [ratelimit](https://github.com/yylego/ratelimit) package in a real HTTP service
- Provide integration tests proving the rate limit works

**What this service does NOT do:**

- Distributed rate limiting across multiple instances — this is a single-process demo
- Persistent rate limit state — counters reset on restart
- Dynamic QPS configuration per key — all keys share the same maxQps setting
- Response body inspection or transformation — the middleware is transparent to handlers

### 2. Key Extraction Approach

The rate limit key comes from the `X-Rate-Limit-Token` request header. This design means:

- **Opt-in throttling**: Requests without the header pass through unthrottled. This is deliberate — not all endpoints need per-key limiting
- **Client-chosen token**: The client decides which token to present. In production, this would be set by an API gateway or auth middleware
- **No coupling to auth**: The rate limit token is independent of authentication, keeping concerns separated

### 3. Sliding Window vs Fixed Window

The [ratelimit](https://github.com/yylego/ratelimit) package uses a **sliding window** algorithm instead of fixed time buckets:

- Fixed window has the "edge burst" problem: 100 QPS at second edges allows 200 requests in 1 second (100 at end of window N, 100 at start of window N+1)
- Sliding window tracks exact timestamps, preventing burst at boundaries
- Trade-off: more memory per key (stores timestamps instead of a single count), but much more accurate

### 4. Memory Management

Per-key counts must be evicted when idle, otherwise memory grows unbounded. The [ratelimit](https://github.com/yylego/ratelimit) package uses a **min-heap** ([heapx](https://github.com/yylego/heapx)) to track the oldest access time per key:

- No background goroutine — eviction happens on demand during `Allow()` calls
- Eviction is O(log N) per key via heap operations
- This is preferable to TTL-based expiration with a background cleanup, which adds concurrence overhead

### 5. Technology Choices

| Choice           | Reasoning                                               | Alternative (not used)                                                                    |
| ---------------- | ------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Gin              | Mature middleware ecosystem, easy to compose            | net/http (no middleware chain), Echo (comparable but less common)                         |
| ratelimit        | Sliding window + heap eviction, no background goroutine | golang.org/x/time/rate (no per-key support), uber-go/ratelimit (token bucket, single key) |
| Header-based key | Decoupled from auth, transparent to handlers            | URL path param (couples routing), query param (not standard for metadata)                 |

## API

### GET /api/ping — Business endpoint (rate-limited)

Returns a random number. When `X-Rate-Limit-Token` header is present, the request counts toward that token's QPS quota.

```bash
# Without token (no throttling)
curl http://localhost:8080/api/ping
# {"value": 4892710192837465}

# With token (throttled)
curl -H "X-Rate-Limit-Token: my_token" http://localhost:8080/api/ping
# 200: {"value": 7293610284756183}
# or 429: {"message": "rate limited"}
```

### GET /health — Health check (not rate-limited)

```bash
curl http://localhost:8080/health
# {"status": "ok"}
```

## Quick Start

```bash
cd cmd
go run main.go
# Listening on :8080 (maxQps=1000)

# Normal request (no throttling)
curl http://localhost:8080/api/ping

# With rate limit token
curl -H "X-Rate-Limit-Token: test_token" http://localhost:8080/api/ping
```

## Project Structure

```
rc-yile-qpslimit/
├── cmd/main.go                      # Service entrance
├── internal/
│   └── service/
│       ├── service.go               # Gin + middleware + routes
│       └── service_test.go          # Integration tests
└── README.md
```

## Tech Stack

| Component     | Choice                                                                    |
| ------------- | ------------------------------------------------------------------------- |
| HTTP          | [Gin](https://github.com/gin-gonic/gin)                                   |
| Rate Limiting | [ratelimit](https://github.com/yylego/ratelimit) (sliding window + heapx) |

---

<!-- TEMPLATE (EN) BEGIN: STANDARD PROJECT FOOTER -->

## 📄 License

MIT License - see [LICENSE](LICENSE).

---

## 💬 Contact & Feedback

Contributions are welcome! Report bugs, suggest features, and contribute code:

- 🐛 **Mistake reports?** Open an issue on GitHub with reproduction steps
- 💡 **Fresh ideas?** Create an issue to discuss
- 📖 **Documentation confusing?** Report it so we can improve
- 🚀 **Need new features?** Share the use cases to help us understand requirements
- ⚡ **Performance issue?** Help us optimize through reporting slow operations
- 🔧 **Configuration problem?** Ask questions about complex setups
- 📢 **Follow project progress?** Watch the repo to get new releases and features
- 🌟 **Success stories?** Share how this package improved the workflow
- 💬 **Feedback?** We welcome suggestions and comments

---

## 🔧 Development

New code contributions, follow this process:

1. **Fork**: Fork the repo on GitHub (using the webpage UI).
2. **Clone**: Clone the forked project (`git clone https://github.com/yourname/repo-name.git`).
3. **Navigate**: Navigate to the cloned project (`cd repo-name`)
4. **Branch**: Create a feature branch (`git checkout -b feature/xxx`).
5. **Code**: Implement the changes with comprehensive tests
6. **Testing**: (Golang project) Ensure tests pass (`go test ./...`) and follow Go code style conventions
7. **Documentation**: Update documentation to support client-facing changes
8. **Stage**: Stage changes (`git add .`)
9. **Commit**: Commit changes (`git commit -m "Add feature xxx"`) ensuring backward compatible code
10. **Push**: Push to the branch (`git push origin feature/xxx`).
11. **PR**: Open a merge request on GitHub (on the GitHub webpage) with detailed description.

Please ensure tests pass and include relevant documentation updates.

---

## 🌟 Support

Welcome to contribute to this project via submitting merge requests and reporting issues.

**Project Support:**

- ⭐ **Give GitHub stars** if this project helps you
- 🤝 **Share with teammates** and (golang) programming friends
- 📝 **Write tech blogs** about development tools and workflows - we provide content writing support
- 🌟 **Join the ecosystem** - committed to supporting open source and the (golang) development scene

**Have Fun Coding with this package!** 🎉🎉🎉

<!-- TEMPLATE (EN) END: STANDARD PROJECT FOOTER -->
