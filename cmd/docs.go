package cmd

import (
	"os"

	"github.com/charmbracelet/glamour"
	k0sctldocs "github.com/k0sproject/k0sctl/docs"
	"github.com/urfave/cli/v2"
)

var docsCommand = &cli.Command{
	Name:  "docs",
	Usage: "Show configuration documentation",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "jsonschema",
			Usage: "Output the JSON Schema for the configuration instead of the Markdown reference",
		},
	},
	Action: func(ctx *cli.Context) error {
		if ctx.Bool("jsonschema") {
			_, err := ctx.App.Writer.Write(k0sctldocs.SchemaJSON)
			return err
		}
		return showDocs(ctx)
	},
}

// showDocs renders configuration.md as styled terminal output when stdout is a
// terminal, or writes the raw Markdown when piped/redirected.
func showDocs(ctx *cli.Context) error {
	md := string(k0sctldocs.ConfigurationMD)

	outFile, isTerminal := ctx.App.Writer.(*os.File)
	if isTerminal {
		fi, err := outFile.Stat()
		if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
			isTerminal = false
		}
	}

	if isTerminal {
		// Render Markdown with ANSI styles for the terminal.
		rendered, err := glamour.Render(md, "auto")
		if err == nil {
			_, err = outFile.WriteString(rendered)
			return err
		}
		// Glamour failed — fall through to plain write below.
	}

	_, err := ctx.App.Writer.Write(k0sctldocs.ConfigurationMD)
	return err
}
