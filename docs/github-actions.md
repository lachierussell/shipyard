# GitHub Actions Integration

## Setup

### 1. Add Repository Secrets

In your GitHub repo, go to **Settings > Secrets and variables > Actions** and add:

| Secret | Description |
|--------|-------------|
| `SHIPYARD_URL` | Your Shipyard instance URL (e.g., `https://shipyard.example.com`) |
| `SHIPYARD_API_KEY` | Site API key from `shipyard.toml` |

### 2. Add Workflow File

Create `.github/workflows/deploy.yml`:

```yaml
name: Deploy Frontend

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install and build
        run: |
          npm ci
          npm run build

      - name: Create artifact
        run: |
          cd dist
          zip -r ../frontend.zip .

      - name: Deploy to Shipyard
        run: |
          curl -f -X POST ${{ secrets.SHIPYARD_URL }}/deploy/frontend \
            -H "X-Shipyard-Key: ${{ secrets.SHIPYARD_API_KEY }}" \
            -F "site=myapp" \
            -F "commit=${{ github.sha }}" \
            -F "artifact=@frontend.zip" \
            -F "nginx_config=@nginx.conf"
```

## Variations

### Deploy on Release Only

```yaml
on:
  release:
    types: [published]
```

### Deploy Specific Paths

```yaml
on:
  push:
    branches: [main]
    paths:
      - 'src/**'
      - 'package.json'
```

### Multiple Environments

```yaml
name: Deploy Frontend

on:
  push:
    branches: [main, staging]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install and build
        run: |
          npm ci
          npm run build

      - name: Create artifact
        run: cd dist && zip -r ../frontend.zip .

      - name: Deploy to production
        if: github.ref == 'refs/heads/main'
        run: |
          curl -f -X POST ${{ secrets.SHIPYARD_URL }}/deploy/frontend \
            -H "X-Shipyard-Key: ${{ secrets.SHIPYARD_API_KEY_PROD }}" \
            -F "site=myapp" \
            -F "commit=${{ github.sha }}" \
            -F "artifact=@frontend.zip" \
            -F "nginx_config=@nginx.conf"

      - name: Deploy to staging
        if: github.ref == 'refs/heads/staging'
        run: |
          curl -f -X POST ${{ secrets.SHIPYARD_URL }}/deploy/frontend \
            -H "X-Shipyard-Key: ${{ secrets.SHIPYARD_API_KEY_STAGING }}" \
            -F "site=myapp-staging" \
            -F "commit=${{ github.sha }}" \
            -F "artifact=@frontend.zip" \
            -F "nginx_config=@nginx.staging.conf"
```

### With Build Matrix (Monorepo)

```yaml
name: Deploy Apps

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        app: [web, admin, docs]
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Build ${{ matrix.app }}
        run: |
          cd apps/${{ matrix.app }}
          npm ci
          npm run build
          cd dist && zip -r ../../../${{ matrix.app }}.zip .

      - name: Deploy ${{ matrix.app }}
        run: |
          curl -f -X POST ${{ secrets.SHIPYARD_URL }}/deploy/frontend \
            -H "X-Shipyard-Key: ${{ secrets[format('SHIPYARD_KEY_{0}', matrix.app)] }}" \
            -F "site=${{ matrix.app }}" \
            -F "commit=${{ github.sha }}" \
            -F "artifact=@${{ matrix.app }}.zip" \
            -F "nginx_config=@apps/${{ matrix.app }}/nginx.conf"
```

## Verify Deployment

Check deployment status:

```bash
curl https://shipyard.example.com/status/myapp
```
