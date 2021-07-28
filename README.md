Portainer SSH
===========

Native shell client for Portainer containers, provided a powerful native terminal to manage your Docker containers.

* It's dead simple. like the ssh cli, you do `portainerssh container_name` to SSH into any containers
* It's flexible. `portainerssh` reads configurations from ENV, from yml or json file
* It's powerful. `portainerssh` searches the whole Portainer deployment, open shell into any containers from your
  workstation, regardless which host it belongs to
* It's smart. `portainerssh` uses fuzzy container name matching. Forget the container name? it doesn't matter, use "*"
  or "%" instead

Is it really an SSH client?
============
No. It's called so for historical purposes. It _acts_ like SSH in terms of providing you shell access to your
containers. Also SSH is what people are likely googling for.


Installation
============

**Via Golang**

`# go get github.com/devbranch-vadym/portainerssh`

**Binary builds**

Sorry, not there yet.

Usage
=====

`portainerssh [<flags>] <container>`

Example
=======

```
portainerssh my-container-name
```

Configuration
=============

The configuration could be read from `config.json` or `config.yml` in `./`, `/etc/portainerssh/` or `~/.portainerssh/` folders.

If you want to use JSON format, create a `config.json` in the folders with content:

```json
{
  "api_url": "https://portainerssh.server/api",
  "user": "your_access_key",
  "password": "your_access_password"
}
```

If you want to use YAML format, create a `config.yml` with content:

```yml
api_url: https://your.portainer.server/api
user: your_access_key
password: your_access_password
```

We accept environment variables as well:

```shell
PORTAINER_API_URL=https://your.portainer.server/api
PORTAINER_USER=your_access_key
PORTAINER_PASSWORD=your_access_password
```

Flags
=====

```
  -h, --help         Show context-sensitive help (also try --help-long and --help-man).
      --version      Show application version.
      --api_url=""   Portainer server API URL, https://your.portainer.server/api .
      --user=""      Portainer API user/accesskey.
      --password=""  Portainer API password/secret.
```

**Args**

`<container>  Container name, fuzzy match`

Limitations
=====
Currently only first Docker instance is supported.

History
=====
`portainerssh` is based on wonderful `rancherssh` utility by Fang Li. In fact, `portainerssh` is a fork and partial
rewrite of `rancherssh`, just for Portainer.
