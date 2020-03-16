/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

// This ugly thing tries to dynamically load GTK+ and have it find icons for exports.

package gtkicons

import (
	"github.com/coreos/pkg/dlopen"
	"github.com/pkg/errors"
	"github.com/refi64/nsbox/internal/log"
	"unsafe"
)

// #include "nsbox-gtkicons.h"
// #include <stdlib.h>
import "C"

var (
	gtkHandle *dlopen.LibHandle

	newSym           unsafe.Pointer
	unrefSym         unsafe.Pointer
	setSearchPathSym unsafe.Pointer
	getIconSizesSym  unsafe.Pointer
	lookupIconSym    unsafe.Pointer
	getFilenameSym   unsafe.Pointer
)

type LookupContext struct {
	iconTheme *C.GtkIconTheme
	Path      string
}

type Icon struct {
	Root string
	Path string
	Size int
}

func loadGtk() error {
	gtk, err := dlopen.GetHandle([]string{"libgtk-3.so", "libgtk-3.so.0"})
	if err != nil {
		return errors.Wrap(err, "failed to open gtk3")
	}

	// We never bother to close the handle, because this will all be freed when the program dies.

	symbols := map[string]*unsafe.Pointer{
		"gtk_icon_theme_new":             &newSym,
		"g_object_unref":                 &unrefSym,
		"gtk_icon_theme_set_search_path": &setSearchPathSym,
		"gtk_icon_theme_get_icon_sizes":  &getIconSizesSym,
		"gtk_icon_theme_lookup_icon":     &lookupIconSym,
		"gtk_icon_info_get_filename":     &getFilenameSym,
	}

	for name, target := range symbols {
		*target, err = gtk.GetSymbolPointer(name)
		if err != nil {
			return err
		}
	}

	gtkHandle = gtk
	return nil
}

func CreateContext(path string) (*LookupContext, error) {
	if gtkHandle == nil {
		if err := loadGtk(); err != nil {
			return nil, errors.Wrap(err, "failed to load gtk3")
		}
	}

	var ctx LookupContext
	ctx.Path = path

	ctx.iconTheme = C.nsbox_gtk_icon_theme_new(newSym)

	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	C.nsbox_gtk_icon_theme_set_search_path(setSearchPathSym, ctx.iconTheme, cpath)

	return &ctx, nil
}

func (ctx *LookupContext) Destroy() {
	C.nsbox_g_object_unref(unrefSym, unsafe.Pointer(ctx.iconTheme))
}

func (ctx *LookupContext) lookupIconBySize(icon string, cicon *C.char, size int) (info Icon, err error) {
	iconInfo := C.nsbox_gtk_icon_theme_lookup_icon(lookupIconSym, ctx.iconTheme, cicon, C.int(size), 0)
	if iconInfo == nil {
		err = errors.Errorf("Could not find %s@%d", icon, size)
		return
	}

	defer C.nsbox_g_object_unref(unrefSym, unsafe.Pointer(iconInfo))

	cpath := C.nsbox_gtk_icon_info_get_filename(getFilenameSym, iconInfo)
	path := C.GoString(cpath)
	info = Icon{ctx.Path, path, size}
	return
}

func (ctx *LookupContext) FindIcon(icon string) []Icon {
	var result []Icon

	cicon := C.CString(icon)
	defer C.free(unsafe.Pointer(cicon))

	iconSizes := C.nsbox_gtk_icon_theme_get_icon_sizes(getIconSizesSym, ctx.iconTheme, cicon)
	defer C.free(unsafe.Pointer(iconSizes))

	if *iconSizes == 0 {
		// If the icon is in pixmaps, it will not be found by get_icon_sizes.
		// Therefore, try to look it up manually.
		info, err := ctx.lookupIconBySize(icon, cicon, 0)
		if err == nil {
			result = append(result, info)
		}
	}

	for *iconSizes != 0 {
		info, err := ctx.lookupIconBySize(icon, cicon, int(*iconSizes))
		if err != nil {
			log.Alert(err)
		} else {
			result = append(result, info)
		}

		iconSizes = (*C.int)(unsafe.Pointer(uintptr(unsafe.Pointer(iconSizes)) + unsafe.Sizeof(*iconSizes)))
	}

	return result
}
