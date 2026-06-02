// Package config defines pindex's typed configuration: built-in defaults that
// mirror the upstream PageIndex config.yaml, YAML loading layered over those
// defaults, and validation. The Python original used an untyped SimpleNamespace;
// pindex makes the config a validated struct.
package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all tunable knobs for indexing and retrieval.
type Config struct {
	// Model is the default LLM used for indexing-time reasoning.
	Model string `yaml:"model"`
	// RetrieveModel is the LLM used for retrieval/ask; falls back to Model when empty.
	RetrieveModel string `yaml:"retrieve_model"`

	// Extractor selects the PDF text-extraction backend (see internal/extract).
	Extractor string `yaml:"extractor"`

	// TOCCheckPageNum bounds how many leading pages are scanned for a table of contents.
	TOCCheckPageNum int `yaml:"toc_check_page_num"`
	// MaxPageNumEachNode and MaxTokenNumEachNode gate recursive splitting of large nodes.
	MaxPageNumEachNode  int `yaml:"max_page_num_each_node"`
	MaxTokenNumEachNode int `yaml:"max_token_num_each_node"`

	// Enrichment toggles (config-gated, mirroring PageIndex's if_add_* flags).
	// Defaults match the verified upstream config.yaml.
	AddNodeID         bool `yaml:"if_add_node_id"`
	AddNodeSummary    bool `yaml:"if_add_node_summary"`
	AddDocDescription bool `yaml:"if_add_doc_description"`
	AddNodeText       bool `yaml:"if_add_node_text"`
}

// Default returns the built-in defaults, mirroring upstream PageIndex config.yaml.
func Default() Config {
	return Config{
		Model:               "gpt-4o-2024-11-20",
		RetrieveModel:       "",
		Extractor:           "mupdf",
		TOCCheckPageNum:     20,
		MaxPageNumEachNode:  10,
		MaxTokenNumEachNode: 20000,
		AddNodeID:           true,
		AddNodeSummary:      true,
		AddDocDescription:   false,
		AddNodeText:         false,
	}
}

// RetrieveModelOrDefault returns RetrieveModel, falling back to Model when unset.
func (c Config) RetrieveModelOrDefault() string {
	if c.RetrieveModel != "" {
		return c.RetrieveModel
	}
	return c.Model
}

// Load reads a YAML file layered over Default(). An empty path or a missing file
// returns the defaults unchanged. A present file is parsed and validated.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

var knownExtractors = map[string]bool{
	"mupdf": true, "poppler": true, "purego": true, "vision": true,
}

// Validate checks invariants, returning a descriptive error on the first violation.
func (c Config) Validate() error {
	if c.Model == "" {
		return errors.New("config: model must not be empty")
	}
	if !knownExtractors[c.Extractor] {
		return fmt.Errorf("config: unknown extractor %q (want mupdf|poppler|purego|vision)", c.Extractor)
	}
	if c.TOCCheckPageNum < 0 {
		return fmt.Errorf("config: toc_check_page_num must be >= 0, got %d", c.TOCCheckPageNum)
	}
	if c.MaxPageNumEachNode <= 0 {
		return fmt.Errorf("config: max_page_num_each_node must be > 0, got %d", c.MaxPageNumEachNode)
	}
	if c.MaxTokenNumEachNode <= 0 {
		return fmt.Errorf("config: max_token_num_each_node must be > 0, got %d", c.MaxTokenNumEachNode)
	}
	return nil
}
