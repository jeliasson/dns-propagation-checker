FROM node:latest

WORKDIR /app
COPY api api
COPY assets assets
COPY components components
COPY layouts layouts
COPY pages pages
COPY plugins plugins
COPY static static
COPY nuxt.config.js nuxt.config.js
COPY package.json package.json
COPY yarn.lock yarn.lock

RUN apt-get update && apt-get -y install dnsutils

RUN yarn

RUN chmod -R 777 /app/node_modules/.cache

EXPOSE 3000

ENTRYPOINT [ "yarn", "start" ]
