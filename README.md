
# DNS Propagation Checker

[![Build Status on master](https://travis-ci.com/jeliasson/dns-propagation-checker.svg?branch=master)](https://travis-ci.com/jeliasson/dns-propagation-checker) 

DNS Propagation Checker is a web application that allows you to check multiple DNS records against one or many Name Servers (NS). This can be useful before changing Name Servers for a Internet Domain from one hosting to another, by validating that a DNS zone resolves correctly on all resolvers before proceeding with the migration. 

## Features
- Check multiple DNS records against multiple resolvers
- Persistant configuration of records, servers and settings based of url
- Color indication of matching DNS records, even if they come back in different order.

You can run it locally, by using node or Docker, or try out the [online demo](https://dns-propagation-checker.eliasson.xyz) if it's functioning. 


![https://github.com/jeliasson/dns-propagation-checker/blob/dev/assets/img/screenshot.png?raw=true](https://github.com/jeliasson/dns-propagation-checker/blob/dev/assets/img/screenshot.png?raw=true)

### Build Status
|Branch|Status|
|--------|-------|
|`master`|[![Build Status on master](https://travis-ci.com/jeliasson/dns-propagation-checker.svg?branch=master)](https://travis-ci.com/jeliasson/dns-propagation-checker) |
| `dev` | [![Build Status on dev](https://travis-ci.com/jeliasson/dns-propagation-checker.svg?branch=dev)](https://travis-ci.com/jeliasson/dns-propagation-checker) |

## Start
First, clone this project and jump into the directory.
```bash
git clone https://github.com/jeliasson/dns-propagation-checker.git
cd dns-propagation-checker
```

Now start the application using either docker, docker-compose or node below. Regardless of choice, the application will by default listen on [`http://localhost:3000`](http://localhost:3000).

### Docker
Build and run a docker container
```
docker built -t dns-propagation-checker .
docker run -it -p 3000:3000 dns-propagation-checker
```

### Docker Compose
Build and run a docker container using docker-compose.
```bash
docker-compose up
```

### Node
```bash
yarn install
yarn start
```

## Contribute
Feel free to send your improvements by PR! ❤️

### Under the hood
First off, it's not pretty and was done in a hurry. It's using the linux utitlity [`dig`](https://en.wikipedia.org/wiki/Dig_(command)) for querying DNS servers, and using [Vue](https://www.vuejs.org) and [Nuxt](https://www.nuxtjs.com) to serve the front- and backend requests and [`shelljs`](https://github.com/shelljs/shelljs) to interact with dig. 

The client side application can be found in `/pages/index.vue`. It's saving the state/settings on client side by changing the url, containing the url parameter `settings` - which is essentially a base64 encoded object holding records and servers. For now, and just because we can, it's using this base64 object when querying the `/api/query` endpoint. 

On the server side of things in `/api/routes/query.js`, we take in posted data and making sure it's not containing any nasty user input. We construct a `dig` command, run it, check if there any difference between earlier resolves on that record, and finally respond with a JSON object.

### Todos
- Add maximum of servers and records that can be queries on a single run
- Soft firewall for Max Reqests
- Prettier interface

### Get started
Fork the project, clone it, and make awesome updates. Send your Pull Request to origin :)
```bash
yarn install
yarn dev            # Listening on localhost:3000 with hot reload
```

# Licence
[MIT](https://choosealicense.com/licenses/mit/)