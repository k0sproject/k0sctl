package phases

// import (
// 	"fmt"
// 	"os"
// 	"text/tabwriter"
// )

// // Describe shows information about the current status of the cluster
// type Describe struct {
// 	BasicPhase
// }

// // Title for the phase
// func (p *Describe) Title() string {
// 	return "Display cluster status"
// }

// // Run does the actual saving of the local state file
// func (p *Describe) Run() error {

// 	p.hostReport()

// 	return nil
// }

// func (p *Describe) hostReport() {
// 	w := new(tabwriter.Writer)

// 	// minwidth, tabwidth, padding, padchar, flags
// 	w.Init(os.Stdout, 8, 8, 1, '\t', 0)

// 	fmt.Fprintf(w, "%s\t\t%s\t%s\t%s\t\n", "ADDRESS", "ROLE", "OS", "k0s")

// 	for _, h := range p.Config.Spec.Hosts {
// 		ev := "n/a"
// 		os := "n/a"
// 		if h.Metadata != nil {
// 			if h.Metadata.K0sVersion != "" {
// 				ev = h.Metadata.K0sVersion
// 			}
// 			if h.Metadata.Os != nil {
// 				os = fmt.Sprintf("%s/%s", h.Metadata.Os.ID, h.Metadata.Os.Version)
// 			}
// 		}
// 		fmt.Fprintf(w,
// 			"%s\t\t%s\t%s\t%s\t\n",
// 			h.Address,
// 			h.Role,
// 			os,
// 			ev,
// 		)
// 	}
// 	w.Flush()
// }
