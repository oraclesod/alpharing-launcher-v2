package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestElevatedModEntryUninstallsWithoutLaunching(t *testing.T) {
	gamePath := t.TempDir()
	modDir := filepath.Join(gamePath, modDLLSubpath)
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, modDLLName), []byte("old mod"), 0644); err != nil {
		t.Fatal(err)
	}
	resultPath := filepath.Join(t.TempDir(), "alpharing-elevation-result.txt")
	if code := runElevatedMod([]string{"uninstall", gamePath, "", "", resultPath}); code != 0 {
		message, _ := os.ReadFile(resultPath)
		t.Fatalf("helper exited %d: %s", code, message)
	}
	if _, err := os.Stat(filepath.Join(modDir, modDLLName)); !os.IsNotExist(err) {
		t.Fatalf("DLL remains in game binaries: %v", err)
	}
	backups, err := filepath.Glob(filepath.Join(gamePath, "alpha_ring", "launcher-backups", "WTSAPI32.*.dll"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("expected one saved DLL, found %v: %v", backups, err)
	}
}

func TestElevatedModEntryInstallsVerifiedDLL(t *testing.T) {
	gamePath := t.TempDir()
	modDir := filepath.Join(gamePath, modDLLSubpath)
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, modDLLName), []byte("previous DLL"), 0644); err != nil {
		t.Fatal(err)
	}
	data := []byte("verified DLL")
	payloadPath := filepath.Join(t.TempDir(), "payload.dll")
	if err := os.WriteFile(payloadPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	expected := hex.EncodeToString(hash[:])
	resultPath := filepath.Join(t.TempDir(), "result.txt")
	if code := runElevatedMod([]string{"install", gamePath, payloadPath, expected, resultPath}); code != 0 {
		message, _ := os.ReadFile(resultPath)
		t.Fatalf("install failed (%d): %s", code, message)
	}
	installed, err := os.ReadFile(filepath.Join(modDir, modDLLName))
	if err != nil || string(installed) != string(data) {
		t.Fatalf("unexpected installed DLL: %q, %v", installed, err)
	}
	backups, err := filepath.Glob(filepath.Join(gamePath, "alpha_ring", "launcher-backups", "WTSAPI32.*.dll"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("previous DLL was not saved: %v, %v", backups, err)
	}
	if err := os.WriteFile(payloadPath, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := runElevatedMod([]string{"install", gamePath, payloadPath, expected, resultPath}); code != 1 {
		t.Fatalf("tampered DLL returned status %d", code)
	}
	installed, _ = os.ReadFile(filepath.Join(modDir, modDLLName))
	if string(installed) != string(data) {
		t.Fatal("tampered install changed the active DLL")
	}
}

func TestElevatedModEntryReportsMissingGameFolder(t *testing.T) {
	resultPath := filepath.Join(t.TempDir(), "alpharing-elevation-result.txt")
	if code := runElevatedMod([]string{"uninstall", t.TempDir(), "", "", resultPath}); code != 1 {
		t.Fatalf("helper exited %d, want 1", code)
	}
	message, err := os.ReadFile(resultPath)
	if err != nil || len(message) == 0 {
		t.Fatalf("helper did not report its error: %v", err)
	}
}

func TestInstalledAtPathVerifiesRecordedSource(t *testing.T) {
	gamePath := t.TempDir()
	modDir := filepath.Join(gamePath, modDLLSubpath)
	if err := os.MkdirAll(modDir, 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte("selected release")
	path := filepath.Join(modDir, modDLLName)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	cfg := config{"installedSource": sourceKirkland, "installedTag": "v2.0.2-experimental", "installedHash": hex.EncodeToString(hash[:])}
	source, version, err := installedAtPath(gamePath, cfg)
	if err != nil || source != sourceKirkland || version != cfg["installedTag"] {
		t.Fatalf("installed source = %q %q, %v", source, version, err)
	}
	if err := os.WriteFile(path, []byte("changed outside launcher"), 0644); err != nil {
		t.Fatal(err)
	}
	source, _, err = installedAtPath(gamePath, cfg)
	if err != nil || source != "unknown" {
		t.Fatalf("changed DLL was reported as %q: %v", source, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	source, _, err = installedAtPath(gamePath, cfg)
	if err != nil || source != sourceVanilla {
		t.Fatalf("missing DLL was reported as %q: %v", source, err)
	}
}
