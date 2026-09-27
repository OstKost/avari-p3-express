package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ManagerKey persists a credential separately from agent tokens, never logging it.
func ManagerKey(path string) (string, error) {
	b, e := os.ReadFile(path)
	if e == nil {
		key := strings.TrimSpace(string(b))
		if len(key) < 32 {
			return "", fmt.Errorf("manager key too short")
		}
		return key, nil
	}
	if !os.IsNotExist(e) {
		return "", e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return "", e
	}
	raw := make([]byte, 32)
	if _, e = rand.Read(raw); e != nil {
		return "", e
	}
	key := hex.EncodeToString(raw)
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return "", e
	}
	_, e = f.WriteString(key + "\n")
	closeErr := f.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	return key, nil
}
