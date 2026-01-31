package persistence

import (
	"encoding/json"
	"os"
	"sync"
)

// Persistence Manager for simple JSON storage when DB is unavailable
// Stores data in "data/" directory.

const (
	DataDir     = "data"
	DevicesFile = "data/devices.json"
	UsersFile   = "data/users.json"
)

type Store struct {
	mu sync.Mutex
}

var instance *Store
var once sync.Once

func GetStore() *Store {
	once.Do(func() {
		if _, err := os.Stat(DataDir); os.IsNotExist(err) {
			os.Mkdir(DataDir, 0755)
		}
		instance = &Store{}
	})
	return instance
}

// Generics would be nice, but interface{} is fine for this utility
func (s *Store) Save(filename string, data interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, bytes, 0644)
}

func (s *Store) Load(filename string, v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes, err := os.ReadFile(filename)
	if err != nil {
		return err // e.g. not exist
	}
	return json.Unmarshal(bytes, v)
}
