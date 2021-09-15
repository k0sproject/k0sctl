package config

import (
	"fmt"

	"github.com/k0sproject/k0sctl/config/cluster"
	"github.com/k0sproject/rig/exec"
	"gopkg.in/yaml.v2"
)

type K0sctlConfigMap struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Data struct {
		Config string `yaml:"config"`
	} `yaml:"data"`
	Config *Cluster `yaml:"-"`
}

func (k *K0sctlConfigMap) Save(h *cluster.Host) error {
	plain, err := yaml.Marshal(k)
	if err != nil {
		return err
	}
	return h.Exec(h.Configurer.K0sCmdf("kubectl apply --namespace kube-system -f -"), exec.Sudo(h), exec.Stdin(string(plain)))
}

func LoadK0sctlConfigMap(h *cluster.Host) (*K0sctlConfigMap, error) {
	out, err := h.ExecOutput(h.Configurer.K0sCmdf("kubectl get configmap --namespace kube-system -o yaml k0sctl"), exec.Sudo(h))
	if err != nil {
		return nil, fmt.Errorf("no previous k0sctl configuration found: %w", err)
	}

	cm := &K0sctlConfigMap{}
	if err := yaml.Unmarshal([]byte(out), cm); err != nil {
		return nil, err
	}

	c := &Cluster{}
	if err := yaml.Unmarshal([]byte(cm.Data.Config), c); err != nil {
		return nil, err
	}

	return cm, nil
}
