class NetworkDashboard {
  constructor() {
    this.ws = null;
    this.charts = {};
    this.data = {
      interfaces: {},
      devices: {},
      pings: {},
    };

    this.init();
  }

  init() {
    this.setupWebSocket();
    this.setupCharts();
    this.setupEventListeners();
    this.fetchInitialData();
  }

  setupWebSocket() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      console.log("WebSocket connected");
      this.updateConnectionStatus(true);
    };

    this.ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      this.updateDashboard(data);
    };

    this.ws.onclose = () => {
      console.log("WebSocket disconnected");
      this.updateConnectionStatus(false);
      // Reconnect after 5 seconds
      setTimeout(() => this.setupWebSocket(), 5000);
    };

    this.ws.onerror = (error) => {
      console.error("WebSocket error:", error);
      this.updateConnectionStatus(false);
    };
  }

  setupCharts() {
    // Bandwidth Chart
    const bandwidthCtx = document
      .getElementById("bandwidth-chart")
      .getContext("2d");
    this.charts.bandwidth = new Chart(bandwidthCtx, {
      type: "line",
      data: {
        labels: [],
        datasets: [
          {
            label: "Download (KB/s)",
            data: [],
            borderColor: "#4CAF50",
            backgroundColor: "rgba(76, 175, 80, 0.1)",
            tension: 0.4,
          },
          {
            label: "Upload (KB/s)",
            data: [],
            borderColor: "#2196F3",
            backgroundColor: "rgba(33, 150, 243, 0.1)",
            tension: 0.4,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          y: {
            beginAtZero: true,
            title: {
              display: true,
              text: "Speed (KB/s)",
            },
          },
        },
        plugins: {
          legend: {
            position: "top",
          },
        },
      },
    });

    // Latency Chart
    const latencyCtx = document
      .getElementById("latency-chart")
      .getContext("2d");
    this.charts.latency = new Chart(latencyCtx, {
      type: "line",
      data: {
        labels: [],
        datasets: [],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          y: {
            beginAtZero: true,
            title: {
              display: true,
              text: "Latency (ms)",
            },
          },
        },
        plugins: {
          legend: {
            position: "top",
          },
        },
      },
    });
  }

  setupEventListeners() {
    // Export button
    document.getElementById("export-btn").addEventListener("click", () => {
      document.getElementById("export-modal").style.display = "block";
    });

    // Close modal
    document.querySelector(".close").addEventListener("click", () => {
      document.getElementById("export-modal").style.display = "none";
    });

    // Close modal when clicking outside
    window.addEventListener("click", (event) => {
      const modal = document.getElementById("export-modal");
      if (event.target === modal) {
        modal.style.display = "none";
      }
    });

    // Show inactive devices toggle
    document.getElementById("show-inactive").addEventListener("change", () => {
      this.updateDevicesTable();
    });

    // Refresh buttons
    document
      .getElementById("refresh-interfaces")
      .addEventListener("click", () => {
        this.fetchTrafficData();
      });

    document.getElementById("refresh-devices").addEventListener("click", () => {
      this.fetchDevicesData();
    });

    document.getElementById("refresh-pings").addEventListener("click", () => {
      this.fetchPingData();
    });
  }

  async fetchInitialData() {
    try {
      await Promise.all([
        this.fetchTrafficData(),
        this.fetchDevicesData(),
        this.fetchPingData(),
      ]);
    } catch (error) {
      console.error("Error fetching initial data:", error);
    }
  }

  async fetchTrafficData() {
    try {
      const response = await fetch("/api/traffic");
      const result = await response.json();
      if (result.status === "success") {
        this.data.interfaces = result.data.interfaces;
        this.updateInterfacesTable();
      }
    } catch (error) {
      console.error("Error fetching traffic data:", error);
    }
  }

  async fetchDevicesData() {
    try {
      const response = await fetch("/api/devices");
      const result = await response.json();
      if (result.status === "success") {
        this.data.devices = {};
        result.data.devices.forEach((device) => {
          this.data.devices[device.ip] = device;
        });
        this.updateDevicesTable();
      }
    } catch (error) {
      console.error("Error fetching devices data:", error);
    }
  }

  async fetchPingData() {
    try {
      const response = await fetch("/api/ping");
      const result = await response.json();
      if (result.status === "success") {
        this.data.pings = result.data.pings;
        this.updatePingTable();
      }
    } catch (error) {
      console.error("Error fetching ping data:", error);
    }
  }

  updateDashboard(data) {
    this.data = {
      interfaces: data.interfaces || {},
      devices: data.devices || {},
      pings: data.pings || {},
    };

    this.updateOverviewCards(data);
    this.updateCharts();
    this.updateTables();
    this.updateLastUpdate();
  }

  updateOverviewCards(data) {
    // Total bandwidth
    const totalRx = this.formatBytes(data.total_rx || 0);
    const totalTx = this.formatBytes(data.total_tx || 0);
    document.getElementById("total-rx").textContent = totalRx + "/s";
    document.getElementById("total-tx").textContent = totalTx + "/s";

    // Device counts
    document.getElementById("active-devices").textContent =
      data.active_devices || 0;
    document.getElementById("total-devices").textContent =
      data.total_devices || 0;

    // Network health
    this.updateNetworkHealth();
  }

  updateNetworkHealth() {
    const pings = Object.values(this.data.pings);
    if (pings.length > 0) {
      const avgLatency =
        pings.reduce((sum, ping) => sum + (ping.avg_latency || 0), 0) /
        pings.length;
      const avgPacketLoss =
        pings.reduce((sum, ping) => sum + (ping.packet_loss || 0), 0) /
        pings.length;

      document.getElementById("avg-latency").textContent =
        this.formatLatency(avgLatency);
      document.getElementById("packet-loss").textContent =
        avgPacketLoss.toFixed(1) + "%";
    }
  }

  updateCharts() {
    this.updateBandwidthChart();
    this.updateLatencyChart();
  }

  updateBandwidthChart() {
    const chart = this.charts.bandwidth;
    const now = new Date();

    // Calculate total speeds
    let totalRx = 0,
      totalTx = 0;
    Object.values(this.data.interfaces).forEach((iface) => {
      totalRx += iface.speed_rx || 0;
      totalTx += iface.speed_tx || 0;
    });

    // Add new data point
    chart.data.labels.push(now.toLocaleTimeString());
    chart.data.datasets[0].data.push(totalRx / 1024); // Convert to KB/s
    chart.data.datasets[1].data.push(totalTx / 1024); // Convert to KB/s

    // Keep only last 20 points
    if (chart.data.labels.length > 20) {
      chart.data.labels.shift();
      chart.data.datasets[0].data.shift();
      chart.data.datasets[1].data.shift();
    }

    chart.update("none");
  }

  updateLatencyChart() {
    const chart = this.charts.latency;
    const pings = this.data.pings;

    // Update datasets for each ping target
    const hosts = Object.keys(pings);
    const colors = ["#FF6384", "#36A2EB", "#FFCE56", "#4BC0C0", "#9966FF"];

    chart.data.datasets = hosts.map((host, index) => ({
      label: host,
      data: [],
      borderColor: colors[index % colors.length],
      backgroundColor: colors[index % colors.length] + "20",
      tension: 0.4,
    }));

    // Get latest data points
    if (hosts.length > 0) {
      const latestTime = new Date().toLocaleTimeString();
      chart.data.labels = [latestTime];

      hosts.forEach((host, index) => {
        const ping = pings[host];
        const latencyMs = ping.last_latency ? ping.last_latency / 1000000 : 0;
        chart.data.datasets[index].data = [latencyMs];
      });
    }

    chart.update("none");
  }

  updateTables() {
    this.updateInterfacesTable();
    this.updateDevicesTable();
    this.updatePingTable();
  }

  updateInterfacesTable() {
    const tbody = document.querySelector("#interfaces-table tbody");
    tbody.innerHTML = "";

    Object.values(this.data.interfaces).forEach((iface) => {
      const row = tbody.insertRow();
      row.innerHTML = `
                <td><strong>${iface.name}</strong></td>
                <td>${this.formatBytes(iface.bytes_rx)}</td>
                <td>${this.formatBytes(iface.bytes_tx)}</td>
                <td class="speed-rx">${this.formatBytes(iface.speed_rx)}/s</td>
                <td class="speed-tx">${this.formatBytes(iface.speed_tx)}/s</td>
            `;
    });
  }

  updateDevicesTable() {
    const tbody = document.querySelector("#devices-table tbody");
    const showInactive = document.getElementById("show-inactive").checked;
    tbody.innerHTML = "";

    Object.values(this.data.devices).forEach((device) => {
      if (!showInactive && !device.is_active) return;

      const row = tbody.insertRow();
      const statusClass = device.is_active ? "active" : "inactive";
      const statusText = device.is_active ? "Online" : "Offline";

      row.innerHTML = `
                <td><strong>${device.ip}</strong></td>
                <td><code>${device.mac}</code></td>
                <td>${device.hostname || "-"}</td>
                <td>${this.formatTimestamp(device.last_seen)}</td>
                <td><span class="status ${statusClass}">${statusText}</span></td>
            `;
    });
  }

  updatePingTable() {
    const tbody = document.querySelector("#ping-table tbody");
    tbody.innerHTML = "";

    Object.values(this.data.pings).forEach((ping) => {
      const row = tbody.insertRow();
      const statusClass =
        ping.packet_loss < 10
          ? "good"
          : ping.packet_loss < 50
          ? "warning"
          : "error";

      row.innerHTML = `
                <td><strong>${ping.host}</strong></td>
                <td>${this.formatLatency(ping.last_latency)}</td>
                <td>${this.formatLatency(ping.avg_latency)}</td>
                <td class="${statusClass}">${ping.packet_loss.toFixed(1)}%</td>
                <td><span class="status ${statusClass}">●</span></td>
            `;
    });
  }

  updateConnectionStatus(connected) {
    const status = document.getElementById("connection-status");
    if (connected) {
      status.textContent = "● Connected";
      status.className = "status connected";
    } else {
      status.textContent = "● Disconnected";
      status.className = "status disconnected";
    }
  }

  updateLastUpdate() {
    document.getElementById(
      "last-update"
    ).textContent = `Last update: ${new Date().toLocaleTimeString()}`;
  }

  formatBytes(bytes) {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
  }

  formatLatency(nanoseconds) {
    if (!nanoseconds) return "0ms";
    const ms = nanoseconds / 1000000;
    return ms.toFixed(1) + "ms";
  }

  formatTimestamp(timestamp) {
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now - date;

    if (diff < 60000) return "Just now";
    if (diff < 3600000) return Math.floor(diff / 60000) + "m ago";
    if (diff < 86400000) return Math.floor(diff / 3600000) + "h ago";

    return date.toLocaleDateString() + " " + date.toLocaleTimeString();
  }
}

// Export functions
function exportData(type) {
  const url = `/api/export/csv?type=${type}`;
  const link = document.createElement("a");
  link.href = url;
  link.download = `network_${type}_${
    new Date().toISOString().split("T")[0]
  }.csv`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);

  // Close modal
  document.getElementById("export-modal").style.display = "none";
}

// Initialize dashboard when DOM is loaded
document.addEventListener("DOMContentLoaded", () => {
  window.dashboard = new NetworkDashboard();
});
