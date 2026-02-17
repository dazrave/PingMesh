// PingMesh Dashboard — API Client, Router, Global State

const API_BASE = '/api/v1';

// ── API Client ──────────────────────────────────────────────────────
const api = {
  async get(path) {
    const res = await fetch(API_BASE + path);
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
    return res.json();
  },
  async post(path, body) {
    const res = await fetch(API_BASE + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || `${res.status} ${res.statusText}`);
    }
    return res.json();
  },
  async put(path, body) {
    const res = await fetch(API_BASE + path, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error || `${res.status} ${res.statusText}`);
    }
    return res.json();
  },
  async del(path) {
    const res = await fetch(API_BASE + path, { method: 'DELETE' });
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
    return res.status === 204 ? null : res.json().catch(() => null);
  },
};

// ── Helpers ─────────────────────────────────────────────────────────
function relativeTime(tsMs) {
  if (!tsMs) return 'never';
  const diff = Date.now() - tsMs;
  if (diff < 0) return 'just now';
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return secs + 's ago';
  const mins = Math.floor(secs / 60);
  if (mins < 60) return mins + 'm ago';
  const hours = Math.floor(mins / 60);
  if (hours < 24) return hours + 'h ago';
  const days = Math.floor(hours / 24);
  return days + 'd ago';
}

function formatTs(tsMs) {
  if (!tsMs) return '—';
  return new Date(tsMs).toLocaleString();
}

function latencyClass(ms) {
  if (ms < 100) return 'latency-good';
  if (ms < 500) return 'latency-warn';
  return 'latency-bad';
}

function statusBadgeClass(status) {
  const map = {
    up: 'badge-up', online: 'badge-online',
    down: 'badge-down', offline: 'badge-offline',
    degraded: 'badge-degraded', suspect: 'badge-suspect',
    confirmed: 'badge-confirmed', resolved: 'badge-resolved',
  };
  return map[status] || 'badge-node';
}

// ── Router ──────────────────────────────────────────────────────────
const PAGES = ['dashboard', 'monitors', 'nodes', 'incidents', 'history'];
const DEFAULT_PAGE = 'dashboard';

function currentPage() {
  const hash = window.location.hash.replace('#', '');
  return PAGES.includes(hash) ? hash : DEFAULT_PAGE;
}

function navigateTo(page) {
  window.location.hash = '#' + page;
}

// ── Global Alpine Store ─────────────────────────────────────────────
document.addEventListener('alpine:init', () => {
  Alpine.store('app', {
    page: currentPage(),
    sidebarOpen: false,
    monitorCache: {},

    init() {
      window.addEventListener('hashchange', () => {
        this.page = currentPage();
        this.sidebarOpen = false;
      });
      // Prefetch monitors for name lookups
      this.refreshMonitorCache();
    },

    async refreshMonitorCache() {
      try {
        const monitors = await api.get('/monitors');
        const cache = {};
        for (const m of monitors) cache[m.id] = m;
        this.monitorCache = cache;
      } catch (e) {
        // silent — cache is best-effort
      }
    },

    monitorName(id) {
      const m = this.monitorCache[id];
      return m ? m.name : id.substring(0, 8) + '...';
    },

    toggleSidebar() { this.sidebarOpen = !this.sidebarOpen; },
    closeSidebar() { this.sidebarOpen = false; },
  });
});

// ── Polling Helper ──────────────────────────────────────────────────
function pollable(fetchFn, intervalMs) {
  return {
    _timer: null,
    startPolling() {
      this.refresh();
      this._timer = setInterval(() => this.refresh(), intervalMs);
    },
    stopPolling() {
      if (this._timer) { clearInterval(this._timer); this._timer = null; }
    },
    async refresh() {
      try { await fetchFn.call(this); } catch (e) { console.error('[poll]', e); }
    },
  };
}
