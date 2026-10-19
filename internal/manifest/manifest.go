package manifest

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultFile     = "skillctl.yaml"
	DefaultSkillDir = "skills"
	APIVersion      = "v1"
)

type Manifest struct {
	APIVersion string  `yaml:"apiVersion"`
	Skills     []Skill `yaml:"skills"`
}

type Skill struct {
	Name        string `yaml:"name"`
	Image       string `yaml:"image"`
	Digest      string `yaml:"digest"`
	ExtractPath string `yaml:"extractPath"`
}

func New() *Manifest {
	return &Manifest{
		APIVersion: APIVersion,
		Skills:     []Skill{},
	}
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	m := &Manifest{}
	if err := yaml.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return m, nil
}

func (m *Manifest) Save(path string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

func (m *Manifest) AddOrUpdate(skill Skill) {
	for i, s := range m.Skills {
		if s.Name == skill.Name {
			m.Skills[i] = skill
			return
		}
	}
	m.Skills = append(m.Skills, skill)
}

func (m *Manifest) Find(name string) *Skill {
	for i := range m.Skills {
		if m.Skills[i].Name == name {
			return &m.Skills[i]
		}
	}
	return nil
}

func SkillExtractPath(name string) string {
	return filepath.Join(DefaultSkillDir, name)
}
