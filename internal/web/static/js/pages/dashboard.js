// PingMesh Dashboard — Dashboard Page Component

function dashboardPage() {
  return {
    health: null,
    status: null,
    recentChecks: [],
    loading: true,
    error: null,
    ...pollable(async function () {
      const [health, status, history] = await Promise.all([
        api.get('/health'),
        api.get('/status'),
        api.get('/history?limit=10'),
      ]);
      this.health = health;
      this.status = status;
      this.recentChecks = history || [];
      this.loading = false;
      this.error = null;
    }, 15000),

    init() { this.startPolling(); },
    destroy() { this.stopPolling(); },

    get onlineNodes() {
      if (!this.status?.nodes) return 0;
      return this.status.nodes.filter(n => n.status === 'online').length;
    },
    get totalNodes() {
      return this.status?.nodes?.length || 0;
    },
    get incidentCount() {
      return this.status?.active_incidents?.length || 0;
    },
    get confirmedIncidents() {
      if (!this.status?.active_incidents) return [];
      return this.status.active_incidents.filter(i => i.status === 'confirmed');
    },
    get suspectIncidents() {
      if (!this.status?.active_incidents) return [];
      return this.status.active_incidents.filter(i => i.status === 'suspect');
    },
  };
}
