# Stage 1: Build display_control_web
FROM node:20-alpine AS display_web_builder
WORKDIR /app
RUN npm install -g pnpm
COPY display_control_web/package.json display_control_web/pnpm-lock.yaml* ./
RUN pnpm install
COPY display_control_web/ ./
RUN pnpm run build-prod
# Remove non-serving files to prepare for a clean copy
RUN rm -rf node_modules src package.json pnpm-lock.yaml tsconfig.json pnpm-workspace.yaml

# Stage 2: Build meeting_control_web
FROM node:20-alpine AS meeting_web_builder
WORKDIR /app
RUN npm install -g pnpm
COPY meeting_control_web/package.json meeting_control_web/pnpm-lock.yaml* ./
RUN pnpm install
COPY meeting_control_web/ ./
RUN pnpm run build
# Remove non-serving files to prepare for a clean copy
RUN rm -rf node_modules src package.json pnpm-lock.yaml tsconfig.json eslint.config.mjs .prettierrc GEMINI.MD pnpm-workspace.yaml

# Stage 3: Build display_server Go binary
FROM golang:1.22-alpine AS go_builder
WORKDIR /app
COPY display_server/main.go .
# Compile a statically linked Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o display_server main.go

# Stage 4: Final distroless image
FROM gcr.io/distroless/static-debian12
WORKDIR /app

# Copy the go binary to the root of the workdir
COPY --from=go_builder /app/display_server /app/display_server

# Copy the built display_control_web files to /app/public
COPY --from=display_web_builder /app /app/public

# Copy the built meeting_control_web files to a separate folder inside /app/public
COPY --from=meeting_web_builder /app /app/public/meeting_control_web

# Distroless launches the display_server serving the public folder
ENTRYPOINT ["/app/display_server", "-dir", "/app/public"]
