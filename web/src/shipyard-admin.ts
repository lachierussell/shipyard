import { LitElement, css, html, nothing } from 'lit';
import { customElement, state } from 'lit/decorators.js';

interface SiteInfo {
  domain: string;
  frontend_root: string;
  has_backend: boolean;
  backend_only: boolean;
  ssl_enabled: boolean;
  health: 'healthy' | 'unhealthy' | 'unknown';
}

interface Toast {
  id: number;
  type: 'success' | 'error' | 'info';
  title: string;
  message: string;
}

interface ApiError {
  status: string;
  error: string;
  detail?: string;
}

interface LogEntry {
  time: string;
  level: string;
  msg: string;
  [key: string]: unknown;
}

const STORAGE_KEYS = {
  serverUrl: 'shipyard:serverUrl',
  adminKey: 'shipyard:adminKey',
} as const;

@customElement('shipyard-admin')
export class ShipyardAdmin extends LitElement {
  @state() private serverUrl = '';
  @state() private adminKey = '';
  @state() private connected = false;
  @state() private loading = false;
  @state() private sites: SiteInfo[] = [];
  @state() private toasts: Toast[] = [];
  private toastId = 0;

  // Selected site for deployment
  @state() private selectedSite: SiteInfo | null = null;
  @state() private commitHash = 'latest';
  @state() private updateLatest = true;
  @state() private frontendArtifact: File | null = null;
  @state() private nginxConfig = '';
  @state() private nginxDefaultConfig = '';
  @state() private backendArtifact: File | null = null;
  @state() private binaryName = '';

  // Create site form
  @state() private showCreateForm = false;
  @state() private newSiteDomain = '';
  @state() private newSiteSSL = true;
  @state() private newSiteWithBackend = false;

  // Self-update
  @state() private selfUpdateBinary: File | null = null;

  // Server info
  @state() private serverVersion = '';
  @state() private serverCommit = '';

  // Site logs
  @state() private siteLogs: Record<string, string[]> = {};
  @state() private siteLogsVisible = new Set<string>();
  @state() private siteLogsLoading = new Set<string>();

  // Log viewer
  @state() private logEntries: LogEntry[] = [];
  @state() private wsConnected = false;
  @state() private showLogViewer = false;
  private ws: WebSocket | null = null;
  private wsReconnectTimer: ReturnType<typeof setTimeout> | null = null;

  connectedCallback(): void {
    super.connectedCallback();
    this.serverUrl = localStorage.getItem(STORAGE_KEYS.serverUrl) || '/api';
    this.adminKey = localStorage.getItem(STORAGE_KEYS.adminKey) || '';

    if (this.adminKey) {
      this.checkConnection();
    }
  }

  private saveToStorage(key: keyof typeof STORAGE_KEYS, value: string): void {
    localStorage.setItem(STORAGE_KEYS[key], value);
  }

