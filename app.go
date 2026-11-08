package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx             context.Context
	videoServerBase string
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if base, err := startVideoServer(); err == nil {
		a.videoServerBase = base
	} else {
		a.log(fmt.Sprintf("Error starting video server: %s", err))
	}
	for _, argument := range os.Args[1:] {
		if argument == "-fullscreen" {
			wailsruntime.WindowFullscreen(ctx)
			break
		}
	}
}

func (a *App) BackgroundVideoURLs() []string {
	if a.videoServerBase == "" {
		return nil
	}
	return []string{a.videoServerBase + "/bkgVideo.webm", a.videoServerBase + "/bkgVideo.mp4"}
}

const (
	gameSubpath    = "steamapps/common/Halo The Master Chief Collection"
	modDLLSubpath  = "MCC/Binaries/Win64"
	modDLLName     = "WTSAPI32.dll"
	configFileName = "launcher.cfg"
	workshopAppID  = 976730
)

var workshopItems = []uint64{3686670451, 3730810482}

func (a *App) log(message string) {
	fmt.Println(message)
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "log", message)
	}
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", homeErr
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "alpharing"), nil
}

func configPath() string {
	dir, err := configDir()
	if err != nil {
		return configFileName
	}
	return filepath.Join(dir, configFileName)
}

func defaultGamePath() string {
	if runtime.GOOS == "windows" {
		return `C:\Program Files (x86)\Steam\steamapps\common\Halo The Master Chief Collection`
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local/share/Steam", gameSubpath)
}

// config holds the settings persisted in launcher.cfg, keyed by name.
// "gamePath" is the MCC installation folder and "steamPath" is the Steam
// executable; both are auto-detected when possible and otherwise filled in
// by prompting the user with a native file/folder browser (see
// promptDirectory/promptFile).
type config map[string]string

var configKeys = []string{"gamePath", "steamPath", "selectedSource", "installedSource", "installedTag", "installedHash"}

func readConfig() (config, error) {
	path := configPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		gamePath := defaultGamePath()
		steamPath, _ := locateSteamExecutable(gamePath)
		if err := writeConfig(config{"gamePath": gamePath, "steamPath": steamPath}); err != nil {
			return nil, err
		}
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := config{}
	for _, line := range strings.Split(string(contents), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		if key != "" {
			cfg[key] = value
		}
	}
	return cfg, nil
}

func writeConfig(cfg config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	var builder strings.Builder
	for _, key := range configKeys {
		fmt.Fprintf(&builder, "%s = %q\n", key, cfg[key])
	}
	return os.WriteFile(path, []byte(builder.String()), 0644)
}

func setConfigValue(key, value string) error {
	cfg, err := readConfig()
	if err != nil {
		cfg = config{}
	}
	cfg[key] = value
	return writeConfig(cfg)
}

// nearestExistingDir walks up from path until it finds a directory that
// exists, for use as a dialog's starting directory (Wails errors if given a
// starting directory that doesn't exist). Returns "" if none is found.
func nearestExistingDir(path string) string {
	for path != "" {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return ""
}

func (a *App) promptDirectory(title, hint string) (string, error) {
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: nearestExistingDir(hint),
	})
}

func (a *App) promptFile(title, hint string, filters []wailsruntime.FileFilter) (string, error) {
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: nearestExistingDir(hint),
		Filters:          filters,
	})
}

func steamRoots(gamePath string) []string {
	root := gamePath
	for range strings.Split(gameSubpath, "/") {
		root = filepath.Dir(root)
	}
	return []string{root}
}

