# Frontend Deployment

## Prerequisites

1. Shipyard running on your server
2. Site configured in `shipyard.toml`
3. Your site's API key

## Configuration

Add your site to `/usr/local/etc/shipyard/shipyard.toml`:

```toml
[site.myapp]
domain        = "myapp.example.com"
frontend_root = "/usr/local/www/myapp.example.com"
api_key       = "sk-live-myapp-your-secret-key"
override_ips  = ["10.0.0.0/8"]  # IPs allowed to use ?override=
```

## Initialize Site (One-Time)

```bash
curl -X POST https://shipyard.example.com/site/init \
  -H "X-Shipyard-Key: sk-admin-xxxxx" \
  -F "site=myapp" \
  -F "nginx_config=@myapp.nginx.conf"
```

## Deploy Frontend

Package your built frontend as a zip and deploy:

```bash
# Create artifact from build output
cd dist && zip -r ../frontend.zip . && cd ..

# Deploy
curl -X POST https://shipyard.example.com/deploy/frontend \
  -H "X-Shipyard-Key: sk-live-myapp-xxxxx" \
  -F "site=myapp" \
  -F "commit=$(git rev-parse HEAD)" \
  -F "artifact=@frontend.zip" \
  -F "nginx_config=@myapp.nginx.conf"
```

### Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `site` | Yes | Site identifier from config |
| `commit` | Yes | 40-char git SHA |
| `artifact` | Yes | Zip file of frontend files |
| `nginx_config` | Yes | Nginx server block config |

### Response Codes

- **200**: Success
- **401**: Invalid API key
- **422**: Frontend deployed but nginx validation failed
- **400**: Invalid request

## Nginx Config Example

```nginx
server {
    listen 80;
    server_name myapp.example.com;
    root /usr/local/www/myapp.example.com/$frontend_version;
    index index.html;

    location / {
        if ($override_access_myapp_example_com = 0) {
            return 403;
        }
        add_header X-Robots-Tag $xrobots_value;
        try_files $uri $uri/ /index.html;
    }

    location ~* \.(js|css|png|jpg|gif|svg|woff2?)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
```

## Override Mechanism

Test deployments before going live from whitelisted IPs:

```
https://myapp.example.com/?override=a1b2c3d4e5f6...
```

The override serves the specified commit instead of `latest` and adds `X-Robots-Tag: noindex, nofollow`.

## Directory Structure

After deployment:

```
/usr/local/www/myapp.example.com/
├── latest -> a1b2c3d4.../    # Symlink to current version
├── a1b2c3d4.../              # Deployed commit
│   ├── index.html
│   ├── assets/
│   └── robots.txt
└── previous.../              # Old versions (for rollback)
```
