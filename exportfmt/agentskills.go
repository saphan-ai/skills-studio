package exportfmt

import (
	"fmt"
	"os"
	"path/filepath"
)

// AgentSkills writes the skill as the open Agent Skills directory: <name>/ with
// SKILL.md byte for byte as read and every other file. A skill that Load
// accepted is already that format, so nothing is dropped and the loss list is
// empty. It also reads that directory back.
type AgentSkills struct{}

func (AgentSkills) Name() string { return "agentskills" }

func (AgentSkills) Export(s Skill) ([]File, []Loss, error) {
	if err := needSkill(s); err != nil {
		return nil, nil, err
	}
	if len(s.SkillMD) == 0 {
		return nil, nil, fmt.Errorf("the skill carries no SKILL.md text; export needs a skill read by Load")
	}
	files := []File{{Path: s.Name + "/SKILL.md", Data: s.SkillMD, Mode: 0o644}}
	for _, f := range s.Files {
		files = append(files, File{Path: s.Name + "/" + f.Path, Data: f.Data, Mode: f.Mode})
	}
	return files, nil, nil
}

// Import reads the directory Export wrote: dir holds <name>/SKILL.md. A skill
// directory itself (dir/SKILL.md) is read too.
func (AgentSkills) Import(dir string) (Skill, error) {
	if info, err := os.Lstat(filepath.Join(dir, "SKILL.md")); err == nil && info.Mode().IsRegular() {
		return Load(dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Skill{}, err
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() {
			if info, err := os.Lstat(filepath.Join(dir, e.Name(), "SKILL.md")); err == nil && info.Mode().IsRegular() {
				found = append(found, e.Name())
			}
		}
	}
	if len(found) != 1 {
		return Skill{}, fmt.Errorf("%s holds %d skill directories; import reads exactly one (<name>/SKILL.md)", dir, len(found))
	}
	return Load(filepath.Join(dir, found[0]))
}
