# Linux

The Linux ABI consists of several special file paths and syscalls.
Most applications targeted at Linux will require additional parts of the environment to be set up to run correctly.

This document attempts to define a minimal set of things that must exist for most applications.

ACIs that define the label `os=linux` can expect this environment by default.

## Devices and File Systems

|     Path     |  Type  |  Notes  |
| ------------ | ------ | ------- |
| /proc        | [procfs](https://www.kernel.org/doc/Documentation/filesystems/sysfs.txt)   | |
| /sys         | [sysfs](https://www.kernel.org/doc/Documentation/filesystems/proc.txt)     | |
| /dev/null    | [device](http://man7.org/linux/man-pages/man4/null.4.html)                 | |
| /dev/zero    | [device](http://man7.org/linux/man-pages/man4/zero.4.html)                 | |
| /dev/full    | [device](http://man7.org/linux/man-pages/man4/full.4.html)                 | |
| /dev/random  | [device](http://man7.org/linux/man-pages/man4/random.4.html)               | |
| /dev/urandom | [device](http://man7.org/linux/man-pages/man4/random.4.html)               | |
| /dev/tty     | [device](http://man7.org/linux/man-pages/man4/tty.4.html)                  | |
| /dev/console | [device](http://man7.org/linux/man-pages/man4/console.4.html)              | |
| /dev/pts     | [devpts](https://www.kernel.org/doc/Documentation/filesystems/devpts.txt)  | |
| /dev/ptmx    | [device](https://www.kernel.org/doc/Documentation/filesystems/devpts.txt)  | Bind-mount or symlink of /dev/pts/ptmx |
| /dev/shm     | [tmpfs](https://www.kernel.org/doc/Documentation/filesystems/tmpfs.txt)    | |
