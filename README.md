# vault-database-plugin-Sybase

A [Vault](https://www.vaultproject.io) plugin for Sybase

This project uses the database plugin interface introduced in Vault version 0.7.1.

## Build

For linux/amd64, pre-built binaries can be found at [the releases page](https://releases.hashicorp.com/vault-plugin-database-sybase/)

For other platforms, there are not currently pre-built binaries available.

Before building, you will need to download the FreeTDS library, which is available from [FreeTDS](http://www.freetds.org/software.html). The libraries and headers should be installed at one of the standard locations for your platform (e.g. on macOS, `/usr/local/opt/freetds/include`&`/usr/local/opt/freetds/lib` or `~/include`&`~/lib`).

Next, create a [`pkg-config`](https://www.freedesktop.org/wiki/Software/pkg-config/) file to point to the library. Create the file `freetds.pc` on your `PKG_CONFIG_PATH`.

An example `freetds.pc` for macOS is:

```
prefix=/usr/local/opt/freetds

version=1.00.104
build=client64

libdir=${prefix}/lib
includedir=${prefix}/include

Name: freetds
Description: FreeTDS Library
Version: ${version}
Libs: -L${libdir} -lclntsh
Libs.private:
Cflags: -I${includedir}
```

Then, `git clone` this repository into your `$GOPATH` and `go build -o vault-plugin-database-sybase ./plugin` from the project directory.

`make test` will run a basic test suite against a Docker version of Sybase.

## Installation

The Vault plugin system is documented on the [Vault documentation site](https://www.vaultproject.io/docs/internals/plugins.html).

You will need to define a plugin directory using the `plugin_directory` configuration directive, then place the `vault-plugin-database-sybase` executable generated above in the directory.

Register the plugin using

```
vault write sys/plugins/catalog/vault-plugin-database-sybase \
    sha_256=<expected SHA256 Hex value of the plugin binary> \
    command="vault-plugin-database-sybase"
```
