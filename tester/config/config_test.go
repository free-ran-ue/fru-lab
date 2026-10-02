package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tester.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(write(t, "apiToken: abc\n"))
	require.NoError(t, err)
	require.Equal(t, &Config{Listen: "0.0.0.0:9100", ApiToken: "abc", Logger: Logger{Level: "info"}}, cfg)
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(write(t, "listen: 127.0.0.1:9200\napiToken: abc\nlogger:\n  level: debug\n"))
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:9200", cfg.Listen)
	require.Equal(t, "debug", cfg.Logger.Level)
}

func TestLoadRequiresToken(t *testing.T) {
	_, err := Load(write(t, "listen: :9100\n"))
	require.ErrorContains(t, err, "apiToken must be set")
}
