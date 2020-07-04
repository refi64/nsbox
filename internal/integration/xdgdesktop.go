/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package integration

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/container"
	"github.com/refi64/nsbox/internal/gtkicons"
	"github.com/refi64/nsbox/internal/log"
	"github.com/refi64/nsbox/internal/paths"
)

var (
	xdgDataDirs = []string{"/usr/share", "/usr/local/share"}
)

func readExportsId(path string) (int, error) {
	target, err := os.Readlink(path)
	if err != nil {
		if os.IsNotExist(err) {
			return -1, nil
		} else {
			return 0, err
		}
	}

	if strings.HasSuffix(target, ".0") {
		return 0, nil
	} else if strings.HasSuffix(target, ".1") {
		return 1, nil
	} else {
		return 0, errors.Errorf("%s does not have a valid export ID", path)
	}
}

type exportContext struct {
	ct       *container.Container
	iconCtxs []*gtkicons.LookupContext
	icons    []gtkicons.Icon

	targetRoot            string
	targetApplicationsDir string
	targetIconsDir        string
}

func (ctx *exportContext) exportDesktopFile(desktopFilePath string) error {
	log.Debug("Exporting desktop file", desktopFilePath)

	source, err := os.Open(desktopFilePath)
	if err != nil {
		return errors.Wrapf(err, "failed to open desktop file %s", desktopFilePath)
	}
	defer source.Close()

	target, err := os.Create(filepath.Join(ctx.targetApplicationsDir, filepath.Base(desktopFilePath)))
	if err != nil {
		return errors.Wrapf(err, "failed to create exported desktop file of %s", desktopFilePath)
	}
	defer target.Close()

	scanner := bufio.NewScanner(source)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "#") && line != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				log.Alertf("%s had invalid line: %s", desktopFilePath, line)
			} else {
				if parts[0] == "Exec" {
					// Exec= should not replay, otherwise the user may be greeted with
					// random delays on starting GUI apps.
					line = fmt.Sprintf("Exec=%s run -no-replay -- %s %s", paths.ProductName, ctx.ct.Name, parts[1])
				} else if parts[0] == "Icon" {
					for _, iconCtx := range ctx.iconCtxs {
						icons := iconCtx.FindIcon(parts[1])
						log.Debug("Icon matches:", icons)
						// If it's an icon outside a theme, don't bother to do any fancy
						// magic. Just save the full path into the desktop file.
						if len(icons) == 1 && icons[0].Size == 0 {
							line = fmt.Sprintf("Icon=%s", icons[0].Path)
							break
						}
						ctx.icons = append(ctx.icons, icons...)
					}
				}
			}
		}

		fmt.Fprintln(target, line)
	}

	if err := scanner.Err(); err != nil {
		return errors.Wrapf(err, "failed to read %s", desktopFilePath)
	}

	return nil
}

func (ctx *exportContext) exportDesktopFiles(desktopFilesDir string) error {
	desktopFiles, err := ioutil.ReadDir(desktopFilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug(desktopFilesDir, "does not exist")
			return nil
		} else {
			return err
		}
	}

	for _, file := range desktopFiles {
		const desktopSuffix = ".desktop"

		if !strings.HasSuffix(file.Name(), desktopSuffix) {
			continue
		}

		name := file.Name()[:len(file.Name())-len(desktopSuffix)]
		for _, pat := range ctx.ct.Config.XdgDesktopExports {
			ok, err := filepath.Match(pat, name)

			if err != nil {
				log.Alertf("%s failed to match: %v", pat, name)
			} else if ok {
				path := filepath.Join(desktopFilesDir, file.Name())
				if err := ctx.exportDesktopFile(path); err != nil {
					log.Alertf("failed to export %s: %v", path, err)
				}

				break
			} else {
				log.Debugf("%s failed to match %s", pat, name)
			}
		}
	}

	return nil
}

func (ctx *exportContext) addIconLoaderContext(iconDir string) (*gtkicons.LookupContext, error) {
	log.Debug("Scanning icon directory:", iconDir)
	iconCtx, err := gtkicons.CreateContext(iconDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create icon context")
	}

	ctx.iconCtxs = append(ctx.iconCtxs, iconCtx)
	return iconCtx, err
}

