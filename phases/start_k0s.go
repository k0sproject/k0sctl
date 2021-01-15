package phases

// import (
// 	"fmt"

// 	"github.com/Mirantis/mcc/pkg/phase"
// 	"github.com/Mirantis/mcc/pkg/product/k0s/api"
// 	log "github.com/sirupsen/logrus"
// )

// // StartK0s start phase
// type StartK0s struct {
// 	phase.Analytics
// 	BasicPhase
// }

// // Title for the phase
// func (p *StartK0s) Title() string {
// 	return "Start K0s"
// }

// // Run executes phase on the hosts
// func (p *StartK0s) Run() error {
// 	for _, host := range p.Config.Spec.Hosts {
// 		if err := p.startK0s(host, p.Config); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func (p *StartK0s) startK0s(h *api.Host, c *api.ClusterConfig) error {
// 	initctl, err := h.ExecWithOutput("basename $(command -v rc-service systemd)")
// 	if err != nil {
// 		return err
// 	}

// 	if h.Role == "server" {
// 		// TODO: belongs to configurer
// 		if initctl == "systemd" {
// 			if err := h.Exec("sudo systemctl start k0s"); err != nil {
// 				return err
// 			}
// 		} else if initctl == "rc-system" {
// 			if err := h.Exec("sudo rc-service k0s start"); err != nil {
// 				return err
// 			}
// 		} else {
// 			return fmt.Errorf("k0s is not installed as systemd or openrc service")
// 		}

// 		// TODO: belongs to configurer
// 		token, err := h.ExecWithOutput("sudo k0s token create --role worker --wait")
// 		if err != nil {
// 			return err
// 		}

// 		c.Spec.K0s.Metadata.JoinToken = token

// 	} else if h.Role == "worker" {
// 		output, err := h.ExecWithOutput(`(rc-service -r k0s 2> /dev/null) || (systemctl show -p FragmentPath k0s.service 2> /dev/null | cut -d"=" -f2)`)
// 		if err != nil {
// 			return err
// 		}

// 		if output != "" {
// 			if err := h.Configurer.WriteFile(h.Configurer.K0sJoinTokenPath(), c.Spec.K0s.Metadata.JoinToken, "0700"); err != nil {
// 				return err
// 			}

// 			// TODO: belongs to configurer as it wont work on all platforms
// 			if err := h.Exec(fmt.Sprintf("sed -i 's^REPLACEME^%s^g' %s", h.Configurer.K0sJoinTokenPath(), output)); err != nil {
// 				return err
// 			}

// 			// TODO: belongs to configurer as it wont work on all platforms
// 			if initctl == "systemd" {
// 				if err := h.Exec("systemctl daemon-reload"); err != nil {
// 					return err
// 				}
// 			}
// 		}

// 		// TODO: belongs to configurer
// 		if initctl == "systemd" {
// 			if err := h.Exec("sudo systemctl start k0s"); err != nil {
// 				return err
// 			}
// 		} else if initctl == "rc-system" {
// 			if err := h.Exec("sudo /etc/init.d/k0s start"); err != nil {
// 				return err
// 			}
// 		} else {
// 			return fmt.Errorf("k0s is not installed as systemd or openrc service")
// 		}

// 	}

// 	log.Infof("%s: writing K0s config", h)

// 	return nil
// }
