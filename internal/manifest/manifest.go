package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultFile     = "skillctl.yaml"
	DefaultSkillDir = ".agents/skills"
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
	Trust       Trust  `yaml:"trust,omitempty"`
}

// Trust declares how a skill's signatures are verified when it is pulled.
// It is either the scalar "none", an explicit opt-out of verification, or a
// map pinning verification material. A skill without a trust declaration is
// rejected: trust is declared when the skill is added via
// "skillctl pull --trust" and then travels with the manifest.
type Trust struct {
	// None opts out of signature verification for the skill.
	None bool
	// Key is the path to a public key, relative to the manifest.
	Key string
}

// Declared reports whether a trust declaration is present.
func (t Trust) Declared() bool {
	return t.None || t.Key != ""
}

// String returns a human-readable form of the trust declaration.
func (t Trust) String() string {
	switch {
	case t.None:
		return "none"
	case t.Key != "":
		return "key:" + t.Key
	}
	return "undeclared"
}

func (t Trust) MarshalYAML() (any, error) {
	switch {
	case t.None && t.Key != "":
		return nil, fmt.Errorf("invalid trust: none and key are mutually exclusive")
	case t.None:
		return "none", nil
	case t.Key != "":
		return map[string]string{"key": t.Key}, nil
	}
	return nil, nil
}

func (t *Trust) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag == "!!null" {
			return nil
		}
		if value.Value == "none" {
			t.None = true
			return nil
		}
		return fmt.Errorf("invalid trust %q: must be \"none\" or a map (e.g. {key: keys/foo.pub})", value.Value)
	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			key, val := value.Content[i], value.Content[i+1]
			switch key.Value {
			case "key":
				if val.Tag == "!!null" || val.Value == "" {
					return fmt.Errorf("trust: key must not be empty")
				}
				t.Key = val.Value
			default:
				return fmt.Errorf("unknown trust field %q", key.Value)
			}
		}
		if t.Key == "" {
			return fmt.Errorf("trust: key is required")
		}
		return nil
	}
	return fmt.Errorf("trust must be \"none\" or a map (e.g. {key: keys/foo.pub})")
}

// ParseTrust parses a --trust flag value of the form "none" or "key=KEY.pub".
func ParseTrust(s string) (Trust, error) {
	switch {
	case s == "none":
		return Trust{None: true}, nil
	case strings.HasPrefix(s, "key="):
		key := strings.TrimPrefix(s, "key=")
		if key == "" {
			return Trust{}, fmt.Errorf("--trust key= requires a path to a public key")
		}
		return Trust{Key: key}, nil
	}
	return Trust{}, fmt.Errorf("invalid trust %q: must be \"none\" or \"key=KEY.pub\"", s)
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

	for _, s := range m.Skills {
		if err := ValidateSkillName(s.Name); err != nil {
			return nil, fmt.Errorf("invalid skill in manifest: %w", err)
		}
	}

	return m, nil
}

func (m *Manifest) Save(path string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".skillctl-*.yaml")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing manifest: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming manifest: %w", err)
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

func (m *Manifest) Remove(name string) bool {
	for i, s := range m.Skills {
		if s.Name == name {
			m.Skills = append(m.Skills[:i], m.Skills[i+1:]...)
			return true
		}
	}
	return false
}

func (m *Manifest) Find(name string) *Skill {
	for i := range m.Skills {
		if m.Skills[i].Name == name {
			return &m.Skills[i]
		}
	}
	return nil
}

func ValidateSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name must not be empty")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("skill name %q must not contain path separators", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("skill name %q is not allowed", name)
	}
	return nil
}

func SkillExtractPath(name string) string {
	return filepath.Join(DefaultSkillDir, name)
}
