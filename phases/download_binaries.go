package phases

// // DownloadBinaries phase downloads k0s binaries from the web to tempfiles on localhost
// type DownloadBinaries struct {
// 	phase.Analytics
// 	BasicPhase

// 	hostsByArch map[string][]*api.Host
// }

// // Title for the phase
// func (p *DownloadBinaries) Title() string {
// 	return "Download K0s Binaries"
// }

// func (p *DownloadBinaries) Prepare(config interface{}) error {
// 	p.Config = config.(*api.ClusterConfig)

// 	for _, h := range p.Config.Spec.Hosts {
// 		if !h.UploadBinary {
// 			continue
// 		}
// 		p.hostsByArch[h.Metadata.Arch] = append(p.hostsByArch[h.Metadata.Arch], h)
// 	}

// 	return nil
// }

// func (p *DownloadBinaries) ShouldRun() bool {
// 	return len(p.hostsByArch) > 0
// }

// // Run collect all the facts from hosts in parallel
// func (p *DownloadBinaries) Run() error {
// 	for arch, hosts := range p.hostsByArch {
// 		log.Infof("localhost: downloading k0s binaries for %s", arch)
// 		tmpfile, err := k0s.DownloadK0s(p.Config.Spec.K0s.Version, arch)
// 		if err != nil {
// 			return err
// 		}
// 		for _, h := range hosts {
// 			h.K0sBinary = tmpfile
// 		}
// 	}

// 	return nil
// }

// func (p *DownloadBinaries) CleanUp() {
// 	for _, hosts := range p.hostsByArch {
// 		tmpfile := hosts[0].K0sBinary
// 		if tmpfile == "" {
// 			continue
// 		}
// 		if err := os.Remove(tmpfile); err != nil {
// 			log.Warnf("failed to remove tempfile %s: %s", tmpfile, err)
// 		}
// 	}
// }
