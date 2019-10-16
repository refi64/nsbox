# Creating custom images

## Concepts

Images on nsbox are built around the idea of *retroactive updates*: most enhancements to the
base image can be retroactively applied to containers already built from that image.

In order to facilitate this, an "image" in nsbox is a directory containing two things:

- An [Ansible](https://www.ansible.com/)
  [playbook](https://docs.ansible.com/ansible/latest/user_guide/playbooks.html).
- A metadata file, which references an OCI image prebuilt from the above playbooks.

A custom tool built on top of
[ansible-bender](https://github.com/ansible-community/ansible-bender) is used to build the OCI
images.

Whenever the image files are updated, nsbox will run the playbook, but this time on the created
container instead of using ansible-bender.

## Building a derived image

### Directory layout

In this example, we'll build a custom image, `fedora-custom`, that's derived from the Fedora
image but with a few extra packages we'd want to use.

To get started, create a directory that will contain both the image directory itself as well
as a README:

```bash
$ mkdir fedora-custom-example
$ cd fedora-custom-example

$ mkdir fedora-custom
$ echo rtfm > README.md
```

In this case, `fedora-custom` is going to be the image.

### The playbook

Inside of `fedora-custom`, create `playbook.yaml` and place the following inside:

```yaml
- hosts: all
  gather_facts: false
  vars:
    ansible_bender:
      layering: false

  tasks:
    - name: Install useful packages
      dnf:
        name:
          - zsh

    - name: Clear the dnf cache
      shell: 'dnf clean all'
      args:
        warn: false
      tags: bend
```

This file is a pretty standard Ansible playbook, with a few things of note:

- `hosts: all` is used so that this playbook can be used to both build the OCI images and
  run on a created container.
- `gather_facts: false` is not particularly relevant to nsbox-bender/ansible-bender, but it will
  make running the playbook on a created container drastically faster. (Fact-gathering is more
  useful when manipulating stuff over a network, which doesn't apply here.)
- Our task that clears the dnf cache has `tags: bend`. This uses ansible-playbook's
  [tag system](https://docs.ansible.com/ansible/latest/user_guide/playbooks_tags.html) to
  specify tasks that should only be run when creating an image.

  When the playbook is running on a created container, it will skip any tasks tagged with "bend".
  This is used here to clear the dnf cache when building OCI images (to make them smaller), but
  it won't clear the cache on an already-created container (where the user probably won't want
  the cache to be cleared every time the playbook is updated).

### The metadata

nsbox also needs a metadata.json file:

```json
{
  "parent": "fedora:{image_tag}",
  "base": "registry.nsbox.dev/fedora:{nsbox_branch}-{image_tag}",
  "target": "nsbox-fedora-custom:{image_tag}",
  "valid_tags": ["30", "31"]
}
```

Here is the meaning of the different keys:

- `parent` is the parent image this one is derived from. In this case, the parent image is
  the base `fedora` image.
- `base` is the location of base image's OCI image. You can find this by looking at the base
  image's metadata.json. (The separation here between parent and base is for both flexibility
  and also to make it easier to build child images without the parent's metadata actually
  being installed.)
- `valid_tags` is a list of tags that the image supports. (A tag is the part after the colon in
  an image name, e.g. given `fedora:30`, the tag is `30`.) If you image accepts no tags (for
  instance, an Arch image has no use for them), simply set `valid_tags` to an empty array.

  Do note that the concept of a tag for an nsbox image is a bit different than a tag for an OCI
  image. In OCI images, the tag is the entire image name, but in nsbox, it's only the part after
  the colon.
- `target` is the target OCI image name for our image.

Do also note that we can use some useful substitutions in our metadata:

- `{image_tag}` is the image tag being used.
- `{nsbox_branch}` is the nsbox release branch (*stable* or *edge*).
- `{nsbox_product_name}` is the "product name" of this nsbox build: *nsbox* for a stable build
  and *nsbox-edge* for an edge build.
- `{nsbox_version}` is the nsbox version number.

### Building the image

To build the image, run this from `fedora-custom-example`:

```bash
$ nsbox-edge-bender fedora-custom:30 -x fedora-custom.tar
```

The first argument is the image name and tag to build. The part before the colon is also
resolved as the image directory relative to the current directory, e.g. if we wanted to build
fedora-custom from another directory, you can do `nsbox-edge-bender the/path/to/image:30 ...`.

`-x` just will also export the generated tarball of the image. This is used to import it into
nsbox.

### Installing the image metadata

nsbox looks in two locations for the image metadata; the one intended for user-installed images
is `/etc/nsbox/images`. In order to ease installation, this install script can be used to
install the image files (place it at `fedora-custom-example/install.sh` and mark as executable):

```bash
#!/bin/bash

if [[ "$UID" != "0" ]]; then
  echo 'This must be run as root.'
  exit 1
fi

set -ex
image=fedora-custom
rm -rf /etc/nsbox/images/$image/*
mkdir -p /etc/nsbox/images/$image
cp -a $image/* /etc/nsbox/images/$image
```

Now you can run `sudo ./install.sh` to install the files.

::: tip
`rm -rf /etc/nsbox/images/$image/*` is preferred over just `rm -rf /etc/nsbox/images/$image` due
to [this](faq.md#why-do-i-get-the-playbook-could-not-be-found-when-i-update-my-images).
:::

To confirm the image is there, run `nsbox images`; it should now show the new image in addition
to the pre-configured ones.

### Testing the image

nsbox can only grab download OCI images from remote container registries, not local container
storage. Therefore, nsbox needs to be given the tarball that was generated above in order to
create a container:

```bash
$ nsbox create -tar fedora-custom.tar fedora-custom:30 my-container
```

This will create a container using the fedora-custom image named `my-container`.

### Remote images

In the above example, the image was stored and imported locally. This isn't particularly useful
if you want to share your images with other users.

In order to remedy this, you'll need to upload your OCI image to some online service (Google
Container Registry is what is used for the official nsbox images, note that quay.io does *not*
work). Then, add the remote URL to the `"remote"` key of the metadata.json:

```json
{
  "parent": "fedora:{image_tag}",
  "base": "registry.nsbox.dev/fedora:{nsbox_branch}-{image_tag}",
  "remote": "docker.io/a-user/fedora-custom:{image_tag}",
  "target": "nsbox-fedora-custom:{image_tag}",
  "valid_tags": ["30", "31"]
}
```

Now, if `-tar` is omitted from `nsbox create`, then nsbox will check the remote mentioned in
your metadata file.

In addition, if if `target` is removed but `remote` is present, then `remote` will be used as the
target name. This metadata file will generated an OCI image with the remote name:

```json
{
  "parent": "fedora:{image_tag}",
  "base": "registry.nsbox.dev/fedora:{nsbox_branch}-{image_tag}",
  "remote": "docker.io/a-user/fedora-custom:{image_tag}",
  "target": "nsbox-fedora-custom:{image_tag}",
  "valid_tags": ["30", "31"]
}
```

## Building an image from scratch

TODO
