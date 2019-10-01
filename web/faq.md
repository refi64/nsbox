# FAQ

## I'm getting errors that say `failed to lock container directory`.

*First off: is this actually a question?*

Your container is currently running, so the config / delete commands cannot lock the directory.
Try killing it first.

## Can I run nsbox on non-systemd distros?

No. nsbox uses several systemd features (transient units, systemd-nspawn, systemd-machined),
and I'm not personally interested in trying to port it to another init system (AFAIK many
others don't even have equivalents for these tools).

## Why is nsbox not using my host login shell?

- Ensure the login shell is installed inside the container.
- If it still is not being detected, [kill](#killing-containers) the container and run it again.

## Why this FAQ so empty?

Why do you care? -_-
