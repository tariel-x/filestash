package plg_backend_rclone

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/Unknwon/goconfig"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

// Install installs the config file handler
func Install() {
	config.SetData(&Storage{})
}

// Storage implements config.Storage for saving and loading config
// data in a simple INI based file.
type Storage struct {
	mu sync.Mutex           // to protect the following variables
	gc *goconfig.ConfigFile // config file loaded - not thread safe
}

func NewStorage(input, password string) (*Storage, error) {
	if err := config.SetConfigPassword(password); err != nil {
		return nil, err
	}

	defer func() {
		config.ClearConfigPassword()
	}()

	s := &Storage{
		mu: sync.Mutex{},
		gc: nil,
	}

	return s, s.load(input)
}

// _load the config from permanent storage, decrypting if necessary
//
// mu must be held when calling this
func (s *Storage) load(input string) (err error) {
	// Make sure we have a sensible default even when we error
	defer func() {
		if s.gc == nil {
			s.gc, _ = goconfig.LoadFromReader(bytes.NewReader([]byte{}))
		}
	}()

	fd := bytes.NewReader([]byte(input))
	cryptReader, err := config.Decrypt(fd)
	if err != nil {
		return err
	}

	gc, err := goconfig.LoadFromReader(cryptReader)
	if err != nil {
		return err
	}
	s.gc = gc

	return nil
}

// Load the config from permanent storage, decrypting if necessary
func (s *Storage) Load() error {
	return nil
}

// Save the config to permanent storage, encrypting if necessary
func (s *Storage) Save() error {
	return nil
}

// Serialize the config into a string
func (s *Storage) Serialize() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var buf bytes.Buffer
	if err := goconfig.SaveConfigData(s.gc, &buf); err != nil {
		return "", fmt.Errorf("failed to save config file: %w", err)
	}

	return buf.String(), nil
}

// HasSection returns true if section exists in the config file
func (s *Storage) HasSection(section string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.gc.GetSection(section)
	return err == nil
}

// DeleteSection removes the named section and all config from the
// config file
func (s *Storage) DeleteSection(section string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gc.DeleteSection(section)
}

// GetSectionList returns a slice of strings with names for all the
// sections
func (s *Storage) GetSectionList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.gc.GetSectionList()
}

// GetKeyList returns the keys in this section
func (s *Storage) GetKeyList(section string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.gc.GetKeyList(section)
}

// GetValue returns the key in section with a found flag
func (s *Storage) GetValue(section string, key string) (value string, found bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, err := s.gc.GetValue(section, key)
	if err != nil {
		return "", false
	}
	return value, true
}

// SetValue sets the value under key in section
func (s *Storage) SetValue(section string, key string, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.HasPrefix(section, ":") {
		fs.Logf(nil, "Can't save config %q for on the fly backend %q", key, section)
		return
	}
	s.gc.SetValue(section, key, value)
}

// DeleteKey removes the key under section
func (s *Storage) DeleteKey(section string, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.gc.DeleteKey(section, key)
}

// Check the interface is satisfied
var _ config.Storage = (*Storage)(nil)