  static styles = css`
    :host {
      display: block;
      font-family: system-ui, -apple-system, sans-serif;
      max-width: 800px;
      margin: 0 auto;
      padding: 20px;
      color: #1a1a1a;
    }

    * { box-sizing: border-box; }

    header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 24px;
      padding-bottom: 16px;
      border-bottom: 1px solid #e0e0e0;
    }

    h1 {
      margin: 0;
      font-size: 22px;
      font-weight: 600;
      display: flex;
      align-items: center;
      gap: 10px;
    }

    h1 span { font-size: 14px; color: #666; font-weight: 400; }

    .status-dot {
      width: 10px;
      height: 10px;
      border-radius: 50%;
      background: #ccc;
    }
    .status-dot.connected { background: #4caf50; }
    .status-dot.error { background: #f44336; }

    .version {
      font-size: 12px;
      color: #888;
      font-family: monospace;
    }

    .card {
      background: white;
      border: 1px solid #e0e0e0;
      border-radius: 8px;
      margin-bottom: 16px;
      overflow: hidden;
    }

    .card-header {
      padding: 12px 16px;
      background: #f8f9fa;
      border-bottom: 1px solid #e0e0e0;
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    .card-header h2 {
      margin: 0;
      font-size: 14px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: #666;
    }

    .card-body { padding: 16px; }

    label {
      display: block;
      font-size: 13px;
      font-weight: 500;
      color: #555;
      margin-bottom: 6px;
    }

    input[type="text"],
    input[type="password"],
    textarea {
      width: 100%;
      padding: 10px 12px;
      border: 1px solid #ddd;
      border-radius: 6px;
      font-size: 14px;
      margin-bottom: 12px;
      transition: border-color 0.2s;
    }

    input:focus, textarea:focus {
      outline: none;
      border-color: #0066cc;
    }

    textarea {
      font-family: 'SF Mono', Monaco, monospace;
      font-size: 13px;
      min-height: 80px;
      resize: vertical;
    }

    .input-row {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 12px;
    }

    .checkbox-row {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 12px;
    }

    .checkbox-row input { margin: 0; width: auto; }
    .checkbox-row label { margin: 0; font-weight: 400; }

    button {
      padding: 10px 16px;
      border: none;
      border-radius: 6px;
      font-size: 14px;
      font-weight: 500;
      cursor: pointer;
      transition: all 0.2s;
    }

    button:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .btn-primary { background: #0066cc; color: white; }
    .btn-primary:hover:not(:disabled) { background: #0055b3; }

    .btn-secondary { background: #f0f0f0; color: #333; }
    .btn-secondary:hover:not(:disabled) { background: #e0e0e0; }

    .btn-success { background: #2e7d32; color: white; }
    .btn-success:hover:not(:disabled) { background: #1b5e20; }

    .btn-danger { background: #d32f2f; color: white; }
    .btn-danger:hover:not(:disabled) { background: #b71c1c; }

    .btn-sm { padding: 6px 12px; font-size: 13px; }

    .btn-group {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
    }

    /* Sites list */
    .sites-empty {
      text-align: center;
      padding: 32px;
      color: #888;
    }

    .site-item {
      padding: 16px;
      border-bottom: 1px solid #eee;
      cursor: pointer;
      transition: background 0.15s;
    }

    .site-item:last-child { border-bottom: none; }
    .site-item:hover { background: #f8f9fa; }
    .site-item.selected { background: #e3f2fd; }

    .site-item-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 6px;
    }

    .site-domain {
      font-weight: 600;
      font-size: 15px;
    }

    .site-health {
      font-size: 12px;
      padding: 3px 10px;
      border-radius: 12px;
      font-weight: 500;
    }

    .site-health.healthy { background: #c8e6c9; color: #2e7d32; }
    .site-health.unhealthy { background: #ffcdd2; color: #c62828; }
    .site-health.unknown { background: #e0e0e0; color: #616161; }

    .site-meta {
      font-size: 13px;
      color: #666;
      display: flex;
      gap: 12px;
    }

    .site-actions {
      margin-top: 12px;
      display: flex;
      gap: 8px;
    }

    /* Deploy panel */
    .deploy-panel {
      background: #f8f9fa;
      border-top: 1px solid #e0e0e0;
      padding: 16px;
    }

    .deploy-panel h3 {
      margin: 0 0 12px;
      font-size: 14px;
      color: #333;
    }

    .file-input-wrapper {
      margin-bottom: 12px;
    }

    .file-info {
      font-size: 12px;
      color: #666;
      margin-top: 4px;
    }

    /* Toast notifications */
    .toasts {
      position: fixed;
      top: 20px;
      right: 20px;
      z-index: 1000;
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .toast {
      padding: 12px 16px;
      border-radius: 8px;
      background: white;
      box-shadow: 0 4px 12px rgba(0,0,0,0.15);
      max-width: 400px;
      animation: slideIn 0.2s ease;
    }

    @keyframes slideIn {
      from { transform: translateX(100%); opacity: 0; }
      to { transform: translateX(0); opacity: 1; }
    }

    .toast-title {
      font-weight: 600;
      font-size: 14px;
      margin-bottom: 4px;
    }

    .toast-message {
      font-size: 13px;
      color: #666;
      white-space: pre-wrap;
      word-break: break-word;
    }

    .toast.success { border-left: 4px solid #4caf50; }
    .toast.error { border-left: 4px solid #f44336; }
    .toast.info { border-left: 4px solid #2196f3; }

    .toast.success .toast-title { color: #2e7d32; }
    .toast.error .toast-title { color: #c62828; }
    .toast.info .toast-title { color: #1565c0; }

    /* Loading spinner */
    .spinner {
      display: inline-block;
      width: 16px;
      height: 16px;
      border: 2px solid #fff;
      border-top-color: transparent;
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
      margin-right: 8px;
    }

    .spinner.dark { border-color: #666; border-top-color: transparent; }

    @keyframes spin {
      to { transform: rotate(360deg); }
    }

    /* Create form */
    .create-form {
      border-top: 1px solid #e0e0e0;
      padding: 16px;
      background: #fafafa;
    }

    .create-form-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 16px;
    }

    .create-form-header h3 {
      margin: 0;
      font-size: 15px;
    }

    /* Collapsible section */
    .collapsible-header {
      cursor: pointer;
      user-select: none;
    }

    .collapse-icon {
      transition: transform 0.2s;
    }

    .collapse-icon.open {
      transform: rotate(90deg);
    }

    /* Log viewer */
    .log-viewer {
      background: #1e1e1e;
      border-radius: 6px;
      max-height: 400px;
      overflow-y: auto;
      padding: 12px;
      font-family: 'SF Mono', Monaco, 'Cascadia Code', monospace;
      font-size: 12px;
      line-height: 1.5;
    }

    .log-entry {
      white-space: pre-wrap;
      word-break: break-all;
      padding: 2px 0;
    }

    .log-time { color: #888; }
    .log-level-DEBUG { color: #64b5f6; }
    .log-level-INFO { color: #81c784; }
    .log-level-WARN { color: #ffb74d; }
    .log-level-ERROR { color: #e57373; }
    .log-msg { color: #ddd; }
    .log-extra { color: #999; }

    .log-header {
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .ws-status {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: #888;
    }
    .ws-status.connected { background: #4caf50; }
  `;