func (ctx *exportContext) exportIcons() {
	if len(ctx.icons) == 0 {
		log.Debug("no icons to export")
		return
	}

	for _, icon := range ctx.icons {
		log.Debugf("Exporting icon %s (from %s)", icon.Path, icon.Root)

		subdir, err := filepath.Rel(icon.Root, filepath.Dir(icon.Path))
		if err != nil {
			log.Alertf("could not make icon %s relative to root %s: %v", icon.Path, icon.Root, err)
			continue
		}

		themeName := strings.Split(subdir, string(os.PathSeparator))[0]
		targetThemeDir := filepath.Join(ctx.targetIconsDir, themeName)
		if err := os.MkdirAll(targetThemeDir, 0755); err != nil {
			log.Alertf("failed to create %s: %v", targetThemeDir, err)
			continue
		}

		sourceIndex := filepath.Join(icon.Root, themeName, "index.theme")
		targetIndex := filepath.Join(targetThemeDir, "index.theme")
		log.Debugf("subdir=%s themeName=%s sourceIndex=%s targetIndex=%s", subdir, themeName, sourceIndex, targetIndex)

		// If the index.theme already exists, it can only be that *we* already linked it, as
		// UpdateDesktopFiles creates a fresh exports directory every time.
		if err := os.Symlink(sourceIndex, targetIndex); err != nil && !os.IsExist(err) {
			log.Alertf("failed to symlink %s -> %s: %v", sourceIndex, targetIndex, err)
			continue
		}

		targetIconDir := filepath.Join(ctx.targetIconsDir, subdir)
		if err := os.MkdirAll(targetIconDir, 0755); err != nil {
			log.Alertf("failed to mkdir %s: %v", targetIconDir, err)
			continue
		}

		targetIcon := filepath.Join(targetIconDir, filepath.Base(icon.Path))
		if err := os.Symlink(icon.Path, targetIcon); err != nil {
			log.Alertf("failed to symlink %s -> %s: %v", icon.Path, targetIcon, err)
			continue
		}
	}
}

func UpdateDesktopFiles(ct *container.Container) error {
	lock, err := ct.Lock(container.ExportsLock, container.NoWaitForLock)
	if err != nil {
		return errors.Wrap(err, "acquire exports lock")
	}

	defer lock.Release()

	activeExportsLink := ct.ExportsLink(false)

	oldExportsInstanceId, err := readExportsId(activeExportsLink)
	if err != nil {
		return err
	}

	var newExportsInstanceId int
	if oldExportsInstanceId == 0 {
		newExportsInstanceId = 1
	} else {
		newExportsInstanceId = 0
	}

	newExportsInstanceDir := ct.ExportsInstance(newExportsInstanceId)

	if err := os.RemoveAll(newExportsInstanceDir); err != nil && !os.IsNotExist(err) {
		return errors.Wrapf(err, "failed to reset new exports instance dir %d", newExportsInstanceId)
	}

	var ctx exportContext
	ctx.ct = ct
	ctx.targetRoot = filepath.Join(newExportsInstanceDir, "share")

	for _, dir := range xdgDataDirs {
		iconDir := ct.StorageChild(filepath.Join(dir, "icons"))
		iconCtx, err := ctx.addIconLoaderContext(iconDir)
		if err != nil {
			return err
		}

		defer iconCtx.Destroy()
	}

	pixmaps := ct.StorageChild("usr/share/pixmaps")
	if _, err := os.Stat(pixmaps); err == nil {
		iconCtx, err := ctx.addIconLoaderContext(pixmaps)
		if err != nil {
			return err
		}

		defer iconCtx.Destroy()
	}

	ctx.targetApplicationsDir = filepath.Join(ctx.targetRoot, "applications")
	ctx.targetIconsDir = filepath.Join(ctx.targetRoot, "icons")

	targetDirsToCreate := []string{ctx.targetApplicationsDir, ctx.targetIconsDir}
	for _, dir := range targetDirsToCreate {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrapf(err, "failed to create %s", dir)
		}
	}

	var desktopFilesDirs []string
	for _, dir := range xdgDataDirs {
		desktopFilesDirs = append(desktopFilesDirs, filepath.Join(dir, "applications"))
	}
	desktopFilesDirs = append(desktopFilesDirs, ct.Config.XdgDesktopExtra...)

	for _, absDesktopFilesDir := range desktopFilesDirs {
		desktopFilesDir := ct.StorageChild(absDesktopFilesDir)
		log.Debug("Scanning desktop files directory:", desktopFilesDir)
		if err := ctx.exportDesktopFiles(desktopFilesDir); err != nil {
			log.Alertf("failed to export desktop files from %s: %v", desktopFilesDir, err)
		}
	}

	ctx.exportIcons()

	tempExportsLink := ct.ExportsLink(true)
	if err := os.Symlink(newExportsInstanceDir, tempExportsLink); err != nil {
		return errors.Wrapf(err, "failed to symlink new instance dir")
	}

	if err := os.Rename(tempExportsLink, activeExportsLink); err != nil {
		return errors.Wrapf(err, "failed to rename onto new instance dir")
	}

	if oldExportsInstanceId != -1 {
		if err := os.RemoveAll(ct.ExportsInstance(oldExportsInstanceId)); err != nil {
			return errors.Wrapf(err, "failed to delete old instance dir")
		}
	}

	return nil
}
