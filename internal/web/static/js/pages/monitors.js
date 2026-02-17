// PingMesh Dashboard — Monitors Page Component

function monitorsPage() {
  return {
    monitors: [],
    loading: true,
    error: null,
    groupFilter: '',
    showModal: false,
    editing: null, // null = create, object = edit
    showDeleteConfirm: false,
    deleteTarget: null,
    form: defaultForm(),
    saving: false,
    formError: null,
    ...pollable(async function () {
      let url = '/monitors';
      if (this.groupFilter) url += '?group=' + encodeURIComponent(this.groupFilter);
      this.monitors = await api.get(url);
      this.loading = false;
    }, 30000),

    init() { this.startPolling(); },
    destroy() { this.stopPolling(); },

    get groups() {
      const set = new Set();
      for (const m of this.monitors) {
        if (m.group_name) set.add(m.group_name);
      }
      return [...set].sort();
    },

    openCreate() {
      this.editing = null;
      this.form = defaultForm();
      this.formError = null;
      this.showModal = true;
    },

    openEdit(m) {
      this.editing = m;
      this.form = {
        name: m.name,
        group_name: m.group_name || '',
        check_type: m.check_type,
        target: m.target,
        port: m.port || 0,
        interval_ms: m.interval_ms,
        timeout_ms: m.timeout_ms,
        retries: m.retries,
        expected_status: m.expected_status || 200,
        expected_keyword: m.expected_keyword || '',
        dns_record_type: m.dns_record_type || 'A',
        dns_expected: m.dns_expected || '',
        failure_threshold: m.failure_threshold,
        recovery_threshold: m.recovery_threshold,
        enabled: m.enabled,
      };
      this.formError = null;
      this.showModal = true;
    },

    closeModal() { this.showModal = false; this.editing = null; },

    needsPort() {
      return ['tcp', 'http', 'https', 'http_keyword'].includes(this.form.check_type);
    },
    needsExpectedStatus() {
      return ['http', 'https', 'http_keyword'].includes(this.form.check_type);
    },
    needsKeyword() {
      return this.form.check_type === 'http_keyword';
    },
    needsDNS() {
      return this.form.check_type === 'dns';
    },

    async save() {
      this.saving = true;
      this.formError = null;
      try {
        const body = { ...this.form };
        body.port = parseInt(body.port) || 0;
        body.interval_ms = parseInt(body.interval_ms);
        body.timeout_ms = parseInt(body.timeout_ms);
        body.retries = parseInt(body.retries);
        body.expected_status = parseInt(body.expected_status) || 0;
        body.failure_threshold = parseInt(body.failure_threshold);
        body.recovery_threshold = parseInt(body.recovery_threshold);

        if (this.editing) {
          await api.put('/monitors/' + this.editing.id, body);
        } else {
          await api.post('/monitors', body);
        }
        this.closeModal();
        await this.refresh();
        Alpine.store('app').refreshMonitorCache();
      } catch (e) {
        this.formError = e.message;
      } finally {
        this.saving = false;
      }
    },

    confirmDelete(m) {
      this.deleteTarget = m;
      this.showDeleteConfirm = true;
    },

    async doDelete() {
      if (!this.deleteTarget) return;
      try {
        await api.del('/monitors/' + this.deleteTarget.id);
        this.showDeleteConfirm = false;
        this.deleteTarget = null;
        await this.refresh();
        Alpine.store('app').refreshMonitorCache();
      } catch (e) {
        alert('Delete failed: ' + e.message);
      }
    },

    async toggleEnabled(m) {
      try {
        await api.put('/monitors/' + m.id, { enabled: !m.enabled });
        await this.refresh();
      } catch (e) {
        alert('Toggle failed: ' + e.message);
      }
    },
  };
}

function defaultForm() {
  return {
    name: '',
    group_name: '',
    check_type: 'icmp',
    target: '',
    port: 0,
    interval_ms: 60000,
    timeout_ms: 5000,
    retries: 1,
    expected_status: 200,
    expected_keyword: '',
    dns_record_type: 'A',
    dns_expected: '',
    failure_threshold: 3,
    recovery_threshold: 2,
    enabled: true,
  };
}