  private showToast(type: Toast['type'], title: string, message: string): void {
    const id = ++this.toastId;
    this.toasts = [...this.toasts, { id, type, title, message }];
    setTimeout(() => {
      this.toasts = this.toasts.filter(t => t.id !== id);
    }, 5000);
  }

  private parseError(data: ApiError | any): string {
    if (data.detail) {
      return `${data.error}: ${data.detail}`;
    }
    if (data.error) {
      return data.error.replace(/_/g, ' ');
    }
    return JSON.stringify(data);
  }

  private async apiRequest(
    path: string,
    options: RequestInit = {}
  ): Promise<{ ok: boolean; data: any }> {
    try {
      const response = await fetch(`${this.serverUrl}${path}`, {
        ...options,
        headers: {
          'X-Shipyard-Key': this.adminKey,
          ...options.headers,
        },
      });

      const data = await response.json();

      if (!response.ok) {
        const errorMsg = this.parseError(data);
        this.showToast('error', 'Request Failed', errorMsg);
        return { ok: false, data };
      }

      return { ok: true, data };
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Network error';
      this.showToast('error', 'Connection Error', errorMsg);
      return { ok: false, data: null };
    }
  }

  private async checkConnection(): Promise<void> {
    this.loading = true;
    try {
      const response = await fetch(`${this.serverUrl}/health`);
      if (response.ok) {
        const data = await response.json();
        this.connected = true;
        this.serverVersion = data.version || '';
        this.serverCommit = data.commit || '';
        this.listSites();
        this.connectWebSocket();
      } else {
        this.connected = false;
        this.disconnectWebSocket();
      }
    } catch {
      this.connected = false;
      this.disconnectWebSocket();
    } finally {
      this.loading = false;
    }
  }

  private async listSites(): Promise<void> {
    if (!this.adminKey) return;

    this.loading = true;
    const { ok, data } = await this.apiRequest('/sites');

    if (ok) {
      this.sites = data.sites || [];
      this.connected = true;
    }
    this.loading = false;
  }

