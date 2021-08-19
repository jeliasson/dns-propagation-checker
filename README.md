# DNS Propagation Checker

> :warning: This repository has been discontinued. Feel free to take inspiration and build something better. <3

[![Build Status on master](https://travis-ci.com/jeliasson/dns-propagation-checker.svg?branch=master)](https://travis-ci.com/jeliasson/dns-propagation-checker) [![Issues](https://img.shields.io/github/issues-raw/jeliasson/dns-propagation-checker)](https://github.com/jeliasson/dns-propagation-checker)

DNS Propagation Checker is a web application that allows you to check multiple DNS records against one or many Name Servers (NS). This can be useful before changing Name Servers for a Internet Domain from one hosting to another, by validating that a DNS zone resolves correctly on all resolvers before proceeding with the migration.

## Features

-   Check multiple DNS records against multiple resolvers
-   Persistant configuration of records, servers and settings based of url
-   Color indication of matching DNS records, even if they come back in different order.

You can run it locally using [docker](#Docker), [docker-compose](#Docker-Compose), [node](#Node) or try out the [online demo](https://dns-propagation-checker.eliasson.xyz).

![https://github.com/jeliasson/dns-propagation-checker/blob/dev/src/assets/img/screenshot.png?raw=true](https://github.com/jeliasson/dns-propagation-checker/blob/dev/src/assets/img/screenshot.png?raw=true)

### Build Status

| Branch   | Status                                                                                                                                                          |
| -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `master` | [![Build Status on master](https://travis-ci.com/jeliasson/dns-propagation-checker.svg?branch=master)](https://travis-ci.com/jeliasson/dns-propagation-checker) |
| `dev`    | [![Build Status on dev](https://travis-ci.com/jeliasson/dns-propagation-checker.svg)](https://travis-ci.com/jeliasson/dns-propagation-checker)                  |

## Start

Start the application using either [docker](#Docker), [docker-compose](#Docker-Compose) or [node](#Node) below. Regardless of choice, the application will by default listen on [`http://localhost:3000`](http://localhost:3000).

### Docker

Run `jeliasson/dns-propagation-checker` which is build on [Docker Hub](https://hub.docker.com/r/jeliasson/dns-propagation-checker).

```docker
docker run -it -p 3000:3000 jeliasson/dns-propagation-checker
```

### Docker - Build & Run

Build and run a docker container

```bash
# Clone repo
git clone https://github.com/jeliasson/dns-propagation-checker.git

# Enter created directory
cd dns-propagation-checker

# Build
docker built -t dns-propagation-checker .

# Start
docker run -it -p 3000:3000 dns-propagation-checker
```

### Docker Compose

Build and run a docker container using docker-compose.

```bash
# Clone repo
git clone https://github.com/jeliasson/dns-propagation-checker.git

# Enter created directory
cd dns-propagation-checker

# Start
docker-compose up
```

### Node

Make sure that you have the Linux utility `dig` installed. See [Under the hood](#Under-the-hood) for more details.

```bash
# Clone repo
git clone https://github.com/jeliasson/dns-propagation-checker.git

# Enter created directory
cd dns-propagation-checker

# Install yarn deps
yarn install

# Start
yarn start
```

## Contribute

Feel free to send your improvements by PR! ❤️

### Under the hood

First off, it's not pretty and was done in a hurry. It's using the Linux utitlity [`dig`](<https://en.wikipedia.org/wiki/Dig_(command)>) for querying DNS servers, and using [Vue](https://www.vuejs.org) and [Nuxt](https://www.nuxtjs.com) to serve the front- and backend requests and [`shelljs`](https://github.com/shelljs/shelljs) to interact with dig.

The client side application can be found in `/pages/index.vue`. It's saving the state/settings on client side by changing the url, containing the url parameter `settings` - which is essentially a base64 encoded object holding records and servers. For now, and just because we can, it's using this base64 object when querying the `/api/query` endpoint.

On the server side of things in `/api/routes/query.js`, we take in posted data and making sure it's not containing any nasty user input. We construct a `dig` command, run it, check if there any difference between earlier resolves on that record, and finally respond with a JSON object.

Requests to `/api/query` is being rate limited to `10 requests` over `60 seconds`.

### Todos

-   Add maximum of servers and records that can be queries on a single run
-   Enable/Disable server and record (server side processing)
-   Prettier interface

### Get started

Fork the project, clone it, and make awesome updates. Send your Pull Request to origin :)

```bash
# Clone repo
git clone https://github.com/jeliasson/dns-propagation-checker.git

# Enter created directory
cd dns-propagation-checker

# Enter source directory
cd src

# Install yarn deps
yarn install

# Start
yarn dev            # Listening on localhost:3000 with hot reload
```

# Licence

[MIT](https://choosealicense.com/licenses/mit/)
