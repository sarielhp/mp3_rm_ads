package msg

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mail_cli/message_info"
)

type Info struct {
	Date          time.Time
	IsSpam        bool
	IsPolitical   bool
	IsBlacklisted bool
	Classified    bool
	SpamScore     float32
}

type dirCache struct {
	mu      sync.RWMutex
	m       map[[36]byte]message_info.Compact
	path    string
	loaded  bool
	entries int64
}

var dirCaches sync.Map

func getDirCache(downloadDir string) *dirCache {
	v, _ := dirCaches.LoadOrStore(downloadDir, &dirCache{})
	return v.(*dirCache)
}

func binPath(downloadDir string) string {
	return filepath.Join(downloadDir, "message_info.bin")
}

func ensureLoaded(downloadDir string) {
	dc := getDirCache(downloadDir)
	binP := binPath(downloadDir)

	dc.mu.RLock()
	if dc.loaded && dc.path == binP {
		dc.mu.RUnlock()
		return
	}
	dc.mu.RUnlock()

	dc.mu.Lock()
	defer dc.mu.Unlock()
	if dc.loaded && dc.path == binP {
		return
	}

	m, err := message_info.ReadFromFile(binP)
	if err != nil {
		slog.Error("Failed to read message_info", slog.String("path", binP), slog.Any("error", err))
		m = make(map[[36]byte]message_info.Compact)
	}
	if len(m) == 0 {
		migrateOldCache(downloadDir, binP)
		m2, err2 := message_info.ReadFromFile(binP)
		if err2 == nil {
			m = m2
		}
	}
	dc.m = m
	dc.path = binP
	dc.loaded = true
	dc.entries = int64(len(m))
}

func migrateOldCache(downloadDir, binP string) {
	oldIndex := filepath.Join(downloadDir, "message_index.json")
	if _, err := os.Stat(oldIndex); os.IsNotExist(err) {
		return
	}
	slog.Info("Migrating old cache to message_info.bin", slog.String("downloadDir", downloadDir))

	if err := os.MkdirAll(filepath.Dir(binP), 0700); err != nil {
		slog.Warn("failed to create migration directory", slog.Any("error", err))
	}

	entries := make(map[[36]byte]message_info.Compact)
	var entryCount int

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(info.Name()) != ".eml" {
			return nil
		}
		id := info.Name()[:len(info.Name())-4]
		if id == "" {
			return nil
		}
		dateDir := filepath.Base(filepath.Dir(path))
		monthDir := filepath.Base(filepath.Dir(filepath.Dir(path)))
		yearDir := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(path))))
		var y, m, d int
		if _, err := fmt.Sscanf(yearDir+" "+monthDir+" "+dateDir, "%d %02d %02d", &y, &m, &d); err != nil {
			return nil
		}
		date := uint32(y*10000 + m*100 + d)
		entries[message_info.IDToBytes(id)] = message_info.Compact{Date: date}
		entryCount++
		return nil
	}

	if err := filepath.Walk(filepath.Join(downloadDir, "messages"), walkFn); err != nil {
		slog.Warn("failed to walk messages directory during migration", slog.Any("error", err))
	}

	if entryCount > 0 {
		if err := message_info.RewriteFile(binP, entries); err != nil {
			slog.Error("Failed to write migrated message_info", slog.Any("error", err))
			return
		}
	}

	os.Remove(filepath.Join(downloadDir, "message_index.json"))
	os.RemoveAll(filepath.Join(downloadDir, "classifications"))
	if files, err := os.ReadDir(downloadDir); err == nil {
		for _, f := range files {
			if !f.IsDir() && filepath.Ext(f.Name()) == ".json" {
				if f.Name() != "labels_cache.json" && f.Name() != "seen_cache.json" && f.Name() != "folder_index.json" {
					os.Remove(filepath.Join(downloadDir, f.Name()))
				}
			}
		}
	} else {
		os.Remove(filepath.Join(downloadDir, "trained_uids.json"))
		os.Remove(filepath.Join(downloadDir, "trained_uids_ham.json"))
		os.Remove(filepath.Join(downloadDir, "trained_message_ids.json"))
	}

	slog.Info("Migration complete", slog.Int("entries", entryCount))
}

func lookupPath(downloadDir, id string) (string, error) {
	info := getCompact(downloadDir, id)
	if info == nil {
		return "", fmt.Errorf("message %s not found in message_info", id)
	}
	y := info.Date / 10000
	m := (info.Date / 100) % 100
	d := info.Date % 100
	datePath := fmt.Sprintf("%04d/%02d/%02d", y, m, d)
	return filepath.Join(downloadDir, "messages", datePath, id+".eml"), nil
}

func getCompact(downloadDir, id string) *message_info.Compact {
	ensureLoaded(downloadDir)
	dc := getDirCache(downloadDir)
	idBytes := message_info.IDToBytes(id)
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	if c, ok := dc.m[idBytes]; ok {
		return &c
	}
	return nil
}

