# Development

## Requirements

- Traefik v2.3+ (with plugin support)
- Go 1.23+
- Standard library only (no external dependencies)

## Building

```bash
make build
```

## Testing

```bash
make test
```

## Linting

```bash
make lint
```

## How to test changes locally

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

**Note:** When making changes to the plugin code, remember to restart the Docker Compose environment for the changes to take effect.

