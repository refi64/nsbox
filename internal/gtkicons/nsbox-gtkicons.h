/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

#pragma once

typedef struct GtkIconTheme GtkIconTheme;
typedef struct GtkIconInfo GtkIconInfo;
typedef char gchar;
typedef int gint;

GtkIconTheme *nsbox_gtk_icon_theme_new(void *actual_func);

void nsbox_g_object_unref(void *actual_func, void *object);

// Unlike the others here, this is not an identical function signature, because we only
// ever pass *one* path from Go land and it would be more difficult to try to pass the
// actual array of strings vs passing one string and having C land use that as an array.
void nsbox_gtk_icon_theme_set_search_path(void *actual_func, GtkIconTheme *icon_theme,
                                          const gchar *path);

gint *nsbox_gtk_icon_theme_get_icon_sizes(void *actual_func, GtkIconTheme *icon_theme,
                                          const gchar *icon_name);

GtkIconInfo *nsbox_gtk_icon_theme_lookup_icon(void *actual_func, GtkIconTheme *icon_theme,
                                              const gchar *icon_name, gint size,
                                              int flags);

const gchar *nsbox_gtk_icon_info_get_filename(void *actual_func, GtkIconInfo *icon_info);
