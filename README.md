# vault-database-plugin-Sybase
This repository contains an unofficial, unsupported [Vault](https://www.vaultproject.io) plugin for Sybase that can be used with Vault's [Database Secrets Engine](https://www.vaultproject.io/docs/secrets/databases).

It was tested with Sybase Adaptive Server Enterprise 16.0 SP03.

## Build

Before building, you will need to download the FreeTDS library, which is available from [FreeTDS](http://www.freetds.org/software.html). The libraries and headers should be installed at one of the standard locations for your platform (e.g. on macOS, `/usr/local/opt/freetds/include`&`/usr/local/opt/freetds/lib` or `~/include`&`~/lib`).

We will also use a fork of the https://github.com/minus5/gofreetds golang SQL driver which will talk to the http://www.freetds.org/ FreeTDS library, but this will be added to the go development environment when building the plugin.

To be specific, we use the fork, https://github.com/rberlind/gofreetds in which the following changes were made in order to get the Sybase plugin to work:
  * I do not append the `statusRowSybase125` query on [line 52](https://github.com/rberlind/gofreetds/blob/master/executesql.go#L52) of executesql.go.
  * I modified the `quote` function of executesql.go to not return an extra single quote. I essentially made the function do nothing other than return the string that had been passed to it. See this [commit](https://github.com/rberlind/gofreetds/commit/658b890bb1310f29a819c9a7355641f7c66afb74) .

### Building and Installing FreeTDS on Ubuntu
It is possible to install FreeTDS using this command:
```
sudo apt install freetds-bin
```

But, if you prefer to build and deploy it, Run the following commands to download and install FreeTDS on a Ubuntu server:
```
curl ftp://ftp.freetds.org/pub/freetds/stable/freetds-patched.tar.gz > freetds-patched.tar.gz
tar -xvf freetds-patched.tar.gz
cd freetds-1.1.36
sudo apt update
sudo apt install build-essential
./configure --enable-sybase-compat
sudo make
sudo make install
sudo make clean
```
Of course, the name of the directory containing FreeTDS will be different if you download a newer version. The most recent version is 1.1.36.

The FreeTDS libraries will be in /usr/local/lib and the headers will be in /usr/local/include.

You can run `tsql -C` to see the FreeTDS configuration. This should return something like this:
```
Compile-time settings (established with the "configure" script)
                            Version: freetds v1.00.104
             freetds.conf directory: /usr/local/etc
     MS db-lib source compatibility: no
        Sybase binary compatibility: yes
                      Thread safety: yes
                      iconv library: yes
                        TDS version: auto
                              iODBC: no
                           unixodbc: no
              SSPI "trusted" logins: no
                           Kerberos: no
                            OpenSSL: no
                             GnuTLS: no
                               MARS: no
```

### Setup a Go development Environment on Linux
See https://medium.com/@patdhlk/how-to-install-go-1-9-1-on-ubuntu-16-04-ee64c073cd79

```
curl -O https://dl.google.com/go/go1.10.3.linux-amd64.tar.gz
tar -xvf go1.10.3.linux-amd64.tar.gz
sudo mv go /usr/local
mkdir gowork
```

Edit ~/.profile and add the following at the bottom:
```
export GOROOT=/usr/local/go
export GOPATH=~/gowork
export PATH=$PATH:$GOROOT/bin
```

Run `source ~/.profile` to load it, or log out and back into your shell.
Run `cd $GOPATH` to cd into your go work directory.

### Add gox to Your Go Environment
We use the [gox](https://github.com/mitchellh/gox) cross compilation tool. Install this into youyr Go environment with these commands:
```
cd $GOPATH/src/github.com
go get github.com/mitchellh/gox
cd mitchellh/gox
go build
sudo cp gox /usr/local/bin/gox
```

### Clone the Sybase Plugin Repository
Use these commands to clone this repository into your Go environment:
```
cd $GOPATH/src/github.com
mkdir rberlind
cd rberlind
git clone https://github.com/rberlind/vault-plugin-database-sybase.git
cd vault-plugin-database-sybase/scripts/linux_amd64
```

Edit freetds.pc in the current directory and make sure that prefix is the correct directory containing the root of your FreeTDS installation.  If you built FreeTDS with the defaults (as done above), this will be /usr/local which means the FreeTDS libraries will be in /usr/local/lib and the headers will be in /usr/local/include.

### Build the Plugin
Install the gofreetds SQL driver and build the plugin with these commands:
```
cd $GOPATH/src/github.com/rberlind/vault-plugin-database-sybase
export VAULT_DEV_BUILD=linux
go get -u
scripts/build.sh
```
The `go get` command gets the gofreetds files from the fork in https://github.com/rberlind/gofreetds.

The linux binary should be built in the bin directory.

Get the SHA 256 Checksum:
`shasum -a 256 bin/vault-plugin-database-sybase`
This should give something like this:
```
5441f0fac44d04721528aac3bddb0162825140ac3bb34175333a14334fb38a5b  vault-plugin-database-sybase
```

SCP the binary to your Vault server and also install FreeTDS on your Vault server.

## Installation of the Sybase Plugin on Your Vault Server
The Vault plugin system is documented on the [Vault documentation site](https://www.vaultproject.io/docs/internals/plugins.html).

You will need to define a plugin directory using the `plugin_directory` stanza in your Vault server's configuration file and then copy the `vault-plugin-database-sybase` executable generated above to that directory.

Please run the following command against the plugin binary after copying it ot the plugin directory:
```
sudo setcap cap_ipc_lock=+ep <vault_plugins_directory>/vault-plugin-database-sybase
```

Enable an instance of Vault's [Database secrets engine](https://www.vaultproject.io/docs/secrets/databases) on the path "sybase" with this command:
```
vault secrets enable -path=sybase database
```

Register the plugin using a command like this:
```
vault write sys/plugins/catalog/database/vault-plugin-database-sybase \
    sha_256=<plugin_checksum> \
    command="vault-plugin-database-sybase"
```
replacing <plugin_checksum\> with the SHA256 checksum you got from running `shasum -a 256` against the plugin binary.

Next, you need to edit the FreeTDS configuration file. There appear to be two diles you can edit: /usr/local/etc/freetds.conf /home/ubuntu/.freetds.conf.

And there are two ways of formatting the file. We show both here:
/usr/local/etc/freetds.conf:
```
[sybase]
        Description=Sybase ASE 12.5 or 15 Server
        ServerName=roger-sybase
        Database=vault
```

/home/ubuntu/.freetds.conf"
```
[roger-sybase]
        host = 18.235.108.175
        port = 5000
        tds version = 5.0
        database vault
```
It is also possible that both files are needed.

Next, you need to create the configuration and a role for the Sybase plugin with commands like these:
```
vault write sybase/config/sybase plugin_name=sybase-database-plugin connection_url='Server=roger-sybase; User Id=sa;Password=<password>; Database=master; App name=vault; compatibility_mode=sybase_12_5' allowed_roles="test"

vault write sybase/roles/test db_name=sybase creation_statements="Use master; CREATE LOGIN {{name}} WITH PASSWORD {{password}} DEFAULT DATABASE vault; USE vault; sp_adduser {{name}};" default_ttl="1h" max_ttl="24h"
```
Of course, you'll need to provide the actual password for the sa user of your Sybase server. You could also use a different user as long as that user can create other users.

We're assuming here that you are connecting to a database server called "roger-sybase" and that that server has a database called "vault".

## Generating Sybase Credentials
If you are able to register your plugin and run the above Vault commands to configure it, you should now be able to dynamically generate credentials for the vault database on your Sybase server that are good for 1 hour with this command:
```
vault read sybase/creds/test
```
This should return something like this:
```
Key                Value
---                -----
lease_id           database/creds/test/ebf8d57c-35f5-485a-e867-7f66b6b44fd0
lease_duration     1h
lease_renewable    true
password           A1a_2CVqMTOCWIzwwhVp
username           v_root_test_4qvNqxvgHfsiEpQGXS
```
Here we see that the Sybase plugin generated a user "v_root_test_4qvNqxvgHfsiEpQGXS" with password "A1a_2CVqMTOCWIzwwhVp".

You can test the generated credentials with `tsql -S roger-sybase -U <user> -P <password>` which should let you connect to the database.

You can then run a query like this:
```
1> select name from sysusers
2> go
```

Vault will delete the user after 1 hour.

## Rotating the Root Credential
The password of the user passed to the sybase/config/sybase path can be rotated with this command:
```
vault write -f database/rotate-root/sybase
```
