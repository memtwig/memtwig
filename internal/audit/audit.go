package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	Timestamp string `json:"timestamp"`
	Actor     string `json:"actor"`
	Role      string `json:"role,omitempty"`
	Harness   string `json:"harness,omitempty"`
	Action    string `json:"action"`
	Vault     string `json:"vault,omitempty"`
	Path      string `json:"path,omitempty"`
	Outcome   string `json:"outcome"` // ok | denied | error
	Detail    string `json:"detail,omitempty"`
}

type Logger struct {
	mu   sync.Mutex
	path string
}

func New(path string) *Logger {
	return &Logger{path: path}
}

func (l *Logger) Log(ev Event) error {
	if l == nil || l.path == "" {
		return nil
	}
	if ev.Timestamp == "" {
		ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func (l *Logger) Tail(limit int) ([]Event, error) {
	if l == nil || l.path == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	f, err := os.Open(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var all []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err == nil {
			all = append(all, ev)
		}
	}
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	return all, sc.Err()
}
