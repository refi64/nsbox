/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

#include "nsbox-gtkicons.h"

GtkIconTheme *nsbox_gtk_icon_theme_new(void *actual_func) {
  typedef GtkIconTheme *(*gtk_icon_theme_new_type)(void);
  return ((gtk_icon_theme_new_type)actual_func)();
}

void nsbox_g_object_unref(void *actual_func, void *object) {
  typedef void (*g_object_unref_type)(void *);
  ((g_object_unref_type)actual_func)(object);
}

void nsbox_gtk_icon_theme_set_search_path(void *actual_func, GtkIconTheme *icon_theme,
                                          const gchar *path) {
  typedef void (*gtk_icon_theme_set_search_path_type)(GtkIconTheme *, const gchar **,
                                                      gint);
  ((gtk_icon_theme_set_search_path_type)actual_func)(icon_theme, &path, 1);
}

gint *nsbox_gtk_icon_theme_get_icon_sizes(void *actual_func, GtkIconTheme *icon_theme,
                                          const gchar *icon_name) {
  typedef gint *(*gtk_icon_theme_get_icon_sizes_type)(GtkIconTheme *, const gchar *);
  return ((gtk_icon_theme_get_icon_sizes_type)actual_func)(icon_theme, icon_name);
}

GtkIconInfo *nsbox_gtk_icon_theme_lookup_icon(void *actual_func, GtkIconTheme *icon_theme,
                                              const gchar *icon_name, gint size,
                                              int flags) {
  typedef GtkIconInfo *(*gtk_icon_theme_lookup_icon_type)(GtkIconTheme *, const gchar *,
                                                          gint, int);
  return ((gtk_icon_theme_lookup_icon_type)actual_func)(icon_theme, icon_name, size,
                                                        flags);
}

const gchar *nsbox_gtk_icon_info_get_filename(void *actual_func, GtkIconInfo *icon_info) {
  typedef const gchar *(*gtk_icon_info_get_filename_type)(GtkIconInfo *);
  return ((gtk_icon_info_get_filename_type)actual_func)(icon_info);
}
