// PingMesh Dashboard — Nodes Page Component

function nodesPage() {
  return {
    nodes: [],
    loading: true,
    error: null,
    showDeleteConfirm: false,
    deleteTarget: null,
    ...pollable(async function () {
      const status = await api.get('/status');
      this.nodes = status.nodes || [];
      this.loading = false;
    }, 15000),

    init() { this.startPolling(); },
    destroy() { this.stopPolling(); },

    confirmDelete(node) {
      this.deleteTarget = node;
      this.showDeleteConfirm = true;
    },

    async doDelete() {
      if (!this.deleteTarget) return;
      try {
        await api.del('/nodes/' + this.deleteTarget.id);
        this.showDeleteConfirm = false;
        this.deleteTarget = null;
        await this.refresh();
      } catch (e) {
        alert('Remove failed: ' + e.message);
      }
    },
  };
}
