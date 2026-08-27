FROM node:24-alpine AS build
WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# Vite inlines env vars at build time, so the API base URL is baked into the
# bundle here rather than read at runtime. Overridable per-build.
ARG VITE_API_BASE=https://api.theflockwatcher.com
ARG VITE_GOOGLE_CLIENT_ID=51742040569-nvihh5edjh8o168i14k8jvek47j0gem6.apps.googleusercontent.com
ENV VITE_API_BASE=$VITE_API_BASE
ENV VITE_GOOGLE_CLIENT_ID=$VITE_GOOGLE_CLIENT_ID
RUN npm run build

FROM caddy:2-alpine
COPY --from=build /app/dist /srv
COPY Caddyfile /etc/caddy/Caddyfile
EXPOSE 8080
