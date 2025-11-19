# traefik-hawkeye

[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/garfik/traefik-hawkeye)](go.mod)
[![GitHub Release](https://img.shields.io/github/v/release/garfik/traefik-hawkeye?display_name=tag)](https://github.com/garfik/traefik-hawkeye/releases)
[![CI](https://github.com/garfik/traefik-hawkeye/actions/workflows/main.yml/badge.svg)](https://github.com/garfik/traefik-hawkeye/actions/workflows/main.yml)
[![Traefik Plugin](https://img.shields.io/badge/Traefik-Plugin-blue.svg)](https://doc.traefik.io/traefik/plugins/)

A lightweight, non-blocking analytics middleware plugin for Traefik that collects HTTP request data and forwards it to an external analytics server.

## Features

- **Non-blocking**: Request flow is never blocked by analytics collection
- **Batched sending**: Events are collected and sent in configurable batches
- **Automatic flushing**: Batches are sent either when size limit is reached or on a timer
- **yaegi-compatible**: Uses only standard library, no external dependencies
- **Graceful shutdown**: Flushes remaining events on context cancellation
- **Configurable**: Customize queue size, batch size, flush intervals and more

## Usage

### Static Configuration

To use `traefik-hawkeye` in Traefik, add it to your static configuration:

```yaml
experimental:
  plugins:
    hawkeye:
      moduleName: github.com/garfik/traefik-hawkeye
      version: v0.1.2
```

Traefik will automatically download and load the plugin from the GitHub repository. Make sure your Traefik instance has internet access to fetch the plugin.

After adding the plugin to your static configuration, restart Traefik and enable the middleware in your dynamic configuration (see Configuration section below).

### Dynamic Configuration

Add the middleware to your Traefik dynamic configuration:

```yaml
http:
  middlewares:
    hawkeye:
      plugin:
        hawkeye:
          endpoint: "http://analytics:8081/ingest"
          includeRequestHeaders:
            - "X-Service-Name"
          includeResponseHeaders:
            - "X-Request-Id"
            - "X-Session-Id"
```

### Configuration Options

| Field                    | Type     | Default | Description                                                  |
| ------------------------ | -------- | ------- | ------------------------------------------------------------ |
| `endpoint`               | string   | -       | **Required.** The analytics endpoint URL to send batches to  |
| `queueSize`              | int      | 500     | Maximum number of events in the queue before dropping        |
| `batchSize`              | int      | 50      | Number of events to accumulate before sending a batch        |
| `flushEveryMs`           | int      | 3000    | Time interval (milliseconds) for periodic batch flushing     |
| `httpTimeoutMs`          | int      | 300     | HTTP client timeout (milliseconds) for sending batches       |
| `includeRequestHeaders`  | []string | []      | List of HTTP header names from request to include in events  |
| `includeResponseHeaders` | []string | []      | List of HTTP header names from response to include in events |

## Behavior

- **Non-blocking queue**: If the event queue is full, new events are dropped (no blocking, no retries)
- **Batching**: Events are sent when either:
  - The batch reaches `batchSize` events, or
  - The `flushEveryMs` timer fires
- **Error handling**: HTTP send errors are silently ignored (fire-and-forget)
- **Graceful shutdown**: On context cancellation, remaining events in the current batch are flushed

## Event Data Model

Each HTTP request produces an event with the following structure:

```json
{
  "ts": "2025-11-15T12:34:56Z",
  "ip": "203.0.113.42",
  "method": "GET",
  "scheme": "https",
  "host": "example.com",
  "path": "/api/test",
  "status": 200,
  "dur_ms": 12,
  "ref": "https://google.com/",
  "ua": "Mozilla/5.0 ...",
  "request_hdr": {
    "X-Service-Name": "..."
  },
  "response_hdr": {
    "X-Request-Id": "...",
    "X-Session-Id": "..."
  }
}
```

### Field Descriptions

- `ts`: RFC3339 timestamp of when the request was processed
- `ip`: Client IP address (extracted from X-Real-Ip, X-Forwarded-For, or RemoteAddr)
- `method`: HTTP method (GET, POST, etc.)
- `scheme`: Request scheme (http or https, from X-Forwarded-Proto or TLS)
- `host`: Request host header
- `path`: Request URL path
- `status`: HTTP response status code
- `dur_ms`: Request duration in milliseconds
- `ref`: Referer header (always included, empty string if not present)
- `ua`: User-Agent header (always included, empty string if not present)
- `request_hdr`: Map of headers from the request (only headers listed in `includeRequestHeaders` config). Always present, empty object `{}` if no headers are included.
- `response_hdr`: Map of headers from the response (only headers listed in `includeResponseHeaders` config). Always present, empty object `{}` if no headers are included.

### IP Extraction Priority

1. `X-Real-Ip` header
2. First value from `X-Forwarded-For` header
3. `RemoteAddr` host portion

### Scheme Detection Priority

1. `X-Forwarded-Proto` header
2. TLS presence check (`req.TLS != nil`)
3. Default: `"http"`

## Development

### Building

To build the plugin:

```bash
make build
```

### Testing

To run all tests:

```bash
make test
```

### Linting

To run linter:

```bash
make lint
```

### How to test changes locally

To test the plugin locally with a full Traefik setup:

1. In one terminal, start the Docker Compose environment from the `local-dev` directory:

```bash
cd local-dev
docker compose up
```

2. In another terminal, make a test request:

```bash
curl -H "Host: localhost" http://localhost:4000/
```

3. Check the echo server logs to verify that events are being received. You should see analytics events arriving at the echo server.

**Note:** When making changes to the plugin code, remember to restart the Docker Compose environment for the changes to take effect

## Requirements

- Traefik v2.3+ (with plugin support)
- Go 1.23+
- Standard library only (no external dependencies)

## 💚 Sponsorship

This project is licensed under the permissive **MIT License**, and commercial use is fully allowed.

If your company benefits from this plugin in a commercial environment,  
please consider supporting the project through sponsorship.

Your contribution helps maintain development and ensures long-term sustainability.

Thank you!
