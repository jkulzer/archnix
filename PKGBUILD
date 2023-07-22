# Maintainer: jkulzer <kulzer dot johannes at tutanota dot com>
pkgname=archnix-bin
pkgver=0.5.4
pkgrel=1
pkgdesc="A tool to manage Pacman and AUR packages declaratively"
arch=(x86_64)
url="https://github.com/jkulzer/archnix"
license=('GPL')
groups=()
depends=()
makedepends=('go')
optdepends=()
provides=()
conflicts=()
replaces=()
backup=()
options=()
install=
changelog=
source=(https://github.com/jkulzer/archnix/releases/download/$pkgver/archnix)
noextract=()

package() {
	mkdir -p $pkgdir/usr/bin
	cp $srcdir/archnix $pkgdir/usr/bin/archnix
}
sha256sums=('305d330739cb42288f307ef54fccd51a102aafbdc4ece9f459012c5bf35c7c05')
