package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The Arch package installs the binary under /opt, the desktop entry with
// an absolute Exec, and the icon, and leaves the .deb and install.sh out.
func TestPackTar(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, "linux-amd64")
	if err := os.Mkdir(stage, 0o755); err != nil {
		t.Fatal(err)
	}
	slug := "mygo-cloudmusic"
	write := func(name string, data []byte, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(stage, name), data, mode); err != nil {
			t.Fatal(err)
		}
	}
	write(slug, []byte("bin"), 0o755)
	write(slug+".desktop", []byte("[Desktop Entry]\nExec="+slug+" %U\nIcon="+slug+"\n"), 0o644)
	var icon bytes.Buffer
	if err := png.Encode(&icon, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	write(slug+".png", icon.Bytes(), 0o644)
	write(slug+"_0.1.26_amd64.deb", []byte("deb"), 0o644)
	write("install.sh", []byte("sh"), 0o755)
	if err := os.Mkdir(filepath.Join(stage, "extra"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join("extra", "note.txt"), []byte("n"), 0o644)

	app := appInfo{Name: "MyGo 云音乐", Slug: slug, Version: "0.1.26"}
	app.Linux.Comment = "第三方网易云音乐桌面客户端"
	app.Updates.GitHub = "ice345/mygo-cloudmusic"
	off := false
	app.Linux.WebView = &off
	built := time.Unix(1_700_000_000, 0)

	raw, err := packTar(stage, app, built)
	if err != nil {
		t.Fatal(err)
	}
	files, err := readTar(raw)
	if err != nil {
		t.Fatal(err)
	}
	info := string(files[".PKGINFO"])
	for _, want := range []string{
		"pkgname = mygo-cloudmusic\n",
		"pkgver = 0.1.26-1\n",
		"arch = x86_64\n",
		"depend = gtk3\n",
		"url = https://github.com/ice345/mygo-cloudmusic\n",
		"builddate = 1700000000\n",
	} {
		if !strings.Contains(info, want) {
			t.Errorf("PKGINFO missing %q\n%s", want, info)
		}
	}
	if strings.Contains(info, "webkit2gtk") {
		t.Errorf("a native app depends on WebKitGTK:\n%s", info)
	}
	if string(files["opt/mygo-cloudmusic/mygo-cloudmusic"]) != "bin" {
		t.Error("the binary is not installed under /opt")
	}
	if string(files["opt/mygo-cloudmusic/extra/note.txt"]) != "n" {
		t.Error("a file next to the binary was left out")
	}
	desk := string(files["usr/share/applications/mygo-cloudmusic.desktop"])
	if !strings.Contains(desk, "Exec=/opt/mygo-cloudmusic/mygo-cloudmusic %U\n") {
		t.Errorf("desktop entry:\n%s", desk)
	}
	if files["usr/share/icons/hicolor/8x8/apps/mygo-cloudmusic.png"] == nil {
		t.Error("the icon is not installed for the desktop")
	}
	for name := range files {
		base := filepath.Base(name)
		if strings.HasSuffix(base, ".deb") || base == "install.sh" {
			t.Errorf("the package contains %s", name)
		}
	}
	if got, err := pkgName(app, stage); err != nil || got != "mygo-cloudmusic-0.1.26-1-x86_64.pkg.tar.zst" {
		t.Errorf("package name %q, %v", got, err)
	}
}