func libraryFolders(path string) []string {
	contents, err := os.ReadFile(filepath.Join(path, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return nil
	}
	var libraries []string
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, `"path"`) {
			parts := strings.Split(trimmed, `"`)
			if len(parts) > 3 {
				libraries = append(libraries, strings.ReplaceAll(parts[3], `\\`, `\`))
			}
		}
	}
	return libraries
}

func locateGamePath(configured string) (string, bool) {
	if configured == "" {
		return "", false
	}
	configured = filepath.Clean(configured)
	for _, root := range steamRoots(configured) {
		libraries := append([]string{root}, libraryFolders(root)...)
		for _, library := range libraries {
			candidate := filepath.Join(library, gameSubpath)
			if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
				return candidate, true
			}
		}
	}
	if info, statErr := os.Stat(configured); statErr == nil && info.IsDir() {
		return configured, true
	}
	return "", false
}

func (a *App) findGamePath() (string, error) {
	cfg, err := readConfig()
	if err != nil {
		return "", err
	}
	if path, ok := locateGamePath(cfg["gamePath"]); ok {
		return path, nil
	}
	a.log("Could not locate the MCC installation, please select the game folder...")
	selected, err := a.promptDirectory("Select the Halo: The Master Chief Collection folder", cfg["gamePath"])
	if err != nil {
		return "", fmt.Errorf("could not open folder browser: %w", err)
	}
	if selected == "" {
		return "", fmt.Errorf("game location was not specified")
	}
	selected = filepath.Clean(selected)
	if err := setConfigValue("gamePath", selected); err != nil {
		return "", err
	}
	return selected, nil
}

func locateSteamExecutable(gamePath string) (string, bool) {
	if runtime.GOOS != "windows" {
		if path, err := exec.LookPath("steam"); err == nil {
			return path, true
		}
		return "", false
	}
	for _, root := range steamRoots(gamePath) {
		candidate := filepath.Join(root, "steam.exe")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func (a *App) steamExecutable(gamePath string) (string, error) {
	cfg, err := readConfig()
	if err != nil {
		return "", err
	}
	if configured := cfg["steamPath"]; configured != "" {
		if info, statErr := os.Stat(configured); statErr == nil && !info.IsDir() {
			return configured, nil
		}
	}
	a.log("Could not locate Steam, please select the Steam executable...")
	var filters []wailsruntime.FileFilter
	hint := ""
	if runtime.GOOS == "windows" {
		filters = []wailsruntime.FileFilter{{DisplayName: "Steam executable (steam.exe)", Pattern: "steam.exe"}}
		if roots := steamRoots(gamePath); len(roots) > 0 {
			hint = roots[0]
		}
	}
	selected, err := a.promptFile("Select the Steam executable", hint, filters)
	if err != nil {
		return "", fmt.Errorf("could not open file browser: %w", err)
	}
	if selected == "" {
		return "", fmt.Errorf("steam executable was not specified")
	}
	selected = filepath.Clean(selected)
	if err := setConfigValue("steamPath", selected); err != nil {
		return "", err
	}
	return selected, nil
}

func workshopPath(gamePath string, item uint64) string {
	steamapps := filepath.Dir(filepath.Dir(gamePath))
	return filepath.Join(steamapps, "workshop", "content", strconv.Itoa(workshopAppID), strconv.FormatUint(item, 10))
}

func (a *App) ensureWorkshopItems(gamePath string) error {
	var missing []uint64
	for _, item := range workshopItems {
		if _, err := os.Stat(workshopPath(gamePath, item)); err != nil {
			missing = append(missing, item)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	sw, err := newSteamworksClient()
	if err != nil {
		return fmt.Errorf("could not connect to Steam to fetch Workshop items: %w", err)
	}
	defer sw.shutdown()

	return sw.subscribeAndDownload(a, gamePath, missing)
}

func fileHash(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

func (a *App) launch(gamePath string, vanilla bool) error {
	a.log("Launching MCC...")
	steam, err := a.steamExecutable(gamePath)
	if err != nil {
		return err
	}
	command := exec.Command(steam)
	if !vanilla {
		command.Args = append(command.Args, "steam://launch/976730/option2")
	} else {
		command.Args = append(command.Args, "steam://launch/976730/option1")
	}
	if err := command.Start(); err != nil {
		return err
	}
	a.log("Game launched, closing launcher...")
	wailsruntime.Quit(a.ctx)
	return nil
}

func (a *App) OpenInstallDir() error {
	directory, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return exec.Command("explorer.exe", directory).Start()
	}
	return exec.Command("xdg-open", directory).Start()
}
