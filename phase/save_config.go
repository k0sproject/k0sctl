package phase

// SaveConfig writes the k0sctl configuration to a configmap on the cluster
type SaveConfig struct {
	GenericPhase
}

// Title returns the phase title
func (p *SaveConfig) Title() string {
	return "Save k0sctl configuration"
}

// Run the phase
func (p *SaveConfig) Run() error {
	cm, err := p.Config.ConfigMap()
	if err != nil {
		return err
	}
	return cm.Save(p.Config.Spec.K0sLeader())
}
