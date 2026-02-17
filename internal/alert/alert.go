package alert

import (
	"log"

	"github.com/pingmesh/pingmesh/internal/model"
)

// Dispatcher handles sending alerts through configured channels.
type Dispatcher struct {
	// TODO (M4): Add webhook URLs, email config, etc.
}

// NewDispatcher creates a new alert dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{}
}

// SendAlert sends an alert for a confirmed incident.
// For MVP, this just logs. M4 will add webhook support.
func (d *Dispatcher) SendAlert(incident *model.Incident, monitor *model.Monitor) {
	log.Printf("[ALERT] INCIDENT CONFIRMED: monitor=%s (%s) target=%s incident=%s confirming_nodes=%v",
		monitor.Name, monitor.CheckType, monitor.Target, incident.ID, incident.ConfirmingNodes)
}

// SendRecovery sends a recovery notification.
func (d *Dispatcher) SendRecovery(incident *model.Incident, monitor *model.Monitor) {
	log.Printf("[ALERT] INCIDENT RESOLVED: monitor=%s (%s) target=%s incident=%s",
		monitor.Name, monitor.CheckType, monitor.Target, incident.ID)
}