  private async createSite(): Promise<void> {
    if (!this.newSiteDomain) {
      this.showToast('error', 'Validation Error', 'Domain is required');
      return;
    }

    this.loading = true;
    const { ok, data } = await this.apiRequest('/site/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        domain: this.newSiteDomain,
        ssl_enabled: this.newSiteSSL,
        with_backend: this.newSiteWithBackend,
      }),
    });

    if (ok) {
      this.showToast('success', 'Site Created', `API Key: ${data.api_key}\n\nSave this key for CI/CD deployments.`);
      this.newSiteDomain = '';
      this.showCreateForm = false;
      this.listSites();
    }
    this.loading = false;
  }

  private async deleteSite(site: SiteInfo, e: Event): Promise<void> {
    e.stopPropagation();

    if (!confirm(`Delete "${site.domain}"?\n\nThis will remove all files and configuration.`)) {
      return;
    }

    this.loading = true;
    const formData = new FormData();
    formData.append('site', site.domain);

    const { ok } = await this.apiRequest('/site/destroy', {
      method: 'POST',
      body: formData,
    });

    if (ok) {
      this.showToast('success', 'Site Deleted', site.domain);
      if (this.selectedSite?.domain === site.domain) {
        this.selectedSite = null;
      }
      this.listSites();
    }
    this.loading = false;
  }

  private selectSite(site: SiteInfo): void {
    if (this.selectedSite?.domain === site.domain) {
      this.selectedSite = null;
      this.nginxDefaultConfig = '';
    } else {
      this.selectedSite = site;
      this.nginxConfig = '';
      this.loadDefaultNginxConfig(site.domain);
    }
  }

  private async loadDefaultNginxConfig(domain: string): Promise<void> {
    const { ok, data } = await this.apiRequest(`/nginx/example?site=${encodeURIComponent(domain)}`);
    if (ok && data.default) {
      this.nginxDefaultConfig = data.default;
    }
  }

  private handleFileSelect(e: Event, target: 'frontend' | 'backend' | 'self'): void {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0] ?? null;

    if (target === 'frontend') this.frontendArtifact = file;
    else if (target === 'backend') this.backendArtifact = file;
    else this.selfUpdateBinary = file;
  }

  private async loadNginxExample(): Promise<void> {
    const { ok, data } = await this.apiRequest('/nginx/example');
    if (ok && data.example) {
      this.nginxConfig = data.example;
      this.showToast('info', 'Example Loaded', 'Edit the template variables as needed');
    }
  }

  private async deployFrontend(): Promise<void> {
    if (!this.selectedSite || !this.frontendArtifact) {
      this.showToast('error', 'Validation Error', 'Select a site and artifact');
      return;
    }

    this.loading = true;
    const formData = new FormData();
    formData.append('site', this.selectedSite.domain);
    formData.append('commit', this.commitHash || 'latest');
    formData.append('artifact', this.frontendArtifact);
    formData.append('nginx_config', this.nginxConfig);
    formData.append('update_latest', this.updateLatest ? 'true' : 'false');

    const { ok, data } = await this.apiRequest('/deploy/frontend', {
      method: 'POST',
      body: formData,
    });

    if (ok) {
      if (data.status === 'partially_deployed') {
        this.showToast('info', 'Partially Deployed',
          `Files deployed but nginx failed:\n${data.detail}`);
      } else {
        this.showToast('success', 'Frontend Deployed',
          `${this.selectedSite.domain} @ ${data.commit}`);
      }
      this.frontendArtifact = null;
    }
    this.loading = false;
  }

  private async deployBackend(): Promise<void> {
    if (!this.selectedSite || !this.backendArtifact) {
      this.showToast('error', 'Validation Error', 'Select a site and artifact');
      return;
    }

    this.loading = true;
    const formData = new FormData();
    formData.append('site', this.selectedSite.domain);
    formData.append('commit', this.commitHash || 'latest');
    formData.append('artifact', this.backendArtifact);
    if (this.binaryName) {
      formData.append('binary_name', this.binaryName);
    }

    const { ok, data } = await this.apiRequest('/deploy/backend', {
      method: 'POST',
      body: formData,
    });

    if (ok) {
      this.showToast('success', 'Backend Deployed',
        `${this.selectedSite.domain} @ ${data.commit}`);
      this.backendArtifact = null;
    }
    this.loading = false;
  }

  private async deploySelf(): Promise<void> {
    if (!this.selfUpdateBinary) {
      this.showToast('error', 'Validation Error', 'Select a binary file');
      return;
    }

    this.loading = true;
    const { ok } = await this.apiRequest('/deploy/self', {
      method: 'POST',
      body: this.selfUpdateBinary,
    });

    if (ok) {
      this.showToast('success', 'Self-Update Complete', 'Server is restarting...');
      this.selfUpdateBinary = null;
      // Reconnect after restart
      setTimeout(() => this.checkConnection(), 3000);
    }
    this.loading = false;
  }

  private async fetchSiteLogs(domain: string): Promise<void> {
    this.siteLogsLoading = new Set([...this.siteLogsLoading, domain]);
    const { ok, data } = await this.apiRequest(`/site/logs?site=${encodeURIComponent(domain)}&lines=200`);
    if (ok) {
      this.siteLogs = { ...this.siteLogs, [domain]: data.lines || [] };
    }
    const next = new Set(this.siteLogsLoading);
    next.delete(domain);
    this.siteLogsLoading = next;
  }

  private toggleSiteLogs(domain: string, e: Event): void {
    e.stopPropagation();
    const next = new Set(this.siteLogsVisible);
    if (next.has(domain)) {
      next.delete(domain);
    } else {
      next.add(domain);
      this.fetchSiteLogs(domain);
    }
    this.siteLogsVisible = next;
  }

  private connectWebSocket(): void {
    if (this.ws) return;

    // Build WSS URL from the server URL.
    let base = this.serverUrl;
    if (base.startsWith('http')) {
      base = base.replace(/^http/, 'ws');
    } else if (base.startsWith('/')) {
      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
      base = `${proto}//${location.host}${base}`;
    }
    const url = `${base}/ws/logs?key=${encodeURIComponent(this.adminKey)}`;

    const ws = new WebSocket(url);
    ws.onopen = () => {
      this.wsConnected = true;
    };
    ws.onmessage = (e) => {
      try {
        const entry: LogEntry = JSON.parse(e.data);
        this.logEntries = [...this.logEntries.slice(-499), entry];
      } catch {
        // ignore malformed messages
      }
    };
    ws.onclose = () => {
      this.wsConnected = false;
      this.ws = null;
      // Auto-reconnect after 3s if still connected to server.
      if (this.connected) {
        this.wsReconnectTimer = setTimeout(() => this.connectWebSocket(), 3000);
      }
    };
    ws.onerror = () => {
      ws.close();
    };
    this.ws = ws;
  }

  private disconnectWebSocket(): void {
    if (this.wsReconnectTimer) {
      clearTimeout(this.wsReconnectTimer);
      this.wsReconnectTimer = null;
    }
    if (this.ws) {
      this.ws.onclose = null;
      this.ws.close();
      this.ws = null;
    }
    this.wsConnected = false;
  }

  private formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  private renderToasts() {
    return html`
      <div class="toasts">
        ${this.toasts.map(toast => html`
          <div class="toast ${toast.type}">
            <div class="toast-title">${toast.title}</div>
            <div class="toast-message">${toast.message}</div>
          </div>
        `)}
      </div>
    `;
  }

  private renderConnection() {
    return html`
      <div class="card">
        <div class="card-header">
          <h2>Connection</h2>
          ${this.connected
            ? html`<span style="color: #2e7d32; font-size: 13px;">Connected</span>`
            : html`<span style="color: #888; font-size: 13px;">Not connected</span>`
          }
        </div>
        <div class="card-body">
          <div class="input-row">
            <div>
              <label>Server URL</label>
              <input
                type="text"
                .value=${this.serverUrl}
                @input=${(e: Event) => {
                  this.serverUrl = (e.target as HTMLInputElement).value;
                  this.saveToStorage('serverUrl', this.serverUrl);
                }}
                placeholder="https://shipyard.example.com:8443"
              />
            </div>
            <div>
              <label>Admin API Key</label>
              <input
                type="password"
                .value=${this.adminKey}
                @input=${(e: Event) => {
                  this.adminKey = (e.target as HTMLInputElement).value;
                  this.saveToStorage('adminKey', this.adminKey);
                }}
                placeholder="sk-admin-..."
              />
            </div>
          </div>
          <button
            class="btn-primary"
            @click=${this.checkConnection}
            ?disabled=${this.loading || !this.serverUrl || !this.adminKey}
          >
            ${this.loading ? html`<span class="spinner"></span>` : nothing}
            Connect
          </button>
        </div>
      </div>
    `;
  }

  private renderSites() {
    return html`
      <div class="card">
        <div class="card-header">
          <h2>Sites</h2>
          <div class="btn-group">
            <button
              class="btn-secondary btn-sm"
              @click=${this.listSites}
              ?disabled=${this.loading || !this.adminKey}
            >
              Refresh
            </button>
            <button
              class="btn-primary btn-sm"
              @click=${() => this.showCreateForm = !this.showCreateForm}
            >
              ${this.showCreateForm ? 'Cancel' : 'New Site'}
            </button>
          </div>
        </div>

        ${this.showCreateForm ? this.renderCreateForm() : nothing}

        ${this.sites.length === 0
          ? html`<div class="sites-empty">No sites configured</div>`
          : this.sites.map(site => this.renderSiteItem(site))
        }
      </div>
    `;
  }

  private renderCreateForm() {
    return html`
      <div class="create-form">
        <label>Domain</label>
        <input
          type="text"
          .value=${this.newSiteDomain}
          @input=${(e: Event) => this.newSiteDomain = (e.target as HTMLInputElement).value}
          placeholder="example.com"
        />
        <div class="checkbox-row">
          <input
            type="checkbox"
            id="newSiteSSL"
            .checked=${this.newSiteSSL}
            @change=${(e: Event) => this.newSiteSSL = (e.target as HTMLInputElement).checked}
          />
          <label for="newSiteSSL">Enable SSL (Let's Encrypt)</label>
        </div>
        <div class="checkbox-row">
          <input
            type="checkbox"
            id="newSiteBackend"
            .checked=${this.newSiteWithBackend}
            @change=${(e: Event) => this.newSiteWithBackend = (e.target as HTMLInputElement).checked}
          />
          <label for="newSiteBackend">Include backend (jail)</label>
        </div>
        <button
          class="btn-success"
          @click=${this.createSite}
          ?disabled=${this.loading || !this.newSiteDomain}
        >
          ${this.loading ? html`<span class="spinner"></span>` : nothing}
          Create Site
        </button>
      </div>
    `;
  }

  private renderSiteItem(site: SiteInfo) {
    const isSelected = this.selectedSite?.domain === site.domain;

    return html`
      <div
        class="site-item ${isSelected ? 'selected' : ''}"
        @click=${() => this.selectSite(site)}
      >
        <div class="site-item-header">
          <span class="site-domain">${site.domain}</span>
          <span class="site-health ${site.health}">${site.health}</span>
        </div>
        <div class="site-meta">
          <span>${site.backend_only ? 'Backend only' : site.has_backend ? 'Frontend + Backend' : 'Frontend only'}</span>
          <span>${site.ssl_enabled ? 'SSL enabled' : 'No SSL'}</span>
          ${site.frontend_root ? html`<span>${site.frontend_root}</span>` : nothing}
        </div>
        <div class="site-actions">
          ${site.has_backend ? html`
            <button
              class="btn-secondary btn-sm"
              @click=${(e: Event) => this.toggleSiteLogs(site.domain, e)}
              ?disabled=${this.siteLogsLoading.has(site.domain)}
            >
              ${this.siteLogsVisible.has(site.domain) ? 'Hide Logs' : 'View Logs'}
            </button>
          ` : nothing}
          <button
            class="btn-danger btn-sm"
            @click=${(e: Event) => this.deleteSite(site, e)}
            ?disabled=${this.loading}
          >
            Delete
          </button>
        </div>

        ${this.siteLogsVisible.has(site.domain) ? this.renderSiteLogPanel(site.domain) : nothing}
        ${isSelected ? this.renderDeployPanel(site) : nothing}
      </div>
    `;
  }

  private renderSiteLogPanel(domain: string) {
    const lines = this.siteLogs[domain] || [];
    const isLoading = this.siteLogsLoading.has(domain);

    return html`
      <div class="deploy-panel" @click=${(e: Event) => e.stopPropagation()}>
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
          <h3 style="margin: 0;">Application Logs</h3>
          <button
            class="btn-secondary btn-sm"
            @click=${() => this.fetchSiteLogs(domain)}
            ?disabled=${isLoading}
          >
            ${isLoading ? html`<span class="spinner dark"></span>` : nothing}
            Refresh
          </button>
        </div>
        <div class="log-viewer">
          ${lines.length === 0
            ? html`<div style="color: #888; text-align: center; padding: 16px;">
                ${isLoading ? 'Loading...' : 'No log entries yet'}
              </div>`
            : lines.map(line => html`
              <div class="log-entry"><span class="log-msg">${line}</span></div>
            `)
          }
        </div>
      </div>
    `;
  }

  private renderDeployPanel(site: SiteInfo) {
    return html`
      <div class="deploy-panel" @click=${(e: Event) => e.stopPropagation()}>
        <h3>Deploy to ${site.domain}</h3>

        <div class="input-row">
          <div>
            <label>Commit / Version</label>
            <input
              type="text"
              .value=${this.commitHash}
              @input=${(e: Event) => this.commitHash = (e.target as HTMLInputElement).value}
              placeholder="latest"
            />
          </div>
          ${!site.backend_only ? html`
            <div class="checkbox-row" style="margin-top: 28px;">
              <input
                type="checkbox"
                id="updateLatest"
                .checked=${this.updateLatest}
                @change=${(e: Event) => this.updateLatest = (e.target as HTMLInputElement).checked}
              />
              <label for="updateLatest">Update 'latest' symlink</label>
            </div>
          ` : nothing}
        </div>

        ${!site.backend_only ? html`
          <div class="file-input-wrapper">
            <label>Frontend Artifact (.zip)</label>
            <input type="file" accept=".zip" @change=${(e: Event) => this.handleFileSelect(e, 'frontend')} />
            ${this.frontendArtifact ? html`
              <div class="file-info">${this.frontendArtifact.name} (${this.formatSize(this.frontendArtifact.size)})</div>
            ` : nothing}
          </div>

          <div style="display: flex; justify-content: space-between; align-items: center;">
            <label>Nginx Config (optional — supports <code>&lt;%.Domain%&gt;</code> template variables)</label>
            <button
              class="btn-secondary btn-sm"
              @click=${this.loadNginxExample}
              ?disabled=${this.loading}
              style="margin-bottom: 6px;"
            >
              Load Example
            </button>
          </div>
          <textarea
            .value=${this.nginxConfig}
            @input=${(e: Event) => this.nginxConfig = (e.target as HTMLTextAreaElement).value}
            .placeholder=${this.nginxDefaultConfig || 'Leave empty for default SPA config. Use <%.Domain%>, <%.FrontendRoot%>, <%.ProxyPath%>, <%.ListenPort%> etc.'}
            rows="8"
          ></textarea>

          <div class="btn-group" style="margin-bottom: 16px;">
            <button
              class="btn-success"
              @click=${this.deployFrontend}
              ?disabled=${this.loading || !this.frontendArtifact}
            >
              ${this.loading ? html`<span class="spinner"></span>` : nothing}
              Deploy Frontend
            </button>
          </div>
        ` : nothing}

        ${site.has_backend ? html`
          <div class="file-input-wrapper">
            <label>Backend Artifact (.zip containing binary)</label>
            <input type="file" accept=".zip" @change=${(e: Event) => this.handleFileSelect(e, 'backend')} />
            ${this.backendArtifact ? html`
              <div class="file-info">${this.backendArtifact.name} (${this.formatSize(this.backendArtifact.size)})</div>
            ` : nothing}
          </div>
          <label>Binary Name (filename inside zip)</label>
          <input
            type="text"
            .value=${this.binaryName}
            @input=${(e: Event) => this.binaryName = (e.target as HTMLInputElement).value}
            placeholder="e.g., myapp-freebsd-arm64"
          />
          <button
            class="btn-success"
            @click=${this.deployBackend}
            ?disabled=${this.loading || !this.backendArtifact}
          >
            ${this.loading ? html`<span class="spinner"></span>` : nothing}
            Deploy Backend
          </button>
        ` : nothing}
      </div>
    `;
  }

  private renderSelfUpdate() {
    return html`
      <div class="card">
        <div class="card-header">
          <h2>Server Update</h2>
        </div>
        <div class="card-body">
          <label>Shipyard Binary (FreeBSD arm64)</label>
          <input type="file" @change=${(e: Event) => this.handleFileSelect(e, 'self')} />
          ${this.selfUpdateBinary ? html`
            <div class="file-info" style="margin-bottom: 12px;">
              ${this.selfUpdateBinary.name} (${this.formatSize(this.selfUpdateBinary.size)})
            </div>
          ` : nothing}
          <button
            class="btn-danger"
            @click=${this.deploySelf}
            ?disabled=${this.loading || !this.selfUpdateBinary || !this.adminKey}
          >
            ${this.loading ? html`<span class="spinner"></span>` : nothing}
            Deploy Update
          </button>
        </div>
      </div>
    `;
  }

  private renderLogViewer() {
    const levelClass = (level: string) => `log-level-${level}`;

    const formatExtra = (entry: LogEntry): string => {
      const skip = new Set(['time', 'level', 'msg']);
      const parts: string[] = [];
      for (const [k, v] of Object.entries(entry)) {
        if (!skip.has(k)) {
          parts.push(`${k}=${typeof v === 'string' ? v : JSON.stringify(v)}`);
        }
      }
      return parts.length > 0 ? ' ' + parts.join(' ') : '';
    };

    return html`
      <div class="card">
        <div class="card-header collapsible-header"
          @click=${() => this.showLogViewer = !this.showLogViewer}
        >
          <h2>
            <span class="collapse-icon ${this.showLogViewer ? 'open' : ''}">&#9654;</span>
            Server Logs
          </h2>
          <div class="log-header">
            <span class="ws-status ${this.wsConnected ? 'connected' : ''}"></span>
            <span style="font-size: 12px; color: #888;">
              ${this.wsConnected ? 'Streaming' : 'Disconnected'}
            </span>
            ${this.showLogViewer ? html`
              <button
                class="btn-secondary btn-sm"
                @click=${(e: Event) => { e.stopPropagation(); this.logEntries = []; }}
              >
                Clear
              </button>
            ` : nothing}
          </div>
        </div>
        ${this.showLogViewer ? html`
          <div class="card-body" style="padding: 0;">
            <div class="log-viewer">
              ${this.logEntries.length === 0
                ? html`<div style="color: #888; text-align: center; padding: 16px;">Waiting for log entries...</div>`
                : this.logEntries.map(entry => html`
                  <div class="log-entry">
                    <span class="log-time">${entry.time?.substring(11, 23) || ''}</span>
                    <span class="${levelClass(entry.level)}">[${entry.level}]</span>
                    <span class="log-msg">${entry.msg}</span>
                    <span class="log-extra">${formatExtra(entry)}</span>
                  </div>
                `)
              }
            </div>
          </div>
        ` : nothing}
      </div>
    `;
  }

  render() {
    return html`
      ${this.renderToasts()}

      <header>
        <h1>
          <span class="status-dot ${this.connected ? 'connected' : ''}"></span>
          Shipyard Helm
          <span>Admin Console</span>
        </h1>
        ${this.connected && this.serverVersion ? html`
          <span class="version">v${this.serverVersion} (${this.serverCommit})</span>
        ` : nothing}
      </header>

      ${this.renderConnection()}
      ${this.connected ? html`
        ${this.renderSites()}
        ${this.renderLogViewer()}
        ${this.renderSelfUpdate()}
      ` : nothing}
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'shipyard-admin': ShipyardAdmin;
  }
}
