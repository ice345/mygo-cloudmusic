// Command archpkg packs a Linux build from mygo into an Arch Linux package.
// The pinned MyGo writes a .deb and leaves Arch out, and a later MyGo only
// writes a PKGBUILD, which -upload does not publish. This writes the same
// files the .deb installs, as a .pkg.tar.zst a pacman -U can install.
package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// archOf maps a Go architecture, as the build directory is named, to Arch's.
var archOf = map[string]string{
	"amd64": "x86_64", "arm64": "aarch64", "386": "i686", "arm": "armv7h", "riscv64": "riscv64",
}

type appInfo struct {
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Version string `json:"version"`
	Linux   struct {
		Comment string `json:"comment"`
		WebView *bool  `json:"webview"`
	} `json:"linux"`
	Updates struct {
		GitHub string `json:"github"`
	} `json:"updates"`
}

func (a appInfo) webView() bool { return a.Linux.WebView == nil || *a.Linux.WebView }

func (a appInfo) desc() string {
	if a.Linux.Comment != "" {
		return a.Linux.Comment
	}
	return a.Name
}

// depends are the Arch packages the app needs, the ones the .deb names.
func (a appInfo) depends() []string {
	deps := []string{"gtk3"}
	if a.webView() {
		deps = append(deps, "webkit2gtk-4.1")
	}
	return deps
}

