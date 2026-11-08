package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type installState struct {
	Selected  string `json:"selected"`
	Installed string `json:"installed"`
	Version   string `json:"version"`
	GameFound bool   `json:"gameFound"`
}

func installedAtPath(gamePath string, cfg config) (string, string, error) {
	path := filepath.Join(gamePath, modDLLSubpath, modDLLName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return sourceVanilla, "", nil
	} else if err != nil {
		return "", "", err
	}
	hash, err := fileHash(path)
	if err != nil {
		return "", "", err
	}
	if cfg["installedHash"] != "" &&
		(cfg["installedSource"] == sourceMegaBit || cfg["installedSource"] == sourceKirkland) &&
		cfg["installedHash"] == hex.EncodeToString(hash) {
		return cfg["installedSource"], cfg["installedTag"], nil
	}
	return "unknown", "", nil
}

func (a *App) GetInstallState() (installState, error) {
	cfg, err := readConfig()
	if err != nil {
		return installState{}, err
	}
	selected := cfg["selectedSource"]
	if !validSource(selected) {
		selected = sourceVanilla
	}
	gamePath, found := locateGamePath(cfg["gamePath"])
	if !found {
		return installState{Selected: selected, Installed: "unavailable"}, nil
	}
	installed, version, err := installedAtPath(gamePath, cfg)
	if err != nil {
		return installState{}, err
	}
	return installState{Selected: selected, Installed: installed, Version: version, GameFound: true}, nil
}

// RecognizeExisting identifies a DLL installed before this launcher recorded its source.
// It reads release metadata only; the installed file is never changed.
func (a *App) RecognizeExisting() (installState, error) {
	cfg, err := readConfig()
	if err != nil {
		return installState{}, err
	}
	gamePath, found := locateGamePath(cfg["gamePath"])
	if !found {
		return a.GetInstallState()
	}
	installed, _, err := installedAtPath(gamePath, cfg)
	if err != nil || installed != "unknown" {
		return a.GetInstallState()
	}
	hash, err := fileHash(filepath.Join(gamePath, modDLLSubpath, modDLLName))
	if err != nil {
		return installState{}, err
	}
	source, tag := matchPublishedDLL(hex.EncodeToString(hash))
	if source != "" {
		// A dropdown change may have saved the selection while GitHub was queried.
		cfg, err = readConfig()
		if err != nil {
			return installState{}, err
		}
		cfg["installedSource"] = source
		cfg["installedTag"] = tag
		cfg["installedHash"] = hex.EncodeToString(hash)
		if err := writeConfig(cfg); err != nil {
			return installState{}, err
		}
	}
	return a.GetInstallState()
}

func (a *App) SelectSource(source string) (installState, error) {
	if !validSource(source) {
		return installState{}, fmt.Errorf("unknown launch selection %q", source)
	}
	if err := setConfigValue("selectedSource", source); err != nil {
		return installState{}, err
	}
	return a.GetInstallState()
}

