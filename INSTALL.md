<!-- DO NOT EDIT — managed by hugoh/go-tools via copier. -->

# Installing hrd

## Homebrew (macOS/Linux)

```sh
brew install hugoh/tap/hrd
```

## Debian/Ubuntu (apt)

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/hugoh/apt/setup.deb.sh' | sudo -E bash
sudo apt install hrd
```

## RHEL/Fedora (dnf)

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/hugoh/rpm/setup.rpm.sh' | sudo -E bash
sudo dnf install hrd
```

Package repository hosting is graciously provided by
[Cloudsmith](https://cloudsmith.com).

## Package files

Download the `.deb` or `.rpm` from the
[releases page](https://github.com/hugoh/hrd/releases) and install it
with your package manager:

```sh
# Debian/Ubuntu
sudo apt install ./hrd_*.deb

# RHEL/Fedora
sudo dnf install ./hrd_*.rpm
```

## mise

```sh
mise use -g github:hugoh/hrd
```
