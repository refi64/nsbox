# FAQ

## Why am I getting errors that say `failed to lock container directory`?

Your container is currently running, so the commands cannot lock the directory. Try killing the
container first.

## Why am I getting errors about the shadow database / entries?

Your system likely uses a remote authentication service like SSSD, in which case nsbox cannot
automatically use your host shadow information inside the container. See
[this section of the guide](guide.md#custom-authentication) for more information.

Note that if nsbox detects this may be the case on your system, it should warn you during
container creation.

## Can I run nsbox on non-systemd distros?

No. nsbox uses several systemd features (transient units, systemd-nspawn, systemd-machined),
and I'm not personally interested in trying to port it to another init system (AFAIK many
others don't even have equivalents for these tools).

## Why is nsbox not using my host login shell?

- Ensure the login shell is installed inside the container.
- If it still is not being detected, [kill](guide.md#killing-containers) the container and run
  it again.

## Why do I get "the playbook: ... could not be found" when I update my images?

If you deleted the image directory and re-created it, you need to
[kill](guide.md#killing-containers) the container and run it again. The reason for this is
that nsbox mounts the image directories into the container, so when you delete the old
directory, the new one will not be mounted.

## How does this compare to [toolbox](https://github.com/containers/toolbox) or [coretoolbox](https://github.com/cgwalters/coretoolbox)?

TL;DR: nsbox intentionally uses systemd-nspawn over rootless podman for flexibility and was
designed to make building custom images easy.

For starters, I'll steal the terminology from coretoolbox's readme and refer to coretoolbox
as "ctb" and the original toolbox as "dtb".

nsbox started out as a simple Python wrapper over systemd-nspawn that I created after running
into multiple bugs revolving around the toolbox script or rootless podman. I felt like there were
two main issues that kept coming up:

- Rootless podman, crun, and fuse-overlayfs are still somewhat young and will encounter bugs.
  This will of course be less of an issue as time passes.
- **OCI containers cannot be modified after they're created.** Neither Docker nor podman allow
  you to e.g. modify a container's mounts without destroying and re-creating the container.
  This was a particular pain point for me because, being on Silverblue, all of my development
  takes place in containers, but having to constanly re-create them was a huge time sink.

  This isn't necessarily a con for podman itself, because OCI containers were really designed
  with containers in mind that are created and destroyed quickly for the cloud. That's great,
  but it doesn't always work out so well for pet containers.

Therefore, when designing the current variant of nsbox, there were several core ideas I had in
mind:

- Host integration, e.g. by being able to export container desktop files to the host system.
  This was largely inspired by Chrome OS's
  [garcon](https://chromium.googlesource.com/chromiumos/platform2/+/master/vm_tools/garcon/)
  daemon used by
  [Crostini](https://chromium.googlesource.com/chromiumos/docs/+/master/containers_and_vms.md).
- Ability to use multiple different base images, built on Ansible playbooks. dtb seems to slowly
  be adding support for this.
- Modifying containers after creation. I don't believe this is something that podman will support,
  at least not for a long time. In short, this means that for podman, you can't e.g. customize
  mount points after container creation.
- Full root access. This will never be possible with rootless containers. The reason I wanted
  this was to be able to talk to the kernel netlink sockets and do other similar tasks via
  projects I was working on from inside the toolbox.

I didn't know about ctb until after nsbox was already a thing. It seems to readily solve a few
of the dtb issues, but there's still the lack of true root access and long-term issues with
being unable to modify containers

## Why this FAQ so empty?

Why do you care? -_-
