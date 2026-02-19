package main

import (
	orderedmap "github.com/wk8/go-ordered-map/v2"

	"github.com/invopop/jsonschema"
)

// prop is a helper to add a property to an ordered map.
func prop(m *orderedmap.OrderedMap[string, *jsonschema.Schema], key string, s *jsonschema.Schema) {
	m.Set(key, s)
}

func sshBastionSchema() *jsonschema.Schema {
	props := jsonschema.NewProperties()
	prop(props, "address", &jsonschema.Schema{
		Type:        "string",
		Description: "IP address or hostname of the bastion host.",
	})
	prop(props, "user", &jsonschema.Schema{
		Type:        "string",
		Description: "SSH user for the bastion host.",
		Default:     "root",
	})
	prop(props, "port", &jsonschema.Schema{
		Type:        "integer",
		Description: "SSH port on the bastion host.",
		Default:     22,
	})
	prop(props, "keyPath", &jsonschema.Schema{
		Type:        "string",
		Description: "Path to an SSH private key for the bastion host.",
	})
	return &jsonschema.Schema{
		Type:        "object",
		Description: "SSH bastion (jump host) configuration.",
		Properties:  props,
		Required:    []string{"address"},
	}
}

func sshSchema() *jsonschema.Schema {
	props := jsonschema.NewProperties()
	prop(props, "address", &jsonschema.Schema{
		Type:        "string",
		Description: "IP address or hostname of the host.",
	})
	prop(props, "user", &jsonschema.Schema{
		Type:        "string",
		Description: "Username to log in as.",
		Default:     "root",
	})
	prop(props, "port", &jsonschema.Schema{
		Type:        "integer",
		Description: "TCP port of the SSH service on the host.",
		Default:     22,
	})
	prop(props, "keyPath", &jsonschema.Schema{
		Type:        "string",
		Description: "Path to an SSH private key file. If a public key is used, ssh-agent is required. When left empty, the default value will first be looked for from the SSH configuration IdentityFile parameter.",
	})
	prop(props, "bastion", sshBastionSchema())
	return &jsonschema.Schema{
		Type:        "object",
		Description: "SSH connection options.",
		Properties:  props,
		Required:    []string{"address"},
	}
}

func winrmSchema() *jsonschema.Schema {
	props := jsonschema.NewProperties()
	prop(props, "address", &jsonschema.Schema{
		Type:        "string",
		Description: "IP address or hostname of the host.",
	})
	prop(props, "user", &jsonschema.Schema{
		Type:        "string",
		Description: "WinRM user name. The user must have administrative privileges.",
		Default:     "Administrator",
	})
	prop(props, "port", &jsonschema.Schema{
		Type:        "integer",
		Description: "TCP port for the WinRM endpoint. When useHTTPS is true, the default port automatically switches to 5986.",
		Default:     5985,
	})
	prop(props, "password", &jsonschema.Schema{
		Type:        "string",
		Description: "Password for the WinRM user. Required unless certificate-based authentication is configured.",
	})
	prop(props, "useHTTPS", &jsonschema.Schema{
		Type:        "boolean",
		Description: "Enable HTTPS for WinRM. When enabled, set caCertPath (and optionally certPath/keyPath) to verify the remote endpoint.",
		Default:     false,
	})
	prop(props, "insecure", &jsonschema.Schema{
		Type:        "boolean",
		Description: "Skip TLS certificate verification when connecting over HTTPS.",
		Default:     false,
	})
	prop(props, "useNTLM", &jsonschema.Schema{
		Type:        "boolean",
		Description: "Use NTLM authentication instead of basic authentication.",
		Default:     false,
	})
	prop(props, "caCertPath", &jsonschema.Schema{
		Type:        "string",
		Description: "Path to a CA bundle used to validate the WinRM server certificate.",
	})
	prop(props, "certPath", &jsonschema.Schema{
		Type:        "string",
		Description: "Client certificate for mutual TLS authentication.",
	})
	prop(props, "keyPath", &jsonschema.Schema{
		Type:        "string",
		Description: "Private key that matches certPath.",
	})
	prop(props, "tlsServerName", &jsonschema.Schema{
		Type:        "string",
		Description: "Override the TLS server name used during certificate verification.",
	})
	prop(props, "bastion", sshBastionSchema())
	return &jsonschema.Schema{
		Type:        "object",
		Description: "WinRM connection options for Windows worker nodes.",
		Properties:  props,
		Required:    []string{"address"},
	}
}

func openSSHSchema() *jsonschema.Schema {
	props := jsonschema.NewProperties()
	prop(props, "address", &jsonschema.Schema{
		Type:        "string",
		Description: "IP address, hostname, or ssh config host alias of the host.",
	})
	prop(props, "user", &jsonschema.Schema{
		Type:        "string",
		Description: "Username to connect as.",
	})
	prop(props, "port", &jsonschema.Schema{
		Type:        "integer",
		Description: "Remote SSH port.",
	})
	prop(props, "keyPath", &jsonschema.Schema{
		Type:        "string",
		Description: "Path to an SSH private key.",
	})
	prop(props, "configPath", &jsonschema.Schema{
		Type:        "string",
		Description: "Path to ssh config. Defaults to ~/.ssh/config with fallback to /etc/ssh/ssh_config.",
	})
	prop(props, "options", &jsonschema.Schema{
		Type:                 "object",
		Description:          "Additional options as key/value pairs passed to the ssh client as -o flags.",
		AdditionalProperties: jsonschema.FalseSchema,
	})
	prop(props, "disableMultiplexing", &jsonschema.Schema{
		Type:        "boolean",
		Description: "Disable SSH connection multiplexing. When true, every remote command requires reconnecting to the host.",
	})
	return &jsonschema.Schema{
		Type:        "object",
		Description: "OpenSSH client connection options. Uses the system's openssh client for connections.",
		Properties:  props,
		Required:    []string{"address"},
	}
}

func localhostSchema() *jsonschema.Schema {
	props := jsonschema.NewProperties()
	prop(props, "enabled", &jsonschema.Schema{
		Type:        "boolean",
		Description: "Must be set to true to enable the localhost connection.",
		Default:     true,
	})
	return &jsonschema.Schema{
		Type:        "object",
		Description: "Localhost connection options. Can be used to use the local host running k0sctl as a node in the cluster.",
		Properties:  props,
		Required:    []string{"enabled"},
	}
}
