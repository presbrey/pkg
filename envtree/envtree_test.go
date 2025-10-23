package envtree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.EnvFileName != ".env" {
		t.Errorf("Expected EnvFileName to be '.env', got '%s'", config.EnvFileName)
	}

	if config.PreferGoResolver != false {
		t.Error("Expected PreferGoResolver to be false")
	}

	if config.Silent != false {
		t.Error("Expected Silent to be false")
	}

	if config.StopAtRoot != true {
		t.Error("Expected StopAtRoot to be true")
	}
}

func TestNew(t *testing.T) {
	loader := New(nil)
	if loader == nil {
		t.Fatal("Expected loader to be created")
	}

	if loader.config == nil {
		t.Fatal("Expected loader to have default config")
	}

	customConfig := &Config{
		EnvFileName: ".env.test",
		Silent:      true,
	}

	loader = New(customConfig)
	if loader.config.EnvFileName != ".env.test" {
		t.Error("Expected custom config to be used")
	}
}

func TestGetEnvFilePaths(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "envloader-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested directories
	level1 := filepath.Join(tmpDir, "level1")
	level2 := filepath.Join(level1, "level2")

	if err := os.MkdirAll(level2, 0755); err != nil {
		t.Fatalf("Failed to create nested dirs: %v", err)
	}

	// Create .env files at different levels
	envRoot := filepath.Join(tmpDir, ".env")
	envLevel1 := filepath.Join(level1, ".env")
	envLevel2 := filepath.Join(level2, ".env")

	for _, path := range []string{envRoot, envLevel1, envLevel2} {
		if err := os.WriteFile(path, []byte("TEST=true\n"), 0644); err != nil {
			t.Fatalf("Failed to create env file %s: %v", path, err)
		}
	}

	// Change to the deepest directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(level2); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Test the loader
	loader := New(&Config{Silent: true})
	paths, err := loader.GetEnvFilePaths()
	if err != nil {
		t.Fatalf("GetEnvFilePaths failed: %v", err)
	}

	// Should find 3 env files
	if len(paths) < 3 {
		t.Errorf("Expected to find at least 3 env files, found %d: %v", len(paths), paths)
	}

	// First path should be the closest one (level2)
	if len(paths) > 0 && paths[0] != envLevel2 {
		t.Errorf("Expected first path to be %s, got %s", envLevel2, paths[0])
	}
}

func TestLoadWithNoEnvFiles(t *testing.T) {
	// Create a temporary directory with no .env files
	tmpDir, err := os.MkdirTemp("", "envloader-test-noenv-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	loader := New(&Config{Silent: true})
	err = loader.Load()

	// Should not error when no env files exist
	if err != nil {
		t.Errorf("Expected no error when no env files exist, got: %v", err)
	}
}

func TestLoadWithValidEnvFile(t *testing.T) {
	// Create a temporary directory with a valid .env file
	tmpDir, err := os.MkdirTemp("", "envloader-test-valid-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	envFile := filepath.Join(tmpDir, ".env")
	testKey := "ENVLOADER_TEST_KEY"
	testValue := "test_value_12345"

	content := testKey + "=" + testValue + "\n"
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create env file: %v", err)
	}

	// Clear the env var first
	os.Unsetenv(testKey)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	loader := New(&Config{Silent: true})
	err = loader.Load()

	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check if the environment variable was loaded
	if os.Getenv(testKey) != testValue {
		t.Errorf("Expected env var %s to be %s, got %s", testKey, testValue, os.Getenv(testKey))
	}

	// Clean up
	os.Unsetenv(testKey)
}

func TestMustLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "envloader-test-must-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	loader := New(&Config{Silent: true})

	// Should not panic when there are no env files
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustLoad panicked unexpectedly: %v", r)
		}
	}()

	loader.MustLoad()
}

func TestCustomEnvFileName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "envloader-test-custom-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	customEnvFile := filepath.Join(tmpDir, ".env.custom")
	testKey := "ENVLOADER_CUSTOM_TEST"
	testValue := "custom_value"

	content := testKey + "=" + testValue + "\n"
	if err := os.WriteFile(customEnvFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create custom env file: %v", err)
	}

	os.Unsetenv(testKey)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	loader := New(&Config{
		EnvFileName: ".env.custom",
		Silent:      true,
	})

	err = loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if os.Getenv(testKey) != testValue {
		t.Errorf("Expected env var %s to be %s, got %s", testKey, testValue, os.Getenv(testKey))
	}

	os.Unsetenv(testKey)
}

func TestLoadDefault(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "envloader-test-default-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Should not error when called directly
	err = LoadDefault()
	if err != nil {
		t.Errorf("LoadDefault failed: %v", err)
	}
}

func TestMustLoadDefault(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "envloader-test-mustdefault-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustLoadDefault panicked unexpectedly: %v", r)
		}
	}()

	MustLoadDefault()
}

func TestAutoLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "envloader-test-auto-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Should not panic or error
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("AutoLoad panicked: %v", r)
		}
	}()

	AutoLoad()
}