// packTar builds the package archive for the files in stage, a mygo output
// directory such as build/linux-amd64. built is when the package says it
// was built.
func packTar(stage string, app appInfo, built time.Time) ([]byte, error) {
	goarch := goarchOf(stage)
	arch, ok := archOf[goarch]
	if !ok {
		return nil, fmt.Errorf("no Arch Linux architecture for %s", goarch)
	}
	entries, err := payload(stage, app.Slug)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := func(name string, mode int64, size int64, typ byte) *tar.Header {
		return &tar.Header{
			Name: name, Mode: mode, Size: size, Typeflag: typ,
			ModTime: built, Uid: 0, Gid: 0, Uname: "root", Gname: "root",
			Format: tar.FormatPAX,
		}
	}
	dirs := map[string]bool{}
	// Declared first so a directory can ask for its parent before itself.
	var dir func(string) error
	dir = func(name string) error {
		name = strings.Trim(name, "/")
		if name == "" || dirs[name] {
			return nil
		}
		if parent := path.Dir(name); parent != "." && parent != name {
			if err := dir(parent); err != nil {
				return err
			}
		}
		dirs[name] = true
		return tw.WriteHeader(hdr(name+"/", 0o755, 0, tar.TypeDir))
	}
	var size int64
	var files []struct {
		name string
		data []byte
		mode int64
		link string
	}
	add := func(name string, data []byte, mode int64, link string) {
		files = append(files, struct {
			name string
			data []byte
			mode int64
			link string
		}{name, data, mode, link})
		size += int64(len(data))
	}

	opt := "opt/" + app.Slug
	for _, e := range entries {
		rel := filepath.ToSlash(e.rel)
		target := opt + "/" + rel
		switch {
		case e.dir:
			continue
		case e.link != "":
			add(target, nil, 0o777, e.link)
		default:
			mode := int64(0o644)
			if e.mode&0o111 != 0 {
				mode = 0o755
			}
			add(target, e.data, mode, "")
		}
	}
	if png, err := os.ReadFile(filepath.Join(stage, app.Slug+".png")); err == nil {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(png))
		if err != nil {
			return nil, fmt.Errorf("icon: %w", err)
		}
		icon := fmt.Sprintf("usr/share/icons/hicolor/%dx%d/apps/%s.png", cfg.Width, cfg.Height, app.Slug)
		add(icon, png, 0o644, "")
	}
	if desktop, err := os.ReadFile(filepath.Join(stage, app.Slug+".desktop")); err == nil {
		text := systemDesktop(string(desktop), "/"+opt+"/"+app.Slug)
		add("usr/share/applications/"+app.Slug+".desktop", []byte(text), 0o644, "")
	}
	info := pkginfo(app, arch, size, built)
	if err := tw.WriteHeader(hdr(".PKGINFO", 0o644, int64(len(info)), tar.TypeReg)); err != nil {
		return nil, err
	}
	if _, err := tw.Write(info); err != nil {
		return nil, err
	}
	for _, f := range files {
		if err := dir(path.Dir(f.name)); err != nil {
			return nil, err
		}
		h := hdr(f.name, f.mode, int64(len(f.data)), tar.TypeReg)
		if f.link != "" {
			h.Typeflag = tar.TypeSymlink
			h.Linkname = f.link
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			return nil, err
		}
		if f.link == "" {
			if _, err := tw.Write(f.data); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pkgName is the file name pacman expects: name-version-rel-arch.pkg.tar.zst.
func pkgName(app appInfo, stage string) (string, error) {
	arch, ok := archOf[goarchOf(stage)]
	if !ok {
		return "", fmt.Errorf("no Arch Linux architecture for %s", goarchOf(stage))
	}
	return fmt.Sprintf("%s-%s-1-%s.pkg.tar.zst", app.Slug, app.Version, arch), nil
}

func goarchOf(stage string) string {
	base := filepath.Base(stage)
	if i := strings.LastIndex(base, "-"); i >= 0 {
		return base[i+1:]
	}
	return base
}

type entry struct {
	rel  string
	dir  bool
	mode fs.FileMode
	link string
	data []byte
}

// payload is what the .deb puts under /opt: the app, not the installers
// mygo leaves beside it, and not the desktop entry and icon, which go
// where desktops look for them.
func payload(stage, slug string) ([]entry, error) {
	var out []entry
	err := filepath.WalkDir(stage, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(stage, p)
		if err != nil || rel == "." {
			return err
		}
		if d.IsDir() {
			out = append(out, entry{rel: rel, dir: true})
			return nil
		}
		// Only the installers and the desktop files mygo leaves next to the
		// binary. A file of the same name inside the app stays.
		if !strings.ContainsRune(rel, os.PathSeparator) && skip(rel, slug) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		e := entry{rel: rel, mode: info.Mode()}
		if info.Mode()&fs.ModeSymlink != 0 {
			e.link, err = os.Readlink(p)
			if err != nil {
				return err
			}
		} else {
			e.data, err = os.ReadFile(p)
			if err != nil {
				return err
			}
		}
		out = append(out, e)
		return nil
	})
	return out, err
}

func skip(base, slug string) bool {
	switch base {
	case "install.sh", "SHA256SUMS", slug + ".desktop", slug + ".png", slug + ".xml":
		return true
	}
	switch {
	case strings.HasSuffix(base, ".deb"),
		strings.HasSuffix(base, ".delta"),
		strings.HasSuffix(base, ".pkg.tar.zst"),
		strings.HasSuffix(base, ".tar.gz"),
		strings.HasPrefix(base, "update-") && strings.HasSuffix(base, ".json"):
		return true
	}
	return false
}

// systemDesktop points Exec at the installed binary. The file mygo writes
// next to the build names the binary alone, which is enough for install.sh
// and not for a package that puts the binary in /opt.
func systemDesktop(text, execPath string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		if rest, ok := strings.CutPrefix(line, "Exec="); ok {
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				line = "Exec=" + execPath
			} else {
				fields[0] = execPath
				line = "Exec=" + strings.Join(fields, " ")
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func pkginfo(app appInfo, arch string, size int64, built time.Time) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "pkgname = %s\n", app.Slug)
	fmt.Fprintf(&b, "pkgbase = %s\n", app.Slug)
	fmt.Fprintf(&b, "pkgver = %s-1\n", app.Version)
	fmt.Fprintf(&b, "pkgdesc = %s\n", app.desc())
	if app.Updates.GitHub != "" {
		fmt.Fprintf(&b, "url = https://github.com/%s\n", app.Updates.GitHub)
	}
	fmt.Fprintf(&b, "builddate = %d\n", built.Unix())
	fmt.Fprintf(&b, "packager = %s\n", app.Name)
	fmt.Fprintf(&b, "size = %d\n", size)
	fmt.Fprintf(&b, "arch = %s\n", arch)
	b.WriteString("license = AGPL-3.0-only\n")
	for _, dep := range app.depends() {
		fmt.Fprintf(&b, "depend = %s\n", dep)
	}
	return []byte(b.String())
}

// readTar returns the names in a tar, and the contents of the regular files.
func readTar(b []byte) (map[string][]byte, error) {
	tr := tar.NewReader(bytes.NewReader(b))
	out := map[string][]byte{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			out[h.Name] = data
			continue
		}
		out[h.Name] = nil
	}
}
