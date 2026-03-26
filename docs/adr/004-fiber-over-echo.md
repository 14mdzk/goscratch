# ADR-004: Fiber Over Echo/Chi/Gin

## Status
Accepted

## Context
We needed an HTTP framework for the REST API. The main contenders were Fiber (fasthttp-based), Echo, Chi (stdlib-compatible), and Gin. Performance, middleware ecosystem, and developer ergonomics were the key criteria.

## Decision
We chose Fiber v2 for its Express-like API, high performance (built on fasthttp), and rich middleware ecosystem. Its familiar API lowers the learning curve for developers coming from Node.js/Express backgrounds.

## Consequences
- **Pro:** High throughput due to fasthttp's zero-allocation design
- **Pro:** Express-like API is intuitive for route definition and middleware chaining
- **Pro:** Built-in middleware for CORS, recovery, compression, and more
- **Con:** Fiber uses `fasthttp.RequestCtx` instead of `net/http`, so `net/http` middleware is not directly compatible (requires adaptor)
- **Con:** `fasthttp` reuses request/response objects, requiring care with async handlers
