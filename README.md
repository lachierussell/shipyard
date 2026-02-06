# Shipyard

A deployment framework for FreeBSD that manages frontend static files and backend services with nginx routing, version previews, and automatic restarts.

## Quick Start

```sh
# On FreeBSD (as root)
cp -r /path/to/shipyard /usr/local/src/shipyard
/usr/local/src/shipyard/install.sh
service shipyard start
```

Open `http://your-server/` to access the Helm control panel. Enter the admin key shown during installation to connect.

## Configuration

Edit `/usr/local/etc/shipyard/shipyard.toml`:

```toml
admin_keys = ["sk-admin-your-secret-key"]

[server]
listen_addr = "127.0.0.1:8443"

[site.myapp]
domain        = "myapp.example.com"
frontend_root = "/usr/local/www/myapp.example.com"
api_key       = "sk-live-myapp-secret"
override_ips  = ["10.0.0.0/8"]  # IPs allowed to use ?override=

# Optional backend service
[site.myapp.backend]
jail_ip     = "127.0.1.2"
listen_port = 8080
proxy_path  = "/api"
binary_name = "myapp-api"
```

## API Reference

All endpoints use the `X-Shipyard-Key` header for authentication.

### Deploy Frontend

```sh
curl -X POST http://localhost:8443/deploy/frontend \
  -H "X-Shipyard-Key: sk-live-myapp-secret" \
  -F "site=myapp" \
  -F "commit=$(git rev-parse HEAD)" \
  -F "artifact=@dist.zip" \
  -F "nginx_config=@nginx.conf"
```

### Deploy Backend

```sh
curl -X POST http://localhost:8443/deploy/backend \
  -H "X-Shipyard-Key: sk-live-myapp-secret" \
  -F "site=myapp" \
  -F "commit=$(git rev-parse HEAD)" \
  -F "artifact=@backend.zip"
```

### Other Endpoints

| Endpoint | Auth | Description |
|----------|------|-------------|
| `GET /health` | None | System status |
| `GET /status/:site` | None | Site status |
| `POST /site/init` | Admin | Initialize site |
| `POST /site/destroy` | Admin | Remove site |
| `POST /deploy/self` | Admin | Update shipyard |

## Version Preview

Test deployments before going live from whitelisted IPs:

```
https://myapp.example.com/?override=abc123def456...
```

This serves the specified commit and adds `X-Robots-Tag: noindex` to prevent indexing.

## CI/CD Integration

See [docs/github-actions.md](docs/github-actions.md) for GitHub Actions examples.

## Development

```sh
make build          # Build for local platform
make build-freebsd  # Build for FreeBSD arm64
make test           # Run tests
make web-dev        # Start admin UI dev server
```
