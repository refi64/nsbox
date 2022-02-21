#include "nsbox-ptyfwd.h"

#define _GNU_SOURCE

#include <errno.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>

#define FMT(...)                \
  ({                            \
    char *_p = NULL;            \
    asprintf(&_p, __VA_ARGS__); \
    _p;                         \
  })

char *nsbox_forward_pty(int dstfd, int srcfd) {
  for (;;) {
    char buffer[4 * 1024 * 1024] = {0};
    ssize_t bytes_read = read(srcfd, &buffer, sizeof(buffer));

    if (bytes_read == -1) {
      // EIO gets returned from transient TTY issues, so ignore it.
      if (errno == EINTR || errno == EIO) {
        continue;
      } else {
        return FMT("Failed to read: %s", strerror(errno));
      }
    } else if (bytes_read == 0) {
      return NULL;
    }

    size_t offs = 0;
    while (offs < bytes_read) {
      ssize_t bytes_written = write(dstfd, &buffer + offs, bytes_read - offs);
      if (bytes_written == -1) {
        if (errno == EINTR) {
          continue;
        } else {
          return FMT("Failed to write: %s", strerror(errno));
        }
      }

      offs += bytes_written;
    }
  }
}
