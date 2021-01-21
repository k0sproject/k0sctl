package cluster

// Spec defines cluster config spec section
type Spec struct {
	Hosts Hosts `yaml:"hosts" validate:"required,dive,min=1"`
	K0s   K0s

	k0sLeader *Host
}

// K0sLeader returns a controller host that is selected to be a "leader",
// or an initial node, a node that creates join tokens for other controllers.
func (c *Spec) K0sLeader() *Host {
	controllers := c.Hosts.Controllers()

	if c.k0sLeader == nil {
		// Pick the first server that reports to be running and persist the choice
		for _, h := range controllers {
			if h.Metadata.K0sVersion != "" { // TODO && h.InitSystem.ServiceIsRunning("k0s") {
				c.k0sLeader = h
			}
		}
	}

	// Still nil?  Fall back to first "server" host, do not persist selection.
	if c.k0sLeader == nil {
		return controllers.First()
	}

	return c.k0sLeader
}
