FROM node:latest
WORKDIR /app

# Copy file deps
COPY src/api api
COPY src/assets assets
COPY src/components components
COPY src/layouts layouts
COPY src/pages pages
COPY src/plugins plugins
COPY src/static static
COPY src/nuxt.config.js nuxt.config.js
COPY src/package.json package.json
COPY src/yarn.lock yarn.lock

# Install 'dig'
RUN apt-get update && apt-get -y install dnsutils

# Install yarn deps
RUN yarn

# Build Nuxt
RUN npx nuxt build

# Expose on port 3000
EXPOSE 3000

# Start nuxt
ENTRYPOINT [ "npx", "nuxt", "start" ]
