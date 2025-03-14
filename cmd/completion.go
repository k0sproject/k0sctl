package cmd

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/urfave/cli/v2"
)

var completionCommand = &cli.Command{
	Name:   "completion",
	Hidden: false,
	Description: `Generates a shell auto-completion script.

   Typical locations for the generated output are:
    - Bash: /etc/bash_completion.d/k0sctl
    - Zsh: /usr/local/share/zsh/site-functions/_k0sctl
    - Fish: ~/.config/fish/completions/k0sctl.fish`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "shell",
			Usage:   "Shell to generate the script for",
			Value:   "bash",
			Aliases: []string{"s"},
			EnvVars: []string{"SHELL"},
		},
	},
	Action: func(ctx *cli.Context) error {
		switch path.Base(ctx.String("shell")) {
		case "bash":
			fmt.Fprint(ctx.App.Writer, bashTemplate())
		case "zsh":
			fmt.Fprint(ctx.App.Writer, zshTemplate())
		case "fish":
			t, err := ctx.App.ToFishCompletion()
			if err != nil {
				return err
			}
			fmt.Fprint(ctx.App.Writer, t)
		default:
			return fmt.Errorf("no completion script available for %s", ctx.String("shell"))
		}

		return nil
	},
}

func prog() string {
	p, err := os.Executable()
	if err != nil || strings.HasSuffix(p, "main") {
		return "k0sctl"
	}
	return path.Base(p)
}

func bashTemplate() string {
	return fmt.Sprintf(`#! /bin/bash

_k0sctl_bash_autocomplete() {
  if [[ "${COMP_WORDS[0]}" != "source" ]]; then
    local cur opts base
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    if [[ "$cur" == "-"* ]]; then
      opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} ${cur} --generate-bash-completion )
    else
      opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} --generate-bash-completion )
    fi
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
  fi
}

complete -o bashdefault -o default -o nospace -F _k0sctl_bash_autocomplete %s
`, prog())
}

// zshTemplate returns a completion script for zsh
func zshTemplate() string {
	p := prog()
	return fmt.Sprintf(`#compdef %s

_k0sctl_zsh_autocomplete() {
  local -a opts
  local cur
  cur=${words[-1]}
  if [[ "$cur" == "-"* ]]; then
    opts=("${(@f)$(_CLI_ZSH_AUTOCOMPLETE_HACK=1 ${words[@]:0:#words[@]-1} ${cur} --generate-bash-completion)}")
  else
    opts=("${(@f)$(_CLI_ZSH_AUTOCOMPLETE_HACK=1 ${words[@]:0:#words[@]-1} --generate-bash-completion)}")
  fi

  if [[ "${opts[1]}" != "" ]]; then
    _describe 'values' opts
  else
    _files
  fi

  return
}

compdef _k0sctl_zsh_autocomplete %s
`, p, p)
}
