# syntax=docker/dockerfile:1

FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend
WORKDIR /src/backend
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN go build -trimpath -ldflags="-s -w" -o /out/expense ./cmd/expense

FROM alpine:3.20
# OpenShift runs as a random UID in group 0, so /data must be group-writable.
RUN addgroup -S app && adduser -S app -G app && mkdir -p /data && chown app:0 /data && chmod 775 /data
WORKDIR /app
COPY --from=backend /out/expense /app/expense
COPY --from=frontend /src/frontend/dist /app/static
USER app
ENV EXPENSE_DB=/data/expense.db EXPENSE_STATIC=/app/static EXPENSE_ADDR=:8080 EXPENSE_SEED=true
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/expense"]
CMD ["serve"]
