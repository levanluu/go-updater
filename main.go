package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type UpdateInfo struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
}

const (
	installDir  = "/opt/myapp"
	versionDir  = "/opt/myapp/versions"
	currentLink = "/opt/myapp/current"
	serviceName = "myapp.service"
)

func getCurrentVersion() string {
	out, err := exec.Command(currentLink+"/myapp", "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func sha256File(path string) (string, error) {
	f, _ := os.Open(path)
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func main() {
	// 1. Fetch metadata
	resp, err := http.Get("https://raw.githubusercontent.com/levanluu/go-updater/main/latest.json")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var info UpdateInfo
	json.NewDecoder(resp.Body).Decode(&info)

	currentVer := getCurrentVersion()
	if currentVer == info.Version {
		fmt.Println("Already latest:", currentVer)
		return
	}

	fmt.Println("Update available:", currentVer, "->", info.Version)

	// 2. Download binary
	os.MkdirAll(versionDir, 0755)
	newFile := filepath.Join(versionDir, "myapp-"+info.Version)

	out, _ := os.Create(newFile)
	binResp, _ := http.Get(info.URL)
	io.Copy(out, binResp.Body)
	out.Close()
	os.Chmod(newFile, 0755)

	// 3. Verify SHA256
	sum, _ := sha256File(newFile)
	if sum != info.SHA256 {
		fmt.Println("SHA mismatch, aborting")
		os.Remove(newFile)
		return
	}

	// 4. Stop app
	exec.Command("systemctl", "stop", serviceName).Run()

	// 5. Atomic swap
	tmp := currentLink + ".tmp"
	os.Remove(tmp)
	os.Symlink(newFile, tmp)
	os.Rename(tmp, currentLink)

	// 6. Restart service
	exec.Command("systemctl", "start", serviceName).Run()

	fmt.Println("Updated to", info.Version)
}
