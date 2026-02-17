// PingMesh Dashboard — Incidents Page Component

function incidentsPage() {
  return {
    incidents: [],
    loading: true,
    error: null,
    activeOnly: true,
    ...pollable(async function () {
      const url = this.activeOnly ? '/incidents?active=true' : '/incidents';
      this.incidents = await api.get(url);
      this.loading = false;
    }, 10000),

    init() { this.startPolling(); },
    destroy() { this.stopPolling(); },

    setFilter(active) {
      this.activeOnly = active;
      this.loading = true;
      this.refresh();
    },

    monitorName(id) {
      return Alpine.store('app').monitorName(id);
    },
  };
}
