// PingMesh Dashboard — History Page Component

function historyPage() {
  return {
    results: [],
    monitors: [],
    nodes: [],
    loading: true,
    error: null,
    filterMonitor: '',
    filterNode: '',
    filterLimit: 50,
    ...pollable(async function () {
      await this.fetchResults();
    }, 30000),

    init() {
      this.loadFilterOptions();
      this.startPolling();
    },
    destroy() { this.stopPolling(); },

    async loadFilterOptions() {
      try {
        const [monList, status] = await Promise.all([
          api.get('/monitors'),
          api.get('/status'),
        ]);
        this.monitors = monList || [];
        this.nodes = status.nodes || [];
      } catch (e) {
        // silent
      }
    },

    async fetchResults() {
      const params = new URLSearchParams();
      if (this.filterMonitor) params.set('monitor', this.filterMonitor);
      if (this.filterNode) params.set('node', this.filterNode);
      params.set('limit', this.filterLimit);
      const qs = params.toString();
      this.results = await api.get('/history' + (qs ? '?' + qs : ''));
      this.loading = false;
    },

    applyFilters() {
      this.loading = true;
      this.fetchResults();
    },

    loadMore() {
      this.filterLimit += 50;
      this.fetchResults();
    },

    monitorName(id) {
      return Alpine.store('app').monitorName(id);
    },
  };
}
