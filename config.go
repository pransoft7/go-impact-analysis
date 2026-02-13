package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	Target     TargetConfig      `json:"target"`
	Dependents []DependentConfig `json:"dependents"`
}

type TargetConfig struct {
	RepoURL           string `json:"repo_url"`
	ModulePrefix      string `json:"module_prefix"`
	ReleasedRef       string `json:"released_ref"`
	ModifiedLocalPath string `json:"modified_local_path"`
}

type DependentConfig struct {
	RepoURL    string `json:"repo_url"`
	ModulePath string `json:"module_path"`
	Ref        string `json:"ref"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
