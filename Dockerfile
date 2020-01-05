FROM node:latest
WORKDIR /app

# Copy file deps
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

# Expose on port 3000
EXPOSE 3000

# Start nuxt
ENTRYPOINT [ "npx", "nuxt", "start" ]