func backupDLL(gamePath string) (string, error) {
	path := filepath.Join(gamePath, modDLLSubpath, modDLLName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	backupDir := filepath.Join(gamePath, "alpha_ring", "launcher-backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}
	backupPath := filepath.Join(backupDir, fmt.Sprintf("WTSAPI32.%d.dll", time.Now().UnixNano()))
	if err := os.Rename(path, backupPath); err != nil {
		return "", err
	}
	return backupPath, nil
}

func removeDLL(gamePath string) error {
	_, err := backupDLL(gamePath)
	return err
}

func replaceDLL(gamePath string, data []byte, expectedHash string) error {
	actual := sha256.Sum256(data)
	if !equalHash(expectedHash, hex.EncodeToString(actual[:])) {
		return fmt.Errorf("staged mod DLL failed checksum verification")
	}
	modDir := filepath.Join(gamePath, modDLLSubpath)
	info, err := os.Stat(modDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("MCC binaries folder was not found")
	}
	temp, err := os.CreateTemp(modDir, ".alpharing-*.dll")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	backupPath, err := backupDLL(gamePath)
	if err != nil {
		return err
	}
	destination := filepath.Join(modDir, modDLLName)
	if err := os.Rename(tempPath, destination); err != nil {
		if backupPath != "" {
			if rollbackErr := os.Rename(backupPath, destination); rollbackErr != nil {
				return fmt.Errorf("install failed: %w; restoring previous DLL also failed: %v", err, rollbackErr)
			}
		}
		return err
	}
	return nil
}

func equalHash(left, right string) bool {
	return len(left) == sha256.Size*2 && len(right) == sha256.Size*2 &&
		strings.EqualFold(left, right)
}

func stageDLL(data []byte) (string, error) {
	temp, err := os.CreateTemp("", "alpharing-dll-*.tmp")
	if err != nil {
		return "", err
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
		return "", err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(temp.Name())
		return "", err
	}
	return temp.Name(), nil
}

func (a *App) Install(source string) (installState, error) {
	if source != sourceMegaBit && source != sourceKirkland {
		return installState{}, fmt.Errorf("select MegaBit or Kirkland to install")
	}
	gamePath, err := a.findGamePath()
	if err != nil {
		return installState{}, err
	}
	a.log(fmt.Sprintf("Downloading %s AlphaRing...", sourceLabel(source)))
	data, tag, expectedHash, err := fetchVerifiedDLL(source)
	if err != nil {
		return installState{}, err
	}
	a.log("Verified mod checksum")
	if err := replaceDLL(gamePath, data, expectedHash); err != nil {
		if runtime.GOOS != "windows" || !os.IsPermission(err) {
			return installState{}, err
		}
		stagedPath, stageErr := stageDLL(data)
		if stageErr != nil {
			return installState{}, stageErr
		}
		defer os.Remove(stagedPath)
		a.log("MCC file changes require administrator access. Requesting UAC permission...")
		if elevatedErr := elevatePrepareMod(gamePath, "install", stagedPath, expectedHash); elevatedErr != nil {
			return installState{}, elevatedErr
		}
	}
	actualHash, err := fileHash(filepath.Join(gamePath, modDLLSubpath, modDLLName))
	if err != nil || !equalHash(expectedHash, hex.EncodeToString(actualHash)) {
		return installState{}, fmt.Errorf("installed DLL could not be verified: %v", err)
	}
	cfg, err := readConfig()
	if err != nil {
		return installState{}, err
	}
	cfg["selectedSource"] = source
	cfg["installedSource"] = source
	cfg["installedTag"] = tag
	cfg["installedHash"] = expectedHash
	if err := writeConfig(cfg); err != nil {
		return installState{}, err
	}
	a.log(fmt.Sprintf("Installed %s %s", sourceLabel(source), tag))
	return a.GetInstallState()
}

func (a *App) Uninstall() (installState, error) {
	gamePath, err := a.findGamePath()
	if err != nil {
		return installState{}, err
	}
	if err := removeDLL(gamePath); err != nil {
		if runtime.GOOS != "windows" || !os.IsPermission(err) {
			return installState{}, err
		}
		a.log("MCC file changes require administrator access. Requesting UAC permission...")
		if elevatedErr := elevatePrepareMod(gamePath, "uninstall", "", ""); elevatedErr != nil {
			return installState{}, elevatedErr
		}
	}
	if _, err := os.Stat(filepath.Join(gamePath, modDLLSubpath, modDLLName)); !os.IsNotExist(err) {
		return installState{}, fmt.Errorf("mod DLL is still installed: %v", err)
	}
	cfg, err := readConfig()
	if err != nil {
		return installState{}, err
	}
	cfg["installedSource"] = ""
	cfg["installedTag"] = ""
	cfg["installedHash"] = ""
	if err := writeConfig(cfg); err != nil {
		return installState{}, err
	}
	a.log("Mod DLL removed; backup and settings preserved")
	return a.GetInstallState()
}

func (a *App) Play(source string) error {
	if !validSource(source) {
		return fmt.Errorf("unknown launch selection %q", source)
	}
	gamePath, err := a.findGamePath()
	if err != nil {
		return err
	}
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	installed, _, err := installedAtPath(gamePath, cfg)
	if err != nil {
		return err
	}
	if installed != source {
		return fmt.Errorf("%s is selected, but %s is installed; install or uninstall first", sourceLabel(source), installed)
	}
	if source != sourceVanilla {
		if err := a.ensureWorkshopItems(gamePath); err != nil {
			return err
		}
	}
	return a.launch(gamePath, source == sourceVanilla)
}
