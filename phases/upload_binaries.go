package phases

// import (
// 	"fmt"
// 	"strings"
// 	"sync"

// 	"github.com/k0sproject/k0sctl/phase"

// 	api "github.com/k0sproject/k0sctl/config"

// 	log "github.com/sirupsen/logrus"
// )

// // UploadBinaries phase implementation to collect facts (OS, version etc.) from hosts
// type UploadBinaries struct {
// 	phase.Analytics
// 	BasicPhase

// 	hosts []*api.Host
// }

// // Title for the phase
// func (p *UploadBinaries) Title() string {
// 	return "Upload K0s Binaries To Hosts"
// }

// func (p *UploadBinaries) Prepare(config interface{}) error {
// 	p.Config = config.(*api.ClusterConfig)

// 	for _, h := range p.Config.Spec.Hosts {
// 		if h.K0sBinary == "" {
// 			continue
// 		}
// 		p.hosts = append(p.hosts, h)
// 	}

// 	return nil
// }

// func (p *UploadBinaries) ShouldRun() bool {
// 	return len(p.hosts) > 0
// }

// // Run collect all the facts from hosts in parallel
// func (p *UploadBinaries) Run() error {
// 	var wg sync.WaitGroup
// 	var errors []string
// 	type erritem struct {
// 		host string
// 		err  error
// 	}
// 	ec := make(chan erritem, 1)

// 	wg.Add(len(p.hosts))

// 	for _, h := range p.hosts {
// 		go func(h *api.Host) {
// 			ec <- erritem{h.String(), p.upload(h)}
// 		}(h)
// 	}

// 	go func() {
// 		for e := range ec {
// 			if e.err != nil {
// 				errors = append(errors, fmt.Sprintf("%s: %s", e.host, e.err.Error()))
// 			}
// 			wg.Done()
// 		}
// 	}()

// 	wg.Wait()

// 	if len(errors) > 0 {
// 		return fmt.Errorf("failed on %d hosts:\n - %s", len(errors), strings.Join(errors, "\n - "))
// 	}

// 	return nil
// }

// func (p *UploadBinaries) upload(h *api.Host) error {
// 	if h.Metadata.K0sVersion == p.Config.Spec.K0s.Version {
// 		log.Infof("%s: K0s version %s already on host", h, h.Metadata.K0sVersion)
// 		return nil
// 	}

// 	binpath := h.Configurer.K0sBinaryPath()

// 	if err := h.Configurer.WriteFileLarge(h.K0sBinary, binpath); err != nil {
// 		h.Configurer.DeleteFile(binpath)
// 		return err
// 	}
// 	if h.IsWindows() {
// 		return nil
// 	}
// 	return h.Exec(fmt.Sprintf("sudo chmod 0755 %s", binpath))
// }
