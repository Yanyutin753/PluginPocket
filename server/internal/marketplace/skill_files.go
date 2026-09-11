package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/Yanyutin753/PluginPocket/server/internal/filestore"
)

const MaxSkillBytes = 32 << 20
const MaxSkillFileBytes = 8 << 20

type SkillFile struct {
	Content    []byte
	Executable bool
}

type FileRef struct {
	filestore.Ref
	Executable bool `json:"executable"`
}

type EncodedFile struct {
	Encoding   string `json:"encoding"`
	Content    string `json:"content"`
	Executable bool   `json:"executable"`
	SHA256     string `json:"sha256,omitempty"`
	Size       int64  `json:"size"`
}

func SafeSkillPath(name string) bool {
	if name == "" || len(name) > 128 || name == "." || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.Contains(name, "..") || strings.ContainsAny(name, "\x00\\:<>\"|?*") || !utf8.ValidString(name) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.TrimRight(part, ". ") != part {
			return false
		}
		base, _, _ := strings.Cut(strings.ToUpper(part), ".")
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return false
		}
		for _, c := range part {
			if c < 32 {
				return false
			}
		}
	}
	return true
}

func ValidateSkillFiles(files map[string]SkillFile) error {
	if len(files) == 0 || len(files) > 32 {
		return errors.New("invalid skill file count")
	}
	skill, ok := files["SKILL.md"]
	if !ok || !utf8.Valid(skill.Content) {
		return errors.New("SKILL.md must be UTF-8")
	}
	total := 0
	for name, file := range files {
		total += len(file.Content)
		if !SafeSkillPath(name) || len(file.Content) > MaxSkillFileBytes || total > MaxSkillBytes {
			return errors.New("unsafe or oversized skill file")
		}
	}
	aliases := map[string]bool{}
	for name := range files {
		alias := strings.ToLower(name)
		if aliases[alias] {
			return errors.New("conflicting skill paths")
		}
		aliases[alias] = true
	}
	for name := range aliases {
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			if aliases[dir] {
				return errors.New("conflicting skill paths")
			}
		}
	}
	return nil
}

func EncodeSkillFiles(files map[string]SkillFile) map[string]EncodedFile {
	out := make(map[string]EncodedFile, len(files))
	for name, file := range files {
		sum := sha256.Sum256(file.Content)
		out[name] = EncodedFile{Encoding: "base64", Content: base64.StdEncoding.EncodeToString(file.Content), SHA256: hex.EncodeToString(sum[:]), Size: int64(len(file.Content)), Executable: file.Executable}
	}
	return out
}

func StoreSkillFiles(ctx context.Context, store *filestore.Store, files map[string]SkillFile) (map[string]FileRef, error) {
	if err := ValidateSkillFiles(files); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, filestore.ErrUnavailable
	}
	manifest := make(map[string]FileRef, len(files))
	for name, file := range files {
		ref, err := store.Put(ctx, file.Content)
		if err != nil {
			return nil, err
		}
		manifest[name] = FileRef{Ref: ref, Executable: file.Executable}
	}
	return manifest, nil
}

func LoadSkillFiles(ctx context.Context, o Options, raw json.RawMessage) (map[string]SkillFile, error) {
	var spec struct {
		Source   string             `json:"source"`
		Files    map[string]string  `json:"files"`
		Manifest map[string]FileRef `json:"file_manifest"`
		Repo     string             `json:"repo"`
		Path     string             `json:"path"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, err
	}
	files := map[string]SkillFile{}
	if len(spec.Manifest) > 0 {
		if o.Files == nil {
			return nil, filestore.ErrUnavailable
		}
		// Reject oversized metadata before allocating or requesting object bodies.
		var total int64
		if len(spec.Manifest) > 32 {
			return nil, errors.New("invalid skill file count")
		}
		for name, ref := range spec.Manifest {
			total += ref.Size
			if !SafeSkillPath(name) || ref.Size < 0 || ref.Size > MaxSkillFileBytes || total > MaxSkillBytes {
				return nil, errors.New("invalid skill manifest")
			}
		}
		for name, ref := range spec.Manifest {
			content, err := o.Files.Get(ctx, ref.Ref)
			if err != nil {
				return nil, err
			}
			files[name] = SkillFile{Content: content, Executable: ref.Executable}
		}
	} else if len(spec.Files) > 0 {
		for name, content := range spec.Files {
			files[name] = SkillFile{Content: []byte(content)}
		}
	} else if spec.Source == "github" {
		return ResolveSkillFilesV2(ctx, o, spec.Repo, spec.Path)
	}
	if err := ValidateSkillFiles(files); err != nil {
		return nil, err
	}
	return files, nil
}

func LegacySkillFiles(files map[string]SkillFile) (map[string]string, error) {
	out := make(map[string]string, len(files))
	for name, file := range files {
		if !utf8.Valid(file.Content) || strings.IndexByte(string(file.Content), 0) >= 0 || file.Executable {
			return nil, errors.New("binary skill requires format=2")
		}
		out[name] = string(file.Content)
	}
	return out, nil
}