func setCompact(downloadDir, id string, info message_info.Compact) error {
	ensureLoaded(downloadDir)
	dc := getDirCache(downloadDir)
	idBytes := message_info.IDToBytes(id)

	dc.mu.Lock()
	dc.m[idBytes] = info
	dc.entries++
	ratio := float64(dc.entries-int64(len(dc.m))) / float64(dc.entries)
	needsCompact := dc.entries > 1000 && ratio > 0.25
	dc.mu.Unlock()

	p := binPath(downloadDir)
	if err := message_info.AppendToFile(p, idBytes, info); err != nil {
		return fmt.Errorf("append message_info: %w", err)
	}

	if needsCompact {
		if err := compactMap(downloadDir); err != nil {
			slog.Error("Failed to compact message_info", slog.Any("error", err))
		}
	}
	return nil
}

func compactMap(downloadDir string) error {
	dc := getDirCache(downloadDir)
	dc.mu.Lock()
	defer dc.mu.Unlock()
	p := binPath(downloadDir)
	if err := message_info.RewriteFile(p, dc.m); err != nil {
		return fmt.Errorf("compact message_info: %w", err)
	}
	dc.entries = int64(len(dc.m))
	slog.Info("Compacted message_info", slog.Int64("entries", dc.entries))
	return nil
}

func Read(downloadDir, msgID string) ([]byte, error) {
	p, err := lookupPath(downloadDir, msgID)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

func Exists(downloadDir, msgID string) (bool, error) {
	p, err := lookupPath(downloadDir, msgID)
	if err != nil {
		return false, nil
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func Store(downloadDir, msgID string, rawBytes []byte, receivedDate time.Time) error {
	dateVal := uint32(receivedDate.Year()*10000 + int(receivedDate.Month())*100 + receivedDate.Day())
	if err := setCompact(downloadDir, msgID, message_info.Compact{Date: dateVal}); err != nil {
		return err
	}
	p, err := lookupPath(downloadDir, msgID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	tmpPath := p + ".tmp"
	if err := os.WriteFile(tmpPath, rawBytes, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, p)
}

func Delete(downloadDir, msgID string) error {
	p, err := lookupPath(downloadDir, msgID)
	if err != nil {
		return nil
	}
	if err := os.Remove(p); err != nil {
		slog.Warn("failed to remove cached message", slog.String("path", p), slog.Any("error", err))
	}

	ensureLoaded(downloadDir)
	dc := getDirCache(downloadDir)
	idBytes := message_info.IDToBytes(msgID)
	dc.mu.Lock()
	delete(dc.m, idBytes)
	dc.mu.Unlock()
	return nil
}

func GetInfo(downloadDir, msgID string) (*Info, error) {
	c := getCompact(downloadDir, msgID)
	if c == nil {
		return nil, fmt.Errorf("message %s not in cache", msgID)
	}
	y := c.Date / 10000
	m := (c.Date / 100) % 100
	d := c.Date % 100
	return &Info{
		Date:          time.Date(int(y), time.Month(m), int(d), 0, 0, 0, 0, time.UTC),
		IsSpam:        c.IsSpam(),
		IsPolitical:   c.IsPolitical(),
		IsBlacklisted: c.IsBlacklisted(),
		Classified:    c.Classified(),
		SpamScore:     c.SpamScore,
	}, nil
}

func SetClassification(downloadDir, msgID string, isSpam, isPolitical, isBlacklisted bool, score float32) error {
	c := getCompact(downloadDir, msgID)
	if c == nil {
		return fmt.Errorf("message %s not in cache", msgID)
	}
	flags := c.Flags
	flags |= message_info.FlagClassified
	if isSpam {
		flags |= message_info.FlagIsSpam
	} else {
		flags &^= message_info.FlagIsSpam
	}
	if isPolitical {
		flags |= message_info.FlagIsPolitical
	} else {
		flags &^= message_info.FlagIsPolitical
	}
	if isBlacklisted {
		flags |= message_info.FlagIsBlacklisted
	} else {
		flags &^= message_info.FlagIsBlacklisted
	}
	return setCompact(downloadDir, msgID, message_info.Compact{
		Date:      c.Date,
		Flags:     flags,
		SpamScore: score,
	})
}

func ClearClassification(downloadDir, msgID string) error {
	c := getCompact(downloadDir, msgID)
	if c == nil {
		return nil
	}
	return setCompact(downloadDir, msgID, message_info.Compact{
		Date:  c.Date,
		Flags: 0,
	})
}

func ForEachID(downloadDir string, fn func(id string) bool) {
	ensureLoaded(downloadDir)
	dc := getDirCache(downloadDir)
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	for idBytes := range dc.m {
		id := string(idBytes[:])
		id = strings.TrimRight(id, "\x00")
		if fn(id) {
			return
		}
	}
}

func ResetAllClassifications(downloadDir string) error {
	ensureLoaded(downloadDir)
	dc := getDirCache(downloadDir)
	dc.mu.Lock()
	for idBytes, c := range dc.m {
		dc.m[idBytes] = message_info.Compact{
			Date:  c.Date,
			Flags: 0,
		}
	}
	dc.entries = int64(len(dc.m))
	dc.mu.Unlock()

	p := binPath(downloadDir)
	return message_info.RewriteFile(p, dc.m)
}
