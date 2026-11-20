package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"al.essio.dev/pkg/shellescape"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var execCommand = &cli.Command{
	Name:      "exec",
	Aliases:   []string{"ssh"},
	Usage:     "Open a remote terminal or run a command on hosts",
	ArgsUsage: "[-- COMMAND ...]",
	Flags: []cli.Flag{
		configFlag,
		&cli.StringSliceFlag{
			Name:    "address",
			Usage:   "Target host address (can be given multiple times)",
			Aliases: []string{"a"},
		},
		&cli.StringFlag{
			Name:    "role",
			Usage:   "Filter hosts by role",
			Aliases: []string{"r"},
		},
		&cli.BoolFlag{
			Name:    "first",
			Usage:   "Use only the first matching host",
			Aliases: []string{"f"},
		},
		&cli.BoolFlag{
			Name:    "parallel",
			Usage:   "Run command on hosts in parallel",
			Aliases: []string{"p"},
		},
		debugFlag,
		traceFlag,
		redactFlag,
	},
	Before: actions(initLogging, initConfig),
	Action: func(ctx *cli.Context) error {
		cfg, err := readConfig(ctx)
		if err != nil {
			return err
		}

		hosts := cfg.Spec.Hosts

		if addresses := ctx.StringSlice("address"); len(addresses) > 0 {
			hosts = hosts.Filter(func(h *cluster.Host) bool {
				for _, a := range addresses {
					if h.Address() == a {
						return true
					}
				}
				return false
			})
		}
		if role := ctx.String("role"); role != "" {
			hosts = hosts.WithRole(role)
		}
		if ctx.Bool("first") && len(hosts) > 0 {
			hosts = hosts[:1]
		}

		if len(hosts) == 0 {
			return fmt.Errorf("no hosts matched the given filters")
		}

		args := ctx.Args().Slice()
		var command string
		if len(args) > 0 {
			quoted := make([]string, len(args))
			for i, a := range args {
				quoted[i] = shellescape.Quote(a)
			}
			command = strings.Join(quoted, " ")
		}

		if command == "" && len(hosts) > 1 {
			return fmt.Errorf("interactive shell requires a single host, %d hosts matched (use --first to select the first one)", len(hosts))
		}

		if err := hosts.ParallelEach(ctx.Context, func(_ context.Context, h *cluster.Host) error {
			log.Debugf("connecting to %s", h.Address())
			if err := h.Connect(); err != nil {
				return fmt.Errorf("failed to connect to %s: %w", h.Address(), err)
			}
			return nil
		}); err != nil {
			return err
		}
		defer func() {
			_ = hosts.Each(ctx.Context, func(_ context.Context, h *cluster.Host) error {
				h.Disconnect()
				return nil
			})
		}()

		if command == "" {
			return hosts[0].ExecInteractive("")
		}

		var stdinData string
		if f, ok := ctx.App.Reader.(*os.File); ok {
			if stat, err := f.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
				data, err := io.ReadAll(f)
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				stdinData = string(data)
			}
		}

		multiHost := len(hosts) > 1
		var mu sync.Mutex
		execOnHost := func(_ context.Context, h *cluster.Host) error {
			var opts []exec.Option
			if stdinData != "" {
				opts = append(opts, exec.Stdin(stdinData))
			}

			output, err := h.ExecOutput(command, opts...)
			if output != "" {
				mu.Lock()
				if multiHost {
					for _, line := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
						fmt.Fprintf(ctx.App.Writer, "%s: %s\n", h.Address(), line)
					}
				} else {
					fmt.Fprint(ctx.App.Writer, output)
					if !strings.HasSuffix(output, "\n") {
						fmt.Fprintln(ctx.App.Writer)
					}
				}
				mu.Unlock()
			}
			if err != nil {
				return fmt.Errorf("%s: command failed: %w", h.Address(), err)
			}
			return nil
		}

		if ctx.Bool("parallel") {
			return hosts.ParallelEach(ctx.Context, execOnHost)
		}
		return hosts.Each(ctx.Context, execOnHost)
	},
}
