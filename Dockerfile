# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/camplist ./cmd/web

# The stylesheet for Pines elements is built from the templates, so the
# templates are copied here as Tailwind's scan sources.
FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS assets
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund
COPY assets/ assets/
COPY internal/views/ internal/views/
COPY static/offline/ static/offline/
RUN npx tailwindcss -i assets/css/tailwind.css -o /out/tailwind.css --minify

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
WORKDIR /app
COPY --from=build /out/camplist /app/camplist
COPY static/ /app/static/
COPY --from=assets /out/tailwind.css /app/static/tailwind.css
USER nonroot:nonroot
EXPOSE 3000
ENTRYPOINT ["/app/camplist"]
