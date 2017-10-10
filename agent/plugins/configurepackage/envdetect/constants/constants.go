package constants

// PlatformFamily marks a family of similar operating systems

// PlatformFamilyWindows uses Ohai identifier for windows platform family
const PlatformFamilyWindows = "windows"

// PlatformFamilyDarwin uses Ohai identifier for darwin platform family
const PlatformFamilyDarwin = "mac_os_x"

// PlatformFamilyDebian uses Ohai identifier for debian platform family
const PlatformFamilyDebian = "debian"

// PlatformFamilyRhel uses Ohai identifier for rhel platform family
const PlatformFamilyRhel = "rhel"

// PlatformFamilyFedora uses Ohai identifier for fedora platform family
const PlatformFamilyFedora = "fedora"

// PlatformFamilyAlpine uses Ohai identifier for alpine platform family
const PlatformFamilyAlpine = "alpine"

// PlatformFamilySuse uses Ohai identifier for opensuse platform family
const PlatformFamilySuse = "suse"

// PlatformFamilyGentoo uses Ohai identifier for gentoo linux platform family
const PlatformFamilyGentoo = "gentoo"

// PlatformFamilyArch uses Ohai identifier for arch linux platform family
const PlatformFamilyArch = "arch"

// Platform marks a specific operating systems

// PlatformDebian uses Ohai identifier for debian platform
const PlatformDebian = "debian"

// PlatformUbuntu uses Ohai identifier for ubuntu platform
const PlatformUbuntu = "ubuntu"

// PlatformRaspbian uses Ohai identifier for raspbian platform
const PlatformRaspbian = "raspbian"

// PlatformRedhat uses Ohai identifier for redhat platform
const PlatformRedhat = "redhat"

// PlatformCentos uses Ohai identifier for centos platform
const PlatformCentos = "centos"

// PlatformFedora uses Ohai identifier for fedora platform
const PlatformFedora = "fedora"

// PlatformAmazon uses Ohai identifier for amazon platform
const PlatformAmazon = "amazon"

// PlatformAlpine uses Ohai identifier for alpine platform
const PlatformAlpine = "alpine"

// PlatformSuse uses Ohai identifier for suse platform
const PlatformSuse = "suse"

// PlatformOpensuse uses Ohai identifier for opensuse platform version < 42
const PlatformOpensuse = "opensuse"

// PlatformOpensuseLeap uses Ohai identifier for amazon platform version >= 42
const PlatformOpensuseLeap = "opensuseleap"

// PlatformGentoo uses Ohai identifier for gentoo platform
const PlatformGentoo = "gentoo"

// PlatformArch uses Ohai identifier for arch platform
const PlatformArch = "arch"

// PlatformWindows uses Ohai identifier for windows platform
const PlatformWindows = "windows"

// PlatformDarwin uses Ohai identifier for darwin platform
const PlatformDarwin = "mac_os_x"

// OperatingSystemSKUs to denote Windows Nano installations
const SKUProductDatacenterNanoServer = "143"
const SKUProductStandardNanoServer = "144"

// Init marks a init system used by the Operating Sysstem

// InitSystemd uses identifier for systemd init system
const InitSystemd = "systemd"

// InitUpstart uses identifier for upstart init system
const InitUpstart = "upstart"

// InitChkconfig uses identifier for chkconfig init system (RHEL)
const InitChkconfig = "chkconfig"

// InitUpdatercd uses Ohai identifier for update-rc.d init system (Debian)
const InitUpdatercd = "updatercd"

// InitOpenrc uses identifier for openrc init system (Gentoo)
const InitOpenrc = "openrc"

// InitService uses identifier for undetected init systems but available
// `service` command to start/stop/restart services
const InitService = "service"

// InitDocker uses docker identifier for systems running inside of docker
// Those systems typically don't use the system init system and instead using a
// shell (bash, sh), a supervisor (runit, supervisord) or just a arbitrary
// command as pid1. Any service control would not work as for systems outside
// of docker.
const InitDocker = "docker"

// InitWindows uses windows identifier for windows init system
const InitWindows = "windows"

// InitLaunchd uses launchd for mac os x init system
const InitLaunchd = "launchd"

// constants for package manager used by the operating system
//
// multiple package manager might be installed but only the main manager for
// the platform is relevant.
//
// Ohai does not detect the package manager so using Ohai identifier does not
// work.
//
// Linux often have a low level package manager (for install, uninstall) and a
// high level package manager (for fetching and dependencies). (dpkg -> apt,
// rpm -> yum, rpm -> dnf, rpm -> zypper, portage -> emerge). Because its easy
// to determine the lower level package manager from the higher level but not
// the other way round we use high level package managers for detection.

// PackageManagerMac is always `mac_os_x` when running on Mac OS X. There are
// multiple competing package formates (.pkg, .dmg, brew, ...) so it depends on
// the use case.
const PackageManagerMac = "mac_os_x"

// PackageManagerWindows is always `windows` when running on Windows systems.
// There are multiple competing package formates (.msi, .exe, chocolatey, ...)
// so it depends on the use case.
const PackageManagerWindows = "windows"

// PackageManagerApt is used on Debian platform families (ubuntu, mate, ...)
const PackageManagerApt = "apt"

// PackageManagerYum is used on RHEL platform families (centos, red hat, amazon, ...)
const PackageManagerYum = "yum"

// PackageManagerPacman is used on ArchLinux
const PackageManagerPacman = "pacman"

// PackageManagerZipper is used on SuSe platform families (OpenSuse, SLES, ...)
const PackageManagerZipper = "zypper"

// PackageManagerAlpine is used on Alpine Linux
const PackageManagerAlpine = "alpine"

// PackageManagerDnf is used on Fedora
const PackageManagerDnf = "dnf"

// PackageManagerEmerge is used on Gentoo platform families (Gentoo, Funtoo, ...)
const PackageManagerEmerge = "emerge"
