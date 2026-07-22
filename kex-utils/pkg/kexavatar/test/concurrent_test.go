package kexavatar_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/roidmc/kex-utils/pkg/kexavatar"
)

func TestConcurrentIdenticonGenerate(t *testing.T) {
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				id := int64(gid*iterations + i)
				_, err := kexavatar.Generate(id)
				if err != nil {
					errCh <- err
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent Generate failed: %v", err)
	}
}

func TestConcurrentSVGGenerate(t *testing.T) {
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				id := int64(gid*iterations + i)
				svg := kexavatar.GenerateSVG(id)
				if svg == "" {
					errCh <- ErrEmptyResult{}
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent GenerateSVG failed: %v", err)
	}
}

func TestConcurrentInitialGenerate(t *testing.T) {
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				name := "TestUser"
				svg := kexavatar.GenerateInitial(name)
				if svg == "" {
					errCh <- ErrEmptyResult{}
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent GenerateInitial failed: %v", err)
	}
}

func TestConcurrentGravatarURL(t *testing.T) {
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				email := "test@example.com"
				url := kexavatar.GravatarURL(email)
				if url == "" {
					errCh <- ErrEmptyResult{}
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent GravatarURL failed: %v", err)
	}
}

func TestDeterministicUnderConcurrency(t *testing.T) {
	const workers = 50
	id := int64(12345)

	results := make(chan string, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svg := kexavatar.GenerateSVG(id)
			results <- svg
		}()
	}

	wg.Wait()
	close(results)

	var first string
	count := 0
	for result := range results {
		if first == "" {
			first = result
		} else if result != first {
			t.Error("concurrent calls should produce identical results")
		}
		count++
	}

	if count != workers {
		t.Errorf("expected %d results, got %d", workers, count)
	}
}

type ErrEmptyResult struct{}

func (e ErrEmptyResult) Error() string {
	return "empty result"
}

func TestVisualOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual output test in short mode")
	}

	outputDir := filepath.Join(".", "temp", t.Name())
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("failed to create output directory: %v", err)
	}

	testCases := []struct {
		name string
		id   int64
	}{
		{"user_1", 1},
		{"user_42", 42},
		{"user_12345", 12345},
		{"user_negative", -100},
		{"user_zero", 0},
		{"user_large", 9223372036854775807},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			subDir := filepath.Join(outputDir, tc.name)
			if err := os.MkdirAll(subDir, 0755); err != nil {
				t.Fatalf("failed to create sub directory: %v", err)
			}

			pngData, err := kexavatar.Generate(tc.id)
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			pngPath := filepath.Join(subDir, "identicon.png")
			if err := os.WriteFile(pngPath, pngData, 0644); err != nil {
				t.Errorf("failed to write PNG: %v", err)
			}

			svgData := kexavatar.GenerateSVG(tc.id)
			svgPath := filepath.Join(subDir, "identicon.svg")
			if err := os.WriteFile(svgPath, []byte(svgData), 0644); err != nil {
				t.Errorf("failed to write SVG: %v", err)
			}
		})
	}

	nameCases := []struct {
		name  string
		input string
	}{
		{"name_john", "John"},
		{"name_alice", "Alice"},
		{"name_chinese", "\u5f20\u4e09"},
		{"name_emoji", "\U0001f600Test"},
		{"name_empty", ""},
	}

	for _, tc := range nameCases {
		t.Run(tc.name, func(t *testing.T) {
			subDir := filepath.Join(outputDir, tc.name)
			if err := os.MkdirAll(subDir, 0755); err != nil {
				t.Fatalf("failed to create sub directory: %v", err)
			}

			svgData := kexavatar.GenerateInitial(tc.input)
			svgPath := filepath.Join(subDir, "initial.svg")
			if err := os.WriteFile(svgPath, []byte(svgData), 0644); err != nil {
				t.Errorf("failed to write SVG: %v", err)
			}
		})
	}

	emailCases := []struct {
		name  string
		email string
	}{
		{"email_test", "test@example.com"},
		{"email_upper", "UPPER@EXAMPLE.COM"},
		{"email_empty", ""},
	}

	for _, tc := range emailCases {
		t.Run(tc.name, func(t *testing.T) {
			subDir := filepath.Join(outputDir, tc.name)
			if err := os.MkdirAll(subDir, 0755); err != nil {
				t.Fatalf("failed to create sub directory: %v", err)
			}

			url := kexavatar.GravatarURL(tc.email)
			urlPath := filepath.Join(subDir, "gravatar.txt")
			if err := os.WriteFile(urlPath, []byte(url), 0644); err != nil {
				t.Errorf("failed to write URL: %v", err)
			}
		})
	}

	t.Logf("Visual outputs written to: %s", outputDir)
}
