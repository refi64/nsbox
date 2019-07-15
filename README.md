# nsbox

```
usage: nsbox [-h] {create,run,kill,import} ...

nsbox is a lightweight, root/sudo-based alternative to the rootless toolbox
script, build on top of systemd-nspawn instead of podman. This gives it
several advantages, such as fewer bugs, a more authentic host experience, and
no need to ever recreate a container in order to take advantage of newer
changes.

positional arguments:
  {create,run,kill,import}
    create              Create a new container
    run                 Run a command inside the container
    kill                Kill a container
    import              Import the packages from a rootless toolbox

optional arguments:
  -h, --help            show this help message and exit
```
