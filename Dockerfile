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

# Install 'dig'
RUN apt-get update && apt-get -y install dnsutils

# Install yarn deps
RUN yarn

# Build Nuxt
RUN npx nuxt build

# Addressing permissions issue. It's ugly and insecure, but it works.
# FATAL  EACCES: permission denied, mkdir '/app/node_modules/.cache'
#RUN chmod -R 777 /app

EXPOSE 3000

ENTRYPOINT [ "npx", "nuxt", "start" ]
