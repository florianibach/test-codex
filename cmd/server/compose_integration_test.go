package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDockerComposeAppReachableAndPersistsDataAcrossRestart(t *testing.T) {
	if os.Getenv("RUN_DOCKER_TESTS") != "1" {
		t.Skip("set RUN_DOCKER_TESTS=1 to run docker compose integration tests")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available in test environment")
	}

	hostPort := "18081"
	projectName := fmt.Sprintf("mvp008-%d", time.Now().UnixNano())
	composeArgs := []string{"compose", "-p", projectName, "-f", filepath.Join("..", "..", "docker-compose.yml")}

	runCompose := func(args ...string) error {
		cmd := exec.Command("docker", append(composeArgs, args...)...)
		cmd.Dir = filepath.Join("..", "..")
		cmd.Env = append(os.Environ(), "HOST_PORT="+hostPort)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("docker %s failed: %w\n%s", strings.Join(args, " "), err, string(out))
		}
		return nil
	}

	defer func() {
		_ = runCompose("down", "-v")
	}()

	if err := runCompose("up", "-d", "--build"); err != nil {
		t.Fatalf("expected docker compose up to succeed: %v", err)
	}

	baseURL := "http://127.0.0.1:" + hostPort
	waitUntil := time.Now().Add(90 * time.Second)
	for {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if time.Now().After(waitUntil) {
			t.Fatalf("app not reachable on %s within timeout", baseURL)
		}
		time.Sleep(1 * time.Second)
	}

	postForm := func(path string, values url.Values) {
		client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := client.PostForm(baseURL+path, values)
		if err != nil {
			t.Fatalf("POST %s failed: %v", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 303 from %s, got %d (%s)", path, resp.StatusCode, string(body))
		}
	}

	postForm("/settings/profile", url.Values{"hourly_wage": {"42"}})
	postForm("/items/new", url.Values{"title": {"Compose Persist Item"}})

	if err := runCompose("restart", "app"); err != nil {
		t.Fatalf("expected docker compose restart to succeed: %v", err)
	}

	waitUntil = time.Now().Add(60 * time.Second)
	for {
		resp, err := http.Get(baseURL + "/")
		if err == nil && resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			text := string(body)
			if !strings.Contains(text, "Compose Persist Item") {
				t.Fatalf("expected item to persist after restart")
			}

			settingsResp, err := http.Get(baseURL + "/settings/profile")
			if err != nil {
				t.Fatalf("GET /settings/profile failed: %v", err)
			}
			settingsBody, _ := io.ReadAll(settingsResp.Body)
			settingsResp.Body.Close()
			if settingsResp.StatusCode != http.StatusOK || !strings.Contains(string(settingsBody), "value=\"42\"") {
				t.Fatalf("expected profile to persist after restart")
			}
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		if time.Now().After(waitUntil) {
			t.Fatalf("app did not recover after restart")
		}
		time.Sleep(1 * time.Second)
	}
}
