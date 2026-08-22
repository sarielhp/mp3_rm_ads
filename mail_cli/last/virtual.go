package last

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"mail_cli/cache/label"
	"mail_cli/cfg_g"
)

// VirtualMailbox represents a named virtual folder containing message IDs and their source folders.
type VirtualMailbox struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	MessageIDs  []string          `json:"message_ids"`
	FolderMap   map[string]string `json:"folder_map"` // message ID -> original folder name
}

// virtualDir returns the path to the virtual mailboxes directory in cache.
func virtualDir(downloadDir string) string {
	return filepath.Join(downloadDir, "virtual")
}

// virtualPath returns the path to a specific virtual mailbox JSON file.
func virtualPath(downloadDir, name string) string {
	return filepath.Join(virtualDir(downloadDir), cfg_g.SanitizeLabelForCache(name)+".json")
}

// Save writes a VirtualMailbox to the cache. It also updates the cache/label index
// so that virtual mailboxes can be inspected using standard label cache APIs.
func Save(downloadDir string, vm *VirtualMailbox) error {
	dir := virtualDir(downloadDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(vm)
	if err != nil {
		return err
	}
	p := virtualPath(downloadDir, vm.Name)
	tmpPath := p + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, p); err != nil {
		return err
	}

	// Also mirror into label index under virtual/<name> or virtual_<name>
	virtualLabelName := "virtual/" + vm.Name
	_ = label.ReplaceAll(downloadDir, virtualLabelName, vm.MessageIDs)
	return nil
}

// Load reads a VirtualMailbox from the cache.
func Load(downloadDir, name string) (*VirtualMailbox, error) {
	p := virtualPath(downloadDir, name)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var vm VirtualMailbox
	if err := json.Unmarshal(data, &vm); err != nil {
		return nil, err
	}
	return &vm, nil
}

// FindMessageFolder looks up which source folder a message originated from across all virtual mailboxes or cached indexes.
func FindMessageFolder(downloadDir, msgID string) string {
	// First check virtual mailboxes
	vDir := virtualDir(downloadDir)
	if entries, err := os.ReadDir(vDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			if vm, err := Load(downloadDir, strings.TrimSuffix(e.Name(), ".json")); err == nil && vm != nil {
				if f, ok := vm.FolderMap[msgID]; ok && f != "" {
					return f
				}
			}
		}
	}
	return ""
}
