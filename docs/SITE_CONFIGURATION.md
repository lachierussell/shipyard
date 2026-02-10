# Site Configuration

## Site Types

Shipyard supports three types of site configurations:

### 1. Frontend Only
Static files served by nginx with SPA fallback.

```json
{
  "domain": "example.com",
  "ssl_enabled": true,
  "with_backend": false
}
```

### 2. Backend Only
All requests proxied to a backend service (e.g., API-only sites).

```json
{
  "domain": "api.example.com",
  "ssl_enabled": true,
  "with_backend": true,
  "proxy_path": "/"
}
```

### 3. Combined Frontend + Backend
Static frontend with backend API at a subpath (most common).

```json
{
  "domain": "app.example.com",
  "ssl_enabled": true,
  "with_backend": true,
  "proxy_path": "/api"
}
```

## Nginx Configuration

### Templates

Shipyard uses Go templates with `<%` `%>` delimiters:

- `backend_proxy.conf.tmpl` - Backend-only HTTP
- `backend_proxy_https.conf.tmpl` - Backend-only HTTPS
- `site_combined.conf.tmpl` - Frontend + Backend HTTP
- `site_combined_https.conf.tmpl` - Frontend + Backend HTTPS

### Path Handling

For combined sites, the backend proxy path is **stripped** before forwarding:

```
Request: /api/users/123
Backend receives: /users/123
```

This is done via nginx rewrite:
```nginx
location /api/ {
    rewrite ^/api/(.*)$ /$1 break;
    proxy_pass http://127.0.0.1:8080;
}
```

### SSL Certificates

SSL certificates are obtained via Let's Encrypt (certbot) using webroot validation:

1. HTTP-only config deployed first (serves `/.well-known/acme-challenge/`)
2. Certbot obtains certificate
3. HTTPS config deployed with certificate paths

Certificates are stored at:
```
/usr/local/etc/letsencrypt/live/{domain}/fullchain.pem
/usr/local/etc/letsencrypt/live/{domain}/privkey.pem
```

## Jail (Pot) Configuration

Backend services run in FreeBSD jails managed by `pot`:

- **Network**: `inherit` mode (shares host network stack)
- **Base**: FreeBSD 15.0 (or configured version)
- **Type**: `single` (single ZFS dataset)

The `inherit` network mode allows backends to make outbound connections (required for proxies, API calls, etc.).

### Pot Binary Path

By default, shipyard looks for `pot` on `$PATH`. When running as a daemon (via rc.d), `$PATH` may not include `/usr/local/bin`, causing `pot` commands to fail. Set an absolute path in `shipyard.toml`:

```toml
[jail]
binary_path = "/usr/local/bin/pot"
```

If `binary_path` is omitted or empty, shipyard falls back to the bare name `"pot"`. This means existing config files from older versions continue to work after a self-update — but adding the absolute path is recommended for daemon deployments.

## Common Issues

### "Too many levels of symbolic links"
The frontend directory exists but the `latest` symlink is broken or points to a non-existent directory. Deploy frontend files or create the target directory.

### "directory index is forbidden"
The frontend directory has no `index.html`. Deploy frontend files or create a placeholder.

### nginx variable names with hyphens
Domain names with hyphens (e.g., `my-app.example.com`) are normalized to underscores for nginx variables (`my_app_example_com`).

### `pot` not found when running as a daemon
When shipyard runs via rc.d, the daemon's `$PATH` may not include `/usr/local/bin`. Set `binary_path` under `[jail]` to the absolute path (see "Pot Binary Path" above). Shipyard logs a warning at startup if the configured pot binary cannot be found.

### HTTP 413 on large uploads
The default nginx proxy location for `/api/` may not have its own `client_max_body_size`. Ensure the `/api/` location block includes:
```nginx
client_max_body_size 500m;
proxy_request_buffering off;
```
New installs via `install.sh` include this automatically. For existing servers, add the directives to your `default.conf` `/api/` location and run `service nginx reload`.

### Backend can't make outbound connections
The jail was created with `alias` networking. Recreate with `inherit`:
```sh
pot stop -p {pot-name}
pot destroy -p {pot-name}
pot create -p {pot-name} -t single -b 15.0 -N inherit
pot start -p {pot-name}
```
Then redeploy the backend.
