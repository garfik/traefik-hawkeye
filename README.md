# traefik-hawkeye

[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/garfik/traefik-hawkeye)](go.mod)
[![GitHub Release](https://img.shields.io/github/v/release/garfik/traefik-hawkeye?display_name=tag)](https://github.com/garfik/traefik-hawkeye/releases)
[![CI](https://github.com/garfik/traefik-hawkeye/actions/workflows/main.yml/badge.svg)](https://github.com/garfik/traefik-hawkeye/actions/workflows/main.yml)
[![Traefik Plugin](https://img.shields.io/badge/Traefik-Plugin-blue.svg)](https://doc.traefik.io/traefik/plugins/)

A lightweight, non-blocking middleware plugin for Traefik that collects HTTP request data and forwards it to an external analytics server.

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
      version: v0.1.4
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
          filterHostMode: "include"
          filterHostList:
            - "example.com"
            - "api.example.com"
          filterContentTypeMode: "exclude"
          filterContentTypeList:
            - "text/html"
          trackingPixelURL: "/hawk.png"
```

### Configuration Options

| Field                    | Type     | Default | Description                                                                                                                                                                                                                                                                                                                                                                        |
| ------------------------ | -------- | ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `endpoint`               | string   | -       | **Required.** The analytics endpoint URL to send batches to                                                                                                                                                                                                                                                                                                                        |
| `queueSize`              | int      | 500     | Maximum number of events in the queue before dropping                                                                                                                                                                                                                                                                                                                              |
| `batchSize`              | int      | 50      | Number of events to accumulate before sending a batch                                                                                                                                                                                                                                                                                                                              |
| `flushEveryMs`           | int      | 3000    | Time interval (milliseconds) for periodic batch flushing                                                                                                                                                                                                                                                                                                                           |
| `httpTimeoutMs`          | int      | 300     | HTTP client timeout (milliseconds) for sending batches                                                                                                                                                                                                                                                                                                                             |
| `includeRequestHeaders`  | []string | []      | List of HTTP header names from request to include in events                                                                                                                                                                                                                                                                                                                        |
| `includeResponseHeaders` | []string | []      | List of HTTP header names from response to include in events                                                                                                                                                                                                                                                                                                                       |
| `filterHostMode`         | string   | -       | Host filtering mode: `"include"` (only listed hosts) or `"exclude"` (all except listed)                                                                                                                                                                                                                                                                                            |
| `filterHostList`         | []string | []      | List of hostnames to filter. Case-insensitive matching                                                                                                                                                                                                                                                                                                                             |
| `filterContentTypeMode`  | string   | -       | Content-Type filtering mode: `"include"` (only listed types) or `"exclude"` (all except listed)                                                                                                                                                                                                                                                                                    |
| `filterContentTypeList`  | []string | []      | List of content types to filter (e.g., `"text/html"`, `"application/json"`). Parameters like charset are ignored. Case-insensitive matching                                                                                                                                                                                                                                        |
| `trackingPixelURL`       | string   | -       | URL path for tracking pixel (e.g., `"/hawk.png"`). When a request to this URL includes a valid `u` query parameter with a URL-encoded path, the event will be logged as if the user visited the path from `u` parameter instead of the tracking pixel URL. Useful for SPA applications to track client-side navigation. See [Tracking Pixel](#tracking-pixel) section for details. |

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
  "content_type": "application/json",
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
- `content_type`: Response Content-Type header value (main type only, without charset or other parameters). Lowercase. Omitted if not present.
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

## Tracking Pixel

The `trackingPixelURL` feature is designed for Single Page Applications (SPA) to track client-side navigation. In SPAs, route changes happen on the client side without full page reloads, so traditional server-side analytics miss these navigation events.

### How It Works

When `trackingPixelURL` is configured (e.g., `"/hawk.png"`), the middleware intercepts requests to this URL. If the request includes a valid `u` query parameter containing a URL-encoded path, the event is logged as if the user visited the path from the `u` parameter, not the tracking pixel URL itself.

**Example request:**

```
GET /hawk.png?u=%2Fmap%2F3a6eaf87-3243-4184-8e9d-34b5e002790c&_=mignsq2wbutgh0ulkka
```

**Resulting event:**

- `path`: `/map/3a6eaf87-3243-4184-8e9d-34b5e002790c` (from `u` parameter)
- `content_type`: `text/html` (always set for tracking pixel events)

### Usage in SPA

In your SPA application, track route changes by making a request to the tracking pixel URL:

```javascript
// Track page view on route change
function trackPageView(path) {
  const encodedPath = encodeURIComponent(path);
  const img = new Image();
  img.src = `/hawk.png?u=${encodedPath}&_=${Date.now()}`;
}

// Example: React Router
router.subscribe((state) => {
  trackPageView(state.location.pathname);
});
```

### Important Notes

- If the `u` parameter is missing, invalid, or empty, the request is processed normally (subject to Content-Type filtering)
- Tracking pixel requests with valid `u` parameter always bypass Content-Type filters and are always logged
- The actual HTTP request to `/hawk.png` is passed through unchanged - only the logged event is modified, so don't forget to actually keep such URL valid, e.g. add an empty PNG file or handle this on the server
- The `content_type` field in the event is always set to `text/html` for tracking pixel events, regardless of the actual response Content-Type

## Development

You're welcome to contribute! See [DEVELOPMENT.md](https://github.com/garfik/traefik-hawkeye/blob/main/DEVELOPMENT.md) for setup instructions on GitHub.

## 💚 Sponsorship

This project is licensed under the permissive **MIT License**, and commercial use is fully allowed.

If your company benefits from this plugin in a commercial environment,  
please consider supporting the project through sponsorship.

Your contribution helps maintain development and ensures long-term sustainability.

Thank you!
